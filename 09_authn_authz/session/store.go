package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Session はログイン状態を表す
type Session struct {
	ID        string
	Username  string
	CreatedAt time.Time
}

// SessionStore はインメモリのセッション保管庫(学習用)
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionStore は空のセッションストアを返す
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

// newID は暗号学的乱数から32バイトのセッションIDを生成する
func newID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // 乱数生成の失敗は継続不能
	}
	return hex.EncodeToString(b)
}

// Create は新しいセッションを生成して保存する
func (s *SessionStore) Create(username string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := &Session{ID: newID(), Username: username, CreatedAt: time.Now()}
	s.sessions[sess.ID] = sess
	return sess
}

// Get はIDからセッションを取得する
func (s *SessionStore) Get(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

// Delete はセッションを破棄する
func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}
