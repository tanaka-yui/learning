FROM golang:1.26-bookworm AS builder

WORKDIR /work

# proto モジュールをコピー（go.work 経由の relative replace に対応）
COPY proto/go.mod proto/go.sum proto/
COPY proto/gen ./proto/gen

# catalog モジュール
WORKDIR /work/services/catalog
COPY services/catalog/go.mod services/catalog/go.sum ./
RUN go mod download

COPY services/catalog/ ./

RUN CGO_ENABLED=0 go build -o /out/catalog .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/catalog /app/catalog
COPY services/catalog/migrations /app/migrations

ENV GRPC_PORT=50051
EXPOSE 50051

ENTRYPOINT ["/app/catalog"]
