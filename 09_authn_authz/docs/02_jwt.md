# JWT認証(JSON Web Token)

## 1. 概要

JWT(JSON Web Token)は、認証情報をトークン自身に埋め込む方式。サーバは発行時に署名し、受け取ったトークンの署名を検証するだけで認証が完結する。セッションストアへのアクセスが不要なため「ステートレス検証」と呼ばれる。

### JWT構造 — header.payload.signature

JWTは`.`で区切られた3つのBase64urlエンコード済みパートから成る。

```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9   ← Header  (アルゴリズム情報)
.eyJzdWIiOiJhbGljZSIsImV4cCI6MTcwMDAwMH0  ← Payload (クレーム: sub, exp, iat, jti …)
.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c  ← Signature (秘密鍵で署名)
```

- **Header**: `{"alg":"HS256","typ":"JWT"}` のようなJSONをBase64urlエンコード
- **Payload**: `sub`(ユーザ)、`exp`(有効期限)、`iat`(発行時刻)、`jti`(トークンID) など
- **Signature**: `HMACSHA256(base64url(header) + "." + base64url(payload), secret)`

Payloadは署名されているが**暗号化されていない**。Base64urlでデコードすれば内容が見える。機密情報は含めないこと。

### セッション(01章)との対比

| 観点 | セッション認証 | JWT |
|------|--------------|-----|
| 状態の置き場 | サーバ(ストア) | トークン自身 |
| 検証方法 | ストアをルックアップ | 署名を検証 |
| 即時失効 | 即時に削除可能 | ブロックリストが必要 |
| 水平スケール | ストア共有(Redis等)が必要 | ステートレスで容易 |
| ペイロードの機密性 | セッションIDのみ流れる | Base64デコードで内容が見える |

JWTはステートレスなため複数サーバへのスケールアウトが容易だが、トークンの即時無効化には工夫が必要(→ ブロックリスト)。

## 2. 仕組み

### アクセストークンとリフレッシュトークン

本デモでは2種類のトークンを使い分ける。

| 種別 | 有効期間 | 用途 |
|------|---------|------|
| アクセストークン | 5分 | APIアクセスの認証 |
| リフレッシュトークン | 24時間 | 新しいトークンペアの取得 |

アクセストークンは短命にすることで、漏洩時の被害範囲を限定する。期限が切れたらリフレッシュトークンで再取得する。

### ローテーションと失効戦略

リフレッシュトークンは使い捨て。`POST /refresh` で新しいペアを発行すると同時に、古いリフレッシュトークンの `jti` をインメモリのブロックリストに追加する。これを「リフレッシュトークンローテーション」と呼ぶ。

```
クライアント                                  サーバ
    |                                           |
    |--- POST /login ─────────────────────────>|
    |                                           | 1. bcryptでパスワード照合
    |<── 200 { access_token, refresh_token } ───|  2. アクセス(5分) + リフレッシュ(24h) 発行
    |                                           |
    |--- GET /protected                         |
    |    Authorization: Bearer <access> ──────>|  3. 署名検証 + exp確認
    |<── 200 { username }  ─────────────────────|
    |                                           |
    |  (5分経過、アクセストークン期限切れ)      |
    |                                           |
    |--- POST /refresh                          |
    |    Authorization: Bearer <refresh> ─────>|  4. 署名検証 + blocklist確認
    |                                           |  5. 旧refresh jti → blocklist追加(再利用防止)
    |<── 200 { new_access, new_refresh } ───────|  6. 新トークンペア発行
    |                                           |
    |--- POST /logout                           |
    |    Authorization: Bearer <refresh> ─────>|  7. refresh jti → blocklist追加
    |<── 200 { status: "logged_out" } ──────────|
    |                                           |
    |--- POST /refresh (古いrefresh) ──────────>|  8. blocklist hit → 401
    |<── 401 Unauthorized  ──────────────────────|
```

## 3. デモ起動

```bash
# プロジェクトルート(09_authn_authz/)から実行
make jwt
```

起動後、ブラウザまたは curl で `http://localhost:9001` にアクセスするとエンドポイント一覧のHTMLが表示される。

テストユーザ:

| ユーザ名 | パスワード |
|----------|-----------|
| alice | password123 |
| bob | pass456 |

停止するには:

```bash
make down
```

## 4. 動作確認

以下の curl コマンドで一通りのフローを確認できる。

### ステップ 1: ログイン(トークンペアを取得する)

```bash
TOKENS=$(curl -s -X POST http://localhost:9001/login \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"password123"}')

ACCESS=$(echo $TOKENS | jq -r .access_token)
REFRESH=$(echo $TOKENS | jq -r .refresh_token)
echo "access=$ACCESS"
echo "refresh=$REFRESH"
```

### ステップ 2: /protected にアクセス(アクセストークンを使う)

```bash
curl -s http://localhost:9001/protected \
  -H "Authorization: Bearer $ACCESS"
# → {"username":"alice"}
```

### ステップ 3: トークンなしで /protected にアクセス(401 を確認)

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:9001/protected
# → 401
```

### ステップ 4: リフレッシュ(ローテーション確認)

```bash
NEW_TOKENS=$(curl -s -X POST http://localhost:9001/refresh \
  -H "Authorization: Bearer $REFRESH")

