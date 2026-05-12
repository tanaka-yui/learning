FROM golang:1.26-bookworm AS builder
WORKDIR /work
COPY proto/go.mod proto/go.sum proto/
COPY proto/gen ./proto/gen
WORKDIR /work/services/user-auth
COPY services/user-auth/go.mod services/user-auth/go.sum ./
RUN go mod download
COPY services/user-auth/ ./
RUN CGO_ENABLED=0 go build -o /out/user-auth .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/user-auth /app/user-auth
COPY services/user-auth/migrations /app/migrations
ENV GRPC_PORT=50052
EXPOSE 50052
ENTRYPOINT ["/app/user-auth"]
