package main

import (
	"encoding/json"
	"net/http"

	"github.com/casbin/casbin/v2"
)

// indexHTML は動作確認用の最小HTML
const indexHTML = `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>RBAC/ABAC認可デモ</title></head>
<body>
<h1>RBAC/ABAC認可デモ</h1>
<p>X-User ヘッダでユーザを指定してください。</p>
<ul>
  <li>alice → admin ロール (editor/viewer を継承し GET/POST/DELETE /docs すべて許可)</li>
  <li>bob → editor ロール (viewer を継承し GET/POST /docs 許可、DELETE 禁止)</li>
  <li>carol → viewer ロール (GET /docs のみ許可)</li>
</ul>
<p>POST /docs/{id}/edit は RBAC と ABAC の両方を満たす場合のみ編集可能:</p>
<ul>
  <li>RBAC: 書き込み権限 (POST /docs) を持つこと (editor 以上)</li>
  <li>ABAC: そのリソースの所有者であること (sub == resource.Owner)</li>
</ul>
<p>例: carol は viewer なので自分の所有リソースでも編集不可 (RBAC で拒否)。bob は editor なので自分の所有リソースのみ編集可。</p>
</body></html>`

// setupRouter は RBAC エンフォーサを受け取り http.Handler を構築する。
// PEP(Policy Enforcement Point)はミドルウェアとして実装する。
func setupRouter(rbac *casbin.Enforcer, abac *casbin.Enforcer) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(indexHTML))
	})

	// RBAC 保護ルート: GET /docs (viewer 以上)
	mux.Handle("GET /docs", requireRBAC(rbac, "/docs", "GET", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"docs": "一覧を返します"})
	}))

	// RBAC 保護ルート: POST /docs (editor 以上)
	mux.Handle("POST /docs", requireRBAC(rbac, "/docs", "POST", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ドキュメントを作成しました"})
	}))

	// RBAC 保護ルート: DELETE /docs/{id} (admin のみ)
	mux.Handle("DELETE /docs/{id}", requireRBAC(rbac, "/docs", "DELETE", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		writeJSON(w, http.StatusOK, map[string]string{"status": "削除しました", "id": id})
	}))

	// RBAC + ABAC 多層防御ルート: POST /docs/{id}/edit
	// (a) RBAC: 書き込み権限 (POST /docs) を持つこと (editor 以上) と
	// (b) ABAC: そのリソースの所有者であること (sub == resource.Owner)
	// の両方を満たした場合のみ許可する。どちらか一方でも欠ければ 403。
	// これにより viewer が自分の所有リソースでも編集できないことを保証する。
	mux.HandleFunc("POST /docs/{id}/edit", func(w http.ResponseWriter, r *http.Request) {
		sub := r.Header.Get("X-User")
		if sub == "" {
			http.Error(w, "X-User ヘッダが必要です", http.StatusUnauthorized)
			return
		}
		// (a) RBAC: 書き込み (POST /docs) 権限を持つロールかを判定する。
		rbacOK, rbacErr := rbac.Enforce(sub, "/docs", "POST")
		if rbacErr != nil || !rbacOK {
			http.Error(w, "アクセス拒否(書き込み権限がありません)", http.StatusForbidden)
			return
		}
		// (b) ABAC: リソース所有者本人かを判定する。
		id := r.PathValue("id")
		// インメモリのリソース所有者マップ(学習用固定値)
		res := lookupResource(id)
		abacOK, abacErr := abac.Enforce(sub, res, "edit")
		if abacErr != nil || !abacOK {
			http.Error(w, "アクセス拒否(所有者のみ編集可能)", http.StatusForbidden)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "編集しました", "id": id})
	})

	return mux
}

// requireRBAC は PEP(Policy Enforcement Point)ミドルウェア。
// X-User ヘッダからサブジェクトを取り出し、RBAC エンフォーサで判定する。
func requireRBAC(e *casbin.Enforcer, obj, act string, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub := r.Header.Get("X-User")
		if sub == "" {
			http.Error(w, "X-User ヘッダが必要です", http.StatusUnauthorized)
			return
		}
		allowed, err := e.Enforce(sub, obj, act)
		if err != nil || !allowed {
			http.Error(w, "アクセス拒否", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

// lookupResource はリソースIDから Resource を返す(学習用インメモリ固定値)。
func lookupResource(id string) Resource {
	owners := map[string]string{
		"doc1": "alice",
		"doc2": "bob",
		"doc3": "carol",
	}
	owner, ok := owners[id]
	if !ok {
		owner = ""
	}
	return Resource{ID: id, Owner: owner}
}

// writeJSON はJSONレスポンスを書き出す
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
