package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type MedallionAssetBody struct {
	PublicID         string         `json:"publicId" format:"uuid"`
	Layer            string         `json:"layer" enum:"bronze,silver,gold" example:"bronze"`
	ResourceKind     string         `json:"resourceKind" example:"drive_file"`
	ResourcePublicID string         `json:"resourcePublicId" format:"uuid"`
	DisplayName      string         `json:"displayName" example:"customers.csv"`
	Status           string         `json:"status" enum:"active,building,failed,skipped,archived" example:"active"`
	RowCount         *int64         `json:"rowCount,omitempty" example:"1000"`
	ByteSize         *int64         `json:"byteSize,omitempty" example:"1048576"`
	SchemaSummary    map[string]any `json:"schemaSummary,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"createdAt" format:"date-time"`
	UpdatedAt        time.Time      `json:"updatedAt" format:"date-time"`
	ArchivedAt       *time.Time     `json:"archivedAt,omitempty" format:"date-time"`
}

type MedallionPipelineRunBody struct {
	PublicID               string         `json:"publicId" format:"uuid"`
	PipelineType           string         `json:"pipelineType" example:"drive_ocr"`
	Status                 string         `json:"status" enum:"pending,processing,completed,failed,skipped" example:"completed"`
	Runtime                string         `json:"runtime,omitempty" example:"clickhouse"`
	TriggerKind            string         `json:"triggerKind" enum:"manual,upload,scheduled,system,read_repair" example:"manual"`
	Retryable              bool           `json:"retryable"`
	ErrorSummary           string         `json:"errorSummary,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
	StartedAt              *time.Time     `json:"startedAt,omitempty" format:"date-time"`
	CompletedAt            *time.Time     `json:"completedAt,omitempty" format:"date-time"`
	CreatedAt              time.Time      `json:"createdAt" format:"date-time"`
	UpdatedAt              time.Time      `json:"updatedAt" format:"date-time"`
	SourceAssetPublicIDs   []string       `json:"sourceAssetPublicIds"`
	TargetAssetPublicIDs   []string       `json:"targetAssetPublicIds"`
	SourceResourceKind     string         `json:"sourceResourceKind,omitempty"`
	SourceResourcePublicID string         `json:"sourceResourcePublicId,omitempty" format:"uuid"`
	TargetResourceKind     string         `json:"targetResourceKind,omitempty"`
	TargetResourcePublicID string         `json:"targetResourcePublicId,omitempty" format:"uuid"`
}

type MedallionCatalogBody struct {
	Asset        MedallionAssetBody         `json:"asset"`
	Upstream     []MedallionAssetBody       `json:"upstream"`
	Downstream   []MedallionAssetBody       `json:"downstream"`
	PipelineRuns []MedallionPipelineRunBody `json:"pipelineRuns"`
}

type MedallionAssetListBody struct {
	Items []MedallionAssetBody `json:"items"`
}

type ListMedallionAssetsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Query         string      `query:"q"`
	Layer         string      `query:"layer" enum:"bronze,silver,gold"`
	ResourceKind  string      `query:"resourceKind" enum:"drive_file,dataset,work_table,ocr_run,product_extraction,gold_table"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"200"`
}

type GetMedallionAssetInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	AssetPublicID string      `path:"assetPublicId" format:"uuid"`
}

type GetMedallionResourceInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	ResourceKind     string      `path:"resourceKind" enum:"drive_file,dataset,work_table,ocr_run,product_extraction,gold_table"`
	ResourcePublicID string      `path:"resourcePublicId" format:"uuid"`
}

type MedallionAssetListOutput struct {
	Body MedallionAssetListBody
}

type MedallionCatalogOutput struct {
	Body MedallionCatalogBody
}

func registerMedallionCatalogRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listMedallionAssets",
		Method:      http.MethodGet,
		Path:        "/api/v1/medallion/assets",
		Summary:     "active tenant の Medallion catalog asset 一覧を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListMedallionAssetsInput) (*MedallionAssetListOutput, error) {
		current, tenant, err := requireMedallionTenant(ctx, deps, input.SessionCookie.Value)
		if err != nil {
			return nil, err
		}
		items, err := deps.MedallionCatalogService.ListAssets(ctx, tenant.ID, current.User.ID, input.Layer, input.ResourceKind, input.Query, input.Limit)
		if err != nil {
			return nil, toMedallionHTTPError(ctx, deps, "listMedallionAssets", err)
		}
		out := &MedallionAssetListOutput{}
		out.Body.Items = make([]MedallionAssetBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toMedallionAssetBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getMedallionAsset",
		Method:      http.MethodGet,
		Path:        "/api/v1/medallion/assets/{assetPublicId}",
		Summary:     "Medallion catalog asset detail を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *GetMedallionAssetInput) (*MedallionCatalogOutput, error) {
		current, tenant, err := requireMedallionTenant(ctx, deps, input.SessionCookie.Value)
		if err != nil {
			return nil, err
		}
		catalog, err := deps.MedallionCatalogService.GetAssetCatalog(ctx, tenant.ID, current.User.ID, input.AssetPublicID)
		if err != nil {
			return nil, toMedallionHTTPError(ctx, deps, "getMedallionAsset", err)
		}
		return &MedallionCatalogOutput{Body: toMedallionCatalogBody(catalog)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getMedallionResourceCatalog",
		Method:      http.MethodGet,
		Path:        "/api/v1/medallion/resources/{resourceKind}/{resourcePublicId}",
		Summary:     "resource public ID から Medallion catalog summary を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *GetMedallionResourceInput) (*MedallionCatalogOutput, error) {
		current, tenant, err := requireMedallionTenant(ctx, deps, input.SessionCookie.Value)
		if err != nil {
			return nil, err
		}
		catalog, err := deps.MedallionCatalogService.GetResourceCatalog(ctx, tenant.ID, current.User.ID, input.ResourceKind, input.ResourcePublicID)
		if err != nil {
			return nil, toMedallionHTTPError(ctx, deps, "getMedallionResourceCatalog", err)
		}
		return &MedallionCatalogOutput{Body: toMedallionCatalogBody(catalog)}, nil
	})
}

func requireMedallionTenant(ctx context.Context, deps Dependencies, sessionID string) (service.CurrentSession, service.TenantAccess, error) {
	if deps.MedallionCatalogService == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error503ServiceUnavailable("medallion catalog service is not configured")
	}
	return requireActiveTenantRole(ctx, deps, sessionID, "", "", "medallion catalog service")
}

func toMedallionCatalogBody(item service.MedallionCatalog) MedallionCatalogBody {
	body := MedallionCatalogBody{
		Upstream:     make([]MedallionAssetBody, 0, len(item.Upstream)),
		Downstream:   make([]MedallionAssetBody, 0, len(item.Downstream)),
		PipelineRuns: make([]MedallionPipelineRunBody, 0, len(item.PipelineRuns)),
	}
	if item.Asset != nil {
		body.Asset = toMedallionAssetBody(*item.Asset)
	}
	for _, asset := range item.Upstream {
		body.Upstream = append(body.Upstream, toMedallionAssetBody(asset))
	}
	for _, asset := range item.Downstream {
		body.Downstream = append(body.Downstream, toMedallionAssetBody(asset))
	}
	for _, run := range item.PipelineRuns {
		body.PipelineRuns = append(body.PipelineRuns, toMedallionPipelineRunBody(run))
	}
	return body
}

func toMedallionAssetBody(item service.MedallionAsset) MedallionAssetBody {
	return MedallionAssetBody{
		PublicID:         item.PublicID,
		Layer:            item.Layer,
		ResourceKind:     item.ResourceKind,
		ResourcePublicID: item.ResourcePublicID,
		DisplayName:      item.DisplayName,
		Status:           item.Status,
		RowCount:         item.RowCount,
		ByteSize:         item.ByteSize,
		SchemaSummary:    item.SchemaSummary,
		Metadata:         item.Metadata,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
		ArchivedAt:       item.ArchivedAt,
	}
}

func toMedallionPipelineRunBody(item service.MedallionPipelineRun) MedallionPipelineRunBody {
	return MedallionPipelineRunBody{
		PublicID:               item.PublicID,
		PipelineType:           item.PipelineType,
		Status:                 item.Status,
		Runtime:                item.Runtime,
		TriggerKind:            item.TriggerKind,
		Retryable:              item.Retryable,
		ErrorSummary:           item.ErrorSummary,
		Metadata:               item.Metadata,
		StartedAt:              item.StartedAt,
		CompletedAt:            item.CompletedAt,
		CreatedAt:              item.CreatedAt,
		UpdatedAt:              item.UpdatedAt,
		SourceAssetPublicIDs:   item.SourceAssetPublicIDs,
		TargetAssetPublicIDs:   item.TargetAssetPublicIDs,
		SourceResourceKind:     item.SourceResourceKind,
		SourceResourcePublicID: item.SourceResourcePublicID,
		TargetResourceKind:     item.TargetResourceKind,
		TargetResourcePublicID: item.TargetResourcePublicID,
	}
}

func toMedallionHTTPError(ctx context.Context, deps Dependencies, operation string, err error) error {
	switch {
	case errors.Is(err, service.ErrMedallionAssetNotFound):
		return huma.Error404NotFound("medallion asset not found")
	case errors.Is(err, service.ErrDriveNotFound), errors.Is(err, service.ErrDatasetNotFound), errors.Is(err, service.ErrDatasetWorkTableNotFound):
		return huma.Error404NotFound("resource not found")
	case errors.Is(err, service.ErrDrivePermissionDenied), errors.Is(err, service.ErrDrivePolicyDenied):
		return huma.Error403Forbidden("resource access denied")
	case errors.Is(err, service.ErrDriveInvalidInput), errors.Is(err, service.ErrInvalidDatasetInput):
		return huma.Error400BadRequest(err.Error())
	default:
		return toHTTPErrorWithLog(ctx, deps, operation, err)
	}
}
