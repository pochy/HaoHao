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
)

var (
	ErrDatasetNotFound             = errors.New("dataset not found")
	ErrDatasetQueryNotFound        = errors.New("dataset query not found")
	ErrInvalidDatasetInput         = errors.New("invalid dataset input")
	ErrUnsafeDatasetSQL            = errors.New("unsafe dataset SQL")
	ErrDatasetClickHouseNotReady   = errors.New("clickhouse is not configured")
	datasetExternalFunctionPattern = regexp.MustCompile(`(?i)\b(file|url|s3|s3cluster|hdfs|hdfscluster|postgresql|mysql|mongodb|jdbc|odbc|remote|cluster)\s*\(`)
	datasetTenantDBPattern         = regexp.MustCompile(`(?i)\bhh_t_([0-9]+)_(raw|work)\b`)
	datasetBlockedDBPattern        = regexp.MustCompile(`(?i)\b(system|information_schema|default)\s*\.`)
)

type DatasetClickHouseConfig struct {
	Addr                string
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
	SourceFileObjectID int64
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

type DatasetService struct {
	pool       *pgxpool.Pool
	queries    *db.Queries
	outbox     *OutboxService
	files      *FileService
	audit      AuditRecorder
	chMu       sync.Mutex
	clickhouse driver.Conn
	chConfig   DatasetClickHouseConfig
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
	if tenantID <= 0 || userID <= 0 || file.ID <= 0 || file.TenantID != tenantID || file.Purpose != DatasetSourceFilePurpose {
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
		SourceFileObjectID: file.ID,
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
		_ = s.failImport(ctx, job.ID, job.DatasetID, fmt.Errorf("get dataset: %w", err))
		return err
	}
	_, _ = s.queries.MarkDatasetImporting(ctx, dataset.ID)
	result, err := s.importCSV(ctx, tenantID, dataset, job)
	if err != nil {
		return s.failImport(ctx, job.ID, dataset.ID, err)
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
	download, err := s.files.DownloadByID(ctx, tenantID, job.SourceFileObjectID)
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
			if len(resultRows) >= datasetPreviewRowLimit {
				break
			}
		}
		if err := rows.Err(); err != nil {
			return columns, resultRows, err
		}
		return columns, resultRows, nil
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

func (s *DatasetService) failImport(ctx context.Context, jobID, datasetID int64, cause error) error {
	message := "dataset import failed"
	if cause != nil {
		message = cause.Error()
	}
	_, _ = s.queries.FailDatasetImportJob(ctx, db.FailDatasetImportJobParams{ID: jobID, Left: message})
	_, _ = s.queries.MarkDatasetFailed(ctx, db.MarkDatasetFailedParams{ID: datasetID, Left: message})
	return cause
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

func datasetFromDB(row db.Dataset) Dataset {
	return Dataset{
		ID:                 row.ID,
		PublicID:           row.PublicID.String(),
		TenantID:           row.TenantID,
		CreatedByUserID:    optionalPgInt8(row.CreatedByUserID),
		SourceFileObjectID: row.SourceFileObjectID,
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
