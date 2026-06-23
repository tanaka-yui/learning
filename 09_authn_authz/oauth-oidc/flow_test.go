package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// testEnv はテスト用のサーバ一式(httptest)とその issuer を保持する。
type testEnv struct {
	server *httptest.Server
	issuer string
	client *http.Client
}

// newTestEnv は httptest.Server を起動し、issuer をそのサーバの URL に設定する。
// RP/RS は同一オリジンなので、issuer = server.URL とすることで実際の HTTP 経由で連携できる。
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	// リダイレクトを追わないクライアント(認可レスポンスの Location を検査するため)。
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	keys := NewKeyMaterial()
	var srv *httptest.Server
	// setupRouter には起動後の URL を issuer として渡したいので、後から差し替える。
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// プレースホルダ(下で置き換える)
		http.NotFound(w, r)
	}))
	store := NewStore(srv.URL)
	srv.Config.Handler = setupRouter(srv.URL, keys, store, client)

	t.Cleanup(srv.Close)
	return &testEnv{server: srv, issuer: srv.URL, client: client}
}

// approveAndGetCode は /authorize に同意フォームを POST し、redirect の Location から
// 認可コードと state を取り出す。
func (e *testEnv) approveAndGetCode(t *testing.T, p map[string]string) (code, state string) {
	t.Helper()
	form := url.Values{}
	for k, v := range p {
		form.Set(k, v)
	}
	form.Set("approve", "yes")

	resp, err := e.client.PostForm(e.issuer+"/authorize", form)
	if err != nil {
		t.Fatalf("/authorize POST 失敗: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("/authorize status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("Location パース失敗: %v", err)
	}
	q := u.Query()
	if errCode := q.Get("error"); errCode != "" {
		t.Fatalf("認可エラーが返りました: %s", errCode)
	}
	return q.Get("code"), q.Get("state")
}

// exchangeCode は /token に authorization_code グラントで交換リクエストを送る。
func (e *testEnv) exchangeCode(t *testing.T, code, verifier, clientID, redirectURI string) (*http.Response, map[string]any) {
	t.Helper()
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("client_id", clientID)
	form.Set("redirect_uri", redirectURI)

	resp, err := e.client.PostForm(e.issuer+"/token", form)
	if err != nil {
		t.Fatalf("/token POST 失敗: %v", err)
	}
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	return resp, out
}

// defaultAuthParams は public クライアントの標準的な認可パラメータを返す。
func (e *testEnv) defaultAuthParams(verifier, state, nonce string) map[string]string {
	return map[string]string{
		"client_id":             publicClientID,
		"redirect_uri":          e.issuer + "/app/callback",
		"response_type":         "code",
		"scope":                 "openid profile email",
		"state":                 state,
		"nonce":                 nonce,
		"code_challenge":        pkceS256Challenge(verifier),
		"code_challenge_method": "S256",
	}
}

// TestAuthorizationCodeFlowReturnsTokens は認可コード + PKCE 交換で
// access_token と id_token が返り、id_token が JWKS の公開鍵で検証できることを確認する。
func TestAuthorizationCodeFlowReturnsTokens(t *testing.T) {
	e := newTestEnv(t)
	verifier := "test-verifier-abc123_known-value-for-pkce"
	nonce := "nonce-xyz-789"
	state := "state-123"

	code, gotState := e.approveAndGetCode(t, e.defaultAuthParams(verifier, state, nonce))
	if code == "" {
		t.Fatal("認可コードが取得できませんでした")
	}
	if gotState != state {
		t.Errorf("state = %q, want %q", gotState, state)
	}

	resp, out := e.exchangeCode(t, code, verifier, publicClientID, e.issuer+"/app/callback")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/token status = %d, want 200 (resp=%v)", resp.StatusCode, out)
	}
	if out["access_token"] == nil {
		t.Error("access_token がありません")
	}
	if out["refresh_token"] == nil {
		t.Error("refresh_token がありません")
	}
	idToken, _ := out["id_token"].(string)
	if idToken == "" {
		t.Fatal("id_token がありません")
	}

	// id_token を JWKS の公開鍵で検証する(iss, aud=client, nonce)。
	pub := fetchJWKSPublicKey(t, e)
	claims := parseAndVerifyIDToken(t, idToken, pub)
	if claims["iss"] != e.issuer {
		t.Errorf("iss = %v, want %v", claims["iss"], e.issuer)
	}
	if !audContains(claims["aud"], publicClientID) {
		t.Errorf("aud = %v, want に %q を含む", claims["aud"], publicClientID)
	}
	if claims["nonce"] != nonce {
		t.Errorf("nonce = %v, want %v", claims["nonce"], nonce)
	}
	if claims["sub"] != demoUserSub {
		t.Errorf("sub = %v, want %v", claims["sub"], demoUserSub)
	}
}

