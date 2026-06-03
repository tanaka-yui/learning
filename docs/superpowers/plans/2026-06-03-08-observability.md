# 08_observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a self-contained `08_observability/` learning chapter where a small Go API and a React+Vite frontend are instrumented with OpenTelemetry (metrics, traces, logs), feeding a Grafana LGTM stack so a learner can follow one trace from the browser to the backend and correlate it with metrics and logs.

**Architecture:** Browser and Go app push OTLP to a single OTel Collector. The Collector fans out: traces → Tempo, metrics → Prometheus (scrape) → remote_write → Mimir, logs → Loki. Grafana reads Tempo/Mimir/Loki with cross-signal correlation (exemplars + trace_id). The Go app uses the same OTel idioms as `06_microservie/bff` (otelhttp, `otlptracegrpc`, `propagation.TraceContext{}`, `resource`+`semconv`), extended with MeterProvider and LoggerProvider.

**Tech Stack:** Go 1.26 (`net/http`, otelhttp, OTel SDK v1.43 line, `log/slog`), React + Vite + TypeScript, `@opentelemetry/sdk-trace-web`, OTel Collector (contrib), Prometheus, Grafana Mimir (monolithic), Tempo, Loki, Grafana, Docker Compose.

**Conventions to follow (from existing chapters):**
- Each Go server is its own module wired by a chapter-level `go.work` (see `07_network/go.work`: `go 1.26` + `toolchain go1.26.0` + `use ./<module>`).
- `Makefile` uses `.PHONY` targets `up/down/logs/test` with `docker compose up -d --build` (see `07_network/Makefile`).
- Docs are Japanese, である調, with tables/command examples and a closing「まとめ / 関連 doc」section (see `07_network/docs/08_observability.md`).
- Binaries produced by `go build` are gitignored per-module (see `07_network/.gitignore`).

**Port allocation (host side, chosen to avoid collisions with chapters 01–07):**

| Component | Host port | Container | Notes |
|---|---|---|---|
| Go API | 9100 | 9100 | `POST /api/checkout`, `GET /healthz` |
| Frontend (Vite) | 5174 | 5173 | 06 already uses 5173 |
| OTel Collector OTLP gRPC | 4319 | 4317 | app → collector (in-network uses 4317) |
| OTel Collector OTLP HTTP | 4320 | 4318 | browser → collector (CORS) |
| OTel Collector Prom exporter | 8889 | 8889 | Prometheus scrape target |
| Prometheus | 9090 | 9090 | scrapes collector, remote_write → mimir |
| Mimir (monolithic) | 9009 | 9009 | metrics store, Grafana datasource |
| Tempo | 3200 | 3200 | traces store + query |
| Loki | 3100 | 3100 | logs store (OTLP ingest at `/otlp`) |
| Grafana | 3001 | 3000 | 06 already uses 3000 |

**In-cluster service URLs (used inside docker-compose network):**
- app → collector: `otel-collector:4317` (gRPC)
- browser → collector: `http://localhost:4320` (host HTTP)
- collector → tempo: `tempo:4317`
- collector → loki: `http://loki:3100/otlp`
- prometheus scrape: `otel-collector:8889`
- prometheus remote_write: `http://mimir:9009/api/v1/push`
- grafana → mimir: `http://mimir:9009/prometheus`
- grafana → tempo: `http://tempo:3200`
- grafana → loki: `http://loki:3100`

---

## File Structure

```
08_observability/
├── README.md
├── go.work                     # go 1.26 + toolchain + use ./app
├── .gitignore
├── docker-compose.yml
├── Makefile
├── app/
│   ├── go.mod                  # module: observability/app
│   ├── main.go                 # wiring: otel init, otelhttp, RED middleware, slog
│   ├── Dockerfile
│   └── internal/
│       ├── obs/
│       │   ├── otel.go         # InitTelemetry: tracer + meter + logger providers
│       │   └── metrics.go      # RED instruments + HTTP middleware
│       └── checkout/
│           ├── checkout.go     # validate/reserveStock/charge (+ FLAKE_RATE)
│           ├── checkout_test.go
│           ├── handler.go      # HTTP handler over checkout
│           └── handler_test.go
├── frontend/
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── index.html
│   ├── Dockerfile.dev
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       └── otel.ts             # web tracer + fetch instrumentation + web-vitals
├── infra/
│   ├── otel-collector/config.yaml
│   ├── prometheus/prometheus.yml
│   ├── mimir/mimir.yaml
│   ├── tempo/tempo.yaml
│   ├── loki/loki.yaml
│   └── grafana/provisioning/
│       ├── datasources/datasources.yaml
│       └── dashboards/dashboards.yaml + red.json
└── docs/
    ├── 01_concepts.md
    ├── 02_otel_sdk_go.md
    ├── 03_traces_e2e.md
    ├── 04_metrics_prom_mimir.md
    ├── 05_logs_loki.md
    ├── 06_grafana_correlation.md
    ├── 07_collector.md
    └── 08_oss_landscape.md
```

---

## Phase 0 — Scaffold

### Task 1: Chapter skeleton

**Files:**
- Create: `08_observability/go.work`
- Create: `08_observability/.gitignore`
- Create: `08_observability/app/go.mod`
- Create: `08_observability/README.md` (placeholder header; full content in Task 24)

- [ ] **Step 1: Create directory tree**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning
mkdir -p 08_observability/app/internal/obs 08_observability/app/internal/checkout
mkdir -p 08_observability/frontend/src
mkdir -p 08_observability/infra/otel-collector 08_observability/infra/prometheus 08_observability/infra/mimir 08_observability/infra/tempo 08_observability/infra/loki
mkdir -p 08_observability/infra/grafana/provisioning/datasources 08_observability/infra/grafana/provisioning/dashboards
mkdir -p 08_observability/docs
```

- [ ] **Step 2: Create `app/go.mod`**

```
module observability/app

go 1.26
```

- [ ] **Step 3: Create `go.work`**

```
go 1.26

toolchain go1.26.0

