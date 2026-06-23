package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testCA はテスト用CA証明書とキーを保持する
type testCA struct {
	cert    *x509.Certificate
	certPEM []byte
	key     *ecdsa.PrivateKey
}

// newTestCA はテスト用CA証明書を生成する
func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("CA鍵生成失敗: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CA証明書生成失敗: %v", err)
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		t.Fatalf("CA証明書パース失敗: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return &testCA{cert: cert, certPEM: certPEM, key: key}
}

// issueServerCert はCA署名のサーバ証明書を生成する
func (ca *testCA) issueServerCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("サーバ鍵生成失敗: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatalf("サーバ証明書生成失敗: %v", err)
	}

	keyDer, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("サーバ鍵マーシャル失敗: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("サーバTLS証明書ペア生成失敗: %v", err)
	}
	return tlsCert
}

// issueClientCert はCA署名のクライアント証明書を生成する
func (ca *testCA) issueClientCert(t *testing.T, cn string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("クライアント鍵生成失敗: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatalf("クライアント証明書生成失敗: %v", err)
	}

	keyDer, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("クライアント鍵マーシャル失敗: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("クライアントTLS証明書ペア生成失敗: %v", err)
	}
	return tlsCert
}

// newMTLSTestServer はテスト用mTLSサーバを構築する
func newMTLSTestServer(t *testing.T, ca *testCA) *httptest.Server {
	t.Helper()
	serverCert := ca.issueServerCert(t)

	caPool := x509.NewCertPool()
	caPool.AddCert(ca.cert)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /mtls/data", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			http.Error(w, "クライアント証明書が必要です", http.StatusUnauthorized)
			return
		}
		cn := r.TLS.PeerCertificates[0].Subject.CommonName
		writeJSON(w, http.StatusOK, map[string]string{"client_cn": cn, "data": "mtls protected resource"})
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
	}
	srv.StartTLS()
	return srv
}

// TestMTLSWithValidClientCert は有効なクライアント証明書付きでアクセスすると200が返ることを検証する
func TestMTLSWithValidClientCert(t *testing.T) {
	ca := newTestCA(t)
	server := newMTLSTestServer(t, ca)
	defer server.Close()

	// CA署名のクライアント証明書を生成
	clientCert := ca.issueClientCert(t, "test-client")
	caPool := x509.NewCertPool()
	caPool.AddCert(ca.cert)

	// クライアント証明書付きHTTPSクライアント
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{clientCert},
				RootCAs:      caPool,
			},
		},
	}

	resp, err := client.Get(server.URL + "/mtls/data")
	if err != nil {
		t.Fatalf("mTLSリクエスト失敗: %v", err)
	}
	defer resp.Body.Close()

	// 有効なクライアント証明書は200を返す
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got map[string]string
	json.NewDecoder(resp.Body).Decode(&got)
	// クライアント証明書のCNが返る
	if got["client_cn"] != "test-client" {
		t.Errorf("client_cn = %q, want test-client", got["client_cn"])
	}
}

// TestMTLSWithoutClientCert はクライアント証明書なしのアクセスがTLSハンドシェイクエラーになることを検証する
func TestMTLSWithoutClientCert(t *testing.T) {
	ca := newTestCA(t)
	server := newMTLSTestServer(t, ca)
	defer server.Close()

	caPool := x509.NewCertPool()
	caPool.AddCert(ca.cert)

	// クライアント証明書なしのHTTPSクライアント
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				// Certificatesを設定しない = クライアント証明書なし
				RootCAs: caPool,
			},
		},
	}

	_, err := client.Get(server.URL + "/mtls/data")
	// クライアント証明書なしはTLSハンドシェイクエラーになる (200は返らない)
	if err == nil {
		t.Fatal("クライアント証明書なしでもエラーなし: mTLS検証が機能していない可能性")
	}
}

// TestMTLSWithUntrustedClientCert は別CAで署名されたクライアント証明書が拒否されることを検証する
func TestMTLSWithUntrustedClientCert(t *testing.T) {
	ca := newTestCA(t)
	server := newMTLSTestServer(t, ca)
	defer server.Close()

	// 別のCAでクライアント証明書を発行(サーバは信頼しない)
	untrustedCA := newTestCA(t)
	clientCert := untrustedCA.issueClientCert(t, "untrusted-client")

	caPool := x509.NewCertPool()
	caPool.AddCert(ca.cert)

	// 信頼されていないクライアント証明書
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{clientCert},
				RootCAs:      caPool,
			},
		},
	}

	_, err := client.Get(server.URL + "/mtls/data")
	// 信頼されていない証明書はTLSハンドシェイクで拒否される
	if err == nil {
		t.Fatal("信頼されていないクライアント証明書でもエラーなし: CA検証が機能していない可能性")
	}
}
