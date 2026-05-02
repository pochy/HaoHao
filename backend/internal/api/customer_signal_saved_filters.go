package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type CustomerSignalSavedFilterBody struct {
	PublicID  string         `json:"publicId" format:"uuid"`
	Name      string         `json:"name"`
	Query     string         `json:"query"`
	Filters   map[string]any `json:"filters"`
	CreatedAt time.Time      `json:"createdAt" format:"date-time"`
	UpdatedAt time.Time      `json:"updatedAt" format:"date-time"`
}

type CustomerSignalSavedFilterRequestBody struct {
	Name    string         `json:"name" minLength:"1" maxLength:"120"`
	Query   string         `json:"query,omitempty" maxLength:"200"`
	Filters map[string]any `json:"filters,omitempty"`
}

type ListCustomerSignalSavedFiltersInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type CreateCustomerSignalSavedFilterInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          CustomerSignalSavedFilterRequestBody
}

type UpdateCustomerSignalSavedFilterInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FilterPublicID string      `path:"filterPublicId" format:"uuid"`
	Body           CustomerSignalSavedFilterRequestBody
}

type DeleteCustomerSignalSavedFilterInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FilterPublicID string      `path:"filterPublicId" format:"uuid"`
}

type CustomerSignalSavedFilterListOutput struct {
	Body struct {
		Items []CustomerSignalSavedFilterBody `json:"items"`
	}
}

type CustomerSignalSavedFilterOutput struct {
	Body CustomerSignalSavedFilterBody
}

type CustomerSignalSavedFilterNoContentOutput struct{}

func registerCustomerSignalSavedFilterRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{OperationID: "listCustomerSignalSavedFilters", Method: http.MethodGet, Path: "/api/v1/customer-signal-filters", Summary: "Customer Signal saved filters を返す", Tags: []string{DocTagCustomerSignals}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *ListCustomerSignalSavedFiltersInput) (*CustomerSignalSavedFilterListOutput, error) {
		current, tenant, err := requireCustomerSignalTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.CustomerSignalSavedFilterService.List(ctx, tenant.ID, current.User.ID)
		if err != nil {
			return nil, toCustomerSignalSavedFilterHTTPError(err)
		}
		out := &CustomerSignalSavedFilterListOutput{}
		out.Body.Items = make([]CustomerSignalSavedFilterBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toCustomerSignalSavedFilterBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createCustomerSignalSavedFilter", Method: http.MethodPost, Path: "/api/v1/customer-signal-filters", Summary: "Customer Signal saved filter を作成する", Tags: []string{DocTagCustomerSignals}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *CreateCustomerSignalSavedFilterInput) (*CustomerSignalSavedFilterOutput, error) {
		current, tenant, err := requireCustomerSignalTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.CustomerSignalSavedFilterService.Create(ctx, tenant.ID, current.User.ID, savedFilterInputFromBody(input.Body), sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toCustomerSignalSavedFilterHTTPError(err)
		}
		return &CustomerSignalSavedFilterOutput{Body: toCustomerSignalSavedFilterBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "updateCustomerSignalSavedFilter", Method: http.MethodPut, Path: "/api/v1/customer-signal-filters/{filterPublicId}", Summary: "Customer Signal saved filter を更新する", Tags: []string{DocTagCustomerSignals}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *UpdateCustomerSignalSavedFilterInput) (*CustomerSignalSavedFilterOutput, error) {
		current, tenant, err := requireCustomerSignalTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.CustomerSignalSavedFilterService.Update(ctx, tenant.ID, current.User.ID, input.FilterPublicID, savedFilterInputFromBody(input.Body), sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toCustomerSignalSavedFilterHTTPError(err)
		}
		return &CustomerSignalSavedFilterOutput{Body: toCustomerSignalSavedFilterBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "deleteCustomerSignalSavedFilter", Method: http.MethodDelete, Path: "/api/v1/customer-signal-filters/{filterPublicId}", Summary: "Customer Signal saved filter を削除する", Tags: []string{DocTagCustomerSignals}, DefaultStatus: http.StatusNoContent, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *DeleteCustomerSignalSavedFilterInput) (*CustomerSignalSavedFilterNoContentOutput, error) {
		current, tenant, err := requireCustomerSignalTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.CustomerSignalSavedFilterService.Delete(ctx, tenant.ID, current.User.ID, input.FilterPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toCustomerSignalSavedFilterHTTPError(err)
		}
		return &CustomerSignalSavedFilterNoContentOutput{}, nil
	})
}

func savedFilterInputFromBody(body CustomerSignalSavedFilterRequestBody) service.CustomerSignalSavedFilterInput {
	return service.CustomerSignalSavedFilterInput{Name: body.Name, Query: body.Query, Filters: body.Filters}
}

func toCustomerSignalSavedFilterBody(item service.CustomerSignalSavedFilter) CustomerSignalSavedFilterBody {
	return CustomerSignalSavedFilterBody{PublicID: item.PublicID, Name: item.Name, Query: item.Query, Filters: item.Filters, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func toCustomerSignalSavedFilterHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidCustomerSignalSavedFilter):
		return huma.Error400BadRequest("invalid customer signal saved filter")
	case errors.Is(err, service.ErrSavedFilterEntitlementDenied):
		return huma.Error403Forbidden("saved filters entitlement is disabled")
	case errors.Is(err, service.ErrCustomerSignalSavedFilterNotFound):
		return huma.Error404NotFound("customer signal saved filter not found")
	default:
		return toHTTPError(err)
	}
}
