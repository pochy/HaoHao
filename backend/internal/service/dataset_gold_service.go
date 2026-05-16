package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrDatasetGoldPublicationNotFound  = errors.New("dataset gold publication not found")
	ErrDatasetGoldPublishRunNotFound   = errors.New("dataset gold publish run not found")
	ErrDatasetGoldPublicationConflict  = errors.New("dataset gold publication already exists")
	ErrDatasetGoldPublishAlreadyActive = errors.New("dataset gold publish is already active")
)

type DatasetGoldPublicationInput struct {
	DisplayName string
	Description string
	GoldTable   string
}

type DatasetGoldPublication struct {
	ID                      int64
	PublicID                string
	TenantID                int64
	SourceWorkTableID       int64
	SourceWorkTablePublicID string
	SourceWorkTableName     string
	SourceWorkTableDatabase string
	SourceWorkTableTable    string
	CreatedByUserID         *int64
	UpdatedByUserID         *int64
	PublishedByUserID       *int64
	UnpublishedByUserID     *int64
	ArchivedByUserID        *int64
	LastPublishRunID        *int64
	DisplayName             string
	Description             string
	GoldDatabase            string
	GoldTable               string
	Status                  string
	RowCount                int64
	TotalBytes              int64
	SchemaSummary           map[string]any
	SourceSCD2Summary       *DatasetWorkTableSCD2Summary
	SourceDataPipelineRun   *DatasetGoldSourceDataPipelineRun
	RefreshPolicy           string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	PublishedAt             *time.Time
	UnpublishedAt           *time.Time
	ArchivedAt              *time.Time
	LatestPublishRun        *DatasetGoldPublishRun
}

type DatasetGoldSourceDataPipelineRun struct {
	RunID            int64
	RunOutputID      int64
	PipelinePublicID string
	PipelineName     string
	RunPublicID      string
	RunStatus        string
	OutputNodeID     string
	OutputRowCount   int64
	OutputWriteMode  string
	SCD2MergePolicy  string
	SCD2UniqueKeys   []string
	QualitySummary   *DatasetGoldSourceQualitySummary
	CompletedAt      *time.Time
}

type DatasetGoldSourceQualitySummary struct {
	StepCount                 int64
	WarningCount              int64
	FailedRows                int64
	ReviewItemCount           int64
	QualityRows               int64
	QualityColumns            int64
	ValidationErrors          int64
	ValidationWarnings        int64
	ConfidencePassRows        int64
	ConfidenceNeedsReviewRows int64
	QuarantinedRows           int64
}

type DatasetGoldPublishRun struct {
	ID                            int64
	PublicID                      string
	TenantID                      int64
	PublicationID                 int64
	PublicationPublicID           string
	SourceWorkTableID             int64
	SourceWorkTablePublicID       string
	SourceDataPipelineRunID       *int64
	SourceDataPipelineRunOutputID *int64
	SourceDataPipelineRun         *DatasetGoldSourceDataPipelineRun
	RequestedByUserID             *int64
	OutboxEventID                 *int64
	Status                        string
	GoldDatabase                  string
	GoldTable                     string
	InternalDatabase              string
	InternalTable                 string
	RowCount                      int64
	TotalBytes                    int64
	SchemaSummary                 map[string]any
	ErrorSummary                  string
	StartedAt                     *time.Time
	CompletedAt                   *time.Time
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
}

type DatasetGoldPublicationPreview struct {
	Database    string
	Table       string
	Columns     []string
	PreviewRows []map[string]any
}

func (s *DatasetService) RequestGoldPublication(ctx context.Context, tenantID, userID int64, workTablePublicID string, input DatasetGoldPublicationInput, auditCtx AuditContext) (DatasetGoldPublication, error) {
	if s == nil || s.pool == nil || s.queries == nil || s.outbox == nil {
		return DatasetGoldPublication{}, fmt.Errorf("dataset service is not configured")
	}
	if err := s.ensureTenantSandbox(ctx, tenantID); err != nil {
		return DatasetGoldPublication{}, err
	}
	workTable, err := s.getManagedWorkTableRow(ctx, tenantID, workTablePublicID)
	if err != nil {
		return DatasetGoldPublication{}, err
	}
	if workTable.Status != "active" || workTable.DroppedAt.Valid {
		return DatasetGoldPublication{}, ErrDatasetWorkTableNotFound
	}
	metadata, err := s.getWorkTableMetadata(ctx, workTable.WorkDatabase, workTable.WorkTable)
	if err != nil {
		return DatasetGoldPublication{}, err
	}
	columns, err := s.listWorkTableColumns(ctx, workTable.WorkDatabase, workTable.WorkTable)
	if err != nil {
		return DatasetGoldPublication{}, err
	}
	displayName := normalizeDatasetGoldDisplayName(input.DisplayName, workTable.DisplayName, workTable.WorkTable)
	goldTable := normalizeDatasetGoldTableName(input.GoldTable, displayName, input.GoldTable == "")
	description := strings.TrimSpace(input.Description)
	if len(description) > 1000 {
		description = description[:1000]
	}
	goldDatabase := datasetGoldDatabaseName(tenantID)
	internalDatabase := datasetGoldInternalDatabaseName(tenantID)
	internalTable := "gp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	schemaSummary := medallionWorkTableSchemaSummary(columns)
	sourceRun := s.goldSourceDataPipelineRunForWorkTable(ctx, tenantID, workTable.ID)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DatasetGoldPublication{}, fmt.Errorf("begin gold publication transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	publication, err := qtx.CreateDatasetGoldPublication(ctx, db.CreateDatasetGoldPublicationParams{
		TenantID:          tenantID,
		SourceWorkTableID: workTable.ID,
		CreatedByUserID:   pgtype.Int8{Int64: userID, Valid: userID > 0},
		UpdatedByUserID:   pgtype.Int8{Int64: userID, Valid: userID > 0},
		DisplayName:       displayName,
		Description:       description,
		GoldDatabase:      goldDatabase,
		GoldTable:         goldTable,
		SchemaSummary:     jsonBytes(schemaSummary),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return DatasetGoldPublication{}, ErrDatasetGoldPublicationConflict
		}
		return DatasetGoldPublication{}, fmt.Errorf("create gold publication: %w", err)
	}
	run, err := qtx.CreateDatasetGoldPublishRun(ctx, db.CreateDatasetGoldPublishRunParams{
		TenantID:                      tenantID,
		PublicationID:                 publication.ID,
		SourceWorkTableID:             workTable.ID,
		SourceDataPipelineRunID:       pgtype.Int8{Int64: sourceRunID(sourceRun), Valid: sourceRun != nil && sourceRun.RunID > 0},
		SourceDataPipelineRunOutputID: pgtype.Int8{Int64: sourceRunOutputID(sourceRun), Valid: sourceRun != nil && sourceRun.RunOutputID > 0},
		RequestedByUserID:             pgtype.Int8{Int64: userID, Valid: userID > 0},
		GoldDatabase:                  goldDatabase,
		GoldTable:                     goldTable,
		InternalDatabase:              internalDatabase,
		InternalTable:                 internalTable,
		SchemaSummary:                 jsonBytes(schemaSummary),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return DatasetGoldPublication{}, ErrDatasetGoldPublishAlreadyActive
		}
		return DatasetGoldPublication{}, fmt.Errorf("create gold publish run: %w", err)
	}
	event, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "dataset_gold_publish",
		AggregateID:   run.PublicID.String(),
		EventType:     "dataset.gold_publish_requested",
		Payload: map[string]any{
			"tenantId":     tenantID,
			"publishRunId": run.ID,
		},
	})
	if err != nil {
		return DatasetGoldPublication{}, fmt.Errorf("enqueue gold publish: %w", err)
	}
	run, err = qtx.LinkDatasetGoldPublishRunOutboxEvent(ctx, db.LinkDatasetGoldPublishRunOutboxEventParams{
		OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true},
		ID:            run.ID,
		TenantID:      tenantID,
	})
	if err != nil {
		return DatasetGoldPublication{}, fmt.Errorf("link gold publish outbox event: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_gold_publication.create",
			TargetType:   "dataset_work_table",
			TargetID:     workTable.PublicID.String(),
			Metadata: map[string]any{
				"goldPublication": publication.PublicID.String(),
				"goldTable":       goldTable,
				"rowCount":        metadata.TotalRows,
			},
		}); err != nil {
			return DatasetGoldPublication{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return DatasetGoldPublication{}, fmt.Errorf("commit gold publication transaction: %w", err)
	}
	item := datasetGoldPublicationFromDB(publication)
	item.LatestPublishRun = ptrDatasetGoldPublishRun(datasetGoldPublishRunFromDB(run))
	s.hydrateGoldPublication(ctx, tenantID, &item)
	s.recordMedallionGoldPublish(ctx, tenantID, workTable, publication, run, MedallionPipelineStatusPending, "", true)
	s.publishGoldPublishRunUpdated(ctx, tenantID, run, "pending", "")
	return item, nil
}

