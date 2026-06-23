# 08 IdP 比較 — Auth0 / AWS Cognito / Keycloak

## 1. 概要

### IdP / IDaaS とは

**Identity Provider (IdP)** とは、ユーザの認証を代行し、認証結果をトークン(ID Token / Access Token)として発行するサービスやソフトウェアを指す。OAuth 2.0 / OIDC の文脈では Authorization Server(AS)と重なる概念であり、SAML の文脈では SSO フェデレーションの起点となる。

**IDaaS (Identity as a Service)** はこれをクラウドサービスとして提供する形態で、認証基盤の構築・運用コストをサービスにオフロードできる。

### 自作 AS との境界 — なぜ本番では既製品が多いか

本モジュールの `oauth-oidc` デモでは、Go で Authorization Server・Resource Server・Relying Party を自作し、OAuth 2.0 Authorization Code + PKCE・Client Credentials・OIDC Discovery・JWKS・ID Token 発行などのフローを可視化した。自作 AS は学習目的として最適だが、本番運用には以下の課題がある。

| 課題 | 内容 |
|------|------|
| セキュリティ保守 | CVE 対応・暗号アルゴリズムの更新を継続する必要がある |
| 標準仕様の追従 | FAPI 2.0・DPoP・Passkeys など仕様の進化に追従し続ける必要がある |
| 高可用性 | HA 構成・DR・セッションストアのレプリケーションを自前で設計する必要がある |
| MFA / ソーシャルログイン | TOTP・WebAuthn・各 SNS OAuth の実装をすべて自前で用意する必要がある |
| コンプライアンス | SOC 2・ISO 27001 等の認証取得に認証基盤の審査が含まれる |

本番では **本モジュールの oauth-oidc デモで自作した AS/OIDC の役割を、これら既製品が提供する** という位置づけになる。

### 比較する 3 製品

| 製品 | カテゴリ | 概要 |
|------|---------|------|
| **Auth0** | IDaaS (SaaS) | Okta 傘下の CIAM プラットフォーム。開発者体験を重視し、豊富な SDK・Actions による拡張性が強み |
| **AWS Cognito** | マネージドサービス | AWS が提供する認証・認可サービス。AWS エコシステムとの統合がシームレスで、IAM や Lambda と直結できる |
| **Keycloak** | OSS セルフホスト | Red Hat 発の OSS IdP。完全なデータ管理とカスタマイズ性が強みだが、運用コストは利用者が負担する |

---

## 2. 対応プロトコル / 機能の比較表

凡例: ○ = 標準サポート / △ = 制限あり・追加設定が必要 / × = 非対応またはサポート外

| 機能 | Auth0 | AWS Cognito | Keycloak |
|------|-------|-------------|----------|
| **OIDC (OpenID Connect)** | ○ | ○ | ○ |
| **OAuth 2.0** | ○ | ○ | ○ |
| **SAML 2.0** | ○ (Enterprise 向け接続) | ○ (IdP フェデレーション) | ○ (IdP・SP 両対応) |
| **WS-Federation** | ○ | × | ○ |
| **FAPI 2.0 / DPoP** | △ (Enterprise) | × | ○ (26.x で GA) |
| **ソーシャルログイン** | ○ (多数の Provider を標準提供) | ○ (Google/Facebook/Apple/Amazon 等) | ○ (SPI で任意追加) |
| **MFA — TOTP** | ○ | ○ (全プラン) | ○ |
| **MFA — SMS OTP** | ○ (別途 SMS 費用) | ○ (別途 SNS 費用) | ○ (SPI で SMS プロバイダ設定) |
| **MFA — WebAuthn / Passkeys** | ○ | ○ (Essentials 以上) | ○ (26.x で UI 統合) |
| **パスワードレス (Magic Link 等)** | ○ | ○ (Essentials 以上) | △ (カスタム SPI または拡張が必要) |
| **ユーザ管理 / ディレクトリ** | ○ | ○ | ○ |
| **LDAP / AD フェデレーション** | ○ (Enterprise 接続) | × (Cognito ネイティブには非対応) | ○ (標準搭載) |
| **フェデレーション (外部 IdP 連携)** | ○ | ○ (SAML / OIDC IdP) | ○ |
| **マルチテナント / B2B Organizations** | ○ (Organizations 機能) | △ (User Pool 分離または Cognito Groups で代替) | ○ (Organizations, Realms で対応) |
| **カスタムブランディング (ログイン UI)** | ○ (Universal Login) | ○ (Managed Login, Essentials 以上) | ○ (Themes / FreeMarker テンプレート) |
| **フロー拡張** | ○ (Actions / Forms) | ○ (Lambda Triggers) | ○ (Service Provider Interface: SPI) |
| **SCIM プロビジョニング** | ○ | × (ネイティブ非対応) | ○ |
| **M2M (Client Credentials)** | ○ (プランで上限あり) | ○ | ○ |

