package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// テスト用のヘルパー: リクエストを送信してレスポンスを返す
func performRequest(t *testing.T, handler http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("リクエストボディのJSON変換に失敗: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// 正常系: nslookupが実行され200が返る
func TestLookup_Normal(t *testing.T) {
	handler := setupRouter()
	rr := performRequest(t, handler, http.MethodPost, "/lookup", map[string]string{"host": "localhost"})

	if rr.Code != http.StatusOK {
		t.Errorf("期待するステータスコード: %d, 実際: %d", http.StatusOK, rr.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("レスポンスのJSON解析に失敗: %v", err)
	}
	if _, ok := resp["output"]; !ok {
		t.Error("レスポンスに'output'フィールドが含まれていない")
	}
}

// コマンドインジェクション: セミコロンで追加コマンドが実行される（脆弱性の証明）
func TestLookup_CommandInjection(t *testing.T) {
	handler := setupRouter()
	rr := performRequest(t, handler, http.MethodPost, "/lookup", map[string]string{"host": "localhost; echo INJECTED"})

	if rr.Code != http.StatusOK {
		t.Errorf("期待するステータスコード: %d, 実際: %d", http.StatusOK, rr.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("レスポンスのJSON解析に失敗: %v", err)
	}
	if !strings.Contains(resp["output"], "INJECTED") {
		t.Errorf("コマンドインジェクションが成功するはず: 出力に'INJECTED'が含まれていない。出力: %s", resp["output"])
	}
}

// 正常系: pingが実行され200が返る
func TestPing_Normal(t *testing.T) {
	handler := setupRouter()
	rr := performRequest(t, handler, http.MethodPost, "/ping", map[string]string{"host": "localhost"})

	if rr.Code != http.StatusOK {
		t.Errorf("期待するステータスコード: %d, 実際: %d", http.StatusOK, rr.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("レスポンスのJSON解析に失敗: %v", err)
	}
	if _, ok := resp["output"]; !ok {
		t.Error("レスポンスに'output'フィールドが含まれていない")
	}
}

// 異常系: hostフィールドが存在しない場合は400を返す
func TestLookup_MissingHost(t *testing.T) {
	handler := setupRouter()
	rr := performRequest(t, handler, http.MethodPost, "/lookup", map[string]string{})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("期待するステータスコード: %d, 実際: %d", http.StatusBadRequest, rr.Code)
	}
}

// 異常系: hostフィールドが空文字列の場合は400を返す
func TestLookup_EmptyHost(t *testing.T) {
	handler := setupRouter()
	rr := performRequest(t, handler, http.MethodPost, "/lookup", map[string]string{"host": ""})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("期待するステータスコード: %d, 実際: %d", http.StatusBadRequest, rr.Code)
	}
}
