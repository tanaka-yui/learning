# 06_microservie Plan 3: Frontend + BFF/user-auth Extensions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the React frontend for the 06_microservie sample app, plus the BFF and user-auth extensions required for product detail, auth probing, sign-out, and trace_id surfacing in the UI.

**Architecture:** Vite + React 18 + TypeScript + React Router v6 single-page app with a thin layered structure (`api/` fetch wrappers → `hooks/` state → `pages/`). All auth is HttpOnly Cookie based. BFF gains a `WriteError` helper that returns `{code,message,trace_id}` JSON, a `TraceID` middleware that sets `X-Trace-Id`, three new endpoints (`GET /api/products/:id`, `GET /api/auth/me`, `POST /api/auth/signout`), and user-auth gRPC adds `GetUser`.

**Tech Stack:** Vite 5, React 18, TypeScript 5, React Router v6, Vitest 1.x, jsdom, Go 1.22, chi, OpenTelemetry, pgx, buf.

**Reference spec:** `docs/superpowers/specs/2026-05-13-microservices-frontend-design.md`

---

## Phase 0: Branch context

This plan runs on branch `feat/microservices` (already 28 commits ahead of main). No worktree split — continue in place.

---

## Phase 1: user-auth GetUser RPC

### Task 1.1: Add `GetUser` to user.proto and regenerate

**Files:**
- Modify: `06_microservie/proto/user/v1/user.proto`
- Regenerate: `06_microservie/proto/gen/go/user/v1/user.pb.go`, `user_grpc.pb.go`

- [ ] **Step 1: Add GetUser RPC and messages to the proto file**

Replace the contents of `06_microservie/proto/user/v1/user.proto` with:

```proto
syntax = "proto3";

package user.v1;

service UserService {
  rpc SignUp(SignUpRequest) returns (SignUpResponse);
  rpc SignIn(SignInRequest) returns (SignInResponse);
  rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
}

message SignUpRequest {
  string email = 1;
  string password = 2;
}

message SignUpResponse {
  string user_id = 1;
}

message SignInRequest {
  string email = 1;
  string password = 2;
}

message SignInResponse {
  string token = 1;
}

message ValidateTokenRequest {
  string token = 1;
}

message ValidateTokenResponse {
  string user_id = 1;
}

message GetUserRequest {
  string user_id = 1;
}

message GetUserResponse {
  string user_id = 1;
  string email = 2;
}
```

- [ ] **Step 2: Regenerate Go code**

Run: `cd 06_microservie && make proto`
Expected: Files in `proto/gen/go/user/v1/` updated, no errors.

- [ ] **Step 3: Confirm the new symbols exist**

Run: `grep -E 'GetUser|GetUserRequest|GetUserResponse' 06_microservie/proto/gen/go/user/v1/*.go | head -20`
Expected: matches for `GetUser`, `GetUserRequest`, `GetUserResponse`.

### Task 1.2: Add `FindByID` to user-auth repo (TDD)

**Files:**
- Modify: `06_microservie/services/user-auth/internal/repo/users.go`
- Modify: `06_microservie/services/user-auth/internal/repo/users_test.go`

- [ ] **Step 1: Add failing test**

In `06_microservie/services/user-auth/internal/repo/users_test.go`, add inside the existing test function (find the last `t.Run(...)` and add after it). If the file uses a single top-level test, append a new top-level test instead. Add:

```go
func TestRepoFindByID(t *testing.T) {
    ctx := context.Background()
    pool := newTestPool(t) // reuse existing helper from this test file
    r := New(pool)

    id, err := r.Create(ctx, "find-by-id@example.com", "hash")
    if err != nil {
        t.Fatalf("Create: %v", err)
    }

    u, err := r.FindByID(ctx, id)
    if err != nil {
        t.Fatalf("FindByID: %v", err)
    }
    if u.Email != "find-by-id@example.com" {
        t.Errorf("email = %q, want find-by-id@example.com", u.Email)
    }

    _, err = r.FindByID(ctx, "00000000-0000-0000-0000-000000000000")
    if !errors.Is(err, ErrUserNotFound) {
        t.Errorf("expected ErrUserNotFound, got %v", err)
    }
}
```

If `newTestPool` is named differently in the existing file, use the existing helper name. Run `grep -n 'func.*test.*Pool\|testcontainers' 06_microservie/services/user-auth/internal/repo/users_test.go` first to confirm.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 06_microservie/services/user-auth && DOCKER_HOST=unix://$HOME/.rd/docker.sock TESTCONTAINERS_RYUK_DISABLED=true go test ./internal/repo -run TestRepoFindByID -v`
Expected: FAIL with `r.FindByID undefined`.

- [ ] **Step 3: Implement FindByID**

Add to `06_microservie/services/user-auth/internal/repo/users.go` after `FindByEmail`:

```go
func (r *Repo) FindByID(ctx context.Context, id string) (User, error) {
    var u User
    err := r.pool.QueryRow(ctx, `SELECT id, email, password_hash FROM users WHERE id=$1`, id).
        Scan(&u.ID, &u.Email, &u.PasswordHash)
    if errors.Is(err, pgx.ErrNoRows) {
        return User{}, ErrUserNotFound
    }
    return u, err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd 06_microservie/services/user-auth && DOCKER_HOST=unix://$HOME/.rd/docker.sock TESTCONTAINERS_RYUK_DISABLED=true go test ./internal/repo -run TestRepoFindByID -v`
Expected: PASS.

### Task 1.3: Add `GetUser` handler to gRPC server (TDD)

**Files:**
- Modify: `06_microservie/services/user-auth/internal/server/grpc.go`
- Modify: `06_microservie/services/user-auth/internal/server/grpc_test.go`

- [ ] **Step 1: Add failing test**

Append to `06_microservie/services/user-auth/internal/server/grpc_test.go`:

```go
func TestServerGetUser(t *testing.T) {
    fake := &fakeRepo{users: map[string]repo.User{
        "u-1": {ID: "u-1", Email: "found@example.com"},
    }}
    j, _ := jwt.NewManager("test-secret-32-bytes-long-padding-pad")
    s := New(fake, j)

    resp, err := s.GetUser(context.Background(), &userv1.GetUserRequest{UserId: "u-1"})
    if err != nil {
        t.Fatalf("GetUser: %v", err)
    }
    if resp.Email != "found@example.com" {
        t.Errorf("email = %q", resp.Email)
    }

    _, err = s.GetUser(context.Background(), &userv1.GetUserRequest{UserId: "u-missing"})
    if status.Code(err) != codes.NotFound {
        t.Errorf("expected NotFound, got %v", err)
    }
}
```

If `fakeRepo` doesn't yet implement `FindByID`, add the method to its struct (look for the existing `fakeRepo` definition in the same file). Append to the `fakeRepo` definition:

```go
func (f *fakeRepo) FindByID(_ context.Context, id string) (repo.User, error) {
    if f.users == nil {
        return repo.User{}, repo.ErrUserNotFound
    }
    u, ok := f.users[id]
    if !ok {
        return repo.User{}, repo.ErrUserNotFound
    }
    return u, nil
}
```

If `fakeRepo` doesn't have a `users` map, modify its struct to add it: change `type fakeRepo struct { ... }` to include `users map[string]repo.User`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 06_microservie/services/user-auth && go test ./internal/server -run TestServerGetUser -v`
Expected: FAIL with `s.GetUser undefined` or compile error.

- [ ] **Step 3: Extend UserRepo interface and implement GetUser**

In `06_microservie/services/user-auth/internal/server/grpc.go`:

Replace the `UserRepo` interface with:
```go
type UserRepo interface {
    Create(ctx context.Context, email, hash string) (string, error)
    FindByEmail(ctx context.Context, email string) (repo.User, error)
    FindByID(ctx context.Context, id string) (repo.User, error)
}
```

Add after the `ValidateToken` method:
```go
func (s *Server) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
    u, err := s.r.FindByID(ctx, req.UserId)
    if errors.Is(err, repo.ErrUserNotFound) {
        return nil, status.Error(codes.NotFound, "user not found")
    }
    if err != nil {
        return nil, status.Error(codes.Internal, err.Error())
    }
    return &userv1.GetUserResponse{UserId: u.ID, Email: u.Email}, nil
}
```

- [ ] **Step 4: Run all user-auth tests**

Run: `cd 06_microservie/services/user-auth && DOCKER_HOST=unix://$HOME/.rd/docker.sock TESTCONTAINERS_RYUK_DISABLED=true go test ./...`
Expected: PASS.

### Task 1.4: Wire `GetUser` into user-auth main.go server registration

**Files:**
- Modify: `06_microservie/services/user-auth/main.go` (only if registration is incomplete — likely no change needed since server registration is via the generated `RegisterUserServiceServer`).

- [ ] **Step 1: Inspect current registration**

Run: `grep -n 'RegisterUserServiceServer\|Server{' 06_microservie/services/user-auth/main.go`
Expected: a call like `userv1.RegisterUserServiceServer(grpcSrv, server.New(...))`. Confirm this already covers GetUser (it does, because we added GetUser to the Server type — the registration is by interface).

- [ ] **Step 2: No code change required if registration is interface-based**

If for any reason the file embeds `UnimplementedUserServiceServer`, ensure the new method is on `*Server`. No edit otherwise.

### Task 1.5: Add `GetUser` method to BFF user-auth client

**Files:**
- Modify: `06_microservie/bff/internal/client/user_auth.go`

- [ ] **Step 1: Add method**

Append to `06_microservie/bff/internal/client/user_auth.go`:

