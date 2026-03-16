package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
		if !entry.IsDir() {
			fileNames = append(fileNames, entry.Name())
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileNames)
}

// ファイルダウンロードハンドラー: 指定されたファイルを安全にダウンロードする
// 【対策】filepath.Base、文字チェック、filepath.Absによるパス検証を実施する
func handleDownload(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "fileパラメータが必要です", http.StatusBadRequest)
		return
	}

	// 対策1: ".." や "/" や "\" を含むファイル名を拒否する
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.Error(w, "不正なファイル名です", http.StatusBadRequest)
		return
	}

	// 対策2: filepath.Base() でファイル名のみを抽出する（ディレクトリ要素を除去）
	safeName := filepath.Base(filename)
	if safeName == "." || safeName == ".." {
		http.Error(w, "不正なファイル名です", http.StatusBadRequest)
		return
	}

	// 対策3: 絶対パスを解決し、ベースディレクトリ内であることを検証する
	baseDir, err := filepath.Abs(filesDir)
	if err != nil {
		http.Error(w, "サーバー内部エラー", http.StatusInternalServerError)
		return
	}

	targetPath, err := filepath.Abs(filepath.Join(filesDir, safeName))
	if err != nil {
		http.Error(w, "サーバー内部エラー", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(targetPath, baseDir+string(os.PathSeparator)) {
		http.Error(w, "不正なファイルパスです", http.StatusBadRequest)
		return
	}

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
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", safeName))
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
