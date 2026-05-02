package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	driveEditSessionTTL       = 15 * time.Minute
	driveSyncDeviceTokenBytes = 32
)

type DriveSearchResult struct {
	Item      DriveItem
	Snippet   string
	IndexedAt *time.Time
}

type DriveIndexRebuildResult struct {
	Indexed int
	Skipped int
	Failed  int
}

type DriveEditSession struct {
	PublicID     string
	FilePublicID string
	ActorUserID  int64
	Status       string
	BaseRevision int64
	Provider     string
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

type DriveSaveEditInput struct {
	TenantID         int64
	ActorUserID      int64
	FilePublicID     string
	SessionPublicID  string
	Content          string
	ExpectedRevision int64
	Filename         string
	ContentType      string
}

type DriveSaveEditResult struct {
	File     DriveFile
	Revision int64
	Conflict bool
}

type DriveSyncDevice struct {
	PublicID           string
	TenantID           int64
	UserID             int64
	DeviceName         string
	Platform           string
	Status             string
	RemoteWipeRequired bool
	LastSeenAt         *time.Time
	CreatedAt          time.Time
	RawToken           string
}

type DriveRegisterDeviceInput struct {
	TenantID    int64
	ActorUserID int64
	DeviceName  string
	Platform    string
	RemoteAddr  string
	UserAgent   string
}

type DriveSyncEvent struct {
	ID               int64
	PublicID         string
	ResourceType     string
	ResourcePublicID string
	Action           string
	ObjectVersion    string
	CreatedAt        time.Time
	Metadata         map[string]any
}

type DriveSyncDelta struct {
	Cursor      string
	Events      []DriveSyncEvent
	RemoteWipe  bool
	FullResync  bool
	DeniedCount int
}

type DriveOfflineOperationInput struct {
	OperationID      string
	OperationType    string
	ResourceType     string
	ResourcePublicID string
	BaseRevision     int64
	Name             string
}

type DriveOfflineReplayResult struct {
	Applied    int
	Denied     int
	Conflicted int
}

type DriveEncryptionPolicy struct {
	Mode         string
	Scope        string
	KeyPublicID  string
	Provider     string
	MaskedKeyRef string
	KeyStatus    string
	UpdatedAt    time.Time
}

type DriveResidencyPolicy struct {
	PrimaryRegion   string
	AllowedRegions  []string
	ReplicationMode string
	IndexRegion     string
	BackupRegion    string
	Status          string
	UpdatedAt       time.Time
}

type DriveLegalCase struct {
	PublicID    string
	Name        string
	Description string
	Status      string
	CreatedAt   time.Time
}

type DriveLegalExport struct {
	PublicID     string
	CasePublicID string
	Status       string
	CreatedAt    time.Time
}

type DriveCleanRoom struct {
	PublicID  string
	Name      string
	Status    string
	CreatedAt time.Time
}

type DriveCleanRoomDataset struct {
	PublicID           string
	CleanRoomPublicID  string
	SourceFilePublicID string
	Status             string
	CreatedAt          time.Time
}

type DriveCleanRoomExport struct {
	PublicID     string
	Status       string
	DeniedReason string
	CreatedAt    time.Time
}

func (s *DriveService) SearchDocuments(ctx context.Context, input DriveSearchInput, auditCtx AuditContext) ([]DriveSearchResult, error) {
	items, err := s.Search(ctx, input, auditCtx)
	if err != nil {
		return nil, err
	}
	results := make([]DriveSearchResult, 0, len(items))
	for _, item := range items {
		result := DriveSearchResult{Item: item}
		if item.File != nil {
			var snippet string
			var indexedAt pgtype.Timestamptz
			err := s.pool.QueryRow(ctx, `
				SELECT COALESCE(snippet, ''), indexed_at
				FROM drive_search_documents
				WHERE tenant_id = $1 AND file_object_id = $2
			`, input.TenantID, item.File.ID).Scan(&snippet, &indexedAt)
			if err == nil {
				result.Snippet = snippet
				result.IndexedAt = optionalPgTime(indexedAt)
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("get drive search snippet: %w", err)
			}
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *DriveService) RebuildDriveSearchIndex(ctx context.Context, tenantID, actorUserID int64, auditCtx AuditContext) (DriveIndexRebuildResult, error) {
	if err := s.ensureConfigured(true); err != nil {
		return DriveIndexRebuildResult{}, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveIndexRebuildResult{}, err
	}
	policy, err := s.drivePolicy(ctx, tenantID)
	if err != nil {
		return DriveIndexRebuildResult{}, err
	}
	if !policy.SearchEnabled {
		return DriveIndexRebuildResult{}, ErrDrivePolicyDenied
	}
	rows, err := s.pool.Query(ctx, `
		SELECT public_id::text
		FROM file_objects
		WHERE tenant_id = $1
		  AND purpose = 'drive'
		  AND deleted_at IS NULL
		ORDER BY id
		LIMIT 1000
	`, tenantID)
	if err != nil {
		return DriveIndexRebuildResult{}, fmt.Errorf("list drive files for index rebuild: %w", err)
	}
	defer rows.Close()
	var result DriveIndexRebuildResult
	for rows.Next() {
		var publicID string
		if err := rows.Scan(&publicID); err != nil {
			return DriveIndexRebuildResult{}, err
		}
		row, err := s.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: publicID})
		if err != nil {
			result.Failed++
			continue
		}
		file := driveFileFromDB(row)
		outcome, err := s.indexDriveFile(ctx, file, "rebuild")
		if err != nil {
			result.Failed++
			continue
		}
		if outcome == "skipped" {
			result.Skipped++
		} else {
			result.Indexed++
		}
	}
	if err := rows.Err(); err != nil {
		return DriveIndexRebuildResult{}, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.search.index_rebuild", "tenant", fmt.Sprintf("%d", tenantID), map[string]any{
		"indexed": result.Indexed,
		"skipped": result.Skipped,
		"failed":  result.Failed,
	})
	return result, nil
}

func (s *DriveService) indexDriveFileBestEffort(ctx context.Context, file DriveFile, reason string) {
	if s == nil || s.queries == nil || s.storage == nil {
		return
	}
	_, _ = s.indexDriveFile(ctx, file, reason)
}

func (s *DriveService) indexDriveFile(ctx context.Context, file DriveFile, reason string) (string, error) {
	if file.ID <= 0 || file.TenantID <= 0 {
		return "failed", ErrDriveInvalidInput
	}
	if file.DeletedAt != nil || file.DLPBlocked || file.ScanStatus == "infected" || file.ScanStatus == "blocked" || file.ScanStatus == "pending" || file.ScanStatus == "failed" {
		if err := s.queries.DeleteDriveSearchDocument(ctx, db.DeleteDriveSearchDocumentParams{TenantID: file.TenantID, FileObjectID: file.ID}); err != nil {
			return "failed", fmt.Errorf("delete drive search document: %w", err)
		}
		_, _ = s.queries.CreateDriveIndexJob(ctx, db.CreateDriveIndexJobParams{
			TenantID:     file.TenantID,
			FileObjectID: file.ID,
			Reason:       reason,
			Status:       "skipped",
		})
		return "skipped", nil
	}
	text := ""
	if driveSearchCanExtract(file) {
		body, err := s.storage.Open(ctx, file.StorageKey)
		if err == nil {
			data, readErr := io.ReadAll(io.LimitReader(body, 256*1024))
			_ = body.Close()
			if readErr == nil {
				text = sanitizeSearchText(string(data))
			}
		}
	}
	snippet := searchSnippet(text, file.OriginalFilename)
	_, err := s.queries.UpsertDriveSearchDocument(ctx, db.UpsertDriveSearchDocumentParams{
		TenantID:        file.TenantID,
		WorkspaceID:     pgInt8(file.WorkspaceID),
		FileObjectID:    file.ID,
		Title:           file.OriginalFilename,
		ContentType:     file.ContentType,
		ExtractedText:   text,
		Snippet:         snippet,
		ContentSha256:   pgText(file.SHA256Hex),
		ObjectUpdatedAt: pgtype.Timestamptz{Time: file.UpdatedAt, Valid: !file.UpdatedAt.IsZero()},
	})
	if err != nil {
		_, _ = s.queries.CreateDriveIndexJob(ctx, db.CreateDriveIndexJobParams{
			TenantID:     file.TenantID,
			FileObjectID: file.ID,
			Reason:       reason,
			Status:       "failed",
		})
		return "failed", fmt.Errorf("upsert drive search document: %w", err)
	}
	_, _ = s.queries.CreateDriveIndexJob(ctx, db.CreateDriveIndexJobParams{
		TenantID:     file.TenantID,
		FileObjectID: file.ID,
		Reason:       reason,
		Status:       "succeeded",
	})
	return "indexed", nil
}