```go
type UserProfile struct {
    UserID string
    Email  string
}

func (u *UserAuth) GetUser(ctx context.Context, userID string) (*UserProfile, error) {
    r, err := u.c.GetUser(ctx, &userv1.GetUserRequest{UserId: userID})
    if err != nil {
        return nil, err
    }
    return &UserProfile{UserID: r.UserId, Email: r.Email}, nil
}
```

- [ ] **Step 2: Build BFF**

Run: `cd 06_microservie/bff && go build ./...`
Expected: no errors.

### Task 1.6: Commit Phase 1

- [ ] **Step 1: Commit**

```bash
git add 06_microservie/proto/user/v1/user.proto \
        06_microservie/proto/gen/go/user/v1/ \
        06_microservie/services/user-auth/ \
        06_microservie/bff/internal/client/user_auth.go
git commit -m "microservices(plan3): add user-auth GetUser RPC and BFF client method"
```

---

## Phase 2: BFF shared infrastructure (error JSON + X-Trace-Id)

### Task 2.1: Create `httpx.WriteError` helper (TDD)

**Files:**
- Create: `06_microservie/bff/internal/httpx/error.go`
- Create: `06_microservie/bff/internal/httpx/error_test.go`

- [ ] **Step 1: Write the failing test**

Create `06_microservie/bff/internal/httpx/error_test.go`:

```go
package httpx

import (
    "encoding/json"
    "net/http/httptest"
    "testing"
)

func TestWriteError_Format(t *testing.T) {
    w := httptest.NewRecorder()
    r := httptest.NewRequest("GET", "/whatever", nil)
    WriteError(w, r, 400, "INVALID_INPUT", "items required")

    if got := w.Result().StatusCode; got != 400 {
        t.Errorf("status = %d", got)
    }
    if ct := w.Header().Get("Content-Type"); ct != "application/json" {
        t.Errorf("Content-Type = %q", ct)
    }

    var body struct {
        Code    string `json:"code"`
        Message string `json:"message"`
        TraceID string `json:"trace_id"`
    }
    if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
        t.Fatalf("decode: %v", err)
    }
    if body.Code != "INVALID_INPUT" || body.Message != "items required" {
        t.Errorf("body = %+v", body)
    }
    // trace_id is empty here because no span in context; should be present (empty string) in JSON.
    if _, ok := lookupKey(w.Body.Bytes(), "trace_id"); !ok {
        t.Errorf("trace_id key missing")
    }
}

func lookupKey(b []byte, key string) (string, bool) {
    m := map[string]any{}
    if err := json.Unmarshal(b, &m); err != nil {
        return "", false
    }
    v, ok := m[key]
    if !ok {
        return "", false
    }
    s, _ := v.(string)
    return s, true
}
```

(The body has already been read by the decoder above, so the second `w.Body.Bytes()` won't see anything. Adjust:)

Replace the test with:

```go
package httpx

import (
    "encoding/json"
    "net/http/httptest"
    "testing"
)

func TestWriteError_Format(t *testing.T) {
    w := httptest.NewRecorder()
    r := httptest.NewRequest("GET", "/whatever", nil)
    WriteError(w, r, 400, "INVALID_INPUT", "items required")

    if got := w.Result().StatusCode; got != 400 {
        t.Errorf("status = %d", got)
    }
    if ct := w.Header().Get("Content-Type"); ct != "application/json" {
        t.Errorf("Content-Type = %q", ct)
    }

    var body map[string]any
    if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
        t.Fatalf("decode: %v", err)
    }
    if body["code"] != "INVALID_INPUT" {
        t.Errorf("code = %v", body["code"])
    }
    if body["message"] != "items required" {
        t.Errorf("message = %v", body["message"])
    }
    if _, ok := body["trace_id"]; !ok {
        t.Errorf("trace_id key missing")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 06_microservie/bff && go test ./internal/httpx/...`
Expected: FAIL with `package microservie/bff/internal/httpx: no Go files`.

- [ ] **Step 3: Write minimal implementation**

Create `06_microservie/bff/internal/httpx/error.go`:

```go
package httpx

import (
    "encoding/json"
    "net/http"

    "go.opentelemetry.io/otel/trace"
)

type ErrorBody struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    TraceID string `json:"trace_id"`
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
    traceID := ""
    if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
        traceID = sc.TraceID().String()
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(ErrorBody{Code: code, Message: message, TraceID: traceID})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd 06_microservie/bff && go test ./internal/httpx/...`
Expected: PASS.

### Task 2.2: Create `TraceID` middleware (TDD)

**Files:**
- Create: `06_microservie/bff/internal/middleware/traceid.go`
- Create: `06_microservie/bff/internal/middleware/traceid_test.go`

- [ ] **Step 1: Write the failing test**

Create `06_microservie/bff/internal/middleware/traceid_test.go`:

```go
package middleware

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "go.opentelemetry.io/otel/trace"
)

func TestTraceID_SetsHeader(t *testing.T) {
    var captured string
    h := TraceID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusNoContent)
        captured = w.Header().Get("X-Trace-Id")
    }))

    // Make a valid SpanContext
    tid, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
    sid, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
    sc := trace.NewSpanContext(trace.SpanContextConfig{
        TraceID:    tid,
        SpanID:     sid,
        TraceFlags: trace.FlagsSampled,
        Remote:     true,
    })
    ctx := trace.ContextWithSpanContext(t.Context(), sc)

    req := httptest.NewRequest("GET", "/whatever", nil).WithContext(ctx)
    rec := httptest.NewRecorder()
    h.ServeHTTP(rec, req)

    if got := rec.Header().Get("X-Trace-Id"); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
        t.Errorf("X-Trace-Id = %q", got)
    }
    _ = captured
}

func TestTraceID_NoSpanLeavesHeaderEmpty(t *testing.T) {
    h := TraceID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusNoContent)
    }))
    req := httptest.NewRequest("GET", "/whatever", nil)
    rec := httptest.NewRecorder()
    h.ServeHTTP(rec, req)
    if got := rec.Header().Get("X-Trace-Id"); got != "" {
        t.Errorf("X-Trace-Id should be empty, got %q", got)
    }
}
```

If `t.Context()` isn't available in the Go version used (added in 1.24), substitute `context.Background()` (import `"context"`).

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 06_microservie/bff && go test ./internal/middleware -run TestTraceID -v`
Expected: FAIL with `TraceID undefined`.

- [ ] **Step 3: Write minimal implementation**

Create `06_microservie/bff/internal/middleware/traceid.go`:

```go
package middleware

import (
    "net/http"

    "go.opentelemetry.io/otel/trace"
)

func TraceID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
            w.Header().Set("X-Trace-Id", sc.TraceID().String())
        }
        next.ServeHTTP(w, r)
    })
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd 06_microservie/bff && go test ./internal/middleware -run TestTraceID -v`
Expected: PASS.

### Task 2.3: Wire `TraceID` middleware into main.go

**Files:**
- Modify: `06_microservie/bff/main.go`

- [ ] **Step 1: Update Handler to wrap with TraceID inside otelhttp**

In `06_microservie/bff/main.go`, find the existing srv definition:
```go
srv := &http.Server{
    Addr:              ":" + port,
    Handler:           otelhttp.NewHandler(r, "bff"),
    ReadHeaderTimeout: 5 * time.Second,
}
```

Replace `Handler: otelhttp.NewHandler(r, "bff")` with:
```go
Handler:           otelhttp.NewHandler(bffmiddleware.TraceID(r), "bff"),
```

The middleware import already exists as `bffmiddleware "microservie/bff/internal/middleware"`.

- [ ] **Step 2: Build and run BFF unit tests**

Run: `cd 06_microservie/bff && go build ./... && go test ./...`
Expected: PASS.

### Task 2.4: Refactor existing BFF handlers to use `WriteError`

**Files:**
- Modify: `06_microservie/bff/internal/handler/products.go`
- Modify: `06_microservie/bff/internal/handler/auth.go`
- Modify: `06_microservie/bff/internal/handler/checkout.go`
- Modify: `06_microservie/bff/internal/handler/orders.go`
- Modify: `06_microservie/bff/internal/middleware/auth.go`
- Modify: corresponding `*_test.go` files

- [ ] **Step 1: Replace all `http.Error(...)` calls with `httpx.WriteError(...)`**

For each file above, replace every occurrence of:
```go
http.Error(w, msg, status)
```
with:
```go
httpx.WriteError(w, r, status, "<CODE>", msg)
```

Use this mapping for `<CODE>`:
- `http.StatusBadRequest` (400) → `"INVALID_INPUT"`
- `http.StatusUnauthorized` (401) → `"UNAUTHORIZED"`
- `http.StatusNotFound` (404) → `"NOT_FOUND"`
- `http.StatusBadGateway` (502) → `"UPSTREAM_FAILED"`
- `http.StatusInternalServerError` (500) → `"INTERNAL"`

Add `"microservie/bff/internal/httpx"` to each modified file's import block.

Example: `products.go` line 36-38 currently:
```go
if err != nil {
    http.Error(w, err.Error(), http.StatusBadGateway)
    return
}
```
becomes:
```go
if err != nil {
    httpx.WriteError(w, r, http.StatusBadGateway, "UPSTREAM_FAILED", err.Error())
    return
}
```

Apply the same substitution to all handler files.

For `middleware/auth.go`, change the two `http.Error(w, "unauthorized", http.StatusUnauthorized)` to `httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")`.

- [ ] **Step 2: Update existing tests that assert plain-text body**

