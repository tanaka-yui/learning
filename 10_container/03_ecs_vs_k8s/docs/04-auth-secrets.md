# Auth + Secrets: ECS vs Kubernetes

## 概念対応表

| 概念 | ECS | Kubernetes |
|---|---|---|
| 実行 IAM | Task Role (アプリ用) + Task Execution Role (ECS エージェント用) | `ServiceAccount` |
| クラウド認可 | Task Role が直接 STS AssumeRole | IRSA (IAM Roles for Service Accounts) / EKS Pod Identity |
| シークレット注入 | SecretsManager / SSM Parameter Store の ARN を `secrets` フィールドで env 注入 | `Secret` リソース + External Secrets Operator (ESO) |
| 保存時暗号化 | SecretsManager が KMS で自動暗号化 | etcd 暗号化 (要設定) / `Secret` は base64 エンコードのみ (平文と同等) |

## Task Role vs IRSA: トークン交換シーケンス

### ECS Task Role

```
ECS Agent
  └─→ STS AssumeRole (Task 定義の taskRoleArn)
        └─→ 一時クレデンシャル (AKID/SAK/Token)
              └─→ Task 内 169.254.170.2 のメタデータエンドポイント経由で取得
```

Task 内で `AWS_CONTAINER_CREDENTIALS_RELATIVE_URI` 環境変数が自動設定される。AWS SDK はこれを参照して透過的にクレデンシャルを取得する。

### IRSA (EKS)

```
Pod (ServiceAccount: my-sa)
  └─→ kubelet が OIDC トークンを /var/run/secrets/eks.amazonaws.com/serviceaccount/token にマウント
        └─→ AWS SDK が STS AssumeRoleWithWebIdentity を呼び出し
              └─→ 一時クレデンシャルを取得
```

EKS の OIDC プロバイダが発行した JWT を STS が検証することで、IAM ロールと ServiceAccount を 1:1 でバインドする。Pod Identity (2023 GA) ではエージェント経由になり設定がさらに簡素化された。

## External Secrets Operator (ESO)

ESO は k8s `Secret` を SecretsManager / SSM / Vault 等の外部シークレットストアと同期するコントローラ。

```
ExternalSecret (CR)
  └─→ ESO Controller
        └─→ SecretsManager から値を取得
              └─→ k8s Secret に書き込み (定期更新可)
```

ECS の `secrets` フィールドは Task 起動時に一度だけ取得するのに対し、ESO は定期ポーリングで Secret を最新状態に保てる。

## 暗号化に関する注意

Kubernetes の `Secret` リソースはデフォルトで base64 エンコードのみであり、etcd への保存時は平文と同等。本番環境では以下のいずれかを設定する。

- `EncryptionConfiguration` で etcd 保存時に KMS を使って暗号化
- Sealed Secrets / ESO を使って Secret を etcd に保管しない設計にする
