package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// indexHTML は動作確認用の最小HTML
const indexHTML = `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>JWT認証デモ</title></head>
<body>
<h1>JWT認証デモ</h1>
<ul>
  <li>POST /login — アクセストークン + リフレッシュトークン取得</li>
  <li>GET /protected — Bearerトークンで認証(アクセストークン)</li>
  <li>POST /refresh — リフレッシュトークンで新しいトークンペアを取得(ローテーション)</li>
  <li>POST /logout — リフレッシュトークンを失効させる</li>
</ul>
<p>テストユーザ: alice / password123, bob / pass456</p>
</body></html>`

// loginRequest はログインの入力
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// tokenResponse はトークン発行のレスポンス
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// setupRouter は依存を受け取り http.Handler を構築する
func setupRouter(signer Signer, blocklist *Blocklist, users *UserStore) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(indexHTML))
	})

	// POST /login — 認証情報を検証してアクセス+リフレッシュトークンを返す
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "不正なリクエスト", http.StatusBadRequest)
			return
		}
		if !users.Verify(req.Username, req.Password) {
			http.Error(w, "認証に失敗しました", http.StatusUnauthorized)
			return
		}
		access, refresh, err := issueTokenPair(signer, req.Username)
		if err != nil {
			http.Error(w, "トークン発行エラー", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, tokenResponse{AccessToken: access, RefreshToken: refresh})
	})

	// GET /protected — アクセストークンを検証してユーザ名を返す
	mux.Handle("GET /protected", requireAccessToken(signer, func(w http.ResponseWriter, r *http.Request, claims *tokenClaims) {
		writeJSON(w, http.StatusOK, map[string]string{"username": claims.Subject})
	}))

	// POST /refresh — リフレッシュトークンを検証し、新しいトークンペアを発行(ローテーション)
	mux.HandleFunc("POST /refresh", func(w http.ResponseWriter, r *http.Request) {
		tokenStr := bearerToken(r)
		if tokenStr == "" {
			// JSONボディからも受け付ける
			var body struct {
				RefreshToken string `json:"refresh_token"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RefreshToken == "" {
				http.Error(w, "リフレッシュトークンが必要です", http.StatusBadRequest)
				return
			}
			tokenStr = body.RefreshToken
		}

		claims, err := parseRefreshToken(signer, blocklist, tokenStr)
		if err != nil {
			http.Error(w, "リフレッシュトークンが無効です", http.StatusUnauthorized)
			return
		}

		// 古いリフレッシュトークンのJTIを失効リストに追加(再利用防止)
		blocklist.Revoke(claims.ID)

		access, refresh, err := issueTokenPair(signer, claims.Subject)
		if err != nil {
			http.Error(w, "トークン発行エラー", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, tokenResponse{AccessToken: access, RefreshToken: refresh})
	})

	// POST /logout — リフレッシュトークンのJTIを失効リストに追加
	mux.HandleFunc("POST /logout", func(w http.ResponseWriter, r *http.Request) {
		tokenStr := bearerToken(r)
		if tokenStr == "" {
			var body struct {
				RefreshToken string `json:"refresh_token"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.RefreshToken != "" {
				tokenStr = body.RefreshToken
			}
		}
		if tokenStr != "" {
			// エラーでも失敗を返さない(べき等性を保つ)
			if claims, err := parseRefreshToken(signer, blocklist, tokenStr); err == nil {
				blocklist.Revoke(claims.ID)
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	})

	return mux
}

// requireAccessToken はアクセストークンを要求するミドルウェア
func requireAccessToken(signer Signer, next func(http.ResponseWriter, *http.Request, *tokenClaims)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := bearerToken(r)
		if tokenStr == "" {
			http.Error(w, "未認証です", http.StatusUnauthorized)
			return
		}
		parsed, err := signer.Parse(tokenStr)
		if err != nil || !parsed.Valid {
			http.Error(w, "トークンが無効です", http.StatusUnauthorized)
			return
		}
		claims, ok := parsed.Claims.(*tokenClaims)
		if !ok || claims.TokenType != "access" {
			http.Error(w, "トークン種別が不正です", http.StatusUnauthorized)
			return
		}
		next(w, r, claims)
	}
}

// parseRefreshToken はリフレッシュトークンを検証してクレームを返す
func parseRefreshToken(signer Signer, blocklist *Blocklist, tokenStr string) (*tokenClaims, error) {
	parsed, err := signer.Parse(tokenStr)
	if err != nil || !parsed.Valid {
		return nil, errors.New("トークン検証失敗")
	}
	claims, ok := parsed.Claims.(*tokenClaims)
	if !ok || claims.TokenType != "refresh" {
		return nil, errors.New("トークン種別が不正です")
	}
	if blocklist.IsRevoked(claims.ID) {
		return nil, errors.New("トークンは失効済みです")
	}
	return claims, nil
}

// bearerToken はAuthorizationヘッダからBearerトークンを取り出す
func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

// writeJSON はJSONレスポンスを書き出す
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
