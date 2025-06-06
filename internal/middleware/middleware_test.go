package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexnthnz/rate-limiter/internal/limiter"
)

func TestMiddlewareHeaders(t *testing.T) {
	rl, err := limiter.NewRateLimiter(limiter.RateLimiterConfig{
		Algorithm: limiter.TokenBucket,
		Capacity:  1,
		Rate:      time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}

	mw := NewRateLimiterMiddleware(rl)

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("missing X-RateLimit-Limit header")
	}
	if resp.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("missing X-RateLimit-Remaining header")
	}
	if resp.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("missing X-RateLimit-Reset header")
	}
}

func TestCustomLimitHandler(t *testing.T) {
	rl, err := limiter.NewRateLimiter(limiter.RateLimiterConfig{
		Algorithm: limiter.TokenBucket,
		Capacity:  1,
		Rate:      time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}

	called := false
	mw := NewRateLimiterMiddleware(rl, WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req) // first request allowed

	resp2 := httptest.NewRecorder()
	handler.ServeHTTP(resp2, req) // second should be limited

	if !called {
		t.Error("custom handler not called")
	}
	if resp2.Code != http.StatusTeapot {
		t.Errorf("unexpected status: %d", resp2.Code)
	}
}

func TestNewRateLimiterMiddlewareWithConfig(t *testing.T) {
	config := limiter.RateLimiterConfig{
		Algorithm: limiter.TokenBucket,
		Capacity:  1,
		Rate:      time.Second,
	}

	mw, err := NewRateLimiterMiddlewareWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.Code)
	}
}

func TestNewRateLimiterMiddlewareWithInvalidConfig(t *testing.T) {
	config := limiter.RateLimiterConfig{
		Algorithm: limiter.TokenBucket,
		Capacity:  0, // Invalid
		Rate:      time.Second,
	}

	_, err := NewRateLimiterMiddlewareWithConfig(config)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}
