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

type Logger interface {
	Printf(format string, v ...interface{})
}

type RateLimiterConfig struct {
	Algorithm    AlgorithmType
	Capacity     int
	Rate         time.Duration
	UseRedis     bool
	RedisClient  interface{}
	RedisKey     string
	CustomWindow time.Duration
	Logger       Logger
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

func (rl *RateLimiter) Config() RateLimiterConfig {
	return rl.config
}

func (rl *RateLimiter) Remaining() int {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	switch rl.config.Algorithm {
	case TokenBucket:
		rl.refill()
		return rl.tokens
	case LeakyBucket:
		return rl.config.Capacity - len(rl.queue)
	case SlidingWindow:
		now := time.Now().UnixNano()
		window := rl.config.CustomWindow.Nanoseconds()
		cutoff := now - window
		count := 0
		for _, ts := range rl.requests {
			if ts >= cutoff {
				count++
			}
		}
		return rl.config.Capacity - count
	default:
		return 0
	}
}

func (rl *RateLimiter) ResetTime() time.Time {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	switch rl.config.Algorithm {
	case SlidingWindow:
		if len(rl.requests) == 0 {
			return time.Now().Add(rl.config.CustomWindow)
		}
		oldest := rl.requests[0]
		return time.Unix(0, oldest).Add(rl.config.CustomWindow)
	default:
		return rl.lastRefill.Add(rl.config.Rate)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
