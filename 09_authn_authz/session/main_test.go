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
