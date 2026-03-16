# 認証の不備（Authentication Bypass）

## 1. 概要

認証の不備とは、Webアプリケーションにおけるユーザー認証の仕組みに脆弱性が存在し、攻撃者が正規の認証プロセスを迂回してシステムにアクセスできてしまう状態を指す。

認証に関する一般的な脆弱性には以下のものがある。

- **ブルートフォース攻撃**: パスワードの総当たり攻撃。レート制限がないシステムでは、攻撃者が自動化ツールで大量のパスワードを試行できる
- **平文パスワードの保存**: パスワードをハッシュ化せずに保存すると、データベースの漏洩時にすべてのパスワードが直接露出する
- **セッション固定攻撃**: ログイン前後でセッションIDが変更されない場合、攻撃者が事前に設定したセッションIDを使って認証済みセッションを乗っ取れる

## 2. 攻撃の仕組み

### ブルートフォース攻撃

攻撃者は一般的に使われるパスワードの辞書（例: `password123`, `admin`, `letmein` など）を使い、自動化スクリプトでログインを大量に試行する。レート制限がない場合、秒間数百〜数千回のリクエストを送信でき、弱いパスワードは短時間で突破される。

```
攻撃者                              脆弱サーバー
  |-- POST /login (admin/password)     -->|  401
  |-- POST /login (admin/123456)       -->|  401
  |-- POST /login (admin/password123)  -->|  200 (成功!)
  |   ... レート制限なし、何回でも試行可能  |
```

### セッション固定攻撃

攻撃者がセッションIDを事前に取得または設定し、被害者がそのセッションIDでログインすると、攻撃者も同じセッションIDで認証済みの状態になる。

```
攻撃者                  被害者               サーバー
  |-- セッションID取得 ----------------------->|
  |<--- session_id=ABC ----------------------|
  |-- session_id=ABCを被害者に渡す -->|       |
  |                     |-- ログイン(session_id=ABC) -->|
  |                     |<--- 認証成功(session_id=ABC) -|
  |-- session_id=ABCで管理画面にアクセス ----->|
  |<--- 認証済みとして応答 -------------------|
```

### 平文パスワードの危険性

パスワードが平文で保存されている場合、SQLインジェクションやデータベースの不正アクセスにより全ユーザーのパスワードがそのまま漏洩する。さらに、ユーザーが同じパスワードを複数サービスで使い回していれば、他のサービスへの不正アクセスにも利用される。

## 3. デモ環境の起動手順

```bash
make auth-bypass
```

このコマンドで以下のサービスが起動する。

| サービス | URL | 説明 |
|---------|-----|------|
| 脆弱版バックエンド | http://localhost:8086 | 平文パスワード、レート制限なし、セッション固定に脆弱 |
| 対策版バックエンド | http://localhost:8087 | bcryptハッシュ、レート制限あり、セッション再生成 |
| 脆弱版フロントエンド | http://localhost:3006 | 脆弱性を確認できるデモ画面 |
| 対策版フロントエンド | http://localhost:3007 | 対策済みのデモ画面 |

テスト用アカウント:
- ユーザー名: `admin` / パスワード: `password123`
- ユーザー名: `user1` / パスワード: `pass456`

## 4. 攻撃手順

### ステップ1: 脆弱版でブルートフォース攻撃を試す

1. ブラウザで http://localhost:3006 を開く
2. 「ブルートフォース攻撃を実行」ボタンを押す
3. adminユーザーに対してよく使われるパスワード20個が連続で試行される
4. レート制限がないため、20回全ての試行が即座に処理されることを確認する
5. `password123` が含まれているため、少なくとも1回は成功（ステータス200）となる

### ステップ2: 対策版で同じ攻撃を試す

1. ブラウザで http://localhost:3007 を開く
2. 「ブルートフォース攻撃を実行」ボタンを押す
3. 5回の失敗後、6回目以降は429（Too Many Requests）エラーが返されることを確認する
4. Retry-Afterヘッダーにより、60秒後に再試行するよう通知されることを確認する

### ステップ3: 平文パスワード vs ハッシュの違いを確認する

脆弱版のユーザー一覧エンドポイントではパスワードが平文で露出する。

```bash
# 脆弱版: パスワードが平文で返される
curl http://localhost:8086/users
# [{"id":1,"username":"admin","password":"password123"},{"id":2,"username":"user1","password":"pass456"}]

# 対策版: パスワードハッシュは返されない
curl http://localhost:8087/users
# [{"id":1,"username":"admin"},{"id":2,"username":"user1"}]
```

### ステップ4: セッション固定の確認

脆弱版でログインすると、既存のセッションIDがそのまま再利用される。対策版では、ログイン成功時に新しいセッションIDが発行される。

## 5. 脆弱なコード解説

### パスワードの平文保存

脆弱版では、ユーザーのパスワードが平文のままデータベースに保存されている。

