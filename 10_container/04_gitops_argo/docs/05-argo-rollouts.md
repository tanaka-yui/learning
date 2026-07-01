# 05 Argo Rollouts

実装参照: `apps/rollouts/`

## 主要リソース

### Rollout

Kubernetes `Deployment` の上位互換リソース。Pod テンプレートはほぼ同じ記法で、`strategy` フィールドに canary / blueGreen / experiment を指定する。

> **注意**: Rollout に置き換えたら、Argo CD が管理していた同名の `Deployment` は削除すること。Argo CD と Rollout が同一リソースを競合管理するとリコンサイルループが発生する。

本章の `apps/rollouts/rollout-canary.yaml` は `demo-prod` Namespace に `api` Rollout を定義している。

### AnalysisTemplate

メトリクスや Job の実行結果をもとに「進行 or ロールバック」を判定するテンプレート。

```yaml
# apps/rollouts/analysistemplate.yaml (抜粋)
spec:
  metrics:
  - name: ready-pods
    interval: 10s
    count: 3
    successCondition: result >= 2
    failureLimit: 2
    provider:
      job:       # Job で kubectl get pods して Ready 数を確認
        spec: ...
```

### AnalysisRun

AnalysisTemplate のインスタンス。Rollout の `analysis` ステップで自動生成される。

### ExperimentRun

複数の ReplicaSet を同時に起動して実験的に比較する高度な機能。A/B テストや複数バージョン並行評価に使う。

## デプロイ戦略

### canary

本番トラフィックを段階的に新バージョンに切り替える戦略。

```yaml
strategy:
  canary:
    steps:
    - setWeight: 20        # 20% を新バージョンに
    - pause: { duration: 30s }
    - setWeight: 50
    - analysis:            # AnalysisTemplate で自動判定
        templates: [{ templateName: pod-ready }]
    - setWeight: 100       # 全量切り替え
```

### blueGreen

旧 (blue) と新 (green) の ReplicaSet を並走させ、切り替え時に Service の selector を変更する。ダウンタイムゼロでの即時切り替えが可能。`prePromotionAnalysis` / `postPromotionAnalysis` で切り替え前後の自動検証ができる。

### experiment

`Experiment` リソースを使って複数の ReplicaSet を一定時間並走させる。通常のカナリアと異なり、本番トラフィックを流す前に閉じた環境で比較できる。

## Metric Provider

| Provider | 用途 |
|---|---|
| Prometheus | クエリ結果の数値で成否判定。最も一般的。 |
| Datadog | Datadog メトリクスで判定。`datadogQuery` に指定。 |
| Web (HTTP) | 外部 API のレスポンス JSON で判定。カスタム判定ロジックに使える。 |
| Job | Kubernetes Job の exit code / stdout で判定。本章の実装はこれを使用。 |
| CloudWatch | AWS CloudWatch メトリクスで判定 (EKS 環境向け)。 |

## Istio VirtualService 連携

Istio を使う場合、`trafficRouting.istio.virtualService` を指定すると Rollout が VirtualService の weight を自動更新する。`setWeight: 20` が VirtualService の `weight: 20` に反映される。サービスメッシュがないクラスタでは Pod レプリカ比率でトラフィック分割するため精度は低くなる。

## kubectl-argo-rollouts プラグイン

```bash
# Rollout 状態確認
kubectl argo rollouts get rollout api -n demo-prod --watch

# 次のステップに進める (pause 中の場合)
kubectl argo rollouts promote api -n demo-prod

# 手動アボート (ロールバック)
kubectl argo rollouts abort api -n demo-prod
```

## CodeDeploy Blue/Green との対応

| Argo Rollouts | AWS CodeDeploy |
|---|---|
| Rollout (canary steps) | Deployment Group (canary %) |
| Rollout (blueGreen) | Deployment Group (blue/green) |
| AnalysisTemplate (Job) | AppSpec hooks (BeforeAllowTraffic/AfterAllowTraffic) |
| setWeight | trafficRoutingConfig.weightBased |
| pause | waitTimeInMinutes |
| AnalysisTemplate (Prometheus) | CloudWatch Alarms での自動ロールバック |

LocalStack Community では CodeDeploy は 501 Non-Community のため宣言のみ。本番 AWS 環境で検証が必要。
