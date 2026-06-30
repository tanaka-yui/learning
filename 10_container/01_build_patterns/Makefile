.PHONY: build-all run-dev run-prod run-runtime size-report verify smoke lint clean

build-all:
	$(MAKE) -C a-same-image  build-dev build-test build-prod
	$(MAKE) -C b-runtime-only build
	$(MAKE) -C c-tool-runner  build

run-dev:     ; docker compose --profile dev     up -d
run-prod:    ; docker compose --profile prod    up -d
run-runtime: ; docker compose --profile runtime up -d

size-report:
	@printf "%-40s %s\n" IMAGE SIZE
	@docker images --format '{{.Repository}}:{{.Tag}}  {{.Size}}' | \
	  grep -E '^(same-image|runtime-go|runtime-node|tool-runner):' | sort

lint:
	docker run --rm -v "$$PWD":/work tool-runner:latest hadolint \
	  a-same-image/Dockerfile \
	  b-runtime-only/Dockerfile \
	  b-runtime-only/Dockerfile.node \
	  c-tool-runner/Dockerfile

smoke:
	docker compose --profile dev up -d
	sleep 3
	curl -fsS http://localhost:8080/healthz
	docker compose --profile dev down
	docker compose --profile prod up -d
	sleep 2
	curl -fsS http://localhost:8081/healthz
	docker compose --profile prod down

verify: build-all smoke lint size-report
	$(MAKE) -C c-tool-runner test

clean:
	docker compose --profile dev     down -v || true
	docker compose --profile prod    down -v || true
	docker compose --profile runtime down -v || true
	$(MAKE) -C a-same-image  clean
	$(MAKE) -C b-runtime-only clean
	$(MAKE) -C c-tool-runner  clean
