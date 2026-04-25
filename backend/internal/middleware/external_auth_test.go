package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeAuthFailureMetrics struct {
	kind   string
	reason string
	count  int
}

func (m *fakeAuthFailureMetrics) IncAuthFailure(kind, reason string) {
	m.kind = kind
	m.reason = reason
	m.count++
}

func TestSCIMAuthRecordsNotConfiguredMetric(t *testing.T) {
	gin.SetMode(gin.TestMode)

	metrics := &fakeAuthFailureMetrics{}
	router := gin.New()
	router.Use(SCIMAuth("/api/scim/v2/", nil, "audience", "scim:provision", metrics))
	router.GET("/api/scim/v2/Users", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/scim/v2/Users", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
	if metrics.count != 1 || metrics.kind != "scim" || metrics.reason != authFailureNotConfigured {
		t.Fatalf("metrics = %#v", metrics)
	}
}
