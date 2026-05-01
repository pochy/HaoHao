package api

import (
	"bufio"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
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

type DatasetListOutput struct {
	Body DatasetListBody
}

type DatasetOutput struct {
	Body DatasetBody
}

type ListDatasetsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
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

func RegisterRawDatasetRoutes(router *gin.Engine, deps Dependencies, maxBytes int64) {
	if router == nil {
		return
	}
	router.POST("/api/v1/datasets", func(c *gin.Context) {
		current, tenant, ok := rawActiveTenant(c, deps, true)
		if !ok {
			return
		}
		if deps.DatasetService == nil || deps.FileService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"title": "dataset service is not configured"})
			return
		}
		if err := deps.DatasetService.EnsureClickHouse(c.Request.Context()); err != nil {
			writeRawDatasetError(c, err)
			return
		}
		reader, err := c.Request.MultipartReader()
		if err != nil {
			if isRequestBodyTooLargeError(err) {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"title": "file is too large"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"title": "invalid multipart form"})
			return
		}
		var name string
		var file service.FileObject
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				if isRequestBodyTooLargeError(err) {
					c.JSON(http.StatusRequestEntityTooLarge, gin.H{"title": "file is too large"})
					return
				}
				c.JSON(http.StatusBadRequest, gin.H{"title": "invalid multipart form"})
				return
			}
			switch part.FormName() {
			case "name":
				value, _ := io.ReadAll(io.LimitReader(part, 512))
				name = strings.TrimSpace(string(value))
			case "file":
				if file.ID > 0 {
					c.JSON(http.StatusBadRequest, gin.H{"title": "only one dataset file is allowed"})
					_ = part.Close()
					return
				}
				uploaded, err := uploadDatasetPart(c.Request.Context(), deps, tenant.ID, current.User.ID, part, maxBytes, sessionAuditContext(c.Request.Context(), current, &tenant.ID))
				if err != nil {
					writeRawDatasetError(c, err)
					_ = part.Close()
					return
				}
				file = uploaded
			}
			_ = part.Close()
		}
		if file.ID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"title": "file is required"})
			return
		}
		item, err := deps.DatasetService.CreateFromSourceFile(c.Request.Context(), tenant.ID, current.User.ID, file, name, sessionAuditContext(c.Request.Context(), current, &tenant.ID))
		if err != nil {
			writeRawDatasetError(c, err)
			return
		}
		c.JSON(http.StatusOK, toDatasetBody(item))
	})
}

func uploadDatasetPart(ctx context.Context, deps Dependencies, tenantID, userID int64, part *multipart.Part, maxBytes int64, auditCtx service.AuditContext) (service.FileObject, error) {
	filename := filepath.Base(strings.TrimSpace(part.FileName()))
	if filename == "" || filename == "." {
		return service.FileObject{}, service.ErrInvalidDatasetInput
	}
	body := bufio.NewReader(part)
	contentType := strings.TrimSpace(part.Header.Get("Content-Type"))
	if contentType == "" {
		sample, _ := body.Peek(512)
		contentType = http.DetectContentType(sample)
	}
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if strings.EqualFold(filepath.Ext(filename), ".csv") || contentType == "application/vnd.ms-excel" || contentType == "application/csv" {
		contentType = "text/csv"
	}
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = "text/csv"
	}
	return deps.FileService.UploadWithMaxBytes(ctx, service.FileUploadInput{
		TenantID:         tenantID,
		UserID:           userID,
		Purpose:          service.DatasetSourceFilePurpose,
		OriginalFilename: filename,
		ContentType:      contentType,
		Body:             body,
	}, auditCtx, maxBytes)
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
		return huma.Error503ServiceUnavailable("clickhouse is not configured")
	default:
		return toHTTPError(err)
	}
}

func writeRawDatasetError(c *gin.Context, err error) {
	switch {
	case isRequestBodyTooLargeError(err):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"title": "file is too large"})
	case errors.Is(err, service.ErrInvalidDatasetInput), errors.Is(err, service.ErrInvalidFileInput):
		c.JSON(http.StatusBadRequest, gin.H{"title": err.Error()})
	case errors.Is(err, service.ErrFileQuotaExceeded):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"title": "tenant file quota exceeded"})
	case errors.Is(err, service.ErrDatasetClickHouseNotReady):
		c.JSON(http.StatusServiceUnavailable, gin.H{"title": "clickhouse is not configured"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"title": "dataset request failed"})
	}
}
