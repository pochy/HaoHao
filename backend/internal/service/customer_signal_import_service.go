package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrCustomerSignalImportNotFound = errors.New("customer signal import not found")
	ErrInvalidCustomerSignalImport  = errors.New("invalid customer signal import")
	ErrCustomerSignalImportEntitled = errors.New("customer signal import/export entitlement denied")
)

const FeatureCustomerSignalImportExport = "customer_signals.import_export"

type CustomerSignalImportJob struct {
	ID                int64
	PublicID          string
	TenantID          int64
	Status            string
	ValidateOnly      bool
	InputFileObjectID int64
	ErrorFileObjectID *int64
	TotalRows         int32
	ValidRows         int32
	InvalidRows       int32
	InsertedRows      int32
	ErrorSummary      string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CompletedAt       *time.Time
}

type CustomerSignalImportInput struct {
	InputFilePublicID string
	ValidateOnly      bool
}

type CustomerSignalImportService struct {
	pool         *pgxpool.Pool
	queries      *db.Queries
	outbox       *OutboxService
	files        *FileService
	entitlements *EntitlementService
	audit        AuditRecorder
}

func NewCustomerSignalImportService(pool *pgxpool.Pool, queries *db.Queries, outbox *OutboxService, files *FileService, entitlements *EntitlementService, audit AuditRecorder) *CustomerSignalImportService {
	return &CustomerSignalImportService{pool: pool, queries: queries, outbox: outbox, files: files, entitlements: entitlements, audit: audit}
}

func (s *CustomerSignalImportService) List(ctx context.Context, tenantID int64, limit int) ([]CustomerSignalImportJob, error) {
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.queries.ListCustomerSignalImportJobs(ctx, db.ListCustomerSignalImportJobsParams{
		TenantID: tenantID,
		Limit:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list customer signal import jobs: %w", err)
	}
	items := make([]CustomerSignalImportJob, 0, len(rows))
	for _, row := range rows {
		items = append(items, customerSignalImportJobFromDB(row))
	}
	return items, nil
}

func (s *CustomerSignalImportService) Get(ctx context.Context, tenantID int64, publicID string) (CustomerSignalImportJob, error) {
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return CustomerSignalImportJob{}, err
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return CustomerSignalImportJob{}, ErrCustomerSignalImportNotFound
	}
	row, err := s.queries.GetCustomerSignalImportJobForTenant(ctx, db.GetCustomerSignalImportJobForTenantParams{
		PublicID: parsed,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerSignalImportJob{}, ErrCustomerSignalImportNotFound
	}
	if err != nil {
		return CustomerSignalImportJob{}, fmt.Errorf("get import job: %w", err)
	}
	return customerSignalImportJobFromDB(row), nil
}

func (s *CustomerSignalImportService) Create(ctx context.Context, tenantID, userID int64, input CustomerSignalImportInput, auditCtx AuditContext) (CustomerSignalImportJob, error) {
	if err := s.requireEnabled(ctx, tenantID); err != nil {
		return CustomerSignalImportJob{}, err
	}
	if s.pool == nil || s.outbox == nil || s.files == nil {
		return CustomerSignalImportJob{}, fmt.Errorf("customer signal import service is not configured")
	}
	file, err := s.files.Get(ctx, tenantID, input.InputFilePublicID)
	if err != nil {
		return CustomerSignalImportJob{}, err
	}
	if file.Purpose != "import" {
		return CustomerSignalImportJob{}, ErrInvalidCustomerSignalImport
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CustomerSignalImportJob{}, fmt.Errorf("begin import transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	row, err := qtx.CreateCustomerSignalImportJob(ctx, db.CreateCustomerSignalImportJobParams{
		TenantID:          tenantID,
		RequestedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		InputFileObjectID: file.ID,
		ValidateOnly:      input.ValidateOnly,
	})
	if err != nil {
		return CustomerSignalImportJob{}, fmt.Errorf("create import job: %w", err)
	}
	event, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "customer_signal_import",
		AggregateID:   row.PublicID.String(),
		EventType:     "customer_signal_import.requested",
		Payload: map[string]any{
			"importJobId": row.ID,
			"tenantId":    tenantID,
		},
	})
	if err != nil {
		return CustomerSignalImportJob{}, err
	}
	row, err = qtx.MarkCustomerSignalImportJobProcessing(ctx, db.MarkCustomerSignalImportJobProcessingParams{
		ID:            row.ID,
		OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true},
	})
	if err != nil {
		return CustomerSignalImportJob{}, fmt.Errorf("mark import processing: %w", err)
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenantID
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "customer_signal_import.create",
			TargetType:   "customer_signal_import",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"validateOnly": row.ValidateOnly,
			},
		}); err != nil {
			return CustomerSignalImportJob{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return CustomerSignalImportJob{}, fmt.Errorf("commit import transaction: %w", err)
	}
	return customerSignalImportJobFromDB(row), nil
}

