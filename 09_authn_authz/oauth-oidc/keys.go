package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
)

// KeyMaterial は認可サーバの署名鍵(RSA)と公開用 kid を保持する。
// 起動時に一度だけ生成し、ID Token / access token の署名と JWKS 公開に使う。
type KeyMaterial struct {
	priv *rsa.PrivateKey
	kid  string
}

// NewKeyMaterial は 2048bit の RSA 鍵ペアを生成して返す(学習用にメモリ上のみ)。
func NewKeyMaterial() *KeyMaterial {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err) // 起動時の鍵生成失敗は継続不能
	}
	return &KeyMaterial{priv: priv, kid: "demo-key-1"}
}

// JWK は JWKS で公開する 1 件の公開鍵(RSA)を表す。
type JWK struct {
	Kty string `json:"kty"` // 鍵種別: RSA
	Use string `json:"use"` // 用途: sig(署名)
	Kid string `json:"kid"` // 鍵ID。トークンの kid ヘッダと突き合わせる
	Alg string `json:"alg"` // 署名アルゴリズム: RS256
	N   string `json:"n"`   // モジュラス(base64url)
	E   string `json:"e"`   // 公開指数(base64url)
}

// JWKS は JWK の集合(/jwks.json のレスポンス本体)。
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// PublicJWKS は公開鍵を JWKS 形式で返す。
// RSA 公開鍵の n(モジュラス)と e(指数)を base64url(パディング無し)で表現する。
func (k *KeyMaterial) PublicJWKS() JWKS {
	pub := &k.priv.PublicKey
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return JWKS{Keys: []JWK{{
		Kty: "RSA",
		Use: "sig",
		Kid: k.kid,
		Alg: "RS256",
		N:   n,
		E:   e,
	}}}
}