func driveSearchCanExtract(file DriveFile) bool {
	contentType := strings.ToLower(strings.TrimSpace(file.ContentType))
	if strings.HasPrefix(contentType, "text/") || strings.Contains(contentType, "json") || strings.Contains(contentType, "xml") {
		return true
	}
	switch strings.ToLower(filepath.Ext(file.OriginalFilename)) {
	case ".txt", ".md", ".csv", ".json", ".log", ".xml":
		return true
	default:
		return false
	}
}

func sanitizeSearchText(value string) string {
	value = strings.ToValidUTF8(value, " ")
	value = strings.ReplaceAll(value, "\x00", " ")
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	if len(fields) > 2000 {
		fields = fields[:2000]
	}
	return strings.Join(fields, " ")
}

func searchSnippet(text, fallback string) string {
	text = sanitizeSearchText(text)
	if text == "" {
		return truncateRunes(strings.ToValidUTF8(fallback, " "), 240)
	}
	return truncateRunes(text, 240)
}

func (s *DriveService) StartEditSession(ctx context.Context, tenantID, actorUserID int64, filePublicID string, auditCtx AuditContext) (DriveEditSession, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveEditSession{}, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveEditSession{}, err
	}
	policy, err := s.drivePolicy(ctx, tenantID)
	if err != nil {
		return DriveEditSession{}, err
	}
	if !policy.CollaborationEnabled {
		return DriveEditSession{}, ErrDrivePolicyDenied
	}
	row, err := s.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID})
	if err != nil {
		return DriveEditSession{}, err
	}
	file := driveFileFromDB(row)
	if err := s.ensureFileMutationAllowed(ctx, tenantID, file.ID); err != nil {
		return DriveEditSession{}, err
	}
	if err := s.authz.CanEditFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.file.edit_session.start", "drive_file", file.PublicID, err, auditCtx)
		return DriveEditSession{}, err
	}
	_, _ = s.pool.Exec(ctx, `DELETE FROM drive_edit_locks WHERE file_object_id = $1 AND expires_at <= now()`, file.ID)
	baseRevision, err := s.currentDriveRevision(ctx, tenantID, file.ID)
	if err != nil {
		return DriveEditSession{}, err
	}
	expiresAt := s.now().Add(driveEditSessionTTL).UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DriveEditSession{}, fmt.Errorf("begin drive edit session: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var session DriveEditSession
	var publicID uuid.UUID
	var createdAt pgtype.Timestamptz
	err = tx.QueryRow(ctx, `
		INSERT INTO drive_edit_sessions (tenant_id, file_object_id, actor_user_id, base_revision, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING public_id, status, provider, base_revision, expires_at, created_at
	`, tenantID, file.ID, actor.UserID, baseRevision, expiresAt).Scan(&publicID, &session.Status, &session.Provider, &session.BaseRevision, &session.ExpiresAt, &createdAt)
	if err != nil {
		return DriveEditSession{}, fmt.Errorf("create drive edit session: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO drive_edit_locks (tenant_id, file_object_id, actor_user_id, session_id, base_revision, expires_at)
		SELECT $1, $2, $3, id, $4, $5
		FROM drive_edit_sessions
		WHERE public_id = $6
	`, tenantID, file.ID, actor.UserID, baseRevision, expiresAt, publicID); err != nil {
		if isUniqueViolation(err) {
			return DriveEditSession{}, ErrDriveLocked
		}
		return DriveEditSession{}, fmt.Errorf("create drive edit lock: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return DriveEditSession{}, fmt.Errorf("commit drive edit session: %w", err)
	}
	session.PublicID = publicID.String()
	session.FilePublicID = file.PublicID
	session.ActorUserID = actor.UserID
	session.CreatedAt = createdAt.Time
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.edit_session.started", "drive_file", file.PublicID, map[string]any{
		"sessionPublicId": session.PublicID,
		"baseRevision":    baseRevision,
	})
	return session, nil
}

func (s *DriveService) HeartbeatEditSession(ctx context.Context, tenantID, actorUserID int64, filePublicID, sessionPublicID string, auditCtx AuditContext) (DriveEditSession, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveEditSession{}, err
	}
	file, session, err := s.editSessionFile(ctx, tenantID, actor.UserID, filePublicID, sessionPublicID)
	if err != nil {
		return DriveEditSession{}, err
	}
	expiresAt := s.now().Add(driveEditSessionTTL).UTC()
	_, err = s.pool.Exec(ctx, `
		UPDATE drive_edit_sessions SET expires_at = $1, updated_at = now()
		WHERE public_id = $2 AND tenant_id = $3 AND actor_user_id = $4 AND status = 'active';
		UPDATE drive_edit_locks SET expires_at = $1, last_heartbeat_at = now(), updated_at = now()
		WHERE session_id = (SELECT id FROM drive_edit_sessions WHERE public_id = $2 AND tenant_id = $3) AND actor_user_id = $4;
	`, expiresAt, sessionPublicID, tenantID, actor.UserID)
	if err != nil {
		return DriveEditSession{}, fmt.Errorf("heartbeat drive edit session: %w", err)
	}
	session.ExpiresAt = expiresAt
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.edit_session.heartbeat", "drive_file", file.PublicID, map[string]any{"sessionPublicId": session.PublicID})
	return session, nil
}

func (s *DriveService) SaveEditSessionContent(ctx context.Context, input DriveSaveEditInput, auditCtx AuditContext) (DriveSaveEditResult, error) {
	if err := s.ensureConfigured(true); err != nil {
		return DriveSaveEditResult{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveSaveEditResult{}, err
	}
	policy, err := s.drivePolicy(ctx, input.TenantID)
	if err != nil {
		return DriveSaveEditResult{}, err
	}
	if !policy.CollaborationEnabled {
		return DriveSaveEditResult{}, ErrDrivePolicyDenied
	}
	file, session, err := s.editSessionFile(ctx, input.TenantID, actor.UserID, input.FilePublicID, input.SessionPublicID)
	if err != nil {
		return DriveSaveEditResult{}, err
	}
	if err := s.ensureFileMutationAllowed(ctx, input.TenantID, file.ID); err != nil {
		return DriveSaveEditResult{}, err
	}
	if err := s.authz.CanEditFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.file.revision.save", "drive_file", file.PublicID, err, auditCtx)
		return DriveSaveEditResult{}, err
	}
	currentRevision, err := s.currentDriveRevision(ctx, input.TenantID, file.ID)
	if err != nil {
		return DriveSaveEditResult{}, err
	}
	if input.ExpectedRevision >= 0 && input.ExpectedRevision != currentRevision {
		_, _ = s.pool.Exec(ctx, `UPDATE drive_edit_sessions SET status = 'conflicted', updated_at = now() WHERE public_id = $1 AND tenant_id = $2`, input.SessionPublicID, input.TenantID)
		s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.conflict.detected", "drive_file", file.PublicID, map[string]any{
			"expectedRevision": input.ExpectedRevision,
			"currentRevision":  currentRevision,
		})
		return DriveSaveEditResult{File: file, Revision: currentRevision, Conflict: true}, ErrDriveLocked
	}
	filename := normalizeDriveName(filepath.Base(strings.TrimSpace(input.Filename)))
	if filename == "" || filename == "." {
		filename = file.OriginalFilename
	}
	contentType := normalizeContentType(input.ContentType)
	if strings.TrimSpace(input.ContentType) == "" {
		contentType = "text/plain"
	}
	storageKey := newDriveStorageKey(input.TenantID, file.WorkspacePublicID, 1)
	stored, err := s.storage.PutObject(ctx, storageKey, strings.NewReader(input.Content), policy.MaxFileSizeBytes, ObjectPutOptions{
		ContentType: contentType,
	})
	if err != nil {
		if errors.Is(err, ErrFileTooLarge) {
			return DriveSaveEditResult{}, ErrInvalidFileInput
		}
		return DriveSaveEditResult{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveSaveEditResult{}, fmt.Errorf("begin drive edit save: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO drive_file_revisions (
			tenant_id, file_object_id, created_by_user_id, actor_type,
			previous_original_filename, previous_content_type, previous_byte_size,
			previous_sha256_hex, previous_storage_driver, previous_storage_key, reason
		) VALUES ($1, $2, $3, 'user', $4, $5, $6, $7, $8, $9, 'collaboration_save')
	`, input.TenantID, file.ID, actor.UserID, file.OriginalFilename, file.ContentType, file.ByteSize, file.SHA256Hex, file.StorageDriver, file.StorageKey); err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveSaveEditResult{}, fmt.Errorf("create drive edit revision: %w", err)
	}
	qtx := s.queries.WithTx(tx)
	updated, err := qtx.UpdateDriveFileObjectMetadata(ctx, db.UpdateDriveFileObjectMetadataParams{
		ContentType:   contentType,
		ByteSize:      stored.Size,
		Sha256Hex:     stored.SHA256Hex,
		StorageDriver: storageDriverForStoredFile(s.storage, stored),
		StorageKey:    stored.Key,
		StorageBucket: pgText(stored.Bucket),
		Etag:          stored.ETag,
		ScanStatus:    driveInitialScanStatus(policy),
		ID:            file.ID,
		TenantID:      input.TenantID,
	})
	if err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveSaveEditResult{}, fmt.Errorf("update drive edit content: %w", err)
	}
	if filename != file.OriginalFilename {
		if renamed, renameErr := qtx.RenameDriveFile(ctx, db.RenameDriveFileParams{OriginalFilename: filename, ID: file.ID, TenantID: input.TenantID}); renameErr == nil {
			updated = renamed
		}
	}
	if _, err := tx.Exec(ctx, `
		WITH ended AS (
			UPDATE drive_edit_sessions
			SET status = 'ended', ended_at = now(), updated_at = now()
			WHERE public_id = $1 AND tenant_id = $2 AND actor_user_id = $3
			RETURNING id
		)
		DELETE FROM drive_edit_locks
		WHERE session_id IN (SELECT id FROM ended)
	`, session.PublicID, input.TenantID, actor.UserID); err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveSaveEditResult{}, fmt.Errorf("close drive edit lock: %w", err)
	}
	result := driveFileFromDB(updated)
	if err := s.recordAuditWithQueries(ctx, qtx, auditCtx, "drive.file.revision.created", "drive_file", result.PublicID, map[string]any{
		"sessionPublicId": session.PublicID,
		"revision":        currentRevision + 1,
	}); err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveSaveEditResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveSaveEditResult{}, fmt.Errorf("commit drive edit save: %w", err)
	}
	_ = s.storage.Delete(ctx, file.StorageKey)
	s.indexDriveFileBestEffort(ctx, result, "collaboration_save")
	s.recordDriveSyncEventBestEffort(ctx, result.ResourceRef(), "file.updated", result.SHA256Hex, map[string]any{"source": "collaboration"})
	return DriveSaveEditResult{File: result, Revision: currentRevision + 1}, nil
}

