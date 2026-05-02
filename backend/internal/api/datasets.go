package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	PublicID                string                `json:"publicId" format:"uuid"`
	SourceKind              string                `json:"sourceKind" example:"file"`
	SourceFileObjectID      *int64                `json:"sourceFileObjectId,omitempty"`
	SourceWorkTableID       *int64                `json:"sourceWorkTableId,omitempty"`
	SourceWorkTablePublicID string                `json:"sourceWorkTablePublicId,omitempty" format:"uuid"`
	SourceWorkTableName     string                `json:"sourceWorkTableName,omitempty"`
	SourceWorkTableDatabase string                `json:"sourceWorkTableDatabase,omitempty"`
	SourceWorkTableTable    string                `json:"sourceWorkTableTable,omitempty"`
	SourceWorkTableStatus   string                `json:"sourceWorkTableStatus,omitempty"`
	Name                    string                `json:"name" example:"Customers"`
	OriginalFilename        string                `json:"originalFilename" example:"customers.csv"`
	ContentType             string                `json:"contentType" example:"text/csv"`
	ByteSize                int64                 `json:"byteSize" example:"104857600"`
	RawDatabase             string                `json:"rawDatabase" example:"hh_t_1_raw"`
	RawTable                string                `json:"rawTable" example:"ds_018f2f05c6c97a49b32d04f4dd84ef4a"`
	WorkDatabase            string                `json:"workDatabase" example:"hh_t_1_work"`
	Status                  string                `json:"status" example:"ready"`
	RowCount                int64                 `json:"rowCount" example:"10000000"`
	ErrorSummary            string                `json:"errorSummary,omitempty"`
	Columns                 []DatasetColumnBody   `json:"columns"`
	ImportJob               *DatasetImportJobBody `json:"importJob,omitempty"`
	LatestSyncJob           *DatasetSyncJobBody   `json:"latestSyncJob,omitempty"`
	CreatedAt               time.Time             `json:"createdAt" format:"date-time"`
	UpdatedAt               time.Time             `json:"updatedAt" format:"date-time"`
	ImportedAt              *time.Time            `json:"importedAt,omitempty" format:"date-time"`
}

type DatasetListBody struct {
	Items []DatasetBody `json:"items"`
}

type DatasetSyncJobBody struct {
	PublicID            string     `json:"publicId" format:"uuid"`
	Mode                string     `json:"mode" enum:"full_refresh" example:"full_refresh"`
	Status              string     `json:"status" enum:"pending,processing,completed,failed" example:"processing"`
	OldRawDatabase      string     `json:"oldRawDatabase" example:"hh_t_1_raw"`
	OldRawTable         string     `json:"oldRawTable" example:"ds_old"`
	NewRawDatabase      string     `json:"newRawDatabase" example:"hh_t_1_raw"`
	NewRawTable         string     `json:"newRawTable" example:"ds_new"`
	RowCount            int64      `json:"rowCount" example:"100000"`
	TotalBytes          int64      `json:"totalBytes" example:"1048576"`
	ErrorSummary        string     `json:"errorSummary,omitempty"`
	CleanupStatus       string     `json:"cleanupStatus,omitempty"`
	CleanupErrorSummary string     `json:"cleanupErrorSummary,omitempty"`
	StartedAt           *time.Time `json:"startedAt,omitempty" format:"date-time"`
	CompletedAt         *time.Time `json:"completedAt,omitempty" format:"date-time"`
	CreatedAt           time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt           time.Time  `json:"updatedAt" format:"date-time"`
}

type DatasetSyncJobListBody struct {
	Items []DatasetSyncJobBody `json:"items"`
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
	PublicID         string     `json:"publicId" format:"uuid"`
	WorkTableID      int64      `json:"workTableId"`
	Format           string     `json:"format" example:"csv"`
	Status           string     `json:"status" example:"processing"`
	Source           string     `json:"source" enum:"manual,scheduled" example:"manual"`
	SchedulePublicID string     `json:"schedulePublicId,omitempty" format:"uuid"`
	ScheduledFor     *time.Time `json:"scheduledFor,omitempty" format:"date-time"`
	ErrorSummary     string     `json:"errorSummary,omitempty"`
	FileObjectID     *int64     `json:"fileObjectId,omitempty"`
	ExpiresAt        time.Time  `json:"expiresAt" format:"date-time"`
	CreatedAt        time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt        time.Time  `json:"updatedAt" format:"date-time"`
	CompletedAt      *time.Time `json:"completedAt,omitempty" format:"date-time"`
}

type DatasetWorkTableExportListBody struct {
	Items []DatasetWorkTableExportBody `json:"items"`
}

type DatasetWorkTableExportCreateBody struct {
	Format string `json:"format,omitempty" enum:"csv,json,parquet" example:"csv"`
}

type DatasetWorkTableExportScheduleBody struct {
	PublicID         string     `json:"publicId" format:"uuid"`
	WorkTableID      int64      `json:"workTableId"`
	Format           string     `json:"format" enum:"csv,json,parquet" example:"csv"`
	Frequency        string     `json:"frequency" enum:"daily,weekly,monthly" example:"daily"`
	Timezone         string     `json:"timezone" example:"Asia/Tokyo"`
	RunTime          string     `json:"runTime" example:"03:00"`
	Weekday          *int32     `json:"weekday,omitempty" minimum:"1" maximum:"7"`
	MonthDay         *int32     `json:"monthDay,omitempty" minimum:"1" maximum:"28"`
	RetentionDays    int32      `json:"retentionDays" minimum:"1" maximum:"365" example:"7"`
	Enabled          bool       `json:"enabled"`
	NextRunAt        time.Time  `json:"nextRunAt" format:"date-time"`
	LastRunAt        *time.Time `json:"lastRunAt,omitempty" format:"date-time"`
	LastStatus       string     `json:"lastStatus,omitempty"`
	LastErrorSummary string     `json:"lastErrorSummary,omitempty"`
	CreatedAt        time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt        time.Time  `json:"updatedAt" format:"date-time"`
}

