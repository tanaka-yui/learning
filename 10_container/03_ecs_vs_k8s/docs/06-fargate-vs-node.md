# Fargate vs Node: サーバレスコンピュート比較

AWS のサーバレスコンテナ実行選択肢は 4 つに大別できる。

| 項目 | ECS Fargate | EKS Fargate | EKS Autopilot | Karpenter + EC2 |
|---|---|---|---|---|
| ノード管理 | 不要 | 不要 | 不要 | 必要 (NodePool 宣言) |
| コスト単価 | vCPU/GB 時課金 | vCPU/GB 時課金 | vCPU/GB 時課金 (高め) | EC2 オンデマンド / スポット |
| コールドスタート | 10〜30 秒 | 10〜30 秒 | 10〜60 秒 | EC2 起動: 2〜3 分 (AMI キャッシュで短縮可) |
| 同時実行レイテンシ | 低 (起動済み Task は即応) | 低 | 低 | ノード補充時は高 |
| ノード制御 | 不可 | 不可 | 不可 | NodePool / Taint で細かく制御可 |
| DaemonSet | 非対応 | 非対応 | 非対応 | 対応 |
| GPU / 特殊ハード | 非対応 | 非対応 | 非対応 | 対応 |
| スポット活用 | FARGATE_SPOT | FARGATE_SPOT | 一部対応 | EC2 スポット (Karpenter が自動フォールバック) |

## コスト特性

**Fargate 系 (ECS / EKS / Autopilot)**

- 実行時間 × (vCPU 単価 + メモリ単価) で課金
- アイドル時間もゼロにはならない (ECS の desiredCount > 0 の限り課金継続)
- 小〜中規模ワークロードでは管理コストを含めた TCO で有利

**Karpenter + EC2**

- EC2 インスタンス料金。スポットを活用すると Fargate の 50〜70% のコストになることが多い
- ノード台数が少ない夜間帯は consolidation で自動縮退し無駄をなくせる
- 大規模・GPU ワークロードでは圧倒的にコスト優位

## コールドスタートの内訳

Fargate のコールドスタートは以下のフェーズで構成される。

```
Fargate 環境プロビジョニング (仮想化 VM 起動)
  └─→ image pull (ECR キャッシュがあれば短縮)
        └─→ container 起動 + healthCheck
```

ECS Fargate では image を ECR に置き、`imagePullPolicy=Always` を避けることでトータルを 15 秒前後に抑えられる。EKS Autopilot はコントロールプレーン処理が加わるため、同条件で若干遅くなる傾向がある。

## 選択指針

- **シンプルな AWS 専用ワークロード + ノード管理ゼロ** → ECS Fargate
- **k8s エコシステムが必要 + ノード管理ゼロ** → EKS Fargate または EKS Autopilot
- **コスト最適化・GPU・DaemonSet・細かい制御** → Karpenter + EC2
- **移行中やハイブリッド** → EKS で Fargate と EC2 ノードを混在 (Fargate Profile で namespace 分割)
