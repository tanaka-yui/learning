package main

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// IDTokenClaims は OIDC の ID Token に載せるクレーム。
// RegisteredClaims を埋め込み、iss/sub/aud/exp/iat を標準クレームとして扱う。
// nonce はリプレイ防止のために RP が生成し、ID Token に反映される。
type IDTokenClaims struct {
	jwt.RegisteredClaims
	Nonce string `json:"nonce,omitempty"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// AccessTokenClaims は Resource Server 向けのアクセストークン(JWT)のクレーム。
// aud を Resource Server (RS) に向け、scope を空白区切りで保持する。
type AccessTokenClaims struct {
	jwt.RegisteredClaims
	Scope    string `json:"scope,omitempty"`
	ClientID string `json:"client_id,omitempty"`
}

// signRS256 は claims を RS256 で署名し、kid ヘッダ付きの JWT 文字列を返す。
func signRS256(priv *rsa.PrivateKey, kid string, claims jwt.Claims) (string, error) {
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	return tok.SignedString(priv)
}

// newIDToken は ID Token を生成する。aud はクライアントID、iss は認可サーバ。
func (k *KeyMaterial) newIDToken(issuer, clientID, sub, nonce, name, email string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := IDTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   sub,
			Audience:  jwt.ClaimStrings{clientID},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Nonce: nonce,
		Name:  name,
		Email: email,
	}
	return signRS256(k.priv, k.kid, claims)
}

// newAccessToken はアクセストークン(JWT)を生成する。aud は Resource Server。
func (k *KeyMaterial) newAccessToken(issuer, audience, sub, clientID, scope string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := AccessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   sub,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Scope:    scope,
		ClientID: clientID,
	}
	return signRS256(k.priv, k.kid, claims)
}

// verifyAccessToken は Resource Server 側でアクセストークンを検証する。
// 署名(RS256 公開鍵)、iss、aud、exp を golang-jwt のオプションで検証する。
func (k *KeyMaterial) verifyAccessToken(tokenStr, issuer, audience string) (*AccessTokenClaims, error) {
	claims := &AccessTokenClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		return &k.priv.PublicKey, nil
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// verifyIDToken は RP(クライアント)側で ID Token を検証する。
// 署名・iss・aud(=クライアントID)・exp を検証し、nonce の一致も確認する。
func (k *KeyMaterial) verifyIDToken(tokenStr, issuer, clientID, expectedNonce string) (*IDTokenClaims, error) {
	claims := &IDTokenClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		return &k.priv.PublicKey, nil
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(clientID),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	if claims.Nonce != expectedNonce {
		return nil, fmt.Errorf("nonce が一致しません: got %q want %q", claims.Nonce, expectedNonce)
	}
	return claims, nil
}

// pkceS256Challenge は code_verifier から S256 の code_challenge を計算する。
// challenge = base64url( SHA-256( verifier ) )(パディング無し)。
func pkceS256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
