package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSystemJobNotFound     = errors.New("system job not found")
	ErrSystemJobNotStoppable = errors.New("system job is not stoppable")
)

type SystemJob struct {
	Type                   string
	PublicID               string
	Title                  string
	SubjectType            string
	SubjectPublicID        string
	RequestedByUserID      *int64
	RequestedByDisplayName string
	RequestedByEmail       string
	Status                 string
	StatusGroup            string
	Action                 string
	ErrorMessage           string
	OutboxEventPublicID    string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	StartedAt              *time.Time
	CompletedAt            *time.Time
	Metadata               map[string]any
	CanStop                bool
}

type SystemJobListFilter struct {
	Query       string
	Type        string
	Status      string
	StatusGroup string
	Limit       int32
	Offset      int32
}

type SystemJobList struct {
	Items  []SystemJob
	Total  int64
	Limit  int32
	Offset int32
}

type SystemJobService struct {
	pool *pgxpool.Pool
}

func NewSystemJobService(pool *pgxpool.Pool) *SystemJobService {
	return &SystemJobService{pool: pool}
}

func (s *SystemJobService) List(ctx context.Context, tenantID int64, filter SystemJobListFilter) (SystemJobList, error) {
	if err := s.ensureConfigured(); err != nil {
		return SystemJobList{}, err
	}
	filter = normalizeSystemJobListFilter(filter)
	where, args := systemJobWhere(tenantID, filter, false)
	countSQL := systemJobUnionSQL + "\nSELECT count(*) FROM jobs " + where
	var total int64
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return SystemJobList{}, fmt.Errorf("count system jobs: %w", err)
	}
	args = append(args, filter.Limit, filter.Offset)
	rows, err := s.pool.Query(ctx, systemJobUnionSQL+"\n"+systemJobSelectSQL+" "+where+"\nORDER BY updated_at DESC, created_at DESC, public_id DESC\nLIMIT $"+fmt.Sprint(len(args)-1)+" OFFSET $"+fmt.Sprint(len(args)), args...)
	if err != nil {
		return SystemJobList{}, fmt.Errorf("list system jobs: %w", err)
	}
	defer rows.Close()
	items, err := scanSystemJobs(rows)
	if err != nil {
		return SystemJobList{}, err
	}
	return SystemJobList{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *SystemJobService) Get(ctx context.Context, tenantID int64, jobType, publicID string) (SystemJob, error) {
	if err := s.ensureConfigured(); err != nil {
		return SystemJob{}, err
	}
	filter := SystemJobListFilter{Type: jobType}
	where, args := systemJobWhere(tenantID, filter, true)
	args = append(args, strings.TrimSpace(publicID))
	rows, err := s.pool.Query(ctx, systemJobUnionSQL+"\n"+systemJobSelectSQL+" "+where+" AND public_id = $"+fmt.Sprint(len(args))+"\nLIMIT 1", args...)
	if err != nil {
		return SystemJob{}, fmt.Errorf("get system job: %w", err)
	}
	defer rows.Close()
	items, err := scanSystemJobs(rows)
	if err != nil {
		return SystemJob{}, err
	}
	if len(items) == 0 {
		return SystemJob{}, ErrSystemJobNotFound
	}
	return items[0], nil
}

func (s *SystemJobService) Stop(ctx context.Context, tenantID int64, jobType, publicID string, actorUserID int64) (SystemJob, error) {
	if err := s.ensureConfigured(); err != nil {
		return SystemJob{}, err
	}
	job, err := s.Get(ctx, tenantID, jobType, publicID)
	if err != nil {
		return SystemJob{}, err
	}
	if !job.CanStop {
		return SystemJob{}, ErrSystemJobNotStoppable
	}
	message := fmt.Sprintf("Stopped manually by user %d", actorUserID)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SystemJob{}, fmt.Errorf("begin stop system job: %w", err)
	}
	defer tx.Rollback(ctx)
	affected, err := stopSystemJob(ctx, tx, tenantID, job.Type, job.PublicID, message)
	if err != nil {
		return SystemJob{}, err
	}
	if affected == 0 {
		return SystemJob{}, ErrSystemJobNotStoppable
	}
	if err := tx.Commit(ctx); err != nil {
		return SystemJob{}, fmt.Errorf("commit stop system job: %w", err)
	}
	return s.Get(ctx, tenantID, job.Type, job.PublicID)
}