---

## 3. ホスティング / 運用モデル

### Auth0 — SaaS (フルマネージド)

- Okta の SaaS プラットフォームとして提供される。インフラ・可用性・スケーリングはすべて Auth0 側で管理される。
- **可用性**: SLA 99.99%（Enterprise プラン）。冗長化・フェイルオーバーは利用者が意識しなくてよい。
- **データ所在**: デフォルトは Auth0 のマルチリージョン。EU・US などリージョン選択は Enterprise で相談可能。GDPR 観点では DPA (Data Processing Agreement) が必要。
- **ロックイン**: OIDC / SAML など標準プロトコルに準拠しているが、Actions・Forms・Universal Login などの独自機能への依存度が上がると移行コストが高まる。パスワードハッシュのエクスポートには制約があり、移行時にユーザへのパスワードリセット要求が必要になる場合がある。

### AWS Cognito — クラウドマネージド (AWS 統合)

- AWS が管理するフルマネージドサービス。Lambda・API Gateway・IAM と直接統合できる。
- **可用性**: AWS の SLA に準拠。マルチ AZ で自動フェイルオーバー。
- **データ所在**: AWS リージョン内にデータが保存される。リージョン選択でデータ主権を管理できるが、AWS から外への移行は困難。
- **ロックイン**: AWS エコシステムへの依存が強い。Lambda Triggers・Cognito 固有 API への依存が深まると他サービスへの移行が困難になる。**パスワードハッシュのエクスポートは非対応**であり、移行時はユーザへのパスワードリセットか、Just-in-time マイグレーション(初回ログイン時にハッシュを移行するパターン)が必要。

### Keycloak — セルフホスト (OSS)

- 利用者が自分のインフラ(オンプレ・Kubernetes・VM)で Keycloak を運用する。
- **可用性**: HA 構成は利用者の責任で設計する。Kubernetes 上では公式 Operator が提供されている。
- **データ所在**: 利用者のインフラにデータが保存される。データ主権が最も強い。GDPR・特定業界規制でデータを外部に出せない要件に対応しやすい。
- **ロックイン**: OSS であるためベンダーロックインがない。OIDC / SAML 準拠で標準的。データベースへの直接アクセスが可能なためパスワードハッシュを含む全データのエクスポート・移行が容易。
- **運用負荷**: パッチ適用・バージョンアップ・監視・スケーリングをすべて自チームで担う。Red Hat Build of Keycloak (RHBK) を使えばサポートを購入できる。

---

## 4. 料金モデル

> **注意**: 以下の情報は執筆時点(2026年6月)のものです。料金・プラン構成は頻繁に変更されるため、**必ず各社の公式サイトで最新情報を確認してください**。

### Auth0

課金軸は **MAU (Monthly Active Users)** と **M2M トークン数**。B2C・B2B でプランが分かれ、B2B は同 MAU 数でも B2C より大幅に高価になる傾向がある(Enterprise SSO・Organizations 機能を含むため)。

| プラン | 特徴 | 無料枠 |
|--------|------|--------|
| Free | 基本認証・ソーシャルログイン・1 エンタープライズ接続 | 25,000 MAU |
| Essentials | カスタムドメイン・RBAC・MFA | 無料枠なし(有償から) |
| Professional | クロスアプリ SSO・カスタム DB 接続・拡張 Actions | 無料枠なし |
| Enterprise | SLA 99.99%・SCIM・Adaptive MFA・専用サポート | カスタム交渉 |

- MAU が増えると段階的に単価が上昇し、スケール時にコストが急増するケースがある。
- M2M トークン(Client Credentials)はプランごとに月間上限があり、上限超過分は別途課金。
- SMS MFA は別途メッセージ費用が発生する。

### AWS Cognito

課金軸は **MAU** と **M2M (Client Credentials) トークン数**。2024年12月から Lite / Essentials / Plus の3段階プランに移行。