type DatasetWorkTableExportScheduleListBody struct {
	Items []DatasetWorkTableExportScheduleBody `json:"items"`
}

type DatasetWorkTableExportScheduleCreateBody struct {
	Format        string `json:"format,omitempty" enum:"csv,json,parquet" example:"csv"`
	Frequency     string `json:"frequency,omitempty" enum:"daily,weekly,monthly" example:"daily"`
	Timezone      string `json:"timezone,omitempty" example:"Asia/Tokyo"`
	RunTime       string `json:"runTime,omitempty" example:"03:00"`
	Weekday       *int32 `json:"weekday,omitempty" minimum:"1" maximum:"7"`
	MonthDay      *int32 `json:"monthDay,omitempty" minimum:"1" maximum:"28"`
	RetentionDays int32  `json:"retentionDays,omitempty" minimum:"1" maximum:"365" example:"7"`
}

type DatasetWorkTableExportScheduleUpdateBody struct {
	Format        string `json:"format,omitempty" enum:"csv,json,parquet" example:"csv"`
	Frequency     string `json:"frequency,omitempty" enum:"daily,weekly,monthly" example:"daily"`
	Timezone      string `json:"timezone,omitempty" example:"Asia/Tokyo"`
	RunTime       string `json:"runTime,omitempty" example:"03:00"`
	Weekday       *int32 `json:"weekday,omitempty" minimum:"1" maximum:"7"`
	MonthDay      *int32 `json:"monthDay,omitempty" minimum:"1" maximum:"28"`
	RetentionDays int32  `json:"retentionDays,omitempty" minimum:"1" maximum:"365" example:"7"`
	Enabled       *bool  `json:"enabled,omitempty"`
}

type DatasetWorkTablePreviewBody struct {
	Database    string           `json:"database" example:"hh_t_1_work"`
	Table       string           `json:"table" example:"hai_category_summary"`
	Columns     []string         `json:"columns"`
	PreviewRows []map[string]any `json:"previewRows"`
}

type DatasetLineageNodeBody struct {
	ID           string                      `json:"id" example:"dataset:018f2f05-c6c9-7a49-b32d-04f4dd84ef4a"`
	ResourceType string                      `json:"resourceType" enum:"dataset,dataset_query_job,dataset_work_table,dataset_work_table_export,dataset_work_table_export_schedule,dataset_sync_job,custom" example:"dataset"`
	PublicID     string                      `json:"publicId,omitempty" format:"uuid"`
	DisplayName  string                      `json:"displayName" example:"Sales"`
	Status       string                      `json:"status,omitempty" example:"ready"`
	NodeKind     string                      `json:"nodeKind,omitempty" enum:"resource,column,custom" example:"resource"`
	SourceKind   string                      `json:"sourceKind,omitempty" enum:"metadata,parser,manual" example:"metadata"`
	ColumnName   string                      `json:"columnName,omitempty" example:"customer_id"`
	Description  string                      `json:"description,omitempty"`
	Editable     bool                        `json:"editable"`
	Position     *DatasetLineagePositionBody `json:"position,omitempty"`
	CreatedAt    *time.Time                  `json:"createdAt,omitempty" format:"date-time"`
	UpdatedAt    *time.Time                  `json:"updatedAt,omitempty" format:"date-time"`
	Metadata     map[string]any              `json:"metadata,omitempty"`
}

type DatasetLineagePositionBody struct {
	X float64 `json:"x" example:"120"`
	Y float64 `json:"y" example:"240"`
}

type DatasetLineageEdgeBody struct {
	ID           string     `json:"id"`
	SourceNodeID string     `json:"sourceNodeId"`
	TargetNodeID string     `json:"targetNodeId"`
	RelationType string     `json:"relationType" enum:"query_input,query_created_work_table,source_dataset,promoted_dataset,work_table_export,export_schedule,scheduled_export_run,dataset_sync_source,dataset_sync_target,column_derives,manual_dependency" example:"source_dataset"`
	Confidence   string     `json:"confidence" enum:"metadata,parser_exact,parser_partial,manual" example:"metadata"`
	SourceKind   string     `json:"sourceKind,omitempty" enum:"metadata,parser,manual" example:"metadata"`
	Label        string     `json:"label,omitempty"`
	Description  string     `json:"description,omitempty"`
	Expression   string     `json:"expression,omitempty"`
	Editable     bool       `json:"editable"`
	CreatedAt    *time.Time `json:"createdAt,omitempty" format:"date-time"`
}

