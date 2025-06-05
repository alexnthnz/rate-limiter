# Rate Limiter

This repository provides a simple rate limiting library for Go applications. It implements several algorithms and includes middleware for `net/http` servers.

## Features

- **Algorithms:** token bucket, leaky bucket, and sliding window.
- **Thread safe:** designed for concurrent usage.
- **Optional Redis backend:** both token bucket and sliding window can use Redis for distributed scenarios.
- **HTTP middleware:** easy integration with `net/http` including rate limit headers.
- **Custom handlers:** middleware allows a custom handler when a request is throttled.
- **Logging:** optional logger to record limit violations.

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

    "github.com/alexnthnz/rate-limiter/internal/limiter"
    "github.com/alexnthnz/rate-limiter/internal/middleware"
)

func main() {
    rl := limiter.NewRateLimiter(limiter.RateLimiterConfig{
        Algorithm: limiter.TokenBucket,
        Capacity:  10,
        Rate:      time.Minute / 10, // 10 requests per minute
    })
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

## Running Tests

Run all tests with:

```
go test ./...
```

