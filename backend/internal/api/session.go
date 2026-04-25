package api

import (
	"context"
	"errors"
	"net/http"

	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type UserResponse struct {
	PublicID    string `json:"publicId" format:"uuid" example:"018f2f05-c6c9-7a49-b32d-04f4dd84ef4a"`
	Email       string `json:"email" format:"email" example:"demo@example.com"`
	DisplayName string `json:"displayName" example:"Demo User"`
}

type SessionBody struct {
	User          UserResponse       `json:"user"`
	SupportAccess *SupportAccessBody `json:"supportAccess,omitempty"`
}

type GetSessionInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type GetSessionOutput struct {
	Body SessionBody
}

type LoginInput struct {
	Body struct {
		Email    string `json:"email" format:"email" example:"demo@example.com"`
		Password string `json:"password" minLength:"8" example:"changeme123"`
	}
}

type LoginOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      SessionBody
}

type GetCSRFInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type GetCSRFOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
}

type RefreshSessionInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
}

type RefreshSessionOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
}

type LogoutInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
}

type LogoutBody struct {
	PostLogoutURL string `json:"postLogoutURL,omitempty" format:"uri" example:"http://localhost:8081/oidc/v1/end_session?id_token_hint=..."`
}

type LogoutOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      LogoutBody
}

func registerSessionRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getSession",
		Method:      http.MethodGet,
		Path:        "/api/v1/session",
		Summary:     "現在のセッションを返す",
		Tags:        []string{"session"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *GetSessionInput) (*GetSessionOutput, error) {
		current, err := deps.SessionService.CurrentSession(ctx, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}

		body := SessionBody{User: toUserResponse(current.User)}
		if current.SupportAccess != nil {
			access := toSupportAccessBody(*current.SupportAccess)
			body.SupportAccess = &access
		}
		return &GetSessionOutput{Body: body}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/api/v1/login",
		Summary:     "ログインして Cookie セッションを払い出す",
		Tags:        []string{"session"},
	}, func(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
		user, sessionID, csrfToken, err := deps.SessionService.Login(ctx, input.Body.Email, input.Body.Password, auditRequest(ctx))
		if err != nil {
			return nil, toHTTPError(err)
		}

		return &LoginOutput{
			SetCookie: []http.Cookie{
				auth.NewSessionCookie(sessionID, deps.CookieSecure, deps.SessionTTL),
				auth.NewXSRFCookie(csrfToken, deps.CookieSecure, deps.SessionTTL),
			},
			Body: SessionBody{
				User: toUserResponse(user),
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "getCSRF",
		Method:        http.MethodGet,
		Path:          "/api/v1/csrf",
		Summary:       "CSRF token を再発行する",
		Tags:          []string{"session"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *GetCSRFInput) (*GetCSRFOutput, error) {
		csrfToken, err := deps.SessionService.ReissueCSRF(ctx, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}

		return &GetCSRFOutput{
			SetCookie: []http.Cookie{
				auth.NewXSRFCookie(csrfToken, deps.CookieSecure, deps.SessionTTL),
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "refreshSession",
		Method:        http.MethodPost,
		Path:          "/api/v1/session/refresh",
		Summary:       "セッションを再発行する",
		Tags:          []string{"session"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *RefreshSessionInput) (*RefreshSessionOutput, error) {
		sessionID, csrfToken, err := deps.SessionService.RefreshSession(ctx, input.SessionCookie.Value, input.CSRFToken, auditRequest(ctx))
		if err != nil {
			return nil, toHTTPError(err)
		}

		return &RefreshSessionOutput{
			SetCookie: []http.Cookie{
				auth.NewSessionCookie(sessionID, deps.CookieSecure, deps.SessionTTL),
				auth.NewXSRFCookie(csrfToken, deps.CookieSecure, deps.SessionTTL),
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "logout",
		Method:        http.MethodPost,
		Path:          "/api/v1/logout",
		Summary:       "セッションを破棄する",
		Tags:          []string{"session"},
		DefaultStatus: http.StatusOK,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *LogoutInput) (*LogoutOutput, error) {
		idTokenHint, err := deps.SessionService.Logout(ctx, input.SessionCookie.Value, input.CSRFToken, auditRequest(ctx))
		if err != nil && !errors.Is(err, service.ErrUnauthorized) {
			return nil, toHTTPError(err)
		}

		return &LogoutOutput{
			SetCookie: []http.Cookie{
				auth.ExpiredSessionCookie(deps.CookieSecure),
				auth.ExpiredXSRFCookie(deps.CookieSecure),
			},
			Body: LogoutBody{
				PostLogoutURL: buildPostLogoutURL(deps, idTokenHint),
			},
		}, nil
	})
}

func toUserResponse(user service.User) UserResponse {
	return UserResponse{
		PublicID:    user.PublicID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
	}
}

func toHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		return huma.Error401Unauthorized("invalid credentials")
	case errors.Is(err, service.ErrUnauthorized):
		return huma.Error401Unauthorized("missing or expired session")
	case errors.Is(err, service.ErrInvalidCSRFToken):
		return huma.Error403Forbidden("invalid csrf token")
	case errors.Is(err, service.ErrAuthModeUnsupported):
		return huma.Error501NotImplemented("password login is disabled for the current auth mode")
	default:
		return huma.Error500InternalServerError("internal server error")
	}
}
