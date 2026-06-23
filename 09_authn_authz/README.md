# 09_authn_authz: 認証・認可 学習

認証(authentication)と認可(authorization)の主要方式を、動くGoデモと解説で学ぶ。
既製IdP(Auth0/Cognito/Keycloak)は比較解説のみ、フロー自体は自作実装で可視化する。

## デモ一覧

| デモ | 内容 | 起動 | ポート |
|------|------|------|--------|
| session | サーバ側セッション + Cookie認証 | `make session` | 9000 |

(jwt / oauth-oidc / mfa / api-m2m / authz は順次追加)

## 前提条件

- Docker / Docker Compose
- (任意) Go 1.26+

## 使い方

```bash
make session   # Session認証デモ起動
make down      # 全サービス停止
make help      # ヘルプ表示
```

## ドキュメント

- [01 セッション認証](docs/01_session.md)
