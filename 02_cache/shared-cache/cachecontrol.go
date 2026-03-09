package main

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
)

func buildCacheKey(method string, path string, rawQuery string) string {
	query, _ := url.ParseQuery(rawQuery)
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
			sortedParts = append(sortedParts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}

	sortedQuery := strings.Join(sortedParts, "&")
	if sortedQuery != "" {
		return method + ":" + path + "?" + sortedQuery
	}
	return method + ":" + path
}

func parseCacheControl(header string) (isPublic bool, noCache bool, noStore bool, maxAge int) {
	maxAge = -1
	directives := strings.Split(header, ",")
	for _, d := range directives {
		d = strings.TrimSpace(strings.ToLower(d))
		if d == "public" {
			isPublic = true
		}
		if d == "no-cache" {
			noCache = true
		}
		if d == "no-store" {
			noStore = true
		}
		if strings.HasPrefix(d, "max-age=") {
			val := strings.TrimPrefix(d, "max-age=")
			if parsed, err := strconv.Atoi(val); err == nil {
				maxAge = parsed
			}
		}
	}
	return
}
