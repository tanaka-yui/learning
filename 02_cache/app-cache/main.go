package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
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
	if len(backends) == 0 {
		log.Fatal("BACKEND_URLS must contain at least one URL")
	}

	log.Printf("Starting reverse proxy with backends: %v", backends)

	var counter uint64
	memCache := newCache()

	handler := func(w http.ResponseWriter, r *http.Request) {
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

		// Round-robin backend selection
		idx := atomic.AddUint64(&counter, 1) - 1
		backend := backends[idx%uint64(len(backends))]

		// Build backend URL
		targetURL := backend + r.URL.RequestURI()

		// Create proxy request
		proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
		if err != nil {
			http.Error(w, "Failed to create proxy request", http.StatusBadGateway)
			log.Printf("ERROR creating proxy request: %v", err)
			return
		}

		// Copy request headers
		for key, values := range r.Header {
			for _, v := range values {
				proxyReq.Header.Add(key, v)
			}
		}

		// Send request to backend
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(proxyReq)
		if err != nil {
			http.Error(w, "Backend request failed", http.StatusBadGateway)
			log.Printf("ERROR from backend: %v", err)
			return
		}
		defer resp.Body.Close()

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Failed to read backend response", http.StatusBadGateway)
			log.Printf("ERROR reading response body: %v", err)
			return
		}

		// Check if response should be cached
		ccHeader := resp.Header.Get("Cache-Control")
		if ccHeader != "" {
			directives := parseCacheControl(ccHeader)
			if shouldCache(directives) {
				if ttl, ok := parseMaxAge(directives); ok {
					entry := &cacheEntry{
						statusCode: resp.StatusCode,
						header:     resp.Header.Clone(),
						body:       body,
						expiresAt:  time.Now().Add(ttl),
					}
					memCache.set(cacheKey, entry)
					log.Printf("CACHED: %s (TTL: %v)", cacheKey, ttl)
				}
			}
		}

		// Write response
		for key, values := range resp.Header {
			for _, v := range values {
				w.Header().Add(key, v)
			}
		}
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
	}

	server := &http.Server{
		Addr:         ":8080",
		Handler:      http.HandlerFunc(handler),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("Listening on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
