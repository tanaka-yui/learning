package main

import (
	"log"
	"net/http"
)

func main() {
	store := NewSessionStore()
	users := NewUserStore()
	addr := ":8080"
	log.Printf("session認証デモ起動: %s", addr)
	if err := http.ListenAndServe(addr, setupRouter(store, users)); err != nil {
		log.Fatal(err)
	}
}