func (s *DatasetService) RequestGoldRefresh(ctx context.Context, tenantID, userID int64, publicID string, auditCtx AuditContext) (DatasetGoldPublishRun, error) {
	if s == nil || s.pool == nil || s.queries == nil || s.outbox == nil {
		return DatasetGoldPublishRun{}, fmt.Errorf("dataset service is not configured")
	}
	publicationRow, err := s.getGoldPublicationRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetGoldPublishRun{}, err
	}
	if publicationRow.Status == "archived" || publicationRow.ArchivedAt.Valid || publicationRow.Status == "unpublished" {
		return DatasetGoldPublishRun{}, ErrDatasetGoldPublicationNotFound
	}
	workTable, err := s.queries.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: publicationRow.SourceWorkTableID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetGoldPublishRun{}, ErrDatasetWorkTableNotFound
	}
	if err != nil {
		return DatasetGoldPublishRun{}, fmt.Errorf("get gold source work table: %w", err)
	}
	if workTable.Status != "active" || workTable.DroppedAt.Valid {
		return DatasetGoldPublishRun{}, ErrDatasetWorkTableNotFound
	}
	if err := s.ensureTenantSandbox(ctx, tenantID); err != nil {
		return DatasetGoldPublishRun{}, err
	}
	columns, err := s.listWorkTableColumns(ctx, workTable.WorkDatabase, workTable.WorkTable)
	if err != nil {
		return DatasetGoldPublishRun{}, err
	}
	run, err := s.createGoldPublishRun(ctx, tenantID, userID, publicationRow, workTable, medallionWorkTableSchemaSummary(columns), auditCtx)
	if err != nil {
		return DatasetGoldPublishRun{}, err
	}
	s.recordMedallionGoldPublish(ctx, tenantID, workTable, publicationRow, run, MedallionPipelineStatusPending, "", true)
	s.publishGoldPublishRunUpdated(ctx, tenantID, run, "pending", "")
	return datasetGoldPublishRunFromDB(run), nil
}

func (s *DatasetService) createGoldPublishRun(ctx context.Context, tenantID, userID int64, publication db.DatasetGoldPublication, workTable db.DatasetWorkTable, schemaSummary map[string]any, auditCtx AuditContext) (db.DatasetGoldPublishRun, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.DatasetGoldPublishRun{}, fmt.Errorf("begin gold publish run transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	sourceRun := s.goldSourceDataPipelineRunForWorkTable(ctx, tenantID, workTable.ID)
	run, err := qtx.CreateDatasetGoldPublishRun(ctx, db.CreateDatasetGoldPublishRunParams{
		TenantID:                      tenantID,
		PublicationID:                 publication.ID,
		SourceWorkTableID:             workTable.ID,
		SourceDataPipelineRunID:       pgtype.Int8{Int64: sourceRunID(sourceRun), Valid: sourceRun != nil && sourceRun.RunID > 0},
		SourceDataPipelineRunOutputID: pgtype.Int8{Int64: sourceRunOutputID(sourceRun), Valid: sourceRun != nil && sourceRun.RunOutputID > 0},
		RequestedByUserID:             pgtype.Int8{Int64: userID, Valid: userID > 0},
		GoldDatabase:                  publication.GoldDatabase,
		GoldTable:                     publication.GoldTable,
		InternalDatabase:              datasetGoldInternalDatabaseName(tenantID),
		InternalTable:                 "gp_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		SchemaSummary:                 jsonBytes(schemaSummary),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.DatasetGoldPublishRun{}, ErrDatasetGoldPublishAlreadyActive
		}
		return db.DatasetGoldPublishRun{}, fmt.Errorf("create gold publish run: %w", err)
	}
	event, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "dataset_gold_publish",
		AggregateID:   run.PublicID.String(),
		EventType:     "dataset.gold_publish_requested",
		Payload: map[string]any{
			"tenantId":     tenantID,
			"publishRunId": run.ID,
		},
	})
	if err != nil {
		return db.DatasetGoldPublishRun{}, fmt.Errorf("enqueue gold publish: %w", err)
	}
	run, err = qtx.LinkDatasetGoldPublishRunOutboxEvent(ctx, db.LinkDatasetGoldPublishRunOutboxEventParams{
		OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true},
		ID:            run.ID,
		TenantID:      tenantID,
	})
	if err != nil {
		return db.DatasetGoldPublishRun{}, fmt.Errorf("link gold publish outbox event: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_gold_publication.refresh",
			TargetType:   "dataset_gold_publication",
			TargetID:     publication.PublicID.String(),
			Metadata: map[string]any{
				"publishRun": run.PublicID.String(),
			},
		}); err != nil {
			return db.DatasetGoldPublishRun{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return db.DatasetGoldPublishRun{}, fmt.Errorf("commit gold publish run transaction: %w", err)
	}
	return run, nil
}

