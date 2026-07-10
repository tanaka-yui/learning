# 11 Stream (非同期メッセージング: queue vs stream) — 設計仕様

- 作成日: 2026-07-10
- 章: `11_stream/`
- 参考構造: `07_network/`（README + docs 番号付き学習動線 + 実装ディレクトリ + docker-compose + Makefile）

## 目的

非同期処理を使う目的・ケースを整理し、queue 系（SQS / ActiveMQ）と data stream 系（Kafka / Kinesis Data Streams / Kinesis Data Firehose）の機能・料金・スループットの違いと使い分けを、同一ワークロードのハンズオン実装と選定ガイドで体系的に学ぶ。

特に「Firehose → Kinesis → Kafka(MSK)」のような乗り換え判断に使える**損益分岐点（料金）とスループット制約の具体的目安**を東京リージョン実単価で提示することが本章の肝。

## スコープ

- **概念**: 同期 vs 非同期、非同期化の目的（デカップリング / 負荷平準化 / ジョブオフロード / fan-out / イベント駆動）、queue vs stream の本質差、配信保証（at-most/at-least/exactly-once）、順序保証、冪等性
- **ハンズオン（4種、Go 実装）**: SQS（LocalStack）、ActiveMQ（公式イメージ）、Kafka（KRaft シングルノード）、Kinesis Data Streams（LocalStack）
- **docs のみ**: Kinesis Data Firehose（配送マネージドサービスでありローカル再現の学習効果が薄いため）
- **選定ガイド**: 機能比較マトリクス、東京リージョン実単価（執筆日・出典 URL 明記）、シナリオ別月額試算表、乗り換え目安と移行時の注意

スコープ外:

- RabbitMQ / Amazon MQ for RabbitMQ（broker 型 MQ は ActiveMQ で代表。docs 内で 1 段落言及可）
- EventBridge / SNS 単体の深掘り（fan-out の文脈で言及のみ）
- Kafka Streams / Flink などのストリーム処理エンジン（本章は「メッセージ基盤の選定」まで）
- exactly-once の実装ハンズオン（概念解説のみ）
- MSK / 実 AWS 環境へのデプロイ（料金比較の対象としては扱うが、コードは全てローカル）

## アーキテクチャ

```
11_stream/
├── README.md              # 概要 + 学習動線 + クイックスタート + ミドルウェア一覧表 + 環境注意
├── docker-compose.yml     # localstack / activemq / kafka
├── Makefile               # up/down/demo-*/verify
├── go.work
├── docs/
│   ├── 01_concepts.md         # 非同期処理の全体像・queue vs stream
│   ├── 02_sqs.md              # queue 代表（LocalStack ハンズオン）
│   ├── 03_activemq.md         # broker 型 MQ（ハンズオン）
│   ├── 04_kafka.md            # stream 代表（ハンズオン）
│   ├── 05_kinesis.md          # マネージド stream（LocalStack ハンズオン）
│   ├── 06_firehose.md         # 配送サービス（docs のみ）
│   └── 07_selection_guide.md  # ★選定ガイド: 使い分け・料金試算・乗り換え目安
└── apps/
    ├── sqs/       # main.go + producer.go + consumer.go + go.mod
    ├── activemq/  # 同上
    ├── kafka/     # 同上
    └── kinesis/   # 同上
```

## docs 設計

07_network の docs と同じ書式: 日本語長文プロース + 表 + `[apps/xxx/file.go:NN](../apps/xxx/file.go#LNN)` 形式のクリッカブルなコード参照。各ハンズオン章は「概念解説 → コードの読みどころ → make demo の実行手順と観察ポイント」の流れ。

### 01_concepts.md — 非同期処理の全体像

- 同期処理との対比で「なぜ非同期にするか」: デカップリング（障害の分離）、負荷平準化（バッファリング）、ジョブオフロード、fan-out、イベント駆動
- 使うべきケース例（画像変換、メール送信、注文処理、ログ収集）と使うべきでないケース（即時応答が必要、強整合性が必要）
- **queue と stream の本質差**（章全体の軸）:
  - queue = 「消費したら消える仕事の受け渡し」。競合コンシューマでスケール。メッセージは 1 回処理されたら消える
  - stream = 「追記専用ログの購読」。オフセットをコンシューマ側が管理。リプレイ可能。複数 consumer group が同一データを独立に読める
