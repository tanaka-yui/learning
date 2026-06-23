# OAuth 2.0(認可フレームワーク)

## 1. 概要

OAuth 2.0 は「あるユーザの権限を、ユーザのパスワードを渡さずに第三者アプリへ委譲する」ための **認可(authorization)** の枠組みである。たとえば「写真印刷サービスに、自分の Google フォトの読み取りだけを許可する」といった場面で使う。印刷サービスに Google のパスワードを渡すのではなく、限定された権限を表す **アクセストークン** だけを渡すのが核心である。

OAuth 2.0 はあくまで **認可** の仕組みであり、「このユーザは誰か」という **認証(authentication)** を標準化したものではない点に注意する。認証層を上に載せたものが OpenID Connect(OIDC)で、次章 `04_oidc.md` で扱う。

### 登場人物(ロール)

| ロール | 英語 | 役割 | 本デモでの対応 |
|--------|------|------|----------------|
| リソースオーナー | Resource Owner (RO) | 権限を持つ本人(ユーザ) | シードユーザ `alice` |
| クライアント | Client | RO の代理でリソースにアクセスするアプリ | RP(`/app/*`) |
| 認可サーバ | Authorization Server (AS) | RO を認証しトークンを発行する | `/authorize`, `/token` |
| リソースサーバ | Resource Server (RS) | アクセストークンを検証し保護リソースを返す | `/api/me` |

「クライアント」はエンドユーザのブラウザではなく、**ユーザの代わりに動くアプリケーション** を指す点が紛らわしいので注意する。

## 2. 4 つのグラント(付与方式)の概観

OAuth 2.0 にはアクセストークンを得る経路(グラントタイプ)が複数ある。

| グラント | 用途 | 現在の推奨度 |
|----------|------|--------------|
| Authorization Code(+ PKCE) | Web/モバイル/SPA など、ユーザが介在するアプリ全般 | **推奨**。PKCE 併用が必須級 |
| Client Credentials | サーバ間(M2M)。ユーザが介在しない | 推奨(該当用途で) |
| Implicit | 旧来の SPA 向け。トークンを URL フラグメントで直接返す | **非推奨**(後述) |
| Resource Owner Password Credentials (ROPC) | アプリがユーザの ID/PW を直接受け取る | **非推奨**。委譲の意味が消える |

現在のベストプラクティス(OAuth 2.1 の方向性)では、ユーザが介在するフローは **Authorization Code + PKCE に一本化** し、Implicit と ROPC は使わない。本デモも Authorization Code + PKCE と Client Credentials の 2 つだけを実装している。

## 3. Authorization Code + PKCE フロー

### PKCE とは

PKCE(Proof Key for Code Exchange、「ピクシー」と読む)は、認可コードの横取り攻撃を防ぐ仕組みである。クライアントは毎回ランダムな `code_verifier` を生成し、その SHA-256 ハッシュ(`code_challenge`)を認可リクエストに載せる。トークン交換時には元の `code_verifier` を提示し、AS は「保存しておいた challenge」と「verifier から計算し直した challenge」が一致するかを検証する。

```
code_verifier  : ランダムな高エントロピー文字列(クライアントだけが知る)
code_challenge : base64url( SHA-256( code_verifier ) )   ← method=S256
```

認可コードを盗んだ攻撃者は、対応する `code_verifier` を知らないためトークン交換に失敗する。これにより、リダイレクトや OS のカスタムスキームでコードが漏れても被害を防げる。

### フロー全体(ASCII 図)

