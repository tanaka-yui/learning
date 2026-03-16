FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o server .

FROM alpine:latest
RUN apk add --no-cache bind-tools iputils
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