func (s *SystemJobService) ensureConfigured() error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("system job service is not configured")
	}
	return nil
}

func normalizeSystemJobListFilter(filter SystemJobListFilter) SystemJobListFilter {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Type = strings.TrimSpace(filter.Type)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.StatusGroup = strings.TrimSpace(filter.StatusGroup)
	if filter.Limit <= 0 {
		filter.Limit = 25
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return filter
}

func systemJobWhere(tenantID int64, filter SystemJobListFilter, exactType bool) (string, []any) {
	args := []any{tenantID}
	clauses := []string{"tenant_id = $1"}
	if filter.Type != "" {
		args = append(args, filter.Type)
		op := "="
		if !exactType {
			op = "="
		}
		clauses = append(clauses, "job_type "+op+" $"+fmt.Sprint(len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, "status = $"+fmt.Sprint(len(args)))
	}
	if filter.StatusGroup != "" {
		args = append(args, filter.StatusGroup)
		clauses = append(clauses, "status_group = $"+fmt.Sprint(len(args)))
	}
	if filter.Query != "" {
		args = append(args, "%"+filter.Query+"%")
		idx := fmt.Sprint(len(args))
		clauses = append(clauses, "(title ILIKE $"+idx+" OR public_id ILIKE $"+idx+" OR job_type ILIKE $"+idx+" OR status ILIKE $"+idx+" OR COALESCE(requested_by_display_name, '') ILIKE $"+idx+" OR COALESCE(requested_by_email, '') ILIKE $"+idx+" OR COALESCE(error_message, '') ILIKE $"+idx+")")
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func scanSystemJobs(rows pgx.Rows) ([]SystemJob, error) {
	items := []SystemJob{}
	for rows.Next() {
		var item SystemJob
		var metadata []byte
		if err := rows.Scan(
			&item.Type,
			&item.PublicID,
			&item.Title,
			&item.SubjectType,
			&item.SubjectPublicID,
			&item.RequestedByUserID,
			&item.RequestedByDisplayName,
			&item.RequestedByEmail,
			&item.Status,
			&item.StatusGroup,
			&item.Action,
			&item.ErrorMessage,
			&item.OutboxEventPublicID,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.StartedAt,
			&item.CompletedAt,
			&metadata,
			&item.CanStop,
		); err != nil {
			return nil, fmt.Errorf("scan system job: %w", err)
		}
		item.Metadata = map[string]any{}
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &item.Metadata)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate system jobs: %w", err)
	}
	return items, nil
}

func stopSystemJob(ctx context.Context, tx pgx.Tx, tenantID int64, jobType, publicID, message string) (int64, error) {
	var sql string
	switch jobType {
	case "outbox_event":
		sql = `UPDATE outbox_events SET status = 'dead', locked_at = NULL, locked_by = NULL, last_error = left($3, 1000), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('pending','processing','failed')`
	case "drive_ocr":
		sql = `UPDATE drive_ocr_runs SET status = 'failed', error_code = 'manual_stopped', error_message = left($3, 2000), completed_at = now(), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('pending','running')`
	case "data_pipeline_run":
		sql = `UPDATE data_pipeline_runs SET status = 'failed', error_summary = left($3, 1000), completed_at = now(), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('pending','processing')`
	case "local_search_index":
		sql = `UPDATE local_search_index_jobs SET status = 'failed', last_error = left($3, 1000), completed_at = now(), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('queued','processing')`
	case "dataset_import":
		sql = `UPDATE dataset_import_jobs SET status = 'failed', error_summary = left($3, 1000), completed_at = now(), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('pending','processing')`
	case "dataset_query":
		sql = `UPDATE dataset_query_jobs SET status = 'failed', error_summary = left($3, 1000), completed_at = now(), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status = 'running'`
	case "dataset_sync":
		sql = `UPDATE dataset_sync_jobs SET status = 'failed', error_summary = left($3, 1000), completed_at = now(), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('pending','processing')`
	case "dataset_work_table_export":
		sql = `UPDATE dataset_work_table_exports SET status = 'failed', error_summary = left($3, 1000), completed_at = now(), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('pending','processing')`
	case "dataset_gold_publish":
		sql = `UPDATE dataset_gold_publish_runs SET status = 'failed', error_summary = left($3, 1000), completed_at = now(), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('pending','processing')`
	case "dataset_lineage_parse":
		sql = `UPDATE dataset_lineage_parse_runs SET status = 'failed', error_summary = left($3, 1000), completed_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status = 'processing'`
	case "tenant_data_export":
		sql = `UPDATE tenant_data_exports SET status = 'failed', error_summary = left($3, 1000), completed_at = now(), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('pending','processing') AND deleted_at IS NULL`
	case "customer_signal_import":
		sql = `UPDATE customer_signal_import_jobs SET status = 'failed', error_summary = left($3, 1000), completed_at = now(), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('pending','processing') AND deleted_at IS NULL`
	case "webhook_delivery":
		sql = `UPDATE webhook_deliveries SET status = 'dead', last_error = left($3, 1000), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('pending','failed')`
	case "drive_index":
		sql = `UPDATE drive_index_jobs SET status = 'failed', last_error = left($3, 1000), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('queued','running')`
	case "drive_ai":
		sql = `UPDATE drive_ai_jobs SET status = 'failed', error_message = left($3, 1000), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status = 'pending'`
	case "drive_key_rotation":
		sql = `UPDATE drive_key_rotation_jobs SET status = 'failed', failure_reason = left($3, 1000), updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('queued','running')`
	case "drive_region_migration":
		sql = `UPDATE drive_region_migration_jobs SET status = 'failed', updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('queued','running','requires_approval')`
	case "drive_clean_room":
		sql = `UPDATE drive_clean_room_jobs SET status = 'failed', updated_at = now() WHERE tenant_id = $1 AND public_id = $2::uuid AND status IN ('queued','running')`
	default:
		return 0, ErrSystemJobNotFound
	}
	tag, err := tx.Exec(ctx, sql, tenantID, publicID, message)
	if err != nil {
		return 0, fmt.Errorf("stop system job: %w", err)
	}
	if tag.RowsAffected() > 0 {
		if err := stopLinkedOutboxEvent(ctx, tx, tenantID, jobType, publicID, message); err != nil {
			return 0, err
		}
		if jobType == "data_pipeline_run" {
			if _, err := tx.Exec(ctx, `UPDATE data_pipeline_run_steps SET status = 'failed', error_summary = left($3, 1000), completed_at = now(), updated_at = now() WHERE tenant_id = $1 AND run_id = (SELECT id FROM data_pipeline_runs WHERE tenant_id = $1 AND public_id = $2::uuid) AND status IN ('pending','processing')`, tenantID, publicID, message); err != nil {
				return 0, fmt.Errorf("stop data pipeline run steps: %w", err)
			}
		}
	}
	return tag.RowsAffected(), nil
}

func stopLinkedOutboxEvent(ctx context.Context, tx pgx.Tx, tenantID int64, jobType, publicID, message string) error {
	subqueries := map[string]string{
		"drive_ocr":                 "SELECT outbox_event_id FROM drive_ocr_runs WHERE tenant_id = $1 AND public_id = $2::uuid",
		"data_pipeline_run":         "SELECT outbox_event_id FROM data_pipeline_runs WHERE tenant_id = $1 AND public_id = $2::uuid",
		"local_search_index":        "SELECT outbox_event_id FROM local_search_index_jobs WHERE tenant_id = $1 AND public_id = $2::uuid",
		"dataset_import":            "SELECT outbox_event_id FROM dataset_import_jobs WHERE tenant_id = $1 AND public_id = $2::uuid",
		"dataset_sync":              "SELECT outbox_event_id FROM dataset_sync_jobs WHERE tenant_id = $1 AND public_id = $2::uuid",
		"dataset_work_table_export": "SELECT outbox_event_id FROM dataset_work_table_exports WHERE tenant_id = $1 AND public_id = $2::uuid",
		"dataset_gold_publish":      "SELECT outbox_event_id FROM dataset_gold_publish_runs WHERE tenant_id = $1 AND public_id = $2::uuid",
		"tenant_data_export":        "SELECT outbox_event_id FROM tenant_data_exports WHERE tenant_id = $1 AND public_id = $2::uuid",
		"customer_signal_import":    "SELECT outbox_event_id FROM customer_signal_import_jobs WHERE tenant_id = $1 AND public_id = $2::uuid",
		"webhook_delivery":          "SELECT outbox_event_id FROM webhook_deliveries WHERE tenant_id = $1 AND public_id = $2::uuid",
	}
	subquery := subqueries[jobType]
	if subquery == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE outbox_events SET status = 'dead', locked_at = NULL, locked_by = NULL, last_error = left($3, 1000), updated_at = now() WHERE tenant_id = $1 AND id IN (`+subquery+`) AND status IN ('pending','processing','failed')`, tenantID, publicID, message)
	if err != nil {
		return fmt.Errorf("stop linked outbox event: %w", err)
	}
	return nil
}

const systemJobSelectSQL = `SELECT
	job_type,
	public_id,
	title,
	COALESCE(subject_type, ''),
	COALESCE(subject_public_id, ''),
	requested_by_user_id,
	COALESCE(requested_by_display_name, ''),
	COALESCE(requested_by_email, ''),
	status,
	status_group,
	COALESCE(action, ''),
	COALESCE(error_message, ''),
	COALESCE(outbox_event_public_id, ''),
	created_at,
	updated_at,
	started_at,
	completed_at,
	metadata,
	can_stop
FROM jobs`

const systemJobUnionSQL = `WITH jobs AS (
	SELECT 'outbox_event' AS job_type, oe.public_id::text AS public_id, oe.tenant_id, 'Outbox: ' || oe.event_type AS title, oe.aggregate_type AS subject_type, oe.aggregate_id AS subject_public_id, NULL::bigint AS requested_by_user_id, NULL::text AS requested_by_display_name, NULL::text AS requested_by_email, oe.status, CASE WHEN oe.status IN ('pending','processing','failed') THEN 'active' ELSE 'terminal' END AS status_group, oe.event_type AS action, oe.last_error AS error_message, NULL::text AS outbox_event_public_id, oe.created_at, oe.updated_at, oe.locked_at AS started_at, oe.processed_at AS completed_at, jsonb_build_object('attempts', oe.attempts, 'maxAttempts', oe.max_attempts, 'lockedBy', oe.locked_by, 'availableAt', oe.available_at) AS metadata, oe.status IN ('pending','processing','failed') AS can_stop FROM outbox_events oe WHERE oe.tenant_id IS NOT NULL
	UNION ALL
	SELECT 'drive_ocr', r.public_id::text, r.tenant_id, 'Drive OCR: ' || COALESCE(f.original_filename, r.public_id::text), 'drive_file', f.public_id::text, r.requested_by_user_id, u.display_name, u.email, r.status, CASE WHEN r.status IN ('pending','running') THEN 'active' ELSE 'terminal' END, r.reason, COALESCE(r.error_message, r.error_code), oe.public_id::text, r.created_at, r.updated_at, r.started_at, r.completed_at, jsonb_build_object('engine', r.engine, 'structuredExtractor', r.structured_extractor, 'pageCount', r.page_count, 'processedPageCount', r.processed_page_count), r.status IN ('pending','running') FROM drive_ocr_runs r LEFT JOIN users u ON u.id = r.requested_by_user_id LEFT JOIN file_objects f ON f.id = r.file_object_id LEFT JOIN outbox_events oe ON oe.id = r.outbox_event_id
	UNION ALL
	SELECT 'data_pipeline_run', r.public_id::text, r.tenant_id, 'Data pipeline: ' || p.name, 'data_pipeline', p.public_id::text, r.requested_by_user_id, u.display_name, u.email, r.status, CASE WHEN r.status IN ('pending','processing') THEN 'active' ELSE 'terminal' END, r.trigger_kind, r.error_summary, oe.public_id::text, r.created_at, r.updated_at, r.started_at, r.completed_at, jsonb_build_object('rowCount', r.row_count, 'versionId', r.version_id, 'scheduleId', r.schedule_id), r.status IN ('pending','processing') FROM data_pipeline_runs r JOIN data_pipelines p ON p.id = r.pipeline_id LEFT JOIN users u ON u.id = r.requested_by_user_id LEFT JOIN outbox_events oe ON oe.id = r.outbox_event_id
	UNION ALL
	SELECT 'local_search_index', j.public_id::text, j.tenant_id, 'Local search index: ' || COALESCE(j.resource_kind, 'rebuild'), j.resource_kind, j.resource_public_id::text, NULL::bigint, NULL::text, NULL::text, j.status, CASE WHEN j.status IN ('queued','processing') THEN 'active' ELSE 'terminal' END, j.reason, j.last_error, oe.public_id::text, j.created_at, j.updated_at, j.started_at, j.completed_at, jsonb_build_object('attempts', j.attempts, 'indexedCount', j.indexed_count, 'skippedCount', j.skipped_count, 'failedCount', j.failed_count), j.status IN ('queued','processing') FROM local_search_index_jobs j LEFT JOIN outbox_events oe ON oe.id = j.outbox_event_id
	UNION ALL
	SELECT 'dataset_import', j.public_id::text, j.tenant_id, 'Dataset import: ' || d.name, 'dataset', d.public_id::text, j.requested_by_user_id, u.display_name, u.email, j.status, CASE WHEN j.status IN ('pending','processing') THEN 'active' ELSE 'terminal' END, 'import', j.error_summary, oe.public_id::text, j.created_at, j.updated_at, NULL::timestamptz, j.completed_at, jsonb_build_object('totalRows', j.total_rows, 'validRows', j.valid_rows, 'invalidRows', j.invalid_rows), j.status IN ('pending','processing') FROM dataset_import_jobs j JOIN datasets d ON d.id = j.dataset_id LEFT JOIN users u ON u.id = j.requested_by_user_id LEFT JOIN outbox_events oe ON oe.id = j.outbox_event_id
	UNION ALL
	SELECT 'dataset_query', j.public_id::text, j.tenant_id, 'Dataset query', 'dataset_query', j.public_id::text, j.requested_by_user_id, u.display_name, u.email, j.status, CASE WHEN j.status = 'running' THEN 'active' ELSE 'terminal' END, left(j.statement, 120), j.error_summary, NULL::text, j.created_at, j.updated_at, j.created_at, j.completed_at, jsonb_build_object('rowCount', j.row_count, 'durationMs', j.duration_ms), j.status = 'running' FROM dataset_query_jobs j LEFT JOIN users u ON u.id = j.requested_by_user_id
	UNION ALL
	SELECT 'dataset_sync', j.public_id::text, j.tenant_id, 'Dataset sync: ' || d.name, 'dataset', d.public_id::text, j.requested_by_user_id, u.display_name, u.email, j.status, CASE WHEN j.status IN ('pending','processing') THEN 'active' ELSE 'terminal' END, j.mode, COALESCE(j.error_summary, j.cleanup_error_summary), oe.public_id::text, j.created_at, j.updated_at, j.started_at, j.completed_at, jsonb_build_object('rowCount', j.row_count, 'totalBytes', j.total_bytes, 'cleanupStatus', j.cleanup_status), j.status IN ('pending','processing') FROM dataset_sync_jobs j JOIN datasets d ON d.id = j.dataset_id LEFT JOIN users u ON u.id = j.requested_by_user_id LEFT JOIN outbox_events oe ON oe.id = j.outbox_event_id
	UNION ALL
	SELECT 'dataset_work_table_export', j.public_id::text, j.tenant_id, 'Work table export: ' || wt.display_name, 'work_table', wt.public_id::text, j.requested_by_user_id, u.display_name, u.email, j.status, CASE WHEN j.status IN ('pending','processing') THEN 'active' ELSE 'terminal' END, j.format, j.error_summary, oe.public_id::text, j.created_at, j.updated_at, NULL::timestamptz, j.completed_at, jsonb_build_object('format', j.format, 'expiresAt', j.expires_at), j.status IN ('pending','processing') FROM dataset_work_table_exports j JOIN dataset_work_tables wt ON wt.id = j.work_table_id LEFT JOIN users u ON u.id = j.requested_by_user_id LEFT JOIN outbox_events oe ON oe.id = j.outbox_event_id
	UNION ALL
	SELECT 'dataset_gold_publish', j.public_id::text, j.tenant_id, 'Gold publish: ' || gp.display_name, 'gold_publication', gp.public_id::text, j.requested_by_user_id, u.display_name, u.email, j.status, CASE WHEN j.status IN ('pending','processing') THEN 'active' ELSE 'terminal' END, 'publish', j.error_summary, oe.public_id::text, j.created_at, j.updated_at, j.started_at, j.completed_at, jsonb_build_object('rowCount', j.row_count, 'totalBytes', j.total_bytes, 'goldTable', j.gold_database || '.' || j.gold_table), j.status IN ('pending','processing') FROM dataset_gold_publish_runs j JOIN dataset_gold_publications gp ON gp.id = j.publication_id LEFT JOIN users u ON u.id = j.requested_by_user_id LEFT JOIN outbox_events oe ON oe.id = j.outbox_event_id
	UNION ALL
	SELECT 'dataset_lineage_parse', j.public_id::text, j.tenant_id, 'Lineage parse', 'dataset_query', q.public_id::text, j.requested_by_user_id, u.display_name, u.email, j.status, CASE WHEN j.status = 'processing' THEN 'active' ELSE 'terminal' END, left(q.statement, 120), j.error_summary, NULL::text, j.created_at, j.created_at, j.created_at, j.completed_at, jsonb_build_object('tableRefCount', j.table_ref_count, 'columnEdgeCount', j.column_edge_count), j.status = 'processing' FROM dataset_lineage_parse_runs j JOIN dataset_query_jobs q ON q.id = j.query_job_id LEFT JOIN users u ON u.id = j.requested_by_user_id
	UNION ALL
	SELECT 'tenant_data_export', j.public_id::text, j.tenant_id, 'Tenant data export: ' || j.format, 'tenant', j.tenant_id::text, j.requested_by_user_id, u.display_name, u.email, j.status, CASE WHEN j.status IN ('pending','processing') THEN 'active' ELSE 'terminal' END, j.format, j.error_summary, oe.public_id::text, j.created_at, j.updated_at, NULL::timestamptz, j.completed_at, jsonb_build_object('format', j.format, 'expiresAt', j.expires_at), j.status IN ('pending','processing') FROM tenant_data_exports j LEFT JOIN users u ON u.id = j.requested_by_user_id LEFT JOIN outbox_events oe ON oe.id = j.outbox_event_id WHERE j.deleted_at IS NULL
	UNION ALL
	SELECT 'customer_signal_import', j.public_id::text, j.tenant_id, 'Customer signal import', 'file', f.public_id::text, j.requested_by_user_id, u.display_name, u.email, j.status, CASE WHEN j.status IN ('pending','processing') THEN 'active' ELSE 'terminal' END, CASE WHEN j.validate_only THEN 'validate' ELSE 'import' END, j.error_summary, oe.public_id::text, j.created_at, j.updated_at, NULL::timestamptz, j.completed_at, jsonb_build_object('totalRows', j.total_rows, 'validRows', j.valid_rows, 'invalidRows', j.invalid_rows, 'insertedRows', j.inserted_rows), j.status IN ('pending','processing') FROM customer_signal_import_jobs j JOIN file_objects f ON f.id = j.input_file_object_id LEFT JOIN users u ON u.id = j.requested_by_user_id LEFT JOIN outbox_events oe ON oe.id = j.outbox_event_id WHERE j.deleted_at IS NULL
	UNION ALL
	SELECT 'webhook_delivery', wd.public_id::text, wd.tenant_id, 'Webhook: ' || wd.event_type, 'webhook_endpoint', we.public_id::text, NULL::bigint, NULL::text, NULL::text, wd.status, CASE WHEN wd.status IN ('pending','failed') THEN 'active' ELSE 'terminal' END, wd.event_type, wd.last_error, oe.public_id::text, wd.created_at, wd.updated_at, NULL::timestamptz, wd.delivered_at, jsonb_build_object('attemptCount', wd.attempt_count, 'maxAttempts', wd.max_attempts, 'lastHttpStatus', wd.last_http_status, 'nextAttemptAt', wd.next_attempt_at), wd.status IN ('pending','failed') FROM webhook_deliveries wd JOIN webhook_endpoints we ON we.id = wd.webhook_endpoint_id LEFT JOIN outbox_events oe ON oe.id = wd.outbox_event_id
	UNION ALL
	SELECT 'drive_index', j.public_id::text, j.tenant_id, 'Drive index: ' || COALESCE(f.original_filename, j.public_id::text), 'drive_file', f.public_id::text, NULL::bigint, NULL::text, NULL::text, j.status, CASE WHEN j.status IN ('queued','running') THEN 'active' ELSE 'terminal' END, j.reason, j.last_error, NULL::text, j.created_at, j.updated_at, NULL::timestamptz, NULL::timestamptz, jsonb_build_object('attempts', j.attempts), j.status IN ('queued','running') FROM drive_index_jobs j LEFT JOIN file_objects f ON f.id = j.file_object_id
	UNION ALL
	SELECT 'drive_ai', j.public_id::text, j.tenant_id, 'Drive AI: ' || j.job_type, 'drive_file', f.public_id::text, j.requested_by_user_id, u.display_name, u.email, j.status, CASE WHEN j.status = 'pending' THEN 'active' ELSE 'terminal' END, j.provider, j.error_message, NULL::text, j.created_at, j.updated_at, NULL::timestamptz, CASE WHEN j.status IN ('completed','failed','denied') THEN j.updated_at ELSE NULL END, jsonb_build_object('jobType', j.job_type, 'provider', j.provider, 'fileRevision', j.file_revision), j.status = 'pending' FROM drive_ai_jobs j LEFT JOIN file_objects f ON f.id = j.file_object_id LEFT JOIN users u ON u.id = j.requested_by_user_id
	UNION ALL
	SELECT 'drive_key_rotation', j.public_id::text, j.tenant_id, 'Drive key rotation', 'drive_key', COALESCE(j.new_kms_key_id::text, j.old_kms_key_id::text), NULL::bigint, NULL::text, NULL::text, j.status, CASE WHEN j.status IN ('queued','running') THEN 'active' ELSE 'terminal' END, 'rotate_keys', j.failure_reason, NULL::text, j.created_at, j.updated_at, NULL::timestamptz, CASE WHEN j.status IN ('succeeded','failed') THEN j.updated_at ELSE NULL END, jsonb_build_object('progressCount', j.progress_count), j.status IN ('queued','running') FROM drive_key_rotation_jobs j
	UNION ALL
	SELECT 'drive_region_migration', j.public_id::text, j.tenant_id, 'Drive region migration: ' || j.source_region || ' to ' || j.target_region, 'drive_workspace', w.public_id::text, j.created_by_user_id, u.display_name, u.email, j.status, CASE WHEN j.status IN ('queued','running','requires_approval') THEN 'active' ELSE 'terminal' END, CASE WHEN j.dry_run THEN 'dry_run' ELSE 'migrate' END, NULL::text, NULL::text, j.created_at, j.updated_at, NULL::timestamptz, CASE WHEN j.status IN ('succeeded','failed','rolled_back') THEN j.updated_at ELSE NULL END, jsonb_build_object('sourceRegion', j.source_region, 'targetRegion', j.target_region, 'dryRun', j.dry_run), j.status IN ('queued','running','requires_approval') FROM drive_region_migration_jobs j LEFT JOIN drive_workspaces w ON w.id = j.workspace_id LEFT JOIN users u ON u.id = j.created_by_user_id
	UNION ALL
	SELECT 'drive_clean_room', j.public_id::text, j.tenant_id, 'Drive clean room: ' || j.job_type, 'drive_clean_room', cr.public_id::text, j.created_by_user_id, u.display_name, u.email, j.status, CASE WHEN j.status IN ('queued','running') THEN 'active' ELSE 'terminal' END, j.job_type, NULL::text, NULL::text, j.created_at, j.updated_at, NULL::timestamptz, CASE WHEN j.status IN ('ready','failed') THEN j.updated_at ELSE NULL END, j.result_metadata, j.status IN ('queued','running') FROM drive_clean_room_jobs j JOIN drive_clean_rooms cr ON cr.id = j.clean_room_id LEFT JOIN users u ON u.id = j.created_by_user_id
)`
