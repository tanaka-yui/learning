# 11_stream（非同期メッセージング: queue vs stream）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** SQS / ActiveMQ / Kafka / Kinesis の同一ワークロード（注文イベント）ハンズオン 4 種と、Firehose 含む 5 サービスの選定ガイド（東京リージョン実単価・試算・乗り換え目安）を持つ学習章 `11_stream/` を作る。

**Architecture:** 07_network と同じ「README + docs 番号付き学習動線 + 実装ディレクトリ + docker-compose + Makefile」。apps/ 配下の 4 つの Go CLI（produce/consume サブコマンド）は独立 go.mod で go.work に束ね、検証は compose 起動前提の `make verify`（各 demo の exit code）で行う。

**Tech Stack:** Go 1.26 / aws-sdk-go-v2（sqs, kinesis）/ franz-go（kafka）/ go-stomp v3（activemq）/ LocalStack / apache/activemq-classic / apache/kafka（KRaft）

**Spec:** `docs/superpowers/specs/2026-07-10-11-stream-async-messaging-design.md`

## Global Constraints

- Go 1.26（`go.work` は `go 1.26` + `toolchain go1.26.0`。07_network と同一）
- docs は日本語長文プロース + 表 + `[apps/xxx/file.go:NN](../apps/xxx/file.go#LNN)` 形式のクリッカブルなコード参照（07_network/docs と同じ書式）
- 使用ライブラリは `github.com/aws/aws-sdk-go-v2`（+ config, credentials, service/sqs, service/kinesis）、`github.com/twmb/franz-go`、`github.com/go-stomp/stomp/v3` のみ。実装時に API が本計画のコードと異なる場合は Context7 で最新 API を確認して合わせる
- 学習コードのためリトライ・メトリクス等の実運用装備は書かない。接続エラーのみ「`make up` を先に実行してください」への変換を行う
- リソース名は固定: SQS queue `orders` / DLQ `orders-dlq`（VisibilityTimeout=5s, maxReceiveCount=2）、ActiveMQ `/queue/orders` `/topic/orders`、Kafka topic `orders`（partitions=3）、Kinesis stream `orders`（shard=1）。リージョンは `ap-northeast-1`、LocalStack credential は `test`/`test`
- コミットメッセージは repo 慣習に従う: `feat(11): ...` / `docs(11): ...`
- `make -j` は想定しない（verify は直列実行前提）

---

### Task 1: インフラ土台（docker-compose + LocalStack init + Makefile 骨格）

**Files:**
- Create: `11_stream/docker-compose.yml`
- Create: `11_stream/localstack-init/init-aws.sh`
- Create: `11_stream/scripts/wait-ready.sh`
- Create: `11_stream/Makefile`

**Interfaces:**
- Consumes: なし（先頭タスク）
- Produces: `localhost:4566`（LocalStack: SQS queue `orders`/`orders-dlq`, Kinesis stream `orders`）、`localhost:61613`（ActiveMQ STOMP, 認証 admin/admin）、`localhost:9092`（Kafka, topic `orders` partitions=3）、`make up`/`make down`/`make logs`

- [ ] **Step 1: docker-compose.yml を書く**

```yaml
services:
  localstack:
    image: localstack/localstack
    ports: ["4566:4566"]
    environment:
      - SERVICES=sqs,kinesis
      - AWS_DEFAULT_REGION=ap-northeast-1
    volumes:
      - ./localstack-init:/etc/localstack/init/ready.d

  activemq:
    image: apache/activemq-classic
    ports:
      - "61613:61613"   # STOMP
      - "8161:8161"     # 管理 Web UI (admin/admin)
    environment:
      ACTIVEMQ_CONNECTION_USER: admin
      ACTIVEMQ_CONNECTION_PASSWORD: admin

  kafka:
    image: apache/kafka
    ports: ["9092:9092"]
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      # 内部(コンテナ間)は kafka:29092、ホストからは localhost:9092 で到達する 2 listener 構成
      KAFKA_LISTENERS: PLAINTEXT://:29092,CONTROLLER://:29093,PLAINTEXT_HOST://:9092
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:29092,PLAINTEXT_HOST://localhost:9092
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:29093
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_MIN_ISR: 1
      KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS: 0
    healthcheck:
      test: ["CMD-SHELL", "/opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:29092 --list >/dev/null 2>&1"]
      interval: 5s
      timeout: 10s
      retries: 12

  kafka-init:
    image: apache/kafka
    depends_on:
      kafka:
        condition: service_healthy
    entrypoint:
      - /bin/sh
      - -c
      - >
        /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:29092
        --create --if-not-exists --topic orders --partitions 3 --replication-factor 1
```

- [ ] **Step 2: LocalStack init スクリプトを書く**

`11_stream/localstack-init/init-aws.sh`:

```bash
#!/bin/bash
set -euo pipefail

# DLQ を先に作り、ARN を本体キューの RedrivePolicy に埋め込む
awslocal sqs create-queue --queue-name orders-dlq
DLQ_ARN=$(awslocal sqs get-queue-attributes \
  --queue-url "http://localhost:4566/000000000000/orders-dlq" \
  --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)

awslocal sqs create-queue --queue-name orders --attributes "{
  \"VisibilityTimeout\": \"5\",
  \"RedrivePolicy\": \"{\\\"deadLetterTargetArn\\\":\\\"${DLQ_ARN}\\\",\\\"maxReceiveCount\\\":\\\"2\\\"}\"
}"

awslocal kinesis create-stream --stream-name orders --shard-count 1

echo "init done: sqs orders(+orders-dlq), kinesis orders"
```

実行権限を付ける: `chmod +x 11_stream/localstack-init/init-aws.sh`

- [ ] **Step 3: 起動待ちスクリプトを書く**

`11_stream/scripts/wait-ready.sh`:

```bash
#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."

printf "localstack"
until docker compose exec -T localstack awslocal sqs get-queue-url --queue-name orders >/dev/null 2>&1; do
  printf .; sleep 2
done
echo " ok"

printf "kafka"
until docker compose exec -T kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:29092 --list 2>/dev/null | grep -q '^orders$'; do
  printf .; sleep 2
done
echo " ok"

printf "activemq"
until nc -z localhost 61613 >/dev/null 2>&1; do
  printf .; sleep 2
done
echo " ok"
```

実行権限を付ける: `chmod +x 11_stream/scripts/wait-ready.sh`

- [ ] **Step 4: Makefile 骨格を書く**

`11_stream/Makefile`（demo ターゲットは後続タスクで追記する）:

```make
.PHONY: up down logs

up:
	docker compose up -d
	./scripts/wait-ready.sh

down:  ; docker compose down
logs:  ; docker compose logs -f
```

