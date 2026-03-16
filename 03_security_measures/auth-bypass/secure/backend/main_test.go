package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// setupTestServer はテスト用のサーバーをセットアップする
func setupTestServer() (*httptest.Server, *SessionStore, *RateLimiter) {
	db := initDB()
	seedData(db)
	store := NewSessionStore()
	limiter := NewRateLimiter()
	handler := setupRouter(db, store, limiter)
	server := httptest.NewServer(handler)
	return server, store, limiter
}

// loginRequest はログインリクエストを送信してレスポンスを返す
func loginRequest(server *httptest.Server, username, password string, cookies []*http.Cookie) *http.Response {
	body, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, _ := client.Do(req)
	return resp
}

// TestPasswordStoredAsBcryptHash はパスワードがbcryptハッシュとして保存されていることを検証する
func TestPasswordStoredAsBcryptHash(t *testing.T) {
	db := initDB()
	seedData(db)

	var storedHash string
	err := db.QueryRow("SELECT password_hash FROM users WHERE username = ?", "admin").Scan(&storedHash)
	if err != nil {
		t.Fatalf("パスワードハッシュの取得に失敗: %v", err)
	}

	// bcryptハッシュは"$2a$"または"$2b$"で始まる
	if len(storedHash) < 4 || (storedHash[:4] != "$2a$" && storedHash[:4] != "$2b$") {
		t.Errorf("パスワードがbcryptハッシュとして保存されていません: %s", storedHash)
	}

	// 平文パスワードと一致することを確認する
	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte("password123"))
	if err != nil {
		t.Errorf("bcryptハッシュが元のパスワードと一致しません: %v", err)
	}

	// 間違ったパスワードでは一致しないことを確認する
	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte("wrongpassword"))
	if err == nil {
		t.Error("間違ったパスワードでbcryptハッシュが一致してしまいました")
	}
}

// TestRateLimitingAfter5FailedAttempts は5回の失敗後に429を返すことを検証する
func TestRateLimitingAfter5FailedAttempts(t *testing.T) {
	server, _, _ := setupTestServer()
	defer server.Close()

	// 5回の失敗したログイン試行を行う
	for i := 0; i < 5; i++ {
		resp := loginRequest(server, "admin", "wrongpassword", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("試行%d: ステータスコード %d を期待しましたが %d でした", i+1, http.StatusUnauthorized, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// 6回目の試行は429 Too Many Requestsを返すべき
	resp := loginRequest(server, "admin", "wrongpassword", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("6回目の試行: ステータスコード %d を期待しましたが %d でした", http.StatusTooManyRequests, resp.StatusCode)
	}

	// Retry-Afterヘッダーが設定されていることを確認する
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-Afterヘッダーが設定されていません")
	}
}

// TestSessionRegenerationOnLogin はログイン成功時にセッションIDが再生成されることを検証する
func TestSessionRegenerationOnLogin(t *testing.T) {
	server, store, _ := setupTestServer()
	defer server.Close()

	// 事前にセッションIDを設定する（セッション固定攻撃のシミュレーション）
	oldSessionID := "attacker-controlled-session-id"
	store.Set(oldSessionID, &Session{Username: "victim"})

	// 古いセッションIDをCookieとして送信してログインする
	cookies := []*http.Cookie{
		{Name: "session_id", Value: oldSessionID},
	}
	resp := loginRequest(server, "admin", "password123", cookies)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ログインに失敗: ステータスコード %d", resp.StatusCode)
	}

	// レスポンスから新しいセッションIDを取得する
	var newSessionID string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "session_id" {
			newSessionID = cookie.Value
			break
		}
	}

	if newSessionID == "" {
		t.Fatal("レスポンスにセッションIDのCookieがありません")
	}

	// 新しいセッションIDが古いものと異なることを確認する
	if newSessionID == oldSessionID {
		t.Error("セッションIDが再生成されていません（セッション固定攻撃に脆弱）")
	}

	// 古いセッションIDが無効化されていることを確認する
	_, exists := store.Get(oldSessionID)
	if exists {
		t.Error("古いセッションIDが無効化されていません")
	}

	// 新しいセッションIDが有効であることを確認する
	session, exists := store.Get(newSessionID)
	if !exists {
		t.Error("新しいセッションIDが有効ではありません")
	}
	if session.Username != "admin" {
		t.Errorf("セッションのユーザー名が 'admin' ではなく '%s' です", session.Username)
	}
}

