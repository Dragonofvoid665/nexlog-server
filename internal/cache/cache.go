// Package cache provides a lightweight in-memory TTL cache backed by sync.Map.
// Safe for concurrent use; suitable for caching CMS data (30s TTL typical).
package cache

import (
	"sync"
	"time"
)

type entry struct {
	value     interface{}
	expiresAt time.Time
}

type Cache struct {
	mu  sync.RWMutex
	m   map[string]entry
	ttl time.Duration
}

func New(ttl time.Duration) *Cache {
	c := &Cache{m: make(map[string]entry), ttl: ttl}
	go c.evict()
	return c
}

func (c *Cache) Set(key string, val interface{}) {
	c.mu.Lock()
	c.m[key] = entry{value: val, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.value, true
}

func (c *Cache) Flush() {
	c.mu.Lock()
	c.m = make(map[string]entry)
	c.mu.Unlock()
}

func (c *Cache) evict() {
	t := time.NewTicker(60 * time.Second)
	for range t.C {
		now := time.Now()
		c.mu.Lock()
		for k, e := range c.m {
			if now.After(e.expiresAt) {
				delete(c.m, k)
			}
		}
		c.mu.Unlock()
	}
}
