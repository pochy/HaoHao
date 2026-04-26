package service

import (
	"context"
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

const defaultAdminContentAccessTTL = 15 * time.Minute

func (s *DriveService) ListWorkspaces(ctx context.Context, tenantID, actorUserID int64, limit int32) ([]DriveWorkspace, error) {
	if err := s.ensureConfigured(false); err != nil {
		return nil, err
	}
	if _, err := s.actor(ctx, tenantID, actorUserID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.queries.ListDriveWorkspaces(ctx, db.ListDriveWorkspacesParams{TenantID: tenantID, LimitCount: limit})
	if err != nil {
		return nil, fmt.Errorf("list drive workspaces: %w", err)
	}
	items := make([]DriveWorkspace, 0, len(rows))
	for _, row := range rows {
		items = append(items, driveWorkspaceFromDB(row))
	}
	return items, nil
}

func (s *DriveService) CreateWorkspace(ctx context.Context, input DriveCreateWorkspaceInput, auditCtx AuditContext) (DriveWorkspace, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveWorkspace{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveWorkspace{}, err
	}
	name := normalizeDriveName(input.Name)
	if name == "" {
		return DriveWorkspace{}, fmt.Errorf("%w: workspace name is required", ErrDriveInvalidInput)
	}
	policy, err := s.drivePolicy(ctx, input.TenantID)
	if err != nil {
		return DriveWorkspace{}, err
	}
	count, err := s.queries.CountDriveWorkspaces(ctx, input.TenantID)
	if err != nil {
		return DriveWorkspace{}, fmt.Errorf("count drive workspaces: %w", err)
	}
	if int(count) >= policy.MaxWorkspaceCount {
		s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.policy.enforcement_denied", "drive_workspace", "new", map[string]any{
			"feature": "workspace_count",
			"plan":    policy.PlanCode,
		})
		return DriveWorkspace{}, ErrDrivePolicyDenied
	}
	override := input.PolicyOverrideJSON
	if len(override) == 0 {
		override = []byte(`{}`)
	}
	row, err := s.queries.CreateDriveWorkspace(ctx, db.CreateDriveWorkspaceParams{
		TenantID:          input.TenantID,
		Name:              name,
		CreatedByUserID:   pgtype.Int8{Int64: actor.UserID, Valid: true},
		StorageQuotaBytes: pgInt8(input.StorageQuotaBytes),
		PolicyOverride:    override,
	})
	if err != nil {
		return DriveWorkspace{}, fmt.Errorf("create drive workspace: %w", err)
	}
	workspace := driveWorkspaceFromDB(row)
	if err := s.authz.WriteWorkspaceOwner(ctx, actor, workspace); err != nil {
		_, _ = s.queries.SoftDeleteDriveWorkspace(context.Background(), db.SoftDeleteDriveWorkspaceParams{PublicID: row.PublicID, TenantID: input.TenantID})
		return DriveWorkspace{}, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.workspace.created", "drive_workspace", workspace.PublicID, map[string]any{
		"name": name,
	})
	return workspace, nil
}

