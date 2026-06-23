package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

// TestPKCEKnownVector は RFC 7636 の既知ベクタで S256 challenge の計算を検証する。
// verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk" のとき
// challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" となる(RFC 7636 Appendix B)。
func TestPKCEKnownVector(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := pkceS256Challenge(verifier); got != want {
		t.Errorf("S256 challenge = %q, want %q", got, want)
	}
}

// TestDiscoveryDocument は /.well-known/openid-configuration が issuer と各エンドポイントを返すことを確認する。
func TestDiscoveryDocument(t *testing.T) {
	e := newTestEnv(t)
	resp, err := e.client.Get(e.issuer + "/.well-known/openid-configuration")
	if err != nil {
		t.Fatalf("discovery 取得失敗: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("discovery status = %d, want 200", resp.StatusCode)
	}
	var doc map[string]any
	json.NewDecoder(resp.Body).Decode(&doc)

	if doc["issuer"] != e.issuer {
		t.Errorf("issuer = %v, want %v", doc["issuer"], e.issuer)
	}
	for _, key := range []string{"authorization_endpoint", "token_endpoint", "jwks_uri", "userinfo_endpoint"} {
		if doc[key] == nil || doc[key] == "" {
			t.Errorf("discovery に %s がありません", key)
		}
	}
	// S256 がサポートされていること。
	methods, _ := doc["code_challenge_methods_supported"].([]any)
	if !containsAny(methods, "S256") {
		t.Errorf("code_challenge_methods_supported に S256 がありません: %v", methods)
	}
}

// TestJWKSReturnsUsableKey は /jwks.json が利用可能な RSA 公開鍵を返すことを確認する。
func TestJWKSReturnsUsableKey(t *testing.T) {
	e := newTestEnv(t)
	pub := fetchJWKSPublicKey(t, e) // 復元できれば使用可能とみなす
	if pub == nil {
		t.Fatal("JWKS から公開鍵を復元できませんでした")
	}
}

// TestAPIMeRequiresValidToken は /api/me がトークン無しで 401、
// 有効なアクセストークンで 200 + クレームを返すことを確認する。
func TestAPIMeRequiresValidToken(t *testing.T) {
	e := newTestEnv(t)

	// トークン無し → 401
	resp, err := e.client.Get(e.issuer + "/api/me")
	if err != nil {
		t.Fatalf("/api/me 取得失敗: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("トークン無しの status = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()

	// 有効なアクセストークンを認可コードフローで取得する。
	verifier := "api-me-verifier-1234567890-abcdefghijklmn"
	code, _ := e.approveAndGetCode(t, e.defaultAuthParams(verifier, "s", "n"))
	_, out := e.exchangeCode(t, code, verifier, publicClientID, e.issuer+"/app/callback")
	access, _ := out["access_token"].(string)
	if access == "" {
		t.Fatal("access_token が取得できませんでした")
	}

	// Bearer 付き → 200 + sub
	req, _ := http.NewRequest(http.MethodGet, e.issuer+"/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	resp2, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("/api/me (Bearer) 失敗: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Bearer 付きの status = %d, want 200", resp2.StatusCode)
	}
	var claims map[string]any
	json.NewDecoder(resp2.Body).Decode(&claims)
	if claims["sub"] != demoUserSub {
		t.Errorf("sub = %v, want %v", claims["sub"], demoUserSub)
	}
}

// TestClientCredentialsGrant は M2M(client_credentials)で得たトークンが
// /api/me で受理されることを確認する。
func TestClientCredentialsGrant(t *testing.T) {
	e := newTestEnv(t)

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", confidentialClient)
	form.Set("client_secret", confidentialSecret)
	form.Set("scope", "api.read")

	resp, err := e.client.PostForm(e.issuer+"/token", form)
	if err != nil {
		t.Fatalf("client_credentials /token 失敗: %v", err)
	}
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("client_credentials status = %d, want 200 (resp=%v)", resp.StatusCode, out)
	}
	access, _ := out["access_token"].(string)
	if access == "" {
		t.Fatal("client_credentials の access_token がありません")
	}
	// M2M はユーザ本人が居ないため id_token は返らない。
	if out["id_token"] != nil {
		t.Error("client_credentials で id_token が返っています(返るべきでない)")
	}

	// このトークンが /api/me で通ること。
	req, _ := http.NewRequest(http.MethodGet, e.issuer+"/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	resp2, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("/api/me (M2M Bearer) 失敗: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("M2M トークンの /api/me status = %d, want 200", resp2.StatusCode)
	}
	var claims map[string]any
	json.NewDecoder(resp2.Body).Decode(&claims)
	if claims["client_id"] != confidentialClient {
		t.Errorf("client_id = %v, want %v", claims["client_id"], confidentialClient)
	}
}

// TestClientCredentialsRejectsBadSecret は誤った client_secret を拒否することを確認する。
func TestClientCredentialsRejectsBadSecret(t *testing.T) {
	e := newTestEnv(t)
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", confidentialClient)
	form.Set("client_secret", "wrong-secret")

	resp, err := e.client.PostForm(e.issuer+"/token", form)
	if err != nil {
		t.Fatalf("/token 失敗: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("誤 secret の status = %d, want 401", resp.StatusCode)
	}
}

// TestFullBrowserFlowViaCallback は RP の /app/login → /authorize 同意 → /app/callback
// までを通し、最終的に /app/ がログイン済みユーザを表示することを確認する(疑似 e2e)。
func TestFullBrowserFlowViaCallback(t *testing.T) {
	e := newTestEnv(t)
	// Cookie を保持しつつリダイレクトを追わないクライアント。
	jarClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	// 1) /app/login → /authorize へのリダイレクト URL を得る。
	loginResp, err := jarClient.Get(e.issuer + "/app/login")
	if err != nil {
		t.Fatalf("/app/login 失敗: %v", err)
	}
	loginResp.Body.Close()
	authorizeURL := loginResp.Header.Get("Location")
	if authorizeURL == "" {
		t.Fatal("/app/login がリダイレクトしませんでした")
	}
	au, _ := url.Parse(authorizeURL)
	q := au.Query()

	// 2) 同意フォームを POST(RP が生成したパラメータをそのまま使う)。
	p := map[string]string{}
	for _, k := range []string{"client_id", "redirect_uri", "response_type", "scope", "state", "nonce", "code_challenge", "code_challenge_method"} {
		p[k] = q.Get(k)
	}
	code, _ := e.approveAndGetCode(t, p)
	if code == "" {
		t.Fatal("認可コードが得られませんでした")
	}

	// 3) /app/callback?code=...&state=... を叩く(RP がコード交換 + id_token 検証を行う)。
	cbURL := e.issuer + "/app/callback?" + url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
	cbResp, err := jarClient.Get(cbURL)
	if err != nil {
		t.Fatalf("/app/callback 失敗: %v", err)
	}
	cbResp.Body.Close()
	if cbResp.StatusCode != http.StatusFound {
		t.Fatalf("/app/callback status = %d, want 302 (/app/ へ)", cbResp.StatusCode)
	}
	var rpCookie *http.Cookie
	for _, c := range cbResp.Cookies() {
		if c.Name == rpCookieName {
			rpCookie = c
		}
	}
	if rpCookie == nil {
		t.Fatal("callback で RP セッション Cookie が設定されませんでした")
	}
	if !rpCookie.HttpOnly {
		t.Error("RP セッション Cookie が HttpOnly ではありません")
	}
}

// containsAny は []any に target 文字列が含まれるか確認する。
func containsAny(items []any, target string) bool {
	for _, x := range items {
		if s, ok := x.(string); ok && s == target {
			return true
		}
	}
	return false
}
