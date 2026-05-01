package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	db "example.com/haohao/backend/internal/db"
	"example.com/haohao/backend/internal/service"

	"github.com/jackc/pgx/v5/pgtype"
)

type dataLifecycleQueries interface {
	DeleteExpiredIdempotencyKeys(context.Context) (int64, error)
	ExpireTenantInvitations(context.Context) (int64, error)
	DeleteProcessedOutboxEventsBefore(context.Context, pgtype.Timestamptz) (int64, error)
	DeleteReadNotificationsBefore(context.Context, pgtype.Timestamptz) (int64, error)
	SoftDeleteExpiredTenantDataExports(context.Context) (int64, error)
}

var _ dataLifecycleQueries = (*db.Queries)(nil)

type DataLifecycleMetrics interface {
	IncDataLifecycleRun(trigger string, err error)
	IncDataLifecycleItems(kind string, count int64)
}

type DeletedFilePurger interface {
	PurgeDeletedBodies(ctx context.Context, input service.FilePurgeInput) (service.FilePurgeResult, error)
}

type DataLifecycleConfig struct {
	Enabled               bool
	Interval              time.Duration
	Timeout               time.Duration
	RunOnStartup          bool
	OutboxRetention       time.Duration
	NotificationRetention time.Duration
	FileDeletedRetention  time.Duration
	FilePurgeBatchSize    int
	FilePurgeLockTimeout  time.Duration
	WorkerID              string
}

type DataLifecycleJob struct {
	queries    dataLifecycleQueries
	filePurger DeletedFilePurger
	config     DataLifecycleConfig
	logger     *slog.Logger
	metrics    DataLifecycleMetrics
	running    atomic.Bool
}

type dataLifecycleRunSummary struct {
	ExpiredIDKeysDeleted     int64
	TenantInvitationsExpired int64
	ProcessedOutboxDeleted   int64
	ReadNotificationsDeleted int64
	TenantDataExportsExpired int64
	FileBodiesClaimed        int64
	FileBodiesPurged         int64
	FileBodyPurgeFailed      int64
}

func (s dataLifecycleRunSummary) changed() bool {
	return s.ExpiredIDKeysDeleted > 0 ||
		s.TenantInvitationsExpired > 0 ||
		s.ProcessedOutboxDeleted > 0 ||
		s.ReadNotificationsDeleted > 0 ||
		s.TenantDataExportsExpired > 0 ||
		s.FileBodiesClaimed > 0 ||
		s.FileBodiesPurged > 0 ||
		s.FileBodyPurgeFailed > 0
}

func (s dataLifecycleRunSummary) attrs() []any {
	return []any{
		"expired_idempotency_keys_deleted", s.ExpiredIDKeysDeleted,
		"tenant_invitations_expired", s.TenantInvitationsExpired,
		"processed_outbox_events_deleted", s.ProcessedOutboxDeleted,
		"read_notifications_deleted", s.ReadNotificationsDeleted,
		"tenant_data_exports_expired", s.TenantDataExportsExpired,
		"file_bodies_claimed", s.FileBodiesClaimed,
		"file_bodies_purged", s.FileBodiesPurged,
		"file_body_purge_failed", s.FileBodyPurgeFailed,
	}
}

func NewDataLifecycleJob(queries *db.Queries, filePurger DeletedFilePurger, config DataLifecycleConfig, logger *slog.Logger, metrics DataLifecycleMetrics) *DataLifecycleJob {
	if logger == nil {
		logger = slog.Default()
	}
	config = normalizeDataLifecycleConfig(config)
	var lifecycleQueries dataLifecycleQueries
	if queries != nil {
		lifecycleQueries = queries
	}
	return &DataLifecycleJob{
		queries:    lifecycleQueries,
		filePurger: filePurger,
		config:     config,
		logger:     logger,
		metrics:    metrics,
	}
}

func (j *DataLifecycleJob) Start(ctx context.Context) {
	if j == nil || j.queries == nil || !j.config.Enabled {
		return
	}
	if j.config.Interval <= 0 || j.config.Timeout <= 0 {
		j.logger.ErrorContext(ctx, "data lifecycle job disabled because interval or timeout is not positive")
		return
	}
	if j.config.RunOnStartup {
		j.runOnce(ctx, "startup")
	}

	ticker := time.NewTicker(j.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.runOnce(ctx, "interval")
		}
	}
}

