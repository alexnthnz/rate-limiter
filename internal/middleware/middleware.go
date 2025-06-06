package middleware

import (
	"net/http"
	"strconv"

	"github.com/alexnthnz/rate-limiter/internal/limiter"
)

// RateLimiterMiddleware wraps an HTTP handler with rate limiting logic.
type RateLimiterMiddleware struct {
	limiter   *limiter.RateLimiter
	onLimited http.HandlerFunc
}

// Option configures RateLimiterMiddleware.
type Option func(*RateLimiterMiddleware)

// WithLimitHandler sets a custom handler for rejected requests.
func WithLimitHandler(h http.HandlerFunc) Option {
	return func(m *RateLimiterMiddleware) { m.onLimited = h }
}

// NewRateLimiterMiddleware creates the middleware with optional settings.
func NewRateLimiterMiddleware(l *limiter.RateLimiter, opts ...Option) *RateLimiterMiddleware {
	m := &RateLimiterMiddleware{limiter: l}
	for _, o := range opts {
		o(m)
	}
	return m
}

// NewRateLimiterMiddlewareWithConfig creates middleware directly from config for convenience.
func NewRateLimiterMiddlewareWithConfig(config limiter.RateLimiterConfig, opts ...Option) (*RateLimiterMiddleware, error) {
	l, err := limiter.NewRateLimiter(config)
	if err != nil {
		return nil, err
	}
	return NewRateLimiterMiddleware(l, opts...), nil
}

// Handler returns the HTTP handler that enforces rate limiting.
func (m *RateLimiterMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		allowed := m.limiter.Allow(ctx)

		limit := m.limiter.Config().Capacity
		remaining := m.limiter.Remaining()
		reset := m.limiter.ResetTime().Unix()

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

		if !allowed {
			if m.onLimited != nil {
				m.onLimited(w, r)
			} else {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}
