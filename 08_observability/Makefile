.PHONY: up down logs test demo load

up:    ; docker compose up -d --build
down:  ; docker compose down -v
logs:  ; docker compose logs -f
test:  ; cd app && go test ./...

demo:
	curl -s -X POST http://localhost:9100/api/checkout \
	  -H 'Content-Type: application/json' \
	  -d '{"item":"book","qty":1}' ; echo

load:
	@for i in $$(seq 1 300); do \
	  curl -s -o /dev/null -X POST http://localhost:9100/api/checkout \
	    -H 'Content-Type: application/json' -d '{"item":"book","qty":1}' ; \
	done ; echo "sent 300 requests"
