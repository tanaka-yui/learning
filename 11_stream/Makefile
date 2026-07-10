.PHONY: up down logs demo-sqs demo-sqs-dlq

up:
	docker compose up -d
	./scripts/wait-ready.sh

down:  ; docker compose down
logs:  ; docker compose logs -f

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
	wait $$C1; S1=$$?; wait $$C2; S2=$$?; \
	[ $$S1 -eq 0 ] && [ $$S2 -eq 0 ]

.PHONY: demo-kafka demo-kafka-replay

demo-kafka:
	go run ./apps/kafka produce -n 5
	go run ./apps/kafka consume -group demo -max 5

# 新しい group 名 + --from-beginning = 再 produce せずに過去レコードを再消費（replay）
# group 名にタイムスタンプと PID を入れて毎回「初見の group」にする（秒内衝突回避）
demo-kafka-replay:
	go run ./apps/kafka consume -group replay-$$(date +%s)-$$$$ --from-beginning -max 5
