package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// User はユーザー情報を表す構造体
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
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

// initDB はSQLiteデータベースを初期化してusersテーブルを作成する
func initDB() *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatalf("データベースの接続に失敗: %v", err)
	}

	// パスワードを平文で保存する（意図的に脆弱な設計）
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE,
		password TEXT
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("テーブルの作成に失敗: %v", err)
	}

	return db
}

// seedData はテスト用の初期データを投入する（パスワードは平文で保存 — 意図的に脆弱）
func seedData(db *sql.DB) {
	users := []struct {
		username string
		password string
	}{
		{"admin", "password123"},
		{"user1", "pass456"},
	}

	for _, u := range users {
		_, err := db.Exec("INSERT OR IGNORE INTO users (username, password) VALUES (?, ?)", u.username, u.password)
		if err != nil {
			log.Fatalf("シードデータの投入に失敗: %v", err)
		}
	}
}

// setCORSHeaders はCORSヘッダーを設定する（意図的に緩い設定）
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Cookie")
}

// setupRouter はHTTPハンドラーを設定して返却する
func setupRouter(db *sql.DB, store *SessionStore) http.Handler {
	mux := http.NewServeMux()

	// POST /login — ログイン処理（レート制限なし、セッション固定攻撃に脆弱）
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

		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "リクエストの解析に失敗", http.StatusBadRequest)
			return
		}

		// 平文パスワードとの比較（意図的に脆弱）
		var storedPassword string
		err := db.QueryRow("SELECT password FROM users WHERE username = ?", req.Username).Scan(&storedPassword)
		if err != nil || storedPassword != req.Password {
			// レート制限なし — ブルートフォース攻撃に脆弱
			http.Error(w, "認証に失敗", http.StatusUnauthorized)
			return
		}

		// セッション固定攻撃に脆弱: 既存のセッションIDがあればそのまま再利用する
		sessionID := ""
		cookie, err := r.Cookie("session_id")
		if err == nil && cookie.Value != "" {
			sessionID = cookie.Value
		} else {
			sessionID = generateSessionID()
		}

		store.Set(sessionID, &Session{Username: req.Username})

		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
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

	// GET /me — 現在のユーザー情報を返す
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
		err = db.QueryRow("SELECT id, username, password FROM users WHERE username = ?", session.Username).Scan(&user.ID, &user.Username, &user.Password)
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
			Name:   "session_id",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "ログアウト成功",
		})
	})

	// GET /users — ユーザー一覧を返す（デモ用: パスワードが平文で露出する）
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		rows, err := db.Query("SELECT id, username, password FROM users")
		if err != nil {
			http.Error(w, "ユーザー一覧の取得に失敗", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		users := make([]User, 0)
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Username, &u.Password); err != nil {
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

	handler := setupRouter(db, store)

	log.Println("サーバーをポート8080で起動します")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("サーバーの起動に失敗: %v", err)
	}
}
