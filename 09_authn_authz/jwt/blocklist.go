package main

import "sync"

// Blocklist は失効済みトークンJTIを保持するインメモリセット
type Blocklist struct {
	mu      sync.RWMutex
	revoked map[string]struct{}
}

// NewBlocklist は空のBlocklistを返す
func NewBlocklist() *Blocklist {
	return &Blocklist{revoked: make(map[string]struct{})}
}

// Revoke はJTIを失効リストに追加する
func (b *Blocklist) Revoke(jti string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.revoked[jti] = struct{}{}
}

// IsRevoked はJTIが失効済みかを返す
func (b *Blocklist) IsRevoked(jti string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.revoked[jti]
	return ok
}