func (s *DriveService) EndEditSession(ctx context.Context, tenantID, actorUserID int64, filePublicID, sessionPublicID string, auditCtx AuditContext) error {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	file, session, err := s.editSessionFile(ctx, tenantID, actor.UserID, filePublicID, sessionPublicID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE drive_edit_sessions SET status = 'ended', ended_at = now(), updated_at = now()
		WHERE public_id = $1 AND tenant_id = $2 AND actor_user_id = $3;
		DELETE FROM drive_edit_locks
		WHERE session_id = (SELECT id FROM drive_edit_sessions WHERE public_id = $1 AND tenant_id = $2)
	`, session.PublicID, tenantID, actor.UserID)
	if err != nil {
		return fmt.Errorf("end drive edit session: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.edit_session.ended", "drive_file", file.PublicID, map[string]any{"sessionPublicId": session.PublicID})
	return nil
}

func (s *DriveService) editSessionFile(ctx context.Context, tenantID, actorUserID int64, filePublicID, sessionPublicID string) (DriveFile, DriveEditSession, error) {
	row, err := s.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID})
	if err != nil {
		return DriveFile{}, DriveEditSession{}, err
	}
	file := driveFileFromDB(row)
	var session DriveEditSession
	var publicID uuid.UUID
	var expiresAt pgtype.Timestamptz
	var createdAt pgtype.Timestamptz
	err = s.pool.QueryRow(ctx, `
		SELECT s.public_id, s.status, s.provider, s.base_revision, s.expires_at, s.created_at
		FROM drive_edit_sessions s
		JOIN drive_edit_locks l ON l.session_id = s.id
		WHERE s.public_id = $1
		  AND s.tenant_id = $2
		  AND s.file_object_id = $3
		  AND s.actor_user_id = $4
		  AND s.status = 'active'
		  AND s.expires_at > now()
		  AND l.expires_at > now()
	`, sessionPublicID, tenantID, file.ID, actorUserID).Scan(&publicID, &session.Status, &session.Provider, &session.BaseRevision, &expiresAt, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveFile{}, DriveEditSession{}, ErrDrivePermissionDenied
	}
	if err != nil {
		return DriveFile{}, DriveEditSession{}, fmt.Errorf("get drive edit session: %w", err)
	}
	session.PublicID = publicID.String()
	session.FilePublicID = file.PublicID
	session.ActorUserID = actorUserID
	session.ExpiresAt = expiresAt.Time
	session.CreatedAt = createdAt.Time
	return file, session, nil
}

func (s *DriveService) currentDriveRevision(ctx context.Context, tenantID, fileObjectID int64) (int64, error) {
	var count int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM drive_file_revisions WHERE tenant_id = $1 AND file_object_id = $2`, tenantID, fileObjectID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count drive revisions: %w", err)
	}
	return count, nil
}

