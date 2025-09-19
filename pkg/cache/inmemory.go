package cache

import (
	"sync"
	"time"
)

type InMemoryCache struct {
	mu       sync.RWMutex
	items    map[string]*cacheItem
	stop     chan struct{}
	interval time.Duration
	maxSize  int
}

type cacheItem struct {
	Value      any   `json:"value"`
	Expiration int64 `json:"expiration"`
}

func New(cleanupInterval time.Duration, maxSize int) *InMemoryCache {
	c := &InMemoryCache{
		items:    make(map[string]*cacheItem),
		stop:     make(chan struct{}),
		interval: cleanupInterval,
		maxSize:  maxSize,
	}
	go c.cleanup()
	return c
}

func (c *InMemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.maxSize > 0 && len(c.items) >= c.maxSize {
		c.evictOne()
	}

	expiration := time.Now().Add(ttl).UnixNano()
	c.items[key] = &cacheItem{
		Value:      value,
		Expiration: expiration,
	}
}

func (c *InMemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists || time.Now().UnixNano() > item.Expiration {
		return nil, false
	}
	return item.Value, true
}

func (c *InMemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *InMemoryCache) Exists(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, exists := c.items[key]
	return exists && time.Now().UnixNano() <= item.Expiration
}

func (c *InMemoryCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]string, 0, len(c.items))
	now := time.Now().UnixNano()
	for k, item := range c.items {
		if now <= item.Expiration {
			keys = append(keys, k)
		}
	}
	return keys
}

func (c *InMemoryCache) evictOne() {
	var oldestKey string
	var oldestExpiration int64 = 1<<63 - 1 // Max int64

	for k, item := range c.items {
		if item.Expiration < oldestExpiration {
			oldestKey = k
			oldestExpiration = item.Expiration
		}
	}
	delete(c.items, oldestKey)
}

func (c *InMemoryCache) cleanup() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now().UnixNano()
			for key, item := range c.items {
				if now > item.Expiration {
					delete(c.items, key)
				}
			}
			c.mu.Unlock()
		case <-c.stop:
			return
		}
	}
}

func (c *InMemoryCache) Stop() {
	close(c.stop)
}
