package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
)

// ホスト名のバリデーション用正規表現
var validHostPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

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

// ホスト名のバリデーション
func validateHost(host string) bool {
	return validHostPattern.MatchString(host)
}

// リクエストボディからホスト名を取得し、バリデーションする共通処理
func parseAndValidateHost(w http.ResponseWriter, r *http.Request) (string, bool) {
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

	// 入力バリデーション: 許可された文字のみを受け入れる
	if !validateHost(req.Host) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "不正なホスト名です。英数字、ドット、ハイフン、アンダースコアのみ使用できます"})
		return "", false
	}

	return req.Host, true
}

// nslookupハンドラー
// 【対策済み】引数を分離してコマンドを実行し、シェルを経由しない
func lookupHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	host, ok := parseAndValidateHost(w, r)
	if !ok {
		return
	}

	// 安全なコマンド実行: シェルを経由せず、引数を分離して渡す
	cmd := exec.Command("nslookup", host)
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

// pingハンドラー
// 【対策済み】引数を分離してコマンドを実行し、シェルを経由しない
func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	host, ok := parseAndValidateHost(w, r)
	if !ok {
		return
	}

	// 安全なコマンド実行: シェルを経由せず、引数を分離して渡す
	cmd := exec.Command("ping", "-c", "1", host)
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
	fmt.Println("コマンドインジェクション対策版サーバーをポート8080で起動します")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		fmt.Printf("サーバー起動エラー: %v\n", err)
	}
}
