package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type TenantInvitationBody struct {
	PublicID                  string    `json:"publicId" format:"uuid"`
	TenantID                  int64     `json:"tenantId"`
	TenantSlug                string    `json:"tenantSlug,omitempty"`
	TenantDisplayName         string    `json:"tenantDisplayName,omitempty"`
	InviteeEmailNormalized    string    `json:"inviteeEmailNormalized" format:"email"`
	RoleCodes                 []string  `json:"roleCodes"`
	Status                    string    `json:"status" example:"pending"`
	AcceptURL                 string    `json:"acceptUrl,omitempty" format:"uri"`
	Token                     string    `json:"token,omitempty"`
	IdentitySetupDeliveryMode string    `json:"identitySetupDeliveryMode,omitempty" example:"return_code"`
	IdentitySetupUserID       string    `json:"identitySetupUserId,omitempty"`
	IdentitySetupInviteCode   string    `json:"identitySetupInviteCode,omitempty"`
	IdentitySetupLoginURL     string    `json:"identitySetupLoginUrl,omitempty" format:"uri"`
	IdentitySetupEmailCode    string    `json:"identitySetupEmailCode,omitempty"`
	ExpiresAt                 time.Time `json:"expiresAt" format:"date-time"`
	CreatedAt                 time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt                 time.Time `json:"updatedAt" format:"date-time"`
}

type ListTenantInvitationsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
}

type TenantInvitationListOutput struct {
	Body struct {
		Items []TenantInvitationBody `json:"items"`
	}
}

type CreateTenantInvitationRequestBody struct {
	Email     string   `json:"email" format:"email"`
	RoleCodes []string `json:"roleCodes,omitempty"`
}

type CreateTenantInvitationInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	IdempotencyKey string      `header:"Idempotency-Key"`
	TenantSlug     string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body           CreateTenantInvitationRequestBody
}

type RevokeTenantInvitationInput struct {
	SessionCookie      http.Cookie `cookie:"SESSION_ID"`
	CSRFToken          string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug         string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	InvitationPublicID string      `path:"invitationPublicId" format:"uuid"`
}

type ProvisionTenantInvitationIdentityInput struct {
	SessionCookie      http.Cookie `cookie:"SESSION_ID"`
	CSRFToken          string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug         string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	InvitationPublicID string      `path:"invitationPublicId" format:"uuid"`
}

type SetupTenantInvitationIdentityRequestBody struct {
	Token string `json:"token"`
}

type SetupTenantInvitationIdentityInput struct {
	InvitationPublicID string `path:"invitationPublicId" format:"uuid"`
	Body               SetupTenantInvitationIdentityRequestBody
}

type AcceptTenantInvitationRequestBody struct {
	Token string `json:"token"`
}

type ResolveTenantInvitationInput struct {
	Token string `query:"token" required:"true"`
}

type AcceptTenantInvitationInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	IdempotencyKey string      `header:"Idempotency-Key"`
	Body           AcceptTenantInvitationRequestBody
}

type TenantInvitationOutput struct {
	Body TenantInvitationBody
}

func registerTenantInvitationRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listTenantInvitations",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/invitations",
		Summary:     "tenant invitation 一覧を返す",
		Tags:        []string{DocTagTenantWorkspace},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListTenantInvitationsInput) (*TenantInvitationListOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		items, err := deps.TenantInvitationService.List(ctx, tenant.ID, 50)
		if err != nil {
			return nil, toTenantInvitationHTTPError(err)
		}
		out := &TenantInvitationListOutput{}
		out.Body.Items = make([]TenantInvitationBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toTenantInvitationBody(item, false))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createTenantInvitation",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/invitations",
		Summary:     "tenant invitation を作成する",
		Tags:        []string{DocTagTenantWorkspace},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *CreateTenantInvitationInput) (*TenantInvitationOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if deps.TenantInvitationService == nil {
			return nil, huma.Error503ServiceUnavailable("tenant invitation service is not configured")
		}
		attempt, err := beginIdempotency(ctx, deps, input.IdempotencyKey, http.MethodPost, "/api/v1/admin/tenants/{tenantSlug}/invitations", current.User.ID, &tenant.ID, input.Body)
		if err != nil {
			return nil, toIdempotencyHTTPError(err)
		}
		if attempt.Replay {
			body, err := replayIdempotencyBody[TenantInvitationBody](attempt)
			if err != nil {
				return nil, err
			}
			return &TenantInvitationOutput{Body: body}, nil
		}
		item, err := deps.TenantInvitationService.Create(ctx, service.TenantInvitationInput{
			TenantID:  tenant.ID,
			ActorID:   current.User.ID,
			Email:     input.Body.Email,
			RoleCodes: input.Body.RoleCodes,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			if deps.IdempotencyService != nil {
				deps.IdempotencyService.Fail(ctx, attempt, http.StatusInternalServerError, err.Error())
			}
			return nil, toTenantInvitationHTTPError(err)
		}
		body := toTenantInvitationBody(item, true)
		if err := completeIdempotency(ctx, deps, attempt, http.StatusOK, body); err != nil {
			return nil, err
		}
		return &TenantInvitationOutput{Body: body}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "revokeTenantInvitation",
		Method:        http.MethodDelete,
		Path:          "/api/v1/admin/tenants/{tenantSlug}/invitations/{invitationPublicId}",
		Summary:       "tenant invitation を revoke する",
		Tags:          []string{DocTagTenantWorkspace},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *RevokeTenantInvitationInput) (*TenantAdminNoContentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.TenantInvitationService.Revoke(ctx, tenant.ID, input.InvitationPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toTenantInvitationHTTPError(err)
		}
		return &TenantAdminNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "provisionTenantInvitationIdentity",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/invitations/{invitationPublicId}/provision-identity",
		Summary:     "pending tenant invitation の Zitadel identity 初期化メールを送る",
		Tags:        []string{DocTagTenantWorkspace},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ProvisionTenantInvitationIdentityInput) (*TenantInvitationOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if deps.TenantInvitationService == nil {
			return nil, huma.Error503ServiceUnavailable("tenant invitation service is not configured")
		}
		item, err := deps.TenantInvitationService.ProvisionIdentityForInvitation(ctx, tenant.ID, input.InvitationPublicID)
		if err != nil {
			return nil, toTenantInvitationHTTPError(err)
		}
		return &TenantInvitationOutput{Body: toTenantInvitationBody(item, false)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "resolveTenantInvitation",
		Method:      http.MethodGet,
		Path:        "/api/v1/invitations/resolve",
		Summary:     "tenant invitation token の公開情報を返す",
		Tags:        []string{DocTagTenantWorkspace},
	}, func(ctx context.Context, input *ResolveTenantInvitationInput) (*TenantInvitationOutput, error) {
		if deps.TenantInvitationService == nil {
			return nil, huma.Error503ServiceUnavailable("tenant invitation service is not configured")
		}
		item, err := deps.TenantInvitationService.Resolve(ctx, input.Token)
		if err != nil {
			return nil, toTenantInvitationHTTPError(err)
		}
		return &TenantInvitationOutput{Body: toTenantInvitationBody(item, false)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "setupTenantInvitationIdentity",
		Method:      http.MethodPost,
		Path:        "/api/v1/invitations/{invitationPublicId}/setup-identity",
		Summary:     "tenant invitation の Zitadel identity 初期設定を開始する",
		Tags:        []string{DocTagTenantWorkspace},
	}, func(ctx context.Context, input *SetupTenantInvitationIdentityInput) (*TenantInvitationOutput, error) {
		if deps.TenantInvitationService == nil {
			return nil, huma.Error503ServiceUnavailable("tenant invitation service is not configured")
		}
		item, err := deps.TenantInvitationService.SetupIdentityForInvitation(ctx, input.InvitationPublicID, input.Body.Token)
		if err != nil {
			return nil, toTenantInvitationHTTPError(err)
		}
		return &TenantInvitationOutput{Body: toTenantInvitationBody(item, false)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "acceptTenantInvitation",
		Method:      http.MethodPost,
		Path:        "/api/v1/invitations/accept",
		Summary:     "tenant invitation を accept する",
		Tags:        []string{DocTagTenantWorkspace},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *AcceptTenantInvitationInput) (*TenantInvitationOutput, error) {
		current, _, err := currentSessionAuthContextWithCSRF(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toHTTPErrorWithLog(ctx, deps, "", err)
		}
		attempt, err := beginIdempotency(ctx, deps, input.IdempotencyKey, http.MethodPost, "/api/v1/invitations/accept", current.User.ID, nil, input.Body)
		if err != nil {
			return nil, toIdempotencyHTTPError(err)
		}
		if attempt.Replay {
			body, err := replayIdempotencyBody[TenantInvitationBody](attempt)
			if err != nil {
				return nil, err
			}
			return &TenantInvitationOutput{Body: body}, nil
		}
		item, err := deps.TenantInvitationService.Accept(ctx, current.User, input.Body.Token, sessionAuditContext(ctx, current, nil))
		if err != nil {
			return nil, toTenantInvitationHTTPError(err)
		}
		body := toTenantInvitationBody(item, false)
		if err := completeIdempotency(ctx, deps, attempt, http.StatusOK, body); err != nil {
			return nil, err
		}
		return &TenantInvitationOutput{Body: body}, nil
	})
}

func toTenantInvitationBody(item service.TenantInvitation, includeToken bool) TenantInvitationBody {
	body := TenantInvitationBody{
		PublicID:                  item.PublicID,
		TenantID:                  item.TenantID,
		TenantSlug:                item.TenantSlug,
		TenantDisplayName:         item.TenantDisplayName,
		InviteeEmailNormalized:    item.InviteeEmailNormalized,
		RoleCodes:                 item.RoleCodes,
		Status:                    item.Status,
		AcceptURL:                 item.AcceptURL,
		IdentitySetupDeliveryMode: item.IdentitySetupDeliveryMode,
		IdentitySetupUserID:       item.IdentitySetupUserID,
		IdentitySetupInviteCode:   item.IdentitySetupInviteCode,
		IdentitySetupLoginURL:     item.IdentitySetupLoginURL,
		IdentitySetupEmailCode:    item.IdentitySetupEmailCode,
		ExpiresAt:                 item.ExpiresAt,
		CreatedAt:                 item.CreatedAt,
		UpdatedAt:                 item.UpdatedAt,
	}
	if includeToken {
		body.Token = item.Token
	}
	return body
}

func toTenantInvitationHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidTenantInvitation):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, service.ErrTenantInvitationNotFound):
		return huma.Error404NotFound("tenant invitation not found")
	case errors.Is(err, service.ErrTenantInvitationEmailMismatch):
		return huma.Error403Forbidden("tenant invitation email mismatch")
	case errors.Is(err, service.ErrTenantInvitationNotPending):
		return huma.Error409Conflict("tenant invitation is not pending")
	case errors.Is(err, service.ErrIdentityProvisioningUnavailable):
		return huma.Error503ServiceUnavailable("Zitadel identity provisioning is not configured")
	case errors.Is(err, service.ErrIdentityProvisioningFailed):
		return huma.Error503ServiceUnavailable("Zitadel identity provisioning failed")
	default:
		return toHTTPError(err)
	}
}
