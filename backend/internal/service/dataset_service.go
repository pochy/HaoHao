package service

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"example.com/haohao/backend/internal/db"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DatasetSourceFilePurpose = "dataset_source"
	datasetColumnType        = "Nullable(String)"
	datasetInsertBatchSize   = 50000
	datasetPreviewRowLimit   = 1000

	datasetWorkTableExportFormatCSV     = "csv"
	datasetWorkTableExportFormatJSON    = "json"
	datasetWorkTableExportFormatParquet = "parquet"

	datasetWorkTableExportSourceManual    = "manual"
	datasetWorkTableExportSourceScheduled = "scheduled"

	datasetWorkTableExportFrequencyDaily   = "daily"
	datasetWorkTableExportFrequencyWeekly  = "weekly"
	datasetWorkTableExportFrequencyMonthly = "monthly"
)

type datasetWorkTableExportFormatSpec struct {
	Extension   string
	ContentType string
}

var (
	ErrDatasetNotFound                        = errors.New("dataset not found")
	ErrDatasetQueryNotFound                   = errors.New("dataset query not found")
	ErrDatasetWorkTableNotFound               = errors.New("dataset work table not found")
	ErrDatasetWorkTableExportNotFound         = errors.New("dataset work table export not found")
	ErrDatasetWorkTableExportNotReady         = errors.New("dataset work table export is not ready")
	ErrDatasetWorkTableExportScheduleNotFound = errors.New("dataset work table export schedule not found")
	ErrInvalidDatasetInput                    = errors.New("invalid dataset input")
	ErrUnsafeDatasetSQL                       = errors.New("unsafe dataset SQL")
	ErrDatasetClickHouseNotReady              = errors.New("clickhouse is not configured")
	datasetExternalFunctionPattern            = regexp.MustCompile(`(?i)\b(file|url|s3|s3cluster|hdfs|hdfscluster|postgresql|mysql|mongodb|jdbc|odbc|remote|cluster)\s*\(`)
	datasetTenantDBPattern                    = regexp.MustCompile(`(?i)\bhh_t_([0-9]+)_(raw|work)\b`)
	datasetBlockedDBPattern                   = regexp.MustCompile(`(?i)\b(system|information_schema|default)\s*\.`)
)

type DatasetClickHouseConfig struct {
	Addr                string
	HTTPURL             string
	Database            string
	Username            string
	Password            string
	TenantPasswordSalt  string
	QueryMaxSeconds     int
	QueryMaxMemoryBytes int64
	QueryMaxRowsToRead  int64
	QueryMaxThreads     int
}

type Dataset struct {
	ID                 int64
	PublicID           string
	TenantID           int64
	CreatedByUserID    *int64
	SourceFileObjectID *int64
	SourceKind         string
	SourceWorkTableID  *int64
	Name               string
	OriginalFilename   string
	ContentType        string
	ByteSize           int64
	RawDatabase        string
	RawTable           string
	WorkDatabase       string
	Status             string
	RowCount           int64
	ErrorSummary       string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ImportedAt         *time.Time
	Columns            []DatasetColumn
	ImportJob          *DatasetImportJob
}

type DatasetColumn struct {
	Ordinal        int32
	OriginalName   string
	ColumnName     string
	ClickHouseType string
}

type DatasetImportJob struct {
	PublicID     string
	Status       string
	TotalRows    int64
	ValidRows    int64
	InvalidRows  int64
	ErrorSample  []DatasetImportError
	ErrorSummary string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  *time.Time
}

type DatasetImportError struct {
	RowNumber int64  `json:"rowNumber"`
	Error     string `json:"error"`
	Raw       string `json:"raw"`
}

type DatasetQueryJob struct {
	PublicID      string
	DatasetID     *int64
	Statement     string
	Status        string
	ResultColumns []string
	ResultRows    []map[string]any
	RowCount      int32
	ErrorSummary  string
	DurationMs    int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CompletedAt   *time.Time
}

type DatasetWorkTable struct {
	ID                    int64
	PublicID              string
	TenantID              int64
	SourceDatasetID       *int64
	OriginDatasetPublicID string
	OriginDatasetName     string
	CreatedFromQueryJobID *int64
	CreatedByUserID       *int64
	Database              string
	Table                 string
	DisplayName           string
	Status                string
	Managed               bool
	Engine                string
	TotalRows             int64
	TotalBytes            int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DroppedAt             *time.Time
	Columns               []DatasetWorkTableColumn
}

type DatasetWorkTableColumn struct {
	Ordinal        int32
	ColumnName     string
	ClickHouseType string
}

type DatasetWorkTablePreview struct {
	Database    string
	Table       string
	Columns     []string
	PreviewRows []map[string]any
}

type DatasetWorkTableExport struct {
	ID               int64
	PublicID         string
	TenantID         int64
	WorkTableID      int64
	Format           string
	Status           string
	Source           string
	ScheduleID       *int64
	SchedulePublicID string
	ScheduledFor     *time.Time
	ErrorSummary     string
	FileObjectID     *int64
	ExpiresAt        time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      *time.Time
}

type DatasetWorkTableExportSchedule struct {
	ID               int64
	PublicID         string
	TenantID         int64
	WorkTableID      int64
	CreatedByUserID  *int64
	Format           string
	Frequency        string
	Timezone         string
	RunTime          string
	Weekday          *int32
	MonthDay         *int32
	RetentionDays    int32
	Enabled          bool
	NextRunAt        time.Time
	LastRunAt        *time.Time
	LastStatus       string
	LastErrorSummary string
	LastExportID     *int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type DatasetWorkTableExportScheduleInput struct {
	Format        string
	Frequency     string
	Timezone      string
	RunTime       string
	Weekday       *int32
	MonthDay      *int32
	RetentionDays int32
	Enabled       *bool
}

type WorkTableExportScheduleRunSummary struct {
	Claimed  int
	Created  int
	Skipped  int
	Failed   int
	Disabled int
}

type datasetWorkTableRef struct {
	Database string
	Table    string
}

type DatasetService struct {
	pool       *pgxpool.Pool
	queries    *db.Queries
	outbox     *OutboxService
	files      *FileService
	audit      AuditRecorder
	chMu       sync.Mutex
	clickhouse driver.Conn
	chConfig   DatasetClickHouseConfig
	realtime   RealtimePublisher
}

func NewDatasetService(pool *pgxpool.Pool, queries *db.Queries, outbox *OutboxService, files *FileService, audit AuditRecorder, clickhouseConn driver.Conn, chConfig DatasetClickHouseConfig) *DatasetService {
	if chConfig.Database == "" {
		chConfig.Database = "default"
	}
	if chConfig.Username == "" {
		chConfig.Username = "default"
	}
	if chConfig.TenantPasswordSalt == "" {
		chConfig.TenantPasswordSalt = "haohao-local-datasets"
	}
	if chConfig.QueryMaxSeconds <= 0 {
		chConfig.QueryMaxSeconds = 60
	}
	if chConfig.QueryMaxMemoryBytes <= 0 {
		chConfig.QueryMaxMemoryBytes = 4 * 1024 * 1024 * 1024
	}
	if chConfig.QueryMaxRowsToRead <= 0 {
		chConfig.QueryMaxRowsToRead = 100000000
	}
	if chConfig.QueryMaxThreads <= 0 {
		chConfig.QueryMaxThreads = 4
	}
	return &DatasetService{
		pool:       pool,
		queries:    queries,
		outbox:     outbox,
		files:      files,
		audit:      audit,
		clickhouse: clickhouseConn,
		chConfig:   chConfig,
	}
}

func (s *DatasetService) SetRealtimeService(realtime RealtimePublisher) {
	if s != nil {
		s.realtime = realtime
	}
}

func (s *DatasetService) ClickHouseReady() bool {
	if s == nil {
		return false
	}
	s.chMu.Lock()
	defer s.chMu.Unlock()
	return s.clickhouse != nil
}

func (s *DatasetService) EnsureClickHouse(ctx context.Context) error {
	if s == nil {
		return ErrDatasetClickHouseNotReady
	}
	s.chMu.Lock()
	defer s.chMu.Unlock()

	if s.clickhouse != nil {
		if err := s.clickhouse.Ping(ctx); err == nil {
			return nil
		}
		_ = s.clickhouse.Close()
		s.clickhouse = nil
	}

	conn, err := s.openClickHouseAdminConn(ctx)
	if err != nil {
		return err
	}
	s.clickhouse = conn
	return nil
}

func (s *DatasetService) openClickHouseAdminConn(ctx context.Context) (driver.Conn, error) {
	addr := strings.TrimSpace(s.chConfig.Addr)
	if addr == "" {
		return nil, fmt.Errorf("%w: CLICKHOUSE_ADDR is required", ErrDatasetClickHouseNotReady)
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: strings.TrimSpace(s.chConfig.Database),
			Username: strings.TrimSpace(s.chConfig.Username),
			Password: s.chConfig.Password,
		},
		DialTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: open clickhouse connection: %v", ErrDatasetClickHouseNotReady, err)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: ping clickhouse: %v", ErrDatasetClickHouseNotReady, err)
	}
	return conn, nil
}

func (s *DatasetService) CreateFromSourceFile(ctx context.Context, tenantID, userID int64, file FileObject, name string, auditCtx AuditContext) (Dataset, error) {
	if s == nil || s.pool == nil || s.queries == nil || s.files == nil || s.outbox == nil {
		return Dataset{}, fmt.Errorf("dataset service is not configured")
	}
	if err := s.EnsureClickHouse(ctx); err != nil {
		return Dataset{}, err
	}
	if tenantID <= 0 || userID <= 0 || file.ID <= 0 || file.TenantID != tenantID || !datasetSourcePurposeAllowed(file.Purpose) {
		return Dataset{}, ErrInvalidDatasetInput
	}
	if !isDatasetCSVSource(file.OriginalFilename, file.ContentType) {
		return Dataset{}, ErrInvalidDatasetInput
	}
	displayName := normalizeDatasetName(name, file.OriginalFilename)
	rawDB := datasetRawDatabaseName(tenantID)
	workDB := datasetWorkDatabaseName(tenantID)
	rawTable := "ds_" + strings.ReplaceAll(uuid.NewString(), "-", "")

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Dataset{}, fmt.Errorf("begin dataset transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	row, err := qtx.CreateDataset(ctx, db.CreateDatasetParams{
		TenantID:           tenantID,
		CreatedByUserID:    pgtype.Int8{Int64: userID, Valid: true},
		SourceFileObjectID: pgtype.Int8{Int64: file.ID, Valid: true},
		Name:               displayName,
		OriginalFilename:   file.OriginalFilename,
		ContentType:        file.ContentType,
		ByteSize:           file.ByteSize,
		RawDatabase:        rawDB,
		RawTable:           rawTable,
		WorkDatabase:       workDB,
	})
	if err != nil {
		return Dataset{}, fmt.Errorf("create dataset: %w", err)
	}
	job, err := qtx.CreateDatasetImportJob(ctx, db.CreateDatasetImportJobParams{
		TenantID:           tenantID,
		DatasetID:          row.ID,
		SourceFileObjectID: file.ID,
		RequestedByUserID:  pgtype.Int8{Int64: userID, Valid: true},
	})
	if err != nil {
		return Dataset{}, fmt.Errorf("create dataset import job: %w", err)
	}
	if _, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "dataset",
		AggregateID:   row.PublicID.String(),
		EventType:     "dataset.import_requested",
		Payload: map[string]any{
			"tenantId":    tenantID,
			"importJobId": job.ID,
			"datasetId":   row.ID,
		},
	}); err != nil {
		return Dataset{}, fmt.Errorf("enqueue dataset import: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset.create",
			TargetType:   "dataset",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"name":             row.Name,
				"sourceFileObject": file.PublicID,
				"byteSize":         file.ByteSize,
			},
		}); err != nil {
			return Dataset{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Dataset{}, fmt.Errorf("commit dataset transaction: %w", err)
	}
	item := datasetFromDB(row)
	importJob := datasetImportJobFromDB(job)
	item.ImportJob = &importJob
	return item, nil
}

func (s *DatasetService) CreateFromDriveFile(ctx context.Context, tenantID, userID int64, file DriveFile, name string, auditCtx AuditContext) (Dataset, error) {
	source := FileObject{
		ID:               file.ID,
		PublicID:         file.PublicID,
		TenantID:         file.TenantID,
		UploadedByUserID: file.UploadedByUserID,
		Purpose:          "drive",
		OriginalFilename: file.OriginalFilename,
		ContentType:      file.ContentType,
		ByteSize:         file.ByteSize,
		SHA256Hex:        file.SHA256Hex,
		StorageDriver:    file.StorageDriver,
		StorageKey:       file.StorageKey,
		Status:           file.Status,
		CreatedAt:        file.CreatedAt,
		UpdatedAt:        file.UpdatedAt,
		DeletedAt:        file.DeletedAt,
	}
	return s.CreateFromSourceFile(ctx, tenantID, userID, source, name, auditCtx)
}

func (s *DatasetService) List(ctx context.Context, tenantID int64, limit int32) ([]Dataset, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("dataset service is not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.queries.ListDatasets(ctx, db.ListDatasetsParams{TenantID: tenantID, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("list datasets: %w", err)
	}
	items := make([]Dataset, 0, len(rows))
	for _, row := range rows {
		item, err := s.inflateDataset(ctx, row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *DatasetService) Get(ctx context.Context, tenantID int64, publicID string) (Dataset, error) {
	if s == nil || s.queries == nil {
		return Dataset{}, fmt.Errorf("dataset service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return Dataset{}, ErrDatasetNotFound
	}
	row, err := s.queries.GetDatasetForTenant(ctx, db.GetDatasetForTenantParams{PublicID: parsed, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Dataset{}, ErrDatasetNotFound
	}
	if err != nil {
		return Dataset{}, fmt.Errorf("get dataset: %w", err)
	}
	return s.inflateDataset(ctx, row)
}

func (s *DatasetService) Delete(ctx context.Context, tenantID int64, publicID string, auditCtx AuditContext) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("dataset service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return ErrDatasetNotFound
	}
	row, err := s.queries.SoftDeleteDataset(ctx, db.SoftDeleteDatasetParams{PublicID: parsed, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDatasetNotFound
	}
	if err != nil {
		return fmt.Errorf("delete dataset: %w", err)
	}
	if s.clickhouse != nil {
		_ = s.clickhouse.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(row.RawDatabase), quoteCHIdent(row.RawTable)))
	}
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset.delete",
			TargetType:   "dataset",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"name": row.Name,
			},
		})
	}
	return nil
}

