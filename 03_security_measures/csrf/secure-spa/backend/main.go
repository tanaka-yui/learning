package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
)

// ユーザー情報を保持する構造体
type user struct {
	Username string `json:"username"`
	Password string `json:"-"`
}

// パスワード変更リクエストのJSON構造体
type changePasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// インメモリのユーザーストア（初期ユーザー付き）
var (
	users = map[string]*user{
		"admin": {Username: "admin", Password: "password123"},
		"user1": {Username: "user1", Password: "pass456"},
	}
	usersMu sync.RWMutex
)

// インメモリのセッションストア（セッションID -> ユーザー名）
var (
	sessions  = map[string]string{}
	sessionMu sync.RWMutex
)

// 許可するオリジンを取得する（環境変数で設定可能）
func getAllowedOrigin() string {
	origin := os.Getenv("ALLOWED_ORIGIN")
	if origin == "" {
		return "http://localhost:3012"
	}
	return origin
}

// ランダムなトークンを生成する
func generateToken() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CORSヘッダーを設定する（安全な設定: 特定のオリジンのみ許可）
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", getAllowedOrigin())
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Custom-Origin")
}

// セッションCookieからユーザー名を取得する
func getUserFromSession(r *http.Request) (string, bool) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return "", false
	}
	sessionMu.RLock()
	defer sessionMu.RUnlock()
	username, ok := sessions[cookie.Value]
	return username, ok
}

// カスタムオリジンヘッダーを検証する（SPA向けCSRF対策）
func validateCustomOrigin(r *http.Request) bool {
	origin := r.Header.Get("X-Custom-Origin")
	if origin == "" {
		return false
	}
	return origin == getAllowedOrigin()
}

// ログインハンドラー: 認証情報を検証しセッションを作成する
func handleLogin(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "メソッドが許可されていません", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "不正なリクエストです", http.StatusBadRequest)
		return
	}

	usersMu.RLock()
	u, exists := users[creds.Username]
	usersMu.RUnlock()

	if !exists || u.Password != creds.Password {
		http.Error(w, "認証情報が無効です", http.StatusUnauthorized)
		return
	}

	sessionID, err := generateToken()
	if err != nil {
		http.Error(w, "サーバー内部エラー", http.StatusInternalServerError)
		return
	}

	sessionMu.Lock()
	sessions[sessionID] = creds.Username
	sessionMu.Unlock()

	// 安全なCookie設定: SameSite=Strict、HttpOnly=true
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   false, // localhost用デモのためfalse
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "ログイン成功"})
}

// パスワード変更ハンドラー: カスタムオリジン検証あり（SPA向けCSRF対策）
func handleChangePassword(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "メソッドが許可されていません", http.StatusMethodNotAllowed)
		return
	}

	username, ok := getUserFromSession(r)
	if !ok {
		http.Error(w, "未認証です", http.StatusUnauthorized)
		return
	}

	// カスタムオリジンヘッダーを検証する（SPA向けCSRF対策）
	if !validateCustomOrigin(r) {
		http.Error(w, "オリジンが無効です", http.StatusForbidden)
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "不正なリクエストです", http.StatusBadRequest)
		return
	}

	if req.NewPassword == "" {
		http.Error(w, "新しいパスワードは必須です", http.StatusBadRequest)
		return
	}

	usersMu.Lock()
	users[username].Password = req.NewPassword
	usersMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "パスワードを変更しました"})
}

// ユーザー情報取得ハンドラー: セッションCookieに基づく
func handleMe(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "メソッドが許可されていません", http.StatusMethodNotAllowed)
		return
	}

	username, ok := getUserFromSession(r)
	if !ok {
		http.Error(w, "未認証です", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"username": username})
}

// ログアウトハンドラー: セッションを破棄する
func handleLogout(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "メソッドが許可されていません", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session_id")
	if err != nil {
		http.Error(w, "未認証です", http.StatusUnauthorized)
		return
	}

	sessionMu.Lock()
	delete(sessions, cookie.Value)
	sessionMu.Unlock()

	// Cookieを無効化する（安全な設定を維持）
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   false,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "ログアウトしました"})
}

// ルーティングを設定する
func newServeMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", handleLogin)
	mux.HandleFunc("/change-password", handleChangePassword)
	mux.HandleFunc("/me", handleMe)
	mux.HandleFunc("/logout", handleLogout)
	return mux
}

func main() {
	mux := newServeMux()
	fmt.Println("CSRF対策版（SPA向け）デモサーバーをポート8080で起動します")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Printf("サーバー起動エラー: %v\n", err)
	}
}
