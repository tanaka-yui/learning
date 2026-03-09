package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	backendURLsEnv := os.Getenv("BACKEND_URLS")
	if backendURLsEnv == "" {
		log.Fatal("BACKEND_URLS environment variable is required")
	}

	backends := strings.Split(backendURLsEnv, ",")
	for i, b := range backends {
		backends[i] = strings.TrimSpace(b)
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

	var counter uint64

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

		// Round-robin backend selection
		idx := atomic.AddUint64(&counter, 1) - 1
		backendURL := backends[idx%uint64(len(backends))]

		// Build proxy request
		targetURL := backendURL + r.URL.RequestURI()
		proxyReq, err := http.NewRequestWithContext(reqCtx, r.Method, targetURL, r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create proxy request: %v", err), http.StatusBadGateway)
			return
		}

		// Copy request headers
		for k, vals := range r.Header {
			for _, v := range vals {
				proxyReq.Header.Add(k, v)
			}
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(proxyReq)
		if err != nil {
			http.Error(w, fmt.Sprintf("backend request failed: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read backend response: %v", err), http.StatusBadGateway)
			return
		}

		// Copy response headers
		for k, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(resp.StatusCode)
		w.Write(body)

		// Determine if we should cache the response
		ccHeader := resp.Header.Get("Cache-Control")
		if ccHeader == "" {
			return
		}

		isPublic, noCache, noStore, maxAge := parseCacheControl(ccHeader)
		if !isPublic || noCache || noStore || maxAge <= 0 {
			return
		}

		cr := CachedResponse{
			StatusCode: resp.StatusCode,
			Headers:    map[string][]string{},
			Body:       string(body),
		}
		for k, vals := range resp.Header {
			cr.Headers[k] = vals
		}

		jsonData, err := json.Marshal(cr)
		if err != nil {
			log.Printf("Failed to marshal response for caching: %v", err)
			return
		}

		if setErr := rdb.SetEx(reqCtx, cacheKey, string(jsonData), time.Duration(maxAge)*time.Second).Err(); setErr != nil {
			log.Printf("Failed to cache response: %v", setErr)
		} else {
			log.Printf("Cached response for %s with TTL %ds", cacheKey, maxAge)
		}
	})

	log.Printf("Shared cache proxy listening on :8080 (backends=%v, valkey=%s)", backends, valkeyAddr)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
