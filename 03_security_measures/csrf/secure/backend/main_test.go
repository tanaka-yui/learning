package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// テスト用のヘルパー: ログインしてセッションCookieを取得する
func loginForTest(t *testing.T, server *httptest.Server) *http.Cookie {
	t.Helper()
	body := `{"username":"admin","password":"password123"}`
	req, err := http.NewRequest(http.MethodPost, server.URL+"/login", strings.NewReader(body))
	if err != nil {
		t.Fatalf("リクエスト作成に失敗: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("ログインリクエストに失敗: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ログインに失敗: ステータスコード %d", resp.StatusCode)
	}

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "session_id" {
			return cookie
		}
	}
	t.Fatal("セッションCookieが見つかりません")
	return nil
}

// テスト用のヘルパー: CSRFトークンを取得する
func getCSRFTokenForTest(t *testing.T, server *httptest.Server, sessionCookie *http.Cookie) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, server.URL+"/csrf-token", nil)
	if err != nil {
		t.Fatalf("リクエスト作成に失敗: %v", err)
	}
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("CSRFトークン取得リクエストに失敗: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CSRFトークン取得に失敗: ステータスコード %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("レスポンスのデコードに失敗: %v", err)
	}

	token, ok := result["token"]
	if !ok || token == "" {
		t.Fatal("CSRFトークンがレスポンスに含まれていません")
	}
	return token
}

// CSRFトークンなしでパスワード変更すると403が返ることを検証する
func TestChangePasswordWithoutCSRFToken(t *testing.T) {
	// テスト間の状態干渉を防ぐためにストアをリセットする
	resetStores()

	server := httptest.NewServer(newServeMux())
	defer server.Close()

	sessionCookie := loginForTest(t, server)

	body := `{"new_password":"newpass123"}`
	req, err := http.NewRequest(http.MethodPost, server.URL+"/change-password", strings.NewReader(body))
	if err != nil {
		t.Fatalf("リクエスト作成に失敗: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("パスワード変更リクエストに失敗: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("CSRFトークンなしで403を期待しましたが、%d が返されました", resp.StatusCode)
	}
}

// 正しいCSRFトークン付きでパスワード変更すると200が返ることを検証する
func TestChangePasswordWithCorrectCSRFToken(t *testing.T) {
	resetStores()

	server := httptest.NewServer(newServeMux())
	defer server.Close()

	sessionCookie := loginForTest(t, server)
	csrfToken := getCSRFTokenForTest(t, server, sessionCookie)

	body := `{"new_password":"newpass123"}`
	req, err := http.NewRequest(http.MethodPost, server.URL+"/change-password", strings.NewReader(body))
	if err != nil {
		t.Fatalf("リクエスト作成に失敗: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("パスワード変更リクエストに失敗: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("正しいCSRFトークン付きで200を期待しましたが、%d が返されました", resp.StatusCode)
	}
}

// GET /csrf-token がCSRFトークンをJSON形式で返すことを検証する
func TestGetCSRFToken(t *testing.T) {
	resetStores()

	server := httptest.NewServer(newServeMux())
	defer server.Close()

	sessionCookie := loginForTest(t, server)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/csrf-token", nil)
	if err != nil {
		t.Fatalf("リクエスト作成に失敗: %v", err)
	}
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("CSRFトークン取得リクエストに失敗: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CSRFトークン取得で200を期待しましたが、%d が返されました", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("レスポンスのデコードに失敗: %v", err)
	}

	token, ok := result["token"]
	if !ok {
		t.Error("レスポンスに 'token' キーが含まれていません")
	}
	if token == "" {
		t.Error("CSRFトークンが空です")
	}
	if len(token) != 32 {
		t.Errorf("CSRFトークンの長さが32文字を期待しましたが、%d でした", len(token))
	}
}

// CookieにSameSite=StrictとHttpOnly=trueが設定されていることを検証する
func TestCookieSecuritySettings(t *testing.T) {
	resetStores()

	server := httptest.NewServer(newServeMux())
	defer server.Close()

	body := `{"username":"admin","password":"password123"}`
	req, err := http.NewRequest(http.MethodPost, server.URL+"/login", strings.NewReader(body))
	if err != nil {
		t.Fatalf("リクエスト作成に失敗: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("ログインリクエストに失敗: %v", err)
	}
	defer resp.Body.Close()

	var sessionCookie *http.Cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "session_id" {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("セッションCookieが見つかりません")
	}

	if !sessionCookie.HttpOnly {
		t.Error("CookieにHttpOnly=trueが設定されていません")
	}

	// SameSite=Strictの検証
	// net/http.Cookieでは SameSite フィールドで確認する
	if sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("CookieのSameSiteがStrictを期待しましたが、%v が設定されていました", sessionCookie.SameSite)
	}
}

// 不正なCSRFトークンでパスワード変更すると403が返ることを検証する
func TestChangePasswordWithInvalidCSRFToken(t *testing.T) {
	resetStores()

	server := httptest.NewServer(newServeMux())
	defer server.Close()

	sessionCookie := loginForTest(t, server)

	body := `{"new_password":"newpass123"}`
	req, err := http.NewRequest(http.MethodPost, server.URL+"/change-password", strings.NewReader(body))
	if err != nil {
		t.Fatalf("リクエスト作成に失敗: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "invalid-token-value")
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("パスワード変更リクエストに失敗: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("不正なCSRFトークンで403を期待しましたが、%d が返されました", resp.StatusCode)
	}
}

// テスト間の状態干渉を防ぐためにストアをリセットする
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

	csrfTokenMu.Lock()
	csrfTokens = map[string]string{}
	csrfTokenMu.Unlock()
}