use ./app
```

- [ ] **Step 4: Create `.gitignore`** (binaries from `go build`, plus node/vite artifacts)

```
# Go binaries
/app/app
/app/main
# Node / Vite
/frontend/node_modules/
/frontend/dist/
```

- [ ] **Step 5: Create placeholder `README.md`**

```markdown
# 08_observability: OpenTelemetry 観測性ハンズオン
```

- [ ] **Step 6: Verify Go workspace resolves**

Run: `cd 08_observability && go work sync && cd ..`
Expected: no error (empty module is valid).

- [ ] **Step 7: Commit**

```bash
git add 08_observability
git commit -m "feat(08_observability): scaffold chapter skeleton"
```

---

## Phase 1 — Go API with OpenTelemetry (TDD)

### Task 2: Checkout domain logic

**Files:**
- Create: `08_observability/app/internal/checkout/checkout.go`
- Test: `08_observability/app/internal/checkout/checkout_test.go`

The domain has three steps. `Charge` fails with probability `flakeRate` so traces/metrics/logs show errors. Determinism in tests is achieved by injecting the random source.

- [ ] **Step 1: Write the failing test**

`app/internal/checkout/checkout_test.go`:

```go
package checkout

import (
	"context"
	"errors"
	"testing"
)

func TestValidate(t *testing.T) {
	if err := Validate(Request{Item: "book", Qty: 1}); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	if err := Validate(Request{Item: "", Qty: 1}); err == nil {
		t.Fatal("empty item should be rejected")
	}
	if err := Validate(Request{Item: "book", Qty: 0}); err == nil {
		t.Fatal("zero qty should be rejected")
	}
}

