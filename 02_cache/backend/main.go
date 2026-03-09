package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}

func main() {
	heavyCalcN := 35
	if v := os.Getenv("HEAVY_CALC_N"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			heavyCalcN = parsed
		}
	}

	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = "unknown"
	}

	cacheTTL := 10
	if v := os.Getenv("CACHE_TTL"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cacheTTL = parsed
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":     "ok",
			"instanceId": instanceID,
		})
	})

	mux.HandleFunc("GET /heavy", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s (from %s)", instanceID, r.Method, r.URL.RequestURI(), r.RemoteAddr)

		n := heavyCalcN
		if qn := r.URL.Query().Get("n"); qn != "" {
			if parsed, err := strconv.Atoi(qn); err == nil {
				n = parsed
			}
		}

		startedAt := time.Now()
		result := fibonacci(n)
		finishedAt := time.Now()
		durationMs := finishedAt.Sub(startedAt).Milliseconds()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", cacheTTL))
		w.Header().Set("X-Backend-Instance", instanceID)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"instanceId": instanceID,
			"n":          n,
			"result":     result,
			"startedAt":  startedAt.UTC().Format(time.RFC3339Nano),
			"finishedAt": finishedAt.UTC().Format(time.RFC3339Nano),
			"durationMs": durationMs,
		})
	})

	fmt.Printf("Backend server listening on :8080 (instance=%s, heavyCalcN=%d, cacheTTL=%d)\n", instanceID, heavyCalcN, cacheTTL)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
