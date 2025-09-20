package cache_test

import (
	"github.com/medvedev-v/in-memory-cache/pkg/cache"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := cache.New(time.Minute, 10)
	if c == nil {
		t.Error("New returned nil")
	}
	c.Stop()
}

func TestSetAndGet(t *testing.T) {
	c := cache.New(time.Minute, 10)
	defer c.Stop()

	c.Set("key1", "value1", time.Minute)

	value, exists := c.Get("key1")
	if !exists {
		t.Error("Key should exist")
	}

	if value != "value1" {
		t.Errorf("Expected 'value1', got %v", value)
	}

	_, exists = c.Get("nonexistent")
	if exists {
		t.Error("Non-existent key should not exist")
	}
}

func TestExpiration(t *testing.T) {
	c := cache.New(10*time.Millisecond, 10)
	defer c.Stop()

	c.Set("key1", "value1", 10*time.Millisecond)

	_, exists := c.Get("key1")
	if !exists {
		t.Error("Key should exist before expiration")
	}

	time.Sleep(20 * time.Millisecond)

	_, exists = c.Get("key1")
	if exists {
		t.Error("Key should not exist after expiration")
	}
}

func TestDelete(t *testing.T) {
	c := cache.New(time.Minute, 10)
	defer c.Stop()

	c.Set("key1", "value1", time.Minute)
	c.Delete("key1")

	_, exists := c.Get("key1")
	if exists {
		t.Error("Key should not exist after deletion")
	}

	c.Delete("nonexistent")
}

func TestExists(t *testing.T) {
	c := cache.New(time.Minute, 10)
	defer c.Stop()

	c.Set("key1", "value1", time.Minute)

	if !c.Exists("key1") {
		t.Error("Exists should return true for existing key")
	}

	if c.Exists("nonexistent") {
		t.Error("Exists should return false for non-existent key")
	}

	c.Set("key2", "value2", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	if c.Exists("key2") {
		t.Error("Exists should return false for expired key")
	}
}

func TestKeys(t *testing.T) {
	c := cache.New(time.Minute, 10)
	defer c.Stop()

	c.Set("key1", "value1", time.Minute)
	c.Set("key2", "value2", time.Minute)
	c.Set("key3", "value3", 10*time.Millisecond)

	keys := c.Keys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	keysMap := make(map[string]bool)
	for _, key := range keys {
		keysMap[key] = true
	}

	if !keysMap["key1"] || !keysMap["key2"] || !keysMap["key3"] {
		t.Error("Not all keys are present in the result")
	}

	time.Sleep(20 * time.Millisecond)
	keys = c.Keys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys after expiration, got %d", len(keys))
	}
}

func TestMaxSize(t *testing.T) {
	c := cache.New(time.Minute, 2)
	defer c.Stop()

	c.Set("key1", "value1", time.Minute)
	c.Set("key2", "value2", time.Minute)

	if len(c.Keys()) != 2 {
		t.Error("Both items should be present")
	}

	c.Set("key3", "value3", time.Minute)

	keys := c.Keys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}

	_, exists := c.Get("key1")
	if exists {
		t.Error("First key should have been evicted")
	}

	_, exists = c.Get("key2")
	if !exists {
		t.Error("Second key should still be present")
	}

	_, exists = c.Get("key3")
	if !exists {
		t.Error("Third key should be present")
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := cache.New(time.Minute, 100)
	defer c.Stop()

	var wg sync.WaitGroup
	numRoutines := 10
	operationsPerRoutine := 100

	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < operationsPerRoutine; j++ {
				key := string(rune('A' + routineID))
				value := j

				if j%2 == 0 {
					c.Set(key, value, time.Minute)
				} else {
					c.Get(key)
				}
			}
		}(i)
	}

	wg.Wait()

	keys := c.Keys()
	if len(keys) > numRoutines {
		t.Errorf("Unexpected number of keys: %d", len(keys))
	}
}