func TestService_Checkout_Success(t *testing.T) {
	svc := NewService(0.0) // never flake
	res, err := svc.Checkout(context.Background(), Request{Item: "book", Qty: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OrderID == "" {
		t.Fatal("expected an order id")
	}
}

func TestService_Checkout_FlakeAlwaysFails(t *testing.T) {
	svc := NewService(1.0) // always flake
	_, err := svc.Checkout(context.Background(), Request{Item: "book", Qty: 2})
	if !errors.Is(err, ErrChargeFailed) {
		t.Fatalf("expected ErrChargeFailed, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 08_observability/app && go test ./internal/checkout/ -run TestService -v`
Expected: FAIL — `undefined: NewService` etc.

- [ ] **Step 3: Write minimal implementation**

`app/internal/checkout/checkout.go`:

```go
// Package checkout is the demo domain: a 3-step checkout whose final
// charge step fails with a configurable probability so the observability
// stack has interesting traces, error metrics, and error logs to show.
package checkout

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
)

type Request struct {
	Item string `json:"item"`
	Qty  int    `json:"qty"`
}

type Result struct {
	OrderID string `json:"order_id"`
}

var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrChargeFailed   = errors.New("charge failed")
)

func Validate(r Request) error {
	if r.Item == "" {
		return fmt.Errorf("%w: item is required", ErrInvalidRequest)
	}
	if r.Qty <= 0 {
		return fmt.Errorf("%w: qty must be > 0", ErrInvalidRequest)
	}
	return nil
}

type Service struct {
	flakeRate float64
	rng       func() float64
}

func NewService(flakeRate float64) *Service {
	return &Service{flakeRate: flakeRate, rng: rand.Float64}
}

func (s *Service) Checkout(ctx context.Context, r Request) (Result, error) {
	if err := Validate(r); err != nil {
		return Result{}, err
	}
	if err := s.reserveStock(ctx, r); err != nil {
		return Result{}, err
	}
	if err := s.charge(ctx, r); err != nil {
		return Result{}, err
	}
	return Result{OrderID: fmt.Sprintf("ord-%s-%d", r.Item, r.Qty)}, nil
}

func (s *Service) reserveStock(_ context.Context, _ Request) error {
	return nil
}

func (s *Service) charge(_ context.Context, _ Request) error {
	if s.rng() < s.flakeRate {
		return ErrChargeFailed
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd 08_observability/app && go test ./internal/checkout/ -run TestService -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add 08_observability/app/internal/checkout/checkout.go 08_observability/app/internal/checkout/checkout_test.go 08_observability/app/go.mod 08_observability/app/go.sum
git commit -m "feat(08_observability): checkout domain with configurable flake"
```

---

### Task 3: HTTP handler over checkout

**Files:**
- Create: `08_observability/app/internal/checkout/handler.go`
- Test: `08_observability/app/internal/checkout/handler_test.go`

The handler decodes JSON, calls the service, and maps errors to status codes (400 for invalid request, 502 for charge failure, 200 for success). Tracing spans are added in Task 5 (wiring) so this stays a pure HTTP test.

- [ ] **Step 1: Write the failing test**

`app/internal/checkout/handler_test.go`:

```go
package checkout

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler_Success(t *testing.T) {
	h := NewHandler(NewService(0.0))
	req := httptest.NewRequest(http.MethodPost, "/api/checkout", strings.NewReader(`{"item":"book","qty":1}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "order_id") {
		t.Fatalf("missing order_id: %s", rec.Body.String())
	}
}

func TestHandler_InvalidRequest(t *testing.T) {
	h := NewHandler(NewService(0.0))
	req := httptest.NewRequest(http.MethodPost, "/api/checkout", strings.NewReader(`{"item":"","qty":0}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestHandler_ChargeFailure(t *testing.T) {
	h := NewHandler(NewService(1.0))
	req := httptest.NewRequest(http.MethodPost, "/api/checkout", strings.NewReader(`{"item":"book","qty":1}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 08_observability/app && go test ./internal/checkout/ -run TestHandler -v`
Expected: FAIL — `undefined: NewHandler`.

- [ ] **Step 3: Write minimal implementation**

`app/internal/checkout/handler.go`:

```go
package checkout

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
		return
	}
	res, err := h.svc.Checkout(r.Context(), req)
	switch {
	case errors.Is(err, ErrInvalidRequest):
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	case errors.Is(err, ErrChargeFailed):
		http.Error(w, `{"error":"charge failed"}`, http.StatusBadGateway)
		return
	case err != nil:
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd 08_observability/app && go test ./internal/checkout/ -v`
Expected: PASS (all checkout tests).

- [ ] **Step 5: Commit**

```bash
git add 08_observability/app/internal/checkout/handler.go 08_observability/app/internal/checkout/handler_test.go
git commit -m "feat(08_observability): checkout HTTP handler with status mapping"
```

---

### Task 4: OTel initialization (tracer + meter + logger providers)

**Files:**
- Create: `08_observability/app/internal/obs/otel.go`

This extends `06_microservie/bff/internal/obs/otel.go` (tracer-only) with a MeterProvider (periodic OTLP push) and a LoggerProvider (for the slog bridge). It returns a single shutdown func. No unit test — verified by `go build` and at runtime in Task 16.

- [ ] **Step 1: Add dependencies**

Run:

```bash
cd 08_observability/app
go get go.opentelemetry.io/otel@v1.43.0
go get go.opentelemetry.io/otel/sdk@v1.43.0
go get go.opentelemetry.io/otel/sdk/metric@v1.43.0
go get go.opentelemetry.io/otel/sdk/log
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.43.0
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc@v1.43.0
go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc
go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.68.0
go get go.opentelemetry.io/contrib/bridges/otelslog
```

Expected: modules added to `go.mod`. (The log SDK/exporter are on the v0.x line tied to v1.43; `go get` resolves the compatible version.)

- [ ] **Step 2: Write `otel.go`**

```go
// Package obs wires OpenTelemetry providers for traces, metrics, and logs.
// It mirrors 06_microservie/bff/internal/obs/otel.go (tracer-only) and adds
// a MeterProvider (periodic OTLP push) and a LoggerProvider (for slog).
package obs

import (
	"context"
	"errors"
	"os"

	"go.opentelemetry.io/otel"
	otlpmetric "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otlplog "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	otlptrace "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTelemetry sets global Tracer/Meter providers and returns the global
// LoggerProvider (for the slog bridge) plus a single shutdown func.
func InitTelemetry(ctx context.Context, serviceName string) (*sdklog.LoggerProvider, func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "otel-collector:4317"
	}

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, nil, err
	}

	traceExp, err := otlptrace.New(ctx, otlptrace.WithEndpoint(endpoint), otlptrace.WithInsecure())
	if err != nil {
		return nil, nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	metricExp, err := otlpmetric.New(ctx, otlpmetric.WithEndpoint(endpoint), otlpmetric.WithInsecure())
	if err != nil {
		return nil, nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	logExp, err := otlplog.New(ctx, otlplog.WithEndpoint(endpoint), otlplog.WithInsecure())
	if err != nil {
		return nil, nil, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)

	shutdown := func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx), lp.Shutdown(ctx))
	}
	return lp, shutdown, nil
}
```

- [ ] **Step 3: Verify it builds**

Run: `cd 08_observability/app && go build ./...`
Expected: success, no errors.

- [ ] **Step 4: Commit**

```bash
git add 08_observability/app/internal/obs/otel.go 08_observability/app/go.mod 08_observability/app/go.sum
git commit -m "feat(08_observability): OTel tracer+meter+logger provider init"
```

---

### Task 5: RED metrics middleware + main wiring + Dockerfile

**Files:**
- Create: `08_observability/app/internal/obs/metrics.go`
- Create: `08_observability/app/main.go`
- Create: `08_observability/app/Dockerfile`
- Test: `08_observability/app/internal/obs/metrics_test.go`

RED = Rate (request count), Errors (error count), Duration (latency histogram). The middleware records all three keyed by route + status. The test asserts the middleware passes requests through and preserves status; instrument correctness is verified end-to-end in Task 16.

- [ ] **Step 1: Write the failing test**

`app/internal/obs/metrics_test.go`:

```go
package obs

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestREDMiddleware_PassesThrough(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	mw, err := NewREDMiddleware("test")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status not preserved: %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 08_observability/app && go test ./internal/obs/ -v`
Expected: FAIL — `undefined: NewREDMiddleware`.

- [ ] **Step 3: Write `metrics.go`**

```go
package obs

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// statusRecorder captures the response status code for metric labelling.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// NewREDMiddleware returns net/http middleware recording the RED signals
// (request count, error count, latency histogram) via the global MeterProvider.
func NewREDMiddleware(name string) (func(http.Handler) http.Handler, error) {
	meter := otel.GetMeterProvider().Meter(name)
	reqs, err := meter.Int64Counter("http.server.requests", metric.WithDescription("total HTTP requests"))
	if err != nil {
		return nil, err
	}
	errs, err := meter.Int64Counter("http.server.errors", metric.WithDescription("HTTP requests with status >= 500"))
	if err != nil {
		return nil, err
	}
	dur, err := meter.Float64Histogram("http.server.duration", metric.WithUnit("ms"), metric.WithDescription("HTTP request latency"))
	if err != nil {
		return nil, err
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			attrs := metric.WithAttributes(
				attribute.String("http.route", r.URL.Path),
				attribute.Int("http.status_code", rec.status),
			)
			reqs.Add(r.Context(), 1, attrs)
			if rec.status >= 500 {
				errs.Add(r.Context(), 1, attrs)
			}
			dur.Record(r.Context(), float64(time.Since(start).Milliseconds()), attrs)
		})
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd 08_observability/app && go test ./internal/obs/ -v`
Expected: PASS. (No global MeterProvider set in the test → `otel.GetMeterProvider()` returns a working no-op-ish provider whose instruments succeed.)

- [ ] **Step 5: Write `main.go`** (wires telemetry, slog bridge, internal spans, otelhttp, RED middleware)

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"

	"observability/app/internal/checkout"
	"observability/app/internal/obs"
)

const serviceName = "checkout-api"

func main() {
	ctx := context.Background()
	lp, shutdown, err := obs.InitTelemetry(ctx, serviceName)
	if err != nil {
		panic(err)
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(sctx)
	}()

	// slog with OTel bridge: every log line carries trace_id/span_id when a span is active.
	logger := slog.New(otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(lp)))
	slog.SetDefault(logger)

	flake := parseFlake(os.Getenv("FLAKE_RATE"))
	svc := checkout.NewService(flake)
	handler := checkout.NewHandler(svc)

	red, err := obs.NewREDMiddleware(serviceName)
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("POST /api/checkout", instrumentedCheckout(handler))

	root := red(otelhttp.NewHandler(mux, "http"))

	addr := ":9100"
	slog.Info("starting checkout-api", "addr", addr, "flake_rate", flake)
	srv := &http.Server{Addr: addr, Handler: root, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
	}
}

// instrumentedCheckout adds an application span and a structured log around the handler.
func instrumentedCheckout(h http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "checkout.process")
		defer span.End()
		slog.InfoContext(ctx, "checkout requested", "path", r.URL.Path)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func parseFlake(s string) float64 {
	switch s {
	case "":
		return 0.0
	default:
		// keep it simple: accept values like "0.3"; ignore parse errors → 0.0
		var f float64
		if _, err := fmtSscan(s, &f); err != nil {
			return 0.0
		}
		return f
	}
}
```

Note: replace `fmtSscan` with a real parse — use `strconv.ParseFloat`. Final `parseFlake`:

```go
func parseFlake(s string) float64 {
	if s == "" {
		return 0.0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return f
}
```

…and add `"strconv"` to the import block; remove the `fmtSscan` placeholder shown above.

- [ ] **Step 6: Verify build + full test**

Run: `cd 08_observability/app && go build ./... && go test ./...`
Expected: build success; all tests PASS.

- [ ] **Step 7: Write `Dockerfile`** (mirror `06_microservie/bff/Dockerfile` style — multi-stage)

```dockerfile
FROM golang:1.26 AS build
WORKDIR /src
COPY app/go.mod app/go.sum ./app/
WORKDIR /src/app
RUN go mod download
COPY app/ ./
RUN CGO_ENABLED=0 go build -o /out/app .

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/app /app
EXPOSE 9100
ENTRYPOINT ["/app"]
```

(If `06_microservie/bff/Dockerfile` differs, match its base images and build-context conventions; the compose build context in Task 14 is `./` with this dockerfile path.)

- [ ] **Step 8: Commit**

```bash
git add 08_observability/app
git commit -m "feat(08_observability): RED middleware, main wiring, Dockerfile"
```

---

## Phase 2 — Frontend (React + Vite + OTel web)

### Task 6: Vite React scaffold calling the API

**Files:**
- Create: `08_observability/frontend/package.json`
- Create: `08_observability/frontend/vite.config.ts`
- Create: `08_observability/frontend/tsconfig.json`
- Create: `08_observability/frontend/index.html`
- Create: `08_observability/frontend/src/main.tsx`
- Create: `08_observability/frontend/src/App.tsx`

Match `06_microservie/frontend` versions where possible (read its `package.json` first and align React/Vite versions).

- [ ] **Step 1: Read 06 frontend versions for alignment**

Run: `cat 06_microservie/frontend/package.json`
Use its React, Vite, and TypeScript versions in the next step.

- [ ] **Step 2: Create `package.json`** (fill `<react>`, `<vite>`, `<typescript>` with the versions from Step 1; add OTel web deps)

```json
{
  "name": "observability-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "<react>",
    "react-dom": "<react>",
    "@opentelemetry/api": "^1.9.0",
    "@opentelemetry/sdk-trace-web": "^2.0.0",
    "@opentelemetry/context-zone": "^2.0.0",
    "@opentelemetry/exporter-trace-otlp-http": "^0.200.0",
    "@opentelemetry/instrumentation": "^0.200.0",
    "@opentelemetry/instrumentation-fetch": "^0.200.0",
    "@opentelemetry/resources": "^2.0.0",
    "@opentelemetry/semantic-conventions": "^1.30.0",
    "web-vitals": "^4.2.0"
  },
  "devDependencies": {
    "@types/react": "<types-react>",
    "@types/react-dom": "<types-react>",
    "@vitejs/plugin-react": "<plugin-react>",
    "typescript": "<typescript>",
    "vite": "<vite>"
  }
}
```

Note: after `npm install`, pin the exact OTel versions npm resolved. If the `^0.200.0`/`^2.0.0` ranges fail to resolve, run `npm install @opentelemetry/sdk-trace-web @opentelemetry/context-zone @opentelemetry/exporter-trace-otlp-http @opentelemetry/instrumentation @opentelemetry/instrumentation-fetch @opentelemetry/resources @opentelemetry/semantic-conventions web-vitals` and accept the resolved latest versions.

- [ ] **Step 3: Create `vite.config.ts`** (port 5173 in-container; host maps to 5174 in compose)

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: { host: "0.0.0.0", port: 5173 },
});
```

- [ ] **Step 4: Create `tsconfig.json`** (copy `06_microservie/frontend/tsconfig.json` verbatim, then adjust only if needed).

- [ ] **Step 5: Create `index.html`**

```html
<!doctype html>
<html lang="ja">
  <head><meta charset="UTF-8" /><title>08 Observability</title></head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 6: Create `src/main.tsx`** (imports `./otel` FIRST so instrumentation registers before any fetch)

```tsx
import "./otel";
import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./App";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

- [ ] **Step 7: Create `src/App.tsx`**

```tsx
import { useState } from "react";

const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:9100";

export function App() {
  const [result, setResult] = useState<string>("");

  async function checkout() {
    setResult("...");
    try {
      const res = await fetch(`${API_BASE}/api/checkout`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ item: "book", qty: 1 }),
      });
      setResult(`${res.status} ${await res.text()}`);
    } catch (e) {
      setResult(`error: ${String(e)}`);
    }
  }

  return (
    <main style={{ fontFamily: "sans-serif", padding: 32 }}>
      <h1>08 Observability デモ</h1>
      <button onClick={checkout}>Checkout を実行</button>
      <pre>{result}</pre>
    </main>
  );
}
```

- [ ] **Step 8: Commit**

```bash
git add 08_observability/frontend
git commit -m "feat(08_observability): React+Vite frontend calling checkout API"
```

---

### Task 7: Browser OTel instrumentation + Dockerfile.dev

**Files:**
- Create: `08_observability/frontend/src/otel.ts`
- Create: `08_observability/frontend/Dockerfile.dev`

`otel.ts` registers a `WebTracerProvider` exporting OTLP/HTTP to the collector, plus `FetchInstrumentation` with `propagateTraceHeaderCorsUrls` so the `traceparent` header reaches the Go API (default propagator is W3C TraceContext). Web Vitals are reported as console+span events (kept minimal; full metric export is optional polish).

- [ ] **Step 1: Write `otel.ts`**

```ts
import { WebTracerProvider, BatchSpanProcessor } from "@opentelemetry/sdk-trace-web";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { ZoneContextManager } from "@opentelemetry/context-zone";
import { registerInstrumentations } from "@opentelemetry/instrumentation";
import { FetchInstrumentation } from "@opentelemetry/instrumentation-fetch";
import { resourceFromAttributes } from "@opentelemetry/resources";
import { ATTR_SERVICE_NAME } from "@opentelemetry/semantic-conventions";
import { onLCP, onCLS, onINP } from "web-vitals";
import { trace } from "@opentelemetry/api";

