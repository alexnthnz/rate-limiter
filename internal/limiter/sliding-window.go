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
	newRequests := rl.requests[:0]
	for _, ts := range rl.requests {
		if ts >= cutoff {
			newRequests = append(newRequests, ts)
		}
	}

	rl.requests = newRequests

	if len(rl.requests) < rl.config.Capacity {
		rl.requests = append(rl.requests, now)
		return true
	}

	return false
}
