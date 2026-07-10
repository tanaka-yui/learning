# 11_stream: 非同期メッセージング学習プロジェクト

queue（SQS / ActiveMQ）と stream（Kafka / Kinesis）という 2 系統の非同期メッセージングを、同一の「注文」ワークロードを 4 通りの Go 実装で動かして体感する章。配信保証・consumer group・replay・shard 制約といった違いから、料金とスループットに基づく乗り換え目安までを掴む。

## 学習動線

1. [01: 非同期メッセージングの全体像](docs/01_concepts.md)
2. [02: マネージド queue の消費ループと DLQ（SQS）](docs/02_sqs.md)
3. [03: ブローカー常駐型 MQ の queue と topic（ActiveMQ）](docs/03_activemq.md)
4. [04: 分散追記ログのパーティション・オフセット・コンシューマグループ（Kafka）](docs/04_kafka.md)
5. [05: shard 制約と Kafka 対応（Kinesis）](docs/05_kinesis.md)
6. [06: consumer を書かないという設計（Firehose）](docs/06_firehose.md)
7. [07: 選定ガイド — 料金とスループットから見る使い分け](docs/07_selection_guide.md)

## クイックスタート

```bash
make up                  # LocalStack + ActiveMQ + Kafka 起動
make demo-sqs            # queue: 送って受けて消える
make demo-sqs-dlq        # visibility timeout と DLQ
make demo-activemq       # broker 型 queue
make demo-activemq-topic # topic の fan-out
make demo-kafka          # stream: consumer group
make demo-kafka-replay   # stream: リプレイ
make demo-kinesis        # マネージド stream
make verify              # 全デモ通し実行
make down
```

## ミドルウェア一覧表

| app | 接続先 | 分類 | 特徴デモ | docs |
|---|---|---|---|---|
| sqs       | localhost:4566 | queue  | DLQ            | [02](docs/02_sqs.md) |
| activemq  | localhost:61613 | queue+topic | fan-out    | [03](docs/03_activemq.md) |
| kafka     | localhost:9092 | stream | replay          | [04](docs/04_kafka.md) |
| kinesis   | localhost:4566 | stream | shard iterator  | [05](docs/05_kinesis.md) |

Firehose は Go 実装を持たず docs のみ（[06](docs/06_firehose.md)）。consumer を書かずにバッファリング配信する設計を扱う。

## 環境注意

- **LocalStack バージョン**: `docker-compose.yml` は `localstack/localstack:3.8.1` に固定している。`latest` タグの新しいバージョンは Community 版の SQS/Kinesis でも有料の auth token を要求するため、意図的にピン留めしている。
- **AWS 認証情報**: SQS/Kinesis アプリはダミー credential `test`/`test`（コード内に固定）で LocalStack にのみ接続する。実 AWS アカウントへは一切アクセスしない。
- **ポート**: `4566`（LocalStack）、`61613`/`8161`（ActiveMQ STOMP / 管理 UI、`8161` は http://localhost:8161 で admin/admin）、`9092`（Kafka）が衝突していないこと。
- `demo-sqs-dlq` は visibility timeout 待ちのため実行に 20 秒程度かかる。
- **Go バージョン**: `go.work` は `go 1.26` を要求し、`toolchain go1.26.0` でツールチェイン自動DLを指定。ローカル Go が 1.25 系でも `go test`/`go build` は走るが、LSP は 1.26 を手動インストールしないと import エラーを誤検知する。
- `docs/07_selection_guide.md` の料金は執筆日（2026-07-10）時点の東京リージョン（ap-northeast-1）単価。料金は変動するため参照時は最新情報を確認すること。
