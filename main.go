package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/medvedev-v/in-memory-cache/pkg/cache"
	"gopkg.in/yaml.v3"
)

type Config struct {
	CacheRefreshRate int `yaml:"cacherefreshrate"`
	CacheMaxSize     int `yaml:"cachemaxsize"`
}

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot read config: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("cannot parse config: %v", err)
	}

	return &config, nil
}

func main() {
	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	c := cache.New(time.Duration(config.CacheRefreshRate)*time.Second, config.CacheMaxSize)
	defer c.Stop()

	fmt.Printf("Cache started (max size: %d, cleanup: %ds)\n", 
		config.CacheMaxSize, config.CacheRefreshRate)
	fmt.Println("Commands: set, get, delete, keys, size, cleanup, exit")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "set":
			if len(parts) >= 4 {
				ttl, err := time.ParseDuration(parts[3])
				if err != nil {
					fmt.Printf("Invalid TTL: %v\n", err)
				} else {
					c.Set(parts[1], parts[2], ttl)
					fmt.Println("OK")
				}
			} else {
				fmt.Println("Usage: set <key> <value> <ttl>")
			}

		case "get":
			if len(parts) >= 2 {
				if val, exists := c.Get(parts[1]); exists {
					fmt.Printf("Value: %v\n", val)
				} else {
					fmt.Println("Key not found or expired")
				}
			} else {
				fmt.Println("Usage: get <key>")
			}

		case "delete":
			if len(parts) >= 2 {
				c.Delete(parts[1])
				fmt.Println("OK")
			} else {
				fmt.Println("Usage: delete <key>")
			}

		case "keys":
			keys := c.Keys()
			if len(keys) == 0 {
				fmt.Println("Cache is empty")
			} else {
				fmt.Printf("Keys (%d): %v\n", len(keys), keys)
			}

		case "size":
			fmt.Printf("Cache size: %d\n", c.Size())

		case "cleanup":
			c.Cleanup()
			fmt.Println("Cleanup completed")

		case "exit", "quit":
			return

		default:
			fmt.Println("Unknown command. Available: set, get, delete, keys, size, cleanup, exit")
		}
	}
}
