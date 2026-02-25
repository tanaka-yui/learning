# Process & Thread Comparison Design

## Overview

HTTP server concurrent request handling comparison across TypeScript (Node.js), Go, Java, PHP, Ruby, Rust, and Python.
Demonstrate single-threaded blocking (Node.js) vs multi-threaded/multi-process parallel processing (Go/Java/PHP/Ruby/Rust/Python), and how reducing worker count to 1 causes serial behavior in PHP, Ruby, Rust, Python, Go (GOMAXPROCS=1), and Java (threads.max=1).

## Goals

- Show that Go/Java handle multiple heavy requests in parallel with different thread/goroutine IDs
- Show that PHP-FPM (workers=4) and Puma (workers=4) handle requests in parallel with different worker process IDs
- Show that Node.js blocks on CPU-bound work, causing subsequent requests to wait
- Show that Rust Actix-web (workers=4) and Axum (Tokio workers=4) handle requests in parallel with different worker thread IDs
- Show that Python Uvicorn (workers=4) handles requests in parallel with different worker process IDs
- Show that Go (GOMAXPROCS=1) exhibits time-sliced concurrency on a single OS thread, causing each request to take longer
- Show that Java (Tomcat threads.max=1) queues requests and processes them serially
- Show that PHP-FPM (workers=1), Puma (workers=1), Actix-web (workers=1), Axum (workers=1), Uvicorn (workers=1), Go (GOMAXPROCS=1), and Java (threads.max=1) exhibit the same serial behavior as Node.js
- Provide a reproducible environment via Docker Compose

## Architecture

```
01_process_thread/
├── docker-compose.yml              # All 15 services
├── scripts/
│   └── load-test.sh                # Concurrent request test script
├── typescript/
│   ├── Dockerfile
│   ├── package.json
│   ├── tsconfig.json
│   └── src/
│       └── index.ts                # Fastify server
├── go/
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go                     # net/http server
├── java/
│   ├── Dockerfile
│   ├── build.gradle
│   └── src/main/java/...           # Spring Boot server
├── php/
│   ├── Dockerfile                  # Multi-stage: composer scaffold → php-fpm + nginx + supervisord
│   ├── nginx.conf                  # Reverse proxy to php-fpm
│   ├── supervisord.conf            # php-fpm + nginx process manager
│   ├── www.conf                    # FPM pool config template (envsubst)
│   ├── entrypoint.sh               # Dynamic FPM worker count via envsubst
│   ├── routes/
│   │   └── web.php                 # Laravel routes
│   └── app/Http/Controllers/
│       └── HeavyController.php     # Laravel controller
├── ruby/
│   ├── Dockerfile                  # Multi-stage: rails new scaffold → ruby-slim
│   ├── config/
│   │   ├── routes.rb               # Rails routes
│   │   └── puma.rb                 # Puma cluster config
│   └── app/controllers/
│       └── heavy_controller.rb     # Rails controller
├── python/
│   ├── Dockerfile
│   ├── requirements.txt
│   └── main.py                     # FastAPI server
└── rust/
    ├── actix-web/
    │   ├── Dockerfile              # Multi-stage: rust:1.93-bookworm → debian:bookworm-slim
    │   ├── Cargo.toml
    │   └── src/
    │       └── main.rs             # Actix-web server
    └── axum/
        ├── Dockerfile              # Multi-stage: rust:1.93-bookworm → debian:bookworm-slim
        ├── Cargo.toml
        └── src/
            └── main.rs             # Axum server
```

## Endpoints (common across all languages)

| Path | Description |
|------|-------------|
| `GET /health` | Health check |
| `GET /heavy` | Execute CPU-bound work, return duration and thread/process ID |

### Response format for `/heavy`

```json
{
  "language": "go",
  "threadId": "goroutine-42",
  "startedAt": "2026-02-25T10:00:00.000Z",
  "finishedAt": "2026-02-25T10:00:03.000Z",
  "durationMs": 3000
}
```

## Server Implementations

### TypeScript (Fastify) — Single-threaded blocking

- CPU-bound: synchronous recursive Fibonacci calculation
- Thread ID: `process.pid` (always same value since single-threaded)
- Expected: concurrent requests are serialized; 2nd request waits for 1st to complete

### Go (net/http) — Goroutine-based concurrency

- CPU-bound: same recursive Fibonacci calculation
- Thread ID: goroutine ID extracted from `runtime.Stack()`
- Expected: concurrent requests handled in parallel by separate goroutines

### Java (Spring Boot) — Thread pool concurrency

- CPU-bound: same recursive Fibonacci calculation
- Thread ID: `Thread.currentThread().getName()` + `Thread.currentThread().getId()`
- Expected: concurrent requests handled by different threads from the pool

### PHP (Laravel + PHP-FPM) — Multi-process concurrency

