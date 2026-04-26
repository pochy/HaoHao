package api

import (
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type Dependencies struct {
	SessionService                   *service.SessionService
	OIDCLoginService                 *service.OIDCLoginService
	DelegationService                *service.DelegationService
	ProvisioningService              *service.ProvisioningService
	AuthzService                     *service.AuthzService
	AuditService                     *service.AuditService
	TenantAdminService               *service.TenantAdminService
	CustomerSignalService            *service.CustomerSignalService
	TodoService                      *service.TodoService
	MachineClientService             *service.MachineClientService
	OutboxService                    *service.OutboxService
	IdempotencyService               *service.IdempotencyService
	NotificationService              *service.NotificationService
	TenantInvitationService          *service.TenantInvitationService
	FileService                      *service.FileService
	DriveService                     *service.DriveService
	TenantSettingsService            *service.TenantSettingsService
	TenantDataExportService          *service.TenantDataExportService
	EntitlementService               *service.EntitlementService
	WebhookService                   *service.WebhookService
	CustomerSignalImportService      *service.CustomerSignalImportService
	CustomerSignalSavedFilterService *service.CustomerSignalSavedFilterService
	SupportAccessService             *service.SupportAccessService
	AuthMode                         string
	EnableLocalPasswordLogin         bool
	SCIMBasePath                     string
	FrontendBaseURL                  string
	ZitadelIssuer                    string
	ZitadelClientID                  string
	ZitadelPostLogoutRedirectURI     string
	CookieSecure                     bool
	SessionTTL                       time.Duration
}

type Surface string

const (
	SurfaceFull     Surface = "full"
	SurfaceBrowser  Surface = "browser"
	SurfaceExternal Surface = "external"
)

func (s Surface) Valid() bool {
	return s == SurfaceFull || s == SurfaceBrowser || s == SurfaceExternal
}

func Register(api huma.API, deps Dependencies) {
	RegisterSurface(api, deps, SurfaceFull)
}

func RegisterSurface(api huma.API, deps Dependencies, surface Surface) {
	if includeBrowser(surface) {
		registerAuthSettingsRoute(api, deps)
		registerOIDCRoutes(api, deps)
		registerSessionRoutes(api, deps)
		registerIntegrationRoutes(api, deps)
		registerTenantRoutes(api, deps)
		registerTenantAdminRoutes(api, deps)
		registerCustomerSignalRoutes(api, deps)
		registerTodoRoutes(api, deps)
		registerNotificationRoutes(api, deps)
		registerTenantInvitationRoutes(api, deps)
		registerFileRoutes(api, deps)
		registerTenantSettingsRoutes(api, deps)
		registerTenantDataExportRoutes(api, deps)
		registerEntitlementRoutes(api, deps)
		registerWebhookRoutes(api, deps)
		registerCustomerSignalImportRoutes(api, deps)
		registerCustomerSignalSavedFilterRoutes(api, deps)
		registerSupportAccessRoutes(api, deps)
		registerMachineClientRoutes(api, deps)
	}

	if includeExternal(surface) {
		registerExternalRoutes(api, deps)
		registerM2MRoutes(api, deps)
		registerSCIMRoutes(api, deps)
	}
}

func includeBrowser(surface Surface) bool {
	return surface == SurfaceFull || surface == SurfaceBrowser
}

func includeExternal(surface Surface) bool {
	return surface == SurfaceFull || surface == SurfaceExternal
}
