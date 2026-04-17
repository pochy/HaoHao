package config

import "os"

type Config struct {
	Address           string
	AppEnv            string
	Version           string
	DatabaseURL       string
	RedisURL          string
	ZitadelIssuerURL  string
	SessionCookieName string
	CSRFCookieName    string
	DocsBearerToken   string
}

func Load() Config {
	return Config{
		Address:           envString("APP_ADDRESS", ":8080"),
		AppEnv:            envString("APP_ENV", "development"),
		Version:           envString("APP_VERSION", "0.1.0"),
		DatabaseURL:       envString("DATABASE_URL", "postgres://haohao:haohao@localhost:5432/haohao?sslmode=disable"),
		RedisURL:          envString("REDIS_URL", "redis://localhost:6379/0"),
		ZitadelIssuerURL:  envString("ZITADEL_ISSUER_URL", ""),
		SessionCookieName: envString("SESSION_COOKIE_NAME", "SESSION_ID"),
		CSRFCookieName:    envString("CSRF_COOKIE_NAME", "XSRF-TOKEN"),
		DocsBearerToken:   envString("DOCS_BEARER_TOKEN", ""),
	}
}

func envString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

