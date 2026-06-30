# 09 モジュール基盤 + Session認証デモ 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `09_authn_authz` モジュールの基盤(go.work / docker-compose / Makefile / README)を作り、最初の動くデモとしてサーバ側セッション + Cookie認証を実装する。

**Architecture:** Go 1.26 の単一HTTPアプリ。インメモリのセッションストアにログイン状態を持ち、HttpOnly Cookie(`session_id`)で認証する。ハンドラは `setupRouter()` が返す `http.Handler` に集約し、`httptest` でテストする。コンテナは distroless で配布、compose の `profiles` で個別起動する。これは後続デモ(jwt/oauth-oidc/mfa/api-m2m/authz)が踏襲する基盤パターンになる。

**Tech Stack:** Go 1.26 / 標準 `net/http` / `golang.org/x/crypto/bcrypt`(pure Go, CGO不要) / Docker(distroless) / docker compose profiles / Make

## Global Constraints

- 言語は Go 1.26 のみ。TypeScript は使わない。
- フロントは別立てせず、Goアプリが最小HTMLをサーバ配信する。
- 認証情報(テストユーザ)はコード内シード。学習用途のため最小限・明示コメント付き。
- セッションストア等の状態はインメモリ(永続DBなし)。
- 既存モジュール 03/08 のコンベンション踏襲: Dockerfile は `golang:1.26` build → `distroless/static`、テストは `httptest` + `setupRouter()` パターン、テストコメントは日本語。
- `03_security_measures/auth-bypass` がパスワードハッシュ/レート制限/セッション固定をカバー済。重複説明は避け、必要箇所はそちらを参照する。
- モジュール内ポート割当(compose前提): session=9000, jwt=9001, oauth-oidc=9100/9101/9102, authz=9200, mfa=9300(+Mailpit 8025), api-m2m=9400/9401。本計画では session のみ配線する。

---

## File Structure

```
09_authn_authz/
├── go.work                 # use ./session(後続デモで use を追加)
├── docker-compose.yml      # session サービスのみ(後続デモで追記)
├── Makefile                # help / session / all / down(後続で拡張)
├── README.md               # モジュール概要 + デモ一覧表
├── .gitignore
├── session/
│   ├── go.mod
│   ├── main.go             # サーバ起動(:8080)
│   ├── store.go            # SessionStore / Session
│   ├── users.go            # UserStore(bcryptシード) / Verify
│   ├── handlers.go         # setupRouter() + 各ハンドラ + 認証ミドルウェア
│   ├── main_test.go        # httptest テスト
│   └── Dockerfile
└── docs/
    └── 01_session.md       # セッション認証の解説(03スタイル)
```

各ファイルの責務:
- `store.go`: セッションの生成・取得・破棄のみ。HTTP非依存。
- `users.go`: 認証情報の保持と照合のみ。HTTP非依存。
- `handlers.go`: HTTP配線。`store`/`users` を受け取り `http.Handler` を返す。状態は持たない。
- `main.go`: 依存を組み立ててサーバ起動するだけ。

---

## Task 1: Session ストアとユーザ照合(HTTP非依存ロジック)

**Files:**
- Create: `09_authn_authz/session/go.mod`
- Create: `09_authn_authz/session/store.go`
- Create: `09_authn_authz/session/users.go`
- Test: `09_authn_authz/session/store_test.go`

**Interfaces:**
- Consumes: なし
- Produces:
  - `type Session struct { ID string; Username string; CreatedAt time.Time }`
  - `type SessionStore struct { ... }`
  - `func NewSessionStore() *SessionStore`
  - `func (s *SessionStore) Create(username string) *Session` — 新規セッションを生成し保存。`ID` は暗号学的乱数の hex。
  - `func (s *SessionStore) Get(id string) (*Session, bool)`
  - `func (s *SessionStore) Delete(id string)`
  - `type UserStore struct { ... }`
  - `func NewUserStore() *UserStore` — bcrypt ハッシュ済のテストユーザをシード(`alice`/`password123`, `bob`/`pass456`)。
  - `func (u *UserStore) Verify(username, password string) bool`

- [ ] **Step 1: go.mod を作成**

`09_authn_authz/session/go.mod`:
```
module authn-authz/session

go 1.26

require golang.org/x/crypto v0.31.0
```

- [ ] **Step 2: 失敗するテストを書く**

