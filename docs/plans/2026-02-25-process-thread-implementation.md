# Process & Thread Comparison Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build 15 HTTP server configurations (Fastify, Go net/http, Spring Boot, Laravel/PHP-FPM, Rails/Puma, Rust Actix-web, Rust Axum, Python FastAPI/Uvicorn — with single-worker variants for Go (GOMAXPROCS=1), Java (Tomcat threads.max=1), PHP, Ruby, Rust, and Python) that demonstrate single-threaded blocking vs multi-threaded/multi-process parallel request handling, all running via Docker Compose.

**Architecture:** Each server exposes `/health` and `/heavy` endpoints. `/heavy` runs naive recursive Fibonacci (CPU-bound) and returns language, thread/process ID, timestamps, and duration. A shell script sends concurrent requests to compare behavior. PHP, Ruby, and Rust include single-worker variants (workers=1) to demonstrate that parallelism comes from multiple workers/threads, not the language runtime.

**Tech Stack:** Fastify 5 + TypeScript (tsx), Go 1.26 net/http, Spring Boot 4 + Java 21, Laravel + PHP-FPM 8.4 + nginx, Rails + Puma + Ruby 3.4, Rust 1.93 Actix-web 4 + Axum 0.8, Python 3.13 FastAPI + Uvicorn, Docker Compose

**Design doc:** `docs/plans/2026-02-25-process-thread-comparison-design.md`

---

### Task 1: TypeScript (Fastify) Server

**Files:**
- Modify: `01_process_thread/typescript/package.json`
- Create: `01_process_thread/typescript/tsconfig.json`
- Create: `01_process_thread/typescript/src/index.ts`

**Step 1: Update package.json with dependencies and scripts**

```json
{
  "name": "process-thread-ts",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "tsx watch src/index.ts",
    "start": "tsx src/index.ts"
  },
  "dependencies": {
    "fastify": "^5.0.0"
  },
  "devDependencies": {
    "typescript": "^5.5.0",
    "@types/node": "^22.0.0",
    "tsx": "^4.0.0",
    "fastify-tsconfig": "^3.0.0"
  },
  "packageManager": "pnpm@10.27.0"
}
```

**Step 2: Create tsconfig.json**

```json
{
  "extends": "fastify-tsconfig",
  "compilerOptions": {
    "outDir": "dist",
    "sourceMap": true
  },
  "include": ["src/**/*.ts"]
}
```

**Step 3: Create src/index.ts**

Fastify server with `/health` and `/heavy` endpoints. The `/heavy` endpoint runs synchronous recursive Fibonacci to block the event loop.

```typescript
import Fastify from "fastify";

const server = Fastify({ logger: true });

const HEAVY_CALC_N = Number(process.env["HEAVY_CALC_N"] ?? "40");

const fibonacci = (n: number): number => {
  if (n <= 1) return n;
  return fibonacci(n - 1) + fibonacci(n - 2);
};

server.get("/health", async () => {
  return { status: "ok", language: "typescript" };
});

server.get("/heavy", async () => {
  const startedAt = new Date().toISOString();
  const start = performance.now();

  fibonacci(HEAVY_CALC_N);

  const end = performance.now();
  const finishedAt = new Date().toISOString();

  return {
    language: "typescript",
    threadId: `pid-${process.pid}`,
    startedAt,
    finishedAt,
    durationMs: Math.round(end - start),
  };
});

server.listen({ port: 3000, host: "0.0.0.0" }, (err) => {
  if (err) {
    server.log.error(err);
    process.exit(1);
  }
});
```

**Step 4: Commit**

```bash
git add 01_process_thread/typescript/
git commit -m "feat: add Fastify server for process/thread comparison"
```

---

### Task 2: Go (net/http) Server

**Files:**
- Create: `01_process_thread/go/go.mod`
- Create: `01_process_thread/go/main.go`

**Step 1: Create go.mod**

```
module process-thread-go

go 1.26
```

**Step 2: Create main.go**

Go net/http server with `/health` and `/heavy` endpoints. Each request is handled in a separate goroutine automatically. Extract goroutine ID from `runtime.Stack()` for demonstration purposes. Uses Go 1.22+ method-qualified routing patterns (`"GET /health"`).

