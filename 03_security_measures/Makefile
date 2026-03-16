.PHONY: all down help sql-injection xss csrf auth-bypass path-traversal command-injection

help: ## ヘルプ表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

sql-injection: ## SQLインジェクションデモ起動
	docker compose --profile sql-injection up --build -d

xss: ## XSSデモ起動
	docker compose --profile xss up --build -d

csrf: ## CSRFデモ起動
	docker compose --profile csrf up --build -d

auth-bypass: ## 認証の不備デモ起動
	docker compose --profile auth-bypass up --build -d

path-traversal: ## パストラバーサルデモ起動
	docker compose --profile path-traversal up --build -d

command-injection: ## コマンドインジェクションデモ起動
	docker compose --profile command-injection up --build -d

all: ## 全デモ起動
	docker compose --profile sql-injection --profile xss --profile csrf --profile auth-bypass --profile path-traversal --profile command-injection up --build -d

down: ## 全サービス停止
	docker compose --profile sql-injection --profile xss --profile csrf --profile auth-bypass --profile path-traversal --profile command-injection down
