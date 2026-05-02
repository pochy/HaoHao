package api

import (
	"context"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DatasetGoldPublicationBody struct {
	PublicID                string                     `json:"publicId" format:"uuid"`
	SourceWorkTablePublicID string                     `json:"sourceWorkTablePublicId,omitempty" format:"uuid"`
	DisplayName             string                     `json:"displayName" example:"Monthly sales mart"`
	Description             string                     `json:"description,omitempty"`
	GoldDatabase            string                     `json:"goldDatabase" example:"hh_t_1_gold"`
	GoldTable               string                     `json:"goldTable" example:"gm_monthly_sales"`
	Status                  string                     `json:"status" enum:"pending,active,failed,unpublished,archived" example:"active"`
	RowCount                int64                      `json:"rowCount" example:"1000"`
	TotalBytes              int64                      `json:"totalBytes" example:"1048576"`
	SchemaSummary           map[string]any             `json:"schemaSummary,omitempty"`
	RefreshPolicy           string                     `json:"refreshPolicy" enum:"manual" example:"manual"`
	LatestPublishRun        *DatasetGoldPublishRunBody `json:"latestPublishRun,omitempty"`
	CreatedAt               time.Time                  `json:"createdAt" format:"date-time"`
	UpdatedAt               time.Time                  `json:"updatedAt" format:"date-time"`
	PublishedAt             *time.Time                 `json:"publishedAt,omitempty" format:"date-time"`
	UnpublishedAt           *time.Time                 `json:"unpublishedAt,omitempty" format:"date-time"`
	ArchivedAt              *time.Time                 `json:"archivedAt,omitempty" format:"date-time"`
}

type DatasetGoldPublishRunBody struct {
	PublicID                string         `json:"publicId" format:"uuid"`
	PublicationPublicID     string         `json:"publicationPublicId,omitempty" format:"uuid"`
	SourceWorkTablePublicID string         `json:"sourceWorkTablePublicId,omitempty" format:"uuid"`
	Status                  string         `json:"status" enum:"pending,processing,completed,failed" example:"completed"`
	GoldDatabase            string         `json:"goldDatabase" example:"hh_t_1_gold"`
	GoldTable               string         `json:"goldTable" example:"gm_monthly_sales"`
	RowCount                int64          `json:"rowCount" example:"1000"`
	TotalBytes              int64          `json:"totalBytes" example:"1048576"`
	SchemaSummary           map[string]any `json:"schemaSummary,omitempty"`
	ErrorSummary            string         `json:"errorSummary,omitempty"`
	StartedAt               *time.Time     `json:"startedAt,omitempty" format:"date-time"`
	CompletedAt             *time.Time     `json:"completedAt,omitempty" format:"date-time"`
	CreatedAt               time.Time      `json:"createdAt" format:"date-time"`
	UpdatedAt               time.Time      `json:"updatedAt" format:"date-time"`
}

type DatasetGoldPublicationListBody struct {
	Items []DatasetGoldPublicationBody `json:"items"`
}

type DatasetGoldPublishRunListBody struct {
	Items []DatasetGoldPublishRunBody `json:"items"`
}

type DatasetGoldPublicationCreateBody struct {
	DisplayName string `json:"displayName,omitempty" example:"Monthly sales mart"`
	Description string `json:"description,omitempty"`
	GoldTable   string `json:"goldTable,omitempty" example:"gm_monthly_sales"`
}

type DatasetGoldPublicationPreviewBody struct {
	Database    string           `json:"database" example:"hh_t_1_gold"`
	Table       string           `json:"table" example:"gm_monthly_sales"`
	Columns     []string         `json:"columns"`
	PreviewRows []map[string]any `json:"previewRows"`
}

type DatasetGoldPublicationOutput struct {
	Body DatasetGoldPublicationBody
}

type DatasetGoldPublicationListOutput struct {
	Body DatasetGoldPublicationListBody
}

type DatasetGoldPublishRunOutput struct {
	Body DatasetGoldPublishRunBody
}

type DatasetGoldPublishRunListOutput struct {
	Body DatasetGoldPublishRunListBody
}

type DatasetGoldPublicationPreviewOutput struct {
	Body DatasetGoldPublicationPreviewBody
}

type DatasetGoldPublicationCreateInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token"`
	WorkTablePublicID string      `path:"workTablePublicId" format:"uuid"`
	Body              DatasetGoldPublicationCreateBody
}

type ListDatasetGoldPublicationsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"100"`
}

type DatasetGoldPublicationInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	GoldPublicID  string      `path:"goldPublicId" format:"uuid"`
}

type DatasetGoldPublicationPreviewInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	GoldPublicID  string      `path:"goldPublicId" format:"uuid"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"100"`
}

type DatasetGoldPublicationMutateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token"`
	GoldPublicID  string      `path:"goldPublicId" format:"uuid"`
}

type DatasetGoldPublicationRunsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	GoldPublicID  string      `path:"goldPublicId" format:"uuid"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"100"`
}

func registerDatasetGoldPublicationRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "createDatasetGoldPublication",
		Method:      http.MethodPost,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}/gold-publications",
		Summary:     "managed work table を Gold data mart として publish する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetGoldPublicationCreateInput) (*DatasetGoldPublicationOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.RequestGoldPublication(ctx, tenant.ID, current.User.ID, input.WorkTablePublicID, service.DatasetGoldPublicationInput{
			DisplayName: input.Body.DisplayName,
			Description: input.Body.Description,
			GoldTable:   input.Body.GoldTable,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "createDatasetGoldPublication", err)
		}
		return &DatasetGoldPublicationOutput{Body: toDatasetGoldPublicationBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetGoldPublications",
		Method:      http.MethodGet,
		Path:        "/api/v1/gold-publications",
		Summary:     "active tenant の Gold data mart 一覧を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDatasetGoldPublicationsInput) (*DatasetGoldPublicationListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.ListGoldPublications(ctx, tenant.ID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "listDatasetGoldPublications", err)
		}
		out := &DatasetGoldPublicationListOutput{}
		out.Body.Items = make([]DatasetGoldPublicationBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDatasetGoldPublicationBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDatasetGoldPublication",
		Method:      http.MethodGet,
		Path:        "/api/v1/gold-publications/{goldPublicId}",
		Summary:     "Gold data mart detail を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetGoldPublicationInput) (*DatasetGoldPublicationOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.GetGoldPublication(ctx, tenant.ID, input.GoldPublicID)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "getDatasetGoldPublication", err)
		}
		return &DatasetGoldPublicationOutput{Body: toDatasetGoldPublicationBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "previewDatasetGoldPublication",
		Method:      http.MethodGet,
		Path:        "/api/v1/gold-publications/{goldPublicId}/preview",
		Summary:     "Gold data mart preview rows を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetGoldPublicationPreviewInput) (*DatasetGoldPublicationPreviewOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.PreviewGoldPublication(ctx, tenant.ID, input.GoldPublicID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "previewDatasetGoldPublication", err)
		}
		return &DatasetGoldPublicationPreviewOutput{Body: DatasetGoldPublicationPreviewBody{
			Database:    item.Database,
			Table:       item.Table,
			Columns:     item.Columns,
			PreviewRows: item.PreviewRows,
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetGoldPublishRuns",
		Method:      http.MethodGet,
		Path:        "/api/v1/gold-publications/{goldPublicId}/publish-runs",
		Summary:     "Gold data mart publish history を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetGoldPublicationRunsInput) (*DatasetGoldPublishRunListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.ListGoldPublishRuns(ctx, tenant.ID, input.GoldPublicID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "listDatasetGoldPublishRuns", err)
		}
		out := &DatasetGoldPublishRunListOutput{}
		out.Body.Items = make([]DatasetGoldPublishRunBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDatasetGoldPublishRunBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "refreshDatasetGoldPublication",
		Method:      http.MethodPost,
		Path:        "/api/v1/gold-publications/{goldPublicId}/refresh",
		Summary:     "Gold data mart の full publish refresh を request する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetGoldPublicationMutateInput) (*DatasetGoldPublishRunOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.RequestGoldRefresh(ctx, tenant.ID, current.User.ID, input.GoldPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "refreshDatasetGoldPublication", err)
		}
		return &DatasetGoldPublishRunOutput{Body: toDatasetGoldPublishRunBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "unpublishDatasetGoldPublication",
		Method:      http.MethodPost,
		Path:        "/api/v1/gold-publications/{goldPublicId}/unpublish",
		Summary:     "Gold data mart を unpublish する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetGoldPublicationMutateInput) (*DatasetGoldPublicationOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.UnpublishGoldPublication(ctx, tenant.ID, current.User.ID, input.GoldPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "unpublishDatasetGoldPublication", err)
		}
		return &DatasetGoldPublicationOutput{Body: toDatasetGoldPublicationBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "archiveDatasetGoldPublication",
		Method:      http.MethodPost,
		Path:        "/api/v1/gold-publications/{goldPublicId}/archive",
		Summary:     "Gold data mart を archive する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetGoldPublicationMutateInput) (*DatasetGoldPublicationOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.ArchiveGoldPublication(ctx, tenant.ID, current.User.ID, input.GoldPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "archiveDatasetGoldPublication", err)
		}
		return &DatasetGoldPublicationOutput{Body: toDatasetGoldPublicationBody(item)}, nil
	})
}

func toDatasetGoldPublicationBody(item service.DatasetGoldPublication) DatasetGoldPublicationBody {
	body := DatasetGoldPublicationBody{
		PublicID:                item.PublicID,
		SourceWorkTablePublicID: item.SourceWorkTablePublicID,
		DisplayName:             item.DisplayName,
		Description:             item.Description,
		GoldDatabase:            item.GoldDatabase,
		GoldTable:               item.GoldTable,
		Status:                  item.Status,
		RowCount:                item.RowCount,
		TotalBytes:              item.TotalBytes,
		SchemaSummary:           item.SchemaSummary,
		RefreshPolicy:           item.RefreshPolicy,
		CreatedAt:               item.CreatedAt,
		UpdatedAt:               item.UpdatedAt,
		PublishedAt:             item.PublishedAt,
		UnpublishedAt:           item.UnpublishedAt,
		ArchivedAt:              item.ArchivedAt,
	}
	if item.LatestPublishRun != nil {
		run := toDatasetGoldPublishRunBody(*item.LatestPublishRun)
		body.LatestPublishRun = &run
	}
	return body
}

func toDatasetGoldPublishRunBody(item service.DatasetGoldPublishRun) DatasetGoldPublishRunBody {
	return DatasetGoldPublishRunBody{
		PublicID:                item.PublicID,
		PublicationPublicID:     item.PublicationPublicID,
		SourceWorkTablePublicID: item.SourceWorkTablePublicID,
		Status:                  item.Status,
		GoldDatabase:            item.GoldDatabase,
		GoldTable:               item.GoldTable,
		RowCount:                item.RowCount,
		TotalBytes:              item.TotalBytes,
		SchemaSummary:           item.SchemaSummary,
		ErrorSummary:            item.ErrorSummary,
		StartedAt:               item.StartedAt,
		CompletedAt:             item.CompletedAt,
		CreatedAt:               item.CreatedAt,
		UpdatedAt:               item.UpdatedAt,
	}
}