The existing tests (`products_test.go`, `auth_test.go` if checking body, `orders_test.go`, `checkout_test.go`, `middleware/auth_test.go`) may assert on `w.Body.String()`. Where any test checks the response body for an error path, change the assertion to parse JSON and check `body["code"]`.

Open each `_test.go` file and search for `Body.String()` assertions in error paths. Replace with:
```go
var body map[string]any
_ = json.Unmarshal(w.Body.Bytes(), &body)
if body["code"] != "<EXPECTED_CODE>" {
    t.Errorf("code = %v", body["code"])
}
```
Add `"encoding/json"` to imports where needed.

- [ ] **Step 3: Run BFF tests**

Run: `cd 06_microservie/bff && go test ./...`
Expected: PASS.

### Task 2.5: Commit Phase 2

- [ ] **Step 1: Commit**

```bash
git add 06_microservie/bff/
git commit -m "microservices(bff): add WriteError JSON helper and X-Trace-Id middleware, refactor handlers"
```

---

## Phase 3: BFF new endpoints

### Task 3.1: Add `GetProduct` handler (TDD)

**Files:**
- Modify: `06_microservie/bff/internal/handler/products.go`
- Modify: `06_microservie/bff/internal/handler/products_test.go`

- [ ] **Step 1: Add failing test**

Append to `06_microservie/bff/internal/handler/products_test.go`:

```go
func TestProductsGet_OK(t *testing.T) {
    fake := &fakeCatalog{products: []*catalogv1.Product{
        {Id: "p-001", Name: "Tea", PriceCents: 500, Description: "Loose leaf"},
    }}
    h := NewProducts(fake)

    r := chi.NewRouter()
    r.Get("/api/products/{id}", h.Get)

    req := httptest.NewRequest("GET", "/api/products/p-001", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if w.Code != 200 {
        t.Fatalf("status = %d", w.Code)
    }
    var body struct {
        ID         string `json:"id"`
        Name       string `json:"name"`
        PriceCents int32  `json:"price_cents"`
    }
    if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
        t.Fatalf("decode: %v", err)
    }
    if body.ID != "p-001" || body.Name != "Tea" || body.PriceCents != 500 {
        t.Errorf("body = %+v", body)
    }
}

func TestProductsGet_NotFound(t *testing.T) {
    fake := &fakeCatalog{products: nil}
    h := NewProducts(fake)

    r := chi.NewRouter()
    r.Get("/api/products/{id}", h.Get)

    req := httptest.NewRequest("GET", "/api/products/missing", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if w.Code != 404 {
        t.Errorf("status = %d", w.Code)
    }
    var body map[string]any
    _ = json.Unmarshal(w.Body.Bytes(), &body)
    if body["code"] != "NOT_FOUND" {
        t.Errorf("code = %v", body["code"])
    }
}
```

Ensure `fakeCatalog` (in the same test file) has a `GetProduct(ctx, id)` method. If absent, add:

```go
func (f *fakeCatalog) GetProduct(_ context.Context, id string) (*catalogv1.Product, error) {
    for _, p := range f.products {
        if p.Id == id {
            return p, nil
        }
    }
    return nil, fmt.Errorf("not found")
}
```

Add imports as needed: `"github.com/go-chi/chi/v5"`, `"net/http/httptest"`, `"fmt"`, `"context"`, `"encoding/json"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 06_microservie/bff && go test ./internal/handler -run TestProductsGet -v`
Expected: FAIL with `h.Get undefined` or interface mismatch.

- [ ] **Step 3: Extend Products handler and CatalogClient interface**

In `06_microservie/bff/internal/handler/products.go`:

Extend the interface:
```go
type CatalogClient interface {
    ListProducts(ctx context.Context) ([]*catalogv1.Product, error)
    GetProduct(ctx context.Context, id string) (*catalogv1.Product, error)
}
```

Add the `Get` handler after `List`:
```go
func (p *Products) Get(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_INPUT", "id required")
        return
    }
    prod, err := p.c.GetProduct(r.Context(), id)
    if err != nil {
        httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error())
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(productDTO{
        ID: prod.Id, Name: prod.Name, Description: prod.Description, PriceCents: prod.PriceCents,
    })
}
```

Imports to add at the top of `products.go`:
```go
"github.com/go-chi/chi/v5"
"microservie/bff/internal/httpx"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd 06_microservie/bff && go test ./internal/handler -run TestProductsGet -v`
Expected: PASS.

### Task 3.2: Add `Me` handler for `GET /api/auth/me` (TDD)

**Files:**
- Modify: `06_microservie/bff/internal/handler/auth.go`
- Modify: `06_microservie/bff/internal/handler/auth_test.go`

- [ ] **Step 1: Add failing test**

Append to `06_microservie/bff/internal/handler/auth_test.go` (and add `"microservie/bff/internal/middleware"` import if missing):

```go
type fakeProfile struct {
    profiles map[string]struct{ id, email string }
    signUp   func(ctx context.Context, email, password string) (string, error)
    signIn   func(ctx context.Context, email, password string) (string, error)
}

func (f *fakeProfile) SignUp(ctx context.Context, email, password string) (string, error) {
    return f.signUp(ctx, email, password)
}
func (f *fakeProfile) SignIn(ctx context.Context, email, password string) (string, error) {
    return f.signIn(ctx, email, password)
}
func (f *fakeProfile) GetUser(_ context.Context, id string) (string, error) {
    if p, ok := f.profiles[id]; ok {
        return p.email, nil
    }
    return "", fmt.Errorf("not found")
}

func TestAuthMe_OK(t *testing.T) {
    fake := &fakeProfile{profiles: map[string]struct{ id, email string }{
        "u-1": {"u-1", "alice@example.com"},
    }}
    h := NewAuth(fake)

    req := httptest.NewRequest("GET", "/api/auth/me", nil)
    ctx := middleware.SetUserID(req.Context(), "u-1")
    req = req.WithContext(ctx)
    w := httptest.NewRecorder()

    h.Me(w, req)
    if w.Code != 200 {
        t.Fatalf("status = %d", w.Code)
    }
    var body struct {
        UserID string `json:"user_id"`
        Email  string `json:"email"`
    }
    if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
        t.Fatalf("decode: %v", err)
    }
    if body.UserID != "u-1" || body.Email != "alice@example.com" {
        t.Errorf("body = %+v", body)
    }
}
```

If `fakeAuthClient` already exists in the file and is used by signin tests, replace your test's `fake` with a hybrid that has both signup/signin and GetUser. Reuse the existing fake if it has GetUser, otherwise define a new one with a different name (e.g. `fakeAuthFull`) and pass it to `NewAuth` instead.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 06_microservie/bff && go test ./internal/handler -run TestAuthMe -v`
Expected: FAIL with `h.Me undefined` or interface mismatch.

- [ ] **Step 3: Extend AuthClient interface and add Me handler**

In `06_microservie/bff/internal/handler/auth.go`, change `AuthClient` to:
```go
type AuthClient interface {
    SignUp(ctx context.Context, email, password string) (string, error)
    SignIn(ctx context.Context, email, password string) (string, error)
    GetUser(ctx context.Context, userID string) (string, error)
}
```

Add the `Me` method:
```go
func (a *Auth) Me(w http.ResponseWriter, r *http.Request) {
    uid := middleware.UserID(r.Context())
    if uid == "" {
        httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
        return
    }
    email, err := a.c.GetUser(r.Context(), uid)
    if err != nil {
        httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "user not found")
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]string{"user_id": uid, "email": email})
}
```

Add imports: `"microservie/bff/internal/middleware"`, `"microservie/bff/internal/httpx"`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd 06_microservie/bff && go test ./internal/handler -run TestAuthMe -v`
Expected: PASS.

### Task 3.3: Add `SignOut` handler (TDD)

**Files:**
- Modify: `06_microservie/bff/internal/handler/auth.go`
- Modify: `06_microservie/bff/internal/handler/auth_test.go`

- [ ] **Step 1: Add failing test**

Append to `06_microservie/bff/internal/handler/auth_test.go`:

```go
func TestAuthSignOut_ClearsCookie(t *testing.T) {
    h := NewAuth(&fakeProfile{}) // GetUser/SignUp/SignIn unused

    req := httptest.NewRequest("POST", "/api/auth/signout", nil)
    w := httptest.NewRecorder()
    h.SignOut(w, req)

    if w.Code != http.StatusNoContent {
        t.Errorf("status = %d", w.Code)
    }
    sc := w.Header().Get("Set-Cookie")
    if !strings.Contains(sc, "session=") || !strings.Contains(sc, "Max-Age=0") {
        t.Errorf("Set-Cookie = %q", sc)
    }
}
```

Add `"strings"` to test imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 06_microservie/bff && go test ./internal/handler -run TestAuthSignOut -v`
Expected: FAIL with `h.SignOut undefined`.

- [ ] **Step 3: Implement SignOut**

Add to `06_microservie/bff/internal/handler/auth.go`:

```go
func (a *Auth) SignOut(w http.ResponseWriter, r *http.Request) {
    http.SetCookie(w, &http.Cookie{
        Name:     "session",
        Value:    "",
        Path:     "/",
        HttpOnly: true,
        SameSite: http.SameSiteLaxMode,
        MaxAge:   0,
        Expires:  time.Unix(0, 0),
    })
    w.WriteHeader(http.StatusNoContent)
}
```

