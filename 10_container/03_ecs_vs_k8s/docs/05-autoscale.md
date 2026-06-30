# Autoscaling: ECS vs Kubernetes

## 概念対応表

| 概念 | ECS | Kubernetes |
|---|---|---|
| 水平スケール (ワークロード) | Service Auto Scaling (Application Auto Scaling) | HPA (Horizontal Pod Autoscaler) |
| 垂直スケール (ワークロード) | ECS 単体では非対応 (Compute Optimizer が推奨値を提案) | VPA (Vertical Pod Autoscaler) |
| 水平スケール (ノード) | Capacity Provider + EC2 ASG / Fargate (自動) | Cluster Autoscaler / Karpenter |
| イベント駆動スケール | EventBridge → RunTask (カスタム実装) | KEDA (Kubernetes Event-Driven Autoscaling) |

## ECS Service Auto Scaling

Application Auto Scaling を使い、ターゲット追跡 / ステップ / スケジュールの 3 ポリシーを組み合わせられる。

```
CloudWatch メトリクス (CPU / ECSServiceAverageCPUUtilization 等)
  └─→ Application Auto Scaling
        └─→ ECS Service の desiredCount を増減
```

スケールイン保護 (`enableExecuteCommand` + `capacityProviderStrategy` の `minimumScalingStepSize`) で急激な縮退を防ぐ。

## Kubernetes HPA + VPA

HPA は `Deployment` / `StatefulSet` の replica 数を自動調整する。Metrics Server またはカスタムメトリクスアダプタからメトリクスを取得する。

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
spec:
  scaleTargetRef:
    kind: Deployment
    name: demo-api
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 60
```

VPA は Pod の `requests` / `limits` を動的に調整する。HPA と組み合わせる場合は CPU メトリクスの競合に注意が必要。

## ノードスケール: Cluster Autoscaler vs Karpenter

| 項目 | Cluster Autoscaler | Karpenter |
|---|---|---|
| 設定単位 | ASG ごと | NodePool (要件宣言) |
| 起動速度 | 普通 (ASG warm-up あり) | 速い (EC2 Fleet API 直呼び) |
| スポット活用 | ASG の設定に依存 | NodePool で複数インスタンスタイプを柔軟指定 |
| bin-packing | 保守的 | 積極的 (consolidation 機能) |

Karpenter の詳細な構成・オーバーレイ設定は [10-2 クラスタ構築](../../02_service_mesh/) を参照。

## KEDA によるイベント駆動スケール

KEDA は SQS キューの深さや Kafka lag などの外部イベントを元に Pod をゼロから N にスケールできる。ECS 側で同等のことをするには EventBridge + Lambda + RunTask をカスタム実装する必要があり、KEDA の方が宣言的で管理が容易。

```
SQS キュー深さ
  └─→ KEDA ScaledObject
        └─→ HPA をバックエンドで操作
              └─→ Deployment replica を増減
```
