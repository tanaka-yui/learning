package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// テスト用のサーバーを作成する
func setupTestServer() http.Handler {
	return setupRoutes()
}

// 正常系: readme.txt をダウンロードできることを確認する
func TestDownloadNormalFile(t *testing.T) {
	handler := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/download?file=readme.txt", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("期待するステータスコード %d に対して %d が返された", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "公開ファイル") {
		t.Errorf("期待するファイル内容が含まれていない: %s", body)
	}
}

// 脆弱性の証明: ディレクトリトラバーサルでソースコードが読み取れることを確認する
func TestDownloadDirectoryTraversal(t *testing.T) {
	handler := setupTestServer()

	// ../main.go を指定して、filesディレクトリの外にあるソースコードを読み取る
	req := httptest.NewRequest(http.MethodGet, "/download?file=../main.go", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("期待するステータスコード %d に対して %d が返された（脆弱性によりアクセス可能であるべき）", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	// ソースコードに含まれるはずのキーワードを確認する
	if !strings.Contains(body, "package main") {
		t.Errorf("ディレクトリトラバーサルでソースコードが読み取れなかった: %s", body)
	}
}

// ファイル一覧: /files エンドポイントがファイルリストを返すことを確認する
func TestFileList(t *testing.T) {
	handler := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/files", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("期待するステータスコード %d に対して %d が返された", http.StatusOK, rec.Code)
	}

	var files []string
	if err := json.NewDecoder(rec.Body).Decode(&files); err != nil {
		t.Fatalf("レスポンスのJSONデコードに失敗した: %v", err)
	}

	// readme.txt と report.csv が含まれていることを確認する
	found := map[string]bool{"readme.txt": false, "report.csv": false}
	for _, f := range files {
		if _, ok := found[f]; ok {
			found[f] = true
		}
	}
	for name, ok := range found {
		if !ok {
			t.Errorf("ファイル一覧に %s が含まれていない", name)
		}
	}
}

// fileパラメータが未指定の場合、400エラーが返ることを確認する
func TestDownloadMissingFileParam(t *testing.T) {
	handler := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/download", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("期待するステータスコード %d に対して %d が返された", http.StatusBadRequest, rec.Code)
	}
}

// 存在しないファイルを指定した場合、404エラーが返ることを確認する
func TestDownloadNonexistentFile(t *testing.T) {
	handler := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/download?file=nonexistent.txt", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("期待するステータスコード %d に対して %d が返された", http.StatusNotFound, rec.Code)
	}
}
