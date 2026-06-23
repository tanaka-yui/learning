# OpenID Connect(OIDC)

## 1. 概要

OpenID Connect(OIDC)は **OAuth 2.0 の上に「認証層」を載せた** プロトコルである。前章 `03_oauth.md` で見たとおり OAuth 2.0 は「権限の委譲(認可)」を扱うが、「このユーザは誰なのか」を標準的に伝える仕組みは持たない。OIDC はそこに **ID Token** という標準化された「本人の身元証明」を追加する。

ひとことで言えば:

```
OIDC = OAuth 2.0(認可) + 認証(誰がログインしたかを ID Token で伝える)
```

OAuth だけを認証に流用すると、「アクセストークンを持っている = 本人」と誤解しがちで危険である。アクセストークンは「リソースへアクセスする鍵」であって「本人の身元証明」ではない。OIDC はこの混同を、用途の異なる 2 種類のトークンに明確に分けることで解消する。

| トークン | 用途 | 受け手(audience) | 中身を見る主体 |
|----------|------|--------------------|----------------|
| アクセストークン | リソースへのアクセス(認可) | リソースサーバ(RS) | RS |
| ID Token | 本人の身元証明(認証) | クライアント(RP) | クライアント |

## 2. OIDC を有効にする `openid` スコープ

OIDC は OAuth の Authorization Code フローに乗る。クライアントが認可リクエストの `scope` に **`openid`** を含めると、トークンエンドポイントは通常のアクセストークンに加えて **ID Token** を返す。`profile` や `email` を追加すると、名前やメールなどの標準クレームが ID Token / UserInfo に含まれる。

本デモの RP は `scope=openid profile email` を要求する(`rp.go`)。AS は `openid` を検出したときだけ ID Token を発行する(`as.go`)。

## 3. ID Token とクレーム

ID Token は **JWT(JSON Web Token)** で表現され、署名(本デモは RS256)が付く。代表的なクレームは次のとおり。

| クレーム | 意味 | 本デモの値 |
|----------|------|-----------|
| `iss` | 発行者(issuer)。AS の URL | `http://localhost:9100` |
| `sub` | 本人の一意な識別子(subject) | `user-alice` |
| `aud` | 受け手(audience)。**クライアントID** | `demo-web-app` |
| `exp` | 有効期限(UNIX 時刻) | iat + 5 分 |
| `iat` | 発行時刻 | 発行時 |
| `nonce` | リプレイ防止の使い捨て値 | RP が生成した値 |
| `name` / `email` | プロフィール情報(scope 次第) | `Alice Example` / `alice@example.com` |

`aud` が **クライアントID** である点が重要である。アクセストークンの `aud` がリソースサーバを指すのと対照的で、これにより「この ID Token は自分(クライアント)宛だ」とクライアントが確認できる。

### nonce(リプレイ防止)

`nonce` はクライアントが認可リクエストごとに生成する使い捨ての乱数である。AS はそれを ID Token にそのまま埋め込み、クライアントは「自分が送った nonce と ID Token の nonce が一致するか」を検証する。これにより、過去に発行された ID Token を攻撃者が使い回す **リプレイ攻撃** を防げる。state が「認可レスポンス全体の CSRF 対策」なのに対し、nonce は「ID Token がこのログイン試行のために発行されたものか」を保証する。

## 4. Discovery と JWKS

### Discovery(.well-known)

クライアントは AS の設定を事前にハードコードする必要がない。`{issuer}/.well-known/openid-configuration` を取得すると、エンドポイント一覧や対応アルゴリズムが JSON で得られる。

```bash
curl -s http://localhost:9100/.well-known/openid-configuration | python3 -m json.tool
```

```json
{
  "issuer": "http://localhost:9100",
  "authorization_endpoint": "http://localhost:9100/authorize",
  "token_endpoint": "http://localhost:9100/token",
  "jwks_uri": "http://localhost:9100/jwks.json",
  "userinfo_endpoint": "http://localhost:9100/userinfo",
  "response_types_supported": ["code"],
  "grant_types_supported": ["authorization_code", "refresh_token", "client_credentials"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "scopes_supported": ["openid", "profile", "email"],
  "code_challenge_methods_supported": ["S256"]
}
```

### JWKS(公開鍵の配布)

ID Token / アクセストークンは AS の **秘密鍵** で署名される。検証側(クライアントやリソースサーバ)は AS の **公開鍵** が必要だが、これを `jwks_uri`(`/jwks.json`)から取得できる。

