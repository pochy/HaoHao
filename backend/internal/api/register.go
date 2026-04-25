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
	AuthMode                     string
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
	registerSCIMRoutes(api, deps)
}