func (s *DatasetService) HandleImportRequested(ctx context.Context, tenantID, importJobID, outboxEventID int64) error {
	if s == nil || s.queries == nil || s.files == nil {
		return fmt.Errorf("dataset service is not configured")
	}
	job, err := s.queries.GetDatasetImportJobByIDForTenant(ctx, db.GetDatasetImportJobByIDForTenantParams{ID: importJobID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDatasetNotFound
	}
	if err != nil {
		return fmt.Errorf("get dataset import job: %w", err)
	}
	if job.Status == "completed" {
		return nil
	}
	if _, err := s.queries.MarkDatasetImportJobProcessing(ctx, db.MarkDatasetImportJobProcessingParams{
		ID:            job.ID,
		OutboxEventID: pgtype.Int8{Int64: outboxEventID, Valid: outboxEventID > 0},
	}); err != nil {
		return fmt.Errorf("mark dataset import processing: %w", err)
	}
	dataset, err := s.queries.GetDatasetByIDForTenant(ctx, db.GetDatasetByIDForTenantParams{ID: job.DatasetID, TenantID: tenantID})
	if err != nil {
		_ = s.failImport(ctx, tenantID, job, job.DatasetID, fmt.Errorf("get dataset: %w", err))
		return err
	}
	_, _ = s.queries.MarkDatasetImporting(ctx, dataset.ID)
	result, err := s.importCSV(ctx, tenantID, dataset, job)
	if err != nil {
		return s.failImport(ctx, tenantID, job, dataset.ID, err)
	}
	errorSample, _ := json.Marshal(result.ErrorSample)
	errorSummary := pgtype.Text{}
	if result.InvalidRows > 0 {
		errorSummary = pgtype.Text{String: fmt.Sprintf("%d invalid rows skipped", result.InvalidRows), Valid: true}
	}
	if _, err := s.queries.CompleteDatasetImportJob(ctx, db.CompleteDatasetImportJobParams{
		ID:           job.ID,
		TotalRows:    result.TotalRows,
		ValidRows:    result.ValidRows,
		InvalidRows:  result.InvalidRows,
		ErrorSample:  errorSample,
		ErrorSummary: errorSummary,
	}); err != nil {
		return fmt.Errorf("complete dataset import job: %w", err)
	}
	if _, err := s.queries.MarkDatasetReady(ctx, db.MarkDatasetReadyParams{ID: dataset.ID, RowCount: result.ValidRows}); err != nil {
		return fmt.Errorf("mark dataset ready: %w", err)
	}
	s.publishDatasetImportJobUpdated(ctx, tenantID, job, "completed", "", dataset.PublicID.String(), result.ValidRows)
	s.publishDatasetUpdated(ctx, tenantID, optionalPgInt8(job.RequestedByUserID), dataset.PublicID.String(), "ready", result.ValidRows, "")
	return nil
}

type datasetImportResult struct {
	TotalRows   int64
	ValidRows   int64
	InvalidRows int64
	ErrorSample []DatasetImportError
}

func (s *DatasetService) importCSV(ctx context.Context, tenantID int64, dataset db.Dataset, job db.DatasetImportJob) (datasetImportResult, error) {
	if s.clickhouse == nil {
		return datasetImportResult{}, ErrDatasetClickHouseNotReady
	}
	if err := s.ensureTenantSandbox(ctx, tenantID); err != nil {
		return datasetImportResult{}, err
	}
	download, err := s.files.DownloadActiveByID(ctx, tenantID, job.SourceFileObjectID)
	if err != nil {
		return datasetImportResult{}, err
	}
	defer download.Body.Close()

	reader := csv.NewReader(download.Body)
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return datasetImportResult{}, fmt.Errorf("%w: read csv header: %v", ErrInvalidDatasetInput, err)
	}
	columns := sanitizeDatasetColumns(header)
	if len(columns) == 0 {
		return datasetImportResult{}, fmt.Errorf("%w: csv header is required", ErrInvalidDatasetInput)
	}
	if err := s.replaceDatasetColumns(ctx, dataset.ID, header, columns); err != nil {
		return datasetImportResult{}, err
	}
	if err := s.recreateRawTable(ctx, dataset, columns); err != nil {
		return datasetImportResult{}, err
	}

	insertSQL := datasetInsertSQL(dataset)
	batch, err := s.clickhouse.PrepareBatch(ctx, insertSQL)
	if err != nil {
		return datasetImportResult{}, fmt.Errorf("prepare clickhouse batch: %w", err)
	}
	defer func() {
		if !batch.IsSent() {
			_ = batch.Abort()
		}
	}()

	result := datasetImportResult{ErrorSample: []DatasetImportError{}}
	rowNumber := int64(1)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		rowNumber++
		if err != nil {
			result.TotalRows++
			result.InvalidRows++
			appendDatasetImportError(&result, rowNumber, err.Error(), "")
			continue
		}
		result.TotalRows++
		if len(record) != len(columns) {
			result.InvalidRows++
			appendDatasetImportError(&result, rowNumber, fmt.Sprintf("expected %d columns, got %d", len(columns), len(record)), strings.Join(record, "|"))
			continue
		}
		values := make([]any, 0, len(columns)+1)
		values = append(values, uint64(rowNumber-1))
		for _, value := range record {
			values = append(values, value)
		}
		if err := batch.Append(values...); err != nil {
			return result, fmt.Errorf("append clickhouse batch: %w", err)
		}
		result.ValidRows++
		if batch.Rows() >= datasetInsertBatchSize {
			if err := batch.Send(); err != nil {
				return result, fmt.Errorf("send clickhouse batch: %w", err)
			}
			batch, err = s.clickhouse.PrepareBatch(ctx, insertSQL)
			if err != nil {
				return result, fmt.Errorf("prepare clickhouse batch: %w", err)
			}
		}
	}
	if batch.Rows() > 0 {
		if err := batch.Send(); err != nil {
			return result, fmt.Errorf("send clickhouse batch: %w", err)
		}
	} else {
		_ = batch.Abort()
	}
	return result, nil
}

func (s *DatasetService) replaceDatasetColumns(ctx context.Context, datasetID int64, original []string, columns []string) error {
	if err := s.queries.DeleteDatasetColumns(ctx, datasetID); err != nil {
		return fmt.Errorf("delete dataset columns: %w", err)
	}
	for i, column := range columns {
		originalName := ""
		if i < len(original) {
			originalName = original[i]
		}
		if _, err := s.queries.CreateDatasetColumn(ctx, db.CreateDatasetColumnParams{
			DatasetID:      datasetID,
			Ordinal:        int32(i + 1),
			OriginalName:   originalName,
			ColumnName:     column,
			ClickhouseType: datasetColumnType,
		}); err != nil {
			return fmt.Errorf("create dataset column: %w", err)
		}
	}
	return nil
}

func (s *DatasetService) recreateRawTable(ctx context.Context, dataset db.Dataset, columns []string) error {
	defs := []string{"`__row_number` UInt64"}
	for _, column := range columns {
		defs = append(defs, fmt.Sprintf("%s %s", quoteCHIdent(column), datasetColumnType))
	}
	table := fmt.Sprintf("%s.%s", quoteCHIdent(dataset.RawDatabase), quoteCHIdent(dataset.RawTable))
	if err := s.clickhouse.Exec(ctx, "DROP TABLE IF EXISTS "+table); err != nil {
		return fmt.Errorf("drop clickhouse raw table: %w", err)
	}
	query := fmt.Sprintf(
		"CREATE TABLE %s (%s) ENGINE = MergeTree ORDER BY __row_number",
		table,
		strings.Join(defs, ", "),
	)
	if err := s.clickhouse.Exec(ctx, query); err != nil {
		return fmt.Errorf("create clickhouse raw table: %w", err)
	}
	return nil
}

func (s *DatasetService) CreateQueryJob(ctx context.Context, tenantID, userID int64, statement string) (DatasetQueryJob, error) {
	return s.createQueryJob(ctx, tenantID, userID, nil, statement)
}

func (s *DatasetService) CreateQueryJobForDataset(ctx context.Context, tenantID, userID int64, datasetPublicID, statement string) (DatasetQueryJob, error) {
	dataset, err := s.Get(ctx, tenantID, datasetPublicID)
	if err != nil {
		return DatasetQueryJob{}, err
	}
	return s.createQueryJob(ctx, tenantID, userID, &dataset.ID, statement)
}

func (s *DatasetService) createQueryJob(ctx context.Context, tenantID, userID int64, datasetID *int64, statement string) (DatasetQueryJob, error) {
	if s == nil || s.queries == nil {
		return DatasetQueryJob{}, fmt.Errorf("dataset service is not configured")
	}
	if err := s.EnsureClickHouse(ctx); err != nil {
		return DatasetQueryJob{}, err
	}
	normalized, err := validateDatasetSQL(tenantID, statement)
	if err != nil {
		return DatasetQueryJob{}, err
	}
	start := time.Now()
	job, err := s.queries.CreateDatasetQueryJob(ctx, db.CreateDatasetQueryJobParams{
		TenantID:          tenantID,
		DatasetID:         pgtype.Int8{Int64: derefInt64(datasetID), Valid: datasetID != nil},
		RequestedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		Statement:         normalized,
	})
	if err != nil {
		return DatasetQueryJob{}, fmt.Errorf("create dataset query job: %w", err)
	}
	columns, rows, execErr := s.executeTenantSQL(ctx, tenantID, normalized)
	durationMs := time.Since(start).Milliseconds()
	if execErr != nil {
		failed, failErr := s.queries.FailDatasetQueryJob(ctx, db.FailDatasetQueryJobParams{
			ID:         job.ID,
			Left:       execErr.Error(),
			DurationMs: durationMs,
		})
		if failErr != nil {
			return DatasetQueryJob{}, fmt.Errorf("fail dataset query job: %w", failErr)
		}
		return datasetQueryJobFromDB(failed), nil
	}
	columnBody, _ := json.Marshal(columns)
	rowBody, _ := json.Marshal(rows)
	completed, err := s.queries.CompleteDatasetQueryJob(ctx, db.CompleteDatasetQueryJobParams{
		ID:            job.ID,
		ResultColumns: columnBody,
		ResultRows:    rowBody,
		RowCount:      int32(len(rows)),
		DurationMs:    durationMs,
	})
	if err != nil {
		return DatasetQueryJob{}, fmt.Errorf("complete dataset query job: %w", err)
	}
	if datasetID != nil {
		for _, ref := range parseDatasetCreateTableRefs(tenantID, normalized) {
			_, _ = s.registerDatasetWorkTableForRef(ctx, tenantID, userID, datasetID, &completed.ID, ref.Database, ref.Table, ref.Table)
		}
	}
	return datasetQueryJobFromDB(completed), nil
}

