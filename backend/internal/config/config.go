package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName                      string
	AppVersion                   string
	HTTPPort                     int
	AppBaseURL                   string
	FrontendBaseURL              string
	DatabaseURL                  string
	AuthMode                     string
	ZitadelIssuer                string
	ZitadelClientID              string
	ZitadelClientSecret          string
	ZitadelRedirectURI           string
	ZitadelPostLogoutRedirectURI string
	ZitadelScopes                string
	RedisAddr                    string
	RedisPassword                string
	RedisDB                      int
	LoginStateTTL                time.Duration
	SessionTTL                   time.Duration
	CookieSecure                 bool
}

func Load() (Config, error) {
	sessionTTL, err := time.ParseDuration(getEnv("SESSION_TTL", "24h"))
	if err != nil {
		return Config{}, err
	}
	loginStateTTL, err := time.ParseDuration(getEnv("LOGIN_STATE_TTL", "10m"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppName:                      getEnv("APP_NAME", "HaoHao API"),
		AppVersion:                   getEnv("APP_VERSION", "0.1.0"),
		HTTPPort:                     getEnvInt("HTTP_PORT", 8080),
		AppBaseURL:                   strings.TrimRight(getEnv("APP_BASE_URL", "http://127.0.0.1:8080"), "/"),
		FrontendBaseURL:              strings.TrimRight(getEnv("FRONTEND_BASE_URL", "http://127.0.0.1:5173"), "/"),
		DatabaseURL:                  getEnv("DATABASE_URL", ""),
		AuthMode:                     getEnv("AUTH_MODE", "local"),
		ZitadelIssuer:                strings.TrimRight(getEnv("ZITADEL_ISSUER", ""), "/"),
		ZitadelClientID:              getEnv("ZITADEL_CLIENT_ID", ""),
		ZitadelClientSecret:          getEnv("ZITADEL_CLIENT_SECRET", ""),
		ZitadelRedirectURI:           getEnv("ZITADEL_REDIRECT_URI", "http://127.0.0.1:8080/api/v1/auth/callback"),
		ZitadelPostLogoutRedirectURI: getEnv("ZITADEL_POST_LOGOUT_REDIRECT_URI", "http://127.0.0.1:5173/login"),
		ZitadelScopes:                getEnv("ZITADEL_SCOPES", "openid profile email"),
		RedisAddr:                    getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:                getEnv("REDIS_PASSWORD", ""),
		RedisDB:                      getEnvInt("REDIS_DB", 0),
		LoginStateTTL:                loginStateTTL,
		SessionTTL:                   sessionTTL,
		CookieSecure:                 getEnvBool("COOKIE_SECURE", false),
	}, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}
