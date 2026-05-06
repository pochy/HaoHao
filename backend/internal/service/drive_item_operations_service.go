package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type driveResourceRefWithState struct {
	resource  DriveResourceRef
	shareRole string
}

func (s *DriveService) ListSharedWithMe(ctx context.Context, tenantID, actorUserID int64, limit int32, auditCtx AuditContext) ([]DriveItem, error) {
	if err := s.ensureConfigured(false); err != nil {
		return nil, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return nil, err
	}
	refs, err := s.queries.ListDriveSharedResourceRefs(ctx, db.ListDriveSharedResourceRefsParams{
		TenantID:   tenantID,
		UserID:     actor.UserID,
		LimitCount: normalizeDriveLimit(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list shared drive resources: %w", err)
	}
	items := make([]driveResourceRefWithState, 0, len(refs))
	for _, ref := range refs {
		items = append(items, driveResourceRefWithState{
			resource:  DriveResourceRef{Type: DriveResourceType(ref.ResourceType), ID: ref.ResourceID, TenantID: tenantID},
			shareRole: ref.Role,
		})
	}
	result, err := s.driveItemsFromRefs(ctx, actor, items)
	if err != nil {
		s.auditDenied(ctx, actor, "drive.shared_with_me.list", "drive", "shared_with_me", err, auditCtx)
		return nil, err
	}
	return result, nil
}

func (s *DriveService) ListStarred(ctx context.Context, tenantID, actorUserID int64, limit int32, auditCtx AuditContext) ([]DriveItem, error) {
	if err := s.ensureConfigured(false); err != nil {
		return nil, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return nil, err
	}
	refs, err := s.queries.ListDriveStarredResourceRefs(ctx, db.ListDriveStarredResourceRefsParams{
		TenantID:   tenantID,
		UserID:     actor.UserID,
		LimitCount: normalizeDriveLimit(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list starred drive resources: %w", err)
	}
	items := make([]driveResourceRefWithState, 0, len(refs))
	for _, ref := range refs {
		items = append(items, driveResourceRefWithState{
			resource: DriveResourceRef{Type: DriveResourceType(ref.ResourceType), ID: ref.ResourceID, TenantID: tenantID},
		})
	}
	result, err := s.driveItemsFromRefs(ctx, actor, items)
	if err != nil {
		s.auditDenied(ctx, actor, "drive.starred.list", "drive", "starred", err, auditCtx)
		return nil, err
	}
	return result, nil
}

func (s *DriveService) StarResource(ctx context.Context, tenantID, actorUserID int64, resource DriveResourceRef, auditCtx AuditContext) error {
	resolved, actor, err := s.resolveViewableResource(ctx, tenantID, actorUserID, resource)
	if err != nil {
		return err
	}
	if err := s.queries.StarDriveItem(ctx, db.StarDriveItemParams{
		TenantID:     tenantID,
		UserID:       actor.UserID,
		ResourceType: string(resolved.Type),
		ResourceID:   resolved.ID,
	}); err != nil {
		return fmt.Errorf("star drive resource: %w", err)
	}
	s.recordDriveActivityBestEffort(ctx, actor, resolved, "updated", map[string]any{"starred": true})
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.star.create", "drive_"+string(resolved.Type), resolved.PublicID, nil)
	return nil
}

func (s *DriveService) UnstarResource(ctx context.Context, tenantID, actorUserID int64, resource DriveResourceRef, auditCtx AuditContext) error {
	resolved, actor, err := s.resolveViewableResource(ctx, tenantID, actorUserID, resource)
	if err != nil {
		return err
	}
	if err := s.queries.UnstarDriveItem(ctx, db.UnstarDriveItemParams{
		TenantID:     tenantID,
		UserID:       actor.UserID,
		ResourceType: string(resolved.Type),
		ResourceID:   resolved.ID,
	}); err != nil {
		return fmt.Errorf("unstar drive resource: %w", err)
	}
	s.recordDriveActivityBestEffort(ctx, actor, resolved, "updated", map[string]any{"starred": false})
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.star.delete", "drive_"+string(resolved.Type), resolved.PublicID, nil)
	return nil
}

func (s *DriveService) ListRecent(ctx context.Context, tenantID, actorUserID int64, limit int32, auditCtx AuditContext) ([]DriveItem, error) {
	if err := s.ensureConfigured(false); err != nil {
		return nil, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return nil, err
	}
	refs, err := s.queries.ListDriveRecentResourceRefs(ctx, db.ListDriveRecentResourceRefsParams{
		TenantID:    tenantID,
		ActorUserID: pgtype.Int8{Int64: actor.UserID, Valid: true},
		LimitCount:  normalizeDriveLimit(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list recent drive resources: %w", err)
	}
	items := make([]driveResourceRefWithState, 0, len(refs))
	for _, ref := range refs {
		items = append(items, driveResourceRefWithState{
			resource: DriveResourceRef{Type: DriveResourceType(ref.ResourceType), ID: ref.ResourceID, TenantID: tenantID},
		})
	}
	result, err := s.driveItemsFromRefs(ctx, actor, items)
	if err != nil {
		s.auditDenied(ctx, actor, "drive.recent.list", "drive", "recent", err, auditCtx)
		return nil, err
	}
	return result, nil
}

func (s *DriveService) ListActivity(ctx context.Context, tenantID, actorUserID int64, resource DriveResourceRef, limit int32, auditCtx AuditContext) ([]DriveActivity, error) {
	resolved, actor, err := s.resolveViewableResource(ctx, tenantID, actorUserID, resource)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListDriveItemActivities(ctx, db.ListDriveItemActivitiesParams{
		TenantID:     tenantID,
		ResourceType: string(resolved.Type),
		ResourceID:   resolved.ID,
		LimitCount:   normalizeDriveLimit(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list drive activity: %w", err)
	}
	out := make([]DriveActivity, 0, len(rows))
	for _, row := range rows {
		metadata := map[string]any{}
		if len(row.Metadata) > 0 {
			_ = json.Unmarshal(row.Metadata, &metadata)
		}
		out = append(out, DriveActivity{
			PublicID:          row.PublicID.String(),
			ResourceType:      DriveResourceType(row.ResourceType),
			ResourceID:        row.ResourceID,
			Action:            row.Action,
			ActorUserPublicID: fmt.Sprint(row.ActorPublicID),
			ActorDisplayName:  row.ActorDisplayName,
			Metadata:          metadata,
			CreatedAt:         row.CreatedAt.Time,
		})
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.activity.list", "drive_"+string(resolved.Type), resolved.PublicID, nil)
	return out, nil
}

func (s *DriveService) GetStorageUsage(ctx context.Context, tenantID, actorUserID int64) (DriveStorageUsage, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveStorageUsage{}, err
	}
	if _, err := s.actor(ctx, tenantID, actorUserID); err != nil {
		return DriveStorageUsage{}, err
	}
	row, err := s.queries.GetDriveStorageUsage(ctx, tenantID)
	if err != nil {
		return DriveStorageUsage{}, fmt.Errorf("get drive storage usage: %w", err)
	}
	var quota *int64
	if s.tenantSettings != nil {
		settings, err := s.tenantSettings.Get(ctx, tenantID)
		if err != nil {
			return DriveStorageUsage{}, err
		}
		quota = &settings.FileQuotaBytes
	}
	return DriveStorageUsage{
		QuotaBytes:     quota,
		UsedBytes:      row.UsedBytes,
		TrashBytes:     row.TrashBytes,
		FileCount:      row.FileCount,
		TrashFileCount: row.TrashFileCount,
		StorageDriver:  row.StorageDriver,
	}, nil
}

func (s *DriveService) ListFolderTree(ctx context.Context, tenantID, actorUserID int64, limit int32, auditCtx AuditContext) (DriveFolderTree, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveFolderTree{}, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveFolderTree{}, err
	}
	rows, err := s.queries.ListDriveFolderTreeCandidates(ctx, db.ListDriveFolderTreeCandidatesParams{
		TenantID:   tenantID,
		LimitCount: normalizeDriveLimit(limit),
	})
	if err != nil {
		return DriveFolderTree{}, fmt.Errorf("list drive folder tree: %w", err)
	}
	folders := make([]DriveFolder, 0, len(rows))
	for _, row := range rows {
		folders = append(folders, driveFolderFromDB(row))
	}
	viewable, err := s.authz.FilterViewableFolders(ctx, actor, folders)
	if err != nil {
		s.auditDenied(ctx, actor, "drive.folder_tree.list", "drive", "folder_tree", err, auditCtx)
		return DriveFolderTree{}, err
	}
	return buildDriveFolderTree(actor, viewable), nil
}

func (s *DriveService) ListShareTargets(ctx context.Context, tenantID, actorUserID int64, query string, limit int32) ([]DriveShareTarget, error) {
	if err := s.ensureConfigured(false); err != nil {
		return nil, err
	}
	if _, err := s.actor(ctx, tenantID, actorUserID); err != nil {
		return nil, err
	}
	q := pgText(strings.TrimSpace(query))
	rows, err := s.queries.ListDriveShareTargets(ctx, db.ListDriveShareTargetsParams{
		TenantID:   tenantID,
		Query:      q,
		LimitCount: normalizeDriveLimit(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list drive share targets: %w", err)
	}
	targets := make([]DriveShareTarget, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, DriveShareTarget{
			Type:        row.TargetType,
			PublicID:    row.PublicID,
			DisplayName: row.DisplayName,
			Secondary:   row.Secondary,
		})
	}
	return targets, nil
}

func (s *DriveService) UpdateShareRole(ctx context.Context, input DriveUpdateShareInput, auditCtx AuditContext) (DriveShare, error) {
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
	role := normalizeDriveRole(input.Role)
	if role == "" || role == DriveRoleOwner {
		return DriveShare{}, fmt.Errorf("%w: share role must be viewer or editor", ErrDriveInvalidInput)
	}
	shareID, err := uuid.Parse(strings.TrimSpace(input.ShareID))
	if err != nil {
		return DriveShare{}, ErrDriveNotFound
	}
	row, err := s.queries.GetDriveResourceShareByPublicIDForTenant(ctx, db.GetDriveResourceShareByPublicIDForTenantParams{PublicID: shareID, TenantID: input.TenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveShare{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveShare{}, fmt.Errorf("get drive share: %w", err)
	}
	if row.ResourceType != string(resource.Type) || row.ResourceID != resource.ID {
		return DriveShare{}, ErrDriveNotFound
	}
	subjectPublicID, err := s.resolveShareSubjectPublicID(ctx, row.TenantID, DriveShareSubjectType(row.SubjectType), row.SubjectID)
	if err != nil {
		return DriveShare{}, err
	}
	oldShare := driveShareFromDB(row, resource, subjectPublicID)
	if err := s.authz.DeleteShareTuple(ctx, oldShare); err != nil {
		return DriveShare{}, err
	}
	updatedRow, err := s.queries.UpdateDriveResourceShareRole(ctx, db.UpdateDriveResourceShareRoleParams{
		Role:     string(role),
		PublicID: shareID,
		TenantID: input.TenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_ = s.authz.WriteShareTuple(context.Background(), oldShare)
		return DriveShare{}, ErrDriveNotFound
	}
	if err != nil {
		_ = s.authz.WriteShareTuple(context.Background(), oldShare)
		return DriveShare{}, fmt.Errorf("update drive share role: %w", err)
	}
	updated := driveShareFromDB(updatedRow, resource, subjectPublicID)
	if err := s.authz.WriteShareTuple(ctx, updated); err != nil {
		_, _ = s.queries.MarkDriveResourceSharePendingSync(context.Background(), db.MarkDriveResourceSharePendingSyncParams{ID: updated.ID, TenantID: updated.TenantID})
		return DriveShare{}, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.share.update", "drive_share", updated.PublicID, map[string]any{"role": updated.Role})
	s.recordDriveActivityBestEffort(ctx, actor, updated.Resource, "shared", map[string]any{
		"sharePublicId": updated.PublicID,
		"role":          updated.Role,
		"operation":     "role_updated",
	})
	return updated, nil
}

func (s *DriveService) TransferOwner(ctx context.Context, input DriveOwnerTransferInput, auditCtx AuditContext) (DriveItem, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveItem{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveItem{}, err
	}
	newOwnerID, err := s.resolveShareSubjectID(ctx, input.TenantID, DriveShareSubjectUser, input.NewOwnerUserPublicID)
	if err != nil {
		return DriveItem{}, err
	}
	newOwnerPublicID, err := s.resolveShareSubjectPublicID(ctx, input.TenantID, DriveShareSubjectUser, newOwnerID)
	if err != nil {
		return DriveItem{}, err
	}

	var item DriveItem
	var resource DriveResourceRef
	switch input.Resource.Type {
	case DriveResourceTypeFile:
		row, err := s.getDriveFileRow(ctx, input.TenantID, input.Resource)
		if err != nil {
			return DriveItem{}, err
		}
		file := driveFileFromDB(row)
		resource = file.ResourceRef()
		if err := s.ensureResourceOwner(ctx, actor, resource); err != nil {
			return DriveItem{}, err
		}
		if err := s.ensureFileMutationAllowed(ctx, input.TenantID, file.ID); err != nil {
			return DriveItem{}, err
		}
		if err := s.authz.WriteResourceOwner(ctx, DriveActor{UserID: newOwnerID, PublicID: newOwnerPublicID, TenantID: input.TenantID}, resource); err != nil {
			return DriveItem{}, err
		}
		updatedRow, err := s.queries.UpdateDriveFileOwner(ctx, db.UpdateDriveFileOwnerParams{
			UploadedByUserID: pgtype.Int8{Int64: newOwnerID, Valid: true},
			ID:               file.ID,
			TenantID:         input.TenantID,
		})
		if err != nil {
			return DriveItem{}, fmt.Errorf("transfer drive file owner: %w", err)
		}
		if input.RevokePreviousOwnerAccess {
			_ = s.authz.DeleteResourceOwner(ctx, actor, resource)
		}
		updated := driveFileFromDB(updatedRow)
		item = DriveItem{Type: DriveItemTypeFile, File: &updated}
	case DriveResourceTypeFolder:
		folder, err := s.getDriveFolder(ctx, input.TenantID, input.Resource)
		if err != nil {
			return DriveItem{}, err
		}
		resource = folder.ResourceRef()
		if err := s.ensureResourceOwner(ctx, actor, resource); err != nil {
			return DriveItem{}, err
		}
		if err := s.ensureFolderMutationAllowed(ctx, input.TenantID, folder.ID); err != nil {
			return DriveItem{}, err
		}
		if err := s.authz.WriteResourceOwner(ctx, DriveActor{UserID: newOwnerID, PublicID: newOwnerPublicID, TenantID: input.TenantID}, resource); err != nil {
			return DriveItem{}, err
		}
		updatedRow, err := s.queries.UpdateDriveFolderOwner(ctx, db.UpdateDriveFolderOwnerParams{
			CreatedByUserID: newOwnerID,
			ID:              folder.ID,
			TenantID:        input.TenantID,
		})
		if err != nil {
			return DriveItem{}, fmt.Errorf("transfer drive folder owner: %w", err)
		}
		if input.RevokePreviousOwnerAccess {
			_ = s.authz.DeleteResourceOwner(ctx, actor, resource)
		}
		updated := driveFolderFromDB(updatedRow)
		item = DriveItem{Type: DriveItemTypeFolder, Folder: &updated}
	default:
		return DriveItem{}, fmt.Errorf("%w: unsupported resource type", ErrDriveInvalidInput)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.owner.transfer", "drive_"+string(resource.Type), resource.PublicID, map[string]any{"newOwnerUserPublicId": newOwnerPublicID})
	s.recordDriveActivityBestEffort(ctx, actor, resource, "updated", map[string]any{
		"operation":            "owner_transferred",
		"newOwnerUserPublicId": newOwnerPublicID,
	})
	return s.enrichDriveItems(ctx, actor, []DriveItem{item})[0], nil
}

func (s *DriveService) CopyResource(ctx context.Context, input DriveCopyResourceInput, auditCtx AuditContext) (DriveItem, error) {
	switch input.Resource.Type {
	case DriveResourceTypeFile:
		return s.copyDriveFile(ctx, input, auditCtx)
	case DriveResourceTypeFolder:
		return s.copyDriveFolder(ctx, input, auditCtx)
	default:
		return DriveItem{}, fmt.Errorf("%w: unsupported resource type", ErrDriveInvalidInput)
	}
}

func (s *DriveService) PermanentlyDeleteResource(ctx context.Context, input DrivePermanentDeleteInput, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return err
	}
	switch input.Resource.Type {
	case DriveResourceTypeFile:
		return s.permanentlyDeleteDriveFile(ctx, actor, input.Resource, auditCtx)
	case DriveResourceTypeFolder:
		return s.permanentlyDeleteDriveFolder(ctx, actor, input.Resource, auditCtx)
	default:
		return fmt.Errorf("%w: unsupported resource type", ErrDriveInvalidInput)
	}
}

func (s *DriveService) DownloadArchive(ctx context.Context, input DriveArchiveInput, auditCtx AuditContext) (DriveArchiveDownload, error) {
	if err := s.ensureConfigured(true); err != nil {
		return DriveArchiveDownload{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveArchiveDownload{}, err
	}
	if len(input.Items) == 0 {
		return DriveArchiveDownload{}, fmt.Errorf("%w: archive items are required", ErrDriveInvalidInput)
	}
	if len(input.Items) > 100 {
		return DriveArchiveDownload{}, fmt.Errorf("%w: archive item limit exceeded", ErrDriveInvalidInput)
	}
	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	usedNames := map[string]int{}
	for _, item := range input.Items {
		switch item.Type {
		case DriveResourceTypeFile:
			row, err := s.getDriveFileRow(ctx, input.TenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: item.PublicID, TenantID: input.TenantID})
			if err != nil {
				_ = zipWriter.Close()
				return DriveArchiveDownload{}, err
			}
			file := driveFileFromDB(row)
			if err := s.addDriveFileToArchive(ctx, zipWriter, actor, file, "", usedNames, auditCtx); err != nil {
				_ = zipWriter.Close()
				return DriveArchiveDownload{}, err
			}
		case DriveResourceTypeFolder:
			folder, err := s.getDriveFolder(ctx, input.TenantID, DriveResourceRef{Type: DriveResourceTypeFolder, PublicID: item.PublicID, TenantID: input.TenantID})
			if err != nil {
				_ = zipWriter.Close()
				return DriveArchiveDownload{}, err
			}
			if err := s.addDriveFolderToArchive(ctx, zipWriter, actor, folder, "", usedNames, auditCtx); err != nil {
				_ = zipWriter.Close()
				return DriveArchiveDownload{}, err
			}
		default:
			_ = zipWriter.Close()
			return DriveArchiveDownload{}, fmt.Errorf("%w: unsupported archive item type", ErrDriveInvalidInput)
		}
	}
	if err := zipWriter.Close(); err != nil {
		return DriveArchiveDownload{}, fmt.Errorf("close drive archive: %w", err)
	}
	filename := sanitizeDriveArchiveName(input.Filename)
	if filename == "" {
		filename = "drive-archive.zip"
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".zip") {
		filename += ".zip"
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.archive.download", "drive", "archive", map[string]any{"itemCount": len(input.Items)})
	return DriveArchiveDownload{Filename: filename, ContentType: "application/zip", Body: buffer.Bytes()}, nil
}

func (s *DriveService) PublicShareLinkFolderChildrenWithVerification(ctx context.Context, token, verificationCookie string, limit int32) ([]DriveItem, error) {
	link, _, folder, err := s.resolvePublicShareLink(ctx, token, false)
	if err != nil {
		return nil, err
	}
	s.hydrateShareLinkPasswordState(ctx, &link)
	if link.PasswordRequired {
		state, err := s.getShareLinkPasswordState(ctx, driveShareLinkTokenHash(token))
		if err != nil {
			return nil, err
		}
		if !verifyShareLinkVerificationCookie(state, verificationCookie, s.now) {
			return nil, ErrDrivePermissionDenied
		}
	}
	if folder == nil {
		return nil, ErrDriveInvalidInput
	}
	limit = normalizeDriveLimit(limit)
	folderRows, err := s.queries.ListDriveChildFolders(ctx, db.ListDriveChildFoldersParams{
		TenantID:       link.TenantID,
		WorkspaceID:    pgInt8(folder.WorkspaceID),
		ParentFolderID: pgtype.Int8{Int64: folder.ID, Valid: true},
		LimitCount:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list public folder child folders: %w", err)
	}
	fileRows, err := s.queries.ListDriveChildFiles(ctx, db.ListDriveChildFilesParams{
		TenantID:      link.TenantID,
		WorkspaceID:   pgInt8(folder.WorkspaceID),
		DriveFolderID: pgtype.Int8{Int64: folder.ID, Valid: true},
		LimitCount:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list public folder child files: %w", err)
	}
	items := make([]DriveItem, 0, len(folderRows)+len(fileRows))
	for _, row := range folderRows {
		child := driveFolderFromDB(row)
		if err := s.authz.checkResource(ctx, openFGAShareLink(link.PublicID), "can_view", openFGAFolder(child.PublicID), "folder", s.authz.currentTimeContext()); err == nil {
			items = append(items, DriveItem{Type: DriveItemTypeFolder, Folder: &child})
		}
	}
	for _, row := range fileRows {
		child := driveFileFromDB(row)
		if err := s.authz.checkResource(ctx, openFGAShareLink(link.PublicID), "can_view", openFGAFile(child.PublicID), "file", s.authz.currentTimeContext()); err == nil {
			items = append(items, DriveItem{Type: DriveItemTypeFile, File: &child})
		}
	}
	s.recordPublicLinkAudit(ctx, link, "drive.share_link.children", map[string]any{"count": len(items)})
	return items, nil
}

func (s *DriveService) PreviewFile(ctx context.Context, tenantID, actorUserID int64, filePublicID string, thumbnail bool, auditCtx AuditContext) (DriveFileDownload, error) {
	if err := s.ensureConfigured(true); err != nil {
		return DriveFileDownload{}, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveFileDownload{}, err
	}
	row, err := s.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID, TenantID: tenantID})
	if err != nil {
		return DriveFileDownload{}, err
	}
	file := driveFileFromDB(row)
	if err := s.authz.CanViewFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.file.preview", "drive_file", file.PublicID, err, auditCtx)
		return DriveFileDownload{}, err
	}
	if !driveFilePreviewSupported(file.ContentType, thumbnail) {
		return DriveFileDownload{}, fmt.Errorf("%w: preview is not supported for this content type", ErrDriveInvalidInput)
	}
	if err := s.ensureFileDownloadAllowed(ctx, actor, file, auditCtx, "drive.file.preview"); err != nil {
		return DriveFileDownload{}, err
	}
	if err := s.ensureDriveEncryptionAvailable(ctx, file.TenantID, file.ID); err != nil {
		return DriveFileDownload{}, err
	}
	body, err := s.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return DriveFileDownload{}, err
	}
	s.recordDriveActivityBestEffort(ctx, actor, file.ResourceRef(), "viewed", map[string]any{"preview": true, "thumbnail": thumbnail})
	return DriveFileDownload{File: file, Body: body}, nil
}

func (s *DriveService) resolveViewableResource(ctx context.Context, tenantID, actorUserID int64, resource DriveResourceRef) (DriveResourceRef, DriveActor, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveResourceRef{}, DriveActor{}, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveResourceRef{}, DriveActor{}, err
	}
	switch resource.Type {
	case DriveResourceTypeFile:
		row, err := s.getDriveFileRow(ctx, tenantID, resource)
		if err != nil {
			return DriveResourceRef{}, DriveActor{}, err
		}
		file := driveFileFromDB(row)
		if err := s.authz.CanViewFile(ctx, actor, file); err != nil {
			return DriveResourceRef{}, DriveActor{}, err
		}
		return file.ResourceRef(), actor, nil
	case DriveResourceTypeFolder:
		folder, err := s.getDriveFolder(ctx, tenantID, resource)
		if err != nil {
			return DriveResourceRef{}, DriveActor{}, err
		}
		if err := s.authz.CanViewFolder(ctx, actor, folder); err != nil {
			return DriveResourceRef{}, DriveActor{}, err
		}
		return folder.ResourceRef(), actor, nil
	default:
		return DriveResourceRef{}, DriveActor{}, fmt.Errorf("%w: unsupported resource type", ErrDriveInvalidInput)
	}
}

func (s *DriveService) driveItemsFromRefs(ctx context.Context, actor DriveActor, refs []driveResourceRefWithState) ([]DriveItem, error) {
	items := make([]DriveItem, 0, len(refs))
	for _, ref := range refs {
		item, err := s.driveItemFromRef(ctx, actor, ref)
		if errors.Is(err, ErrDriveNotFound) || errors.Is(err, ErrDrivePermissionDenied) {
			continue
		}
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return s.enrichDriveItems(ctx, actor, items), nil
}

func (s *DriveService) driveItemFromRef(ctx context.Context, actor DriveActor, state driveResourceRefWithState) (DriveItem, error) {
	switch state.resource.Type {
	case DriveResourceTypeFile:
		row, err := s.getDriveFileRow(ctx, actor.TenantID, state.resource)
		if err != nil {
			return DriveItem{}, err
		}
		file := driveFileFromDB(row)
		if err := s.authz.CanViewFile(ctx, actor, file); err != nil {
			return DriveItem{}, err
		}
		return DriveItem{Type: DriveItemTypeFile, File: &file, Metadata: DriveItemMetadata{SharedWithMe: state.shareRole != "", ShareRole: state.shareRole}}, nil
	case DriveResourceTypeFolder:
		folder, err := s.getDriveFolder(ctx, actor.TenantID, state.resource)
		if err != nil {
			return DriveItem{}, err
		}
		if err := s.authz.CanViewFolder(ctx, actor, folder); err != nil {
			return DriveItem{}, err
		}
		return DriveItem{Type: DriveItemTypeFolder, Folder: &folder, Metadata: DriveItemMetadata{SharedWithMe: state.shareRole != "", ShareRole: state.shareRole}}, nil
	default:
		return DriveItem{}, ErrDriveNotFound
	}
}

func (s *DriveService) enrichDriveItems(ctx context.Context, actor DriveActor, items []DriveItem) []DriveItem {
	for i := range items {
		resource, ownerUserID, source := itemResourceOwnerAndSource(items[i])
		items[i].Metadata.Source = source
		items[i].Metadata.OwnedByMe = ownerUserID == actor.UserID
		if ownerUserID > 0 {
			if user, err := s.queries.GetUserByID(ctx, ownerUserID); err == nil {
				items[i].Metadata.OwnerUserPublicID = user.PublicID.String()
				items[i].Metadata.OwnerDisplayName = user.DisplayName
			}
		}
		if resource.ID > 0 {
			if starred, err := s.queries.IsDriveItemStarredByUser(ctx, db.IsDriveItemStarredByUserParams{
				TenantID:     actor.TenantID,
				UserID:       actor.UserID,
				ResourceType: string(resource.Type),
				ResourceID:   resource.ID,
			}); err == nil {
				items[i].Metadata.StarredByMe = starred
			}
			if tags, err := s.queries.ListDriveTagsForItem(ctx, db.ListDriveTagsForItemParams{
				TenantID:     actor.TenantID,
				ResourceType: string(resource.Type),
				ResourceID:   resource.ID,
			}); err == nil {
				items[i].Metadata.Tags = tags
			}
		}
	}
	return items
}

func (s *DriveService) applyDriveListFilter(items []DriveItem, filter DriveListItemsFilter) []DriveItem {
	filter.Type = normalizeDriveFilterValue(filter.Type, "all")
	filter.Owner = normalizeDriveFilterValue(filter.Owner, "all")
	filter.Source = normalizeDriveFilterValue(filter.Source, "all")
	filter.Sort = normalizeDriveFilterValue(filter.Sort, "updated_at")
	filter.Direction = normalizeDriveFilterValue(filter.Direction, "desc")

	filtered := make([]DriveItem, 0, len(items))
	for _, item := range items {
		if filter.Type == "file" && item.Type != DriveItemTypeFile {
			continue
		}
		if filter.Type == "folder" && item.Type != DriveItemTypeFolder {
			continue
		}
		if (filter.Owner == "me" || filter.Owner == "owned_by_me") && !item.Metadata.OwnedByMe {
			continue
		}
		if filter.Owner == "shared_with_me" && !item.Metadata.SharedWithMe {
			continue
		}
		if filter.Source != "all" && filter.Source != item.Metadata.Source {
			if !(filter.Source == "uploaded" && item.Metadata.Source == "upload") {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		less := driveItemLess(filtered[i], filtered[j], filter.Sort)
		if filter.Direction == "asc" {
			return less
		}
		return !less
	})
	return filtered
}

func itemResourceOwnerAndSource(item DriveItem) (DriveResourceRef, int64, string) {
	if item.File != nil {
		ownerID := int64(0)
		if item.File.UploadedByUserID != nil {
			ownerID = *item.File.UploadedByUserID
		}
		source := "upload"
		if item.File.StorageGatewayID != nil {
			source = "sync"
		}
		return item.File.ResourceRef(), ownerID, source
	}
	if item.Folder != nil {
		return item.Folder.ResourceRef(), item.Folder.CreatedByUserID, "generated"
	}
	return DriveResourceRef{}, 0, ""
}

func driveItemLess(a, b DriveItem, key string) bool {
	switch key {
	case "name":
		return strings.ToLower(driveItemNameForSort(a)) < strings.ToLower(driveItemNameForSort(b))
	case "size":
		return driveItemSizeForSort(a) < driveItemSizeForSort(b)
	case "type":
		return string(a.Type) < string(b.Type)
	default:
		return driveItemUpdatedAtForSort(a).Before(driveItemUpdatedAtForSort(b))
	}
}

func driveItemNameForSort(item DriveItem) string {
	if item.File != nil {
		return item.File.OriginalFilename
	}
	if item.Folder != nil {
		return item.Folder.Name
	}
	return ""
}

func driveItemSizeForSort(item DriveItem) int64 {
	if item.File != nil {
		return item.File.ByteSize
	}
	return -1
}

func driveItemUpdatedAtForSort(item DriveItem) time.Time {
	if item.File != nil {
		return item.File.UpdatedAt
	}
	if item.Folder != nil {
		return item.Folder.UpdatedAt
	}
	return time.Time{}
}

func normalizeDriveLimit(limit int32) int32 {
	if limit <= 0 || limit > 500 {
		return 100
	}
	return limit
}

func normalizeDriveFilterValue(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}

func (s *DriveService) copyDriveFile(ctx context.Context, input DriveCopyResourceInput, auditCtx AuditContext) (DriveItem, error) {
	if err := s.ensureConfigured(true); err != nil {
		return DriveItem{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveItem{}, err
	}
	row, err := s.getDriveFileRow(ctx, input.TenantID, input.Resource)
	if err != nil {
		return DriveItem{}, err
	}
	file := driveFileFromDB(row)
	if err := s.authz.CanViewFile(ctx, actor, file); err != nil {
		return DriveItem{}, err
	}
	if err := s.authz.CanDownloadFile(ctx, actor, file); err != nil {
		return DriveItem{}, err
	}
	parentID, parentRef, workspace, err := s.resolveDriveCopyDestination(ctx, actor, file.WorkspaceID, file.DriveFolderID, input.ParentFolderPublicID)
	if err != nil {
		return DriveItem{}, err
	}
	workspaceID := pgtype.Int8{}
	workspacePublicID := file.WorkspacePublicID
	if workspace != nil {
		workspaceID = pgtype.Int8{Int64: workspace.ID, Valid: true}
		workspacePublicID = workspace.PublicID
	} else if file.WorkspaceID != nil {
		workspaceID = pgtype.Int8{Int64: *file.WorkspaceID, Valid: true}
	}
	body, err := s.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return DriveItem{}, err
	}
	defer body.Close()
	maxBytes := file.ByteSize + 1
	if maxBytes <= 0 {
		maxBytes = 1
	}
	stored, err := s.storage.PutObject(ctx, newDriveStorageKey(input.TenantID, workspacePublicID, 1), body, maxBytes, ObjectPutOptions{ContentType: file.ContentType})
	if err != nil {
		return DriveItem{}, err
	}
	name := normalizeDriveName(strings.TrimSpace(input.Name))
	if name == "" {
		name = copyDriveName(file.OriginalFilename)
	}
	copiedRow, err := s.queries.CopyDriveFileObject(ctx, db.CopyDriveFileObjectParams{
		UploadedByUserID:   pgtype.Int8{Int64: actor.UserID, Valid: true},
		WorkspaceID:        workspaceID,
		DriveFolderID:      parentID,
		OriginalFilename:   name,
		ByteSize:           stored.Size,
		Sha256Hex:          stored.SHA256Hex,
		StorageDriver:      storageDriverForStoredFile(s.storage, stored),
		StorageKey:         stored.Key,
		StorageBucket:      pgText(stored.Bucket),
		Etag:               stored.ETag,
		InheritanceEnabled: parentRef != nil,
		SourceID:           file.ID,
		TenantID:           input.TenantID,
	})
	if err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveItem{}, fmt.Errorf("copy drive file: %w", err)
	}
	copied := driveFileFromDB(copiedRow)
	if err := s.authz.WriteResourceCreateTuplesWithWorkspace(ctx, actor, copied.ResourceRef(), parentRef, workspace); err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		_, _ = s.queries.SoftDeleteDriveFile(context.Background(), db.SoftDeleteDriveFileParams{
			DeletedByUserID: pgtype.Int8{Int64: actor.UserID, Valid: true},
			ID:              copied.ID,
			TenantID:        copied.TenantID,
		})
		return DriveItem{}, err
	}
	if tags, err := s.queries.ListDriveTagsForItem(ctx, db.ListDriveTagsForItemParams{TenantID: input.TenantID, ResourceType: string(DriveResourceTypeFile), ResourceID: file.ID}); err == nil && len(tags) > 0 {
		_ = s.replaceDriveItemTags(ctx, input.TenantID, actor.UserID, copied.ResourceRef(), tags)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.copy", "drive_file", copied.PublicID, map[string]any{"sourceFilePublicId": file.PublicID})
	s.recordDriveActivityBestEffort(ctx, actor, copied.ResourceRef(), "uploaded", map[string]any{"operation": "copied", "sourceFilePublicId": file.PublicID})
	s.recordDriveFilePreviewStateBestEffort(ctx, copied)
	return s.enrichDriveItems(ctx, actor, []DriveItem{{Type: DriveItemTypeFile, File: &copied}})[0], nil
}

func (s *DriveService) copyDriveFolder(ctx context.Context, input DriveCopyResourceInput, auditCtx AuditContext) (DriveItem, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveItem{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveItem{}, err
	}
	folder, err := s.getDriveFolder(ctx, input.TenantID, input.Resource)
	if err != nil {
		return DriveItem{}, err
	}
	if err := s.authz.CanViewFolder(ctx, actor, folder); err != nil {
		return DriveItem{}, err
	}
	parentID, parentRef, workspace, err := s.resolveDriveCopyDestination(ctx, actor, folder.WorkspaceID, folder.ParentFolderID, input.ParentFolderPublicID)
	if err != nil {
		return DriveItem{}, err
	}
	workspaceID := pgtype.Int8{}
	if workspace != nil {
		workspaceID = pgtype.Int8{Int64: workspace.ID, Valid: true}
	} else if folder.WorkspaceID != nil {
		workspaceID = pgtype.Int8{Int64: *folder.WorkspaceID, Valid: true}
	}
	name := normalizeDriveName(strings.TrimSpace(input.Name))
	if name == "" {
		name = copyDriveName(folder.Name)
	}
	copiedRow, err := s.queries.CopyDriveFolder(ctx, db.CopyDriveFolderParams{
		WorkspaceID:        workspaceID,
		ParentFolderID:     parentID,
		Name:               name,
		CreatedByUserID:    actor.UserID,
		InheritanceEnabled: parentRef != nil,
		SourceID:           folder.ID,
		TenantID:           input.TenantID,
	})
	if err != nil {
		return DriveItem{}, fmt.Errorf("copy drive folder: %w", err)
	}
	copied := driveFolderFromDB(copiedRow)
	if err := s.authz.WriteResourceCreateTuplesWithWorkspace(ctx, actor, copied.ResourceRef(), parentRef, workspace); err != nil {
		_, _ = s.queries.SoftDeleteDriveFolder(context.Background(), db.SoftDeleteDriveFolderParams{
			DeletedByUserID: pgtype.Int8{Int64: actor.UserID, Valid: true},
			ID:              copied.ID,
			TenantID:        copied.TenantID,
		})
		return DriveItem{}, err
	}
	if tags, err := s.queries.ListDriveTagsForItem(ctx, db.ListDriveTagsForItemParams{TenantID: input.TenantID, ResourceType: string(DriveResourceTypeFolder), ResourceID: folder.ID}); err == nil && len(tags) > 0 {
		_ = s.replaceDriveItemTags(ctx, input.TenantID, actor.UserID, copied.ResourceRef(), tags)
	}
	if err := s.copyDriveFolderChildren(ctx, actor, folder, copied, auditCtx); err != nil {
		return DriveItem{}, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.folder.copy", "drive_folder", copied.PublicID, map[string]any{"sourceFolderPublicId": folder.PublicID})
	s.recordDriveActivityBestEffort(ctx, actor, copied.ResourceRef(), "updated", map[string]any{"operation": "copied", "sourceFolderPublicId": folder.PublicID})
	return s.enrichDriveItems(ctx, actor, []DriveItem{{Type: DriveItemTypeFolder, Folder: &copied}})[0], nil
}

func (s *DriveService) permanentlyDeleteDriveFile(ctx context.Context, actor DriveActor, resource DriveResourceRef, auditCtx AuditContext) error {
	publicID, err := uuid.Parse(strings.TrimSpace(resource.PublicID))
	if err != nil {
		return ErrDriveNotFound
	}
	row, err := s.queries.GetDeletedDriveFileByPublicIDForTenant(ctx, db.GetDeletedDriveFileByPublicIDForTenantParams{PublicID: publicID, TenantID: actor.TenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	}
	if err != nil {
		return fmt.Errorf("get deleted drive file: %w", err)
	}
	file := driveFileFromDB(row)
	if err := s.ensureResourceOwner(ctx, actor, file.ResourceRef()); err != nil {
		return err
	}
	if err := s.ensureFileMutationAllowed(ctx, actor.TenantID, file.ID); err != nil {
		return err
	}
	if s.storage != nil && strings.TrimSpace(file.StorageKey) != "" {
		_ = s.storage.Delete(ctx, file.StorageKey)
	}
	if _, err := s.queries.MarkDriveFilePermanentlyDeleted(ctx, db.MarkDriveFilePermanentlyDeletedParams{ID: file.ID, TenantID: actor.TenantID}); errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveLocked
	} else if err != nil {
		return fmt.Errorf("permanently delete drive file: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.permanent_delete", "drive_file", file.PublicID, nil)
	s.recordDriveActivityBestEffort(ctx, actor, file.ResourceRef(), "deleted", map[string]any{"permanent": true})
	return nil
}

func (s *DriveService) permanentlyDeleteDriveFolder(ctx context.Context, actor DriveActor, resource DriveResourceRef, auditCtx AuditContext) error {
	publicID, err := uuid.Parse(strings.TrimSpace(resource.PublicID))
	if err != nil {
		return ErrDriveNotFound
	}
	row, err := s.queries.GetDeletedDriveFolderByPublicIDForTenant(ctx, db.GetDeletedDriveFolderByPublicIDForTenantParams{PublicID: publicID, TenantID: actor.TenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	}
	if err != nil {
		return fmt.Errorf("get deleted drive folder: %w", err)
	}
	folder := driveFolderFromDB(row)
	if err := s.ensureResourceOwner(ctx, actor, folder.ResourceRef()); err != nil {
		return err
	}
	if err := s.ensureFolderMutationAllowed(ctx, actor.TenantID, folder.ID); err != nil {
		return err
	}
	childCount, err := s.queries.CountDriveFolderChildrenAnyState(ctx, db.CountDriveFolderChildrenAnyStateParams{TenantID: actor.TenantID, FolderID: pgtype.Int8{Int64: folder.ID, Valid: true}})
	if err != nil {
		return fmt.Errorf("check drive folder children: %w", err)
	}
	if childCount > 0 {
		return fmt.Errorf("%w: folder must be empty before permanent delete", ErrDriveInvalidInput)
	}
	if _, err := s.queries.DeleteDriveFolderPermanently(ctx, db.DeleteDriveFolderPermanentlyParams{ID: folder.ID, TenantID: actor.TenantID}); errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveLocked
	} else if err != nil {
		return fmt.Errorf("permanently delete drive folder: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.folder.permanent_delete", "drive_folder", folder.PublicID, nil)
	s.recordDriveActivityBestEffort(ctx, actor, folder.ResourceRef(), "deleted", map[string]any{"permanent": true})
	return nil
}

func (s *DriveService) resolveDriveCopyDestination(ctx context.Context, actor DriveActor, currentWorkspaceID, currentParentID *int64, parentPublicID string) (pgtype.Int8, *DriveResourceRef, *DriveWorkspace, error) {
	parentID := pgtype.Int8{}
	var parentRef *DriveResourceRef
	var workspace *DriveWorkspace
	workspaceID := currentWorkspaceID
	parentPublicID = strings.TrimSpace(parentPublicID)
	if parentPublicID != "" && parentPublicID != "root" {
		parent, err := s.getDriveFolder(ctx, actor.TenantID, DriveResourceRef{Type: DriveResourceTypeFolder, PublicID: parentPublicID, TenantID: actor.TenantID})
		if err != nil {
			return pgtype.Int8{}, nil, nil, err
		}
		if err := s.authz.CanEditFolder(ctx, actor, parent); err != nil {
			return pgtype.Int8{}, nil, nil, err
		}
		parentID = pgtype.Int8{Int64: parent.ID, Valid: true}
		ref := parent.ResourceRef()
		parentRef = &ref
		workspaceID = parent.WorkspaceID
	} else if parentPublicID == "" && currentParentID != nil {
		parent, err := s.getFolderByID(ctx, actor.TenantID, *currentParentID)
		if err != nil {
			return pgtype.Int8{}, nil, nil, err
		}
		if err := s.authz.CanEditFolder(ctx, actor, parent); err != nil {
			return pgtype.Int8{}, nil, nil, err
		}
		parentID = pgtype.Int8{Int64: parent.ID, Valid: true}
		ref := parent.ResourceRef()
		parentRef = &ref
		workspaceID = parent.WorkspaceID
	}
	if workspaceID != nil {
		item, err := s.getWorkspaceByID(ctx, actor.TenantID, *workspaceID)
		if err != nil {
			return pgtype.Int8{}, nil, nil, err
		}
		workspace = &item
	}
	return parentID, parentRef, workspace, nil
}

func (s *DriveService) ensureResourceOwner(ctx context.Context, actor DriveActor, resource DriveResourceRef) error {
	if err := validateDriveActorResourceForDeleted(actor, resource); err != nil {
		return err
	}
	if actor.PlatformAdmin {
		return nil
	}
	return s.authz.checkResource(ctx, openFGAUser(actor.PublicID), "owner", openFGAResourceObject(resource), string(resource.Type), s.authz.currentTimeContext())
}

func copyDriveName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Copy"
	}
	return "Copy of " + name
}

func driveFilePreviewSupported(contentType string, thumbnail bool) bool {
	contentType = normalizeContentType(contentType)
	if strings.HasPrefix(contentType, "image/") {
		return true
	}
	if thumbnail {
		return false
	}
	return contentType == "application/pdf" ||
		strings.HasPrefix(contentType, "text/") ||
		contentType == "application/json" ||
		contentType == "application/xml" ||
		contentType == "application/x-ndjson" ||
		contentType == "text/csv"
}

func (s *DriveService) recordDriveActivityBestEffort(ctx context.Context, actor DriveActor, resource DriveResourceRef, action string, metadata map[string]any) {
	if s == nil || s.queries == nil || actor.TenantID <= 0 || resource.ID <= 0 {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	body, err := json.Marshal(metadata)
	if err != nil {
		body = []byte("{}")
	}
	_ = s.queries.RecordDriveItemActivity(ctx, db.RecordDriveItemActivityParams{
		TenantID:     actor.TenantID,
		ActorUserID:  pgtype.Int8{Int64: actor.UserID, Valid: actor.UserID > 0},
		ResourceType: string(resource.Type),
		ResourceID:   resource.ID,
		Action:       action,
		Metadata:     body,
	})
}

func (s *DriveService) replaceDriveItemTags(ctx context.Context, tenantID, actorUserID int64, resource DriveResourceRef, tags []string) error {
	normalized := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, tag)
	}
	if err := s.queries.DeleteDriveTagsForItem(ctx, db.DeleteDriveTagsForItemParams{
		TenantID:     tenantID,
		ResourceType: string(resource.Type),
		ResourceID:   resource.ID,
	}); err != nil {
		return fmt.Errorf("delete drive tags: %w", err)
	}
	for _, tag := range normalized {
		if err := s.queries.AddDriveItemTag(ctx, db.AddDriveItemTagParams{
			TenantID:        tenantID,
			ResourceType:    string(resource.Type),
			ResourceID:      resource.ID,
			Tag:             tag,
			CreatedByUserID: pgtype.Int8{Int64: actorUserID, Valid: actorUserID > 0},
		}); err != nil {
			return fmt.Errorf("add drive tag: %w", err)
		}
	}
	return nil
}

func (s *DriveService) copyDriveFolderChildren(ctx context.Context, actor DriveActor, source, destination DriveFolder, auditCtx AuditContext) error {
	childFiles, err := s.queries.ListDriveChildFiles(ctx, db.ListDriveChildFilesParams{
		TenantID:      source.TenantID,
		WorkspaceID:   pgInt8(source.WorkspaceID),
		DriveFolderID: pgtype.Int8{Int64: source.ID, Valid: true},
		LimitCount:    500,
	})
	if err != nil {
		return fmt.Errorf("list source folder files: %w", err)
	}
	for _, row := range childFiles {
		child := driveFileFromDB(row)
		if _, err := s.copyDriveFile(ctx, DriveCopyResourceInput{
			TenantID:             actor.TenantID,
			ActorUserID:          actor.UserID,
			Resource:             child.ResourceRef(),
			ParentFolderPublicID: destination.PublicID,
			Name:                 child.OriginalFilename,
		}, auditCtx); err != nil {
			return err
		}
	}
	childFolders, err := s.queries.ListDriveChildFolders(ctx, db.ListDriveChildFoldersParams{
		TenantID:       source.TenantID,
		WorkspaceID:    pgInt8(source.WorkspaceID),
		ParentFolderID: pgtype.Int8{Int64: source.ID, Valid: true},
		LimitCount:     500,
	})
	if err != nil {
		return fmt.Errorf("list source folder folders: %w", err)
	}
	for _, row := range childFolders {
		child := driveFolderFromDB(row)
		if _, err := s.copyDriveFolder(ctx, DriveCopyResourceInput{
			TenantID:             actor.TenantID,
			ActorUserID:          actor.UserID,
			Resource:             child.ResourceRef(),
			ParentFolderPublicID: destination.PublicID,
			Name:                 child.Name,
		}, auditCtx); err != nil {
			return err
		}
	}
	return nil
}

func (s *DriveService) addDriveFolderToArchive(ctx context.Context, zipWriter *zip.Writer, actor DriveActor, folder DriveFolder, parentPath string, usedNames map[string]int, auditCtx AuditContext) error {
	if err := s.authz.CanViewFolder(ctx, actor, folder); err != nil {
		s.auditDenied(ctx, actor, "drive.archive.folder", "drive_folder", folder.PublicID, err, auditCtx)
		return err
	}
	folderPath := joinDriveArchivePath(parentPath, sanitizeDriveArchiveName(folder.Name))
	if folderPath == "" {
		folderPath = folder.PublicID
	}
	dirName := ensureDriveArchiveSlash(uniqueDriveArchivePath(folderPath, usedNames))
	if _, err := zipWriter.Create(dirName); err != nil {
		return fmt.Errorf("create archive folder entry: %w", err)
	}
	childFolders, err := s.queries.ListDriveChildFolders(ctx, db.ListDriveChildFoldersParams{
		TenantID:       folder.TenantID,
		WorkspaceID:    pgInt8(folder.WorkspaceID),
		ParentFolderID: pgtype.Int8{Int64: folder.ID, Valid: true},
		LimitCount:     500,
	})
	if err != nil {
		return fmt.Errorf("list archive folder children: %w", err)
	}
	for _, row := range childFolders {
		child := driveFolderFromDB(row)
		if err := s.addDriveFolderToArchive(ctx, zipWriter, actor, child, strings.TrimSuffix(dirName, "/"), usedNames, auditCtx); err != nil {
			return err
		}
	}
	childFiles, err := s.queries.ListDriveChildFiles(ctx, db.ListDriveChildFilesParams{
		TenantID:      folder.TenantID,
		WorkspaceID:   pgInt8(folder.WorkspaceID),
		DriveFolderID: pgtype.Int8{Int64: folder.ID, Valid: true},
		LimitCount:    500,
	})
	if err != nil {
		return fmt.Errorf("list archive folder files: %w", err)
	}
	for _, row := range childFiles {
		file := driveFileFromDB(row)
		if err := s.addDriveFileToArchive(ctx, zipWriter, actor, file, strings.TrimSuffix(dirName, "/"), usedNames, auditCtx); err != nil {
			return err
		}
	}
	return nil
}

func (s *DriveService) addDriveFileToArchive(ctx context.Context, zipWriter *zip.Writer, actor DriveActor, file DriveFile, parentPath string, usedNames map[string]int, auditCtx AuditContext) error {
	if err := s.authz.CanViewFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.archive.file", "drive_file", file.PublicID, err, auditCtx)
		return err
	}
	if err := s.authz.CanDownloadFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.archive.file", "drive_file", file.PublicID, err, auditCtx)
		return err
	}
	body, err := s.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return err
	}
	defer body.Close()
	name := joinDriveArchivePath(parentPath, sanitizeDriveArchiveName(file.OriginalFilename))
	if name == "" {
		name = file.PublicID
	}
	writer, err := zipWriter.Create(uniqueDriveArchivePath(name, usedNames))
	if err != nil {
		return fmt.Errorf("create archive file entry: %w", err)
	}
	if _, err := io.Copy(writer, body); err != nil {
		return fmt.Errorf("write archive file entry: %w", err)
	}
	s.recordDriveActivityBestEffort(ctx, actor, file.ResourceRef(), "downloaded", map[string]any{"archive": true})
	return nil
}

func sanitizeDriveArchiveName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "/\\")
	name = strings.ReplaceAll(name, "\x00", "")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	if name == "." || name == ".." {
		return ""
	}
	return name
}

func joinDriveArchivePath(parent, child string) string {
	parent = strings.Trim(parent, "/")
	child = strings.Trim(child, "/")
	switch {
	case parent == "":
		return child
	case child == "":
		return parent
	default:
		return parent + "/" + child
	}
}

func ensureDriveArchiveSlash(name string) string {
	if strings.HasSuffix(name, "/") {
		return name
	}
	return name + "/"
}

func uniqueDriveArchivePath(name string, used map[string]int) string {
	if used == nil {
		return name
	}
	if count := used[name]; count > 0 {
		used[name] = count + 1
		trimmed := strings.TrimSuffix(name, "/")
		slash := ""
		if strings.HasSuffix(name, "/") {
			slash = "/"
		}
		return fmt.Sprintf("%s (%d)%s", trimmed, count+1, slash)
	}
	used[name] = 1
	return name
}

func (s *DriveService) recordDriveFilePreviewStateBestEffort(ctx context.Context, file DriveFile) {
	if s == nil || s.queries == nil || file.ID <= 0 || file.TenantID <= 0 {
		return
	}
	status := "skipped"
	thumbnailKey := pgtype.Text{}
	previewKey := pgtype.Text{}
	errorCode := pgtype.Text{}
	contentType := pgText(file.ContentType)
	if driveFilePreviewSupported(file.ContentType, false) {
		status = "ready"
		previewKey = pgText(file.StorageKey)
		if driveFilePreviewSupported(file.ContentType, true) {
			thumbnailKey = pgText(file.StorageKey)
		}
	} else {
		errorCode = pgText("unsupported_content_type")
	}
	_ = s.queries.UpsertDriveFilePreviewState(ctx, db.UpsertDriveFilePreviewStateParams{
		TenantID:            file.TenantID,
		FileObjectID:        file.ID,
		Status:              status,
		ThumbnailStorageKey: thumbnailKey,
		PreviewStorageKey:   previewKey,
		ContentType:         contentType,
		ErrorCode:           errorCode,
	})
}

func buildDriveFolderTree(actor DriveActor, folders []DriveFolder) DriveFolderTree {
	byID := map[int64]DriveFolder{}
	children := map[int64][]DriveFolder{}
	for _, folder := range folders {
		byID[folder.ID] = folder
		parentID := int64(0)
		if folder.ParentFolderID != nil {
			parentID = *folder.ParentFolderID
		}
		children[parentID] = append(children[parentID], folder)
	}

	var nodeFor func(folder DriveFolder) DriveFolderTreeNode
	nodeFor = func(folder DriveFolder) DriveFolderTreeNode {
		node := DriveFolderTreeNode{PublicID: folder.PublicID, Name: folder.Name}
		for _, child := range children[folder.ID] {
			if child.CreatedByUserID == folder.CreatedByUserID {
				node.Children = append(node.Children, nodeFor(child))
			}
		}
		return node
	}

	tree := DriveFolderTree{}
	for _, folder := range folders {
		parentVisible := false
		if folder.ParentFolderID != nil {
			_, parentVisible = byID[*folder.ParentFolderID]
		}
		if parentVisible && folder.CreatedByUserID == byID[*folder.ParentFolderID].CreatedByUserID {
			continue
		}
		if folder.CreatedByUserID == actor.UserID {
			tree.OwnedRoots = append(tree.OwnedRoots, nodeFor(folder))
		} else {
			tree.SharedRoots = append(tree.SharedRoots, nodeFor(folder))
		}
	}
	return tree
}
