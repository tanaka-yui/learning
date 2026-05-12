package jwt

import (
	"errors"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func New(secret []byte, ttl time.Duration) *Manager {
	return &Manager{secret: secret, ttl: ttl}
}

func (m *Manager) Issue(userID string) (string, error) {
	now := time.Now()
	claims := jwtv5.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": now.Add(m.ttl).Unix(),
	}
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return tok.SignedString(m.secret)
}

func (m *Manager) Verify(tokenStr string) (string, error) {
	tok, err := jwtv5.Parse(tokenStr, func(t *jwtv5.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || !tok.Valid {
		return "", ErrInvalidToken
	}
	claims, ok := tok.Claims.(jwtv5.MapClaims)
	if !ok {
		return "", ErrInvalidToken
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", ErrInvalidToken
	}
	return sub, nil
}
