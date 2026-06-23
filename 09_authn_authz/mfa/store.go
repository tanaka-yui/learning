package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// newToken は暗号学的乱数から32バイトのトークン/IDを生成する。
func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // 乱数生成の失敗は継続不能
	}
	return hex.EncodeToString(b)
}

// Session はログイン完了状態を表す。
type Session struct {
	ID        string
	Username  string
	CreatedAt time.Time
}

// SessionStore はインメモリのセッション保管庫(学習用)。
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionStore は空のセッションストアを返す。
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

// Create は新しいセッションを生成して保存する。
func (s *SessionStore) Create(username string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := &Session{ID: newToken(), Username: username, CreatedAt: time.Now()}
	s.sessions[sess.ID] = sess
	return sess
}

// Get はIDからセッションを取得する。
func (s *SessionStore) Get(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

// MagicToken は使い捨てのマジックリンクトークンを表す。
type MagicToken struct {
	Token     string
	Username  string
	ExpiresAt time.Time
}

// MagicStore はマジックリンクのトークンをインメモリで保持する(学習用)。
type MagicStore struct {
	mu     sync.Mutex
	tokens map[string]*MagicToken
	ttl    time.Duration
}

// NewMagicStore は指定TTLのマジックリンクストアを返す。
func NewMagicStore(ttl time.Duration) *MagicStore {
	return &MagicStore{tokens: make(map[string]*MagicToken), ttl: ttl}
}

// Issue はユーザ向けに使い捨てトークンを発行して保存する。
func (m *MagicStore) Issue(username string) *MagicToken {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := &MagicToken{
		Token:     newToken(),
		Username:  username,
		ExpiresAt: time.Now().Add(m.ttl),
	}
	m.tokens[t.Token] = t
	return t
}

// Consume はトークンを1回だけ消費する。
// 有効なら (username, true) を返し、トークンを削除する(再利用不可)。
// 期限切れ・存在しない場合は ("", false)。
func (m *MagicStore) Consume(token string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tokens[token]
	if !ok {
		return "", false
	}
	// 使い捨て: 検証可否にかかわらず取得時点で削除する。
	delete(m.tokens, token)
	if time.Now().After(t.ExpiresAt) {
		return "", false
	}
	return t.Username, true
}

// WebAuthnSessionStore は WebAuthn セレモニー中の SessionData を保持する(学習用)。
// 本来はサーバ側セッションや暗号化Cookieに格納するが、デモのため単一ユーザ前提のインメモリ。
type WebAuthnSessionStore struct {
	mu       sync.Mutex
	register map[string]*webauthn.SessionData
	login    map[string]*webauthn.SessionData
}

// NewWebAuthnSessionStore は空の WebAuthn セッションストアを返す。
func NewWebAuthnSessionStore() *WebAuthnSessionStore {
	return &WebAuthnSessionStore{
		register: make(map[string]*webauthn.SessionData),
		login:    make(map[string]*webauthn.SessionData),
	}
}

// SaveRegister は登録セレモニーの SessionData を保存する。
func (w *WebAuthnSessionStore) SaveRegister(username string, sd *webauthn.SessionData) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.register[username] = sd
}

// LoadRegister は登録セレモニーの SessionData を取得する。
func (w *WebAuthnSessionStore) LoadRegister(username string) (*webauthn.SessionData, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	sd, ok := w.register[username]
	return sd, ok
}

// SaveLogin はログインセレモニーの SessionData を保存する。
func (w *WebAuthnSessionStore) SaveLogin(username string, sd *webauthn.SessionData) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.login[username] = sd
}

// LoadLogin はログインセレモニーの SessionData を取得する。
func (w *WebAuthnSessionStore) LoadLogin(username string) (*webauthn.SessionData, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	sd, ok := w.login[username]
	return sd, ok
}
