package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
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
		instanceID = "shared-cache"
	}

	cacheTTL := 10
	if v := os.Getenv("CACHE_TTL"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cacheTTL = parsed
		}
	}

	valkeyAddr := os.Getenv("VALKEY_ADDR")
	if valkeyAddr == "" {
		valkeyAddr = "valkey:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: valkeyAddr,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: could not connect to Valkey at %s: %v", valkeyAddr, err)
	} else {
		log.Printf("Connected to Valkey at %s", valkeyAddr)
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
		cacheKey := buildCacheKey(r.Method, r.URL.Path, r.URL.RawQuery)
		reqCtx := r.Context()

		// Try cache lookup
		cached, err := rdb.Get(reqCtx, cacheKey).Result()
		if err == nil {
			var cr CachedResponse
			if jsonErr := json.Unmarshal([]byte(cached), &cr); jsonErr == nil {
				log.Printf("CACHE HIT: %s", cacheKey)
				for k, vals := range cr.Headers {
					for _, v := range vals {
						w.Header().Add(k, v)
					}
				}
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(cr.StatusCode)
				w.Write([]byte(cr.Body))
				return
			} else {
				log.Printf("Failed to unmarshal cached response for %s: %v", cacheKey, jsonErr)
			}
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

		respHeaders := map[string][]string{
			"Content-Type":       {"application/json"},
			"Cache-Control":      {fmt.Sprintf("public, max-age=%d", cacheTTL)},
			"X-Backend-Instance": {instanceID},
		}

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

		// Write response
		for k, vals := range respHeaders {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(http.StatusOK)
		w.Write(body)

		// Cache the response in Valkey
		cr := CachedResponse{
			StatusCode: http.StatusOK,
			Headers:    respHeaders,
			Body:       string(body),
		}

		jsonData, err := json.Marshal(cr)
		if err != nil {
			log.Printf("Failed to marshal response for caching: %v", err)
			return
		}

		if setErr := rdb.SetEx(reqCtx, cacheKey, string(jsonData), time.Duration(cacheTTL)*time.Second).Err(); setErr != nil {
			log.Printf("Failed to cache response: %v", setErr)
		} else {
			log.Printf("Cached response for %s with TTL %ds", cacheKey, cacheTTL)
		}
	})

	log.Printf("shared-cache listening on :8080 (instance=%s, heavyCalcN=%d, cacheTTL=%d, valkey=%s)", instanceID, heavyCalcN, cacheTTL, valkeyAddr)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
