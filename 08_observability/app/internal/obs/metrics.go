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
