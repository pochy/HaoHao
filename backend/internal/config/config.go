package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName                       string
	AppVersion                    string
	HTTPPort                      int
	AppBaseURL                    string
	FrontendBaseURL               string
	LogLevel                      string
	LogFormat                     string
	MetricsEnabled                bool
	MetricsPath                   string
	SecurityHeadersEnabled        bool
	SecurityCSP                   string
	SecurityHSTSEnabled           bool
	SecurityHSTSMaxAge            int
	MaxRequestBodyBytes           int64
	TrustedProxyCIDRs             []string
	CORSAllowedOrigins            []string
	OTELTracingEnabled            bool
	OTELServiceName               string
	OTELExporterOTLPEndpoint      string
	OTELExporterOTLPInsecure      bool
	OTELTraceSampleRatio          float64
	DatabaseURL                   string
	AuthMode                      string
	ZitadelIssuer                 string
	ZitadelClientID               string
	ZitadelClientSecret           string
	ZitadelRedirectURI            string
	ZitadelPostLogoutRedirectURI  string
	ZitadelScopes                 string
	ExternalExpectedAudience      string
	ExternalRequiredScopePrefix   string
	ExternalRequiredRole          string
	ExternalAllowedOrigins        []string
	M2MExpectedAudience           string
	M2MRequiredScopePrefix        string
	DownstreamTokenEncryptionKey  string
	DownstreamTokenKeyVersion     int
	DownstreamRefreshTokenTTL     time.Duration
	DownstreamAccessTokenSkew     time.Duration
	DownstreamDefaultScopes       string
	SCIMBasePath                  string
	SCIMBearerAudience            string
	SCIMRequiredScope             string
	ReadinessTimeout              time.Duration
	ReadinessCheckZitadel         bool
	SCIMReconcileEnabled          bool
	SCIMReconcileInterval         time.Duration
	SCIMReconcileTimeout          time.Duration
	SCIMReconcileRunOnStartup     bool
	OutboxWorkerEnabled           bool
	OutboxWorkerInterval          time.Duration
	OutboxWorkerTimeout           time.Duration
	OutboxWorkerBatchSize         int
	OutboxWorkerMaxAttempts       int
	IdempotencyTTL                time.Duration
	EmailDeliveryMode             string
	EmailFrom                     string
	InvitationTTL                 time.Duration
	FileLocalDir                  string
	FileMaxBytes                  int64
	FileAllowedMIMETypes          []string
	RateLimitEnabled              bool
	RateLimitLoginPerMinute       int
	RateLimitBrowserAPIPerMinute  int
	RateLimitExternalAPIPerMinute int
	TenantDefaultFileQuotaBytes   int64
	DataExportTTL                 time.Duration
	DataLifecycleEnabled          bool
	DataLifecycleInterval         time.Duration
	DataLifecycleTimeout          time.Duration
	DataLifecycleRunOnStartup     bool
	OutboxRetention               time.Duration
	NotificationRetention         time.Duration
	FileDeletedRetention          time.Duration
	RedisAddr                     string
	RedisPassword                 string
	RedisDB                       int
	LoginStateTTL                 time.Duration
	SessionTTL                    time.Duration
	CookieSecure                  bool
	DocsAuthRequired              bool
	EnableLocalPasswordLogin      bool
}

