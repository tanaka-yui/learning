package main

import (
	"sync"

	"github.com/go-webauthn/webauthn/webauthn"
)

// User はインメモリのユーザを表す(学習用)。
// パスワード・TOTPシークレット・WebAuthn資格情報・メールアドレスを保持する。
// WebAuthn の webauthn.User インターフェースもこの型で実装する。
type User struct {
	mu sync.Mutex

	id          []byte // WebAuthn 用のユーザハンドル(安定したランダムバイト列)
	Username    string
	Email       string
	Password    string // 学習用に平文を保持(本来は bcrypt ハッシュ)
	TOTPSecret  string // TOTP のBase32シークレット(エンロール後に設定)
	credentials []webauthn.Credential
}

// UserStore はユーザをインメモリで保持する(学習用シード)。
type UserStore struct {
	mu      sync.RWMutex
	byName  map[string]*User
	byEmail map[string]*User
}

// NewUserStore はシードユーザ alice を登録したストアを返す。
func NewUserStore() *UserStore {
	alice := &User{
		// 学習用に固定のユーザハンドルを使う(本来はランダム生成して保存する)。
		id:       []byte("alice-webauthn-handle"),
		Username: "alice",
		Email:    "alice@example.com",
		Password: "password123",
	}
	return &UserStore{
		byName:  map[string]*User{alice.Username: alice},
		byEmail: map[string]*User{alice.Email: alice},
	}
}

// ByName はユーザ名からユーザを取得する。
func (s *UserStore) ByName(name string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byName[name]
	return u, ok
}

// ByEmail はメールアドレスからユーザを取得する。
func (s *UserStore) ByEmail(email string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byEmail[email]
	return u, ok
}

// VerifyPassword は第1要素(パスワード)を照合する。
// 学習用に平文比較(本来は bcrypt.CompareHashAndPassword を使う)。
func (s *UserStore) VerifyPassword(name, password string) bool {
	u, ok := s.ByName(name)
	if !ok {
		return false
	}
	return u.Password == password
}

// AddCredential は WebAuthn の登録資格情報を追加する。
func (u *User) AddCredential(c webauthn.Credential) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.credentials = append(u.credentials, c)
}

// HasCredential は登録済みの資格情報があるかを返す。
func (u *User) HasCredential() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.credentials) > 0
}

// --- webauthn.User インターフェースの実装 ---

// WebAuthnID はユーザを一意に識別する安定したバイト列を返す。
func (u *User) WebAuthnID() []byte { return u.id }

// WebAuthnName はユーザ名(ハンドル)を返す。
func (u *User) WebAuthnName() string { return u.Username }

// WebAuthnDisplayName は表示名を返す。
func (u *User) WebAuthnDisplayName() string { return u.Username }

// WebAuthnCredentials は登録済みの資格情報を返す。
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.credentials
}
