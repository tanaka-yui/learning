FROM golang:1.26-bookworm AS builder

WORKDIR /work

COPY proto/go.mod proto/go.sum proto/
COPY proto/gen ./proto/gen

WORKDIR /work/bff
COPY bff/go.mod bff/go.sum ./
RUN go mod download

COPY bff/ ./

RUN CGO_ENABLED=0 go build -o /out/bff .

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=builder /out/bff /app/bff

ENV HTTP_PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/bff"]
