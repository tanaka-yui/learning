# セッション認証(Server-side Session)

## 1. 概要

セッション認証は、ログイン状態をサーバ側に保持し、クライアントには識別子(セッションID)だけを Cookie で渡す方式。サーバは受け取ったセッションID をキーに、自分が保持する状態を引いて認証済みかを判断する。状態がサーバにあるため、ログアウトやセッション破棄をサーバ主導で即時に行えるのが特徴。

### セッション vs JWT

| 観点 | セッション認証 | JWT(02章) |
|------|--------------|-----------|
| 状態の置き場 | サーバ(ストア) | トークン自身 |
| 失効の容易さ | 即時に削除可能 | 有効期限まで失効困難 |
| 水平スケール | ストア共有(Redis等)が必要 | ステートレスで容易 |
| ペイロードの機密性 | セッションID のみ流れる | Base64デコードで内容が見える |

セッション認証は「サーバが状態を握る」モデルのため、アカウント停止や強制ログアウトをリアルタイムに反映できる。一方、複数サーバで同じセッションを参照するには共有ストア(Redis や RDB)が必要になる。JWT はその逆で、ステートレスな検証を実現するがトークン即時無効化が難しい。

> **注:** パスワードのハッシュ化・レート制限・セッション固定攻撃への対策については
> `03_security_measures/docs/auth-bypass.md` で詳しく扱っている。本章はセッション認証の
> フロー全体の理解に集中する。

## 2. 仕組み

ブラウザ/クライアントとサーバの間で次のシーケンスが起きる。

```
クライアント                              サーバ
    |                                       |
    |--- POST /login (username, password) -->|
    |                                       | 1. パスワードを bcrypt で照合
    |                                       | 2. SessionStore.Create() でセッション生成
    |                                       |    (crypto/rand で 32 バイトのランダム ID)
    |<--- 200 OK                            |
    |     Set-Cookie: session_id=<ID>       |
    |                                       |
    |--- GET /profile                    -->|
    |    Cookie: session_id=<ID>            | 3. Cookie からセッションID を取得
    |                                       | 4. SessionStore.Get(ID) でストアを照合
    |                                       | 5. 見つかれば認証済みと判断
    |<--- 200 OK { username: "alice" }      |
    |                                       |
    |--- POST /logout                    -->|
    |    Cookie: session_id=<ID>            | 6. SessionStore.Delete(ID) で即時破棄
    |<--- 200 OK { status: "logged_out" }   |
    |                                       |
    |--- GET /profile (Cookie なし)      -->|
    |                                       | 7. Cookie がないため 401
    |<--- 401 Unauthorized                  |
```

ポイントはステップ 4 にある。Cookie にはセッション ID という「番号札」だけが載っており、ユーザ情報そのものは乗らない。サーバ側ストアにアクセスして初めて「誰のセッションか」が分かる。

## 3. デモ起動手順

```bash
# プロジェクトルート(09_authn_authz/)から実行
make session
```

起動後、ブラウザまたは curl で `http://localhost:9000` にアクセスすると動作確認用の HTML が表示される。

テストユーザ:

| ユーザ名 | パスワード |
|----------|-----------|
| alice | password123 |
| bob | pass456 |

停止するには:

```bash
make down
```

## 4. 動作確認手順

以下の curl コマンドで一通りのフローを確認できる。Cookie をファイルに保存しながら操作するのがポイント。

### ステップ 1: ログイン(セッション ID を取得する)

```bash
curl -s -c /tmp/cookies.txt \
  -X POST http://localhost:9000/login \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"password123"}'
```

成功すると JSON と `Set-Cookie: session_id=<ランダムなID>` が返る。

```json
{"status":"ok","username":"alice"}
```

### ステップ 2: /profile にアクセス(認証済み状態)

```bash
curl -s -b /tmp/cookies.txt http://localhost:9000/profile
```

Cookie が有効であれば認証情報が返る。

```json
{"username":"alice"}
```

### ステップ 3: Cookie なしで /profile にアクセス(401 を確認)

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:9000/profile
```

Cookie を送らないため `401` が返る。

### ステップ 4: ログアウト

```bash
curl -s -b /tmp/cookies.txt -c /tmp/cookies.txt \
  -X POST http://localhost:9000/logout
```

サーバ側でセッションが削除され、`Max-Age=-1` の Cookie によりクライアント側の Cookie も削除される。

```json
{"status":"logged_out"}
```

### ステップ 5: ログアウト後に /profile へアクセス(セッション破棄を確認)

```bash
curl -s -o /dev/null -w "%{http_code}" -b /tmp/cookies.txt \
  http://localhost:9000/profile