const COLLECTOR = import.meta.env.VITE_OTEL_COLLECTOR_URL ?? "http://localhost:4320";
const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:9100";

const provider = new WebTracerProvider({
  resource: resourceFromAttributes({ [ATTR_SERVICE_NAME]: "checkout-frontend" }),
  spanProcessors: [
    new BatchSpanProcessor(new OTLPTraceExporter({ url: `${COLLECTOR}/v1/traces` })),
  ],
});

provider.register({ contextManager: new ZoneContextManager() });

registerInstrumentations({
  instrumentations: [
    new FetchInstrumentation({
      // make the browser send `traceparent` to the API across origins
      propagateTraceHeaderCorsUrls: [new RegExp(API_BASE)],
    }),
  ],
});

// Web Vitals → span events on a short-lived span (kept simple for the lab).
const tracer = trace.getTracer("web-vitals");
function reportVital(name: string, value: number) {
  const span = tracer.startSpan(`web-vital.${name}`);
  span.setAttribute("web_vital.value", value);
  span.end();
}
onLCP((m) => reportVital("LCP", m.value));
onCLS((m) => reportVital("CLS", m.value));
onINP((m) => reportVital("INP", m.value));
```

(If a `web-vitals` callback name differs in the resolved version, align with that version's exports — the package is stable on `onLCP/onCLS/onINP` in v4.)

- [ ] **Step 2: Write `Dockerfile.dev`** (mirror `06_microservie/frontend/Dockerfile.dev`)

```dockerfile
FROM node:22-alpine
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install
COPY . .
EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
```

- [ ] **Step 3: Verify the frontend builds locally**

Run: `cd 08_observability/frontend && npm install && npm run build`
Expected: `tsc` passes and `vite build` produces `dist/`. Fix any type errors before continuing.

- [ ] **Step 4: Commit** (include the generated `package-lock.json`; pin OTel versions resolved by npm into `package.json`)

```bash
git add 08_observability/frontend/src/otel.ts 08_observability/frontend/Dockerfile.dev 08_observability/frontend/package.json 08_observability/frontend/package-lock.json
git commit -m "feat(08_observability): browser OTel tracing + traceparent propagation"
```

---

## Phase 3 — Observability infrastructure

### Task 8: OTel Collector config

**Files:**
- Create: `08_observability/infra/otel-collector/config.yaml`

Receives OTLP (gRPC for the app, HTTP+CORS for the browser); exports traces→Tempo, metrics→Prometheus-scrapeable endpoint (:8889), logs→Loki OTLP.

- [ ] **Step 1: Write `config.yaml`**

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
        cors:
          allowed_origins:
            - "http://localhost:5174"

processors:
  batch:
    timeout: 1s
    send_batch_size: 512

exporters:
  otlp/tempo:
    endpoint: tempo:4317
    tls:
      insecure: true
  prometheus:
    endpoint: 0.0.0.0:8889
    enable_open_metrics: true   # exemplars for metric→trace jumps
  otlphttp/loki:
    endpoint: http://loki:3100/otlp
  debug:
    verbosity: basic

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/tempo, debug]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/loki]
```

