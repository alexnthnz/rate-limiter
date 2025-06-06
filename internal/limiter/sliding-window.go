package limiter

import (
	"context"
	"time"
)

func (rl *RateLimiter) allowSlidingWindow(ctx context.Context) bool {
	if rl.config.UseRedis {
		return rl.redisSlidingWindow(ctx)
	}
	return rl.inMemorySlidingWindow()
}

func (rl *RateLimiter) inMemorySlidingWindow() bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now().UnixNano()
	window := rl.config.CustomWindow.Nanoseconds()
	cutoff := now - window

	// Filter out expired requests and rebuild slice to prevent memory growth
	validRequests := make([]int64, 0, rl.config.Capacity)
	for _, ts := range rl.requests {
		if ts >= cutoff {
			validRequests = append(validRequests, ts)
		}
	}

	// Replace the old slice with the new one to free memory
	rl.requests = validRequests

	if len(rl.requests) < rl.config.Capacity {
		rl.requests = append(rl.requests, now)
		if rl.config.MetricsCollector != nil {
			rl.config.MetricsCollector.IncrementAllowed()
		}
		return true
	}

	if rl.config.MetricsCollector != nil {
		rl.config.MetricsCollector.IncrementDenied()
	}
	if rl.config.Logger != nil {
		rl.config.Logger.Printf("rate limit exceeded")
	}
	return false
}