func (s *DriveService) RegisterSyncDevice(ctx context.Context, input DriveRegisterDeviceInput, auditCtx AuditContext) (DriveSyncDevice, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveSyncDevice{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveSyncDevice{}, err
	}
	policy, err := s.drivePolicy(ctx, input.TenantID)
	if err != nil {
		return DriveSyncDevice{}, err
	}
	if !policy.SyncEnabled {
		return DriveSyncDevice{}, ErrDrivePolicyDenied
	}
	name := strings.TrimSpace(input.DeviceName)
	if name == "" {
		return DriveSyncDevice{}, fmt.Errorf("%w: device name is required", ErrDriveInvalidInput)
	}
	platform := normalizeDriveDevicePlatform(input.Platform)
	rawToken, tokenHash, err := generateDriveDeviceToken()
	if err != nil {
		return DriveSyncDevice{}, err
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO drive_sync_devices (
			tenant_id, user_id, device_name, platform, token_hash, last_seen_at, last_ip, last_user_agent
		) VALUES ($1, $2, $3, $4, $5, now(), $6, $7)
		RETURNING public_id::text, tenant_id, user_id, device_name, platform, status, remote_wipe_required, last_seen_at, created_at
	`, input.TenantID, actor.UserID, name, platform, tokenHash, input.RemoteAddr, input.UserAgent)
	device, err := scanDriveSyncDevice(row)
	if err != nil {
		return DriveSyncDevice{}, fmt.Errorf("register drive sync device: %w", err)
	}
	device.RawToken = rawToken
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.sync.device.registered", "drive_sync_device", device.PublicID, map[string]any{
		"platform": platform,
	})
	return device, nil
}

func (s *DriveService) HeartbeatSyncDevice(ctx context.Context, token, remoteAddr, userAgent string) (DriveSyncDevice, error) {
	device, err := s.resolveDriveDeviceToken(ctx, token, true, remoteAddr, userAgent)
	if err != nil {
		return DriveSyncDevice{}, err
	}
	return device, nil
}

func (s *DriveService) SyncDelta(ctx context.Context, token, cursor, remoteAddr, userAgent string) (DriveSyncDelta, error) {
	device, err := s.resolveDriveDeviceToken(ctx, token, false, remoteAddr, userAgent)
	if err != nil {
		return DriveSyncDelta{}, err
	}
	if device.RemoteWipeRequired {
		return DriveSyncDelta{Cursor: cursor, RemoteWipe: true}, nil
	}
	actor, err := s.actor(ctx, device.TenantID, device.UserID)
	if err != nil {
		return DriveSyncDelta{}, err
	}
	policy, err := s.drivePolicy(ctx, device.TenantID)
	if err != nil {
		return DriveSyncDelta{}, err
	}
	if !policy.SyncEnabled {
		return DriveSyncDelta{}, ErrDrivePolicyDenied
	}
	lastID := parseDriveCursor(cursor)
	rows, err := s.pool.Query(ctx, `
		SELECT id, public_id::text, resource_type, resource_id, action, COALESCE(object_version, ''), metadata, created_at
		FROM drive_sync_events
		WHERE tenant_id = $1 AND id > $2
		ORDER BY id
		LIMIT 100
	`, device.TenantID, lastID)
	if err != nil {
		return DriveSyncDelta{}, fmt.Errorf("list drive sync events: %w", err)
	}
	defer rows.Close()
	events := []DriveSyncEvent{}
	denied := 0
	maxID := lastID
	for rows.Next() {
		var event DriveSyncEvent
		var resourceID int64
		var metadata []byte
		if err := rows.Scan(&event.ID, &event.PublicID, &event.ResourceType, &resourceID, &event.Action, &event.ObjectVersion, &metadata, &event.CreatedAt); err != nil {
			return DriveSyncDelta{}, err
		}
		maxID = event.ID
		event.Metadata = map[string]any{}
		_ = json.Unmarshal(metadata, &event.Metadata)
		allowed, publicID := s.syncEventAllowed(ctx, actor, event.ResourceType, resourceID)
		if !allowed {
			denied++
			continue
		}
		event.ResourcePublicID = publicID
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return DriveSyncDelta{}, err
	}
	nextCursor := fmt.Sprintf("%d", maxID)
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO drive_sync_cursors (tenant_id, device_id, cursor_value)
		VALUES ($1, (SELECT id FROM drive_sync_devices WHERE public_id = $2), $3)
		ON CONFLICT (device_id) DO UPDATE
		SET cursor_value = EXCLUDED.cursor_value, last_issued_at = now(), updated_at = now()
	`, device.TenantID, device.PublicID, maxID)
	return DriveSyncDelta{Cursor: nextCursor, Events: events, DeniedCount: denied}, nil
}

func (s *DriveService) RevokeSyncDevice(ctx context.Context, tenantID, actorUserID int64, devicePublicID, reason string, auditCtx AuditContext) error {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "tenant_admin", "drive_sync_admin"); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE drive_sync_devices
		SET status = 'revoked', remote_wipe_required = true, updated_at = now()
		WHERE public_id = $1 AND tenant_id = $2
	`, devicePublicID, tenantID)
	if err != nil {
		return fmt.Errorf("revoke drive sync device: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDriveNotFound
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO drive_remote_wipe_requests (tenant_id, device_id, requested_by_user_id, reason)
		SELECT tenant_id, id, $1, $2
		FROM drive_sync_devices
		WHERE public_id = $3 AND tenant_id = $4
	`, actor.UserID, nonEmpty(reason, "manual"), devicePublicID, tenantID)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.sync.device.revoked", "drive_sync_device", devicePublicID, nil)
	return nil
}

func (s *DriveService) ReplayMobileOfflineOperations(ctx context.Context, token string, operations []DriveOfflineOperationInput, auditCtx AuditContext) (DriveOfflineReplayResult, error) {
	device, err := s.resolveDriveDeviceToken(ctx, token, true, "", "")
	if err != nil {
		return DriveOfflineReplayResult{}, err
	}
	policy, err := s.drivePolicy(ctx, device.TenantID)
	if err != nil {
		return DriveOfflineReplayResult{}, err
	}
	if !policy.MobileOfflineEnabled || !policy.OfflineCacheAllowed {
		return DriveOfflineReplayResult{}, ErrDrivePolicyDenied
	}
	result := DriveOfflineReplayResult{}
	for _, op := range operations {
		status := "denied"
		failure := ""
		switch strings.ToLower(strings.TrimSpace(op.OperationType)) {
		case "rename_file":
			_, err := s.UpdateFile(ctx, DriveUpdateFileInput{
				TenantID:     device.TenantID,
				ActorUserID:  device.UserID,
				FilePublicID: op.ResourcePublicID,
				Filename:     &op.Name,
			}, auditCtx)
			if err == nil {
				result.Applied++
				status = "applied"
			} else if errors.Is(err, ErrDriveLocked) {
				result.Conflicted++
				status = "conflicted"
				failure = "conflict"
			} else {
				result.Denied++
				failure = "permission_or_policy"
			}
		default:
			result.Denied++
			failure = "unsupported_operation"
		}
		resourceID, parseErr := uuid.Parse(strings.TrimSpace(op.ResourcePublicID))
		if parseErr == nil {
			_, _ = s.pool.Exec(ctx, `
				INSERT INTO drive_mobile_offline_operations (
					tenant_id, device_id, operation_type, resource_type, resource_public_id, base_revision, status, failure_reason, applied_at
				)
				SELECT $1, id, $2, $3, $4, $5, $6, NULLIF($7, ''), CASE WHEN $6 = 'applied' THEN now() ELSE NULL END
				FROM drive_sync_devices
				WHERE public_id = $8 AND tenant_id = $1
			`, device.TenantID, op.OperationType, nonEmpty(op.ResourceType, "file"), resourceID, op.BaseRevision, status, failure, device.PublicID)
		}
	}
	return result, nil
}