- [ ] **Step 2: Commit**

```bash
git add 08_observability/infra/otel-collector/config.yaml
git commit -m "feat(08_observability): OTel Collector pipelines for traces/metrics/logs"
```

---

### Task 9: Tempo config

**Files:**
- Create: `08_observability/infra/tempo/tempo.yaml`

Single-binary Tempo with OTLP receiver and local storage.

- [ ] **Step 1: Write `tempo.yaml`**

```yaml
server:
  http_listen_port: 3200

distributor:
  receivers:
    otlp:
      protocols:
        grpc:
          endpoint: 0.0.0.0:4317

ingester:
  max_block_duration: 5m

storage:
  trace:
    backend: local
    local:
      path: /var/tempo/blocks
    wal:
      path: /var/tempo/wal
```

- [ ] **Step 2: Commit**

```bash
git add 08_observability/infra/tempo/tempo.yaml
git commit -m "feat(08_observability): Tempo single-binary config with OTLP receiver"
```

---

### Task 10: Prometheus config (scrape collector + remote_write → Mimir)

**Files:**
- Create: `08_observability/infra/prometheus/prometheus.yml`

This is the requested production pattern: Prometheus scrapes the collector's :8889 endpoint and forwards every sample to Mimir via `remote_write`.

- [ ] **Step 1: Write `prometheus.yml`**

```yaml
global:
  scrape_interval: 5s
  evaluation_interval: 5s

scrape_configs:
  - job_name: otel-collector
    static_configs:
      - targets: ["otel-collector:8889"]

remote_write:
  - url: http://mimir:9009/api/v1/push
    headers:
      X-Scope-OrgID: anonymous
```

- [ ] **Step 2: Commit**

```bash
git add 08_observability/infra/prometheus/prometheus.yml
git commit -m "feat(08_observability): Prometheus scrape collector + remote_write to Mimir"
```

---

### Task 11: Mimir monolithic config

**Files:**
- Create: `08_observability/infra/mimir/mimir.yaml`

Single-binary (`-target=all`) Mimir with filesystem storage and anonymous tenant — enough to receive `remote_write` and serve Grafana queries.

- [ ] **Step 1: Write `mimir.yaml`**

```yaml
multitenancy_enabled: false

server:
  http_listen_port: 9009

common:
  storage:
    backend: filesystem
    filesystem:
      dir: /data/mimir

blocks_storage:
  backend: filesystem
  filesystem:
    dir: /data/mimir/blocks
  bucket_store:
    sync_dir: /data/mimir/tsdb-sync
  tsdb:
    dir: /data/mimir/tsdb

compactor:
  data_dir: /data/mimir/compactor

ruler_storage:
  backend: filesystem
  filesystem:
    dir: /data/mimir/ruler

ingester:
  ring:
    replication_factor: 1
```

