package platform

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMetricsHandlerExportsCollectors(t *testing.T) {
	metrics := NewMetrics("test")
	metrics.ObserveDependencyPing("postgres", time.Millisecond, nil)
	metrics.IncAuthFailure("scim", "missing_token")

	body := scrapeMetrics(t, metrics)
	for _, want := range []string{
		"# HELP haohao_dependency_ping_duration_seconds ",
		"# HELP haohao_auth_failures_total ",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body does not contain %q\n%s", want, body)
		}
	}
}

func TestMetricsHTTPMiddlewareUsesRouteTemplateAndStatusClass(t *testing.T) {
	gin.SetMode(gin.TestMode)

	metrics := NewMetrics("test")
	router := gin.New()
	router.Use(metrics.HTTPMiddleware("/metrics"))
	router.GET("/items/:id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/items/123", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}

	body := scrapeMetrics(t, metrics)
	for _, want := range []string{
		`haohao_http_requests_total{app_version="test",method="GET",route="/items/:id",status_class="2xx"} 1`,
		`haohao_http_request_duration_seconds_count{app_version="test",method="GET",route="/items/:id",status_class="2xx"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body does not contain %q\n%s", want, body)
		}
	}
}

func TestMetricsDependencyPingFailure(t *testing.T) {
	metrics := NewMetrics("test")

	metrics.ObserveDependencyPing("postgres", time.Millisecond, errors.New("postgres down"))

	body := scrapeMetrics(t, metrics)
	for _, want := range []string{
		`haohao_dependency_ping_duration_seconds_count{app_version="test",dependency="postgres",status="error"} 1`,
		`haohao_readiness_failures_total{app_version="test",dependency="postgres"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body does not contain %q\n%s", want, body)
		}
	}
}

func scrapeMetrics(t *testing.T, metrics *Metrics) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d", recorder.Code)
	}

	return recorder.Body.String()
}
