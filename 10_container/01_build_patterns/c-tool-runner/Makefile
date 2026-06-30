.PHONY: build test clean
build: ; docker build -t tool-runner:latest .
test:
	docker run --rm tool-runner:latest migrate -version
	docker run --rm tool-runner:latest sqlc version
	docker run --rm tool-runner:latest golangci-lint --version
	docker run --rm tool-runner:latest buf --version
	docker run --rm tool-runner:latest mockgen -version
	docker run --rm tool-runner:latest hadolint --version
clean: ; docker rmi tool-runner:latest || true
