package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/alexnthnz/rate-limiter/internal/limiter"
	"github.com/alexnthnz/rate-limiter/internal/middleware"
)

func main() {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:63792",
	})

	tokenConfig := limiter.RateLimiterConfig{
		Algorithm:   limiter.TokenBucket,
		Capacity:    5,
		Rate:        time.Second / 5,
		UseRedis:    true,
		RedisClient: redisClient,
		RedisKey:    "rate:token",
	}

	rl := limiter.NewRateLimiter(tokenConfig)
	mw := middleware.NewRateLimiterMiddleware(rl)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})

	handler := mw.Handler(mux)
	fmt.Println("Server starting on :8080")
	http.ListenAndServe(":8080", handler)
}