func (s *DriveService) UpdateWorkspace(ctx context.Context, input DriveUpdateWorkspaceInput, auditCtx AuditContext) (DriveWorkspace, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveWorkspace{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveWorkspace{}, err
	}
	publicID, err := uuid.Parse(strings.TrimSpace(input.WorkspacePublicID))
	if err != nil {
		return DriveWorkspace{}, ErrDriveNotFound
	}
	name := normalizeDriveName(input.Name)
	if name == "" {
		return DriveWorkspace{}, fmt.Errorf("%w: workspace name is required", ErrDriveInvalidInput)
	}
	override := input.PolicyOverrideJSON
	if len(override) == 0 {
		override = nil
	}
	row, err := s.queries.UpdateDriveWorkspace(ctx, db.UpdateDriveWorkspaceParams{
		Name:              name,
		StorageQuotaBytes: pgInt8(input.StorageQuotaBytes),
		PolicyOverride:    override,
		PublicID:          publicID,
		TenantID:          input.TenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveWorkspace{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveWorkspace{}, fmt.Errorf("update drive workspace: %w", err)
	}
	workspace := driveWorkspaceFromDB(row)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.workspace.updated", "drive_workspace", workspace.PublicID, nil)
	return workspace, nil
}

func (s *DriveService) DeleteWorkspace(ctx context.Context, tenantID, actorUserID int64, workspacePublicID string, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	publicID, err := uuid.Parse(strings.TrimSpace(workspacePublicID))
	if err != nil {
		return ErrDriveNotFound
	}
	var childCount int64
	if err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM drive_folders WHERE tenant_id = $1 AND workspace_id = w.id AND deleted_at IS NULL) +
			(SELECT count(*) FROM file_objects WHERE tenant_id = $1 AND workspace_id = w.id AND purpose = 'drive' AND deleted_at IS NULL)
		FROM drive_workspaces w
		WHERE w.tenant_id = $1 AND w.public_id = $2 AND w.deleted_at IS NULL
	`, tenantID, publicID).Scan(&childCount); errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	} else if err != nil {
		return fmt.Errorf("check drive workspace children: %w", err)
	}
	if childCount > 0 {
		return fmt.Errorf("%w: workspace must be empty before delete", ErrDriveInvalidInput)
	}
	row, err := s.queries.SoftDeleteDriveWorkspace(ctx, db.SoftDeleteDriveWorkspaceParams{PublicID: publicID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	}
	if err != nil {
		return fmt.Errorf("delete drive workspace: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.workspace.deleted", "drive_workspace", row.PublicID.String(), nil)
	return nil
}

func (s *DriveService) StartAdminContentAccessSession(ctx context.Context, input DriveStartAdminContentAccessInput, auditCtx AuditContext) (DriveAdminContentAccessSession, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveAdminContentAccessSession{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveAdminContentAccessSession{}, err
	}
	if err := s.ensureDriveContentAdmin(ctx, actor.UserID); err != nil {
		s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.admin_content_access.denied", "tenant", fmt.Sprintf("%d", input.TenantID), map[string]any{"reason": "missing_role"})
		return DriveAdminContentAccessSession{}, err
	}
	policy, err := s.drivePolicy(ctx, input.TenantID)
	if err != nil {
		return DriveAdminContentAccessSession{}, err
	}
	if policy.AdminContentAccessMode != "break_glass" {
		s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.admin_content_access.denied", "tenant", fmt.Sprintf("%d", input.TenantID), map[string]any{"reason": "policy_disabled"})
		return DriveAdminContentAccessSession{}, ErrDrivePolicyDenied
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return DriveAdminContentAccessSession{}, fmt.Errorf("%w: reason is required", ErrDriveInvalidInput)
	}
	category := normalizeAdminContentReasonCategory(input.ReasonCategory)
	ttl := input.TTL
	if ttl <= 0 || ttl > defaultAdminContentAccessTTL {
		ttl = defaultAdminContentAccessTTL
	}
	expiresAt := s.now().Add(ttl).UTC()
	row := s.pool.QueryRow(ctx, `
		INSERT INTO drive_admin_content_access_sessions (
			tenant_id,
			actor_user_id,
			reason,
			reason_category,
			expires_at
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, public_id, tenant_id, actor_user_id, reason, reason_category, expires_at, ended_at, created_at
	`, input.TenantID, actor.UserID, reason, category, expiresAt)
	session, err := scanDriveAdminContentAccessSession(row)
	if err != nil {
		return DriveAdminContentAccessSession{}, fmt.Errorf("start drive admin content access session: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.admin_content_access.session_started", "drive_admin_content_access_session", session.PublicID, map[string]any{
		"reasonCategory": category,
		"reasonLength":   len(reason),
		"expiresAt":      expiresAt,
	})
	return session, nil
}

func (s *DriveService) EndAdminContentAccessSession(ctx context.Context, tenantID, actorUserID int64, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE drive_admin_content_access_sessions
		SET ended_at = now()
		WHERE tenant_id = $1
		  AND actor_user_id = $2
		  AND ended_at IS NULL
		  AND expires_at > now()
	`, tenantID, actor.UserID)
	if err != nil {
		return fmt.Errorf("end drive admin content access session: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.admin_content_access.session_ended", "tenant", fmt.Sprintf("%d", tenantID), map[string]any{
		"ended": tag.RowsAffected(),
	})
	return nil
}

func (s *DriveService) GetAdminDriveFileMetadata(ctx context.Context, tenantID, actorUserID int64, filePublicID string, auditCtx AuditContext) (DriveFile, error) {
	file, _, err := s.adminDriveFileAccess(ctx, tenantID, actorUserID, filePublicID, auditCtx, "metadata_viewed")
	return file, err
}

func (s *DriveService) AdminDriveFileContent(ctx context.Context, tenantID, actorUserID int64, filePublicID string, auditCtx AuditContext) (DriveFileDownload, error) {
	file, actor, err := s.adminDriveFileAccess(ctx, tenantID, actorUserID, filePublicID, auditCtx, "content_viewed")
	if err != nil {
		return DriveFileDownload{}, err
	}
	if err := s.ensureFileDownloadAllowed(ctx, actor, file, auditCtx, "drive.admin_content_access.content_viewed"); err != nil {
		return DriveFileDownload{}, err
	}
	body, err := s.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return DriveFileDownload{}, err
	}
	return DriveFileDownload{File: file, Body: body}, nil
}

func (s *DriveService) PublicShareLinkOverwriteContentWithVerification(ctx context.Context, input DrivePublicEditorOverwriteInput) (DriveFile, error) {
	if err := s.ensureConfigured(true); err != nil {
		return DriveFile{}, err
	}
	link, file, _, err := s.resolvePublicShareLink(ctx, input.Token, false)
	if err != nil {
		return DriveFile{}, err
	}
	if file == nil || link.Role != DriveRoleEditor {
		return DriveFile{}, ErrDrivePermissionDenied
	}
	policy, err := s.drivePolicy(ctx, link.TenantID)
	if err != nil {
		return DriveFile{}, err
	}
	if !policy.AnonymousEditorLinksEnabled {
		s.recordPublicLinkAudit(ctx, link, "drive.share_link.editor_disabled", map[string]any{"reason": "policy_disabled"})
		return DriveFile{}, ErrDrivePolicyDenied
	}
	if link.PasswordRequired && !s.publicShareLinkPasswordVerified(ctx, link, input.VerificationCookie) {
		s.recordPublicLinkAudit(ctx, link, "drive.share_link.editor_denied", map[string]any{"reason": "password_required"})
		return DriveFile{}, ErrDrivePermissionDenied
	}
	if err := s.authz.CanEditWithShareLink(ctx, link); err != nil {
		s.recordPublicLinkAudit(ctx, link, "drive.share_link.editor_denied", map[string]any{"reason": "openfga_denied"})
		return DriveFile{}, err
	}
	if err := s.ensureFileMutationAllowed(ctx, link.TenantID, file.ID); err != nil {
		return DriveFile{}, err
	}
	filename := normalizeDriveName(filepath.Base(strings.TrimSpace(input.Filename)))
	if filename == "" || filename == "." {
		filename = file.OriginalFilename
	}
	contentType := normalizeContentType(input.ContentType)
	storageKey := fmt.Sprintf("tenants/%d/drive/%s", link.TenantID, uuid.NewString())
	stored, err := s.storage.Save(ctx, storageKey, input.Body, policy.MaxFileSizeBytes)
	if err != nil {
		if errors.Is(err, ErrFileTooLarge) {
			return DriveFile{}, ErrInvalidFileInput
		}
		return DriveFile{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveFile{}, fmt.Errorf("begin public editor overwrite transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	if _, err := tx.Exec(ctx, `
		INSERT INTO drive_file_revisions (
			tenant_id,
			file_object_id,
			actor_type,
			previous_original_filename,
			previous_content_type,
			previous_byte_size,
			previous_sha256_hex,
			previous_storage_driver,
			previous_storage_key,
			reason
		) VALUES ($1, $2, 'anonymous_share_link', $3, $4, $5, $6, $7, $8, 'public_editor_overwrite')
	`, file.TenantID, file.ID, file.OriginalFilename, file.ContentType, file.ByteSize, file.SHA256Hex, file.StorageDriver, file.StorageKey); err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveFile{}, fmt.Errorf("create drive revision: %w", err)
	}
	qtx := s.queries.WithTx(tx)
	updatedRow, err := qtx.UpdateDriveFileObjectMetadata(ctx, db.UpdateDriveFileObjectMetadataParams{
		ContentType:   contentType,
		ByteSize:      stored.Size,
		Sha256Hex:     stored.SHA256Hex,
		StorageDriver: "local",
		StorageKey:    stored.Key,
		ScanStatus:    driveInitialScanStatus(policy),
		ID:            file.ID,
		TenantID:      file.TenantID,
	})
	if err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveFile{}, fmt.Errorf("update public editor content: %w", err)
	}
	if filename != file.OriginalFilename {
		if renamed, renameErr := qtx.RenameDriveFile(ctx, db.RenameDriveFileParams{OriginalFilename: filename, ID: file.ID, TenantID: file.TenantID}); renameErr == nil {
			updatedRow = renamed
		}
	}
	updated := driveFileFromDB(updatedRow)
	if err := s.recordAuditWithQueries(ctx, qtx, AuditContext{ActorType: AuditActorSystem, TenantID: &file.TenantID}, "drive.share_link.editor_updated_content", "drive_file", file.PublicID, map[string]any{
		"actorType":         "anonymous_share_link",
		"shareLinkPublicId": link.PublicID,
		"contentType":       updated.ContentType,
		"byteSize":          updated.ByteSize,
	}); err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveFile{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveFile{}, fmt.Errorf("commit public editor overwrite transaction: %w", err)
	}
	return updated, nil
}

func (s *DriveService) DriveOperationsHealth(ctx context.Context, tenantID int64) (DriveOperationsHealth, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveOperationsHealth{}, err
	}
	health := DriveOperationsHealth{
		TenantID:                tenantID,
		StorageOrphanCheckState: "not_run",
		CheckedAt:               s.now().UTC(),
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM drive_workspaces WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID).Scan(&health.WorkspaceCount); err != nil {
		return DriveOperationsHealth{}, fmt.Errorf("count drive workspaces: %w", err)
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM drive_folders WHERE tenant_id = $1 AND deleted_at IS NULL AND workspace_id IS NULL) +
			(SELECT count(*) FROM file_objects WHERE tenant_id = $1 AND purpose = 'drive' AND deleted_at IS NULL AND workspace_id IS NULL)
	`, tenantID).Scan(&health.MissingWorkspaceCount); err != nil {
		return DriveOperationsHealth{}, fmt.Errorf("count missing drive workspace bindings: %w", err)
	}
	if s.authz != nil {
		if drift, err := s.OpenFGADrift(ctx, tenantID); err == nil {
			health.OpenFGADriftCount = len(drift.Items)
		}
	}
	missing, err := s.storageMissingObjectCount(ctx, tenantID, 50)
	if err == nil {
		health.StorageMissingCount = missing
		health.StorageOrphanCheckState = "sampled"
	}
	return health, nil
}

func (s *DriveService) ensureFileDownloadAllowed(ctx context.Context, actor DriveActor, file DriveFile, auditCtx AuditContext, action string) error {
	policy, err := s.drivePolicy(ctx, file.TenantID)
	if err != nil {
		return err
	}
	if file.DLPBlocked || file.ScanStatus == "infected" || file.ScanStatus == "blocked" {
		s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.download_denied_scan", "drive_file", file.PublicID, map[string]any{
			"scanStatus": file.ScanStatus,
			"dlpBlocked": file.DLPBlocked,
			"operation":  action,
		})
		return ErrDrivePolicyDenied
	}
	if policy.ContentScanEnabled && policy.BlockDownloadUntilScanComplete && file.ScanStatus == "pending" {
		s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.download_denied_scan", "drive_file", file.PublicID, map[string]any{
			"scanStatus": file.ScanStatus,
			"operation":  action,
		})
		return ErrDrivePolicyDenied
	}
	return nil
}

