package chat

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestModelContextService_BasicOperations(t *testing.T) {
	service := NewModelContextService()
	ctx := t.Context()
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		service.Shutdown(ctx)
	}()

	// Test Set and Get
	err := service.Set("test-model", 4096)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	value, found := service.Get("test-model")
	if !found {
		t.Fatal("Expected to find test-model")
	}
	if value != 4096 {
		t.Fatalf("Expected 4096, got %d", value)
	}

	// Test Get non-existent key
	value, found = service.Get("non-existent")
	if found {
		t.Fatal("Expected not to find non-existent key")
	}
	if value != 0 {
		t.Fatalf("Expected 0 for non-existent key, got %d", value)
	}
}

func TestModelContextService_Clear(t *testing.T) {
	service := NewModelContextService()
	ctx := t.Context()
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		service.Shutdown(ctx)
	}()

	// Add some data
	service.Set("model1", 1024)
	service.Set("model2", 2048)

	// Verify data exists
	_, found1 := service.Get("model1")
	_, found2 := service.Get("model2")
	if !found1 || !found2 {
		t.Fatal("Expected to find both models before clear")
	}

	// Clear cache
	err := service.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify data is gone
	_, found1 = service.Get("model1")
	_, found2 = service.Get("model2")
	if found1 || found2 {
		t.Fatal("Expected not to find models after clear")
	}
}

func TestModelContextService_ConcurrentAccess(t *testing.T) {
	service := NewModelContextService()
	ctx := t.Context()
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		service.Shutdown(ctx)
	}()

	// Run concurrent operations (reduced count for race detector)
	done := make(chan bool, 20)

	// Multiple goroutines setting values
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer func() { done <- true }()
			key := fmt.Sprintf("model-%d", i)
			value := 1000 + i
			err := service.Set(key, value)
			if err != nil {
				t.Errorf("Set failed for %s: %v", key, err)
			}
		}(i)
	}

	// Multiple goroutines getting values
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer func() { done <- true }()
			key := fmt.Sprintf("model-%d", i)
			// Give set operations a chance to complete
			time.Sleep(1 * time.Millisecond)
			value, found := service.Get(key)
			if found {
				expected := 1000 + i
				if value != expected {
					t.Errorf("Expected %d for %s, got %d", expected, key, value)
				}
			}
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < 20; i++ {
		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}

func TestModelContextService_ShutdownGraceful(t *testing.T) {
	service := NewModelContextService()

	// Add some data
	service.Set("test", 1234)
	ctx := t.Context()
	// Shutdown with generous timeout
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	err := service.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// Operations after shutdown should fail gracefully
	_, found := service.Get("test")
	if found {
		t.Fatal("Get should not succeed after shutdown")
	}

	err = service.Set("new-test", 5678)
	if err == nil {
		t.Fatal("Set should fail after shutdown")
	}
}

func TestModelContextService_ShutdownTimeout(t *testing.T) {
	service := NewModelContextService()
	ctx := t.Context()
	// Shutdown with very short timeout
	ctx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()

	err := service.Shutdown(ctx)
	if err == nil {
		t.Fatal("Expected timeout error")
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("Expected DeadlineExceeded, got %v", err)
	}
}

// Benchmark to compare with mutex-based approach
func BenchmarkModelContextService_Get(b *testing.B) {
	service := NewModelContextService()
	ctx := b.Context()
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		service.Shutdown(ctx)
	}()

	// Pre-populate with test data
	service.Set("test-model", 4096)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			service.Get("test-model")
		}
	})
}

func BenchmarkModelContextService_Set(b *testing.B) {
	service := NewModelContextService()
	ctx := b.Context()
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		service.Shutdown(ctx)
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("model-%d", i)
			service.Set(key, 4096)
			i++
		}
	})
}

// Benchmark the original mutex-based implementation for comparison
func BenchmarkOriginalMutexCache_Get(b *testing.B) {
	// Pre-populate the global cache
	contextSizeCacheMutex.Lock()
	contextSizeCache["test-model"] = 4096
	contextSizeCacheMutex.Unlock()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			contextSizeCacheMutex.RLock()
			_, found := contextSizeCache["test-model"]
			contextSizeCacheMutex.RUnlock()
			_ = found
		}
	})
}

func BenchmarkOriginalMutexCache_Set(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("model-%d", i)
			contextSizeCacheMutex.Lock()
			contextSizeCache[key] = 4096
			contextSizeCacheMutex.Unlock()
			i++
		}
	})
}

// Test to ensure API compatibility with existing cache patterns
func TestModelContextService_APICompatibility(t *testing.T) {
	service := NewModelContextService()
	ctx := t.Context()
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		service.Shutdown(ctx)
	}()

	// Test the same cache key pattern used in the original code
	cacheKey := "llama2" + "@" + "http://localhost:11434"

	// Set value
	err := service.Set(cacheKey, 4096)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get value (similar to how the original cache is used)
	value, found := service.Get(cacheKey)
	if !found {
		t.Fatal("Expected to find cached value")
	}
	if value != 4096 {
		t.Fatalf("Expected 4096, got %d", value)
	}

	// This mimics the check-then-use pattern in the original code
	if value, found := service.Get(cacheKey); found {
		// This is how the original code uses the cached value
		if value <= 0 {
			t.Fatal("Expected positive context size")
		}
	} else {
		t.Fatal("Expected to find cached value in compatibility test")
	}
}

// Test the dual implementation functionality
func TestDualImplementation_ChannelVsMutex(t *testing.T) {
	// Clear any existing cache state
	contextSizeCacheMutex.Lock()
	contextSizeCache = make(map[string]int)
	contextSizeCacheMutex.Unlock()

	// Disable channel service to test mutex path
	DisableChannelBasedCache()

	// Test with mutex implementation (should not affect our new service)
	cacheKey := "test-model@http://localhost:11434"
	contextSizeCacheMutex.Lock()
	contextSizeCache[cacheKey] = 2048
	contextSizeCacheMutex.Unlock()

	// Verify mutex cache works
	contextSizeCacheMutex.RLock()
	value, found := contextSizeCache[cacheKey]
	contextSizeCacheMutex.RUnlock()

	if !found || value != 2048 {
		t.Fatal("Mutex cache should work independently")
	}

	// Now enable channel-based cache
	EnableChannelBasedCache()
	defer DisableChannelBasedCache()

	// The channel service should be independent
	channelValue, channelFound := modelContextService.Get(cacheKey)
	if channelFound {
		t.Fatal("Channel service should not have mutex cache data")
	}

	// Set something in channel service
	err := modelContextService.Set(cacheKey, 4096)
	if err != nil {
		t.Fatalf("Failed to set in channel service: %v", err)
	}

	// Verify both caches are independent
	channelValue, channelFound = modelContextService.Get(cacheKey)
	if !channelFound || channelValue != 4096 {
		t.Fatal("Channel service should have its own data")
	}

	// Mutex cache should be unchanged
	contextSizeCacheMutex.RLock()
	mutexValue, mutexFound := contextSizeCache[cacheKey]
	contextSizeCacheMutex.RUnlock()

	if !mutexFound || mutexValue != 2048 {
		t.Fatal("Mutex cache should be unchanged")
	}
}