- CPU-bound: same recursive Fibonacci calculation
- Thread ID: `getmypid()` → `fpm-worker-<PID>` (different PID per FPM worker)
- Runtime: php-fpm with nginx reverse proxy, managed by supervisord in single container
- Config: `pm = static`, `clear_env = no` (required for env var propagation to workers)
- Worker count: configurable at runtime via `PHP_FPM_WORKERS` env var (envsubst in entrypoint.sh)
- Expected (workers=4): concurrent requests handled in parallel by different FPM worker processes
- Expected (workers=1): concurrent requests serialized, same behavior as Node.js

### Ruby (Rails + Puma) — Multi-process concurrency

- CPU-bound: same recursive Fibonacci calculation
- Thread ID: `Process.pid` → `puma-worker-<PID>` (different PID per Puma worker)
- Runtime: Puma in cluster mode (forked workers)
- Config: `workers = WEB_CONCURRENCY`, `threads = 1` (single thread per worker, since GVL prevents CPU-bound thread parallelism)
- Expected (workers=4): concurrent requests handled in parallel by different Puma worker processes
- Expected (workers=1): concurrent requests serialized, same behavior as Node.js

### Python (FastAPI + Uvicorn) — Multi-process concurrency

- CPU-bound: same recursive Fibonacci calculation
- Thread ID: `os.getpid()` → `uvicorn-worker-<PID>` (different PID per Uvicorn worker process)
- Runtime: Uvicorn ASGI server with configurable worker processes via `--workers`
- Config: `UVICORN_WORKERS` env var (default 4). Shell form CMD with `exec` for env var expansion
- Key design: `async def` handlers are used (not `def`) so fibonacci blocks the event loop directly. If `def` were used, FastAPI would auto-run it in a thread pool, defeating the demo purpose
- Expected (workers=4): concurrent requests handled in parallel by different Uvicorn worker processes
- Expected (workers=1): concurrent requests serialized, same behavior as Node.js (CPU-bound work blocks the single event loop)

### Rust (Actix-web) — Multi-threaded concurrency

- CPU-bound: same recursive Fibonacci calculation
- Thread ID: `std::thread::current().id()` → `actix-worker-<N>` (different thread ID per Actix worker)
- Runtime: Actix-web with configurable worker threads via `.workers(n)`
- Config: `ACTIX_WORKERS` env var (default 4)
- Expected (workers=4): concurrent requests handled in parallel by different Actix worker threads
- Expected (workers=1): concurrent requests serialized, same behavior as Node.js

### Rust (Axum) — Tokio multi-threaded concurrency

- CPU-bound: same recursive Fibonacci calculation
- Thread ID: `std::thread::current().id()` → `tokio-worker-<N>` (different thread ID per Tokio worker)
- Runtime: Axum on Tokio runtime with configurable worker threads via `tokio::runtime::Builder::new_multi_thread().worker_threads(n)`
- Config: `TOKIO_WORKERS` env var (default 4). Uses manual `fn main()` + `runtime.block_on()` instead of `#[tokio::main]` for runtime worker count control
- Key difference from Actix-web: Actix-web has its own worker model (`HttpServer.workers(n)`), Axum relies directly on Tokio runtime (`Builder.worker_threads(n)`)
- Expected (workers=4): concurrent requests handled in parallel by different Tokio worker threads
- Expected (workers=1): concurrent requests serialized, same behavior as Node.js

### CPU-bound work specification

- Algorithm: naive recursive Fibonacci (no memoization)
- Default N values tuned per language for ~2-3 second processing time:
  - TypeScript/Go/Java/Rust: `HEAVY_CALC_N=43`
  - PHP: `HEAVY_CALC_N_PHP=38` (interpreted language, slower per iteration)
  - Python: `HEAVY_CALC_N_PYTHON=38` (interpreted language, slower per iteration)
  - Ruby: `HEAVY_CALC_N_RUBY=40` (interpreted language, slower per iteration)
- Target duration: ~2-3 seconds per request

## Docker Configuration

### Port mapping

| Language | Service | Port | Concurrency Model |
|----------|---------|------|--------------------|
| TypeScript (Fastify) | ts-server | 3000 | Single-threaded (serial) |
| Go (net/http) | go-server | 8081 | goroutine (parallel) |
| Go (net/http) | go-server-single | 8082 | GOMAXPROCS=1 (time-sliced) |
| Java (Spring Boot) | java-server | 8083 | Thread pool (parallel) |
| Java (Spring Boot) | java-server-single | 8084 | Tomcat threads.max=1 (serial) |
| PHP (Laravel/FPM) | php-server | 8085 | Multi-process, workers=4 (parallel) |
| PHP (Laravel/FPM) | php-server-single | 8086 | Single-process, workers=1 (serial) |
| Ruby (Rails/Puma) | ruby-server | 8087 | Multi-process, workers=4 (parallel) |
| Ruby (Rails/Puma) | ruby-server-single | 8088 | Single-process, workers=1 (serial) |
| Rust (Actix-web) | rust-server | 8089 | Multi-threaded, Actix workers=4 (parallel) |
| Rust (Actix-web) | rust-server-single | 8090 | Single-threaded, Actix workers=1 (serial) |
| Rust (Axum) | rust-axum-server | 8091 | Multi-threaded, Tokio workers=4 (parallel) |
| Rust (Axum) | rust-axum-server-single | 8092 | Single-threaded, Tokio workers=1 (serial) |
| Python (FastAPI/Uvicorn) | python-server | 8093 | Multi-process, Uvicorn workers=4 (parallel) |
| Python (FastAPI/Uvicorn) | python-server-single | 8094 | Single-process, Uvicorn workers=1 (serial) |

