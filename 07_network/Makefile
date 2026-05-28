.PHONY: up down logs test test-race

up:        ; docker compose up -d --build
down:      ; docker compose down
logs:      ; docker compose logs -f
test:      ; go test ./servers/...
test-race: ; go test -race ./servers/...

.PHONY: demo-tcp
demo-tcp:
	echo "hello" | nc -w1 localhost 9001

.PHONY: demo-udp
demo-udp:
	echo "hello" | nc -u -w1 localhost 9002

.PHONY: demo-http
demo-http:
	curl -v http://localhost:9003/

.PHONY: demo-dns
demo-dns:
	dig @localhost -p 5353 example.local +short
