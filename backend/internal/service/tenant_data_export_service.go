package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidTenantDataExport  = errors.New("invalid tenant data export")
	ErrTenantDataExportNotFound = errors.New("tenant data export not found")
	ErrTenantDataExportNotReady = errors.New("tenant data export is not ready")
)

type TenantDataExport struct {
	ID           int64
	PublicID     string
	TenantID     int64
	Format       string
	Status       string
	ErrorSummary string
	FileObjectID *int64
	ExpiresAt    time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  *time.Time
}

type TenantDataExportService struct {
	pool         *pgxpool.Pool
	queries      *db.Queries
	outbox       *OutboxService
	files        *FileService
	entitlements *EntitlementService
	audit        AuditRecorder
	ttl          time.Duration
}

func NewTenantDataExportService(pool *pgxpool.Pool, queries *db.Queries, outbox *OutboxService, files *FileService, audit AuditRecorder, ttl time.Duration, entitlements ...*EntitlementService) *TenantDataExportService {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	var entitlementService *EntitlementService
	if len(entitlements) > 0 {
		entitlementService = entitlements[0]
	}
	return &TenantDataExportService{
		pool:         pool,
		queries:      queries,
		outbox:       outbox,
		files:        files,
		entitlements: entitlementService,
		audit:        audit,
		ttl:          ttl,
	}
}

func (s *TenantDataExportService) Create(ctx context.Context, tenantID, userID int64, format string, auditCtx AuditContext) (TenantDataExport, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return TenantDataExport{}, fmt.Errorf("tenant data export service is not configured")
	}
	format = normalizeExportFormat(format)
	if format == "" {
		return TenantDataExport{}, ErrInvalidTenantDataExport
	}
	if format == "csv" && s.entitlements != nil {
		enabled, err := s.entitlements.IsEnabled(ctx, tenantID, FeatureCustomerSignalImportExport)
		if err != nil {
			return TenantDataExport{}, err
		}
		if !enabled {
			return TenantDataExport{}, ErrCustomerSignalImportEntitled
		}
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TenantDataExport{}, fmt.Errorf("begin tenant data export transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	row, err := qtx.CreateTenantDataExport(ctx, db.CreateTenantDataExportParams{
		TenantID:          tenantID,
		RequestedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		Format:            format,
		ExpiresAt:         pgTimestamp(time.Now().Add(s.ttl)),
	})
	if err != nil {
		return TenantDataExport{}, fmt.Errorf("create tenant data export: %w", err)
	}
	if s.outbox != nil {
		event, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
			TenantID:      &tenantID,
			AggregateType: "tenant_data_export",
			AggregateID:   row.PublicID.String(),
			EventType:     "tenant_data_export.requested",
			Payload: map[string]any{
				"exportId": row.ID,
				"tenantId": row.TenantID,
			},
		})
		if err != nil {
			return TenantDataExport{}, err
		}
		row, err = qtx.MarkTenantDataExportProcessing(ctx, db.MarkTenantDataExportProcessingParams{
			ID:            row.ID,
			OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true},
		})
		if err != nil {
			return TenantDataExport{}, fmt.Errorf("mark tenant data export processing: %w", err)
		}
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "tenant_data_export.create",
			TargetType:   "tenant_data_export",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"format": row.Format,
			},
		}); err != nil {
			return TenantDataExport{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return TenantDataExport{}, fmt.Errorf("commit tenant data export transaction: %w", err)
	}
	return tenantDataExportFromDB(row), nil
}

