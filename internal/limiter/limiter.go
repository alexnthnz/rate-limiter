package limiter

import (
	"context"
	"sync"
	"time"
)

type AlgorithmType string

const (
	TokenBucket   AlgorithmType = "token_bucket"
	LeakyBucket   AlgorithmType = "leaky_bucket"
	SlidingWindow AlgorithmType = "sliding_window"
)

type RateLimiterConfig struct {
	Algorithm    AlgorithmType
	Capacity     int
	Rate         time.Duration
	UseRedis     bool
	RedisClient  interface{}
	RedisKey     string
	CustomWindow time.Duration
}

type RateLimiter struct {
	config     RateLimiterConfig
	tokens     int
	mutex      sync.Mutex
	lastRefill time.Time
	queue      chan struct{}
	requests   []int64
}

func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		config:     config,
		lastRefill: time.Now(),
		requests:   make([]int64, config.Capacity),
	}

	if config.Algorithm == LeakyBucket {
		rl.queue = make(chan struct{}, config.Capacity)
	}

	if !config.UseRedis {
		rl.tokens = config.Capacity
	}

	return rl
}

func (rl *RateLimiter) Allow(ctx context.Context) bool {
	switch rl.config.Algorithm {
	case TokenBucket:
		return rl.allowTokenBucket(ctx)
	case LeakyBucket:
		return rl.allowLeakyBucket(ctx)
	case SlidingWindow:
		return rl.allowSlidingWindow(ctx)
	default:
		return true
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