`09_authn_authz/session/store_test.go`:
```go
package main

import "testing"

// TestSessionCreateGetDelete はセッションの生成・取得・破棄を検証する
func TestSessionCreateGetDelete(t *testing.T) {
	store := NewSessionStore()

	sess := store.Create("alice")
	if sess.ID == "" {
		t.Fatal("セッションIDが空です")
	}
	if sess.Username != "alice" {
		t.Errorf("Username = %q, want alice", sess.Username)
	}

	got, ok := store.Get(sess.ID)
	if !ok {
		t.Fatal("生成したセッションを取得できません")
	}
	if got.Username != "alice" {
		t.Errorf("取得した Username = %q, want alice", got.Username)
	}

	store.Delete(sess.ID)
	if _, ok := store.Get(sess.ID); ok {
		t.Error("破棄後もセッションが残っています")
	}
}

// TestSessionIDsAreUnique は生成のたびに異なるIDになることを検証する
func TestSessionIDsAreUnique(t *testing.T) {
	store := NewSessionStore()
	a := store.Create("alice")
	b := store.Create("alice")
	if a.ID == b.ID {
		t.Error("セッションIDが重複しています")
	}
}

// TestUserVerify はユーザ照合を検証する
func TestUserVerify(t *testing.T) {
	users := NewUserStore()
	if !users.Verify("alice", "password123") {
		t.Error("正しい認証情報が拒否されました")
	}
	if users.Verify("alice", "wrong") {
		t.Error("誤ったパスワードが受理されました")
	}
	if users.Verify("nobody", "password123") {
		t.Error("存在しないユーザが受理されました")
	}
}
```

- [ ] **Step 3: テストを実行して失敗を確認**

Run: `cd 09_authn_authz/session && go test ./... -run 'TestSession|TestUser' -v`
Expected: コンパイルエラー(`NewSessionStore` 等が未定義)で FAIL

注: この時点では `go mod tidy` を走らせない(bcrypt 未importのため `x/crypto` が require から消えてしまう)。tidy は Step 6 で実行する。

- [ ] **Step 4: store.go を実装**

`09_authn_authz/session/store.go`:
```go
package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Session はログイン状態を表す
type Session struct {
	ID        string
	Username  string
	CreatedAt time.Time
}

// SessionStore はインメモリのセッション保管庫(学習用)
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionStore は空のセッションストアを返す
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

// newID は暗号学的乱数から32バイトのセッションIDを生成する
func newID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // 乱数生成の失敗は継続不能
	}
	return hex.EncodeToString(b)
}

// Create は新しいセッションを生成して保存する
func (s *SessionStore) Create(username string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := &Session{ID: newID(), Username: username, CreatedAt: time.Now()}
	s.sessions[sess.ID] = sess
	return sess
}

// Get はIDからセッションを取得する
func (s *SessionStore) Get(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

// Delete はセッションを破棄する
func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}
```

- [ ] **Step 5: users.go を実装**

`09_authn_authz/session/users.go`:
```go
package main

import "golang.org/x/crypto/bcrypt"

// UserStore はテストユーザの認証情報を保持する(学習用シード)
type UserStore struct {
	hashes map[string]string // username -> bcryptハッシュ
}

// seedUsers はテスト用の平文パスワード(学習用途のため明示)
var seedUsers = map[string]string{
	"alice": "password123",
	"bob":   "pass456",
}

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

- [ ] **Step 6: 依存を解決してテストを実行**

Run: `cd 09_authn_authz/session && go mod tidy && go test ./... -run 'TestSession|TestUser' -v`
Expected: `go.sum` が生成され、すべて PASS

- [ ] **Step 7: コミット**

```bash
git add 09_authn_authz/session/go.mod 09_authn_authz/session/go.sum 09_authn_authz/session/store.go 09_authn_authz/session/users.go 09_authn_authz/session/store_test.go
git commit -m "feat(09_authn_authz): session store and user verification"
```

---

## Task 2: HTTP ハンドラと認証ミドルウェア

**Files:**
- Create: `09_authn_authz/session/handlers.go`
- Create: `09_authn_authz/session/main.go`
- Test: `09_authn_authz/session/main_test.go`

**Interfaces:**
- Consumes: `SessionStore`, `UserStore`(Task 1)
- Produces:
  - `func setupRouter(store *SessionStore, users *UserStore) http.Handler`
  - エンドポイント: `GET /`(HTML), `POST /login`(JSON `{username,password}`), `POST /logout`, `GET /profile`(保護)
  - Cookie名は `session_id`(HttpOnly, SameSite=Lax, Path=/)

- [ ] **Step 1: 失敗するテストを書く**

`09_authn_authz/session/main_test.go`:
```go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestServer はテスト用サーバを起動する
func newTestServer() *httptest.Server {
	return httptest.NewServer(setupRouter(NewSessionStore(), NewUserStore()))
}

