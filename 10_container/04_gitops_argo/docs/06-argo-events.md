# 06 Argo Events

実装参照: `apps/events/`

## アーキテクチャ概要

Argo Events はイベント駆動型のワークフロートリガーシステム。3 つのコンポーネントで構成される。

```
EventSource → (EventBus) → Sensor → Trigger → Workflow/Job/HTTP など
```

1. **EventSource**: 外部イベントを受信して EventBus に publish する
2. **EventBus**: イベントのメッセージブローカー
3. **Sensor**: EventBus を subscribe し、条件一致時に Trigger を実行する

## EventBus

メッセージブローカーの実装を選択する。

| 種類 | 特徴 |
|---|---|
| NATS (旧) | デフォルト。シンプルだが At-most-once |
| JetStream (推奨) | NATS の持続化層。At-least-once 保証。本章実装 |
| Kafka | 既存 Kafka クラスタと統合する場合 |

```yaml
# apps/events/eventbus.yaml
spec:
  jetstream:
    version: "2.10.10"
    replicas: 3    # 本番は奇数台で HA
```

## EventSource の種類

| 種類 | 用途 |
|---|---|
| webhook | HTTP POST で外部からイベントを受信 (本章実装) |
| S3 | S3 バケットへのオブジェクト作成/削除イベント |
| Kafka | Kafka トピックからのメッセージ受信 |
| Cron | cron 式によるスケジュールイベント |
| GitHub / GitLab | PR / push webhook (GitHub App 連携) |
| GCP Pub/Sub | Google Cloud Pub/Sub メッセージ受信 |
| SQS | AWS SQS キューからのメッセージ受信 |

```yaml
# apps/events/eventsource-webhook.yaml (抜粋)
spec:
  service:
    ports: [{ port: 12000, targetPort: 12000 }]
  webhook:
    example:
      port: "12000"
      endpoint: /example
      method: POST
```

## Sensor: Dependencies + Triggers

Sensor は `dependencies` でどの EventSource のどのイベントを待つかを定義し、条件が揃ったら `triggers` を実行する。

```yaml
# apps/events/sensor-trigger-workflow.yaml (抜粋)
spec:
  template:
    serviceAccountName: operate-workflow-sa
  dependencies:
  - name: webhook-dep
    eventSourceName: webhook
    eventName: example
  triggers:
  - template:
      name: submit-workflow
      k8s:
        operation: create
        source:
          resource:
            apiVersion: argoproj.io/v1alpha1
            kind: Workflow
```

### Trigger の種類

| Trigger | 用途 |
|---|---|
| k8s | Kubernetes リソースを create/update/delete |
| http | 任意の HTTP エンドポイントを呼び出す |
| aws-lambda | Lambda 関数を invoke |
| aws-sqs | SQS にメッセージを送信 |
| argoWorkflow | Argo Workflows を直接 submit |
| log | ログ出力のみ (デバッグ用) |

## RBAC 注意点 (Task 6 gotcha)

Sensor が Workflow を作成するには、`template.serviceAccountName` に指定した ServiceAccount (本章では `operate-workflow-sa`) に以下の権限が必要:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: { name: operate-workflow-role, namespace: argo }
rules:
- apiGroups: [argoproj.io]
  resources: [workflows]
  verbs: [create]
---
kind: RoleBinding
# operate-workflow-sa を bind
```

`operate-workflow-sa` が Argo Events の Namespace (`argo-events`) と Workflow の Namespace (`argo`) 両方に存在する必要があることに注意。Namespace をまたぐ場合は ClusterRole が必要になる。

## EventBridge との対応

| Argo Events | AWS EventBridge |
|---|---|
| EventBus (JetStream) | EventBridge Event Bus |
| EventSource (webhook) | EventBridge API Destination |
| EventSource (SQS) | SQS → EventBridge Pipe |
| EventSource (Cron) | EventBridge Scheduler |
| Sensor | EventBridge Rule |
| Trigger (k8s) | EventBridge Rule Target (ECS Task / Lambda) |
| Sensor dependencies (複数条件) | EventBridge Pipe フィルタ / Step Functions |

LocalStack Community では EventBridge Rule/Target がサポートされており、Terraform で定義・確認ができる (`comparison/aws/terraform/eventbridge.tf`)。

## 動作確認

```bash
# EventSource Pod が Running かを確認
kubectl -n argo-events get pods

# webhook にイベントを送信
kubectl -n argo-events port-forward svc/webhook-eventsource-svc 12000:12000 &
curl -X POST http://localhost:12000/example -d '{"message": "test"}'

# Sensor のログで Trigger 実行を確認
kubectl -n argo-events logs deploy/webhook-to-workflow-sensor
```
