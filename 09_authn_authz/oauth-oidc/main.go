package main

import (
	"log"
	"net/http"
	"os"
)

// defaultIssuer は単一オリジン構成の基底 URL(ホスト側で 9100 にマップする想定)。
const defaultIssuer = "http://localhost:9100"

func main() {
	issuer := os.Getenv("ISSUER")
	if issuer == "" {
		issuer = defaultIssuer
	}

	keys := NewKeyMaterial()
	store := NewStore(issuer)
	// RP は自分自身(同一オリジン)の /token や /api/me を呼ぶため http.Client を使う。
	router := setupRouter(issuer, keys, store, http.DefaultClient)

	addr := ":8080"
	log.Printf("OAuth2.0/OIDC デモ起動: %s (issuer=%s)", addr, issuer)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}