| プラン | 特徴 | 無料枠 |
|--------|------|--------|
| Lite | 基本認証・SAML/OIDC フェデレーション・TOTP MFA | 10,000 MAU |
| Essentials | Passkeys・パスワードレス・Managed Login UI・Access Token カスタマイズ | 10,000 MAU |
| Plus | 脅威保護・不審ログイン検出・ユーザ行動ログ | 10,000 MAU |

- 無料枠は 2024年の改定で 50,000 MAU → 10,000 MAU に縮小された。
- SAML/OIDC フェデレーションユーザは最初の 50 MAU が無料、以降は別途課金。
- SMS OTP は Amazon SNS の費用が別途発生する。
- MAU 増加に伴い自動でボリュームディスカウントが適用される(段階的単価逓減)。

### Keycloak

Keycloak 本体は **Apache License 2.0 の無償 OSS**。ライセンス費用はゼロだが、実際のコストは次の要素からなる。

| コスト要素 | 内容 |
|------------|------|
| インフラ費用 | VM・Kubernetes クラスタ・DB(PostgreSQL 等)の稼働コスト |
| 運用人件費 | パッチ適用・バージョンアップ・監視・インシデント対応 |
| Red Hat Build of Keycloak (RHBK) | Red Hat サブスクリプションを購入すれば商用サポートを取得可能 |

MAU 課金がないため、ユーザ数が多い場合は Keycloak の総コストが SaaS より低くなる可能性がある。一方、小規模組織では運用人件費が SaaS 料金を上回る場合もある。

---

## 5. サポート / SLA / エンタープライズ機能

| 項目 | Auth0 | AWS Cognito | Keycloak (OSS) |
|------|-------|-------------|----------------|
| **SLA** | 99.99% (Enterprise) | AWS 基盤に準拠 | 自チームの構成次第 |
| **商用サポート** | Standard / Premier サポート (有償) | AWS サポートプラン | Red Hat Build of Keycloak (RHBK) |
| **監査ログ保持** | 1日 (Free) 〜 30日 (Enterprise) / ストリーム出力対応 | Plus プランでログエクスポート対応 | 組み込み Admin Events / ログエクスポートは自前設定 |
| **SOC 2 Type II** | ○ | ○ | 利用者のインフラ側で取得する必要あり |
| **ISO 27001** | ○ | ○ | 利用者のインフラ側で取得する必要あり |
| **HIPAA** | ○ (Enterprise / BAA 締結) | ○ (BAA 締結) | 利用者のインフラ次第 |
| **FedRAMP** | ○ (Enterprise 一部) | ○ | 対象外 |
| **SCIM** | ○ (Essentials 以上) | × | ○ |
| **LDAP / AD 統合** | ○ (Enterprise 接続) | × | ○ (標準搭載) |
| **Fine-Grained Admin Permissions** | △ | △ | ○ (FGAP: 26.x で強化) |

---

## 6. 移行性 / ロックイン

### 標準プロトコル準拠度

3 製品はいずれも OIDC・OAuth 2.0・SAML 2.0 に準拠しており、RP (アプリケーション) 側のコードを大きく変えずに IdP を切り替えることができる。ただし、各製品の独自機能(Auth0 Actions・Cognito Lambda Triggers・Keycloak SPI)に依存した実装は移行時に書き直しが必要になる。

### パスワードハッシュのエクスポート

| 製品 | パスワードハッシュ エクスポート | 備考 |
|------|-------------------------------|------|
| Auth0 | 制限あり | 直接エクスポートは不可。カスタム DB 接続を使って段階移行が可能 |
| AWS Cognito | **非対応** | ハッシュをエクスポートできない。JIT マイグレーションかパスワードリセットが必要 |
| Keycloak | **完全対応** | DB に直接アクセスできるため、全データ(ハッシュを含む)を自由にエクスポートできる |

### 移行の難所

- **Auth0 → 他**: Actions・Forms・Rules などの拡張ロジックの書き直しが主なコスト。Universal Login のカスタマイズも移植が必要。
- **Cognito → 他**: パスワードハッシュが取り出せないため、ユーザへのパスワードリセット通知またはログイン時に JIT でパスワードを移行する仕組みの実装が必要。Lambda Triggers の再実装も必要。
- **Keycloak → 他**: DB から全データ(ユーザ・ロール・フェデレーション設定・カスタムテーマ)をエクスポートできるため、技術的な移行ハードルは低い。ただし Realm・Client・SPI の設定量が多いと移行設計に工数がかかる。

---

## 7. 選定ガイド

### ユースケース別の推奨

