package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"example.com/haohao/backend/internal/db"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDataPipelineNotFound           = errors.New("data pipeline not found")
	ErrDataPipelineVersionNotFound    = errors.New("data pipeline version not found")
	ErrDataPipelineRunNotFound        = errors.New("data pipeline run not found")
	ErrDataPipelineScheduleNotFound   = errors.New("data pipeline schedule not found")
	ErrInvalidDataPipelineInput       = errors.New("invalid data pipeline input")
	ErrInvalidDataPipelineGraph       = errors.New("invalid data pipeline graph")
	ErrDataPipelineVersionUnpublished = errors.New("data pipeline version is not published")
)

type DataPipeline struct {
	ID                 int64
	PublicID           string
	TenantID           int64
	CreatedByUserID    *int64
	UpdatedByUserID    *int64
	Name               string
	Description        string
	Status             string
	PublishedVersionID *int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ArchivedAt         *time.Time
}

type DataPipelineVersion struct {
	ID                int64
	PublicID          string
	TenantID          int64
	PipelineID        int64
	VersionNumber     int32
	Status            string
	Graph             DataPipelineGraph
	ValidationSummary DataPipelineValidationSummary
	CreatedByUserID   *int64
	PublishedByUserID *int64
	CreatedAt         time.Time
	PublishedAt       *time.Time
}

type DataPipelineRun struct {
	ID                int64
	PublicID          string
	TenantID          int64
	PipelineID        int64
	VersionID         int64
	ScheduleID        *int64
	RequestedByUserID *int64
	TriggerKind       string
	Status            string
	OutputWorkTableID *int64
	OutboxEventID     *int64
	RowCount          int64
	ErrorSummary      string
	StartedAt         *time.Time
	CompletedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Steps             []DataPipelineRunStep
}

