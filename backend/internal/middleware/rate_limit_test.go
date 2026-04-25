package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type recordingRateLimitMetrics struct {
	events []rateLimitMetricEvent
}

type rateLimitMetricEvent struct {
	policy string
	result string
}

func (m *recordingRateLimitMetrics) IncRateLimit(policy, result string) {
	m.events = append(m.events, rateLimitMetricEvent{policy: policy, result: result})
}

func TestRateLimitDefaultConfigBlocksAfterLimit(t *testing.T) {
	router, metrics := newRateLimitTestRouter(t, RateLimitConfig{
		Enabled:             true,
		BrowserAPIPerMinute: 2,
	})

	if status := performRateLimitRequest(router, nil).Code; status != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", status, http.StatusNoContent)
	}
	if status := performRateLimitRequest(router, nil).Code; status != http.StatusNoContent {
		t.Fatalf("second status = %d, want %d", status, http.StatusNoContent)
	}
	recorder := performRateLimitRequest(router, nil)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("third status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if got := recorder.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want 60", got)
	}
	if !metrics.has("browser_api", "blocked") {
		t.Fatalf("missing blocked metric: %#v", metrics.events)
	}
}

func TestRateLimitResolverOverrideBlocksAfterOverrideLimit(t *testing.T) {
	router, metrics := newRateLimitTestRouter(t, RateLimitConfig{
		Enabled:             true,
		BrowserAPIPerMinute: 50,
		Resolver: func(ctx context.Context, c *gin.Context, policy string, defaultLimit int) (RateLimitDecision, error) {
			return RateLimitDecision{
				Policy:         policy,
				LimitPerMinute: 1,
				BucketKey:      RateLimitBucketKey("tenant_user", "tenant-1", "user-1"),
			}, nil
		},
	})

	if status := performRateLimitRequest(router, nil).Code; status != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", status, http.StatusNoContent)
	}
	recorder := performRateLimitRequest(router, nil)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if !metrics.has("browser_api", "blocked") {
		t.Fatalf("missing blocked metric: %#v", metrics.events)
	}
}

func TestRateLimitDifferentBucketKeysDoNotShareCounters(t *testing.T) {
	router, _ := newRateLimitTestRouter(t, RateLimitConfig{
		Enabled:             true,
		BrowserAPIPerMinute: 50,
		Resolver: func(ctx context.Context, c *gin.Context, policy string, defaultLimit int) (RateLimitDecision, error) {
			return RateLimitDecision{
				Policy:         policy,
				LimitPerMinute: 1,
				BucketKey:      RateLimitBucketKey("tenant_user", "tenant-1", c.GetHeader("X-Requester")),
			}, nil
		},
	})

	if status := performRateLimitRequest(router, map[string]string{"X-Requester": "user-1"}).Code; status != http.StatusNoContent {
		t.Fatalf("user-1 first status = %d, want %d", status, http.StatusNoContent)
	}
	if status := performRateLimitRequest(router, map[string]string{"X-Requester": "user-2"}).Code; status != http.StatusNoContent {
		t.Fatalf("user-2 first status = %d, want %d", status, http.StatusNoContent)
	}
	if status := performRateLimitRequest(router, map[string]string{"X-Requester": "user-1"}).Code; status != http.StatusTooManyRequests {
		t.Fatalf("user-1 second status = %d, want %d", status, http.StatusTooManyRequests)
	}
}

func TestRateLimitResolverErrorFallsBackToDefaultLimit(t *testing.T) {
	router, _ := newRateLimitTestRouter(t, RateLimitConfig{
		Enabled:             true,
		BrowserAPIPerMinute: 2,
		Resolver: func(ctx context.Context, c *gin.Context, policy string, defaultLimit int) (RateLimitDecision, error) {
			return RateLimitDecision{}, errors.New("settings lookup failed")
		},
	})

	if status := performRateLimitRequest(router, nil).Code; status != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", status, http.StatusNoContent)
	}
	if status := performRateLimitRequest(router, nil).Code; status != http.StatusNoContent {
		t.Fatalf("second status = %d, want %d", status, http.StatusNoContent)
	}
	if status := performRateLimitRequest(router, nil).Code; status != http.StatusTooManyRequests {
		t.Fatalf("third status = %d, want %d", status, http.StatusTooManyRequests)
	}
}

func TestRateLimitRedisFailureFailsOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	server.Close()

	metrics := &recordingRateLimitMetrics{}
	router := gin.New()
	router.Use(RateLimit(client, RateLimitConfig{
		Enabled:             true,
		BrowserAPIPerMinute: 1,
	}, metrics))
	router.GET("/api/v1/customer-signals", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	if status := performRateLimitRequest(router, nil).Code; status != http.StatusNoContent {
		t.Fatalf("status = %d, want fail-open %d", status, http.StatusNoContent)
	}
	if !metrics.has("browser_api", "error") {
		t.Fatalf("missing error metric: %#v", metrics.events)
	}
}

func TestRateLimitBucketKeyHidesRawValues(t *testing.T) {
	key := RateLimitBucketKey("tenant_user", "tenant-1", "user-1")
	if !strings.HasPrefix(key, "tenant_user:") {
		t.Fatalf("key = %q, want tenant_user prefix", key)
	}
	if strings.Contains(key, "tenant-1") || strings.Contains(key, "user-1") {
		t.Fatalf("key exposes raw values: %q", key)
	}
}

func newRateLimitTestRouter(t *testing.T, cfg RateLimitConfig) (*gin.Engine, *recordingRateLimitMetrics) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	metrics := &recordingRateLimitMetrics{}
	router := gin.New()
	router.Use(RateLimit(client, cfg, metrics))
	router.GET("/api/v1/customer-signals", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return router, metrics
}

func performRateLimitRequest(router *gin.Engine, headers map[string]string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/customer-signals", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func (m *recordingRateLimitMetrics) has(policy, result string) bool {
	for _, event := range m.events {
		if event.policy == policy && event.result == result {
			return true
		}
	}
	return false
}
