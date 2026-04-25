package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type CustomerSignalImportJobBody struct {
	PublicID          string     `json:"publicId" format:"uuid"`
	Status            string     `json:"status"`
	ValidateOnly      bool       `json:"validateOnly"`
	InputFileObjectID int64      `json:"inputFileObjectId"`
	ErrorFileObjectID *int64     `json:"errorFileObjectId,omitempty"`
	TotalRows         int32      `json:"totalRows"`
	ValidRows         int32      `json:"validRows"`
	InvalidRows       int32      `json:"invalidRows"`
	InsertedRows      int32      `json:"insertedRows"`
	ErrorSummary      string     `json:"errorSummary,omitempty"`
	CreatedAt         time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt         time.Time  `json:"updatedAt" format:"date-time"`
	CompletedAt       *time.Time `json:"completedAt,omitempty" format:"date-time"`
}

type CustomerSignalImportRequestBody struct {
	InputFilePublicID string `json:"inputFilePublicId" format:"uuid"`
	ValidateOnly      bool   `json:"validateOnly,omitempty"`
}

type ListCustomerSignalImportsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
}

type CreateCustomerSignalImportInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          CustomerSignalImportRequestBody
}

type GetCustomerSignalImportInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	TenantSlug     string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	ImportPublicID string      `path:"importPublicId" format:"uuid"`
}

type CustomerSignalImportListOutput struct {
	Body struct {
		Items []CustomerSignalImportJobBody `json:"items"`
	}
}

type CustomerSignalImportOutput struct {
	Body CustomerSignalImportJobBody
}

func registerCustomerSignalImportRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{OperationID: "listCustomerSignalImports", Method: http.MethodGet, Path: "/api/v1/admin/tenants/{tenantSlug}/imports", Summary: "Customer Signals import job 一覧を返す", Tags: []string{"customer-signal-imports"}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *ListCustomerSignalImportsInput) (*CustomerSignalImportListOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		items, err := deps.CustomerSignalImportService.List(ctx, tenant.ID, 50)
		if err != nil {
			return nil, toCustomerSignalImportHTTPError(err)
		}
		out := &CustomerSignalImportListOutput{}
		out.Body.Items = make([]CustomerSignalImportJobBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toCustomerSignalImportJobBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createCustomerSignalImport", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/imports", Summary: "Customer Signals CSV import job を作成する", Tags: []string{"customer-signal-imports"}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *CreateCustomerSignalImportInput) (*CustomerSignalImportOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.CustomerSignalImportService.Create(ctx, tenant.ID, current.User.ID, service.CustomerSignalImportInput{
			InputFilePublicID: input.Body.InputFilePublicID,
			ValidateOnly:      input.Body.ValidateOnly,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toCustomerSignalImportHTTPError(err)
		}
		return &CustomerSignalImportOutput{Body: toCustomerSignalImportJobBody(item)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "getCustomerSignalImport", Method: http.MethodGet, Path: "/api/v1/admin/tenants/{tenantSlug}/imports/{importPublicId}", Summary: "Customer Signals import job detail を返す", Tags: []string{"customer-signal-imports"}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *GetCustomerSignalImportInput) (*CustomerSignalImportOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.CustomerSignalImportService.Get(ctx, tenant.ID, input.ImportPublicID)
		if err != nil {
			return nil, toCustomerSignalImportHTTPError(err)
		}
		return &CustomerSignalImportOutput{Body: toCustomerSignalImportJobBody(item)}, nil
	})
}

func toCustomerSignalImportJobBody(item service.CustomerSignalImportJob) CustomerSignalImportJobBody {
	return CustomerSignalImportJobBody{PublicID: item.PublicID, Status: item.Status, ValidateOnly: item.ValidateOnly, InputFileObjectID: item.InputFileObjectID, ErrorFileObjectID: item.ErrorFileObjectID, TotalRows: item.TotalRows, ValidRows: item.ValidRows, InvalidRows: item.InvalidRows, InsertedRows: item.InsertedRows, ErrorSummary: item.ErrorSummary, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, CompletedAt: item.CompletedAt}
}

func toCustomerSignalImportHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidCustomerSignalImport), errors.Is(err, service.ErrInvalidFileInput):
		return huma.Error400BadRequest("invalid customer signal import")
	case errors.Is(err, service.ErrCustomerSignalImportEntitled):
		return huma.Error403Forbidden("customer signal import/export entitlement is disabled")
	case errors.Is(err, service.ErrCustomerSignalImportNotFound), errors.Is(err, service.ErrFileNotFound):
		return huma.Error404NotFound("customer signal import not found")
	default:
		return toHTTPError(err)
	}
}
