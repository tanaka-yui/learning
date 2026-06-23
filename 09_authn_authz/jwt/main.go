package main

import (
	"log"
	"net/http"
)

func main() {
	signer := newSigner()
	blocklist := NewBlocklist()
	users := NewUserStore()
	addr := ":8080"
	log.Printf("JWT認証デモ起動: %s", addr)
	if err := http.ListenAndServe(addr, setupRouter(signer, blocklist, users)); err != nil {
		log.Fatal(err)
	}
}