func (s *DatasetService) HandleGoldPublishRequested(ctx context.Context, tenantID, publishRunID, outboxEventID int64) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("dataset service is not configured")
	}
	run, err := s.queries.GetDatasetGoldPublishRunByIDForTenant(ctx, db.GetDatasetGoldPublishRunByIDForTenantParams{ID: publishRunID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDatasetGoldPublishRunNotFound
	}
	if err != nil {
		return fmt.Errorf("get gold publish run: %w", err)
	}
	if run.Status == "completed" || run.Status == "failed" {
		return nil
	}
	run, err = s.queries.MarkDatasetGoldPublishRunProcessing(ctx, db.MarkDatasetGoldPublishRunProcessingParams{
		OutboxEventID: pgtype.Int8{Int64: outboxEventID, Valid: outboxEventID > 0},
		ID:            run.ID,
		TenantID:      tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("mark gold publish run processing: %w", err)
	}
	s.publishGoldPublishRunUpdated(ctx, tenantID, run, "processing", "")

	publication, err := s.queries.GetDatasetGoldPublicationByIDForTenant(ctx, db.GetDatasetGoldPublicationByIDForTenantParams{ID: run.PublicationID, TenantID: tenantID})
	if err != nil {
		s.failGoldPublishRun(ctx, tenantID, run, fmt.Sprintf("get gold publication: %v", err), true)
		return nil
	}
	workTable, err := s.queries.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: run.SourceWorkTableID, TenantID: tenantID})
	if err != nil {
		s.failGoldPublishRun(ctx, tenantID, run, fmt.Sprintf("get gold source work table: %v", err), true)
		return nil
	}
	s.recordMedallionGoldPublish(ctx, tenantID, workTable, publication, run, MedallionPipelineStatusProcessing, "", true)
	if publication.ArchivedAt.Valid || publication.Status == "archived" || publication.Status == "unpublished" {
		s.failGoldPublishRun(ctx, tenantID, run, "gold publication is not publishable", true)
		return nil
	}
	if workTable.Status != "active" || workTable.DroppedAt.Valid {
		s.failGoldPublishRun(ctx, tenantID, run, ErrDatasetWorkTableNotFound.Error(), true)
		return nil
	}
	if err := s.ensureTenantSandbox(ctx, tenantID); err != nil {
		s.failGoldPublishRun(ctx, tenantID, run, err.Error(), true)
		return nil
	}
	if err := s.copyWorkTableToGoldInternal(ctx, tenantID, workTable, run.InternalDatabase, run.InternalTable); err != nil {
		s.dropGoldInternalTableBestEffort(ctx, run.InternalDatabase, run.InternalTable)
		s.failGoldPublishRun(ctx, tenantID, run, err.Error(), true)
		return nil
	}
	metadata, err := s.getWorkTableMetadata(ctx, run.InternalDatabase, run.InternalTable)
	if err != nil {
		s.dropGoldInternalTableBestEffort(ctx, run.InternalDatabase, run.InternalTable)
		s.failGoldPublishRun(ctx, tenantID, run, err.Error(), true)
		return nil
	}
	columns, err := s.listWorkTableColumns(ctx, run.InternalDatabase, run.InternalTable)
	if err != nil {
		s.dropGoldInternalTableBestEffort(ctx, run.InternalDatabase, run.InternalTable)
		s.failGoldPublishRun(ctx, tenantID, run, err.Error(), true)
		return nil
	}
	schemaSummary := medallionWorkTableSchemaSummary(columns)
	if err := s.replaceGoldView(ctx, run.GoldDatabase, run.GoldTable, run.InternalDatabase, run.InternalTable); err != nil {
		s.dropGoldInternalTableBestEffort(ctx, run.InternalDatabase, run.InternalTable)
		s.failGoldPublishRun(ctx, tenantID, run, err.Error(), true)
		return nil
	}
	completed, err := s.queries.CompleteDatasetGoldPublishRun(ctx, db.CompleteDatasetGoldPublishRunParams{
		RowCount:      metadata.TotalRows,
		TotalBytes:    metadata.TotalBytes,
		SchemaSummary: jsonBytes(schemaSummary),
		ID:            run.ID,
		TenantID:      tenantID,
	})
	if err != nil {
		return fmt.Errorf("complete gold publish run: %w", err)
	}
	active, err := s.queries.MarkDatasetGoldPublicationActive(ctx, db.MarkDatasetGoldPublicationActiveParams{
		RowCount:          metadata.TotalRows,
		TotalBytes:        metadata.TotalBytes,
		SchemaSummary:     jsonBytes(schemaSummary),
		LastPublishRunID:  pgtype.Int8{Int64: completed.ID, Valid: true},
		PublishedByUserID: completed.RequestedByUserID,
		UpdatedByUserID:   completed.RequestedByUserID,
		ID:                publication.ID,
		TenantID:          tenantID,
	})
	if err != nil {
		return fmt.Errorf("mark gold publication active: %w", err)
	}
	s.recordMedallionGoldPublish(ctx, tenantID, workTable, active, completed, MedallionPipelineStatusCompleted, "", false)
	s.requestGoldLocalSearchIndex(ctx, tenantID, active, "gold_publish_completed")
	s.publishGoldPublishRunUpdated(ctx, tenantID, completed, "completed", "")
	s.publishGoldPublicationUpdated(ctx, tenantID, active, "active", "")
	return nil
}

