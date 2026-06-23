package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
)

// setupMTLSServer はmTLSサーバを構築して返す
// certsDir が空か証明書ファイルが存在しない場合は nil を返す
func setupMTLSServer(certsDir string) (*http.Server, error) {
	caCertPath := certsDir + "/ca.crt"
	serverCertPath := certsDir + "/server.crt"
	serverKeyPath := certsDir + "/server.key"

	for _, p := range []string{caCertPath, serverCertPath, serverKeyPath} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return nil, fmt.Errorf("証明書ファイルが見つかりません: %s", p)
		}
	}

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("CA証明書読み込み失敗: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("CA証明書のパース失敗")
	}

	serverCert, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
	if err != nil {
		return nil, fmt.Errorf("サーバ証明書読み込み失敗: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /mtls/data", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			// RequireAndVerifyClientCert が設定されているため通常到達しない
			http.Error(w, "クライアント証明書が必要です", http.StatusUnauthorized)
			return
		}
		cn := r.TLS.PeerCertificates[0].Subject.CommonName
		writeJSON(w, http.StatusOK, map[string]string{"client_cn": cn, "data": "mtls protected resource"})
	})

	srv := &http.Server{
		Addr:      ":8443",
		Handler:   mux,
		TLSConfig: tlsConfig,
	}
	return srv, nil
}

// startMTLSServer はmTLSサーバをゴルーチンで起動する
// 証明書が存在しない場合はスキップしてログに記録する
func startMTLSServer(certsDir string) {
	srv, err := setupMTLSServer(certsDir)
	if err != nil {
		log.Printf("mTLSサーバをスキップします: %v", err)
		return
	}
	log.Printf("mTLSサーバ起動: %s", srv.Addr)
	go func() {
		if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("mTLSサーバエラー: %v", err)
		}
	}()
}
