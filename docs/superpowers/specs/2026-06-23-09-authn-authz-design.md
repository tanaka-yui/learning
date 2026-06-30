# 09_authn_authz 学習モジュール 設計書

- 日付: 2026-06-23
- 対象: `09_authn_authz/`(新規モジュール)
- ゴール: 認証(authentication)と認可(authorization)の主要方式を、動くGoデモ + 日本語解説で学べる自己完結型の学習セットを作る。

## 1. 目的と方針

### 目的
認証・認可の代表的なプロトコル/方式の「中身」を、実際に動かしながら理解する。既製IdP(Auth0/Cognito/Keycloak)は比較解説のみで扱い、フロー自体は自作実装で可視化する。

### 方針
- **言語: Go 1.26 のみ**(既存モジュール 03/08 と統一)。TypeScript実装・対応表は作らない。
- **フロント: 各Goアプリが最小HTMLをサーバ配信**。Vite等の別フロントは立てない。WebAuthn / Magic Link のみ `navigator.credentials` 等のため最小HTML+JSをGoが配信する。
- **オーケストレーション**: 単一 `docker-compose.yml` + `profiles`。`make <topic>` / `make all` / `make down` / `make help`(03のMakefile踏襲)。
- **既存との非重複**: `03_security_measures/auth-bypass` がパスワードハッシュ(bcrypt)・レート制限・セッション固定を既にカバー済。09はそれらの脆弱性ではなく **プロトコル・フローそのもの** に集中する。重複説明は避け、必要箇所は03へ相互参照する。
- **外部アカウント不要・self-contained**: OAuth/OIDCの認可サーバ(AS/IdP)はGoで自作するため、Keycloakや外部サービスのアカウントは不要。

### 設計判断: OAuth/OIDC の IdP をどう用意するか
OAuth2/OIDCは Resource Owner / Client / Authorization Server / Resource Server の4者プロトコルで、トークンを発行する認可サーバが必ず動いている必要がある。本モジュールでは **Goで最小の認可サーバ(AS)を自作** する。理由:
- 中身(認可エンドポイント、トークンエンドポイント、JWKS、ID Token生成)が全部見えて学習価値が最大。
- 外部アカウント不要・Keycloakイメージ不要で self-contained。
- Go統一の方針と一致。

Keycloak/Auth0/Cognito は `docs/08_idp_comparison.md` で比較解説のみ(実装しない)。

## 2. ディレクトリ構成

```
09_authn_authz/
├── Makefile
├── docker-compose.yml
├── go.work                # 各デモのgo moduleをまとめる(08踏襲)
├── README.md
├── .gitignore
├── session/               # デモ: Cookieセッション認証
├── jwt/                   # デモ: JWT(access/refresh/rotation/失効)
├── oauth-oidc/
│   ├── authz-server/      # 自作 Authorization Server(IdP)
│   ├── client/            # Client / Relying Party(RP)
│   └── resource-server/   # 保護リソースAPI
├── mfa/                   # デモ: TOTP + WebAuthn/Passkeys + Magic Link
├── api-m2m/               # デモ: API Key + mTLS
├── authz/                 # デモ: Casbin(RBAC + ABAC)
├── certs/                 # mTLS用証明書(make gen-certs で生成、git管理外)
└── docs/                  # 日本語解説(00〜10)
```

## 3. 動くデモ仕様

各デモは独立したGoアプリ。compose `profiles` で個別起動。各アプリは `/` に動作確認用の最小HTMLを返す。ポートはモジュール内compose前提で衝突回避のため割当て。

