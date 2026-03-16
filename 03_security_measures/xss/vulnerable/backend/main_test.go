package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// XSSを含む投稿が正常に保存されることを検証する
func TestPostWithXSSContent(t *testing.T) {
	// 初期投稿をリセットするためにサーバーを新規作成
	mux := setupRouter()

	xssPayload := `<script>alert('XSS')</script>`
	body := map[string]string{"content": xssPayload}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期待するステータスコード: %d, 実際: %d", http.StatusOK, w.Code)
	}
}

// 投稿一覧取得時にXSSペイロードがサニタイズされずにそのまま返されることを検証する（脆弱性の証明）
func TestGetPostsReturnsUnsanitizedContent(t *testing.T) {
	mux := setupRouter()

	xssPayload := `<script>alert('XSS')</script>`
	body := map[string]string{"content": xssPayload}
	jsonBody, _ := json.Marshal(body)

	// まずXSSペイロードを投稿
	postReq := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(jsonBody))
	postReq.Header.Set("Content-Type", "application/json")
	postW := httptest.NewRecorder()
	mux.ServeHTTP(postW, postReq)

	// 投稿一覧を取得
	getReq := httptest.NewRequest(http.MethodGet, "/posts", nil)
	getW := httptest.NewRecorder()
	mux.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("期待するステータスコード: %d, 実際: %d", http.StatusOK, getW.Code)
	}

	// レスポンスにサニタイズされていないXSSペイロードが含まれていることを確認
	responseBody := getW.Body.String()
	if !bytes.Contains([]byte(responseBody), []byte(xssPayload)) {
		t.Errorf("レスポンスにサニタイズされていないXSSペイロードが含まれていません: %s", responseBody)
	}
}

// 空のコンテンツで投稿した場合に400エラーが返されることを検証する
func TestPostWithEmptyContentReturns400(t *testing.T) {
	mux := setupRouter()

	body := map[string]string{"content": ""}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期待するステータスコード: %d, 実際: %d", http.StatusBadRequest, w.Code)
	}
}