type DataPipelineRunStep struct {
	ID           int64
	TenantID     int64
	RunID        int64
	NodeID       string
	StepType     string
	Status       string
	RowCount     int64
	ErrorSummary string
	ErrorSample  []map[string]any
	Metadata     map[string]any
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type DataPipelineSchedule struct {
	ID               int64
	PublicID         string
	TenantID         int64
	PipelineID       int64
	VersionID        int64
	CreatedByUserID  *int64
	Frequency        string
	Timezone         string
	RunTime          string
	Weekday          *int32
	MonthDay         *int32
	Enabled          bool
	NextRunAt        time.Time
	LastRunAt        *time.Time
	LastStatus       string
	LastErrorSummary string
	LastRunID        *int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type DataPipelineDetail struct {
	Pipeline         DataPipeline
	PublishedVersion *DataPipelineVersion
	Versions         []DataPipelineVersion
	Runs             []DataPipelineRun
	Schedules        []DataPipelineSchedule
}

type DataPipelineInput struct {
	Name        string
	Description string
}

type DataPipelineScheduleInput struct {
	Frequency string
	Timezone  string
	RunTime   string
	Weekday   *int32
	MonthDay  *int32
	Enabled   *bool
}

type DataPipelinePreview struct {
	NodeID      string
	StepType    string
	Columns     []string
	PreviewRows []map[string]any
}

type DataPipelineScheduleRunSummary struct {
	Claimed  int
	Created  int
	Skipped  int
	Failed   int
	Disabled int
}

type DataPipelineService struct {
	pool      *pgxpool.Pool
	queries   *db.Queries
	outbox    *OutboxService
	datasets  *DatasetService
	medallion *MedallionCatalogService
	audit     AuditRecorder
}

func NewDataPipelineService(pool *pgxpool.Pool, queries *db.Queries, outbox *OutboxService, datasets *DatasetService, medallion *MedallionCatalogService, audit AuditRecorder) *DataPipelineService {
	return &DataPipelineService{
		pool:      pool,
		queries:   queries,
		outbox:    outbox,
		datasets:  datasets,
		medallion: medallion,
		audit:     audit,
	}
}

func (s *DataPipelineService) List(ctx context.Context, tenantID int64, limit int32) ([]DataPipeline, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("data pipeline service is not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.queries.ListDataPipelines(ctx, db.ListDataPipelinesParams{TenantID: tenantID, LimitCount: limit})
	if err != nil {
		return nil, fmt.Errorf("list data pipelines: %w", err)
	}
	items := make([]DataPipeline, 0, len(rows))
	for _, row := range rows {
		items = append(items, dataPipelineFromDB(row))
	}
	return items, nil
}

func (s *DataPipelineService) Create(ctx context.Context, tenantID, userID int64, input DataPipelineInput, auditCtx AuditContext) (DataPipeline, error) {
	if s == nil || s.queries == nil {
		return DataPipeline{}, fmt.Errorf("data pipeline service is not configured")
	}
	normalized, err := normalizeDataPipelineInput(input)
	if err != nil {
		return DataPipeline{}, err
	}
	row, err := s.queries.CreateDataPipeline(ctx, db.CreateDataPipelineParams{
		TenantID:        tenantID,
		CreatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		UpdatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		Name:            normalized.Name,
		Description:     normalized.Description,
	})
	if err != nil {
		return DataPipeline{}, fmt.Errorf("create data pipeline: %w", err)
	}
	item := dataPipelineFromDB(row)
	s.recordAudit(ctx, auditCtx, "data_pipeline.create", "data_pipeline", item.PublicID, nil)
	return item, nil
}

func (s *DataPipelineService) Get(ctx context.Context, tenantID int64, publicID string) (DataPipelineDetail, error) {
	if s == nil || s.queries == nil {
		return DataPipelineDetail{}, fmt.Errorf("data pipeline service is not configured")
	}
	row, err := s.getPipelineRow(ctx, tenantID, publicID)
	if err != nil {
		return DataPipelineDetail{}, err
	}
	pipeline := dataPipelineFromDB(row)
	versions, err := s.listVersionsForPipeline(ctx, tenantID, row.ID, 20)
	if err != nil {
		return DataPipelineDetail{}, err
	}
	runs, err := s.ListRuns(ctx, tenantID, publicID, 25)
	if err != nil {
		return DataPipelineDetail{}, err
	}
	schedules, err := s.ListSchedules(ctx, tenantID, publicID)
	if err != nil {
		return DataPipelineDetail{}, err
	}
	var published *DataPipelineVersion
	if row.PublishedVersionID.Valid {
		versionRow, err := s.queries.GetDataPipelineVersionByIDForTenant(ctx, db.GetDataPipelineVersionByIDForTenantParams{TenantID: tenantID, ID: row.PublishedVersionID.Int64})
		if err == nil {
			item, err := dataPipelineVersionFromDB(versionRow)
			if err != nil {
				return DataPipelineDetail{}, err
			}
			published = &item
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return DataPipelineDetail{}, fmt.Errorf("get published data pipeline version: %w", err)
		}
	}
	return DataPipelineDetail{
		Pipeline:         pipeline,
		PublishedVersion: published,
		Versions:         versions,
		Runs:             runs,
		Schedules:        schedules,
	}, nil
}

func (s *DataPipelineService) Update(ctx context.Context, tenantID, userID int64, publicID string, input DataPipelineInput, auditCtx AuditContext) (DataPipeline, error) {
	normalized, err := normalizeDataPipelineInput(input)
	if err != nil {
		return DataPipeline{}, err
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return DataPipeline{}, ErrDataPipelineNotFound
	}
	row, err := s.queries.UpdateDataPipeline(ctx, db.UpdateDataPipelineParams{
		Name:            normalized.Name,
		Description:     normalized.Description,
		UpdatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		TenantID:        tenantID,
		PublicID:        parsed,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DataPipeline{}, ErrDataPipelineNotFound
	}
	if err != nil {
		return DataPipeline{}, fmt.Errorf("update data pipeline: %w", err)
	}
	item := dataPipelineFromDB(row)
	s.recordAudit(ctx, auditCtx, "data_pipeline.update", "data_pipeline", item.PublicID, nil)
	return item, nil
}

func (s *DataPipelineService) SaveDraftVersion(ctx context.Context, tenantID, userID int64, pipelinePublicID string, graph DataPipelineGraph, auditCtx AuditContext) (DataPipelineVersion, error) {
	if s == nil || s.queries == nil {
		return DataPipelineVersion{}, fmt.Errorf("data pipeline service is not configured")
	}
	pipeline, err := s.getPipelineRow(ctx, tenantID, pipelinePublicID)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	summary := validateDataPipelineGraph(graph)
	if !summary.Valid {
		return DataPipelineVersion{}, fmt.Errorf("%w: %s", ErrInvalidDataPipelineGraph, strings.Join(summary.Errors, "; "))
	}
	graphJSON, err := encodeDataPipelineJSON(graph)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	summaryJSON, err := encodeDataPipelineJSON(summary)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	row, err := s.queries.CreateDataPipelineVersion(ctx, db.CreateDataPipelineVersionParams{
		TenantID:          tenantID,
		PipelineID:        pipeline.ID,
		Graph:             graphJSON,
		ValidationSummary: summaryJSON,
		CreatedByUserID:   pgtype.Int8{Int64: userID, Valid: userID > 0},
	})
	if err != nil {
		return DataPipelineVersion{}, fmt.Errorf("create data pipeline version: %w", err)
	}
	item, err := dataPipelineVersionFromDB(row)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	s.recordAudit(ctx, auditCtx, "data_pipeline.version.save", "data_pipeline_version", item.PublicID, nil)
	return item, nil
}

func (s *DataPipelineService) PublishVersion(ctx context.Context, tenantID, userID int64, versionPublicID string, auditCtx AuditContext) (DataPipelineVersion, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return DataPipelineVersion{}, fmt.Errorf("data pipeline service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(versionPublicID))
	if err != nil {
		return DataPipelineVersion{}, ErrDataPipelineVersionNotFound
	}
	version, err := s.queries.GetDataPipelineVersionForTenant(ctx, db.GetDataPipelineVersionForTenantParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return DataPipelineVersion{}, ErrDataPipelineVersionNotFound
	}
	if err != nil {
		return DataPipelineVersion{}, fmt.Errorf("get data pipeline version: %w", err)
	}
	graph, err := decodeDataPipelineGraph(version.Graph)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	summary := validateDataPipelineGraph(graph)
	if !summary.Valid {
		return DataPipelineVersion{}, fmt.Errorf("%w: %s", ErrInvalidDataPipelineGraph, strings.Join(summary.Errors, "; "))
	}
	summaryJSON, err := encodeDataPipelineJSON(summary)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DataPipelineVersion{}, fmt.Errorf("begin data pipeline publish transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	published, err := qtx.PublishDataPipelineVersion(ctx, db.PublishDataPipelineVersionParams{
		ValidationSummary: summaryJSON,
		PublishedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		TenantID:          tenantID,
		PublicID:          parsed,
	})
	if err != nil {
		return DataPipelineVersion{}, fmt.Errorf("publish data pipeline version: %w", err)
	}
	if err := qtx.ArchivePublishedDataPipelineVersionsExcept(ctx, db.ArchivePublishedDataPipelineVersionsExceptParams{TenantID: tenantID, PipelineID: version.PipelineID, VersionID: version.ID}); err != nil {
		return DataPipelineVersion{}, fmt.Errorf("archive previous data pipeline versions: %w", err)
	}
	if _, err := qtx.SetDataPipelinePublishedVersion(ctx, db.SetDataPipelinePublishedVersionParams{
		VersionID:       pgtype.Int8{Int64: version.ID, Valid: true},
		UpdatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		TenantID:        tenantID,
		PipelineID:      version.PipelineID,
	}); err != nil {
		return DataPipelineVersion{}, fmt.Errorf("set pipeline published version: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return DataPipelineVersion{}, fmt.Errorf("commit data pipeline publish transaction: %w", err)
	}
	item, err := dataPipelineVersionFromDB(published)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	s.recordAudit(ctx, auditCtx, "data_pipeline.version.publish", "data_pipeline_version", item.PublicID, nil)
	return item, nil
}

func (s *DataPipelineService) Preview(ctx context.Context, tenantID int64, versionPublicID, nodeID string, limit int32) (DataPipelinePreview, error) {
	version, err := s.getVersionRow(ctx, tenantID, versionPublicID)
	if err != nil {
		return DataPipelinePreview{}, err
	}
	graph, err := decodeDataPipelineGraph(version.Graph)
	if err != nil {
		return DataPipelinePreview{}, err
	}
	return s.previewGraph(ctx, tenantID, graph, nodeID, limit)
}

func (s *DataPipelineService) PreviewDraft(ctx context.Context, tenantID int64, pipelinePublicID string, graph DataPipelineGraph, nodeID string, limit int32) (DataPipelinePreview, error) {
	if _, err := s.getPipelineRow(ctx, tenantID, pipelinePublicID); err != nil {
		return DataPipelinePreview{}, err
	}
	return s.previewGraph(ctx, tenantID, graph, nodeID, limit)
}

func (s *DataPipelineService) previewGraph(ctx context.Context, tenantID int64, graph DataPipelineGraph, nodeID string, limit int32) (DataPipelinePreview, error) {
	summary := validateDataPipelineGraph(graph)
	if !summary.Valid {
		return DataPipelinePreview{}, fmt.Errorf("%w: %s", ErrInvalidDataPipelineGraph, strings.Join(summary.Errors, "; "))
	}
	compiled, err := s.compilePreviewSelect(ctx, tenantID, graph, nodeID, limit)
	if err != nil {
		return DataPipelinePreview{}, err
	}
	if err := s.datasets.ensureTenantSandbox(ctx, tenantID); err != nil {
		return DataPipelinePreview{}, err
	}
	conn, err := s.datasets.openTenantConn(ctx, tenantID)
	if err != nil {
		return DataPipelinePreview{}, err
	}
	defer conn.Close()
	rows, err := conn.Query(clickhouse.Context(ctx, clickhouse.WithSettings(s.datasets.querySettings())), compiled.SQL)
	if err != nil {
		return DataPipelinePreview{}, fmt.Errorf("preview data pipeline: %w", err)
	}
	defer rows.Close()
	columns, previewRows, err := scanDatasetRows(rows, int(limit))
	if err != nil {
		return DataPipelinePreview{}, err
	}
	return DataPipelinePreview{
		NodeID:      compiled.NodeID,
		StepType:    compiled.StepType,
		Columns:     columns,
		PreviewRows: previewRows,
	}, nil
}

func (s *DataPipelineService) RequestRun(ctx context.Context, tenantID int64, userID *int64, versionPublicID string, triggerKind string, scheduleID *int64, auditCtx AuditContext) (DataPipelineRun, error) {
	if s == nil || s.pool == nil || s.queries == nil || s.outbox == nil {
		return DataPipelineRun{}, fmt.Errorf("data pipeline service is not configured")
	}
	version, err := s.getVersionRow(ctx, tenantID, versionPublicID)
	if err != nil {
		return DataPipelineRun{}, err
	}
	if version.Status != "published" {
		return DataPipelineRun{}, ErrDataPipelineVersionUnpublished
	}
	pipeline, err := s.queries.GetDataPipelineByIDForTenant(ctx, db.GetDataPipelineByIDForTenantParams{TenantID: tenantID, ID: version.PipelineID})
	if err != nil {
		return DataPipelineRun{}, fmt.Errorf("get data pipeline for run: %w", err)
	}
	if !pipeline.PublishedVersionID.Valid || pipeline.PublishedVersionID.Int64 != version.ID {
		return DataPipelineRun{}, ErrDataPipelineVersionUnpublished
	}
	graph, err := decodeDataPipelineGraph(version.Graph)
	if err != nil {
		return DataPipelineRun{}, err
	}
	summary := validateDataPipelineGraph(graph)
	if !summary.Valid {
		return DataPipelineRun{}, fmt.Errorf("%w: %s", ErrInvalidDataPipelineGraph, strings.Join(summary.Errors, "; "))
	}
	if triggerKind == "" {
		triggerKind = "manual"
	}
	if triggerKind != "manual" && triggerKind != "scheduled" {
		return DataPipelineRun{}, fmt.Errorf("%w: unsupported trigger kind", ErrInvalidDataPipelineInput)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DataPipelineRun{}, fmt.Errorf("begin data pipeline run transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	run, err := s.createRunWithQueries(ctx, qtx, tenantID, version, userID, triggerKind, scheduleID)
	if err != nil {
		return DataPipelineRun{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DataPipelineRun{}, fmt.Errorf("commit data pipeline run transaction: %w", err)
	}
	item := dataPipelineRunFromDB(run)
	s.recordAudit(ctx, auditCtx, "data_pipeline.run.request", "data_pipeline_run", item.PublicID, map[string]any{"triggerKind": triggerKind})
	return item, nil
}

func (s *DataPipelineService) ListRuns(ctx context.Context, tenantID int64, pipelinePublicID string, limit int32) ([]DataPipelineRun, error) {
	pipeline, err := s.getPipelineRow(ctx, tenantID, pipelinePublicID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.queries.ListDataPipelineRuns(ctx, db.ListDataPipelineRunsParams{TenantID: tenantID, PipelineID: pipeline.ID, LimitCount: limit})
	if err != nil {
		return nil, fmt.Errorf("list data pipeline runs: %w", err)
	}
	items := make([]DataPipelineRun, 0, len(rows))
	for _, row := range rows {
		item := dataPipelineRunFromDB(row)
		steps, err := s.listRunSteps(ctx, tenantID, row.ID)
		if err != nil {
			return nil, err
		}
		item.Steps = steps
		items = append(items, item)
	}
	return items, nil
}

func (s *DataPipelineService) CreateSchedule(ctx context.Context, tenantID, userID int64, pipelinePublicID string, input DataPipelineScheduleInput, auditCtx AuditContext) (DataPipelineSchedule, error) {
	pipeline, err := s.getPipelineRow(ctx, tenantID, pipelinePublicID)
	if err != nil {
		return DataPipelineSchedule{}, err
	}
	if !pipeline.PublishedVersionID.Valid {
		return DataPipelineSchedule{}, ErrDataPipelineVersionUnpublished
	}
	normalized, nextRun, err := normalizeDataPipelineScheduleInput(input, time.Now())
	if err != nil {
		return DataPipelineSchedule{}, err
	}
	enabled := true
	if normalized.Enabled != nil {
		enabled = *normalized.Enabled
	}
	row, err := s.queries.CreateDataPipelineSchedule(ctx, db.CreateDataPipelineScheduleParams{
		TenantID:        tenantID,
		PipelineID:      pipeline.ID,
		VersionID:       pipeline.PublishedVersionID.Int64,
		CreatedByUserID: pgtype.Int8{Int64: userID, Valid: userID > 0},
		Frequency:       normalized.Frequency,
		Timezone:        normalized.Timezone,
		RunTime:         normalized.RunTime,
		Weekday:         pgInt2(normalized.Weekday),
		MonthDay:        pgInt2(normalized.MonthDay),
		Enabled:         enabled,
		NextRunAt:       pgTimestamp(nextRun),
	})
	if err != nil {
		return DataPipelineSchedule{}, fmt.Errorf("create data pipeline schedule: %w", err)
	}
	item := dataPipelineScheduleFromDB(row)
	s.recordAudit(ctx, auditCtx, "data_pipeline.schedule.create", "data_pipeline_schedule", item.PublicID, nil)
	return item, nil
}

func (s *DataPipelineService) ListSchedules(ctx context.Context, tenantID int64, pipelinePublicID string) ([]DataPipelineSchedule, error) {
	pipeline, err := s.getPipelineRow(ctx, tenantID, pipelinePublicID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListDataPipelineSchedules(ctx, db.ListDataPipelineSchedulesParams{TenantID: tenantID, PipelineID: pipeline.ID})
	if err != nil {
		return nil, fmt.Errorf("list data pipeline schedules: %w", err)
	}
	items := make([]DataPipelineSchedule, 0, len(rows))
	for _, row := range rows {
		items = append(items, dataPipelineScheduleFromDB(row))
	}
	return items, nil
}

func (s *DataPipelineService) UpdateSchedule(ctx context.Context, tenantID, userID int64, schedulePublicID string, input DataPipelineScheduleInput, auditCtx AuditContext) (DataPipelineSchedule, error) {
	existing, err := s.getScheduleRow(ctx, tenantID, schedulePublicID)
	if err != nil {
		return DataPipelineSchedule{}, err
	}
	normalized, nextRun, err := normalizeDataPipelineScheduleInput(input, time.Now())
	if err != nil {
		return DataPipelineSchedule{}, err
	}
	enabled := existing.Enabled
	if normalized.Enabled != nil {
		enabled = *normalized.Enabled
	}
	parsed, _ := uuid.Parse(strings.TrimSpace(schedulePublicID))
	row, err := s.queries.UpdateDataPipelineSchedule(ctx, db.UpdateDataPipelineScheduleParams{
		Frequency: normalized.Frequency,
		Timezone:  normalized.Timezone,
		RunTime:   normalized.RunTime,
		Weekday:   pgInt2(normalized.Weekday),
		MonthDay:  pgInt2(normalized.MonthDay),
		Enabled:   enabled,
		NextRunAt: pgTimestamp(nextRun),
		TenantID:  tenantID,
		PublicID:  parsed,
	})
	if err != nil {
		return DataPipelineSchedule{}, fmt.Errorf("update data pipeline schedule: %w", err)
	}
	item := dataPipelineScheduleFromDB(row)
	s.recordAudit(ctx, auditCtx, "data_pipeline.schedule.update", "data_pipeline_schedule", item.PublicID, map[string]any{"actorUserID": userID})
	return item, nil
}

func (s *DataPipelineService) DisableSchedule(ctx context.Context, tenantID, userID int64, schedulePublicID string, auditCtx AuditContext) (DataPipelineSchedule, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(schedulePublicID))
	if err != nil {
		return DataPipelineSchedule{}, ErrDataPipelineScheduleNotFound
	}
	row, err := s.queries.DisableDataPipelineSchedule(ctx, db.DisableDataPipelineScheduleParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return DataPipelineSchedule{}, ErrDataPipelineScheduleNotFound
	}
	if err != nil {
		return DataPipelineSchedule{}, fmt.Errorf("disable data pipeline schedule: %w", err)
	}
	item := dataPipelineScheduleFromDB(row)
	s.recordAudit(ctx, auditCtx, "data_pipeline.schedule.disable", "data_pipeline_schedule", item.PublicID, map[string]any{"actorUserID": userID})
	return item, nil
}

func (s *DataPipelineService) HandleRunRequested(ctx context.Context, tenantID, runID, outboxEventID int64) error {
	run, err := s.queries.MarkDataPipelineRunProcessing(ctx, db.MarkDataPipelineRunProcessingParams{TenantID: tenantID, ID: runID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDataPipelineRunNotFound
	}
	if err != nil {
		return fmt.Errorf("mark data pipeline run processing: %w", err)
	}
	version, err := s.queries.GetDataPipelineVersionByIDForTenant(ctx, db.GetDataPipelineVersionByIDForTenantParams{TenantID: tenantID, ID: run.VersionID})
	if err != nil {
		s.failRunBestEffort(ctx, tenantID, runID, err.Error())
		return fmt.Errorf("get data pipeline run version: %w", err)
	}
	graph, err := decodeDataPipelineGraph(version.Graph)
	if err != nil {
		s.failRunBestEffort(ctx, tenantID, runID, err.Error())
		return err
	}
	for _, node := range graph.Nodes {
		if _, err := s.queries.CreateDataPipelineRunStep(ctx, db.CreateDataPipelineRunStepParams{TenantID: tenantID, RunID: runID, NodeID: node.ID, StepType: node.Data.StepType}); err != nil {
			s.failRunBestEffort(ctx, tenantID, runID, err.Error())
			return fmt.Errorf("create data pipeline run step: %w", err)
		}
		_, _ = s.queries.MarkDataPipelineRunStepProcessing(ctx, db.MarkDataPipelineRunStepProcessingParams{TenantID: tenantID, RunID: runID, NodeID: node.ID})
	}

	workTable, compiled, err := s.executeRun(ctx, tenantID, run, version)
	if err != nil {
		errorSample, _ := encodeDataPipelineJSON([]map[string]any{{"error": err.Error()}})
		for _, node := range graph.Nodes {
			_, _ = s.queries.FailDataPipelineRunStep(ctx, db.FailDataPipelineRunStepParams{TenantID: tenantID, RunID: runID, NodeID: node.ID, ErrorSummary: err.Error(), ErrorSample: errorSample})
		}
		s.failRunBestEffort(ctx, tenantID, runID, err.Error())
		s.recordMedallionRun(ctx, tenantID, run, version, compiled, nil, MedallionPipelineStatusFailed, err.Error())
		return err
	}
	meta, _ := encodeDataPipelineJSON(map[string]any{})
	for _, node := range graph.Nodes {
		_, _ = s.queries.CompleteDataPipelineRunStep(ctx, db.CompleteDataPipelineRunStepParams{TenantID: tenantID, RunID: runID, NodeID: node.ID, RowCount: workTable.TotalRows, Metadata: meta})
	}
	completed, err := s.queries.CompleteDataPipelineRun(ctx, db.CompleteDataPipelineRunParams{TenantID: tenantID, ID: runID, OutputWorkTableID: pgtype.Int8{Int64: workTable.ID, Valid: true}, RowCount: workTable.TotalRows})
	if err != nil {
		return fmt.Errorf("complete data pipeline run: %w", err)
	}
	s.recordMedallionRun(ctx, tenantID, completed, version, compiled, &workTable, MedallionPipelineStatusCompleted, "")
	s.recordAudit(ctx, AuditContext{ActorType: "system", TenantID: &tenantID}, "data_pipeline.run.complete", "data_pipeline_run", completed.PublicID.String(), map[string]any{"outboxEventID": outboxEventID})
	return nil
}

func (s *DataPipelineService) RunDueSchedules(ctx context.Context, now time.Time, batchSize int32) (DataPipelineScheduleRunSummary, error) {
	var summary DataPipelineScheduleRunSummary
	if s == nil || s.pool == nil || s.queries == nil || s.outbox == nil {
		return summary, fmt.Errorf("data pipeline service is not configured")
	}
	if now.IsZero() {
		now = time.Now()
	}
	if batchSize <= 0 {
		batchSize = 20
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return summary, fmt.Errorf("begin data pipeline schedule transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	schedules, err := qtx.ClaimDueDataPipelineSchedules(ctx, db.ClaimDueDataPipelineSchedulesParams{Now: pgTimestamp(now), BatchLimit: batchSize})
	if err != nil {
		return summary, fmt.Errorf("claim data pipeline schedules: %w", err)
	}
	summary.Claimed = len(schedules)
	for _, schedule := range schedules {
		nextRun, err := nextDataPipelineScheduleRunAfter(schedule.Frequency, schedule.Timezone, schedule.RunTime, optionalPgInt2(schedule.Weekday), optionalPgInt2(schedule.MonthDay), now)
		if err != nil {
			_, markErr := qtx.MarkDataPipelineScheduleFailed(ctx, db.MarkDataPipelineScheduleFailedParams{Enabled: false, LastRunAt: pgTimestamp(now), LastStatus: pgText("disabled"), ErrorSummary: err.Error(), NextRunAt: schedule.NextRunAt, TenantID: schedule.TenantID, ID: schedule.ID})
			if markErr != nil {
				return summary, markErr
			}
			summary.Failed++
			summary.Disabled++
			continue
		}
		pipeline, err := qtx.GetDataPipelineByIDForTenant(ctx, db.GetDataPipelineByIDForTenantParams{TenantID: schedule.TenantID, ID: schedule.PipelineID})
		if errors.Is(err, pgx.ErrNoRows) {
			_, markErr := qtx.MarkDataPipelineScheduleFailed(ctx, db.MarkDataPipelineScheduleFailedParams{Enabled: false, LastRunAt: pgTimestamp(now), LastStatus: pgText("disabled"), ErrorSummary: ErrDataPipelineVersionUnpublished.Error(), NextRunAt: pgTimestamp(nextRun), TenantID: schedule.TenantID, ID: schedule.ID})
			if markErr != nil {
				return summary, markErr
			}
			summary.Disabled++
			continue
		}
		if err != nil {
			return summary, fmt.Errorf("get scheduled data pipeline: %w", err)
		}
		if !pipeline.PublishedVersionID.Valid || pipeline.PublishedVersionID.Int64 != schedule.VersionID {
			_, markErr := qtx.MarkDataPipelineScheduleFailed(ctx, db.MarkDataPipelineScheduleFailedParams{Enabled: false, LastRunAt: pgTimestamp(now), LastStatus: pgText("disabled"), ErrorSummary: ErrDataPipelineVersionUnpublished.Error(), NextRunAt: pgTimestamp(nextRun), TenantID: schedule.TenantID, ID: schedule.ID})
			if markErr != nil {
				return summary, markErr
			}
			summary.Disabled++
			continue
		}
		active, err := qtx.CountActiveDataPipelineRunsForSchedule(ctx, db.CountActiveDataPipelineRunsForScheduleParams{
			TenantID:   schedule.TenantID,
			ScheduleID: pgtype.Int8{Int64: schedule.ID, Valid: true},
		})
		if err != nil {
			return summary, fmt.Errorf("count active data pipeline runs: %w", err)
		}
		if active > 0 {
			if _, err := qtx.MarkDataPipelineScheduleSkipped(ctx, db.MarkDataPipelineScheduleSkippedParams{LastRunAt: pgTimestamp(now), ErrorSummary: "previous scheduled run is still pending or processing", NextRunAt: pgTimestamp(nextRun), TenantID: schedule.TenantID, ID: schedule.ID}); err != nil {
				return summary, err
			}
			summary.Skipped++
			continue
		}
		version, err := qtx.GetDataPipelineVersionByIDForTenant(ctx, db.GetDataPipelineVersionByIDForTenantParams{TenantID: schedule.TenantID, ID: schedule.VersionID})
		if err != nil {
			return summary, fmt.Errorf("get scheduled data pipeline version: %w", err)
		}
		scheduleID := schedule.ID
		run, err := s.createRunWithQueries(ctx, qtx, schedule.TenantID, version, optionalPgInt8(schedule.CreatedByUserID), "scheduled", &scheduleID)
		if err != nil {
			return summary, err
		}
		if _, err := qtx.MarkDataPipelineScheduleCreated(ctx, db.MarkDataPipelineScheduleCreatedParams{LastRunAt: pgTimestamp(now), LastRunID: pgtype.Int8{Int64: run.ID, Valid: true}, NextRunAt: pgTimestamp(nextRun), TenantID: schedule.TenantID, ID: schedule.ID}); err != nil {
			return summary, err
		}
		summary.Created++
	}
	if err := tx.Commit(ctx); err != nil {
		return summary, fmt.Errorf("commit data pipeline schedule transaction: %w", err)
	}
	return summary, nil
}

func (s *DataPipelineService) createRunWithQueries(ctx context.Context, qtx *db.Queries, tenantID int64, version db.DataPipelineVersion, userID *int64, triggerKind string, scheduleID *int64) (db.DataPipelineRun, error) {
	run, err := qtx.CreateDataPipelineRun(ctx, db.CreateDataPipelineRunParams{
		TenantID:          tenantID,
		PipelineID:        version.PipelineID,
		VersionID:         version.ID,
		ScheduleID:        pgInt8(scheduleID),
		RequestedByUserID: pgInt8(userID),
		TriggerKind:       triggerKind,
	})
	if err != nil {
		return db.DataPipelineRun{}, fmt.Errorf("create data pipeline run: %w", err)
	}
	pipelinePublicID := ""
	pipeline, err := qtx.GetDataPipelineByIDForTenant(ctx, db.GetDataPipelineByIDForTenantParams{TenantID: tenantID, ID: version.PipelineID})
	if err != nil {
		return db.DataPipelineRun{}, fmt.Errorf("get data pipeline for run event: %w", err)
	}
	pipelinePublicID = pipeline.PublicID.String()
	event, err := s.outbox.EnqueueWithQueries(ctx, qtx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "data_pipeline_run",
		AggregateID:   run.PublicID.String(),
		EventType:     "data_pipeline.run_requested",
		Payload: map[string]any{
			"tenantId":         tenantID,
			"runId":            run.ID,
			"pipelinePublicId": pipelinePublicID,
			"versionPublicId":  version.PublicID.String(),
		},
	})
	if err != nil {
		return db.DataPipelineRun{}, err
	}
	run, err = qtx.SetDataPipelineRunOutboxEvent(ctx, db.SetDataPipelineRunOutboxEventParams{TenantID: tenantID, ID: run.ID, OutboxEventID: pgtype.Int8{Int64: event.ID, Valid: true}})
	if err != nil {
		return db.DataPipelineRun{}, fmt.Errorf("set data pipeline run outbox event: %w", err)
	}
	return run, nil
}

func (s *DataPipelineService) listVersionsForPipeline(ctx context.Context, tenantID, pipelineID int64, limit int32) ([]DataPipelineVersion, error) {
	rows, err := s.queries.ListDataPipelineVersions(ctx, db.ListDataPipelineVersionsParams{TenantID: tenantID, PipelineID: pipelineID, LimitCount: limit})
	if err != nil {
		return nil, fmt.Errorf("list data pipeline versions: %w", err)
	}
	items := make([]DataPipelineVersion, 0, len(rows))
	for _, row := range rows {
		item, err := dataPipelineVersionFromDB(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *DataPipelineService) listRunSteps(ctx context.Context, tenantID, runID int64) ([]DataPipelineRunStep, error) {
	rows, err := s.queries.ListDataPipelineRunSteps(ctx, db.ListDataPipelineRunStepsParams{TenantID: tenantID, RunID: runID})
	if err != nil {
		return nil, fmt.Errorf("list data pipeline run steps: %w", err)
	}
	items := make([]DataPipelineRunStep, 0, len(rows))
	for _, row := range rows {
		items = append(items, dataPipelineRunStepFromDB(row))
	}
	return items, nil
}

func (s *DataPipelineService) getPipelineRow(ctx context.Context, tenantID int64, publicID string) (db.DataPipeline, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return db.DataPipeline{}, ErrDataPipelineNotFound
	}
	row, err := s.queries.GetDataPipelineForTenant(ctx, db.GetDataPipelineForTenantParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DataPipeline{}, ErrDataPipelineNotFound
	}
	if err != nil {
		return db.DataPipeline{}, fmt.Errorf("get data pipeline: %w", err)
	}
	return row, nil
}

func (s *DataPipelineService) getVersionRow(ctx context.Context, tenantID int64, publicID string) (db.DataPipelineVersion, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return db.DataPipelineVersion{}, ErrDataPipelineVersionNotFound
	}
	row, err := s.queries.GetDataPipelineVersionForTenant(ctx, db.GetDataPipelineVersionForTenantParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DataPipelineVersion{}, ErrDataPipelineVersionNotFound
	}
	if err != nil {
		return db.DataPipelineVersion{}, fmt.Errorf("get data pipeline version: %w", err)
	}
	return row, nil
}

func (s *DataPipelineService) getScheduleRow(ctx context.Context, tenantID int64, publicID string) (db.DataPipelineSchedule, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return db.DataPipelineSchedule{}, ErrDataPipelineScheduleNotFound
	}
	row, err := s.queries.GetDataPipelineScheduleForTenant(ctx, db.GetDataPipelineScheduleForTenantParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DataPipelineSchedule{}, ErrDataPipelineScheduleNotFound
	}
	if err != nil {
		return db.DataPipelineSchedule{}, fmt.Errorf("get data pipeline schedule: %w", err)
	}
	return row, nil
}

func (s *DataPipelineService) failRunBestEffort(ctx context.Context, tenantID, runID int64, message string) {
	if s == nil || s.queries == nil {
		return
	}
	_, _ = s.queries.FailDataPipelineRun(ctx, db.FailDataPipelineRunParams{TenantID: tenantID, ID: runID, ErrorSummary: message})
}

func (s *DataPipelineService) recordAudit(ctx context.Context, auditCtx AuditContext, action, targetType, targetID string, metadata map[string]any) {
	if s == nil || s.audit == nil {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	s.audit.RecordBestEffort(ctx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       action,
		TargetType:   targetType,
		TargetID:     targetID,
		Metadata:     metadata,
	})
}

func (s *DataPipelineService) recordMedallionRun(ctx context.Context, tenantID int64, run db.DataPipelineRun, version db.DataPipelineVersion, compiled dataPipelineCompiledSelect, workTable *DatasetWorkTable, status, errorSummary string) {
	if s == nil || s.medallion == nil || compiled.Source == nil {
		return
	}
	var targetKind string
	var targetID int64
	var targetPublicID string
	var targetAssets []MedallionAsset
	if workTable != nil && workTable.ID > 0 {
		targetKind = MedallionResourceWorkTable
		targetID = workTable.ID
		targetPublicID = workTable.PublicID
		if asset, err := s.medallion.EnsureWorkTableAsset(ctx, *workTable, optionalPgInt8(run.RequestedByUserID)); err == nil {
			targetAssets = append(targetAssets, asset)
		}
	}
	sourceKind := MedallionResourceDataset
	if compiled.Source.Kind == "work_table" {
		sourceKind = MedallionResourceWorkTable
	}
	if status == "" {
		status = MedallionPipelineStatusPending
	}
	var completedAt *time.Time
	if status == MedallionPipelineStatusCompleted || status == MedallionPipelineStatusFailed || status == MedallionPipelineStatusSkipped {
		now := time.Now()
		completedAt = &now
	}
	_, _ = s.medallion.RecordPipelineRun(ctx, medallionPipelineRunInput{
		TenantID:               tenantID,
		PipelineType:           MedallionPipelineDataPipeline,
		RunKey:                 run.PublicID.String(),
		SourceResourceKind:     sourceKind,
		SourceResourceID:       compiled.Source.ID,
		SourceResourcePublicID: compiled.Source.PublicID,
		TargetResourceKind:     targetKind,
		TargetResourceID:       targetID,
		TargetResourcePublicID: targetPublicID,
		Status:                 status,
		Runtime:                "clickhouse",
		TriggerKind:            run.TriggerKind,
		Retryable:              status == MedallionPipelineStatusFailed,
		ErrorSummary:           errorSummary,
		Metadata: map[string]any{
			"versionPublicId": version.PublicID.String(),
		},
		RequestedByUserID: optionalPgInt8(run.RequestedByUserID),
		CompletedAt:       completedAt,
		TargetAssets:      targetAssets,
	})
}

func normalizeDataPipelineInput(input DataPipelineInput) (DataPipelineInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" {
		return DataPipelineInput{}, fmt.Errorf("%w: name is required", ErrInvalidDataPipelineInput)
	}
	if len([]rune(input.Name)) > 160 {
		return DataPipelineInput{}, fmt.Errorf("%w: name is too long", ErrInvalidDataPipelineInput)
	}
	if len([]rune(input.Description)) > 2000 {
		return DataPipelineInput{}, fmt.Errorf("%w: description is too long", ErrInvalidDataPipelineInput)
	}
	return input, nil
}

func normalizeDataPipelineScheduleInput(input DataPipelineScheduleInput, after time.Time) (DataPipelineScheduleInput, time.Time, error) {
	input.Frequency = strings.TrimSpace(input.Frequency)
	if input.Frequency == "" {
		input.Frequency = "daily"
	}
	input.Timezone = strings.TrimSpace(input.Timezone)
	if input.Timezone == "" {
		input.Timezone = "Asia/Tokyo"
	}
	input.RunTime = strings.TrimSpace(input.RunTime)
	if input.RunTime == "" {
		input.RunTime = "03:00"
	}
	nextRun, err := nextDataPipelineScheduleRunAfter(input.Frequency, input.Timezone, input.RunTime, input.Weekday, input.MonthDay, after)
	if err != nil {
		return DataPipelineScheduleInput{}, time.Time{}, err
	}
	return input, nextRun, nil
}

func dataPipelineFromDB(row db.DataPipeline) DataPipeline {
	return DataPipeline{
		ID:                 row.ID,
		PublicID:           row.PublicID.String(),
		TenantID:           row.TenantID,
		CreatedByUserID:    optionalPgInt8(row.CreatedByUserID),
		UpdatedByUserID:    optionalPgInt8(row.UpdatedByUserID),
		Name:               row.Name,
		Description:        row.Description,
		Status:             row.Status,
		PublishedVersionID: optionalPgInt8(row.PublishedVersionID),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
		ArchivedAt:         optionalPgTime(row.ArchivedAt),
	}
}

func dataPipelineVersionFromDB(row db.DataPipelineVersion) (DataPipelineVersion, error) {
	graph, err := decodeDataPipelineGraph(row.Graph)
	if err != nil {
		return DataPipelineVersion{}, err
	}
	var summary DataPipelineValidationSummary
	if len(row.ValidationSummary) > 0 {
		_ = json.Unmarshal(row.ValidationSummary, &summary)
	}
	return DataPipelineVersion{
		ID:                row.ID,
		PublicID:          row.PublicID.String(),
		TenantID:          row.TenantID,
		PipelineID:        row.PipelineID,
		VersionNumber:     row.VersionNumber,
		Status:            row.Status,
		Graph:             graph,
		ValidationSummary: summary,
		CreatedByUserID:   optionalPgInt8(row.CreatedByUserID),
		PublishedByUserID: optionalPgInt8(row.PublishedByUserID),
		CreatedAt:         row.CreatedAt.Time,
		PublishedAt:       optionalPgTime(row.PublishedAt),
	}, nil
}

func dataPipelineRunFromDB(row db.DataPipelineRun) DataPipelineRun {
	return DataPipelineRun{
		ID:                row.ID,
		PublicID:          row.PublicID.String(),
		TenantID:          row.TenantID,
		PipelineID:        row.PipelineID,
		VersionID:         row.VersionID,
		ScheduleID:        optionalPgInt8(row.ScheduleID),
		RequestedByUserID: optionalPgInt8(row.RequestedByUserID),
		TriggerKind:       row.TriggerKind,
		Status:            row.Status,
		OutputWorkTableID: optionalPgInt8(row.OutputWorkTableID),
		OutboxEventID:     optionalPgInt8(row.OutboxEventID),
		RowCount:          row.RowCount,
		ErrorSummary:      optionalText(row.ErrorSummary),
		StartedAt:         optionalPgTime(row.StartedAt),
		CompletedAt:       optionalPgTime(row.CompletedAt),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
}

func dataPipelineRunStepFromDB(row db.DataPipelineRunStep) DataPipelineRunStep {
	var sample []map[string]any
	_ = json.Unmarshal(row.ErrorSample, &sample)
	if sample == nil {
		sample = []map[string]any{}
	}
	return DataPipelineRunStep{
		ID:           row.ID,
		TenantID:     row.TenantID,
		RunID:        row.RunID,
		NodeID:       row.NodeID,
		StepType:     row.StepType,
		Status:       row.Status,
		RowCount:     row.RowCount,
		ErrorSummary: optionalText(row.ErrorSummary),
		ErrorSample:  sample,
		Metadata:     decodeDataPipelineJSONMap(row.Metadata),
		StartedAt:    optionalPgTime(row.StartedAt),
		CompletedAt:  optionalPgTime(row.CompletedAt),
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func dataPipelineScheduleFromDB(row db.DataPipelineSchedule) DataPipelineSchedule {
	return DataPipelineSchedule{
		ID:               row.ID,
		PublicID:         row.PublicID.String(),
		TenantID:         row.TenantID,
		PipelineID:       row.PipelineID,
		VersionID:        row.VersionID,
		CreatedByUserID:  optionalPgInt8(row.CreatedByUserID),
		Frequency:        row.Frequency,
		Timezone:         row.Timezone,
		RunTime:          row.RunTime,
		Weekday:          optionalPgInt2(row.Weekday),
		MonthDay:         optionalPgInt2(row.MonthDay),
		Enabled:          row.Enabled,
		NextRunAt:        row.NextRunAt.Time,
		LastRunAt:        optionalPgTime(row.LastRunAt),
		LastStatus:       optionalText(row.LastStatus),
		LastErrorSummary: optionalText(row.LastErrorSummary),
		LastRunID:        optionalPgInt8(row.LastRunID),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}