| dir | profile | 内容 | ポート | 主ライブラリ |
|---|---|---|---|---|
| `session/` | `session` | サーバ側セッションストア(インメモリ) + Cookie。ログイン/ログアウト/保護ページ。JWTとの対比基準 | 9000 | 標準 `net/http` |
| `jwt/` | `jwt` | JWT発行/検証(HS256 + RS256選択)、access+refreshトークン、refresh rotation、失効(blocklist/jti) | 9001 | `github.com/golang-jwt/jwt/v5` |
| `oauth-oidc/` | `oauth-oidc` | 自作AS + RP + Resource Server。Authorization Code + PKCE、OIDC(ID Token, `/.well-known/openid-configuration`, JWKS, UserInfo)、Client Credentials(M2M) | AS 9100 / RP 9101 / RS 9102 | `golang.org/x/oauth2`, `github.com/coreos/go-oidc/v3`, `golang-jwt/jwt/v5` |
| `mfa/` | `mfa` | パスワード + 第二要素。TOTP(2FA, QR表示)、WebAuthn/Passkeys(登録/認証)、Magic Link(メールはMailpitで受信) | app 9300 / Mailpit UI 8025 | `github.com/pquerna/otp`, `github.com/go-webauthn/webauthn` |
| `api-m2m/` | `api-m2m` | サービス間認証。API Keyガード(ヘッダ検証 + 定数時間比較)、mTLS(クライアント証明書検証) | API Key 9400 / mTLS 9401 | 標準 `crypto/tls`, `crypto/subtle` |
| `authz/` | `authz` | Casbinで RBAC(ロール継承含む)と ABAC(属性ベース)を実動。ガード付きHTTP API | 9200 | `github.com/casbin/casbin/v2` |

### デモ別の要点

#### session/
- ログインで `username/password`(bcryptハッシュ照合、ユーザはシード)→ サーバ側にセッション生成 → `session_id` をHttpOnly Cookieで返す。
- 保護エンドポイントはCookieのセッション存在で認可。ログアウトでセッション破棄。
- セキュア属性(HttpOnly/SameSite)は設定するが、固定攻撃等の詳細対策は03を参照。

#### jwt/
- ログインで access(短命, 例5分) + refresh(長命) を発行。
- access検証ミドルウェア。`/refresh` でrefresh rotation(旧refreshを失効し新ペア発行)。
- 失効: refreshは `jti` + blocklist(インメモリ)で無効化可能。短命access + refresh失効の運用パターンを実演。
- HS256(共有鍵)とRS256(公開鍵検証, JWKS連携の布石)の両方を切替で示す。

#### oauth-oidc/
- **authz-server(AS)**: `/authorize`(Authorization Code + PKCE `S256`)、`/token`(code交換 / refresh / client_credentials)、`/.well-known/openid-configuration`、`/jwks.json`、`/userinfo`。ID Tokenを署名(RS256)。ユーザ/クライアントはシード。
- **client(RP)**: `x/oauth2` + `go-oidc` で認可リクエスト → コールバックでcode交換 → ID Token検証 → セッション確立。state/nonce/PKCE検証。
- **resource-server(RS)**: ASのJWKSでアクセストークンを検証し保護APIを提供。
- **M2M**: clientがClient Credentials grantでトークン取得 → RSを呼ぶ例。`docs/07_api_m2m.md` から参照。

#### mfa/
- ベースはパスワードログイン。第二要素を3方式で実演:
  - **TOTP**: シークレット生成 → `otpauth://` QR表示 → 認証アプリのコード検証。
  - **WebAuthn/Passkeys**: 登録(attestation)と認証(assertion)。最小HTML+JSで `navigator.credentials.create/get`。`localhost` はsecure context扱いのためHTTPで動作。RP IDは `localhost`。
  - **Magic Link**: メールにワンタイムリンク送信 → クリックでログイン。SMTP送信先は **Mailpit**(08のパターン踏襲)、UIで受信確認。
- ストレージはインメモリ(クレデンシャル/チャレンジ)。

#### api-m2m/
- **API Key**: `Authorization: Bearer <key>` または `X-API-Key` を `crypto/subtle.ConstantTimeCompare` で検証。発行済キーはシード。
- **mTLS**: サーバが `tls.Config{ClientAuth: RequireAndVerifyClientCert}` でクライアント証明書を要求・検証。CA/サーバ/クライアント証明書は `make gen-certs` で生成し `certs/` に置く(git管理外)。`curl --cert/--key` で検証手順を示す。
- Client Credentials(M2M)は `oauth-oidc/` 側で実演し、ここからは解説で参照。

#### authz/
- Casbinの `model.conf` + `policy.csv` で:
  - **RBAC**: user→role→permission、ロール継承(`g`)。
  - **ABAC**: リソース所有者・属性に基づく判定(`r.sub.X` 形式 or matcher内属性評価)。
- HTTPミドルウェアが Casbin Enforcer で `(sub, obj, act)` を判定。許可/拒否をAPIで確認。
- PEP(ミドルウェア) / PDP(Enforcer) / ポリシー(PIP相当のデータ)の分離を実例で示し、`docs/10_authz_design.md` と接続。

