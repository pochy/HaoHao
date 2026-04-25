package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type CustomerSignalBody struct {
	PublicID     string    `json:"publicId" format:"uuid" example:"018f2f05-c6c9-7a49-b32d-04f4dd84ef4a"`
	CustomerName string    `json:"customerName" minLength:"1" maxLength:"120" example:"Acme"`
	Title        string    `json:"title" minLength:"1" maxLength:"200" example:"Export CSV from reports"`
	Body         string    `json:"body" maxLength:"4000" example:"Customer asked for monthly report export."`
	Source       string    `json:"source" enum:"support,sales,customer_success,research,internal,other" example:"support"`
	Priority     string    `json:"priority" enum:"low,medium,high,urgent" example:"medium"`
	Status       string    `json:"status" enum:"new,triaged,planned,closed" example:"new"`
	CreatedAt    time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt    time.Time `json:"updatedAt" format:"date-time"`
}

type CustomerSignalListBody struct {
	Items []CustomerSignalBody `json:"items"`
}

type ListCustomerSignalsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type CustomerSignalListOutput struct {
	Body CustomerSignalListBody
}

type CreateCustomerSignalBody struct {
	CustomerName string `json:"customerName" minLength:"1" maxLength:"120" example:"Acme"`
	Title        string `json:"title" minLength:"1" maxLength:"200" example:"Export CSV from reports"`
	Body         string `json:"body,omitempty" maxLength:"4000" example:"Customer asked for monthly report export."`
	Source       string `json:"source,omitempty" enum:"support,sales,customer_success,research,internal,other" example:"support"`
	Priority     string `json:"priority,omitempty" enum:"low,medium,high,urgent" example:"medium"`
	Status       string `json:"status,omitempty" enum:"new,triaged,planned,closed" example:"new"`
}

type CreateCustomerSignalInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          CreateCustomerSignalBody
}

type CustomerSignalOutput struct {
	Body CustomerSignalBody
}

type GetCustomerSignalInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	SignalPublicID string      `path:"signalPublicId" format:"uuid"`
}

type UpdateCustomerSignalBody struct {
	CustomerName *string `json:"customerName,omitempty" minLength:"1" maxLength:"120" example:"Acme"`
	Title        *string `json:"title,omitempty" minLength:"1" maxLength:"200" example:"Export CSV from reports"`
	Body         *string `json:"body,omitempty" maxLength:"4000" example:"Customer asked for monthly report export."`
	Source       *string `json:"source,omitempty" enum:"support,sales,customer_success,research,internal,other" example:"support"`
	Priority     *string `json:"priority,omitempty" enum:"low,medium,high,urgent" example:"medium"`
	Status       *string `json:"status,omitempty" enum:"new,triaged,planned,closed" example:"triaged"`
}

type UpdateCustomerSignalInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	SignalPublicID string      `path:"signalPublicId" format:"uuid"`
	Body           UpdateCustomerSignalBody
}

type DeleteCustomerSignalInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	SignalPublicID string      `path:"signalPublicId" format:"uuid"`
}

type DeleteCustomerSignalOutput struct{}

func registerCustomerSignalRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listCustomerSignals",
		Method:      http.MethodGet,
		Path:        "/api/v1/customer-signals",
		Summary:     "active tenant の Customer Signals 一覧を返す",
		Tags:        []string{"customer-signals"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListCustomerSignalsInput) (*CustomerSignalListOutput, error) {
		_, tenant, err := requireCustomerSignalTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}

		items, err := deps.CustomerSignalService.List(ctx, tenant.ID)
		if err != nil {
			return nil, toCustomerSignalHTTPError(err)
		}

		out := &CustomerSignalListOutput{}
		out.Body.Items = make([]CustomerSignalBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toCustomerSignalBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createCustomerSignal",
		Method:      http.MethodPost,
		Path:        "/api/v1/customer-signals",
		Summary:     "active tenant に Customer Signal を作成する",
		Tags:        []string{"customer-signals"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *CreateCustomerSignalInput) (*CustomerSignalOutput, error) {
		current, tenant, err := requireCustomerSignalTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}

		item, err := deps.CustomerSignalService.Create(ctx, tenant.ID, current.User.ID, customerSignalCreateInputFromBody(input.Body), userAuditContext(ctx, current.User.ID, &tenant.ID))
		if err != nil {
			return nil, toCustomerSignalHTTPError(err)
		}
		return &CustomerSignalOutput{Body: toCustomerSignalBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getCustomerSignal",
		Method:      http.MethodGet,
		Path:        "/api/v1/customer-signals/{signalPublicId}",
		Summary:     "active tenant の Customer Signal detail を返す",
		Tags:        []string{"customer-signals"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *GetCustomerSignalInput) (*CustomerSignalOutput, error) {
		_, tenant, err := requireCustomerSignalTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}

		item, err := deps.CustomerSignalService.Get(ctx, tenant.ID, input.SignalPublicID)
		if err != nil {
			return nil, toCustomerSignalHTTPError(err)
		}
		return &CustomerSignalOutput{Body: toCustomerSignalBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateCustomerSignal",
		Method:      http.MethodPatch,
		Path:        "/api/v1/customer-signals/{signalPublicId}",
		Summary:     "active tenant の Customer Signal を更新する",
		Tags:        []string{"customer-signals"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *UpdateCustomerSignalInput) (*CustomerSignalOutput, error) {
		current, tenant, err := requireCustomerSignalTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}

		item, err := deps.CustomerSignalService.Update(ctx, tenant.ID, input.SignalPublicID, customerSignalUpdateInputFromBody(input.Body), userAuditContext(ctx, current.User.ID, &tenant.ID))
		if err != nil {
			return nil, toCustomerSignalHTTPError(err)
		}
		return &CustomerSignalOutput{Body: toCustomerSignalBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteCustomerSignal",
		Method:        http.MethodDelete,
		Path:          "/api/v1/customer-signals/{signalPublicId}",
		Summary:       "active tenant の Customer Signal を soft delete する",
		Tags:          []string{"customer-signals"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *DeleteCustomerSignalInput) (*DeleteCustomerSignalOutput, error) {
		current, tenant, err := requireCustomerSignalTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}

		if err := deps.CustomerSignalService.Delete(ctx, tenant.ID, input.SignalPublicID, userAuditContext(ctx, current.User.ID, &tenant.ID)); err != nil {
			return nil, toCustomerSignalHTTPError(err)
		}
		return &DeleteCustomerSignalOutput{}, nil
	})
}

func requireCustomerSignalTenant(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, service.TenantAccess, error) {
	if deps.CustomerSignalService == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error503ServiceUnavailable("customer signal service is not configured")
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
			return service.CurrentSession{}, service.TenantAccess{}, err
		}
		return service.CurrentSession{}, service.TenantAccess{}, toHTTPError(err)
	}
	if authCtx.ActiveTenant == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error409Conflict("active tenant is required")
	}
	if !tenantHasRole(*authCtx.ActiveTenant, "customer_signal_user") {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error403Forbidden("customer_signal_user tenant role is required")
	}
	return current, *authCtx.ActiveTenant, nil
}

func customerSignalCreateInputFromBody(body CreateCustomerSignalBody) service.CustomerSignalCreateInput {
	return service.CustomerSignalCreateInput{
		CustomerName: body.CustomerName,
		Title:        body.Title,
		Body:         body.Body,
		Source:       body.Source,
		Priority:     body.Priority,
		Status:       body.Status,
	}
}

func customerSignalUpdateInputFromBody(body UpdateCustomerSignalBody) service.CustomerSignalUpdateInput {
	return service.CustomerSignalUpdateInput{
		CustomerName: body.CustomerName,
		Title:        body.Title,
		Body:         body.Body,
		Source:       body.Source,
		Priority:     body.Priority,
		Status:       body.Status,
	}
}

func toCustomerSignalBody(item service.CustomerSignal) CustomerSignalBody {
	return CustomerSignalBody{
		PublicID:     item.PublicID,
		CustomerName: item.CustomerName,
		Title:        item.Title,
		Body:         item.Body,
		Source:       item.Source,
		Priority:     item.Priority,
		Status:       item.Status,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func toCustomerSignalHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidCustomerSignalInput):
		return huma.Error400BadRequest("invalid customer signal input")
	case errors.Is(err, service.ErrInvalidCustomerSignalUpdate):
		return huma.Error400BadRequest("invalid customer signal update")
	case errors.Is(err, service.ErrCustomerSignalNotFound):
		return huma.Error404NotFound("customer signal not found")
	default:
		return toHTTPError(err)
	}
}
