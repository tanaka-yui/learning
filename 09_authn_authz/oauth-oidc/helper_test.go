package main

import (
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// jwkToRSAPublicKey は JWKS の 1 件(n, e)から rsa.PublicKey を復元する。
// これは RP/RS が jwks_uri から検証鍵を組み立てる処理を、テストで再現するもの。
func jwkToRSAPublicKey(j JWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, err
	}
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}

// parseAndVerifyIDToken は JWKS から復元した公開鍵で id_token の署名を検証し、
// クレームを MapClaims で返す。署名が不正なら t.Fatal する。
func parseAndVerifyIDToken(t *testing.T, idToken string, pub any) jwt.MapClaims {
	t.Helper()
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(idToken, claims, func(*jwt.Token) (any, error) {
		return pub, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		t.Fatalf("id_token の署名検証に失敗: %v", err)
	}
	return claims
}