```
ブラウザ              クライアント(RP)        認可サーバ(AS)        リソースサーバ(RS)
  |                       |                        |                       |
  |--- GET /app/login -->|                        |                       |
  |                       | 1. state / nonce 生成   |                       |
  |                       | 2. code_verifier 生成   |                       |
  |                       |    challenge=S256(v)    |                       |
  |<-- 302 /authorize?... |                        |                       |
  |    client_id, redirect_uri, response_type=code,|                       |
  |    scope, state, code_challenge, S256          |                       |
  |                       |                        |                       |
  |------------- GET /authorize?... ------------->|                       |
  |                       |                        | 3. RO を認証/同意取得  |
  |<------------ 同意画面(approve) --------------|                       |
  |------------- POST /authorize (approve) ------>|                       |
  |                       |                        | 4. 認可コード発行       |
  |                       |                        |    (challenge/redirect/|
  |                       |                        |     nonce を束縛, 単回) |
  |<-- 302 /app/callback?code=...&state=... ------|                       |
  |                       |                        |                       |
  |--- GET /app/callback?code,state -->|          |                       |
  |                       | 5. state 検証(CSRF)  |                       |
  |                       |--- POST /token ------->|                       |
  |                       |    grant_type=          | 6. code 単回消費       |
  |                       |     authorization_code, |    S256(verifier)==     |
  |                       |    code, code_verifier, |       challenge ?       |
  |                       |    redirect_uri,        |                       |
  |                       |    client_id            |                       |
  |                       |<-- access_token,        |                       |
  |                       |    id_token,            |                       |
  |                       |    refresh_token -------|                       |
  |                       | 7. id_token 検証(後述) |                       |
  |<-- 302 /app/ (Cookie) |                        |                       |
  |                       |                        |                       |
  |--- GET /app/ ------->|                        |                       |
  |                       |--- GET /api/me (Bearer access_token) --------->|
  |                       |                        |  8. 署名/iss/aud/exp 検証
  |                       |<------------------ 200 {sub, scope, ...} ------|
  |<-- ユーザ情報 + /api/me 応答 |                  |                       |
```

ポイントは次の 4 つ。

1. **アクセストークンがブラウザを通らない**: トークンはクライアント(サーバ)と AS の `/token` 通信の中だけで授受される(ステップ 6)。ブラウザの URL に乗るのは「使い捨ての認可コード」だけ。
2. **認可コードは単回・短命・束縛**: コードは `client_id` / `redirect_uri` / PKCE challenge / nonce に束縛され、1 回使うと無効化される。
3. **state で CSRF を防ぐ**: クライアントは `/authorize` 前に `state` を保存し、`/callback` で一致を確認する(ステップ 5)。一致しないコールバックは攻撃の疑いとして拒否する。
4. **PKCE でコード横取りを防ぐ**: 上記のとおり verifier を知らない攻撃者は交換できない(ステップ 6)。

## 4. なぜ Implicit は非推奨か

Implicit グラントは、認可コードを経由せずアクセストークンを **リダイレクト URL のフラグメント(`#access_token=...`)で直接ブラウザに返す** 方式だった。次の理由で現在は使わない。

- **トークンが URL に露出する**: ブラウザ履歴、Referer、ログにトークンが残りうる。
- **横取りに弱い**: フラグメントは JavaScript から読めるため、XSS や悪意あるスクリプトに奪われやすい。
- **送信元の確証がない**: クライアント認証や PKCE のような「正規のクライアントが交換した」という保証がない。

現在は SPA でも **Authorization Code + PKCE** を使う。これにより、トークンはバックチャネル(`/token`)で受け取られ、ブラウザの URL には短命な認可コードしか出ない。

## 5. scope(権限の範囲)

`scope` は「クライアントが要求する権限の範囲」をスペース区切りで表す。AS は同意画面で scope をユーザに提示し、発行するトークンに反映する。リソースサーバは scope を見てアクセス可否を判断できる。

本デモでは認可リクエストで `scope=openid profile email` を要求する。`openid` は OIDC を有効化する特別なスコープ(次章参照)で、これがあると ID Token が追加で発行される。発行されたアクセストークンの scope は `/api/me` のレスポンスにそのまま現れる。

## 6. Client Credentials(M2M)

ユーザが介在しない **サーバ間通信(machine-to-machine)** では、クライアント自身がリソースの主体になる。バッチ処理やマイクロサービス間呼び出しが典型例である。

```
サービスA(confidential client)            認可サーバ(AS)
   |--- POST /token --------------------->|
   |    grant_type=client_credentials      |
   |    client_id, client_secret           | client_secret を検証
   |    scope=api.read                     |
   |<-- access_token (id_token は無し) ----|
   |                                       |
   |--- GET /api/me (Bearer access_token) ----> リソースサーバ
```

このフローには本人(RO)が居ないため **ID Token は発行されない**(認証する「人」が居ないため)。クライアントは事前に登録された `client_secret` で自身を認証する。本デモでは confidential クライアント `demo-service` が `client_secret_post`(または Basic 認証)で認証し、アクセストークンのみを受け取る。

