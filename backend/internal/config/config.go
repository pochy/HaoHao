package config

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	Address                      string
	AppEnv                       string
	Version                      string
	DatabaseURL                  string
	RedisURL                     string
	ZitadelIssuerURL             string
	ZitadelClientID              string
	ZitadelClientSecret          string
	ZitadelRedirectURI           string
	ZitadelPostLogoutRedirectURI string
	ZitadelScopes                []string
	FrontendOrigin               string
	SessionTTL                   time.Duration
	SessionCookieName            string
	SessionCookiePath            string
	SessionCookieSameSite        string
	SessionCookieSecure          bool
	CSRFCookieName               string
	DocsBearerToken              string
}

func Load() Config {
	return Config{
		Address:                      envString("APP_ADDRESS", ":8080"),
		AppEnv:                       envString("APP_ENV", "development"),
		Version:                      envString("APP_VERSION", "0.1.0"),
		DatabaseURL:                  envString("DATABASE_URL", "postgres://haohao:haohao@localhost:5432/haohao?sslmode=disable"),
		RedisURL:                     envString("REDIS_URL", "redis://localhost:6379/0"),
		ZitadelIssuerURL:             envString("ZITADEL_ISSUER_URL", "http://localhost:8081"),
		ZitadelClientID:              envString("ZITADEL_CLIENT_ID", ""),
		ZitadelClientSecret:          envString("ZITADEL_CLIENT_SECRET", ""),
		ZitadelRedirectURI:           envString("ZITADEL_REDIRECT_URI", "http://localhost:8080/auth/callback"),
		ZitadelPostLogoutRedirectURI: envString("ZITADEL_POST_LOGOUT_REDIRECT_URI", "http://localhost:8080/auth/logout/callback"),
		ZitadelScopes:                envList("ZITADEL_SCOPES", []string{"openid", "profile", "email"}),
		FrontendOrigin:               envString("FRONTEND_ORIGIN", "http://localhost:5173"),
		SessionTTL:                   envDuration("SESSION_TTL", 8*time.Hour),
		SessionCookieName:            envString("SESSION_COOKIE_NAME", "SESSION_ID"),
		SessionCookiePath:            envString("SESSION_COOKIE_PATH", "/"),
		SessionCookieSameSite:        envString("SESSION_COOKIE_SAME_SITE", "Lax"),
		SessionCookieSecure:          envBool("SESSION_COOKIE_SECURE", defaultSessionCookieSecure(envString("APP_ENV", "development"))),
		CSRFCookieName:               envString("CSRF_COOKIE_NAME", "XSRF-TOKEN"),
		DocsBearerToken:              envString("DOCS_BEARER_TOKEN", ""),
	}
}

func (c Config) ValidateAuthRuntime() error {
	missing := make([]string, 0, 10)
	if c.ZitadelIssuerURL == "" {
		missing = append(missing, "ZITADEL_ISSUER_URL")
	}
	if c.ZitadelClientID == "" {
		missing = append(missing, "ZITADEL_CLIENT_ID")
	}
	if c.ZitadelClientSecret == "" {
		missing = append(missing, "ZITADEL_CLIENT_SECRET")
	}
	if c.ZitadelRedirectURI == "" {
		missing = append(missing, "ZITADEL_REDIRECT_URI")
	}
	if c.ZitadelPostLogoutRedirectURI == "" {
		missing = append(missing, "ZITADEL_POST_LOGOUT_REDIRECT_URI")
	}
	if c.FrontendOrigin == "" {
		missing = append(missing, "FRONTEND_ORIGIN")
	}
	if len(c.ZitadelScopes) == 0 {
		missing = append(missing, "ZITADEL_SCOPES")
	}
	if c.SessionTTL <= 0 {
		missing = append(missing, "SESSION_TTL")
	}
	if c.SessionCookieName == "" {
		missing = append(missing, "SESSION_COOKIE_NAME")
	}
	if c.SessionCookiePath == "" {
		missing = append(missing, "SESSION_COOKIE_PATH")
	}
	if _, err := parseSameSite(c.SessionCookieSameSite); err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required auth env: %s", strings.Join(missing, ", "))
	}

	return nil
}

func (c Config) SessionCookieSameSiteMode() http.SameSite {
	mode, err := parseSameSite(c.SessionCookieSameSite)
	if err != nil {
		return http.SameSiteLaxMode
	}

	return mode
}

func defaultSessionCookieSecure(appEnv string) bool {
	return !strings.EqualFold(appEnv, "development")
}

func parseSameSite(value string) (http.SameSite, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "default":
		return http.SameSiteDefaultMode, nil
	case "lax":
		return http.SameSiteLaxMode, nil
	case "strict":
		return http.SameSiteStrictMode, nil
	case "none":
		return http.SameSiteNoneMode, nil
	default:
		return http.SameSiteDefaultMode, fmt.Errorf("invalid SESSION_COOKIE_SAME_SITE: %q", value)
	}
}

func envString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envList(key string, fallback []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return append([]string(nil), fallback...)
	}

	parts := strings.Fields(value)
	if len(parts) == 0 {
		return append([]string(nil), fallback...)
	}

	return parts
}
