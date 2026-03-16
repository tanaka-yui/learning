package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// コマンドインジェクション阻止: セミコロンによるコマンド連結が400で拒否される
func TestLookup_InjectionBlocked_Semicolon(t *testing.T) {
	handler := setupRouter()
	rr := performRequest(t, handler, http.MethodPost, "/lookup", map[string]string{"host": "localhost; echo INJECTED"})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("期待するステータスコード: %d, 実際: %d", http.StatusBadRequest, rr.Code)
	}
}

// コマンドインジェクション阻止: &&によるコマンド連結が400で拒否される
func TestLookup_InjectionBlocked_And(t *testing.T) {
	handler := setupRouter()
	rr := performRequest(t, handler, http.MethodPost, "/lookup", map[string]string{"host": "localhost && cat /etc/passwd"})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("期待するステータスコード: %d, 実際: %d", http.StatusBadRequest, rr.Code)
	}
}

// コマンドインジェクション阻止: パイプによるコマンド連結が400で拒否される
func TestLookup_InjectionBlocked_Pipe(t *testing.T) {
	handler := setupRouter()
	rr := performRequest(t, handler, http.MethodPost, "/lookup", map[string]string{"host": "localhost | ls"})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("期待するステータスコード: %d, 実際: %d", http.StatusBadRequest, rr.Code)
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

// 異常系: hostフィールドが空文字列の場合は400を返す
func TestLookup_EmptyHost(t *testing.T) {
	handler := setupRouter()
	rr := performRequest(t, handler, http.MethodPost, "/lookup", map[string]string{"host": ""})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("期待するステータスコード: %d, 実際: %d", http.StatusBadRequest, rr.Code)
	}
}

// バリデーション: ドット付きホスト名は許可される
func TestLookup_ValidHostWithDots(t *testing.T) {
	handler := setupRouter()
	rr := performRequest(t, handler, http.MethodPost, "/lookup", map[string]string{"host": "example.com"})

	// nslookupが失敗しても200が返る（出力があれば）
	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("期待するステータスコード: 200 or 500, 実際: %d", rr.Code)
	}
}

// バリデーション: ハイフン付きホスト名は許可される
func TestLookup_ValidHostWithHyphen(t *testing.T) {
	handler := setupRouter()
	rr := performRequest(t, handler, http.MethodPost, "/lookup", map[string]string{"host": "my-host.example.com"})

	if rr.Code != http.StatusOK && rr.Code != http.StatusInternalServerError {
		t.Errorf("期待するステータスコード: 200 or 500, 実際: %d", rr.Code)
	}
}