func (s *DriveService) ensureResourceShareAllowed(ctx context.Context, actor DriveActor, resource DriveResourceRef, auditCtx AuditContext) error {
	if resource.Type != DriveResourceTypeFile {
		return nil
	}
	row, err := s.getDriveFileRow(ctx, actor.TenantID, resource)
	if err != nil {
		return err
	}
	file := driveFileFromDB(row)
	policy, err := s.drivePolicy(ctx, actor.TenantID)
	if err != nil {
		return err
	}
	if file.DLPBlocked || file.ScanStatus == "infected" || file.ScanStatus == "blocked" || (policy.ContentScanEnabled && policy.BlockShareUntilScanComplete && file.ScanStatus == "pending") {
		s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.share.denied_scan", "drive_file", file.PublicID, map[string]any{
			"scanStatus": file.ScanStatus,
			"dlpBlocked": file.DLPBlocked,
		})
		return ErrDrivePolicyDenied
	}
	return nil
}

func (s *DriveService) resolveWorkspaceForCreate(ctx context.Context, tenantID int64, actor DriveActor, workspacePublicID string, parent *DriveFolder) (DriveWorkspace, error) {
	if parent != nil && parent.WorkspaceID != nil {
		return s.getWorkspaceByID(ctx, tenantID, *parent.WorkspaceID)
	}
	if strings.TrimSpace(workspacePublicID) != "" {
		return s.getWorkspaceByPublicID(ctx, tenantID, workspacePublicID)
	}
	return s.ensureDefaultWorkspace(ctx, tenantID, actor)
}

