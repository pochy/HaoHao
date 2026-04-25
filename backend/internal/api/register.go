package api

import (
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type Dependencies struct {
	SessionService               *service.SessionService
	OIDCLoginService             *service.OIDCLoginService
	DelegationService            *service.DelegationService
	ProvisioningService          *service.ProvisioningService
	AuthzService                 *service.AuthzService
	AuditService                 *service.AuditService
	TenantAdminService           *service.TenantAdminService
	CustomerSignalService        *service.CustomerSignalService
	TodoService                  *service.TodoService
	MachineClientService         *service.MachineClientService
	OutboxService                *service.OutboxService
	IdempotencyService           *service.IdempotencyService
	NotificationService          *service.NotificationService
	TenantInvitationService      *service.TenantInvitationService
	FileService                  *service.FileService
	TenantSettingsService        *service.TenantSettingsService
	TenantDataExportService      *service.TenantDataExportService
	AuthMode                     string
	EnableLocalPasswordLogin     bool
	SCIMBasePath                 string
	FrontendBaseURL              string
	ZitadelIssuer                string
	ZitadelClientID              string
	ZitadelPostLogoutRedirectURI string
	CookieSecure                 bool
	SessionTTL                   time.Duration
}

func Register(api huma.API, deps Dependencies) {
	registerAuthSettingsRoute(api, deps)
	registerOIDCRoutes(api, deps)
	registerSessionRoutes(api, deps)
	registerExternalRoutes(api, deps)
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
	registerMachineClientRoutes(api, deps)
	registerM2MRoutes(api, deps)
	registerSCIMRoutes(api, deps)
}