- 配信保証（at-most-once / at-least-once / exactly-once）、順序保証、冪等性の基礎
- 本章 5 サービスの queue/stream 軸マッピング表

### 02_sqs.md — queue の代表（ハンズオン）

- standard vs FIFO（順序・重複排除・スループット上限の差）
- visibility timeout の挙動（`consume --no-delete` で復活を観察）
- DLQ + maxReceiveCount（`demo-sqs-dlq` で失敗メッセージが DLQ に落ちるのを観察）
- long polling、at-least-once ゆえの重複と冪等処理の必要性

### 03_activemq.md — broker 型 MQ（ハンズオン）

- ブローカー常駐型の世界観（JMS 由来）、STOMP プロトコル
- queue（競合コンシューマ）と topic（pub/sub fan-out）の違いを同一ブローカーで観察
- ACK モード、永続化
- SQS との比較: マネージド従量課金 vs ブローカー管理、プロトコル標準性、オンプレ/クラウド可搬性

### 04_kafka.md — stream の代表（ハンズオン）

- partition / offset / consumer group / retention / replay
- キーによる partition 内順序保証
- なぜ高スループットか（追記ログ + ページキャッシュ/ゼロコピー + バッチング）
- デモ: 同一 topic を 2 つの consumer group で独立に読む、`--from-beginning` でリプレイ

### 05_kinesis.md — マネージド stream（ハンズオン）

- shard モデルと Kafka 対応表（shard ≈ partition、sequence number ≈ offset、KCL ≈ consumer group）
- API・制約差: GetRecords 5 回/秒/shard、read 2 MB/s/shard、write 1 MB/s or 1,000 records/s/shard、pull 型ポーリング、Enhanced Fan-Out の位置づけ
- オンデマンド vs プロビジョンドの選択
- デモ: `consume --shard-iterator-type TRIM_HORIZON` でリプレイ

### 06_firehose.md — 配送マネージドサービス（docs のみ）

- Firehose は「stream を消費して S3 / Redshift / OpenSearch へ配送する ETL サービス」であり consumer コードを書かないこと
- バッファリング（サイズ/時間閾値）＝ニアリアルタイムであることの意味
- Kinesis Data Streams をソースにできる関係性（DirectPUT vs KDS ソース）
- Lambda による変換、動的パーティショニング
- ローカルハンズオンを置かない理由を明記

### 07_selection_guide.md — 選定ガイド（★肝）

- 使い分けディシジョンツリー: ジョブ処理 → queue / イベントの複数消費・リプレイ → stream / 変換して S3 等へ流すだけ → Firehose
- 機能比較マトリクス（順序、リプレイ、fan-out、保持期間、スループット上限、運用負荷、プロトコル可搬性）
- **料金構造の比較**（東京リージョン実単価、執筆日と出典 URL 明記。実装時に Web で最新単価を調査）:
  - SQS: リクエスト数課金
  - Amazon MQ (ActiveMQ): ブローカーインスタンス時間 + ストレージ課金
  - Kinesis Data Streams: プロビジョンド（shard 時間 + PUT ペイロード）/ オンデマンド（GB 課金）
  - Kinesis Data Firehose: 取り込み GB 課金（+ 変換・配送オプション）
  - MSK: broker インスタンス時間 + ストレージ（+ MSK Serverless の GB 課金）
  - （参考）セルフホスト Kafka on EC2
- **試算表**: 1 MB/s・10 MB/s・50 MB/s の 3 シナリオ × 全サービスの月額横並び比較（計算過程を式で示し、単価改定時に再計算できるようにする）
- **乗り換え目安**:
  - Firehose → Kinesis: 損益分岐スループット（GB 課金 vs shard 課金のクロスポイント）と、リアルタイム性・複数コンシューマ要件が出た時
  - Kinesis → MSK/Kafka: shard 数・月額のクロスポイント、shard 制約（read/write 上限、GetRecords 制限）が設計を歪め始める兆候
  - SQS → Kafka: リクエスト課金が月額いくらを超えたら、あるいは fan-out/リプレイ要件が出たら
