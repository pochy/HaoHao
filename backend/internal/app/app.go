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
)

type App struct {
	Router *gin.Engine
	API    huma.API
}

func New(cfg config.Config, logger *slog.Logger, sessionService *service.SessionService, oidcLoginService *service.OIDCLoginService, delegationService *service.DelegationService, provisioningService *service.ProvisioningService, authzService *service.AuthzService, auditService *service.AuditService, tenantAdminService *service.TenantAdminService, todoService *service.TodoService, machineClientService *service.MachineClientService, bearerVerifier *auth.BearerVerifier, m2mVerifier *auth.M2MVerifier, metrics *platform.Metrics) *App {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	router := gin.New()
	handlers := []gin.HandlerFunc{
		middleware.RequestID(),
	}
	if cfg.OTELTracingEnabled {
		handlers = append(handlers, middleware.Trace(cfg.OTELServiceName))
	}
	if cfg.MetricsEnabled && metrics != nil {
		handlers = append(handlers, metrics.HTTPMiddleware(cfg.MetricsPath))
		router.GET(cfg.MetricsPath, gin.WrapH(metrics.Handler()))
	}
	handlers = append(handlers,
		middleware.RequestLogger(logger),
		gin.Recovery(),
		middleware.DocsAuth(cfg.DocsAuthRequired, sessionService, authzService),
		middleware.ExternalCORS("/api/external/", cfg.ExternalAllowedOrigins),
		middleware.ExternalAuth("/api/external/", bearerVerifier, authzService, "zitadel", cfg.ExternalExpectedAudience, cfg.ExternalRequiredScopePrefix, cfg.ExternalRequiredRole, metrics),
		middleware.M2MAuth("/api/m2m/", m2mVerifier, machineClientService, "zitadel", metrics),
		middleware.SCIMAuth(cfg.SCIMBasePath+"/", bearerVerifier, cfg.SCIMBearerAudience, cfg.SCIMRequiredScope, metrics),
	)
	router.Use(handlers...)

	humaConfig := huma.DefaultConfig(cfg.AppName, cfg.AppVersion)
	humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"cookieAuth": {
			Type: "apiKey",
			In:   "cookie",
			Name: auth.SessionCookieName,
		},
		"bearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		},
		"m2mBearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		},
	}

	api := humagin.New(router, humaConfig)

	backendapi.Register(api, backendapi.Dependencies{
		SessionService:               sessionService,
		OIDCLoginService:             oidcLoginService,
		DelegationService:            delegationService,
		ProvisioningService:          provisioningService,
		AuthzService:                 authzService,
		AuditService:                 auditService,
		TenantAdminService:           tenantAdminService,
		TodoService:                  todoService,
		MachineClientService:         machineClientService,
		AuthMode:                     cfg.AuthMode,
		EnableLocalPasswordLogin:     cfg.EnableLocalPasswordLogin,
		SCIMBasePath:                 cfg.SCIMBasePath,
		FrontendBaseURL:              cfg.FrontendBaseURL,
		ZitadelIssuer:                cfg.ZitadelIssuer,
		ZitadelClientID:              cfg.ZitadelClientID,
		ZitadelPostLogoutRedirectURI: cfg.ZitadelPostLogoutRedirectURI,
		CookieSecure:                 cfg.CookieSecure,
		SessionTTL:                   cfg.SessionTTL,
	})

	return &App{
		Router: router,
		API:    api,
	}
}
