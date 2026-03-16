# CSRF（クロスサイトリクエストフォージェリ）

## 1. 概要

CSRF（Cross-Site Request Forgery）は、ユーザーが認証済みのWebアプリケーションに対して、攻撃者が意図しないリクエストを送信させる攻撃手法である。

ユーザーがログイン中のサイトに対して、別のサイト（攻撃者サイト）から自動的にリクエストを送信することで、パスワード変更、送金、設定変更などの操作をユーザーの意図なく実行させることができる。

### なぜ危険か

- ユーザーの認証情報（Cookie）がブラウザによって自動的に送信されるため、攻撃者はユーザーのパスワードを知らなくても操作を実行できる
- 攻撃対象のユーザーは、悪意のあるサイトを訪問するだけで被害を受ける
- サーバー側からは正規のユーザーからのリクエストと区別がつかない

## 2. 攻撃の仕組み

CSRF攻撃のフローは以下の通りである。

1. ユーザーが正規サイト（例: http://localhost:3004）にログインする
2. ブラウザにセッションCookieが保存される
3. ユーザーが別タブで攻撃者サイト（例: http://localhost:4000）にアクセスする
4. 攻撃者サイトのJavaScriptが正規サイトのAPIに対してリクエストを自動送信する
5. ブラウザが正規サイトのCookieを自動的に含めてリクエストを送信する
6. サーバーはCookieを検証し、正規のリクエストとして処理する
7. ユーザーの意図しない操作（パスワード変更など）が実行される

```
ユーザー          攻撃者サイト          正規サイト
  |                   |                    |
  |-- ログイン --------------------------->|
  |<------------ Cookie発行 --------------|
  |                   |                    |
  |-- 攻撃者サイト訪問 -->|                |
  |                   |-- fetch(credentials: include) -->|
  |                   |   (Cookieが自動送付)              |
  |                   |<--- パスワード変更完了 ----------|
  |                   |                    |
```

## 3. デモ環境の起動手順

```bash
make csrf
```

このコマンドで以下のサービスが起動する。

| サービス | URL | 説明 |
|---------|-----|------|
| 脆弱版バックエンド | http://localhost:8084 | CSRFトークン検証なし |
| 対策版バックエンド | http://localhost:8085 | CSRFトークン検証+SameSite Cookie |
| 脆弱版フロントエンド | http://localhost:3004 | CSRF対策なしのパスワード変更画面 |
| 対策版フロントエンド | http://localhost:3005 | CSRFトークン付きのパスワード変更画面 |
| 攻撃者サイト | http://localhost:4000 | 自動でパスワード変更を試みる悪意のあるページ |

## 4. 攻撃手順

### ステップ1: 脆弱版でログインする

1. ブラウザで http://localhost:3004 を開く
2. 以下の認証情報でログインする
   - ユーザー名: `admin`
   - パスワード: `password123`
3. ログインが成功し、ユーザー情報とパスワード変更フォームが表示されることを確認する

### ステップ2: 攻撃者サイトにアクセスする

1. ログイン状態のまま、別のタブで http://localhost:4000 を開く
2. 攻撃者サイトが自動的に脆弱版バックエンドの `/change-password` にリクエストを送信する
3. 画面に「攻撃成功! パスワードが「hacked123」に変更されました」と表示される

### ステップ3: パスワードが変更されたことを確認する

1. 脆弱版フロントエンド（http://localhost:3004）に戻る
2. ログアウトする
3. `admin` / `password123` でログインを試みると失敗する
4. `admin` / `hacked123` でログインすると成功する（パスワードが攻撃者によって変更された）

### ステップ4: 対策版で同じ攻撃が失敗することを確認する

1. ブラウザで http://localhost:3005 を開く
2. `admin` / `password123` でログインする
3. 別タブで http://localhost:4000 を開く
4. 攻撃者サイトはCORSポリシーまたはCSRFトークンの不一致により攻撃に失敗する
5. 対策版ではパスワードが変更されていないことを確認する

## 5. 脆弱なコード解説

### バックエンド（Go）

脆弱版のバックエンドには以下の問題がある。

**CORSの設定が全オリジンを許可している:**

```go
// CORSヘッダーを設定する（意図的に脆弱な設定）
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
```

`Access-Control-Allow-Origin: *` により、どのオリジンからのリクエストも許可される。

**Cookie設定にSameSite属性がない:**

```go
// 意図的に脆弱なCookie設定: SameSiteなし、HttpOnlyなし
http.SetCookie(w, &http.Cookie{
	Name:  "session_id",
	Value: sessionID,
	Path:  "/",
})
```