func (s *DriveService) UpsertEncryptionPolicy(ctx context.Context, tenantID, actorUserID int64, mode, provider, keyRef string, auditCtx AuditContext) (DriveEncryptionPolicy, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveEncryptionPolicy{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "tenant_admin", "drive_security_admin"); err != nil {
		return DriveEncryptionPolicy{}, err
	}
	policy, err := s.drivePolicy(ctx, tenantID)
	if err != nil {
		return DriveEncryptionPolicy{}, err
	}
	if !policy.CMKEnabled {
		return DriveEncryptionPolicy{}, ErrDrivePolicyDenied
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "service_managed"
	}
	switch mode {
	case "service_managed":
		keyRef = ""
	case "tenant_managed":
		if strings.TrimSpace(keyRef) == "" {
			return DriveEncryptionPolicy{}, fmt.Errorf("%w: keyRef is required", ErrDriveInvalidInput)
		}
	default:
		return DriveEncryptionPolicy{}, ErrDriveInvalidInput
	}
	var keyID any
	if keyRef != "" {
		keyRow := s.pool.QueryRow(ctx, `
			INSERT INTO drive_kms_keys (tenant_id, provider, key_ref, masked_key_ref, status, last_verified_at, created_by_user_id)
			VALUES ($1, $2, $3, $4, 'active', now(), $5)
			RETURNING id
		`, tenantID, nonEmpty(provider, "external"), keyRef, maskKeyRef(keyRef), actor.UserID)
		var id int64
		if err := keyRow.Scan(&id); err != nil {
			return DriveEncryptionPolicy{}, fmt.Errorf("upsert drive kms key: %w", err)
		}
		keyID = id
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO drive_encryption_policies (tenant_id, mode, kms_key_id, scope, status)
		VALUES ($1, $2, $3, 'tenant', 'active')
		ON CONFLICT (tenant_id) DO UPDATE
		SET mode = EXCLUDED.mode, kms_key_id = EXCLUDED.kms_key_id, updated_at = now()
	`, tenantID, mode, keyID); err != nil {
		return DriveEncryptionPolicy{}, fmt.Errorf("upsert drive encryption policy: %w", err)
	}
	out, err := s.GetEncryptionPolicy(ctx, tenantID)
	if err != nil {
		return DriveEncryptionPolicy{}, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.encryption.policy.updated", "tenant", fmt.Sprintf("%d", tenantID), map[string]any{"mode": mode})
	return out, nil
}

func (s *DriveService) GetEncryptionPolicy(ctx context.Context, tenantID int64) (DriveEncryptionPolicy, error) {
	var out DriveEncryptionPolicy
	var keyPublicID pgtype.UUID
	var provider, masked, keyStatus pgtype.Text
	var updatedAt pgtype.Timestamptz
	err := s.pool.QueryRow(ctx, `
		SELECT p.mode, p.scope, k.public_id, COALESCE(k.provider, ''), COALESCE(k.masked_key_ref, ''), COALESCE(k.status, ''), p.updated_at
		FROM drive_encryption_policies p
		LEFT JOIN drive_kms_keys k ON k.id = p.kms_key_id
		WHERE p.tenant_id = $1
	`, tenantID).Scan(&out.Mode, &out.Scope, &keyPublicID, &provider, &masked, &keyStatus, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveEncryptionPolicy{Mode: "service_managed", Scope: "tenant", KeyStatus: "active"}, nil
	}
	if err != nil {
		return DriveEncryptionPolicy{}, fmt.Errorf("get drive encryption policy: %w", err)
	}
	if keyPublicID.Valid {
		out.KeyPublicID = uuid.UUID(keyPublicID.Bytes).String()
	}
	out.Provider = optionalText(provider)
	out.MaskedKeyRef = optionalText(masked)
	out.KeyStatus = optionalText(keyStatus)
	if out.KeyStatus == "" {
		out.KeyStatus = "active"
	}
	out.UpdatedAt = updatedAt.Time
	return out, nil
}

func (s *DriveService) SetEncryptionKeyStatus(ctx context.Context, tenantID, actorUserID int64, keyPublicID, status string, auditCtx AuditContext) (DriveEncryptionPolicy, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveEncryptionPolicy{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "tenant_admin", "drive_security_admin"); err != nil {
		return DriveEncryptionPolicy{}, err
	}
	switch status {
	case "active", "disabled", "unavailable", "deleted":
	default:
		return DriveEncryptionPolicy{}, ErrDriveInvalidInput
	}
	tag, err := s.pool.Exec(ctx, `UPDATE drive_kms_keys SET status = $1, updated_at = now() WHERE public_id = $2 AND tenant_id = $3`, status, keyPublicID, tenantID)
	if err != nil {
		return DriveEncryptionPolicy{}, fmt.Errorf("update drive kms key status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return DriveEncryptionPolicy{}, ErrDriveNotFound
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.encryption.key_status.updated", "drive_kms_key", keyPublicID, map[string]any{"status": status})
	return s.GetEncryptionPolicy(ctx, tenantID)
}

func (s *DriveService) ensureDriveEncryptionAvailable(ctx context.Context, tenantID, fileObjectID int64) error {
	policy, err := s.GetEncryptionPolicy(ctx, tenantID)
	if err != nil {
		return err
	}
	if policy.Mode == "" || policy.Mode == "service_managed" {
		return s.ensureDriveHSMAvailable(ctx, tenantID, fileObjectID)
	}
	if policy.KeyStatus != "active" {
		return ErrDrivePolicyDenied
	}
	return s.ensureDriveHSMAvailable(ctx, tenantID, fileObjectID)
}

func (s *DriveService) UpsertResidencyPolicy(ctx context.Context, tenantID, actorUserID int64, input DriveResidencyPolicy, auditCtx AuditContext) (DriveResidencyPolicy, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveResidencyPolicy{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "tenant_admin", "drive_security_admin"); err != nil {
		return DriveResidencyPolicy{}, err
	}
	policy, err := s.drivePolicy(ctx, tenantID)
	if err != nil {
		return DriveResidencyPolicy{}, err
	}
	if !policy.DataResidencyEnabled {
		return DriveResidencyPolicy{}, ErrDrivePolicyDenied
	}
	input.PrimaryRegion = strings.ToLower(strings.TrimSpace(input.PrimaryRegion))
	if input.PrimaryRegion == "" {
		input.PrimaryRegion = "global"
	}
	input.AllowedRegions = normalizeRegionList(input.AllowedRegions)
	if len(input.AllowedRegions) == 0 {
		input.AllowedRegions = []string{input.PrimaryRegion}
	}
	if !stringSliceContains(input.AllowedRegions, input.PrimaryRegion) {
		return DriveResidencyPolicy{}, fmt.Errorf("%w: primary region must be allowed", ErrDriveInvalidInput)
	}
	if input.ReplicationMode == "" {
		input.ReplicationMode = "none"
	}
	if input.IndexRegion == "" {
		input.IndexRegion = "same_as_primary"
	}
	if input.BackupRegion == "" {
		input.BackupRegion = "same_jurisdiction"
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO drive_region_policies (tenant_id, primary_region, allowed_regions, replication_mode, index_region, backup_region)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id) DO UPDATE
		SET primary_region = EXCLUDED.primary_region,
		    allowed_regions = EXCLUDED.allowed_regions,
		    replication_mode = EXCLUDED.replication_mode,
		    index_region = EXCLUDED.index_region,
		    backup_region = EXCLUDED.backup_region,
		    updated_at = now()
	`, tenantID, input.PrimaryRegion, input.AllowedRegions, input.ReplicationMode, input.IndexRegion, input.BackupRegion); err != nil {
		return DriveResidencyPolicy{}, fmt.Errorf("upsert drive residency policy: %w", err)
	}
	out, err := s.GetResidencyPolicy(ctx, tenantID)
	if err != nil {
		return DriveResidencyPolicy{}, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.residency.policy.updated", "tenant", fmt.Sprintf("%d", tenantID), map[string]any{
		"primaryRegion": out.PrimaryRegion,
	})
	return out, nil
}

