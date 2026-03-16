package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
)

// リクエストボディの構造体
type commandRequest struct {
	Host string `json:"host"`
}

// レスポンスボディの構造体
type commandResponse struct {
	Output string `json:"output"`
}

// エラーレスポンスの構造体
type errorResponse struct {
	Error string `json:"error"`
}

// CORSヘッダーを設定するミドルウェア
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// OPTIONSプリフライトリクエストへの応答
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// リクエストボディからホスト名を取得する共通処理
func parseHost(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req commandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "不正なJSONです"})
		return "", false
	}

	if req.Host == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "hostフィールドは必須です"})
		return "", false
	}

	return req.Host, true
}

// nslookupハンドラー
// 【意図的な脆弱性】ユーザー入力をそのままシェルに渡している
func lookupHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	host, ok := parseHost(w, r)
	if !ok {
		return
	}

	// 脆弱なコマンド実行: ユーザー入力をシェル経由で直接実行
	cmd := exec.Command("sh", "-c", "nslookup "+host)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// コマンドがエラーでも出力があればそれを返す（インジェクション結果を見せるため）
		if len(output) > 0 {
			json.NewEncoder(w).Encode(commandResponse{Output: string(output)})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse{Error: fmt.Sprintf("コマンド実行エラー: %v", err)})
		return
	}

	json.NewEncoder(w).Encode(commandResponse{Output: string(output)})
}

// pingハンドラー
// 【意図的な脆弱性】ユーザー入力をそのままシェルに渡している
func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	host, ok := parseHost(w, r)
	if !ok {
		return
	}

	// 脆弱なコマンド実行: ユーザー入力をシェル経由で直接実行
	cmd := exec.Command("sh", "-c", "ping -c 1 "+host)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			json.NewEncoder(w).Encode(commandResponse{Output: string(output)})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse{Error: fmt.Sprintf("コマンド実行エラー: %v", err)})
		return
	}

	json.NewEncoder(w).Encode(commandResponse{Output: string(output)})
}

// ルーターの設定
func setupRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/lookup", lookupHandler)
	mux.HandleFunc("/ping", pingHandler)
	return corsMiddleware(mux)
}

func main() {
	handler := setupRouter()
	fmt.Println("コマンドインジェクション脆弱版サーバーをポート8080で起動します")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		fmt.Printf("サーバー起動エラー: %v\n", err)
	}
}
