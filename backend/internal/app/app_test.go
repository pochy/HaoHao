package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pochy/haohao/backend/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	application, err := New(config.Load())
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Accept", "application/json")

	rec := httptest.NewRecorder()
	application.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if body.Status != "ok" {
		t.Fatalf("status body = %q, want %q", body.Status, "ok")
	}
	if body.Service != "haohao-browser-api" {
		t.Fatalf("service body = %q, want %q", body.Service, "haohao-browser-api")
	}
}

func TestSessionEndpointBootstrapsCSRFCookie(t *testing.T) {
	application, err := New(config.Load())
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	req.Header.Set("Accept", "application/json")

	rec := httptest.NewRecorder()
	application.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	setCookie := strings.Join(rec.Header().Values("Set-Cookie"), "\n")
	if !strings.Contains(setCookie, "XSRF-TOKEN=") {
		t.Fatalf("Set-Cookie = %q, want XSRF-TOKEN cookie", setCookie)
	}

	var body struct {
		Authenticated bool   `json:"authenticated"`
		CSRFCookie    string `json:"csrfCookie"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode session response: %v", err)
	}

	if body.Authenticated {
		t.Fatalf("authenticated = true, want false")
	}
	if body.CSRFCookie != "XSRF-TOKEN" {
		t.Fatalf("csrfCookie = %q, want %q", body.CSRFCookie, "XSRF-TOKEN")
	}
}