```bash
curl -s http://localhost:9100/jwks.json | python3 -m json.tool
```

```json
{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "demo-key-1",
      "alg": "RS256",
      "n": "<base64url のモジュラス>",
      "e": "AQAB"
    }
  ]
}
```

`kid`(Key ID)で鍵を識別する。トークンのヘッダにも `kid` が入っており、複数鍵があるときはこれで正しい鍵を選ぶ。鍵ローテーション(古い鍵と新しい鍵を一時的に併置する)もこの仕組みで成り立つ。本デモは起動時に RSA 鍵を 1 つ生成し、`n`(モジュラス)と `e`(公開指数)を base64url で公開している(`keys.go`)。

## 5. UserInfo エンドポイント

`/userinfo` は **アクセストークン** を Bearer で提示すると本人のクレーム(sub/name/email)を返す OIDC のエンドポイントである。ID Token に含めるクレームを最小限にしておき、必要になったら UserInfo で追加取得する、という使い分けができる。

```bash
curl -s http://localhost:9100/userinfo -H "Authorization: Bearer $ACCESS" | python3 -m json.tool
```

注意: UserInfo は **アクセストークン** で認可される(ID Token ではない)。ID Token はクライアント宛の身元証明、アクセストークンはリソース(UserInfo を含む)へのアクセス鍵、という役割分担がここでも一貫している。

## 6. セッション章・JWT との関係

本リポジトリの 9 章では認証方式を段階的に扱っている。OIDC はそれらの組み合わせとして理解できる。

- **`01_session.md`(セッション認証)**: OIDC でログインが完了した後、RP は **自前のセッション Cookie** を発行してログイン状態を保持する。本デモの `/app/callback` は ID Token 検証後に `rp_session` Cookie を立てる。つまり「ログインの方式が OIDC」「ログイン後の状態保持がサーバ側セッション」という二段構えである。
- **JWT 章**: ID Token もアクセストークンも実体は **JWT** である。JWT 章で学んだ「署名・クレーム・検証」の知識がそのまま OIDC のトークン検証に活きる。OIDC は「誰がどの鍵で署名し、どのクレームを必須にするか」を標準化したものと言える。

OIDC のアクセストークンはステートレス(JWT)なので失効が難しいというトレードオフは JWT 章と同じである。本デモが TTL を 5 分と短くしているのはそのためで、長期のアクセスはリフレッシュトークンで更新する設計になっている。

## 7. コード解説

### tokens.go — ID Token の生成

```go
func (k *KeyMaterial) newIDToken(issuer, clientID, sub, nonce, name, email string, ttl time.Duration) (string, error) {
    now := time.Now()
    claims := IDTokenClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    issuer,
            Subject:   sub,
            Audience:  jwt.ClaimStrings{clientID}, // aud = クライアントID
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
        },
        Nonce: nonce,
        Name:  name,
        Email: email,
    }
    return signRS256(k.priv, k.kid, claims)
}
```

`golang-jwt/jwt/v5` の `RegisteredClaims` を埋め込み、標準クレーム(iss/sub/aud/iat/exp)を型安全に設定する。`aud` を **クライアントID** にしている点が ID Token の肝である。`signRS256` は `kid` をヘッダに入れて RS256 で署名する。

### tokens.go — RP 側の ID Token 検証

```go
func (k *KeyMaterial) verifyIDToken(tokenStr, issuer, clientID, expectedNonce string) (*IDTokenClaims, error) {
    claims := &IDTokenClaims{}
    _, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
        return &k.priv.PublicKey, nil
    },
        jwt.WithValidMethods([]string{"RS256"}),
        jwt.WithIssuer(issuer),
        jwt.WithAudience(clientID),
        jwt.WithExpirationRequired(),
    )
    if err != nil {
        return nil, err
    }
    if claims.Nonce != expectedNonce {
        return nil, fmt.Errorf("nonce が一致しません: got %q want %q", claims.Nonce, expectedNonce)
    }
    return claims, nil
}
```

ID Token 検証は次の 5 点を満たす必要がある。本デモはそれを golang-jwt のオプションと nonce チェックで実装している。

