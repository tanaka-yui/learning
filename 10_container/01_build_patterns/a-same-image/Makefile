.PHONY: build-dev build-test build-prod run-dev run-prod test clean
build-dev:  ; docker build --target dev  -t same-image:dev  .
build-test: ; docker build --target test -t same-image:test .
build-prod: ; docker build --target prod -t same-image:prod .
run-dev:    ; docker run --rm -p 8080:8080 same-image:dev
run-prod:   ; docker run --rm -p 8081:8080 same-image:prod
test:       ; go test ./...
clean:      ; docker rmi same-image:dev same-image:test same-image:prod || true
