package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// XSSペイロードを投稿して200が返ることを検証する
func TestPostXSSPayloadReturns200(t *testing.T) {
	handler := setupRouter()

	body := `{"content":"<script>alert('XSS')</script>"}`
	req := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコードが200であるべきところ、%d が返された", rec.Code)
	}
}

// 投稿取得時にscriptタグがエスケープされていることを検証する
func TestGetPostsEscapesScriptTag(t *testing.T) {
	handler := setupRouter()

	// XSSペイロードを投稿する
	body := `{"content":"<script>alert('XSS')</script>"}`
	postReq := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewBufferString(body))
	postReq.Header.Set("Content-Type", "application/json")
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)

	// 投稿一覧を取得する
	getReq := httptest.NewRequest(http.MethodGet, "/posts", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("ステータスコードが200であるべきところ、%d が返された", getRec.Code)
	}

	// レスポンスボディにエスケープ済みのタグが含まれていることを確認する
	responseBody := getRec.Body.String()

	// 生のscriptタグが含まれていないことを検証する
	if strings.Contains(responseBody, "<script>") {
		t.Error("レスポンスに生の<script>タグが含まれている。サニタイズが機能していない")
	}

	// エスケープ済みのタグが含まれていることを検証する
	var posts []Post
	if err := json.Unmarshal([]byte(responseBody), &posts); err != nil {
		t.Fatalf("レスポンスのJSONパースに失敗: %v", err)
	}

	found := false
	for _, post := range posts {
		if strings.Contains(post.Content, "&lt;script&gt;") {
			found = true
			break
		}
	}
	if !found {
		t.Error("エスケープ済みの&lt;script&gt;タグが投稿内に見つからない")
	}
}

// Content-Security-Policyヘッダーが設定されていることを検証する
func TestContentSecurityPolicyHeader(t *testing.T) {
	handler := setupRouter()

	req := httptest.NewRequest(http.MethodGet, "/posts", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policyヘッダーが設定されていない")
	}
	if csp != "default-src 'self'" {
		t.Errorf("Content-Security-Policyの値が不正: %s", csp)
	}
}

// X-Content-Type-Optionsヘッダーが設定されていることを検証する
func TestXContentTypeOptionsHeader(t *testing.T) {
	handler := setupRouter()

	req := httptest.NewRequest(http.MethodGet, "/posts", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	xcto := rec.Header().Get("X-Content-Type-Options")
	if xcto == "" {
		t.Error("X-Content-Type-Optionsヘッダーが設定されていない")
	}
	if xcto != "nosniff" {
		t.Errorf("X-Content-Type-Optionsの値が不正: %s", xcto)
	}
}
