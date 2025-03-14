package limiter

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestTokenBucketInMemory(t *testing.T) {
	config := RateLimiterConfig{
		Algorithm: TokenBucket,
		Capacity:  3,
		Rate:      200 * time.Millisecond, // 1 token every 200ms
		UseRedis:  false,
	}
	rl := NewRateLimiter(config)
	ctx := context.Background()

	// Test initial burst
	for i := 0; i < 3; i++ {
		if !rl.Allow(ctx) {
			t.Errorf("Request %d should be allowed in initial burst", i)
		}
	}
	if rl.Allow(ctx) {
		t.Error("Request after capacity should be blocked")
	}

	// Test refill
	time.Sleep(250 * time.Millisecond) // Wait for at least one token to refill
	if !rl.Allow(ctx) {
		t.Error("Request should be allowed after refill")
	}
}

func TestLeakyBucket(t *testing.T) {
	config := RateLimiterConfig{
		Algorithm: LeakyBucket,
		Capacity:  2,
		Rate:      100 * time.Millisecond, // Process 1 request every 100ms
	}
	rl := NewRateLimiter(config)
	ctx := context.Background()

	// Test queue capacity
	for i := 0; i < 2; i++ {
		if !rl.Allow(ctx) {
			t.Errorf("Request %d should be allowed within capacity", i)
		}
	}
	if rl.Allow(ctx) {
		t.Error("Request should be blocked when queue is full")
	}

	// Test processing rate
	time.Sleep(150 * time.Millisecond) // Wait for at least one to process
	if !rl.Allow(ctx) {
		t.Error("Request should be allowed after processing")
	}
}

func TestSlidingWindowInMemory(t *testing.T) {
	config := RateLimiterConfig{
		Algorithm:    SlidingWindow,
		Capacity:     3,
		CustomWindow: 500 * time.Millisecond,
		UseRedis:     false,
	}
	rl := NewRateLimiter(config)
	ctx := context.Background()

	// Test window capacity
	for i := 0; i < 3; i++ {
		if !rl.Allow(ctx) {
			t.Errorf("Request %d should be allowed within window", i)
		}
	}
	if rl.Allow(ctx) {
		t.Error("Request should be blocked when window is full")
	}

	// Test window sliding
	time.Sleep(600 * time.Millisecond) // Wait for window to slide
	for i := 0; i < 3; i++ {
		if !rl.Allow(ctx) {
			t.Errorf("Request %d should be allowed after window slides", i)
		}
	}
}

func TestTokenBucketRedis(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	config := RateLimiterConfig{
		Algorithm:   TokenBucket,
		Capacity:    2,
		Rate:        200 * time.Millisecond,
		UseRedis:    true,
		RedisClient: client,
		RedisKey:    "test:token",
	}
	rl := NewRateLimiter(config)
	ctx := context.Background()

	// Ensure Redis is clean
	client.Del(ctx, config.RedisKey)
	client.Del(ctx, config.RedisKey+":last_refill")

	// Test initial burst
	for i := 0; i < 2; i++ {
		if !rl.Allow(ctx) {
			t.Errorf("Request %d should be allowed in initial burst", i)
		}
	}
	if rl.Allow(ctx) {
		t.Error("Request after capacity should be blocked")
	}

	// Test refill
	time.Sleep(250 * time.Millisecond)
	if !rl.Allow(ctx) {
		t.Error("Request should be allowed after refill")
	}
}

func TestSlidingWindowRedis(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer client.Close()

	config := RateLimiterConfig{
		Algorithm:    SlidingWindow,
		Capacity:     2,
		CustomWindow: 500 * time.Millisecond,
		UseRedis:     true,
		RedisClient:  client,
		RedisKey:     "test:slide",
	}
	rl := NewRateLimiter(config)
	ctx := context.Background()

	// Ensure Redis is clean
	client.Del(ctx, config.RedisKey)

	// Test window capacity
	for i := 0; i < 2; i++ {
		if !rl.Allow(ctx) {
			t.Errorf("Request %d should be allowed within window", i)
		}
	}
	if rl.Allow(ctx) {
		t.Error("Request should be blocked when window is full")
	}

	// Test window sliding
	time.Sleep(600 * time.Millisecond)
	for i := 0; i < 2; i++ {
		if !rl.Allow(ctx) {
			t.Errorf("Request %d should be allowed after window slides", i)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	config := RateLimiterConfig{
		Algorithm: TokenBucket,
		Capacity:  5,
		Rate:      100 * time.Millisecond,
		UseRedis:  false,
	}
	rl := NewRateLimiter(config)
	ctx := context.Background()

	var wg sync.WaitGroup
	successCount := 0
	mu := sync.Mutex{}

	// Simulate 10 concurrent requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Allow(ctx) {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	if successCount != 5 {
		t.Errorf("Expected 5 successful requests, got %d", successCount)
	}
}