func (s *DatasetService) GetQueryJob(ctx context.Context, tenantID int64, publicID string) (DatasetQueryJob, error) {
	if s == nil || s.queries == nil {
		return DatasetQueryJob{}, fmt.Errorf("dataset service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return DatasetQueryJob{}, ErrDatasetQueryNotFound
	}
	row, err := s.queries.GetDatasetQueryJobForTenant(ctx, db.GetDatasetQueryJobForTenantParams{PublicID: parsed, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetQueryJob{}, ErrDatasetQueryNotFound
	}
	if err != nil {
		return DatasetQueryJob{}, fmt.Errorf("get dataset query job: %w", err)
	}
	return datasetQueryJobFromDB(row), nil
}

func (s *DatasetService) ListQueryJobs(ctx context.Context, tenantID int64, limit int32) ([]DatasetQueryJob, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("dataset service is not configured")
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.queries.ListDatasetQueryJobs(ctx, db.ListDatasetQueryJobsParams{TenantID: tenantID, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("list dataset query jobs: %w", err)
	}
	items := make([]DatasetQueryJob, 0, len(rows))
	for _, row := range rows {
		items = append(items, datasetQueryJobFromDB(row))
	}
	return items, nil
}

func (s *DatasetService) ListQueryJobsForDataset(ctx context.Context, tenantID int64, datasetPublicID string, limit int32) ([]DatasetQueryJob, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("dataset service is not configured")
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	dataset, err := s.Get(ctx, tenantID, datasetPublicID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListDatasetQueryJobsForDataset(ctx, db.ListDatasetQueryJobsForDatasetParams{
		TenantID:  tenantID,
		DatasetID: pgtype.Int8{Int64: dataset.ID, Valid: true},
		Limit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list dataset query jobs for dataset: %w", err)
	}
	items := make([]DatasetQueryJob, 0, len(rows))
	for _, row := range rows {
		items = append(items, datasetQueryJobFromDB(row))
	}
	return items, nil
}

func (s *DatasetService) ListWorkTables(ctx context.Context, tenantID int64, limit int32) ([]DatasetWorkTable, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("dataset service is not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if err := s.ensureTenantSandbox(ctx, tenantID); err != nil {
		return nil, err
	}
	chItems, err := s.listClickHouseWorkTables(ctx, tenantID, limit)
	if err != nil {
		return nil, err
	}
	managedRows, err := s.queries.ListDatasetWorkTables(ctx, db.ListDatasetWorkTablesParams{
		TenantID: tenantID,
		Limit:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list managed dataset work tables: %w", err)
	}
	chByRef := make(map[string]DatasetWorkTable, len(chItems))
	for _, item := range chItems {
		chByRef[workTableKey(item.Database, item.Table)] = item
	}
	activeManagedRefs := map[string]bool{}
	items := make([]DatasetWorkTable, 0, len(managedRows)+len(chItems))
	for _, row := range managedRows {
		item := datasetWorkTableFromDB(row)
		if item.Status == "active" {
			key := workTableKey(item.Database, item.Table)
			activeManagedRefs[key] = true
			if meta, ok := chByRef[key]; ok {
				mergeWorkTableMetadata(&item, meta)
			}
		}
		items = append(items, item)
	}
	for _, item := range chItems {
		if activeManagedRefs[workTableKey(item.Database, item.Table)] {
			continue
		}
		items = append(items, item)
	}
	if err := s.hydrateWorkTableOrigins(ctx, tenantID, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *DatasetService) listClickHouseWorkTables(ctx context.Context, tenantID int64, limit int32) ([]DatasetWorkTable, error) {
	rows, err := s.clickhouse.Query(
		clickhouse.Context(ctx, clickhouse.WithSettings(s.querySettings())),
		fmt.Sprintf(`
SELECT
	database,
	name,
	engine,
	ifNull(total_rows, toUInt64(0)) AS total_rows,
	ifNull(total_bytes, toUInt64(0)) AS total_bytes,
	metadata_modification_time
FROM system.tables
WHERE database = ? AND is_temporary = 0
ORDER BY name ASC
LIMIT %d`, limit),
		datasetWorkDatabaseName(tenantID),
	)
	if err != nil {
		return nil, fmt.Errorf("list dataset work tables: %w", err)
	}
	defer rows.Close()

	items := make([]DatasetWorkTable, 0)
	for rows.Next() {
		var item DatasetWorkTable
		var totalRows uint64
		var totalBytes uint64
		if err := rows.Scan(&item.Database, &item.Table, &item.Engine, &totalRows, &totalBytes, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan dataset work table: %w", err)
		}
		item.DisplayName = item.Table
		item.Status = "unmanaged"
		item.UpdatedAt = item.CreatedAt
		item.TotalRows = uint64ToDatasetInt64(totalRows)
		item.TotalBytes = uint64ToDatasetInt64(totalBytes)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dataset work tables: %w", err)
	}
	return items, nil
}

func (s *DatasetService) GetWorkTable(ctx context.Context, tenantID int64, database, table string) (DatasetWorkTable, error) {
	if s == nil || s.queries == nil {
		return DatasetWorkTable{}, fmt.Errorf("dataset service is not configured")
	}
	database, table, err := validateDatasetWorkTableRef(tenantID, database, table)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	if err := s.ensureTenantSandbox(ctx, tenantID); err != nil {
		return DatasetWorkTable{}, err
	}
	item, err := s.getWorkTableMetadata(ctx, database, table)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	columns, err := s.listWorkTableColumns(ctx, database, table)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	if row, err := s.queries.GetActiveDatasetWorkTableByRefForTenant(ctx, db.GetActiveDatasetWorkTableByRefForTenantParams{
		TenantID:     tenantID,
		WorkDatabase: database,
		WorkTable:    table,
	}); err == nil {
		managed := datasetWorkTableFromDB(row)
		mergeWorkTableMetadata(&managed, item)
		managed.Columns = columns
		managedItems := []DatasetWorkTable{managed}
		if err := s.hydrateWorkTableOrigins(ctx, tenantID, managedItems); err != nil {
			return DatasetWorkTable{}, err
		}
		return managedItems[0], nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return DatasetWorkTable{}, fmt.Errorf("get managed dataset work table by ref: %w", err)
	}
	item.Columns = columns
	item.DisplayName = item.Table
	item.Status = "unmanaged"
	item.UpdatedAt = item.CreatedAt
	return item, nil
}

func (s *DatasetService) ListWorkTablesForDataset(ctx context.Context, tenantID int64, datasetPublicID string, limit int32) ([]DatasetWorkTable, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("dataset service is not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	dataset, err := s.Get(ctx, tenantID, datasetPublicID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListDatasetWorkTablesForDataset(ctx, db.ListDatasetWorkTablesForDatasetParams{
		TenantID:        tenantID,
		SourceDatasetID: pgtype.Int8{Int64: dataset.ID, Valid: true},
		Limit:           limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list dataset work tables for dataset: %w", err)
	}
	items := make([]DatasetWorkTable, 0, len(rows))
	for _, row := range rows {
		item := datasetWorkTableFromDB(row)
		if meta, err := s.getWorkTableMetadata(ctx, item.Database, item.Table); err == nil {
			mergeWorkTableMetadata(&item, meta)
		} else if !errors.Is(err, ErrDatasetWorkTableNotFound) {
			return nil, err
		}
		item.OriginDatasetPublicID = dataset.PublicID
		item.OriginDatasetName = dataset.Name
		items = append(items, item)
	}
	return items, nil
}

func (s *DatasetService) GetManagedWorkTable(ctx context.Context, tenantID int64, publicID string) (DatasetWorkTable, error) {
	row, err := s.getManagedWorkTableRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	item := datasetWorkTableFromDB(row)
	if item.Status == "active" {
		metadata, err := s.getWorkTableMetadata(ctx, item.Database, item.Table)
		if err != nil {
			return DatasetWorkTable{}, err
		}
		columns, err := s.listWorkTableColumns(ctx, item.Database, item.Table)
		if err != nil {
			return DatasetWorkTable{}, err
		}
		mergeWorkTableMetadata(&item, metadata)
		item.Columns = columns
	}
	items := []DatasetWorkTable{item}
	if err := s.hydrateWorkTableOrigins(ctx, tenantID, items); err != nil {
		return DatasetWorkTable{}, err
	}
	return items[0], nil
}

func (s *DatasetService) RegisterWorkTable(ctx context.Context, tenantID, userID int64, database, table, datasetPublicID, displayName string, auditCtx AuditContext) (DatasetWorkTable, error) {
	if s == nil || s.queries == nil {
		return DatasetWorkTable{}, fmt.Errorf("dataset service is not configured")
	}
	database, table, err := validateDatasetWorkTableRef(tenantID, database, table)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	var sourceDatasetID *int64
	var origin Dataset
	if strings.TrimSpace(datasetPublicID) != "" {
		origin, err = s.Get(ctx, tenantID, datasetPublicID)
		if err != nil {
			return DatasetWorkTable{}, err
		}
		sourceDatasetID = &origin.ID
	}
	item, err := s.registerDatasetWorkTableForRef(ctx, tenantID, userID, sourceDatasetID, nil, database, table, displayName)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	if origin.ID > 0 {
		item.OriginDatasetPublicID = origin.PublicID
		item.OriginDatasetName = origin.Name
	}
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_work_table.register",
			TargetType:   "dataset_work_table",
			TargetID:     item.PublicID,
			Metadata: map[string]any{
				"database": database,
				"table":    table,
			},
		})
	}
	return item, nil
}

func (s *DatasetService) LinkWorkTable(ctx context.Context, tenantID int64, publicID, datasetPublicID string, auditCtx AuditContext) (DatasetWorkTable, error) {
	if s == nil || s.queries == nil {
		return DatasetWorkTable{}, fmt.Errorf("dataset service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return DatasetWorkTable{}, ErrDatasetWorkTableNotFound
	}
	dataset, err := s.Get(ctx, tenantID, datasetPublicID)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	row, err := s.queries.LinkDatasetWorkTableToDataset(ctx, db.LinkDatasetWorkTableToDatasetParams{
		PublicID:        parsed,
		TenantID:        tenantID,
		SourceDatasetID: pgtype.Int8{Int64: dataset.ID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetWorkTable{}, ErrDatasetWorkTableNotFound
	}
	if err != nil {
		return DatasetWorkTable{}, fmt.Errorf("link dataset work table: %w", err)
	}
	item := datasetWorkTableFromDB(row)
	item.OriginDatasetPublicID = dataset.PublicID
	item.OriginDatasetName = dataset.Name
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_work_table.link",
			TargetType:   "dataset_work_table",
			TargetID:     item.PublicID,
			Metadata: map[string]any{
				"dataset": dataset.PublicID,
			},
		})
	}
	return item, nil
}

func (s *DatasetService) PreviewWorkTable(ctx context.Context, tenantID int64, database, table string, limit int32) (DatasetWorkTablePreview, error) {
	if s == nil {
		return DatasetWorkTablePreview{}, fmt.Errorf("dataset service is not configured")
	}
	database, table, err := validateDatasetWorkTableRef(tenantID, database, table)
	if err != nil {
		return DatasetWorkTablePreview{}, err
	}
	if limit <= 0 || limit > datasetPreviewRowLimit {
		limit = 100
	}
	if err := s.ensureTenantSandbox(ctx, tenantID); err != nil {
		return DatasetWorkTablePreview{}, err
	}
	if _, err := s.getWorkTableMetadata(ctx, database, table); err != nil {
		return DatasetWorkTablePreview{}, err
	}
	conn, err := s.openTenantConn(ctx, tenantID)
	if err != nil {
		return DatasetWorkTablePreview{}, err
	}
	defer conn.Close()

	query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d", quoteCHIdent(database), quoteCHIdent(table), limit)
	rows, err := conn.Query(clickhouse.Context(ctx, clickhouse.WithSettings(s.querySettings())), query)
	if err != nil {
		return DatasetWorkTablePreview{}, fmt.Errorf("preview dataset work table: %w", err)
	}
	defer rows.Close()
	columns, previewRows, err := scanDatasetRows(rows, int(limit))
	if err != nil {
		return DatasetWorkTablePreview{}, fmt.Errorf("scan dataset work table preview: %w", err)
	}
	return DatasetWorkTablePreview{
		Database:    database,
		Table:       table,
		Columns:     columns,
		PreviewRows: previewRows,
	}, nil
}

func (s *DatasetService) PreviewManagedWorkTable(ctx context.Context, tenantID int64, publicID string, limit int32) (DatasetWorkTablePreview, error) {
	row, err := s.getManagedWorkTableRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetWorkTablePreview{}, err
	}
	if row.Status != "active" || row.DroppedAt.Valid {
		return DatasetWorkTablePreview{}, ErrDatasetWorkTableNotFound
	}
	return s.PreviewWorkTable(ctx, tenantID, row.WorkDatabase, row.WorkTable, limit)
}

func (s *DatasetService) RenameWorkTable(ctx context.Context, tenantID int64, publicID, newTable string, auditCtx AuditContext) (DatasetWorkTable, error) {
	row, err := s.getManagedWorkTableRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	if row.Status != "active" || row.DroppedAt.Valid {
		return DatasetWorkTable{}, ErrDatasetWorkTableNotFound
	}
	database, newTable, err := validateDatasetWorkTableRef(tenantID, row.WorkDatabase, newTable)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	if row.WorkTable == newTable {
		return s.GetManagedWorkTable(ctx, tenantID, publicID)
	}
	if _, err := s.getWorkTableMetadata(ctx, database, newTable); err == nil {
		return DatasetWorkTable{}, fmt.Errorf("%w: destination work table already exists", ErrInvalidDatasetInput)
	} else if !errors.Is(err, ErrDatasetWorkTableNotFound) {
		return DatasetWorkTable{}, err
	}
	statement := fmt.Sprintf("RENAME TABLE %s.%s TO %s.%s", quoteCHIdent(row.WorkDatabase), quoteCHIdent(row.WorkTable), quoteCHIdent(database), quoteCHIdent(newTable))
	if err := s.clickhouse.Exec(ctx, statement); err != nil {
		return DatasetWorkTable{}, fmt.Errorf("rename dataset work table: %w", err)
	}
	metadata, err := s.getWorkTableMetadata(ctx, database, newTable)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	updated, err := s.queries.RenameDatasetWorkTableRecord(ctx, db.RenameDatasetWorkTableRecordParams{
		PublicID:    row.PublicID,
		TenantID:    tenantID,
		WorkTable:   newTable,
		DisplayName: newTable,
		RowCount:    metadata.TotalRows,
		TotalBytes:  metadata.TotalBytes,
		Engine:      metadata.Engine,
	})
	if err != nil {
		return DatasetWorkTable{}, fmt.Errorf("update renamed dataset work table: %w", err)
	}
	item := datasetWorkTableFromDB(updated)
	mergeWorkTableMetadata(&item, metadata)
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_work_table.rename",
			TargetType:   "dataset_work_table",
			TargetID:     item.PublicID,
			Metadata: map[string]any{
				"from": row.WorkTable,
				"to":   newTable,
			},
		})
	}
	return item, nil
}

func (s *DatasetService) TruncateWorkTable(ctx context.Context, tenantID int64, publicID string, auditCtx AuditContext) (DatasetWorkTable, error) {
	row, err := s.getManagedWorkTableRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	if row.Status != "active" || row.DroppedAt.Valid {
		return DatasetWorkTable{}, ErrDatasetWorkTableNotFound
	}
	if err := s.clickhouse.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s.%s", quoteCHIdent(row.WorkDatabase), quoteCHIdent(row.WorkTable))); err != nil {
		return DatasetWorkTable{}, fmt.Errorf("truncate dataset work table: %w", err)
	}
	metadata, err := s.getWorkTableMetadata(ctx, row.WorkDatabase, row.WorkTable)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	updated, err := s.queries.UpdateDatasetWorkTableStats(ctx, db.UpdateDatasetWorkTableStatsParams{
		PublicID:   row.PublicID,
		TenantID:   tenantID,
		RowCount:   metadata.TotalRows,
		TotalBytes: metadata.TotalBytes,
		Engine:     metadata.Engine,
	})
	if err != nil {
		return DatasetWorkTable{}, fmt.Errorf("update truncated dataset work table: %w", err)
	}
	item := datasetWorkTableFromDB(updated)
	mergeWorkTableMetadata(&item, metadata)
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_work_table.truncate",
			TargetType:   "dataset_work_table",
			TargetID:     item.PublicID,
			Metadata: map[string]any{
				"database": row.WorkDatabase,
				"table":    row.WorkTable,
			},
		})
	}
	return item, nil
}

func (s *DatasetService) DropWorkTable(ctx context.Context, tenantID int64, publicID string, auditCtx AuditContext) error {
	row, err := s.getManagedWorkTableRow(ctx, tenantID, publicID)
	if err != nil {
		return err
	}
	if row.Status != "active" || row.DroppedAt.Valid {
		return ErrDatasetWorkTableNotFound
	}
	if err := s.clickhouse.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(row.WorkDatabase), quoteCHIdent(row.WorkTable))); err != nil {
		return fmt.Errorf("drop dataset work table: %w", err)
	}
	dropped, err := s.queries.MarkDatasetWorkTableDropped(ctx, db.MarkDatasetWorkTableDroppedParams{PublicID: row.PublicID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDatasetWorkTableNotFound
	}
	if err != nil {
		return fmt.Errorf("mark dataset work table dropped: %w", err)
	}
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_work_table.drop",
			TargetType:   "dataset_work_table",
			TargetID:     dropped.PublicID.String(),
			Metadata: map[string]any{
				"database": row.WorkDatabase,
				"table":    row.WorkTable,
			},
		})
	}
	return nil
}

func (s *DatasetService) PromoteWorkTable(ctx context.Context, tenantID, userID int64, publicID, name string, auditCtx AuditContext) (Dataset, error) {
	if s == nil || s.pool == nil || s.queries == nil || s.outbox == nil {
		return Dataset{}, fmt.Errorf("dataset service is not configured")
	}
	row, err := s.getManagedWorkTableRow(ctx, tenantID, publicID)
	if err != nil {
		return Dataset{}, err
	}
	if row.Status != "active" || row.DroppedAt.Valid {
		return Dataset{}, ErrDatasetWorkTableNotFound
	}
	metadata, err := s.getWorkTableMetadata(ctx, row.WorkDatabase, row.WorkTable)
	if err != nil {
		return Dataset{}, err
	}
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = row.DisplayName
	}
	if displayName == "" {
		displayName = row.WorkTable
	}
	if len(displayName) > 160 {
		displayName = displayName[:160]
	}
	rawTable := "ds_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Dataset{}, fmt.Errorf("begin work table promotion transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	dataset, err := qtx.CreateDatasetFromWorkTable(ctx, db.CreateDatasetFromWorkTableParams{
		TenantID:          tenantID,
		CreatedByUserID:   pgtype.Int8{Int64: userID, Valid: userID > 0},
		SourceWorkTableID: pgtype.Int8{Int64: row.ID, Valid: true},
		Name:              displayName,
		OriginalFilename:  row.WorkTable + ".work-table",
		ContentType:       "application/vnd.haohao.work-table",
		ByteSize:          metadata.TotalBytes,
		RawDatabase:       datasetRawDatabaseName(tenantID),
		RawTable:          rawTable,
		WorkDatabase:      datasetWorkDatabaseName(tenantID),
		RowCount:          0,
	})
	if err != nil {
		return Dataset{}, fmt.Errorf("create promoted dataset: %w", err)
	}
	if _, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "dataset",
		AggregateID:   dataset.PublicID.String(),
		EventType:     "dataset.work_table_promote_requested",
		Payload: map[string]any{
			"tenantId":    tenantID,
			"datasetId":   dataset.ID,
			"workTableId": row.ID,
		},
	}); err != nil {
		return Dataset{}, fmt.Errorf("enqueue work table promotion: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_work_table.promote",
			TargetType:   "dataset_work_table",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"dataset": dataset.PublicID.String(),
			},
		}); err != nil {
			return Dataset{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Dataset{}, fmt.Errorf("commit work table promotion transaction: %w", err)
	}
	return s.inflateDataset(ctx, dataset)
}