func (s *DatasetService) ListGoldPublications(ctx context.Context, tenantID int64, limit int32) ([]DatasetGoldPublication, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("dataset service is not configured")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.queries.ListDatasetGoldPublications(ctx, db.ListDatasetGoldPublicationsParams{
		TenantID:        tenantID,
		IncludeArchived: pgtype.Bool{Bool: false, Valid: true},
		LimitCount:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list gold publications: %w", err)
	}
	items := make([]DatasetGoldPublication, 0, len(rows))
	for _, row := range rows {
		item := datasetGoldPublicationFromDB(row)
		s.hydrateGoldPublication(ctx, tenantID, &item)
		items = append(items, item)
	}
	return items, nil
}

func (s *DatasetService) ListGoldPublicationsForWorkTable(ctx context.Context, tenantID int64, workTablePublicID string, limit int32) ([]DatasetGoldPublication, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("dataset service is not configured")
	}
	workTable, err := s.getManagedWorkTableRow(ctx, tenantID, workTablePublicID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.queries.ListDatasetGoldPublicationsForWorkTable(ctx, db.ListDatasetGoldPublicationsForWorkTableParams{
		TenantID:          tenantID,
		SourceWorkTableID: workTable.ID,
		LimitCount:        limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list work table gold publications: %w", err)
	}
	items := make([]DatasetGoldPublication, 0, len(rows))
	for _, row := range rows {
		item := datasetGoldPublicationFromDB(row)
		s.hydrateGoldPublication(ctx, tenantID, &item)
		items = append(items, item)
	}
	return items, nil
}

func (s *DatasetService) GetGoldPublication(ctx context.Context, tenantID int64, publicID string) (DatasetGoldPublication, error) {
	row, err := s.getGoldPublicationRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetGoldPublication{}, err
	}
	item := datasetGoldPublicationFromDB(row)
	s.hydrateGoldPublication(ctx, tenantID, &item)
	s.hydrateGoldPublicationSCD2Summary(ctx, tenantID, &item)
	s.hydrateGoldPublicationDataPipelineSource(ctx, tenantID, &item)
	return item, nil
}

func (s *DatasetService) PreviewGoldPublication(ctx context.Context, tenantID int64, publicID string, limit int32) (DatasetGoldPublicationPreview, error) {
	row, err := s.getGoldPublicationRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetGoldPublicationPreview{}, err
	}
	if row.Status != "active" || row.ArchivedAt.Valid {
		return DatasetGoldPublicationPreview{}, ErrDatasetGoldPublicationNotFound
	}
	if err := s.ensureTenantSandbox(ctx, tenantID); err != nil {
		return DatasetGoldPublicationPreview{}, err
	}
	if limit <= 0 || limit > datasetPreviewRowLimit {
		limit = datasetPreviewRowLimit
	}
	rows, err := s.clickhouse.Query(
		clickhouse.Context(ctx, clickhouse.WithSettings(s.querySettings())),
		fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d", quoteCHIdent(row.GoldDatabase), quoteCHIdent(row.GoldTable), limit),
	)
	if err != nil {
		return DatasetGoldPublicationPreview{}, fmt.Errorf("preview gold publication: %w", err)
	}
	defer rows.Close()
	columns, previewRows, err := scanDatasetRows(rows, int(limit))
	if err != nil {
		return DatasetGoldPublicationPreview{}, fmt.Errorf("scan gold publication preview: %w", err)
	}
	return DatasetGoldPublicationPreview{Database: row.GoldDatabase, Table: row.GoldTable, Columns: columns, PreviewRows: previewRows}, nil
}

func (s *DatasetService) ListGoldPublishRuns(ctx context.Context, tenantID int64, publicID string, limit int32) ([]DatasetGoldPublishRun, error) {
	publication, err := s.getGoldPublicationRow(ctx, tenantID, publicID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.queries.ListDatasetGoldPublishRuns(ctx, db.ListDatasetGoldPublishRunsParams{
		TenantID:      tenantID,
		PublicationID: publication.ID,
		LimitCount:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list gold publish runs: %w", err)
	}
	items := make([]DatasetGoldPublishRun, 0, len(rows))
	for _, row := range rows {
		item := datasetGoldPublishRunFromDB(row)
		item.PublicationPublicID = publication.PublicID.String()
		s.hydrateGoldPublishRun(ctx, tenantID, &item)
		s.hydrateGoldPublishRunDataPipelineSource(ctx, tenantID, &item)
		items = append(items, item)
	}
	return items, nil
}

func (s *DatasetService) UnpublishGoldPublication(ctx context.Context, tenantID, userID int64, publicID string, auditCtx AuditContext) (DatasetGoldPublication, error) {
	row, err := s.getGoldPublicationRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetGoldPublication{}, err
	}
	if row.Status == "active" {
		if err := s.dropGoldView(ctx, row.GoldDatabase, row.GoldTable); err != nil {
			return DatasetGoldPublication{}, err
		}
	}
	updated, err := s.queries.UnpublishDatasetGoldPublication(ctx, db.UnpublishDatasetGoldPublicationParams{
		UnpublishedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		UpdatedByUserID:     pgtype.Int8{Int64: userID, Valid: userID > 0},
		PublicID:            row.PublicID,
		TenantID:            tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetGoldPublication{}, ErrDatasetGoldPublicationNotFound
	}
	if err != nil {
		return DatasetGoldPublication{}, fmt.Errorf("unpublish gold publication: %w", err)
	}
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_gold_publication.unpublish",
			TargetType:   "dataset_gold_publication",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"goldTable": row.GoldTable,
			},
		})
	}
	item := datasetGoldPublicationFromDB(updated)
	s.hydrateGoldPublication(ctx, tenantID, &item)
	s.recordMedallionGoldPublicationStatus(ctx, tenantID, updated, userID)
	s.requestGoldLocalSearchIndex(ctx, tenantID, updated, "gold_unpublished")
	s.publishGoldPublicationUpdated(ctx, tenantID, updated, "unpublished", "")
	return item, nil
}

func (s *DatasetService) ArchiveGoldPublication(ctx context.Context, tenantID, userID int64, publicID string, auditCtx AuditContext) (DatasetGoldPublication, error) {
	row, err := s.getGoldPublicationRow(ctx, tenantID, publicID)
	if err != nil {
		return DatasetGoldPublication{}, err
	}
	if row.Status == "active" {
		if err := s.dropGoldView(ctx, row.GoldDatabase, row.GoldTable); err != nil {
			return DatasetGoldPublication{}, err
		}
	}
	updated, err := s.queries.ArchiveDatasetGoldPublication(ctx, db.ArchiveDatasetGoldPublicationParams{
		ArchivedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		UpdatedByUserID:  pgtype.Int8{Int64: userID, Valid: userID > 0},
		PublicID:         row.PublicID,
		TenantID:         tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetGoldPublication{}, ErrDatasetGoldPublicationNotFound
	}
	if err != nil {
		return DatasetGoldPublication{}, fmt.Errorf("archive gold publication: %w", err)
	}
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "dataset_gold_publication.archive",
			TargetType:   "dataset_gold_publication",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"goldTable": row.GoldTable,
			},
		})
	}
	item := datasetGoldPublicationFromDB(updated)
	s.hydrateGoldPublication(ctx, tenantID, &item)
	s.recordMedallionGoldPublicationStatus(ctx, tenantID, updated, userID)
	s.requestGoldLocalSearchIndex(ctx, tenantID, updated, "gold_archived")
	s.publishGoldPublicationUpdated(ctx, tenantID, updated, "archived", "")
	return item, nil
}

