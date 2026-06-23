package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	// 証明書ディレクトリ (デフォルト: /certs)
	certsDir := os.Getenv("CERTS_DIR")
	if certsDir == "" {
		certsDir = "/certs"
	}

	// mTLSサーバを起動 (証明書がなければスキップ)
	startMTLSServer(certsDir)

	// APIキーサーバを起動
	addr := ":8080"
	log.Printf("APIキー認証デモ起動: %s", addr)
	if err := http.ListenAndServe(addr, setupRouter()); err != nil {
		log.Fatal(err)
	}
}
