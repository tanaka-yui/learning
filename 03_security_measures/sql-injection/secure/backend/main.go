package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
)

// User はユーザー情報を表す構造体
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// initDB はSQLiteデータベースを初期化してusersテーブルを作成する
func initDB() *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatalf("データベースの接続に失敗: %v", err)
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		email TEXT,
		password TEXT
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("テーブルの作成に失敗: %v", err)
	}

	return db
}

// seedData はテスト用の初期データを投入する
func seedData(db *sql.DB) {
	users := []struct {
		name     string
		email    string
		password string
	}{
		{"Alice", "alice@example.com", "password123"},
		{"Bob", "bob@example.com", "password456"},
		{"Charlie", "charlie@example.com", "password789"},
	}

	for _, u := range users {
		_, err := db.Exec("INSERT INTO users (name, email, password) VALUES (?, ?, ?)", u.name, u.email, u.password)
		if err != nil {
			log.Fatalf("シードデータの投入に失敗: %v", err)
		}
	}
}

// setCORSHeaders はCORSヘッダーを設定する
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// setupRouter はHTTPハンドラーを設定して返却する
func setupRouter(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		name := r.URL.Query().Get("name")

		var rows *sql.Rows
		var err error

		if name != "" {
			// プリペアドステートメントを使用してSQLインジェクションを防止する
			// プレースホルダー(?)を使うことで、ユーザー入力がSQL文として解釈されなくなる
			rows, err = db.Query("SELECT id, name, email FROM users WHERE name = ?", name)
		} else {
			rows, err = db.Query("SELECT id, name, email FROM users")
		}

		if err != nil {
			http.Error(w, fmt.Sprintf("クエリの実行に失敗: %v", err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		users := make([]User, 0)
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
				http.Error(w, fmt.Sprintf("行の読み取りに失敗: %v", err), http.StatusInternalServerError)
				return
			}
			users = append(users, u)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(users); err != nil {
			http.Error(w, fmt.Sprintf("JSONのエンコードに失敗: %v", err), http.StatusInternalServerError)
			return
		}
	})

	return mux
}

func main() {
	db := initDB()
	seedData(db)

	handler := setupRouter(db)

	log.Println("サーバーをポート8080で起動します")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("サーバーの起動に失敗: %v", err)
	}
}