### Base images

| Language | Build | Runtime |
|----------|-------|---------|
| TypeScript | `node:24-slim` | same (tsx execution) |
| Go | `golang:1.26-bookworm` | `gcr.io/distroless/static` |
| Java | `gradle:8-jdk21` | `eclipse-temurin:21-jre` |
| PHP | `composer:2` (scaffold) | `php:8.4-fpm` + nginx + supervisord |
| Ruby | `ruby:3.4` (scaffold) | `ruby:3.4-slim` |
| Rust (Actix-web) | `rust:1.93-bookworm` | `debian:bookworm-slim` |
| Rust (Axum) | `rust:1.93-bookworm` | `debian:bookworm-slim` |
| Python | `python:3.13-slim-bookworm` | same (interpreted) |

### Environment variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HEAVY_CALC_N` | Fibonacci N for TypeScript/Go/Java/Rust | 43 |
| `HEAVY_CALC_N_PHP` | Fibonacci N for PHP | 38 |
| `HEAVY_CALC_N_RUBY` | Fibonacci N for Ruby | 40 |
| `PHP_FPM_WORKERS` | Number of PHP-FPM worker processes | 4 |
| `WEB_CONCURRENCY` | Number of Puma worker processes | 4 |
| `ACTIX_WORKERS` | Number of Actix-web worker threads | 4 |
| `TOKIO_WORKERS` | Number of Tokio runtime worker threads (Axum) | 4 |
| `HEAVY_CALC_N_PYTHON` | Fibonacci N for Python | 38 |
| `UVICORN_WORKERS` | Number of Uvicorn worker processes | 4 |
| `GOMAXPROCS` | Number of OS threads for Go runtime | (not set = all CPUs) |
| `SERVER_TOMCAT_THREADS_MAX` | Max Tomcat request processing threads (Java) | 200 |

## Test Script

`scripts/load-test.sh` sends N concurrent `curl` requests (default 3) to a specified port using background processes (`&`), echoing each result immediately as it completes.

Usage: `./scripts/load-test.sh [port] [concurrent_count]`

This makes it visually clear whether requests were processed serially (results appear one by one with staggered timing) or in parallel (results appear nearly simultaneously with different thread/process IDs).

## Key Implementation Notes

- **PHP-FPM `clear_env = no`**: Without this, `HEAVY_CALC_N` and other env vars are not propagated to FPM worker processes
- **PHP `SESSION_DRIVER=array`**: Laravel defaults to SQLite sessions which causes errors in a read-only container filesystem
- **PHP `getenv()` vs `env()`**: `getenv('HEAVY_CALC_N')` is used instead of Laravel's `env()` for reliable PHP-FPM env var access
- **Rails `SECRET_KEY_BASE`**: Required in production mode; set to a dummy value in Dockerfile for this demo
- **Ruby GVL**: Global VM Lock prevents true thread parallelism for CPU-bound work, so Puma workers (processes) are used instead of threads
- **Go port 8081**: Host port changed from 8080 to avoid conflict with other local services; go-server-single uses port 8082
- **Rust ThreadId parsing**: stable Rust では `ThreadId::as_u64()` が未安定のため、`Debug` 出力 `ThreadId(N)` を文字列パースして数値を取得
- **Rust Actix-web vs Axum**: Actix-web は独自のワーカーモデル（`HttpServer.workers(n)`）、Axum は Tokio ランタイムの `Builder.worker_threads(n)` で制御。同じ Rust でもフレームワーク層の抽象化が異なる
- **Rust Axum entry point**: `#[tokio::main]` はワーカー数のランタイム制御ができないため、同期 `fn main()` + `tokio::runtime::Builder` + `runtime.block_on()` パターンを使用
- **Rust image version**: `rust:1.93-bookworm` を使用（actix-web 4.13.0 が rustc 1.88 以上を要求）
- **Python `async def` vs `def`**: FastAPI は `def` (同期) ハンドラを自動的にスレッドプールで実行する。CPU バウンドのデモには `async def` を使用して、イベントループを直接ブロックさせる必要がある
- **Python Uvicorn `exec`**: Dockerfile の shell form CMD で `exec` を付けて PID 1 で uvicorn を実行し、シグナル伝播を確保
- **Go GOMAXPROCS=1**: Go 1.14+ のプリエンプティブスケジューリングにより、GOMAXPROCS=1 でも goroutine が時分割で切り替わる。完全な直列処理ではなく「1スレッド上の時分割並行」になり、各リクエストの所要時間が約3倍に増加する
- **Java Tomcat threads.max=1**: Spring Boot の relaxed binding により `SERVER_TOMCAT_THREADS_MAX` 環境変数で `server.tomcat.threads.max` を制御。1スレッドのためリクエストがキューに入り完全な直列処理になる