Add `"time"` import.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd 06_microservie/bff && go test ./internal/handler -run TestAuthSignOut -v`
Expected: PASS.

### Task 3.4: Wire new routes and main.go interface adapter

**Files:**
- Modify: `06_microservie/bff/main.go`

- [ ] **Step 1: Add an inline adapter type matching the new AuthClient interface**

The new `handler.AuthClient` interface expects `GetUser(ctx, id) (string, error)`, but `*client.UserAuth` has `GetUser(ctx, id) (*client.UserProfile, error)`. Add an inline adapter in `06_microservie/bff/main.go` (top-level, outside any function):

```go
type uaAdapter struct{ *client.UserAuth }

func (a uaAdapter) GetUser(ctx context.Context, userID string) (string, error) {
    p, err := a.UserAuth.GetUser(ctx, userID)
    if err != nil {
        return "", err
    }
    return p.Email, nil
}
```

The outer `GetUser` method shadows the embedded one, so `uaAdapter` satisfies `handler.AuthClient`.

Replace `authHandler := handler.NewAuth(ua)` with:
```go
authHandler := handler.NewAuth(uaAdapter{ua})
```

- [ ] **Step 2: Register the new routes**

In the same file, replace the existing routing block:
```go
r.Get("/api/products", products.List)
r.Post("/api/auth/signup", authHandler.SignUp)
r.Post("/api/auth/signin", authHandler.SignIn)
```
with:
```go
r.Get("/api/products", products.List)
r.Get("/api/products/{id}", products.Get)
r.Post("/api/auth/signup", authHandler.SignUp)
r.Post("/api/auth/signin", authHandler.SignIn)
r.Post("/api/auth/signout", authHandler.SignOut)
```

And inside the protected group, add the `/api/auth/me` route:
```go
p.Get("/api/auth/me", authHandler.Me)
```

- [ ] **Step 3: Build and run all BFF tests**

Run: `cd 06_microservie/bff && go build ./... && go test ./...`
Expected: PASS.

### Task 3.5: Commit Phase 3

- [ ] **Step 1: Commit**

```bash
git add 06_microservie/bff/
git commit -m "microservices(bff): add product detail, auth me, and signout endpoints"
```

---

## Phase 4: Frontend project scaffolding

### Task 4.1: Create the frontend package structure

**Files:**
- Create: `06_microservie/frontend/package.json`
- Create: `06_microservie/frontend/tsconfig.json`
- Create: `06_microservie/frontend/tsconfig.node.json`
- Create: `06_microservie/frontend/vite.config.ts`
- Create: `06_microservie/frontend/index.html`
- Create: `06_microservie/frontend/.gitignore`
- Create: `06_microservie/frontend/public/.gitkeep`

- [ ] **Step 1: Write package.json**

Create `06_microservie/frontend/package.json`:

```json
{
  "name": "microservie-frontend",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "test": "vitest"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-router-dom": "^6.26.2"
  },
  "devDependencies": {
    "@types/react": "^18.3.5",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.1",
    "jsdom": "^25.0.0",
    "typescript": "^5.5.4",
    "vite": "^5.4.5",
    "vitest": "^2.0.5"
  }
}
```

- [ ] **Step 2: Write tsconfig.json**

Create `06_microservie/frontend/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "types": ["vite/client", "vitest/globals"]
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 3: Write tsconfig.node.json**

Create `06_microservie/frontend/tsconfig.node.json`:

```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true,
    "strict": true
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **Step 4: Write vite.config.ts**

Create `06_microservie/frontend/vite.config.ts`:

```ts
/// <reference types="vitest" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: { port: 5173 },
  test: {
    globals: true,
    environment: 'jsdom',
  },
});
```

- [ ] **Step 5: Write index.html**

Create `06_microservie/frontend/index.html`:

```html
<!doctype html>
<html lang="ja">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Microservie Shop</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 6: Write .gitignore**

Create `06_microservie/frontend/.gitignore`:

```
node_modules
dist
.vite
*.local
```

- [ ] **Step 7: public/.gitkeep**

Create empty file `06_microservie/frontend/public/.gitkeep`.

- [ ] **Step 8: Install dependencies locally to validate package.json**

Run: `cd 06_microservie/frontend && npm install`
Expected: lockfile created at `package-lock.json`, no errors.

### Task 4.2: Write the main entry and global styles

**Files:**
- Create: `06_microservie/frontend/src/main.tsx`
- Create: `06_microservie/frontend/src/App.tsx`
- Create: `06_microservie/frontend/src/styles.css`

- [ ] **Step 1: Write main.tsx**

Create `06_microservie/frontend/src/main.tsx`:

```tsx
import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { AuthProvider } from './hooks/useAuth';
import App from './App';
import './styles.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <App />
      </AuthProvider>
    </BrowserRouter>
  </React.StrictMode>,
);
```

- [ ] **Step 2: Write a stub App.tsx (full version in Phase 7)**

Create `06_microservie/frontend/src/App.tsx`:

```tsx
import { Routes, Route } from 'react-router-dom';

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<div>Welcome (scaffold)</div>} />
    </Routes>
  );
}
```

- [ ] **Step 3: Write minimal styles.css**

Create `06_microservie/frontend/src/styles.css`:

```css
:root {
  font-family: system-ui, -apple-system, "Hiragino Kaku Gothic ProN", "Segoe UI", sans-serif;
  line-height: 1.5;
  color: #1a1a1a;
  background: #fafafa;
}

* { box-sizing: border-box; }

body {
  margin: 0;
  padding: 0;
}

.layout {
  max-width: 960px;
  margin: 0 auto;
  padding: 16px;
}

.nav {
  display: flex;
  gap: 16px;
  padding: 12px 16px;
  border-bottom: 1px solid #e5e5e5;
  background: white;
  align-items: center;
}

.nav a {
  color: #1a1a1a;
  text-decoration: none;
}
.nav a:hover { text-decoration: underline; }

.nav .spacer { flex: 1; }

.badge {
  display: inline-block;
  background: #1a1a1a;
  color: white;
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 12px;
  margin-left: 4px;
}

.product-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 16px;
}

.card {
  background: white;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
  padding: 16px;
}

.btn {
  background: #1a1a1a;
  color: white;
  border: none;
  border-radius: 4px;
  padding: 8px 16px;
  cursor: pointer;
}

.btn:disabled {
  background: #aaa;
  cursor: not-allowed;
}

.btn-secondary {
  background: white;
  color: #1a1a1a;
  border: 1px solid #1a1a1a;
}

.error-banner {
  background: #ffe9e9;
  border: 1px solid #d33;
  color: #900;
  padding: 12px 16px;
  border-radius: 8px;
  margin-bottom: 16px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.trace-chip {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  font-family: ui-monospace, "SF Mono", Menlo, monospace;
  font-size: 12px;
  color: #555;
}

.trace-chip button {
  background: transparent;
  border: 1px solid #ccc;
  border-radius: 4px;
  cursor: pointer;
  padding: 2px 4px;
}

.input {
  display: block;
  width: 100%;
  padding: 8px;
  border: 1px solid #ccc;
  border-radius: 4px;
  margin-top: 4px;
}

.row {
  display: flex;
  gap: 12px;
  align-items: center;
}

table { width: 100%; border-collapse: collapse; }
th, td { text-align: left; padding: 8px; border-bottom: 1px solid #eee; }

.muted { color: #666; font-size: 13px; }
```

- [ ] **Step 4: Verify the dev server boots**

Run: `cd 06_microservie/frontend && npm run dev -- --port 5173 &` then `sleep 2 && curl -s http://localhost:5173 | head -5 ; kill %1`
Expected: HTML output containing `<div id="root">`.

(If `kill %1` fails on your shell, use `pkill -f "vite.*5173"`.)

### Task 4.3: Commit Phase 4

- [ ] **Step 1: Commit**

```bash
git add 06_microservie/frontend/
git commit -m "microservices(frontend): scaffold Vite + React + TS project with Vitest"
```

---

## Phase 5: Frontend `api/` and `lib/` layers

### Task 5.1: Implement `apiFetch` and `ApiError` (TDD)

**Files:**
- Create: `06_microservie/frontend/src/api/http.ts`
- Create: `06_microservie/frontend/src/api/http.test.ts`

- [ ] **Step 1: Write the failing test**

Create `06_microservie/frontend/src/api/http.test.ts`:

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { apiFetch, ApiError } from './http';

