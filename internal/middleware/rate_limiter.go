package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Allmight-456/ticketflow/internal/repository/cache"
)

// RateLimiter returns a middleware that enforces a sliding-window rate limit
// per client IP address using Redis.
//
// windowDuration: time window (e.g. 1*time.Minute)
// maxRequests:    maximum allowed requests per window per IP
func RateLimiter(redisRepo *cache.RedisRepo, windowDuration time.Duration, maxRequests int) func(http.Handler) http.Handler {
	windowMs := windowDuration.Milliseconds()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)
			// Key includes the path prefix so different endpoints share limits independently.
			key := fmt.Sprintf("rl:%s:%s", ip, pathGroup(r.URL.Path))
			nowMs := time.Now().UnixMilli()

			allowed, err := redisRepo.Allow(r.Context(), key, windowMs, maxRequests, nowMs)
			if err != nil {
				// On Redis error, fail open — don't block legitimate traffic for an
				// infrastructure hiccup. Log the error upstream via zerolog.
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(windowDuration.Seconds())))
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractIP returns the most-trusted client IP from the request.
func extractIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		// X-Forwarded-For can be a comma-separated list; take the first (client) IP.
		return strings.SplitN(fwd, ",", 2)[0]
	}
	// RemoteAddr includes port — strip it.
	if i := strings.LastIndex(r.RemoteAddr, ":"); i != -1 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}

// pathGroup buckets paths into coarse groups so /events/abc and /events/def
// share the same rate-limit bucket.
func pathGroup(path string) string {
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 3)
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "root"
}