// TestLoginWithCorrectCredentials は正しい認証情報で200を返すことを検証する
func TestLoginWithCorrectCredentials(t *testing.T) {
	server, _, _ := setupTestServer()
	defer server.Close()

	resp := loginRequest(server, "admin", "password123", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("ステータスコード %d を期待しましたが %d でした", http.StatusOK, resp.StatusCode)
	}

	// レスポンスボディを確認する
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["username"] != "admin" {
		t.Errorf("ユーザー名 'admin' を期待しましたが '%s' でした", result["username"])
	}

	// セッションCookieが設定されていることを確認する
	var sessionCookie *http.Cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "session_id" {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("セッションCookieが設定されていません")
	}

	// HttpOnlyフラグが設定されていることを確認する
	if !sessionCookie.HttpOnly {
		t.Error("CookieにHttpOnlyフラグが設定されていません")
	}
}

// TestAdminWithValidSession は有効なセッションで/adminが200を返すことを検証する
func TestAdminWithValidSession(t *testing.T) {
	server, _, _ := setupTestServer()
	defer server.Close()

	// まずログインしてセッションを取得する
	resp := loginRequest(server, "admin", "password123", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ログインに失敗: ステータスコード %d", resp.StatusCode)
	}

	var sessionCookie *http.Cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "session_id" {
			sessionCookie = cookie
			break
		}
	}
	resp.Body.Close()

	if sessionCookie == nil {
		t.Fatal("ログインレスポンスにセッションCookieがありません")
	}

	// セッションCookieを使って/adminにアクセスする
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/admin", nil)
	req.AddCookie(sessionCookie)
	client := &http.Client{}
	adminResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("/adminリクエストに失敗: %v", err)
	}
	defer adminResp.Body.Close()

	if adminResp.StatusCode != http.StatusOK {
		t.Errorf("ステータスコード %d を期待しましたが %d でした", http.StatusOK, adminResp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(adminResp.Body).Decode(&result)
	if result["username"] != "admin" {
		t.Errorf("ユーザー名 'admin' を期待しましたが '%s' でした", result["username"])
	}
}

// TestUsersEndpointDoesNotExposePasswords は/usersエンドポイントがパスワードを露出しないことを検証する
func TestUsersEndpointDoesNotExposePasswords(t *testing.T) {
	server, _, _ := setupTestServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/users")
	if err != nil {
		t.Fatalf("/usersリクエストに失敗: %v", err)
	}
	defer resp.Body.Close()

	var users []User
	json.NewDecoder(resp.Body).Decode(&users)

	for _, u := range users {
		if u.Password != "" {
			t.Errorf("ユーザー '%s' のパスワードが露出しています", u.Username)
		}
	}
}

// TestRateLimitResetsAfterSuccessfulLogin はログイン成功後にレート制限がリセットされることを検証する
func TestRateLimitResetsAfterSuccessfulLogin(t *testing.T) {
	server, _, _ := setupTestServer()
	defer server.Close()

	// 4回の失敗したログイン試行を行う
	for i := 0; i < 4; i++ {
		resp := loginRequest(server, "admin", "wrongpassword", nil)
		resp.Body.Close()
	}

	// 正しい認証情報でログインする
	resp := loginRequest(server, "admin", "password123", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("正しい認証情報でのログインに失敗: ステータスコード %d", resp.StatusCode)
	}
	resp.Body.Close()

	// リセット後に再度失敗を試みる（429にならないことを確認する）
	resp = loginRequest(server, "admin", "wrongpassword", nil)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		t.Error("ログイン成功後にレート制限がリセットされていません")
	}
}
