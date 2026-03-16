package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// setupTestServer はテスト用のサーバーを初期化して返す
func setupTestServer() (*httptest.Server, *SessionStore) {
	db := initDB()
	seedData(db)
	store := NewSessionStore()
	handler := setupRouter(db, store)
	return httptest.NewServer(handler), store
}

// loginRequest はログインリクエストを送信するヘルパー関数
func loginRequest(t *testing.T, serverURL, username, password string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	req, err := http.NewRequest(http.MethodPost, serverURL+"/login", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("リクエストの作成に失敗: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("リクエストの送信に失敗: %v", err)
	}
	return resp
}

// TestBruteForceNoRateLimit はレート制限がないことを検証する（脆弱性の証明）
func TestBruteForceNoRateLimit(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	// 100回連続で間違ったパスワードでログインを試みる
	for i := 0; i < 100; i++ {
		resp := loginRequest(t, server.URL, "admin", "wrongpassword", nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("試行 %d: レート制限なしで401が期待されたが、%d を受信", i+1, resp.StatusCode)
		}
	}
	// 全て401を返す = レート制限が存在しない（脆弱性）
}

// TestPasswordStoredInPlaintext はパスワードが平文で保存されていることを検証する（脆弱性の証明）
func TestPasswordStoredInPlaintext(t *testing.T) {
	db := initDB()
	seedData(db)

	var storedPassword string
	err := db.QueryRow("SELECT password FROM users WHERE username = ?", "admin").Scan(&storedPassword)
	if err != nil {
		t.Fatalf("パスワードの取得に失敗: %v", err)
	}

	// パスワードが平文で保存されていることを確認（ハッシュ化されていない）
	if storedPassword != "password123" {
		t.Fatalf("パスワードが平文で保存されていない: got %s, want password123", storedPassword)
	}
}

// TestSessionFixation はセッション固定攻撃が可能であることを検証する（脆弱性の証明）
func TestSessionFixation(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	// 攻撃者が設定したセッションIDを使ってログインする
	attackerSessionID := "attacker-controlled-session-id"
	cookies := []*http.Cookie{
		{Name: "session_id", Value: attackerSessionID},
	}

	resp := loginRequest(t, server.URL, "admin", "password123", cookies)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ログインに失敗: %d", resp.StatusCode)
	}

	// レスポンスのCookieを確認: セッションIDが変更されていないこと
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session_id" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("セッションCookieが設定されていない")
	}

	// セッションIDが攻撃者のものと同じであれば、セッション固定攻撃が成功する（脆弱性）
	if sessionCookie.Value != attackerSessionID {
		t.Fatalf("セッション固定攻撃が機能していない: got %s, want %s", sessionCookie.Value, attackerSessionID)
	}
}

// TestLoginSuccess は正しい認証情報でログインできることを検証する
func TestLoginSuccess(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	resp := loginRequest(t, server.URL, "admin", "password123", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ログインに失敗: expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("レスポンスの解析に失敗: %v", err)
	}

	if result["username"] != "admin" {
		t.Fatalf("ユーザー名が一致しない: got %s, want admin", result["username"])
	}
}

// TestAdminWithValidSession は有効なセッションで管理者ページにアクセスできることを検証する
func TestAdminWithValidSession(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	// まずログインする
	resp := loginRequest(t, server.URL, "admin", "password123", nil)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ログインに失敗: %d", resp.StatusCode)
	}

	// セッションCookieを取得する
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session_id" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("セッションCookieが設定されていない")
	}

	// 管理者ページにアクセスする
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/admin", nil)
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	adminResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("管理者ページへのアクセスに失敗: %v", err)
	}
	defer adminResp.Body.Close()

	if adminResp.StatusCode != http.StatusOK {
		t.Fatalf("管理者ページへのアクセスで200が期待されたが、%d を受信", adminResp.StatusCode)
	}
}

// TestAdminWithoutSession はセッションなしで管理者ページにアクセスすると401になることを検証する
func TestAdminWithoutSession(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/admin")
	if err != nil {
		t.Fatalf("リクエストの送信に失敗: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("セッションなしで401が期待されたが、%d を受信", resp.StatusCode)
	}
}

// TestUsersEndpointExposesPlaintextPasswords はユーザー一覧でパスワードが平文で返されることを検証する
func TestUsersEndpointExposesPlaintextPasswords(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/users")
	if err != nil {
		t.Fatalf("リクエストの送信に失敗: %v", err)
	}
	defer resp.Body.Close()

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		t.Fatalf("レスポンスの解析に失敗: %v", err)
	}

	if len(users) < 2 {
		t.Fatalf("ユーザー数が不足: got %d, want >= 2", len(users))
	}

	// パスワードが平文で返されていることを確認
	for _, u := range users {
		if u.Password == "" {
			t.Fatalf("ユーザー %s のパスワードが空", u.Username)
		}
	}

	// adminのパスワードが平文であることを確認
	found := false
	for _, u := range users {
		if u.Username == "admin" && u.Password == "password123" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("adminのパスワードが平文で返されていない")
	}
}

// TestLogout はログアウト処理を検証する
func TestLogout(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	// ログインする
	resp := loginRequest(t, server.URL, "admin", "password123", nil)
	resp.Body.Close()

	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session_id" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("セッションCookieが設定されていない")
	}

	// ログアウトする
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/logout", nil)
	req.AddCookie(sessionCookie)
	client := &http.Client{}
	logoutResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("ログアウトに失敗: %v", err)
	}
	logoutResp.Body.Close()

	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("ログアウトで200が期待されたが、%d を受信", logoutResp.StatusCode)
	}

	// ログアウト後に管理者ページにアクセスすると401になることを確認
	adminReq, _ := http.NewRequest(http.MethodGet, server.URL+"/admin", nil)
	adminReq.AddCookie(sessionCookie)
	adminResp, err := client.Do(adminReq)
	if err != nil {
		t.Fatalf("管理者ページへのアクセスに失敗: %v", err)
	}
	defer adminResp.Body.Close()

	if adminResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ログアウト後に401が期待されたが、%d を受信", adminResp.StatusCode)
	}
}
