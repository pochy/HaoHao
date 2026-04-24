package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppName       string
	AppVersion    string
	HTTPPort      int
	DatabaseURL   string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	SessionTTL    time.Duration
	CookieSecure  bool
}

func Load() (Config, error) {
	sessionTTL, err := time.ParseDuration(getEnv("SESSION_TTL", "24h"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppName:       getEnv("APP_NAME", "HaoHao API"),
		AppVersion:    getEnv("APP_VERSION", "0.1.0"),
		HTTPPort:      getEnvInt("HTTP_PORT", 8080),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),
		SessionTTL:    sessionTTL,
		CookieSecure:  getEnvBool("COOKIE_SECURE", false),
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

