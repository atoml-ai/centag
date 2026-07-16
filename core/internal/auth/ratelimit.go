package auth

import (
	"context"
	"math"
	"sync"
	"time"

	"centag/core/pkg/config"
	"centag/core/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// RateLimiter checks whether a request is within per-key rate limits (RPM/TPM).
type RateLimiter interface {
	// Allow returns ok=false when the request is rate-limited.
	// remainingRPM and remainingTPM are the remaining counts after this request.
	// err is non-nil only if the backend is unavailable (caller should allow the request).
	Allow(ctx context.Context, key string, rpm, tpm int) (ok bool, remainingRPM, remainingTPM int, err error)
}

// NewRateLimiter creates a RateLimiter backed by Redis when available, falling
// back to an in-memory token bucket when Redis is unreachable.
func NewRateLimiter(cfg config.RedisConfig) RateLimiter {
	if !cfg.Enabled {
		logger.Infof("rate limiter: Redis disabled, using memory token bucket")
		return newMemoryTokenBucket()
	}

	opts := &redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	}

	rdb := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Warnf("rate limiter: Redis ping failed (%v), falling back to memory token bucket", err)
		rdb.Close()
		return newMemoryTokenBucket()
	}

	logger.Infof("rate limiter: Redis sliding window (addr=%s)", cfg.Addr)
	return &redisSlidingWindow{rdb: rdb, allowScript: redis.NewScript(slidingWindowLua)}
}

const windowMs = 60_000 // 60-second sliding window

func redisRPMKey(key string) string { return "ratelimit:vk:" + key + ":rpm" }
func redisTPMKey(key string) string { return "ratelimit:vk:" + key + ":tpm" }

// Lua script for atomic sliding window rate limit.
// KEYS[1] = rpm key, KEYS[2] = tpm key
// ARGV[1] = now (ms), ARGV[2] = window (ms)
// Returns {ok (0/1), remaining_rpm, remaining_tpm}
// When a limit is 0, it is treated as unlimited and the corresponding
// returned remaining is math.MaxInt64.
const slidingWindowLua = `
local function check(key, limit, now, window)
    if limit <= 0 then return 1, 9223372036854775807 end
    redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
    local count = redis.call('ZCARD', key)
    if count >= limit then
        return 0, 0
    end
    local member = now .. ':' .. redis.call('INCR', key .. ':seq')
    redis.call('ZADD', key, now, member)
    redis.call('EXPIRE', key, 120)
    return 1, limit - count - 1
end
local rpm_ok, rpm_rem = check(KEYS[1], tonumber(ARGV[3]), tonumber(ARGV[1]), tonumber(ARGV[2]))
if rpm_ok == 0 then
    return {0, 0, 0}
end
local tpm_ok, tpm_rem = check(KEYS[2], tonumber(ARGV[4]), tonumber(ARGV[1]), tonumber(ARGV[2]))
if tpm_ok == 0 then
    return {0, 0, 0}
end
return {1, rpm_rem, tpm_rem}
`

// ── Redis sliding window ────────────────────────────────────────────────────

type redisSlidingWindow struct {
	rdb          *redis.Client
	allowScript  *redis.Script
}

func (r *redisSlidingWindow) Allow(ctx context.Context, key string, rpm, tpm int) (ok bool, remainingRPM, remainingTPM int, err error) {
	now := time.Now().UnixMilli()

	keys := []string{redisRPMKey(key), redisTPMKey(key)}
	args := []interface{}{now, windowMs, rpm, tpm}

	val, err := r.allowScript.Run(ctx, r.rdb, keys, args...).Result()
	if err != nil {
		logger.Warnf("rate limiter: Redis script error (%v), allowing request", err)
		return true, math.MaxInt32, math.MaxInt32, nil
	}

	vals := val.([]interface{})
	ok = vals[0].(int64) == 1
	remainingRPM = int(vals[1].(int64))
	remainingTPM = int(vals[2].(int64))
	return
}

// ── Memory token bucket (fallback) ─────────────────────────────────────────

type memoryTokenBucket struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	rpmTokens     float64
	tpmTokens     float64
	rpmLastRefill time.Time
	tpmLastRefill time.Time
}

func newMemoryTokenBucket() *memoryTokenBucket {
	return &memoryTokenBucket{
		buckets: make(map[string]*bucket),
	}
}

func (m *memoryTokenBucket) Allow(ctx context.Context, key string, rpm, tpm int) (ok bool, remainingRPM, remainingTPM int, err error) {
	if rpm <= 0 && tpm <= 0 {
		return true, 0, 0, nil
	}

	m.mu.Lock()
	b, ok := m.buckets[key]
	if !ok {
		b = &bucket{
			rpmTokens:       float64(rpm),
			tpmTokens:       float64(tpm),
			rpmLastRefill:   time.Now(),
			tpmLastRefill:   time.Now(),
		}
		m.buckets[key] = b
	}

	now := time.Now()

	if rpm > 0 {
		elapsed := now.Sub(b.rpmLastRefill).Seconds()
		b.rpmTokens = math.Min(float64(rpm), b.rpmTokens+elapsed*float64(rpm)/60.0)
		b.rpmLastRefill = now
		if b.rpmTokens < 1 {
			m.mu.Unlock()
			return false, 0, 0, nil
		}
		b.rpmTokens--
		remainingRPM = int(math.Floor(b.rpmTokens))
	}

	if tpm > 0 {
		elapsed := now.Sub(b.tpmLastRefill).Seconds()
		b.tpmTokens = math.Min(float64(tpm), b.tpmTokens+elapsed*float64(tpm)/60.0)
		b.tpmLastRefill = now
		if b.tpmTokens < 1 {
			m.mu.Unlock()
			return false, 0, 0, nil
		}
		b.tpmTokens--
		remainingTPM = int(math.Floor(b.tpmTokens))
	}

	m.mu.Unlock()
	return true, remainingRPM, remainingTPM, nil
}