| ユースケース | 推奨 | 理由 |
|------------|------|------|
| **スタートアップ / 素早く立ち上げたい** | Auth0 | SDK・ドキュメントが充実しており、最短で認証を組み込める。初期は無料枠(25,000 MAU)で賄える |
| **AWS を中心としたスタック** | AWS Cognito | API Gateway・Lambda・IAM との統合がネイティブ。AWS の他サービスを多用しているチームには運用コストが低い |
| **データ主権・OSS 志向 / オンプレ必須** | Keycloak | ハッシュを含む全データを自社インフラに保持できる。MAU 課金がないため大規模ユーザにもコスト効率が良い |
| **B2B SaaS / エンタープライズ SSO** | Auth0 (Organizations) または Keycloak | Auth0 Organizations は B2B マルチテナントを迅速に実装できる。コスト許容度が低い場合は Keycloak も有力 |
| **LDAP / AD と統合が必要** | Keycloak | LDAP/AD フェデレーションを標準搭載。Cognito はネイティブ非対応、Auth0 は Enterprise 接続が必要 |
| **コンプライアンス要件が厳しい (HIPAA 等)** | Auth0 Enterprise または AWS Cognito | 認証済みのコンプライアンスパッケージが揃っており、BAA 締結も可能。Keycloak は自社インフラでの認証取得が必要 |
| **大規模ユーザ基盤 (100万 MAU 超)** | Keycloak または Cognito | Auth0 は MAU ベース課金でスケール時にコストが急増しやすい。Cognito はボリュームディスカウントあり。Keycloak はライセンス費用ゼロ |

### 判断フロー

```
AWS を主軸に使っているか？
  ├─ YES → AWS Cognito（AWSネイティブ統合・LDAP不要の場合）
  └─ NO
       ├─ 自社インフラへのデータ保持が必要か？
       │    ├─ YES → Keycloak
       │    └─ NO
       │         ├─ 素早く立ち上げたい / B2B SSO が重要か？
       │         │    ├─ YES → Auth0
       │         │    └─ NO → 規模・コストで Cognito または Keycloak を再検討
```

---

## 8. まとめ

### 要点の再掲

| 比較軸 | Auth0 | AWS Cognito | Keycloak |
|--------|-------|-------------|----------|
| **ホスティング** | SaaS (Okta 管理) | AWS マネージド | セルフホスト |
| **OIDC / OAuth 2.0** | ○ | ○ | ○ |
| **SAML 2.0** | ○ | ○ (フェデレーション) | ○ (IdP・SP 両対応) |
| **Passkeys** | ○ | ○ (Essentials 以上) | ○ (26.x) |
| **LDAP / AD** | ○ (Enterprise 接続) | × | ○ |
| **B2B / マルチテナント** | ○ (Organizations) | △ | ○ (Organizations) |
| **拡張性** | Actions / Forms | Lambda Triggers | SPI (Java) |
| **料金モデル** | MAU + M2M 課金 | MAU 課金 (3プラン) | 無償 OSS (インフラ/運用コストのみ) |
| **無料枠** | 25,000 MAU | 10,000 MAU | N/A |
| **パスワードハッシュ エクスポート** | 制限あり | **不可** | **完全可** |
| **データ所在の制御** | 低 (Auth0 管理) | 中 (AWSリージョン) | 高 (自社管理) |
| **運用負荷** | 低 | 低 | 高 |
| **SLA** | 99.99% (Enterprise) | AWS 基盤準拠 | 自チーム次第 |

### 自作 AS との接続点

本モジュールの `oauth-oidc` デモで実装した Authorization Server・OIDC Discovery・JWKS Endpoint・Authorization Code + PKCE・Client Credentials などの仕組みは、これら 3 製品がすべて内包している。自作 AS で学んだフローの理解が、既製品の設定・デバッグ・トラブルシューティングに直接役立つ。

---

> **料金・機能の再確認について**: 本ドキュメントの料金・プラン情報は **2026年6月時点** の公式情報をもとに記載しています。各製品の料金・機能は頻繁に変更されるため、実際の採用・見積もりの際は必ず下記の公式サイトで最新情報を確認してください。
>
> - Auth0: https://auth0.com/pricing
> - AWS Cognito: https://aws.amazon.com/cognito/pricing/ および https://docs.aws.amazon.com/cognito/latest/developerguide/cognito-sign-in-feature-plans.html
> - Keycloak: https://www.keycloak.org/