func (s *DatasetService) getGoldPublicationRow(ctx context.Context, tenantID int64, publicID string) (db.DatasetGoldPublication, error) {
	if s == nil || s.queries == nil {
		return db.DatasetGoldPublication{}, fmt.Errorf("dataset service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return db.DatasetGoldPublication{}, ErrDatasetGoldPublicationNotFound
	}
	row, err := s.queries.GetDatasetGoldPublicationForTenant(ctx, db.GetDatasetGoldPublicationForTenantParams{PublicID: parsed, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DatasetGoldPublication{}, ErrDatasetGoldPublicationNotFound
	}
	if err != nil {
		return db.DatasetGoldPublication{}, fmt.Errorf("get gold publication: %w", err)
	}
	if row.GoldDatabase != datasetGoldDatabaseName(tenantID) {
		return db.DatasetGoldPublication{}, ErrDatasetGoldPublicationNotFound
	}
	return row, nil
}

func (s *DatasetService) copyWorkTableToGoldInternal(ctx context.Context, tenantID int64, workTable db.DatasetWorkTable, internalDatabase, internalTable string) error {
	if err := s.ensureTenantSandbox(ctx, tenantID); err != nil {
		return err
	}
	if internalDatabase != datasetGoldInternalDatabaseName(tenantID) {
		return ErrInvalidDatasetInput
	}
	if strings.TrimSpace(internalTable) == "" || hasDatasetIdentifierControlRune(internalTable) {
		return ErrInvalidDatasetInput
	}
	columns, err := s.listWorkTableColumns(ctx, workTable.WorkDatabase, workTable.WorkTable)
	if err != nil {
		return err
	}
	if len(columns) == 0 {
		return fmt.Errorf("%w: work table has no columns", ErrInvalidDatasetInput)
	}
	target := fmt.Sprintf("%s.%s", quoteCHIdent(internalDatabase), quoteCHIdent(internalTable))
	source := fmt.Sprintf("%s.%s", quoteCHIdent(workTable.WorkDatabase), quoteCHIdent(workTable.WorkTable))
	if err := s.clickhouse.Exec(ctx, "DROP TABLE IF EXISTS "+target); err != nil {
		return fmt.Errorf("drop gold internal table: %w", err)
	}
	statement := fmt.Sprintf("CREATE TABLE %s ENGINE = MergeTree ORDER BY tuple() AS SELECT * FROM %s", target, source)
	if err := s.clickhouse.Exec(ctx, statement); err != nil {
		return fmt.Errorf("copy work table to gold internal table: %w", err)
	}
	return nil
}

func (s *DatasetService) replaceGoldView(ctx context.Context, goldDatabase, goldTable, internalDatabase, internalTable string) error {
	statement := fmt.Sprintf(
		"CREATE OR REPLACE VIEW %s.%s SQL SECURITY DEFINER AS SELECT * FROM %s.%s",
		quoteCHIdent(goldDatabase),
		quoteCHIdent(goldTable),
		quoteCHIdent(internalDatabase),
		quoteCHIdent(internalTable),
	)
	if err := s.clickhouse.Exec(ctx, statement); err != nil {
		return fmt.Errorf("replace gold view: %w", err)
	}
	return nil
}

func (s *DatasetService) dropGoldView(ctx context.Context, goldDatabase, goldTable string) error {
	if s == nil || s.clickhouse == nil {
		return nil
	}
	if err := s.clickhouse.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(goldDatabase), quoteCHIdent(goldTable))); err != nil {
		return fmt.Errorf("drop gold view: %w", err)
	}
	return nil
}

func (s *DatasetService) dropGoldInternalTableBestEffort(ctx context.Context, internalDatabase, internalTable string) {
	if s == nil || s.clickhouse == nil || strings.TrimSpace(internalDatabase) == "" || strings.TrimSpace(internalTable) == "" {
		return
	}
	_ = s.clickhouse.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteCHIdent(internalDatabase), quoteCHIdent(internalTable)))
}

func (s *DatasetService) failGoldPublishRun(ctx context.Context, tenantID int64, run db.DatasetGoldPublishRun, message string, retryable bool) {
	if message == "" {
		message = "gold publish failed"
	}
	updated, err := s.queries.FailDatasetGoldPublishRun(ctx, db.FailDatasetGoldPublishRunParams{
		ErrorSummary: message,
		ID:           run.ID,
		TenantID:     tenantID,
	})
	if err == nil {
		run = updated
	}
	publication, pubErr := s.queries.MarkDatasetGoldPublicationPublishFailed(ctx, db.MarkDatasetGoldPublicationPublishFailedParams{
		LastPublishRunID: pgtype.Int8{Int64: run.ID, Valid: true},
		UpdatedByUserID:  run.RequestedByUserID,
		ID:               run.PublicationID,
		TenantID:         tenantID,
	})
	if pubErr == nil {
		if workTable, err := s.queries.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: run.SourceWorkTableID, TenantID: tenantID}); err == nil {
			s.recordMedallionGoldPublish(ctx, tenantID, workTable, publication, run, MedallionPipelineStatusFailed, "gold publish failed", retryable)
		}
	}
	s.publishGoldPublishRunUpdated(ctx, tenantID, run, "failed", message)
}

