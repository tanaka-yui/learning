package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// User はユーザー情報を表す構造体
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

// Session はセッション情報を保持する構造体
type Session struct {
	Username string
}

// SessionStore はインメモリのセッションストア
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionStore はセッションストアを初期化する
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
	}
}

// Get はセッションIDからセッションを取得する
func (s *SessionStore) Get(sessionID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	return session, ok
}

// Set はセッションを保存する
func (s *SessionStore) Set(sessionID string, session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = session
}

// Delete はセッションを削除する
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

// generateSessionID はランダムなセッションIDを生成する
func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("セッションIDの生成に失敗: %v", err)
	}
	return hex.EncodeToString(b)
}

// RateLimiter はIPアドレスごとのログイン試行回数を追跡する
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

// NewRateLimiter はレートリミッターを初期化する
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string][]time.Time),
	}
}

// maxAttempts は1分間に許可される最大試行回数
const maxAttempts = 5

// windowDuration はレート制限のウィンドウ期間
const windowDuration = 1 * time.Minute

// IsAllowed は指定されたIPアドレスのリクエストが許可されるか判定する
func (rl *RateLimiter) IsAllowed(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-windowDuration)

	// 期限切れの試行記録を削除する
	valid := make([]time.Time, 0)
	for _, t := range rl.attempts[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.attempts[ip] = valid

	return len(valid) < maxAttempts
}

// RecordAttempt は失敗した試行を記録する
func (rl *RateLimiter) RecordAttempt(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.attempts[ip] = append(rl.attempts[ip], time.Now())
}

// Reset は指定されたIPアドレスの試行記録をリセットする
func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}

// hashPassword はbcryptでパスワードをハッシュ化する
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// checkPassword はbcryptハッシュとパスワードを比較する
func checkPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// initDB はSQLiteデータベースを初期化してusersテーブルを作成する
func initDB() *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatalf("データベースの接続に失敗: %v", err)
	}

	// パスワードはbcryptハッシュとして保存する
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE,
		password_hash TEXT
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("テーブルの作成に失敗: %v", err)
	}

	return db
}

// seedData はテスト用の初期データを投入する（パスワードはbcryptでハッシュ化して保存）
func seedData(db *sql.DB) {
	users := []struct {
		username string
		password string
	}{
		{"admin", "password123"},
		{"user1", "pass456"},
	}

	for _, u := range users {
		hash, err := hashPassword(u.password)
		if err != nil {
			log.Fatalf("パスワードのハッシュ化に失敗: %v", err)
		}
		_, err = db.Exec("INSERT OR IGNORE INTO users (username, password_hash) VALUES (?, ?)", u.username, hash)
		if err != nil {
			log.Fatalf("シードデータの投入に失敗: %v", err)
		}
	}
}

// setCORSHeaders はCORSヘッダーを設定する
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Cookie")
}

// setupRouter はHTTPハンドラーを設定して返却する
func setupRouter(db *sql.DB, store *SessionStore, limiter *RateLimiter) http.Handler {
	mux := http.NewServeMux()

	// POST /login — セキュアなログイン処理（bcrypt比較、レート制限、セッション再生成）
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "メソッドが許可されていません", http.StatusMethodNotAllowed)
			return
		}

		// IPアドレスを取得してレート制限を確認する（ポート番号を除去）
		ip, _, splitErr := net.SplitHostPort(r.RemoteAddr)
		if splitErr != nil {
			ip = r.RemoteAddr
		}
		if !limiter.IsAllowed(ip) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "試行回数の上限に達しました。しばらく待ってから再試行してください", http.StatusTooManyRequests)
			return
		}

		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "リクエストの解析に失敗", http.StatusBadRequest)
			return
		}

		// bcryptハッシュとの比較
		var storedHash string
		err := db.QueryRow("SELECT password_hash FROM users WHERE username = ?", req.Username).Scan(&storedHash)
		if err != nil || !checkPassword(storedHash, req.Password) {
			// 失敗した試行を記録する
			limiter.RecordAttempt(ip)
			http.Error(w, "認証に失敗", http.StatusUnauthorized)
			return
		}

		// ログイン成功時にレート制限カウンターをリセットする
		limiter.Reset(ip)

		// セッション再生成: 既存のセッションを削除して新しいIDを発行する
		if oldCookie, err := r.Cookie("session_id"); err == nil && oldCookie.Value != "" {
			store.Delete(oldCookie.Value)
		}
		sessionID := generateSessionID()
		store.Set(sessionID, &Session{Username: req.Username})

		// セキュアなCookie設定: HttpOnly, SameSite=Strict
		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message":  "ログイン成功",
			"username": req.Username,
		})
	})

	// GET /admin — 認証が必要な保護されたエンドポイント
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		cookie, err := r.Cookie("session_id")
		if err != nil || cookie.Value == "" {
			http.Error(w, "認証が必要です", http.StatusUnauthorized)
			return
		}

		session, ok := store.Get(cookie.Value)
		if !ok {
			http.Error(w, "無効なセッションです", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message":  "管理者ページへようこそ",
			"username": session.Username,
		})
	})

	// GET /me — 現在のユーザー情報を返す（パスワードハッシュは返さない）
	mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		cookie, err := r.Cookie("session_id")
		if err != nil || cookie.Value == "" {
			http.Error(w, "認証が必要です", http.StatusUnauthorized)
			return
		}

		session, ok := store.Get(cookie.Value)
		if !ok {
			http.Error(w, "無効なセッションです", http.StatusUnauthorized)
			return
		}

		var user User
		err = db.QueryRow("SELECT id, username FROM users WHERE username = ?", session.Username).Scan(&user.ID, &user.Username)
		if err != nil {
			http.Error(w, "ユーザー情報の取得に失敗", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	})

	// POST /logout — セッションを破棄する
	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
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
		if err == nil && cookie.Value != "" {
			store.Delete(cookie.Value)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "ログアウト成功",
		})
	})

	// GET /users — ユーザー一覧を返す（パスワードハッシュは返さない）
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		rows, err := db.Query("SELECT id, username FROM users")
		if err != nil {
			http.Error(w, "ユーザー一覧の取得に失敗", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		users := make([]User, 0)
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Username); err != nil {
				http.Error(w, "行の読み取りに失敗", http.StatusInternalServerError)
				return
			}
			users = append(users, u)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})

	return mux
}

func main() {
	db := initDB()
	seedData(db)
	store := NewSessionStore()
	limiter := NewRateLimiter()

	handler := setupRouter(db, store, limiter)

	log.Println("セキュアサーバーをポート8080で起動します")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("サーバーの起動に失敗: %v", err)
	}
}
