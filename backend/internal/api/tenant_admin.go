package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type TenantAdminTenantBody struct {
	ID                int64     `json:"id" example:"1"`
	Slug              string    `json:"slug" example:"acme"`
	DisplayName       string    `json:"displayName" example:"Acme"`
	Active            bool      `json:"active" example:"true"`
	ActiveMemberCount int64     `json:"activeMemberCount" example:"3"`
	CreatedAt         time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt         time.Time `json:"updatedAt" format:"date-time"`
}

type TenantAdminRoleBindingBody struct {
	RoleCode string `json:"roleCode" example:"todo_user"`
	Source   string `json:"source" example:"local_override"`
	Active   bool   `json:"active" example:"true"`
}

type TenantAdminMembershipBody struct {
	UserPublicID string                       `json:"userPublicId" format:"uuid"`
	Email        string                       `json:"email" format:"email"`
	DisplayName  string                       `json:"displayName" example:"Demo User"`
	Deactivated  bool                         `json:"deactivated" example:"false"`
	Roles        []TenantAdminRoleBindingBody `json:"roles"`
}

type TenantAdminTenantDetailBody struct {
	Tenant      TenantAdminTenantBody       `json:"tenant"`
	Memberships []TenantAdminMembershipBody `json:"memberships"`
}

type TenantAdminTenantRequestBody struct {
	Slug        string `json:"slug,omitempty" minLength:"3" maxLength:"64" example:"acme"`
	DisplayName string `json:"displayName" minLength:"1" maxLength:"120" example:"Acme"`
	Active      *bool  `json:"active,omitempty" example:"true"`
}

type TenantAdminMembershipRequestBody struct {
	UserEmail string `json:"userEmail" format:"email" example:"demo@example.com"`
	RoleCode  string `json:"roleCode" example:"todo_user"`
}

type ListTenantAdminTenantsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type ListTenantAdminTenantsOutput struct {
	Body struct {
		Items []TenantAdminTenantBody `json:"items"`
	}
}

type CreateTenantAdminTenantInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          TenantAdminTenantRequestBody
}

type TenantAdminTenantOutput struct {
	Body TenantAdminTenantBody
}

type TenantAdminBySlugInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
}

type GetTenantAdminTenantOutput struct {
	Body TenantAdminTenantDetailBody
}

type UpdateTenantAdminTenantInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          TenantAdminTenantRequestBody
}

type DeactivateTenantAdminTenantInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
}

type GrantTenantAdminRoleInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          TenantAdminMembershipRequestBody
}

type RevokeTenantAdminRoleInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	UserPublicID  string      `path:"userPublicId" format:"uuid"`
	RoleCode      string      `path:"roleCode"`
}

type TenantAdminNoContentOutput struct{}

func registerTenantAdminRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listTenantAdminTenants",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants",
		Summary:     "tenant admin 用の tenant 一覧を返す",
		Tags:        []string{"tenant-admin"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListTenantAdminTenantsInput) (*ListTenantAdminTenantsOutput, error) {
		if _, err := requireTenantAdmin(ctx, deps, input.SessionCookie.Value, ""); err != nil {
			return nil, err
		}
		items, err := deps.TenantAdminService.ListTenants(ctx)
		if err != nil {
			return nil, toTenantAdminHTTPError(err)
		}
		out := &ListTenantAdminTenantsOutput{}
		out.Body.Items = make([]TenantAdminTenantBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toTenantAdminTenantBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createTenantAdminTenant",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants",
		Summary:     "tenant を作成する",
		Tags:        []string{"tenant-admin"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *CreateTenantAdminTenantInput) (*TenantAdminTenantOutput, error) {
		current, err := requireTenantAdmin(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		tenant, err := deps.TenantAdminService.CreateTenant(ctx, tenantAdminTenantInputFromBody(input.Body), sessionAuditContext(ctx, current, nil))
		if err != nil {
			return nil, toTenantAdminHTTPError(err)
		}
		return &TenantAdminTenantOutput{Body: toTenantAdminTenantBody(tenant)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getTenantAdminTenant",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}",
		Summary:     "tenant detail と membership を返す",
		Tags:        []string{"tenant-admin"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *TenantAdminBySlugInput) (*GetTenantAdminTenantOutput, error) {
		if _, err := requireTenantAdmin(ctx, deps, input.SessionCookie.Value, ""); err != nil {
			return nil, err
		}
		detail, err := deps.TenantAdminService.GetTenant(ctx, input.TenantSlug)
		if err != nil {
			return nil, toTenantAdminHTTPError(err)
		}
		return &GetTenantAdminTenantOutput{Body: toTenantAdminTenantDetailBody(detail)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateTenantAdminTenant",
		Method:      http.MethodPut,
		Path:        "/api/v1/admin/tenants/{tenantSlug}",
		Summary:     "tenant を更新する",
		Tags:        []string{"tenant-admin"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *UpdateTenantAdminTenantInput) (*TenantAdminTenantOutput, error) {
		current, err := requireTenantAdmin(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		tenant, err := deps.TenantAdminService.UpdateTenant(ctx, input.TenantSlug, tenantAdminTenantInputFromBody(input.Body), sessionAuditContext(ctx, current, nil))
		if err != nil {
			return nil, toTenantAdminHTTPError(err)
		}
		return &TenantAdminTenantOutput{Body: toTenantAdminTenantBody(tenant)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deactivateTenantAdminTenant",
		Method:        http.MethodDelete,
		Path:          "/api/v1/admin/tenants/{tenantSlug}",
		Summary:       "tenant を無効化する",
		Tags:          []string{"tenant-admin"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *DeactivateTenantAdminTenantInput) (*TenantAdminNoContentOutput, error) {
		current, err := requireTenantAdmin(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if _, err := deps.TenantAdminService.DeactivateTenant(ctx, input.TenantSlug, sessionAuditContext(ctx, current, nil)); err != nil {
			return nil, toTenantAdminHTTPError(err)
		}
		return &TenantAdminNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "grantTenantAdminRole",
		Method:        http.MethodPost,
		Path:          "/api/v1/admin/tenants/{tenantSlug}/memberships",
		Summary:       "tenant local role を付与する",
		Tags:          []string{"tenant-admin"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *GrantTenantAdminRoleInput) (*TenantAdminNoContentOutput, error) {
		current, err := requireTenantAdmin(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if _, err := deps.TenantAdminService.GrantRole(ctx, input.TenantSlug, service.TenantRoleGrantInput{
			UserEmail: input.Body.UserEmail,
			RoleCode:  input.Body.RoleCode,
		}, sessionAuditContext(ctx, current, nil)); err != nil {
			return nil, toTenantAdminHTTPError(err)
		}
		return &TenantAdminNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "revokeTenantAdminRole",
		Method:        http.MethodDelete,
		Path:          "/api/v1/admin/tenants/{tenantSlug}/memberships/{userPublicId}/roles/{roleCode}",
		Summary:       "tenant local role を無効化する",
		Tags:          []string{"tenant-admin"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *RevokeTenantAdminRoleInput) (*TenantAdminNoContentOutput, error) {
		current, err := requireTenantAdmin(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.TenantAdminService.RevokeLocalRole(ctx, input.TenantSlug, input.UserPublicID, input.RoleCode, sessionAuditContext(ctx, current, nil)); err != nil {
			return nil, toTenantAdminHTTPError(err)
		}
		return &TenantAdminNoContentOutput{}, nil
	})
}

func requireTenantAdmin(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, error) {
	if deps.TenantAdminService == nil {
		return service.CurrentSession{}, huma.Error503ServiceUnavailable("tenant admin service is not configured")
	}

	var current service.CurrentSession
	var authCtx service.AuthContext
	var err error
	if csrfToken == "" {
		current, authCtx, err = currentSessionAuthContext(ctx, deps, sessionID)
	} else {
		current, authCtx, err = currentSessionAuthContextWithCSRF(ctx, deps, sessionID, csrfToken)
	}
	if err != nil {
		var statusErr huma.StatusError
		if errors.As(err, &statusErr) {
			return service.CurrentSession{}, err
		}
		return service.CurrentSession{}, toHTTPError(err)
	}
	if !authCtx.HasRole("tenant_admin") {
		return service.CurrentSession{}, huma.Error403Forbidden("tenant_admin role is required")
	}
	return current, nil
}

func tenantAdminTenantInputFromBody(body TenantAdminTenantRequestBody) service.TenantAdminTenantInput {
	return service.TenantAdminTenantInput{
		Slug:        body.Slug,
		DisplayName: body.DisplayName,
		Active:      body.Active,
	}
}

func toTenantAdminTenantBody(item service.TenantAdminTenant) TenantAdminTenantBody {
	return TenantAdminTenantBody{
		ID:                item.ID,
		Slug:              item.Slug,
		DisplayName:       item.DisplayName,
		Active:            item.Active,
		ActiveMemberCount: item.ActiveMemberCount,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func toTenantAdminTenantDetailBody(detail service.TenantAdminTenantDetail) TenantAdminTenantDetailBody {
	body := TenantAdminTenantDetailBody{
		Tenant:      toTenantAdminTenantBody(detail.Tenant),
		Memberships: make([]TenantAdminMembershipBody, 0, len(detail.Memberships)),
	}
	for _, item := range detail.Memberships {
		body.Memberships = append(body.Memberships, toTenantAdminMembershipBody(item))
	}
	return body
}

func toTenantAdminMembershipBody(item service.TenantAdminMembership) TenantAdminMembershipBody {
	body := TenantAdminMembershipBody{
		UserPublicID: item.UserPublicID,
		Email:        item.Email,
		DisplayName:  item.DisplayName,
		Deactivated:  item.Deactivated,
		Roles:        make([]TenantAdminRoleBindingBody, 0, len(item.Roles)),
	}
	for _, role := range item.Roles {
		body.Roles = append(body.Roles, TenantAdminRoleBindingBody{
			RoleCode: role.RoleCode,
			Source:   role.Source,
			Active:   role.Active,
		})
	}
	return body
}

func toTenantAdminHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrTenantAdminInvalidInput):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, service.ErrTenantAdminDuplicateTenant):
		return huma.Error409Conflict("tenant already exists")
	case errors.Is(err, service.ErrTenantAdminTenantNotFound):
		return huma.Error404NotFound("tenant not found")
	case errors.Is(err, service.ErrTenantAdminUserNotFound):
		return huma.Error404NotFound("user not found")
	case errors.Is(err, service.ErrTenantAdminUserInactive):
		return huma.Error400BadRequest("user is inactive")
	case errors.Is(err, service.ErrTenantAdminRoleNotFound):
		return huma.Error400BadRequest("unsupported tenant role")
	case errors.Is(err, service.ErrTenantAdminLocalRoleNotFound):
		return huma.Error404NotFound("local tenant role not found")
	default:
		return huma.Error500InternalServerError("internal server error")
	}
}
