package config

import "testing"

func TestDefaultFrontendBaseURL(t *testing.T) {
	appBaseURL := "http://127.0.0.1:8080"

	if got := defaultFrontendBaseURL(appBaseURL, false); got != "http://127.0.0.1:5173" {
		t.Fatalf("defaultFrontendBaseURL(..., false) = %q", got)
	}
	if got := defaultFrontendBaseURL(appBaseURL, true); got != appBaseURL {
		t.Fatalf("defaultFrontendBaseURL(..., true) = %q", got)
	}
}

func TestResolveFrontendBaseURLForEmbeddedBuild(t *testing.T) {
	appBaseURL := "http://127.0.0.1:8080"

	tests := []struct {
		name       string
		configured string
		want       string
	}{
		{
			name:       "rewrites 127 vite dev default",
			configured: "http://127.0.0.1:5173",
			want:       appBaseURL,
		},
		{
			name:       "rewrites localhost vite dev default",
			configured: "http://localhost:5173",
			want:       appBaseURL,
		},
		{
			name:       "keeps explicit production frontend",
			configured: "https://app.example.com",
			want:       "https://app.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveFrontendBaseURL(appBaseURL, tt.configured, true); got != tt.want {
				t.Fatalf("resolveFrontendBaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveZitadelPostLogoutRedirectURIForEmbeddedBuild(t *testing.T) {
	frontendBaseURL := "http://127.0.0.1:8080"

	tests := []struct {
		name       string
		configured string
		want       string
	}{
		{
			name:       "rewrites vite dev post logout",
			configured: "http://127.0.0.1:5173/login",
			want:       "http://127.0.0.1:8080/login",
		},
		{
			name:       "keeps explicit production post logout",
			configured: "https://app.example.com/login",
			want:       "https://app.example.com/login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveZitadelPostLogoutRedirectURI(frontendBaseURL, tt.configured, true); got != tt.want {
				t.Fatalf("resolveZitadelPostLogoutRedirectURI() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadNormalizesViteFrontendURLForEmbeddedBuild(t *testing.T) {
	if !frontendEmbedded {
		t.Skip("requires embed_frontend build tag")
	}

	restoreEnv(t,
		"APP_BASE_URL",
		"FRONTEND_BASE_URL",
		"ZITADEL_POST_LOGOUT_REDIRECT_URI",
		"DATABASE_URL",
	)

	t.Setenv("APP_BASE_URL", "http://127.0.0.1:18080")
	t.Setenv("FRONTEND_BASE_URL", "http://127.0.0.1:5173")
	t.Setenv("ZITADEL_POST_LOGOUT_REDIRECT_URI", "http://127.0.0.1:5173/login")
	t.Setenv("DATABASE_URL", "postgres://example")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.FrontendBaseURL != "http://127.0.0.1:18080" {
		t.Fatalf("FrontendBaseURL = %q", cfg.FrontendBaseURL)
	}
	if cfg.ZitadelPostLogoutRedirectURI != "http://127.0.0.1:18080/login" {
		t.Fatalf("ZitadelPostLogoutRedirectURI = %q", cfg.ZitadelPostLogoutRedirectURI)
	}
}