func (s *DriveService) GetResidencyPolicy(ctx context.Context, tenantID int64) (DriveResidencyPolicy, error) {
	var out DriveResidencyPolicy
	var updatedAt pgtype.Timestamptz
	err := s.pool.QueryRow(ctx, `
		SELECT primary_region, allowed_regions, replication_mode, index_region, backup_region, status, updated_at
		FROM drive_region_policies
		WHERE tenant_id = $1
	`, tenantID).Scan(&out.PrimaryRegion, &out.AllowedRegions, &out.ReplicationMode, &out.IndexRegion, &out.BackupRegion, &out.Status, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveResidencyPolicy{
			PrimaryRegion: "global", AllowedRegions: []string{"global"},
			ReplicationMode: "none", IndexRegion: "same_as_primary", BackupRegion: "same_jurisdiction", Status: "active",
		}, nil
	}
	if err != nil {
		return DriveResidencyPolicy{}, fmt.Errorf("get drive residency policy: %w", err)
	}
	out.UpdatedAt = updatedAt.Time
	return out, nil
}

func (s *DriveService) CreateLegalCase(ctx context.Context, tenantID, actorUserID int64, name, description string, auditCtx AuditContext) (DriveLegalCase, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveLegalCase{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "legal_admin"); err != nil {
		return DriveLegalCase{}, err
	}
	policy, err := s.drivePolicy(ctx, tenantID)
	if err != nil {
		return DriveLegalCase{}, err
	}
	if !policy.LegalDiscoveryEnabled {
		return DriveLegalCase{}, ErrDrivePolicyDenied
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return DriveLegalCase{}, fmt.Errorf("%w: case name is required", ErrDriveInvalidInput)
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO drive_legal_cases (tenant_id, name, description, created_by_user_id)
		VALUES ($1, $2, $3, $4)
		RETURNING public_id::text, name, description, status, created_at
	`, tenantID, name, strings.TrimSpace(description), actor.UserID)
	item, err := scanDriveLegalCase(row)
	if err != nil {
		return DriveLegalCase{}, fmt.Errorf("create drive legal case: %w", err)
	}
	s.recordChainOfCustodyBestEffort(ctx, tenantID, item.PublicID, "", actor.UserID, "case.created", nil)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.legal.case.created", "drive_legal_case", item.PublicID, nil)
	return item, nil
}

func (s *DriveService) AddLegalCaseFile(ctx context.Context, tenantID, actorUserID int64, casePublicID, filePublicID, reason string, auditCtx AuditContext) error {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "legal_admin"); err != nil {
		return err
	}
	caseID, err := s.legalCaseID(ctx, tenantID, casePublicID)
	if err != nil {
		return err
	}
	row, err := s.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID})
	if err != nil {
		return err
	}
	file := driveFileFromDB(row)
	if strings.TrimSpace(reason) == "" {
		reason = "legal discovery hold"
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin drive legal hold: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO drive_legal_case_resources (tenant_id, case_id, resource_type, resource_id, hold_enabled, added_by_user_id)
		VALUES ($1, $2, 'file', $3, true, $4)
		ON CONFLICT (case_id, resource_type, resource_id) DO NOTHING
	`, tenantID, caseID, file.ID, actor.UserID); err != nil {
		return fmt.Errorf("add drive legal case file: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO drive_legal_holds (tenant_id, case_id, resource_type, resource_id, reason, created_by_user_id)
		VALUES ($1, $2, 'file', $3, $4, $5)
	`, tenantID, caseID, file.ID, reason, actor.UserID); err != nil {
		return fmt.Errorf("create drive legal hold: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE file_objects
		SET legal_hold_at = COALESCE(legal_hold_at, now()),
		    legal_hold_by_user_id = $1,
		    legal_hold_reason = $2,
		    purge_block_reason = 'legal_hold',
		    updated_at = now()
		WHERE id = $3 AND tenant_id = $4 AND purpose = 'drive'
	`, actor.UserID, reason, file.ID, tenantID); err != nil {
		return fmt.Errorf("apply drive legal hold: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit drive legal hold: %w", err)
	}
	s.recordChainOfCustodyBestEffort(ctx, tenantID, casePublicID, "", actor.UserID, "hold.created", map[string]any{"filePublicId": file.PublicID})
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.legal.hold.created", "drive_file", file.PublicID, map[string]any{"casePublicId": casePublicID})
	return nil
}

func (s *DriveService) CreateLegalExport(ctx context.Context, tenantID, actorUserID int64, casePublicID string, auditCtx AuditContext) (DriveLegalExport, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveLegalExport{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "legal_exporter"); err != nil {
		return DriveLegalExport{}, err
	}
	policy, err := s.drivePolicy(ctx, tenantID)
	if err != nil {
		return DriveLegalExport{}, err
	}
	if !policy.LegalDiscoveryEnabled {
		return DriveLegalExport{}, ErrDrivePolicyDenied
	}
	caseID, err := s.legalCaseID(ctx, tenantID, casePublicID)
	if err != nil {
		return DriveLegalExport{}, err
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO drive_legal_exports (tenant_id, case_id, requested_by_user_id, status)
		VALUES ($1, $2, $3, 'pending_approval')
		RETURNING public_id::text, status, created_at
	`, tenantID, caseID, actor.UserID)
	var export DriveLegalExport
	export.CasePublicID = casePublicID
	if err := row.Scan(&export.PublicID, &export.Status, &export.CreatedAt); err != nil {
		return DriveLegalExport{}, fmt.Errorf("create drive legal export: %w", err)
	}
	s.recordChainOfCustodyBestEffort(ctx, tenantID, casePublicID, export.PublicID, actor.UserID, "export.requested", nil)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.legal.export.requested", "drive_legal_export", export.PublicID, nil)
	return export, nil
}

func (s *DriveService) CreateCleanRoom(ctx context.Context, tenantID, actorUserID int64, name string, auditCtx AuditContext) (DriveCleanRoom, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveCleanRoom{}, err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "clean_room_admin"); err != nil {
		return DriveCleanRoom{}, err
	}
	policy, err := s.drivePolicy(ctx, tenantID)
	if err != nil {
		return DriveCleanRoom{}, err
	}
	if !policy.CleanRoomEnabled {
		return DriveCleanRoom{}, ErrDrivePolicyDenied
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return DriveCleanRoom{}, fmt.Errorf("%w: clean room name is required", ErrDriveInvalidInput)
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO drive_clean_rooms (tenant_id, name, created_by_user_id)
		VALUES ($1, $2, $3)
		RETURNING public_id::text, name, status, created_at
	`, tenantID, name, actor.UserID)
	room, err := scanDriveCleanRoom(row)
	if err != nil {
		return DriveCleanRoom{}, fmt.Errorf("create drive clean room: %w", err)
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO drive_clean_room_participants (clean_room_id, tenant_id, participant_tenant_id, user_id, role)
		SELECT id, tenant_id, tenant_id, $1, 'owner'
		FROM drive_clean_rooms
		WHERE public_id = $2 AND tenant_id = $3
	`, actor.UserID, room.PublicID, tenantID)
	if err := s.authz.WriteCleanRoomTuple(ctx, room.PublicID, actor.PublicID, "owner"); err != nil {
		return DriveCleanRoom{}, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.clean_room.created", "drive_clean_room", room.PublicID, nil)
	return room, nil
}

func (s *DriveService) AddCleanRoomParticipant(ctx context.Context, tenantID, actorUserID int64, roomPublicID, userPublicID, role string, auditCtx AuditContext) error {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	if err := s.ensureAnyGlobalRole(ctx, actor.UserID, "clean_room_admin"); err != nil {
		return err
	}
	roomID, err := s.cleanRoomID(ctx, tenantID, roomPublicID)
	if err != nil {
		return err
	}
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = "participant"
	}
	if role != "participant" && role != "reviewer" && role != "owner" {
		return ErrDriveInvalidInput
	}
	var userID int64
	var resolvedUserPublicID string
	if err := s.pool.QueryRow(ctx, `SELECT id, public_id::text FROM users WHERE public_id = $1`, userPublicID).Scan(&userID, &resolvedUserPublicID); errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	} else if err != nil {
		return fmt.Errorf("resolve clean room participant: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO drive_clean_room_participants (clean_room_id, tenant_id, participant_tenant_id, user_id, role)
		VALUES ($1, $2, $2, $3, $4)
		ON CONFLICT (clean_room_id, participant_tenant_id, user_id, role) DO UPDATE
		SET status = 'active', updated_at = now()
	`, roomID, tenantID, userID, role); err != nil {
		return fmt.Errorf("add drive clean room participant: %w", err)
	}
	if err := s.authz.WriteCleanRoomTuple(ctx, roomPublicID, resolvedUserPublicID, role); err != nil {
		return err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.clean_room.participant.added", "drive_clean_room", roomPublicID, map[string]any{"role": role})
	return nil
}

func (s *DriveService) SubmitCleanRoomDataset(ctx context.Context, tenantID, actorUserID int64, roomPublicID, filePublicID string, auditCtx AuditContext) (DriveCleanRoomDataset, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveCleanRoomDataset{}, err
	}
	roomID, err := s.cleanRoomID(ctx, tenantID, roomPublicID)
	if err != nil {
		return DriveCleanRoomDataset{}, err
	}
	policy, err := s.drivePolicy(ctx, tenantID)
	if err != nil {
		return DriveCleanRoomDataset{}, err
	}
	if !policy.CleanRoomEnabled {
		return DriveCleanRoomDataset{}, ErrDrivePolicyDenied
	}
	if err := s.authz.CanSubmitCleanRoomDataset(ctx, actor, roomPublicID); err != nil {
		s.recordCleanRoomDecision(ctx, roomID, tenantID, actor.TenantID, tenantID, "deny", "openfga_denied")
		return DriveCleanRoomDataset{}, err
	}
	row, err := s.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID})
	if err != nil {
		return DriveCleanRoomDataset{}, err
	}
	file := driveFileFromDB(row)
	if err := s.authz.CanViewFile(ctx, actor, file); err != nil {
		s.recordCleanRoomDecision(ctx, roomID, tenantID, actor.TenantID, file.TenantID, "deny", "source_file_view_denied")
		return DriveCleanRoomDataset{}, err
	}
	if err := s.authz.CanShareFile(ctx, actor, file); err != nil {
		s.recordCleanRoomDecision(ctx, roomID, tenantID, actor.TenantID, file.TenantID, "deny", "source_file_share_denied")
		return DriveCleanRoomDataset{}, err
	}
	if file.DLPBlocked || file.ScanStatus == "infected" || file.ScanStatus == "blocked" {
		s.recordCleanRoomDecision(ctx, roomID, tenantID, actor.TenantID, file.TenantID, "deny", "dlp_or_scan_blocked")
		return DriveCleanRoomDataset{}, ErrDrivePolicyDenied
	}
	row2 := s.pool.QueryRow(ctx, `
		INSERT INTO drive_clean_room_datasets (clean_room_id, tenant_id, source_file_object_id, submitted_by_user_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (clean_room_id, source_file_object_id) DO UPDATE
		SET status = 'submitted', updated_at = now()
		RETURNING public_id::text, status, created_at
	`, roomID, tenantID, file.ID, actor.UserID)
	var dataset DriveCleanRoomDataset
	dataset.CleanRoomPublicID = roomPublicID
	dataset.SourceFilePublicID = file.PublicID
	if err := row2.Scan(&dataset.PublicID, &dataset.Status, &dataset.CreatedAt); err != nil {
		return DriveCleanRoomDataset{}, fmt.Errorf("submit drive clean room dataset: %w", err)
	}
	s.recordCleanRoomDecision(ctx, roomID, tenantID, actor.TenantID, file.TenantID, "allow", "dataset_submitted")
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.clean_room.dataset.submitted", "drive_clean_room", roomPublicID, map[string]any{"filePublicId": file.PublicID})
	return dataset, nil
}

