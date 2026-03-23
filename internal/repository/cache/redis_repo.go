package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrLockNotAcquired = errors.New("lock already held by another process")

// RedisRepo provides distributed locking and event availability caching.
type RedisRepo struct {
	client *redis.Client
}

func NewRedisRepo(redisURL string, db int) (*RedisRepo, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	opts.DB = db
	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisRepo{client: client}, nil
}

func (r *RedisRepo) Close() error {
	return r.client.Close()
}

// ─── Distributed Lock ────────────────────────────────────────────────────────

// AcquireLock attempts a SET NX PX lock. Returns the unique token needed to release it.
// The caller MUST call ReleaseLock(token) in a defer to prevent deadlocks.
func (r *RedisRepo) AcquireLock(ctx context.Context, key string, ttl time.Duration) (string, error) {
	token := uuid.New().String()
	ok, err := r.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return "", fmt.Errorf("acquire lock %s: %w", key, err)
	}
	if !ok {
		return "", ErrLockNotAcquired
	}
	return token, nil
}

// releaseLockScript atomically deletes a key only if the stored value equals the token.
// This prevents a slow lock-holder from releasing a lock that was already re-acquired.
var releaseLockScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end`)

func (r *RedisRepo) ReleaseLock(ctx context.Context, key, token string) error {
	return releaseLockScript.Run(ctx, r.client, []string{key}, token).Err()
}

// ─── Availability Cache ───────────────────────────────────────────────────────

func eventCacheKey(eventID uuid.UUID) string {
	return fmt.Sprintf("event:avail:%s", eventID)
}

// CacheAvailability stores the remaining ticket count with a short TTL.
func (r *RedisRepo) CacheAvailability(ctx context.Context, eventID uuid.UUID, remaining int, ttl time.Duration) error {
	return r.client.Set(ctx, eventCacheKey(eventID), remaining, ttl).Err()
}

// GetAvailability returns (remaining, true) on cache hit, or (0, false) on miss.
func (r *RedisRepo) GetAvailability(ctx context.Context, eventID uuid.UUID) (int, bool) {
	val, err := r.client.Get(ctx, eventCacheKey(eventID)).Int()
	if err != nil {
		return 0, false
	}
	return val, true
}

// InvalidateEvent removes the cached availability for an event after a booking.
func (r *RedisRepo) InvalidateEvent(ctx context.Context, eventID uuid.UUID) {
	_ = r.client.Del(ctx, eventCacheKey(eventID)).Err()
}

// ─── Rate Limiter ─────────────────────────────────────────────────────────────

// slidingWindowScript implements a Redis sliding-window counter.
// KEYS[1]   = rate limit key (e.g. "rl:<ip>")
// ARGV[1]   = window size in milliseconds
// ARGV[2]   = max requests allowed in window
// ARGV[3]   = current timestamp in milliseconds
// ARGV[4]   = unique request ID (makes each ZADD member distinct)
// Returns 1 if request is allowed, 0 if rate-limited.
var slidingWindowScript = redis.NewScript(`
local key      = KEYS[1]
local window   = tonumber(ARGV[1])
local limit    = tonumber(ARGV[2])
local now      = tonumber(ARGV[3])
local req_id   = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
local count = redis.call('ZCARD', key)
if count < limit then
    redis.call('ZADD', key, now, req_id)
    redis.call('PEXPIRE', key, window)
    return 1
end
return 0`)

// Allow returns true if the request is within the rate limit for the given key.
func (r *RedisRepo) Allow(ctx context.Context, key string, windowMs int64, limit int, nowMs int64) (bool, error) {
	reqID := uuid.New().String()
	res, err := slidingWindowScript.Run(ctx, r.client,
		[]string{key},
		windowMs, limit, nowMs, reqID,
	).Int()
	if err != nil {
		return false, fmt.Errorf("rate limit script: %w", err)
	}
	return res == 1, nil
}
