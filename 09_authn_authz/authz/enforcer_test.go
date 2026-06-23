package main

import (
	"testing"
)

// TestRBACEnforcer_AdminAllowed は admin ロールがすべての操作を許可されることを検証する
func TestRBACEnforcer_AdminAllowed(t *testing.T) {
	e, err := newRBACEnforcer()
	if err != nil {
		t.Fatalf("エンフォーサ初期化失敗: %v", err)
	}

	cases := []struct{ act string }{{"GET"}, {"POST"}, {"DELETE"}}
	for _, c := range cases {
		allowed, err := e.Enforce("alice", "/docs", c.act)
		if err != nil {
			t.Fatalf("Enforce エラー: %v", err)
		}
		if !allowed {
			t.Errorf("admin(alice) の %s /docs は許可されるべきですが拒否されました", c.act)
		}
	}
}

// TestRBACEnforcer_EditorAllowed は editor ロールが GET/POST を許可されることを検証する
func TestRBACEnforcer_EditorAllowed(t *testing.T) {
	e, err := newRBACEnforcer()
	if err != nil {
		t.Fatalf("エンフォーサ初期化失敗: %v", err)
	}

	for _, act := range []string{"GET", "POST"} {
		allowed, err := e.Enforce("bob", "/docs", act)
		if err != nil {
			t.Fatalf("Enforce エラー: %v", err)
		}
		if !allowed {
			t.Errorf("editor(bob) の %s /docs は許可されるべきですが拒否されました", act)
		}
	}
}

// TestRBACEnforcer_EditorDeniedDelete は editor ロールが DELETE を拒否されることを検証する
func TestRBACEnforcer_EditorDeniedDelete(t *testing.T) {
	e, err := newRBACEnforcer()
	if err != nil {
		t.Fatalf("エンフォーサ初期化失敗: %v", err)
	}

	allowed, err := e.Enforce("bob", "/docs", "DELETE")
	if err != nil {
		t.Fatalf("Enforce エラー: %v", err)
	}
	if allowed {
		t.Error("editor(bob) の DELETE /docs は拒否されるべきですが許可されました")
	}
}

// TestRBACEnforcer_ViewerCanReadOnly は viewer ロールが GET のみ許可されることを検証する
func TestRBACEnforcer_ViewerCanReadOnly(t *testing.T) {
	e, err := newRBACEnforcer()
	if err != nil {
		t.Fatalf("エンフォーサ初期化失敗: %v", err)
	}

	allowed, err := e.Enforce("carol", "/docs", "GET")
	if err != nil {
		t.Fatalf("Enforce エラー: %v", err)
	}
	if !allowed {
		t.Error("viewer(carol) の GET /docs は許可されるべきですが拒否されました")
	}

	for _, act := range []string{"POST", "DELETE"} {
		allowed, err = e.Enforce("carol", "/docs", act)
		if err != nil {
			t.Fatalf("Enforce エラー: %v", err)
		}
		if allowed {
			t.Errorf("viewer(carol) の %s /docs は拒否されるべきですが許可されました", act)
		}
	}
}

// TestRBACEnforcer_RoleInheritance はロール継承(admin>editor>viewer)が
// 実際に権限へ伝播することを検証する。
// 各能力は最下位ロールにのみ付与されており(viewer→GET, editor→POST, admin→DELETE)、
// 上位ロールはそれを継承して許可されなければならない。
func TestRBACEnforcer_RoleInheritance(t *testing.T) {
	e, err := newRBACEnforcer()
	if err != nil {
		t.Fatalf("エンフォーサ初期化失敗: %v", err)
	}

	// editor(bob) は viewer の GET 権限を継承する(POST は editor 直付与)。
	allowed, err := e.Enforce("bob", "/docs", "GET")
	if err != nil {
		t.Fatalf("Enforce エラー: %v", err)
	}
	if !allowed {
		t.Error("editor(bob) は viewer から GET を継承するべきですが拒否されました")
	}

	// admin(alice) は editor の POST と viewer の GET を継承する(DELETE は admin 直付与)。
	for _, act := range []string{"GET", "POST"} {
		allowed, err := e.Enforce("alice", "/docs", act)
		if err != nil {
			t.Fatalf("Enforce エラー: %v", err)
		}
		if !allowed {
			t.Errorf("admin(alice) は下位ロールから %s を継承するべきですが拒否されました", act)
		}
	}

	// 継承の階層が g で表現されていることも確認する。
	roles, err := e.GetRolesForUser("alice")
	if err != nil {
		t.Fatalf("GetRolesForUser エラー: %v", err)
	}
	if !contains(roles, "admin") {
		t.Errorf("alice のロール = %v, want admin を含むこと", roles)
	}
}

// contains はスライスに目的の文字列が含まれるかを返す小ヘルパー。
func contains(xs []string, target string) bool {
	for _, x := range xs {
		if x == target {
			return true
		}
	}
	return false
}

// TestABACEnforcer_OwnerCanEdit は所有者がリソースを編集できることを検証する
func TestABACEnforcer_OwnerCanEdit(t *testing.T) {
	e, err := newABACEnforcer()
	if err != nil {
		t.Fatalf("ABAC エンフォーサ初期化失敗: %v", err)
	}

	// alice は doc1 の所有者 → 編集許可
	res := Resource{ID: "doc1", Owner: "alice"}
	allowed, err := e.Enforce("alice", res, "edit")
	if err != nil {
		t.Fatalf("Enforce エラー: %v", err)
	}
	if !allowed {
		t.Error("所有者(alice)の編集は許可されるべきですが拒否されました")
	}
}

// TestABACEnforcer_NonOwnerDenied は非所有者がリソースを編集できないことを検証する
func TestABACEnforcer_NonOwnerDenied(t *testing.T) {
	e, err := newABACEnforcer()
	if err != nil {
		t.Fatalf("ABAC エンフォーサ初期化失敗: %v", err)
	}

	// bob は doc1 の所有者でない → 編集拒否
	res := Resource{ID: "doc1", Owner: "alice"}
	allowed, err := e.Enforce("bob", res, "edit")
	if err != nil {
		t.Fatalf("Enforce エラー: %v", err)
	}
	if allowed {
		t.Error("非所有者(bob)の編集は拒否されるべきですが許可されました")
	}
}

// TestABACEnforcer_OwnerEditsOwnResource は各ユーザが自分のリソースのみ編集できることを検証する
func TestABACEnforcer_OwnerEditsOwnResource(t *testing.T) {
	e, err := newABACEnforcer()
	if err != nil {
		t.Fatalf("ABAC エンフォーサ初期化失敗: %v", err)
	}

	cases := []struct {
		sub     string
		resID   string
		owner   string
		wantOK  bool
	}{
		{"alice", "doc1", "alice", true},  // alice が自分の doc1 を編集
		{"bob", "doc2", "bob", true},      // bob が自分の doc2 を編集
		{"carol", "doc1", "alice", false}, // carol が alice の doc1 を編集 → 拒否
		{"alice", "doc2", "bob", false},   // alice が bob の doc2 を編集 → 拒否
	}

	for _, c := range cases {
		res := Resource{ID: c.resID, Owner: c.owner}
		allowed, err := e.Enforce(c.sub, res, "edit")
		if err != nil {
			t.Fatalf("Enforce エラー: %v", err)
		}
		if allowed != c.wantOK {
			t.Errorf("%s が %s(%s所有) を edit: got %v, want %v", c.sub, c.resID, c.owner, allowed, c.wantOK)
		}
	}
}
