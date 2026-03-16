package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// テスト用のサーバーを作成する
func setupTestServer() http.Handler {
	db := initDB()
	seedData(db)
	return setupRouter(db)
}

// 通常の検索: name=Alice でAliceのみ返却されることを検証する
func TestNormalSearch(t *testing.T) {
	handler := setupTestServer()

	req := httptest.NewRequest("GET", "/users?name=Alice", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ステータスコードが不正: got %d, want %d", w.Code, http.StatusOK)
	}

	var users []User
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatalf("JSONのパースに失敗: %v", err)
	}

	if len(users) != 1 {
		t.Fatalf("返却されたユーザー数が不正: got %d, want 1", len(users))
	}

	if users[0].Name != "Alice" {
		t.Errorf("ユーザー名が不正: got %s, want Alice", users[0].Name)
	}

	if users[0].Email != "alice@example.com" {
		t.Errorf("メールアドレスが不正: got %s, want alice@example.com", users[0].Email)
	}
}

// SQLインジェクション攻撃がブロックされることを検証する
// プリペアドステートメントにより、攻撃ペイロードは単なる文字列として扱われ、
// 該当するユーザーが存在しないため0件が返却される
func TestSQLInjectionBlocked(t *testing.T) {
	handler := setupTestServer()

	// SQLインジェクションペイロード: ' OR '1'='1
	req := httptest.NewRequest("GET", "/users?name='+OR+'1'%3D'1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ステータスコードが不正: got %d, want %d", w.Code, http.StatusOK)
	}

	var users []User
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatalf("JSONのパースに失敗: %v", err)
	}

	// プリペアドステートメントにより攻撃が無効化され、0件が返却されることを確認する
	if len(users) != 0 {
		t.Fatalf("SQLインジェクションがブロックされるべき: got %d件, want 0件", len(users))
	}
}

// 全件取得: nameパラメータなしで全ユーザーが返却されることを検証する
func TestListAllUsers(t *testing.T) {
	handler := setupTestServer()

	req := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ステータスコードが不正: got %d, want %d", w.Code, http.StatusOK)
	}

	var users []User
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatalf("JSONのパースに失敗: %v", err)
	}

	if len(users) != 3 {
		t.Fatalf("返却されたユーザー数が不正: got %d, want 3", len(users))
	}

	// シードデータの名前を検証する
	expectedNames := map[string]bool{"Alice": false, "Bob": false, "Charlie": false}
	for _, u := range users {
		if _, ok := expectedNames[u.Name]; ok {
			expectedNames[u.Name] = true
		}
	}
	for name, found := range expectedNames {
		if !found {
			t.Errorf("期待されるユーザーが見つからない: %s", name)
		}
	}
}

// CORSヘッダーが正しく設定されていることを検証する
func TestCORSHeaders(t *testing.T) {
	handler := setupTestServer()

	req := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Originが不正: got %s, want *", got)
	}

	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Errorf("Access-Control-Allow-Methodsが不正: got %s, want GET, OPTIONS", got)
	}

	if got := w.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Errorf("Access-Control-Allow-Headersが不正: got %s, want Content-Type", got)
	}
}

// レスポンスにパスワードが含まれていないことを検証する
func TestPasswordNotExposed(t *testing.T) {
	handler := setupTestServer()

	req := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if contains(body, "password") {
		t.Error("レスポンスにpasswordフィールドが含まれている")
	}
}

// 文字列に指定のサブ文字列が含まれるかを判定する
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