(The container is started with `-target=all` in compose. If Mimir rejects any field on the pinned image version, run the image once with `-modules` / `-help-all` to confirm field names and adjust — Mimir config keys are version-sensitive.)

- [ ] **Step 2: Commit**

```bash
git add 08_observability/infra/mimir/mimir.yaml
git commit -m "feat(08_observability): Mimir monolithic config with filesystem storage"
```

---

### Task 12: Loki config

**Files:**
- Create: `08_observability/infra/loki/loki.yaml`

Single-binary Loki with filesystem storage; OTLP ingestion is enabled by default at `/otlp/v1/logs` in Loki 3.x.

- [ ] **Step 1: Write `loki.yaml`**

```yaml
auth_enabled: false

server:
  http_listen_port: 3100

common:
  instance_addr: 127.0.0.1
  path_prefix: /loki
  storage:
    filesystem:
      chunks_directory: /loki/chunks
      rules_directory: /loki/rules
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory

schema_config:
  configs:
    - from: 2024-01-01
      store: tsdb
      object_store: filesystem
      schema: v13
      index:
        prefix: index_
        period: 24h

limits_config:
  allow_structured_metadata: true
```

- [ ] **Step 2: Commit**

```bash
git add 08_observability/infra/loki/loki.yaml
git commit -m "feat(08_observability): Loki single-binary config with OTLP ingestion"
```

---

### Task 13: Grafana provisioning (datasources with correlation + RED dashboard)

**Files:**
- Create: `08_observability/infra/grafana/provisioning/datasources/datasources.yaml`
- Create: `08_observability/infra/grafana/provisioning/dashboards/dashboards.yaml`
- Create: `08_observability/infra/grafana/provisioning/dashboards/red.json`

Mimir (metrics, default), Tempo (traces, with tracesToLogs→Loki and tracesToMetrics→Mimir), Loki (logs, with derived field trace_id→Tempo).

- [ ] **Step 1: Write `datasources.yaml`**

```yaml
apiVersion: 1
datasources:
  - name: Mimir
    type: prometheus
    access: proxy
    url: http://mimir:9009/prometheus
    isDefault: true
    jsonData:
      httpHeaderName1: X-Scope-OrgID
      exemplarTraceIdDestinations:
        - name: trace_id
          datasourceUid: tempo
    secureJsonData:
      httpHeaderValue1: anonymous

  - name: Tempo
    type: tempo
    uid: tempo
    access: proxy
    url: http://tempo:3200
    jsonData:
      tracesToLogsV2:
        datasourceUid: loki
        filterByTraceID: true
      tracesToMetrics:
        datasourceUid: mimir

  - name: Loki
    type: loki
    uid: loki
    access: proxy
    url: http://loki:3100
    jsonData:
      derivedFields:
        - name: trace_id
          matcherRegex: "trace_id=(\\w+)"
          url: "$${__value.raw}"
          datasourceUid: tempo
```

(Set the Mimir datasource `uid: mimir` so `tracesToMetrics.datasourceUid: mimir` resolves; add `uid: mimir` under the Mimir entry.)

- [ ] **Step 2: Write `dashboards.yaml`** (dashboard provider)

```yaml
apiVersion: 1
providers:
  - name: default
    folder: ""
    type: file
    options:
      path: /etc/grafana/provisioning/dashboards
```

- [ ] **Step 3: Write `red.json`** — a minimal dashboard with three panels backed by Mimir/PromQL:
  - Request rate: `sum(rate(http_server_requests_total[1m]))`
  - Error rate: `sum(rate(http_server_errors_total[1m]))`
  - p95 latency: `histogram_quantile(0.95, sum(rate(http_server_duration_milliseconds_bucket[5m])) by (le))`

Use Grafana's standard dashboard JSON schema (`{"title":"RED — checkout-api","panels":[...],"schemaVersion":39,"templating":{"list":[]},"time":{"from":"now-15m","to":"now"}}`) with three `timeseries` panels whose `datasource` is `{"type":"prometheus","uid":"mimir"}` and the targets above. (Metric names follow OTel→Prometheus translation: `http.server.requests` → `http_server_requests_total`, `http.server.duration` ms histogram → `http_server_duration_milliseconds_bucket`. Confirm exact names in Task 16 via the Prometheus UI and adjust the JSON if needed.)

- [ ] **Step 4: Commit**

```bash
git add 08_observability/infra/grafana
git commit -m "feat(08_observability): Grafana datasources with cross-signal correlation + RED dashboard"
```

---

### Task 14: docker-compose wiring

**Files:**
- Create: `08_observability/docker-compose.yml`

- [ ] **Step 1: Write `docker-compose.yml`**

```yaml
services:
  app:
    build: { context: ., dockerfile: app/Dockerfile }
    environment:
      OTEL_EXPORTER_OTLP_ENDPOINT: otel-collector:4317
      FLAKE_RATE: "${FLAKE_RATE:-0.0}"
    ports: ["9100:9100"]
    depends_on: [otel-collector]

  frontend:
    build: { context: ./frontend, dockerfile: Dockerfile.dev }
    environment:
      VITE_API_BASE: http://localhost:9100
      VITE_OTEL_COLLECTOR_URL: http://localhost:4320
    ports: ["5174:5173"]
    volumes:
      - ./frontend:/app
      - /app/node_modules
    depends_on: [app]

  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.110.0
    command: ["--config=/etc/otel/config.yaml"]
    volumes:
      - ./infra/otel-collector/config.yaml:/etc/otel/config.yaml:ro
    ports: ["4319:4317", "4320:4318", "8889:8889"]
    depends_on: [tempo, loki]

  tempo:
    image: grafana/tempo:2.6.0
    command: ["-config.file=/etc/tempo.yaml"]
    volumes:
      - ./infra/tempo/tempo.yaml:/etc/tempo.yaml:ro
    ports: ["3200:3200"]

  loki:
    image: grafana/loki:3.2.0
    command: ["-config.file=/etc/loki/loki.yaml"]
    volumes:
      - ./infra/loki/loki.yaml:/etc/loki/loki.yaml:ro
    ports: ["3100:3100"]

  mimir:
    image: grafana/mimir:2.13.0
    command: ["-config.file=/etc/mimir.yaml", "-target=all"]
    volumes:
      - ./infra/mimir/mimir.yaml:/etc/mimir.yaml:ro
    ports: ["9009:9009"]

  prometheus:
    image: prom/prometheus:v2.54.1
    command:
      - "--config.file=/etc/prometheus/prometheus.yml"
      - "--enable-feature=exemplar-storage"
    volumes:
      - ./infra/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
    ports: ["9090:9090"]
    depends_on: [mimir, otel-collector]

  grafana:
    image: grafana/grafana:11.2.0
    environment:
      GF_AUTH_ANONYMOUS_ENABLED: "true"
      GF_AUTH_ANONYMOUS_ORG_ROLE: Admin
      GF_AUTH_DISABLE_LOGIN_FORM: "true"
    volumes:
      - ./infra/grafana/provisioning:/etc/grafana/provisioning:ro
    ports: ["3001:3000"]
    depends_on: [mimir, tempo, loki]
```

