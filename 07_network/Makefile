.PHONY: up down logs test test-race

up:        ; docker compose up -d --build
down:      ; docker compose down
logs:      ; docker compose logs -f
test:      ; go test ./servers/...
test-race: ; go test -race ./servers/...