- [ ] **Step 5: 起動確認**

Run: `cd 11_stream && make up`
Expected: `localstack... ok` / `kafka... ok` / `activemq ok` が表示されて exit 0

Run: `docker compose exec -T localstack awslocal sqs list-queues && docker compose exec -T localstack awslocal kinesis list-streams`
Expected: `orders` と `orders-dlq` の QueueUrl、StreamNames に `orders`

- [ ] **Step 6: Commit**

```bash
git add 11_stream/docker-compose.yml 11_stream/localstack-init 11_stream/scripts 11_stream/Makefile
git commit -m "feat(11): docker-compose + LocalStack init + Makefile 骨格"
```

---

### Task 2: sqs app（produce/consume + visibility timeout/DLQ デモ）

**Files:**
- Create: `11_stream/apps/sqs/go.mod`
- Create: `11_stream/apps/sqs/main.go`
- Create: `11_stream/apps/sqs/producer.go`
- Create: `11_stream/apps/sqs/consumer.go`
- Create: `11_stream/go.work`
- Modify: `11_stream/Makefile`（demo-sqs / demo-sqs-dlq 追記）

**Interfaces:**
- Consumes: Task 1 の LocalStack（queue `orders` / `orders-dlq`、VisibilityTimeout=5s、maxReceiveCount=2）
- Produces: CLI `go run ./apps/sqs produce -n <N>` / `go run ./apps/sqs consume -max <N> [--no-delete] [--queue <name>]`（consume は受信数 < max で exit 1）。`Order` 構造体 `{id, item, amount, created_at}`（JSON）— 以降の全 app が同型を各自定義する

- [ ] **Step 1: go.mod / go.work を作る**

```bash
cd 11_stream/apps/sqs && go mod init stream/sqs
cd ../.. && cat > go.work <<'EOF'
go 1.26

toolchain go1.26.0

use ./apps/sqs
EOF
```

- [ ] **Step 2: main.go（CLI 入口 + Order 型 + クライアント生成 + 接続エラー変換）**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// Order は 4 つの app 共通のワークロード（注文イベント）。
type Order struct {
	ID        string `json:"id"`
	Item      string `json:"item"`
	Amount    int    `json:"amount"`
	CreatedAt string `json:"created_at"`
}

func newOrder(i int) Order {
	return Order{
		ID:        fmt.Sprintf("order-%04d", i),
		Item:      "book",
		Amount:    1000 + i,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
}

func newSQSClient(ctx context.Context) (*sqs.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("ap-northeast-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		return nil, err
	}
	return sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	}), nil
}

