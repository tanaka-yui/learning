package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

// seededAPIKeys はシード済みAPIキー → クライアント名のマッピング(学習用)
var seededAPIKeys = map[string]string{
	"key-service-a-secret-1234": "service-a",
	"key-service-b-secret-5678": "service-b",
}

// indexHTML は動作確認用の最小HTML
const indexHTML = `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>M2M認証デモ</title></head>
<body>
<h1>M2M認証デモ</h1>
<p>API Key: GET /api/data に X-API-Key ヘッダまたは Authorization: Bearer &lt;key&gt; を付与してください。</p>
<p>mTLS: https://localhost:8443/mtls/data にクライアント証明書付きでアクセスしてください。</p>
</body></html>`

// lookupAPIKey はキーをSHA-256で固定長ダイジェスト化してから定数時間比較し、
// 長さ・内容のいずれもタイミングで漏らさない。一致するキーが存在しない場合は空文字と false を返す。
func lookupAPIKey(provided string) (string, bool) {
	providedDigest := sha256.Sum256([]byte(provided))
	found := false
	var clientName string
	for k, v := range seededAPIKeys {
		// 鍵をSHA-256で固定長ダイジェスト化してから定数時間比較し、長さ・内容のいずれもタイミングで漏らさない
		seedDigest := sha256.Sum256([]byte(k))
		if subtle.ConstantTimeCompare(seedDigest[:], providedDigest[:]) == 1 {
			found = true
			clientName = v
		}
	}
	return clientName, found
}

// extractAPIKey はリクエストから X-API-Key ヘッダまたは Authorization: Bearer を取り出す
func extractAPIKey(r *http.Request) string {
	if k := r.Header.Get("X-API-Key"); k != "" {
		return k
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// requireAPIKey はAPIキーを検証するミドルウェア
func requireAPIKey(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := extractAPIKey(r)
		client, ok := lookupAPIKey(key)
		if !ok {
			http.Error(w, "認証に失敗しました", http.StatusUnauthorized)
			return
		}
		next(w, r, client)
	}
}

// setupRouter は依存を受け取り http.Handler を構築する
func setupRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(indexHTML))
	})

	mux.Handle("GET /api/data", requireAPIKey(func(w http.ResponseWriter, r *http.Request, client string) {
		writeJSON(w, http.StatusOK, map[string]string{"client": client, "data": "protected resource"})
	}))

	return mux
}

// writeJSON はJSONレスポンスを書き出す
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
