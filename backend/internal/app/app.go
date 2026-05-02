package app

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

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

type MarkdownDocsFS struct {
	FS fs.FS
}

func New(cfg config.Config, logger *slog.Logger, sessionService *service.SessionService, oidcLoginService *service.OIDCLoginService, delegationService *service.DelegationService, provisioningService *service.ProvisioningService, authzService *service.AuthzService, auditService *service.AuditService, tenantAdminService *service.TenantAdminService, customerSignalService *service.CustomerSignalService, todoService *service.TodoService, machineClientService *service.MachineClientService, outboxService *service.OutboxService, idempotencyService *service.IdempotencyService, notificationService *service.NotificationService, tenantInvitationService *service.TenantInvitationService, fileService *service.FileService, tenantSettingsService *service.TenantSettingsService, tenantDataExportService *service.TenantDataExportService, bearerVerifier *auth.BearerVerifier, m2mVerifier *auth.M2MVerifier, redisClient *redis.Client, metrics *platform.Metrics, extras ...any) *App {
	var entitlementService *service.EntitlementService
	var webhookService *service.WebhookService
	var customerSignalImportService *service.CustomerSignalImportService
	var customerSignalSavedFilterService *service.CustomerSignalSavedFilterService
	var supportAccessService *service.SupportAccessService
	var driveService *service.DriveService
	var driveOCRService *service.DriveOCRService
	var datasetService *service.DatasetService
	var medallionCatalogService *service.MedallionCatalogService
	var realtimeService *service.RealtimeService
	markdownDocsFS := fs.FS(os.DirFS("docs"))
	for _, extra := range extras {
		switch item := extra.(type) {
		case *service.EntitlementService:
			entitlementService = item
		case *service.WebhookService:
			webhookService = item
		case *service.CustomerSignalImportService:
			customerSignalImportService = item
		case *service.CustomerSignalSavedFilterService:
			customerSignalSavedFilterService = item
		case *service.SupportAccessService:
			supportAccessService = item
		case *service.DriveService:
			driveService = item
		case *service.DriveOCRService:
			driveOCRService = item
		case *service.DatasetService:
			datasetService = item
		case *service.MedallionCatalogService:
			medallionCatalogService = item
		case *service.RealtimeService:
			realtimeService = item
		case MarkdownDocsFS:
			if item.FS != nil {
				markdownDocsFS = item.FS
			}
		}
	}
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
		middleware.BodyLimitWithOverrides(cfg.MaxRequestBodyBytes, []middleware.BodyLimitOverride{
			{Method: "POST", PathPrefix: "/api/v1/drive/files", MaxBytes: maxInt64(cfg.FileMaxBytes, cfg.DatasetMaxUploadBytes) + 1024*1024},
			{Method: "PUT", PathPrefix: "/api/v1/drive/files", MaxBytes: maxInt64(cfg.FileMaxBytes, cfg.DatasetMaxUploadBytes) + 1024*1024},
		}),
		middleware.BrowserCORS(cfg.CORSAllowedOrigins),
	}
	if cfg.OTELTracingEnabled {
		handlers = append(handlers, middleware.Trace(cfg.OTELServiceName))
	}
	if cfg.MetricsEnabled && metrics != nil {
		handlers = append(handlers, metrics.HTTPMiddleware(cfg.MetricsPath))
		router.GET(cfg.MetricsPath, gin.WrapH(metrics.Handler()))
	}
	rateLimitDefaults := service.RateLimitDefaults{
		LoginPerMinute:       cfg.RateLimitLoginPerMinute,
		BrowserAPIPerMinute:  cfg.RateLimitBrowserAPIPerMinute,
		ExternalAPIPerMinute: cfg.RateLimitExternalAPIPerMinute,
	}
	handlers = append(handlers,
		middleware.RateLimit(redisClient, middleware.RateLimitConfig{
			Enabled:              cfg.RateLimitEnabled,
			LoginPerMinute:       cfg.RateLimitLoginPerMinute,
			BrowserAPIPerMinute:  cfg.RateLimitBrowserAPIPerMinute,
			ExternalAPIPerMinute: cfg.RateLimitExternalAPIPerMinute,
			Resolver:             browserAPIRateLimitResolver(sessionService, tenantSettingsService, rateLimitDefaults),
		}, metrics),
		middleware.RequestLogger(logger),
		middleware.Recovery(logger),
		middleware.DocsAuth(cfg.DocsAuthRequired, sessionService, authzService),
		middleware.MarkdownDocs(middleware.MarkdownDocsConfig{FS: markdownDocsFS}),
		middleware.ExternalCORS("/api/external/", cfg.ExternalAllowedOrigins),
		middleware.ExternalAuth("/api/external/", bearerVerifier, authzService, "zitadel", cfg.ExternalExpectedAudience, cfg.ExternalRequiredScopePrefix, cfg.ExternalRequiredRole, metrics),
		middleware.M2MAuth("/api/m2m/", m2mVerifier, machineClientService, "zitadel", metrics),
		middleware.SCIMAuth(cfg.SCIMBasePath+"/", bearerVerifier, cfg.SCIMBearerAudience, cfg.SCIMRequiredScope, metrics),
	)
	router.Use(handlers...)

	api := humagin.New(router, humaConfigForSurface(cfg, backendapi.SurfaceFull))

	deps := dependenciesWithConfig(cfg, backendapi.Dependencies{
		Logger:                           logger,
		SessionService:                   sessionService,
		OIDCLoginService:                 oidcLoginService,
		DelegationService:                delegationService,
		ProvisioningService:              provisioningService,
		AuthzService:                     authzService,
		AuditService:                     auditService,
		TenantAdminService:               tenantAdminService,
		CustomerSignalService:            customerSignalService,
		TodoService:                      todoService,
		MachineClientService:             machineClientService,
		OutboxService:                    outboxService,
		IdempotencyService:               idempotencyService,
		NotificationService:              notificationService,
		RealtimeService:                  realtimeService,
		TenantInvitationService:          tenantInvitationService,
		FileService:                      fileService,
		DriveService:                     driveService,
		DriveOCRService:                  driveOCRService,
		TenantSettingsService:            tenantSettingsService,
		TenantDataExportService:          tenantDataExportService,
		EntitlementService:               entitlementService,
		WebhookService:                   webhookService,
		CustomerSignalImportService:      customerSignalImportService,
		CustomerSignalSavedFilterService: customerSignalSavedFilterService,
		SupportAccessService:             supportAccessService,
		DatasetService:                   datasetService,
		MedallionCatalogService:          medallionCatalogService,
	})
	backendapi.Register(api, deps)
	backendapi.RegisterRawFileRoutes(router, backendapi.Dependencies{
		Logger:                logger,
		SessionService:        sessionService,
		AuthzService:          authzService,
		FileService:           fileService,
		TenantAdminService:    tenantAdminService,
		CustomerSignalService: customerSignalService,
		TodoService:           todoService,
	}, cfg.FileMaxBytes)
	backendapi.RegisterRawDriveRoutes(router, deps, cfg.FileMaxBytes, cfg.DatasetMaxUploadBytes)
	backendapi.RegisterRawRealtimeRoutes(router, deps)

	return &App{
		Router: router,
		API:    api,
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func browserAPIRateLimitResolver(sessionService *service.SessionService, tenantSettingsService *service.TenantSettingsService, defaults service.RateLimitDefaults) middleware.RateLimitResolver {
	return func(ctx context.Context, c *gin.Context, policy string, defaultLimit int) (middleware.RateLimitDecision, error) {
		if policy != "browser_api" || sessionService == nil || tenantSettingsService == nil {
			return defaultRateLimitDecision(c, policy, defaultLimit), nil
		}

		sessionCookie, err := c.Request.Cookie(auth.SessionCookieName)
		if err != nil || strings.TrimSpace(sessionCookie.Value) == "" {
			return defaultRateLimitDecision(c, policy, defaultLimit), nil
		}

		current, err := sessionService.CurrentSession(ctx, sessionCookie.Value)
		if err != nil || current.ActiveTenantID == nil {
			return defaultRateLimitDecision(c, policy, defaultLimit), nil
		}

		lookupCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()

		limit, err := tenantSettingsService.ResolveEffectiveRateLimit(lookupCtx, *current.ActiveTenantID, policy, defaults)
		if err != nil || limit <= 0 {
			limit = defaultLimit
		}

		return middleware.RateLimitDecision{
			Policy:         policy,
			LimitPerMinute: limit,
			BucketKey: middleware.RateLimitBucketKey(
				"tenant_user",
				strconv.FormatInt(*current.ActiveTenantID, 10),
				strconv.FormatInt(rateLimitRequesterID(current), 10),
			),
		}, nil
	}
}

func defaultRateLimitDecision(c *gin.Context, policy string, defaultLimit int) middleware.RateLimitDecision {
	return middleware.RateLimitDecision{
		Policy:         policy,
		LimitPerMinute: defaultLimit,
		BucketKey:      middleware.RateLimitBucketKey("ip", c.ClientIP()),
	}
}

func rateLimitRequesterID(current service.CurrentSession) int64 {
	if current.ActorUser != nil {
		return current.ActorUser.ID
	}
	return current.User.ID
}