func (s *DatasetService) HandleWorkTablePromotionRequested(ctx context.Context, tenantID, datasetID, workTableID int64) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("dataset service is not configured")
	}
	dataset, err := s.queries.GetDatasetByIDForTenant(ctx, db.GetDatasetByIDForTenantParams{ID: datasetID, TenantID: tenantID})
	if err != nil {
		return fmt.Errorf("get promoted dataset: %w", err)
	}
	workTable, err := s.queries.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: workTableID, TenantID: tenantID})
	if err != nil {
		_, _ = s.queries.MarkDatasetFailed(ctx, db.MarkDatasetFailedParams{ID: datasetID, Left: err.Error()})
		s.publishDatasetUpdated(ctx, tenantID, optionalPgInt8(dataset.CreatedByUserID), dataset.PublicID.String(), "failed", 0, err.Error())
		return fmt.Errorf("get source work table: %w", err)
	}
	if workTable.Status != "active" || workTable.DroppedAt.Valid {
		err := ErrDatasetWorkTableNotFound
		_, _ = s.queries.MarkDatasetFailed(ctx, db.MarkDatasetFailedParams{ID: datasetID, Left: err.Error()})
		s.publishDatasetUpdated(ctx, tenantID, optionalPgInt8(dataset.CreatedByUserID), dataset.PublicID.String(), "failed", 0, err.Error())
		return err
	}
	_, _ = s.queries.MarkDatasetImporting(ctx, dataset.ID)
	if err := s.copyWorkTableToDatasetRaw(ctx, tenantID, dataset, workTable); err != nil {
		_, _ = s.queries.MarkDatasetFailed(ctx, db.MarkDatasetFailedParams{ID: datasetID, Left: err.Error()})
		s.publishDatasetUpdated(ctx, tenantID, optionalPgInt8(dataset.CreatedByUserID), dataset.PublicID.String(), "failed", 0, err.Error())
		return err
	}
	metadata, err := s.getWorkTableMetadata(ctx, dataset.RawDatabase, dataset.RawTable)
	if err != nil {
		_, _ = s.queries.MarkDatasetFailed(ctx, db.MarkDatasetFailedParams{ID: datasetID, Left: err.Error()})
		s.publishDatasetUpdated(ctx, tenantID, optionalPgInt8(dataset.CreatedByUserID), dataset.PublicID.String(), "failed", 0, err.Error())
		return err
	}
	if _, err := s.queries.MarkDatasetReady(ctx, db.MarkDatasetReadyParams{ID: dataset.ID, RowCount: metadata.TotalRows}); err != nil {
		return fmt.Errorf("mark promoted dataset ready: %w", err)
	}
	s.publishDatasetUpdated(ctx, tenantID, optionalPgInt8(dataset.CreatedByUserID), dataset.PublicID.String(), "ready", metadata.TotalRows, "")
	return nil
}

func (s *DatasetService) CreateWorkTableExport(ctx context.Context, tenantID, userID int64, publicID, format string, auditCtx AuditContext) (DatasetWorkTableExport, error) {
	if s == nil || s.pool == nil || s.queries == nil || s.outbox == nil {
		return DatasetWorkTableExport{}, fmt.Errorf("dataset service is not configured")
	}
	format, err := normalizeDatasetWorkTableExportFormat(format)
	if err != nil {
		return DatasetWorkTableExport{}, err
	}
	row, err := s.getManagedWorkTableRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetWorkTableExport{}, err
	}
	if row.Status != "active" || row.DroppedAt.Valid {
		return DatasetWorkTableExport{}, ErrDatasetWorkTableNotFound
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DatasetWorkTableExport{}, fmt.Errorf("begin work table export transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	export, err := s.createWorkTableExportWithQueries(ctx, qtx, tenantID, row, userID, format, time.Now().Add(7*24*time.Hour), nil, nil)
	if err != nil {
		return DatasetWorkTableExport{}, err
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_work_table.export",
			TargetType:   "dataset_work_table",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"export": export.PublicID.String(),
				"format": format,
			},
		}); err != nil {
			return DatasetWorkTableExport{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return DatasetWorkTableExport{}, fmt.Errorf("commit work table export transaction: %w", err)
	}
	s.publishWorkTableExportUpdated(ctx, export, "processing", "", "")
	return datasetWorkTableExportFromDB(export), nil
}

func (s *DatasetService) createWorkTableExportWithQueries(ctx context.Context, qtx *db.Queries, tenantID int64, row db.DatasetWorkTable, userID int64, format string, expiresAt time.Time, scheduleID *int64, scheduledFor *time.Time) (db.DatasetWorkTableExport, error) {
	export, err := qtx.CreateDatasetWorkTableExport(ctx, db.CreateDatasetWorkTableExportParams{
		TenantID:          tenantID,
		WorkTableID:       row.ID,
		RequestedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		Format:            format,
		ExpiresAt:         pgTimestamp(expiresAt),
		ScheduleID:        pgInt8(scheduleID),
		ScheduledFor:      pgTimestampPtr(scheduledFor),
	})
	if err != nil {
		return db.DatasetWorkTableExport{}, fmt.Errorf("create work table export: %w", err)
	}
	event, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "dataset_work_table_export",
		AggregateID:   export.PublicID.String(),
		EventType:     "dataset.work_table_export_requested",
		Payload: map[string]any{
			"tenantId": tenantID,
			"exportId": export.ID,
		},
	})
	if err != nil {
		return db.DatasetWorkTableExport{}, err
	}
	export, err = qtx.MarkDatasetWorkTableExportProcessing(ctx, db.MarkDatasetWorkTableExportProcessingParams{
		ID:            export.ID,
		OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true},
	})
	if err != nil {
		return db.DatasetWorkTableExport{}, fmt.Errorf("mark work table export processing: %w", err)
	}
	return export, nil
}

func (s *DatasetService) ListWorkTableExportSchedules(ctx context.Context, tenantID int64, publicID string) ([]DatasetWorkTableExportSchedule, error) {
	row, err := s.getManagedWorkTableRow(ctx, tenantID, publicID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListDatasetWorkTableExportSchedules(ctx, db.ListDatasetWorkTableExportSchedulesParams{
		TenantID:    tenantID,
		WorkTableID: row.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("list work table export schedules: %w", err)
	}
	items := make([]DatasetWorkTableExportSchedule, 0, len(rows))
	for _, row := range rows {
		items = append(items, datasetWorkTableExportScheduleFromDB(row))
	}
	return items, nil
}

func (s *DatasetService) CreateWorkTableExportSchedule(ctx context.Context, tenantID, userID int64, workTablePublicID string, input DatasetWorkTableExportScheduleInput, auditCtx AuditContext) (DatasetWorkTableExportSchedule, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("dataset service is not configured")
	}
	row, err := s.getManagedWorkTableRow(ctx, tenantID, workTablePublicID)
	if err != nil {
		return DatasetWorkTableExportSchedule{}, err
	}
	if row.Status != "active" || row.DroppedAt.Valid {
		return DatasetWorkTableExportSchedule{}, ErrDatasetWorkTableNotFound
	}
	normalized, nextRun, err := normalizeWorkTableExportScheduleInput(input, time.Now(), nil)
	if err != nil {
		return DatasetWorkTableExportSchedule{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("begin work table export schedule transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	schedule, err := qtx.CreateDatasetWorkTableExportSchedule(ctx, db.CreateDatasetWorkTableExportScheduleParams{
		TenantID:        tenantID,
		WorkTableID:     row.ID,
		CreatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		Format:          normalized.Format,
		Frequency:       normalized.Frequency,
		Timezone:        normalized.Timezone,
		RunTime:         normalized.RunTime,
		RetentionDays:   normalized.RetentionDays,
		Enabled:         normalized.Enabled == nil || *normalized.Enabled,
		NextRunAt:       pgTimestamp(nextRun),
		Weekday:         pgInt2(normalized.Weekday),
		MonthDay:        pgInt2(normalized.MonthDay),
	})
	if err != nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("create work table export schedule: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_work_table_export_schedule.create",
			TargetType:   "dataset_work_table",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"schedule":  schedule.PublicID.String(),
				"format":    schedule.Format,
				"frequency": schedule.Frequency,
			},
		}); err != nil {
			return DatasetWorkTableExportSchedule{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("commit work table export schedule transaction: %w", err)
	}
	return datasetWorkTableExportScheduleFromDB(schedule), nil
}

func (s *DatasetService) UpdateWorkTableExportSchedule(ctx context.Context, tenantID, userID int64, schedulePublicID string, input DatasetWorkTableExportScheduleInput, auditCtx AuditContext) (DatasetWorkTableExportSchedule, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("dataset service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(schedulePublicID))
	if err != nil {
		return DatasetWorkTableExportSchedule{}, ErrDatasetWorkTableExportScheduleNotFound
	}
	existing, err := s.queries.GetDatasetWorkTableExportScheduleForTenant(ctx, db.GetDatasetWorkTableExportScheduleForTenantParams{PublicID: parsed, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetWorkTableExportSchedule{}, ErrDatasetWorkTableExportScheduleNotFound
	}
	if err != nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("get work table export schedule: %w", err)
	}
	merged := mergeWorkTableExportScheduleInput(existing, input)
	normalized, nextRun, err := normalizeWorkTableExportScheduleInput(merged, time.Now(), &existing)
	if err != nil {
		return DatasetWorkTableExportSchedule{}, err
	}
	enabled := existing.Enabled
	if normalized.Enabled != nil {
		enabled = *normalized.Enabled
	}
	if enabled {
		workTable, err := s.queries.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: existing.WorkTableID, TenantID: tenantID})
		if errors.Is(err, pgx.ErrNoRows) {
			return DatasetWorkTableExportSchedule{}, ErrDatasetWorkTableNotFound
		}
		if err != nil {
			return DatasetWorkTableExportSchedule{}, fmt.Errorf("get schedule work table: %w", err)
		}
		if workTable.Status != "active" || workTable.DroppedAt.Valid {
			return DatasetWorkTableExportSchedule{}, ErrDatasetWorkTableNotFound
		}
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("begin work table export schedule update transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	schedule, err := qtx.UpdateDatasetWorkTableExportSchedule(ctx, db.UpdateDatasetWorkTableExportScheduleParams{
		PublicID:      parsed,
		TenantID:      tenantID,
		Format:        normalized.Format,
		Frequency:     normalized.Frequency,
		Timezone:      normalized.Timezone,
		RunTime:       normalized.RunTime,
		RetentionDays: normalized.RetentionDays,
		Enabled:       enabled,
		NextRunAt:     pgTimestamp(nextRun),
		Weekday:       pgInt2(normalized.Weekday),
		MonthDay:      pgInt2(normalized.MonthDay),
	})
	if err != nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("update work table export schedule: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_work_table_export_schedule.update",
			TargetType:   "dataset_work_table_export_schedule",
			TargetID:     schedule.PublicID.String(),
			Metadata: map[string]any{
				"format":    schedule.Format,
				"frequency": schedule.Frequency,
				"enabled":   schedule.Enabled,
			},
		}); err != nil {
			return DatasetWorkTableExportSchedule{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("commit work table export schedule update transaction: %w", err)
	}
	return datasetWorkTableExportScheduleFromDB(schedule), nil
}