func (s *DatasetService) recordMedallionGoldPublish(ctx context.Context, tenantID int64, workTableRow db.DatasetWorkTable, publicationRow db.DatasetGoldPublication, runRow db.DatasetGoldPublishRun, status, errorSummary string, retryable bool) {
	if s == nil || s.medallion == nil {
		return
	}
	actorID := optionalPgInt8(runRow.RequestedByUserID)
	table := datasetWorkTableFromDB(workTableRow)
	source, sourceErr := s.medallion.EnsureWorkTableAsset(ctx, table, actorID)
	publication := datasetGoldPublicationFromDB(publicationRow)
	publication.SourceWorkTablePublicID = workTableRow.PublicID.String()
	publication.SourceWorkTableName = workTableRow.DisplayName
	publication.SourceWorkTableDatabase = workTableRow.WorkDatabase
	publication.SourceWorkTableTable = workTableRow.WorkTable
	target, targetErr := s.medallion.EnsureGoldTableAsset(ctx, publication, actorID)
	if sourceErr == nil && targetErr == nil {
		s.medallion.LinkAssets(ctx, tenantID, source, target, "published_gold", map[string]any{
			"publishRunPublicId": runRow.PublicID.String(),
		})
	}
	startedAt := optionalPgTime(runRow.StartedAt)
	completedAt := optionalPgTime(runRow.CompletedAt)
	if completedAt == nil && (status == MedallionPipelineStatusCompleted || status == MedallionPipelineStatusFailed || status == MedallionPipelineStatusSkipped) {
		completedAt = ptrTime(time.Now())
	}
	if startedAt == nil && status != MedallionPipelineStatusPending {
		startedAt = ptrTime(time.Now())
	}
	input := medallionPipelineRunInput{
		TenantID:               tenantID,
		PipelineType:           MedallionPipelineGoldPublish,
		RunKey:                 "gold_publish:" + runRow.PublicID.String(),
		SourceResourceKind:     MedallionResourceWorkTable,
		SourceResourceID:       workTableRow.ID,
		SourceResourcePublicID: workTableRow.PublicID.String(),
		TargetResourceKind:     MedallionResourceGoldTable,
		TargetResourceID:       publicationRow.ID,
		TargetResourcePublicID: publicationRow.PublicID.String(),
		Status:                 status,
		Runtime:                "clickhouse",
		TriggerKind:            MedallionTriggerManual,
		Retryable:              retryable,
		ErrorSummary:           errorSummary,
		Metadata: map[string]any{
			"goldDatabase": publicationRow.GoldDatabase,
			"goldTable":    publicationRow.GoldTable,
		},
		RequestedByUserID: actorID,
		StartedAt:         startedAt,
		CompletedAt:       completedAt,
	}
	if sourceErr == nil {
		input.SourceAssets = []MedallionAsset{source}
	}
	if targetErr == nil {
		input.TargetAssets = []MedallionAsset{target}
	}
	_, _ = s.medallion.RecordPipelineRun(ctx, input)
}

func (s *DatasetService) recordMedallionGoldPublicationStatus(ctx context.Context, tenantID int64, publicationRow db.DatasetGoldPublication, userID int64) {
	if s == nil || s.medallion == nil {
		return
	}
	publication := datasetGoldPublicationFromDB(publicationRow)
	s.hydrateGoldPublication(ctx, tenantID, &publication)
	var actorID *int64
	if userID > 0 {
		actorID = &userID
	}
	_, _ = s.medallion.EnsureGoldTableAsset(ctx, publication, actorID)
}

func (s *DatasetService) requestGoldLocalSearchIndex(ctx context.Context, tenantID int64, publicationRow db.DatasetGoldPublication, reason string) {
	if s == nil || s.localSearch == nil {
		return
	}
	s.localSearch.RequestIndexBestEffort(ctx, tenantID, LocalSearchResourceGoldTable, publicationRow.ID, publicationRow.PublicID.String(), reason)
}

func (s *DatasetService) hydrateGoldPublication(ctx context.Context, tenantID int64, item *DatasetGoldPublication) {
	if s == nil || s.queries == nil || item == nil {
		return
	}
	if workTable, err := s.queries.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: item.SourceWorkTableID, TenantID: tenantID}); err == nil {
		item.SourceWorkTablePublicID = workTable.PublicID.String()
		item.SourceWorkTableName = workTable.DisplayName
		item.SourceWorkTableDatabase = workTable.WorkDatabase
		item.SourceWorkTableTable = workTable.WorkTable
	}
	if item.LastPublishRunID != nil {
		if row, err := s.queries.GetDatasetGoldPublishRunByIDForTenant(ctx, db.GetDatasetGoldPublishRunByIDForTenantParams{ID: *item.LastPublishRunID, TenantID: tenantID}); err == nil {
			run := datasetGoldPublishRunFromDB(row)
			run.PublicationPublicID = item.PublicID
			s.hydrateGoldPublishRun(ctx, tenantID, &run)
			s.hydrateGoldPublishRunDataPipelineSource(ctx, tenantID, &run)
			item.LatestPublishRun = &run
		}
	}
}

func (s *DatasetService) hydrateGoldPublicationSCD2Summary(ctx context.Context, tenantID int64, item *DatasetGoldPublication) {
	if s == nil || s.queries == nil || item == nil || item.SourceWorkTableID <= 0 {
		return
	}
	workTable, err := s.queries.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: item.SourceWorkTableID, TenantID: tenantID})
	if err != nil || workTable.Status != "active" || workTable.DroppedAt.Valid {
		return
	}
	summary, err := s.summarizeManagedWorkTableSCD2(ctx, tenantID, workTable)
	if err == nil {
		item.SourceSCD2Summary = summary
	}
}

func (s *DatasetService) hydrateGoldPublicationDataPipelineSource(ctx context.Context, tenantID int64, item *DatasetGoldPublication) {
	if item == nil || item.SourceWorkTableID <= 0 {
		return
	}
	item.SourceDataPipelineRun = s.goldSourceDataPipelineRunForWorkTable(ctx, tenantID, item.SourceWorkTableID)
}

func (s *DatasetService) hydrateGoldPublishRunDataPipelineSource(ctx context.Context, tenantID int64, item *DatasetGoldPublishRun) {
	if item == nil || item.SourceWorkTableID <= 0 {
		return
	}
	if item.SourceDataPipelineRunOutputID != nil {
		item.SourceDataPipelineRun = s.goldSourceDataPipelineRunForOutput(ctx, tenantID, *item.SourceDataPipelineRunOutputID)
		if item.SourceDataPipelineRun != nil {
			return
		}
	}
	item.SourceDataPipelineRun = s.goldSourceDataPipelineRunForWorkTable(ctx, tenantID, item.SourceWorkTableID)
}

func (s *DatasetService) goldSourceDataPipelineRunForWorkTable(ctx context.Context, tenantID, workTableID int64) *DatasetGoldSourceDataPipelineRun {
	if s == nil || s.pool == nil || workTableID <= 0 {
		return nil
	}
	const query = `
SELECT
	r.id,
	o.id,
	p.public_id::text,
	p.name,
	r.public_id::text,
	r.status,
	o.node_id,
	o.row_count,
	o.metadata,
	o.completed_at
FROM data_pipeline_run_outputs o
JOIN data_pipeline_runs r ON r.id = o.run_id AND r.tenant_id = o.tenant_id
JOIN data_pipelines p ON p.id = r.pipeline_id AND p.tenant_id = o.tenant_id
WHERE o.tenant_id = $1
  AND o.output_work_table_id = $2
  AND o.status = 'completed'
ORDER BY o.completed_at DESC NULLS LAST, o.id DESC
LIMIT 1`
	return s.scanGoldSourceDataPipelineRun(ctx, query, tenantID, workTableID)
}