func (s *CustomerSignalImportService) HandleRequested(ctx context.Context, tenantID, importJobID int64) error {
	if s == nil || s.queries == nil || s.files == nil {
		return fmt.Errorf("customer signal import service is not configured")
	}
	job, err := s.queries.GetCustomerSignalImportJobByIDForTenant(ctx, db.GetCustomerSignalImportJobByIDForTenantParams{
		ID:       importJobID,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCustomerSignalImportNotFound
	}
	if err != nil {
		return fmt.Errorf("get import job: %w", err)
	}
	download, err := s.files.DownloadByID(ctx, tenantID, job.InputFileObjectID)
	if err != nil {
		_, _ = s.queries.FailCustomerSignalImportJob(ctx, db.FailCustomerSignalImportJobParams{ID: importJobID, Left: err.Error()})
		return err
	}
	defer download.Body.Close()
	body, err := io.ReadAll(download.Body)
	if err != nil {
		_, _ = s.queries.FailCustomerSignalImportJob(ctx, db.FailCustomerSignalImportJobParams{ID: importJobID, Left: err.Error()})
		return err
	}
	result := s.validateCSV(body)
	var errorFileID pgtype.Int8
	errorSummary := pgtype.Text{}
	if result.InvalidRows > 0 {
		errorBody := result.ErrorCSV()
		file, err := s.files.CreateGeneratedFile(ctx, tenantID, optionalPgInt8(job.RequestedByUserID), "import", fmt.Sprintf("customer-signal-import-errors-%s.csv", job.PublicID.String()), "text/csv", bytes.NewReader(errorBody))
		if err != nil {
			_, _ = s.queries.FailCustomerSignalImportJob(ctx, db.FailCustomerSignalImportJobParams{ID: importJobID, Left: err.Error()})
			return err
		}
		errorFileID = pgtype.Int8{Int64: file.ID, Valid: true}
		errorSummary = pgtype.Text{String: fmt.Sprintf("%d invalid rows", result.InvalidRows), Valid: true}
	}
	inserted := int32(0)
	if result.InvalidRows == 0 && !job.ValidateOnly {
		for _, item := range result.ValidItems {
			_, err := s.queries.CreateCustomerSignal(ctx, db.CreateCustomerSignalParams{
				TenantID:        tenantID,
				CreatedByUserID: job.RequestedByUserID,
				CustomerName:    item.CustomerName,
				Title:           item.Title,
				Body:            item.Body,
				Source:          item.Source,
				Priority:        item.Priority,
				Status:          item.Status,
			})
			if err != nil {
				_, _ = s.queries.FailCustomerSignalImportJob(ctx, db.FailCustomerSignalImportJobParams{ID: importJobID, Left: err.Error()})
				return err
			}
			inserted++
		}
	}
	if _, err := s.queries.CompleteCustomerSignalImportJob(ctx, db.CompleteCustomerSignalImportJobParams{
		ID:                importJobID,
		TotalRows:         result.TotalRows,
		ValidRows:         int32(len(result.ValidItems)),
		InvalidRows:       result.InvalidRows,
		InsertedRows:      inserted,
		ErrorFileObjectID: errorFileID,
		ErrorSummary:      errorSummary,
	}); err != nil {
		return fmt.Errorf("complete import job: %w", err)
	}
	return nil
}

func (s *CustomerSignalImportService) requireEnabled(ctx context.Context, tenantID int64) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("customer signal import service is not configured")
	}
	if s.entitlements == nil {
		return nil
	}
	enabled, err := s.entitlements.IsEnabled(ctx, tenantID, FeatureCustomerSignalImportExport)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrCustomerSignalImportEntitled
	}
	return nil
}

