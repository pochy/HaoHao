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

func TestIsDocsPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/docs/openapi", want: true},
		{path: "/docs/openapi.json", want: true},
		{path: "/docs/openapi.yaml", want: true},
		{path: "/docs/openapi-3.0.json", want: true},
		{path: "/docs/openapi-3.0.yaml", want: true},
		{path: "/docs", want: true},
		{path: "/docs/", want: true},
		{path: "/docs/other", want: true},
		{path: "/openapi.json", want: false},
		{path: "/openapi.yaml", want: false},
		{path: "/openapi-3.0.yaml", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isDocsPath(tt.path); got != tt.want {
				t.Fatalf("isDocsPath(%q) = %t, want %t", tt.path, got, tt.want)
			}
		})
	}
}
