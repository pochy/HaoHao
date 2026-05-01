package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidFileInput  = errors.New("invalid file input")
	ErrFileNotFound      = errors.New("file not found")
	ErrFileQuotaExceeded = errors.New("file quota exceeded")
)

type FileObject struct {
	ID               int64
	PublicID         string
	TenantID         int64
	UploadedByUserID *int64
	Purpose          string
	AttachedToType   string
	AttachedToID     string
	OriginalFilename string
	ContentType      string
	ByteSize         int64
	SHA256Hex        string
	StorageDriver    string
	StorageKey       string
	Status           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
	PurgedAt         *time.Time
}

type FileUploadInput struct {
	TenantID         int64
	UserID           int64
	Purpose          string
	AttachedToType   string
	AttachedToID     string
	OriginalFilename string
	ContentType      string
	Body             io.Reader
}

type FileDownload struct {
	File FileObject
	Body FileReadCloser
}

type FilePurgeInput struct {
	Cutoff      time.Time
	BatchSize   int32
	WorkerID    string
	LockTimeout time.Duration
}

type FilePurgeResult struct {
	Claimed int64
	Purged  int64
	Failed  int64
}

type FileQuotaMetrics interface {
	IncFileQuotaExceeded(purpose string)
}

type FileService struct {
	pool             *pgxpool.Pool
	queries          *db.Queries
	storage          FileStorage
	settings         *TenantSettingsService
	audit            AuditRecorder
	maxBytes         int64
	allowedMIMETypes map[string]struct{}
	metrics          FileQuotaMetrics
}

func NewFileService(pool *pgxpool.Pool, queries *db.Queries, storage FileStorage, settings *TenantSettingsService, audit AuditRecorder, maxBytes int64, allowedMIMETypes []string, metrics FileQuotaMetrics) *FileService {
	allowed := make(map[string]struct{}, len(allowedMIMETypes))
	for _, item := range allowedMIMETypes {
		trimmed := strings.ToLower(strings.TrimSpace(item))
		if trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	if len(allowed) > 0 {
		allowed["text/csv"] = struct{}{}
	}
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024
	}
	return &FileService{
		pool:             pool,
		queries:          queries,
		storage:          storage,
		settings:         settings,
		audit:            audit,
		maxBytes:         maxBytes,
		allowedMIMETypes: allowed,
		metrics:          metrics,
	}
}

func (s *FileService) Upload(ctx context.Context, input FileUploadInput, auditCtx AuditContext) (FileObject, error) {
	return s.uploadWithLimit(ctx, input, auditCtx, s.maxBytes)
}

func (s *FileService) UploadWithMaxBytes(ctx context.Context, input FileUploadInput, auditCtx AuditContext, maxBytes int64) (FileObject, error) {
	return s.uploadWithLimit(ctx, input, auditCtx, maxBytes)
}