func (s *TenantDataExportService) List(ctx context.Context, tenantID int64, limit int) ([]TenantDataExport, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("tenant data export service is not configured")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.queries.ListTenantDataExports(ctx, db.ListTenantDataExportsParams{
		TenantID: tenantID,
		Limit:    int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list tenant data exports: %w", err)
	}
	items := make([]TenantDataExport, 0, len(rows))
	for _, row := range rows {
		items = append(items, tenantDataExportFromDB(row))
	}
	return items, nil
}

func (s *TenantDataExportService) Get(ctx context.Context, tenantID int64, publicID string) (TenantDataExport, error) {
	row, err := s.getRow(ctx, tenantID, publicID)
	if err != nil {
		return TenantDataExport{}, err
	}
	return tenantDataExportFromDB(row), nil
}

func (s *TenantDataExportService) Download(ctx context.Context, tenantID int64, publicID string) (FileDownload, error) {
	row, err := s.getRow(ctx, tenantID, publicID)
	if err != nil {
		return FileDownload{}, err
	}
	if row.Status != "ready" || !row.FileObjectID.Valid || row.ExpiresAt.Time.Before(time.Now()) {
		return FileDownload{}, ErrTenantDataExportNotReady
	}
	file, err := s.queries.GetFileObjectByIDForTenant(ctx, db.GetFileObjectByIDForTenantParams{
		ID:       row.FileObjectID.Int64,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return FileDownload{}, ErrTenantDataExportNotReady
	}
	if err != nil {
		return FileDownload{}, fmt.Errorf("get export file: %w", err)
	}
	body, err := s.files.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return FileDownload{}, err
	}
	return FileDownload{File: fileObjectFromDB(file), Body: body}, nil
}

func (s *TenantDataExportService) HandleRequested(ctx context.Context, tenantID, exportID int64) error {
	if s == nil || s.queries == nil || s.files == nil {
		return fmt.Errorf("tenant data export service is not configured")
	}
	row, err := s.queries.GetTenantDataExportByIDForTenant(ctx, db.GetTenantDataExportByIDForTenantParams{
		ID:       exportID,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTenantDataExportNotFound
	}
	if err != nil {
		return fmt.Errorf("get tenant data export: %w", err)
	}
	if row.Format == "csv" {
		return s.handleCustomerSignalsCSVExport(ctx, row)
	}
	signals, err := s.queries.ListCustomerSignalsByTenantID(ctx, tenantID)
	if err != nil {
		_, _ = s.queries.MarkTenantDataExportFailed(ctx, db.MarkTenantDataExportFailedParams{ID: exportID, Left: err.Error()})
		return fmt.Errorf("list export customer signals: %w", err)
	}
	files, err := s.queries.ListActiveFileObjectsForTenant(ctx, db.ListActiveFileObjectsForTenantParams{
		TenantID: tenantID,
		Limit:    1000,
	})
	if err != nil {
		_, _ = s.queries.MarkTenantDataExportFailed(ctx, db.MarkTenantDataExportFailedParams{ID: exportID, Left: err.Error()})
		return fmt.Errorf("list export files: %w", err)
	}
	payload := map[string]any{
		"generatedAt":     time.Now().UTC().Format(time.RFC3339),
		"tenantId":        tenantID,
		"exportId":        row.PublicID.String(),
		"customerSignals": exportCustomerSignals(signals),
		"files":           exportFiles(files),
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		_, _ = s.queries.MarkTenantDataExportFailed(ctx, db.MarkTenantDataExportFailedParams{ID: exportID, Left: err.Error()})
		return err
	}
	userID := optionalPgInt8(row.RequestedByUserID)
	file, err := s.files.CreateGeneratedFile(ctx, tenantID, userID, "export", fmt.Sprintf("tenant-export-%s.json", row.PublicID.String()), "application/json", bytes.NewReader(body))
	if err != nil {
		_, _ = s.queries.MarkTenantDataExportFailed(ctx, db.MarkTenantDataExportFailedParams{ID: exportID, Left: err.Error()})
		return err
	}
	if _, err := s.queries.MarkTenantDataExportReady(ctx, db.MarkTenantDataExportReadyParams{
		ID:           exportID,
		FileObjectID: pgtype.Int8{Int64: file.ID, Valid: true},
	}); err != nil {
		return fmt.Errorf("mark tenant data export ready: %w", err)
	}
	return nil
}

func (s *TenantDataExportService) getRow(ctx context.Context, tenantID int64, publicID string) (db.TenantDataExport, error) {
	if s == nil || s.queries == nil {
		return db.TenantDataExport{}, fmt.Errorf("tenant data export service is not configured")
	}
	parsed, err := uuid.Parse(publicID)
	if err != nil {
		return db.TenantDataExport{}, ErrTenantDataExportNotFound
	}
	row, err := s.queries.GetTenantDataExportForTenant(ctx, db.GetTenantDataExportForTenantParams{
		PublicID: parsed,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.TenantDataExport{}, ErrTenantDataExportNotFound
	}
	if err != nil {
		return db.TenantDataExport{}, fmt.Errorf("get tenant data export: %w", err)
	}
	return row, nil
}

func normalizeExportFormat(value string) string {
	switch value {
	case "", "json":
		return "json"
	case "csv":
		return "csv"
	default:
		return ""
	}
}

func (s *TenantDataExportService) handleCustomerSignalsCSVExport(ctx context.Context, row db.TenantDataExport) error {
	signals, err := s.queries.ListCustomerSignalsByTenantID(ctx, row.TenantID)
	if err != nil {
		_, _ = s.queries.MarkTenantDataExportFailed(ctx, db.MarkTenantDataExportFailedParams{ID: row.ID, Left: err.Error()})
		return fmt.Errorf("list export customer signals: %w", err)
	}
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{"public_id", "customer_name", "title", "body", "source", "priority", "status", "created_at", "updated_at"})
	for _, signal := range signals {
		_ = writer.Write([]string{
			signal.PublicID.String(),
			signal.CustomerName,
			signal.Title,
			signal.Body,
			signal.Source,
			signal.Priority,
			signal.Status,
			timestamptzTime(signal.CreatedAt).UTC().Format(time.RFC3339),
			timestamptzTime(signal.UpdatedAt).UTC().Format(time.RFC3339),
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		_, _ = s.queries.MarkTenantDataExportFailed(ctx, db.MarkTenantDataExportFailedParams{ID: row.ID, Left: err.Error()})
		return err
	}
	userID := optionalPgInt8(row.RequestedByUserID)
	file, err := s.files.CreateGeneratedFile(ctx, row.TenantID, userID, "export", fmt.Sprintf("customer-signals-%s.csv", row.PublicID.String()), "text/csv", bytes.NewReader(buf.Bytes()))
	if err != nil {
		_, _ = s.queries.MarkTenantDataExportFailed(ctx, db.MarkTenantDataExportFailedParams{ID: row.ID, Left: err.Error()})
		return err
	}
	if _, err := s.queries.MarkTenantDataExportReady(ctx, db.MarkTenantDataExportReadyParams{
		ID:           row.ID,
		FileObjectID: pgtype.Int8{Int64: file.ID, Valid: true},
	}); err != nil {
		return fmt.Errorf("mark tenant csv export ready: %w", err)
	}
	return nil
}

func tenantDataExportFromDB(row db.TenantDataExport) TenantDataExport {
	return TenantDataExport{
		ID:           row.ID,
		PublicID:     row.PublicID.String(),
		TenantID:     row.TenantID,
		Format:       row.Format,
		Status:       row.Status,
		ErrorSummary: optionalText(row.ErrorSummary),
		FileObjectID: optionalPgInt8(row.FileObjectID),
		ExpiresAt:    row.ExpiresAt.Time,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
		CompletedAt:  timeFromPg(row.CompletedAt),
	}
}

func exportCustomerSignals(rows []db.CustomerSignal) []map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"publicId":     row.PublicID.String(),
			"customerName": row.CustomerName,
			"title":        row.Title,
			"source":       row.Source,
			"priority":     row.Priority,
			"status":       row.Status,
			"createdAt":    row.CreatedAt.Time,
			"updatedAt":    row.UpdatedAt.Time,
		})
	}
	return items
}

func exportFiles(rows []db.FileObject) []map[string]any {
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"publicId":    row.PublicID.String(),
			"purpose":     row.Purpose,
			"contentType": row.ContentType,
			"byteSize":    row.ByteSize,
			"sha256Hex":   row.Sha256Hex,
			"createdAt":   row.CreatedAt.Time,
		})
	}
	return items
}