func (s *DriveService) ensureDefaultWorkspace(ctx context.Context, tenantID int64, actor DriveActor) (DriveWorkspace, error) {
	row, err := s.queries.GetDefaultDriveWorkspace(ctx, tenantID)
	if err == nil {
		return driveWorkspaceFromDB(row), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return DriveWorkspace{}, fmt.Errorf("get default drive workspace: %w", err)
	}
	row, err = s.queries.CreateDriveWorkspace(ctx, db.CreateDriveWorkspaceParams{
		TenantID:          tenantID,
		Name:              "Default workspace",
		CreatedByUserID:   pgtype.Int8{Int64: actor.UserID, Valid: true},
		StorageQuotaBytes: pgtype.Int8{},
		PolicyOverride:    []byte(`{}`),
	})
	if err != nil {
		return DriveWorkspace{}, fmt.Errorf("create default drive workspace: %w", err)
	}
	workspace := driveWorkspaceFromDB(row)
	_ = s.authz.WriteWorkspaceOwner(ctx, actor, workspace)
	return workspace, nil
}

func (s *DriveService) getWorkspaceByID(ctx context.Context, tenantID, workspaceID int64) (DriveWorkspace, error) {
	row, err := s.queries.GetDriveWorkspaceByIDForTenant(ctx, db.GetDriveWorkspaceByIDForTenantParams{ID: workspaceID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveWorkspace{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveWorkspace{}, fmt.Errorf("get drive workspace: %w", err)
	}
	return driveWorkspaceFromDB(row), nil
}

func (s *DriveService) getWorkspaceByPublicID(ctx context.Context, tenantID int64, workspacePublicID string) (DriveWorkspace, error) {
	publicID, err := uuid.Parse(strings.TrimSpace(workspacePublicID))
	if err != nil {
		return DriveWorkspace{}, ErrDriveNotFound
	}
	row, err := s.queries.GetDriveWorkspaceByPublicIDForTenant(ctx, db.GetDriveWorkspaceByPublicIDForTenantParams{PublicID: publicID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveWorkspace{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveWorkspace{}, fmt.Errorf("get drive workspace: %w", err)
	}
	return driveWorkspaceFromDB(row), nil
}

func (s *DriveService) ensureDriveContentAdmin(ctx context.Context, userID int64) error {
	roles, err := s.queries.ListRoleCodesByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("list user roles: %w", err)
	}
	hasTenantAdmin := false
	hasDriveContentAdmin := false
	for _, role := range roles {
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "tenant_admin":
			hasTenantAdmin = true
		case "drive_content_admin":
			hasDriveContentAdmin = true
		}
	}
	if !hasTenantAdmin || !hasDriveContentAdmin {
		return ErrDrivePermissionDenied
	}
	return nil
}

func (s *DriveService) adminDriveFileAccess(ctx context.Context, tenantID, actorUserID int64, filePublicID string, auditCtx AuditContext, eventSuffix string) (DriveFile, DriveActor, error) {
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveFile{}, DriveActor{}, err
	}
	if err := s.ensureDriveContentAdmin(ctx, actor.UserID); err != nil {
		return DriveFile{}, DriveActor{}, err
	}
	policy, err := s.drivePolicy(ctx, tenantID)
	if err != nil {
		return DriveFile{}, DriveActor{}, err
	}
	if policy.AdminContentAccessMode != "break_glass" {
		return DriveFile{}, DriveActor{}, ErrDrivePolicyDenied
	}
	session, err := s.activeAdminContentAccessSession(ctx, tenantID, actor.UserID)
	if err != nil {
		s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.admin_content_access.denied", "drive_file", filePublicID, map[string]any{"reason": "missing_session"})
		return DriveFile{}, DriveActor{}, err
	}
	row, err := s.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID})
	if err != nil {
		return DriveFile{}, DriveActor{}, err
	}
	file := driveFileFromDB(row)
	if file.DeletedAt != nil || file.LockedAt != nil {
		return DriveFile{}, DriveActor{}, ErrDriveLocked
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.admin_content_access."+eventSuffix, "drive_file", file.PublicID, map[string]any{
		"sessionPublicId": session.PublicID,
		"reasonCategory":  session.ReasonCategory,
	})
	return file, actor, nil
}