func (s *DatasetService) goldSourceDataPipelineRunForOutput(ctx context.Context, tenantID, outputID int64) *DatasetGoldSourceDataPipelineRun {
	if s == nil || s.pool == nil || outputID <= 0 {
		return nil
	}
	const query = `
SELECT
	r.id,
	o.id,
	p.public_id::text,
	p.name,
	r.public_id::text,
	r.status,
	o.node_id,
	o.row_count,
	o.metadata,
	o.completed_at
FROM data_pipeline_run_outputs o
JOIN data_pipeline_runs r ON r.id = o.run_id AND r.tenant_id = o.tenant_id
JOIN data_pipelines p ON p.id = r.pipeline_id AND p.tenant_id = o.tenant_id
WHERE o.tenant_id = $1
  AND o.id = $2
LIMIT 1`
	return s.scanGoldSourceDataPipelineRun(ctx, query, tenantID, outputID)
}

func (s *DatasetService) scanGoldSourceDataPipelineRun(ctx context.Context, query string, tenantID, id int64) *DatasetGoldSourceDataPipelineRun {
	var source DatasetGoldSourceDataPipelineRun
	var completedAt pgtype.Timestamptz
	var metadataBytes []byte
	err := s.pool.QueryRow(ctx, query, tenantID, id).Scan(
		&source.RunID,
		&source.RunOutputID,
		&source.PipelinePublicID,
		&source.PipelineName,
		&source.RunPublicID,
		&source.RunStatus,
		&source.OutputNodeID,
		&source.OutputRowCount,
		&metadataBytes,
		&completedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return nil
	}
	metadata := jsonObjectFromBytes(metadataBytes)
	source.OutputWriteMode = dataPipelineString(metadata, "writeMode")
	source.SCD2MergePolicy = dataPipelineString(metadata, "scd2MergePolicy")
	source.SCD2UniqueKeys = dataPipelineStringSlice(metadata, "scd2UniqueKeys")
	source.QualitySummary = s.summarizeGoldPublicationSourceQuality(ctx, tenantID, source.RunID)
	source.CompletedAt = optionalPgTime(completedAt)
	return &source
}

func (s *DatasetService) summarizeGoldPublicationSourceQuality(ctx context.Context, tenantID, runID int64) *DatasetGoldSourceQualitySummary {
	if s == nil || s.queries == nil || runID <= 0 {
		return nil
	}
	rows, err := s.queries.ListDataPipelineRunSteps(ctx, db.ListDataPipelineRunStepsParams{TenantID: tenantID, RunID: runID})
	if err != nil || len(rows) == 0 {
		return nil
	}
	summary := &DatasetGoldSourceQualitySummary{StepCount: int64(len(rows))}
	for _, row := range rows {
		metadata := decodeDataPipelineJSONMap(row.Metadata)
		summary.WarningCount += metadataInt64(metadata, "warningCount")
		summary.FailedRows += metadataInt64(metadata, "failedRows")
		summary.ReviewItemCount += metadataInt64(metadata, "reviewItemCount")
		summary.QuarantinedRows += metadataInt64(metadata, "quarantinedRows")
		if quality := metadataMap(metadata, "quality"); quality != nil {
			summary.QualityRows = maxInt64(summary.QualityRows, metadataInt64(quality, "rowCount"))
			summary.QualityColumns = maxInt64(summary.QualityColumns, metadataInt64(quality, "columnCount"))
		}
		if validation := metadataMap(metadata, "validation"); validation != nil {
			summary.ValidationErrors += metadataInt64(validation, "errorCount")
			summary.ValidationWarnings += metadataInt64(validation, "warningCount")
		}
		if confidenceGate := metadataMap(metadata, "confidenceGate"); confidenceGate != nil {
			summary.ConfidencePassRows += metadataInt64(confidenceGate, "passRows")
			summary.ConfidenceNeedsReviewRows += metadataInt64(confidenceGate, "needsReviewRows")
		}
	}
	return summary
}

func metadataMap(metadata map[string]any, key string) map[string]any {
	value, ok := metadata[key]
	if !ok || value == nil {
		return nil
	}
	out, _ := value.(map[string]any)
	return out
}

func metadataInt64(metadata map[string]any, key string) int64 {
	value, ok := metadata[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	default:
		return 0
	}
}

func maxInt64(left, right int64) int64 {
	if right > left {
		return right
	}
	return left
}

