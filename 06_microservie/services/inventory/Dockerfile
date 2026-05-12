FROM golang:1.26-bookworm AS builder

WORKDIR /work

COPY proto/go.mod proto/go.sum proto/
COPY proto/gen ./proto/gen

WORKDIR /work/services/inventory
COPY services/inventory/go.mod services/inventory/go.sum ./
RUN go mod download

COPY services/inventory/ ./

RUN CGO_ENABLED=0 go build -o /out/inventory .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/inventory /app/inventory
COPY services/inventory/migrations /app/migrations

ENV GRPC_PORT=50054
EXPOSE 50054

ENTRYPOINT ["/app/inventory"]
