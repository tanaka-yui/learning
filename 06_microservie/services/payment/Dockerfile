FROM golang:1.26-bookworm AS builder

WORKDIR /work

COPY proto/go.mod proto/go.sum proto/
COPY proto/gen ./proto/gen

WORKDIR /work/services/payment
COPY services/payment/go.mod services/payment/go.sum ./
RUN go mod download

COPY services/payment/ ./

RUN CGO_ENABLED=0 go build -o /out/payment .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/payment /app/payment
COPY services/payment/migrations /app/migrations

ENV GRPC_PORT=50055
EXPOSE 50055

ENTRYPOINT ["/app/payment"]
