package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// newTestApp はHS256またはRS256でテスト用サーバを構築するヘルパー
func newTestApp(signer Signer) *httptest.Server {
	bl := NewBlocklist()
	users := NewUserStore()
	return httptest.NewServer(setupRouter(signer, bl, users))
}

// doLogin はPOST /login を実行してtokenResponseを返す
func doLogin(t *testing.T, server *httptest.Server, username, password string) tokenResponse {
	t.Helper()
	body, _ := json.Marshal(loginRequest{Username: username, Password: password})
	resp, err := http.Post(server.URL+"/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("ログインリクエスト失敗: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ログインステータス = %d, want 200", resp.StatusCode)
	}
	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		t.Fatalf("レスポンスのデコード失敗: %v", err)
	}
	return tr
}

// getProtected はGET /protected をBearerトークン付きで実行してステータスとボディを返す
func getProtected(t *testing.T, server *httptest.Server, accessToken string) (int, map[string]string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/protected", nil)
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("protectedリクエスト失敗: %v", err)
	}
	defer resp.Body.Close()
	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

// doRefresh はPOST /refresh をBearerトークン付きで実行してtokenResponseを返す
func doRefresh(t *testing.T, server *httptest.Server, refreshToken string) (int, tokenResponse) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+refreshToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("refreshリクエスト失敗: %v", err)
	}
	defer resp.Body.Close()
	var tr tokenResponse
	json.NewDecoder(resp.Body).Decode(&tr)
	return resp.StatusCode, tr
}

// doLogout はPOST /logout をBearerトークン付きで実行してステータスを返す
func doLogout(t *testing.T, server *httptest.Server, refreshToken string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/logout", nil)
	req.Header.Set("Authorization", "Bearer "+refreshToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("logoutリクエスト失敗: %v", err)
	}
	return resp.StatusCode
}

// issueExpiredAccessToken はテスト専用の期限切れアクセストークンを発行する
func issueExpiredAccessToken(t *testing.T, signer Signer, username string) string {
	t.Helper()
	claims := &tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-10 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-5 * time.Minute)), // 過去に設定
			ID:        newTokenID(),
		},
		TokenType: "access",
	}
	token, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("期限切れトークン発行失敗: %v", err)
	}
	return token
}

// TestLoginReturnsTokens はHS256/RS256の両方でログインするとトークンペアが返ることを検証する
func TestLoginReturnsTokens(t *testing.T) {
	tests := []struct {
		name   string
		signer Signer
	}{
		{"HS256", newHMACSigner()},
		{"RS256", newRSASigner()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newTestApp(tt.signer)
			defer server.Close()

			// 正しい認証情報でログインするとトークンが返る
			tr := doLogin(t, server, "alice", "password123")
			if tr.AccessToken == "" {
				t.Error("アクセストークンが空です")
			}
			if tr.RefreshToken == "" {
				t.Error("リフレッシュトークンが空です")
			}
			if tr.AccessToken == tr.RefreshToken {
				t.Error("アクセストークンとリフレッシュトークンが同一です")
			}
		})
	}
}

// TestProtectedEndpoint は /protected のアクセス制御を検証する
func TestProtectedEndpoint(t *testing.T) {
	signer := newHMACSigner()
	server := newTestApp(signer)
	defer server.Close()

	// トークンなし → 401
	status, _ := getProtected(t, server, "")
	if status != http.StatusUnauthorized {
		t.Errorf("トークンなしの status = %d, want 401", status)
	}

	// 正常なアクセストークン → 200 + username
	tr := doLogin(t, server, "alice", "password123")
	status, body := getProtected(t, server, tr.AccessToken)
	if status != http.StatusOK {
		t.Errorf("有効トークンの status = %d, want 200", status)
	}
	if body["username"] != "alice" {
		t.Errorf("username = %q, want alice", body["username"])
	}

	// 改ざんされたトークン → 401
	tampered := tr.AccessToken[:len(tr.AccessToken)-4] + "XXXX"
	status, _ = getProtected(t, server, tampered)
	if status != http.StatusUnauthorized {
		t.Errorf("改ざんトークンの status = %d, want 401", status)
	}

	// 期限切れトークン → 401
	expired := issueExpiredAccessToken(t, signer, "alice")
	status, _ = getProtected(t, server, expired)
	if status != http.StatusUnauthorized {
		t.Errorf("期限切れトークンの status = %d, want 401", status)
	}

	// リフレッシュトークンをアクセストークンとして使用 → 401
	status, _ = getProtected(t, server, tr.RefreshToken)
	if status != http.StatusUnauthorized {
		t.Errorf("リフレッシュトークン使用の status = %d, want 401", status)
	}
}