```go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func getGoroutineID() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	s := string(buf[:n])
	s = strings.TrimPrefix(s, "goroutine ")
	if idx := strings.IndexByte(s, ' '); idx > 0 {
		return s[:idx]
	}
	return "unknown"
}

func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}

func main() {
	heavyCalcN := 40
	if v := os.Getenv("HEAVY_CALC_N"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			heavyCalcN = parsed
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":   "ok",
			"language": "go",
		})
	})

	mux.HandleFunc("GET /heavy", func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()

		fibonacci(heavyCalcN)

		finishedAt := time.Now()
		durationMs := finishedAt.Sub(startedAt).Milliseconds()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"language":   "go",
			"threadId":   fmt.Sprintf("goroutine-%s", getGoroutineID()),
			"startedAt":  startedAt.UTC().Format(time.RFC3339Nano),
			"finishedAt": finishedAt.UTC().Format(time.RFC3339Nano),
			"durationMs": durationMs,
		})
	})

	fmt.Println("Go server listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 3: Commit**

```bash
git add 01_process_thread/go/
git commit -m "feat: add Go net/http server for process/thread comparison"
```

---

### Task 3: Java (Spring Boot) Server

**Files:**
- Create: `01_process_thread/java/build.gradle`
- Create: `01_process_thread/java/settings.gradle`
- Create: `01_process_thread/java/src/main/java/com/example/App.java`
- Create: `01_process_thread/java/src/main/java/com/example/HeavyController.java`
- Create: `01_process_thread/java/src/main/resources/application.properties`

**Step 1: Create settings.gradle**

```groovy
rootProject.name = 'process-thread-java'
```

**Step 2: Create build.gradle**

```groovy
plugins {
    id 'java'
    id 'org.springframework.boot' version '4.0.3'
    id 'io.spring.dependency-management' version '1.1.7'
}

group = 'com.example'
version = '0.0.1'

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(21)
    }
}

repositories {
    mavenCentral()
}

dependencies {
    implementation 'org.springframework.boot:spring-boot-starter-web'
}
```

**Step 3: Create application.properties**

```properties
server.port=8081
```

**Step 4: Create App.java**

```java
package com.example;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class App {
    public static void main(String[] args) {
        SpringApplication.run(App.class, args);
    }
}
```

**Step 5: Create HeavyController.java**

```java
package com.example;

import java.time.Instant;
import java.util.Map;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class HeavyController {

    @Value("${heavy.calc.n:40}")
    private int heavyCalcN;

    private static long fibonacci(int n) {
        if (n <= 1) return n;
        return fibonacci(n - 1) + fibonacci(n - 2);
    }

    @GetMapping("/health")
    public Map<String, String> health() {
        return Map.of("status", "ok", "language", "java");
    }

    @GetMapping("/heavy")
    public Map<String, Object> heavy() {
        Instant startedAt = Instant.now();
        long startMs = System.currentTimeMillis();

        fibonacci(heavyCalcN);

        long endMs = System.currentTimeMillis();
        Instant finishedAt = Instant.now();

        return Map.of(
            "language", "java",
            "threadId", Thread.currentThread().getName() + "-" + Thread.currentThread().getId(),
            "startedAt", startedAt.toString(),
            "finishedAt", finishedAt.toString(),
            "durationMs", endMs - startMs
        );
    }
}
```

**Step 6: Commit**

```bash
git add 01_process_thread/java/
git commit -m "feat: add Spring Boot server for process/thread comparison"
```

---

### Task 4: Dockerfiles (TypeScript, Go, Java)

**Files:**
- Create: `01_process_thread/typescript/Dockerfile`
- Create: `01_process_thread/go/Dockerfile`
- Create: `01_process_thread/java/Dockerfile`

**Step 1: Create TypeScript Dockerfile**

```dockerfile
FROM node:24-slim

RUN corepack enable && corepack prepare pnpm@10.27.0 --activate

WORKDIR /app

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY tsconfig.json ./
COPY src/ ./src/

EXPOSE 3000

CMD ["pnpm", "start"]
```

**Step 2: Create Go Dockerfile**

```dockerfile
FROM golang:1.26-bookworm AS builder

WORKDIR /app

COPY go.mod ./
COPY main.go ./

RUN CGO_ENABLED=0 go build -o server main.go

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/server /server

EXPOSE 8080

ENTRYPOINT ["/server"]
```

**Step 3: Create Java Dockerfile**

```dockerfile
FROM gradle:8-jdk21 AS builder

WORKDIR /app

COPY build.gradle settings.gradle ./
COPY src/ ./src/

RUN gradle bootJar --no-daemon

FROM eclipse-temurin:21-jre

