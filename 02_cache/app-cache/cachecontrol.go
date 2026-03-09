package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

func buildCacheKey(r *http.Request) string {
	query := r.URL.Query()
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sortedParts []string
	for _, k := range keys {
		values := query[k]
		sort.Strings(values)
		for _, v := range values {
			sortedParts = append(sortedParts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	sortedQuery := strings.Join(sortedParts, "&")

	return fmt.Sprintf("%s:%s?%s", r.Method, r.URL.Path, sortedQuery)
}

func parseCacheControl(value string) map[string]string {
	directives := make(map[string]string)
	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, "="); idx != -1 {
			directives[strings.TrimSpace(part[:idx])] = strings.TrimSpace(part[idx+1:])
		} else {
			directives[part] = ""
		}
	}
	return directives
}

func shouldCache(directives map[string]string) bool {
	if _, ok := directives["no-cache"]; ok {
		return false
	}
	if _, ok := directives["no-store"]; ok {
		return false
	}
	if _, ok := directives["public"]; !ok {
		return false
	}
	return true
}

func parseMaxAge(directives map[string]string) (time.Duration, bool) {
	maxAgeStr, ok := directives["max-age"]
	if !ok {
		return 0, false
	}
	var seconds int
	_, err := fmt.Sscanf(maxAgeStr, "%d", &seconds)
	if err != nil || seconds <= 0 {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}