## 4. ドキュメント仕様(`docs/`、日本語)

スタイルは03踏襲: 概要 → 仕組み/フロー図(ASCII) → デモ起動手順 → 動作/確認手順 → コード解説 → まとめ。

| ファイル | 内容 |
|---|---|
| `00_overview.md` | 全体像・学習パス。認証 vs 認可の違い、用語(主体/資格情報/クレーム、PEP/PDP/PIP)、各デモの位置づけ |
| `01_session.md` | セッション認証の概念、サーバ側状態、Cookie属性、スケール時の論点(共有ストア)。session vs token対比の起点 |
| `02_jwt.md` | JWT構造(header/payload/signature)、署名(HS256/RS256)、検証、access/refresh設計、rotation、失効戦略 |
| `03_oauth.md` | OAuth2.0の役割と4グラント、Authorization Code + PKCE、なぜImplicitが非推奨か、scope |
| `04_oidc.md` | OIDC = OAuth2 + 認証層。ID Token、claims、Discovery、JWKS、UserInfo、nonce |
| `05_token_ops.md` | トークン運用実務: access短命+refresh、blocklist、rotation、保存場所(Cookie vs storage)とトレードオフ |
| `06_passwordless_mfa.md` | TOTP / WebAuthn(Passkeys) / Magic Link の仕組み・脅威耐性・選定基準。フィッシング耐性の比較 |
| `07_api_m2m.md` | サービス間/API認証: API Key / mTLS / OAuth Client Credentials の使い分け |
| `08_idp_comparison.md` | **Auth0 / AWS Cognito / Keycloak** の機能比較・サポート範囲・料金感・OSS/セルフホスト可否・移行性。**実装なし** |
| `09_authz_frameworks.md` | **Goの認可フレームワーク比較**: Casbin / OPA(Rego) / OpenFGA(Zanzibar型) / Cerbos(PDP) / oso。適性・モデル・運用形態。**Go限定、Go/TS対応表は作らない** |
| `10_authz_design.md` | 認可の設計指針: RBAC / ABAC / ReBAC / PBAC、PEP/PDP/PIP分離、認可をどこに置くか、段階的進化(RBAC→ABAC→ReBAC) |

## 5. 横断要素

- **Makefile**: `make session|jwt|oauth-oidc|mfa|api-m2m|authz`(各profile起動)、`make all`、`make down`、`make gen-certs`(mTLS証明書)、`make help`。
- **go.work**: 各go moduleを束ねる(08踏襲)。
- **シードデータ**: 各デモにテストユーザ/クライアント/キーをコード内シード(認証情報はハードコード回避が困難な学習用途のため最小限・明示コメント)。
- **README.md**: モジュール概要、デモ一覧表(03スタイルのポート表)、起動方法、docsへの導線。
- **.gitignore**: `certs/`、ビルド成果物。
- **ライブラリAPI**: 実装時に各ライブラリ(golang-jwt, go-oidc, pquerna/otp, go-webauthn, casbin)の最新APIをContext7で確認してから書く。

## 6. スコープ外(YAGNI)

- TypeScript実装、Go/TS対応表。
- Auth0/Cognito/Keycloakの実コード連携(比較docのみ)。
- 永続DB(各デモはインメモリ。学習に十分)。
- 本番運用のHTTPS終端・秘密管理(localhost前提)。
- 03で扱う脆弱性詳説(相互参照で済ます)。

## 7. 成功基準

1. `make <topic>` で各デモが起動し、docsの確認手順どおりに動く(ログイン/トークン取得/フロー完走/認可の許可・拒否)。
2. `oauth-oidc` で Authorization Code + PKCE が外部依存なしに完走し、ID Token/アクセストークンが検証される。
3. `mfa` で TOTP・WebAuthn・Magic Link の3方式が動作(Magic LinkはMailpitで受信確認)。
4. `api-m2m` で API Key と mTLS の許可/拒否が確認できる。
5. `authz` で Casbin の RBAC/ABAC 判定が許可/拒否として確認できる。
6. docs 00〜10 が揃い、08(IdP比較)・09(Go認可FW比較)・10(認可設計)が実装なしで完結している。