func (s *DatasetService) DisableWorkTableExportSchedule(ctx context.Context, tenantID, userID int64, schedulePublicID string, auditCtx AuditContext) (DatasetWorkTableExportSchedule, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("dataset service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(schedulePublicID))
	if err != nil {
		return DatasetWorkTableExportSchedule{}, ErrDatasetWorkTableExportScheduleNotFound
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("begin work table export schedule disable transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	schedule, err := qtx.DisableDatasetWorkTableExportSchedule(ctx, db.DisableDatasetWorkTableExportScheduleParams{PublicID: parsed, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetWorkTableExportSchedule{}, ErrDatasetWorkTableExportScheduleNotFound
	}
	if err != nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("disable work table export schedule: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_work_table_export_schedule.disable",
			TargetType:   "dataset_work_table_export_schedule",
			TargetID:     schedule.PublicID.String(),
			Metadata: map[string]any{
				"actorUserId": userID,
			},
		}); err != nil {
			return DatasetWorkTableExportSchedule{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return DatasetWorkTableExportSchedule{}, fmt.Errorf("commit work table export schedule disable transaction: %w", err)
	}
	return datasetWorkTableExportScheduleFromDB(schedule), nil
}

func (s *DatasetService) RunDueWorkTableExportSchedules(ctx context.Context, now time.Time, batchSize int32) (WorkTableExportScheduleRunSummary, error) {
	var summary WorkTableExportScheduleRunSummary
	if s == nil || s.pool == nil || s.queries == nil || s.outbox == nil {
		return summary, fmt.Errorf("dataset service is not configured")
	}
	if now.IsZero() {
		now = time.Now()
	}
	if batchSize <= 0 {
		batchSize = 20
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return summary, fmt.Errorf("begin work table export schedule run transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	schedules, err := qtx.ClaimDueDatasetWorkTableExportSchedules(ctx, db.ClaimDueDatasetWorkTableExportSchedulesParams{
		Now:        pgTimestamp(now),
		BatchLimit: batchSize,
	})
	if err != nil {
		return summary, fmt.Errorf("claim due work table export schedules: %w", err)
	}
	summary.Claimed = len(schedules)
	createdExports := make([]db.DatasetWorkTableExport, 0, len(schedules))
	for _, schedule := range schedules {
		nextRun, err := nextWorkTableExportScheduleRunAfter(schedule.Frequency, schedule.Timezone, schedule.RunTime, optionalPgInt2(schedule.Weekday), optionalPgInt2(schedule.MonthDay), now)
		if err != nil {
			_, markErr := qtx.MarkDatasetWorkTableExportScheduleFailed(ctx, db.MarkDatasetWorkTableExportScheduleFailedParams{
				Enabled:      false,
				LastRunAt:    pgTimestamp(now),
				LastStatus:   pgText("disabled"),
				ErrorSummary: err.Error(),
				NextRunAt:    schedule.NextRunAt,
				ID:           schedule.ID,
			})
			if markErr != nil {
				return summary, fmt.Errorf("disable invalid work table export schedule: %w", markErr)
			}
			summary.Failed++
			summary.Disabled++
			continue
		}
		workTable, err := qtx.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: schedule.WorkTableID, TenantID: schedule.TenantID})
		if errors.Is(err, pgx.ErrNoRows) {
			_, markErr := qtx.MarkDatasetWorkTableExportScheduleFailed(ctx, db.MarkDatasetWorkTableExportScheduleFailedParams{
				Enabled:      false,
				LastRunAt:    pgTimestamp(now),
				LastStatus:   pgText("disabled"),
				ErrorSummary: ErrDatasetWorkTableNotFound.Error(),
				NextRunAt:    pgTimestamp(nextRun),
				ID:           schedule.ID,
			})
			if markErr != nil {
				return summary, fmt.Errorf("disable inactive work table export schedule: %w", markErr)
			}
			summary.Disabled++
			continue
		}
		if err != nil {
			return summary, fmt.Errorf("get scheduled export work table: %w", err)
		}
		if workTable.Status != "active" || workTable.DroppedAt.Valid {
			_, markErr := qtx.MarkDatasetWorkTableExportScheduleFailed(ctx, db.MarkDatasetWorkTableExportScheduleFailedParams{
				Enabled:      false,
				LastRunAt:    pgTimestamp(now),
				LastStatus:   pgText("disabled"),
				ErrorSummary: ErrDatasetWorkTableNotFound.Error(),
				NextRunAt:    pgTimestamp(nextRun),
				ID:           schedule.ID,
			})
			if markErr != nil {
				return summary, fmt.Errorf("disable inactive work table export schedule: %w", markErr)
			}
			summary.Disabled++
			continue
		}
		activeCount, err := qtx.CountActiveDatasetWorkTableExportsForSchedule(ctx, pgtype.Int8{Int64: schedule.ID, Valid: true})
		if err != nil {
			return summary, fmt.Errorf("count active scheduled exports: %w", err)
		}
		if activeCount > 0 {
			_, err := qtx.MarkDatasetWorkTableExportScheduleSkipped(ctx, db.MarkDatasetWorkTableExportScheduleSkippedParams{
				LastRunAt:    pgTimestamp(now),
				ErrorSummary: "previous scheduled export is still pending or processing",
				NextRunAt:    pgTimestamp(nextRun),
				ID:           schedule.ID,
			})
			if err != nil {
				return summary, fmt.Errorf("mark scheduled export skipped: %w", err)
			}
			summary.Skipped++
			continue
		}
		scheduleID := schedule.ID
		scheduledFor := schedule.NextRunAt.Time
		export, err := s.createWorkTableExportWithQueries(ctx, qtx, schedule.TenantID, workTable, schedule.CreatedByUserID.Int64, schedule.Format, now.Add(time.Duration(schedule.RetentionDays)*24*time.Hour), &scheduleID, &scheduledFor)
		if err != nil {
			return summary, err
		}
		_, err = qtx.MarkDatasetWorkTableExportScheduleCreated(ctx, db.MarkDatasetWorkTableExportScheduleCreatedParams{
			LastRunAt:    pgTimestamp(now),
			LastExportID: pgtype.Int8{Int64: export.ID, Valid: true},
			NextRunAt:    pgTimestamp(nextRun),
			ID:           schedule.ID,
		})
		if err != nil {
			return summary, fmt.Errorf("mark scheduled export created: %w", err)
		}
		createdExports = append(createdExports, export)
		summary.Created++
	}
	if err := tx.Commit(ctx); err != nil {
		return summary, fmt.Errorf("commit work table export schedule run transaction: %w", err)
	}
	for _, export := range createdExports {
		s.publishWorkTableExportUpdated(ctx, export, "processing", "", "")
	}
	return summary, nil
}

