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
	User UserResponse `json:"user"`
}

type GetSessionInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID" required:"true"`
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

type LogoutInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID" required:"true"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
}

type LogoutOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
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
		user, err := deps.SessionService.CurrentUser(ctx, input.SessionCookie.Value)
		if err != nil {
			return nil, toHTTPError(err)
		}

		return &GetSessionOutput{
			Body: SessionBody{
				User: toUserResponse(user),
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/api/v1/login",
		Summary:     "ログインして Cookie セッションを払い出す",
		Tags:        []string{"session"},
	}, func(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
		user, sessionID, csrfToken, err := deps.SessionService.Login(ctx, input.Body.Email, input.Body.Password)
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
		OperationID:   "logout",
		Method:        http.MethodPost,
		Path:          "/api/v1/logout",
		Summary:       "セッションを破棄する",
		Tags:          []string{"session"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *LogoutInput) (*LogoutOutput, error) {
		if err := deps.SessionService.Logout(ctx, input.SessionCookie.Value, input.CSRFToken); err != nil {
			return nil, toHTTPError(err)
		}

		return &LogoutOutput{
			SetCookie: []http.Cookie{
				auth.ExpiredSessionCookie(deps.CookieSecure),
				auth.ExpiredXSRFCookie(deps.CookieSecure),
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
	default:
		return huma.Error500InternalServerError("internal server error")
	}
}
