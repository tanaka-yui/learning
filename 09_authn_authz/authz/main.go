package main

import (
	"log"
	"net/http"
)

func main() {
	rbac, err := newRBACEnforcer()
	if err != nil {
		log.Fatalf("RBAC エンフォーサ初期化失敗: %v", err)
	}
	abac, err := newABACEnforcer()
	if err != nil {
		log.Fatalf("ABAC エンフォーサ初期化失敗: %v", err)
	}

	addr := ":8080"
	log.Printf("RBAC/ABAC認可デモ起動: %s", addr)
	if err := http.ListenAndServe(addr, setupRouter(rbac, abac)); err != nil {
		log.Fatal(err)
	}
}
