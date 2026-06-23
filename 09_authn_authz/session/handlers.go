package main

import (
	"encoding/json"
	"net/http"
)

const sessionCookieName = "session_id"

// indexHTML は動作確認用の最小HTML
const indexHTML = `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>Session認証デモ</title></head>
<body>
<h1>Session認証デモ</h1>
<p>POST /login, GET /profile, POST /logout を確認してください。</p>
<p>テストユーザ: alice / password123, bob / pass456</p>
</body></html>`

// loginRequest はログインの入力
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// setupRouter は依存を受け取り http.Handler を構築する
func setupRouter(store *SessionStore, users *UserStore) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(indexHTML))
	})

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
		sess := store.Create(req.Username)
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sess.ID,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "username": req.Username})
	})

	mux.HandleFunc("POST /logout", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookieName); err == nil {
			store.Delete(c.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1})
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	})

	mux.Handle("GET /profile", requireSession(store, func(w http.ResponseWriter, r *http.Request, sess *Session) {
		writeJSON(w, http.StatusOK, map[string]string{"username": sess.Username})
	}))

	return mux
}

// requireSession は有効なセッションを要求するミドルウェア
func requireSession(store *SessionStore, next func(http.ResponseWriter, *http.Request, *Session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Error(w, "未認証です", http.StatusUnauthorized)
			return
		}
		sess, ok := store.Get(c.Value)
		if !ok {
			http.Error(w, "未認証です", http.StatusUnauthorized)
			return
		}
		next(w, r, sess)
	}
}

// writeJSON はJSONレスポンスを書き出す
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
