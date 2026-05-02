package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DatasetColumnBody struct {
	Ordinal        int32  `json:"ordinal" example:"1"`
	OriginalName   string `json:"originalName" example:"Customer ID"`
	ColumnName     string `json:"columnName" example:"customer_id"`
	ClickHouseType string `json:"clickHouseType" example:"Nullable(String)"`
}

type DatasetImportErrorBody struct {
	RowNumber int64  `json:"rowNumber" example:"10"`
	Error     string `json:"error" example:"expected 5 columns, got 4"`
	Raw       string `json:"raw,omitempty"`
}

type DatasetImportJobBody struct {
	PublicID     string                   `json:"publicId" format:"uuid"`
	Status       string                   `json:"status" example:"processing"`
	TotalRows    int64                    `json:"totalRows" example:"10000000"`
	ValidRows    int64                    `json:"validRows" example:"9999990"`
	InvalidRows  int64                    `json:"invalidRows" example:"10"`
	ErrorSample  []DatasetImportErrorBody `json:"errorSample"`
	ErrorSummary string                   `json:"errorSummary,omitempty"`
	CreatedAt    time.Time                `json:"createdAt" format:"date-time"`
	UpdatedAt    time.Time                `json:"updatedAt" format:"date-time"`
	CompletedAt  *time.Time               `json:"completedAt,omitempty" format:"date-time"`
}

type DatasetBody struct {
	PublicID         string                `json:"publicId" format:"uuid"`
	Name             string                `json:"name" example:"Customers"`
	OriginalFilename string                `json:"originalFilename" example:"customers.csv"`
	ContentType      string                `json:"contentType" example:"text/csv"`
	ByteSize         int64                 `json:"byteSize" example:"104857600"`
	RawDatabase      string                `json:"rawDatabase" example:"hh_t_1_raw"`
	RawTable         string                `json:"rawTable" example:"ds_018f2f05c6c97a49b32d04f4dd84ef4a"`
	WorkDatabase     string                `json:"workDatabase" example:"hh_t_1_work"`
	Status           string                `json:"status" example:"ready"`
	RowCount         int64                 `json:"rowCount" example:"10000000"`
	ErrorSummary     string                `json:"errorSummary,omitempty"`
	Columns          []DatasetColumnBody   `json:"columns"`
	ImportJob        *DatasetImportJobBody `json:"importJob,omitempty"`
	CreatedAt        time.Time             `json:"createdAt" format:"date-time"`
	UpdatedAt        time.Time             `json:"updatedAt" format:"date-time"`
	ImportedAt       *time.Time            `json:"importedAt,omitempty" format:"date-time"`
}

type DatasetListBody struct {
	Items []DatasetBody `json:"items"`
}

type DatasetSourceFileBody struct {
	PublicID         string    `json:"publicId" format:"uuid"`
	OriginalFilename string    `json:"originalFilename" example:"customers.csv"`
	ContentType      string    `json:"contentType" example:"text/csv"`
	ByteSize         int64     `json:"byteSize" example:"104857600"`
	SHA256Hex        string    `json:"sha256Hex"`
	UpdatedAt        time.Time `json:"updatedAt" format:"date-time"`
	CreatedAt        time.Time `json:"createdAt" format:"date-time"`
}

type DatasetSourceFileListBody struct {
	Items []DatasetSourceFileBody `json:"items"`
}

type DatasetListOutput struct {
	Body DatasetListBody
}

type DatasetSourceFileListOutput struct {
	Body DatasetSourceFileListBody
}

type DatasetOutput struct {
	Body DatasetBody
}

type DatasetCreateBody struct {
	DriveFilePublicID string `json:"driveFilePublicId" format:"uuid"`
	Name              string `json:"name,omitempty" maxLength:"160"`
}

type DatasetCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          DatasetCreateBody
}

type ListDatasetsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"200" default:"100"`
}

type ListDatasetSourceFilesInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Query         string      `query:"q"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"200" default:"100"`
}

type DatasetByPublicIDInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	DatasetPublicID string      `path:"datasetPublicId" format:"uuid"`
}

type DeleteDatasetInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	DatasetPublicID string      `path:"datasetPublicId" format:"uuid"`
}

type DeleteDatasetOutput struct{}

type DatasetQueryCreateBody struct {
	Statement string `json:"statement" example:"SELECT count() FROM hh_t_1_raw.ds_abc"`
}

type DatasetQueryCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          DatasetQueryCreateBody
}

type DatasetQueryJobBody struct {
	PublicID      string           `json:"publicId" format:"uuid"`
	Statement     string           `json:"statement"`
	Status        string           `json:"status" example:"completed"`
	ResultColumns []string         `json:"resultColumns"`
	ResultRows    []map[string]any `json:"resultRows"`
	RowCount      int32            `json:"rowCount" example:"1000"`
	ErrorSummary  string           `json:"errorSummary,omitempty"`
	DurationMs    int64            `json:"durationMs" example:"120"`
	CreatedAt     time.Time        `json:"createdAt" format:"date-time"`
	UpdatedAt     time.Time        `json:"updatedAt" format:"date-time"`
	CompletedAt   *time.Time       `json:"completedAt,omitempty" format:"date-time"`
}

type DatasetQueryOutput struct {
	Body DatasetQueryJobBody
}

type DatasetQueryListBody struct {
	Items []DatasetQueryJobBody `json:"items"`
}

type DatasetQueryListOutput struct {
	Body DatasetQueryListBody
}

type ListDatasetQueryJobsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"100" default:"25"`
}

type DatasetQueryByPublicIDInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	QueryJobPublicID string      `path:"queryJobPublicId" format:"uuid"`
}

func registerDatasetRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "createDataset",
		Method:      http.MethodPost,
		Path:        "/api/v1/datasets",
		Summary:     "Drive CSV file から active tenant の dataset を作成する",
		Tags:        []string{"datasets"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetCreateInput) (*DatasetOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if deps.DriveService == nil {
			return nil, huma.Error503ServiceUnavailable("drive service is not configured")
		}
		auditCtx := sessionAuditContext(ctx, current, &tenant.ID)
		file, err := deps.DriveService.PrepareDatasetSourceFile(ctx, tenant.ID, current.User.ID, input.Body.DriveFilePublicID, auditCtx)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		item, err := deps.DatasetService.CreateFromDriveFile(ctx, tenant.ID, current.User.ID, file, input.Body.Name, auditCtx)
		if err != nil {
			return nil, toDatasetHTTPError(err)
		}
		return &DatasetOutput{Body: toDatasetBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasets",
		Method:      http.MethodGet,
		Path:        "/api/v1/datasets",
		Summary:     "active tenant の dataset 一覧を返す",
		Tags:        []string{"datasets"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDatasetsInput) (*DatasetListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.List(ctx, tenant.ID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(err)
		}
		out := &DatasetListOutput{}
		out.Body.Items = make([]DatasetBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDatasetBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetSourceFiles",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-source-files",
		Summary:     "Dataset に取り込める Drive CSV file 一覧を返す",
		Tags:        []string{"datasets"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDatasetSourceFilesInput) (*DatasetSourceFileListOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		if deps.DriveService == nil {
			return nil, huma.Error503ServiceUnavailable("drive service is not configured")
		}
		files, err := deps.DriveService.ListDatasetSourceFiles(ctx, service.DriveListDatasetSourceFilesInput{
			TenantID:    tenant.ID,
			ActorUserID: current.User.ID,
			Query:       input.Query,
			Limit:       input.Limit,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		out := &DatasetSourceFileListOutput{}
		out.Body.Items = make([]DatasetSourceFileBody, 0, len(files))
		for _, file := range files {
			out.Body.Items = append(out.Body.Items, toDatasetSourceFileBody(file))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDataset",
		Method:      http.MethodGet,
		Path:        "/api/v1/datasets/{datasetPublicId}",
		Summary:     "active tenant の dataset detail を返す",
		Tags:        []string{"datasets"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetByPublicIDInput) (*DatasetOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.Get(ctx, tenant.ID, input.DatasetPublicID)
		if err != nil {
			return nil, toDatasetHTTPError(err)
		}
		return &DatasetOutput{Body: toDatasetBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDataset",
		Method:        http.MethodDelete,
		Path:          "/api/v1/datasets/{datasetPublicId}",
		Summary:       "active tenant の dataset を削除する",
		Tags:          []string{"datasets"},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DeleteDatasetInput) (*DeleteDatasetOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DatasetService.Delete(ctx, tenant.ID, input.DatasetPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDatasetHTTPError(err)
		}
		return &DeleteDatasetOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDatasetQueryJob",
		Method:      http.MethodPost,
		Path:        "/api/v1/dataset-query-jobs",
		Summary:     "active tenant の dataset SQL query job を作成する",
		Tags:        []string{"datasets"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetQueryCreateInput) (*DatasetQueryOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.CreateQueryJob(ctx, tenant.ID, current.User.ID, input.Body.Statement)
		if err != nil {
			return nil, toDatasetHTTPError(err)
		}
		return &DatasetQueryOutput{Body: toDatasetQueryJobBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetQueryJobs",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-query-jobs",
		Summary:     "active tenant の dataset query job 一覧を返す",
		Tags:        []string{"datasets"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDatasetQueryJobsInput) (*DatasetQueryListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.ListQueryJobs(ctx, tenant.ID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(err)
		}
		out := &DatasetQueryListOutput{}
		out.Body.Items = make([]DatasetQueryJobBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDatasetQueryJobBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDatasetQueryJob",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-query-jobs/{queryJobPublicId}",
		Summary:     "active tenant の dataset query job detail を返す",
		Tags:        []string{"datasets"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetQueryByPublicIDInput) (*DatasetQueryOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.GetQueryJob(ctx, tenant.ID, input.QueryJobPublicID)
		if err != nil {
			return nil, toDatasetHTTPError(err)
		}
		return &DatasetQueryOutput{Body: toDatasetQueryJobBody(item)}, nil
	})
}

func requireDatasetTenant(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, service.TenantAccess, error) {
	if deps.DatasetService == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error503ServiceUnavailable("dataset service is not configured")
	}
	return requireActiveTenantRole(ctx, deps, sessionID, csrfToken, "", "dataset service")
}

func toDatasetBody(item service.Dataset) DatasetBody {
	body := DatasetBody{
		PublicID:         item.PublicID,
		Name:             item.Name,
		OriginalFilename: item.OriginalFilename,
		ContentType:      item.ContentType,
		ByteSize:         item.ByteSize,
		RawDatabase:      item.RawDatabase,
		RawTable:         item.RawTable,
		WorkDatabase:     item.WorkDatabase,
		Status:           item.Status,
		RowCount:         item.RowCount,
		ErrorSummary:     item.ErrorSummary,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
		ImportedAt:       item.ImportedAt,
		Columns:          make([]DatasetColumnBody, 0, len(item.Columns)),
	}
	for _, column := range item.Columns {
		body.Columns = append(body.Columns, DatasetColumnBody{
			Ordinal:        column.Ordinal,
			OriginalName:   column.OriginalName,
			ColumnName:     column.ColumnName,
			ClickHouseType: column.ClickHouseType,
		})
	}
	if item.ImportJob != nil {
		importJob := toDatasetImportJobBody(*item.ImportJob)
		body.ImportJob = &importJob
	}
	return body
}

func toDatasetSourceFileBody(item service.DriveFile) DatasetSourceFileBody {
	return DatasetSourceFileBody{
		PublicID:         item.PublicID,
		OriginalFilename: item.OriginalFilename,
		ContentType:      item.ContentType,
		ByteSize:         item.ByteSize,
		SHA256Hex:        item.SHA256Hex,
		UpdatedAt:        item.UpdatedAt,
		CreatedAt:        item.CreatedAt,
	}
}

func toDatasetImportJobBody(item service.DatasetImportJob) DatasetImportJobBody {
	body := DatasetImportJobBody{
		PublicID:     item.PublicID,
		Status:       item.Status,
		TotalRows:    item.TotalRows,
		ValidRows:    item.ValidRows,
		InvalidRows:  item.InvalidRows,
		ErrorSummary: item.ErrorSummary,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
		CompletedAt:  item.CompletedAt,
		ErrorSample:  make([]DatasetImportErrorBody, 0, len(item.ErrorSample)),
	}
	for _, row := range item.ErrorSample {
		body.ErrorSample = append(body.ErrorSample, DatasetImportErrorBody{
			RowNumber: row.RowNumber,
			Error:     row.Error,
			Raw:       row.Raw,
		})
	}
	return body
}

func toDatasetQueryJobBody(item service.DatasetQueryJob) DatasetQueryJobBody {
	return DatasetQueryJobBody{
		PublicID:      item.PublicID,
		Statement:     item.Statement,
		Status:        item.Status,
		ResultColumns: item.ResultColumns,
		ResultRows:    item.ResultRows,
		RowCount:      item.RowCount,
		ErrorSummary:  item.ErrorSummary,
		DurationMs:    item.DurationMs,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
		CompletedAt:   item.CompletedAt,
	}
}

func toDatasetHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrDatasetNotFound):
		return huma.Error404NotFound("dataset not found")
	case errors.Is(err, service.ErrDatasetQueryNotFound):
		return huma.Error404NotFound("dataset query not found")
	case errors.Is(err, service.ErrInvalidDatasetInput):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, service.ErrUnsafeDatasetSQL):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, service.ErrDatasetClickHouseNotReady):
		return huma.Error503ServiceUnavailable(err.Error())
	default:
		return toHTTPError(err)
	}
}
