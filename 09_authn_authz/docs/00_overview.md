# 09_authn_authz — 全体像

## 1. 概要

本モジュールは、Webアプリケーションにおける認証(authentication)と認可(authorization)の主要方式を、動くGoデモと解説で学ぶ。Auth0・Cognito・Keycloak などの既製 IdP は比較解説のみにとどめ、フロー自体は自作Goデモで可視化する方針としている。

扱うテーマは次の通り。

- ステートフルなセッション認証と、ステートレスな JWT の対比
- OAuth 2.0 による権限委譲と、OIDC による認証層の標準化
- 多要素認証(TOTP / WebAuthn / Magic Link)によるパスワードへの上乗せと代替
- サービス間(M2M)認証の実装パターン(API Key / mTLS / Client Credentials)
- Casbin を使ったアプリケーション内認可(RBAC / ABAC)

## 2. 認証(authn) vs 認可(authz)

| 概念 | 問いかけ | 典型的な手段 |
|------|---------|-------------|
| **認証 (authentication / authn)** | 「あなたは誰ですか」 | パスワード、JWT、OIDC、MFA |
| **認可 (authorization / authz)** | 「あなたは何をしてよいですか」 | RBAC、ABAC、スコープ、ポリシー |

混同されやすいポイント: OAuth 2.0 は「**認可**」の枠組みであり、それ自体は認証を定義しない。「アクセストークンを持っている = 本人」と見なすのは誤りで、認証が必要な場合は OIDC を用いる(→ [04_oidc.md](04_oidc.md))。

## 3. 用語集

| 用語 | 英語 | 定義 |
|------|------|------|
| 主体 | Subject / Principal | 認証・認可の対象となるエンティティ(ユーザ・サービス) |
| 資格情報 | Credential | 本人であることを示す提示物(パスワード・証明書・トークン) |
| クレーム | Claims | トークンに埋め込まれた属性(sub・exp・roles 等) |
| トークン | Token | 権限や身元を表す署名付き文字列(JWT 等) |
| セッション | Session | サーバ側に保持されるログイン状態とその識別子 |
| PEP | Policy Enforcement Point | アクセスを遮断・許可する実行点(ミドルウェア等) |
| PDP | Policy Decision Point | ポリシーを評価してアクセス可否を決定するコンポーネント |
| PIP | Policy Information Point | 判断に必要な属性情報を提供するコンポーネント |
| IdP | Identity Provider | ユーザ認証を行いトークンを発行するサービス(AS を含む) |
| RP | Relying Party | IdP/AS に認証・認可を依存するアプリケーション |
| AS | Authorization Server | OAuth 2.0 / OIDC のトークン発行サーバ |
| RS | Resource Server | アクセストークンを検証して保護リソースを返すサーバ |

## 4. 学習パス

デモを学ぶ推奨順序と各章の位置づけを示す。

### 認証フロー

| 順 | ドキュメント | 内容と位置づけ |
|----|------------|--------------|
| 1 | [01_session.md](01_session.md) | サーバ側セッション + Cookie。最も基本的なステートフルモデルを確認する |
| 2 | [02_jwt.md](02_jwt.md) | JWT でステートレス検証へ。アクセス/リフレッシュ/ローテーション/ブロックリストを学ぶ |
| 3 | 05_token_ops.md *(追加予定)* | トークン運用(期限・失効・鍵ローテーション)の実践パターン |
| 4 | [03_oauth.md](03_oauth.md) | OAuth 2.0 による権限委譲。Authorization Code + PKCE と Client Credentials(M2M) |
| 5 | [04_oidc.md](04_oidc.md) | OIDC で認証層を追加。ID Token / Discovery / JWKS |
| 6 | [06_passwordless_mfa.md](06_passwordless_mfa.md) | TOTP / WebAuthn(パスキー) / Magic Link |
| 7 | [07_api_m2m.md](07_api_m2m.md) | API Key / mTLS によるサービス間認証 |
| 8 | 08_idp_compare.md *(追加予定)* | Auth0・Cognito・Keycloak の比較解説 |

### 認可設計

| 順 | ドキュメント | 内容と位置づけ |
|----|------------|--------------|
| 9 | 09_authz_framework.md *(追加予定)* | 認可モデルの概念(RBAC / ABAC / ReBAC / OPA) |
| 10 | 10_authz_design.md *(追加予定)* | 認可設計パターン + Casbin(authzデモ)の詳解 |

## 5. デモ一覧と起動

各デモはプロジェクトルート(`09_authn_authz/`)から `make <デモ名>` で起動できる。

| デモ名 | 内容 | ポート |
|--------|------|--------|
| `session` | サーバ側セッション + Cookie 認証 | 9000 |
| `jwt` | JWT(アクセス/リフレッシュ/ローテーション/失効) | 9001 |
| `oauth-oidc` | 自作 AS + RP + RS、Authorization Code + PKCE / OIDC / Client Credentials | 9100 |
| `authz` | Casbin による RBAC + ABAC | 9200 |
| `mfa` | TOTP / WebAuthn / Magic Link(Mailpit: 8025) | 9300 |
| `api-m2m` | API Key / mTLS | 9400 / 9401 |

```bash
make session      # セッション認証デモ
make jwt          # JWTデモ
make oauth-oidc   # OAuth 2.0 / OIDCデモ
make authz        # 認可(Casbin)デモ
make mfa          # MFA/パスワードレスデモ
make gen-certs    # mTLS 用証明書を生成(api-m2m の前に実行)
make api-m2m      # API Key / mTLSデモ
make down         # 全サービス停止
```

## 6. 前提・スコープ

- **言語 / ランタイム**: Go 1.26+
- **インフラ**: Docker / Docker Compose(各デモは `make <名前>` で compose up)
- **状態管理**: すべてインメモリ実装。サーバ再起動で状態はリセットされる
- **ネットワーク**: `localhost` 前提。HTTPS は mTLS デモ(`https://localhost:9401`)のみ

### 本モジュールが扱わないこと

パスワードのハッシュ化(bcrypt)・レート制限・セッション固定攻撃への対策は `03_security_measures/docs/auth-bypass.md` で詳しく扱っている。本モジュールは各認証・認可フローの仕組みと実装パターンの理解に集中し、これらの基礎対策は既習とみなす。