type DatasetLineageTimelineItemBody struct {
	ID           string         `json:"id"`
	NodeID       string         `json:"nodeId"`
	ResourceType string         `json:"resourceType" enum:"dataset,dataset_query_job,dataset_work_table,dataset_work_table_export,dataset_work_table_export_schedule,dataset_sync_job" example:"dataset_work_table"`
	PublicID     string         `json:"publicId,omitempty" format:"uuid"`
	RelationType string         `json:"relationType" enum:"query_input,query_created_work_table,source_dataset,promoted_dataset,work_table_export,export_schedule,scheduled_export_run,dataset_sync_source,dataset_sync_target" example:"query_created_work_table"`
	Status       string         `json:"status,omitempty" example:"completed"`
	OccurredAt   *time.Time     `json:"occurredAt,omitempty" format:"date-time"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type DatasetLineageBody struct {
	Root     DatasetLineageNodeBody           `json:"root"`
	Nodes    []DatasetLineageNodeBody         `json:"nodes"`
	Edges    []DatasetLineageEdgeBody         `json:"edges"`
	Timeline []DatasetLineageTimelineItemBody `json:"timeline"`
}

type DatasetLineageChangeSetBody struct {
	PublicID             string     `json:"publicId" format:"uuid"`
	QueryJobPublicID     string     `json:"queryJobPublicId,omitempty" format:"uuid"`
	RootResourceType     string     `json:"rootResourceType"`
	RootResourcePublicID string     `json:"rootResourcePublicId,omitempty" format:"uuid"`
	SourceKind           string     `json:"sourceKind" enum:"parser,manual"`
	Status               string     `json:"status" enum:"draft,published,rejected,archived"`
	Title                string     `json:"title"`
	Description          string     `json:"description"`
	CreatedAt            time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt            time.Time  `json:"updatedAt" format:"date-time"`
	PublishedAt          *time.Time `json:"publishedAt,omitempty" format:"date-time"`
	RejectedAt           *time.Time `json:"rejectedAt,omitempty" format:"date-time"`
	ArchivedAt           *time.Time `json:"archivedAt,omitempty" format:"date-time"`
}

type DatasetLineageChangeSetGraphBody struct {
	ChangeSet DatasetLineageChangeSetBody `json:"changeSet"`
	Nodes     []DatasetLineageNodeBody    `json:"nodes"`
	Edges     []DatasetLineageEdgeBody    `json:"edges"`
}

type DatasetLineageChangeSetListBody struct {
	Items []DatasetLineageChangeSetBody `json:"items"`
}

type DatasetLineageParseRunBody struct {
	PublicID          string     `json:"publicId" format:"uuid"`
	QueryJobPublicID  string     `json:"queryJobPublicId" format:"uuid"`
	ChangeSetPublicID string     `json:"changeSetPublicId,omitempty" format:"uuid"`
	Status            string     `json:"status" enum:"processing,completed,failed"`
	TableRefCount     int32      `json:"tableRefCount"`
	ColumnEdgeCount   int32      `json:"columnEdgeCount"`
	ErrorSummary      string     `json:"errorSummary,omitempty"`
	CreatedAt         time.Time  `json:"createdAt" format:"date-time"`
	CompletedAt       *time.Time `json:"completedAt,omitempty" format:"date-time"`
}

type DatasetLineageParseRunListBody struct {
	Items []DatasetLineageParseRunBody `json:"items"`
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

type DatasetWorkTableExportScheduleOutput struct {
	Body DatasetWorkTableExportScheduleBody
}

type DatasetWorkTableExportScheduleListOutput struct {
	Body DatasetWorkTableExportScheduleListBody
}

type DatasetSyncJobOutput struct {
	Body DatasetSyncJobBody
}

type DatasetSyncJobListOutput struct {
	Body DatasetSyncJobListBody
}

type DatasetWorkTablePreviewOutput struct {
	Body DatasetWorkTablePreviewBody
}

type DatasetLineageOutput struct {
	Body DatasetLineageBody
}

type DatasetLineageChangeSetOutput struct {
	Body DatasetLineageChangeSetBody
}

type DatasetLineageChangeSetGraphOutput struct {
	Body DatasetLineageChangeSetGraphBody
}

type DatasetLineageChangeSetListOutput struct {
	Body DatasetLineageChangeSetListBody
}

type DatasetLineageParseRunOutput struct {
	Body DatasetLineageParseRunBody
}

type DatasetLineageParseRunListOutput struct {
	Body DatasetLineageParseRunListBody
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

type DatasetSyncJobCreateBody struct {
	Mode string `json:"mode,omitempty" enum:"full_refresh" example:"full_refresh"`
}

type DatasetSyncJobCreateInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	CSRFToken       string      `header:"X-CSRF-Token" required:"true"`
	DatasetPublicID string      `path:"datasetPublicId" format:"uuid"`
	Body            *DatasetSyncJobCreateBody
}

type ListDatasetSyncJobsInput struct {
	SessionCookie   http.Cookie `cookie:"SESSION_ID"`
	DatasetPublicID string      `path:"datasetPublicId" format:"uuid"`
	Limit           int32       `query:"limit" minimum:"1" maximum:"100" default:"25"`
}

type DatasetLineageInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	DatasetPublicID   string      `path:"datasetPublicId" format:"uuid"`
	Direction         string      `query:"direction" enum:"upstream,downstream,both" default:"both"`
	Depth             int32       `query:"depth" minimum:"1" maximum:"2" default:"1"`
	IncludeHistory    bool        `query:"includeHistory" default:"true"`
	Limit             int32       `query:"limit" minimum:"1" maximum:"100" default:"50"`
	Level             string      `query:"level" enum:"table,column,both" default:"table"`
	Sources           string      `query:"sources" example:"metadata,parser,manual"`
	IncludeDraft      bool        `query:"includeDraft" default:"false"`
	ChangeSetPublicID string      `query:"changeSetPublicId" format:"uuid"`
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

type DatasetWorkTableLineageInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	WorkTablePublicID string      `path:"workTablePublicId" format:"uuid"`
	Direction         string      `query:"direction" enum:"upstream,downstream,both" default:"both"`
	Depth             int32       `query:"depth" minimum:"1" maximum:"2" default:"1"`
	IncludeHistory    bool        `query:"includeHistory" default:"true"`
	Limit             int32       `query:"limit" minimum:"1" maximum:"100" default:"50"`
	Level             string      `query:"level" enum:"table,column,both" default:"table"`
	Sources           string      `query:"sources" example:"metadata,parser,manual"`
	IncludeDraft      bool        `query:"includeDraft" default:"false"`
	ChangeSetPublicID string      `query:"changeSetPublicId" format:"uuid"`
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
	Body              *DatasetWorkTableExportCreateBody
}

type DatasetWorkTableExportScheduleInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	WorkTablePublicID string      `path:"workTablePublicId" format:"uuid"`
}

type DatasetWorkTableExportScheduleCreateInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token" required:"true"`
	WorkTablePublicID string      `path:"workTablePublicId" format:"uuid"`
	Body              DatasetWorkTableExportScheduleCreateBody
}

type DatasetWorkTableExportScheduleMutateInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	CSRFToken        string      `header:"X-CSRF-Token" required:"true"`
	SchedulePublicID string      `path:"schedulePublicId" format:"uuid"`
}

type DatasetWorkTableExportScheduleUpdateInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	CSRFToken        string      `header:"X-CSRF-Token" required:"true"`
	SchedulePublicID string      `path:"schedulePublicId" format:"uuid"`
	Body             DatasetWorkTableExportScheduleUpdateBody
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

type DatasetQueryLineageInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	QueryJobPublicID  string      `path:"queryJobPublicId" format:"uuid"`
	Direction         string      `query:"direction" enum:"upstream,downstream,both" default:"both"`
	Depth             int32       `query:"depth" minimum:"1" maximum:"2" default:"1"`
	IncludeHistory    bool        `query:"includeHistory" default:"true"`
	Limit             int32       `query:"limit" minimum:"1" maximum:"100" default:"50"`
	Level             string      `query:"level" enum:"table,column,both" default:"table"`
	Sources           string      `query:"sources" example:"metadata,parser,manual"`
	IncludeDraft      bool        `query:"includeDraft" default:"false"`
	ChangeSetPublicID string      `query:"changeSetPublicId" format:"uuid"`
}

type DatasetQueryLineageParseInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	CSRFToken        string      `header:"X-CSRF-Token" required:"true"`
	QueryJobPublicID string      `path:"queryJobPublicId" format:"uuid"`
}

type DatasetQueryLineageParseRunsInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	QueryJobPublicID string      `path:"queryJobPublicId" format:"uuid"`
	Limit            int32       `query:"limit" minimum:"1" maximum:"100" default:"25"`
}

type DatasetLineageChangeSetCreateBody struct {
	RootResourceType     string `json:"rootResourceType,omitempty" example:"dataset_query_job"`
	RootResourcePublicID string `json:"rootResourcePublicId,omitempty" format:"uuid"`
	SourceKind           string `json:"sourceKind,omitempty" enum:"manual,parser" example:"manual"`
	Title                string `json:"title,omitempty" maxLength:"160"`
	Description          string `json:"description,omitempty" maxLength:"2000"`
}

type DatasetLineageChangeSetCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          DatasetLineageChangeSetCreateBody
}

type DatasetLineageChangeSetListInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Status        string      `query:"status" enum:"draft,published,rejected,archived"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"100" default:"50"`
}

type DatasetLineageChangeSetInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	ChangeSetPublicID string      `path:"changeSetPublicId" format:"uuid"`
}

type DatasetLineageChangeSetMutateInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token" required:"true"`
	ChangeSetPublicID string      `path:"changeSetPublicId" format:"uuid"`
}

type DatasetLineageGraphSaveInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token" required:"true"`
	ChangeSetPublicID string      `path:"changeSetPublicId" format:"uuid"`
	Body              DatasetLineageGraphSaveBody
}

type DatasetLineageGraphSaveBody struct {
	Nodes []DatasetLineageNodeWriteBody `json:"nodes"`
	Edges []DatasetLineageEdgeWriteBody `json:"edges"`
}

type DatasetLineageNodeWriteBody struct {
	ID           string                      `json:"id"`
	ResourceType string                      `json:"resourceType,omitempty"`
	PublicID     string                      `json:"publicId,omitempty" format:"uuid"`
	DisplayName  string                      `json:"displayName,omitempty"`
	NodeKind     string                      `json:"nodeKind,omitempty" enum:"resource,column,custom"`
	SourceKind   string                      `json:"sourceKind,omitempty" enum:"parser,manual"`
	ColumnName   string                      `json:"columnName,omitempty"`
	Description  string                      `json:"description,omitempty"`
	Position     *DatasetLineagePositionBody `json:"position,omitempty"`
	Metadata     map[string]any              `json:"metadata,omitempty"`
}

