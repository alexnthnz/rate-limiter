package middleware

import (
	"net/http"

	"github.com/alexnthnz/rate-limiter/internal/limiter"
)

type RateLimiterMiddleware struct {
	limiter *limiter.RateLimiter
}

func NewRateLimiterMiddleware(limiter *limiter.RateLimiter) *RateLimiterMiddleware {
	return &RateLimiterMiddleware{limiter: limiter}
}

func (m *RateLimiterMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if !m.limiter.Allow(ctx) {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
