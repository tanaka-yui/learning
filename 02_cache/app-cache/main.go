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
		instanceID = "app-cache"
	}

	cacheTTL := 10
	if v := os.Getenv("CACHE_TTL"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cacheTTL = parsed
		}
	}

	memCache := newCache()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":     "ok",
			"instanceId": instanceID,
		})
	})

	mux.HandleFunc("GET /heavy", func(w http.ResponseWriter, r *http.Request) {
		cacheKey := buildCacheKey(r)

		// Check cache
		if entry, ok := memCache.get(cacheKey); ok {
			log.Printf("CACHE HIT: %s", cacheKey)
			for key, values := range entry.header {
				for _, v := range values {
					w.Header().Add(key, v)
				}
			}
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(entry.statusCode)
			w.Write(entry.body)
			return
		}

		log.Printf("CACHE MISS: %s", cacheKey)

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

		respHeader := http.Header{}
		respHeader.Set("Content-Type", "application/json")
		respHeader.Set("Cache-Control", fmt.Sprintf("public, max-age=%d", cacheTTL))
		respHeader.Set("X-Backend-Instance", instanceID)

		body, err := json.Marshal(map[string]interface{}{
			"instanceId": instanceID,
			"n":          n,
			"result":     result,
			"startedAt":  startedAt.UTC().Format(time.RFC3339Nano),
			"finishedAt": finishedAt.UTC().Format(time.RFC3339Nano),
			"durationMs": durationMs,
		})
		if err != nil {
			http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
			return
		}

		// Cache the response
		ccHeader := respHeader.Get("Cache-Control")
		directives := parseCacheControl(ccHeader)
		if shouldCache(directives) {
			if ttl, ok := parseMaxAge(directives); ok {
				entry := &cacheEntry{
					statusCode: http.StatusOK,
					header:     respHeader.Clone(),
					body:       body,
					expiresAt:  time.Now().Add(ttl),
				}
				memCache.set(cacheKey, entry)
				log.Printf("CACHED: %s (TTL: %v)", cacheKey, ttl)
			}
		}

		// Write response
		for key, values := range respHeader {
			for _, v := range values {
				w.Header().Add(key, v)
			}
		}
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("app-cache listening on :8080 (instance=%s, heavyCalcN=%d, cacheTTL=%d)", instanceID, heavyCalcN, cacheTTL)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
