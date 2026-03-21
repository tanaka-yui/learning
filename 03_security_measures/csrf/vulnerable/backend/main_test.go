package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// テスト実行前にユーザーストアとセッションストアを初期状態にリセットする
func resetStores() {
	usersMu.Lock()
	users = map[string]*user{
		"admin": {Username: "admin", Password: "password123"},
		"user1": {Username: "user1", Password: "pass456"},
	}
	usersMu.Unlock()

	sessionMu.Lock()
	sessions = map[string]string{}
	sessionMu.Unlock()
}

// ログインしてセッションCookieを取得するヘルパー関数
func loginAndGetCookie(t *testing.T, server *httptest.Server, username, password string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	resp, err := http.Post(server.URL+"/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("ログインリクエストに失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ログインが失敗しました: ステータス %d", resp.StatusCode)
	}

	for _, c := range resp.Cookies() {
		if c.Name == "session_id" {
			return c
		}
	}
	t.Fatal("レスポンスにsession_id Cookieが含まれていません")
	return nil
}

// 有効な認証情報でのログインが200とSet-Cookie sessionヘッダーを返すことを検証する
func TestLogin_ValidCredentials(t *testing.T) {
	resetStores()
	server := httptest.NewServer(newServeMux())
	defer server.Close()

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "password123",
	})
	resp, err := http.Post(server.URL+"/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("リクエスト送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期待ステータス 200, 実際 %d", resp.StatusCode)
	}

	var foundSessionCookie bool
	for _, c := range resp.Cookies() {
		if c.Name == "session_id" && c.Value != "" {
			foundSessionCookie = true
			break
		}
	}
	if !foundSessionCookie {
		t.Error("レスポンスにsession_id Cookieが設定されていません")
	}
}

// 無効な認証情報でのログインが401を返すことを検証する
func TestLogin_InvalidCredentials(t *testing.T) {
	resetStores()
	server := httptest.NewServer(newServeMux())
	defer server.Close()

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "wrongpassword",
	})
	resp, err := http.Post(server.URL+"/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("リクエスト送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("期待ステータス 401, 実際 %d", resp.StatusCode)
	}
}

// セッションCookie付きのパスワード変更が成功することを検証する
func TestChangePassword_WithSession(t *testing.T) {
	resetStores()
	server := httptest.NewServer(newServeMux())
	defer server.Close()

	cookie := loginAndGetCookie(t, server, "admin", "password123")

	// パスワードを変更する
	body, _ := json.Marshal(map[string]string{"new_password": "newpass789"})
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/change-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("リクエスト送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期待ステータス 200, 実際 %d", resp.StatusCode)
	}

	// 新しいパスワードでログインできることを確認する
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "newpass789",
	})
	resp2, err := http.Post(server.URL+"/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("再ログインリクエストに失敗しました: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("新しいパスワードでのログインが失敗しました: ステータス %d", resp2.StatusCode)
	}
}

// セッションCookieなしのパスワード変更が401を返すことを検証する
func TestChangePassword_WithoutSession(t *testing.T) {
	resetStores()
	server := httptest.NewServer(newServeMux())
	defer server.Close()

	body, _ := json.Marshal(map[string]string{"new_password": "newpass789"})
	resp, err := http.Post(server.URL+"/change-password", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("リクエスト送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("期待ステータス 401, 実際 %d", resp.StatusCode)
	}
}

// セッションCookie付きの/meが現在のユーザー情報を返すことを検証する
func TestMe_WithSession(t *testing.T) {
	resetStores()
	server := httptest.NewServer(newServeMux())
	defer server.Close()

	cookie := loginAndGetCookie(t, server, "user1", "pass456")

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/me", nil)
	req.AddCookie(cookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("リクエスト送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期待ステータス 200, 実際 %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("レスポンスのデコードに失敗しました: %v", err)
	}

	if result["username"] != "user1" {
		t.Errorf("期待ユーザー名 'user1', 実際 '%s'", result["username"])
	}
}

// クロスオリジンリクエストがセッションCookieのみで成功することを検証する
// （CSRFトークン検証がないことを証明する）
func TestCSRFVulnerability_CrossOriginWithCookieSucceeds(t *testing.T) {
	resetStores()
	server := httptest.NewServer(newServeMux())
	defer server.Close()

	cookie := loginAndGetCookie(t, server, "admin", "password123")

	// 異なるオリジンからのリクエストをシミュレートする
	body, _ := json.Marshal(map[string]string{"new_password": "hacked123"})
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/change-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil-site.com")
	req.AddCookie(cookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("リクエスト送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	// CSRF脆弱性: 異なるオリジンからのリクエストがCSRFトークンなしで成功する
	if resp.StatusCode != http.StatusOK {
		t.Errorf("CSRF脆弱性テスト: 異なるオリジンからのリクエストが成功するはず。期待 200, 実際 %d", resp.StatusCode)
	}

	// CORSヘッダーが許可的に設定されていることを確認する（リクエスト元のオリジンをそのまま返す）
	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowOrigin != "http://evil-site.com" {
		t.Errorf("Access-Control-Allow-Originがリクエスト元のオリジンであるべき, 実際 '%s'", allowOrigin)
	}

	allowCreds := resp.Header.Get("Access-Control-Allow-Credentials")
	if allowCreds != "true" {
		t.Errorf("Access-Control-Allow-Credentialsが 'true' であるべき, 実際 '%s'", allowCreds)
	}

	// パスワードが実際に変更されたことを確認する
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "hacked123",
	})
	resp2, err := http.Post(server.URL+"/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("再ログインリクエストに失敗しました: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("攻撃者が設定したパスワードでのログインが成功するはず: ステータス %d", resp2.StatusCode)
	}
}

// ログアウトが正しくセッションを破棄することを検証する
func TestLogout(t *testing.T) {
	resetStores()
	server := httptest.NewServer(newServeMux())
	defer server.Close()

	cookie := loginAndGetCookie(t, server, "admin", "password123")

	// ログアウトする
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/logout", nil)
	req.AddCookie(cookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("リクエスト送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期待ステータス 200, 実際 %d", resp.StatusCode)
	}

	// ログアウト後に/meへアクセスすると401になることを確認する
	req2, _ := http.NewRequest(http.MethodGet, server.URL+"/me", nil)
	req2.AddCookie(cookie)

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("リクエスト送信に失敗しました: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("ログアウト後は401を返すべき, 実際 %d", resp2.StatusCode)
	}
}
