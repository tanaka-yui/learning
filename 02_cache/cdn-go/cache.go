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
	cachedAt   time.Time
	expiresAt  time.Time
}

type cdnCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

func newCDNCache() *cdnCache {
	return &cdnCache{
		entries: make(map[string]*cacheEntry),
	}
}

func (c *cdnCache) get(key string) (*cacheEntry, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, false
	}
	return entry, true
}

func (c *cdnCache) set(key string, entry *cacheEntry) {
	c.mu.Lock()
	c.entries[key] = entry
	c.mu.Unlock()
}