SameSite属性が未設定のため、異なるオリジンからのリクエストにもCookieが送信される。

**CSRFトークンの検証がない:**

```go
// パスワード変更ハンドラー: CSRFトークン検証なし（意図的に脆弱）
func handleChangePassword(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	// ...
	username, ok := getUserFromSession(r)
	if !ok {
		http.Error(w, "未認証です", http.StatusUnauthorized)
		return
	}
	// CSRFトークンの検証が一切ない
	// ...
}
```

### フロントエンド（React）

脆弱版のフロントエンドではCSRFトークンなしでパスワード変更リクエストを送信している。

```tsx
const response = await fetch(`${API_URL}/change-password`, {
  method: "POST",
  credentials: "include",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ new_password: newPassword }),
});
```

## 6. 対策コード解説

### CSRFトークンによる検証

対策版では、パスワード変更前にCSRFトークンを取得し、リクエストヘッダーに含めて送信する。

**バックエンド — トークン発行:**

```go
// CSRFトークン発行ハンドラー: セッションに紐づくCSRFトークンを生成して返す
func handleCSRFToken(w http.ResponseWriter, r *http.Request) {
	// ...
	token, err := generateToken()
	// ...
	// CSRFトークンをセッションに紐づけて保存する
	csrfTokenMu.Lock()
	csrfTokens[sessionID] = token
	csrfTokenMu.Unlock()
	// ...
}
```

**バックエンド — トークン検証:**

```go
// CSRFトークンを検証する
func validateCSRFToken(r *http.Request) bool {
	sessionID, ok := getSessionID(r)
	if !ok {
		return false
	}
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		return false
	}
	csrfTokenMu.RLock()
	storedToken, exists := csrfTokens[sessionID]
	csrfTokenMu.RUnlock()
	return exists && storedToken == token
}
```

攻撃者サイトからはCSRFトークンを取得できないため、有効なトークンをリクエストに含めることができず攻撃が失敗する。

### SameSite Cookie

対策版では、Cookieに `SameSite=Strict` を設定している。

```go
// 安全なCookie設定: SameSite=Strict、HttpOnly=true
http.SetCookie(w, &http.Cookie{
	Name:     "session_id",
	Value:    sessionID,
	Path:     "/",
	SameSite: http.SameSiteStrictMode,
	HttpOnly: true,
	Secure:   false, // localhost用デモのためfalse
})
```

`SameSite=Strict` により、異なるオリジンからのリクエストにはCookieが送信されなくなる。`HttpOnly` フラグにより、JavaScriptからCookieにアクセスすることもできない。

### Origin検証（CORSの適切な設定）

対策版では、許可するオリジンを特定のドメインに限定している。

```go
// CORSヘッダーを設定する（安全な設定: 特定のオリジンのみ許可）
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", getAllowedOrigin())
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
}
```

`Access-Control-Allow-Origin` にフロントエンドのオリジン（`http://localhost:3005`）のみを指定することで、攻撃者サイトからのリクエストはブラウザのCORSポリシーによりブロックされる。

### フロントエンド（React）

対策版のフロントエンドでは、パスワード変更前にCSRFトークンを取得し、`X-CSRF-Token` ヘッダーに含めて送信する。

```tsx
// CSRFトークンを取得する
const tokenResponse = await fetch(`${API_URL}/csrf-token`, {
  credentials: "include",
});
const tokenData: CsrfTokenResponse = await tokenResponse.json();

// CSRFトークンをヘッダーに含めてパスワード変更リクエストを送信する
const response = await fetch(`${API_URL}/change-password`, {
  method: "POST",
  credentials: "include",
  headers: {
    "Content-Type": "application/json",
    "X-CSRF-Token": tokenData.token,
  },
  body: JSON.stringify({ new_password: newPassword }),
});
```

## 7. まとめ

CSRF対策のベストプラクティスは以下の通りである。

- **CSRFトークンの使用**: サーバー側でセッションに紐づいたトークンを発行し、状態変更リクエストごとにトークンを検証する
- **SameSite Cookie属性の設定**: `SameSite=Strict` または `SameSite=Lax` を設定し、異なるオリジンからのCookie送信を防止する
- **CORSの適切な設定**: `Access-Control-Allow-Origin` に `*` を使わず、信頼するオリジンのみを許可する
- **HttpOnly Cookieの使用**: `HttpOnly` フラグを設定し、JavaScriptからのCookieアクセスを防止する
- **多層防御**: CSRFトークン、SameSite Cookie、CORS設定を組み合わせ、単一の防御層に依存しない
