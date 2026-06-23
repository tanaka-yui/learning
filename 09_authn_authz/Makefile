.PHONY: help session jwt oauth-oidc mfa api-m2m authz gen-certs all down

help: ## ヘルプ表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

session: ## Session認証デモ起動 (http://localhost:9000)
	docker compose --profile session up --build -d

jwt: ## JWT認証デモ起動 (http://localhost:9001)
	docker compose --profile jwt up --build -d

oauth-oidc: ## OAuth2/OIDCデモ起動 (http://localhost:9100/app/login)
	docker compose --profile oauth-oidc up --build -d

mfa: ## MFA/パスワードレスデモ起動 (http://localhost:9300, Mailpit http://localhost:8025)
	docker compose --profile mfa up --build -d

api-m2m: ## API Key/mTLSデモ起動 (http://localhost:9400, https://localhost:9401)
	docker compose --profile api-m2m up --build -d

authz: ## 認可(Casbin)デモ起動 (http://localhost:9200)
	docker compose --profile authz up --build -d

gen-certs: ## mTLS用の証明書を生成 (api-m2m/certs/)
	./api-m2m/gen-certs.sh

all: ## 全デモ起動
	docker compose --profile session --profile jwt --profile oauth-oidc --profile mfa --profile api-m2m --profile authz up --build -d

down: ## 全サービス停止
	docker compose --profile session --profile jwt --profile oauth-oidc --profile mfa --profile api-m2m --profile authz down
