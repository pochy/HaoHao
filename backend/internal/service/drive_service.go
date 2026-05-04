package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	"github.com/jackc/pgx/v5/pgxpool"
)

type DriveService struct {
	pool           *pgxpool.Pool
	queries        *db.Queries
	files          *FileService
	storage        FileStorage
	authz          *DriveAuthorizationService
	tenantSettings *TenantSettingsService
	outbox         *OutboxService
	audit          AuditRecorder
	medallion      *MedallionCatalogService
	localSearch    *LocalSearchService
	now            func() time.Time
}

func NewDriveService(pool *pgxpool.Pool, queries *db.Queries, files *FileService, storage FileStorage, authz *DriveAuthorizationService, tenantSettings *TenantSettingsService, audit AuditRecorder) *DriveService {
	if storage == nil && files != nil {
		storage = files.storage
	}
	now := time.Now
	return &DriveService{
		pool:           pool,
		queries:        queries,
		files:          files,
		storage:        storage,
		authz:          authz,
		tenantSettings: tenantSettings,
		audit:          audit,
		now:            now,
	}
}

func (s *DriveService) SetOutboxService(outbox *OutboxService) {
	if s != nil {
		s.outbox = outbox
	}
}

func (s *DriveService) SetMedallionCatalogService(medallion *MedallionCatalogService) {
	if s != nil {
		s.medallion = medallion
	}
}

func (s *DriveService) SetLocalSearchService(localSearch *LocalSearchService) {
	if s != nil {
		s.localSearch = localSearch
	}
}

