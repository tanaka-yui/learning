package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// RP は Relying Party(OIDC クライアント)の依存をまとめる。
// 単一オリジン構成のため、ID Token 検証は AS と同じ公開鍵(keys)を直接使う。
type RP struct {
	issuer   string // = authorization server の origin
	clientID string
	keys     *KeyMaterial
	sessions *RPSessionStore
	pending  *PendingStore
	http     *http.Client
}

// RPSession は RP がログイン後に保持するユーザ情報。
type RPSession struct {
	Sub         string
	Name        string
	Email       string
	AccessToken string
}

// RPSessionStore は RP のログインセッション(インメモリ)。
type RPSessionStore struct {
	mu sync.Mutex
	m  map[string]*RPSession
}

func NewRPSessionStore() *RPSessionStore { return &RPSessionStore{m: make(map[string]*RPSession)} }

func (s *RPSessionStore) Set(id string, sess *RPSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[id] = sess
}

func (s *RPSessionStore) Get(id string) (*RPSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.m[id]
	return sess, ok
}

// pendingAuth は /authorize へリダイレクトする前に保持する state/nonce/verifier。
type pendingAuth struct {
	State    string
	Nonce    string
	Verifier string
}

// PendingStore は認可リクエスト進行中の state ごとの状態を保持する(CSRF/PKCE 用)。
type PendingStore struct {
	mu sync.Mutex
	m  map[string]*pendingAuth // state -> pending
}

func NewPendingStore() *PendingStore { return &PendingStore{m: make(map[string]*pendingAuth)} }

func (p *PendingStore) Save(pa *pendingAuth) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[pa.State] = pa
}

func (p *PendingStore) Take(state string) (*pendingAuth, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pa, ok := p.m[state]
	delete(p.m, state) // state は単回使用
	return pa, ok
}

const rpCookieName = "rp_session"

// appTmpl は /app/ のログイン後ページ。
var appTmpl = template.Must(template.New("app").Parse(`<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>RP: ログイン済み</title></head>
<body>
<h1>Relying Party (クライアント)</h1>
<p>ログイン済みユーザ: <b>{{.Name}}</b> ({{.Email}})</p>
<p>sub: <code>{{.Sub}}</code></p>
<h2>/api/me (Resource Server) の応答</h2>
<pre>{{.APIMe}}</pre>
</body></html>`))

// randVerifier は PKCE の code_verifier(43〜128 文字)を生成する。
func randVerifier() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// handleLogin は state/nonce/PKCE を生成し、認可エンドポイントへリダイレクトする。
func (rp *RP) handleLogin(w http.ResponseWriter, r *http.Request) {
	pa := &pendingAuth{
		State:    randToken(),
		Nonce:    randToken(),
		Verifier: randVerifier(),
	}
	rp.pending.Save(pa)

	q := url.Values{}
	q.Set("client_id", rp.clientID)
	q.Set("redirect_uri", rp.issuer+"/app/callback")
	q.Set("response_type", "code")
	q.Set("scope", "openid profile email")
	q.Set("state", pa.State)
	q.Set("nonce", pa.Nonce)
	q.Set("code_challenge", pkceS256Challenge(pa.Verifier))
	q.Set("code_challenge_method", "S256")

	http.Redirect(w, r, rp.issuer+"/authorize?"+q.Encode(), http.StatusFound)
}

// handleCallback は認可コードを受け取り、state 検証 → コード交換 → ID Token 検証を行う。
func (rp *RP) handleCallback(w http.ResponseWriter, r *http.Request) {
	if errCode := r.URL.Query().Get("error"); errCode != "" {
		http.Error(w, "認可エラー: "+errCode, http.StatusBadRequest)
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	pa, ok := rp.pending.Take(state)
	if !ok {
		// 保存していない state = CSRF の疑い。
		http.Error(w, "state が一致しません(CSRF の疑い)", http.StatusBadRequest)
		return
	}

	// トークンエンドポイントへコードと code_verifier を送って交換する。
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", rp.issuer+"/app/callback")
	form.Set("client_id", rp.clientID)
	form.Set("code_verifier", pa.Verifier)

	tokenResp, err := rp.postForm(rp.issuer+"/token", form)
	if err != nil {
		http.Error(w, "トークン交換失敗: "+err.Error(), http.StatusBadGateway)
		return
	}

	idToken, _ := tokenResp["id_token"].(string)
	accessToken, _ := tokenResp["access_token"].(string)
	if idToken == "" {
		http.Error(w, "id_token がありません", http.StatusBadGateway)
		return
	}

	// ID Token を検証する(署名・iss・aud=client・exp・nonce)。
	claims, err := rp.keys.verifyIDToken(idToken, rp.issuer, rp.clientID, pa.Nonce)
	if err != nil {
		http.Error(w, "id_token 検証失敗: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// 検証済みクレームでセッションを確立し、Cookie を発行する。
	sid := randToken()
	rp.sessions.Set(sid, &RPSession{
		Sub:         claims.Subject,
		Name:        claims.Name,
		Email:       claims.Email,
		AccessToken: accessToken,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     rpCookieName,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/app/", http.StatusFound)
}

// handleApp はログイン済みユーザを表示し、保持中のアクセストークンで /api/me を呼ぶ。
func (rp *RP) handleApp(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(rpCookieName)
	if err != nil {
		http.Redirect(w, r, "/app/login", http.StatusFound)
		return
	}
	sess, ok := rp.sessions.Get(c.Value)
	if !ok {
		http.Redirect(w, r, "/app/login", http.StatusFound)
		return
	}

	apiMe := rp.callAPIMe(sess.AccessToken)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	appTmpl.Execute(w, map[string]string{
		"Name":  sess.Name,
		"Email": sess.Email,
		"Sub":   sess.Sub,
		"APIMe": apiMe,
	})
}

// callAPIMe は Resource Server の /api/me を Bearer トークンで呼び、本文を文字列で返す。
func (rp *RP) callAPIMe(accessToken string) string {
	req, _ := http.NewRequest(http.MethodGet, rp.issuer+"/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := rp.http.Do(req)
	if err != nil {
		return "呼び出し失敗: " + err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return fmt.Sprintf("HTTP %d\n%s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// postForm はフォームを POST し、JSON レスポンスを map で返す。
func (rp *RP) postForm(endpoint string, form url.Values) (map[string]any, error) {
	resp, err := rp.http.PostForm(endpoint, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		if e, ok := out["error"].(string); ok {
			return nil, fmt.Errorf("token endpoint error: %s", e)
		}
		return nil, fmt.Errorf("token endpoint status %d", resp.StatusCode)
	}
	return out, nil
}