func (s *FileService) uploadWithLimit(ctx context.Context, input FileUploadInput, auditCtx AuditContext, maxBytes int64) (FileObject, error) {
	if s == nil || s.pool == nil || s.queries == nil || s.storage == nil {
		return FileObject{}, fmt.Errorf("file service is not configured")
	}
	if maxBytes <= 0 {
		maxBytes = s.maxBytes
	}
	normalized, err := s.normalizeUploadInput(input)
	if err != nil {
		return FileObject{}, err
	}
	if err := s.validateAttachment(ctx, normalized); err != nil {
		return FileObject{}, err
	}
	storageKey := fmt.Sprintf("tenants/%d/files/%s", normalized.TenantID, uuid.NewString())
	stored, err := s.storage.PutObject(ctx, storageKey, normalized.Body, maxBytes, ObjectPutOptions{
		ContentType: normalized.ContentType,
	})
	if err != nil {
		if errors.Is(err, ErrFileTooLarge) {
			return FileObject{}, ErrInvalidFileInput
		}
		return FileObject{}, err
	}
	if s.settings != nil {
		ok, _, _, err := s.settings.CheckFileQuota(ctx, normalized.TenantID, stored.Size)
		if err != nil {
			return FileObject{}, err
		}
		if !ok {
			_ = s.storage.Delete(ctx, stored.Key)
			if s.metrics != nil {
				s.metrics.IncFileQuotaExceeded(normalized.Purpose)
			}
			return FileObject{}, ErrFileQuotaExceeded
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return FileObject{}, fmt.Errorf("begin file transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	uploadedBy := pgtype.Int8{}
	if normalized.UserID > 0 {
		uploadedBy = pgtype.Int8{Int64: normalized.UserID, Valid: true}
	}
	row, err := qtx.CreateFileObject(ctx, db.CreateFileObjectParams{
		TenantID:         normalized.TenantID,
		UploadedByUserID: uploadedBy,
		Purpose:          normalized.Purpose,
		AttachedToType:   pgText(normalized.AttachedToType),
		AttachedToID:     pgText(normalized.AttachedToID),
		OriginalFilename: normalized.OriginalFilename,
		ContentType:      normalized.ContentType,
		ByteSize:         stored.Size,
		Sha256Hex:        stored.SHA256Hex,
		StorageDriver:    storageDriverForStoredFile(s.storage, stored),
		StorageKey:       stored.Key,
		StorageBucket:    pgText(stored.Bucket),
		Etag:             stored.ETag,
	})
	if err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return FileObject{}, fmt.Errorf("create file object: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "file.upload",
			TargetType:   "file",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"purpose":     row.Purpose,
				"contentType": row.ContentType,
				"byteSize":    row.ByteSize,
			},
		}); err != nil {
			_ = s.storage.Delete(ctx, stored.Key)
			return FileObject{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return FileObject{}, fmt.Errorf("commit file transaction: %w", err)
	}
	return fileObjectFromDB(row), nil
}

func (s *FileService) CreateGeneratedFile(ctx context.Context, tenantID int64, userID *int64, purpose, filename, contentType string, body io.Reader) (FileObject, error) {
	input := FileUploadInput{
		TenantID:         tenantID,
		Purpose:          purpose,
		OriginalFilename: filename,
		ContentType:      contentType,
		Body:             body,
	}
	if userID != nil {
		input.UserID = *userID
	}
	return s.Upload(ctx, input, AuditContext{ActorType: AuditActorSystem, TenantID: &tenantID})
}

func (s *FileService) ListForAttachment(ctx context.Context, tenantID int64, attachedToType, attachedToID string) ([]FileObject, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("file service is not configured")
	}
	rows, err := s.queries.ListFileObjectsForAttachment(ctx, db.ListFileObjectsForAttachmentParams{
		TenantID:       tenantID,
		AttachedToType: pgText(strings.ToLower(strings.TrimSpace(attachedToType))),
		AttachedToID:   pgText(strings.TrimSpace(attachedToID)),
	})
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	items := make([]FileObject, 0, len(rows))
	for _, row := range rows {
		items = append(items, fileObjectFromDB(row))
	}
	return items, nil
}

func (s *FileService) Download(ctx context.Context, tenantID int64, publicID string) (FileDownload, error) {
	file, err := s.Get(ctx, tenantID, publicID)
	if err != nil {
		return FileDownload{}, err
	}
	body, err := s.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return FileDownload{}, err
	}
	return FileDownload{File: file, Body: body}, nil
}

func (s *FileService) DownloadByID(ctx context.Context, tenantID, fileID int64) (FileDownload, error) {
	if s == nil || s.queries == nil || s.storage == nil {
		return FileDownload{}, fmt.Errorf("file service is not configured")
	}
	row, err := s.queries.GetFileObjectByIDForTenant(ctx, db.GetFileObjectByIDForTenantParams{
		ID:       fileID,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return FileDownload{}, ErrFileNotFound
	}
	if err != nil {
		return FileDownload{}, fmt.Errorf("get file object: %w", err)
	}
	file := fileObjectFromDB(row)
	body, err := s.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return FileDownload{}, err
	}
	return FileDownload{File: file, Body: body}, nil
}

