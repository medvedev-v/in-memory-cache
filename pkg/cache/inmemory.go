package cache

import (
	"sync"
	"sync/atomic"
	"time"
)

type InMemoryCache struct {
	mu       sync.RWMutex
	items    map[string]*cacheItem
	stop     chan struct{}
	interval time.Duration
	maxSize  int
	counter  uint64 // Atomic counter for access ordering
}

type cacheItem struct {
	value      any
	expiration int64
	accessSeq  uint64 // Sequence number for LRU eviction
}

func New(cleanupInterval time.Duration, maxSize int) *InMemoryCache {
	c := &InMemoryCache{
		items:    make(map[string]*cacheItem),
		stop:     make(chan struct{}),
		interval: cleanupInterval,
		maxSize:  maxSize,
	}
	
	if cleanupInterval > 0 {
		go c.cleanup()
	}
	return c
}

func (c *InMemoryCache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiration := now.Add(ttl).UnixNano()
	seq := atomic.AddUint64(&c.counter, 1)

	// Если элемент существует - обновляем
	if item, exists := c.items[key]; exists {
		item.value = value
		item.expiration = expiration
		item.accessSeq = seq
		return
	}

	// Если достигнут максимальный размер, удаляем самый старый элемент (LRU)
	if c.maxSize > 0 && len(c.items) >= c.maxSize {
		c.evictLRU()
	}

	c.items[key] = &cacheItem{
		value:      value,
		expiration: expiration,
		accessSeq:  seq,
	}
}

func (c *InMemoryCache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Проверяем не истек ли TTL
	now := time.Now().UnixNano()
	if now > item.expiration {
		delete(c.items, key)
		return nil, false
	}

	// Обновляем последовательность доступа для LRU
	item.accessSeq = atomic.AddUint64(&c.counter, 1)
	return item.value, true
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
	if !exists {
		return false
	}
	return time.Now().UnixNano() <= item.expiration
}

func (c *InMemoryCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now().UnixNano()
	keys := make([]string, 0, len(c.items))
	for key, item := range c.items {
		if now <= item.expiration {
			keys = append(keys, key)
		}
	}
	return keys
}

func (c *InMemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	now := time.Now().UnixNano()
	count := 0
	for _, item := range c.items {
		if now <= item.expiration {
			count++
		}
	}
	return count
}

func (c *InMemoryCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanupExpired()
}

// Удаляет элемент с наименьшим accessSeq (LRU)
func (c *InMemoryCache) evictLRU() {
	if len(c.items) == 0 {
		return
	}
	
	var lruKey string
	var lruSeq uint64 = ^uint64(0) // Max uint64

	for key, item := range c.items {
		if item.accessSeq < lruSeq {
			lruKey = key
			lruSeq = item.accessSeq
		}
	}
	
	if lruKey != "" {
		delete(c.items, lruKey)
	}
}

func (c *InMemoryCache) cleanupExpired() {
	now := time.Now().UnixNano()
	for key, item := range c.items {
		if now > item.expiration {
			delete(c.items, key)
		}
	}
}

func (c *InMemoryCache) cleanup() {
	if c.interval <= 0 {
		return
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.Cleanup()
		case <-c.stop:
			return
		}
	}
}

func (c *InMemoryCache) Stop() {
	close(c.stop)
}