// TestPKCERejectsWrongVerifier は誤った code_verifier では /token が失敗することを確認する。
func TestPKCERejectsWrongVerifier(t *testing.T) {
	e := newTestEnv(t)
	verifier := "correct-verifier-value-1234567890-abcdef"

	code, _ := e.approveAndGetCode(t, e.defaultAuthParams(verifier, "s", "n"))
	if code == "" {
		t.Fatal("認可コードが取得できませんでした")
	}

	// 違う verifier を送ると S256 challenge が一致しないため拒否される。
	resp, out := e.exchangeCode(t, code, "WRONG-verifier-value", publicClientID, e.issuer+"/app/callback")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("誤 verifier の status = %d, want 400 (resp=%v)", resp.StatusCode, out)
	}
	if out["error"] != "invalid_grant" {
		t.Errorf("error = %v, want invalid_grant", out["error"])
	}
}

// TestAuthorizationCodeIsSingleUse は認可コードが単回使用であることを確認する。
func TestAuthorizationCodeIsSingleUse(t *testing.T) {
	e := newTestEnv(t)
	verifier := "single-use-verifier-1234567890-abcdefgh"

	code, _ := e.approveAndGetCode(t, e.defaultAuthParams(verifier, "s", "n"))

	// 1 回目は成功する。
	resp1, _ := e.exchangeCode(t, code, verifier, publicClientID, e.issuer+"/app/callback")
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("1 回目の status = %d, want 200", resp1.StatusCode)
	}
	// 2 回目は同じコードが削除済みのため失敗する。
	resp2, out2 := e.exchangeCode(t, code, verifier, publicClientID, e.issuer+"/app/callback")
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("2 回目の status = %d, want 400 (resp=%v)", resp2.StatusCode, out2)
	}
	if out2["error"] != "invalid_grant" {
		t.Errorf("error = %v, want invalid_grant", out2["error"])
	}
}

// TestRefreshTokenGrant はリフレッシュトークンで新しいアクセストークンが得られることを確認する。
func TestRefreshTokenGrant(t *testing.T) {
	e := newTestEnv(t)
	verifier := "refresh-flow-verifier-1234567890-abcdefg"

	code, _ := e.approveAndGetCode(t, e.defaultAuthParams(verifier, "s", "n"))
	_, out := e.exchangeCode(t, code, verifier, publicClientID, e.issuer+"/app/callback")
	rt, _ := out["refresh_token"].(string)
	if rt == "" {
		t.Fatal("refresh_token がありません")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", rt)
	resp, err := e.client.PostForm(e.issuer+"/token", form)
	if err != nil {
		t.Fatalf("refresh /token 失敗: %v", err)
	}
	var refreshed map[string]any
	json.NewDecoder(resp.Body).Decode(&refreshed)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200", resp.StatusCode)
	}
	if refreshed["access_token"] == nil {
		t.Error("リフレッシュ後の access_token がありません")
	}
}

// --- JWKS / 検証ヘルパ ---

// fetchJWKSPublicKey は /jwks.json から RSA 公開鍵を復元する(検証鍵の取得を実機どおり行う)。
func fetchJWKSPublicKey(t *testing.T, e *testEnv) any {
	t.Helper()
	resp, err := e.client.Get(e.issuer + "/jwks.json")
	if err != nil {
		t.Fatalf("/jwks.json 取得失敗: %v", err)
	}
	defer resp.Body.Close()
	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		t.Fatalf("JWKS デコード失敗: %v", err)
	}
	if len(jwks.Keys) == 0 {
		t.Fatal("JWKS に鍵がありません")
	}
	pub, err := jwkToRSAPublicKey(jwks.Keys[0])
	if err != nil {
		t.Fatalf("JWK から公開鍵復元失敗: %v", err)
	}
	return pub
}

// audContains は aud(文字列 or 配列)に target が含まれるか確認する。
func audContains(aud any, target string) bool {
	switch v := aud.(type) {
	case string:
		return v == target
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && s == target {
				return true
			}
		}
	}
	return false
}
