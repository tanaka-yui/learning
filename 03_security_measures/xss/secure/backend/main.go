package main

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"sync"
	"time"
)

// 掲示板の投稿を表すデータ構造
type Post struct {
	ID        int    `json:"id"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

// 投稿作成リクエストのボディ
type CreatePostRequest struct {
	Content string `json:"content"`
}

// スレッドセーフな投稿ストレージ
type PostStore struct {
	mu     sync.Mutex
	posts  []Post
	nextID int
}

// 新しいPostStoreを初期化し、デモ用の初期データを投入する
func newPostStore() *PostStore {
	store := &PostStore{
		posts:  make([]Post, 0),
		nextID: 1,
	}

	// デモ用の初期投稿
	seedPosts := []string{
		"こんにちは、掲示板へようこそ！",
		"今日はいい天気ですね。",
		"この掲示板はXSS対策のデモ用です。",
	}

	for _, content := range seedPosts {
		store.addPost(content)
	}

	return store
}

// 投稿を追加する（html.EscapeStringでサニタイズ済み）
func (s *PostStore) addPost(content string) Post {
	s.mu.Lock()
	defer s.mu.Unlock()

	post := Post{
		ID:        s.nextID,
		Content:   html.EscapeString(content), // 保存前にHTMLエスケープを適用する
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	s.nextID++
	s.posts = append(s.posts, post)
	return post
}

// 全投稿を取得する
func (s *PostStore) getAllPosts() []Post {
	s.mu.Lock()
	defer s.mu.Unlock()

	// コピーを返してデータ競合を防ぐ
	result := make([]Post, len(s.posts))
	copy(result, s.posts)
	return result
}

// CORSヘッダーを設定するミドルウェア
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// OPTIONSプリフライトリクエストの処理
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// セキュリティヘッダーを付与するミドルウェア
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// ルーターを構築する（テストでも使用）
func setupRouter() http.Handler {
	store := newPostStore()
	mux := http.NewServeMux()

	// 投稿一覧取得エンドポイント - サニタイズ済みのコンテンツを返す
	mux.HandleFunc("GET /posts", func(w http.ResponseWriter, r *http.Request) {
		posts := store.getAllPosts()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(posts)
	})

	// 投稿作成エンドポイント - 入力をサニタイズしてから保存する
	mux.HandleFunc("POST /posts", func(w http.ResponseWriter, r *http.Request) {
		var req CreatePostRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"不正なリクエストボディです"}`, http.StatusBadRequest)
			return
		}

		if req.Content == "" {
			http.Error(w, `{"error":"コンテンツは必須です"}`, http.StatusBadRequest)
			return
		}

		// html.EscapeStringでサニタイズしてから保存する
		post := store.addPost(req.Content)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(post)
	})

	// ミドルウェアチェーン: CORS -> セキュリティヘッダー -> ハンドラー
	return corsMiddleware(securityHeadersMiddleware(mux))
}

func main() {
	handler := setupRouter()

	port := 8080
	fmt.Printf("XSS対策デモサーバーをポート %d で起動します...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), handler))
}