(Pin image tags to versions that exist; the tags above are reasonable but verify with `docker pull` in Task 16 and bump if a tag is missing.)

- [ ] **Step 2: Validate compose syntax**

Run: `cd 08_observability && docker compose config >/dev/null && cd ..`
Expected: no error (valid YAML + schema).

- [ ] **Step 3: Commit**

```bash
git add 08_observability/docker-compose.yml
git commit -m "feat(08_observability): docker-compose for full LGTM stack"
```

---

### Task 15: Makefile

**Files:**
- Create: `08_observability/Makefile`

- [ ] **Step 1: Write `Makefile`** (follow `07_network/Makefile` style)

```makefile
.PHONY: up down logs test demo load

up:    ; docker compose up -d --build
down:  ; docker compose down -v
logs:  ; docker compose logs -f
test:  ; cd app && go test ./...

# Single request (will 200 unless FLAKE_RATE is high)
demo:
	curl -s -X POST http://localhost:9100/api/checkout \
	  -H 'Content-Type: application/json' \
	  -d '{"item":"book","qty":1}' ; echo

# Generate load so metrics/traces accumulate (50 requests)
load:
	@for i in $$(seq 1 50); do \
	  curl -s -o /dev/null -X POST http://localhost:9100/api/checkout \
	    -H 'Content-Type: application/json' -d '{"item":"book","qty":1}' ; \
	done ; echo "sent 50 requests"
```

- [ ] **Step 2: Verify a target runs**

Run: `cd 08_observability && make test && cd ..`
Expected: Go tests PASS.

- [ ] **Step 3: Commit**

```bash
git add 08_observability/Makefile
git commit -m "feat(08_observability): Makefile (up/down/logs/test/demo/load)"
```

---

## Phase 4 — End-to-end verification

### Task 16: Bring up the stack and verify all acceptance criteria

**Files:** none (verification only; fix configs from earlier tasks as needed)

- [ ] **Step 1: Pull images / bring up**

Run: `cd 08_observability && make up`
Expected: all containers start. If any image tag is missing, bump it and re-run.

- [ ] **Step 2: Wait for health, then check the API**

Run: `sleep 20 && curl -s localhost:9100/healthz -o /dev/null -w "%{http_code}\n" && make demo`
Expected: `200`, then a JSON body containing `order_id`.

- [ ] **Step 3: Generate load**

Run: `make load`
Expected: `sent 50 requests`.

- [ ] **Step 4: Verify metrics reached Mimir (criterion 3)**

Run: `curl -s -H 'X-Scope-OrgID: anonymous' 'http://localhost:9009/prometheus/api/v1/query?query=http_server_requests_total' | head -c 400; echo`
Expected: a JSON result with non-empty `data.result`. If the metric name differs, note the real name (check `curl -s localhost:8889/metrics | grep http_server`) and update `red.json` (Task 13 Step 3).

- [ ] **Step 5: Verify Prometheus is scraping + remote_writing**

Open `http://localhost:9090/targets` → `otel-collector` target is UP. This confirms scrape; Step 4 confirms remote_write reached Mimir.

- [ ] **Step 6: Verify end-to-end trace (criterion 2)**

In a browser, open `http://localhost:5174`, click "Checkout を実行". Then open Grafana `http://localhost:3001` → Explore → Tempo → Search. Open a recent trace.
Expected: one trace contains a `checkout-frontend` fetch span (parent) → `checkout-api` HTTP server span → `checkout.process` span. If the frontend span is missing, check the browser console for OTLP export errors (usually CORS — confirm collector `allowed_origins` includes `http://localhost:5174`).

- [ ] **Step 7: Verify logs + trace_id correlation (criterion 4)**

Grafana → Explore → Loki → query `{service_name="checkout-api"} | json`. Confirm log lines include `trace_id`. Click a `trace_id`-derived link to jump to Tempo.
Expected: logs visible; derived field jumps to the matching trace.

- [ ] **Step 8: Verify exemplars metric→trace (criterion 5)**

Grafana → Explore → Mimir → query `http_server_duration_milliseconds_bucket`, enable Exemplars. Click an exemplar dot → jumps to Tempo trace.
Expected: exemplar links resolve. (If absent, this is acceptable to defer — note it in README "既知の制約". Exemplars require the histogram to carry trace context; document the limitation rather than blocking.)

- [ ] **Step 9: Verify flake drives error signals (criterion 6)**

Run: `make down && FLAKE_RATE=0.8 make up && sleep 20 && make load`
Then Grafana RED dashboard: error rate and error traces/logs rise.
Expected: visible increase in `http_server_errors_total` and 502 traces.

- [ ] **Step 10: Tear down**

Run: `make down`

- [ ] **Step 11: Commit any config fixes discovered during verification**

```bash
git add 08_observability
git commit -m "fix(08_observability): align configs after end-to-end verification"
```

---

## Phase 5 — Documentation

Each doc follows the existing chapter style (Japanese, である調, tables + command examples, closing「まとめ / 関連 doc」). Each task = write the doc + commit. Required content is enumerated; write full prose from these outlines (no placeholders in the shipped docs).

### Task 17: `docs/01_concepts.md`

