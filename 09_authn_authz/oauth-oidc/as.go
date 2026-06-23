package main

import (
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// 各トークンの有効期限(学習用に短め)。
const (
	accessTokenTTL = 5 * time.Minute
	idTokenTTL     = 5 * time.Minute
	rsAudience     = "demo-resource-server" // アクセストークンの aud(Resource Server)
)

// AS は Authorization Server の依存をまとめる。
type AS struct {
	issuer string
	keys   *KeyMaterial
	store  *Store
}

// approveTmpl は /authorize の同意画面(学習用の最小フォーム)。
// hidden フィールドで認可リクエストのパラメータを /authorize の POST に引き継ぐ。
var approveTmpl = template.Must(template.New("approve").Parse(`<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>認可の同意</title></head>
<body>
<h1>認可サーバ: ログイン &amp; 同意</h1>
<p>クライアント <b>{{.ClientID}}</b> が以下のスコープを要求しています:</p>
<p><code>{{.Scope}}</code></p>
<p>デモユーザ <b>Alice</b> として承認しますか?</p>
<form method="POST" action="/authorize">
  <input type="hidden" name="client_id" value="{{.ClientID}}">
  <input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
  <input type="hidden" name="response_type" value="code">
  <input type="hidden" name="scope" value="{{.Scope}}">
  <input type="hidden" name="state" value="{{.State}}">
  <input type="hidden" name="nonce" value="{{.Nonce}}">
  <input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
  <input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">
  <button type="submit" name="approve" value="yes">Alice として承認する</button>
</form>
</body></html>`))

// authorizeParams は /authorize で受け取る認可リクエストのパラメータ。
type authorizeParams struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	State               string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
}

// parseAuthorizeParams はクエリ/フォームから認可パラメータを取り出す。
func parseAuthorizeParams(r *http.Request) authorizeParams {
	get := r.FormValue
	return authorizeParams{
		ClientID:            get("client_id"),
		RedirectURI:         get("redirect_uri"),
		ResponseType:        get("response_type"),
		Scope:               get("scope"),
		State:               get("state"),
		Nonce:               get("nonce"),
		CodeChallenge:       get("code_challenge"),
		CodeChallengeMethod: get("code_challenge_method"),
	}
}

// validRedirect は redirect_uri が登録済みか(完全一致)を確認する。
func validRedirect(c *Client, uri string) bool {
	for _, u := range c.RedirectURIs {
		if u == uri {
			return true
		}
	}
	return false
}

