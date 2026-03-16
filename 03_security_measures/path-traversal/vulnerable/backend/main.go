package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

const (
	filesDir    = "files"
	defaultPort = "8080"
)

// CORSミドルウェア: すべてのオリジンからのアクセスを許可する
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ファイル一覧ハンドラー: filesディレクトリ内のファイルを一覧表示する（非再帰）
func handleFileList(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(filesDir)
	if err != nil {
		http.Error(w, "ファイル一覧の取得に失敗しました", http.StatusInternalServerError)
		return
	}

	var fileNames []string
	for _, entry := range entries {
		// ディレクトリは除外し、ファイルのみを返す
		if !entry.IsDir() {
			fileNames = append(fileNames, entry.Name())
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileNames)
}

// ファイルダウンロードハンドラー: 指定されたファイルをダウンロードする
// 【脆弱性】パスの検証を行わないため、ディレクトリトラバーサル攻撃が可能
func handleDownload(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "fileパラメータが必要です", http.StatusBadRequest)
		return
	}

	// 【脆弱性】ユーザー入力をそのまま結合しており、パスの検証を行っていない
	// 攻撃者は "../" を使ってfilesディレクトリの外にあるファイルを読み取ることができる
	targetPath := filepath.Join(filesDir, filename)

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "ファイルが見つかりません", http.StatusNotFound)
			return
		}
		http.Error(w, "ファイルの読み取りに失敗しました", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filename)))
	w.Write(data)
}

// ルーティングを設定する
func setupRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", handleFileList)
	mux.HandleFunc("/download", handleDownload)
	return corsMiddleware(mux)
}

func main() {
	handler := setupRoutes()

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	fmt.Printf("サーバーをポート %s で起動します\n", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		fmt.Printf("サーバーの起動に失敗しました: %v\n", err)
		os.Exit(1)
	}
}
