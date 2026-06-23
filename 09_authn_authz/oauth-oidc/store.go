package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// User はシードユーザ(認可される本人 = Resource Owner)。
type User struct {
	Sub   string
	Name  string
	Email string
}

// Client は登録済み OAuth クライアント。
// public クライアント(PKCE 利用)と confidential クライアント(client_secret 保持)を表現する。
type Client struct {
	ID           string
	Secret       string // confidential のみ。public は空
	RedirectURIs []string
	Confidential bool
}

// AuthCode は認可コード。PKCE challenge・redirect_uri・nonce・本人を束縛し、
// 短い TTL かつ単回使用(交換後は削除)とする。
type AuthCode struct {
	Code          string
	ClientID      string
	RedirectURI   string
	CodeChallenge string // S256 の code_challenge
	Nonce         string
	Sub           string
	Scope         string
	ExpiresAt     time.Time
}

// RefreshToken はリフレッシュトークン(インメモリ)。
type RefreshToken struct {
	Token    string
	ClientID string
	Sub      string
	Scope    string
}

// Store は認可サーバの揮発状態(コード・リフレッシュトークン・ユーザ・クライアント)。
type Store struct {
	mu       sync.Mutex
	codes    map[string]*AuthCode
	refresh  map[string]*RefreshToken
	users    map[string]*User   // sub -> User
	clients  map[string]*Client // client_id -> Client
}

// 学習用のシード値。public クライアント(RP)と confidential クライアント(M2M)を用意する。
const (
	demoUserSub        = "user-alice"
	publicClientID     = "demo-web-app"        // RP(PKCE 利用の public クライアント)
	confidentialClient = "demo-service"        // M2M(client_credentials)
	confidentialSecret = "demo-service-secret" // 学習用に明示
)

// NewStore はシードユーザ・クライアントを登録した Store を返す。
func NewStore(issuer string) *Store {
	return &Store{
		codes:   make(map[string]*AuthCode),
		refresh: make(map[string]*RefreshToken),
		users: map[string]*User{
			demoUserSub: {Sub: demoUserSub, Name: "Alice Example", Email: "alice@example.com"},
		},
		clients: map[string]*Client{
			publicClientID: {
				ID:           publicClientID,
				RedirectURIs: []string{issuer + "/app/callback"},
				Confidential: false,
			},
			confidentialClient: {
				ID:           confidentialClient,
				Secret:       confidentialSecret,
				Confidential: true,
			},
		},
	}
}

// randToken は暗号学的乱数から 16 進トークンを生成する。
func randToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Client はクライアントを取得する。
func (s *Store) Client(id string) (*Client, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clients[id]
	return c, ok
}

// User はユーザを取得する。
func (s *Store) User(sub string) (*User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[sub]
	return u, ok
}

// SaveCode は認可コードを保存する(TTL 60 秒)。
func (s *Store) SaveCode(c *AuthCode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.ExpiresAt = time.Now().Add(60 * time.Second)
	s.codes[c.Code] = c
}

// TakeCode は認可コードを取り出して即削除する(単回使用)。
// 期限切れの場合も削除し、(nil,false) を返す。
func (s *Store) TakeCode(code string) (*AuthCode, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.codes[code]
	if !ok {
		return nil, false
	}
	delete(s.codes, code) // 取り出した時点で削除 = 単回使用を保証
	if time.Now().After(c.ExpiresAt) {
		return nil, false
	}
	return c, true
}

// SaveRefresh はリフレッシュトークンを保存する。
func (s *Store) SaveRefresh(rt *RefreshToken) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refresh[rt.Token] = rt
}

// Refresh はリフレッシュトークンを取得する。
func (s *Store) Refresh(token string) (*RefreshToken, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt, ok := s.refresh[token]
	return rt, ok
}
