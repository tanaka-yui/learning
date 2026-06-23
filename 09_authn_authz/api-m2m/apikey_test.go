package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newAPIKeyTestServer はAPIキーテスト用サーバを構築する
func newAPIKeyTestServer() *httptest.Server {
	return httptest.NewServer(setupRouter())
}

// TestAPIKeyValidXAPIKeyHeader はX-API-Keyヘッダで有効なキーを送ると200が返ることを検証する
func TestAPIKeyValidXAPIKeyHeader(t *testing.T) {
	server := newAPIKeyTestServer()
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/data", nil)
	req.Header.Set("X-API-Key", "key-service-a-secret-1234")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("リクエスト失敗: %v", err)
	}
	defer resp.Body.Close()

	// 有効なAPIキーは200を返す
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got map[string]string
	json.NewDecoder(resp.Body).Decode(&got)
	// クライアント名が正しく返る
	if got["client"] != "service-a" {
		t.Errorf("client = %q, want service-a", got["client"])
	}
}

// TestAPIKeyValidBearerToken はBearerトークン形式で有効なキーを送ると200が返ることを検証する
func TestAPIKeyValidBearerToken(t *testing.T) {
	server := newAPIKeyTestServer()
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/data", nil)
	req.Header.Set("Authorization", "Bearer key-service-b-secret-5678")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("リクエスト失敗: %v", err)
	}
	defer resp.Body.Close()

	// Bearerトークン形式でも認証が成功する
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got map[string]string
	json.NewDecoder(resp.Body).Decode(&got)
	// service-b として識別される
	if got["client"] != "service-b" {
		t.Errorf("client = %q, want service-b", got["client"])
	}
}

// TestAPIKeyMissingKey はAPIキーなしのリクエストが401を返すことを検証する
func TestAPIKeyMissingKey(t *testing.T) {
	server := newAPIKeyTestServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/data")
	if err != nil {
		t.Fatalf("リクエスト失敗: %v", err)
	}
	defer resp.Body.Close()

	// キーなしは401
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// TestAPIKeyInvalidKey は無効なAPIキーが401を返すことを検証する
func TestAPIKeyInvalidKey(t *testing.T) {
	server := newAPIKeyTestServer()
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/data", nil)
	req.Header.Set("X-API-Key", "invalid-key-that-does-not-exist")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("リクエスト失敗: %v", err)
	}
	defer resp.Body.Close()

	// 無効なキーは401
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// TestAPIKeyConstantTimeCompare は定数時間比較の動作を検証する
// 実際のタイミング計測は行わず、正しいキーと前方一致するキーがいずれも拒否されることを確認する
func TestAPIKeyConstantTimeCompare(t *testing.T) {
	server := newAPIKeyTestServer()
	defer server.Close()

	// 正しいキーの前半部分(プレフィックス一致)は拒否される
	prefixKey := "key-service-a-secret"
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/data", nil)
	req.Header.Set("X-API-Key", prefixKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("リクエスト失敗: %v", err)
	}
	defer resp.Body.Close()

	// プレフィックス一致は定数時間比較により拒否される
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("プレフィックス一致 status = %d, want 401 (定数時間比較が機能していない可能性)", resp.StatusCode)
	}
}

// TestIndexPage はルートパスが200を返すことを検証する
func TestIndexPage(t *testing.T) {
	server := newAPIKeyTestServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("リクエスト失敗: %v", err)
	}
	defer resp.Body.Close()

	// インデックスページは認証不要で200
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
