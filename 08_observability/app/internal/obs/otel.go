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
