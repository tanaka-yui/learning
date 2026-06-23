package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestServer はテスト用にエンフォーサを初期化してサーバを起動する
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	rbac, err := newRBACEnforcer()
	if err != nil {
		t.Fatalf("RBAC エンフォーサ初期化失敗: %v", err)
	}
	abac, err := newABACEnforcer()
	if err != nil {
		t.Fatalf("ABAC エンフォーサ初期化失敗: %v", err)
	}
	return httptest.NewServer(setupRouter(rbac, abac))
}

// doReq は X-User ヘッダ付きのリクエストを送り、レスポンスを返す
func doReq(t *testing.T, server *httptest.Server, method, path, user string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, nil)
	if err != nil {
		t.Fatalf("リクエスト生成失敗: %v", err)
	}
	if user != "" {
		req.Header.Set("X-User", user)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("リクエスト送信失敗: %v", err)
	}
	return resp
}

// assertStatus はステータスコードを検証するヘルパー
func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Errorf("status = %d, want %d", resp.StatusCode, want)
	}
}

// TestIndexReturnsHTML はルートが HTML を返すことを確認する
func TestIndexReturnsHTML(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET / 失敗: %v", err)
	}
	assertStatus(t, resp, http.StatusOK)
}

// TestRBAC_AdminAllowed は admin(alice)がすべての操作を実行できることを検証する
func TestRBAC_AdminAllowed(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	// GET /docs: admin は viewer 以上なので許可される
	assertStatus(t, doReq(t, server, http.MethodGet, "/docs", "alice"), http.StatusOK)
	// POST /docs: admin は editor 以上なので許可される
	assertStatus(t, doReq(t, server, http.MethodPost, "/docs", "alice"), http.StatusOK)
	// DELETE /docs/doc1: admin のみ許可
	assertStatus(t, doReq(t, server, http.MethodDelete, "/docs/doc1", "alice"), http.StatusOK)
}

// TestRBAC_EditorCanWriteButNotDelete は editor(bob)が GET/POST は許可されるが DELETE は拒否されることを検証する
func TestRBAC_EditorCanWriteButNotDelete(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	// GET /docs: editor は viewer 以上なので許可される
	assertStatus(t, doReq(t, server, http.MethodGet, "/docs", "bob"), http.StatusOK)
	// POST /docs: editor は許可される
	assertStatus(t, doReq(t, server, http.MethodPost, "/docs", "bob"), http.StatusOK)
	// DELETE /docs/doc1: editor は admin でないので拒否される
	assertStatus(t, doReq(t, server, http.MethodDelete, "/docs/doc1", "bob"), http.StatusForbidden)
}

// TestRBAC_ViewerCanReadOnly は viewer(carol)が GET のみ許可されることを検証する
func TestRBAC_ViewerCanReadOnly(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	// GET /docs: viewer は許可される
	assertStatus(t, doReq(t, server, http.MethodGet, "/docs", "carol"), http.StatusOK)
	// POST /docs: viewer は拒否される
	assertStatus(t, doReq(t, server, http.MethodPost, "/docs", "carol"), http.StatusForbidden)
	// DELETE /docs/doc1: viewer は拒否される
	assertStatus(t, doReq(t, server, http.MethodDelete, "/docs/doc1", "carol"), http.StatusForbidden)
}

// TestRBAC_NoUserHeaderReturnsUnauthorized は X-User ヘッダなしで 401 になることを検証する
func TestRBAC_NoUserHeaderReturnsUnauthorized(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	assertStatus(t, doReq(t, server, http.MethodGet, "/docs", ""), http.StatusUnauthorized)
}

// TestRBAC_UnknownUserDenied は未定義ユーザが 403 になることを検証する
func TestRBAC_UnknownUserDenied(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	// viewer-user はポリシーに存在しないので拒否される
	assertStatus(t, doReq(t, server, http.MethodGet, "/docs", "viewer-user"), http.StatusForbidden)
	assertStatus(t, doReq(t, server, http.MethodPost, "/docs", "viewer-user"), http.StatusForbidden)
}

// TestEdit_ViewerOwnerDeniedByRBAC は viewer(carol)が自分の所有リソース(doc3)を
// 編集しようとしても RBAC の書き込み権限がないため 403 になることを検証する。
// これは「ABAC 単独ガードによる認可バイパス」の修正(RBAC+ABAC 多層化)を保証する回帰テスト。
func TestEdit_ViewerOwnerDeniedByRBAC(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	// carol は doc3 の所有者だが viewer なので書き込み(POST /docs)権限がない → 403
	assertStatus(t, doReq(t, server, http.MethodPost, "/docs/doc3/edit", "carol"), http.StatusForbidden)
}

// TestEdit_EditorOwnerAllowed は editor(bob)が自分の所有リソース(doc2)を
// 編集できる(RBAC 書き込み可 かつ ABAC 所有者一致)ことを検証する。
func TestEdit_EditorOwnerAllowed(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	// bob は editor かつ doc2 の所有者 → 200
	assertStatus(t, doReq(t, server, http.MethodPost, "/docs/doc2/edit", "bob"), http.StatusOK)
}

// TestEdit_EditorNonOwnerDeniedByABAC は editor(bob)が他人の所有リソース(doc1, alice所有)を
// 編集しようとすると ABAC の所有者チェックで拒否され 403 になることを検証する。
func TestEdit_EditorNonOwnerDeniedByABAC(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	// bob は editor だが doc1 の所有者ではない(所有者は alice) → ABAC で 403
	assertStatus(t, doReq(t, server, http.MethodPost, "/docs/doc1/edit", "bob"), http.StatusForbidden)
}

// TestEdit_NoUserHeaderReturnsUnauthorized は X-User なしで編集ルートが 401 になることを検証する。
func TestEdit_NoUserHeaderReturnsUnauthorized(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	assertStatus(t, doReq(t, server, http.MethodPost, "/docs/doc2/edit", ""), http.StatusUnauthorized)
}
