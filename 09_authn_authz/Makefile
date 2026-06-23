.PHONY: help session all down

help: ## ヘルプ表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

session: ## Session認証デモ起動 (http://localhost:9000)
	docker compose --profile session up --build -d

all: ## 全デモ起動
	docker compose --profile session up --build -d

down: ## 全サービス停止
	docker compose --profile session down
