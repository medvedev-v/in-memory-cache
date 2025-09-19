package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
	"os"
	"gopkg.in/yaml.v3"
)

type Cache struct {
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

// Запрос для установки значения
type SetRequest struct {
	Value any    `json:"value"`
	TTL   string `json:"ttl"` // в формате "300ms", "2s", "5m"
}

// Ответ для получения значения
type GetResponse struct {
	Value  any  `json:"value,omitempty"`
	Exists bool `json:"exists"`
}

// Ответ для получения всех ключей
type KeysResponse struct {
	Keys []string `json:"keys"`
}

type Config struct {
	CacheRefreshRate int `yaml:"cacherefreshrate"`
	CacheMaxSize int `yaml:"cachemaxsize"`
}

func (config *Config) init() *Config {
	yamlFile, error := os.ReadFile("config.yaml")
	if error != nil {
		panic(error)
	}
	error = yaml.Unmarshal(yamlFile, config)
	if error != nil {
		panic(error)
	}

	return config
}

func NewCache(cleanupInterval time.Duration, maxSize int) *Cache {
	c := &Cache{
		items:    make(map[string]*cacheItem),
		stop:     make(chan struct{}),
		interval: cleanupInterval,
		maxSize:  maxSize,
	}
	go c.cleanup()
	return c
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
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

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists || time.Now().UnixNano() > item.Expiration {
		return nil, false
	}
	return item.Value, true
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *Cache) Exists(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, exists := c.items[key]
	return exists && time.Now().UnixNano() <= item.Expiration
}

func (c *Cache) Keys() []string {
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

func (c *Cache) evictOne() {
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

func (c *Cache) cleanup() {
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

func (c *Cache) Stop() {
	close(c.stop)
}

func main() {
	// Инициализация конфига из файла config.yaml
	var config Config
	config.init()
	cache := NewCache(time.Duration(config.CacheRefreshRate) * time.Second, config.CacheMaxSize)
	defer cache.Stop()

	// Регистрируем HTTP обработчики
	http.HandleFunc("/cache/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/cache/"):]

		switch r.Method {
		case http.MethodGet:
			handleGet(cache, key, w)
		case http.MethodPut:
			handleSet(cache, key, w, r)
		case http.MethodDelete:
			handleDelete(cache, key, w)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/cache", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleKeys(cache, w)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Запускаем сервер на порту 8080
	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleGet(cache *Cache, key string, w http.ResponseWriter) {
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	value, exists := cache.Get(key)
	response := GetResponse{
		Value:  value,
		Exists: exists,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleSet(cache *Cache, key string, w http.ResponseWriter, r *http.Request) {
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ttl, err := time.ParseDuration(req.TTL)
	if err != nil {
		http.Error(w, "Invalid TTL format", http.StatusBadRequest)
		return
	}

	cache.Set(key, req.Value, ttl)
	w.WriteHeader(http.StatusCreated)
}

func handleDelete(cache *Cache, key string, w http.ResponseWriter) {
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	cache.Delete(key)
	w.WriteHeader(http.StatusNoContent)
}

func handleKeys(cache *Cache, w http.ResponseWriter) {
	keys := cache.Keys()
	response := KeysResponse{Keys: keys}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