func (s *FileService) Get(ctx context.Context, tenantID int64, publicID string) (FileObject, error) {
	if s == nil || s.queries == nil {
		return FileObject{}, fmt.Errorf("file service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return FileObject{}, ErrFileNotFound
	}
	row, err := s.queries.GetFileObjectForTenant(ctx, db.GetFileObjectForTenantParams{
		PublicID: parsed,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return FileObject{}, ErrFileNotFound
	}
	if err != nil {
		return FileObject{}, fmt.Errorf("get file object: %w", err)
	}
	return fileObjectFromDB(row), nil
}

func (s *FileService) Delete(ctx context.Context, tenantID int64, publicID string, auditCtx AuditContext) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("file service is not configured")
	}
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return ErrFileNotFound
	}
	row, err := s.queries.SoftDeleteFileObjectForTenant(ctx, db.SoftDeleteFileObjectForTenantParams{
		PublicID: parsed,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrFileNotFound
	}
	if err != nil {
		return fmt.Errorf("delete file object: %w", err)
	}
	if s.audit != nil {
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "file.delete",
			TargetType:   "file",
			TargetID:     row.PublicID.String(),
			Metadata: map[string]any{
				"purpose": row.Purpose,
			},
		})
	}
	return nil
}

func (s *FileService) PurgeDeletedBodies(ctx context.Context, input FilePurgeInput) (FilePurgeResult, error) {
	if s == nil || s.queries == nil || s.storage == nil {
		return FilePurgeResult{}, fmt.Errorf("file service is not configured")
	}
	if input.BatchSize <= 0 {
		input.BatchSize = 50
	}
	if input.LockTimeout <= 0 {
		input.LockTimeout = 15 * time.Minute
	}
	if input.Cutoff.IsZero() {
		return FilePurgeResult{}, fmt.Errorf("%w: purge cutoff is required", ErrInvalidFileInput)
	}

	rows, err := s.queries.ClaimDeletedFileObjectsForPurge(ctx, db.ClaimDeletedFileObjectsForPurgeParams{
		WorkerID:       strings.TrimSpace(input.WorkerID),
		Cutoff:         pgTimestamp(input.Cutoff),
		LockTimeout:    pgtype.Interval{Microseconds: input.LockTimeout.Microseconds(), Valid: true},
		BatchSize:      input.BatchSize,
		StorageDrivers: []string{FileStorageDriverName(s.storage)},
	})
	if err != nil {
		return FilePurgeResult{}, fmt.Errorf("claim deleted file objects: %w", err)
	}

	result := FilePurgeResult{Claimed: int64(len(rows))}
	for _, row := range rows {
		if err := s.purgeDeletedBody(ctx, row); err != nil {
			result.Failed++
			continue
		}
		result.Purged++
	}
	return result, nil
}

func (s *FileService) purgeDeletedBody(ctx context.Context, row db.FileObject) error {
	if row.StorageDriver != FileStorageDriverName(s.storage) {
		err := fmt.Errorf("unsupported storage driver for purge")
		_, _ = s.queries.MarkFileObjectPurgeFailed(ctx, db.MarkFileObjectPurgeFailedParams{
			ID:        row.ID,
			LastError: err.Error(),
		})
		return err
	}

	if err := s.storage.Delete(ctx, row.StorageKey); err != nil {
		_, _ = s.queries.MarkFileObjectPurgeFailed(ctx, db.MarkFileObjectPurgeFailedParams{
			ID:        row.ID,
			LastError: err.Error(),
		})
		return err
	}

	purged, err := s.queries.MarkFileObjectBodyPurged(ctx, row.ID)
	if err != nil {
		return fmt.Errorf("mark file body purged: %w", err)
	}

	if s.audit != nil {
		tenantID := purged.TenantID
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: AuditContext{
				ActorType: AuditActorSystem,
				TenantID:  &tenantID,
			},
			Action:     "file.purge",
			TargetType: "file",
			TargetID:   purged.PublicID.String(),
			Metadata: map[string]any{
				"purpose":       purged.Purpose,
				"contentType":   purged.ContentType,
				"byteSize":      purged.ByteSize,
				"storageDriver": purged.StorageDriver,
			},
		})
	}

	return nil
}

