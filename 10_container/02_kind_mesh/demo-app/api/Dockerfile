# syntax=docker/dockerfile:1.7
ARG APP_VERSION=v1
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api .

FROM gcr.io/distroless/static-debian12
ARG APP_VERSION
ENV APP_VERSION=${APP_VERSION}
COPY --from=build /out/api /api
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/api"]
