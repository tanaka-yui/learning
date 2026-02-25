FROM rust:1.93-bookworm AS builder

WORKDIR /app

COPY Cargo.toml ./
COPY src/ ./src/

RUN cargo build --release

FROM debian:bookworm-slim

COPY --from=builder /app/target/release/process-thread-rust-axum /server

EXPOSE 8080

ENTRYPOINT ["/server"]
