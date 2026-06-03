package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
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
	if s == "" {
		return 0.0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return f
}