func TestCleanup(t *testing.T) {
	c := cache.New(5*time.Millisecond, 10)
	defer c.Stop()

	c.Set("key1", "value1", 10*time.Millisecond)
	c.Set("key2", "value2", time.Minute)

	time.Sleep(20 * time.Millisecond)

	_, exists := c.Get("key1")
	if exists {
		t.Error("Expired key should have been cleaned up")
	}

	_, exists = c.Get("key2")
	if !exists {
		t.Error("Non-expired key should still exist")
	}
}

func TestStop(t *testing.T) {
	c := cache.New(time.Minute, 10)

	c.Set("key1", "value1", time.Minute)
	c.Set("key2", "value2", time.Minute)

	c.Stop()

	defer func() {
		if r := recover(); r != nil {
			t.Error("Using stopped cache caused panic")
		}
	}()

	c.Set("key3", "value3", time.Minute)
	_, _ = c.Get("key1")
}

func TestEvictionPolicy(t *testing.T) {
	c := cache.New(time.Minute, 2)
	defer c.Stop()

	c.Set("key1", "value1", 10*time.Millisecond)
	c.Set("key2", "value2", time.Minute)

	time.Sleep(20 * time.Millisecond)

	c.Set("key3", "value3", time.Minute)

	_, exists := c.Get("key1")
	if exists {
		t.Error("Expired key should have been evicted")
	}

	_, exists = c.Get("key2")
	if !exists {
		t.Error("Non-expired key should still be present")
	}

	_, exists = c.Get("key3")
	if !exists {
		t.Error("New key should be present")
	}
}

func TestDifferentValueTypes(t *testing.T) {
	c := cache.New(time.Minute, 10)
	defer c.Stop()

	testCases := []struct {
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

	for _, tc := range testCases {
		c.Set(tc.key, tc.value, time.Minute)
		retrieved, exists := c.Get(tc.key)
		if !exists {
			t.Errorf("Value for key %s should exist", tc.key)
		}

		switch v := retrieved.(type) {
		case []int:
			expected := tc.value.([]int)
			if len(v) != len(expected) {
				t.Errorf("Slice length mismatch for key %s", tc.key)
			}
		case map[string]int:
			expected := tc.value.(map[string]int)
			if len(v) != len(expected) {
				t.Errorf("Map length mismatch for key %s", tc.key)
			}
		default:
			if retrieved != tc.value {
				t.Errorf("Value mismatch for key %s", tc.key)
			}
		}
	}
}

func TestMaxSizeWithExpiration(t *testing.T) {
	c := cache.New(time.Minute, 2)
	defer c.Stop()

	c.Set("key1", "value1", 10*time.Millisecond)
	c.Set("key2", "value2", 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)

	c.Set("key3", "value3", time.Minute)

	_, exists := c.Get("key1")
	if exists {
		t.Error("Expired key1 should have been evicted")
	}

	_, exists = c.Get("key2")
	if exists {
		t.Error("Expired key2 should have been evicted")
	}

	_, exists = c.Get("key3")
	if !exists {
		t.Error("New key3 should be present")
	}

	if len(c.Keys()) != 1 {
		t.Errorf("Expected 1 key, got %d", len(c.Keys()))
	}
}

func TestSize(t *testing.T) {
	c := cache.New(time.Minute, 10)
	defer c.Stop()

	if c.Size() != 0 {
		t.Errorf("Expected size 0, got %d", c.Size())
	}

	c.Set("key1", "value1", time.Minute)
	if c.Size() != 1 {
		t.Errorf("Expected size 1, got %d", c.Size())
	}

	c.Set("key2", "value2", 10*time.Millisecond)
	if c.Size() != 2 {
		t.Errorf("Expected size 2, got %d", c.Size())
	}

	time.Sleep(20 * time.Millisecond)

	c.Cleanup()
	if c.Size() != 1 {
		t.Errorf("Expected size 1 after cleanup, got %d", c.Size())
	}
}
