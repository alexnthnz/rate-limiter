package limiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func (rl *RateLimiter) redisTokenBucket(ctx context.Context) bool {
	client := rl.config.RedisClient.(*redis.Client)
	key := rl.config.RedisKey
	now := time.Now().UnixNano()

	// Lua script for atomic token bucket operation
	script := `
		local tokens_key = KEYS[1]
		local last_refill_key = KEYS[1] .. ":last_refill"
		
		-- Get current tokens and last refill time
		local tokens = redis.call('get', tokens_key)
		local last_refill = redis.call('get', last_refill_key)
		
		-- Initialize if not exists
		if not tokens then
			redis.call('set', tokens_key, ARGV[1])
			redis.call('set', last_refill_key, ARGV[2])
			redis.call('pexpire', tokens_key, ARGV[3])
			redis.call('pexpire', last_refill_key, ARGV[3])
			return 1
		end
		
		-- Calculate tokens to add based on elapsed time
		local elapsed = tonumber(ARGV[2]) - tonumber(last_refill)
		local rate = tonumber(ARGV[4])
		local capacity = tonumber(ARGV[1])
		local new_tokens = math.floor(elapsed / rate)
		
		-- Update tokens
		tokens = tonumber(tokens)
		tokens = math.min(capacity, tokens + new_tokens)
		
		-- Check if we can allow the request
		if tokens > 0 then
			redis.call('set', tokens_key, tokens - 1)
			redis.call('set', last_refill_key, ARGV[2])
			redis.call('pexpire', tokens_key, ARGV[3])
			redis.call('pexpire', last_refill_key, ARGV[3])
			return 1
		end
		return 0
	`

	// Arguments: capacity, current time, expiration (rate in ms), rate (in ns)
	result, err := client.Eval(ctx, script, []string{key},
		rl.config.Capacity,            // ARGV[1]: max tokens
		now,                           // ARGV[2]: current time
		rl.config.Rate.Milliseconds(), // ARGV[3]: expiration
		rl.config.Rate.Nanoseconds(),  // ARGV[4]: rate per token
	).Int()

	return err == nil && result == 1
}

func (rl *RateLimiter) redisSlidingWindow(ctx context.Context) bool {
	client := rl.config.RedisClient.(*redis.Client)
	key := rl.config.RedisKey
	now := time.Now().UnixNano()
	window := rl.config.CustomWindow.Nanoseconds()

	script := `
    local current = redis.call('zcount', KEYS[1], ARGV[1], '+inf')
    if current < tonumber(ARGV[2]) then
      redis.call('zadd', KEYS[1], ARGV[3], ARGV[3])
      redis.call('zremrangebyscore', KEYS[1], '-inf', ARGV[1])
      redis.call('pexpire', KEYS[1], ARGV[4])
      return 1
    end
    return 0
  `

	result, err := client.Eval(ctx, script, []string{key}, now-window, rl.config.Capacity, now, rl.config.CustomWindow.Milliseconds()).
		Int()
	return err == nil && result == 1
}