type DatasetLineageEdgeWriteBody struct {
	ID           string         `json:"id,omitempty"`
	SourceNodeID string         `json:"sourceNodeId"`
	TargetNodeID string         `json:"targetNodeId"`
	RelationType string         `json:"relationType,omitempty"`
	Confidence   string         `json:"confidence,omitempty" enum:"parser_exact,parser_partial,manual"`
	SourceKind   string         `json:"sourceKind,omitempty" enum:"parser,manual"`
	Label        string         `json:"label,omitempty"`
	Description  string         `json:"description,omitempty"`
	Expression   string         `json:"expression,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
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
		OperationID: "getDatasetWorkTableLineage",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}/lineage",
		Summary:     "managed work table の lineage graph を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableLineageInput) (*DatasetLineageOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		graph, err := deps.DatasetService.GetWorkTableLineage(ctx, tenant.ID, input.WorkTablePublicID, service.DatasetLineageOptions{
			Direction:         input.Direction,
			Depth:             input.Depth,
			IncludeHistory:    input.IncludeHistory,
			Limit:             input.Limit,
			Level:             input.Level,
			Sources:           datasetLineageSourcesFromQuery(input.Sources),
			IncludeDraft:      input.IncludeDraft,
			ChangeSetPublicID: input.ChangeSetPublicID,
		})
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "getDatasetWorkTableLineage", err)
		}
		return &DatasetLineageOutput{Body: toDatasetLineageBody(graph)}, nil
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
		Summary:     "managed work table の export を request する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableExportCreateInput) (*DatasetWorkTableExportOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		format := ""
		if input.Body != nil {
			format = input.Body.Format
		}
		item, err := deps.DatasetService.CreateWorkTableExport(ctx, tenant.ID, current.User.ID, input.WorkTablePublicID, format, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "createDatasetWorkTableExport", err)
		}
		return &DatasetWorkTableExportOutput{Body: toDatasetWorkTableExportBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetWorkTableExports",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}/exports",
		Summary:     "managed work table の export 一覧を返す",
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
		OperationID: "listDatasetWorkTableExportSchedules",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}/export-schedules",
		Summary:     "managed work table の export schedule 一覧を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableExportScheduleInput) (*DatasetWorkTableExportScheduleListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.ListWorkTableExportSchedules(ctx, tenant.ID, input.WorkTablePublicID)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "listDatasetWorkTableExportSchedules", err)
		}
		out := &DatasetWorkTableExportScheduleListOutput{}
		out.Body.Items = make([]DatasetWorkTableExportScheduleBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDatasetWorkTableExportScheduleBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDatasetWorkTableExportSchedule",
		Method:      http.MethodPost,
		Path:        "/api/v1/dataset-work-tables/{workTablePublicId}/export-schedules",
		Summary:     "managed work table の export schedule を作成する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableExportScheduleCreateInput) (*DatasetWorkTableExportScheduleOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.CreateWorkTableExportSchedule(ctx, tenant.ID, current.User.ID, input.WorkTablePublicID, datasetWorkTableExportScheduleInputFromCreateBody(input.Body), sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "createDatasetWorkTableExportSchedule", err)
		}
		return &DatasetWorkTableExportScheduleOutput{Body: toDatasetWorkTableExportScheduleBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateDatasetWorkTableExportSchedule",
		Method:      http.MethodPatch,
		Path:        "/api/v1/dataset-work-table-export-schedules/{schedulePublicId}",
		Summary:     "work table export schedule を更新する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableExportScheduleUpdateInput) (*DatasetWorkTableExportScheduleOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.UpdateWorkTableExportSchedule(ctx, tenant.ID, current.User.ID, input.SchedulePublicID, datasetWorkTableExportScheduleInputFromUpdateBody(input.Body), sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "updateDatasetWorkTableExportSchedule", err)
		}
		return &DatasetWorkTableExportScheduleOutput{Body: toDatasetWorkTableExportScheduleBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "disableDatasetWorkTableExportSchedule",
		Method:      http.MethodPost,
		Path:        "/api/v1/dataset-work-table-export-schedules/{schedulePublicId}/disable",
		Summary:     "work table export schedule を disable する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetWorkTableExportScheduleMutateInput) (*DatasetWorkTableExportScheduleOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.DisableWorkTableExportSchedule(ctx, tenant.ID, current.User.ID, input.SchedulePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "disableDatasetWorkTableExportSchedule", err)
		}
		return &DatasetWorkTableExportScheduleOutput{Body: toDatasetWorkTableExportScheduleBody(item)}, nil
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
		OperationID: "getDatasetLineage",
		Method:      http.MethodGet,
		Path:        "/api/v1/datasets/{datasetPublicId}/lineage",
		Summary:     "active tenant の dataset lineage graph を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetLineageInput) (*DatasetLineageOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		graph, err := deps.DatasetService.GetDatasetLineage(ctx, tenant.ID, input.DatasetPublicID, service.DatasetLineageOptions{
			Direction:         input.Direction,
			Depth:             input.Depth,
			IncludeHistory:    input.IncludeHistory,
			Limit:             input.Limit,
			Level:             input.Level,
			Sources:           datasetLineageSourcesFromQuery(input.Sources),
			IncludeDraft:      input.IncludeDraft,
			ChangeSetPublicID: input.ChangeSetPublicID,
		})
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "getDatasetLineage", err)
		}
		return &DatasetLineageOutput{Body: toDatasetLineageBody(graph)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDatasetSyncJob",
		Method:      http.MethodPost,
		Path:        "/api/v1/datasets/{datasetPublicId}/syncs",
		Summary:     "work table 由来 dataset の再同期を request する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetSyncJobCreateInput) (*DatasetSyncJobOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		mode := ""
		if input.Body != nil {
			mode = input.Body.Mode
		}
		item, err := deps.DatasetService.RequestDatasetSync(ctx, tenant.ID, current.User.ID, input.DatasetPublicID, mode, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "createDatasetSyncJob", err)
		}
		return &DatasetSyncJobOutput{Body: toDatasetSyncJobBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetSyncJobs",
		Method:      http.MethodGet,
		Path:        "/api/v1/datasets/{datasetPublicId}/syncs",
		Summary:     "dataset の sync history を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDatasetSyncJobsInput) (*DatasetSyncJobListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.ListDatasetSyncJobs(ctx, tenant.ID, input.DatasetPublicID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "listDatasetSyncJobs", err)
		}
		out := &DatasetSyncJobListOutput{}
		out.Body.Items = make([]DatasetSyncJobBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDatasetSyncJobBody(item))
		}
		return out, nil
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

	huma.Register(api, huma.Operation{
		OperationID: "getDatasetQueryJobLineage",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-query-jobs/{queryJobPublicId}/lineage",
		Summary:     "active tenant の dataset query job lineage graph を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetQueryLineageInput) (*DatasetLineageOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		graph, err := deps.DatasetService.GetQueryJobLineage(ctx, tenant.ID, input.QueryJobPublicID, service.DatasetLineageOptions{
			Direction:         input.Direction,
			Depth:             input.Depth,
			IncludeHistory:    input.IncludeHistory,
			Limit:             input.Limit,
			Level:             input.Level,
			Sources:           datasetLineageSourcesFromQuery(input.Sources),
			IncludeDraft:      input.IncludeDraft,
			ChangeSetPublicID: input.ChangeSetPublicID,
		})
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "getDatasetQueryJobLineage", err)
		}
		return &DatasetLineageOutput{Body: toDatasetLineageBody(graph)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "parseDatasetQueryJobLineage",
		Method:      http.MethodPost,
		Path:        "/api/v1/dataset-query-jobs/{queryJobPublicId}/lineage/parse",
		Summary:     "dataset query job の SQL を parse して lineage draft を作成する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetQueryLineageParseInput) (*DatasetLineageParseRunOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		run, _, err := deps.DatasetService.ParseQueryJobLineage(ctx, tenant.ID, current.User.ID, input.QueryJobPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "parseDatasetQueryJobLineage", err)
		}
		return &DatasetLineageParseRunOutput{Body: toDatasetLineageParseRunBody(run)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetQueryJobLineageParseRuns",
		Method:      http.MethodGet,
		Path:        "/api/v1/dataset-query-jobs/{queryJobPublicId}/lineage/parse-runs",
		Summary:     "dataset query job の lineage parse run 一覧を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetQueryLineageParseRunsInput) (*DatasetLineageParseRunListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.ListLineageParseRuns(ctx, tenant.ID, input.QueryJobPublicID, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "listDatasetQueryJobLineageParseRuns", err)
		}
		out := &DatasetLineageParseRunListOutput{}
		out.Body.Items = make([]DatasetLineageParseRunBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDatasetLineageParseRunBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDatasetLineageChangeSet",
		Method:      http.MethodPost,
		Path:        "/api/v1/lineage/change-sets",
		Summary:     "lineage change set draft を作成する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetLineageChangeSetCreateInput) (*DatasetLineageChangeSetOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.CreateLineageChangeSet(ctx, tenant.ID, current.User.ID, service.DatasetLineageChangeSetInput{
			RootResourceType:     input.Body.RootResourceType,
			RootResourcePublicID: input.Body.RootResourcePublicID,
			SourceKind:           input.Body.SourceKind,
			Title:                input.Body.Title,
			Description:          input.Body.Description,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "createDatasetLineageChangeSet", err)
		}
		return &DatasetLineageChangeSetOutput{Body: toDatasetLineageChangeSetBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDatasetLineageChangeSets",
		Method:      http.MethodGet,
		Path:        "/api/v1/lineage/change-sets",
		Summary:     "lineage change set 一覧を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetLineageChangeSetListInput) (*DatasetLineageChangeSetListOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DatasetService.ListLineageChangeSets(ctx, tenant.ID, input.Status, input.Limit)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "listDatasetLineageChangeSets", err)
		}
		out := &DatasetLineageChangeSetListOutput{}
		out.Body.Items = make([]DatasetLineageChangeSetBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDatasetLineageChangeSetBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDatasetLineageChangeSet",
		Method:      http.MethodGet,
		Path:        "/api/v1/lineage/change-sets/{changeSetPublicId}",
		Summary:     "lineage change set graph を返す",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetLineageChangeSetInput) (*DatasetLineageChangeSetGraphOutput, error) {
		_, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.GetLineageChangeSet(ctx, tenant.ID, input.ChangeSetPublicID)
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "getDatasetLineageChangeSet", err)
		}
		return &DatasetLineageChangeSetGraphOutput{Body: toDatasetLineageChangeSetGraphBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateDatasetLineageChangeSetGraph",
		Method:      http.MethodPut,
		Path:        "/api/v1/lineage/change-sets/{changeSetPublicId}/graph",
		Summary:     "lineage change set draft graph を保存する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetLineageGraphSaveInput) (*DatasetLineageChangeSetGraphOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.SaveLineageChangeSetGraph(ctx, tenant.ID, input.ChangeSetPublicID, datasetLineageGraphInputFromBody(input.Body), sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "updateDatasetLineageChangeSetGraph", err)
		}
		return &DatasetLineageChangeSetGraphOutput{Body: toDatasetLineageChangeSetGraphBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "publishDatasetLineageChangeSet",
		Method:      http.MethodPost,
		Path:        "/api/v1/lineage/change-sets/{changeSetPublicId}/publish",
		Summary:     "lineage change set draft を publish する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetLineageChangeSetMutateInput) (*DatasetLineageChangeSetOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if _, err := requireTenantAdmin(ctx, deps, input.SessionCookie.Value, ""); err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.PublishLineageChangeSet(ctx, tenant.ID, current.User.ID, input.ChangeSetPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "publishDatasetLineageChangeSet", err)
		}
		return &DatasetLineageChangeSetOutput{Body: toDatasetLineageChangeSetBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "rejectDatasetLineageChangeSet",
		Method:      http.MethodPost,
		Path:        "/api/v1/lineage/change-sets/{changeSetPublicId}/reject",
		Summary:     "lineage change set draft を reject する",
		Tags:        []string{DocTagDataDatasets},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DatasetLineageChangeSetMutateInput) (*DatasetLineageChangeSetOutput, error) {
		current, tenant, err := requireDatasetTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if _, err := requireTenantAdmin(ctx, deps, input.SessionCookie.Value, ""); err != nil {
			return nil, err
		}
		item, err := deps.DatasetService.RejectLineageChangeSet(ctx, tenant.ID, current.User.ID, input.ChangeSetPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDatasetHTTPError(ctx, deps, "rejectDatasetLineageChangeSet", err)
		}
		return &DatasetLineageChangeSetOutput{Body: toDatasetLineageChangeSetBody(item)}, nil
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
		PublicID:                item.PublicID,
		SourceKind:              item.SourceKind,
		SourceFileObjectID:      item.SourceFileObjectID,
		SourceWorkTableID:       item.SourceWorkTableID,
		SourceWorkTablePublicID: item.SourceWorkTablePublicID,
		SourceWorkTableName:     item.SourceWorkTableName,
		SourceWorkTableDatabase: item.SourceWorkTableDatabase,
		SourceWorkTableTable:    item.SourceWorkTableTable,
		SourceWorkTableStatus:   item.SourceWorkTableStatus,
		Name:                    item.Name,
		OriginalFilename:        item.OriginalFilename,
		ContentType:             item.ContentType,
		ByteSize:                item.ByteSize,
		RawDatabase:             item.RawDatabase,
		RawTable:                item.RawTable,
		WorkDatabase:            item.WorkDatabase,
		Status:                  item.Status,
		RowCount:                item.RowCount,
		ErrorSummary:            item.ErrorSummary,
		CreatedAt:               item.CreatedAt,
		UpdatedAt:               item.UpdatedAt,
		ImportedAt:              item.ImportedAt,
		Columns:                 make([]DatasetColumnBody, 0, len(item.Columns)),
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
	if item.LatestSyncJob != nil {
		syncJob := toDatasetSyncJobBody(*item.LatestSyncJob)
		body.LatestSyncJob = &syncJob
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
		PublicID:         item.PublicID,
		WorkTableID:      item.WorkTableID,
		Format:           item.Format,
		Status:           item.Status,
		Source:           item.Source,
		SchedulePublicID: item.SchedulePublicID,
		ScheduledFor:     item.ScheduledFor,
		ErrorSummary:     item.ErrorSummary,
		FileObjectID:     item.FileObjectID,
		ExpiresAt:        item.ExpiresAt,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
		CompletedAt:      item.CompletedAt,
	}
}

func toDatasetWorkTableExportScheduleBody(item service.DatasetWorkTableExportSchedule) DatasetWorkTableExportScheduleBody {
	return DatasetWorkTableExportScheduleBody{
		PublicID:         item.PublicID,
		WorkTableID:      item.WorkTableID,
		Format:           item.Format,
		Frequency:        item.Frequency,
		Timezone:         item.Timezone,
		RunTime:          item.RunTime,
		Weekday:          item.Weekday,
		MonthDay:         item.MonthDay,
		RetentionDays:    item.RetentionDays,
		Enabled:          item.Enabled,
		NextRunAt:        item.NextRunAt,
		LastRunAt:        item.LastRunAt,
		LastStatus:       item.LastStatus,
		LastErrorSummary: item.LastErrorSummary,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func datasetWorkTableExportScheduleInputFromCreateBody(body DatasetWorkTableExportScheduleCreateBody) service.DatasetWorkTableExportScheduleInput {
	return service.DatasetWorkTableExportScheduleInput{
		Format:        body.Format,
		Frequency:     body.Frequency,
		Timezone:      body.Timezone,
		RunTime:       body.RunTime,
		Weekday:       body.Weekday,
		MonthDay:      body.MonthDay,
		RetentionDays: body.RetentionDays,
	}
}

func datasetWorkTableExportScheduleInputFromUpdateBody(body DatasetWorkTableExportScheduleUpdateBody) service.DatasetWorkTableExportScheduleInput {
	return service.DatasetWorkTableExportScheduleInput{
		Format:        body.Format,
		Frequency:     body.Frequency,
		Timezone:      body.Timezone,
		RunTime:       body.RunTime,
		Weekday:       body.Weekday,
		MonthDay:      body.MonthDay,
		RetentionDays: body.RetentionDays,
		Enabled:       body.Enabled,
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

func toDatasetLineageBody(item service.DatasetLineageGraph) DatasetLineageBody {
	body := DatasetLineageBody{
		Root:     toDatasetLineageNodeBody(item.Root),
		Nodes:    make([]DatasetLineageNodeBody, 0, len(item.Nodes)),
		Edges:    make([]DatasetLineageEdgeBody, 0, len(item.Edges)),
		Timeline: make([]DatasetLineageTimelineItemBody, 0, len(item.Timeline)),
	}
	for _, node := range item.Nodes {
		body.Nodes = append(body.Nodes, toDatasetLineageNodeBody(node))
	}
	for _, edge := range item.Edges {
		body.Edges = append(body.Edges, DatasetLineageEdgeBody{
			ID:           edge.ID,
			SourceNodeID: edge.SourceNodeID,
			TargetNodeID: edge.TargetNodeID,
			RelationType: edge.RelationType,
			Confidence:   edge.Confidence,
			SourceKind:   edge.SourceKind,
			Label:        edge.Label,
			Description:  edge.Description,
			Expression:   edge.Expression,
			Editable:     edge.Editable,
			CreatedAt:    edge.CreatedAt,
		})
	}
	for _, entry := range item.Timeline {
		body.Timeline = append(body.Timeline, DatasetLineageTimelineItemBody{
			ID:           entry.ID,
			NodeID:       entry.NodeID,
			ResourceType: entry.ResourceType,
			PublicID:     entry.PublicID,
			RelationType: entry.RelationType,
			Status:       entry.Status,
			OccurredAt:   entry.OccurredAt,
			Metadata:     entry.Metadata,
		})
	}
	return body
}

func toDatasetLineageNodeBody(item service.DatasetLineageNode) DatasetLineageNodeBody {
	return DatasetLineageNodeBody{
		ID:           item.ID,
		ResourceType: item.ResourceType,
		PublicID:     item.PublicID,
		DisplayName:  item.DisplayName,
		Status:       item.Status,
		NodeKind:     item.NodeKind,
		SourceKind:   item.SourceKind,
		ColumnName:   item.ColumnName,
		Description:  item.Description,
		Editable:     item.Editable,
		Position:     toDatasetLineagePositionBody(item.Position),
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
		Metadata:     item.Metadata,
	}
}

func toDatasetLineagePositionBody(item *service.DatasetLineagePosition) *DatasetLineagePositionBody {
	if item == nil {
		return nil
	}
	return &DatasetLineagePositionBody{X: item.X, Y: item.Y}
}

func toDatasetLineageChangeSetBody(item service.DatasetLineageChangeSet) DatasetLineageChangeSetBody {
	return DatasetLineageChangeSetBody{
		PublicID:             item.PublicID,
		QueryJobPublicID:     item.QueryJobPublicID,
		RootResourceType:     item.RootResourceType,
		RootResourcePublicID: item.RootResourcePublicID,
		SourceKind:           item.SourceKind,
		Status:               item.Status,
		Title:                item.Title,
		Description:          item.Description,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
		PublishedAt:          item.PublishedAt,
		RejectedAt:           item.RejectedAt,
		ArchivedAt:           item.ArchivedAt,
	}
}

func toDatasetLineageChangeSetGraphBody(item service.DatasetLineageChangeSetWithGraph) DatasetLineageChangeSetGraphBody {
	body := DatasetLineageChangeSetGraphBody{
		ChangeSet: toDatasetLineageChangeSetBody(item.ChangeSet),
		Nodes:     make([]DatasetLineageNodeBody, 0, len(item.Nodes)),
		Edges:     make([]DatasetLineageEdgeBody, 0, len(item.Edges)),
	}
	for _, node := range item.Nodes {
		body.Nodes = append(body.Nodes, toDatasetLineageNodeBody(node))
	}
	for _, edge := range item.Edges {
		body.Edges = append(body.Edges, DatasetLineageEdgeBody{
			ID:           edge.ID,
			SourceNodeID: edge.SourceNodeID,
			TargetNodeID: edge.TargetNodeID,
			RelationType: edge.RelationType,
			Confidence:   edge.Confidence,
			SourceKind:   edge.SourceKind,
			Label:        edge.Label,
			Description:  edge.Description,
			Expression:   edge.Expression,
			Editable:     edge.Editable,
			CreatedAt:    edge.CreatedAt,
		})
	}
	return body
}

func toDatasetLineageParseRunBody(item service.DatasetLineageParseRun) DatasetLineageParseRunBody {
	return DatasetLineageParseRunBody{
		PublicID:          item.PublicID,
		QueryJobPublicID:  item.QueryJobPublicID,
		ChangeSetPublicID: item.ChangeSetPublicID,
		Status:            item.Status,
		TableRefCount:     item.TableRefCount,
		ColumnEdgeCount:   item.ColumnEdgeCount,
		ErrorSummary:      item.ErrorSummary,
		CreatedAt:         item.CreatedAt,
		CompletedAt:       item.CompletedAt,
	}
}

func datasetLineageGraphInputFromBody(body DatasetLineageGraphSaveBody) service.DatasetLineageGraphInput {
	out := service.DatasetLineageGraphInput{
		Nodes: make([]service.DatasetLineageNodeInput, 0, len(body.Nodes)),
		Edges: make([]service.DatasetLineageEdgeInput, 0, len(body.Edges)),
	}
	for _, node := range body.Nodes {
		out.Nodes = append(out.Nodes, service.DatasetLineageNodeInput{
			ID:           node.ID,
			ResourceType: node.ResourceType,
			PublicID:     node.PublicID,
			DisplayName:  node.DisplayName,
			NodeKind:     node.NodeKind,
			SourceKind:   node.SourceKind,
			ColumnName:   node.ColumnName,
			Description:  node.Description,
			Position:     datasetLineagePositionFromBody(node.Position),
			Metadata:     node.Metadata,
		})
	}
	for _, edge := range body.Edges {
		out.Edges = append(out.Edges, service.DatasetLineageEdgeInput{
			ID:           edge.ID,
			SourceNodeID: edge.SourceNodeID,
			TargetNodeID: edge.TargetNodeID,
			RelationType: edge.RelationType,
			Confidence:   edge.Confidence,
			SourceKind:   edge.SourceKind,
			Label:        edge.Label,
			Description:  edge.Description,
			Expression:   edge.Expression,
			Metadata:     edge.Metadata,
		})
	}
	return out
}

func datasetLineagePositionFromBody(body *DatasetLineagePositionBody) *service.DatasetLineagePosition {
	if body == nil {
		return nil
	}
	return &service.DatasetLineagePosition{X: body.X, Y: body.Y}
}

func datasetLineageSourcesFromQuery(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func toDatasetSyncJobBody(item service.DatasetSyncJob) DatasetSyncJobBody {
	return DatasetSyncJobBody{
		PublicID:            item.PublicID,
		Mode:                item.Mode,
		Status:              item.Status,
		OldRawDatabase:      item.OldRawDatabase,
		OldRawTable:         item.OldRawTable,
		NewRawDatabase:      item.NewRawDatabase,
		NewRawTable:         item.NewRawTable,
		RowCount:            item.RowCount,
		TotalBytes:          item.TotalBytes,
		ErrorSummary:        item.ErrorSummary,
		CleanupStatus:       item.CleanupStatus,
		CleanupErrorSummary: item.CleanupErrorSummary,
		StartedAt:           item.StartedAt,
		CompletedAt:         item.CompletedAt,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
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
	case errors.Is(err, service.ErrDatasetWorkTableExportScheduleNotFound):
		return huma.Error404NotFound("dataset work table export schedule not found")
	case errors.Is(err, service.ErrDatasetSyncNotFound):
		return huma.Error404NotFound("dataset sync job not found")
	case errors.Is(err, service.ErrDatasetWorkTableExportNotReady):
		return huma.Error409Conflict("dataset work table export is not ready")
	case errors.Is(err, service.ErrDatasetSyncAlreadyActive):
		return huma.Error409Conflict("dataset sync job is already active")
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