func (j *DataLifecycleJob) runOnce(parent context.Context, trigger string) {
	if !j.running.CompareAndSwap(false, true) {
		j.logger.DebugContext(parent, "data lifecycle job skipped because previous run is still active", "trigger", trigger)
		return
	}
	defer j.running.Store(false)

	ctx, cancel := context.WithTimeout(parent, j.config.Timeout)
	defer cancel()

	summary, err := j.runOnceWithSummary(ctx)
	if j.metrics != nil {
		j.metrics.IncDataLifecycleRun(trigger, err)
	}
	if err != nil {
		j.logger.ErrorContext(ctx, "data lifecycle job failed", "trigger", trigger, "error", err.Error())
		return
	}
	if trigger == "startup" || summary.changed() {
		attrs := append([]any{"trigger", trigger}, summary.attrs()...)
		j.logger.InfoContext(ctx, "data lifecycle job completed", attrs...)
		return
	}
	j.logger.DebugContext(ctx, "data lifecycle job completed", "trigger", trigger)
}

func (j *DataLifecycleJob) RunOnce(ctx context.Context) error {
	_, err := j.runOnceWithSummary(ctx)
	return err
}

func (j *DataLifecycleJob) runOnceWithSummary(ctx context.Context) (dataLifecycleRunSummary, error) {
	var summary dataLifecycleRunSummary
	if j == nil || j.queries == nil {
		return summary, nil
	}
	now := time.Now()

	expiredIDKeys, err := j.queries.DeleteExpiredIdempotencyKeys(ctx)
	if err != nil {
		return summary, fmt.Errorf("delete expired idempotency keys: %w", err)
	}
	summary.ExpiredIDKeysDeleted = expiredIDKeys
	j.addItems("idempotency_keys", expiredIDKeys)

	expiredInvitations, err := j.queries.ExpireTenantInvitations(ctx)
	if err != nil {
		return summary, fmt.Errorf("expire tenant invitations: %w", err)
	}
	summary.TenantInvitationsExpired = expiredInvitations
	j.addItems("tenant_invitations", expiredInvitations)

	outboxBefore := now.Add(-j.config.OutboxRetention)
	outboxDeleted, err := j.queries.DeleteProcessedOutboxEventsBefore(ctx, pgtype.Timestamptz{Time: outboxBefore, Valid: true})
	if err != nil {
		return summary, fmt.Errorf("delete processed outbox events: %w", err)
	}
	summary.ProcessedOutboxDeleted = outboxDeleted
	j.addItems("outbox_events", outboxDeleted)

	notificationBefore := now.Add(-j.config.NotificationRetention)
	notificationDeleted, err := j.queries.DeleteReadNotificationsBefore(ctx, pgtype.Timestamptz{Time: notificationBefore, Valid: true})
	if err != nil {
		return summary, fmt.Errorf("delete read notifications: %w", err)
	}
	summary.ReadNotificationsDeleted = notificationDeleted
	j.addItems("notifications", notificationDeleted)

	fileBefore := now.Add(-j.config.FileDeletedRetention)
	if j.filePurger != nil {
		result, err := j.filePurger.PurgeDeletedBodies(ctx, service.FilePurgeInput{
			Cutoff:      fileBefore,
			BatchSize:   int32(j.config.FilePurgeBatchSize),
			WorkerID:    j.config.WorkerID,
			LockTimeout: j.config.FilePurgeLockTimeout,
		})
		if err != nil {
			return summary, fmt.Errorf("purge deleted file bodies: %w", err)
		}
		summary.FileBodiesClaimed = result.Claimed
		summary.FileBodiesPurged = result.Purged
		summary.FileBodyPurgeFailed = result.Failed
		j.logger.DebugContext(ctx, "file body purge completed", "claimed", result.Claimed, "purged", result.Purged, "failed", result.Failed)
		j.addItems("file_objects_body_purged", result.Purged)
		j.addItems("file_objects_body_purge_failed", result.Failed)
	}

	expiredExports, err := j.queries.SoftDeleteExpiredTenantDataExports(ctx)
	if err != nil {
		return summary, fmt.Errorf("soft delete expired tenant data exports: %w", err)
	}
	summary.TenantDataExportsExpired = expiredExports
	j.addItems("tenant_data_exports", expiredExports)

	return summary, nil
}

func (j *DataLifecycleJob) addItems(kind string, count int64) {
	if j.metrics != nil {
		j.metrics.IncDataLifecycleItems(kind, count)
	}
}

func normalizeDataLifecycleConfig(config DataLifecycleConfig) DataLifecycleConfig {
	if config.FilePurgeBatchSize <= 0 {
		config.FilePurgeBatchSize = 50
	}
	if config.FilePurgeLockTimeout <= 0 {
		config.FilePurgeLockTimeout = 15 * time.Minute
	}
	config.WorkerID = strings.TrimSpace(config.WorkerID)
	if config.WorkerID == "" {
		hostname, err := os.Hostname()
		if err != nil || strings.TrimSpace(hostname) == "" {
			hostname = "unknown-host"
		}
		config.WorkerID = fmt.Sprintf("%s:%d", strings.TrimSpace(hostname), os.Getpid())
	}
	return config
}
