package main

import "golang.org/x/crypto/bcrypt"

// UserStore はテストユーザの認証情報を保持する(学習用シード)
type UserStore struct {
	hashes map[string]string // username -> bcryptハッシュ
}

// seedUsers はテスト用の平文パスワード(学習用途のため明示)
var seedUsers = map[string]string{
	"alice": "password123",
	"bob":   "pass456",
}

// NewUserStore はシードユーザのパスワードをbcryptでハッシュ化して保持する
func NewUserStore() *UserStore {
	hashes := make(map[string]string, len(seedUsers))
	for name, pw := range seedUsers {
		h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
		if err != nil {
			panic(err)
		}
		hashes[name] = string(h)
	}
	return &UserStore{hashes: hashes}
}

// Verify はユーザ名とパスワードの組が正しいか判定する
func (u *UserStore) Verify(username, password string) bool {
	h, ok := u.hashes[username]
	if !ok {
		// ユーザ不在でも比較を行いタイミング差を抑える
		bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinv"), []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(h), []byte(password)) == nil
}
