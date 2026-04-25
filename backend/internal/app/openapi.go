package app

import (
	"fmt"

	backendapi "example.com/haohao/backend/internal/api"
	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/config"
	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

func NewOpenAPIExport(cfg config.Config, surface backendapi.Surface) (*huma.OpenAPI, error) {
	if !surface.Valid() {
		return nil, fmt.Errorf("invalid OpenAPI surface %q", surface)
	}

	router := gin.New()
	api := humagin.New(router, humaConfigForSurface(cfg, surface))
	backendapi.RegisterSurface(api, openAPIExportDependencies(cfg), surface)

	return api.OpenAPI(), nil
}

func humaConfigForSurface(cfg config.Config, surface backendapi.Surface) huma.Config {
	humaConfig := huma.DefaultConfig(cfg.AppName, cfg.AppVersion)
	humaConfig.Components.SecuritySchemes = securitySchemesForSurface(surface)
	return humaConfig
}

func securitySchemesForSurface(surface backendapi.Surface) map[string]*huma.SecurityScheme {
	schemes := map[string]*huma.SecurityScheme{}

	if surface == backendapi.SurfaceFull || surface == backendapi.SurfaceBrowser {
		schemes["cookieAuth"] = &huma.SecurityScheme{
			Type: "apiKey",
			In:   "cookie",
			Name: auth.SessionCookieName,
		}
	}

	if surface == backendapi.SurfaceFull || surface == backendapi.SurfaceExternal {
		schemes["bearerAuth"] = &huma.SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		}
		schemes["m2mBearerAuth"] = &huma.SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		}
	}

	return schemes
}

func openAPIExportDependencies(cfg config.Config) backendapi.Dependencies {
	auditService := service.NewAuditService(nil)
	outboxService := service.NewOutboxService(nil, nil, cfg.OutboxWorkerMaxAttempts)
	idempotencyService := service.NewIdempotencyService(nil, cfg.IdempotencyTTL)
	notificationService := service.NewNotificationService(nil, auditService)
	tenantSettingsService := service.NewTenantSettingsService(nil, auditService, cfg.TenantDefaultFileQuotaBytes)
	fileService := service.NewFileService(nil, nil, nil, tenantSettingsService, auditService, cfg.FileMaxBytes, cfg.FileAllowedMIMETypes, nil)
	tenantInvitationService := service.NewTenantInvitationService(nil, nil, outboxService, auditService, cfg.InvitationTTL, cfg.FrontendBaseURL)
	tenantDataExportService := service.NewTenantDataExportService(nil, nil, outboxService, fileService, auditService, cfg.DataExportTTL)

	return dependenciesWithConfig(cfg, backendapi.Dependencies{
		AuditService:            auditService,
		TenantAdminService:      service.NewTenantAdminService(nil, nil, auditService),
		CustomerSignalService:   service.NewCustomerSignalService(nil, nil, auditService),
		TodoService:             service.NewTodoService(nil, nil, auditService),
		MachineClientService:    service.NewMachineClientService(nil, nil, "", auditService),
		OutboxService:           outboxService,
		IdempotencyService:      idempotencyService,
		NotificationService:     notificationService,
		TenantInvitationService: tenantInvitationService,
		FileService:             fileService,
		TenantSettingsService:   tenantSettingsService,
		TenantDataExportService: tenantDataExportService,
	})
}

func dependenciesWithConfig(cfg config.Config, deps backendapi.Dependencies) backendapi.Dependencies {
	deps.AuthMode = cfg.AuthMode
	deps.EnableLocalPasswordLogin = cfg.EnableLocalPasswordLogin
	deps.SCIMBasePath = cfg.SCIMBasePath
	deps.FrontendBaseURL = cfg.FrontendBaseURL
	deps.ZitadelIssuer = cfg.ZitadelIssuer
	deps.ZitadelClientID = cfg.ZitadelClientID
	deps.ZitadelPostLogoutRedirectURI = cfg.ZitadelPostLogoutRedirectURI
	deps.CookieSecure = cfg.CookieSecure
	deps.SessionTTL = cfg.SessionTTL
	return deps
}
