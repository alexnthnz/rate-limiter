package limiter

import (
	"context"
	"time"
)

func (rl *RateLimiter) allowLeakyBucket(ctx context.Context) bool {
	select {
	case rl.queue <- struct{}{}:
		go rl.processLeakyBucketItem()
		if rl.config.MetricsCollector != nil {
			rl.config.MetricsCollector.IncrementAllowed()
		}
		return true
	case <-ctx.Done():
		return false
	default:
		if rl.config.MetricsCollector != nil {
			rl.config.MetricsCollector.IncrementDenied()
		}
		if rl.config.Logger != nil {
			rl.config.Logger.Printf("rate limit exceeded")
		}
		return false
	}
}

func (rl *RateLimiter) processLeakyBucketItem() {
	select {
	case <-time.After(rl.config.Rate):
		<-rl.queue
	case <-rl.ctx.Done():
		// Drain the queue item if context is cancelled
		<-rl.queue
		return
	}
}