```go
// パスワードを平文で保存する（意図的に脆弱な設計）
createTableSQL := `
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE,
    password TEXT
);`
```

シードデータも平文で投入される。

```go
// seedData はテスト用の初期データを投入する（パスワードは平文で保存 — 意図的に脆弱）
func seedData(db *sql.DB) {
    users := []struct {
        username string
        password string
    }{
        {"admin", "password123"},
        {"user1", "pass456"},
    }

    for _, u := range users {
        _, err := db.Exec("INSERT OR IGNORE INTO users (username, password) VALUES (?, ?)", u.username, u.password)
```

### レート制限の欠如

脆弱版のログインハンドラーには、試行回数の制限が一切ない。

```go
// 平文パスワードとの比較（意図的に脆弱）
var storedPassword string
err := db.QueryRow("SELECT password FROM users WHERE username = ?", req.Username).Scan(&storedPassword)
if err != nil || storedPassword != req.Password {
    // レート制限なし — ブルートフォース攻撃に脆弱
    http.Error(w, "認証に失敗", http.StatusUnauthorized)
    return
}
```

### セッション固定攻撃への脆弱性

脆弱版では、ログイン時に既存のセッションIDが存在する場合、それをそのまま再利用する。

```go
// セッション固定攻撃に脆弱: 既存のセッションIDがあればそのまま再利用する
sessionID := ""
cookie, err := r.Cookie("session_id")
if err == nil && cookie.Value != "" {
    sessionID = cookie.Value
} else {
    sessionID = generateSessionID()
}
```

## 6. 対策コード解説

### bcryptによるパスワードハッシュ化

対策版では、パスワードをbcryptでハッシュ化して保存する。bcryptは計算コストが高いアルゴリズムであり、ブルートフォース攻撃への耐性がある。

```go
// hashPassword はbcryptでパスワードをハッシュ化する
func hashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return "", err
    }
    return string(hash), nil
}

// checkPassword はbcryptハッシュとパスワードを比較する
func checkPassword(hashedPassword, password string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
    return err == nil
}
```

### レート制限

対策版では、IPアドレスごとにログイン試行回数を追跡し、1分間に5回を超える試行を拒否する。

```go
// maxAttempts は1分間に許可される最大試行回数
const maxAttempts = 5

// windowDuration はレート制限のウィンドウ期間
const windowDuration = 1 * time.Minute

// IsAllowed は指定されたIPアドレスのリクエストが許可されるか判定する
func (rl *RateLimiter) IsAllowed(ip string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    now := time.Now()
    cutoff := now.Add(-windowDuration)

    // 期限切れの試行記録を削除する
    valid := make([]time.Time, 0)
    for _, t := range rl.attempts[ip] {
        if t.After(cutoff) {
            valid = append(valid, t)
        }
    }
    rl.attempts[ip] = valid

    return len(valid) < maxAttempts
}
```

レート制限に達した場合は `Retry-After` ヘッダーとともに429ステータスを返す。

```go
if !limiter.IsAllowed(ip) {
    w.Header().Set("Retry-After", "60")
    http.Error(w, "試行回数の上限に達しました。しばらく待ってから再試行してください", http.StatusTooManyRequests)
    return
}
```

### セッション再生成

対策版では、ログイン成功時に既存のセッションを削除して新しいセッションIDを発行する。

```go
// セッション再生成: 既存のセッションを削除して新しいIDを発行する
if oldCookie, err := r.Cookie("session_id"); err == nil && oldCookie.Value != "" {
    store.Delete(oldCookie.Value)
}
sessionID := generateSessionID()
store.Set(sessionID, &Session{Username: req.Username})

// セキュアなCookie設定: HttpOnly, SameSite=Strict
http.SetCookie(w, &http.Cookie{
    Name:     "session_id",
    Value:    sessionID,
    Path:     "/",
    HttpOnly: true,
    SameSite: http.SameSiteStrictMode,
})
```

## 7. まとめ

認証の不備に対するベストプラクティスは以下の通りである。

- **パスワードのハッシュ化**: bcrypt、Argon2などの適切なハッシュアルゴリズムを使用し、平文でパスワードを保存しない
- **レート制限の実装**: ログイン試行回数を制限し、ブルートフォース攻撃を防止する。制限に達した場合はRetry-Afterヘッダーで待機時間を通知する
- **セッションの再生成**: ログイン成功時にセッションIDを再生成し、セッション固定攻撃を防止する
- **セキュアなCookie設定**: `HttpOnly`、`SameSite=Strict`、`Secure`（HTTPS環境）属性を設定する
- **パスワードハッシュの非公開**: APIレスポンスにパスワードやパスワードハッシュを含めない
- **アカウントロックアウト**: 一定回数の失敗後にアカウントを一時的にロックする仕組みを検討する
- **多要素認証（MFA）**: パスワード以外の認証要素を追加し、パスワード漏洩時のリスクを軽減する
