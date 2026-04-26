package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/haohao/backend/internal/platform"

	"github.com/gin-gonic/gin"
)

func TestRegisterHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterHealthRoutes(router, platform.ReadinessChecker{
		PostgresPing: func(context.Context) error { return nil },
		RedisPing:    func(context.Context) error { return nil },
	}, time.Second)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("/healthz status = %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("/readyz status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"postgres":{"status":"ok"}`) {
		t.Fatalf("/readyz body = %s", recorder.Body.String())
	}
}

func TestRegisterHealthRoutesReadinessFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterHealthRoutes(router, platform.ReadinessChecker{}, time.Second)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestRegisterHealthRoutesOpenFGAFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	openFGAServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer openFGAServer.Close()

	router := gin.New()
	RegisterHealthRoutes(router, platform.ReadinessChecker{
		PostgresPing: func(context.Context) error { return nil },
		RedisPing:    func(context.Context) error { return nil },
		OpenFGAURL:   openFGAServer.URL,
		CheckOpenFGA: true,
		HTTPClient:   platform.ReadinessTimeoutClient(time.Second),
	}, time.Second)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"openfga":{"status":"error"`) {
		t.Fatalf("/readyz body = %s", recorder.Body.String())
	}
}
