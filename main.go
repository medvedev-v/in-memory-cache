package main

import (
	"encoding/json"
	"fmt"
	"github.com/medvedev-v/in-memory-cache/pkg/cache"
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"os"
	"time"
)

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
	CacheMaxSize     int `yaml:"cachemaxsize"`
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

func main() {
	// Инициализация конфига из файла config.yaml
	var config Config
	config.init()
	cache := cache.New(time.Duration(config.CacheRefreshRate)*time.Second, config.CacheMaxSize)
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

func handleGet(cache *cache.InMemoryCache, key string, w http.ResponseWriter) {
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

func handleSet(cache *cache.InMemoryCache, key string, w http.ResponseWriter, r *http.Request) {
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

func handleDelete(cache *cache.InMemoryCache, key string, w http.ResponseWriter) {
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	cache.Delete(key)
	w.WriteHeader(http.StatusNoContent)
}

func handleKeys(cache *cache.InMemoryCache, w http.ResponseWriter) {
	keys := cache.Keys()
	response := KeysResponse{Keys: keys}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
