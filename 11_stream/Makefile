.PHONY: up down logs

up:
	docker compose up -d
	./scripts/wait-ready.sh

down:  ; docker compose down
logs:  ; docker compose logs -f
