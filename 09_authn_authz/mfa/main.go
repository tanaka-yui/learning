package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// newApp はデモ用の依存を初期化して app を返す。
func newApp() (*app, error) {
	wa, err := webauthn.New(&webauthn.Config{
		RPID:          "localhost",
		RPDisplayName: "MFA Demo",
		RPOrigins:     []string{"http://localhost:9300"},
	})
	if err != nil {
		return nil, err
	}
	return &app{
		users:      NewUserStore(),
		sessions:   NewSessionStore(),
		magic:      NewMagicStore(10 * time.Minute),
		waSessions: NewWebAuthnSessionStore(),
		wa:         wa,
		baseURL:    "http://localhost:9300",
		mailHost:   mailpitHost(),
	}, nil
}

func main() {
	a, err := newApp()
	if err != nil {
		log.Fatalf("初期化に失敗しました: %v", err)
	}
	addr := ":8080"
	log.Printf("MFA/パスワードレス デモ起動: %s (SMTP送信先=%s)", addr, a.mailHost)
	if err := http.ListenAndServe(addr, setupRouter(a)); err != nil {
		log.Fatal(err)
	}
}