1. **署名**: AS の公開鍵で RS256 署名を検証(`WithValidMethods` でアルゴリズム固定 = `alg=none` 攻撃の防止)。
2. **iss**: 期待する issuer と一致(`WithIssuer`)。
3. **aud**: 自分のクライアントIDが含まれる(`WithAudience`)。
4. **exp**: 有効期限内、かつ exp が必須(`WithExpirationRequired`)。
5. **nonce**: 認可リクエスト時に送った値と一致。

> **単一オリジン構成での簡略化:** 本デモは AS・RP・RS が同一プロセスに同居するため、検証側は `jwks_uri` を HTTP で取得する代わりに同じ `KeyMaterial` の公開鍵を直接参照している。実運用では RP/RS は `/jwks.json` を取得し、`kid` で鍵を選んで検証する。テスト(`helper_test.go` の `jwkToRSAPublicKey`)では、あえて JWKS の `n`/`e` から `rsa.PublicKey` を復元して検証し、実機の経路を再現している。

### rp.go — コールバックでの検証とセッション確立

```go
func (rp *RP) handleCallback(w http.ResponseWriter, r *http.Request) {
    // ...
    pa, ok := rp.pending.Take(state)
    if !ok {
        http.Error(w, "state が一致しません(CSRF の疑い)", http.StatusBadRequest)
        return
    }
    // /token へ code + code_verifier を送って交換(PKCE)
    tokenResp, err := rp.postForm(rp.issuer+"/token", form)
    // ...
    // ID Token を検証(署名・iss・aud=client・exp・nonce)
    claims, err := rp.keys.verifyIDToken(idToken, rp.issuer, rp.clientID, pa.Nonce)
    if err != nil {
        http.Error(w, "id_token 検証失敗: "+err.Error(), http.StatusUnauthorized)
        return
    }
    // 検証済みクレームでセッションを確立し、Cookie を発行
    sid := randToken()
    rp.sessions.Set(sid, &RPSession{Sub: claims.Subject, Name: claims.Name, Email: claims.Email, AccessToken: accessToken})
    http.SetCookie(w, &http.Cookie{Name: rpCookieName, Value: sid, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
    http.Redirect(w, r, "/app/", http.StatusFound)
}
```

`state` を保存・照合して CSRF を防ぎ(`pending.Take` は単回使用)、コードを `code_verifier` 付きで交換し、ID Token を検証して初めてログイン成功とする。その後はセッション章と同じく `HttpOnly` + `SameSite=Lax` の Cookie でログイン状態を保持する。OIDC は「ログインのやり方」、Cookie セッションは「その後の状態保持」という役割分担になっている。

### as.go — Discovery と JWKS の公開

```go
func (a *AS) handleDiscovery(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusOK, map[string]any{
        "issuer":                                a.issuer,
        "authorization_endpoint":                a.issuer + "/authorize",
        "token_endpoint":                        a.issuer + "/token",
        "jwks_uri":                              a.issuer + "/jwks.json",
        "userinfo_endpoint":                     a.issuer + "/userinfo",
        "id_token_signing_alg_values_supported": []string{"RS256"},
        "code_challenge_methods_supported":      []string{"S256"},
        // ...
    })
}
```

エンドポイントの URL はすべて `issuer`(環境変数 `ISSUER`、既定 `http://localhost:9100`)を基底に組み立てる。これにより、ホスト名やポートが変わってもクライアントは Discovery を読むだけで追従できる。

## 8. まとめ

- **OIDC = OAuth 2.0 + 認証層**。OAuth が扱わない「誰がログインしたか」を **ID Token** で標準化する。
- **ID Token とアクセストークンは別物**。ID Token はクライアント宛の身元証明(`aud`=クライアントID)、アクセストークンはリソースへのアクセス鍵(`aud`=リソースサーバ)。
- **`openid` スコープ** で OIDC が有効になり、ID Token が追加発行される。
- **ID Token 検証の 5 点**: 署名(アルゴリズム固定)・iss・aud・exp・nonce。`alg=none` 攻撃や ID Token の使い回しを防ぐ。
- **Discovery / JWKS** によりクライアントは設定や公開鍵を動的に取得でき、鍵ローテーション(`kid`)にも対応できる。
- **UserInfo** はアクセストークンで本人クレームを追加取得するエンドポイント。
- OIDC でログインした後の状態保持は、**セッション章の Cookie** に接続する。トークン検証は **JWT 章** の知識がそのまま使える。OIDC はこれらを「標準化された認証フロー」として束ねたものである。
```
