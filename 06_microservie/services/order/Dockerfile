FROM golang:1.26-bookworm AS builder

WORKDIR /work

COPY proto/go.mod proto/go.sum proto/
COPY proto/gen ./proto/gen

WORKDIR /work/services/order
COPY services/order/go.mod services/order/go.sum ./
RUN go mod download

COPY services/order/ ./

RUN CGO_ENABLED=0 go build -o /out/order .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/order /app/order
COPY services/order/migrations /app/migrations

ENV GRPC_PORT=50053
EXPOSE 50053

ENTRYPOINT ["/app/order"]