## 7. デモ起動手順

```bash
# プロジェクトルート(09_authn_authz/)から実行
make oauth-oidc
```

起動後、ブラウザで次にアクセスする。

| URL | 内容 |
|-----|------|
| http://localhost:9100/ | 入口ページ(各エンドポイントへのリンク) |
| http://localhost:9100/app/login | Authorization Code + PKCE でログイン開始 |
| http://localhost:9100/.well-known/openid-configuration | OIDC Discovery |
| http://localhost:9100/jwks.json | 署名検証用の公開鍵(JWKS) |

`/app/login` を開くと認可サーバの同意画面に遷移する。「Alice として承認する」を押すと、コード交換と ID Token 検証を経てログインが完了し、`/app/` にユーザ情報と `/api/me` の応答が表示される。

停止するには:

```bash
make down
```

> **設計メモ:** 本デモは **1 つのバイナリ・1 つのポート・1 つのオリジン**(`http://localhost:9100`)に AS・RP・RS をパスで同居させている。`issuer`・`jwks_uri`・リダイレクト URI がすべて同一オリジンになるため、「コンテナ内部 URL とブラウザから見える URL の食い違い」という OAuth デモでよくハマる問題を避けられる。基底 URL は環境変数 `ISSUER` で差し替え可能。

## 8. 動作確認手順(curl)

ブラウザ無しでも、curl でフローの中核を再現できる。

### ステップ 1: Discovery と JWKS を確認する

```bash
curl -s http://localhost:9100/.well-known/openid-configuration | python3 -m json.tool
curl -s http://localhost:9100/jwks.json | python3 -m json.tool
```

### ステップ 2: PKCE の verifier / challenge を用意する

```bash
VERIFIER="dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
# challenge = base64url(SHA-256(verifier)) パディング無し
CHALLENGE=$(printf '%s' "$VERIFIER" | openssl dgst -binary -sha256 | openssl base64 | tr '+/' '-_' | tr -d '=')
echo "challenge=$CHALLENGE"   # → E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM
```

### ステップ 3: 認可コードを取得する(同意フォームを POST)

```bash
# -i で Location ヘッダから code を取り出す
curl -si -X POST http://localhost:9100/authorize \
  --data-urlencode "client_id=demo-web-app" \
  --data-urlencode "redirect_uri=http://localhost:9100/app/callback" \
  --data-urlencode "response_type=code" \
  --data-urlencode "scope=openid profile email" \
  --data-urlencode "state=xyz" \
  --data-urlencode "nonce=n-123" \
  --data-urlencode "code_challenge=$CHALLENGE" \
  --data-urlencode "code_challenge_method=S256" \
  --data-urlencode "approve=yes" | grep -i location
# → Location: http://localhost:9100/app/callback?code=<CODE>&state=xyz
```

### ステップ 4: コードをトークンに交換する

```bash
CODE="<上で得た code>"
curl -s -X POST http://localhost:9100/token \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "code=$CODE" \
  --data-urlencode "code_verifier=$VERIFIER" \
  --data-urlencode "client_id=demo-web-app" \
  --data-urlencode "redirect_uri=http://localhost:9100/app/callback" | python3 -m json.tool
# → access_token / id_token / refresh_token が返る
```

### ステップ 5: アクセストークンで保護 API を叩く

```bash
ACCESS="<access_token>"
curl -s http://localhost:9100/api/me -H "Authorization: Bearer $ACCESS" | python3 -m json.tool
# トークン無しなら 401
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:9100/api/me
```

### ステップ 6: M2M(Client Credentials)を試す

```bash
curl -s -X POST http://localhost:9100/token \
  --data-urlencode "grant_type=client_credentials" \
  --data-urlencode "client_id=demo-service" \
  --data-urlencode "client_secret=demo-service-secret" \
  --data-urlencode "scope=api.read" | python3 -m json.tool
# 返った access_token は /api/me で使える(id_token は返らない)
```

## 9. コード解説

### as.go — 認可エンドポイント(/authorize)

