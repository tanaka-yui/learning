package main

import "golang.org/x/crypto/bcrypt"

// UserStore はテストユーザの認証情報を保持する(学習用シード)
type UserStore struct {
	hashes    map[string]string // username -> bcryptハッシュ
	dummyHash []byte            // ユーザ不在時の比較用ダミー(有効なbcryptハッシュ)
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
	// ユーザ不在時の比較に使う有効なダミーハッシュを生成しておく
	dummy, err := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return &UserStore{hashes: hashes, dummyHash: dummy}
}

// Verify はユーザ名とパスワードの組が正しいか判定する
func (u *UserStore) Verify(username, password string) bool {
	h, ok := u.hashes[username]
	if !ok {
		// ユーザ不在でも有効なハッシュと比較し、応答時間の差(ユーザ列挙)を抑える
		bcrypt.CompareHashAndPassword(u.dummyHash, []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(h), []byte(password)) == nil
}
