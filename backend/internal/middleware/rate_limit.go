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

type RateLimitDecision struct {
	Policy         string
	LimitPerMinute int
	BucketKey      string
}

type RateLimitResolver func(ctx context.Context, c *gin.Context, policy string, defaultLimit int) (RateLimitDecision, error)

type RateLimitConfig struct {
	Enabled              bool
	LoginPerMinute       int
	BrowserAPIPerMinute  int
	ExternalAPIPerMinute int
	Resolver             RateLimitResolver
}

func RateLimit(client *redis.Client, cfg RateLimitConfig, metrics RateLimitMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		policy, defaultLimit := rateLimitPolicy(c, cfg)
		if !cfg.Enabled || client == nil || policy == "" || defaultLimit <= 0 {
			c.Next()
			return
		}

		decision := RateLimitDecision{
			Policy:         policy,
			LimitPerMinute: defaultLimit,
			BucketKey:      RateLimitBucketKey("ip", c.ClientIP()),
		}
		if cfg.Resolver != nil {
			resolved, err := cfg.Resolver(c.Request.Context(), c, policy, defaultLimit)
			if err == nil && resolved.LimitPerMinute > 0 && resolved.BucketKey != "" {
				if resolved.Policy == "" {
					resolved.Policy = policy
				}
				decision = resolved
			}
		}

		window := time.Now().UTC().Format("200601021504")
		key := "rate_limit:" + decision.Policy + ":" + decision.BucketKey + ":" + window
		ctx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
		defer cancel()
		count, err := client.Incr(ctx, key).Result()
		if err != nil {
			if metrics != nil {
				metrics.IncRateLimit(decision.Policy, "error")
			}
			c.Next()
			return
		}
		if count == 1 {
			_ = client.Expire(ctx, key, time.Minute).Err()
		}
		if count > int64(decision.LimitPerMinute) {
			if metrics != nil {
				metrics.IncRateLimit(decision.Policy, "blocked")
			}
			c.Header("Retry-After", "60")
			writeProblem(c, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		if metrics != nil {
			metrics.IncRateLimit(decision.Policy, "allowed")
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

func RateLimitBucketKey(scope string, values ...string) string {
	joined := scope + "\x00" + strings.Join(values, "\x00")
	sum := sha256.Sum256([]byte(joined))
	return scope + ":" + hex.EncodeToString(sum[:])
}