describe('apiFetch', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn());
    vi.stubEnv('VITE_API_BASE', 'http://api.test');
  });

  it('returns data and traceId on success', async () => {
    (fetch as unknown as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      new Response(JSON.stringify({ products: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json', 'X-Trace-Id': 'abc123' },
      }),
    );
    const { data, traceId } = await apiFetch<{ products: unknown[] }>('/api/products');
    expect(data.products).toEqual([]);
    expect(traceId).toBe('abc123');
  });

  it('returns undefined data on 204', async () => {
    (fetch as unknown as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      new Response(null, { status: 204, headers: { 'X-Trace-Id': 'xyz' } }),
    );
    const { data, traceId } = await apiFetch<undefined>('/api/auth/signin', { method: 'POST' });
    expect(data).toBeUndefined();
    expect(traceId).toBe('xyz');
  });

  it('throws ApiError with code/message/traceId on non-2xx', async () => {
    (fetch as unknown as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      new Response(JSON.stringify({ code: 'INVALID_INPUT', message: 'bad', trace_id: 'tr-1' }), {
        status: 400,
        headers: { 'Content-Type': 'application/json', 'X-Trace-Id': 'tr-1' },
      }),
    );
    await expect(apiFetch('/api/whatever')).rejects.toMatchObject({
      name: 'ApiError',
      code: 'INVALID_INPUT',
      message: 'bad',
      traceId: 'tr-1',
    });
    try {
      await apiFetch('/api/whatever');
    } catch (e) {
      expect(e instanceof ApiError).toBe(true);
    }
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 06_microservie/frontend && npm test -- --run`
Expected: FAIL with `Cannot find module './http'` or similar.

- [ ] **Step 3: Implement http.ts**

Create `06_microservie/frontend/src/api/http.ts`:

```ts
export class ApiError extends Error {
  code: string;
  traceId: string;
  constructor(code: string, message: string, traceId: string) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.traceId = traceId;
  }
}

export interface ApiResult<T> {
  data: T;
  traceId: string;
}

export async function apiFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<ApiResult<T>> {
  const base = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080';
  const res = await fetch(base + path, { credentials: 'include', ...init });
  const headerTraceId = res.headers.get('X-Trace-Id') ?? '';

  if (!res.ok) {
    let body: { code?: string; message?: string; trace_id?: string } = {};
    try {
      body = await res.json();
    } catch {
      // ignore parse errors
    }
    throw new ApiError(
      body.code ?? 'UNKNOWN',
      body.message ?? res.statusText,
      body.trace_id ?? headerTraceId,
    );
  }

  if (res.status === 204) {
    return { data: undefined as T, traceId: headerTraceId };
  }
  const data = (await res.json()) as T;
  return { data, traceId: headerTraceId };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd 06_microservie/frontend && npm test -- --run`
Expected: PASS.

### Task 5.2: Implement `lib/format.ts` (TDD)

**Files:**
- Create: `06_microservie/frontend/src/lib/format.ts`
- Create: `06_microservie/frontend/src/lib/format.test.ts`

- [ ] **Step 1: Write the failing test**

Create `06_microservie/frontend/src/lib/format.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { formatPrice, shortTraceId } from './format';

describe('formatPrice', () => {
  it('formats cents to JPY-like string', () => {
    expect(formatPrice(0)).toBe('¥0');
    expect(formatPrice(500)).toBe('¥500');
    expect(formatPrice(123456)).toBe('¥123,456');
  });
});

describe('shortTraceId', () => {
  it('returns first 4 + ellipsis + last 2', () => {
    expect(shortTraceId('4bf92f3577b34da6a3ce929d0e0e4736')).toBe('4bf9…36');
  });

  it('returns empty when input is empty', () => {
    expect(shortTraceId('')).toBe('');
  });

  it('returns input unchanged when too short to shorten', () => {
    expect(shortTraceId('abc')).toBe('abc');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 06_microservie/frontend && npm test -- --run`
Expected: FAIL with `Cannot find module './format'`.

- [ ] **Step 3: Implement format.ts**

Create `06_microservie/frontend/src/lib/format.ts`:

```ts
export function formatPrice(cents: number): string {
  return '¥' + cents.toLocaleString('en-US');
}

export function shortTraceId(id: string): string {
  if (!id) return '';
  if (id.length <= 6) return id;
  return id.slice(0, 4) + '…' + id.slice(-2);
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd 06_microservie/frontend && npm test -- --run`
Expected: PASS.

### Task 5.3: Implement resource-specific api modules

**Files:**
- Create: `06_microservie/frontend/src/api/products.ts`
- Create: `06_microservie/frontend/src/api/auth.ts`
- Create: `06_microservie/frontend/src/api/checkout.ts`
- Create: `06_microservie/frontend/src/api/orders.ts`

- [ ] **Step 1: products.ts**

Create `06_microservie/frontend/src/api/products.ts`:

```ts
import { apiFetch } from './http';

export interface Product {
  id: string;
  name: string;
  description: string;
  price_cents: number;
}

export async function listProducts() {
  const { data } = await apiFetch<{ products: Product[] }>('/api/products');
  return data.products;
}

export async function getProduct(id: string) {
  const { data } = await apiFetch<Product>('/api/products/' + encodeURIComponent(id));
  return data;
}
```

- [ ] **Step 2: auth.ts**

Create `06_microservie/frontend/src/api/auth.ts`:

```ts
import { apiFetch } from './http';

export interface MeResponse {
  user_id: string;
  email: string;
}

export async function me() {
  const { data } = await apiFetch<MeResponse>('/api/auth/me');
  return data;
}

export async function signIn(email: string, password: string) {
  await apiFetch<undefined>('/api/auth/signin', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ Email: email, Password: password }),
  });
}

export async function signUp(email: string, password: string) {
  const { data } = await apiFetch<{ user_id: string }>('/api/auth/signup', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ Email: email, Password: password }),
  });
  return data;
}

export async function signOut() {
  await apiFetch<undefined>('/api/auth/signout', { method: 'POST' });
}
```

(Capital-letter `Email`/`Password` matches the existing BFF struct decoding — see `bff/internal/handler/auth.go`.)

- [ ] **Step 3: checkout.ts**

Create `06_microservie/frontend/src/api/checkout.ts`:

```ts
import { apiFetch } from './http';

export interface CheckoutItem {
  product_id: string;
  quantity: number;
}

export interface CheckoutResponse {
  order_id: string;
  status: string;
}

export async function postCheckout(items: CheckoutItem[]) {
  return apiFetch<CheckoutResponse>('/api/checkout', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ items }),
  });
}
```

(Note: this one returns the full `{data, traceId}` so the Checkout page can show the trace.)

- [ ] **Step 4: orders.ts**

Create `06_microservie/frontend/src/api/orders.ts`:

```ts
import { apiFetch } from './http';

export interface OrderItem {
  product_id: string;
  quantity: number;
  unit_price_cents: number;
}

export interface Order {
  id: string;
  user_id: string;
  status: string;
  total_cents: number;
  items: OrderItem[];
}

export async function listOrders() {
  const { data } = await apiFetch<{ orders: Order[] }>('/api/orders');
  return data.orders;
}

export async function getOrder(id: string) {
  return apiFetch<Order>('/api/orders/' + encodeURIComponent(id));
}
```

- [ ] **Step 5: Run typecheck**

Run: `cd 06_microservie/frontend && npx tsc --noEmit`
Expected: no type errors.

### Task 5.4: Commit Phase 5

- [ ] **Step 1: Commit**

```bash
git add 06_microservie/frontend/src/api/ 06_microservie/frontend/src/lib/
git commit -m "microservices(frontend): add api fetch layer and format utilities with tests"
```

---

## Phase 6: Frontend hooks + shared components

### Task 6.1: Implement `useCart` (TDD)

**Files:**
- Create: `06_microservie/frontend/src/hooks/useCart.ts`
- Create: `06_microservie/frontend/src/hooks/useCart.test.ts`

- [ ] **Step 1: Write the failing test**

We test the pure cart reducers and the storage read/write directly. The hook itself is just a thin React adapter and is verified by the manual smoke test.

Create `06_microservie/frontend/src/hooks/useCart.test.ts`:

```ts
import { describe, it, expect, beforeEach } from 'vitest';
import {
  addToCart,
  setQuantityInCart,
  removeFromCart,
  readCart,
  writeCart,
} from './useCart';

describe('cart reducers', () => {
  it('addToCart appends new item', () => {
    expect(addToCart([], 'p-001', 2)).toEqual([{ productId: 'p-001', quantity: 2 }]);
  });

  it('addToCart increments existing item', () => {
    const before = [{ productId: 'p-001', quantity: 1 }];
    expect(addToCart(before, 'p-001', 3)).toEqual([{ productId: 'p-001', quantity: 4 }]);
  });

  it('setQuantityInCart updates value', () => {
    const before = [{ productId: 'p-001', quantity: 1 }];
    expect(setQuantityInCart(before, 'p-001', 5)).toEqual([{ productId: 'p-001', quantity: 5 }]);
  });

  it('setQuantityInCart with 0 removes', () => {
    const before = [{ productId: 'p-001', quantity: 1 }];
    expect(setQuantityInCart(before, 'p-001', 0)).toEqual([]);
  });

  it('removeFromCart filters', () => {
    const before = [{ productId: 'a', quantity: 1 }, { productId: 'b', quantity: 1 }];
    expect(removeFromCart(before, 'a')).toEqual([{ productId: 'b', quantity: 1 }]);
  });
});

