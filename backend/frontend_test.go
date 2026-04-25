package backend

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func TestRegisterFrontendRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		method      string
		path        string
		wantStatus  int
		wantContent string
	}{
		{
			name:        "root returns index",
			method:      http.MethodGet,
			path:        "/",
			wantStatus:  http.StatusOK,
			wantContent: "text/html; charset=utf-8",
		},
		{
			name:        "spa route falls back to index",
			method:      http.MethodGet,
			path:        "/integrations",
			wantStatus:  http.StatusOK,
			wantContent: "text/html; charset=utf-8",
		},
		{
			name:        "static asset is served",
			method:      http.MethodGet,
			path:        "/assets/app.js",
			wantStatus:  http.StatusOK,
			wantContent: "text/javascript; charset=utf-8",
		},
		{
			name:       "missing static asset is not spa fallback",
			method:     http.MethodGet,
			path:       "/assets/missing.js",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "reserved api route is not spa fallback",
			method:     http.MethodGet,
			path:       "/api/v1/session",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "reserved docs route is not spa fallback",
			method:     http.MethodGet,
			path:       "/docs",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "reserved openapi yaml route is not spa fallback",
			method:     http.MethodGet,
			path:       "/openapi.yaml",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "reserved openapi 3 yaml route is not spa fallback",
			method:     http.MethodGet,
			path:       "/openapi-3.0.yaml",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "post is not spa fallback",
			method:     http.MethodPost,
			path:       "/login",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			if err := registerFrontendRoutes(router, testFrontendFS()); err != nil {
				t.Fatalf("registerFrontendRoutes() error = %v", err)
			}

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}

			if tt.wantContent != "" && recorder.Header().Get("Content-Type") != tt.wantContent {
				t.Fatalf("content-type = %q, want %q", recorder.Header().Get("Content-Type"), tt.wantContent)
			}
		})
	}
}

func TestRegisterFrontendRoutesRequiresIndexHTML(t *testing.T) {
	gin.SetMode(gin.TestMode)

	err := registerFrontendRoutes(gin.New(), fstest.MapFS{})
	if err == nil {
		t.Fatal("registerFrontendRoutes() error = nil, want missing index.html error")
	}
}

func testFrontendFS() fs.FS {
	return fstest.MapFS{
		"index.html": {
			Data: []byte("<!doctype html><div id=\"app\"></div>"),
			Mode: 0o644,
		},
		"assets/app.js": {
			Data: []byte("console.log('haohao')"),
			Mode: 0o644,
		},
	}
}
