package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/haohao/backend/internal/config"
	"example.com/haohao/backend/internal/platform"

	"github.com/gin-gonic/gin"
)

func TestAppRegistersMetricsRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	metrics := platform.NewMetrics("test")
	application := New(config.Config{
		AppName:        "HaoHao API",
		AppVersion:     "test",
		MetricsEnabled: true,
		MetricsPath:    "/metrics",
		SCIMBasePath:   "/api/scim/v2",
	}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, metrics)

	recorder := httptest.NewRecorder()
	application.Router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil))

	recorder = httptest.NewRecorder()
	application.Router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "# HELP haohao_http_requests_total ") {
		t.Fatalf("/metrics body = %s", recorder.Body.String())
	}
}
