package main

import (
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

// カスタムオリジンヘッダーなしでパスワード変更すると403が返ることを検証する
func TestChangePasswordWithoutCustomOrigin(t *testing.T) {
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
		t.Errorf("カスタムオリジンヘッダーなしで403を期待しましたが、%d が返されました", resp.StatusCode)
	}
}

// 正しいカスタムオリジンヘッダー付きでパスワード変更すると200が返ることを検証する
func TestChangePasswordWithCorrectCustomOrigin(t *testing.T) {
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
	req.Header.Set("X-Custom-Origin", getAllowedOrigin())
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("パスワード変更リクエストに失敗: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("正しいカスタムオリジン付きで200を期待しましたが、%d が返されました", resp.StatusCode)
	}
}

// 不正なカスタムオリジンヘッダーでパスワード変更すると403が返ることを検証する
func TestChangePasswordWithInvalidCustomOrigin(t *testing.T) {
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
	req.Header.Set("X-Custom-Origin", "http://evil-site.com")
	req.AddCookie(sessionCookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("パスワード変更リクエストに失敗: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("不正なカスタムオリジンで403を期待しましたが、%d が返されました", resp.StatusCode)
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

	if sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("CookieのSameSiteがStrictを期待しましたが、%v が設定されていました", sessionCookie.SameSite)
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
}