func (s *DriveService) RequestCleanRoomExport(ctx context.Context, tenantID, actorUserID int64, roomPublicID string, rawDatasetExport bool, auditCtx AuditContext) (DriveCleanRoomExport, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveCleanRoomExport{}, err
	}
	roomID, err := s.cleanRoomID(ctx, tenantID, roomPublicID)
	if err != nil {
		return DriveCleanRoomExport{}, err
	}
	policy, err := s.drivePolicy(ctx, tenantID)
	if err != nil {
		return DriveCleanRoomExport{}, err
	}
	status := "pending_approval"
	deniedReason := ""
	if rawDatasetExport && !policy.CleanRoomRawExportEnabled {
		status = "denied"
		deniedReason = "raw_dataset_export_disabled"
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO drive_clean_room_exports (clean_room_id, tenant_id, requested_by_user_id, status, raw_dataset_export)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING public_id::text, status, created_at
	`, roomID, tenantID, actor.UserID, status, rawDatasetExport)
	var export DriveCleanRoomExport
	export.DeniedReason = deniedReason
	if err := row.Scan(&export.PublicID, &export.Status, &export.CreatedAt); err != nil {
		return DriveCleanRoomExport{}, fmt.Errorf("request drive clean room export: %w", err)
	}
	decision := "allow"
	if status == "denied" {
		decision = "deny"
	}
	s.recordCleanRoomDecision(ctx, roomID, tenantID, actor.TenantID, tenantID, decision, nonEmpty(deniedReason, "export_requested"))
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.clean_room.export.requested", "drive_clean_room", roomPublicID, map[string]any{"status": status})
	if status == "denied" {
		return export, ErrDrivePolicyDenied
	}
	return export, nil
}

func (s *DriveService) recordDriveSyncEventBestEffort(ctx context.Context, resource DriveResourceRef, action, objectVersion string, metadata map[string]any) {
	if resource.TenantID <= 0 || resource.ID <= 0 {
		return
	}
	data, _ := json.Marshal(metadata)
	var workspaceID any
	switch resource.Type {
	case DriveResourceTypeFile:
		var id pgtype.Int8
		if err := s.pool.QueryRow(ctx, `SELECT workspace_id FROM file_objects WHERE id = $1 AND tenant_id = $2 AND purpose = 'drive'`, resource.ID, resource.TenantID).Scan(&id); err == nil && id.Valid {
			workspaceID = id.Int64
		}
	case DriveResourceTypeFolder:
		var id pgtype.Int8
		if err := s.pool.QueryRow(ctx, `SELECT workspace_id FROM drive_folders WHERE id = $1 AND tenant_id = $2`, resource.ID, resource.TenantID).Scan(&id); err == nil && id.Valid {
			workspaceID = id.Int64
		}
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO drive_sync_events (tenant_id, workspace_id, resource_type, resource_id, action, object_version, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
	`, resource.TenantID, workspaceID, string(resource.Type), resource.ID, action, objectVersion, string(data))
}

