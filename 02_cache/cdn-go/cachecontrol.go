package main

import (
	"net/http"
	"sort"
	"strings"
)

func parseCacheControl(value string) map[string]string {
	directives := make(map[string]string)
	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if idx := strings.Index(part, "="); idx != -1 {
			key := strings.TrimSpace(part[:idx])
			val := strings.TrimSpace(part[idx+1:])
			directives[strings.ToLower(key)] = val
		} else {
			directives[strings.ToLower(part)] = ""
		}
	}
	return directives
}

func buildCacheKey(r *http.Request, varyHeaders []string) string {
	key := r.Method + " " + r.URL.Path

	query := r.URL.Query()
	queryKeys := make([]string, 0, len(query))
	for k := range query {
		queryKeys = append(queryKeys, k)
	}
	sort.Strings(queryKeys)

	queryParts := make([]string, 0, len(queryKeys))
	for _, k := range queryKeys {
		values := query[k]
		sort.Strings(values)
		for _, v := range values {
			queryParts = append(queryParts, k+"="+v)
		}
	}
	if len(queryParts) > 0 {
		key += "?" + strings.Join(queryParts, "&")
	}

	if len(varyHeaders) > 0 {
		sortedHeaders := make([]string, len(varyHeaders))
		copy(sortedHeaders, varyHeaders)
		sort.Strings(sortedHeaders)

		headerParts := make([]string, 0, len(sortedHeaders))
		for _, h := range sortedHeaders {
			val := r.Header.Get(h)
			if val != "" {
				headerParts = append(headerParts, h+":"+val)
			}
		}
		if len(headerParts) > 0 {
			key += " [" + strings.Join(headerParts, ",") + "]"
		}
	}

	return key
}
