package main

import "net/http"

// RS は Resource Server(保護リソース)の依存をまとめる。
// 単一オリジン構成のため、検証鍵は AS と同じ公開鍵(keys)を直接使う。
// 実運用では jwks_uri から公開鍵を取得してキャッシュする。
type RS struct {
	issuer string
	keys   *KeyMaterial
}

// handleMe は保護 API。Bearer アクセストークンを検証し、クレームを返す。
// トークンが無効/不在なら 401 を返す。
func (rs *RS) handleMe(w http.ResponseWriter, r *http.Request) {
	tok := bearerToken(r)
	if tok == "" {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "missing bearer token", http.StatusUnauthorized)
		return
	}
	// 署名(RS256・公開鍵)・iss・aud(=Resource Server)・exp を検証する。
	claims, err := rs.keys.verifyAccessToken(tok, rs.issuer, rsAudience)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		http.Error(w, "invalid_token", http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sub":       claims.Subject,
		"client_id": claims.ClientID,
		"scope":     claims.Scope,
		"aud":       claims.Audience,
		"iss":       claims.Issuer,
	})
}
