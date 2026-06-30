# syntax=docker/dockerfile:1.7
FROM golang:1.26 AS base
WORKDIR /src
COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

FROM base AS deps
COPY . .

FROM deps AS dev
RUN --mount=type=cache,target=/go/pkg/mod go install github.com/air-verse/air@latest
ENV APP_ENV=dev PORT=8080
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]

FROM deps AS test
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go test ./...

FROM deps AS build
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .

FROM gcr.io/distroless/static-debian12 AS prod
COPY --from=build /out/server /server
ENV APP_ENV=prod
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/server"]
