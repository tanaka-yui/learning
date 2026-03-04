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
	// format: "goroutine 123 [running]:\n..."
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
		n := heavyCalcN
		if qn := r.URL.Query().Get("n"); qn != "" {
			if parsed, err := strconv.Atoi(qn); err == nil {
				n = parsed
			}
		}

		startedAt := time.Now()

		fibonacci(n)

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

	fmt.Printf("Go server listening on :8080 (GOMAXPROCS=%d)\n", runtime.GOMAXPROCS(0))
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
