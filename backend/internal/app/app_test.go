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

func TestExternalHealthEndpoint(t *testing.T) {
	application, err := New(config.Load())
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/external/v1/health", nil)
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
		t.Fatalf("decode external health response: %v", err)
	}

	if body.Status != "ok" {
		t.Fatalf("status body = %q, want %q", body.Status, "ok")
	}
	if body.Service != "haohao-external-api" {
		t.Fatalf("service body = %q, want %q", body.Service, "haohao-external-api")
	}
}

func TestOpenAPISecuritySchemesAreSeparatedByAPI(t *testing.T) {
	application, err := New(config.Load())
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	spec := application.API.OpenAPI()
	if spec.Components == nil {
		t.Fatal("components = nil, want security schemes")
	}

	if _, ok := spec.Components.SecuritySchemes["cookieAuth"]; !ok {
		t.Fatal("cookieAuth security scheme missing")
	}
	if _, ok := spec.Components.SecuritySchemes["bearerAuth"]; !ok {
		t.Fatal("bearerAuth security scheme missing")
	}

	sessionPath := spec.Paths["/api/v1/session"]
	if sessionPath == nil || sessionPath.Get == nil {
		t.Fatal("session operation missing from spec")
	}
	if len(sessionPath.Get.Security) != 1 || len(sessionPath.Get.Security[0]["cookieAuth"]) != 0 {
		t.Fatalf("session security = %#v, want cookieAuth only", sessionPath.Get.Security)
	}
	if _, ok := sessionPath.Get.Security[0]["bearerAuth"]; ok {
		t.Fatalf("session security = %#v, should not include bearerAuth", sessionPath.Get.Security)
	}

	externalPath := spec.Paths["/external/v1/health"]
	if externalPath == nil || externalPath.Get == nil {
		t.Fatal("external health operation missing from spec")
	}
	if len(externalPath.Get.Security) != 1 || len(externalPath.Get.Security[0]["bearerAuth"]) != 0 {
		t.Fatalf("external health security = %#v, want bearerAuth only", externalPath.Get.Security)
	}
	if _, ok := externalPath.Get.Security[0]["cookieAuth"]; ok {
		t.Fatalf("external health security = %#v, should not include cookieAuth", externalPath.Get.Security)
	}
}
