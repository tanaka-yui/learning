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
