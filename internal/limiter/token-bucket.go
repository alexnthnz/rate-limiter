package limiter

import (
	"context"
	"time"
)

func (rl *RateLimiter) allowTokenBucket(ctx context.Context) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	if rl.config.UseRedis {
		return rl.redisTokenBucket(ctx)
	}

	rl.refill()
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	newTokens := int(elapsed / rl.config.Rate)
	if newTokens > 0 {
		rl.tokens = min(rl.config.Capacity, rl.tokens+newTokens)
		rl.lastRefill = now
	}
}
