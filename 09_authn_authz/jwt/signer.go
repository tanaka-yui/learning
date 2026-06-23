package main

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Signer はHS256/RS256の両方に対応するトークン署名/検証の抽象
type Signer interface {
	// Sign はclaimsに署名してトークン文字列を返す
	Sign(claims jwt.Claims) (string, error)
	// Parse はトークン文字列を検証してClaimsを返す
	Parse(tokenString string) (*jwt.Token, error)
}

// hmacSigner はHS256署名を行う
type hmacSigner struct {
	secret []byte
}

func (s *hmacSigner) Sign(claims jwt.Claims) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(s.secret)
}

func (s *hmacSigner) Parse(tokenString string) (*jwt.Token, error) {
	return jwt.ParseWithClaims(tokenString, &tokenClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("予期しない署名アルゴリズム: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
}

// rsaSigner はRS256署名を行う
type rsaSigner struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func (s *rsaSigner) Sign(claims jwt.Claims) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return t.SignedString(s.privateKey)
}

func (s *rsaSigner) Parse(tokenString string) (*jwt.Token, error) {
	return jwt.ParseWithClaims(tokenString, &tokenClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("予期しない署名アルゴリズム: %v", t.Header["alg"])
		}
		return s.publicKey, nil
	})
}

// newHMACSigner は暗号学的乱数でHS256 Signerを生成する
func newHMACSigner() Signer {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		panic(fmt.Sprintf("秘密鍵生成失敗: %v", err))
	}
	return &hmacSigner{secret: secret}
}

// newRSASigner はRSA鍵ペアを生成してRS256 Signerを返す
func newRSASigner() Signer {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Sprintf("RSA鍵生成失敗: %v", err))
	}
	return &rsaSigner{privateKey: priv, publicKey: &priv.PublicKey}
}

// newSigner は環境変数 JWT_ALG に基づいて Signer を返す(デフォルトHS256)
func newSigner() Signer {
	alg := os.Getenv("JWT_ALG")
	if alg == "RS256" {
		return newRSASigner()
	}
	return newHMACSigner()
}

// tokenClaims はアクセストークン/リフレッシュトークン共通のカスタムクレーム
type tokenClaims struct {
	jwt.RegisteredClaims
	// TokenType はトークン種別("access" or "refresh")を識別する
	TokenType string `json:"typ_custom"`
}

// accessTTL はアクセストークンの有効期間
const accessTTL = 5 * time.Minute

// refreshTTL はリフレッシュトークンの有効期間
const refreshTTL = 24 * time.Hour

// newTokenID は暗号学的乱数からユニークなトークンIDを生成する
func newTokenID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", b)
}

// issueTokenPair はアクセストークンとリフレッシュトークンのペアを発行する
func issueTokenPair(signer Signer, username string) (accessToken, refreshToken string, err error) {
	now := time.Now()

	accessClaims := &tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
			ID:        newTokenID(),
		},
		TokenType: "access",
	}
	accessToken, err = signer.Sign(accessClaims)
	if err != nil {
		return
	}

	refreshClaims := &tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshTTL)),
			ID:        newTokenID(),
		},
		TokenType: "refresh",
	}
	refreshToken, err = signer.Sign(refreshClaims)
	return
}