func (s *DatasetService) ListWorkTableExports(ctx context.Context, tenantID int64, publicID string, limit int32) ([]DatasetWorkTableExport, error) {
	row, err := s.getManagedWorkTableRow(ctx, tenantID, publicID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.queries.ListDatasetWorkTableExports(ctx, db.ListDatasetWorkTableExportsParams{
		TenantID:    tenantID,
		WorkTableID: row.ID,
		Limit:       limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list work table exports: %w", err)
	}
	items := make([]DatasetWorkTableExport, 0, len(rows))
	for _, row := range rows {
		items = append(items, datasetWorkTableExportFromDB(row))
	}
	s.hydrateWorkTableExportSchedulePublicIDs(ctx, tenantID, items)
	return items, nil
}

func (s *DatasetService) GetWorkTableExport(ctx context.Context, tenantID int64, publicID string) (DatasetWorkTableExport, error) {
	row, err := s.getWorkTableExportRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetWorkTableExport{}, err
	}
	item := datasetWorkTableExportFromDB(row)
	if item.ScheduleID != nil {
		if schedule, err := s.queries.GetDatasetWorkTableExportScheduleByIDForTenant(ctx, db.GetDatasetWorkTableExportScheduleByIDForTenantParams{ID: *item.ScheduleID, TenantID: tenantID}); err == nil {
			item.SchedulePublicID = schedule.PublicID.String()
		}
	}
	return item, nil
}

func (s *DatasetService) DownloadWorkTableExport(ctx context.Context, tenantID int64, publicID string) (FileDownload, error) {
	row, err := s.getWorkTableExportRow(ctx, tenantID, publicID)
	if err != nil {
		return FileDownload{}, err
	}
	if row.Status != "ready" || !row.FileObjectID.Valid || row.ExpiresAt.Time.Before(time.Now()) {
		return FileDownload{}, ErrDatasetWorkTableExportNotReady
	}
	file, err := s.queries.GetFileObjectByIDForTenant(ctx, db.GetFileObjectByIDForTenantParams{ID: row.FileObjectID.Int64, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return FileDownload{}, ErrDatasetWorkTableExportNotReady
	}
	if err != nil {
		return FileDownload{}, fmt.Errorf("get work table export file: %w", err)
	}
	body, err := s.files.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return FileDownload{}, err
	}
	return FileDownload{File: fileObjectFromDB(file), Body: body}, nil
}

func (s *DatasetService) HandleWorkTableExportRequested(ctx context.Context, tenantID, exportID int64) error {
	if s == nil || s.queries == nil || s.files == nil {
		return fmt.Errorf("dataset service is not configured")
	}
	export, err := s.queries.GetDatasetWorkTableExportByIDForTenant(ctx, db.GetDatasetWorkTableExportByIDForTenantParams{ID: exportID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDatasetWorkTableExportNotFound
	}
	if err != nil {
		return fmt.Errorf("get work table export: %w", err)
	}
	workTable, err := s.queries.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: export.WorkTableID, TenantID: tenantID})
	if err != nil {
		s.markWorkTableExportFailed(ctx, export, err.Error())
		return fmt.Errorf("get export work table: %w", err)
	}
	spec, ok := datasetWorkTableExportFormatSpecFor(export.Format)
	if !ok {
		err := fmt.Errorf("%w: unsupported export format %q", ErrInvalidDatasetInput, export.Format)
		s.markWorkTableExportFailed(ctx, export, err.Error())
		return nil
	}

	tmp, err := os.CreateTemp("", "haohao-work-table-export-*"+spec.Extension)
	if err != nil {
		s.markWorkTableExportFailed(ctx, export, "create temporary export file failed")
		return fmt.Errorf("create temporary work table export file: %w", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	if err := s.writeWorkTableExport(ctx, workTable.WorkDatabase, workTable.WorkTable, export.Format, tmp); err != nil {
		s.markWorkTableExportFailed(ctx, export, err.Error())
		if errors.Is(err, ErrInvalidDatasetInput) {
			return nil
		}
		return err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		s.markWorkTableExportFailed(ctx, export, "prepare export file failed")
		return fmt.Errorf("seek work table export file: %w", err)
	}
	file, err := s.files.CreateGeneratedFile(ctx, tenantID, optionalPgInt8(export.RequestedByUserID), "export", fmt.Sprintf("work-table-%s%s", export.PublicID.String(), spec.Extension), spec.ContentType, tmp)
	if err != nil {
		s.markWorkTableExportFailed(ctx, export, err.Error())
		return err
	}
	ready, err := s.queries.MarkDatasetWorkTableExportReady(ctx, db.MarkDatasetWorkTableExportReadyParams{
		ID:           exportID,
		FileObjectID: pgtype.Int8{Int64: file.ID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("mark work table export ready: %w", err)
	}
	s.markWorkTableExportScheduleStatus(ctx, ready, "ready", "")
	s.publishWorkTableExportUpdated(ctx, ready, "ready", "", file.PublicID)
	return nil
}

func (s *DatasetService) getWorkTableMetadata(ctx context.Context, database, table string) (DatasetWorkTable, error) {
	rows, err := s.clickhouse.Query(
		clickhouse.Context(ctx, clickhouse.WithSettings(s.querySettings())),
		`
SELECT
	database,
	name,
	engine,
	ifNull(total_rows, toUInt64(0)) AS total_rows,
	ifNull(total_bytes, toUInt64(0)) AS total_bytes,
	metadata_modification_time
FROM system.tables
WHERE database = ? AND name = ? AND is_temporary = 0
LIMIT 1`,
		database,
		table,
	)
	if err != nil {
		return DatasetWorkTable{}, fmt.Errorf("get dataset work table: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return DatasetWorkTable{}, fmt.Errorf("iterate dataset work table: %w", err)
		}
		return DatasetWorkTable{}, ErrDatasetWorkTableNotFound
	}
	var item DatasetWorkTable
	var totalRows uint64
	var totalBytes uint64
	if err := rows.Scan(&item.Database, &item.Table, &item.Engine, &totalRows, &totalBytes, &item.CreatedAt); err != nil {
		return DatasetWorkTable{}, fmt.Errorf("scan dataset work table: %w", err)
	}
	item.TotalRows = uint64ToDatasetInt64(totalRows)
	item.TotalBytes = uint64ToDatasetInt64(totalBytes)
	if rows.Next() {
		return DatasetWorkTable{}, fmt.Errorf("get dataset work table returned multiple rows")
	}
	if err := rows.Err(); err != nil {
		return DatasetWorkTable{}, fmt.Errorf("iterate dataset work table: %w", err)
	}
	return item, nil
}

func (s *DatasetService) listWorkTableColumns(ctx context.Context, database, table string) ([]DatasetWorkTableColumn, error) {
	rows, err := s.clickhouse.Query(
		clickhouse.Context(ctx, clickhouse.WithSettings(s.querySettings())),
		`
SELECT position, name, type
FROM system.columns
WHERE database = ? AND table = ?
ORDER BY position ASC`,
		database,
		table,
	)
	if err != nil {
		return nil, fmt.Errorf("list dataset work table columns: %w", err)
	}
	defer rows.Close()

	columns := make([]DatasetWorkTableColumn, 0)
	for rows.Next() {
		var position uint64
		var column DatasetWorkTableColumn
		if err := rows.Scan(&position, &column.ColumnName, &column.ClickHouseType); err != nil {
			return nil, fmt.Errorf("scan dataset work table column: %w", err)
		}
		column.Ordinal = int32(uint64ToDatasetInt64(position))
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dataset work table columns: %w", err)
	}
	return columns, nil
}

func (s *DatasetService) getManagedWorkTableRow(ctx context.Context, tenantID int64, publicID string) (db.DatasetWorkTable, error) {
	if s == nil || s.queries == nil {
		return db.DatasetWorkTable{}, fmt.Errorf("dataset service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return db.DatasetWorkTable{}, ErrDatasetWorkTableNotFound
	}
	row, err := s.queries.GetDatasetWorkTableForTenant(ctx, db.GetDatasetWorkTableForTenantParams{PublicID: parsed, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DatasetWorkTable{}, ErrDatasetWorkTableNotFound
	}
	if err != nil {
		return db.DatasetWorkTable{}, fmt.Errorf("get managed dataset work table: %w", err)
	}
	if _, _, err := validateDatasetWorkTableRef(tenantID, row.WorkDatabase, row.WorkTable); err != nil {
		return db.DatasetWorkTable{}, err
	}
	return row, nil
}

func (s *DatasetService) getWorkTableExportRow(ctx context.Context, tenantID int64, publicID string) (db.DatasetWorkTableExport, error) {
	if s == nil || s.queries == nil {
		return db.DatasetWorkTableExport{}, fmt.Errorf("dataset service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return db.DatasetWorkTableExport{}, ErrDatasetWorkTableExportNotFound
	}
	row, err := s.queries.GetDatasetWorkTableExportForTenant(ctx, db.GetDatasetWorkTableExportForTenantParams{PublicID: parsed, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DatasetWorkTableExport{}, ErrDatasetWorkTableExportNotFound
	}
	if err != nil {
		return db.DatasetWorkTableExport{}, fmt.Errorf("get dataset work table export: %w", err)
	}
	return row, nil
}

func (s *DatasetService) hydrateWorkTableExportSchedulePublicIDs(ctx context.Context, tenantID int64, items []DatasetWorkTableExport) {
	if s == nil || s.queries == nil {
		return
	}
	cache := make(map[int64]string)
	for i := range items {
		if items[i].ScheduleID == nil {
			continue
		}
		scheduleID := *items[i].ScheduleID
		if publicID, ok := cache[scheduleID]; ok {
			items[i].SchedulePublicID = publicID
			continue
		}
		schedule, err := s.queries.GetDatasetWorkTableExportScheduleByIDForTenant(ctx, db.GetDatasetWorkTableExportScheduleByIDForTenantParams{ID: scheduleID, TenantID: tenantID})
		if err != nil {
			continue
		}
		publicID := schedule.PublicID.String()
		cache[scheduleID] = publicID
		items[i].SchedulePublicID = publicID
	}
}

func (s *DatasetService) registerDatasetWorkTableForRef(ctx context.Context, tenantID, userID int64, sourceDatasetID *int64, queryJobID *int64, database, table, displayName string) (DatasetWorkTable, error) {
	database, table, err := validateDatasetWorkTableRef(tenantID, database, table)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	metadata, err := s.getWorkTableMetadata(ctx, database, table)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	if strings.TrimSpace(displayName) == "" {
		displayName = table
	}
	row, err := s.queries.UpsertDatasetWorkTable(ctx, db.UpsertDatasetWorkTableParams{
		TenantID:              tenantID,
		SourceDatasetID:       pgInt8(sourceDatasetID),
		CreatedFromQueryJobID: pgInt8(queryJobID),
		CreatedByUserID:       pgtype.Int8{Int64: userID, Valid: userID > 0},
		WorkDatabase:          database,
		WorkTable:             table,
		DisplayName:           displayName,
		RowCount:              metadata.TotalRows,
		TotalBytes:            metadata.TotalBytes,
		Engine:                metadata.Engine,
	})
	if err != nil {
		return DatasetWorkTable{}, fmt.Errorf("register dataset work table: %w", err)
	}
	item := datasetWorkTableFromDB(row)
	mergeWorkTableMetadata(&item, metadata)
	columns, err := s.listWorkTableColumns(ctx, database, table)
	if err != nil {
		return DatasetWorkTable{}, err
	}
	item.Columns = columns
	items := []DatasetWorkTable{item}
	if err := s.hydrateWorkTableOrigins(ctx, tenantID, items); err != nil {
		return DatasetWorkTable{}, err
	}
	return items[0], nil
}

func (s *DatasetService) hydrateWorkTableOrigins(ctx context.Context, tenantID int64, items []DatasetWorkTable) error {
	for i := range items {
		if items[i].SourceDatasetID == nil {
			continue
		}
		dataset, err := s.queries.GetDatasetByIDForTenant(ctx, db.GetDatasetByIDForTenantParams{ID: *items[i].SourceDatasetID, TenantID: tenantID})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("hydrate work table source dataset: %w", err)
		}
		items[i].OriginDatasetPublicID = dataset.PublicID.String()
		items[i].OriginDatasetName = dataset.Name
	}
	return nil
}

func (s *DatasetService) copyWorkTableToDatasetRaw(ctx context.Context, tenantID int64, dataset db.Dataset, workTable db.DatasetWorkTable) error {
	if err := s.ensureTenantSandbox(ctx, tenantID); err != nil {
		return err
	}
	columns, err := s.listWorkTableColumns(ctx, workTable.WorkDatabase, workTable.WorkTable)
	if err != nil {
		return err
	}
	if len(columns) == 0 {
		return fmt.Errorf("%w: work table has no columns", ErrInvalidDatasetInput)
	}
	if err := s.replaceDatasetColumnsWithTypes(ctx, dataset.ID, columns); err != nil {
		return err
	}
	target := fmt.Sprintf("%s.%s", quoteCHIdent(dataset.RawDatabase), quoteCHIdent(dataset.RawTable))
	source := fmt.Sprintf("%s.%s", quoteCHIdent(workTable.WorkDatabase), quoteCHIdent(workTable.WorkTable))
	if err := s.clickhouse.Exec(ctx, "DROP TABLE IF EXISTS "+target); err != nil {
		return fmt.Errorf("drop promoted dataset raw table: %w", err)
	}
	statement := fmt.Sprintf("CREATE TABLE %s ENGINE = MergeTree ORDER BY tuple() AS SELECT * FROM %s", target, source)
	if err := s.clickhouse.Exec(ctx, statement); err != nil {
		return fmt.Errorf("copy work table to raw dataset: %w", err)
	}
	return nil
}

func (s *DatasetService) replaceDatasetColumnsWithTypes(ctx context.Context, datasetID int64, columns []DatasetWorkTableColumn) error {
	if err := s.queries.DeleteDatasetColumns(ctx, datasetID); err != nil {
		return fmt.Errorf("delete dataset columns: %w", err)
	}
	for i, column := range columns {
		if _, err := s.queries.CreateDatasetColumn(ctx, db.CreateDatasetColumnParams{
			DatasetID:      datasetID,
			Ordinal:        int32(i + 1),
			OriginalName:   column.ColumnName,
			ColumnName:     column.ColumnName,
			ClickhouseType: column.ClickHouseType,
		}); err != nil {
			return fmt.Errorf("create dataset column: %w", err)
		}
	}
	return nil
}

func (s *DatasetService) writeWorkTableExport(ctx context.Context, database, table, format string, out io.Writer) error {
	switch format {
	case datasetWorkTableExportFormatCSV:
		return s.writeWorkTableCSV(ctx, database, table, out)
	case datasetWorkTableExportFormatJSON:
		return s.writeWorkTableJSONLines(ctx, database, table, out)
	case datasetWorkTableExportFormatParquet:
		return s.writeWorkTableParquet(ctx, database, table, out)
	default:
		return fmt.Errorf("%w: unsupported export format %q", ErrInvalidDatasetInput, format)
	}
}

func (s *DatasetService) writeWorkTableRows(ctx context.Context, database, table string, handle func(driver.Rows) error) error {
	if _, err := s.getWorkTableMetadata(ctx, database, table); err != nil {
		return err
	}
	rows, err := s.clickhouse.Query(
		clickhouse.Context(ctx, clickhouse.WithSettings(s.exportQuerySettings())),
		fmt.Sprintf("SELECT * FROM %s.%s", quoteCHIdent(database), quoteCHIdent(table)),
	)
	if err != nil {
		return fmt.Errorf("query work table export: %w", err)
	}
	defer rows.Close()
	if err := handle(rows); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate work table export rows: %w", err)
	}
	return nil
}

func (s *DatasetService) writeWorkTableCSV(ctx context.Context, database, table string, out io.Writer) error {
	return s.writeWorkTableRows(ctx, database, table, func(rows driver.Rows) error {
		return writeDatasetRowsCSV(rows, out)
	})
}

func writeDatasetRowsCSV(rows driver.Rows, out io.Writer) error {
	columns := rows.Columns()
	writer := csv.NewWriter(out)
	if err := writer.Write(columns); err != nil {
		return err
	}
	columnTypes := rows.ColumnTypes()
	for rows.Next() {
		holders, dest := datasetScanDestinations(columnTypes, len(columns))
		if err := rows.Scan(dest...); err != nil {
			return fmt.Errorf("scan work table export row: %w", err)
		}
		record := make([]string, len(columns))
		for i := range columns {
			value := datasetJSONValue(datasetScannedValue(holders[i]))
			if value != nil {
				record[i] = fmt.Sprint(value)
			}
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	return nil
}

func (s *DatasetService) writeWorkTableJSONLines(ctx context.Context, database, table string, out io.Writer) error {
	return s.writeWorkTableRows(ctx, database, table, func(rows driver.Rows) error {
		return writeDatasetRowsJSONLines(rows, out)
	})
}

func writeDatasetRowsJSONLines(rows driver.Rows, out io.Writer) error {
	columns := rows.Columns()
	columnTypes := rows.ColumnTypes()
	encoder := json.NewEncoder(out)
	for rows.Next() {
		holders, dest := datasetScanDestinations(columnTypes, len(columns))
		if err := rows.Scan(dest...); err != nil {
			return fmt.Errorf("scan work table export row: %w", err)
		}
		record := make(map[string]any, len(columns))
		for i, column := range columns {
			record[column] = datasetJSONValue(datasetScannedValue(holders[i]))
		}
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}
	return nil
}

func (s *DatasetService) writeWorkTableParquet(ctx context.Context, database, table string, out io.Writer) error {
	if _, err := s.getWorkTableMetadata(ctx, database, table); err != nil {
		return err
	}
	columns, err := s.listWorkTableColumns(ctx, database, table)
	if err != nil {
		return err
	}
	for _, column := range columns {
		if datasetClickHouseTypeUnsupportedForParquet(column.ClickHouseType) {
			return fmt.Errorf("%w: parquet export does not support column %s type %s", ErrInvalidDatasetInput, column.ColumnName, column.ClickHouseType)
		}
	}
	return s.copyWorkTableParquet(ctx, database, table, out)
}

func (s *DatasetService) copyWorkTableParquet(ctx context.Context, database, table string, out io.Writer) error {
	endpoint := strings.TrimSpace(s.chConfig.HTTPURL)
	if endpoint == "" {
		return fmt.Errorf("%w: CLICKHOUSE_HTTP_URL is required for parquet export", ErrDatasetClickHouseNotReady)
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%w: invalid CLICKHOUSE_HTTP_URL", ErrDatasetClickHouseNotReady)
	}
	query := fmt.Sprintf("SELECT * FROM %s.%s FORMAT Parquet", quoteCHIdent(database), quoteCHIdent(table))
	values := parsed.Query()
	if s.chConfig.Database != "" {
		values.Set("database", s.chConfig.Database)
	}
	for key, value := range s.exportQuerySettings() {
		values.Set(key, fmt.Sprint(value))
	}
	parsed.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), strings.NewReader(query))
	if err != nil {
		return fmt.Errorf("create clickhouse parquet request: %w", err)
	}
	username := strings.TrimSpace(s.chConfig.Username)
	if username == "" {
		username = "default"
	}
	req.SetBasicAuth(username, s.chConfig.Password)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request clickhouse parquet export: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return fmt.Errorf("clickhouse parquet export failed: %s", message)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("copy clickhouse parquet export: %w", err)
	}
	return nil
}

func (s *DatasetService) executeTenantSQL(ctx context.Context, tenantID int64, statement string) ([]string, []map[string]any, error) {
	if err := s.ensureTenantSandbox(ctx, tenantID); err != nil {
		return nil, nil, err
	}
	conn, err := s.openTenantConn(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()

	queryCtx := clickhouse.Context(ctx, clickhouse.WithSettings(s.querySettings()))
	if datasetSQLReturnsRows(statement) {
		rows, err := conn.Query(queryCtx, statement)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()
		return scanDatasetRows(rows, datasetPreviewRowLimit)
	}
	if err := conn.Exec(queryCtx, statement); err != nil {
		return nil, nil, err
	}
	return []string{}, []map[string]any{}, nil
}

func (s *DatasetService) ensureTenantSandbox(ctx context.Context, tenantID int64) error {
	if err := s.EnsureClickHouse(ctx); err != nil {
		return err
	}
	rawDB := datasetRawDatabaseName(tenantID)
	workDB := datasetWorkDatabaseName(tenantID)
	user := datasetTenantUserName(tenantID)
	password := s.tenantPassword(tenantID)
	statements := []string{
		"CREATE DATABASE IF NOT EXISTS " + quoteCHIdent(rawDB),
		"CREATE DATABASE IF NOT EXISTS " + quoteCHIdent(workDB),
		fmt.Sprintf("CREATE USER IF NOT EXISTS %s IDENTIFIED WITH sha256_password BY %s", quoteCHIdent(user), quoteCHString(password)),
		fmt.Sprintf("GRANT SELECT ON %s.* TO %s", quoteCHIdent(rawDB), quoteCHIdent(user)),
		fmt.Sprintf("GRANT ALL ON %s.* TO %s", quoteCHIdent(workDB), quoteCHIdent(user)),
	}
	for _, statement := range statements {
		if err := s.clickhouse.Exec(ctx, statement); err != nil {
			return fmt.Errorf("ensure clickhouse tenant sandbox: %w", err)
		}
	}
	return nil
}

func (s *DatasetService) openTenantConn(ctx context.Context, tenantID int64) (driver.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{s.chConfig.Addr},
		Auth: clickhouse.Auth{
			Database: datasetWorkDatabaseName(tenantID),
			Username: datasetTenantUserName(tenantID),
			Password: s.tenantPassword(tenantID),
		},
		DialTimeout: 5 * time.Second,
		ReadTimeout: time.Duration(s.chConfig.QueryMaxSeconds+5) * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open clickhouse tenant connection: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping clickhouse tenant connection: %w", err)
	}
	return conn, nil
}

func (s *DatasetService) querySettings() clickhouse.Settings {
	return clickhouse.Settings{
		"max_execution_time":   s.chConfig.QueryMaxSeconds,
		"max_memory_usage":     s.chConfig.QueryMaxMemoryBytes,
		"max_rows_to_read":     s.chConfig.QueryMaxRowsToRead,
		"max_result_rows":      datasetPreviewRowLimit,
		"result_overflow_mode": "break",
		"max_threads":          s.chConfig.QueryMaxThreads,
	}
}

func (s *DatasetService) exportQuerySettings() clickhouse.Settings {
	return clickhouse.Settings{
		"max_execution_time": s.chConfig.QueryMaxSeconds,
		"max_memory_usage":   s.chConfig.QueryMaxMemoryBytes,
		"max_rows_to_read":   s.chConfig.QueryMaxRowsToRead,
		"max_threads":        s.chConfig.QueryMaxThreads,
	}
}

func (s *DatasetService) tenantPassword(tenantID int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", s.chConfig.TenantPasswordSalt, tenantID)))
	return hex.EncodeToString(sum[:])
}

func (s *DatasetService) inflateDataset(ctx context.Context, row db.Dataset) (Dataset, error) {
	item := datasetFromDB(row)
	columns, err := s.queries.ListDatasetColumns(ctx, row.ID)
	if err != nil {
		return Dataset{}, fmt.Errorf("list dataset columns: %w", err)
	}
	item.Columns = make([]DatasetColumn, 0, len(columns))
	for _, column := range columns {
		item.Columns = append(item.Columns, datasetColumnFromDB(column))
	}
	job, err := s.queries.GetLatestDatasetImportJob(ctx, row.ID)
	if err == nil {
		importJob := datasetImportJobFromDB(job)
		item.ImportJob = &importJob
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Dataset{}, fmt.Errorf("get latest dataset import job: %w", err)
	}
	return item, nil
}

func (s *DatasetService) failImport(ctx context.Context, tenantID int64, job db.DatasetImportJob, datasetID int64, cause error) error {
	message := "dataset import failed"
	if cause != nil {
		message = cause.Error()
	}
	_, _ = s.queries.FailDatasetImportJob(ctx, db.FailDatasetImportJobParams{ID: job.ID, Left: message})
	_, _ = s.queries.MarkDatasetFailed(ctx, db.MarkDatasetFailedParams{ID: datasetID, Left: message})
	datasetPublicID := ""
	if dataset, err := s.queries.GetDatasetByIDForTenant(ctx, db.GetDatasetByIDForTenantParams{ID: datasetID, TenantID: tenantID}); err == nil {
		datasetPublicID = dataset.PublicID.String()
		s.publishDatasetUpdated(ctx, tenantID, optionalPgInt8(job.RequestedByUserID), datasetPublicID, "failed", 0, message)
	}
	s.publishDatasetImportJobUpdated(ctx, tenantID, job, "failed", message, datasetPublicID, 0)
	return cause
}

func (s *DatasetService) publishDatasetImportJobUpdated(ctx context.Context, tenantID int64, job db.DatasetImportJob, status, errorSummary, datasetPublicID string, rowCount int64) {
	if s == nil || s.realtime == nil || !job.RequestedByUserID.Valid {
		return
	}
	payload := map[string]any{
		"status":            status,
		"importJobPublicId": job.PublicID.String(),
	}
	if datasetPublicID != "" {
		payload["datasetPublicId"] = datasetPublicID
	}
	if rowCount > 0 {
		payload["rowCount"] = rowCount
	}
	if errorSummary != "" {
		payload["errorSummary"] = errorSummary
	}
	_, _ = s.realtime.Publish(ctx, RealtimeEventInput{
		TenantID:         &tenantID,
		RecipientUserID:  job.RequestedByUserID.Int64,
		EventType:        "job.updated",
		ResourceType:     "dataset_import",
		ResourcePublicID: job.PublicID.String(),
		Payload:          payload,
	})
}

func (s *DatasetService) publishDatasetUpdated(ctx context.Context, tenantID int64, recipientUserID *int64, datasetPublicID, status string, rowCount int64, errorSummary string) {
	if s == nil || s.realtime == nil || recipientUserID == nil || datasetPublicID == "" {
		return
	}
	payload := map[string]any{
		"status":          status,
		"datasetPublicId": datasetPublicID,
	}
	if rowCount > 0 {
		payload["rowCount"] = rowCount
	}
	if errorSummary != "" {
		payload["errorSummary"] = errorSummary
	}
	_, _ = s.realtime.Publish(ctx, RealtimeEventInput{
		TenantID:         &tenantID,
		RecipientUserID:  *recipientUserID,
		EventType:        "job.updated",
		ResourceType:     "dataset",
		ResourcePublicID: datasetPublicID,
		Payload:          payload,
	})
}

func (s *DatasetService) markWorkTableExportFailed(ctx context.Context, export db.DatasetWorkTableExport, message string) {
	if message == "" {
		message = "work table export failed"
	}
	updated, err := s.queries.MarkDatasetWorkTableExportFailed(ctx, db.MarkDatasetWorkTableExportFailedParams{ID: export.ID, Left: message})
	if err == nil {
		export = updated
	}
	s.markWorkTableExportScheduleStatus(ctx, export, "failed", message)
	s.publishWorkTableExportUpdated(ctx, export, "failed", message, "")
}

func (s *DatasetService) markWorkTableExportScheduleStatus(ctx context.Context, export db.DatasetWorkTableExport, status, errorSummary string) {
	if s == nil || s.queries == nil || !export.ScheduleID.Valid {
		return
	}
	_, _ = s.queries.MarkDatasetWorkTableExportScheduleExportStatus(ctx, db.MarkDatasetWorkTableExportScheduleExportStatusParams{
		LastStatus:   pgText(status),
		ErrorSummary: errorSummary,
		ID:           export.ScheduleID.Int64,
		LastExportID: pgtype.Int8{Int64: export.ID, Valid: true},
	})
}

func (s *DatasetService) publishWorkTableExportUpdated(ctx context.Context, export db.DatasetWorkTableExport, status, errorSummary, filePublicID string) {
	if s == nil || s.realtime == nil || !export.RequestedByUserID.Valid {
		return
	}
	payload := map[string]any{
		"status":         status,
		"exportPublicId": export.PublicID.String(),
		"format":         export.Format,
		"source":         datasetWorkTableExportSourceManual,
		"expiresAt":      export.ExpiresAt.Time,
	}
	if export.ScheduledFor.Valid {
		payload["source"] = datasetWorkTableExportSourceScheduled
		payload["scheduledFor"] = export.ScheduledFor.Time
	}
	if s.queries != nil {
		if workTable, err := s.queries.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: export.WorkTableID, TenantID: export.TenantID}); err == nil {
			payload["workTablePublicId"] = workTable.PublicID.String()
		}
		if export.ScheduleID.Valid {
			if schedule, err := s.queries.GetDatasetWorkTableExportScheduleByIDForTenant(ctx, db.GetDatasetWorkTableExportScheduleByIDForTenantParams{ID: export.ScheduleID.Int64, TenantID: export.TenantID}); err == nil {
				payload["schedulePublicId"] = schedule.PublicID.String()
			}
		}
	}
	if filePublicID != "" {
		payload["filePublicId"] = filePublicID
	}
	if errorSummary != "" {
		payload["errorSummary"] = errorSummary
	}
	_, _ = s.realtime.Publish(ctx, RealtimeEventInput{
		TenantID:         &export.TenantID,
		RecipientUserID:  export.RequestedByUserID.Int64,
		EventType:        "job.updated",
		ResourceType:     "dataset_work_table_export",
		ResourcePublicID: export.PublicID.String(),
		Payload:          payload,
	})
}

func validateDatasetSQL(tenantID int64, statement string) (string, error) {
	normalized := strings.TrimSpace(statement)
	normalized = strings.TrimSuffix(normalized, ";")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return "", fmt.Errorf("%w: SQL statement is required", ErrInvalidDatasetInput)
	}
	if hasMultipleDatasetStatements(statement) {
		return "", fmt.Errorf("%w: only one SQL statement is allowed", ErrUnsafeDatasetSQL)
	}
	stripped := stripDatasetSQLComments(normalized)
	identifierText := datasetSQLIdentifierText(stripped)
	if datasetExternalFunctionPattern.MatchString(identifierText) {
		return "", fmt.Errorf("%w: external table functions are disabled", ErrUnsafeDatasetSQL)
	}
	if datasetBlockedDBPattern.MatchString(identifierText) {
		return "", fmt.Errorf("%w: system/default databases are not available", ErrUnsafeDatasetSQL)
	}
	for _, match := range datasetTenantDBPattern.FindAllStringSubmatch(identifierText, -1) {
		if len(match) < 2 {
			continue
		}
		id, _ := strconv.ParseInt(match[1], 10, 64)
		if id != tenantID {
			return "", fmt.Errorf("%w: cross-tenant databases are not available", ErrUnsafeDatasetSQL)
		}
	}
	return normalized, nil
}

func validateDatasetWorkTableRef(tenantID int64, database, table string) (string, string, error) {
	database = strings.TrimSpace(database)
	table = strings.TrimSpace(table)
	if database != datasetWorkDatabaseName(tenantID) {
		return "", "", ErrDatasetWorkTableNotFound
	}
	if table == "" || len(table) > 256 || hasDatasetIdentifierControlRune(table) {
		return "", "", ErrInvalidDatasetInput
	}
	return database, table, nil
}

func hasDatasetIdentifierControlRune(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func hasMultipleDatasetStatements(statement string) bool {
	inSingle := false
	inDouble := false
	inBacktick := false
	seenTerminator := false
	for i, r := range statement {
		switch r {
		case '\'':
			if !inDouble && !inBacktick {
				if i == 0 || rune(statement[i-1]) != '\\' {
					inSingle = !inSingle
				}
			}
		case '"':
			if !inSingle && !inBacktick {
				inDouble = !inDouble
			}
		case '`':
			if !inSingle && !inDouble {
				inBacktick = !inBacktick
			}
		case ';':
			if !inSingle && !inDouble && !inBacktick {
				if strings.TrimSpace(statement[i+1:]) != "" {
					seenTerminator = true
				}
			}
		}
	}
	return seenTerminator
}

func stripDatasetSQLComments(statement string) string {
	lines := strings.Split(statement, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

func datasetSQLIdentifierText(statement string) string {
	return strings.NewReplacer("`", "", `"`, "").Replace(statement)
}

func datasetSQLReturnsRows(statement string) bool {
	trimmed := strings.TrimSpace(strings.TrimLeft(statement, "("))
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "select", "with", "show", "describe", "desc", "explain":
		return true
	default:
		return false
	}
}

func parseDatasetCreateTableRefs(tenantID int64, statement string) []datasetWorkTableRef {
	tokens := datasetSQLTokens(stripDatasetSQLComments(statement))
	refs := make([]datasetWorkTableRef, 0)
	for i := 0; i < len(tokens); i++ {
		if !strings.EqualFold(tokens[i], "create") {
			continue
		}
		j := i + 1
		if j+1 < len(tokens) && strings.EqualFold(tokens[j], "or") && strings.EqualFold(tokens[j+1], "replace") {
			j += 2
		}
		if j < len(tokens) && strings.EqualFold(tokens[j], "temporary") {
			j++
		}
		if j >= len(tokens) || !strings.EqualFold(tokens[j], "table") {
			continue
		}
		j++
		if j+2 < len(tokens) && strings.EqualFold(tokens[j], "if") && strings.EqualFold(tokens[j+1], "not") && strings.EqualFold(tokens[j+2], "exists") {
			j += 3
		}
		if j >= len(tokens) || tokens[j] == "(" {
			continue
		}
		database := datasetWorkDatabaseName(tenantID)
		table := unquoteDatasetIdentifier(tokens[j])
		if j+2 < len(tokens) && tokens[j+1] == "." {
			database = unquoteDatasetIdentifier(tokens[j])
			table = unquoteDatasetIdentifier(tokens[j+2])
		}
		if database == datasetWorkDatabaseName(tenantID) && table != "" {
			refs = append(refs, datasetWorkTableRef{Database: database, Table: table})
		}
	}
	return refs
}

func datasetSQLTokens(statement string) []string {
	tokens := make([]string, 0)
	for i := 0; i < len(statement); {
		r := rune(statement[i])
		if unicode.IsSpace(r) {
			i++
			continue
		}
		switch statement[i] {
		case '.', '(':
			tokens = append(tokens, string(statement[i]))
			i++
			continue
		case '`', '"':
			quote := statement[i]
			start := i
			i++
			for i < len(statement) {
				if statement[i] == quote {
					if i+1 < len(statement) && statement[i+1] == quote {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			tokens = append(tokens, statement[start:i])
			continue
		case '\'':
			i++
			for i < len(statement) {
				if statement[i] == '\\' && i+1 < len(statement) {
					i += 2
					continue
				}
				if statement[i] == '\'' {
					i++
					break
				}
				i++
			}
			continue
		}
		start := i
		for i < len(statement) {
			ch := statement[i]
			if unicode.IsSpace(rune(ch)) || ch == '.' || ch == '(' || ch == '`' || ch == '"' || ch == '\'' {
				break
			}
			i++
		}
		tokens = append(tokens, statement[start:i])
	}
	return tokens
}

func unquoteDatasetIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		first := value[0]
		last := value[len(value)-1]
		if (first == '`' && last == '`') || (first == '"' && last == '"') {
			inner := value[1 : len(value)-1]
			return strings.ReplaceAll(inner, string([]byte{first, first}), string(first))
		}
	}
	return value
}

func sanitizeDatasetColumns(header []string) []string {
	used := map[string]int{}
	columns := make([]string, 0, len(header))
	for i, item := range header {
		column := sanitizeDatasetColumnName(item)
		if column == "" {
			column = fmt.Sprintf("column_%d", i+1)
		}
		count := used[column]
		used[column] = count + 1
		if count > 0 {
			column = fmt.Sprintf("%s_%d", column, count+1)
			for used[column] > 0 {
				count++
				column = fmt.Sprintf("%s_%d", column, count+1)
			}
			used[column] = 1
		}
		columns = append(columns, column)
	}
	return columns
}

func sanitizeDatasetColumnName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		ok := r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
		if ok {
			if r > unicode.MaxASCII {
				if !lastUnderscore {
					b.WriteByte('_')
					lastUnderscore = true
				}
				continue
			}
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	column := strings.Trim(b.String(), "_")
	if column == "" {
		return ""
	}
	if column[0] >= '0' && column[0] <= '9' {
		column = "c_" + column
	}
	if len(column) > 64 {
		column = strings.TrimRight(column[:64], "_")
	}
	return column
}

func datasetRawDatabaseName(tenantID int64) string {
	return fmt.Sprintf("hh_t_%d_raw", tenantID)
}

func datasetWorkDatabaseName(tenantID int64) string {
	return fmt.Sprintf("hh_t_%d_work", tenantID)
}

func datasetTenantUserName(tenantID int64) string {
	return fmt.Sprintf("hh_t_%d_user", tenantID)
}

func datasetSourcePurposeAllowed(purpose string) bool {
	switch strings.ToLower(strings.TrimSpace(purpose)) {
	case DatasetSourceFilePurpose, "drive":
		return true
	default:
		return false
	}
}

func isDatasetCSVSource(filename, contentType string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if ext == ".csv" {
		return true
	}
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch contentType {
	case "text/csv", "application/csv", "application/vnd.ms-excel":
		return true
	default:
		return false
	}
}

func datasetInsertSQL(dataset db.Dataset) string {
	return fmt.Sprintf("INSERT INTO %s.%s", quoteCHIdent(dataset.RawDatabase), quoteCHIdent(dataset.RawTable))
}

func quoteCHIdent(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func quoteCHString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func normalizeDatasetName(name, filename string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(strings.TrimSpace(filename)), filepath.Ext(filename))
	}
	if name == "" || name == "." {
		name = "Dataset"
	}
	if len(name) > 160 {
		name = name[:160]
	}
	return name
}

func appendDatasetImportError(result *datasetImportResult, rowNumber int64, message, raw string) {
	if len(result.ErrorSample) >= 20 {
		return
	}
	result.ErrorSample = append(result.ErrorSample, DatasetImportError{
		RowNumber: rowNumber,
		Error:     message,
		Raw:       raw,
	})
}

func datasetScanDestinations(columnTypes []driver.ColumnType, columnCount int) ([]any, []any) {
	holders := make([]any, columnCount)
	dest := make([]any, columnCount)
	for i := 0; i < columnCount; i++ {
		var scanType reflect.Type
		if i < len(columnTypes) {
			scanType = columnTypes[i].ScanType()
		}
		if scanType == nil {
			var value any
			holders[i] = &value
			dest[i] = &value
			continue
		}
		holder := reflect.New(scanType)
		holders[i] = holder.Interface()
		dest[i] = holder.Interface()
	}
	return holders, dest
}

func scanDatasetRows(rows driver.Rows, limit int) ([]string, []map[string]any, error) {
	if limit <= 0 || limit > datasetPreviewRowLimit {
		limit = datasetPreviewRowLimit
	}
	columns := rows.Columns()
	columnTypes := rows.ColumnTypes()
	resultRows := make([]map[string]any, 0)
	for rows.Next() {
		holders, dest := datasetScanDestinations(columnTypes, len(columns))
		if err := rows.Scan(dest...); err != nil {
			return columns, resultRows, err
		}
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			row[column] = datasetJSONValue(datasetScannedValue(holders[i]))
		}
		resultRows = append(resultRows, row)
		if len(resultRows) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return columns, resultRows, err
	}
	return columns, resultRows, nil
}

func datasetScannedValue(holder any) any {
	value := reflect.ValueOf(holder)
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return nil
	}
	return value.Interface()
}

func uint64ToDatasetInt64(value uint64) int64 {
	const maxInt64AsUint64 = uint64(1<<63 - 1)
	if value > maxInt64AsUint64 {
		return int64(maxInt64AsUint64)
	}
	return int64(value)
}

func datasetJSONValue(value any) any {
	switch item := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(item)
	case time.Time:
		return item.Format(time.RFC3339Nano)
	case fmt.Stringer:
		return item.String()
	default:
		return item
	}
}

func normalizeDatasetWorkTableExportFormat(value string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(value))
	if format == "" {
		format = datasetWorkTableExportFormatCSV
	}
	if _, ok := datasetWorkTableExportFormatSpecFor(format); !ok {
		return "", fmt.Errorf("%w: unsupported export format %q", ErrInvalidDatasetInput, value)
	}
	return format, nil
}