type customerSignalImportCSVResult struct {
	TotalRows   int32
	InvalidRows int32
	ValidItems  []CustomerSignalCreateInput
	Errors      [][]string
}

func (s *CustomerSignalImportService) validateCSV(body []byte) customerSignalImportCSVResult {
	reader := csv.NewReader(bytes.NewReader(body))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	result := customerSignalImportCSVResult{Errors: [][]string{{"row_number", "error", "raw"}}}
	if err != nil {
		result.InvalidRows = 1
		result.Errors = append(result.Errors, []string{"0", err.Error(), ""})
		return result
	}
	if len(records) == 0 {
		result.InvalidRows = 1
		result.Errors = append(result.Errors, []string{"0", "missing header", ""})
		return result
	}
	expected := []string{"customer_name", "title", "body", "source", "priority", "status"}
	for i, header := range expected {
		if i >= len(records[0]) || strings.TrimSpace(records[0][i]) != header {
			result.InvalidRows = 1
			result.Errors = append(result.Errors, []string{"0", "invalid header", strings.Join(records[0], "|")})
			return result
		}
	}
	for index, record := range records[1:] {
		result.TotalRows++
		rowNumber := index + 2
		if len(record) < len(expected) {
			result.InvalidRows++
			result.Errors = append(result.Errors, []string{fmt.Sprintf("%d", rowNumber), "too few columns", strings.Join(record, "|")})
			continue
		}
		item, err := normalizeCustomerSignalCreateInput(CustomerSignalCreateInput{
			CustomerName: record[0],
			Title:        record[1],
			Body:         record[2],
			Source:       record[3],
			Priority:     record[4],
			Status:       record[5],
		})
		if err != nil {
			result.InvalidRows++
			result.Errors = append(result.Errors, []string{fmt.Sprintf("%d", rowNumber), err.Error(), strings.Join(record, "|")})
			continue
		}
		result.ValidItems = append(result.ValidItems, item)
	}
	return result
}

func (r customerSignalImportCSVResult) ErrorCSV() []byte {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.WriteAll(r.Errors)
	writer.Flush()
	return buf.Bytes()
}

func customerSignalImportJobFromDB(row db.CustomerSignalImportJob) CustomerSignalImportJob {
	return CustomerSignalImportJob{
		ID:                row.ID,
		PublicID:          row.PublicID.String(),
		TenantID:          row.TenantID,
		Status:            row.Status,
		ValidateOnly:      row.ValidateOnly,
		InputFileObjectID: row.InputFileObjectID,
		ErrorFileObjectID: optionalPgInt8(row.ErrorFileObjectID),
		TotalRows:         row.TotalRows,
		ValidRows:         row.ValidRows,
		InvalidRows:       row.InvalidRows,
		InsertedRows:      row.InsertedRows,
		ErrorSummary:      optionalText(row.ErrorSummary),
		CreatedAt:         timestamptzTime(row.CreatedAt),
		UpdatedAt:         timestamptzTime(row.UpdatedAt),
		CompletedAt:       timeFromPg(row.CompletedAt),
	}
}