- 移行時の注意: API 非互換（SDK 書き換え）、デュアルライト期間、オフセット/シーケンスの引き継ぎ不可、コンシューマの冪等性が移行の前提になること

## コード設計

### 共通方針

- 4 つの app は**同一ワークロード「注文イベント（JSON）を送る → 受ける」**に統一し、コードの差分＝ミドルウェアの概念差として読めるようにする
- 各 app は独立 go.mod、`go.work` で束ねる（07_network と同じ）
- サブコマンド型 CLI: `go run . produce -n 10` / `go run . consume`
- 構成: `main.go`（CLI パース）+ `producer.go` + `consumer.go`
- 学習コードとしてシンプル優先。リトライ・メトリクス等の実運用装備は入れない

### app ごとの特徴フラグ（ミドルウェア固有挙動のデモ用）

| app | ライブラリ | 特徴フラグ | 観察できる概念 |
|---|---|---|---|
| sqs | aws-sdk-go-v2 | `consume --no-delete` | visibility timeout による再配信 / DLQ |
| activemq | go-stomp | `produce/consume --topic` | queue（競合）vs topic（fan-out） |
| kafka | franz-go | `consume --group <name>` / `--from-beginning` | consumer group / リプレイ |
| kinesis | aws-sdk-go-v2 | `consume --shard-iterator-type TRIM_HORIZON\|LATEST` | shard iterator / リプレイ |

- ライブラリの最新 API は実装時に Context7 で確認する

### docker-compose.yml（3 コンテナ）

| サービス | イメージ | ポート | 備考 |
|---|---|---|---|
| localstack | `localstack/localstack` | 4566 | SQS + Kinesis。init スクリプト（`/etc/localstack/init/ready.d/`）で queue / DLQ / stream を自動作成 |
| activemq | `apache/activemq-classic` | 61613 (STOMP), 8161 (管理 UI) | |
| kafka | `apache/kafka`（KRaft シングルノード） | 9092 | topic は起動時 init（`kafka-topics.sh`）で明示作成 |

### Makefile

```
make up / down
make demo-sqs / demo-sqs-dlq
make demo-activemq / demo-activemq-topic
make demo-kafka / demo-kafka-replay
make demo-kinesis
make verify        # 全デモを順に流す E2E スモーク
```

## エラーハンドリング

- compose 未起動時の接続エラーは「`make up` を先に実行してください」という明確なメッセージに変換して即終了
- それ以外の防御的コードは書かない（学習コードのため）

## 検証

- クライアントコードは実ミドルウェアなしでは意味のあるテストにならないため、**compose 起動を前提にした `make verify`** を検証手段とする: 各 demo を実行し、期待メッセージ数の受信（および DLQ 移動・fan-out 到達などの期待挙動）を確認して exit code で判定
- 純粋ロジック（メッセージ整形等）が出た場合のみユニットテストを追加
- docs 内のコード参照（file#L 行番号）は実装完了後に実ファイルと突き合わせて検証

## README.md

07_network/README.md と同じ構成:

1. 章の概要（2〜3 文）
2. 学習動線（docs 01→07 の番号付きリンク）
3. クイックスタート（make コマンド一覧）
4. ミドルウェア一覧表（app / 接続先ポート / 分類 queue|stream / docs リンク）
5. 環境注意（LocalStack のダミー credential、ポート衝突、料金情報の執筆日）

## 成功基準

1. `make up && make verify` が全デモを通しで成功させる
2. docs 01〜07 が揃い、07_selection_guide に東京リージョン実単価（出典・執筆日付き）と 3 シナリオ試算表・乗り換え目安が載っている
3. README から 07_network と同じ動線（概要 → docs → クイックスタート）で学習を開始できる
4. 各 docs のコード参照が実ファイル・行番号と一致している
