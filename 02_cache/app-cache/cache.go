package main

import (
	"net/http"
	"sync"
	"time"
)

type cacheEntry struct {
	statusCode int
	header     http.Header
	body       []byte
	expiresAt  time.Time
}

type cache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

func newCache() *cache {
	return &cache{
		entries: make(map[string]*cacheEntry),
	}
}

func (c *cache) get(key string) (*cacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry, true
}

func (c *cache) set(key string, entry *cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry
}