func (s *DatasetService) summarizeManagedWorkTableSCD2(ctx context.Context, tenantID int64, workTable db.DatasetWorkTable) (*DatasetWorkTableSCD2Summary, error) {
	columns, err := s.listWorkTableColumns(ctx, workTable.WorkDatabase, workTable.WorkTable)
	if err != nil {
		return nil, err
	}
	columnNames := make([]string, 0, len(columns))
	for _, column := range columns {
		columnNames = append(columnNames, column.ColumnName)
	}
	if !datasetWorkTableHasSCD2Columns(columnNames) {
		return nil, nil
	}
	keyColumnHints := s.managedWorkTableSCD2KeyColumns(ctx, tenantID, workTable.ID, columnNames)
	conn, err := s.openTenantConn(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return s.summarizeSCD2WorkTable(ctx, conn, workTable.WorkDatabase, workTable.WorkTable, columnNames, keyColumnHints)
}

func (s *DatasetService) hydrateGoldPublishRun(ctx context.Context, tenantID int64, item *DatasetGoldPublishRun) {
	if s == nil || s.queries == nil || item == nil {
		return
	}
	if workTable, err := s.queries.GetDatasetWorkTableByIDForTenant(ctx, db.GetDatasetWorkTableByIDForTenantParams{ID: item.SourceWorkTableID, TenantID: tenantID}); err == nil {
		item.SourceWorkTablePublicID = workTable.PublicID.String()
	}
	if item.PublicationPublicID == "" {
		if publication, err := s.queries.GetDatasetGoldPublicationByIDForTenant(ctx, db.GetDatasetGoldPublicationByIDForTenantParams{ID: item.PublicationID, TenantID: tenantID}); err == nil {
			item.PublicationPublicID = publication.PublicID.String()
		}
	}
}

func datasetGoldPublicationFromDB(row db.DatasetGoldPublication) DatasetGoldPublication {
	return DatasetGoldPublication{
		ID:                  row.ID,
		PublicID:            row.PublicID.String(),
		TenantID:            row.TenantID,
		SourceWorkTableID:   row.SourceWorkTableID,
		CreatedByUserID:     optionalPgInt8(row.CreatedByUserID),
		UpdatedByUserID:     optionalPgInt8(row.UpdatedByUserID),
		PublishedByUserID:   optionalPgInt8(row.PublishedByUserID),
		UnpublishedByUserID: optionalPgInt8(row.UnpublishedByUserID),
		ArchivedByUserID:    optionalPgInt8(row.ArchivedByUserID),
		LastPublishRunID:    optionalPgInt8(row.LastPublishRunID),
		DisplayName:         row.DisplayName,
		Description:         row.Description,
		GoldDatabase:        row.GoldDatabase,
		GoldTable:           row.GoldTable,
		Status:              row.Status,
		RowCount:            row.RowCount,
		TotalBytes:          row.TotalBytes,
		SchemaSummary:       jsonObjectFromBytes(row.SchemaSummary),
		RefreshPolicy:       row.RefreshPolicy,
		CreatedAt:           row.CreatedAt.Time,
		UpdatedAt:           row.UpdatedAt.Time,
		PublishedAt:         optionalPgTime(row.PublishedAt),
		UnpublishedAt:       optionalPgTime(row.UnpublishedAt),
		ArchivedAt:          optionalPgTime(row.ArchivedAt),
	}
}

func datasetGoldPublishRunFromDB(row db.DatasetGoldPublishRun) DatasetGoldPublishRun {
	return DatasetGoldPublishRun{
		ID:                            row.ID,
		PublicID:                      row.PublicID.String(),
		TenantID:                      row.TenantID,
		PublicationID:                 row.PublicationID,
		SourceWorkTableID:             row.SourceWorkTableID,
		SourceDataPipelineRunID:       optionalPgInt8(row.SourceDataPipelineRunID),
		SourceDataPipelineRunOutputID: optionalPgInt8(row.SourceDataPipelineRunOutputID),
		RequestedByUserID:             optionalPgInt8(row.RequestedByUserID),
		OutboxEventID:                 optionalPgInt8(row.OutboxEventID),
		Status:                        row.Status,
		GoldDatabase:                  row.GoldDatabase,
		GoldTable:                     row.GoldTable,
		InternalDatabase:              row.InternalDatabase,
		InternalTable:                 row.InternalTable,
		RowCount:                      row.RowCount,
		TotalBytes:                    row.TotalBytes,
		SchemaSummary:                 jsonObjectFromBytes(row.SchemaSummary),
		ErrorSummary:                  optionalText(row.ErrorSummary),
		StartedAt:                     optionalPgTime(row.StartedAt),
		CompletedAt:                   optionalPgTime(row.CompletedAt),
		CreatedAt:                     row.CreatedAt.Time,
		UpdatedAt:                     row.UpdatedAt.Time,
	}
}

func ptrDatasetGoldPublishRun(item DatasetGoldPublishRun) *DatasetGoldPublishRun {
	return &item
}

func sourceRunID(item *DatasetGoldSourceDataPipelineRun) int64 {
	if item == nil {
		return 0
	}
	return item.RunID
}

func sourceRunOutputID(item *DatasetGoldSourceDataPipelineRun) int64 {
	if item == nil {
		return 0
	}
	return item.RunOutputID
}

func normalizeDatasetGoldDisplayName(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len(value) > 160 {
			return value[:160]
		}
		return value
	}
	return "Gold data mart"
}

func normalizeDatasetGoldTableName(input, fallback string, generated bool) string {
	name := strings.TrimSpace(input)
	if name == "" {
		name = strings.TrimSpace(fallback)
	}
	if name == "" {
		name = "gold_data_mart"
	}
	base := sanitizeDatasetColumnName(name)
	if base == "" {
		base = "gold_data_mart"
	}
	if !strings.HasPrefix(base, "gm_") {
		base = "gm_" + base
	}
	if generated {
		suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
		if len(base) > 54 {
			base = strings.TrimRight(base[:54], "_")
		}
		base = base + "_" + suffix
	}
	if len(base) > 80 {
		base = strings.TrimRight(base[:80], "_")
	}
	return base
}

func (s *DatasetService) publishGoldPublishRunUpdated(ctx context.Context, tenantID int64, run db.DatasetGoldPublishRun, status, errorSummary string) {
	if s == nil || s.realtime == nil || !run.RequestedByUserID.Valid {
		return
	}
	payload := map[string]any{
		"status":             status,
		"publishRunPublicId": run.PublicID.String(),
	}
	if publication, err := s.queries.GetDatasetGoldPublicationByIDForTenant(ctx, db.GetDatasetGoldPublicationByIDForTenantParams{ID: run.PublicationID, TenantID: tenantID}); err == nil {
		payload["goldPublicationPublicId"] = publication.PublicID.String()
		payload["goldTable"] = publication.GoldTable
	}
	if errorSummary != "" {
		payload["errorSummary"] = errorSummary
	}
	if run.RowCount > 0 {
		payload["rowCount"] = run.RowCount
	}
	_, _ = s.realtime.Publish(ctx, RealtimeEventInput{
		TenantID:         &tenantID,
		RecipientUserID:  run.RequestedByUserID.Int64,
		EventType:        "job.updated",
		ResourceType:     "dataset_gold_publish",
		ResourcePublicID: run.PublicID.String(),
		Payload:          payload,
	})
}

func (s *DatasetService) publishGoldPublicationUpdated(ctx context.Context, tenantID int64, publication db.DatasetGoldPublication, status, errorSummary string) {
	if s == nil || s.realtime == nil || !publication.CreatedByUserID.Valid {
		return
	}
	payload := map[string]any{
		"status":                  status,
		"goldPublicationPublicId": publication.PublicID.String(),
		"goldTable":               publication.GoldTable,
		"rowCount":                publication.RowCount,
	}
	if errorSummary != "" {
		payload["errorSummary"] = errorSummary
	}
	_, _ = s.realtime.Publish(ctx, RealtimeEventInput{
		TenantID:         &tenantID,
		RecipientUserID:  publication.CreatedByUserID.Int64,
		EventType:        "job.updated",
		ResourceType:     "dataset_gold_publication",
		ResourcePublicID: publication.PublicID.String(),
		Payload:          payload,
	})
}
