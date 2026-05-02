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
	PublicID           string                `json:"publicId" format:"uuid"`
	SourceKind         string                `json:"sourceKind" example:"file"`
	SourceFileObjectID *int64                `json:"sourceFileObjectId,omitempty"`
	SourceWorkTableID  *int64                `json:"sourceWorkTableId,omitempty"`
	Name               string                `json:"name" example:"Customers"`
	OriginalFilename   string                `json:"originalFilename" example:"customers.csv"`
	ContentType        string                `json:"contentType" example:"text/csv"`
	ByteSize           int64                 `json:"byteSize" example:"104857600"`
	RawDatabase        string                `json:"rawDatabase" example:"hh_t_1_raw"`
	RawTable           string                `json:"rawTable" example:"ds_018f2f05c6c97a49b32d04f4dd84ef4a"`
	WorkDatabase       string                `json:"workDatabase" example:"hh_t_1_work"`
	Status             string                `json:"status" example:"ready"`
	RowCount           int64                 `json:"rowCount" example:"10000000"`
	ErrorSummary       string                `json:"errorSummary,omitempty"`
	Columns            []DatasetColumnBody   `json:"columns"`
	ImportJob          *DatasetImportJobBody `json:"importJob,omitempty"`
	CreatedAt          time.Time             `json:"createdAt" format:"date-time"`
	UpdatedAt          time.Time             `json:"updatedAt" format:"date-time"`
	ImportedAt         *time.Time            `json:"importedAt,omitempty" format:"date-time"`
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

type DatasetWorkTableColumnBody struct {
	Ordinal        int32  `json:"ordinal" example:"1"`
	ColumnName     string `json:"columnName" example:"category"`
	ClickHouseType string `json:"clickHouseType" example:"Nullable(String)"`
}

type DatasetWorkTableBody struct {
	PublicID              string                       `json:"publicId,omitempty" format:"uuid"`
	Database              string                       `json:"database" example:"hh_t_1_work"`
	Table                 string                       `json:"table" example:"hai_category_summary"`
	DisplayName           string                       `json:"displayName" example:"hai_category_summary"`
	Status                string                       `json:"status" example:"active"`
	Managed               bool                         `json:"managed"`
	OriginDatasetPublicID string                       `json:"originDatasetPublicId,omitempty" format:"uuid"`
	OriginDatasetName     string                       `json:"originDatasetName,omitempty"`
	Engine                string                       `json:"engine" example:"MergeTree"`
	TotalRows             int64                        `json:"totalRows" example:"1000"`
	TotalBytes            int64                        `json:"totalBytes" example:"1048576"`
	CreatedAt             time.Time                    `json:"createdAt" format:"date-time"`
	UpdatedAt             time.Time                    `json:"updatedAt" format:"date-time"`
	DroppedAt             *time.Time                   `json:"droppedAt,omitempty" format:"date-time"`
	Columns               []DatasetWorkTableColumnBody `json:"columns,omitempty"`
}

type DatasetWorkTableListBody struct {
	Items []DatasetWorkTableBody `json:"items"`
}

type DatasetWorkTableExportBody struct {
	PublicID     string     `json:"publicId" format:"uuid"`
	WorkTableID  int64      `json:"workTableId"`
	Format       string     `json:"format" example:"csv"`
	Status       string     `json:"status" example:"processing"`
	ErrorSummary string     `json:"errorSummary,omitempty"`
	FileObjectID *int64     `json:"fileObjectId,omitempty"`
	ExpiresAt    time.Time  `json:"expiresAt" format:"date-time"`
	CreatedAt    time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt    time.Time  `json:"updatedAt" format:"date-time"`
	CompletedAt  *time.Time `json:"completedAt,omitempty" format:"date-time"`
}

type DatasetWorkTableExportListBody struct {
	Items []DatasetWorkTableExportBody `json:"items"`
}