COPY --from=builder /app/build/libs/*.jar /app/app.jar

EXPOSE 8081

ENTRYPOINT ["java", "-jar", "/app/app.jar"]
```

**Step 4: Commit**

```bash
git add 01_process_thread/typescript/Dockerfile 01_process_thread/go/Dockerfile 01_process_thread/java/Dockerfile
git commit -m "feat: add Dockerfiles for TypeScript, Go, Java servers"
```

---

### Task 5: Docker Compose and Load Test Script

**Files:**
- Create: `01_process_thread/docker-compose.yml`
- Create: `01_process_thread/scripts/load-test.sh`

**Step 1: Create docker-compose.yml**

```yaml
services:
  ts-server:
    build:
      context: ./typescript
    ports:
      - "3000:3000"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N:-43}

  go-server:
    build:
      context: ./go
    ports:
      - "8081:8080"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N:-43}

  go-server-single:
    build:
      context: ./go
    ports:
      - "8082:8080"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N:-43}
      - GOMAXPROCS=1

  java-server:
    build:
      context: ./java
    ports:
      - "8083:8081"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N:-43}

  java-server-single:
    build:
      context: ./java
    ports:
      - "8084:8081"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N:-43}
      - SERVER_TOMCAT_THREADS_MAX=1

  php-server:
    build:
      context: ./php
    ports:
      - "8085:8080"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N_PHP:-38}
      - PHP_FPM_WORKERS=4

  php-server-single:
    build:
      context: ./php
    ports:
      - "8086:8080"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N_PHP:-38}
      - PHP_FPM_WORKERS=1

  ruby-server:
    build:
      context: ./ruby
    ports:
      - "8087:8080"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N_RUBY:-40}
      - WEB_CONCURRENCY=4

  ruby-server-single:
    build:
      context: ./ruby
    ports:
      - "8088:8080"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N_RUBY:-40}
      - WEB_CONCURRENCY=1

  rust-server:
    build:
      context: ./rust/actix-web
    ports:
      - "8089:8080"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N:-43}
      - ACTIX_WORKERS=4

  rust-server-single:
    build:
      context: ./rust/actix-web
    ports:
      - "8090:8080"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N:-43}
      - ACTIX_WORKERS=1

  rust-axum-server:
    build:
      context: ./rust/axum
    ports:
      - "8091:8080"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N:-43}
      - TOKIO_WORKERS=4

  rust-axum-server-single:
    build:
      context: ./rust/axum
    ports:
      - "8092:8080"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N:-43}
      - TOKIO_WORKERS=1

  python-server:
    build:
      context: ./python
    ports:
      - "8093:8080"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N_PYTHON:-38}
      - UVICORN_WORKERS=4

  python-server-single:
    build:
      context: ./python
    ports:
      - "8094:8080"
    environment:
      - HEAVY_CALC_N=${HEAVY_CALC_N_PYTHON:-38}
      - UVICORN_WORKERS=1
```

**Step 2: Create scripts/load-test.sh**

Results are echoed immediately as each request completes, making serial vs parallel behavior visually obvious.

```bash
#!/usr/bin/env bash
set -uo pipefail

PORT=${1:-3000}
CONCURRENT=${2:-3}
URL="http://localhost:${PORT}/heavy"

echo "=== Load Test ==="
echo "Target: ${URL}"
echo "Concurrent requests: ${CONCURRENT}"
echo ""

for i in $(seq 1 "${CONCURRENT}"); do
  (
    RESPONSE=$(curl -s "${URL}")
    echo "[Request ${i}] ${RESPONSE}"
  ) &
done

wait
```

**Step 3: Make script executable and commit**

```bash
chmod +x 01_process_thread/scripts/load-test.sh
git add 01_process_thread/docker-compose.yml 01_process_thread/scripts/
git commit -m "feat: add docker-compose.yml and load test script"
```

---

### Task 6: Verify TypeScript, Go, Java with Docker Compose

**Step 1: Build and start all services**

```bash
cd 01_process_thread && docker compose up --build -d
```

**Step 2: Health check**

```bash
curl http://localhost:3000/health   # TypeScript
curl http://localhost:8081/health   # Go
curl http://localhost:8083/health   # Java
```

**Step 3: Run load tests**

```bash
./scripts/load-test.sh 3000   # TypeScript — expect serial
./scripts/load-test.sh 8081   # Go — expect parallel
./scripts/load-test.sh 8083   # Java — expect parallel
```

**Step 4: Verify results**

- **TypeScript**: Same `threadId`, staggered `startedAt` (serial)
- **Go**: Different `threadId` (goroutine IDs), nearly simultaneous `startedAt` (parallel)
- **Java**: Different `threadId` (thread names/IDs), nearly simultaneous `startedAt` (parallel)

---

### Task 7: PHP (Laravel + PHP-FPM) Server

**Files:**
- Create: `01_process_thread/php/app/Http/Controllers/HeavyController.php`
- Create: `01_process_thread/php/routes/web.php`
- Create: `01_process_thread/php/www.conf`
- Create: `01_process_thread/php/nginx.conf`
- Create: `01_process_thread/php/supervisord.conf`
- Create: `01_process_thread/php/entrypoint.sh`
- Create: `01_process_thread/php/Dockerfile`

**Step 1: Create HeavyController.php**

Uses `getmypid()` for FPM worker process ID and `getenv('HEAVY_CALC_N')` (not Laravel's `env()`) for reliable PHP-FPM env var access.

```php
<?php

namespace App\Http\Controllers;

use Illuminate\Http\JsonResponse;

class HeavyController extends Controller
{
    public function health(): JsonResponse
    {
        return response()->json([
            'status' => 'ok',
            'language' => 'php',
        ]);
    }

    public function heavy(): JsonResponse
    {
        $n = (int) (getenv('HEAVY_CALC_N') ?: 38);

        $startedAt = gmdate('Y-m-d\TH:i:s.v\Z');
        $startNs = hrtime(true);

        $this->fibonacci($n);

        $endNs = hrtime(true);
        $finishedAt = gmdate('Y-m-d\TH:i:s.v\Z');
        $durationMs = (int) (($endNs - $startNs) / 1_000_000);

        return response()->json([
            'language' => 'php',
            'threadId' => 'fpm-worker-' . getmypid(),
            'startedAt' => $startedAt,
            'finishedAt' => $finishedAt,
            'durationMs' => $durationMs,
        ]);
    }

    private function fibonacci(int $n): int
    {
        if ($n <= 1) return $n;
        return $this->fibonacci($n - 1) + $this->fibonacci($n - 2);
    }
}
```

**Step 2: Create routes/web.php**

```php
<?php

use App\Http\Controllers\HeavyController;
use Illuminate\Support\Facades\Route;

Route::get('/health', [HeavyController::class, 'health']);
Route::get('/heavy', [HeavyController::class, 'heavy']);
```

**Step 3: Create www.conf (FPM pool template)**

`${PHP_FPM_WORKERS}` is substituted at runtime by `envsubst` in `entrypoint.sh`. `clear_env = no` is critical for env var propagation.

```ini
[www]
user = www-data
group = www-data
listen = 127.0.0.1:9000
pm = static
pm.max_children = ${PHP_FPM_WORKERS}
clear_env = no
```

**Step 4: Create nginx.conf**

```nginx
server {
    listen 8080;
    server_name _;
    root /var/www/html/public;
    index index.php;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        fastcgi_pass 127.0.0.1:9000;
        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
        include fastcgi_params;
    }
}
```

**Step 5: Create supervisord.conf**

```ini
[supervisord]
nodaemon=true
user=root

[program:php-fpm]
command=php-fpm --nodaemonize
autostart=true
autorestart=true

[program:nginx]
command=nginx -g "daemon off;"
autostart=true
autorestart=true
```

**Step 6: Create entrypoint.sh**

Dynamically sets FPM worker count from `PHP_FPM_WORKERS` env var using `envsubst`.

```bash
#!/usr/bin/env bash
set -euo pipefail

export PHP_FPM_WORKERS="${PHP_FPM_WORKERS:-4}"

envsubst '${PHP_FPM_WORKERS}' < /usr/local/etc/php-fpm.d/www.conf.template > /usr/local/etc/php-fpm.d/www.conf

exec /usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf
```

**Step 7: Create Dockerfile**

Multi-stage build: composer scaffolds Laravel project, then runtime stage adds nginx + supervisord.

```dockerfile
FROM composer:2 AS builder

WORKDIR /app

RUN composer create-project laravel/laravel . --prefer-dist --no-interaction --no-dev

COPY routes/web.php /app/routes/web.php
COPY app/Http/Controllers/HeavyController.php /app/app/Http/Controllers/HeavyController.php

RUN php artisan key:generate && \
    php artisan config:clear && \
    php artisan route:clear

FROM php:8.4-fpm

ENV SESSION_DRIVER=array
ENV APP_DEBUG=false

RUN apt-get update && apt-get install -y --no-install-recommends \
    nginx \
    supervisor \
    gettext-base \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /var/www/html

COPY --from=builder /app /var/www/html

RUN chown -R www-data:www-data /var/www/html/storage /var/www/html/bootstrap/cache

COPY nginx.conf /etc/nginx/sites-available/default
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY www.conf /usr/local/etc/php-fpm.d/www.conf.template
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
```

**Step 8: Commit**

```bash
git add 01_process_thread/php/
git commit -m "feat: add PHP Laravel server with PHP-FPM for process/thread comparison"
```

---

### Task 8: Ruby (Rails + Puma) Server

**Files:**
- Create: `01_process_thread/ruby/app/controllers/heavy_controller.rb`
- Create: `01_process_thread/ruby/config/routes.rb`
- Create: `01_process_thread/ruby/config/puma.rb`
- Create: `01_process_thread/ruby/Dockerfile`

**Step 1: Create heavy_controller.rb**

Uses `Process.pid` for Puma worker process ID and `Process.clock_gettime` for nanosecond precision timing.

```ruby
class HeavyController < ApplicationController
  def health
    render json: { status: 'ok', language: 'ruby' }
  end

  def heavy
    n = (ENV['HEAVY_CALC_N'] || '40').to_i

    started_at = Time.now.utc.strftime('%Y-%m-%dT%H:%M:%S.%3NZ')
    start_ns = Process.clock_gettime(Process::CLOCK_MONOTONIC, :nanosecond)

    fibonacci(n)

    end_ns = Process.clock_gettime(Process::CLOCK_MONOTONIC, :nanosecond)
    finished_at = Time.now.utc.strftime('%Y-%m-%dT%H:%M:%S.%3NZ')
    duration_ms = ((end_ns - start_ns) / 1_000_000.0).round

    render json: {
      language: 'ruby',
      threadId: "puma-worker-#{Process.pid}",
      startedAt: started_at,
      finishedAt: finished_at,
      durationMs: duration_ms
    }
  end

  private

  def fibonacci(n)
    return n if n <= 1
    fibonacci(n - 1) + fibonacci(n - 2)
  end
end
```

**Step 2: Create config/routes.rb**

```ruby
Rails.application.routes.draw do
  get 'health', to: 'heavy#health'
  get 'heavy', to: 'heavy#heavy'
end
```

**Step 3: Create config/puma.rb**

Single thread per worker since GVL prevents CPU-bound thread parallelism.

```ruby
workers ENV.fetch("WEB_CONCURRENCY", 4)

threads_count = ENV.fetch("RAILS_MAX_THREADS", 1).to_i
threads threads_count, threads_count

port ENV.fetch("PORT", 8080)
environment ENV.fetch("RAILS_ENV", "production")

preload_app!
```

**Step 4: Create Dockerfile**

Multi-stage build: `rails new --api --minimal` scaffolds the project, then slim runtime image.

```dockerfile
FROM ruby:3.4 AS builder

WORKDIR /app

RUN gem install rails --no-document && \
    rails new . --api --minimal --skip-git --skip-test --skip-system-test \
    --skip-active-record --skip-action-mailer --skip-action-mailbox \
    --skip-action-text --skip-active-job --skip-active-storage \
    --skip-action-cable --no-interaction

COPY config/routes.rb /app/config/routes.rb
COPY config/puma.rb /app/config/puma.rb
COPY app/controllers/heavy_controller.rb /app/app/controllers/heavy_controller.rb

FROM ruby:3.4-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libgmp-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app /app
COPY --from=builder /usr/local/bundle /usr/local/bundle

ENV RAILS_ENV=production
ENV RAILS_LOG_TO_STDOUT=true
ENV SECRET_KEY_BASE=dummy-secret-for-demo-only

EXPOSE 8080

CMD ["bundle", "exec", "puma", "-C", "config/puma.rb"]
```

**Step 5: Commit**

```bash
git add 01_process_thread/ruby/
git commit -m "feat: add Ruby on Rails server with Puma for process/thread comparison"
```

---

### Task 9: Integration — docker-compose.yml, .gitignore, README.md

**Files:**
- Modify: `01_process_thread/docker-compose.yml` — Add PHP and Ruby services (multi-process and single-process)
- Modify: `01_process_thread/.gitignore` — Add PHP/Ruby artifact patterns
- Modify: `01_process_thread/README.md` — Update documentation

**Step 1: Update docker-compose.yml**

Add `php-server` (8085, workers=4), `php-server-single` (8086, workers=1), `ruby-server` (8087, workers=4), `ruby-server-single` (8088, workers=1). Use separate env vars `HEAVY_CALC_N_PHP` and `HEAVY_CALC_N_RUBY` for language-appropriate Fibonacci N values.

**Step 2: Update .gitignore**

Add patterns for PHP vendor directory and Ruby tmp/log directories.

**Step 3: Update README.md**

Add PHP and Ruby to the comparison table, add single-process variant descriptions, update test commands, and document new environment variables.

**Step 4: Commit**

```bash
git add 01_process_thread/docker-compose.yml 01_process_thread/.gitignore 01_process_thread/README.md
git commit -m "feat: integrate PHP and Ruby servers into docker-compose and docs"
```

---

### Task 11: Rust (Actix-web) Server

**Files:**
- Create: `01_process_thread/rust/actix-web/Cargo.toml`
- Create: `01_process_thread/rust/actix-web/src/main.rs`
- Create: `01_process_thread/rust/actix-web/Dockerfile`

**Step 1: Create Cargo.toml**

```toml
[package]
name = "process-thread-rust"
version = "0.1.0"
edition = "2021"

[dependencies]
actix-web = "4"
serde = { version = "1", features = ["derive"] }
serde_json = "1"
chrono = "0.4"
```

**Step 2: Create src/main.rs**

Actix-web server with `/health` and `/heavy` endpoints. Uses `#[actix_web::main]` macro. Worker count controlled via `ACTIX_WORKERS` env var and `HttpServer.workers(n)`. Thread ID extracted from `std::thread::current().id()` Debug output `ThreadId(N)` → `actix-worker-N`.

```rust
use actix_web::{web, App, HttpResponse, HttpServer};
use chrono::{SecondsFormat, Utc};
use serde_json::json;
use std::env;
use std::time::Instant;

fn fibonacci(n: u32) -> u64 {
    if n <= 1 { return n as u64; }
    fibonacci(n - 1) + fibonacci(n - 2)
}

fn get_thread_id() -> String {
    let debug_str = format!("{:?}", std::thread::current().id());
    let id_num = debug_str
        .trim_start_matches("ThreadId(")
        .trim_end_matches(')')
        .to_string();
    format!("actix-worker-{}", id_num)
}

async fn health() -> HttpResponse {
    HttpResponse::Ok().json(json!({"status": "ok", "language": "rust"}))
}

async fn heavy(data: web::Data<AppState>) -> HttpResponse {
    let started_at = Utc::now();
    let start = Instant::now();
    fibonacci(data.heavy_calc_n);
    let duration_ms = start.elapsed().as_millis() as u64;
    let finished_at = Utc::now();
    HttpResponse::Ok().json(json!({
        "language": "rust",
        "threadId": get_thread_id(),
        "startedAt": started_at.to_rfc3339_opts(SecondsFormat::Millis, true),
        "finishedAt": finished_at.to_rfc3339_opts(SecondsFormat::Millis, true),
        "durationMs": duration_ms
    }))
}

struct AppState { heavy_calc_n: u32 }

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    let heavy_calc_n: u32 = env::var("HEAVY_CALC_N")
        .ok().and_then(|v| v.parse().ok()).unwrap_or(43);
    let workers: usize = env::var("ACTIX_WORKERS")
        .ok().and_then(|v| v.parse().ok()).unwrap_or(4);
    HttpServer::new(move || {
        App::new()
            .app_data(web::Data::new(AppState { heavy_calc_n }))
            .route("/health", web::get().to(health))
            .route("/heavy", web::get().to(heavy))
    })
    .workers(workers)
    .bind(("0.0.0.0", 8080))?
    .run()
    .await
}
```

**Step 3: Create Dockerfile**

```dockerfile
FROM rust:1.93-bookworm AS builder
WORKDIR /app
COPY Cargo.toml ./
COPY src/ ./src/
RUN cargo build --release

FROM debian:bookworm-slim
COPY --from=builder /app/target/release/process-thread-rust /server
EXPOSE 8080
ENTRYPOINT ["/server"]
```

**Step 4: Commit**

```bash
git add 01_process_thread/rust/actix-web/
git commit -m "feat: add Rust Actix-web server for process/thread comparison"
```

---

### Task 12: Rust (Axum) Server

**Files:**
- Create: `01_process_thread/rust/axum/Cargo.toml`
- Create: `01_process_thread/rust/axum/src/main.rs`
- Create: `01_process_thread/rust/axum/Dockerfile`

**Step 1: Create Cargo.toml**

```toml
[package]
name = "process-thread-rust-axum"
version = "0.1.0"
edition = "2021"

[dependencies]
axum = "0.8"
tokio = { version = "1", features = ["full"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
chrono = "0.4"
```

**Step 2: Create src/main.rs**

Axum server with `/health` and `/heavy` endpoints. Uses manual `fn main()` + `tokio::runtime::Builder::new_multi_thread().worker_threads(n)` for runtime worker count control (unlike `#[tokio::main]` which doesn't support runtime configuration). Thread ID: `tokio-worker-N`. State requires `#[derive(Clone)]`.

```rust
use axum::{extract::State, routing::get, Json, Router};
use chrono::{SecondsFormat, Utc};
use serde_json::{json, Value};
use std::env;
use std::time::Instant;

fn fibonacci(n: u32) -> u64 {
    if n <= 1 { return n as u64; }
    fibonacci(n - 1) + fibonacci(n - 2)
}

fn get_thread_id() -> String {
    let debug_str = format!("{:?}", std::thread::current().id());
    let id_num = debug_str
        .trim_start_matches("ThreadId(")
        .trim_end_matches(')')
        .to_string();
    format!("tokio-worker-{}", id_num)
}

#[derive(Clone)]
struct AppState { heavy_calc_n: u32 }

async fn health() -> Json<Value> {
    Json(json!({"status": "ok", "language": "rust-axum"}))
}

async fn heavy(State(state): State<AppState>) -> Json<Value> {
    let started_at = Utc::now();
    let start = Instant::now();
    fibonacci(state.heavy_calc_n);
    let duration_ms = start.elapsed().as_millis() as u64;
    let finished_at = Utc::now();
    Json(json!({
        "language": "rust-axum",
        "threadId": get_thread_id(),
        "startedAt": started_at.to_rfc3339_opts(SecondsFormat::Millis, true),
        "finishedAt": finished_at.to_rfc3339_opts(SecondsFormat::Millis, true),
        "durationMs": duration_ms
    }))
}

fn main() {
    let heavy_calc_n: u32 = env::var("HEAVY_CALC_N")
        .ok().and_then(|v| v.parse().ok()).unwrap_or(43);
    let workers: usize = env::var("TOKIO_WORKERS")
        .ok().and_then(|v| v.parse().ok()).unwrap_or(4);

    let runtime = tokio::runtime::Builder::new_multi_thread()
        .worker_threads(workers)
        .enable_all()
        .build()
        .expect("Failed to build Tokio runtime");

    runtime.block_on(async {
        let app = Router::new()
            .route("/health", get(health))
            .route("/heavy", get(heavy))
            .with_state(AppState { heavy_calc_n });
        let listener = tokio::net::TcpListener::bind("0.0.0.0:8080").await.unwrap();
        println!("Rust (Axum) server listening on :8080 (workers={})", workers);
        axum::serve(listener, app).await.unwrap();
    });
}
```

**Step 3: Create Dockerfile**

```dockerfile
FROM rust:1.93-bookworm AS builder
WORKDIR /app
COPY Cargo.toml ./
COPY src/ ./src/
RUN cargo build --release

FROM debian:bookworm-slim
COPY --from=builder /app/target/release/process-thread-rust-axum /server
EXPOSE 8080
ENTRYPOINT ["/server"]
```

**Step 4: Commit**

```bash
git add 01_process_thread/rust/axum/
git commit -m "feat: add Rust Axum server for process/thread comparison"
```

---

### Task 13: Integration — Rust services in docker-compose.yml, .gitignore, README.md

**Files:**
- Modify: `01_process_thread/docker-compose.yml` — Add 4 Rust services (Actix-web multi/single, Axum multi/single)
- Modify: `01_process_thread/.gitignore` — Add `/rust/actix-web/target/` and `/rust/axum/target/`
- Modify: `01_process_thread/README.md` — Add Rust entries to all sections

**Step 1: Update docker-compose.yml**

Add `rust-server` (8089, ACTIX_WORKERS=4), `rust-server-single` (8090, ACTIX_WORKERS=1), `rust-axum-server` (8091, TOKIO_WORKERS=4), `rust-axum-server-single` (8092, TOKIO_WORKERS=1). Build contexts point to `./rust/actix-web` and `./rust/axum` respectively.

**Step 2: Update .gitignore and README.md**

**Step 3: Commit**

```bash
git add 01_process_thread/docker-compose.yml 01_process_thread/.gitignore 01_process_thread/README.md
git commit -m "feat: integrate Rust Actix-web and Axum servers into docker-compose and docs"
```

---

### Task 15: Python (FastAPI + Uvicorn) Server

**Files:**
- Create: `01_process_thread/python/requirements.txt`
- Create: `01_process_thread/python/main.py`
- Create: `01_process_thread/python/Dockerfile`

**Step 1: Create requirements.txt**

```
fastapi
uvicorn[standard]
```

`[standard]` extras で uvloop / httptools がインストールされ、マルチワーカー対応になる。

**Step 2: Create main.py**

FastAPI server with `/health` and `/heavy` endpoints. Uses `async def` (not `def`) so fibonacci blocks the event loop directly — if `def` were used, FastAPI would auto-run it in a thread pool, defeating the demo purpose. Uses `os.getpid()` for worker process ID.

```python
import os
import time
from datetime import datetime, timezone

from fastapi import FastAPI
from fastapi.responses import JSONResponse

app = FastAPI()

HEAVY_CALC_N = int(os.environ.get("HEAVY_CALC_N", "38"))


def fibonacci(n: int) -> int:
    if n <= 1:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)


def utc_now_iso() -> str:
    now = datetime.now(timezone.utc)
    return now.strftime("%Y-%m-%dT%H:%M:%S.") + f"{now.microsecond // 1000:03d}Z"


@app.get("/health")
async def health() -> JSONResponse:
    return JSONResponse({"status": "ok", "language": "python"})


@app.get("/heavy")
async def heavy() -> JSONResponse:
    started_at = utc_now_iso()
    start = time.monotonic()

    fibonacci(HEAVY_CALC_N)

    duration_ms = round((time.monotonic() - start) * 1000)
    finished_at = utc_now_iso()

    return JSONResponse({
        "language": "python",
        "threadId": f"uvicorn-worker-{os.getpid()}",
        "startedAt": started_at,
        "finishedAt": finished_at,
        "durationMs": duration_ms,
    })
```

**Step 3: Create Dockerfile**

Single-stage build using `python:3.13-slim-bookworm`. Shell form CMD with `exec` for env var expansion and signal propagation.

```dockerfile
FROM python:3.13-slim-bookworm

WORKDIR /app

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py ./

EXPOSE 8080

CMD exec uvicorn main:app --host 0.0.0.0 --port 8080 --workers ${UVICORN_WORKERS:-4}
```

**Step 4: Commit**

```bash
git add 01_process_thread/python/
git commit -m "feat: add Python FastAPI server with Uvicorn for process/thread comparison"
```

---

### Task 16: Integration — Python services in docker-compose.yml, .gitignore, README.md

**Files:**
- Modify: `01_process_thread/docker-compose.yml` — Add 2 Python services (multi/single worker)
- Modify: `01_process_thread/.gitignore` — Add `/python/__pycache__/`
- Modify: `01_process_thread/README.md` — Add Python entries to all sections

**Step 1: Update docker-compose.yml**

Add `python-server` (8093, UVICORN_WORKERS=4), `python-server-single` (8094, UVICORN_WORKERS=1). Use `HEAVY_CALC_N_PYTHON` (default 38) for language-appropriate Fibonacci N value.

**Step 2: Update .gitignore and README.md**

**Step 3: Commit**

```bash
git add 01_process_thread/docker-compose.yml 01_process_thread/.gitignore 01_process_thread/README.md
git commit -m "feat: integrate Python FastAPI server into docker-compose and docs"
```

---

### Task 14: Final Verification

**Step 1: Build and start all 15 services**

```bash
cd 01_process_thread && docker compose up --build -d
```

**Step 2: Health check all services**

```bash
curl http://localhost:3000/health   # TypeScript
curl http://localhost:8081/health   # Go
curl http://localhost:8082/health   # Go (single)
curl http://localhost:8083/health   # Java
curl http://localhost:8084/health   # Java (single)
curl http://localhost:8085/health   # PHP (multi-process)
curl http://localhost:8086/health   # PHP (single-process)
curl http://localhost:8087/health   # Ruby (multi-process)
curl http://localhost:8088/health   # Ruby (single-process)
curl http://localhost:8089/health   # Rust Actix-web (multi-threaded)
curl http://localhost:8090/health   # Rust Actix-web (single-threaded)
curl http://localhost:8091/health   # Rust Axum (multi-threaded)
curl http://localhost:8092/health   # Rust Axum (single-threaded)
curl http://localhost:8093/health   # Python (multi-process)
curl http://localhost:8094/health   # Python (single-process)
```

**Step 3: Run load tests**

```bash
./scripts/load-test.sh 3000   # TypeScript — expect serial
./scripts/load-test.sh 8081   # Go — expect parallel (different goroutine IDs)
./scripts/load-test.sh 8082   # Go single — expect time-sliced (different goroutine IDs, longer duration)
./scripts/load-test.sh 8083   # Java — expect parallel (different thread IDs)
./scripts/load-test.sh 8084   # Java single — expect serial (same thread ID)
./scripts/load-test.sh 8085   # PHP multi-process — expect parallel (different fpm-worker PIDs)
./scripts/load-test.sh 8086   # PHP single-process — expect serial (same fpm-worker PID)
./scripts/load-test.sh 8087   # Ruby multi-process — expect parallel (different puma-worker PIDs)
./scripts/load-test.sh 8088   # Ruby single-process — expect serial (same puma-worker PID)
./scripts/load-test.sh 8089   # Rust Actix-web multi — expect parallel (different actix-worker IDs)
./scripts/load-test.sh 8090   # Rust Actix-web single — expect serial (same actix-worker ID)
./scripts/load-test.sh 8091   # Rust Axum multi — expect parallel (different tokio-worker IDs)
./scripts/load-test.sh 8092   # Rust Axum single — expect serial (same tokio-worker ID)
./scripts/load-test.sh 8093   # Python multi-process — expect parallel (different uvicorn-worker PIDs)
./scripts/load-test.sh 8094   # Python single-process — expect serial (same uvicorn-worker PID)
```

**Step 4: Verify results**

| Service | Expected threadId | Expected Behavior |
|---------|-------------------|-------------------|
| TypeScript (3000) | Same `pid-N` | Serial (staggered start times) |
| Go (8081) | Different `goroutine-N` | Parallel (simultaneous) |
| Go single (8082) | Different `goroutine-N` | Time-sliced (each ~3x longer) |
| Java (8083) | Different `nio-N-M` | Parallel (simultaneous) |
| Java single (8084) | Same `nio-N-M` | Serial (staggered) |
| PHP multi (8085) | Different `fpm-worker-N` | Parallel (simultaneous) |
| PHP single (8086) | Same `fpm-worker-N` | Serial (staggered) |
| Ruby multi (8087) | Different `puma-worker-N` | Parallel (simultaneous) |
| Ruby single (8088) | Same `puma-worker-N` | Serial (staggered) |
| Rust Actix-web multi (8089) | Different `actix-worker-N` | Parallel (simultaneous) |
| Rust Actix-web single (8090) | Same `actix-worker-N` | Serial (staggered) |
| Rust Axum multi (8091) | Different `tokio-worker-N` | Parallel (simultaneous) |
| Rust Axum single (8092) | Same `tokio-worker-N` | Serial (staggered) |
| Python multi (8093) | Different `uvicorn-worker-N` | Parallel (simultaneous) |
| Python single (8094) | Same `uvicorn-worker-N` | Serial (staggered) |

**Step 5: Clean up**

```bash
docker compose down
```
