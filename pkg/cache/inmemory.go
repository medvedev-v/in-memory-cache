package cache

import (
	"container/heap"
	"sync"
	"time"
)

type InMemoryCache struct {
	mu       sync.RWMutex
	items    map[string]*cacheItem
	expQueue expirationQueue
	stop     chan struct{}
	interval time.Duration
	maxSize  int
}

type cacheItem struct {
	key        string
	value      any
	expiration int64
	index      int
}

// реализация container.heap
type expirationQueue []*cacheItem

func (eq expirationQueue) Len() int { return len(eq) }

func (eq expirationQueue) Less(i, j int) bool {
	return eq[i].expiration < eq[j].expiration
}

func (eq expirationQueue) Swap(i, j int) {
	eq[i], eq[j] = eq[j], eq[i]
	eq[i].index = i
	eq[j].index = j
}

func (eq *expirationQueue) Push(x any) {
	n := len(*eq)
	item := x.(*cacheItem)
	item.index = n
	*eq = append(*eq, item)
}

func (eq *expirationQueue) Pop() any {
	old := *eq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*eq = old[0 : n-1]
	return item
}

// New создает новый экземпляр кэша
func New(cleanupInterval time.Duration, maxSize int) *InMemoryCache {
	c := &InMemoryCache{
		items:    make(map[string]*cacheItem),
		expQueue: make(expirationQueue, 0),
		stop:     make(chan struct{}),
		interval: cleanupInterval,
		maxSize:  maxSize,
	}
	heap.Init(&c.expQueue)
	go c.cleanup()
	return c
}

// Set добавляет или обновляет значение в кэше
func (c *InMemoryCache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiration := time.Now().Add(ttl).UnixNano()

	if item, exists := c.items[key]; exists {
		item.value = value
		item.expiration = expiration
		heap.Fix(&c.expQueue, item.index)
		return
	}

	// Если достигнут максимальный размер, сначала удаляем просроченные
	if c.maxSize > 0 && len(c.items) >= c.maxSize {
		c.deleteExpired()

		// Если после очистки просроченных все еще достигнут максимальный размер,
		// удаляем самый старый элемент
		if len(c.items) >= c.maxSize {
			if c.expQueue.Len() > 0 {
				oldest := heap.Pop(&c.expQueue).(*cacheItem)
				delete(c.items, oldest.key)
			}
		}
	}

	item := &cacheItem{
		key:        key,
		value:      value,
		expiration: expiration,
	}
	heap.Push(&c.expQueue, item)
	c.items[key] = item
}

// Get возвращает значение по ключу
func (c *InMemoryCache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists || time.Now().UnixNano() > item.expiration {
		return nil, false
	}
	return item.value, true
}

// Delete удаляет значение по ключу
func (c *InMemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, exists := c.items[key]; exists {
		heap.Remove(&c.expQueue, item.index)
		delete(c.items, key)
	}
}

// Exists проверяет существование ключа
func (c *InMemoryCache) Exists(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	return exists && time.Now().UnixNano() <= item.expiration
}

// Keys возвращает все действительные ключи
func (c *InMemoryCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now().UnixNano()
	keys := make([]string, 0, len(c.items))
	for k, item := range c.items {
		if now <= item.expiration {
			keys = append(keys, k)
		}
	}
	return keys
}

// Size возвращает текущее количество элементов (включая просроченные)
func (c *InMemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Cleanup удаляет все просроченные записи
func (c *InMemoryCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteExpired()
}

// очистка просроченных элементов
func (c *InMemoryCache) deleteExpired() {
	now := time.Now().UnixNano()
	maxCleanup := 100 // Ограничение на количество удаляемых элементов за раз

	for c.expQueue.Len() > 0 && maxCleanup > 0 {
		oldest := c.expQueue[0]
		if oldest.expiration > now {
			break
		}

		item := heap.Pop(&c.expQueue).(*cacheItem)
		delete(c.items, item.key)
		maxCleanup--
	}
}

// cleanup запускает фоновую очистку
func (c *InMemoryCache) cleanup() {
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

// Stop останавливает фоновую очистку
func (c *InMemoryCache) Stop() {
	close(c.stop)
}