type DatasetWorkTablePreviewBody struct {
	Database    string           `json:"database" example:"hh_t_1_work"`
	Table       string           `json:"table" example:"hai_category_summary"`
	Columns     []string         `json:"columns"`
	PreviewRows []map[string]any `json:"previewRows"`
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

type DatasetWorkTableListOutput struct {
	Body DatasetWorkTableListBody
}

type DatasetWorkTableOutput struct {
	Body DatasetWorkTableBody
}

type DatasetWorkTableExportOutput struct {
	Body DatasetWorkTableExportBody
}

type DatasetWorkTableExportListOutput struct {
	Body DatasetWorkTableExportListBody
}

type DatasetWorkTablePreviewOutput struct {
	Body DatasetWorkTablePreviewBody
}

type DownloadDatasetWorkTableExportOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	Body               []byte
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

type ListDatasetWorkTablesInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"200" default:"100"`
}

type DatasetByPublicIDInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	DatasetPublicID string      `path:"datasetPublicId" format:"uuid"`
}

type DatasetWorkTableInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Database      string      `path:"database" maxLength:"128"`
	Table         string      `path:"table" maxLength:"256"`
}

type DatasetWorkTableByPublicIDInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	WorkTablePublicID string      `path:"workTablePublicId" format:"uuid"`
}

type DatasetWorkTablePreviewInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Database      string      `path:"database" maxLength:"128"`
	Table         string      `path:"table" maxLength:"256"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"1000" default:"100"`
}

type DatasetWorkTablePreviewByPublicIDInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	WorkTablePublicID string      `path:"workTablePublicId" format:"uuid"`
	Limit             int32       `query:"limit" minimum:"1" maximum:"1000" default:"100"`
}

type DatasetWorkTableRegisterBody struct {
	Database        string `json:"database" maxLength:"128"`
	Table           string `json:"table" maxLength:"256"`
	DisplayName     string `json:"displayName,omitempty" maxLength:"160"`
	DatasetPublicID string `json:"datasetPublicId,omitempty" format:"uuid"`
}

type DatasetWorkTableRegisterInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          DatasetWorkTableRegisterBody
}

type DatasetWorkTableLinkBody struct {
	DatasetPublicID string `json:"datasetPublicId" format:"uuid"`
}

type DatasetWorkTableLinkInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token" required:"true"`
	WorkTablePublicID string      `path:"workTablePublicId" format:"uuid"`
	Body              DatasetWorkTableLinkBody
}

type DatasetWorkTableRenameBody struct {
	Table string `json:"table" maxLength:"256"`
}

type DatasetWorkTableRenameInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token" required:"true"`
	WorkTablePublicID string      `path:"workTablePublicId" format:"uuid"`
	Body              DatasetWorkTableRenameBody
}

type DatasetWorkTableMutateInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token" required:"true"`
	WorkTablePublicID string      `path:"workTablePublicId" format:"uuid"`
}

type DatasetWorkTablePromoteBody struct {
	Name string `json:"name,omitempty" maxLength:"160"`
}

type DatasetWorkTablePromoteInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token" required:"true"`
	WorkTablePublicID string      `path:"workTablePublicId" format:"uuid"`
	Body              DatasetWorkTablePromoteBody
}

type ListDatasetScopedWorkTablesInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	DatasetPublicID string      `path:"datasetPublicId" format:"uuid"`
	Limit           int32       `query:"limit" minimum:"1" maximum:"200" default:"100"`
}

type DatasetWorkTableExportInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	WorkTablePublicID string      `path:"workTablePublicId" format:"uuid"`
	Limit             int32       `query:"limit" minimum:"1" maximum:"100" default:"25"`
}

type DatasetWorkTableExportCreateInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token" required:"true"`
	WorkTablePublicID string      `path:"workTablePublicId" format:"uuid"`
}

type DatasetWorkTableExportByPublicIDInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	ExportPublicID string      `path:"exportPublicId" format:"uuid"`
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

type DatasetScopedQueryCreateInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	DatasetPublicID string      `path:"datasetPublicId" format:"uuid"`
	Body            DatasetQueryCreateBody
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

type ListDatasetScopedQueryJobsInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	DatasetPublicID string      `path:"datasetPublicId" format:"uuid"`
	Limit           int32       `query:"limit" minimum:"1" maximum:"100" default:"25"`
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
		Tags:        []string{DocTagDataDatasets},
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
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "createDataset", err)
		}
		item, err := deps.DatasetService.CreateFromDriveFile(ctx, tenant.ID, current.User.ID, file, input.Body.Name, auditCtx)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "createDataset", err)
		}
		return &DatasetOutput{Body: toDatasetBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasets",
		Method:      http.MethodGet,
		Path:        "/api/v1/datasets",
		Summary:     "active tenant の dataset 一覧を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDatasetsInput) (*DatasetListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.List(ctx, tenant.ID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "listDatasets", err)
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
		Tags:        []string{DocTagDataDatasets},
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
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "listDatasetSourceFiles", err)
		}
		out := &DatasetSourceFileListOutput{}
		out.Body.Items = make([]DatasetSourceFileBody, 0, len(files))
		for _, file := range files {
			out.Body.Items = append(out.Body.Items, toDatasetSourceFileBody(file))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetWorkTables",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-work-tables",
		Summary:     "active tenant の ClickHouse work table 一覧を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDatasetWorkTablesInput) (*DatasetWorkTableListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.ListWorkTables(ctx, tenant.ID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "listDatasetWorkTables", err)
		}
		out := &DatasetWorkTableListOutput{}
		out.Body.Items = make([]DatasetWorkTableBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDatasetWorkTableBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "registerDatasetWorkTable",
		Method:      http.MethodPost,
		Path:        "/api/v1/dataset-work-tables/register",
		Summary:     "active tenant の ClickHouse work table を管理レコード化する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableRegisterInput) (*DatasetWorkTableOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.RegisterWorkTable(ctx, tenant.ID, current.User.ID, input.Body.Database, input.Body.Table, input.Body.DatasetPublicID, input.Body.DisplayName, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "registerDatasetWorkTable", err)
		}
		return &DatasetWorkTableOutput{Body: toDatasetWorkTableBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getManagedDatasetWorkTable",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}",
		Summary:     "managed work table detail を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableByPublicIDInput) (*DatasetWorkTableOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.GetManagedWorkTable(ctx, tenant.ID, input.WorkTablePublicID)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "getManagedDatasetWorkTable", err)
		}
		return &DatasetWorkTableOutput{Body: toDatasetWorkTableBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getManagedDatasetWorkTablePreview",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}/preview",
		Summary:     "managed work table preview rows を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTablePreviewByPublicIDInput) (*DatasetWorkTablePreviewOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		preview, err := deps.DatasetService.PreviewManagedWorkTable(ctx, tenant.ID, input.WorkTablePublicID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "getManagedDatasetWorkTablePreview", err)
		}
		return &DatasetWorkTablePreviewOutput{Body: toDatasetWorkTablePreviewBody(preview)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "linkDatasetWorkTable",
		Method:      http.MethodPost,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}/link",
		Summary:     "managed work table を dataset に紐付ける",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableLinkInput) (*DatasetWorkTableOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.LinkWorkTable(ctx, tenant.ID, input.WorkTablePublicID, input.Body.DatasetPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "linkDatasetWorkTable", err)
		}
		return &DatasetWorkTableOutput{Body: toDatasetWorkTableBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "renameDatasetWorkTable",
		Method:      http.MethodPost,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}/rename",
		Summary:     "managed work table を rename する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableRenameInput) (*DatasetWorkTableOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.RenameWorkTable(ctx, tenant.ID, input.WorkTablePublicID, input.Body.Table, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "renameDatasetWorkTable", err)
		}
		return &DatasetWorkTableOutput{Body: toDatasetWorkTableBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "truncateDatasetWorkTable",
		Method:      http.MethodPost,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}/truncate",
		Summary:     "managed work table を truncate する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableMutateInput) (*DatasetWorkTableOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.TruncateWorkTable(ctx, tenant.ID, input.WorkTablePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "truncateDatasetWorkTable", err)
		}
		return &DatasetWorkTableOutput{Body: toDatasetWorkTableBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDatasetWorkTable",
		Method:        http.MethodDelete,
		Path:          "/api/v1/dataset-work-tables/{workTablePublicId}",
		Summary:       "managed work table を drop する",
		Tags:          []string{DocTagDataDatasets},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableMutateInput) (*DeleteDatasetOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DatasetService.DropWorkTable(ctx, tenant.ID, input.WorkTablePublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "deleteDatasetWorkTable", err)
		}
		return &DeleteDatasetOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "promoteDatasetWorkTable",
		Method:      http.MethodPost,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}/promote",
		Summary:     "managed work table を dataset 化する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTablePromoteInput) (*DatasetOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.PromoteWorkTable(ctx, tenant.ID, current.User.ID, input.WorkTablePublicID, input.Body.Name, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "promoteDatasetWorkTable", err)
		}
		return &DatasetOutput{Body: toDatasetBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDatasetWorkTableExport",
		Method:      http.MethodPost,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}/exports",
		Summary:     "managed work table の CSV export を request する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableExportCreateInput) (*DatasetWorkTableExportOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.CreateWorkTableExport(ctx, tenant.ID, current.User.ID, input.WorkTablePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "createDatasetWorkTableExport", err)
		}
		return &DatasetWorkTableExportOutput{Body: toDatasetWorkTableExportBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetWorkTableExports",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}/exports",
		Summary:     "managed work table の CSV export 一覧を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableExportInput) (*DatasetWorkTableExportListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.ListWorkTableExports(ctx, tenant.ID, input.WorkTablePublicID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "listDatasetWorkTableExports", err)
		}
		out := &DatasetWorkTableExportListOutput{}
		out.Body.Items = make([]DatasetWorkTableExportBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDatasetWorkTableExportBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDatasetWorkTableExport",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-work-table-exports/{exportPublicId}",
		Summary:     "work table export detail を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableExportByPublicIDInput) (*DatasetWorkTableExportOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.GetWorkTableExport(ctx, tenant.ID, input.ExportPublicID)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "getDatasetWorkTableExport", err)
		}
		return &DatasetWorkTableExportOutput{Body: toDatasetWorkTableExportBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "downloadDatasetWorkTableExport",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-work-table-exports/{exportPublicId}/download",
		Summary:     "ready work table export を download する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableExportByPublicIDInput) (*DownloadDatasetWorkTableExportOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		download, err := deps.DatasetService.DownloadWorkTableExport(ctx, tenant.ID, input.ExportPublicID)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "downloadDatasetWorkTableExport", err)
		}
		defer download.Body.Close()
		body, err := io.ReadAll(download.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to read export body")
		}
		return &DownloadDatasetWorkTableExportOutput{
			ContentType:        download.File.ContentType,
			ContentDisposition: fmt.Sprintf("attachment; filename=%q", download.File.OriginalFilename),
			Body:               body,
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDatasetWorkTable",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-work-tables/by-ref/{database}/{table}",
		Summary:     "active tenant の ClickHouse work table detail を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableInput) (*DatasetWorkTableOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.GetWorkTable(ctx, tenant.ID, input.Database, input.Table)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "getDatasetWorkTable", err)
		}
		return &DatasetWorkTableOutput{Body: toDatasetWorkTableBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDatasetWorkTablePreview",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-work-tables/by-ref/{database}/{table}/preview",
		Summary:     "active tenant の ClickHouse work table preview rows を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTablePreviewInput) (*DatasetWorkTablePreviewOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		preview, err := deps.DatasetService.PreviewWorkTable(ctx, tenant.ID, input.Database, input.Table, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "getDatasetWorkTablePreview", err)
		}
		return &DatasetWorkTablePreviewOutput{Body: toDatasetWorkTablePreviewBody(preview)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDataset",
		Method:      http.MethodGet,
		Path:        "/api/v1/datasets/{datasetPublicId}",
		Summary:     "active tenant の dataset detail を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetByPublicIDInput) (*DatasetOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.Get(ctx, tenant.ID, input.DatasetPublicID)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "getDataset", err)
		}
		return &DatasetOutput{Body: toDatasetBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetScopedWorkTables",
		Method:      http.MethodGet,
		Path:        "/api/v1/datasets/{datasetPublicId}/work-tables",
		Summary:     "dataset に紐づく managed work table 一覧を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDatasetScopedWorkTablesInput) (*DatasetWorkTableListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.ListWorkTablesForDataset(ctx, tenant.ID, input.DatasetPublicID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "listDatasetScopedWorkTables", err)
		}
		out := &DatasetWorkTableListOutput{}
		out.Body.Items = make([]DatasetWorkTableBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDatasetWorkTableBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDataset",
		Method:        http.MethodDelete,
		Path:          "/api/v1/datasets/{datasetPublicId}",
		Summary:       "active tenant の dataset を削除する",
		Tags:          []string{DocTagDataDatasets},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DeleteDatasetInput) (*DeleteDatasetOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DatasetService.Delete(ctx, tenant.ID, input.DatasetPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "deleteDataset", err)
		}
		return &DeleteDatasetOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDatasetScopedQueryJob",
		Method:      http.MethodPost,
		Path:        "/api/v1/datasets/{datasetPublicId}/query-jobs",
		Summary:     "active tenant の dataset に紐づく SQL query job を作成する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetScopedQueryCreateInput) (*DatasetQueryOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.CreateQueryJobForDataset(ctx, tenant.ID, current.User.ID, input.DatasetPublicID, input.Body.Statement)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "createDatasetScopedQueryJob", err)
		}
		return &DatasetQueryOutput{Body: toDatasetQueryJobBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetScopedQueryJobs",
		Method:      http.MethodGet,
		Path:        "/api/v1/datasets/{datasetPublicId}/query-jobs",
		Summary:     "active tenant の dataset に紐づく query job 一覧を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDatasetScopedQueryJobsInput) (*DatasetQueryListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.ListQueryJobsForDataset(ctx, tenant.ID, input.DatasetPublicID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "listDatasetScopedQueryJobs", err)
		}
		out := &DatasetQueryListOutput{}
		out.Body.Items = make([]DatasetQueryJobBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDatasetQueryJobBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDatasetQueryJob",
		Method:      http.MethodPost,
		Path:        "/api/v1/dataset-query-jobs",
		Summary:     "active tenant の dataset SQL query job を作成する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetQueryCreateInput) (*DatasetQueryOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.CreateQueryJob(ctx, tenant.ID, current.User.ID, input.Body.Statement)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "createDatasetQueryJob", err)
		}
		return &DatasetQueryOutput{Body: toDatasetQueryJobBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetQueryJobs",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-query-jobs",
		Summary:     "active tenant の dataset query job 一覧を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDatasetQueryJobsInput) (*DatasetQueryListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.ListQueryJobs(ctx, tenant.ID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "listDatasetQueryJobs", err)
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
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetQueryByPublicIDInput) (*DatasetQueryOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.GetQueryJob(ctx, tenant.ID, input.QueryJobPublicID)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "getDatasetQueryJob", err)
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
		PublicID:           item.PublicID,
		SourceKind:         item.SourceKind,
		SourceFileObjectID: item.SourceFileObjectID,
		SourceWorkTableID:  item.SourceWorkTableID,
		Name:               item.Name,
		OriginalFilename:   item.OriginalFilename,
		ContentType:        item.ContentType,
		ByteSize:           item.ByteSize,
		RawDatabase:        item.RawDatabase,
		RawTable:           item.RawTable,
		WorkDatabase:       item.WorkDatabase,
		Status:             item.Status,
		RowCount:           item.RowCount,
		ErrorSummary:       item.ErrorSummary,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
		ImportedAt:         item.ImportedAt,
		Columns:            make([]DatasetColumnBody, 0, len(item.Columns)),
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

