# syntax=docker/dockerfile:1.7
FROM golang:1.23 AS tools
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install -v github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1 && \
    go install -v github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 && \
    go install -v github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2 && \
    go install -v github.com/bufbuild/buf/cmd/buf@v1.47.2 && \
    go install -v go.uber.org/mock/mockgen@v0.5.0

FROM hadolint/hadolint:v2.12.1-beta-debian AS hadolint

FROM debian:12-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates git make curl && rm -rf /var/lib/apt/lists/*
COPY --from=tools    /go/bin/migrate           /usr/local/bin/migrate
COPY --from=tools    /go/bin/sqlc              /usr/local/bin/sqlc
COPY --from=tools    /go/bin/golangci-lint     /usr/local/bin/golangci-lint
COPY --from=tools    /go/bin/buf               /usr/local/bin/buf
COPY --from=tools    /go/bin/mockgen           /usr/local/bin/mockgen
COPY --from=hadolint /bin/hadolint             /usr/local/bin/hadolint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
WORKDIR /work
ENTRYPOINT ["/entrypoint.sh"]