func (s *DriveService) CreateFolder(ctx context.Context, input DriveCreateFolderInput, auditCtx AuditContext) (DriveFolder, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveFolder{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveFolder{}, err
	}
	name := normalizeDriveName(input.Name)
	if name == "" {
		return DriveFolder{}, fmt.Errorf("%w: folder name is required", ErrDriveInvalidInput)
	}

	var parentRef *DriveResourceRef
	var parentFolderForWorkspace *DriveFolder
	parentID := pgtype.Int8{}
	parentPublicIDInput := strings.TrimSpace(input.ParentFolderPublicID)
	if input.ParentFolderID != nil || (parentPublicIDInput != "" && parentPublicIDInput != "root") {
		var parent DriveFolder
		var err error
		if input.ParentFolderID != nil {
			parent, err = s.getFolderByID(ctx, input.TenantID, *input.ParentFolderID)
		} else {
			parent, err = s.getDriveFolder(ctx, input.TenantID, DriveResourceRef{Type: DriveResourceTypeFolder, PublicID: parentPublicIDInput})
		}
		if err != nil {
			return DriveFolder{}, err
		}
		if err := s.authz.CanEditFolder(ctx, actor, parent); err != nil {
			s.auditDenied(ctx, actor, "drive.folder.create", "folder", parent.PublicID, err, auditCtx)
			return DriveFolder{}, err
		}
		ref := parent.ResourceRef()
		parentRef = &ref
		parentFolderForWorkspace = &parent
		parentID = pgtype.Int8{Int64: parent.ID, Valid: true}
	}
	workspace, err := s.resolveWorkspaceForCreate(ctx, input.TenantID, actor, input.WorkspacePublicID, parentFolderForWorkspace)
	if err != nil {
		return DriveFolder{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DriveFolder{}, fmt.Errorf("begin drive folder transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	row, err := qtx.CreateDriveFolder(ctx, db.CreateDriveFolderParams{
		TenantID:           input.TenantID,
		WorkspaceID:        pgtype.Int8{Int64: workspace.ID, Valid: true},
		ParentFolderID:     parentID,
		Name:               name,
		CreatedByUserID:    actor.UserID,
		InheritanceEnabled: true,
	})
	if err != nil {
		return DriveFolder{}, fmt.Errorf("create drive folder: %w", err)
	}
	folder := driveFolderFromDB(row)
	if err := s.recordAuditWithQueries(ctx, qtx, auditCtx, "drive.folder.create", "drive_folder", folder.PublicID, map[string]any{
		"name": name,
	}); err != nil {
		return DriveFolder{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DriveFolder{}, fmt.Errorf("commit drive folder transaction: %w", err)
	}

	if err := s.authz.WriteResourceCreateTuplesWithWorkspace(ctx, actor, folder.ResourceRef(), parentRef, &workspace); err != nil {
		_, _ = s.queries.SoftDeleteDriveFolder(context.Background(), db.SoftDeleteDriveFolderParams{
			DeletedByUserID: pgtype.Int8{Int64: actor.UserID, Valid: true},
			ID:              folder.ID,
			TenantID:        folder.TenantID,
		})
		s.auditFailed(ctx, actor, "drive.folder.create", "drive_folder", folder.PublicID, err, auditCtx)
		return DriveFolder{}, err
	}
	s.recordDriveActivityBestEffort(ctx, actor, folder.ResourceRef(), "updated", map[string]any{"name": folder.Name, "operation": "created"})
	s.recordDriveSyncEventBestEffort(ctx, folder.ResourceRef(), "folder.created", "", map[string]any{"name": folder.Name})
	return folder, nil
}

func (s *DriveService) UploadFile(ctx context.Context, input DriveUploadFileInput, auditCtx AuditContext) (DriveFile, error) {
	if err := s.ensureConfigured(true); err != nil {
		return DriveFile{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveFile{}, err
	}
	filename := normalizeDriveName(filepath.Base(strings.TrimSpace(input.Filename)))
	if filename == "" || filename == "." {
		return DriveFile{}, NewDriveCodedError(ErrDriveInvalidInput, DriveErrorFilenameRequired, "Filename is required.")
	}
	if input.Body == nil {
		return DriveFile{}, NewDriveCodedError(ErrDriveInvalidInput, DriveErrorFileRequired, "File body is required.")
	}

	var parentRef *DriveResourceRef
	var parentFolderForWorkspace *DriveFolder
	parentID := pgtype.Int8{}
	parentPublicIDInput := strings.TrimSpace(input.ParentFolderPublicID)
	if input.ParentFolderID != nil || (parentPublicIDInput != "" && parentPublicIDInput != "root") {
		var parent DriveFolder
		var err error
		if input.ParentFolderID != nil {
			parent, err = s.getFolderByID(ctx, input.TenantID, *input.ParentFolderID)
		} else {
			parent, err = s.getDriveFolder(ctx, input.TenantID, DriveResourceRef{Type: DriveResourceTypeFolder, PublicID: parentPublicIDInput})
		}
		if err != nil {
			if errors.Is(err, ErrDriveNotFound) {
				return DriveFile{}, NewDriveCodedError(err, DriveErrorParentFolderNotFound, "Parent folder was not found.")
			}
			return DriveFile{}, err
		}
		if err := s.authz.CanEditFolder(ctx, actor, parent); err != nil {
			s.auditDenied(ctx, actor, "drive.file.create", "drive_folder", parent.PublicID, err, auditCtx)
			return DriveFile{}, err
		}
		ref := parent.ResourceRef()
		parentRef = &ref
		parentFolderForWorkspace = &parent
		parentID = pgtype.Int8{Int64: parent.ID, Valid: true}
	}
	workspace, err := s.resolveWorkspaceForCreate(ctx, input.TenantID, actor, input.WorkspacePublicID, parentFolderForWorkspace)
	if err != nil {
		if errors.Is(err, ErrDriveNotFound) && strings.TrimSpace(input.WorkspacePublicID) != "" {
			return DriveFile{}, NewDriveCodedError(err, DriveErrorWorkspaceNotFound, "Workspace was not found.")
		}
		return DriveFile{}, err
	}

	contentType := normalizeContentType(input.ContentType)
	policy, err := s.drivePolicy(ctx, input.TenantID)
	if err != nil {
		return DriveFile{}, err
	}
	maxBytes := driveEffectiveUploadMaxBytes(fileServiceMaxBytes(s.files), input.MaxBytes, policy.MaxFileSizeBytes)
	storageKey := newDriveStorageKey(input.TenantID, workspace.PublicID, 1)
	stored, err := s.storage.PutObject(ctx, storageKey, input.Body, maxBytes, ObjectPutOptions{
		ContentType: contentType,
	})
	if err != nil {
		if errors.Is(err, ErrFileTooLarge) {
			return DriveFile{}, NewDriveCodedError(ErrInvalidFileInput, DriveErrorFileTooLarge, fmt.Sprintf("File exceeds the Drive upload limit of %s.", formatDriveByteLimit(maxBytes)))
		}
		return DriveFile{}, err
	}
	if s.tenantSettings != nil {
		ok, _, _, err := s.tenantSettings.CheckFileQuota(ctx, input.TenantID, stored.Size)
		if err != nil {
			_ = s.storage.Delete(ctx, stored.Key)
			return DriveFile{}, err
		}
		if !ok {
			_ = s.storage.Delete(ctx, stored.Key)
			if s.files != nil && s.files.metrics != nil {
				s.files.metrics.IncFileQuotaExceeded("drive")
			}
			return DriveFile{}, NewDriveCodedError(ErrFileQuotaExceeded, DriveErrorQuotaExceeded, "Tenant file quota is exceeded.")
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveFile{}, fmt.Errorf("begin drive file transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()
	qtx := s.queries.WithTx(tx)
	row, err := qtx.CreateDriveFileObject(ctx, db.CreateDriveFileObjectParams{
		TenantID:           input.TenantID,
		UploadedByUserID:   pgtype.Int8{Int64: actor.UserID, Valid: true},
		WorkspaceID:        pgtype.Int8{Int64: workspace.ID, Valid: true},
		DriveFolderID:      parentID,
		OriginalFilename:   filename,
		ContentType:        contentType,
		ByteSize:           stored.Size,
		Sha256Hex:          stored.SHA256Hex,
		StorageDriver:      storageDriverForStoredFile(s.storage, stored),
		StorageKey:         stored.Key,
		StorageBucket:      pgText(stored.Bucket),
		Etag:               stored.ETag,
		ScanStatus:         driveInitialScanStatus(policy),
		InheritanceEnabled: true,
	})
	if err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveFile{}, fmt.Errorf("create drive file object: %w", err)
	}
	file := driveFileFromDB(row)
	if err := s.recordAuditWithQueries(ctx, qtx, auditCtx, "drive.file.create", "drive_file", file.PublicID, map[string]any{
		"contentType": file.ContentType,
		"byteSize":    file.ByteSize,
	}); err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveFile{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveFile{}, fmt.Errorf("commit drive file transaction: %w", err)
	}

	if err := s.authz.WriteResourceCreateTuplesWithWorkspace(ctx, actor, file.ResourceRef(), parentRef, &workspace); err != nil {
		_, _ = s.queries.SoftDeleteDriveFile(context.Background(), db.SoftDeleteDriveFileParams{
			DeletedByUserID: pgtype.Int8{Int64: actor.UserID, Valid: true},
			ID:              file.ID,
			TenantID:        file.TenantID,
		})
		s.auditFailed(ctx, actor, "drive.file.create", "drive_file", file.PublicID, err, auditCtx)
		return DriveFile{}, err
	}
	s.recordDriveActivityBestEffort(ctx, actor, file.ResourceRef(), "uploaded", map[string]any{"filename": file.OriginalFilename, "byteSize": file.ByteSize})
	s.recordDriveFilePreviewStateBestEffort(ctx, file)
	s.indexDriveFileBestEffort(ctx, file, "file_created")
	s.enqueueDriveOCRBestEffort(ctx, actor, file, "upload")
	if s.medallion != nil {
		actorID := actor.UserID
		_, _, _ = s.medallion.EnsureDriveFileAsset(ctx, file, &actorID)
	}
	s.recordDriveSyncEventBestEffort(ctx, file.ResourceRef(), "file.created", file.SHA256Hex, map[string]any{"filename": file.OriginalFilename})
	return file, nil
}

func (s *DriveService) CreateShare(ctx context.Context, input DriveCreateShareInput, auditCtx AuditContext) (DriveShare, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveShare{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveShare{}, err
	}
	resource, err := s.resolveShareableResource(ctx, actor, input.Resource)
	if err != nil {
		return DriveShare{}, err
	}
	if err := s.ensureResourceShareAllowed(ctx, actor, resource, auditCtx); err != nil {
		return DriveShare{}, err
	}
	role := normalizeDriveRole(input.Role)
	if role == "" {
		return DriveShare{}, fmt.Errorf("%w: share role is required", ErrDriveInvalidInput)
	}

	subjectID := input.SubjectID
	if subjectID <= 0 && strings.TrimSpace(input.SubjectPublicID) != "" {
		subjectID, err = s.resolveShareSubjectID(ctx, input.TenantID, input.SubjectType, input.SubjectPublicID)
		if err != nil {
			return DriveShare{}, err
		}
	}
	subjectPublicID, err := s.resolveShareSubjectPublicID(ctx, input.TenantID, input.SubjectType, subjectID)
	if err != nil {
		return DriveShare{}, err
	}
	row, err := s.queries.CreateDriveResourceShare(ctx, db.CreateDriveResourceShareParams{
		TenantID:        input.TenantID,
		ResourceType:    string(resource.Type),
		ResourceID:      resource.ID,
		SubjectType:     string(input.SubjectType),
		SubjectID:       subjectID,
		Role:            string(role),
		Status:          "active",
		CreatedByUserID: actor.UserID,
	})
	if err != nil {
		return DriveShare{}, fmt.Errorf("create drive share: %w", err)
	}
	share := driveShareFromDB(row, resource, subjectPublicID)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.share.create", "drive_share", share.PublicID, map[string]any{
		"resourceType": share.Resource.Type,
		"role":         share.Role,
		"subjectType":  share.SubjectType,
	})
	if err := s.authz.WriteShareTuple(ctx, share); err != nil {
		_, _ = s.queries.MarkDriveResourceSharePendingSync(context.Background(), db.MarkDriveResourceSharePendingSyncParams{
			ID:       share.ID,
			TenantID: share.TenantID,
		})
		s.auditFailed(ctx, actor, "drive.share.create", "drive_share", share.PublicID, err, auditCtx)
		return DriveShare{}, err
	}
	s.recordDriveActivityBestEffort(ctx, actor, share.Resource, "shared", map[string]any{
		"subjectType": share.SubjectType,
		"role":        share.Role,
	})
	return share, nil
}

func (s *DriveService) RevokeShare(ctx context.Context, input DriveRevokeShareInput, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return err
	}
	shareID, err := uuid.Parse(strings.TrimSpace(input.ShareID))
	if err != nil {
		return ErrDriveNotFound
	}
	row, err := s.queries.GetDriveResourceShareByPublicIDForTenant(ctx, db.GetDriveResourceShareByPublicIDForTenantParams{
		PublicID: shareID,
		TenantID: input.TenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	}
	if err != nil {
		return fmt.Errorf("get drive share: %w", err)
	}
	resource, err := s.resolveShareableResource(ctx, actor, DriveResourceRef{Type: DriveResourceType(row.ResourceType), ID: row.ResourceID, TenantID: row.TenantID})
	if err != nil {
		return err
	}
	subjectPublicID, err := s.resolveShareSubjectPublicID(ctx, row.TenantID, DriveShareSubjectType(row.SubjectType), row.SubjectID)
	if err != nil {
		return err
	}
	share := driveShareFromDB(row, resource, subjectPublicID)
	if err := s.authz.DeleteShareTuple(ctx, share); err != nil {
		return err
	}
	_, err = s.queries.RevokeDriveResourceShare(ctx, db.RevokeDriveResourceShareParams{
		RevokedByUserID: pgtype.Int8{Int64: actor.UserID, Valid: true},
		PublicID:        shareID,
		TenantID:        input.TenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	}
	if err != nil {
		return fmt.Errorf("revoke drive share: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.share.revoke", "drive_share", share.PublicID, map[string]any{
		"resourceType": share.Resource.Type,
		"role":         share.Role,
		"subjectType":  share.SubjectType,
	})
	s.recordDriveActivityBestEffort(ctx, actor, share.Resource, "unshared", map[string]any{"sharePublicId": share.PublicID})
	return nil
}

func (s *DriveService) CreateGroup(ctx context.Context, tenantID, actorUserID int64, name, description string, auditCtx AuditContext) (DriveGroup, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveGroup{}, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveGroup{}, err
	}
	name = normalizeDriveName(name)
	if name == "" {
		return DriveGroup{}, fmt.Errorf("%w: group name is required", ErrDriveInvalidInput)
	}
	row, err := s.queries.CreateDriveGroup(ctx, db.CreateDriveGroupParams{
		TenantID:        tenantID,
		Name:            name,
		Description:     strings.TrimSpace(description),
		CreatedByUserID: actor.UserID,
	})
	if err != nil {
		return DriveGroup{}, fmt.Errorf("create drive group: %w", err)
	}
	group := driveGroupFromDB(row)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.group.create", "drive_group", group.PublicID, nil)
	return group, nil
}

func (s *DriveService) AddGroupMember(ctx context.Context, tenantID, actorUserID, groupID, userID int64, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	group, err := s.getGroupByID(ctx, tenantID, groupID)
	if err != nil {
		return err
	}
	userPublicID, err := s.ensureUserInTenant(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	member, err := s.queries.AddDriveGroupMember(ctx, db.AddDriveGroupMemberParams{
		GroupID:       group.ID,
		UserID:        userID,
		AddedByUserID: actor.UserID,
	})
	if err != nil {
		return fmt.Errorf("add drive group member: %w", err)
	}
	if err := s.authz.WriteGroupMemberTuple(ctx, group, userPublicID); err != nil {
		_, _ = s.queries.RemoveDriveGroupMember(context.Background(), db.RemoveDriveGroupMemberParams{GroupID: group.ID, UserID: userID})
		s.auditFailed(ctx, actor, "drive.group_member.add", "drive_group_member", fmt.Sprintf("%d", member.ID), err, auditCtx)
		return err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.group_member.add", "drive_group", group.PublicID, map[string]any{
		"userId": userID,
	})
	return nil
}

func (s *DriveService) RemoveGroupMember(ctx context.Context, tenantID, actorUserID, groupID, userID int64, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	group, err := s.getGroupByID(ctx, tenantID, groupID)
	if err != nil {
		return err
	}
	userPublicID, err := s.ensureUserInTenant(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if err := s.authz.DeleteGroupMemberTuple(ctx, group, userPublicID); err != nil {
		return err
	}
	if _, err := s.queries.RemoveDriveGroupMember(ctx, db.RemoveDriveGroupMemberParams{GroupID: group.ID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	} else if err != nil {
		return fmt.Errorf("remove drive group member: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.group_member.remove", "drive_group", group.PublicID, map[string]any{
		"userId": userID,
	})
	return nil
}

func (s *DriveService) CreateShareLink(ctx context.Context, input DriveCreateShareLinkInput, auditCtx AuditContext) (DriveShareLink, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveShareLink{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveShareLink{}, err
	}
	policy, err := s.drivePolicy(ctx, input.TenantID)
	if err != nil {
		return DriveShareLink{}, err
	}
	if !policy.LinkSharingEnabled || !policy.PublicLinksEnabled {
		return DriveShareLink{}, ErrDrivePolicyDenied
	}
	role := normalizeDriveRole(input.Role)
	if role == "" {
		role = DriveRoleViewer
	}
	if role == DriveRoleOwner {
		return DriveShareLink{}, fmt.Errorf("%w: share link role is not supported", ErrDriveInvalidInput)
	}
	if role == DriveRoleEditor {
		if !policy.AnonymousEditorLinksEnabled {
			return DriveShareLink{}, ErrDrivePolicyDenied
		}
		if input.Resource.Type != DriveResourceTypeFile {
			return DriveShareLink{}, fmt.Errorf("%w: editor links are file-only", ErrDriveInvalidInput)
		}
		if policy.AnonymousEditorLinksRequirePassword && strings.TrimSpace(input.Password) == "" {
			return DriveShareLink{}, fmt.Errorf("%w: editor link password is required", ErrDriveInvalidInput)
		}
		if input.ExpiresAt.IsZero() || input.ExpiresAt.After(s.now().Add(time.Duration(policy.AnonymousEditorLinkMaxTTLMinutes)*time.Minute)) {
			return DriveShareLink{}, ErrDrivePolicyDenied
		}
	}
	if policy.RequireShareLinkPassword && strings.TrimSpace(input.Password) == "" {
		return DriveShareLink{}, fmt.Errorf("%w: share link password is required", ErrDriveInvalidInput)
	}
	if strings.TrimSpace(input.Password) != "" && !policy.PasswordProtectedLinksEnabled {
		return DriveShareLink{}, ErrDrivePolicyDenied
	}
	if input.ExpiresAt.IsZero() {
		ttlHours := policy.MaxShareLinkTTLHours
		if ttlHours <= 0 {
			ttlHours = defaultDrivePolicy().MaxShareLinkTTLHours
		}
		input.ExpiresAt = s.now().Add(time.Duration(ttlHours) * time.Hour)
	}
	if !input.ExpiresAt.IsZero() && policy.MaxShareLinkTTLHours > 0 && input.ExpiresAt.After(s.now().Add(time.Duration(policy.MaxShareLinkTTLHours)*time.Hour)) {
		return DriveShareLink{}, ErrDrivePolicyDenied
	}
	resource, err := s.resolveShareableResource(ctx, actor, input.Resource)
	if err != nil {
		return DriveShareLink{}, err
	}
	if err := s.ensureResourceShareAllowed(ctx, actor, resource, auditCtx); err != nil {
		return DriveShareLink{}, err
	}
	var activeLinkCount int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM drive_share_links WHERE tenant_id = $1 AND status = 'active'`, input.TenantID).Scan(&activeLinkCount); err != nil {
		return DriveShareLink{}, fmt.Errorf("count drive share links: %w", err)
	}
	if policy.MaxPublicLinkCount > 0 && int(activeLinkCount) >= policy.MaxPublicLinkCount {
		s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.policy.enforcement_denied", "drive_share_link", "new", map[string]any{
			"feature": "public_link_count",
			"plan":    policy.PlanCode,
		})
		return DriveShareLink{}, ErrDrivePolicyDenied
	}
	rawToken, tokenHash, err := newDriveShareLinkToken()
	if err != nil {
		return DriveShareLink{}, err
	}
	canDownload := input.CanDownload && policy.ViewerDownloadEnabled && role == DriveRoleViewer
	row, err := s.queries.CreateDriveShareLink(ctx, db.CreateDriveShareLinkParams{
		TenantID:        input.TenantID,
		ResourceType:    string(resource.Type),
		ResourceID:      resource.ID,
		TokenHash:       tokenHash,
		Role:            string(role),
		CanDownload:     canDownload,
		ExpiresAt:       pgTimestamp(input.ExpiresAt),
		Status:          "active",
		CreatedByUserID: actor.UserID,
	})
	if err != nil {
		return DriveShareLink{}, fmt.Errorf("create drive share link: %w", err)
	}
	link := driveShareLinkFromDB(row, resource)
	link.RawToken = rawToken
	if strings.TrimSpace(input.Password) != "" {
		if err := s.setShareLinkPassword(ctx, link.ID, link.TenantID, input.Password); err != nil {
			return DriveShareLink{}, err
		}
		link.PasswordRequired = true
	}
	if err := s.authz.WriteShareLinkTuple(ctx, link); err != nil {
		_, _ = s.queries.MarkDriveShareLinkPendingSync(context.Background(), db.MarkDriveShareLinkPendingSyncParams{ID: link.ID, TenantID: link.TenantID})
		s.auditFailed(ctx, actor, "drive.share_link.create", "drive_share_link", link.PublicID, err, auditCtx)
		return DriveShareLink{}, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.share_link.create", "drive_share_link", link.PublicID, map[string]any{
		"resourceType":     resource.Type,
		"role":             role,
		"canDownload":      canDownload,
		"passwordRequired": link.PasswordRequired,
		"expiresAt":        link.ExpiresAt,
	})
	return link, nil
}

func (s *DriveService) DisableShareLink(ctx context.Context, input DriveDisableShareLinkInput, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return err
	}
	linkID, err := uuid.Parse(strings.TrimSpace(input.ShareLinkID))
	if err != nil {
		return ErrDriveNotFound
	}
	row, err := s.queries.GetDriveShareLinkByPublicIDForTenant(ctx, db.GetDriveShareLinkByPublicIDForTenantParams{PublicID: linkID, TenantID: input.TenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	}
	if err != nil {
		return fmt.Errorf("get drive share link: %w", err)
	}
	resource, err := s.resolveShareableResource(ctx, actor, DriveResourceRef{Type: DriveResourceType(row.ResourceType), ID: row.ResourceID, TenantID: row.TenantID})
	if err != nil {
		return err
	}
	link := driveShareLinkFromDB(row, resource)
	if err := s.authz.DeleteShareLinkTuple(ctx, link); err != nil {
		return err
	}
	if _, err := s.queries.DisableDriveShareLink(ctx, db.DisableDriveShareLinkParams{
		DisabledByUserID: pgtype.Int8{Int64: actor.UserID, Valid: true},
		PublicID:         linkID,
		TenantID:         input.TenantID,
	}); errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	} else if err != nil {
		return fmt.Errorf("disable drive share link: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.share_link.disable", "drive_share_link", link.PublicID, map[string]any{
		"resourceType": resource.Type,
	})
	return nil
}

func (s *DriveService) StopInheritance(ctx context.Context, tenantID, actorUserID int64, resource DriveResourceRef, auditCtx AuditContext) error {
	return s.setInheritance(ctx, tenantID, actorUserID, resource, false, auditCtx)
}

func (s *DriveService) ResumeInheritance(ctx context.Context, tenantID, actorUserID int64, resource DriveResourceRef, auditCtx AuditContext) error {
	return s.setInheritance(ctx, tenantID, actorUserID, resource, true, auditCtx)
}

func (s *DriveService) setInheritance(ctx context.Context, tenantID, actorUserID int64, resource DriveResourceRef, enabled bool, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	resolved, parent, err := s.resolveResourceAndParent(ctx, actor, resource)
	if err != nil {
		return err
	}
	if enabled {
		err = s.authz.WriteResourceParent(ctx, resolved, parent)
	} else {
		err = s.authz.DeleteResourceParent(ctx, resolved, parent)
	}
	if err != nil {
		return err
	}
	if _, err := s.updateInheritanceFlag(ctx, tenantID, resolved, parent, enabled); err != nil {
		return err
	}
	action := "drive.inheritance.stop"
	if enabled {
		action = "drive.inheritance.resume"
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, action, "drive_"+string(resolved.Type), resolved.PublicID, nil)
	return nil
}

func (s *DriveService) ensureConfigured(requireStorage bool) error {
	if s == nil || s.pool == nil || s.queries == nil || s.authz == nil {
		return fmt.Errorf("drive service is not configured")
	}
	if requireStorage && s.storage == nil {
		return fmt.Errorf("drive storage is not configured")
	}
	return nil
}

func (s *DriveService) actor(ctx context.Context, tenantID, userID int64) (DriveActor, error) {
	if tenantID <= 0 || userID <= 0 {
		return DriveActor{}, ErrDrivePermissionDenied
	}
	publicID, platformAdmin, err := s.ensureActorInTenant(ctx, tenantID, userID)
	if err != nil {
		return DriveActor{}, err
	}
	return DriveActor{UserID: userID, PublicID: publicID, TenantID: tenantID, PlatformAdmin: platformAdmin}, nil
}

func (s *DriveService) ensureActorInTenant(ctx context.Context, tenantID, userID int64) (string, bool, error) {
	publicID, err := s.ensureUserInTenant(ctx, tenantID, userID)
	if err == nil {
		return publicID, false, nil
	}
	if !errors.Is(err, ErrDrivePermissionDenied) {
		return "", false, err
	}

	user, userErr := s.queries.GetUserByID(ctx, userID)
	if errors.Is(userErr, pgx.ErrNoRows) {
		return "", false, ErrDrivePermissionDenied
	}
	if userErr != nil {
		return "", false, fmt.Errorf("get user: %w", userErr)
	}
	if user.DeactivatedAt.Valid {
		return "", false, ErrDrivePermissionDenied
	}
	roles, roleErr := s.queries.ListRoleCodesByUserID(ctx, userID)
	if roleErr != nil {
		return "", false, fmt.Errorf("list user roles: %w", roleErr)
	}
	if !driveUserIsPlatformTenantAdmin(roles) {
		return "", false, ErrDrivePermissionDenied
	}
	tenant, tenantErr := s.queries.GetTenantByID(ctx, tenantID)
	if errors.Is(tenantErr, pgx.ErrNoRows) {
		return "", false, ErrDrivePermissionDenied
	}
	if tenantErr != nil {
		return "", false, fmt.Errorf("get tenant: %w", tenantErr)
	}
	if !tenant.Active {
		return "", false, ErrDrivePermissionDenied
	}
	return user.PublicID.String(), true, nil
}

func (s *DriveService) ensureUserInTenant(ctx context.Context, tenantID, userID int64) (string, error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrDrivePermissionDenied
	}
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}
	if user.DeactivatedAt.Valid {
		return "", ErrDrivePermissionDenied
	}
	memberships, err := s.queries.ListTenantMembershipRowsByUserID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("list tenant memberships: %w", err)
	}
	for _, membership := range memberships {
		if membership.TenantID == tenantID && membership.TenantActive && membership.MembershipActive {
			return user.PublicID.String(), nil
		}
	}
	return "", ErrDrivePermissionDenied
}

func driveUserIsPlatformTenantAdmin(roleCodes []string) bool {
	return hasRoleCode(roleCodes, "tenant_admin")
}

func (s *DriveService) getFolderByID(ctx context.Context, tenantID, folderID int64) (DriveFolder, error) {
	row, err := s.queries.GetDriveFolderByIDForTenant(ctx, db.GetDriveFolderByIDForTenantParams{ID: folderID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveFolder{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveFolder{}, fmt.Errorf("get drive folder: %w", err)
	}
	return driveFolderFromDB(row), nil
}

func (s *DriveService) getGroupByID(ctx context.Context, tenantID, groupID int64) (DriveGroup, error) {
	row, err := s.queries.GetDriveGroupByIDForTenant(ctx, db.GetDriveGroupByIDForTenantParams{ID: groupID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveGroup{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveGroup{}, fmt.Errorf("get drive group: %w", err)
	}
	return driveGroupFromDB(row), nil
}

func (s *DriveService) resolveShareableResource(ctx context.Context, actor DriveActor, ref DriveResourceRef) (DriveResourceRef, error) {
	switch ref.Type {
	case DriveResourceTypeFile:
		row, err := s.getDriveFileRow(ctx, actor.TenantID, ref)
		if err != nil {
			return DriveResourceRef{}, err
		}
		file := driveFileFromDB(row)
		if err := s.authz.CanShareFile(ctx, actor, file); err != nil {
			s.auditDenied(ctx, actor, "drive.share.check", "drive_file", file.PublicID, err, AuditContext{})
			return DriveResourceRef{}, err
		}
		return file.ResourceRef(), nil
	case DriveResourceTypeFolder:
		folder, err := s.getDriveFolder(ctx, actor.TenantID, ref)
		if err != nil {
			return DriveResourceRef{}, err
		}
		if err := s.authz.CanShareFolder(ctx, actor, folder); err != nil {
			s.auditDenied(ctx, actor, "drive.share.check", "drive_folder", folder.PublicID, err, AuditContext{})
			return DriveResourceRef{}, err
		}
		return folder.ResourceRef(), nil
	default:
		return DriveResourceRef{}, fmt.Errorf("%w: unsupported resource type", ErrDriveInvalidInput)
	}
}

func (s *DriveService) getDriveFileRow(ctx context.Context, tenantID int64, ref DriveResourceRef) (db.FileObject, error) {
	if ref.ID > 0 {
		row, err := s.queries.GetDriveFileByIDForTenant(ctx, db.GetDriveFileByIDForTenantParams{ID: ref.ID, TenantID: tenantID})
		if errors.Is(err, pgx.ErrNoRows) {
			return db.FileObject{}, ErrDriveNotFound
		}
		if err != nil {
			return db.FileObject{}, fmt.Errorf("get drive file: %w", err)
		}
		return row, nil
	}
	publicID, err := uuid.Parse(strings.TrimSpace(ref.PublicID))
	if err != nil {
		return db.FileObject{}, ErrDriveNotFound
	}
	row, err := s.queries.GetDriveFileByPublicIDForTenant(ctx, db.GetDriveFileByPublicIDForTenantParams{PublicID: publicID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.FileObject{}, ErrDriveNotFound
	}
	if err != nil {
		return db.FileObject{}, fmt.Errorf("get drive file: %w", err)
	}
	return row, nil
}

func (s *DriveService) getDriveFolder(ctx context.Context, tenantID int64, ref DriveResourceRef) (DriveFolder, error) {
	if ref.ID > 0 {
		return s.getFolderByID(ctx, tenantID, ref.ID)
	}
	publicID, err := uuid.Parse(strings.TrimSpace(ref.PublicID))
	if err != nil {
		return DriveFolder{}, ErrDriveNotFound
	}
	row, err := s.queries.GetDriveFolderByPublicIDForTenant(ctx, db.GetDriveFolderByPublicIDForTenantParams{PublicID: publicID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveFolder{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveFolder{}, fmt.Errorf("get drive folder: %w", err)
	}
	return driveFolderFromDB(row), nil
}

func (s *DriveService) resolveShareSubjectPublicID(ctx context.Context, tenantID int64, subjectType DriveShareSubjectType, subjectID int64) (string, error) {
	switch subjectType {
	case DriveShareSubjectUser:
		return s.ensureUserInTenant(ctx, tenantID, subjectID)
	case DriveShareSubjectGroup:
		group, err := s.getGroupByID(ctx, tenantID, subjectID)
		if err != nil {
			return "", err
		}
		return group.PublicID, nil
	default:
		return "", fmt.Errorf("%w: unsupported share subject type", ErrDriveInvalidInput)
	}
}

func (s *DriveService) resolveResourceAndParent(ctx context.Context, actor DriveActor, resource DriveResourceRef) (DriveResourceRef, DriveResourceRef, error) {
	switch resource.Type {
	case DriveResourceTypeFile:
		fileRow, err := s.getDriveFileRow(ctx, actor.TenantID, resource)
		if err != nil {
			return DriveResourceRef{}, DriveResourceRef{}, err
		}
		file := driveFileFromDB(fileRow)
		if err := s.authz.CanShareFile(ctx, actor, file); err != nil {
			return DriveResourceRef{}, DriveResourceRef{}, err
		}
		if file.DriveFolderID == nil {
			return DriveResourceRef{}, DriveResourceRef{}, fmt.Errorf("%w: file has no parent", ErrDriveInvalidInput)
		}
		parent, err := s.getFolderByID(ctx, actor.TenantID, *file.DriveFolderID)
		if err != nil {
			return DriveResourceRef{}, DriveResourceRef{}, err
		}
		return file.ResourceRef(), parent.ResourceRef(), nil
	case DriveResourceTypeFolder:
		folder, err := s.getDriveFolder(ctx, actor.TenantID, resource)
		if err != nil {
			return DriveResourceRef{}, DriveResourceRef{}, err
		}
		if err := s.authz.CanShareFolder(ctx, actor, folder); err != nil {
			return DriveResourceRef{}, DriveResourceRef{}, err
		}
		if folder.ParentFolderID == nil {
			return DriveResourceRef{}, DriveResourceRef{}, fmt.Errorf("%w: folder has no parent", ErrDriveInvalidInput)
		}
		parent, err := s.getFolderByID(ctx, actor.TenantID, *folder.ParentFolderID)
		if err != nil {
			return DriveResourceRef{}, DriveResourceRef{}, err
		}
		return folder.ResourceRef(), parent.ResourceRef(), nil
	default:
		return DriveResourceRef{}, DriveResourceRef{}, fmt.Errorf("%w: unsupported resource type", ErrDriveInvalidInput)
	}
}

func (s *DriveService) updateInheritanceFlag(ctx context.Context, tenantID int64, resource, parent DriveResourceRef, enabled bool) (DriveResourceRef, error) {
	switch resource.Type {
	case DriveResourceTypeFile:
		parentFolder, err := s.getFolderByID(ctx, tenantID, parent.ID)
		if err != nil {
			return DriveResourceRef{}, err
		}
		row, err := s.queries.MoveDriveFile(ctx, db.MoveDriveFileParams{
			DriveFolderID:      pgtype.Int8{Int64: parent.ID, Valid: true},
			WorkspaceID:        pgInt8(parentFolder.WorkspaceID),
			InheritanceEnabled: enabled,
			ID:                 resource.ID,
			TenantID:           tenantID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return DriveResourceRef{}, ErrDriveNotFound
		}
		if err != nil {
			return DriveResourceRef{}, fmt.Errorf("update drive file inheritance: %w", err)
		}
		return driveFileFromDB(row).ResourceRef(), nil
	case DriveResourceTypeFolder:
		parentFolder, err := s.getFolderByID(ctx, tenantID, parent.ID)
		if err != nil {
			return DriveResourceRef{}, err
		}
		row, err := s.queries.MoveDriveFolder(ctx, db.MoveDriveFolderParams{
			ParentFolderID:     pgtype.Int8{Int64: parent.ID, Valid: true},
			WorkspaceID:        pgInt8(parentFolder.WorkspaceID),
			InheritanceEnabled: enabled,
			ID:                 resource.ID,
			TenantID:           tenantID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return DriveResourceRef{}, ErrDriveNotFound
		}
		if err != nil {
			return DriveResourceRef{}, fmt.Errorf("update drive folder inheritance: %w", err)
		}
		return driveFolderFromDB(row).ResourceRef(), nil
	default:
		return DriveResourceRef{}, fmt.Errorf("%w: unsupported resource type", ErrDriveInvalidInput)
	}
}

func (s *DriveService) drivePolicy(ctx context.Context, tenantID int64) (DrivePolicy, error) {
	if s.tenantSettings == nil {
		return defaultDrivePolicy(), nil
	}
	return s.tenantSettings.GetDrivePolicy(ctx, tenantID)
}

func (s *DriveService) enqueueDriveOCRBestEffort(ctx context.Context, actor DriveActor, file DriveFile, reason string) {
	if s == nil || s.outbox == nil || file.ID <= 0 || file.TenantID <= 0 {
		return
	}
	policy, err := s.drivePolicy(ctx, file.TenantID)
	if err != nil || !policy.OCR.Enabled {
		return
	}
	tenantID := file.TenantID
	_, _ = s.outbox.Enqueue(ctx, OutboxEventInput{
		TenantID:      &tenantID,
		AggregateType: "drive_file",
		AggregateID:   file.PublicID,
		EventType:     "drive.ocr.requested",
		Payload: map[string]any{
			"tenantId":     file.TenantID,
			"fileObjectId": file.ID,
			"filePublicId": file.PublicID,
			"actorUserId":  actor.UserID,
			"reason":       reason,
		},
	})
}

func (s *DriveService) recordAuditWithQueries(ctx context.Context, queries *db.Queries, auditCtx AuditContext, action, targetType, targetID string, metadata map[string]any) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.RecordWithQueries(ctx, queries, AuditEventInput{
		AuditContext: auditCtx,
		Action:       action,
		TargetType:   targetType,
		TargetID:     targetID,
		Metadata:     metadata,
	})
}

func (s *DriveService) recordAuditBestEffort(ctx context.Context, actor DriveActor, auditCtx AuditContext, action, targetType, targetID string, metadata map[string]any) {
	if s.audit == nil {
		return
	}
	auditCtx.TenantID = &actor.TenantID
	if auditCtx.ActorUserID == nil {
		auditCtx.ActorUserID = &actor.UserID
	}
	s.audit.RecordBestEffort(ctx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       action,
		TargetType:   targetType,
		TargetID:     targetID,
		Metadata:     metadata,
	})
}

func (s *DriveService) auditDenied(ctx context.Context, actor DriveActor, action, targetType, targetID string, reason error, auditCtx AuditContext) {
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.authz.denied", targetType, targetID, map[string]any{
		"action": action,
		"reason": reasonString(reason),
	})
}

func (s *DriveService) auditFailed(ctx context.Context, actor DriveActor, action, targetType, targetID string, reason error, auditCtx AuditContext) {
	s.recordAuditBestEffort(ctx, actor, auditCtx, action+".failed", targetType, targetID, map[string]any{
		"reason": reasonString(reason),
	})
}

func driveShareFromDB(row db.DriveResourceShare, resource DriveResourceRef, subjectPublicID string) DriveShare {
	return DriveShare{
		ID:              row.ID,
		PublicID:        row.PublicID.String(),
		TenantID:        row.TenantID,
		Resource:        resource,
		SubjectType:     DriveShareSubjectType(row.SubjectType),
		SubjectID:       row.SubjectID,
		SubjectPublicID: subjectPublicID,
		Role:            DriveRole(row.Role),
		Status:          row.Status,
		CreatedByUserID: row.CreatedByUserID,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}
}

func driveShareLinkFromDB(row db.DriveShareLink, resource DriveResourceRef) DriveShareLink {
	return DriveShareLink{
		ID:              row.ID,
		PublicID:        row.PublicID.String(),
		TenantID:        row.TenantID,
		Resource:        resource,
		Role:            DriveRole(row.Role),
		CanDownload:     row.CanDownload,
		ExpiresAt:       row.ExpiresAt.Time,
		Status:          row.Status,
		CreatedByUserID: row.CreatedByUserID,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}
}

func normalizeDriveName(value string) string {
	return strings.TrimSpace(value)
}

func normalizeContentType(value string) string {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}

func fileServiceMaxBytes(files *FileService) int64 {
	if files != nil && files.maxBytes > 0 {
		return files.maxBytes
	}
	return 10 * 1024 * 1024
}

func driveEffectiveUploadMaxBytes(defaultMaxBytes, overrideMaxBytes, policyMaxBytes int64) int64 {
	maxBytes := defaultMaxBytes
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024
	}
	if overrideMaxBytes > maxBytes {
		maxBytes = overrideMaxBytes
	}
	if overrideMaxBytes <= 0 && policyMaxBytes > 0 && policyMaxBytes < maxBytes {
		maxBytes = policyMaxBytes
	}
	return maxBytes
}

func formatDriveByteLimit(bytes int64) string {
	if bytes <= 0 {
		return "the configured limit"
	}
	const mb = 1024 * 1024
	if bytes%mb == 0 {
		return fmt.Sprintf("%d MB", bytes/mb)
	}
	if bytes >= mb {
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	}
	const kb = 1024
	if bytes%kb == 0 {
		return fmt.Sprintf("%d KB", bytes/kb)
	}
	return fmt.Sprintf("%d bytes", bytes)
}

func newDriveStorageKey(tenantID int64, workspacePublicID string, revision int) string {
	if revision <= 0 {
		revision = 1
	}
	workspaceSegment := strings.TrimSpace(workspacePublicID)
	if workspaceSegment == "" {
		workspaceSegment = "default"
	}
	return fmt.Sprintf("tenants/%d/workspaces/%s/files/%s/v%d/body", tenantID, workspaceSegment, uuid.NewString(), revision)
}

func normalizeDriveRole(role DriveRole) DriveRole {
	switch role {
	case DriveRoleOwner, DriveRoleEditor, DriveRoleViewer:
		return role
	default:
		return ""
	}
}

func newDriveShareLinkToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", "", fmt.Errorf("generate share link token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}

func reasonString(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrDrivePermissionDenied):
		return "permission_denied"
	case errors.Is(err, ErrDriveAuthzUnavailable):
		return "authz_unavailable"
	case errors.Is(err, ErrDriveLocked):
		return "locked"
	case errors.Is(err, ErrDrivePolicyDenied):
		return "policy_denied"
	case errors.Is(err, ErrDriveNotFound):
		return "not_found"
	default:
		return "failed"
	}
}