func datasetWorkTableExportFormatSpecFor(format string) (datasetWorkTableExportFormatSpec, bool) {
	switch format {
	case datasetWorkTableExportFormatCSV:
		return datasetWorkTableExportFormatSpec{Extension: ".csv", ContentType: "text/csv"}, true
	case datasetWorkTableExportFormatJSON:
		return datasetWorkTableExportFormatSpec{Extension: ".ndjson", ContentType: "application/x-ndjson"}, true
	case datasetWorkTableExportFormatParquet:
		return datasetWorkTableExportFormatSpec{Extension: ".parquet", ContentType: "application/vnd.apache.parquet"}, true
	default:
		return datasetWorkTableExportFormatSpec{}, false
	}
}

func mergeWorkTableExportScheduleInput(existing db.DatasetWorkTableExportSchedule, input DatasetWorkTableExportScheduleInput) DatasetWorkTableExportScheduleInput {
	out := DatasetWorkTableExportScheduleInput{
		Format:        existing.Format,
		Frequency:     existing.Frequency,
		Timezone:      existing.Timezone,
		RunTime:       existing.RunTime,
		Weekday:       optionalPgInt2(existing.Weekday),
		MonthDay:      optionalPgInt2(existing.MonthDay),
		RetentionDays: existing.RetentionDays,
		Enabled:       &existing.Enabled,
	}
	if strings.TrimSpace(input.Format) != "" {
		out.Format = input.Format
	}
	if strings.TrimSpace(input.Frequency) != "" {
		out.Frequency = input.Frequency
	}
	if strings.TrimSpace(input.Timezone) != "" {
		out.Timezone = input.Timezone
	}
	if strings.TrimSpace(input.RunTime) != "" {
		out.RunTime = input.RunTime
	}
	if input.Weekday != nil {
		out.Weekday = input.Weekday
	}
	if input.MonthDay != nil {
		out.MonthDay = input.MonthDay
	}
	if input.RetentionDays > 0 {
		out.RetentionDays = input.RetentionDays
	}
	if input.Enabled != nil {
		out.Enabled = input.Enabled
	}
	return out
}

