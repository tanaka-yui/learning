package main

import (
	"encoding/json"
	"net/http"
)

// indexHTML は動作確認用の入口ページ。
const indexHTML = `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>OAuth2.0 / OIDC デモ</title></head>
<body>
<h1>OAuth2.0 / OpenID Connect デモ</h1>
<p>1 つのバイナリ・1 つのオリジンに、認可サーバ(AS)・クライアント(RP)・リソースサーバ(RS)を同居させています。</p>
<ul>
  <li><a href="/app/login">/app/login</a> — Authorization Code + PKCE でログイン</li>
  <li><a href="/.well-known/openid-configuration">/.well-known/openid-configuration</a> — OIDC Discovery</li>
  <li><a href="/jwks.json">/jwks.json</a> — 署名検証用の公開鍵(JWKS)</li>
</ul>
</body></html>`

// setupRouter は AS / RP / RS を 1 つの origin にパスでマウントした http.Handler を返す。
// issuer は外部から見える基底 URL(= origin)。リンク・リダイレクト・iss・jwks_uri に使う。
func setupRouter(issuer string, keys *KeyMaterial, store *Store, client *http.Client) http.Handler {
	as := &AS{issuer: issuer, keys: keys, store: store}
	rp := &RP{
		issuer:   issuer,
		clientID: publicClientID,
		keys:     keys,
		sessions: NewRPSessionStore(),
		pending:  NewPendingStore(),
		http:     client,
	}
	rs := &RS{issuer: issuer, keys: keys}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(indexHTML))
	})

	// --- Authorization Server (AS) ---
	mux.HandleFunc("GET /authorize", as.handleAuthorizeGET)
	mux.HandleFunc("POST /authorize", as.handleAuthorizePOST)
	mux.HandleFunc("POST /token", as.handleToken)
	mux.HandleFunc("GET /.well-known/openid-configuration", as.handleDiscovery)
	mux.HandleFunc("GET /jwks.json", as.handleJWKS)
	mux.HandleFunc("GET /userinfo", as.handleUserInfo)

	// --- Relying Party / client (RP) ---
	mux.HandleFunc("GET /app/login", rp.handleLogin)
	mux.HandleFunc("GET /app/callback", rp.handleCallback)
	mux.HandleFunc("GET /app/", rp.handleApp)

	// --- Resource Server (RS) ---
	mux.HandleFunc("GET /api/me", rs.handleMe)

	return mux
}

// writeJSON はJSONレスポンスを書き出す。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
