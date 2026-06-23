# サービス間認証 (M2M) — API Key と mTLS

## 概要

M2M (Machine-to-Machine) 認証とは、ユーザが介在しないサービス間通信において、呼び出し元を正当なサービスとして識別・承認する仕組みです。

### 認証方式の使い分け

| 方式 | 特徴 | 適する場面 |
|------|------|------------|
| **API Key** | シンプル。ヘッダにキーを付与するだけ。 | 信頼境界が明確な社内サービス・簡易連携 |
| **mTLS** | 双方向 TLS 証明書検証。鍵を共有しない。 | ゼロトラスト環境・高セキュリティが必要な場面 |
| **OAuth 2.0 Client Credentials** | 短命トークン。認可サーバが介在。 | 外部公開 API・多数のクライアントを管理する場面 |

> Client Credentials フローは `oauth` デモの認可サーバ (AS) を参照してください。

---

## 仕組み

### API Key — 定数時間比較

クライアントは `X-API-Key` ヘッダまたは `Authorization: Bearer <key>` にキーを付与します。
サーバ側はシード済みキーの全件を `crypto/subtle.ConstantTimeCompare` で照合します。

```
クライアント          サーバ
  |                    |
  |--- X-API-Key: K -->|
  |                    | for k, v in seededAPIKeys {
  |                    |   ConstantTimeCompare(k, K) == 1 ?
  |                    | }
  |<-- 200 {client} ---|  (一致) or 401 (不一致)
```

通常の文字列比較は最初の不一致バイトで早期リターンするため、応答時間からキーの前方一致長を推測できます（タイミング攻撃）。`ConstantTimeCompare` は比較時間がキーの長さのみに依存し、内容に依存しないため、この攻撃を防ぎます。

### mTLS — 相互証明書検証

TLS ハンドシェイクでサーバとクライアントが**互いに**証明書を提示し、共通の CA が署名していることを検証します。

```
クライアント                      サーバ
  |                                |
  |--- ClientHello --------------->|
  |<-- ServerHello, server.crt ----|
  |    (サーバ証明書を検証)          |
  |--- client.crt, Finished ------>|
  |    (クライアント証明書を検証)    |
  |<-- Finished -------------------|
  |                                |
  |--- GET /mtls/data ------------>|
  |<-- 200 {client_cn} ------------|  CN を抽出して応答
```

サーバは `tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: <CAプール>}` を設定します。クライアント証明書が存在しない場合、または CA に署名されていない場合はハンドシェイクが失敗し、HTTP レイヤーに到達しません。

---

## デモ起動

### 証明書の生成

```bash
make gen-certs
# または直接
cd 09_authn_authz/api-m2m && ./gen-certs.sh
```

`api-m2m/certs/` に以下が生成されます: `ca.crt`, `ca.key`, `server.crt`, `server.key`, `client.crt`, `client.key`

### サーバの起動

```bash
make api-m2m
```

- API Key サーバ: `http://localhost:9400`
- mTLS サーバ: `https://localhost:9401`

---

## 動作確認

### API Key 認証

```bash
# X-API-Key ヘッダ: 成功
curl -s http://localhost:9400/api/data \
  -H "X-API-Key: key-service-a-secret-1234"
# => {"client":"service-a","data":"protected resource"}

# Bearer トークン形式: 成功
curl -s http://localhost:9400/api/data \
  -H "Authorization: Bearer key-service-b-secret-5678"
# => {"client":"service-b","data":"protected resource"}

# キーなし: 失敗
curl -s -o /dev/null -w "%{http_code}" http://localhost:9400/api/data
# => 401
```

### mTLS 認証

```bash
CERTS=09_authn_authz/api-m2m/certs

# クライアント証明書付き: 成功
curl -s https://localhost:9401/mtls/data \
  --cacert ${CERTS}/ca.crt \
  --cert   ${CERTS}/client.crt \
  --key    ${CERTS}/client.key
# => {"client_cn":"m2m-client","data":"mtls protected resource"}

# クライアント証明書なし: TLS ハンドシェイクエラー
curl -s https://localhost:9401/mtls/data \
  --cacert ${CERTS}/ca.crt
# => curl: (56) OpenSSL SSL_read: error:...
```

---

## コード解説

### `apikey.go` — API Key ミドルウェア

```go
func lookupAPIKey(provided string) (string, bool) {
    providedBytes := []byte(provided)
    found := false
    var clientName string
    for k, v := range seededAPIKeys {
        if subtle.ConstantTimeCompare([]byte(k), providedBytes) == 1 {
            found = true
            clientName = v
        }
    }
    return clientName, found
}
```

ポイント:
- 一致しても `break` しない: 全件を走査することで、キーの存在有無に関わらず一定の処理時間を確保します。
- `extractAPIKey` で `X-API-Key` ヘッダと `Authorization: Bearer` を両対応します。

### `mtls.go` — mTLS サーバ設定

```go
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{serverCert},
    ClientAuth:   tls.RequireAndVerifyClientCert,
    ClientCAs:    caPool,
}
```

- `ClientAuth: tls.RequireAndVerifyClientCert`: クライアント証明書を**必須**かつ**CA 検証**します。
- `CERTS_DIR` 環境変数で証明書ディレクトリを指定。証明書が存在しない場合は mTLS サーバをスキップし、API Key サーバのみ動作します。

### `main.go` — 単一バイナリで両サーバを起動

`startMTLSServer` をゴルーチンで起動後、メインゴルーチンで API Key サーバを `http.ListenAndServe` で起動します。

---

## まとめ

| 実装ポイント | 内容 |
|------------|------|
| API Key 定数時間比較 | `crypto/subtle.ConstantTimeCompare` でタイミング攻撃を防止 |
| 両ヘッダ対応 | `X-API-Key` と `Authorization: Bearer` を `extractAPIKey` で統一処理 |
| mTLS 相互検証 | `tls.RequireAndVerifyClientCert` + CA プールで TLS レイヤーで認証 |
| 証明書なし時のグレースフルスキップ | `startMTLSServer` は証明書未生成時にエラーではなくログを出して継続 |
| 単一バイナリ | `main.go` が両サーバを起動。Dockerfile もシンプルに保てる |

**次のステップ**: OAuth 2.0 Client Credentials フロー (`oauth` デモ) では、認可サーバが短命アクセストークンを発行し、API キーのローテーション問題を解消する方法を学びます。
