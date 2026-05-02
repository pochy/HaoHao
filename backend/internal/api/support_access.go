package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type SupportAccessBody struct {
	PublicID                    string    `json:"publicId" format:"uuid"`
	SupportUserEmail            string    `json:"supportUserEmail" format:"email"`
	SupportUserDisplayName      string    `json:"supportUserDisplayName"`
	ImpersonatedUserEmail       string    `json:"impersonatedUserEmail" format:"email"`
	ImpersonatedUserDisplayName string    `json:"impersonatedUserDisplayName"`
	TenantSlug                  string    `json:"tenantSlug"`
	TenantDisplayName           string    `json:"tenantDisplayName"`
	Reason                      string    `json:"reason"`
	StartedAt                   time.Time `json:"startedAt" format:"date-time"`
	ExpiresAt                   time.Time `json:"expiresAt" format:"date-time"`
}

type StartSupportAccessBody struct {
	TenantSlug               string `json:"tenantSlug" minLength:"3" maxLength:"64"`
	ImpersonatedUserPublicID string `json:"impersonatedUserPublicId" format:"uuid"`
	Reason                   string `json:"reason" minLength:"8" maxLength:"1000"`
	DurationMinutes          int    `json:"durationMinutes,omitempty" minimum:"1"`
}

type StartSupportAccessInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          StartSupportAccessBody
}

type GetSupportAccessInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type EndSupportAccessInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
}

type SupportAccessOutput struct {
	Body struct {
		Active bool               `json:"active"`
		Access *SupportAccessBody `json:"access,omitempty"`
	}
}

type SupportAccessNoContentOutput struct{}

func registerSupportAccessRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{OperationID: "startSupportAccess", Method: http.MethodPost, Path: "/api/v1/support/access/start", Summary: "support access を開始する", Tags: []string{"support-access"}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *StartSupportAccessInput) (*SupportAccessOutput, error) {
		current, authCtx, err := currentSessionAuthContextWithCSRF(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if current.SupportAccess != nil {
			return nil, huma.Error409Conflict("support access is already active")
		}
		if !authCtx.HasRole("support_agent") {
			return nil, huma.Error403Forbidden("support_agent role is required")
		}
		item, err := deps.SupportAccessService.Start(ctx, input.SessionCookie.Value, current.User.ID, service.SupportAccessStartInput{
			TenantSlug:               input.Body.TenantSlug,
			ImpersonatedUserPublicID: input.Body.ImpersonatedUserPublicID,
			Reason:                   input.Body.Reason,
			DurationMinutes:          input.Body.DurationMinutes,
		}, sessionAuditContext(ctx, current, nil))
		if err != nil {
			return nil, toSupportAccessHTTPError(err)
		}
		out := &SupportAccessOutput{}
		body := toSupportAccessBody(item)
		out.Body.Active = true
		out.Body.Access = &body
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "getCurrentSupportAccess", Method: http.MethodGet, Path: "/api/v1/support/access/current", Summary: "現在の support access を返す", Tags: []string{"support-access"}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *GetSupportAccessInput) (*SupportAccessOutput, error) {
		item, ok, err := deps.SupportAccessService.Current(ctx, input.SessionCookie.Value)
		if err != nil {
			return nil, toSupportAccessHTTPError(err)
		}
		out := &SupportAccessOutput{}
		out.Body.Active = ok
		if ok {
			body := toSupportAccessBody(item)
			out.Body.Access = &body
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "endSupportAccess", Method: http.MethodPost, Path: "/api/v1/support/access/end", Summary: "support access を終了する", Tags: []string{"support-access"}, DefaultStatus: http.StatusNoContent, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *EndSupportAccessInput) (*SupportAccessNoContentOutput, error) {
		current, err := deps.SessionService.CurrentSessionWithCSRF(ctx, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPErrorWithLog(ctx, deps, "", err)
		}
		auditUserID := current.User.ID
		if current.ActorUser != nil {
			auditUserID = current.ActorUser.ID
		}
		if err := deps.SupportAccessService.End(ctx, input.SessionCookie.Value, userAuditContext(ctx, auditUserID, nil)); err != nil {
			return nil, toSupportAccessHTTPError(err)
		}
		return &SupportAccessNoContentOutput{}, nil
	})
}

func toSupportAccessBody(item service.SupportAccess) SupportAccessBody {
	return SupportAccessBody{PublicID: item.PublicID, SupportUserEmail: item.SupportUserEmail, SupportUserDisplayName: item.SupportUserDisplayName, ImpersonatedUserEmail: item.ImpersonatedUserEmail, ImpersonatedUserDisplayName: item.ImpersonatedUserDisplayName, TenantSlug: item.TenantSlug, TenantDisplayName: item.TenantDisplayName, Reason: item.Reason, StartedAt: item.StartedAt, ExpiresAt: item.ExpiresAt}
}

func toSupportAccessHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidSupportAccessInput):
		return huma.Error400BadRequest("invalid support access input")
	case errors.Is(err, service.ErrSupportAccessEntitlement):
		return huma.Error403Forbidden("support access entitlement is disabled")
	case errors.Is(err, service.ErrSupportAccessTenantMissing):
		return huma.Error409Conflict("impersonated user is not active in tenant")
	case errors.Is(err, service.ErrSupportAccessNotFound):
		return huma.Error404NotFound("support access not found")
	default:
		return toHTTPError(err)
	}
}