func (s *DriveService) activeAdminContentAccessSession(ctx context.Context, tenantID, actorUserID int64) (DriveAdminContentAccessSession, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, public_id, tenant_id, actor_user_id, reason, reason_category, expires_at, ended_at, created_at
		FROM drive_admin_content_access_sessions
		WHERE tenant_id = $1
		  AND actor_user_id = $2
		  AND ended_at IS NULL
		  AND expires_at > now()
		ORDER BY expires_at DESC, id DESC
		LIMIT 1
	`, tenantID, actorUserID)
	session, err := scanDriveAdminContentAccessSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveAdminContentAccessSession{}, ErrDrivePermissionDenied
	}
	return session, err
}

func scanDriveAdminContentAccessSession(row pgx.Row) (DriveAdminContentAccessSession, error) {
	var session DriveAdminContentAccessSession
	var publicID uuid.UUID
	var endedAt pgtype.Timestamptz
	var expiresAt pgtype.Timestamptz
	var createdAt pgtype.Timestamptz
	if err := row.Scan(&session.ID, &publicID, &session.TenantID, &session.ActorUserID, &session.Reason, &session.ReasonCategory, &expiresAt, &endedAt, &createdAt); err != nil {
		return DriveAdminContentAccessSession{}, err
	}
	session.PublicID = publicID.String()
	session.ExpiresAt = expiresAt.Time
	session.EndedAt = optionalPgTime(endedAt)
	session.CreatedAt = createdAt.Time
	return session, nil
}

func normalizeAdminContentReasonCategory(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "incident", "legal", "security":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "manual"
	}
}

func driveInitialScanStatus(policy DrivePolicy) string {
	if policy.ContentScanEnabled {
		return "pending"
	}
	return "skipped"
}

func (s *DriveService) storageMissingObjectCount(ctx context.Context, tenantID int64, limit int) (int, error) {
	if s.storage == nil {
		return 0, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT storage_key
		FROM file_objects
		WHERE tenant_id = $1
		  AND purpose = 'drive'
		  AND deleted_at IS NULL
		  AND upload_state = 'active'
		ORDER BY id DESC
		LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return 0, fmt.Errorf("list drive storage keys: %w", err)
	}
	defer rows.Close()
	missing := 0
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return 0, err
		}
		if _, err := s.storage.HeadObject(ctx, key); err != nil {
			missing++
		}
	}
	return missing, rows.Err()
}

func (s *DriveService) publicShareLinkPasswordVerified(ctx context.Context, link DriveShareLink, verificationCookie string) bool {
	if !link.PasswordRequired {
		return true
	}
	state, err := s.getShareLinkPasswordState(ctx, driveShareLinkTokenHash(link.RawToken))
	if err != nil {
		state, err = s.getShareLinkPasswordStateByPublicID(ctx, link.PublicID)
		if err != nil {
			return false
		}
	}
	return verifyShareLinkVerificationCookie(state, verificationCookie, s.now)
}

func (s *DriveService) getShareLinkPasswordStateByPublicID(ctx context.Context, publicID string) (shareLinkPasswordState, error) {
	var state shareLinkPasswordState
	var passwordUpdatedAt pgtype.Timestamptz
	err := s.pool.QueryRow(ctx, `
SELECT id, public_id::text, tenant_id, token_hash, password_required, COALESCE(password_hash, ''),
       password_updated_at, status, expires_at
FROM drive_share_links
WHERE public_id = $1::uuid
  AND status = 'active'
  AND expires_at > now()`, publicID).Scan(
		&state.ID,
		&state.PublicID,
		&state.TenantID,
		&state.TokenHash,
		&state.PasswordRequired,
		&state.PasswordHash,
		&passwordUpdatedAt,
		&state.Status,
		&state.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return shareLinkPasswordState{}, ErrDriveNotFound
	}
	if err != nil {
		return shareLinkPasswordState{}, fmt.Errorf("get share link password state: %w", err)
	}
	if passwordUpdatedAt.Valid {
		state.PasswordUpdatedAt = passwordUpdatedAt.Time
	}
	return state, nil
}

func compactJSONMap(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return []byte(`{}`)
	}
	out, err := json.Marshal(decoded)
	if err != nil {
		return []byte(`{}`)
	}
	return out
}

func closeQuietly(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}