func (s *DriveService) syncEventAllowed(ctx context.Context, actor DriveActor, resourceType string, resourceID int64) (bool, string) {
	switch DriveResourceType(resourceType) {
	case DriveResourceTypeFile:
		row, err := s.getDriveFileRow(ctx, actor.TenantID, DriveResourceRef{Type: DriveResourceTypeFile, ID: resourceID})
		if err != nil {
			return false, ""
		}
		file := driveFileFromDB(row)
		return s.authz.CanViewFile(ctx, actor, file) == nil, file.PublicID
	case DriveResourceTypeFolder:
		folder, err := s.getFolderByID(ctx, actor.TenantID, resourceID)
		if err != nil {
			return false, ""
		}
		return s.authz.CanViewFolder(ctx, actor, folder) == nil, folder.PublicID
	default:
		return false, ""
	}
}

func (s *DriveService) resolveDriveDeviceToken(ctx context.Context, token string, requireActive bool, remoteAddr, userAgent string) (DriveSyncDevice, error) {
	hash := driveDeviceTokenHash(token)
	if hash == "" {
		return DriveSyncDevice{}, ErrDrivePermissionDenied
	}
	row := s.pool.QueryRow(ctx, `
		UPDATE drive_sync_devices
		SET last_seen_at = now(),
		    last_ip = COALESCE(NULLIF($2, ''), last_ip),
		    last_user_agent = COALESCE(NULLIF($3, ''), last_user_agent),
		    updated_at = now()
		WHERE token_hash = $1
		RETURNING public_id::text, tenant_id, user_id, device_name, platform, status, remote_wipe_required, last_seen_at, created_at
	`, hash, remoteAddr, userAgent)
	device, err := scanDriveSyncDevice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveSyncDevice{}, ErrDrivePermissionDenied
	}
	if err != nil {
		return DriveSyncDevice{}, fmt.Errorf("resolve drive sync device: %w", err)
	}
	if requireActive && device.Status != "active" {
		return DriveSyncDevice{}, ErrDrivePermissionDenied
	}
	return device, nil
}

func scanDriveSyncDevice(row pgx.Row) (DriveSyncDevice, error) {
	var device DriveSyncDevice
	var lastSeen pgtype.Timestamptz
	if err := row.Scan(&device.PublicID, &device.TenantID, &device.UserID, &device.DeviceName, &device.Platform, &device.Status, &device.RemoteWipeRequired, &lastSeen, &device.CreatedAt); err != nil {
		return DriveSyncDevice{}, err
	}
	device.LastSeenAt = optionalPgTime(lastSeen)
	return device, nil
}

func generateDriveDeviceToken() (string, string, error) {
	buf := make([]byte, driveSyncDeviceTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate drive device token: %w", err)
	}
	raw := "hhd_" + base64.RawURLEncoding.EncodeToString(buf)
	return raw, driveDeviceTokenHash(raw), nil
}

func driveDeviceTokenHash(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeDriveDevicePlatform(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "mobile":
		return "mobile"
	case "web":
		return "web"
	default:
		return "desktop"
	}
}

func parseDriveCursor(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	var id int64
	_, _ = fmt.Sscanf(value, "%d", &id)
	if id < 0 {
		return 0
	}
	return id
}

func scanDriveLegalCase(row pgx.Row) (DriveLegalCase, error) {
	var item DriveLegalCase
	if err := row.Scan(&item.PublicID, &item.Name, &item.Description, &item.Status, &item.CreatedAt); err != nil {
		return DriveLegalCase{}, err
	}
	return item, nil
}

func (s *DriveService) legalCaseID(ctx context.Context, tenantID int64, publicID string) (int64, error) {
	var id int64
	if err := s.pool.QueryRow(ctx, `SELECT id FROM drive_legal_cases WHERE public_id = $1 AND tenant_id = $2 AND status = 'active'`, publicID, tenantID).Scan(&id); errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrDriveNotFound
	} else if err != nil {
		return 0, fmt.Errorf("get drive legal case: %w", err)
	}
	return id, nil
}

func scanDriveCleanRoom(row pgx.Row) (DriveCleanRoom, error) {
	var item DriveCleanRoom
	if err := row.Scan(&item.PublicID, &item.Name, &item.Status, &item.CreatedAt); err != nil {
		return DriveCleanRoom{}, err
	}
	return item, nil
}

func (s *DriveService) cleanRoomID(ctx context.Context, tenantID int64, publicID string) (int64, error) {
	var id int64
	if err := s.pool.QueryRow(ctx, `SELECT id FROM drive_clean_rooms WHERE public_id = $1 AND tenant_id = $2 AND status = 'active'`, publicID, tenantID).Scan(&id); errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrDriveNotFound
	} else if err != nil {
		return 0, fmt.Errorf("get drive clean room: %w", err)
	}
	return id, nil
}

func (s *DriveService) recordChainOfCustodyBestEffort(ctx context.Context, tenantID int64, casePublicID, exportPublicID string, actorUserID int64, action string, metadata map[string]any) {
	data, _ := json.Marshal(metadata)
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO drive_chain_of_custody_events (tenant_id, case_id, export_id, actor_user_id, action, metadata)
		VALUES (
			$1,
			(SELECT id FROM drive_legal_cases WHERE public_id = NULLIF($2, '')::uuid AND tenant_id = $1),
			(SELECT id FROM drive_legal_exports WHERE public_id = NULLIF($3, '')::uuid AND tenant_id = $1),
			$4,
			$5,
			COALESCE(NULLIF($6, '')::jsonb, '{}'::jsonb)
		)
	`, tenantID, casePublicID, exportPublicID, actorUserID, action, string(data))
}

func (s *DriveService) recordCleanRoomDecision(ctx context.Context, roomID, tenantID, actorTenantID, resourceTenantID int64, decision, reason string) {
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO drive_clean_room_policy_decisions (clean_room_id, tenant_id, actor_tenant_id, resource_tenant_id, decision, reason)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, roomID, tenantID, actorTenantID, resourceTenantID, decision, reason)
}

func (s *DriveService) ensureAnyGlobalRole(ctx context.Context, userID int64, allowed ...string) error {
	roles, err := s.queries.ListRoleCodesByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("list user roles: %w", err)
	}
	required := map[string]struct{}{}
	for _, role := range allowed {
		required[strings.ToLower(strings.TrimSpace(role))] = struct{}{}
	}
	for _, role := range roles {
		if _, ok := required[strings.ToLower(strings.TrimSpace(role))]; ok {
			return nil
		}
	}
	return ErrDrivePermissionDenied
}

func maskKeyRef(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

func nonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(needle)) {
			return true
		}
	}
	return false
}
