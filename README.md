# Rate Limiter

This repository provides a robust rate limiting library for Go applications. It implements several algorithms and includes middleware for `net/http` servers.

## Features

- **Algorithms:** token bucket, leaky bucket, and sliding window.
- **Thread safe:** designed for concurrent usage.
- **Optional Redis backend:** both token bucket and sliding window can use Redis for distributed scenarios.
- **HTTP middleware:** easy integration with `net/http` including rate limit headers.
- **Custom handlers:** middleware allows a custom handler when a request is throttled.
- **Logging:** optional logger to record limit violations and errors.
- **Metrics:** built-in metrics collection for monitoring and observability.
- **Graceful shutdown:** proper cleanup of resources and goroutines.
- **Configuration validation:** comprehensive validation of configuration parameters.
- **Error handling:** detailed error information and fallback behavior.

## Installation

```
go get github.com/alexnthnz/rate-limiter
```

## Usage

Create a rate limiter with the desired algorithm and plug it into your HTTP handlers:

```go
package main

import (
    "net/http"
    "time"

    "github.com/alexnthnz/rate-limiter/internal/limiter"
    "github.com/alexnthnz/rate-limiter/internal/middleware"
)

func main() {
    rl, err := limiter.NewRateLimiter(limiter.RateLimiterConfig{
        Algorithm: limiter.TokenBucket,
        Capacity:  10,
        Rate:      time.Minute / 10, // 10 requests per minute
    })
    if err != nil {
        panic(err)
    }
    defer rl.Close() // Important: cleanup resources
    
    mw := middleware.NewRateLimiterMiddleware(rl,
        middleware.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
            http.Error(w, "slow down", http.StatusTooManyRequests)
        }),
    )
    http.Handle("/", mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("hello"))
    })))
    http.ListenAndServe(":8080", nil)
}
```

### Advanced Usage

#### With Custom Metrics Collection

```go
// Create a custom metrics collector
collector := limiter.NewDefaultMetricsCollector()

rl, err := limiter.NewRateLimiter(limiter.RateLimiterConfig{
    Algorithm:        limiter.TokenBucket,
    Capacity:         100,
    Rate:             time.Second,
    MetricsCollector: collector,
})
if err != nil {
    panic(err)
}
defer rl.Close()

// Check metrics
metrics := rl.GetMetrics()
fmt.Printf("Allowed: %d, Denied: %d\n", metrics.RequestsAllowed, metrics.RequestsDenied)
```

#### With Redis for Distributed Rate Limiting

```go
import "github.com/redis/go-redis/v9"

client := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

rl, err := limiter.NewRateLimiter(limiter.RateLimiterConfig{
    Algorithm:   limiter.TokenBucket,
    Capacity:    100,
    Rate:        time.Second,
    UseRedis:    true,
    RedisClient: client,
    RedisKey:    "rate_limit:api",
})
if err != nil {
    panic(err)
}
defer rl.Close()
```

#### Sliding Window Algorithm

```go
rl, err := limiter.NewRateLimiter(limiter.RateLimiterConfig{
    Algorithm:    limiter.SlidingWindow,
    Capacity:     100,
    Rate:         time.Second, // Required but not used for sliding window
    CustomWindow: 5 * time.Minute, // 100 requests per 5 minutes
})
if err != nil {
    panic(err)
}
defer rl.Close()
```

#### Detailed Error Handling

```go
result := rl.AllowWithResult(ctx)
if !result.Allowed {
    if result.Error != nil {
        // Handle system error (e.g., Redis failure)
        log.Printf("Rate limiter error: %v", result.Error)
    } else {
        // Handle rate limit exceeded
        log.Printf("Rate limit exceeded")
    }
}
```

## Configuration

The `RateLimiterConfig` struct supports the following options:

- `Algorithm`: The rate limiting algorithm (`TokenBucket`, `LeakyBucket`, or `SlidingWindow`)
- `Capacity`: Maximum number of requests allowed
- `Rate`: Rate of token generation (for token bucket) or processing time (for leaky bucket)
- `CustomWindow`: Time window for sliding window algorithm
- `UseRedis`: Enable Redis backend for distributed rate limiting
- `RedisClient`: Redis client instance (required if `UseRedis` is true)
- `RedisKey`: Redis key prefix (required if `UseRedis` is true)
- `Logger`: Optional logger for debugging and monitoring
- `MetricsCollector`: Optional metrics collector for observability

## Error Handling

The library provides comprehensive error handling:

- **Configuration errors**: Invalid parameters are caught at initialization
- **Redis errors**: Graceful fallback when Redis is unavailable
- **Context cancellation**: Proper handling of request cancellation
- **Resource cleanup**: Automatic cleanup of goroutines and resources

## Metrics

Built-in metrics tracking includes:

- `RequestsAllowed`: Number of requests that were allowed
- `RequestsDenied`: Number of requests that were rate limited
- `RedisErrors`: Number of Redis operation failures
- `ConfigErrors`: Number of configuration validation errors

## Running Tests

Run all tests with:

```
go test ./...
```

Run tests with coverage:

```
go test -cover ./...
```

