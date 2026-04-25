package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type RateLimitMetrics interface {
	IncRateLimit(policy, result string)
}

type RateLimitConfig struct {
	Enabled              bool
	LoginPerMinute       int
	BrowserAPIPerMinute  int
	ExternalAPIPerMinute int
}

func RateLimit(client *redis.Client, cfg RateLimitConfig, metrics RateLimitMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		policy, limit := rateLimitPolicy(c, cfg)
		if !cfg.Enabled || client == nil || policy == "" || limit <= 0 {
			c.Next()
			return
		}

		key := "rate_limit:" + policy + ":" + hashRateLimitKey(c.ClientIP())
		ctx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
		defer cancel()
		count, err := client.Incr(ctx, key).Result()
		if err != nil {
			if metrics != nil {
				metrics.IncRateLimit(policy, "error")
			}
			c.Next()
			return
		}
		if count == 1 {
			_ = client.Expire(ctx, key, time.Minute).Err()
		}
		if count > int64(limit) {
			if metrics != nil {
				metrics.IncRateLimit(policy, "blocked")
			}
			c.Header("Retry-After", "60")
			writeProblem(c, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		if metrics != nil {
			metrics.IncRateLimit(policy, "allowed")
		}
		c.Next()
	}
}

func rateLimitPolicy(c *gin.Context, cfg RateLimitConfig) (string, int) {
	path := c.Request.URL.Path
	switch {
	case c.Request.Method == http.MethodPost && path == "/api/v1/login":
		return "login", cfg.LoginPerMinute
	case strings.HasPrefix(path, "/api/external/") || strings.HasPrefix(path, "/api/m2m/"):
		return "external_api", cfg.ExternalAPIPerMinute
	case strings.HasPrefix(path, "/api/v1/"):
		return "browser_api", cfg.BrowserAPIPerMinute
	default:
		return "", 0
	}
}

func hashRateLimitKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
