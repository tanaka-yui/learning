// Package httpmw holds small net/http middlewares for the demo API.
package httpmw

import "net/http"

// CORS lets the browser frontend (a different origin: localhost:5174) call the
// API on localhost:9100. The fetch is "non-simple" because it sends a JSON
// content-type and the `traceparent` header injected by the OTel fetch
// instrumentation, so the browser first issues an OPTIONS preflight. This
// middleware answers that preflight and adds the headers to actual responses.
func CORS(allowOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", allowOrigin)
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, traceparent, tracestate")
			h.Add("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
