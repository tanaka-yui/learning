package main

import "testing"

// TestSessionCreateGetDelete はセッションの生成・取得・破棄を検証する
func TestSessionCreateGetDelete(t *testing.T) {
	store := NewSessionStore()

	sess := store.Create("alice")
	if sess.ID == "" {
		t.Fatal("セッションIDが空です")
	}
	if sess.Username != "alice" {
		t.Errorf("Username = %q, want alice", sess.Username)
	}

	got, ok := store.Get(sess.ID)
	if !ok {
		t.Fatal("生成したセッションを取得できません")
	}
	if got.Username != "alice" {
		t.Errorf("取得した Username = %q, want alice", got.Username)
	}

	store.Delete(sess.ID)
	if _, ok := store.Get(sess.ID); ok {
		t.Error("破棄後もセッションが残っています")
	}
}

// TestSessionIDsAreUnique は生成のたびに異なるIDになることを検証する
func TestSessionIDsAreUnique(t *testing.T) {
	store := NewSessionStore()
	a := store.Create("alice")
	b := store.Create("alice")
	if a.ID == b.ID {
		t.Error("セッションIDが重複しています")
	}
}

// TestUserVerify はユーザ照合を検証する
func TestUserVerify(t *testing.T) {
	users := NewUserStore()
	if !users.Verify("alice", "password123") {
		t.Error("正しい認証情報が拒否されました")
	}
	if users.Verify("alice", "wrong") {
		t.Error("誤ったパスワードが受理されました")
	}
	if users.Verify("nobody", "password123") {
		t.Error("存在しないユーザが受理されました")
	}
}