describe('cart storage', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('readCart returns [] when no storage', () => {
    expect(readCart()).toEqual([]);
  });

  it('writeCart then readCart round-trips', () => {
    writeCart([{ productId: 'p-001', quantity: 1 }]);
    expect(readCart()).toEqual([{ productId: 'p-001', quantity: 1 }]);
  });

  it('readCart recovers from broken JSON', () => {
    localStorage.setItem('cart', '{{not json');
    expect(readCart()).toEqual([]);
  });

  it('readCart filters non-conforming items', () => {
    localStorage.setItem('cart', JSON.stringify([{ productId: 'a', quantity: 1 }, { bogus: true }]));
    expect(readCart()).toEqual([{ productId: 'a', quantity: 1 }]);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd 06_microservie/frontend && npm test -- --run`
Expected: FAIL with `Cannot find module './useCart'`.

- [ ] **Step 3: Implement useCart**

Create `06_microservie/frontend/src/hooks/useCart.ts`:

```ts
import { useEffect, useState, useCallback } from 'react';

export interface CartItem {
  productId: string;
  quantity: number;
}

const STORAGE_KEY = 'cart';

export function addToCart(items: CartItem[], productId: string, qty: number): CartItem[] {
  const existing = items.find((it) => it.productId === productId);
  if (existing) {
    return items.map((it) =>
      it.productId === productId ? { ...it, quantity: it.quantity + qty } : it,
    );
  }
  return [...items, { productId, quantity: qty }];
}

export function setQuantityInCart(items: CartItem[], productId: string, quantity: number): CartItem[] {
  if (quantity <= 0) return items.filter((it) => it.productId !== productId);
  return items.map((it) =>
    it.productId === productId ? { ...it, quantity } : it,
  );
}

export function removeFromCart(items: CartItem[], productId: string): CartItem[] {
  return items.filter((it) => it.productId !== productId);
}

export function readCart(): CartItem[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (it): it is CartItem =>
        typeof it === 'object' &&
        it !== null &&
        typeof it.productId === 'string' &&
        typeof it.quantity === 'number',
    );
  } catch {
    return [];
  }
}

export function writeCart(items: CartItem[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(items));
}

export function useCart() {
  const [items, setItems] = useState<CartItem[]>(() => readCart());

  useEffect(() => {
    writeCart(items);
  }, [items]);

  const add = useCallback((productId: string, quantity: number = 1) => {
    setItems((prev) => addToCart(prev, productId, quantity));
  }, []);

  const setQuantity = useCallback((productId: string, quantity: number) => {
    setItems((prev) => setQuantityInCart(prev, productId, quantity));
  }, []);

  const remove = useCallback((productId: string) => {
    setItems((prev) => removeFromCart(prev, productId));
  }, []);

  const clear = useCallback(() => setItems([]), []);

  return { items, add, setQuantity, remove, clear };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd 06_microservie/frontend && npm test -- --run`
Expected: PASS.

### Task 6.2: Implement `useAuth` and `AuthProvider`

**Files:**
- Create: `06_microservie/frontend/src/hooks/useAuth.tsx`

- [ ] **Step 1: Write the AuthProvider**

Create `06_microservie/frontend/src/hooks/useAuth.tsx`:

```tsx
import { createContext, useContext, useEffect, useState, useCallback, ReactNode } from 'react';
import { me, signOut as apiSignOut } from '../api/auth';
import { ApiError } from '../api/http';

interface User {
  id: string;
  email: string;
}

export type AuthState =
  | { status: 'loading' }
  | { status: 'authenticated'; user: User }
  | { status: 'unauthenticated' };

interface AuthContextValue {
  state: AuthState;
  refresh: () => Promise<void>;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' });

  const refresh = useCallback(async () => {
    try {
      const data = await me();
      setState({ status: 'authenticated', user: { id: data.user_id, email: data.email } });
    } catch (e) {
      if (e instanceof ApiError && e.code === 'UNAUTHORIZED') {
        setState({ status: 'unauthenticated' });
        return;
      }
      // For any other error, treat as unauthenticated but log
      console.error('auth probe failed', e);
      setState({ status: 'unauthenticated' });
    }
  }, []);

  const signOut = useCallback(async () => {
    try {
      await apiSignOut();
    } catch (e) {
      console.error('signOut failed', e);
    }
    setState({ status: 'unauthenticated' });
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <AuthContext.Provider value={{ state, refresh, signOut }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider');
  return ctx;
}
```

- [ ] **Step 2: Typecheck**

Run: `cd 06_microservie/frontend && npx tsc --noEmit`
Expected: no errors.

### Task 6.3: Implement shared components

**Files:**
- Create: `06_microservie/frontend/src/components/TraceIdChip.tsx`
- Create: `06_microservie/frontend/src/components/ErrorBanner.tsx`
- Create: `06_microservie/frontend/src/components/RequireAuth.tsx`
- Create: `06_microservie/frontend/src/components/Layout.tsx`
- Create: `06_microservie/frontend/src/components/ProductCard.tsx`

- [ ] **Step 1: TraceIdChip.tsx**

Create `06_microservie/frontend/src/components/TraceIdChip.tsx`:

```tsx
import { shortTraceId } from '../lib/format';

export function TraceIdChip({ traceId }: { traceId: string }) {
  if (!traceId) return null;
  const jaeger = import.meta.env.VITE_JAEGER_URL ?? 'http://localhost:16686';
  const copy = () => {
    void navigator.clipboard.writeText(traceId);
  };
  return (
    <span className="trace-chip">
      trace: <code>{shortTraceId(traceId)}</code>
      <button onClick={copy} aria-label="copy trace id">📋</button>
      <a href={`${jaeger}/trace/${traceId}`} target="_blank" rel="noreferrer">Jaeger</a>
    </span>
  );
}
```

- [ ] **Step 2: ErrorBanner.tsx**

Create `06_microservie/frontend/src/components/ErrorBanner.tsx`:

```tsx
import { ApiError } from '../api/http';
import { TraceIdChip } from './TraceIdChip';

export function ErrorBanner({ error }: { error: unknown }) {
  if (!error) return null;
  if (error instanceof ApiError) {
    return (
      <div className="error-banner">
        <span>{error.message} <span className="muted">({error.code})</span></span>
        <span style={{ flex: 1 }} />
        <TraceIdChip traceId={error.traceId} />
      </div>
    );
  }
  const message = error instanceof Error ? error.message : String(error);
  return <div className="error-banner">{message}</div>;
}
```

- [ ] **Step 3: RequireAuth.tsx**

Create `06_microservie/frontend/src/components/RequireAuth.tsx`:

```tsx
import { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';

export function RequireAuth({ children }: { children: ReactNode }) {
  const { state } = useAuth();
  const location = useLocation();
  if (state.status === 'loading') return <div>読み込み中...</div>;
  if (state.status === 'unauthenticated') {
    const next = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`/signin?next=${next}`} replace />;
  }
  return <>{children}</>;
}
```

- [ ] **Step 4: Layout.tsx**

Create `06_microservie/frontend/src/components/Layout.tsx`:

```tsx
import { Link, Outlet, useNavigate } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';
import { useCart } from '../hooks/useCart';

export function Layout() {
  const auth = useAuth();
  const cart = useCart();
  const navigate = useNavigate();
  const cartCount = cart.items.reduce((sum, it) => sum + it.quantity, 0);

  const onSignOut = async () => {
    await auth.signOut();
    navigate('/');
  };

  return (
    <div>
      <nav className="nav">
        <Link to="/">Shop</Link>
        <Link to="/cart">
          Cart {cartCount > 0 && <span className="badge">{cartCount}</span>}
        </Link>
        {auth.state.status === 'authenticated' && <Link to="/orders">Orders</Link>}
        <span className="spacer" />
        {auth.state.status === 'authenticated' ? (
          <>
            <span className="muted">{auth.state.user.email}</span>
            <button className="btn btn-secondary" onClick={onSignOut}>Sign out</button>
          </>
        ) : auth.state.status === 'unauthenticated' ? (
          <>
            <Link to="/signin">Sign in</Link>
            <Link to="/signup">Sign up</Link>
          </>
        ) : (
          <span className="muted">…</span>
        )}
      </nav>
      <main className="layout">
        <Outlet />
      </main>
    </div>
  );
}
```

- [ ] **Step 5: ProductCard.tsx**

Create `06_microservie/frontend/src/components/ProductCard.tsx`:

```tsx
import { Link } from 'react-router-dom';
import { Product } from '../api/products';
import { formatPrice } from '../lib/format';

export function ProductCard({ product, onAdd }: { product: Product; onAdd: () => void }) {
  return (
    <div className="card">
      <h3><Link to={`/products/${product.id}`}>{product.name}</Link></h3>
      <p className="muted">{product.description}</p>
      <div className="row">
        <strong>{formatPrice(product.price_cents)}</strong>
        <span style={{ flex: 1 }} />
        <button className="btn" onClick={onAdd}>Add</button>
      </div>
    </div>
  );
}
```

- [ ] **Step 6: Typecheck**

Run: `cd 06_microservie/frontend && npx tsc --noEmit`
Expected: no errors.

### Task 6.4: Commit Phase 6

- [ ] **Step 1: Commit**

```bash
git add 06_microservie/frontend/src/hooks/ 06_microservie/frontend/src/components/
git commit -m "microservices(frontend): add auth/cart hooks and shared UI components"
```

---

## Phase 7: Frontend pages and routing

### Task 7.1: Implement `SignIn` and `SignUp` pages

**Files:**
- Create: `06_microservie/frontend/src/pages/SignIn.tsx`
- Create: `06_microservie/frontend/src/pages/SignUp.tsx`

- [ ] **Step 1: SignIn.tsx**

Create `06_microservie/frontend/src/pages/SignIn.tsx`:

```tsx
import { FormEvent, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { signIn } from '../api/auth';
import { useAuth } from '../hooks/useAuth';
import { ErrorBanner } from '../components/ErrorBanner';

export default function SignIn() {
  const [email, setEmail] = useState('alice@example.com');
  const [password, setPassword] = useState('password');
  const [error, setError] = useState<unknown>(null);
  const [submitting, setSubmitting] = useState(false);
  const auth = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await signIn(email, password);
      await auth.refresh();
      const next = params.get('next');
      navigate(next ?? '/');
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <h1>Sign in</h1>
      <ErrorBanner error={error} />
      <form onSubmit={onSubmit}>
        <label>Email
          <input className="input" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </label>
        <label>Password
          <input className="input" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </label>
        <p />
        <button className="btn" type="submit" disabled={submitting}>
          {submitting ? '...' : 'Sign in'}
        </button>
      </form>
    </div>
  );
}
```

- [ ] **Step 2: SignUp.tsx**

Create `06_microservie/frontend/src/pages/SignUp.tsx`:

```tsx
import { FormEvent, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { signUp, signIn } from '../api/auth';
import { useAuth } from '../hooks/useAuth';
import { ErrorBanner } from '../components/ErrorBanner';

export default function SignUp() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [submitting, setSubmitting] = useState(false);
  const auth = useAuth();
  const navigate = useNavigate();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await signUp(email, password);
      await signIn(email, password);
      await auth.refresh();
      navigate('/');
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <h1>Sign up</h1>
      <ErrorBanner error={error} />
      <form onSubmit={onSubmit}>
        <label>Email
          <input className="input" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </label>
        <label>Password
          <input className="input" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </label>
        <p />
        <button className="btn" type="submit" disabled={submitting}>
          {submitting ? '...' : 'Sign up'}
        </button>
      </form>
    </div>
  );
}
```

### Task 7.2: Implement `Products` and `ProductDetail` pages

**Files:**
- Create: `06_microservie/frontend/src/pages/Products.tsx`
- Create: `06_microservie/frontend/src/pages/ProductDetail.tsx`

- [ ] **Step 1: Products.tsx**

Create `06_microservie/frontend/src/pages/Products.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { listProducts, Product } from '../api/products';
import { ProductCard } from '../components/ProductCard';
import { ErrorBanner } from '../components/ErrorBanner';
import { useCart } from '../hooks/useCart';

export default function Products() {
  const [products, setProducts] = useState<Product[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  const cart = useCart();

  useEffect(() => {
    listProducts().then(setProducts).catch(setError);
  }, []);

  if (error) return <ErrorBanner error={error} />;
  if (!products) return <p>Loading...</p>;

  return (
    <div>
      <h1>商品一覧</h1>
      <div className="product-grid">
        {products.map((p) => (
          <ProductCard key={p.id} product={p} onAdd={() => cart.add(p.id)} />
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: ProductDetail.tsx**

Create `06_microservie/frontend/src/pages/ProductDetail.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { getProduct, Product } from '../api/products';
import { ErrorBanner } from '../components/ErrorBanner';
import { useCart } from '../hooks/useCart';
import { formatPrice } from '../lib/format';

export default function ProductDetail() {
  const { id } = useParams();
  const [product, setProduct] = useState<Product | null>(null);
  const [error, setError] = useState<unknown>(null);
  const cart = useCart();

  useEffect(() => {
    if (!id) return;
    getProduct(id).then(setProduct).catch(setError);
  }, [id]);

  if (error) return <ErrorBanner error={error} />;
  if (!product) return <p>Loading...</p>;

  return (
    <div>
      <h1>{product.name}</h1>
      <p>{product.description}</p>
      <p><strong>{formatPrice(product.price_cents)}</strong></p>
      <button className="btn" onClick={() => cart.add(product.id)}>Add to cart</button>
    </div>
  );
}
```

### Task 7.3: Implement `Cart` and `Checkout` pages

**Files:**
- Create: `06_microservie/frontend/src/pages/Cart.tsx`
- Create: `06_microservie/frontend/src/pages/Checkout.tsx`

- [ ] **Step 1: Cart.tsx**

Create `06_microservie/frontend/src/pages/Cart.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { listProducts, Product } from '../api/products';
import { ErrorBanner } from '../components/ErrorBanner';
import { useCart } from '../hooks/useCart';
import { formatPrice } from '../lib/format';

export default function Cart() {
  const cart = useCart();
  const [products, setProducts] = useState<Product[] | null>(null);
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    listProducts().then(setProducts).catch(setError);
  }, []);

  if (error) return <ErrorBanner error={error} />;
  if (cart.items.length === 0) {
    return (
      <div>
        <h1>カート</h1>
        <p className="muted">カートは空です。</p>
        <Link to="/" className="btn btn-secondary">商品一覧へ</Link>
      </div>
    );
  }
  if (!products) return <p>Loading...</p>;

  const lookup = new Map(products.map((p) => [p.id, p]));
  const rows = cart.items.map((it) => {
    const p = lookup.get(it.productId);
    const subtotal = p ? p.price_cents * it.quantity : 0;
    return { it, p, subtotal };
  });
  const total = rows.reduce((s, r) => s + r.subtotal, 0);

  return (
    <div>
      <h1>カート</h1>
      <table>
        <thead>
          <tr><th>商品</th><th>単価</th><th>数量</th><th>小計</th><th></th></tr>
        </thead>
        <tbody>
          {rows.map(({ it, p, subtotal }) => (
            <tr key={it.productId}>
              <td>{p?.name ?? it.productId}</td>
              <td>{p ? formatPrice(p.price_cents) : '-'}</td>
              <td>
                <input
                  type="number" min={0} value={it.quantity}
                  onChange={(e) => cart.setQuantity(it.productId, parseInt(e.target.value, 10) || 0)}
                  style={{ width: 64 }}
                />
              </td>
              <td>{formatPrice(subtotal)}</td>
              <td><button className="btn btn-secondary" onClick={() => cart.remove(it.productId)}>Remove</button></td>
            </tr>
          ))}
        </tbody>
        <tfoot>
          <tr><th colSpan={3}>合計</th><th colSpan={2}>{formatPrice(total)}</th></tr>
        </tfoot>
      </table>
      <p>
        <Link to="/checkout" className="btn">注文確定へ</Link>
      </p>
    </div>
  );
}
```

- [ ] **Step 2: Checkout.tsx**

Create `06_microservie/frontend/src/pages/Checkout.tsx`:

```tsx
import { useState } from 'react';
import { Link } from 'react-router-dom';
import { postCheckout } from '../api/checkout';
import { useCart } from '../hooks/useCart';
import { ErrorBanner } from '../components/ErrorBanner';
import { TraceIdChip } from '../components/TraceIdChip';

interface Success {
  orderId: string;
  status: string;
  traceId: string;
}

export default function Checkout() {
  const cart = useCart();
  const [error, setError] = useState<unknown>(null);
  const [success, setSuccess] = useState<Success | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onConfirm() {
    setError(null);
    setSubmitting(true);
    try {
      const result = await postCheckout(
        cart.items.map((it) => ({ product_id: it.productId, quantity: it.quantity })),
      );
      setSuccess({ orderId: result.data.order_id, status: result.data.status, traceId: result.traceId });
      cart.clear();
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  }

  if (success) {
    return (
      <div>
        <h1>注文が確定しました</h1>
        <p>Order ID: <code>{success.orderId}</code></p>
        <p>Status: {success.status}</p>
        <p><TraceIdChip traceId={success.traceId} /></p>
        <p><Link to="/orders" className="btn">注文履歴を見る</Link></p>
      </div>
    );
  }

  if (cart.items.length === 0) {
    return (
      <div>
        <h1>注文確定</h1>
        <p className="muted">カートが空です。</p>
        <Link to="/" className="btn btn-secondary">商品一覧へ</Link>
      </div>
    );
  }

  return (
    <div>
      <h1>注文確定</h1>
      <ErrorBanner error={error} />
      <p>{cart.items.length} 種類の商品を注文します。</p>
      <button className="btn" onClick={onConfirm} disabled={submitting}>
        {submitting ? '送信中...' : '注文する'}
      </button>
    </div>
  );
}
```

### Task 7.4: Implement `Orders` and `OrderDetail` pages

**Files:**
- Create: `06_microservie/frontend/src/pages/Orders.tsx`
- Create: `06_microservie/frontend/src/pages/OrderDetail.tsx`

- [ ] **Step 1: Orders.tsx**

Create `06_microservie/frontend/src/pages/Orders.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { listOrders, Order } from '../api/orders';
import { ErrorBanner } from '../components/ErrorBanner';
import { formatPrice } from '../lib/format';

export default function Orders() {
  const [orders, setOrders] = useState<Order[] | null>(null);
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    listOrders().then(setOrders).catch(setError);
  }, []);

  if (error) return <ErrorBanner error={error} />;
  if (!orders) return <p>Loading...</p>;
  if (orders.length === 0) return <p className="muted">注文履歴はありません。</p>;

  return (
    <div>
      <h1>注文履歴</h1>
      <table>
        <thead>
          <tr><th>Order ID</th><th>Status</th><th>Total</th><th></th></tr>
        </thead>
        <tbody>
          {orders.map((o) => (
            <tr key={o.id}>
              <td><code>{o.id}</code></td>
              <td>{o.status}</td>
              <td>{formatPrice(o.total_cents)}</td>
              <td><Link to={`/orders/${o.id}`}>詳細</Link></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 2: OrderDetail.tsx**

Create `06_microservie/frontend/src/pages/OrderDetail.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { getOrder, Order } from '../api/orders';
import { ErrorBanner } from '../components/ErrorBanner';
import { TraceIdChip } from '../components/TraceIdChip';
import { formatPrice } from '../lib/format';

export default function OrderDetail() {
  const { id } = useParams();
  const [order, setOrder] = useState<Order | null>(null);
  const [traceId, setTraceId] = useState('');
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    if (!id) return;
    getOrder(id)
      .then((r) => { setOrder(r.data); setTraceId(r.traceId); })
      .catch(setError);
  }, [id]);

  if (error) return <ErrorBanner error={error} />;
  if (!order) return <p>Loading...</p>;

  return (
    <div>
      <h1>注文詳細</h1>
      <p>Order ID: <code>{order.id}</code></p>
      <p>Status: {order.status}</p>
      <table>
        <thead><tr><th>商品 ID</th><th>数量</th><th>単価</th><th>小計</th></tr></thead>
        <tbody>
          {order.items.map((it) => (
            <tr key={it.product_id}>
              <td><code>{it.product_id}</code></td>
              <td>{it.quantity}</td>
              <td>{formatPrice(it.unit_price_cents)}</td>
              <td>{formatPrice(it.unit_price_cents * it.quantity)}</td>
            </tr>
          ))}
        </tbody>
        <tfoot><tr><th colSpan={3}>合計</th><th>{formatPrice(order.total_cents)}</th></tr></tfoot>
      </table>
      <p><TraceIdChip traceId={traceId} /></p>
    </div>
  );
}
```

### Task 7.5: Wire routing in `App.tsx`

**Files:**
- Modify: `06_microservie/frontend/src/App.tsx`

- [ ] **Step 1: Replace App.tsx with full routing**

Replace the entire contents of `06_microservie/frontend/src/App.tsx` with:

```tsx
import { Routes, Route } from 'react-router-dom';
import { Layout } from './components/Layout';
import { RequireAuth } from './components/RequireAuth';
import Products from './pages/Products';
import ProductDetail from './pages/ProductDetail';
import Cart from './pages/Cart';
import Checkout from './pages/Checkout';
import Orders from './pages/Orders';
import OrderDetail from './pages/OrderDetail';
import SignIn from './pages/SignIn';
import SignUp from './pages/SignUp';

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Products />} />
        <Route path="/products/:id" element={<ProductDetail />} />
        <Route path="/cart" element={<Cart />} />
        <Route path="/checkout" element={<RequireAuth><Checkout /></RequireAuth>} />
        <Route path="/orders" element={<RequireAuth><Orders /></RequireAuth>} />
        <Route path="/orders/:id" element={<RequireAuth><OrderDetail /></RequireAuth>} />
        <Route path="/signin" element={<SignIn />} />
        <Route path="/signup" element={<SignUp />} />
        <Route path="*" element={<div>404 Not Found</div>} />
      </Route>
    </Routes>
  );
}
```

- [ ] **Step 2: Typecheck and test**

Run: `cd 06_microservie/frontend && npx tsc --noEmit && npm test -- --run`
Expected: typecheck clean, all tests pass.

### Task 7.6: Commit Phase 7

- [ ] **Step 1: Commit**

```bash
git add 06_microservie/frontend/src/pages/ 06_microservie/frontend/src/App.tsx
git commit -m "microservices(frontend): add pages and wire routing with auth guard"
```

---

## Phase 8: Docker integration, Makefile, and parent spec update

### Task 8.1: Add Dockerfile.dev for frontend

**Files:**
- Create: `06_microservie/frontend/Dockerfile.dev`

- [ ] **Step 1: Write Dockerfile.dev**

Create `06_microservie/frontend/Dockerfile.dev`:

```dockerfile
FROM node:22-alpine
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci || npm install
COPY . .
EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
```

### Task 8.2: Add frontend service to docker-compose

**Files:**
- Modify: `06_microservie/docker-compose.yml`

- [ ] **Step 1: Append frontend service**

In `06_microservie/docker-compose.yml`, before the `# === 観測性スタック ===` comment, add:

```yaml
  frontend:
    build: { context: ./frontend, dockerfile: Dockerfile.dev }
    environment:
      VITE_API_BASE: http://localhost:8080
      VITE_JAEGER_URL: http://localhost:16686
    ports: ["5173:5173"]
    volumes:
      - ./frontend:/app
      - /app/node_modules
    depends_on:
      - bff
    command: npm run dev -- --host 0.0.0.0
```

### Task 8.3: Add frontend Makefile targets

**Files:**
- Modify: `06_microservie/Makefile`

- [ ] **Step 1: Extend the PHONY list**

In `06_microservie/Makefile`, replace the `.PHONY:` line with:

```makefile
.PHONY: help up down logs proto seed seed/catalog seed/inventory seed/user-auth test test/frontend clean up/flaky-20 up/flaky-60 demo/happy demo/retry demo/circuit frontend/install frontend/dev
```

- [ ] **Step 2: Extend the test target and add frontend targets**

Replace the existing `test:` target with:

```makefile
test: test/frontend ## 各 Go モジュールのユニット/インテグレーションテスト + frontend Vitest
	$(TEST_ENV) sh -c 'cd services/catalog && go test ./...'
	$(TEST_ENV) sh -c 'cd services/inventory && go test ./...'
	$(TEST_ENV) sh -c 'cd services/user-auth && go test ./...'
	$(TEST_ENV) sh -c 'cd services/payment && go test ./...'
	$(TEST_ENV) sh -c 'cd services/order && go test ./...'
	cd bff && go test ./...

test/frontend: ## frontend の Vitest を走らせる
	cd frontend && npm test -- --run

frontend/install: ## frontend の依存をローカル npm install
	cd frontend && npm install

frontend/dev: ## frontend をローカルで vite dev 起動（docker を使わない場合）
	cd frontend && npm run dev
```

### Task 8.4: Update top-level `.gitignore`

**Files:**
- Modify: `06_microservie/.gitignore`

- [ ] **Step 1: Append**

Append the following lines to `06_microservie/.gitignore`:

```
frontend/node_modules/
frontend/dist/
frontend/.vite/
```

### Task 8.5: Update parent spec verification checklist

**Files:**
- Modify: `docs/superpowers/specs/2026-05-12-microservices-chapter-design.md`

- [ ] **Step 1: Add a row to section 6.4 verification checklist**

In `docs/superpowers/specs/2026-05-12-microservices-chapter-design.md`, find the table under `### 6.4 章完成の検証チェックリスト`. Add this row after the existing `React UI から商品閲覧 → ログイン → 注文ができる` row:

```
| frontend が `:5173` で動き、注文確定後に trace_id を Jaeger で開ける | ブラウザで手動。完了画面の trace チップから Jaeger に飛ぶ |
```

### Task 8.6: Update README to mention frontend

**Files:**
- Modify: `06_microservie/README.md`

- [ ] **Step 1: Show current contents**

Run: `cat 06_microservie/README.md`

- [ ] **Step 2: Append a section**

Append to `06_microservie/README.md`:

```markdown

## クイックスタート

```bash
make up      # 全コンテナ起動（frontend 含む）
make seed    # 商品10件・ユーザ2件投入
```

その後:
- `http://localhost:5173` — React UI
- `http://localhost:16686` — Jaeger UI

サインインに使えるダミーユーザ:
- `alice@example.com` / `password`
- `bob@example.com` / `password`
```

### Task 8.7: Commit Phase 8

- [ ] **Step 1: Commit**

```bash
git add 06_microservie/docker-compose.yml \
        06_microservie/Makefile \
        06_microservie/.gitignore \
        06_microservie/README.md \
        06_microservie/frontend/Dockerfile.dev \
        docs/superpowers/specs/2026-05-12-microservices-chapter-design.md
git commit -m "microservices(plan3): integrate frontend into compose, Makefile, and parent spec"
```

---

## Phase 9: Manual verification

### Task 9.1: Boot everything and run happy path

- [ ] **Step 1: Build and bring everything up**

Run: `cd 06_microservie && make down && make up && sleep 30 && docker compose ps`
Expected: all services show `running` and `healthy` (postgres ones).

- [ ] **Step 2: Seed**

Run: `cd 06_microservie && make seed`
Expected: no errors. `psql` outputs INSERT counts.

- [ ] **Step 3: Open UI in browser**

Open `http://localhost:5173` manually.

- [ ] **Step 4: Verify happy flow**

Click through:
1. Sign in as `alice@example.com` / `password`
2. Add a product to cart from `/`
3. Go to `/cart`, adjust quantity
4. Go to `/checkout` and click 注文する
5. Confirm the success page shows `Order ID` and a trace chip
6. Click the trace chip's Jaeger link; confirm the trace shows bff → order → inventory → payment

- [ ] **Step 5: Verify auth guard**

In a private window (no cookies), open `http://localhost:5173/checkout`. Expected: redirect to `/signin?next=%2Fcheckout`.

- [ ] **Step 6: Verify sign-out**

Click `Sign out` in the nav. Expected: nav shows `Sign in / Sign up`, accessing `/orders` redirects to signin.

### Task 9.2: Verify resilience demo + error trace surfacing

- [ ] **Step 1: Bring up with high flake rate**

Run: `cd 06_microservie && FLAKE_RATE=0.6 docker compose up -d --build`
Expected: payment service rebuilds with `FLAKE_RATE=0.6`.

- [ ] **Step 2: Trigger failing checkout from the UI**

Open the UI, sign in, add a product, go to `/checkout`, click 注文する. Repeat several times.

- [ ] **Step 3: Verify error banner trace chip**

When a checkout fails, the error banner should show `(UPSTREAM_FAILED)` (or similar code) with a trace chip. Click `Jaeger`. Confirm the failing span is visible.

- [ ] **Step 4: Tear down**

Run: `cd 06_microservie && make down`

### Task 9.3: Final smoke

- [ ] **Step 1: Run all tests**

Run: `cd 06_microservie && make test`
Expected: exit 0 across all Go modules and frontend Vitest.

### Task 9.4: Final commit (if README or anything was tweaked during verification)

- [ ] **Step 1: Commit if needed**

```bash
git status
# If any drift, commit. Otherwise skip.
```

---

## Self-Review Notes

After writing the plan, I checked it against the spec:

- **Spec coverage**: All sections (1–11) of the design spec are addressed. Section 7 (trace_id surfacing) is covered by Task 5.1 (apiFetch), Task 6.3 (TraceIdChip / ErrorBanner), and Tasks 7.3–7.4 (success page and order detail).
- **Type consistency**: `Product.price_cents` (snake_case) is used consistently across api/products.ts, ProductCard, and Cart. `CartItem.productId` (camelCase) is consistent.
- **No placeholders found.**
