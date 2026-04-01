package cache

import (
	"context"
	"sync"
	"time"
)

type item struct {
	value   interface{}
	expires time.Time
}

// TTLCache in-memory cache with TTL. For 500 concurrency consider Redis.
type TTLCache struct {
	mu    sync.RWMutex
	items map[string]item
	ttl   time.Duration
	stop  chan struct{}
}

func NewTTLCache(ttl time.Duration, cleanupInterval time.Duration) *TTLCache {
	c := &TTLCache{items: make(map[string]item), ttl: ttl, stop: make(chan struct{})}
	if cleanupInterval > 0 {
		go c.cleanup(cleanupInterval)
	}
	return c
}

func (c *TTLCache) Get(ctx context.Context, key string) (interface{}, bool) {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(it.expires) {
		return nil, false
	}
	return it.value, true
}

func (c *TTLCache) Set(ctx context.Context, key string, value interface{}) {
	c.mu.Lock()
	c.items[key] = item{value: value, expires: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *TTLCache) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for k, v := range c.items {
				if now.After(v.expires) {
					delete(c.items, k)
				}
			}
			c.mu.Unlock()
		}
	}
}

func (c *TTLCache) Close() { close(c.stop) }
