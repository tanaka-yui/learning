package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

func main() {
	backendURLsEnv := os.Getenv("BACKEND_URLS")
	if backendURLsEnv == "" {
		log.Fatal("BACKEND_URLS environment variable is required")
	}

	rawURLs := strings.Split(backendURLsEnv, ",")
	backends := make([]*url.URL, 0, len(rawURLs))
	for _, raw := range rawURLs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			log.Fatalf("invalid backend URL %q: %v", raw, err)
		}
		backends = append(backends, u)
	}
	if len(backends) == 0 {
		log.Fatal("no valid backend URLs provided")
	}

	var varyHeaders []string
	varyHeadersEnv := os.Getenv("CACHE_VARY_HEADERS")
	if varyHeadersEnv != "" {
		for _, h := range strings.Split(varyHeadersEnv, ",") {
			h = strings.TrimSpace(h)
			if h != "" {
				varyHeaders = append(varyHeaders, http.CanonicalHeaderKey(h))
			}
		}
	}

	cache := newCDNCache()
	var counter uint64

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cacheKey := buildCacheKey(r, varyHeaders)

		// Check cache for GET requests
		if r.Method == http.MethodGet {
			if entry, ok := cache.get(cacheKey); ok {
				ttlRemaining := time.Until(entry.expiresAt).Truncate(time.Second)
				backendInstance := entry.header.Get("X-Backend-Instance")
				log.Printf("CACHE HIT: %s (TTL残り: %s, 元backend: %s)", cacheKey, ttlRemaining, backendInstance)
				for k, vals := range entry.header {
					for _, v := range vals {
						w.Header().Add(k, v)
					}
				}
				w.Header().Set("X-Cache", "HIT")
				w.Header().Set("X-Cache-Key", cacheKey)
				w.WriteHeader(entry.statusCode)
				w.Write(entry.body)
				return
			}
		}

		log.Printf("CACHE MISS: %s", cacheKey)
		proxyStart := time.Now()

		// Round-robin backend selection
		idx := atomic.AddUint64(&counter, 1) - 1
		target := backends[idx%uint64(len(backends))]

		proxy := httputil.NewSingleHostReverseProxy(target)
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = target.Host
		}

		// Intercept the response via ModifyResponse to capture body and headers
		var capturedStatus int
		var capturedHeader http.Header
		var capturedBody []byte

		proxy.ModifyResponse = func(resp *http.Response) error {
			capturedStatus = resp.StatusCode
			capturedHeader = resp.Header.Clone()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			resp.Body.Close()
			capturedBody = body
			resp.Body = io.NopCloser(bytes.NewReader(body))

			// Add cache headers before the proxy writes the response
			resp.Header.Set("X-Cache", "MISS")
			resp.Header.Set("X-Cache-Key", cacheKey)

			return nil
		}

		proxy.ServeHTTP(w, r)
		proxyDuration := time.Since(proxyStart)

		// Determine if we should cache
		if r.Method == http.MethodGet && capturedStatus == http.StatusOK {
			ccHeader := capturedHeader.Get("Cache-Control")
			if ccHeader != "" {
				directives := parseCacheControl(ccHeader)
				_, hasNoCache := directives["no-cache"]
				_, hasNoStore := directives["no-store"]
				_, hasPrivate := directives["private"]
				_, hasPublic := directives["public"]
				maxAgeStr, hasMaxAge := directives["max-age"]

				if !hasNoCache && !hasNoStore && !hasPrivate && hasPublic && hasMaxAge {
					parsed, err := strconv.Atoi(maxAgeStr)
					if err == nil && parsed > 0 {
						now := time.Now()
						entry := &cacheEntry{
							statusCode: capturedStatus,
							header:     capturedHeader,
							body:       capturedBody,
							cachedAt:   now,
							expiresAt:  now.Add(time.Duration(parsed) * time.Second),
						}
						cache.set(cacheKey, entry)
						log.Printf("  -> %s (応答: %dms, キャッシュ保存: TTL=%ds)", target, proxyDuration.Milliseconds(), parsed)
					}
				} else {
					log.Printf("  -> %s (応答: %dms, キャッシュ対象外)", target, proxyDuration.Milliseconds())
				}
			} else {
				log.Printf("  -> %s (応答: %dms, Cache-Controlなし)", target, proxyDuration.Milliseconds())
			}
		}
	})

	addr := ":8080"
	fmt.Printf("CDN cache server listening on %s\n", addr)
	fmt.Printf("Backends: %v\n", backends)
	fmt.Printf("Vary headers: %v\n", varyHeaders)
	log.Fatal(http.ListenAndServe(addr, handler))
}
