package api

import (
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type Dependencies struct {
	SessionService *service.SessionService
	CookieSecure   bool
	SessionTTL     time.Duration
}

func Register(api huma.API, deps Dependencies) {
	registerSessionRoutes(api, deps)
}
