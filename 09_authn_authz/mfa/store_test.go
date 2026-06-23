package main

import (
	"testing"
	"time"
)

// peekToken はテスト専用ヘルパ。発行済みの最初のトークン文字列を返す。
// 本来トークンはメールでしか伝わらないが、SMTP に依存せずテストするため
// インプロセスでストアから直接読み取る(消費はしない)。
func (m *MagicStore) peekToken(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for tok := range m.tokens {
		return tok
	}
	t.Fatal("発行済みトークンがありません")
	return ""
}

// TestMagicStoreConsumeOnce はトークンが1回だけ消費可能であることを検証する。
func TestMagicStoreConsumeOnce(t *testing.T) {
	m := NewMagicStore(time.Minute)
	tok := m.Issue("alice")

	name, ok := m.Consume(tok.Token)
	if !ok || name != "alice" {
		t.Fatalf("1回目の Consume = (%q, %v), want (alice, true)", name, ok)
	}
	if _, ok := m.Consume(tok.Token); ok {
		t.Error("2回目の Consume が成功しました(使い捨てではない)")
	}
}

// TestMagicStoreExpired は期限切れトークンが拒否されることを検証する。
func TestMagicStoreExpired(t *testing.T) {
	m := NewMagicStore(-time.Second) // 即座に期限切れ
	tok := m.Issue("alice")
	if _, ok := m.Consume(tok.Token); ok {
		t.Error("期限切れトークンが受理されました")
	}
}

// TestSessionStoreCreateGet はセッションの生成・取得を検証する。
func TestSessionStoreCreateGet(t *testing.T) {
	s := NewSessionStore()
	sess := s.Create("alice")
	if sess.ID == "" {
		t.Fatal("セッションIDが空です")
	}
	got, ok := s.Get(sess.ID)
	if !ok || got.Username != "alice" {
		t.Errorf("Get = (%v, %v), want alice", got, ok)
	}
}