```go
func (a *AS) handleAuthorizeGET(w http.ResponseWriter, r *http.Request) {
    p := parseAuthorizeParams(r)
    client, ok := a.store.Client(p.ClientID)
    if !ok {
        http.Error(w, "unknown client_id", http.StatusBadRequest)
        return
    }
    if !validRedirect(client, p.RedirectURI) {
        // redirect_uri が不正な場合はリダイレクトせず直接エラー(オープンリダイレクト防止)
        http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
        return
    }
    if p.CodeChallenge == "" || p.CodeChallengeMethod != "S256" {
        a.redirectError(w, r, p.RedirectURI, p.State, "invalid_request")
        return
    }
    // ...同意画面を表示
}
```

`redirect_uri` は登録済みの値と **完全一致** するときのみ受け付ける。一致しない場合はそこへリダイレクトせず直接エラーにする(攻撃者の指定する URL へコードを送らないため = オープンリダイレクト防止)。また public クライアントには PKCE(S256)を必須にしている。

### as.go — トークンエンドポイント(/token)の PKCE 検証

```go
func (a *AS) grantAuthorizationCode(w http.ResponseWriter, r *http.Request) {
    code := r.FormValue("code")
    verifier := r.FormValue("code_verifier")

    ac, ok := a.store.TakeCode(code) // 取り出し時に削除 = 単回使用
    if !ok {
        tokenError(w, http.StatusBadRequest, "invalid_grant")
        return
    }
    if ac.ClientID != clientID || ac.RedirectURI != redirectURI {
        tokenError(w, http.StatusBadRequest, "invalid_grant")
        return
    }
    // PKCE: 保存した challenge と verifier から計算した S256 が一致するか
    if verifier == "" || pkceS256Challenge(verifier) != ac.CodeChallenge {
        tokenError(w, http.StatusBadRequest, "invalid_grant")
        return
    }
    // ...アクセストークン/ID Token/リフレッシュトークンを発行
}
```

`TakeCode` は取り出すと同時にコードを削除するため、**2 回目の交換は必ず失敗** する(単回使用)。続いて、保存しておいた `code_challenge` と、提示された `code_verifier` から計算し直した S256 値を比較する。これが PKCE 検証の核心である。

### store.go — 認可コードの単回・短命管理

```go
func (s *Store) TakeCode(code string) (*AuthCode, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    c, ok := s.codes[code]
    if !ok {
        return nil, false
    }
    delete(s.codes, code) // 取り出した時点で削除 = 単回使用を保証
    if time.Now().After(c.ExpiresAt) {
        return nil, false
    }
    return c, true
}
```

ロック内で「取得 → 即削除 → 期限確認」を行う。コードは発行時に TTL 60 秒を設定しており、短命かつ単回であることをストア側で保証している。

### tokens.go — S256 challenge の計算

```go
func pkceS256Challenge(verifier string) string {
    sum := sha256.Sum256([]byte(verifier))
    return base64.RawURLEncoding.EncodeToString(sum[:])
}
```

`base64.RawURLEncoding` を使うことで、URL セーフ(`+/` ではなく `-_`)かつパディング無しの base64url になる。RFC 7636 の定義どおりで、既知ベクタ(`dBjftJ...` → `E9Melh...`)とも一致する(`endpoints_test.go` の `TestPKCEKnownVector`)。

## 10. まとめ

- OAuth 2.0 は **認可(権限の委譲)** の枠組みであり、認証そのものではない(認証は OIDC = 次章)。
- ユーザが介在するフローは **Authorization Code + PKCE** に統一する。Implicit と ROPC は使わない。
- **アクセストークンはバックチャネルでのみ授受** し、ブラウザの URL には短命な認可コードしか乗せない。
- **認可コードは単回・短命・束縛**(client_id/redirect_uri/PKCE/nonce)で、横取りを防ぐ。
- **PKCE** は `challenge = S256(verifier)` の照合で「コードを交換しているのが正規クライアントか」を保証する。
- **state** で CSRF を、**redirect_uri の完全一致** でオープンリダイレクトを防ぐ。
- **Client Credentials** はユーザの居ない M2M 用で、ID Token は発行されない。
- 次章 `04_oidc.md` では、このアクセストークン発行の上に **認証層(ID Token)** を載せる OpenID Connect を扱う。
```
