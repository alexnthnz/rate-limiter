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
			<-rl.queue
		}()
		return true
	case <-ctx.Done():
		return false
	default:
		if rl.config.Logger != nil {
			rl.config.Logger.Printf("rate limit exceeded")
		}
		return false
	}
}
