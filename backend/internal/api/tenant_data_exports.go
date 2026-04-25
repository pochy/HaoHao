package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type TenantDataExportBody struct {
	PublicID     string     `json:"publicId" format:"uuid"`
	TenantID     int64      `json:"tenantId"`
	Format       string     `json:"format" example:"json"`
	Status       string     `json:"status" example:"processing"`
	ErrorSummary string     `json:"errorSummary,omitempty"`
	FileObjectID *int64     `json:"fileObjectId,omitempty"`
	ExpiresAt    time.Time  `json:"expiresAt" format:"date-time"`
	CreatedAt    time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt    time.Time  `json:"updatedAt" format:"date-time"`
	CompletedAt  *time.Time `json:"completedAt,omitempty" format:"date-time"`
}

type ListTenantDataExportsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
}

type TenantDataExportListOutput struct {
	Body struct {
		Items []TenantDataExportBody `json:"items"`
	}
}

type CreateTenantDataExportRequestBody struct {
	Format string `json:"format,omitempty" enum:"json,csv" example:"json"`
}

type CreateTenantDataExportInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	IdempotencyKey string      `header:"Idempotency-Key"`
	TenantSlug     string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body           CreateTenantDataExportRequestBody
}

type GetTenantDataExportInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	TenantSlug     string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	ExportPublicID string      `path:"exportPublicId" format:"uuid"`
}

type TenantDataExportOutput struct {
	Body TenantDataExportBody
}

type DownloadTenantDataExportOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	Body               []byte
}

func registerTenantDataExportRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listTenantDataExports",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/exports",
		Summary:     "tenant data export 一覧を返す",
		Tags:        []string{"tenant-data-exports"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListTenantDataExportsInput) (*TenantDataExportListOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if deps.TenantDataExportService == nil {
			return nil, huma.Error503ServiceUnavailable("tenant data export service is not configured")
		}
		items, err := deps.TenantDataExportService.List(ctx, tenant.ID, 50)
		if err != nil {
			return nil, toTenantDataExportHTTPError(err)
		}
		out := &TenantDataExportListOutput{}
		out.Body.Items = make([]TenantDataExportBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toTenantDataExportBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createTenantDataExport",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/exports",
		Summary:     "tenant data export を request する",
		Tags:        []string{"tenant-data-exports"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *CreateTenantDataExportInput) (*TenantDataExportOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if deps.TenantDataExportService == nil {
			return nil, huma.Error503ServiceUnavailable("tenant data export service is not configured")
		}
		attempt, err := beginIdempotency(ctx, deps, input.IdempotencyKey, http.MethodPost, "/api/v1/admin/tenants/{tenantSlug}/exports", current.User.ID, &tenant.ID, input.Body)
		if err != nil {
			return nil, toIdempotencyHTTPError(err)
		}
		if attempt.Replay {
			body, err := replayIdempotencyBody[TenantDataExportBody](attempt)
			if err != nil {
				return nil, err
			}
			return &TenantDataExportOutput{Body: body}, nil
		}
		item, err := deps.TenantDataExportService.Create(ctx, tenant.ID, current.User.ID, input.Body.Format, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toTenantDataExportHTTPError(err)
		}
		body := toTenantDataExportBody(item)
		if err := completeIdempotency(ctx, deps, attempt, http.StatusOK, body); err != nil {
			return nil, err
		}
		return &TenantDataExportOutput{Body: body}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getTenantDataExport",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/exports/{exportPublicId}",
		Summary:     "tenant data export detail を返す",
		Tags:        []string{"tenant-data-exports"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *GetTenantDataExportInput) (*TenantDataExportOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		item, err := deps.TenantDataExportService.Get(ctx, tenant.ID, input.ExportPublicID)
		if err != nil {
			return nil, toTenantDataExportHTTPError(err)
		}
		return &TenantDataExportOutput{Body: toTenantDataExportBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "downloadTenantDataExport",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/exports/{exportPublicId}/download",
		Summary:     "ready tenant data export を download する",
		Tags:        []string{"tenant-data-exports"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *GetTenantDataExportInput) (*DownloadTenantDataExportOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		download, err := deps.TenantDataExportService.Download(ctx, tenant.ID, input.ExportPublicID)
		if err != nil {
			return nil, toTenantDataExportHTTPError(err)
		}
		defer download.Body.Close()
		body, err := io.ReadAll(download.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to read export body")
		}
		return &DownloadTenantDataExportOutput{
			ContentType:        download.File.ContentType,
			ContentDisposition: fmt.Sprintf("attachment; filename=%q", download.File.OriginalFilename),
			Body:               body,
		}, nil
	})
}

func toTenantDataExportBody(item service.TenantDataExport) TenantDataExportBody {
	return TenantDataExportBody{
		PublicID:     item.PublicID,
		TenantID:     item.TenantID,
		Format:       item.Format,
		Status:       item.Status,
		ErrorSummary: item.ErrorSummary,
		FileObjectID: item.FileObjectID,
		ExpiresAt:    item.ExpiresAt,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
		CompletedAt:  item.CompletedAt,
	}
}

func toTenantDataExportHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidTenantDataExport):
		return huma.Error400BadRequest("invalid tenant data export")
	case errors.Is(err, service.ErrTenantDataExportNotFound):
		return huma.Error404NotFound("tenant data export not found")
	case errors.Is(err, service.ErrTenantDataExportNotReady):
		return huma.Error409Conflict("tenant data export is not ready")
	case errors.Is(err, service.ErrCustomerSignalImportEntitled):
		return huma.Error403Forbidden("customer signal import/export entitlement is disabled")
	default:
		return toHTTPError(err)
	}
}
