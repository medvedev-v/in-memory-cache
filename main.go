package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/medvedev-v/in-memory-cache/pkg/cache"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	CacheRefreshRate int `yaml:"cacherefreshrate"`
	CacheMaxSize     int `yaml:"cachemaxsize"`
}

func loadConfig(filename string) (*Config, error) {
	config := &Config{}

	yamlFile, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла конфигурации: %v", err)
	}

	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка разбора YAML: %v", err)
	}

	return config, nil
}

func main() {
	// Загрузка конфигурации
	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Не удалось загрузить конфигурацию: %v", err)
	}

	// Создание in-memory кэша
	cache := cache.New(time.Duration(config.CacheRefreshRate)*time.Second, config.CacheMaxSize)
	defer cache.Stop()

	fmt.Printf("In-Memory Cache запущен (макс. размер: %d, интервал очистки: %d сек)\n",
		config.CacheMaxSize, config.CacheRefreshRate)
	fmt.Println("Введите команду (help - для справки):")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		command := parts[0]

		switch command {
		case "set":
			handleSet(cache, parts[1], parts[2], parts[3])
		case "get":
			handleGet(cache, parts[1])
		case "delete":
			handleDelete(cache, parts[1])
		case "keys":
			handleKeys(cache)
		case "exists":
			handleExists(cache, parts[1])
		case "exit", "quit":
			fmt.Println("Завершение работы...")
			return
		case "help":
			handleHelp()
		default:
			fmt.Printf("Неизвестная команда: %s. Введите 'help' для справки.\n", command)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Ошибка чтения ввода: %v", err)
	}
}

func handleSet(cache *cache.InMemoryCache, key, valueStr, ttlStr string) {
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		fmt.Printf("Ошибка: неверный формат TTL: %v\n", err)
		return
	}

	// Пытаемся преобразовать значение в число, если возможно
	var value interface{} = valueStr
	if intValue, err := strconv.Atoi(valueStr); err == nil {
		value = intValue
	} else if floatValue, err := strconv.ParseFloat(valueStr, 64); err == nil {
		value = floatValue
	} else if boolValue, err := strconv.ParseBool(valueStr); err == nil {
		value = boolValue
	}

	cache.Set(key, value, ttl)
	fmt.Printf("Ключ '%s' установлен со значением '%v' и TTL %v\n", key, value, ttl)
}

func handleGet(cache *cache.InMemoryCache, key string) {
	value, exists := cache.Get(key)

	if exists {
		// Форматируем вывод в зависимости от типа значения
		switch v := value.(type) {
		case string:
			fmt.Printf("Значение: %s\n", v)
		case int, float64, bool:
			fmt.Printf("Значение: %v\n", v)
		default:
			jsonValue, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				fmt.Printf("Значение: %v\n", v)
			} else {
				fmt.Printf("Значение: %s\n", string(jsonValue))
			}
		}
	} else {
		fmt.Printf("Ключ '%s' не найден или истек\n", key)
	}
}

func handleDelete(cache *cache.InMemoryCache, key string) {
	cache.Delete(key)
	fmt.Printf("Ключ '%s' удален\n", key)
}

func handleKeys(cache *cache.InMemoryCache) {
	keys := cache.Keys()

	if len(keys) == 0 {
		fmt.Println("Кэш пуст")
		return
	}

	fmt.Println("Ключи в кэше:")
	for i, key := range keys {
		fmt.Printf("%d. %s\n", i+1, key)
	}
}

func handleExists(cache *cache.InMemoryCache, key string) {
	exists := cache.Exists(key)

	if exists {
		fmt.Printf("Ключ '%s' существует\n", key)
	} else {
		fmt.Printf("Ключ '%s' не существует или истек\n", key)
	}
}

func handleHelp() {
	fmt.Println("Доступные команды:")
	fmt.Println("  set <key> <value> <ttl>     - установить значение")
	fmt.Println("  get <key>                   - получить значение")
	fmt.Println("  delete <key>                - удалить значение")
	fmt.Println("  keys                        - показать все ключи")
	fmt.Println("  exists <key>                - проверить существование ключа")
	fmt.Println("  help                        - показать справку")
	fmt.Println("  exit, quit                  - выйти из приложения")
	fmt.Println()
	fmt.Println("Примеры:")
	fmt.Println("  set vladivostok 2000 5m    - установить int значение на 5 минут")
	fmt.Println("  set chita city 1h          - установить string значение на 1 час")
	fmt.Println("  set samara true 30m        - установить bool значение на 30 минут")
}