func Load() (Config, error) {
	if err := loadDotEnvFiles(); err != nil {
		return Config{}, err
	}

	sessionTTL, err := time.ParseDuration(getEnv("SESSION_TTL", "24h"))
	if err != nil {
		return Config{}, err
	}
	loginStateTTL, err := time.ParseDuration(getEnv("LOGIN_STATE_TTL", "10m"))
	if err != nil {
		return Config{}, err
	}
	downstreamRefreshTokenTTL, err := time.ParseDuration(getEnv("DOWNSTREAM_REFRESH_TOKEN_TTL", "2160h"))
	if err != nil {
		return Config{}, err
	}
	downstreamAccessTokenSkew, err := time.ParseDuration(getEnv("DOWNSTREAM_ACCESS_TOKEN_SKEW", "30s"))
	if err != nil {
		return Config{}, err
	}
	readinessTimeout, err := getEnvPositiveDuration("READINESS_TIMEOUT", "2s")
	if err != nil {
		return Config{}, err
	}
	scimReconcileInterval, err := getEnvPositiveDuration("SCIM_RECONCILE_INTERVAL", "1h")
	if err != nil {
		return Config{}, err
	}
	scimReconcileTimeout, err := getEnvPositiveDuration("SCIM_RECONCILE_TIMEOUT", "30s")
	if err != nil {
		return Config{}, err
	}
	outboxWorkerInterval, err := getEnvPositiveDuration("OUTBOX_WORKER_INTERVAL", "5s")
	if err != nil {
		return Config{}, err
	}
	outboxWorkerTimeout, err := getEnvPositiveDuration("OUTBOX_WORKER_TIMEOUT", "10s")
	if err != nil {
		return Config{}, err
	}
	idempotencyTTL, err := getEnvPositiveDuration("IDEMPOTENCY_TTL", "24h")
	if err != nil {
		return Config{}, err
	}
	invitationTTL, err := getEnvPositiveDuration("INVITATION_TTL", "168h")
	if err != nil {
		return Config{}, err
	}
	dataExportTTL, err := getEnvPositiveDuration("DATA_EXPORT_TTL", "168h")
	if err != nil {
		return Config{}, err
	}
	dataLifecycleInterval, err := getEnvPositiveDuration("DATA_LIFECYCLE_INTERVAL", "1h")
	if err != nil {
		return Config{}, err
	}
	dataLifecycleTimeout, err := getEnvPositiveDuration("DATA_LIFECYCLE_TIMEOUT", "30s")
	if err != nil {
		return Config{}, err
	}
	outboxRetention, err := getEnvPositiveDuration("OUTBOX_RETENTION", "720h")
	if err != nil {
		return Config{}, err
	}
	notificationRetention, err := getEnvPositiveDuration("NOTIFICATION_RETENTION", "4320h")
	if err != nil {
		return Config{}, err
	}
	fileDeletedRetention, err := getEnvPositiveDuration("FILE_DELETED_RETENTION", "720h")
	if err != nil {
		return Config{}, err
	}

	appBaseURL := strings.TrimRight(getEnv("APP_BASE_URL", "http://127.0.0.1:8080"), "/")
	frontendBaseURL := resolveFrontendBaseURL(appBaseURL, getEnv("FRONTEND_BASE_URL", defaultFrontendBaseURL(appBaseURL, frontendEmbedded)), frontendEmbedded)
	zitadelPostLogoutRedirectURI := resolveZitadelPostLogoutRedirectURI(frontendBaseURL, getEnv("ZITADEL_POST_LOGOUT_REDIRECT_URI", defaultZitadelPostLogoutRedirectURI(frontendBaseURL)), frontendEmbedded)
	metricsPath := normalizePath(getEnv("METRICS_PATH", "/metrics"), "/metrics")
	otelTraceSampleRatio := clampFloat64(getEnvFloat64("OTEL_TRACES_SAMPLER_RATIO", 0.1), 0, 1)

	return Config{
		AppName:                       getEnv("APP_NAME", "HaoHao API"),
		AppVersion:                    getEnv("APP_VERSION", "0.1.0"),
		HTTPPort:                      getEnvInt("HTTP_PORT", 8080),
		AppBaseURL:                    appBaseURL,
		FrontendBaseURL:               frontendBaseURL,
		LogLevel:                      getEnv("LOG_LEVEL", "info"),
		LogFormat:                     getEnv("LOG_FORMAT", "json"),
		MetricsEnabled:                getEnvBool("METRICS_ENABLED", true),
		MetricsPath:                   metricsPath,
		SecurityHeadersEnabled:        getEnvBool("SECURITY_HEADERS_ENABLED", true),
		SecurityCSP:                   getEnv("SECURITY_CSP", "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; object-src 'none'"),
		SecurityHSTSEnabled:           getEnvBool("SECURITY_HSTS_ENABLED", false),
		SecurityHSTSMaxAge:            positiveInt(getEnvInt("SECURITY_HSTS_MAX_AGE", 31536000), 31536000),
		MaxRequestBodyBytes:           positiveInt64(getEnvInt64("MAX_REQUEST_BODY_BYTES", 10485760), 10485760),
		TrustedProxyCIDRs:             getEnvCSV("TRUSTED_PROXY_CIDRS"),
		CORSAllowedOrigins:            getEnvCSV("CORS_ALLOWED_ORIGINS"),
		OTELTracingEnabled:            getEnvBool("OTEL_TRACING_ENABLED", false),
		OTELServiceName:               getEnv("OTEL_SERVICE_NAME", "haohao"),
		OTELExporterOTLPEndpoint:      getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTELExporterOTLPInsecure:      getEnvBool("OTEL_EXPORTER_OTLP_INSECURE", true),
		OTELTraceSampleRatio:          otelTraceSampleRatio,
		DatabaseURL:                   getEnv("DATABASE_URL", ""),
		AuthMode:                      getEnv("AUTH_MODE", "local"),
		ZitadelIssuer:                 strings.TrimRight(getEnv("ZITADEL_ISSUER", ""), "/"),
		ZitadelClientID:               getEnv("ZITADEL_CLIENT_ID", ""),
		ZitadelClientSecret:           getEnv("ZITADEL_CLIENT_SECRET", ""),
		ZitadelRedirectURI:            getEnv("ZITADEL_REDIRECT_URI", "http://127.0.0.1:8080/api/v1/auth/callback"),
		ZitadelPostLogoutRedirectURI:  zitadelPostLogoutRedirectURI,
		ZitadelScopes:                 getEnv("ZITADEL_SCOPES", "openid profile email"),
		ExternalExpectedAudience:      getEnv("EXTERNAL_EXPECTED_AUDIENCE", "haohao-external"),
		ExternalRequiredScopePrefix:   getEnv("EXTERNAL_REQUIRED_SCOPE_PREFIX", ""),
		ExternalRequiredRole:          getEnv("EXTERNAL_REQUIRED_ROLE", "external_api_user"),
		ExternalAllowedOrigins:        getEnvCSV("EXTERNAL_ALLOWED_ORIGINS"),
		M2MExpectedAudience:           getEnv("M2M_EXPECTED_AUDIENCE", "haohao-m2m"),
		M2MRequiredScopePrefix:        getEnv("M2M_REQUIRED_SCOPE_PREFIX", "m2m:"),
		DownstreamTokenEncryptionKey:  getEnv("DOWNSTREAM_TOKEN_ENCRYPTION_KEY", ""),
		DownstreamTokenKeyVersion:     getEnvInt("DOWNSTREAM_TOKEN_KEY_VERSION", 1),
		DownstreamRefreshTokenTTL:     downstreamRefreshTokenTTL,
		DownstreamAccessTokenSkew:     downstreamAccessTokenSkew,
		DownstreamDefaultScopes:       getEnv("DOWNSTREAM_DEFAULT_SCOPES", "offline_access"),
		SCIMBasePath:                  strings.TrimRight(getEnv("SCIM_BASE_PATH", "/api/scim/v2"), "/"),
		SCIMBearerAudience:            getEnv("SCIM_BEARER_AUDIENCE", "scim-provisioning"),
		SCIMRequiredScope:             getEnv("SCIM_REQUIRED_SCOPE", "scim:provision"),
		ReadinessTimeout:              readinessTimeout,
		ReadinessCheckZitadel:         getEnvBool("READINESS_CHECK_ZITADEL", false),
		SCIMReconcileEnabled:          getEnvBool("SCIM_RECONCILE_ENABLED", false),
		SCIMReconcileInterval:         scimReconcileInterval,
		SCIMReconcileTimeout:          scimReconcileTimeout,
		SCIMReconcileRunOnStartup:     getEnvBool("SCIM_RECONCILE_RUN_ON_STARTUP", false),
		OutboxWorkerEnabled:           getEnvBool("OUTBOX_WORKER_ENABLED", true),
		OutboxWorkerInterval:          outboxWorkerInterval,
		OutboxWorkerTimeout:           outboxWorkerTimeout,
		OutboxWorkerBatchSize:         positiveInt(getEnvInt("OUTBOX_WORKER_BATCH_SIZE", 20), 20),
		OutboxWorkerMaxAttempts:       positiveInt(getEnvInt("OUTBOX_WORKER_MAX_ATTEMPTS", 8), 8),
		IdempotencyTTL:                idempotencyTTL,
		EmailDeliveryMode:             strings.ToLower(strings.TrimSpace(getEnv("EMAIL_DELIVERY_MODE", "log"))),
		EmailFrom:                     getEnv("EMAIL_FROM", "no-reply@example.com"),
		InvitationTTL:                 invitationTTL,
		FileLocalDir:                  getEnv("FILE_LOCAL_DIR", ".data/files"),
		FileMaxBytes:                  positiveInt64(getEnvInt64("FILE_MAX_BYTES", 10485760), 10485760),
		FileAllowedMIMETypes:          getEnvCSV("FILE_ALLOWED_MIME_TYPES"),
		RateLimitEnabled:              getEnvBool("RATE_LIMIT_ENABLED", true),
		RateLimitLoginPerMinute:       positiveInt(getEnvInt("RATE_LIMIT_LOGIN_PER_MINUTE", 20), 20),
		RateLimitBrowserAPIPerMinute:  positiveInt(getEnvInt("RATE_LIMIT_BROWSER_API_PER_MINUTE", 120), 120),
		RateLimitExternalAPIPerMinute: positiveInt(getEnvInt("RATE_LIMIT_EXTERNAL_API_PER_MINUTE", 120), 120),
		TenantDefaultFileQuotaBytes:   positiveInt64(getEnvInt64("TENANT_DEFAULT_FILE_QUOTA_BYTES", 104857600), 104857600),
		DataExportTTL:                 dataExportTTL,
		DataLifecycleEnabled:          getEnvBool("DATA_LIFECYCLE_ENABLED", true),
		DataLifecycleInterval:         dataLifecycleInterval,
		DataLifecycleTimeout:          dataLifecycleTimeout,
		DataLifecycleRunOnStartup:     getEnvBool("DATA_LIFECYCLE_RUN_ON_STARTUP", false),
		OutboxRetention:               outboxRetention,
		NotificationRetention:         notificationRetention,
		FileDeletedRetention:          fileDeletedRetention,
		RedisAddr:                     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:                 getEnv("REDIS_PASSWORD", ""),
		RedisDB:                       getEnvInt("REDIS_DB", 0),
		LoginStateTTL:                 loginStateTTL,
		SessionTTL:                    sessionTTL,
		CookieSecure:                  getEnvBool("COOKIE_SECURE", false),
		DocsAuthRequired:              getEnvBool("DOCS_AUTH_REQUIRED", false),
		EnableLocalPasswordLogin:      getEnvBool("ENABLE_LOCAL_PASSWORD_LOGIN", true),
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

func getEnvInt64(key string, fallback int64) int64 {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}

	return parsed
}

func positiveInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func positiveInt64(value, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func getEnvFloat64(key string, fallback float64) float64 {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}

	return parsed
}

func clampFloat64(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}

	return value
}

func getEnvPositiveDuration(key, fallback string) (time.Duration, error) {
	value := getEnv(key, fallback)
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}

	return parsed, nil
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

func getEnvCSV(key string) []string {
	value := strings.TrimSpace(getEnv(key, ""))
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}

func normalizePath(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}

	return trimmed
}