func (s *FileService) normalizeUploadInput(input FileUploadInput) (FileUploadInput, error) {
	input.Purpose = strings.ToLower(strings.TrimSpace(input.Purpose))
	if input.Purpose == "" {
		input.Purpose = "attachment"
	}
	if input.Purpose == "drive" {
		return FileUploadInput{}, fmt.Errorf("%w: drive files must use drive service", ErrInvalidFileInput)
	}
	input.AttachedToType = strings.ToLower(strings.TrimSpace(input.AttachedToType))
	input.AttachedToID = strings.TrimSpace(input.AttachedToID)
	input.OriginalFilename = filepath.Base(strings.TrimSpace(input.OriginalFilename))
	input.ContentType = strings.ToLower(strings.TrimSpace(strings.Split(input.ContentType, ";")[0]))
	if input.ContentType == "" || input.ContentType == "application/octet-stream" {
		input.ContentType = "application/octet-stream"
	}
	if input.TenantID <= 0 || input.Body == nil || input.OriginalFilename == "" || input.OriginalFilename == "." {
		return FileUploadInput{}, fmt.Errorf("%w: tenant, body, and filename are required", ErrInvalidFileInput)
	}
	if input.UserID < 0 {
		return FileUploadInput{}, fmt.Errorf("%w: invalid user", ErrInvalidFileInput)
	}
	if len(s.allowedMIMETypes) > 0 {
		if _, ok := s.allowedMIMETypes[input.ContentType]; !ok {
			return FileUploadInput{}, fmt.Errorf("%w: unsupported content type", ErrInvalidFileInput)
		}
	}
	return input, nil
}

func (s *FileService) validateAttachment(ctx context.Context, input FileUploadInput) error {
	if input.AttachedToType == "" && input.AttachedToID == "" {
		return nil
	}
	if input.AttachedToType == "" || input.AttachedToID == "" {
		return fmt.Errorf("%w: attachment target is incomplete", ErrInvalidFileInput)
	}
	switch input.AttachedToType {
	case "customer_signal":
		parsed, err := uuid.Parse(input.AttachedToID)
		if err != nil {
			return fmt.Errorf("%w: invalid customer signal", ErrInvalidFileInput)
		}
		if _, err := s.queries.GetCustomerSignalByPublicIDForTenant(ctx, db.GetCustomerSignalByPublicIDForTenantParams{
			PublicID: parsed,
			TenantID: input.TenantID,
		}); errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: attachment target not found", ErrInvalidFileInput)
		} else if err != nil {
			return fmt.Errorf("validate attachment target: %w", err)
		}
	default:
		return fmt.Errorf("%w: unsupported attachment target", ErrInvalidFileInput)
	}
	return nil
}

func fileObjectFromDB(row db.FileObject) FileObject {
	return FileObject{
		ID:               row.ID,
		PublicID:         row.PublicID.String(),
		TenantID:         row.TenantID,
		UploadedByUserID: optionalPgInt8(row.UploadedByUserID),
		Purpose:          row.Purpose,
		AttachedToType:   optionalText(row.AttachedToType),
		AttachedToID:     optionalText(row.AttachedToID),
		OriginalFilename: row.OriginalFilename,
		ContentType:      row.ContentType,
		ByteSize:         row.ByteSize,
		SHA256Hex:        row.Sha256Hex,
		StorageDriver:    row.StorageDriver,
		StorageKey:       row.StorageKey,
		Status:           row.Status,
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
		DeletedAt:        optionalPgTime(row.DeletedAt),
		PurgedAt:         optionalPgTime(row.PurgedAt),
	}
}

func optionalText(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func optionalPgTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}

func DetectContentType(sample []byte) string {
	return strings.ToLower(http.DetectContentType(sample))
}

func sortedMIMETypes(values map[string]struct{}) []string {
	items := make([]string, 0, len(values))
	for item := range values {
		items = append(items, item)
	}
	sort.Strings(items)
	return items
}
