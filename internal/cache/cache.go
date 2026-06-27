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
	mu   sync.RWMutex
	data map[string]entry
	ttl  time.Duration
}

func New(ttl time.Duration) *Cache {
	c := &Cache{data: make(map[string]entry), ttl: ttl}
	go c.evict()
	return c
}

func (c *Cache) Set(key string, val interface{}) {
	c.mu.Lock()
	c.data[key] = entry{value: val, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.value, true
}

func (c *Cache) Flush() {
	c.mu.Lock()
	c.data = make(map[string]entry)
	c.mu.Unlock()
}

func (c *Cache) evict() {
	t := time.NewTicker(60 * time.Second)
	for range t.C {
		now := time.Now()
		c.mu.Lock()
		for k, e := range c.data {
			if now.After(e.expiresAt) {
				delete(c.data, k)
			}
		}
		c.mu.Unlock()
	}
}
