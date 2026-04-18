package config

import (
	"net/http"
	"testing"
	"time"
)

func TestConfigSessionCookieSameSiteMode(t *testing.T) {
	cfg := Config{
		SessionCookieSameSite: "Strict",
	}

	if got := cfg.SessionCookieSameSiteMode(); got != http.SameSiteStrictMode {
		t.Fatalf("sameSite = %v, want %v", got, http.SameSiteStrictMode)
	}
}

func TestConfigValidateAuthRuntimeRejectsInvalidSameSite(t *testing.T) {
	cfg := Config{
		ZitadelIssuerURL:             "http://localhost:8081",
		ZitadelClientID:              "client-id",
		ZitadelClientSecret:          "client-secret",
		ZitadelRedirectURI:           "http://localhost:8080/auth/callback",
		ZitadelPostLogoutRedirectURI: "http://localhost:8080/auth/logout/callback",
		ZitadelScopes:                []string{"openid"},
		FrontendOrigin:               "http://localhost:5173",
		SessionTTL:                   8 * time.Hour,
		SessionCookieName:            "SESSION_ID",
		SessionCookiePath:            "/",
		SessionCookieSameSite:        "bogus",
	}

	if err := cfg.ValidateAuthRuntime(); err == nil {
		t.Fatal("ValidateAuthRuntime error = nil, want invalid same-site error")
	}
}

func TestDefaultSessionCookieSecure(t *testing.T) {
	if defaultSessionCookieSecure("development") {
		t.Fatal("development default secure = true, want false")
	}
	if !defaultSessionCookieSecure("production") {
		t.Fatal("production default secure = false, want true")
	}
}