NEW_ACCESS=$(echo $NEW_TOKENS | jq -r .access_token)
NEW_REFRESH=$(echo $NEW_TOKENS | jq -r .refresh_token)

# 新しいアクセストークンで /protected にアクセス
curl -s http://localhost:9001/protected \
  -H "Authorization: Bearer $NEW_ACCESS"
# → {"username":"alice"}
```

### ステップ 5: 古いリフレッシュトークンが使えないことを確認

```bash
curl -s -o /dev/null -w "%{http_code}" \
  -X POST http://localhost:9001/refresh \
  -H "Authorization: Bearer $REFRESH"
# → 401 (旧トークンは失効済み)
```

### ステップ 6: ログアウト後にリフレッシュできないことを確認

```bash
curl -s -X POST http://localhost:9001/logout \
  -H "Authorization: Bearer $NEW_REFRESH"
# → {"status":"logged_out"}

curl -s -o /dev/null -w "%{http_code}" \
  -X POST http://localhost:9001/refresh \
  -H "Authorization: Bearer $NEW_REFRESH"
# → 401 (ログアウトで失効済み)
```

## 5. コード解説

### signer.go — 署名/検証の抽象化

HS256とRS256を切り替えられるよう `Signer` インタフェースで抽象化している。

```go
// Signer はHS256/RS256の両方に対応するトークン署名/検証の抽象
type Signer interface {
    Sign(claims jwt.Claims) (string, error)
    Parse(tokenString string) (*jwt.Token, error)
}
```

環境変数 `JWT_ALG=RS256` を設定するとRS256で起動する。RS256では起動時に `rsa.GenerateKey` で2048ビットの鍵ペアを生成し、秘密鍵で署名、公開鍵で検証する。

```go
func (s *rsaSigner) Parse(tokenString string) (*jwt.Token, error) {
    return jwt.ParseWithClaims(tokenString, &tokenClaims{}, func(t *jwt.Token) (any, error) {
        if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("予期しない署名アルゴリズム: %v", t.Header["alg"])
        }
        return s.publicKey, nil
    })
}
```

`keyFunc` の中でアルゴリズムの型チェックを行うことで、「alg: none」攻撃やアルゴリズム混在を防ぐ。

### handlers.go — 検証ミドルウェアとローテーション

アクセストークンの検証はミドルウェアとして実装する。

```go
// requireAccessToken はアクセストークンを要求するミドルウェア
func requireAccessToken(signer Signer, next func(http.ResponseWriter, *http.Request, *tokenClaims)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        tokenStr := bearerToken(r)
        if tokenStr == "" {
            http.Error(w, "未認証です", http.StatusUnauthorized)
            return
        }
        parsed, err := signer.Parse(tokenStr)
        if err != nil || !parsed.Valid {
            http.Error(w, "トークンが無効です", http.StatusUnauthorized)
            return
        }
        claims, ok := parsed.Claims.(*tokenClaims)
        if !ok || claims.TokenType != "access" {
            http.Error(w, "トークン種別が不正です", http.StatusUnauthorized)
            return
        }
        next(w, r, claims)
    }
}
```

リフレッシュトークンのローテーションは `POST /refresh` で行う。古いJTIをブロックリストに追加してから新しいペアを発行する。

```go
// 古いリフレッシュトークンのJTIを失効リストに追加(再利用防止)
blocklist.Revoke(claims.ID)

access, refresh, err := issueTokenPair(signer, claims.Subject)
```

### blocklist.go — インメモリ失効リスト

`jti`(JWT ID)をキーとするセットで失効済みトークンを追跡する。

```go
// Blocklist は失効済みトークンJTIを保持するインメモリセット
type Blocklist struct {
    mu      sync.RWMutex
    revoked map[string]struct{}
}
```

`sync.RWMutex` で並行アクセスに対応。読み込み専用の `IsRevoked` は `RLock`、書き込みの `Revoke` は `Lock` を使い分ける。

> **注:** インメモリ実装のため、サーバを再起動すると失効リストが消える。本番環境ではRedis等の共有ストアに保存すること。

## 6. まとめ

- **JWTはステートレス**: 署名検証だけで認証が完結し、ストアルックアップが不要。水平スケールが容易。
- **アクセストークンは短命に**: 漏洩時の被害期間を5分に限定する。
- **リフレッシュトークンローテーション**: 使い捨てにすることで、漏洩したリフレッシュトークンの再利用を検知できる。
- **ブロックリストで即時失効**: ステートレスのデメリット(即時失効困難)をJTIの失効リストで補う。リストが膨らむ問題は本番では有効期限付きエントリで対応する。
- **アルゴリズムを型チェック**: `keyFunc` 内でアルゴリズムの型を確認し、`alg: none`攻撃とアルゴリズム混在を防ぐ。
- **HS256 vs RS256**: HS256は高速だが秘密鍵の共有が必要。RS256は公開鍵配布だけで検証できるためマイクロサービス間に向く。
