package limiter

import (
	"context"
	"time"
)

func (rl *RateLimiter) allowLeakyBucket(ctx context.Context) bool {
	select {
	case rl.queue <- struct{}{}:
		go func() {
			time.Sleep(rl.config.Rate)
		}()
		return true
	case <-ctx.Done():
		return false
	default:
		return false
	}
}