- [ ] Write covering: 監視(monitoring)と観測性(observability)の違い / 「未知の未知」を問える性質 / 3本柱(Metrics・Traces・Logs)の定義と使い分け表 / signal ごとのデータモデル(時系列 vs span ツリー vs イベント) / OpenTelemetry の立ち位置(ベンダ中立の計装+OTLP) / 本章スタック全体図(README のアーキ図を再掲). 関連 doc: 02,03,04,05.
- [ ] Commit: `docs(08_observability): 01_concepts`

### Task 18: `docs/02_otel_sdk_go.md`

- [ ] Write covering: OTel Go の Provider 三種(Tracer/Meter/Logger) / `resource`+`semconv.ServiceName` / OTLP gRPC exporter / `propagation.TraceContext{}` の役割 / `otelhttp.NewHandler` 自動計装 / `otelslog` ブリッジで log に trace_id を載せる仕組み / `app/internal/obs/otel.go` と `metrics.go` の実コード抜粋と解説 / 06 との差分(traces only → 3本柱). 関連 doc: 03,04,05,07.
- [ ] Commit: `docs(08_observability): 02_otel_sdk_go`

### Task 19: `docs/03_traces_e2e.md`

- [ ] Write covering: span/trace/context の定義 / 親子関係と trace_id 伝播 / W3C `traceparent` ヘッダの形式 / ブラウザ(FetchInstrumentation)→ `traceparent` → Go(otelhttp が継続)→内部 span の流れ図 / `propagateTraceHeaderCorsUrls` がなぜ必要か(CORS) / Tempo で1本のトレースを読む手順(Grafana スクショ説明) / よくある失敗(CORS, 伝播漏れ). 関連 doc: 02,06.
- [ ] Commit: `docs(08_observability): 03_traces_e2e`

### Task 20: `docs/04_metrics_prom_mimir.md`

- [ ] Write covering: RED メソッド(Rate/Errors/Duration)と USE との対比 / Counter vs Histogram / OTel metrics → Collector → Prometheus(scrape) → remote_write → Mimir の経路図 / なぜ Prometheus と Mimir を併用するか(短期 scrape役 と 長期/スケール/HA の役割分担) / `remote_write` の意味 / Thanos との比較を1段落 / PromQL 例(rate, histogram_quantile) / exemplars で trace へ飛ぶ. 関連 doc: 06,07,08.
- [ ] Commit: `docs(08_observability): 04_metrics_prom_mimir`

### Task 21: `docs/05_logs_loki.md`

- [ ] Write covering: 構造化ログとは / なぜ trace_id を埋めるか(三本柱の接着剤) / `slog`+`otelslog` の仕組み / Collector → Loki(OTLP ingest) / Loki のラベル設計とカーディナリティ注意 / LogQL 例(`{service_name="checkout-api"} | json | trace_id="..."`) / logs⇄traces 往復. 関連 doc: 02,06.
- [ ] Commit: `docs(08_observability): 05_logs_loki`

### Task 22: `docs/06_grafana_correlation.md`

- [ ] Write covering: Grafana データソース3種の provisioning / 相関設定の解説(`exemplarTraceIdDestinations`, `tracesToLogsV2`, Loki `derivedFields`) / Explore での3本柱往復ワークフロー(metric→trace→log) / RED ダッシュボードの読み方 / ダッシュボードを provisioning で配る方法. 関連 doc: 04,05.
- [ ] Commit: `docs(08_observability): 06_grafana_correlation`

### Task 23: `docs/07_collector.md`

- [ ] Write covering: Collector の役割(計装と保存先の分離) / receivers/processors/exporters/pipelines の構造 / 本章 `config.yaml` の各セクション解説 / なぜ集約ハブにするか(バックエンド差し替え可能性) / agent vs gateway デプロイ / batch processor の意味. 関連 doc: 02,08.
- [ ] Commit: `docs(08_observability): 07_collector`

### Task 24: `docs/08_oss_landscape.md` + finalize `README.md`

- [ ] Write `08_oss_landscape.md` covering, each with「いつ選ぶか」一言: 採用した Grafana LGTM の総括 / Grafana Alloy(新世代収集エージェント, Collector 互換) / SigNoz・OpenObserve・Uptrace(オールインワン統合観測, ClickHouse/Rust 系) / VictoriaMetrics(Prometheus/Mimir 代替の軽量 TSDB) / Grafana Pyroscope(継続プロファイリング=第4の柱) / Grafana Beyla・OpenTelemetry eBPF(コード変更なし自動計装). 比較表(導入容易さ/UI 統合/スケール/OTel ネイティブ度)で締める.
- [ ] Rewrite `README.md` with: 章の狙い / 学習動線(docs 01–08 へのリンク) / クイックスタート(`make up` → ブラウザ → Grafana) / アクセス先一覧表(app/frontend/grafana/prometheus URL) / アーキ図 / 既知の制約(あれば exemplars 等) / 環境注意(ポート, Docker リソース). Style に合わせて `07_network/README.md` を参照.
- [ ] Commit: `docs(08_observability): OSS landscape + finalize README`

---

## Self-Review Notes (completed by plan author)

- **Spec coverage:** 題材=最小Go API(Task 2-5) / フロント React+Vite+OTel(Task 6-7) / Collector(8) / Tempo(9) / Prometheus→Mimir(10-11) / Loki(12) / Grafana相関(13) / compose(14) / Makefile(15) / e2e検証=受け入れ基準1-8(Task 16) / docs 8本+OSS landscape(Task 17-24). 全 spec 節に対応タスクあり。
- **Deferred-but-explicit items:** OTel JS/log-SDK の正確なバージョン(npm/go get が解決 → コミット時に固定), 一部メトリクス名のOTel→Prom変換(Task 16 で実値確認しdashboard修正), Mimir/イメージタグの version 差(Task 11/14/16 で確認). いずれも放置 placeholder ではなく検証手順付き。
- **Type consistency:** `NewService`/`NewHandler`/`InitTelemetry`/`NewREDMiddleware` はタスク間で一貫。`InitTelemetry` の戻り値(`*sdklog.LoggerProvider, shutdown, error`)を main(Task 5) が `otelslog.WithLoggerProvider(lp)` で利用、整合。
```
