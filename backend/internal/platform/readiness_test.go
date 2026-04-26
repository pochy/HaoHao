package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadinessCheckerCheckSuccess(t *testing.T) {
	checker := ReadinessChecker{
		PostgresPing: func(context.Context) error { return nil },
		RedisPing:    func(context.Context) error { return nil },
	}

	result := checker.Check(context.Background())
	if result.Status != "ok" {
		t.Fatalf("Status = %q", result.Status)
	}
	if result.Checks["postgres"].Status != "ok" || result.Checks["redis"].Status != "ok" {
		t.Fatalf("Checks = %#v", result.Checks)
	}
}

func TestReadinessCheckerCheckFailure(t *testing.T) {
	checker := ReadinessChecker{
		PostgresPing: func(context.Context) error { return errors.New("postgres down") },
		RedisPing:    func(context.Context) error { return nil },
	}

	result := checker.Check(context.Background())
	if result.Status != "error" {
		t.Fatalf("Status = %q", result.Status)
	}
	if result.Checks["postgres"].Error == "" {
		t.Fatalf("postgres error is empty: %#v", result.Checks["postgres"])
	}
}

func TestReadinessCheckerCheckZitadel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"` + serverURL(r) + `"}`))
	}))
	defer server.Close()

	checker := ReadinessChecker{
		PostgresPing:  func(context.Context) error { return nil },
		RedisPing:     func(context.Context) error { return nil },
		ZitadelIssuer: server.URL,
		CheckZitadel:  true,
		HTTPClient:    server.Client(),
	}

	result := checker.Check(context.Background())
	if result.Status != "ok" {
		t.Fatalf("Status = %q, checks = %#v", result.Status, result.Checks)
	}
	if result.Checks["zitadel"].Status != "ok" {
		t.Fatalf("zitadel check = %#v", result.Checks["zitadel"])
	}
}

func TestReadinessCheckerOpenFGAOK(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result := ReadinessChecker{
		PostgresPing: func(context.Context) error { return nil },
		RedisPing:    func(context.Context) error { return nil },
		OpenFGAURL:   server.URL + "/",
		OpenFGAToken: "test-token",
		CheckOpenFGA: true,
		HTTPClient:   ReadinessTimeoutClient(time.Second),
	}.Check(context.Background())

	if result.Status != "ok" {
		t.Fatalf("Status = %q checks = %#v", result.Status, result.Checks)
	}
	if result.Checks["openfga"].Status != "ok" {
		t.Fatalf("openfga check = %#v", result.Checks["openfga"])
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}

func TestReadinessCheckerOpenFGAFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	result := ReadinessChecker{
		PostgresPing: func(context.Context) error { return nil },
		RedisPing:    func(context.Context) error { return nil },
		OpenFGAURL:   server.URL,
		CheckOpenFGA: true,
		HTTPClient:   ReadinessTimeoutClient(time.Second),
	}.Check(context.Background())

	if result.Status != "error" {
		t.Fatalf("Status = %q checks = %#v", result.Status, result.Checks)
	}
	if result.Checks["openfga"].Status != "error" {
		t.Fatalf("openfga check = %#v", result.Checks["openfga"])
	}
}

func serverURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