// noRedirectClient はリダイレクトを追わないHTTPクライアントを返す
func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

// login はログインリクエストを送り、レスポンスを返す
func login(t *testing.T, server *httptest.Server, user, pass string) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	resp, err := noRedirectClient().Post(server.URL+"/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("ログインリクエスト失敗: %v", err)
	}
	return resp
}

// sessionCookie はレスポンスから session_id Cookie を取り出す
func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == "session_id" {
			return c
		}
	}
	return nil
}

// TestLoginSetsHttpOnlyCookie は正しい認証情報でHttpOnly Cookieが返ることを検証する
func TestLoginSetsHttpOnlyCookie(t *testing.T) {
	server := newTestServer()
	defer server.Close()

	resp := login(t, server, "alice", "password123")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	c := sessionCookie(resp)
	if c == nil {
		t.Fatal("session_id Cookie が設定されていません")
	}
	if !c.HttpOnly {
		t.Error("session_id Cookie が HttpOnly ではありません")
	}
}

// TestLoginRejectsBadCredentials は誤った認証情報を拒否することを検証する
func TestLoginRejectsBadCredentials(t *testing.T) {
	server := newTestServer()
	defer server.Close()

	resp := login(t, server, "alice", "wrong")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if sessionCookie(resp) != nil {
		t.Error("認証失敗時に Cookie が設定されています")
	}
}

// TestProtectedRequiresSession は /profile が認証を要求することを検証する
func TestProtectedRequiresSession(t *testing.T) {
	server := newTestServer()
	defer server.Close()

	// Cookieなし → 401
	resp, _ := http.Get(server.URL + "/profile")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Cookieなしの status = %d, want 401", resp.StatusCode)
	}

	// ログイン後のCookie付き → 200 + username
	loginResp := login(t, server, "alice", "password123")
	c := sessionCookie(loginResp)
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/profile", nil)
	req.AddCookie(c)
	resp2, _ := http.DefaultClient.Do(req)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Cookie付きの status = %d, want 200", resp2.StatusCode)
	}
	var got map[string]string
	json.NewDecoder(resp2.Body).Decode(&got)
	if got["username"] != "alice" {
		t.Errorf("username = %q, want alice", got["username"])
	}
}