// TestRefreshRotation はリフレッシュトークンのローテーションを検証する
func TestRefreshRotation(t *testing.T) {
	signer := newHMACSigner()
	server := newTestApp(signer)
	defer server.Close()

	// ログインして最初のトークンペアを取得する
	tr1 := doLogin(t, server, "alice", "password123")

	// リフレッシュで新しいトークンペアを取得する
	status, tr2 := doRefresh(t, server, tr1.RefreshToken)
	if status != http.StatusOK {
		t.Fatalf("refreshの status = %d, want 200", status)
	}
	if tr2.AccessToken == "" || tr2.RefreshToken == "" {
		t.Fatal("新しいトークンペアが空です")
	}

	// 新しいアクセストークンで /protected にアクセスできる
	status, body := getProtected(t, server, tr2.AccessToken)
	if status != http.StatusOK {
		t.Errorf("新アクセストークンの status = %d, want 200", status)
	}
	if body["username"] != "alice" {
		t.Errorf("username = %q, want alice", body["username"])
	}

	// 古いリフレッシュトークンは失効済みのため再利用できない(401)
	status, _ = doRefresh(t, server, tr1.RefreshToken)
	if status != http.StatusUnauthorized {
		t.Errorf("古いリフレッシュトークン再利用の status = %d, want 401", status)
	}
}

// TestLogoutRevokesRefreshToken はログアウトでリフレッシュトークンが失効することを検証する
func TestLogoutRevokesRefreshToken(t *testing.T) {
	signer := newHMACSigner()
	server := newTestApp(signer)
	defer server.Close()

	// ログインしてトークンを取得する
	tr := doLogin(t, server, "bob", "pass456")

	// ログアウトでリフレッシュトークンを失効させる
	status := doLogout(t, server, tr.RefreshToken)
	if status != http.StatusOK {
		t.Fatalf("logoutの status = %d, want 200", status)
	}

	// ログアウト後にリフレッシュを試みると 401 が返る
	status, _ = doRefresh(t, server, tr.RefreshToken)
	if status != http.StatusUnauthorized {
		t.Errorf("ログアウト後のrefreshの status = %d, want 401", status)
	}
}

// TestRS256LoginAndProtected はRS256でログインと /protected を検証する
func TestRS256LoginAndProtected(t *testing.T) {
	// RS256 Signerを生成して検証する
	signer := newRSASigner()
	server := newTestApp(signer)
	defer server.Close()

	// ログインでトークンが返る
	tr := doLogin(t, server, "alice", "password123")
	if tr.AccessToken == "" {
		t.Fatal("RS256: アクセストークンが空です")
	}

	// 有効なアクセストークンで /protected にアクセスできる
	status, body := getProtected(t, server, tr.AccessToken)
	if status != http.StatusOK {
		t.Errorf("RS256 protectedの status = %d, want 200", status)
	}
	if body["username"] != "alice" {
		t.Errorf("RS256 username = %q, want alice", body["username"])
	}

	// RS256トークンをHS256 Signerで検証しようとすると 401 になる
	hmacSigner := newHMACSigner()
	hmacServer := newTestApp(hmacSigner)
	defer hmacServer.Close()
	status, _ = getProtected(t, hmacServer, tr.AccessToken)
	if status != http.StatusUnauthorized {
		t.Errorf("アルゴリズム不一致の status = %d, want 401", status)
	}
}
