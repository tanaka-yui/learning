FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /seed .

FROM alpine:3.21
COPY --from=builder /seed /seed
ENTRYPOINT ["/seed"]