func toDatasetWorkTableBody(item service.DatasetWorkTable) DatasetWorkTableBody {
	body := DatasetWorkTableBody{
		PublicID:              item.PublicID,
		Database:              item.Database,
		Table:                 item.Table,
		DisplayName:           item.DisplayName,
		Status:                item.Status,
		Managed:               item.Managed,
		OriginDatasetPublicID: item.OriginDatasetPublicID,
		OriginDatasetName:     item.OriginDatasetName,
		Engine:                item.Engine,
		TotalRows:             item.TotalRows,
		TotalBytes:            item.TotalBytes,
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
		DroppedAt:             item.DroppedAt,
		Columns:               make([]DatasetWorkTableColumnBody, 0, len(item.Columns)),
	}
	for _, column := range item.Columns {
		body.Columns = append(body.Columns, DatasetWorkTableColumnBody{
			Ordinal:        column.Ordinal,
			ColumnName:     column.ColumnName,
			ClickHouseType: column.ClickHouseType,
		})
	}
	return body
}

func toDatasetWorkTableExportBody(item service.DatasetWorkTableExport) DatasetWorkTableExportBody {
	return DatasetWorkTableExportBody{
		PublicID:     item.PublicID,
		WorkTableID:  item.WorkTableID,
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

func toDatasetWorkTablePreviewBody(item service.DatasetWorkTablePreview) DatasetWorkTablePreviewBody {
	return DatasetWorkTablePreviewBody{
		Database:    item.Database,
		Table:       item.Table,
		Columns:     item.Columns,
		PreviewRows: item.PreviewRows,
	}
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

func toDatasetHTTPError(ctx context.Context, deps Dependencies, operation string, err error) error {
	switch {
	case errors.Is(err, service.ErrDatasetNotFound):
		return huma.Error404NotFound("dataset not found")
	case errors.Is(err, service.ErrDatasetQueryNotFound):
		return huma.Error404NotFound("dataset query not found")
	case errors.Is(err, service.ErrDatasetWorkTableNotFound):
		return huma.Error404NotFound("dataset work table not found")
	case errors.Is(err, service.ErrDatasetWorkTableExportNotFound):
		return huma.Error404NotFound("dataset work table export not found")
	case errors.Is(err, service.ErrDatasetWorkTableExportNotReady):
		return huma.Error409Conflict("dataset work table export is not ready")
	case errors.Is(err, service.ErrInvalidDatasetInput):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, service.ErrUnsafeDatasetSQL):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, service.ErrDatasetClickHouseNotReady):
		return huma.Error503ServiceUnavailable(err.Error())
	default:
		return toHTTPErrorWithLog(ctx, deps, operation, err)
	}
}