func connectHint(err error) error {
	if err != nil && (strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such host")) {
		return fmt.Errorf("ミドルウェアに接続できません。`make up` を先に実行してください: %w", err)
	}
	return err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sqs <produce|consume> [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "produce":
		err = runProduce(os.Args[2:])
	case "consume":
		err = runConsume(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: producer.go**

```go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func runProduce(args []string) error {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	n := fs.Int("n", 5, "number of messages")
	queue := fs.String("queue", "orders", "queue name")
	fs.Parse(args)

	ctx := context.Background()
	client, err := newSQSClient(ctx)
	if err != nil {
		return err
	}
	urlOut, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: queue})
	if err != nil {
		return connectHint(err)
	}
	for i := 1; i <= *n; i++ {
		body, _ := json.Marshal(newOrder(i))
		if _, err := client.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:    urlOut.QueueUrl,
			MessageBody: aws.String(string(body)),
		}); err != nil {
			return connectHint(err)
		}
		fmt.Printf("sent: %s\n", body)
	}
	return nil
}
```

- [ ] **Step 4: consumer.go**

```go
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func runConsume(args []string) error {
	fs := flag.NewFlagSet("consume", flag.ExitOnError)
	max := fs.Int("max", 5, "expected number of messages")
	queue := fs.String("queue", "orders", "queue name")
	noDelete := fs.Bool("no-delete", false, "receive without deleting (visibility timeout demo)")
	fs.Parse(args)

	ctx := context.Background()
	client, err := newSQSClient(ctx)
	if err != nil {
		return err
	}
	urlOut, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: queue})
	if err != nil {
		return connectHint(err)
	}

	received, emptyPolls := 0, 0
	for received < *max && emptyPolls < 3 {
		out, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:                    urlOut.QueueUrl,
			MaxNumberOfMessages:         10,
			WaitTimeSeconds:             2, // long polling
			MessageSystemAttributeNames: []types.MessageSystemAttributeName{types.MessageSystemAttributeNameApproximateReceiveCount},
		})
		if err != nil {
			return connectHint(err)
		}
		if len(out.Messages) == 0 {
			emptyPolls++
			continue
		}
		for _, m := range out.Messages {
			received++
			fmt.Printf("received (receiveCount=%s): %s\n", m.Attributes["ApproximateReceiveCount"], *m.Body)
			if !*noDelete {
				if _, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
					QueueUrl:      urlOut.QueueUrl,
					ReceiptHandle: m.ReceiptHandle,
				}); err != nil {
					return err
				}
			}
		}
	}
	if received < *max {
		return fmt.Errorf("expected %d messages, got %d", *max, received)
	}
	return nil
}
```

- [ ] **Step 5: ビルドと依存解決**

Run: `cd 11_stream/apps/sqs && go mod tidy && go build ./...`
Expected: エラーなし（aws-sdk-go-v2 の API 名が違う場合は Context7 `/aws/aws-sdk-go-v2` で確認して修正）

- [ ] **Step 6: Makefile に demo ターゲット追記**

```make
.PHONY: demo-sqs demo-sqs-dlq

demo-sqs:
	go run ./apps/sqs produce -n 5
	go run ./apps/sqs consume -max 5

# maxReceiveCount=2 のため、2 回受信(削除なし)した後の 3 回目の受信試行で DLQ へ移動する。
# 3 回目の consume が「空」になるのが観察ポイントなので exit code は無視する。
demo-sqs-dlq:
	go run ./apps/sqs produce -n 1
	go run ./apps/sqs consume -max 1 --no-delete
	sleep 6
	go run ./apps/sqs consume -max 1 --no-delete
	sleep 6
	-go run ./apps/sqs consume -max 1
	go run ./apps/sqs consume -max 1 --queue orders-dlq
```

- [ ] **Step 7: デモ実行で検証**

Run: `cd 11_stream && make demo-sqs`
Expected: `sent:` × 5 → `received (receiveCount=1):` × 5、exit 0

Run: `make demo-sqs-dlq`
Expected: receiveCount=1 → receiveCount=2 と増え、3 回目は `expected 1 messages, got 0`（無視される）、最後に DLQ から receiveCount=1 で受信して exit 0

- [ ] **Step 8: Commit**

```bash
git add 11_stream/apps/sqs 11_stream/go.work 11_stream/Makefile
git commit -m "feat(11): sqs producer/consumer + visibility timeout/DLQ デモ"
```

---

### Task 3: activemq app（queue vs topic の fan-out デモ）

**Files:**
- Create: `11_stream/apps/activemq/go.mod`
- Create: `11_stream/apps/activemq/main.go`
- Create: `11_stream/apps/activemq/producer.go`
- Create: `11_stream/apps/activemq/consumer.go`
- Modify: `11_stream/go.work`（use 追記）
- Modify: `11_stream/Makefile`（demo-activemq / demo-activemq-topic 追記）

**Interfaces:**
- Consumes: Task 1 の ActiveMQ（STOMP localhost:61613、admin/admin）
- Produces: CLI `go run ./apps/activemq produce [-n N] [--topic]` / `consume [-max N] [--topic] [-timeout 15s]`（受信数 < max で exit 1）

- [ ] **Step 1: go.mod 作成 + go.work 追記**

```bash
cd 11_stream/apps/activemq && go mod init stream/activemq
cd ../.. && go work use ./apps/activemq
```

- [ ] **Step 2: main.go**

```go
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-stomp/stomp/v3"
)

type Order struct {
	ID        string `json:"id"`
	Item      string `json:"item"`
	Amount    int    `json:"amount"`
	CreatedAt string `json:"created_at"`
}

func newOrder(i int) Order {
	return Order{
		ID:        fmt.Sprintf("order-%04d", i),
		Item:      "book",
		Amount:    1000 + i,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
}

// destination で queue（競合コンシューマ）と topic（fan-out）を切り替えるのが STOMP 流。
func destination(topic bool) string {
	if topic {
		return "/topic/orders"
	}
	return "/queue/orders"
}

func dial() (*stomp.Conn, error) {
	conn, err := stomp.Dial("tcp", "localhost:61613",
		stomp.ConnOpt.Login("admin", "admin"),
	)
	if err != nil && strings.Contains(err.Error(), "connection refused") {
		return nil, fmt.Errorf("ActiveMQ に接続できません。`make up` を先に実行してください: %w", err)
	}
	return conn, err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: activemq <produce|consume> [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "produce":
		err = runProduce(os.Args[2:])
	case "consume":
		err = runConsume(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: producer.go**

```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
)

func runProduce(args []string) error {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	n := fs.Int("n", 5, "number of messages")
	topic := fs.Bool("topic", false, "send to /topic/orders instead of /queue/orders")
	fs.Parse(args)

	conn, err := dial()
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	dest := destination(*topic)
	for i := 1; i <= *n; i++ {
		body, _ := json.Marshal(newOrder(i))
		if err := conn.Send(dest, "application/json", body); err != nil {
			return err
		}
		fmt.Printf("sent to %s: %s\n", dest, body)
	}
	return nil
}
```

- [ ] **Step 4: consumer.go**

```go
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/go-stomp/stomp/v3"
)

func runConsume(args []string) error {
	fs := flag.NewFlagSet("consume", flag.ExitOnError)
	max := fs.Int("max", 5, "expected number of messages")
	topic := fs.Bool("topic", false, "subscribe /topic/orders instead of /queue/orders")
	timeout := fs.Duration("timeout", 15*time.Second, "overall deadline")
	fs.Parse(args)

	conn, err := dial()
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	dest := destination(*topic)
	sub, err := conn.Subscribe(dest, stomp.AckAuto)
	if err != nil {
		return err
	}

	received := 0
	deadline := time.After(*timeout)
	for received < *max {
		select {
		case msg := <-sub.C:
			if msg.Err != nil {
				return msg.Err
			}
			received++
			fmt.Printf("received from %s: %s\n", dest, msg.Body)
		case <-deadline:
			return fmt.Errorf("expected %d messages, got %d (timeout)", *max, received)
		}
	}
	return nil
}
```

- [ ] **Step 5: ビルド**

Run: `cd 11_stream/apps/activemq && go mod tidy && go build ./...`
Expected: エラーなし（go-stomp/v3 の API 名が違う場合は Context7 で `stomp` を再検索して修正）

- [ ] **Step 6: Makefile に demo ターゲット追記**

```make
.PHONY: demo-activemq demo-activemq-topic

# queue: 先に produce しても broker が保持し、後から consume できる
demo-activemq:
	go run ./apps/activemq produce -n 5
	go run ./apps/activemq consume -max 5

# topic: 購読者が「先に」いないと届かない。2 購読者が同じ 3 通を全部受け取る = fan-out
demo-activemq-topic:
	@go run ./apps/activemq consume --topic -max 3 & C1=$$!; \
	go run ./apps/activemq consume --topic -max 3 & C2=$$!; \
	sleep 2; \
	go run ./apps/activemq produce --topic -n 3; \
	wait $$C1 && wait $$C2
```

- [ ] **Step 7: デモ実行で検証**

Run: `cd 11_stream && make demo-activemq`
Expected: sent × 5 → received × 5、exit 0

Run: `make demo-activemq-topic`
Expected: sent × 3 に対し received 行が合計 6 行（各購読者 3 行ずつ）、exit 0

- [ ] **Step 8: Commit**

```bash
git add 11_stream/apps/activemq 11_stream/go.work 11_stream/Makefile
git commit -m "feat(11): activemq producer/consumer + queue/topic fan-out デモ"
```

---

### Task 4: kafka app（consumer group / replay デモ）

**Files:**
- Create: `11_stream/apps/kafka/go.mod`
- Create: `11_stream/apps/kafka/main.go`
- Create: `11_stream/apps/kafka/producer.go`
- Create: `11_stream/apps/kafka/consumer.go`
- Modify: `11_stream/go.work`（use 追記）
- Modify: `11_stream/Makefile`（demo-kafka / demo-kafka-replay 追記）

**Interfaces:**
- Consumes: Task 1 の Kafka（localhost:9092、topic `orders` partitions=3）
- Produces: CLI `go run ./apps/kafka produce [-n N]`（key=注文ID で partition 分散） / `consume -group <name> [-max N] [--from-beginning] [-timeout 15s]`（受信数 < max で exit 1）

- [ ] **Step 1: go.mod 作成 + go.work 追記**

```bash
cd 11_stream/apps/kafka && go mod init stream/kafka
cd ../.. && go work use ./apps/kafka
```

- [ ] **Step 2: main.go**

```go
package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Order struct {
	ID        string `json:"id"`
	Item      string `json:"item"`
	Amount    int    `json:"amount"`
	CreatedAt string `json:"created_at"`
}

func newOrder(i int) Order {
	return Order{
		ID:        fmt.Sprintf("order-%04d", i),
		Item:      "book",
		Amount:    1000 + i,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
}

func connectHint(err error) error {
	if err != nil && (strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "unable to dial")) {
		return fmt.Errorf("Kafka に接続できません。`make up` を先に実行してください: %w", err)
	}
	return err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: kafka <produce|consume> [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "produce":
		err = runProduce(os.Args[2:])
	case "consume":
		err = runConsume(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: producer.go**

```go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func runProduce(args []string) error {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	n := fs.Int("n", 5, "number of messages")
	fs.Parse(args)

	cl, err := kgo.NewClient(
		kgo.SeedBrokers("localhost:9092"),
		kgo.RecordDeliveryTimeout(10*time.Second), // 未起動時に素早く失敗させる
	)
	if err != nil {
		return err
	}
	defer cl.Close()

	ctx := context.Background()
	for i := 1; i <= *n; i++ {
		o := newOrder(i)
		body, _ := json.Marshal(o)
		// key に注文 ID を使う → 同じ key は必ず同じ partition = key 単位の順序保証
		rec := &kgo.Record{Topic: "orders", Key: []byte(o.ID), Value: body}
		if err := cl.ProduceSync(ctx, rec).FirstErr(); err != nil {
			return connectHint(err)
		}
		fmt.Printf("sent: key=%s partition=%d offset=%d %s\n", o.ID, rec.Partition, rec.Offset, body)
	}
	return nil
}
```

- [ ] **Step 4: consumer.go**

```go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func runConsume(args []string) error {
	fs := flag.NewFlagSet("consume", flag.ExitOnError)
	max := fs.Int("max", 5, "expected number of messages")
	group := fs.String("group", "demo", "consumer group id")
	fromBeginning := fs.Bool("from-beginning", false, "start from the earliest offset when the group has no committed offset")
	timeout := fs.Duration("timeout", 15*time.Second, "overall deadline")
	fs.Parse(args)

	opts := []kgo.Opt{
		kgo.SeedBrokers("localhost:9092"),
		kgo.ConsumerGroup(*group),
		kgo.ConsumeTopics("orders"),
	}
	if *fromBeginning {
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return err
	}
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	received := 0
	for received < *max {
		fetches := cl.PollFetches(ctx)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			break
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			return connectHint(fmt.Errorf("fetch errors: %v", errs))
		}
		iter := fetches.RecordIter()
		for !iter.Done() {
			rec := iter.Next()
			received++
			fmt.Printf("received: group=%s partition=%d offset=%d key=%s %s\n",
				*group, rec.Partition, rec.Offset, rec.Key, rec.Value)
		}
	}
	// group の committed offset を確定させてから終了（次回は続きから読む）
	if err := cl.CommitUncommittedOffsets(context.Background()); err != nil {
		return err
	}
	if received < *max {
		return fmt.Errorf("expected %d messages, got %d", *max, received)
	}
	return nil
}
```

- [ ] **Step 5: ビルド**

Run: `cd 11_stream/apps/kafka && go mod tidy && go build ./...`
Expected: エラーなし（franz-go の API 名が違う場合は Context7 `/twmb/franz-go` で確認して修正）

- [ ] **Step 6: Makefile に demo ターゲット追記**

```make
.PHONY: demo-kafka demo-kafka-replay

demo-kafka:
	go run ./apps/kafka produce -n 5
	go run ./apps/kafka consume -group demo -max 5

# 新しい group 名 + --from-beginning = 再 produce せずに過去レコードを再消費（replay）
# group 名にタイムスタンプを入れて毎回「初見の group」にする
demo-kafka-replay:
	go run ./apps/kafka consume -group replay-$$(date +%s) --from-beginning -max 5
```

- [ ] **Step 7: デモ実行で検証**

Run: `cd 11_stream && make demo-kafka`
Expected: sent × 5（partition 0〜2 に分散）→ received × 5、exit 0

Run: `make demo-kafka-replay`
Expected: produce していないのに received × 5（demo-kafka で送った分を再読）、exit 0

Run: `make demo-kafka-replay` をもう一度
Expected: 再び received × 5（ログが残っている限り何度でも replay できる）

- [ ] **Step 8: Commit**

```bash
git add 11_stream/apps/kafka 11_stream/go.work 11_stream/Makefile
git commit -m "feat(11): kafka producer/consumer + consumer group/replay デモ"
```

---

### Task 5: kinesis app（shard iterator / replay デモ）

**Files:**
- Create: `11_stream/apps/kinesis/go.mod`
- Create: `11_stream/apps/kinesis/main.go`
- Create: `11_stream/apps/kinesis/producer.go`
- Create: `11_stream/apps/kinesis/consumer.go`
- Modify: `11_stream/go.work`（use 追記）
- Modify: `11_stream/Makefile`（demo-kinesis 追記）

**Interfaces:**
- Consumes: Task 1 の LocalStack Kinesis（stream `orders`、shard=1）
- Produces: CLI `go run ./apps/kinesis produce [-n N]` / `consume [-max N] [-iterator TRIM_HORIZON|LATEST] [-timeout 15s]`（受信数 < max で exit 1）

- [ ] **Step 1: go.mod 作成 + go.work 追記**

```bash
cd 11_stream/apps/kinesis && go mod init stream/kinesis
cd ../.. && go work use ./apps/kinesis
```

- [ ] **Step 2: main.go**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
)

type Order struct {
	ID        string `json:"id"`
	Item      string `json:"item"`
	Amount    int    `json:"amount"`
	CreatedAt string `json:"created_at"`
}

func newOrder(i int) Order {
	return Order{
		ID:        fmt.Sprintf("order-%04d", i),
		Item:      "book",
		Amount:    1000 + i,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
}

func newKinesisClient(ctx context.Context) (*kinesis.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("ap-northeast-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		return nil, err
	}
	return kinesis.NewFromConfig(cfg, func(o *kinesis.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	}), nil
}

func connectHint(err error) error {
	if err != nil && (strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such host")) {
		return fmt.Errorf("LocalStack に接続できません。`make up` を先に実行してください: %w", err)
	}
	return err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: kinesis <produce|consume> [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "produce":
		err = runProduce(os.Args[2:])
	case "consume":
		err = runConsume(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: producer.go**

```go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
)

func runProduce(args []string) error {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	n := fs.Int("n", 5, "number of records")
	fs.Parse(args)

	ctx := context.Background()
	client, err := newKinesisClient(ctx)
	if err != nil {
		return err
	}
	for i := 1; i <= *n; i++ {
		o := newOrder(i)
		body, _ := json.Marshal(o)
		// PartitionKey が shard を決める（Kafka の key ≈ PartitionKey）
		out, err := client.PutRecord(ctx, &kinesis.PutRecordInput{
			StreamName:   aws.String("orders"),
			PartitionKey: aws.String(o.ID),
			Data:         body,
		})
		if err != nil {
			return connectHint(err)
		}
		fmt.Printf("sent: shard=%s seq=%s %s\n", *out.ShardId, *out.SequenceNumber, body)
	}
	return nil
}
```

- [ ] **Step 4: consumer.go**

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
)

func runConsume(args []string) error {
	fs := flag.NewFlagSet("consume", flag.ExitOnError)
	max := fs.Int("max", 5, "expected number of records")
	iterType := fs.String("iterator", "TRIM_HORIZON", "shard iterator type: TRIM_HORIZON|LATEST")
	timeout := fs.Duration("timeout", 15*time.Second, "overall deadline")
	fs.Parse(args)

	ctx := context.Background()
	client, err := newKinesisClient(ctx)
	if err != nil {
		return err
	}

	// Kafka と違い consumer group はない。shard を列挙し iterator を自分で管理する
	ls, err := client.ListShards(ctx, &kinesis.ListShardsInput{StreamName: aws.String("orders")})
	if err != nil {
		return connectHint(err)
	}
	itOut, err := client.GetShardIterator(ctx, &kinesis.GetShardIteratorInput{
		StreamName:        aws.String("orders"),
		ShardId:           ls.Shards[0].ShardId,
		ShardIteratorType: types.ShardIteratorType(*iterType),
	})
	if err != nil {
		return err
	}

	iter := itOut.ShardIterator
	received := 0
	deadline := time.Now().Add(*timeout)
	for received < *max && time.Now().Before(deadline) {
		out, err := client.GetRecords(ctx, &kinesis.GetRecordsInput{
			ShardIterator: iter,
			Limit:         aws.Int32(100),
		})
		if err != nil {
			return connectHint(err)
		}
		for _, r := range out.Records {
			received++
			fmt.Printf("received: seq=%s %s\n", *r.SequenceNumber, r.Data)
		}
		iter = out.NextShardIterator
		if len(out.Records) == 0 {
			// 実 AWS では GetRecords は shard あたり 5 回/秒まで。ポーリング間隔を空ける
			time.Sleep(500 * time.Millisecond)
		}
	}
	if received < *max {
		return fmt.Errorf("expected %d records, got %d", *max, received)
	}
	return nil
}
```

- [ ] **Step 5: ビルド**

Run: `cd 11_stream/apps/kinesis && go mod tidy && go build ./...`
Expected: エラーなし

- [ ] **Step 6: Makefile に demo ターゲット追記**

```make
.PHONY: demo-kinesis

demo-kinesis:
	go run ./apps/kinesis produce -n 5
	go run ./apps/kinesis consume -iterator TRIM_HORIZON -max 5
```

- [ ] **Step 7: デモ実行で検証**

Run: `cd 11_stream && make demo-kinesis`
Expected: sent × 5（全て同一 shard）→ received × 5、exit 0。2 回実行すると TRIM_HORIZON なので過去分も含めて 5 件以上読める（-max 5 で打ち切り）

- [ ] **Step 8: Commit**

```bash
git add 11_stream/apps/kinesis 11_stream/go.work 11_stream/Makefile
git commit -m "feat(11): kinesis producer/consumer + shard iterator デモ"
```

---

### Task 6: make verify（全デモ E2E スモーク）

**Files:**
- Modify: `11_stream/Makefile`（verify 追記）

**Interfaces:**
- Consumes: Task 1〜5 の全 demo ターゲット（失敗時 exit != 0）
- Produces: `make verify`（compose 起動から全デモまで直列実行、全部通ると `ALL DEMOS PASSED`）

- [ ] **Step 1: verify ターゲット追記**

```make
.PHONY: verify

# 直列実行前提（-j 不可）。demo-kafka-replay は demo-kafka が先に produce している事に依存する
verify: up demo-sqs demo-sqs-dlq demo-activemq demo-activemq-topic demo-kafka demo-kafka-replay demo-kinesis
	@echo "ALL DEMOS PASSED"
```

- [ ] **Step 2: クリーン状態から通し実行**

Run: `cd 11_stream && make down && make verify`
Expected: 全デモが順に成功し最後に `ALL DEMOS PASSED`、exit 0

- [ ] **Step 3: 失敗系の確認（エラーメッセージ）**

Run: `make down && go run ./apps/sqs produce -n 1`
Expected: exit 1、stderr に「ミドルウェアに接続できません。`make up` を先に実行してください」を含む

Run: `make up`（次タスク以降のために復旧）

- [ ] **Step 4: Commit**

```bash
git add 11_stream/Makefile
git commit -m "feat(11): make verify で全デモ E2E スモーク"
```

---

### Task 7: docs/01_concepts.md（非同期処理の全体像）

**Files:**
- Create: `11_stream/docs/01_concepts.md`

**Interfaces:**
- Consumes: Task 2〜5 の実コード（コード参照の行番号は `grep -n` で実ファイルから取る）
- Produces: docs 02〜07 が前提とする用語定義（queue/stream、at-least-once、冪等性、fan-out）

**書式（docs 全タスク共通）:** 07_network/docs と同じ日本語長文プロース。`# 01_concepts: タイトル` → 導入 2〜3 文 → `---` 区切りの番号付きセクション。コード参照は `[apps/sqs/consumer.go:42](../apps/sqs/consumer.go#L42)` 形式で、執筆時に `grep -n <目印文字列> <file>` で行番号を確認して書く。分量目安 150〜250 行。

- [ ] **Step 1: 以下のセクション構成で執筆**

1. **イントロ — 同期処理の限界**: Web アプリが注文を受けて「メール送信・在庫更新・分析ログ」を同期で行うと、レスポンスは最も遅い処理に律速され、下流障害が注文受付自体を止める。この結合を切るのが非同期メッセージング
2. **非同期化の 5 つの目的**: デカップリング（障害の分離）/ 負荷平準化（スパイクをバッファで吸収）/ ジョブオフロード（重い処理を後ろへ）/ fan-out（1 イベントを複数系へ配る）/ イベント駆動（状態変化の通知で系を繋ぐ）— それぞれ具体例 1 つ（画像変換、セール時の注文、メール送信、注文イベント→在庫/分析/通知、マイクロサービス連携）
3. **使うべきでないケース**: 即時応答が必要（残高照会）、強整合が必要（在庫の確定引当）、単純な 1:1 同期 RPC で足りる場合。「非同期にした瞬間、結果整合・重複・順序という 3 つの複雑さを引き受ける」ことを明記
4. **queue と stream の本質差**（章の軸）: 比較表 —

| 観点 | queue（SQS / ActiveMQ） | stream（Kafka / Kinesis） |
|---|---|---|
| データの寿命 | 消費（ACK/削除）したら消える | 保持期間まで残る追記ログ |
| 読み手の位置管理 | ブローカーが未配信を管理 | コンシューマがオフセットを管理 |
| 複数コンシューマ | 競合（1 通は誰か 1 人） | group ごとに独立して全件読める |
| リプレイ | 不可（消えたら終わり） | 可能（オフセットを巻き戻す） |
| 典型用途 | ジョブ配布・タスクキュー | イベントログ・ストリーム分析・複数系配信 |

  それぞれ本章のデモとの対応を書く（queue の競合 → demo-sqs、stream のリプレイ → demo-kafka-replay）
5. **配信保証と冪等性**: at-most-once / at-least-once / exactly-once の定義、本章の 4 ミドルウェアは実用上すべて at-least-once であること、ゆえに「コンシューマの冪等化が非同期設計の前提」であること。demo-sqs-dlq の receiveCount 増加をコード参照付きで例示
6. **本章 5 サービスのマッピング**: 表 — SQS（マネージド queue）/ ActiveMQ（セルフホスト broker 型 MQ）/ Kafka（セルフホスト stream）/ Kinesis Data Streams（マネージド stream）/ Firehose（stream 配送サービス、consumer コードを書かない）。docs 02〜07 への読み進め案内で締める

- [ ] **Step 2: コード参照の行番号を検証**

Run: `grep -n "ApproximateReceiveCount" 11_stream/apps/sqs/consumer.go` などで参照行番号が本文と一致することを確認

- [ ] **Step 3: Commit**

```bash
git add 11_stream/docs/01_concepts.md
git commit -m "docs(11): 01_concepts 非同期処理の全体像と queue vs stream"
```

---

### Task 8: docs/02_sqs.md

**Files:**
- Create: `11_stream/docs/02_sqs.md`

**Interfaces:**
- Consumes: apps/sqs 実コード、Makefile の demo-sqs / demo-sqs-dlq、01_concepts の用語
- Produces: 07_selection_guide が参照する SQS の機能・制約の事実

- [ ] **Step 1: 以下のセクション構成で執筆**

1. **イントロ**: SQS は「サーバーを 1 台も持たない queue」。API 3 つ（Send/Receive/Delete）で成立する設計思想
2. **standard vs FIFO**: 順序保証なし・無制限スループット vs MessageGroupId 単位の順序・300〜3,000 TPS 上限（high throughput mode 含む）。重複排除（deduplication ID）は FIFO のみ
3. **visibility timeout の仕組み**: 「受信」はロックであり削除ではない。図解（テキストで時系列）: Receive → invisible(5s) → 削除しなければ再出現。[apps/sqs/consumer.go の --no-delete 分岐] をコード参照。**Receive と Delete が分かれていること自体が at-least-once の実装**であることを強調
4. **DLQ と maxReceiveCount**: RedrivePolicy の JSON（localstack-init/init-aws.sh をコード参照）、demo-sqs-dlq の出力を貼り「3 回目の受信試行が空になる = 毒メッセージの隔離」を解説
5. **long polling**: WaitTimeSeconds の意味（空振り API 課金の削減。SQS はリクエスト課金なので直接コストに効く — 07 章への伏線）
6. **make demo-sqs / demo-sqs-dlq の実行手順と観察ポイント**: 期待出力を貼り、receiveCount の増加、DLQ 到達を観察させる

- [ ] **Step 2: 行番号検証 + Commit**

```bash
git add 11_stream/docs/02_sqs.md
git commit -m "docs(11): 02_sqs visibility timeout と DLQ"
```

---

### Task 9: docs/03_activemq.md

**Files:**
- Create: `11_stream/docs/03_activemq.md`

**Interfaces:**
- Consumes: apps/activemq 実コード、demo-activemq / demo-activemq-topic
- Produces: 07_selection_guide が参照する broker 型 MQ の機能・運用負荷の事実

- [ ] **Step 1: 以下のセクション構成で執筆**

1. **イントロ**: ActiveMQ は JMS 由来の「ブローカー常駐型」MQ。SQS と違い自分でプロセスを飼う。管理 UI（localhost:8161, admin/admin）でキューの中身が見える
2. **queue と topic**: 同一ブローカーが 2 つの配信モデルを持つ。destination 名だけで切り替わる（[apps/activemq/main.go の destination 関数] をコード参照）。queue = 競合コンシューマ（ワーカー分散）、topic = fan-out（購読者全員に届く。ただし**購読していない間のメッセージは消える**）。demo-activemq-topic で「produce の前に consumer を起動する」順序に意味があることを解説
3. **STOMP / AMQP / OpenWire**: プロトコルが標準化されている＝クライアント言語が自由、他ブローカー（Artemis, RabbitMQ）への可搬性。SQS の独自 HTTPS API との対比
4. **ACK モードと永続化**: AckAuto / client ack、persistent メッセージ。broker 停止時の生存性
5. **SQS との使い分け**: 表 — 運用（マネージド vs 自前/Amazon MQ）、課金（リクエスト vs インスタンス時間）、プロトコル可搬性、順序、スループット天井。「AWS 内で完結し queue 相当で良ければ SQS が第一候補、JMS 資産・オンプレ・標準プロトコル要件があれば ActiveMQ/Amazon MQ」
6. **make demo-activemq / demo-activemq-topic の実行手順と観察ポイント**: fan-out で received が 6 行になる出力を貼る

- [ ] **Step 2: 行番号検証 + Commit**

```bash
git add 11_stream/docs/03_activemq.md
git commit -m "docs(11): 03_activemq broker 型 MQ と queue/topic"
```

---

### Task 10: docs/04_kafka.md

**Files:**
- Create: `11_stream/docs/04_kafka.md`

**Interfaces:**
- Consumes: apps/kafka 実コード、demo-kafka / demo-kafka-replay
- Produces: 05_kinesis が使う Kafka 側の概念（partition/offset/consumer group）、07 が参照するスループット根拠

- [ ] **Step 1: 以下のセクション構成で執筆**

1. **イントロ**: Kafka は queue ではなく「分散追記ログ」。メッセージは消費されても消えず、retention まで残る
2. **partition / offset**: topic は partition に分割された追記ログの集合。offset は「ログ内の位置」でありコンシューマの所有物。produce 出力の `partition=X offset=N` を貼って解説。key → partition の割当（[apps/kafka/producer.go の key 設定] をコード参照）と「key 単位の順序保証」
3. **consumer group**: group 内では partition を分担（競合 = queue 的）、group 間では独立（fan-out = pub/sub 的）。**queue と pub/sub を 1 つのモデルで包含する**のが Kafka の設計上の勝利点。committed offset は「group がどこまで読んだか」の記録に過ぎない
4. **replay**: demo-kafka-replay の仕組み（新 group + ConsumeResetOffset）。障害後の再処理・新システムへのバックフィル・バグ修正後の再集計という実務ユースケース
5. **なぜ高スループットか**: 追記シーケンシャル I/O、OS ページキャッシュ、sendfile ゼロコピー、バッチ圧縮。「速いのは魔法ではなく、ランダムアクセスを設計で排除したから」
6. **運用の現実**: broker/controller の面倒、rebalance、retention 容量管理 — 07 章の「マネージドに寄せるか」の伏線
7. **make demo-kafka / demo-kafka-replay の実行手順と観察ポイント**

- [ ] **Step 2: 行番号検証 + Commit**

```bash
git add 11_stream/docs/04_kafka.md
git commit -m "docs(11): 04_kafka partition/offset/consumer group/replay"
```

---

### Task 11: docs/05_kinesis.md

**Files:**
- Create: `11_stream/docs/05_kinesis.md`

**Interfaces:**
- Consumes: apps/kinesis 実コード、demo-kinesis、04_kafka の概念
- Produces: 07 が参照する shard 制約の事実（乗り換え目安の根拠）

- [ ] **Step 1: 以下のセクション構成で執筆**

1. **イントロ**: Kinesis Data Streams は「マネージド Kafka 的なもの」だが、抽象の切り方が違う。対応表 —

| Kafka | Kinesis | 備考 |
|---|---|---|
| partition | shard | スケール単位 |
| offset | sequence number | ログ内の位置 |
| key | partition key | ハッシュで shard 決定 |
| consumer group | KCL + DynamoDB checkpoint | **本体機能ではなくクライアント側ライブラリ** |
| retention (無制限可) | 24h〜365d | |

2. **shard の制約が全てを決める**: write 1 MB/s or 1,000 records/s、read 2 MB/s、GetRecords 5 回/秒 — **per shard** の固定枠。consumer.go の 500ms sleep をコード参照し「この sleep が実 AWS の GetRecords 制限の写し」と解説。複数アプリが同じ shard を読むと 2 MB/s を分け合う → Enhanced Fan-Out（消費者ごとに 2 MB/s、push 型）の位置づけ
3. **consumer group がないことの意味**: 本章の consumer は checkpoint を持たない（毎回 TRIM_HORIZON/LATEST から）。実務では KCL が DynamoDB に checkpoint を書く。Kafka では broker の機能だったものがクライアント責務になっている
4. **オンデマンド vs プロビジョンド**: shard 管理を AWS に任せる（GB 課金）か自分で握る（shard 時間課金）か。トラフィックが読めない初期はオンデマンド、定常が読めたらプロビジョンドが安くなる — 07 章への伏線
5. **make demo-kinesis の実行手順と観察ポイント**: TRIM_HORIZON で過去分から読めること（= stream であり queue ではない）

- [ ] **Step 2: 行番号検証 + Commit**

```bash
git add 11_stream/docs/05_kinesis.md
git commit -m "docs(11): 05_kinesis shard モデルと Kafka 対応"
```

---

### Task 12: docs/06_firehose.md（docs のみ、コードなし）

**Files:**
- Create: `11_stream/docs/06_firehose.md`

**Interfaces:**
- Consumes: 05_kinesis の概念
- Produces: 07 が参照する Firehose の位置づけ・制約の事実

- [ ] **Step 1: 以下のセクション構成で執筆**

1. **イントロ + 本章にハンズオンがない理由**: Firehose は producer/consumer を書くサービスではなく「stream を受けて S3/Redshift/OpenSearch/HTTP endpoint へ配送する完全マネージドのパイプ」。consumer コードが存在しないことこそが学習ポイントなので、ローカル再現より概念理解が重要（README の環境注意にも同旨を記載済みとする）
2. **バッファリングという本質**: buffering size（MB）/ interval（秒）の閾値に達するまで貯めてから書き出す = **リアルタイムではなくニアリアルタイム**。「S3 に 1 レコード 1 ファイルで置かれても困る」という配送先都合から必然的にこうなる
3. **入力ソース**: Direct PUT vs Kinesis Data Streams をソースにする構成。後者は「KDS でリアルタイム消費しつつ、同じデータを Firehose で S3 へアーカイブ」という定番構成（構成図をテキストで）
4. **変換と整形**: Lambda 変換、Parquet 変換、動的パーティショニング（S3 プレフィックス）
5. **KDS との使い分け**: 「自分でコンシューマを書きたい/複数系に配りたい/秒単位のレイテンシが要る → KDS。決まった配送先へ流すだけ → Firehose」。Firehose は consumer 開発・運用コストがゼロである対価として、レイテンシと柔軟性を差し出している

- [ ] **Step 2: Commit**

```bash
git add 11_stream/docs/06_firehose.md
git commit -m "docs(11): 06_firehose 配送マネージドサービスの位置づけ"
```

---

### Task 13: 料金調査 + docs/07_selection_guide.md（★肝）

**Files:**
- Create: `11_stream/docs/07_selection_guide.md`

**Interfaces:**
- Consumes: docs 01〜06 の事実、Web の最新料金ページ
- Produces: 完成した選定ガイド（README から最終章としてリンクされる）

- [ ] **Step 1: 東京リージョンの実単価を Web 調査**

WebSearch / WebFetch で以下の公式料金ページを開き、**ap-northeast-1 の単価**を取得してメモする。取得日を「執筆日」として本文に明記し、各表に出典 URL を付ける:

- https://aws.amazon.com/sqs/pricing/ — standard/FIFO の 100 万リクエスト単価（64KB 課金単位も確認）
- https://aws.amazon.com/amazon-mq/pricing/ — ActiveMQ の mq.m5.large 級ブローカー時間単価 + ストレージ
- https://aws.amazon.com/kinesis/data-streams/pricing/ — プロビジョンド: shard 時間 + PUT ペイロードユニット（25KB）/ オンデマンド: ストリーム時間 + 取込 GB + 読出 GB
- https://aws.amazon.com/firehose/pricing/ — Direct PUT 取込 GB 単価（5KB 切り上げ課金単位も確認）
- https://aws.amazon.com/msk/pricing/ — kafka.m5.large 級 broker 時間 + ストレージ GB（+ MSK Serverless の単価も参考取得）

- [ ] **Step 2: 以下のセクション構成で執筆**

1. **選定ディシジョンツリー**（テキストフローチャート）:
   - 完了したら消えてよい「仕事」を配る → queue → AWS 完結なら SQS、標準プロトコル/オンプレ要件なら ActiveMQ(Amazon MQ)
   - 「イベントの記録」を複数系が読む/リプレイが要る → stream → 消費側を書かず決まった先へ流すだけなら Firehose、書くなら KDS、規模とエコシステム要件で Kafka/MSK
2. **機能比較マトリクス**（5 サービス × 順序保証 / リプレイ / fan-out 方式 / 保持期間 / スループット上限 / 運用負荷 / レイテンシ / プロトコル可搬性）— docs 02〜06 で示した事実の集約であること
3. **料金構造の違い**: 「何に課金されるか」の表 + Step 1 の実単価表（執筆日・出典付き）:

| サービス | 課金軸 | 月額の式 |
|---|---|---|
| SQS standard | リクエスト数 | (send + receive + delete リクエスト数 / 100万) × 単価 ※64KB = 1 リクエスト単位 |
| Amazon MQ | ブローカー常駐時間 | インスタンス時間 × 730h × 単価 + ストレージ |
| KDS プロビジョンド | shard 時間 + PUT | shard 数 × 730h × 単価 + PUT ユニット(25KB)数 × 単価 |
| KDS オンデマンド | データ量 | ストリーム時間 + 取込 GB × 単価 + 読出 GB × 単価 |
| Firehose | 取込データ量 | 取込 GB × 単価 ※レコードは 5KB 切り上げ |
| MSK | broker 常駐時間 | broker 数 × 730h × 単価 + ストレージ GB |

4. **シナリオ試算表**: 前提「1 レコード = 1KB、24h365d 定常、コンシューマ 1 系統」を明記し、**1 MB/s / 10 MB/s / 50 MB/s** の 3 列 × 上記 6 行の月額を計算過程（式に単価を代入した形）付きで掲載。必要 shard 数の式 `shards = max(ceil(write MB/s ÷ 1), ceil(records/s ÷ 1000), ceil(read MB/s ÷ 2))` も示す。SQS が stream 的流量では桁違いに高くなること、MQ/MSK は流量ゼロでも定額であることが表から読めるようにする
5. **乗り換え目安**（★ユーザー要求の核心。試算表の実数から損益分岐を導く）:
   - **Firehose → KDS**: 料金分岐（GB 課金 vs shard 課金のクロスポイントを試算から算出。例「X MB/s を超えると KDS プロビジョンドの方が安い」）+ 機能分岐（秒単位レイテンシが要る、消費系が 2 つ以上になった、変換が Lambda で収まらない）
   - **KDS → MSK/Kafka**: 料金分岐（shard 積み上げ額が MSK 最小構成の定額を超える点 = 概ね何 MB/s か試算から算出）+ 制約分岐（GetRecords 5 回/秒がレイテンシ要件と衝突、shard 上限・re-shard 運用が痛い、Kafka エコシステム（Connect/Streams）が欲しい）
   - **SQS → stream 系**: リクエスト課金の月額が試算でいくらを超えたか + fan-out/リプレイ要件の発生
   - 逆方向（過剰スペックの引き下げ: Kafka → SQS で十分だった等）も 1 段落
6. **移行時の注意**: API 非互換で SDK 層の書き換えが必須なこと、デュアルライト期間と切替手順、オフセット/シーケンス番号は移行先に引き継げない（= コンシューマ冪等性が移行の前提条件）、順序保証の粒度が変わる（MessageGroupId / key / partition key）
7. **RabbitMQ / SNS / EventBridge への言及**（各 1 段落、スコープ外の明示）

- [ ] **Step 3: 試算の検算**

試算表の各セルについて式 → 数値の再計算を行い、単価×数量の桁誤りがないか確認（特に SQS のリクエスト数は 10^9 オーダーになるので注意）

- [ ] **Step 4: Commit**

```bash
git add 11_stream/docs/07_selection_guide.md
git commit -m "docs(11): 07_selection_guide 使い分け・料金試算・乗り換え目安"
```

---

### Task 14: README.md + 最終検証

**Files:**
- Create: `11_stream/README.md`

**Interfaces:**
- Consumes: 全タスクの成果物
- Produces: 章の入口（07_network/README.md と同構成）

- [ ] **Step 1: README.md を書く**（07_network/README.md と同じ 5 部構成）

1. `# 11_stream: 非同期メッセージング学習プロジェクト` + 概要 2〜3 文（queue と stream の違いを同一ワークロードの 4 実装で体感し、料金・スループットから乗り換え目安まで掴む章）
2. **学習動線**: docs 01〜07 の番号付きリンク
3. **クイックスタート**:

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

4. **ミドルウェア一覧表**: | app | 接続先 | 分類 | 特徴デモ | docs | の 4 行（sqs: localhost:4566 / queue / DLQ / 02、activemq: 61613 / queue+topic / fan-out / 03、kafka: 9092 / stream / replay / 04、kinesis: 4566 / stream / shard iterator / 05）+ Firehose は docs のみ（06）の注記
5. **環境注意**: LocalStack はダミー credential（`test`/`test`、コード内固定）で実 AWS には接続しない / ポート 4566・61613・8161・9092 の衝突 / demo-sqs-dlq は visibility timeout 待ちで 20 秒程度かかる / ActiveMQ 管理 UI は http://localhost:8161 (admin/admin) / docs/07 の料金は執筆日時点（日付明記）の東京リージョン単価

- [ ] **Step 2: 最終検証**

Run: `cd 11_stream && make down && make verify`
Expected: `ALL DEMOS PASSED`

Run: docs 内リンク切れ確認 — README と docs 各ファイルの相対リンク先が存在することを確認:
`grep -oh '](\.\.\?/[^)]*)' 11_stream/README.md 11_stream/docs/*.md | sed 's/](\(.*\))/\1/' | sed 's/#.*//' | sort -u` の各パスが存在すること

Run: `cd 11_stream && gofmt -l apps/`
Expected: 出力なし

- [ ] **Step 3: Commit**

```bash
git add 11_stream/README.md
git commit -m "docs(11): README 学習動線とクイックスタート"
```
