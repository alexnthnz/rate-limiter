package limiter

import (
	"context"
	"errors"
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

// Metrics represents rate limiter metrics
type Metrics struct {
	RequestsAllowed  int64
	RequestsDenied   int64
	RedisErrors      int64
	ConfigErrors     int64
}

// MetricsCollector is an interface for collecting metrics
type MetricsCollector interface {
	IncrementAllowed()
	IncrementDenied()
	IncrementRedisError()
	IncrementConfigError()
	GetMetrics() Metrics
}

// DefaultMetricsCollector is a simple in-memory metrics collector
type DefaultMetricsCollector struct {
	mutex   sync.RWMutex
	metrics Metrics
}

func NewDefaultMetricsCollector() *DefaultMetricsCollector {
	return &DefaultMetricsCollector{}
}

func (c *DefaultMetricsCollector) IncrementAllowed() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.metrics.RequestsAllowed++
}

func (c *DefaultMetricsCollector) IncrementDenied() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.metrics.RequestsDenied++
}

func (c *DefaultMetricsCollector) IncrementRedisError() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.metrics.RedisErrors++
}

func (c *DefaultMetricsCollector) IncrementConfigError() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.metrics.ConfigErrors++
}

func (c *DefaultMetricsCollector) GetMetrics() Metrics {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.metrics
}

type RateLimiterConfig struct {
	Algorithm        AlgorithmType
	Capacity         int
	Rate             time.Duration
	UseRedis         bool
	RedisClient      interface{}
	RedisKey         string
	CustomWindow     time.Duration
	Logger           Logger
	MetricsCollector MetricsCollector
}

type RateLimiter struct {
	config     RateLimiterConfig
	tokens     int
	mutex      sync.Mutex
	lastRefill time.Time
	queue      chan struct{}
	requests   []int64
	ctx        context.Context
	cancel     context.CancelFunc
}

var (
	ErrInvalidCapacity     = errors.New("capacity must be greater than 0")
	ErrInvalidRate         = errors.New("rate must be greater than 0")
	ErrInvalidWindow       = errors.New("custom window must be greater than 0 for sliding window algorithm")
	ErrInvalidAlgorithm    = errors.New("invalid algorithm type")
	ErrMissingRedisClient  = errors.New("redis client is required when UseRedis is true")
	ErrMissingRedisKey     = errors.New("redis key is required when UseRedis is true")
)

func NewRateLimiter(config RateLimiterConfig) (*RateLimiter, error) {
	if err := validateConfig(config); err != nil {
		if config.MetricsCollector != nil {
			config.MetricsCollector.IncrementConfigError()
		}
		return nil, err
	}

	// Initialize default metrics collector if none provided
	if config.MetricsCollector == nil {
		config.MetricsCollector = NewDefaultMetricsCollector()
	}

	ctx, cancel := context.WithCancel(context.Background())
	rl := &RateLimiter{
		config:     config,
		lastRefill: time.Now(),
		requests:   make([]int64, 0, config.Capacity), // Use 0 length, config.Capacity capacity
		ctx:        ctx,
		cancel:     cancel,
	}

	if config.Algorithm == LeakyBucket {
		rl.queue = make(chan struct{}, config.Capacity)
	}

	if !config.UseRedis {
		rl.tokens = config.Capacity
	}

	return rl, nil
}

// Close gracefully shuts down the rate limiter and cleans up resources
func (rl *RateLimiter) Close() {
	if rl.cancel != nil {
		rl.cancel()
	}
}

func validateConfig(config RateLimiterConfig) error {
	if config.Capacity <= 0 {
		return ErrInvalidCapacity
	}

	if config.Rate <= 0 {
		return ErrInvalidRate
	}

	switch config.Algorithm {
	case TokenBucket, LeakyBucket:
		// Valid algorithms
	case SlidingWindow:
		if config.CustomWindow <= 0 {
			return ErrInvalidWindow
		}
	case "":
		return ErrInvalidAlgorithm
	default:
		return ErrInvalidAlgorithm
	}

	if config.UseRedis {
		if config.RedisClient == nil {
			return ErrMissingRedisClient
		}
		if config.RedisKey == "" {
			return ErrMissingRedisKey
		}
	}

	return nil
}

// AllowResult represents the result of a rate limit check
type AllowResult struct {
	Allowed bool
	Error   error
}

// Allow checks if a request should be allowed and returns detailed result
func (rl *RateLimiter) Allow(ctx context.Context) bool {
	result := rl.AllowWithResult(ctx)
	return result.Allowed
}

// AllowWithResult checks if a request should be allowed and returns detailed result with error info
func (rl *RateLimiter) AllowWithResult(ctx context.Context) AllowResult {
	switch rl.config.Algorithm {
	case TokenBucket:
		allowed := rl.allowTokenBucket(ctx)
		return AllowResult{Allowed: allowed}
	case LeakyBucket:
		allowed := rl.allowLeakyBucket(ctx)
		return AllowResult{Allowed: allowed}
	case SlidingWindow:
		allowed := rl.allowSlidingWindow(ctx)
		return AllowResult{Allowed: allowed}
	default:
		return AllowResult{Allowed: true}
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

// GetMetrics returns the current metrics for this rate limiter
func (rl *RateLimiter) GetMetrics() Metrics {
	if rl.config.MetricsCollector != nil {
		return rl.config.MetricsCollector.GetMetrics()
	}
	return Metrics{}
}
