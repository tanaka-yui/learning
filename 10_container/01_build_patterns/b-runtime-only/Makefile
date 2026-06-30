.PHONY: build build-go build-node sizes test clean
build: build-go build-node
build-go:
	docker build --target distroless     -t runtime-go:distroless .
	docker build --target scratch-final  -t runtime-go:scratch    .
build-node:
	docker build -f Dockerfile.node -t runtime-node:distroless .
sizes:
	@docker images --format '{{.Repository}}:{{.Tag}} {{.Size}}' | grep -E '^(runtime-go|runtime-node):'
test:
	go test ./api/...
clean:
	docker rmi runtime-go:distroless runtime-go:scratch runtime-node:distroless || true
