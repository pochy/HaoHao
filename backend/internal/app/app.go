package app

import (
	backendapi "example.com/haohao/backend/internal/api"
	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/config"
	"example.com/haohao/backend/internal/middleware"
	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

type App struct {
	Router *gin.Engine
	API    huma.API
}

func New(cfg config.Config, sessionService *service.SessionService, oidcLoginService *service.OIDCLoginService, delegationService *service.DelegationService, authzService *service.AuthzService, bearerVerifier *auth.BearerVerifier) *App {
	router := gin.New()
	router.Use(
		gin.Logger(),
		gin.Recovery(),
		middleware.ExternalCORS("/api/external/", cfg.ExternalAllowedOrigins),
		middleware.ExternalAuth("/api/external/", bearerVerifier, authzService, "zitadel", cfg.ExternalExpectedAudience, cfg.ExternalRequiredScopePrefix, cfg.ExternalRequiredRole),
	)

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
	}

	api := humagin.New(router, humaConfig)

	backendapi.Register(api, backendapi.Dependencies{
		SessionService:               sessionService,
		OIDCLoginService:             oidcLoginService,
		DelegationService:            delegationService,
		AuthMode:                     cfg.AuthMode,
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
