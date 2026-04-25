package app

import (
	"io"
	"log/slog"

	backendapi "example.com/haohao/backend/internal/api"
	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/config"
	"example.com/haohao/backend/internal/middleware"
	"example.com/haohao/backend/internal/platform"
	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type App struct {
	Router *gin.Engine
	API    huma.API
}

func New(cfg config.Config, logger *slog.Logger, sessionService *service.SessionService, oidcLoginService *service.OIDCLoginService, delegationService *service.DelegationService, provisioningService *service.ProvisioningService, authzService *service.AuthzService, auditService *service.AuditService, tenantAdminService *service.TenantAdminService, customerSignalService *service.CustomerSignalService, todoService *service.TodoService, machineClientService *service.MachineClientService, outboxService *service.OutboxService, idempotencyService *service.IdempotencyService, notificationService *service.NotificationService, tenantInvitationService *service.TenantInvitationService, fileService *service.FileService, tenantSettingsService *service.TenantSettingsService, tenantDataExportService *service.TenantDataExportService, bearerVerifier *auth.BearerVerifier, m2mVerifier *auth.M2MVerifier, redisClient *redis.Client, metrics *platform.Metrics) *App {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	router := gin.New()
	trustedProxies := cfg.TrustedProxyCIDRs
	if len(trustedProxies) == 0 {
		trustedProxies = []string{"127.0.0.1", "::1"}
	}
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		logger.Warn("failed to set trusted proxies", "error", err)
	}
	handlers := []gin.HandlerFunc{
		middleware.RequestID(),
		middleware.SecurityHeaders(middleware.SecurityHeadersConfig{
			Enabled:     cfg.SecurityHeadersEnabled,
			CSP:         cfg.SecurityCSP,
			HSTSEnabled: cfg.SecurityHSTSEnabled,
			HSTSMaxAge:  cfg.SecurityHSTSMaxAge,
		}),
		middleware.BodyLimit(cfg.MaxRequestBodyBytes),
		middleware.BrowserCORS(cfg.CORSAllowedOrigins),
	}
	if cfg.OTELTracingEnabled {
		handlers = append(handlers, middleware.Trace(cfg.OTELServiceName))
	}
	if cfg.MetricsEnabled && metrics != nil {
		handlers = append(handlers, metrics.HTTPMiddleware(cfg.MetricsPath))
		router.GET(cfg.MetricsPath, gin.WrapH(metrics.Handler()))
	}
	handlers = append(handlers,
		middleware.RateLimit(redisClient, middleware.RateLimitConfig{
			Enabled:              cfg.RateLimitEnabled,
			LoginPerMinute:       cfg.RateLimitLoginPerMinute,
			BrowserAPIPerMinute:  cfg.RateLimitBrowserAPIPerMinute,
			ExternalAPIPerMinute: cfg.RateLimitExternalAPIPerMinute,
		}, metrics),
		middleware.RequestLogger(logger),
		gin.Recovery(),
		middleware.DocsAuth(cfg.DocsAuthRequired, sessionService, authzService),
		middleware.ExternalCORS("/api/external/", cfg.ExternalAllowedOrigins),
		middleware.ExternalAuth("/api/external/", bearerVerifier, authzService, "zitadel", cfg.ExternalExpectedAudience, cfg.ExternalRequiredScopePrefix, cfg.ExternalRequiredRole, metrics),
		middleware.M2MAuth("/api/m2m/", m2mVerifier, machineClientService, "zitadel", metrics),
		middleware.SCIMAuth(cfg.SCIMBasePath+"/", bearerVerifier, cfg.SCIMBearerAudience, cfg.SCIMRequiredScope, metrics),
	)
	router.Use(handlers...)

	api := humagin.New(router, humaConfigForSurface(cfg, backendapi.SurfaceFull))

	deps := dependenciesWithConfig(cfg, backendapi.Dependencies{
		SessionService:          sessionService,
		OIDCLoginService:        oidcLoginService,
		DelegationService:       delegationService,
		ProvisioningService:     provisioningService,
		AuthzService:            authzService,
		AuditService:            auditService,
		TenantAdminService:      tenantAdminService,
		CustomerSignalService:   customerSignalService,
		TodoService:             todoService,
		MachineClientService:    machineClientService,
		OutboxService:           outboxService,
		IdempotencyService:      idempotencyService,
		NotificationService:     notificationService,
		TenantInvitationService: tenantInvitationService,
		FileService:             fileService,
		TenantSettingsService:   tenantSettingsService,
		TenantDataExportService: tenantDataExportService,
	})
	backendapi.Register(api, deps)
	backendapi.RegisterRawFileRoutes(router, backendapi.Dependencies{
		SessionService:        sessionService,
		AuthzService:          authzService,
		FileService:           fileService,
		TenantAdminService:    tenantAdminService,
		CustomerSignalService: customerSignalService,
		TodoService:           todoService,
	}, cfg.FileMaxBytes)

	return &App{
		Router: router,
		API:    api,
	}
}
