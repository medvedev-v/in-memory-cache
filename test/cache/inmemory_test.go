package cache_test

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/medvedev-v/in-memory-cache/pkg/cache"
)

func TestBasicOperations(t *testing.T) {
	c := cache.New(time.Minute, 10)
	defer c.Stop()

	// Test Set/Get
	c.Set("key1", "value1", time.Minute)
	if val, exists := c.Get("key1"); !exists || val != "value1" {
		t.Error("Basic Set/Get failed")
	}

	// Test Exists
	if !c.Exists("key1") {
		t.Error("Exists failed for existing key")
	}
	if c.Exists("nonexistent") {
		t.Error("Exists should return false for non-existent key")
	}

	// Test Delete
	c.Delete("key1")
	if _, exists := c.Get("key1"); exists {
		t.Error("Delete failed")
	}
}

func TestTTLExpiration(t *testing.T) {
	c := cache.New(100*time.Millisecond, 10)
	defer c.Stop()

	c.Set("key1", "value1", 50*time.Millisecond)
	
	// Should exist before expiration
	if !c.Exists("key1") {
		t.Error("Key should exist before TTL expiration")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should not exist after expiration
	if c.Exists("key1") {
		t.Error("Key should not exist after TTL expiration")
	}
}

func TestLRUEviction(t *testing.T) {
	c := cache.New(time.Minute, 2)
	defer c.Stop()

	// Fill cache to max size
	c.Set("key1", "value1", time.Minute)
	c.Set("key2", "value2", time.Minute)

	if c.Size() != 2 {
		t.Error("Cache should have 2 items")
	}

	// Access key1 to update its access sequence
	c.Get("key1")

	// Add third item - should trigger LRU eviction of key2 (least recently used)
	c.Set("key3", "value3", time.Minute)

	if c.Size() != 2 {
		t.Errorf("Cache size should not exceed max size, got %d", c.Size())
	}

	// key2 should be evicted, key1 and key3 should remain
	if _, exists := c.Get("key2"); exists {
		t.Error("LRU key should have been evicted")
	}
	if _, exists := c.Get("key1"); !exists {
		t.Error("Recently used key should still exist")
	}
	if _, exists := c.Get("key3"); !exists {
		t.Error("New key should exist")
	}
}

func TestConcurrency(t *testing.T) {
	c := cache.New(time.Minute, 100)
	defer c.Stop()

	var wg sync.WaitGroup
	const goroutines = 10
	const operations = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := string(rune('A' + id))
				c.Set(key, j, time.Minute)
				c.Get(key)
			}
		}(i)
	}

	wg.Wait()
}

func TestCleanup(t *testing.T) {
	c := cache.New(50*time.Millisecond, 10)
	defer c.Stop()

	c.Set("temp", "value", 10*time.Millisecond)
	time.Sleep(100 * time.Millisecond)

	if c.Size() != 0 {
		t.Error("Cleanup should remove expired items")
	}
}

func TestDifferentValueTypes(t *testing.T) {
	c := cache.New(time.Minute, 10)
	defer c.Stop()

	tests := []struct {
		key   string
		value any
	}{
		{"string", "hello"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"slice", []int{1, 2, 3}},
		{"map", map[string]int{"a": 1, "b": 2}},
	}

	for _, tc := range tests {
		c.Set(tc.key, tc.value, time.Minute)
		val, exists := c.Get(tc.key)
		if !exists {
			t.Errorf("Value for key %s should exist", tc.key)
			continue
		}

		// Используем reflect.DeepEqual для корректного сравнения сложных типов
		if !reflect.DeepEqual(val, tc.value) {
			t.Errorf("Value mismatch for key %s: got %v (%T), want %v (%T)", 
				tc.key, val, val, tc.value, tc.value)
		}
	}
}

// Дополнительный тест для проверки последовательности LRU
func TestLRUSequence(t *testing.T) {
	c := cache.New(time.Minute, 3)
	defer c.Stop()

	// Добавляем три элемента
	c.Set("key1", "value1", time.Minute)
	c.Set("key2", "value2", time.Minute) 
	c.Set("key3", "value3", time.Minute)

	// Обращаемся к key1 и key3, делая key2 наименее используемым
	c.Get("key1")
	c.Get("key3")

	// Добавляем четвертый элемент - должен вытеснить key2
	c.Set("key4", "value4", time.Minute)

	// Проверяем, что key2 вытеснен, а остальные остались
	if _, exists := c.Get("key2"); exists {
		t.Error("key2 should have been evicted as LRU")
	}
	if _, exists := c.Get("key1"); !exists {
		t.Error("key1 should still exist")
	}
	if _, exists := c.Get("key3"); !exists {
		t.Error("key3 should still exist")
	}
	if _, exists := c.Get("key4"); !exists {
		t.Error("key4 should exist")
	}
}