// handleAuthorizeGET は認可エンドポイント(GET)。バリデーション後に同意画面を表示する。
func (a *AS) handleAuthorizeGET(w http.ResponseWriter, r *http.Request) {
	p := parseAuthorizeParams(r)
	client, ok := a.store.Client(p.ClientID)
	if !ok {
		http.Error(w, "unknown client_id", http.StatusBadRequest)
		return
	}
	if !validRedirect(client, p.RedirectURI) {
		// redirect_uri が不正な場合はリダイレクトせず直接エラー(オープンリダイレクト防止)。
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	if p.ResponseType != "code" {
		a.redirectError(w, r, p.RedirectURI, p.State, "unsupported_response_type")
		return
	}
	if p.CodeChallenge == "" || p.CodeChallengeMethod != "S256" {
		// public クライアントには PKCE(S256)を必須とする。
		a.redirectError(w, r, p.RedirectURI, p.State, "invalid_request")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	approveTmpl.Execute(w, p)
}

// handleAuthorizePOST は同意フォームの送信を受け、認可コードを発行して redirect_uri に戻す。
func (a *AS) handleAuthorizePOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	p := parseAuthorizeParams(r)
	client, ok := a.store.Client(p.ClientID)
	if !ok || !validRedirect(client, p.RedirectURI) {
		http.Error(w, "invalid client/redirect", http.StatusBadRequest)
		return
	}
	if r.FormValue("approve") != "yes" {
		a.redirectError(w, r, p.RedirectURI, p.State, "access_denied")
		return
	}
	// 学習用: 認証は省略し、シードユーザ alice を本人として扱う。
	code := &AuthCode{
		Code:          randToken(),
		ClientID:      p.ClientID,
		RedirectURI:   p.RedirectURI,
		CodeChallenge: p.CodeChallenge,
		Nonce:         p.Nonce,
		Sub:           demoUserSub,
		Scope:         p.Scope,
	}
	a.store.SaveCode(code)

	u, _ := url.Parse(p.RedirectURI)
	q := u.Query()
	q.Set("code", code.Code)
	if p.State != "" {
		q.Set("state", p.State)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// redirectError は認可エラーを redirect_uri に error/state を付けて返す。
func (a *AS) redirectError(w http.ResponseWriter, r *http.Request, redirectURI, state, errCode string) {
	u, err := url.Parse(redirectURI)
	if err != nil || redirectURI == "" {
		http.Error(w, errCode, http.StatusBadRequest)
		return
	}
	q := u.Query()
	q.Set("error", errCode)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// handleToken はトークンエンドポイント。grant_type ごとに分岐する。
func (a *AS) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		tokenError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	switch r.FormValue("grant_type") {
	case "authorization_code":
		a.grantAuthorizationCode(w, r)
	case "refresh_token":
		a.grantRefreshToken(w, r)
	case "client_credentials":
		a.grantClientCredentials(w, r)
	default:
		tokenError(w, http.StatusBadRequest, "unsupported_grant_type")
	}
}

// grantAuthorizationCode は認可コード + PKCE を検証してトークンを発行する。
func (a *AS) grantAuthorizationCode(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	verifier := r.FormValue("code_verifier")
	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")

	ac, ok := a.store.TakeCode(code) // 取り出し時に削除 = 単回使用
	if !ok {
		tokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	// コードに束縛した client_id / redirect_uri と一致するか確認する。
	if ac.ClientID != clientID || ac.RedirectURI != redirectURI {
		tokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	// PKCE: 保存した code_challenge と、verifier から計算した S256 が一致するか。
	if verifier == "" || pkceS256Challenge(verifier) != ac.CodeChallenge {
		tokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}

	user, ok := a.store.User(ac.Sub)
	if !ok {
		tokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}

	access, err := a.keys.newAccessToken(a.issuer, rsAudience, ac.Sub, ac.ClientID, ac.Scope, accessTokenTTL)
	if err != nil {
		tokenError(w, http.StatusInternalServerError, "server_error")
		return
	}
	resp := map[string]any{
		"access_token": access,
		"token_type":   "Bearer",
		"expires_in":   int(accessTokenTTL.Seconds()),
		"scope":        ac.Scope,
	}
	// openid スコープがあれば ID Token を発行する(OIDC)。
	if hasScope(ac.Scope, "openid") {
		idTok, err := a.keys.newIDToken(a.issuer, ac.ClientID, ac.Sub, ac.Nonce, user.Name, user.Email, idTokenTTL)
		if err != nil {
			tokenError(w, http.StatusInternalServerError, "server_error")
			return
		}
		resp["id_token"] = idTok
	}
	// リフレッシュトークンを発行・保存する。
	rt := &RefreshToken{Token: randToken(), ClientID: ac.ClientID, Sub: ac.Sub, Scope: ac.Scope}
	a.store.SaveRefresh(rt)
	resp["refresh_token"] = rt.Token

	writeJSON(w, http.StatusOK, resp)
}

// grantRefreshToken はリフレッシュトークンから新しいアクセストークンを発行する。
func (a *AS) grantRefreshToken(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("refresh_token")
	rt, ok := a.store.Refresh(token)
	if !ok {
		tokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	access, err := a.keys.newAccessToken(a.issuer, rsAudience, rt.Sub, rt.ClientID, rt.Scope, accessTokenTTL)
	if err != nil {
		tokenError(w, http.StatusInternalServerError, "server_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(accessTokenTTL.Seconds()),
		"scope":         rt.Scope,
		"refresh_token": rt.Token,
	})
}

// grantClientCredentials は M2M(サーバ間)向け。confidential クライアントを認証し
// アクセストークンのみ発行する(ユーザ本人が介在しないため id_token は無い)。
func (a *AS) grantClientCredentials(w http.ResponseWriter, r *http.Request) {
	clientID, secret := clientCredentials(r)
	client, ok := a.store.Client(clientID)
	if !ok || !client.Confidential || client.Secret != secret {
		tokenError(w, http.StatusUnauthorized, "invalid_client")
		return
	}
	scope := r.FormValue("scope")
	// sub にはクライアント自身を入れる(本人ユーザは存在しない)。
	access, err := a.keys.newAccessToken(a.issuer, rsAudience, clientID, clientID, scope, accessTokenTTL)
	if err != nil {
		tokenError(w, http.StatusInternalServerError, "server_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access,
		"token_type":   "Bearer",
		"expires_in":   int(accessTokenTTL.Seconds()),
		"scope":        scope,
	})
}

// clientCredentials は client_id/client_secret を Basic 認証またはフォームから取り出す。
func clientCredentials(r *http.Request) (string, string) {
	if id, secret, ok := r.BasicAuth(); ok {
		return id, secret
	}
	return r.FormValue("client_id"), r.FormValue("client_secret")
}

// handleDiscovery は OIDC Discovery ドキュメント(.well-known)を返す。
func (a *AS) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                a.issuer,
		"authorization_endpoint":                a.issuer + "/authorize",
		"token_endpoint":                        a.issuer + "/token",
		"jwks_uri":                              a.issuer + "/jwks.json",
		"userinfo_endpoint":                     a.issuer + "/userinfo",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token", "client_credentials"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      []string{"openid", "profile", "email"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post", "none"},
	})
}

// handleJWKS は署名検証用の公開鍵を JWKS で返す。
func (a *AS) handleJWKS(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.keys.PublicJWKS())
}

// handleUserInfo は Bearer アクセストークンを検証し、ユーザのクレームを返す(OIDC UserInfo)。
func (a *AS) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	tok := bearerToken(r)
	claims, err := a.keys.verifyAccessToken(tok, a.issuer, rsAudience)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		http.Error(w, "invalid_token", http.StatusUnauthorized)
		return
	}
	resp := map[string]any{"sub": claims.Subject}
	if u, ok := a.store.User(claims.Subject); ok {
		resp["name"] = u.Name
		resp["email"] = u.Email
	}
	writeJSON(w, http.StatusOK, resp)
}

// bearerToken は Authorization: Bearer ヘッダからトークンを取り出す。
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if after, found := strings.CutPrefix(h, "Bearer "); found {
		return after
	}
	return ""
}

// hasScope はスペース区切りの scope に target が含まれるか判定する。
func hasScope(scope, target string) bool {
	for _, s := range strings.Fields(scope) {
		if s == target {
			return true
		}
	}
	return false
}

// tokenError は OAuth のエラーレスポンス(JSON)を返す。
func tokenError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