func normalizeWorkTableExportScheduleInput(input DatasetWorkTableExportScheduleInput, now time.Time, _ *db.DatasetWorkTableExportSchedule) (DatasetWorkTableExportScheduleInput, time.Time, error) {
	format, err := normalizeDatasetWorkTableExportFormat(input.Format)
	if err != nil {
		return DatasetWorkTableExportScheduleInput{}, time.Time{}, err
	}
	frequency := strings.ToLower(strings.TrimSpace(input.Frequency))
	if frequency == "" {
		frequency = datasetWorkTableExportFrequencyDaily
	}
	switch frequency {
	case datasetWorkTableExportFrequencyDaily, datasetWorkTableExportFrequencyWeekly, datasetWorkTableExportFrequencyMonthly:
	default:
		return DatasetWorkTableExportScheduleInput{}, time.Time{}, fmt.Errorf("%w: unsupported schedule frequency %q", ErrInvalidDatasetInput, input.Frequency)
	}
	timezone := strings.TrimSpace(input.Timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return DatasetWorkTableExportScheduleInput{}, time.Time{}, fmt.Errorf("%w: invalid schedule timezone %q", ErrInvalidDatasetInput, input.Timezone)
	}
	runTime := strings.TrimSpace(input.RunTime)
	if runTime == "" {
		runTime = "03:00"
	}
	if _, _, err := parseWorkTableExportScheduleRunTime(runTime); err != nil {
		return DatasetWorkTableExportScheduleInput{}, time.Time{}, err
	}
	retentionDays := input.RetentionDays
	if retentionDays == 0 {
		retentionDays = 7
	}
	if retentionDays < 1 || retentionDays > 365 {
		return DatasetWorkTableExportScheduleInput{}, time.Time{}, fmt.Errorf("%w: retentionDays must be between 1 and 365", ErrInvalidDatasetInput)
	}
	normalized := DatasetWorkTableExportScheduleInput{
		Format:        format,
		Frequency:     frequency,
		Timezone:      timezone,
		RunTime:       runTime,
		RetentionDays: retentionDays,
		Enabled:       input.Enabled,
	}
	switch frequency {
	case datasetWorkTableExportFrequencyDaily:
		normalized.Weekday = nil
		normalized.MonthDay = nil
	case datasetWorkTableExportFrequencyWeekly:
		if input.Weekday == nil || *input.Weekday < 1 || *input.Weekday > 7 {
			return DatasetWorkTableExportScheduleInput{}, time.Time{}, fmt.Errorf("%w: weekday must be between 1 and 7 for weekly schedules", ErrInvalidDatasetInput)
		}
		weekday := *input.Weekday
		normalized.Weekday = &weekday
		normalized.MonthDay = nil
	case datasetWorkTableExportFrequencyMonthly:
		if input.MonthDay == nil || *input.MonthDay < 1 || *input.MonthDay > 28 {
			return DatasetWorkTableExportScheduleInput{}, time.Time{}, fmt.Errorf("%w: monthDay must be between 1 and 28 for monthly schedules", ErrInvalidDatasetInput)
		}
		monthDay := *input.MonthDay
		normalized.Weekday = nil
		normalized.MonthDay = &monthDay
	}
	nextRun, err := nextWorkTableExportScheduleRunAfter(normalized.Frequency, normalized.Timezone, normalized.RunTime, normalized.Weekday, normalized.MonthDay, now)
	if err != nil {
		return DatasetWorkTableExportScheduleInput{}, time.Time{}, err
	}
	return normalized, nextRun, nil
}

func nextWorkTableExportScheduleRunAfter(frequency, timezone, runTime string, weekday, monthDay *int32, after time.Time) (time.Time, error) {
	loc, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: invalid schedule timezone %q", ErrInvalidDatasetInput, timezone)
	}
	hour, minute, err := parseWorkTableExportScheduleRunTime(runTime)
	if err != nil {
		return time.Time{}, err
	}
	local := after.In(loc)
	candidateForDate := func(t time.Time, day int) time.Time {
		return time.Date(t.Year(), t.Month(), day, hour, minute, 0, 0, loc)
	}
	var candidate time.Time
	switch strings.ToLower(strings.TrimSpace(frequency)) {
	case datasetWorkTableExportFrequencyDaily:
		candidate = candidateForDate(local, local.Day())
		if !candidate.After(local) {
			candidate = candidate.AddDate(0, 0, 1)
		}
	case datasetWorkTableExportFrequencyWeekly:
		if weekday == nil || *weekday < 1 || *weekday > 7 {
			return time.Time{}, fmt.Errorf("%w: weekday must be between 1 and 7 for weekly schedules", ErrInvalidDatasetInput)
		}
		current := int32(local.Weekday())
		if current == 0 {
			current = 7
		}
		daysUntil := int(*weekday - current)
		if daysUntil < 0 {
			daysUntil += 7
		}
		base := local.AddDate(0, 0, daysUntil)
		candidate = candidateForDate(base, base.Day())
		if !candidate.After(local) {
			candidate = candidate.AddDate(0, 0, 7)
		}
	case datasetWorkTableExportFrequencyMonthly:
		if monthDay == nil || *monthDay < 1 || *monthDay > 28 {
			return time.Time{}, fmt.Errorf("%w: monthDay must be between 1 and 28 for monthly schedules", ErrInvalidDatasetInput)
		}
		candidate = candidateForDate(local, int(*monthDay))
		if !candidate.After(local) {
			nextMonth := local.AddDate(0, 1, 0)
			candidate = time.Date(nextMonth.Year(), nextMonth.Month(), int(*monthDay), hour, minute, 0, 0, loc)
		}
	default:
		return time.Time{}, fmt.Errorf("%w: unsupported schedule frequency %q", ErrInvalidDatasetInput, frequency)
	}
	return candidate.UTC(), nil
}

func parseWorkTableExportScheduleRunTime(value string) (int, int, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, 0, fmt.Errorf("%w: runTime must use HH:MM", ErrInvalidDatasetInput)
	}
	return parsed.Hour(), parsed.Minute(), nil
}

func pgInt2(value *int32) pgtype.Int2 {
	if value == nil {
		return pgtype.Int2{}
	}
	return pgtype.Int2{Int16: int16(*value), Valid: true}
}

func optionalPgInt2(value pgtype.Int2) *int32 {
	if !value.Valid {
		return nil
	}
	v := int32(value.Int16)
	return &v
}

func datasetClickHouseTypeUnsupportedForParquet(chType string) bool {
	base := datasetClickHouseTypeBase(chType)
	unsupportedPrefixes := []string{
		"array(",
		"map(",
		"tuple(",
		"nested(",
		"object(",
		"json",
		"variant(",
		"dynamic",
		"aggregatefunction(",
		"simpleaggregatefunction(",
	}
	for _, prefix := range unsupportedPrefixes {
		if strings.HasPrefix(base, prefix) {
			return true
		}
	}
	return false
}

func datasetClickHouseTypeBase(chType string) string {
	value := strings.ToLower(strings.TrimSpace(chType))
	for {
		switch {
		case strings.HasPrefix(value, "nullable(") && strings.HasSuffix(value, ")"):
			value = strings.TrimSpace(value[len("nullable(") : len(value)-1])
		case strings.HasPrefix(value, "lowcardinality(") && strings.HasSuffix(value, ")"):
			value = strings.TrimSpace(value[len("lowcardinality(") : len(value)-1])
		default:
			return value
		}
	}
}

func datasetFromDB(row db.Dataset) Dataset {
	return Dataset{
		ID:                 row.ID,
		PublicID:           row.PublicID.String(),
		TenantID:           row.TenantID,
		CreatedByUserID:    optionalPgInt8(row.CreatedByUserID),
		SourceFileObjectID: optionalPgInt8(row.SourceFileObjectID),
		SourceKind:         row.SourceKind,
		SourceWorkTableID:  optionalPgInt8(row.SourceWorkTableID),
		Name:               row.Name,
		OriginalFilename:   row.OriginalFilename,
		ContentType:        row.ContentType,
		ByteSize:           row.ByteSize,
		RawDatabase:        row.RawDatabase,
		RawTable:           row.RawTable,
		WorkDatabase:       row.WorkDatabase,
		Status:             row.Status,
		RowCount:           row.RowCount,
		ErrorSummary:       optionalText(row.ErrorSummary),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
		ImportedAt:         optionalPgTime(row.ImportedAt),
	}
}

func datasetWorkTableFromDB(row db.DatasetWorkTable) DatasetWorkTable {
	return DatasetWorkTable{
		ID:                    row.ID,
		PublicID:              row.PublicID.String(),
		TenantID:              row.TenantID,
		SourceDatasetID:       optionalPgInt8(row.SourceDatasetID),
		CreatedFromQueryJobID: optionalPgInt8(row.CreatedFromQueryJobID),
		CreatedByUserID:       optionalPgInt8(row.CreatedByUserID),
		Database:              row.WorkDatabase,
		Table:                 row.WorkTable,
		DisplayName:           row.DisplayName,
		Status:                row.Status,
		Managed:               true,
		Engine:                row.Engine,
		TotalRows:             row.RowCount,
		TotalBytes:            row.TotalBytes,
		CreatedAt:             row.CreatedAt.Time,
		UpdatedAt:             row.UpdatedAt.Time,
		DroppedAt:             optionalPgTime(row.DroppedAt),
	}
}

func datasetWorkTableExportFromDB(row db.DatasetWorkTableExport) DatasetWorkTableExport {
	source := datasetWorkTableExportSourceManual
	if row.ScheduleID.Valid {
		source = datasetWorkTableExportSourceScheduled
	}
	return DatasetWorkTableExport{
		ID:           row.ID,
		PublicID:     row.PublicID.String(),
		TenantID:     row.TenantID,
		WorkTableID:  row.WorkTableID,
		Format:       row.Format,
		Status:       row.Status,
		Source:       source,
		ScheduleID:   optionalPgInt8(row.ScheduleID),
		ScheduledFor: optionalPgTime(row.ScheduledFor),
		ErrorSummary: optionalText(row.ErrorSummary),
		FileObjectID: optionalPgInt8(row.FileObjectID),
		ExpiresAt:    row.ExpiresAt.Time,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
		CompletedAt:  optionalPgTime(row.CompletedAt),
	}
}

func datasetWorkTableExportScheduleFromDB(row db.DatasetWorkTableExportSchedule) DatasetWorkTableExportSchedule {
	return DatasetWorkTableExportSchedule{
		ID:               row.ID,
		PublicID:         row.PublicID.String(),
		TenantID:         row.TenantID,
		WorkTableID:      row.WorkTableID,
		CreatedByUserID:  optionalPgInt8(row.CreatedByUserID),
		Format:           row.Format,
		Frequency:        row.Frequency,
		Timezone:         row.Timezone,
		RunTime:          row.RunTime,
		Weekday:          optionalPgInt2(row.Weekday),
		MonthDay:         optionalPgInt2(row.MonthDay),
		RetentionDays:    row.RetentionDays,
		Enabled:          row.Enabled,
		NextRunAt:        row.NextRunAt.Time,
		LastRunAt:        optionalPgTime(row.LastRunAt),
		LastStatus:       optionalText(row.LastStatus),
		LastErrorSummary: optionalText(row.LastErrorSummary),
		LastExportID:     optionalPgInt8(row.LastExportID),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}

func mergeWorkTableMetadata(item *DatasetWorkTable, metadata DatasetWorkTable) {
	item.Database = metadata.Database
	item.Table = metadata.Table
	if item.DisplayName == "" {
		item.DisplayName = metadata.Table
	}
	item.Engine = metadata.Engine
	item.TotalRows = metadata.TotalRows
	item.TotalBytes = metadata.TotalBytes
	if !metadata.CreatedAt.IsZero() {
		item.CreatedAt = metadata.CreatedAt
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	}
}

func workTableKey(database, table string) string {
	return database + "\x00" + table
}

func datasetColumnFromDB(row db.DatasetColumn) DatasetColumn {
	return DatasetColumn{
		Ordinal:        row.Ordinal,
		OriginalName:   row.OriginalName,
		ColumnName:     row.ColumnName,
		ClickHouseType: row.ClickhouseType,
	}
}

func datasetImportJobFromDB(row db.DatasetImportJob) DatasetImportJob {
	var sample []DatasetImportError
	if len(row.ErrorSample) > 0 {
		_ = json.Unmarshal(row.ErrorSample, &sample)
	}
	return DatasetImportJob{
		PublicID:     row.PublicID.String(),
		Status:       row.Status,
		TotalRows:    row.TotalRows,
		ValidRows:    row.ValidRows,
		InvalidRows:  row.InvalidRows,
		ErrorSample:  sample,
		ErrorSummary: optionalText(row.ErrorSummary),
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
		CompletedAt:  optionalPgTime(row.CompletedAt),
	}
}

func datasetQueryJobFromDB(row db.DatasetQueryJob) DatasetQueryJob {
	var columns []string
	var rows []map[string]any
	if len(row.ResultColumns) > 0 {
		_ = json.Unmarshal(row.ResultColumns, &columns)
	}
	if len(row.ResultRows) > 0 {
		_ = json.Unmarshal(row.ResultRows, &rows)
	}
	return DatasetQueryJob{
		PublicID:      row.PublicID.String(),
		DatasetID:     optionalPgInt8(row.DatasetID),
		Statement:     row.Statement,
		Status:        row.Status,
		ResultColumns: columns,
		ResultRows:    rows,
		RowCount:      row.RowCount,
		ErrorSummary:  optionalText(row.ErrorSummary),
		DurationMs:    row.DurationMs,
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
		CompletedAt:   optionalPgTime(row.CompletedAt),
	}
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