```

サーバ側でセッションが削除済みのため、Cookie ファイルに session_id が残っていても `401` が返る。これがサーバ主導での即時失効の動作確認になる。

## 5. コード解説

### store.go — インメモリ セッションストア

```go
// newID は暗号学的乱数から32バイトのセッションIDを生成する
func newID() string {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        panic(err) // 乱数生成の失敗は継続不能
    }
    return hex.EncodeToString(b)
}
```

`crypto/rand` を使うことで予測不可能な 64 文字の 16 進数 ID を生成している。`math/rand` のような疑似乱数は使わない。

```go
// SessionStore はインメモリのセッション保管庫(学習用)
type SessionStore struct {
    mu       sync.RWMutex
    sessions map[string]*Session
}
```

並行アクセスへの対応として `sync.RWMutex` を使う。読み込み専用の `Get` は `RLock`/`RUnlock`、書き込みを伴う `Create`/`Delete` は `Lock`/`Unlock` を使い分けることで、複数の読み込みリクエストが同時に来ても書き込みと競合しないようにしている。

本デモはインメモリ実装のため、サーバを再起動するとセッションは消える。本番環境では Redis や RDB をストアに使う。

### users.go — bcrypt によるパスワード照合

```go
// NewUserStore はシードユーザのパスワードをbcryptでハッシュ化して保持する
func NewUserStore() *UserStore {
    hashes := make(map[string]string, len(seedUsers))
    for name, pw := range seedUsers {
        h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
        if err != nil {
            panic(err)
        }
        hashes[name] = string(h)
    }
    return &UserStore{hashes: hashes}
}
```

起動時にシードユーザのパスワードを bcrypt でハッシュ化し、平文はメモリに残さない。

```go
// Verify はユーザ名とパスワードの組が正しいか判定する
func (u *UserStore) Verify(username, password string) bool {
    h, ok := u.hashes[username]
    if !ok {
        // ユーザ不在でも比較を行いタイミング差を抑える
        bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinv"), []byte(password))
        return false
    }
    return bcrypt.CompareHashAndPassword([]byte(h), []byte(password)) == nil
}
```

ユーザが存在しない場合でもダミーハッシュとの比較処理を実行することで、「ユーザが存在するか否か」をレスポンス時間から推測されるタイミング攻撃を緩和している。bcrypt のハッシュ比較/パスワードハッシュ化の詳細は `03_security_measures/docs/auth-bypass.md` を参照。

### handlers.go — ルーティングとミドルウェア

```go
// setupRouter は依存を受け取り http.Handler を構築する
func setupRouter(store *SessionStore, users *UserStore) http.Handler {
    mux := http.NewServeMux()
    // ...
    mux.Handle("GET /profile", requireSession(store, func(w http.ResponseWriter, r *http.Request, sess *Session) {
        writeJSON(w, http.StatusOK, map[string]string{"username": sess.Username})
    }))
    return mux
}
```

`setupRouter` は依存(SessionStore・UserStore)を引数で受け取り `http.Handler` を返す。テスト時にモックを差し込みやすい設計になっている。

```go
// requireSession は有効なセッションを要求するミドルウェア
func requireSession(store *SessionStore, next func(http.ResponseWriter, *http.Request, *Session)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        c, err := r.Cookie(sessionCookieName)
        if err != nil {
            http.Error(w, "未認証です", http.StatusUnauthorized)
            return
        }
        sess, ok := store.Get(c.Value)
        if !ok {
            http.Error(w, "未認証です", http.StatusUnauthorized)
            return
        }
        next(w, r, sess)
    }
}
```

`requireSession` ミドルウェアは「Cookie がない」「Cookie の値がストアに存在しない」どちらの場合も `401` を返す。Cookie が存在しても、サーバ側のセッションが削除されていれば弾かれる。これが「サーバ主導の即時失効」の核心部分。

```go
http.SetCookie(w, &http.Cookie{
    Name:     sessionCookieName,
    Value:    sess.ID,
    Path:     "/",
    HttpOnly: true,
    SameSite: http.SameSiteLaxMode,
})
```

Cookie には `HttpOnly: true` を設定しており、JavaScript から `document.cookie` で読み取れない。`SameSite: Lax` により外部サイトからの GET ナビゲーションは Cookie を送信するが、クロスサイトの POST では送信されない。HTTPS 環境では `Secure: true` も付与すること(本デモはローカル HTTP のため省略)。

## 6. まとめ

- **サーバ側に状態を持つ** ことで、ログアウト・アカウント停止を即時に反映できる。
- **セッション ID は暗号学的乱数** (`crypto/rand`) で生成し、推測不可能にする。
- **Cookie は `HttpOnly` + `SameSite=Lax`** で保護する。HTTPS 環境では `Secure` も必須。
- **並行アクセスには `sync.RWMutex`** を使い、読み込みと書き込みを適切に分離する。
- **水平スケール時はストアの共有が課題** になる。本番では Redis や RDB をストアとして使う。
- **次章の JWT** は状態をトークン自身に持つことでステートレスな検証を実現するが、即時失効が難しいというトレードオフがある。セッション認証はそのトレードオフの逆側に位置する。
