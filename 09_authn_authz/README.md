# 09_authn_authz: 認証・認可 学習

認証(authentication)と認可(authorization)の主要方式を、動くGoデモと解説で学ぶ。
既製IdP(Auth0/Cognito/Keycloak)は比較解説のみ、フロー自体は自作実装で可視化する。

## デモ一覧

| デモ | 内容 | 起動 | ポート |
|------|------|------|--------|
| session | サーバ側セッション + Cookie認証 | `make session` | 9000 |
| jwt | JWT(access/refresh/rotation/失効) | `make jwt` | 9001 |
| oauth-oidc | 自作AS + RP + RS、Authorization Code + PKCE / OIDC / Client Credentials(M2M) | `make oauth-oidc` | 9100 |
| authz | Casbin による RBAC + ABAC | `make authz` | 9200 |
| mfa | TOTP(2FA) / WebAuthn(Passkeys) / Magic Link | `make mfa` | 9300 (Mailpit 8025) |
| api-m2m | API Key / mTLS | `make api-m2m` | 9400 / 9401 |

## 前提条件

- Docker / Docker Compose
- (任意) Go 1.26+
- (mTLSデモのみ) OpenSSL — `make gen-certs` で証明書生成に使用

## 使い方

```bash
make session      # Session認証デモ
make jwt          # JWTデモ
make oauth-oidc   # OAuth2/OIDCデモ
make authz        # 認可(Casbin)デモ
make mfa          # MFA/パスワードレスデモ(メールは http://localhost:8025 で確認)
make gen-certs    # mTLS用の証明書を api-m2m/certs/ に生成(api-m2m の前に実行)
make api-m2m      # API Key / mTLSデモ
make all          # 全デモ起動
make down         # 全サービス停止
make help         # ヘルプ表示
```

## ドキュメント

- [00 全体像・学習ガイド](docs/00_overview.md)
- [01 セッション認証](docs/01_session.md)
- [02 JWT](docs/02_jwt.md)
- [03 OAuth 2.0](docs/03_oauth.md)
- [04 OpenID Connect](docs/04_oidc.md)
- [05 トークン運用の実務](docs/05_token_ops.md)
- [06 パスワードレス / MFA](docs/06_passwordless_mfa.md)
- [07 API / M2M 認証](docs/07_api_m2m.md)
- [08 IdP比較(Auth0 / Cognito / Keycloak)](docs/08_idp_comparison.md)
- [09 Go 認可フレームワーク比較](docs/09_authz_frameworks.md)
- [10 認可の設計(RBAC / ABAC / ReBAC / PBAC)](docs/10_authz_design.md)
