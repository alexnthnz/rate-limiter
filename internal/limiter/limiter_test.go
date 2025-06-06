package limiter

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestTokenBucketInMemory(t *testing.T) {
	config := RateLimiterConfig{
		Algorithm: TokenBucket,
		Capacity:  3,
		Rate:      200 * time.Millisecond, // 1 token every 200ms
		UseRedis:  false,
	}
	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
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
	capacity := 2
	config := RateLimiterConfig{
		Algorithm: LeakyBucket,
		Capacity:  capacity,
		Rate:      100 * time.Millisecond, // Process 1 request every 100ms
	}
	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
	ctx := context.Background()

	// Test queue capacity
	for i := 0; i < capacity; i++ {
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
	capacity := 3
	config := RateLimiterConfig{
		Algorithm:    SlidingWindow,
		Capacity:     capacity,
		Rate:         time.Second,
		CustomWindow: 500 * time.Millisecond,
		UseRedis:     false,
	}
	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
	ctx := context.Background()

	// Test window capacity
	for i := 0; i < capacity; i++ {
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

func TestConcurrentAccess(t *testing.T) {
	capacity := 5
	config := RateLimiterConfig{
		Algorithm: TokenBucket,
		Capacity:  capacity,
		Rate:      100 * time.Millisecond,
		UseRedis:  false,
	}
	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
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
	if successCount != capacity {
		t.Errorf("Expected %d successful requests, got %d", capacity, successCount)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      RateLimiterConfig
		expectedErr error
	}{
		{
			name: "valid token bucket config",
			config: RateLimiterConfig{
				Algorithm: TokenBucket,
				Capacity:  10,
				Rate:      time.Second,
			},
			expectedErr: nil,
		},
		{
			name: "invalid capacity",
			config: RateLimiterConfig{
				Algorithm: TokenBucket,
				Capacity:  0,
				Rate:      time.Second,
			},
			expectedErr: ErrInvalidCapacity,
		},
		{
			name: "invalid rate",
			config: RateLimiterConfig{
				Algorithm: TokenBucket,
				Capacity:  10,
				Rate:      0,
			},
			expectedErr: ErrInvalidRate,
		},
		{
			name: "invalid algorithm",
			config: RateLimiterConfig{
				Algorithm: "invalid",
				Capacity:  10,
				Rate:      time.Second,
			},
			expectedErr: ErrInvalidAlgorithm,
		},
		{
			name: "sliding window without custom window",
			config: RateLimiterConfig{
				Algorithm: SlidingWindow,
				Capacity:  10,
				Rate:      time.Second,
			},
			expectedErr: ErrInvalidWindow,
		},
		{
			name: "redis without client",
			config: RateLimiterConfig{
				Algorithm: TokenBucket,
				Capacity:  10,
				Rate:      time.Second,
				UseRedis:  true,
			},
			expectedErr: ErrMissingRedisClient,
		},
		{
			name: "redis without key",
			config: RateLimiterConfig{
				Algorithm:   TokenBucket,
				Capacity:    10,
				Rate:        time.Second,
				UseRedis:    true,
				RedisClient: &struct{}{}, // dummy client
			},
			expectedErr: ErrMissingRedisKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRateLimiter(tt.config)
			if err != tt.expectedErr {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestGracefulShutdown(t *testing.T) {
	config := RateLimiterConfig{
		Algorithm: LeakyBucket,
		Capacity:  2,
		Rate:      100 * time.Millisecond,
	}
	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
	defer rl.Close()

	ctx := context.Background()
	
	// Fill the queue
	for i := 0; i < 2; i++ {
		if !rl.Allow(ctx) {
			t.Errorf("Request %d should be allowed", i)
		}
	}

	// Close should not hang
	done := make(chan bool)
	go func() {
		rl.Close()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("Close() took too long, possible goroutine leak")
	}
}

func TestAllowWithResult(t *testing.T) {
	config := RateLimiterConfig{
		Algorithm: TokenBucket,
		Capacity:  1,
		Rate:      time.Hour, // Very slow refill
	}
	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
	defer rl.Close()

	ctx := context.Background()
	
	// First request should be allowed
	result := rl.AllowWithResult(ctx)
	if !result.Allowed {
		t.Error("First request should be allowed")
	}
	if result.Error != nil {
		t.Errorf("Unexpected error: %v", result.Error)
	}

	// Second request should be denied
	result = rl.AllowWithResult(ctx)
	if result.Allowed {
		t.Error("Second request should be denied")
	}
	if result.Error != nil {
		t.Errorf("Unexpected error: %v", result.Error)
	}
}

func TestMetrics(t *testing.T) {
	config := RateLimiterConfig{
		Algorithm: TokenBucket,
		Capacity:  2,
		Rate:      time.Hour, // Very slow refill
	}
	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
	defer rl.Close()

	ctx := context.Background()
	
	// Allow 2 requests
	for i := 0; i < 2; i++ {
		if !rl.Allow(ctx) {
			t.Errorf("Request %d should be allowed", i)
		}
	}

	// Deny 3 requests
	for i := 0; i < 3; i++ {
		if rl.Allow(ctx) {
			t.Errorf("Request %d should be denied", i+2)
		}
	}

	metrics := rl.GetMetrics()
	if metrics.RequestsAllowed != 2 {
		t.Errorf("Expected 2 allowed requests, got %d", metrics.RequestsAllowed)
	}
	if metrics.RequestsDenied != 3 {
		t.Errorf("Expected 3 denied requests, got %d", metrics.RequestsDenied)
	}
	if metrics.RedisErrors != 0 {
		t.Errorf("Expected 0 redis errors, got %d", metrics.RedisErrors)
	}
	if metrics.ConfigErrors != 0 {
		t.Errorf("Expected 0 config errors, got %d", metrics.ConfigErrors)
	}
}

func TestCustomMetricsCollector(t *testing.T) {
	collector := NewDefaultMetricsCollector()
	config := RateLimiterConfig{
		Algorithm:        TokenBucket,
		Capacity:         1,
		Rate:             time.Hour,
		MetricsCollector: collector,
	}
	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
	defer rl.Close()

	ctx := context.Background()
	
	// One allowed, one denied
	rl.Allow(ctx)
	rl.Allow(ctx)

	metrics := collector.GetMetrics()
	if metrics.RequestsAllowed != 1 {
		t.Errorf("Expected 1 allowed request, got %d", metrics.RequestsAllowed)
	}
	if metrics.RequestsDenied != 1 {
		t.Errorf("Expected 1 denied request, got %d", metrics.RequestsDenied)
	}
}