// TestLogoutInvalidatesSession はログアウトでセッションが無効化されることを検証する
func TestLogoutInvalidatesSession(t *testing.T) {
	server := newTestServer()
	defer server.Close()

	loginResp := login(t, server, "alice", "password123")
	c := sessionCookie(loginResp)

	// ログアウト
	logoutReq, _ := http.NewRequest(http.MethodPost, server.URL+"/logout", nil)
	logoutReq.AddCookie(c)
	if _, err := http.DefaultClient.Do(logoutReq); err != nil {
		t.Fatalf("ログアウト失敗: %v", err)
	}

	// 同じCookieで /profile → 401
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/profile", nil)
	req.AddCookie(c)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("ログアウト後の status = %d, want 401", resp.StatusCode)
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `cd 09_authn_authz/session && go test ./... -run TestLogin -v`
Expected: コンパイルエラー(`setupRouter` 未定義)で FAIL

- [ ] **Step 3: handlers.go を実装**

`09_authn_authz/session/handlers.go`:
```go
package main

import (
	"encoding/json"
	"net/http"
)

const sessionCookieName = "session_id"

// indexHTML は動作確認用の最小HTML
const indexHTML = `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>Session認証デモ</title></head>
<body>
<h1>Session認証デモ</h1>
<p>POST /login, GET /profile, POST /logout を確認してください。</p>
<p>テストユーザ: alice / password123, bob / pass456</p>
</body></html>`

// loginRequest はログインの入力
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// setupRouter は依存を受け取り http.Handler を構築する
func setupRouter(store *SessionStore, users *UserStore) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(indexHTML))
	})

	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "不正なリクエスト", http.StatusBadRequest)
			return
		}
		if !users.Verify(req.Username, req.Password) {
			http.Error(w, "認証に失敗しました", http.StatusUnauthorized)
			return
		}
		sess := store.Create(req.Username)
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sess.ID,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "username": req.Username})
	})

	mux.HandleFunc("POST /logout", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookieName); err == nil {
			store.Delete(c.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1})
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	})

	mux.Handle("GET /profile", requireSession(store, func(w http.ResponseWriter, r *http.Request, sess *Session) {
		writeJSON(w, http.StatusOK, map[string]string{"username": sess.Username})
	}))

	return mux
}

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

// writeJSON はJSONレスポンスを書き出す
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: main.go を実装**

`09_authn_authz/session/main.go`:
```go
package main

import (
	"log"
	"net/http"
)

func main() {
	store := NewSessionStore()
	users := NewUserStore()
	addr := ":8080"
	log.Printf("session認証デモ起動: %s", addr)
	if err := http.ListenAndServe(addr, setupRouter(store, users)); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: テストを実行して通過を確認**

Run: `cd 09_authn_authz/session && go test ./... -v`
Expected: すべて PASS

- [ ] **Step 6: コミット**

```bash
git add 09_authn_authz/session/handlers.go 09_authn_authz/session/main.go 09_authn_authz/session/main_test.go
git commit -m "feat(09_authn_authz): session login/logout/profile handlers"
```

---

## Task 3: コンテナ化とモジュール基盤(go.work / Dockerfile / compose / Makefile)

**Files:**
- Create: `09_authn_authz/session/Dockerfile`
- Create: `09_authn_authz/go.work`
- Create: `09_authn_authz/docker-compose.yml`
- Create: `09_authn_authz/Makefile`
- Create: `09_authn_authz/.gitignore`

**Interfaces:**
- Consumes: `session/`(Task 1-2)
- Produces: `make session` で `http://localhost:9000` にデモが立ち上がる

- [ ] **Step 1: Dockerfile を作成**

`09_authn_authz/session/Dockerfile`:
```dockerfile
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server .

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
```

- [ ] **Step 2: go.work を作成**

`09_authn_authz/go.work`:
```
go 1.26

toolchain go1.26.0

use ./session
```

- [ ] **Step 3: docker-compose.yml を作成**

`09_authn_authz/docker-compose.yml`:
```yaml
services:
  session:
    build: ./session
    ports:
      - "9000:8080"
    profiles:
      - session
```

- [ ] **Step 4: Makefile を作成**

`09_authn_authz/Makefile`:
```makefile
.PHONY: help session all down

help: ## ヘルプ表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

session: ## Session認証デモ起動 (http://localhost:9000)
	docker compose --profile session up --build -d

all: ## 全デモ起動
	docker compose --profile session up --build -d

down: ## 全サービス停止
	docker compose --profile session down
```

- [ ] **Step 5: .gitignore を作成**

`09_authn_authz/.gitignore`:
```
/certs/
*.out
```

- [ ] **Step 6: ビルドと起動を確認**

Run:
```bash
cd 09_authn_authz && make session && sleep 3 && \
curl -s -i -X POST http://localhost:9000/login -H 'Content-Type: application/json' -d '{"username":"alice","password":"password123"}'
```
Expected: `HTTP/1.1 200 OK` と `Set-Cookie: session_id=...; HttpOnly` を含む

- [ ] **Step 7: 保護エンドポイントを確認して停止**

Run:
```bash
cd 09_authn_authz && \
COOKIE=$(curl -s -i -X POST http://localhost:9000/login -H 'Content-Type: application/json' -d '{"username":"alice","password":"password123"}' | grep -i '^set-cookie:' | sed 's/.*session_id=\([^;]*\).*/\1/') && \
curl -s http://localhost:9000/profile -H "Cookie: session_id=$COOKIE" && echo "" && \
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:9000/profile && \
make down
```
Expected: 1行目 `{"username":"alice"}`、2行目 `401`(Cookieなし)

- [ ] **Step 8: コミット**

```bash
git add 09_authn_authz/session/Dockerfile 09_authn_authz/go.work 09_authn_authz/docker-compose.yml 09_authn_authz/Makefile 09_authn_authz/.gitignore
git commit -m "build(09_authn_authz): module scaffold and session demo container"
```

---

## Task 4: ドキュメント(README + 01_session.md)

**Files:**
- Create: `09_authn_authz/README.md`
- Create: `09_authn_authz/docs/01_session.md`

**Interfaces:**
- Consumes: session デモ(Task 1-3)
- Produces: モジュールの入口ドキュメント

- [ ] **Step 1: README.md を作成**

`09_authn_authz/README.md`:
```markdown
# 09_authn_authz: 認証・認可 学習

認証(authentication)と認可(authorization)の主要方式を、動くGoデモと解説で学ぶ。
既製IdP(Auth0/Cognito/Keycloak)は比較解説のみ、フロー自体は自作実装で可視化する。

## デモ一覧

| デモ | 内容 | 起動 | ポート |
|------|------|------|--------|
| session | サーバ側セッション + Cookie認証 | `make session` | 9000 |

(jwt / oauth-oidc / mfa / api-m2m / authz は順次追加)

## 前提条件

- Docker / Docker Compose
- (任意) Go 1.26+

## 使い方

```bash
make session   # Session認証デモ起動
make down      # 全サービス停止
make help      # ヘルプ表示
```

## ドキュメント

- [01 セッション認証](docs/01_session.md)
```

- [ ] **Step 2: docs/01_session.md を作成**

`09_authn_authz/docs/01_session.md` を以下の構成(03スタイル: 概要→仕組み→デモ起動→確認手順→コード解説→まとめ)で記述する:

```markdown
# セッション認証(Server-side Session)

## 1. 概要

セッション認証は、ログイン状態をサーバ側に保持し、クライアントには識別子(セッションID)だけを
Cookieで渡す方式。サーバは受け取ったセッションIDをキーに、自分が保持する状態を引いて認証済みかを判断する。
状態がサーバにあるため、ログアウトやセッション破棄をサーバ主導で即時に行えるのが特徴。

JWT(02章)との対比:
- セッション = 状態はサーバ。失効が容易。水平スケール時はストア共有が必要。
- JWT = 状態はトークン自身。検証はステートレスだが、即時失効が難しい。

(注: パスワードハッシュ・レート制限・セッション固定攻撃の詳細は
`03_security_measures/auth-bypass` を参照。本章はフローの理解に集中する。)

## 2. 仕組み

(ログイン → Set-Cookie → 保護リソースへのCookie送信 → サーバ側ストア照合 の
ASCIIシーケンス図を記載)

## 3. デモ起動手順

`make session` で http://localhost:9000 に起動する。テストユーザ: alice/password123, bob/pass456。

## 4. 動作確認手順

(curl での login → Set-Cookie 取得 → /profile アクセス → logout の手順を記載。
Task 3 Step 6-7 のコマンドを流用)

## 5. コード解説

- `store.go`: インメモリのセッションストア。`Create` で暗号学的乱数のIDを発行。
- `users.go`: bcryptハッシュでのパスワード照合。
- `handlers.go`: `setupRouter` でHTTP配線。`requireSession` ミドルウェアでCookieを検証。
  Cookieは `HttpOnly` + `SameSite=Lax`。

(各ファイルの該当コードを引用しながら解説)

## 6. まとめ

- セッション認証は状態をサーバに持つため失効制御が容易。
- 識別子Cookieは `HttpOnly`/`SameSite`/`Secure`(HTTPS時)で保護する。
- スケール時はセッションストアの共有(Redis等)が課題になる。
- 次章のJWTはこのトレードオフの逆側に位置する。
```

実装者は上記アウトラインの各節を、実際のコード片を引用しながら日本語で埋めること。プレースホルダ表記(「(...)」)は最終成果物に残さない。

- [ ] **Step 3: コミット**

```bash
git add 09_authn_authz/README.md 09_authn_authz/docs/01_session.md
git commit -m "docs(09_authn_authz): module README and session auth guide"
```

---

## 後続計画(本計画の範囲外)

依存順に別計画として作成する。各計画は本計画で確立したパターン(go.mod / setupRouter / httptest / Dockerfile / compose profile / Makefile拡張 / docs)を踏襲する:

1. `09-02` jwt — JWT発行/検証、access+refresh、rotation、失効
2. `09-03` oauth-oidc — 自作AS + RP + Resource Server、AuthCode+PKCE、OIDC、Client Credentials
3. `09-04` mfa — TOTP + WebAuthn/Passkeys + Magic Link(Mailpit)
4. `09-05` api-m2m — API Key + mTLS
5. `09-06` authz — Casbin(RBAC + ABAC)
6. `09-07` docs — 00_overview / 05_token_ops / 06_passwordless_mfa / 07_api_m2m / 08_idp_comparison(Auth0/Cognito/Keycloak比較) / 09_authz_frameworks(Go認可FW比較) / 10_authz_design
