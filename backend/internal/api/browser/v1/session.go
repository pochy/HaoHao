package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/pochy/haohao/backend/internal/config"
	"github.com/pochy/haohao/backend/internal/service"
)

type SessionBody struct {
	Authenticated bool   `json:"authenticated" example:"false"`
	AuthMode      string `json:"authMode" example:"stub"`
	ApiSurface    string `json:"apiSurface" example:"browser"`
	CsrfCookie    string `json:"csrfCookie" example:"XSRF-TOKEN"`
}

type SessionOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      SessionBody
}

func RegisterSession(api huma.API, cfg config.Config, sessions *service.SessionService) {
	huma.Register(api, huma.Operation{
		OperationID: "getSession",
		Method:      http.MethodGet,
		Path:        "/api/v1/session",
		Summary:     "Get browser session bootstrap state",
		Tags:        []string{"session"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *struct{}) (*SessionOutput, error) {
		_ = input

		snapshot := sessions.Snapshot(ctx)
		csrfCookie := http.Cookie{
			Name:     cfg.CSRFCookieName,
			Value:    sessions.NewCSRFCookieValue(),
			Path:     "/",
			HttpOnly: false,
			SameSite: http.SameSiteLaxMode,
		}

		return &SessionOutput{
			SetCookie: []http.Cookie{csrfCookie},
			Body: SessionBody{
				Authenticated: snapshot.Authenticated,
				AuthMode:      snapshot.AuthMode,
				ApiSurface:    snapshot.APISurface,
				CsrfCookie:    cfg.CSRFCookieName,
			},
		}, nil
	})
}

