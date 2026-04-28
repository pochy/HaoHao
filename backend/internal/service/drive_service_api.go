package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *DriveService) GetFolder(ctx context.Context, tenantID, actorUserID int64, folderPublicID string, auditCtx AuditContext) (DriveFolder, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveFolder{}, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveFolder{}, err
	}
	folder, err := s.getDriveFolder(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFolder, PublicID: folderPublicID})
	if err != nil {
		return DriveFolder{}, err
	}
	if err := s.authz.CanViewFolder(ctx, actor, folder); err != nil {
		s.auditDenied(ctx, actor, "drive.folder.view", "drive_folder", folder.PublicID, err, auditCtx)
		return DriveFolder{}, err
	}
	s.recordDriveActivityBestEffort(ctx, actor, folder.ResourceRef(), "viewed", nil)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.folder.view", "drive_folder", folder.PublicID, nil)
	return folder, nil
}

func (s *DriveService) ListChildren(ctx context.Context, input DriveListChildrenInput, auditCtx AuditContext) ([]DriveItem, error) {
	if err := s.ensureConfigured(false); err != nil {
		return nil, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	parentID := pgtype.Int8{}
	workspaceID := pgtype.Int8{}
	if parentPublicID := strings.TrimSpace(input.ParentFolderPublicID); parentPublicID != "" && parentPublicID != "root" {
		parent, err := s.getDriveFolder(ctx, input.TenantID, DriveResourceRef{Type: DriveResourceTypeFolder, PublicID: input.ParentFolderPublicID})
		if err != nil {
			return nil, err
		}
		if err := s.authz.CanViewFolder(ctx, actor, parent); err != nil {
			s.auditDenied(ctx, actor, "drive.folder.children", "drive_folder", parent.PublicID, err, auditCtx)
			return nil, err
		}
		parentID = pgtype.Int8{Int64: parent.ID, Valid: true}
		if parent.WorkspaceID != nil {
			workspaceID = pgtype.Int8{Int64: *parent.WorkspaceID, Valid: true}
		}
	} else {
		workspace, err := s.resolveWorkspaceForCreate(ctx, input.TenantID, actor, input.WorkspacePublicID, nil)
		if err != nil {
			return nil, err
		}
		workspaceID = pgtype.Int8{Int64: workspace.ID, Valid: true}
	}

	folderRows, err := s.queries.ListDriveChildFolders(ctx, db.ListDriveChildFoldersParams{
		TenantID:       input.TenantID,
		WorkspaceID:    workspaceID,
		ParentFolderID: parentID,
		LimitCount:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list drive child folders: %w", err)
	}
	fileRows, err := s.queries.ListDriveChildFiles(ctx, db.ListDriveChildFilesParams{
		TenantID:      input.TenantID,
		WorkspaceID:   workspaceID,
		DriveFolderID: parentID,
		LimitCount:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list drive child files: %w", err)
	}

	folders := make([]DriveFolder, 0, len(folderRows))
	for _, row := range folderRows {
		folders = append(folders, driveFolderFromDB(row))
	}
	files := make([]DriveFile, 0, len(fileRows))
	for _, row := range fileRows {
		files = append(files, driveFileFromDB(row))
	}

	viewableFolders, err := s.authz.FilterViewableFolders(ctx, actor, folders)
	if err != nil {
		s.auditDenied(ctx, actor, "drive.folder.children", "drive_folder", strings.TrimSpace(input.ParentFolderPublicID), err, auditCtx)
		return nil, err
	}
	viewableFiles, err := s.authz.FilterViewableFiles(ctx, actor, files)
	if err != nil {
		s.auditDenied(ctx, actor, "drive.folder.children", "drive_folder", strings.TrimSpace(input.ParentFolderPublicID), err, auditCtx)
		return nil, err
	}

	items := make([]DriveItem, 0, len(viewableFolders)+len(viewableFiles))
	for i := range viewableFolders {
		folder := viewableFolders[i]
		items = append(items, DriveItem{Type: DriveItemTypeFolder, Folder: &folder})
	}
	for i := range viewableFiles {
		file := viewableFiles[i]
		items = append(items, DriveItem{Type: DriveItemTypeFile, File: &file})
	}
	return s.applyDriveListFilter(s.enrichDriveItems(ctx, actor, items), input.Filter), nil
}

func (s *DriveService) Search(ctx context.Context, input DriveSearchInput, auditCtx AuditContext) ([]DriveItem, error) {
	if err := s.ensureConfigured(false); err != nil {
		return nil, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	policy, err := s.drivePolicy(ctx, input.TenantID)
	if err != nil {
		return nil, err
	}
	if !policy.SearchEnabled {
		return nil, ErrDrivePolicyDenied
	}
	query := pgText(strings.TrimSpace(input.Query))
	contentType := pgText(normalizeContentType(input.ContentType))
	if strings.TrimSpace(input.ContentType) == "" {
		contentType = pgtype.Text{}
	}

	folderRows, err := s.queries.SearchDriveFolderCandidates(ctx, db.SearchDriveFolderCandidatesParams{
		TenantID:      input.TenantID,
		Query:         query,
		UpdatedAfter:  pgTimestampPtr(input.UpdatedAfter),
		UpdatedBefore: pgTimestampPtr(input.UpdatedBefore),
		LimitCount:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search drive folders: %w", err)
	}
	fileRows, err := s.queries.SearchDriveIndexedFileCandidates(ctx, db.SearchDriveIndexedFileCandidatesParams{
		TenantID:      input.TenantID,
		Query:         query,
		ContentType:   contentType,
		UpdatedAfter:  pgTimestampPtr(input.UpdatedAfter),
		UpdatedBefore: pgTimestampPtr(input.UpdatedBefore),
		LimitCount:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search drive files: %w", err)
	}
	folders := make([]DriveFolder, 0, len(folderRows))
	for _, row := range folderRows {
		folders = append(folders, driveFolderFromDB(row))
	}
	files := make([]DriveFile, 0, len(fileRows))
	for _, row := range fileRows {
		files = append(files, driveFileFromDB(row))
	}
	viewableFolders, err := s.authz.FilterViewableFolders(ctx, actor, folders)
	if err != nil {
		s.auditDenied(ctx, actor, "drive.search", "drive", "search", err, auditCtx)
		return nil, err
	}
	viewableFiles, err := s.authz.FilterViewableFiles(ctx, actor, files)
	if err != nil {
		s.auditDenied(ctx, actor, "drive.search", "drive", "search", err, auditCtx)
		return nil, err
	}
	items := make([]DriveItem, 0, len(viewableFolders)+len(viewableFiles))
	for i := range viewableFolders {
		folder := viewableFolders[i]
		items = append(items, DriveItem{Type: DriveItemTypeFolder, Folder: &folder})
	}
	for i := range viewableFiles {
		file := viewableFiles[i]
		items = append(items, DriveItem{Type: DriveItemTypeFile, File: &file})
	}
	return s.applyDriveListFilter(s.enrichDriveItems(ctx, actor, items), input.Filter), nil
}

func (s *DriveService) ListTrash(ctx context.Context, input DriveListTrashInput, auditCtx AuditContext) ([]DriveItem, error) {
	if err := s.ensureConfigured(false); err != nil {
		return nil, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	folderRows, err := s.queries.ListDeletedDriveFolders(ctx, db.ListDeletedDriveFoldersParams{
		TenantID:   input.TenantID,
		LimitCount: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list deleted drive folders: %w", err)
	}
	fileRows, err := s.queries.ListDeletedDriveFiles(ctx, db.ListDeletedDriveFilesParams{
		TenantID:   input.TenantID,
		LimitCount: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list deleted drive files: %w", err)
	}

	items := make([]DriveItem, 0, len(folderRows)+len(fileRows))
	for _, row := range folderRows {
		folder := driveFolderFromDB(row)
		if err := s.authz.CanRestoreFolder(ctx, actor, folder); err != nil {
			if errors.Is(err, ErrDrivePermissionDenied) || errors.Is(err, ErrDriveNotFound) {
				continue
			}
			s.auditDenied(ctx, actor, "drive.trash.list", "drive_folder", folder.PublicID, err, auditCtx)
			return nil, err
		}
		items = append(items, DriveItem{Type: DriveItemTypeFolder, Folder: &folder})
	}
	for _, row := range fileRows {
		file := driveFileFromDB(row)
		if err := s.authz.CanRestoreFile(ctx, actor, file); err != nil {
			if errors.Is(err, ErrDrivePermissionDenied) || errors.Is(err, ErrDriveNotFound) {
				continue
			}
			s.auditDenied(ctx, actor, "drive.trash.list", "drive_file", file.PublicID, err, auditCtx)
			return nil, err
		}
		items = append(items, DriveItem{Type: DriveItemTypeFile, File: &file})
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.trash.list", "drive", "trash", map[string]any{
		"count": len(items),
	})
	return items, nil
}

func (s *DriveService) UpdateFolder(ctx context.Context, input DriveUpdateFolderInput, auditCtx AuditContext) (DriveFolder, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveFolder{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveFolder{}, err
	}
	folder, err := s.getDriveFolder(ctx, input.TenantID, DriveResourceRef{Type: DriveResourceTypeFolder, PublicID: input.FolderPublicID})
	if err != nil {
		return DriveFolder{}, err
	}
	if err := s.ensureFolderMutationAllowed(ctx, input.TenantID, folder.ID); err != nil {
		return DriveFolder{}, err
	}
	if err := s.authz.CanEditFolder(ctx, actor, folder); err != nil {
		s.auditDenied(ctx, actor, "drive.folder.update", "drive_folder", folder.PublicID, err, auditCtx)
		return DriveFolder{}, err
	}

	if input.Name != nil {
		name := normalizeDriveName(*input.Name)
		if name == "" {
			return DriveFolder{}, fmt.Errorf("%w: folder name is required", ErrDriveInvalidInput)
		}
		row, err := s.queries.RenameDriveFolder(ctx, db.RenameDriveFolderParams{
			Name:     name,
			ID:       folder.ID,
			TenantID: input.TenantID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return DriveFolder{}, ErrDriveNotFound
		}
		if err != nil {
			return DriveFolder{}, fmt.Errorf("rename drive folder: %w", err)
		}
		folder = driveFolderFromDB(row)
	}

	if input.Description != nil {
		row, err := s.queries.UpdateDriveFolderDescription(ctx, db.UpdateDriveFolderDescriptionParams{
			Description: strings.TrimSpace(*input.Description),
			ID:          folder.ID,
			TenantID:    input.TenantID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return DriveFolder{}, ErrDriveNotFound
		}
		if err != nil {
			return DriveFolder{}, fmt.Errorf("update drive folder description: %w", err)
		}
		folder = driveFolderFromDB(row)
	}

	if input.Tags != nil {
		if err := s.replaceDriveItemTags(ctx, input.TenantID, actor.UserID, folder.ResourceRef(), *input.Tags); err != nil {
			return DriveFolder{}, err
		}
	}

	if input.ParentFolderPublicID != nil {
		moved, err := s.moveFolder(ctx, actor, folder, *input.ParentFolderPublicID)
		if err != nil {
			return DriveFolder{}, err
		}
		folder = moved
	}

	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.folder.update", "drive_folder", folder.PublicID, nil)
	s.recordDriveActivityBestEffort(ctx, actor, folder.ResourceRef(), "updated", map[string]any{"name": folder.Name})
	s.recordDriveSyncEventBestEffort(ctx, folder.ResourceRef(), "folder.updated", "", map[string]any{"name": folder.Name})
	return folder, nil
}

func (s *DriveService) DeleteFolder(ctx context.Context, tenantID, actorUserID int64, folderPublicID string, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	folder, err := s.getDriveFolder(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFolder, PublicID: folderPublicID})
	if err != nil {
		return err
	}
	if err := s.ensureFolderMutationAllowed(ctx, tenantID, folder.ID); err != nil {
		return err
	}
	if err := s.authz.CanDeleteFolder(ctx, actor, folder); err != nil {
		s.auditDenied(ctx, actor, "drive.folder.delete", "drive_folder", folder.PublicID, err, auditCtx)
		return err
	}
	childFolders, err := s.queries.ListDriveChildFolders(ctx, db.ListDriveChildFoldersParams{
		TenantID:       tenantID,
		ParentFolderID: pgtype.Int8{Int64: folder.ID, Valid: true},
		LimitCount:     1,
	})
	if err != nil {
		return fmt.Errorf("check drive child folders: %w", err)
	}
	childFiles, err := s.queries.ListDriveChildFiles(ctx, db.ListDriveChildFilesParams{
		TenantID:      tenantID,
		DriveFolderID: pgtype.Int8{Int64: folder.ID, Valid: true},
		LimitCount:    1,
	})
	if err != nil {
		return fmt.Errorf("check drive child files: %w", err)
	}
	if len(childFolders) > 0 || len(childFiles) > 0 {
		return fmt.Errorf("%w: folder must be empty before delete", ErrDriveInvalidInput)
	}
	_, _ = s.pool.Exec(ctx, `UPDATE drive_folders SET deleted_parent_folder_id = parent_folder_id WHERE id = $1 AND tenant_id = $2`, folder.ID, tenantID)
	if _, err := s.queries.SoftDeleteDriveFolder(ctx, db.SoftDeleteDriveFolderParams{
		DeletedByUserID: pgtype.Int8{Int64: actor.UserID, Valid: true},
		ID:              folder.ID,
		TenantID:        tenantID,
	}); errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	} else if err != nil {
		return fmt.Errorf("delete drive folder: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.folder.delete", "drive_folder", folder.PublicID, nil)
	s.recordDriveActivityBestEffort(ctx, actor, folder.ResourceRef(), "deleted", nil)
	s.recordDriveSyncEventBestEffort(ctx, folder.ResourceRef(), "folder.deleted", "", map[string]any{"name": folder.Name})
	return nil
}

func (s *DriveService) RestoreFolder(ctx context.Context, input DriveRestoreResourceInput, auditCtx AuditContext) (DriveFolder, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveFolder{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveFolder{}, err
	}
	publicID, err := uuid.Parse(strings.TrimSpace(input.ResourcePublicID))
	if err != nil {
		return DriveFolder{}, ErrDriveNotFound
	}
	row, err := s.queries.GetDeletedDriveFolderByPublicIDForTenant(ctx, db.GetDeletedDriveFolderByPublicIDForTenantParams{
		PublicID: publicID,
		TenantID: input.TenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveFolder{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveFolder{}, fmt.Errorf("get deleted drive folder: %w", err)
	}
	folder := driveFolderFromDB(row)
	if err := s.authz.CanRestoreFolder(ctx, actor, folder); err != nil {
		s.auditDenied(ctx, actor, "drive.folder.restore", "drive_folder", folder.PublicID, err, auditCtx)
		return DriveFolder{}, err
	}

	previousParentID := row.ParentFolderID
	if !previousParentID.Valid {
		previousParentID = row.DeletedParentFolderID
	}
	placement, err := s.resolveDriveRestorePlacement(ctx, actor, input.ParentFolderPublicID, previousParentID, row.WorkspaceID)
	if err != nil {
		return DriveFolder{}, err
	}
	resource := folder.ResourceRef()
	if err := s.applyDriveRestoreTuples(ctx, resource, placement); err != nil {
		s.rollbackDriveRestoreTuples(context.Background(), resource, placement)
		return DriveFolder{}, err
	}

	restored, err := s.queries.RestoreDriveFolder(ctx, db.RestoreDriveFolderParams{
		ParentFolderID: placement.parentID,
		WorkspaceID:    placement.workspaceID,
		ID:             folder.ID,
		TenantID:       input.TenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		s.rollbackDriveRestoreTuples(context.Background(), resource, placement)
		return DriveFolder{}, ErrDriveNotFound
	}
	if isUniqueViolation(err) {
		s.rollbackDriveRestoreTuples(context.Background(), resource, placement)
		return DriveFolder{}, fmt.Errorf("%w: active folder with this name already exists", ErrDriveInvalidInput)
	}
	if err != nil {
		s.rollbackDriveRestoreTuples(context.Background(), resource, placement)
		return DriveFolder{}, fmt.Errorf("restore drive folder: %w", err)
	}
	result := driveFolderFromDB(restored)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.folder.restore", "drive_folder", result.PublicID, nil)
	s.recordDriveActivityBestEffort(ctx, actor, result.ResourceRef(), "restored", nil)
	s.recordDriveSyncEventBestEffort(ctx, result.ResourceRef(), "folder.restored", "", map[string]any{"name": result.Name})
	return result, nil
}

func (s *DriveService) GetFile(ctx context.Context, tenantID, actorUserID int64, filePublicID string, auditCtx AuditContext) (DriveFile, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveFile{}, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveFile{}, err
	}
	row, err := s.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID})
	if err != nil {
		return DriveFile{}, err
	}
	file := driveFileFromDB(row)
	if err := s.authz.CanViewFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.file.view", "drive_file", file.PublicID, err, auditCtx)
		return DriveFile{}, err
	}
	s.recordDriveActivityBestEffort(ctx, actor, file.ResourceRef(), "viewed", nil)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.view", "drive_file", file.PublicID, nil)
	return file, nil
}

func (s *DriveService) DownloadFile(ctx context.Context, tenantID, actorUserID int64, filePublicID string, auditCtx AuditContext) (DriveFileDownload, error) {
	if err := s.ensureConfigured(true); err != nil {
		return DriveFileDownload{}, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveFileDownload{}, err
	}
	row, err := s.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID})
	if err != nil {
		return DriveFileDownload{}, err
	}
	file := driveFileFromDB(row)
	if err := s.authz.CanViewFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.file.download", "drive_file", file.PublicID, err, auditCtx)
		return DriveFileDownload{}, err
	}
	if err := s.authz.CanDownloadFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.file.download", "drive_file", file.PublicID, err, auditCtx)
		return DriveFileDownload{}, err
	}
	if err := s.ensureFileDownloadAllowed(ctx, actor, file, auditCtx, "drive.file.download"); err != nil {
		return DriveFileDownload{}, err
	}
	if err := s.ensureDriveEncryptionAvailable(ctx, file.TenantID, file.ID); err != nil {
		s.auditDenied(ctx, actor, "drive.file.download", "drive_file", file.PublicID, err, auditCtx)
		return DriveFileDownload{}, err
	}
	body, err := s.storage.Open(ctx, file.StorageKey)
	if err != nil {
		return DriveFileDownload{}, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.download", "drive_file", file.PublicID, map[string]any{
		"contentType": file.ContentType,
		"byteSize":    file.ByteSize,
	})
	s.recordDriveActivityBestEffort(ctx, actor, file.ResourceRef(), "downloaded", map[string]any{"byteSize": file.ByteSize})
	return DriveFileDownload{File: file, Body: body}, nil
}

func (s *DriveService) UpdateFile(ctx context.Context, input DriveUpdateFileInput, auditCtx AuditContext) (DriveFile, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveFile{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveFile{}, err
	}
	row, err := s.getDriveFileRow(ctx, input.TenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: input.FilePublicID})
	if err != nil {
		return DriveFile{}, err
	}
	file := driveFileFromDB(row)
	if err := s.ensureFileMutationAllowed(ctx, input.TenantID, file.ID); err != nil {
		return DriveFile{}, err
	}
	if err := s.authz.CanEditFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.file.update", "drive_file", file.PublicID, err, auditCtx)
		return DriveFile{}, err
	}

	if input.Filename != nil {
		filename := normalizeDriveName(filepath.Base(strings.TrimSpace(*input.Filename)))
		if filename == "" || filename == "." {
			return DriveFile{}, fmt.Errorf("%w: filename is required", ErrDriveInvalidInput)
		}
		renamed, err := s.queries.RenameDriveFile(ctx, db.RenameDriveFileParams{
			OriginalFilename: filename,
			ID:               file.ID,
			TenantID:         input.TenantID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return DriveFile{}, ErrDriveNotFound
		}
		if err != nil {
			return DriveFile{}, fmt.Errorf("rename drive file: %w", err)
		}
		file = driveFileFromDB(renamed)
	}

	if input.Description != nil {
		updated, err := s.queries.UpdateDriveFileDescription(ctx, db.UpdateDriveFileDescriptionParams{
			Description: strings.TrimSpace(*input.Description),
			ID:          file.ID,
			TenantID:    input.TenantID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return DriveFile{}, ErrDriveNotFound
		}
		if err != nil {
			return DriveFile{}, fmt.Errorf("update drive file description: %w", err)
		}
		file = driveFileFromDB(updated)
	}

	if input.Tags != nil {
		if err := s.replaceDriveItemTags(ctx, input.TenantID, actor.UserID, file.ResourceRef(), *input.Tags); err != nil {
			return DriveFile{}, err
		}
	}

	if input.ParentFolderPublicID != nil {
		moved, err := s.moveFile(ctx, actor, file, *input.ParentFolderPublicID)
		if err != nil {
			return DriveFile{}, err
		}
		file = moved
	}

	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.update", "drive_file", file.PublicID, nil)
	s.recordDriveActivityBestEffort(ctx, actor, file.ResourceRef(), "updated", map[string]any{"filename": file.OriginalFilename})
	s.indexDriveFileBestEffort(ctx, file, "metadata_changed")
	s.recordDriveSyncEventBestEffort(ctx, file.ResourceRef(), "file.metadata_updated", file.SHA256Hex, map[string]any{"filename": file.OriginalFilename})
	return file, nil
}

func (s *DriveService) OverwriteFile(ctx context.Context, input DriveOverwriteFileInput, auditCtx AuditContext) (DriveFile, error) {
	if err := s.ensureConfigured(true); err != nil {
		return DriveFile{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveFile{}, err
	}
	row, err := s.getDriveFileRow(ctx, input.TenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: input.FilePublicID})
	if err != nil {
		return DriveFile{}, err
	}
	file := driveFileFromDB(row)
	if err := s.ensureFileMutationAllowed(ctx, input.TenantID, file.ID); err != nil {
		return DriveFile{}, err
	}
	if err := s.authz.CanEditFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.file.update", "drive_file", file.PublicID, err, auditCtx)
		return DriveFile{}, err
	}
	if input.Body == nil {
		return DriveFile{}, NewDriveCodedError(ErrDriveInvalidInput, DriveErrorFileRequired, "File body is required.")
	}
	filename := strings.TrimSpace(input.Filename)
	if filename == "" {
		filename = file.OriginalFilename
	}
	filename = normalizeDriveName(filepath.Base(filename))
	if filename == "" || filename == "." {
		return DriveFile{}, NewDriveCodedError(ErrDriveInvalidInput, DriveErrorFilenameRequired, "Filename is required.")
	}
	contentType := normalizeContentType(input.ContentType)
	storageKey := newDriveStorageKey(input.TenantID, file.WorkspacePublicID, 1)
	maxBytes := int64(10 * 1024 * 1024)
	if s.files != nil && s.files.maxBytes > 0 {
		maxBytes = s.files.maxBytes
	}
	policy, err := s.drivePolicy(ctx, input.TenantID)
	if err != nil {
		return DriveFile{}, err
	}
	if policy.MaxFileSizeBytes > 0 && policy.MaxFileSizeBytes < maxBytes {
		maxBytes = policy.MaxFileSizeBytes
	}
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
	updated, err := s.queries.UpdateDriveFileObjectMetadata(ctx, db.UpdateDriveFileObjectMetadataParams{
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
	if errors.Is(err, pgx.ErrNoRows) {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveFile{}, ErrDriveNotFound
	}
	if err != nil {
		_ = s.storage.Delete(ctx, stored.Key)
		return DriveFile{}, fmt.Errorf("update drive file content: %w", err)
	}
	if filename != file.OriginalFilename {
		renamed, err := s.queries.RenameDriveFile(ctx, db.RenameDriveFileParams{
			OriginalFilename: filename,
			ID:               file.ID,
			TenantID:         input.TenantID,
		})
		if err == nil {
			updated = renamed
		}
	}
	_ = s.storage.Delete(ctx, file.StorageKey)
	result := driveFileFromDB(updated)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.update", "drive_file", result.PublicID, map[string]any{
		"contentType": result.ContentType,
		"byteSize":    result.ByteSize,
	})
	s.recordDriveActivityBestEffort(ctx, actor, result.ResourceRef(), "updated", map[string]any{"filename": result.OriginalFilename, "byteSize": result.ByteSize})
	s.recordDriveFilePreviewStateBestEffort(ctx, result)
	s.indexDriveFileBestEffort(ctx, result, "content_updated")
	s.enqueueDriveOCRBestEffort(ctx, actor, result, "overwrite")
	s.recordDriveSyncEventBestEffort(ctx, result.ResourceRef(), "file.updated", result.SHA256Hex, map[string]any{"filename": result.OriginalFilename})
	return result, nil
}

func (s *DriveService) DeleteFile(ctx context.Context, tenantID, actorUserID int64, filePublicID string, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	row, err := s.getDriveFileRow(ctx, tenantID, DriveResourceRef{Type: DriveResourceTypeFile, PublicID: filePublicID})
	if err != nil {
		return err
	}
	file := driveFileFromDB(row)
	if err := s.ensureFileMutationAllowed(ctx, tenantID, file.ID); err != nil {
		return err
	}
	if err := s.authz.CanDeleteFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.file.delete", "drive_file", file.PublicID, err, auditCtx)
		return err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE file_objects SET deleted_parent_folder_id = drive_folder_id WHERE id = $1 AND tenant_id = $2 AND purpose = 'drive'`, file.ID, tenantID)
	if _, err := s.queries.SoftDeleteDriveFile(ctx, db.SoftDeleteDriveFileParams{
		DeletedByUserID: pgtype.Int8{Int64: actor.UserID, Valid: true},
		ID:              file.ID,
		TenantID:        tenantID,
	}); errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	} else if err != nil {
		return fmt.Errorf("delete drive file: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.delete", "drive_file", file.PublicID, nil)
	s.recordDriveActivityBestEffort(ctx, actor, file.ResourceRef(), "deleted", nil)
	_ = s.queries.DeleteDriveSearchDocument(ctx, db.DeleteDriveSearchDocumentParams{TenantID: tenantID, FileObjectID: file.ID})
	s.recordDriveSyncEventBestEffort(ctx, file.ResourceRef(), "file.deleted", file.SHA256Hex, map[string]any{"filename": file.OriginalFilename})
	return nil
}

func (s *DriveService) RestoreFile(ctx context.Context, input DriveRestoreResourceInput, auditCtx AuditContext) (DriveFile, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveFile{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveFile{}, err
	}
	publicID, err := uuid.Parse(strings.TrimSpace(input.ResourcePublicID))
	if err != nil {
		return DriveFile{}, ErrDriveNotFound
	}
	row, err := s.queries.GetDeletedDriveFileByPublicIDForTenant(ctx, db.GetDeletedDriveFileByPublicIDForTenantParams{
		PublicID: publicID,
		TenantID: input.TenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveFile{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveFile{}, fmt.Errorf("get deleted drive file: %w", err)
	}
	file := driveFileFromDB(row)
	if err := s.authz.CanRestoreFile(ctx, actor, file); err != nil {
		s.auditDenied(ctx, actor, "drive.file.restore", "drive_file", file.PublicID, err, auditCtx)
		return DriveFile{}, err
	}

	previousParentID := row.DriveFolderID
	if !previousParentID.Valid {
		previousParentID = row.DeletedParentFolderID
	}
	placement, err := s.resolveDriveRestorePlacement(ctx, actor, input.ParentFolderPublicID, previousParentID, row.WorkspaceID)
	if err != nil {
		return DriveFile{}, err
	}
	resource := file.ResourceRef()
	if err := s.applyDriveRestoreTuples(ctx, resource, placement); err != nil {
		s.rollbackDriveRestoreTuples(context.Background(), resource, placement)
		return DriveFile{}, err
	}

	restored, err := s.queries.RestoreDriveFile(ctx, db.RestoreDriveFileParams{
		DriveFolderID: placement.parentID,
		WorkspaceID:   placement.workspaceID,
		ID:            file.ID,
		TenantID:      input.TenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		s.rollbackDriveRestoreTuples(context.Background(), resource, placement)
		return DriveFile{}, ErrDriveNotFound
	}
	if err != nil {
		s.rollbackDriveRestoreTuples(context.Background(), resource, placement)
		return DriveFile{}, fmt.Errorf("restore drive file: %w", err)
	}
	result := driveFileFromDB(restored)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.file.restore", "drive_file", result.PublicID, nil)
	s.recordDriveActivityBestEffort(ctx, actor, result.ResourceRef(), "restored", nil)
	s.indexDriveFileBestEffort(ctx, result, "file_restored")
	s.recordDriveSyncEventBestEffort(ctx, result.ResourceRef(), "file.restored", result.SHA256Hex, map[string]any{"filename": result.OriginalFilename})
	return result, nil
}

func (s *DriveService) ListPermissions(ctx context.Context, tenantID, actorUserID int64, resource DriveResourceRef, auditCtx AuditContext) (DrivePermissions, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DrivePermissions{}, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DrivePermissions{}, err
	}
	resolved, parent, inheritanceEnabled, err := s.resolvePermissionResource(ctx, actor, resource)
	if err != nil {
		return DrivePermissions{}, err
	}
	direct, err := s.permissionsForResource(ctx, tenantID, resolved, "")
	if err != nil {
		return DrivePermissions{}, err
	}
	result := DrivePermissions{Direct: direct}
	if inheritanceEnabled && parent != nil {
		inherited, err := s.permissionsForResource(ctx, tenantID, *parent, parent.PublicID)
		if err != nil {
			return DrivePermissions{}, err
		}
		result.Inherited = inherited
	}
	return result, nil
}

func (s *DriveService) ListGroups(ctx context.Context, tenantID, actorUserID int64, limit int32) ([]DriveGroup, error) {
	if err := s.ensureConfigured(false); err != nil {
		return nil, err
	}
	if _, err := s.actor(ctx, tenantID, actorUserID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.queries.ListDriveGroups(ctx, db.ListDriveGroupsParams{TenantID: tenantID, LimitCount: limit})
	if err != nil {
		return nil, fmt.Errorf("list drive groups: %w", err)
	}
	items := make([]DriveGroup, 0, len(rows))
	for _, row := range rows {
		items = append(items, driveGroupFromDB(row))
	}
	return items, nil
}

func (s *DriveService) GetGroup(ctx context.Context, tenantID, actorUserID int64, groupPublicID string) (DriveGroup, []string, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveGroup{}, nil, err
	}
	if _, err := s.actor(ctx, tenantID, actorUserID); err != nil {
		return DriveGroup{}, nil, err
	}
	group, err := s.getGroupByPublicID(ctx, tenantID, groupPublicID)
	if err != nil {
		return DriveGroup{}, nil, err
	}
	rows, err := s.queries.ListDriveGroupMembers(ctx, group.ID)
	if err != nil {
		return DriveGroup{}, nil, fmt.Errorf("list drive group members: %w", err)
	}
	members := make([]string, 0, len(rows))
	for _, row := range rows {
		user, err := s.queries.GetUserByID(ctx, row.UserID)
		if err == nil {
			members = append(members, user.PublicID.String())
		}
	}
	return group, members, nil
}

func (s *DriveService) UpdateGroup(ctx context.Context, tenantID, actorUserID int64, groupPublicID, name, description string, auditCtx AuditContext) (DriveGroup, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveGroup{}, err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return DriveGroup{}, err
	}
	group, err := s.getGroupByPublicID(ctx, tenantID, groupPublicID)
	if err != nil {
		return DriveGroup{}, err
	}
	name = normalizeDriveName(name)
	if name == "" {
		return DriveGroup{}, fmt.Errorf("%w: group name is required", ErrDriveInvalidInput)
	}
	row, err := s.queries.UpdateDriveGroup(ctx, db.UpdateDriveGroupParams{
		Name:        name,
		Description: strings.TrimSpace(description),
		ID:          group.ID,
		TenantID:    tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveGroup{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveGroup{}, fmt.Errorf("update drive group: %w", err)
	}
	updated := driveGroupFromDB(row)
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.group.update", "drive_group", updated.PublicID, nil)
	return updated, nil
}

func (s *DriveService) DeleteGroup(ctx context.Context, tenantID, actorUserID int64, groupPublicID string, auditCtx AuditContext) error {
	if err := s.ensureConfigured(false); err != nil {
		return err
	}
	actor, err := s.actor(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	group, err := s.getGroupByPublicID(ctx, tenantID, groupPublicID)
	if err != nil {
		return err
	}
	shares, err := s.queries.ListActiveDriveResourceSharesBySubject(ctx, db.ListActiveDriveResourceSharesBySubjectParams{
		TenantID:    tenantID,
		SubjectType: string(DriveShareSubjectGroup),
		SubjectID:   group.ID,
		LimitCount:  1,
	})
	if err != nil {
		return fmt.Errorf("check drive group shares: %w", err)
	}
	if len(shares) > 0 {
		return fmt.Errorf("%w: revoke group shares before delete", ErrDriveInvalidInput)
	}
	members, err := s.queries.ListDriveGroupMembers(ctx, group.ID)
	if err != nil {
		return fmt.Errorf("check drive group members: %w", err)
	}
	if len(members) > 0 {
		return fmt.Errorf("%w: remove group members before delete", ErrDriveInvalidInput)
	}
	if _, err := s.queries.SoftDeleteDriveGroup(ctx, db.SoftDeleteDriveGroupParams{ID: group.ID, TenantID: tenantID}); errors.Is(err, pgx.ErrNoRows) {
		return ErrDriveNotFound
	} else if err != nil {
		return fmt.Errorf("delete drive group: %w", err)
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.group.delete", "drive_group", group.PublicID, nil)
	return nil
}

func (s *DriveService) AddGroupMemberByPublicID(ctx context.Context, tenantID, actorUserID int64, groupPublicID, userPublicID string, auditCtx AuditContext) error {
	group, err := s.getGroupByPublicID(ctx, tenantID, groupPublicID)
	if err != nil {
		return err
	}
	userID, err := s.userIDForPublicID(ctx, tenantID, userPublicID)
	if err != nil {
		return err
	}
	return s.AddGroupMember(ctx, tenantID, actorUserID, group.ID, userID, auditCtx)
}

func (s *DriveService) RemoveGroupMemberByPublicID(ctx context.Context, tenantID, actorUserID int64, groupPublicID, userPublicID string, auditCtx AuditContext) error {
	group, err := s.getGroupByPublicID(ctx, tenantID, groupPublicID)
	if err != nil {
		return err
	}
	userID, err := s.userIDForPublicID(ctx, tenantID, userPublicID)
	if err != nil {
		return err
	}
	return s.RemoveGroupMember(ctx, tenantID, actorUserID, group.ID, userID, auditCtx)
}

func (s *DriveService) UpdateShareLink(ctx context.Context, input DriveUpdateShareLinkInput, auditCtx AuditContext) (DriveShareLink, error) {
	if err := s.ensureConfigured(false); err != nil {
		return DriveShareLink{}, err
	}
	actor, err := s.actor(ctx, input.TenantID, input.ActorUserID)
	if err != nil {
		return DriveShareLink{}, err
	}
	linkID, err := uuid.Parse(strings.TrimSpace(input.ShareLinkID))
	if err != nil {
		return DriveShareLink{}, ErrDriveNotFound
	}
	row, err := s.queries.GetDriveShareLinkByPublicIDForTenant(ctx, db.GetDriveShareLinkByPublicIDForTenantParams{PublicID: linkID, TenantID: input.TenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveShareLink{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveShareLink{}, fmt.Errorf("get drive share link: %w", err)
	}
	resource, err := s.resolveShareableResource(ctx, actor, DriveResourceRef{Type: DriveResourceType(row.ResourceType), ID: row.ResourceID, TenantID: row.TenantID})
	if err != nil {
		return DriveShareLink{}, err
	}
	policy, err := s.drivePolicy(ctx, input.TenantID)
	if err != nil {
		return DriveShareLink{}, err
	}
	link := driveShareLinkFromDB(row, resource)
	canDownload := link.CanDownload
	if input.CanDownload != nil {
		canDownload = *input.CanDownload && policy.ViewerDownloadEnabled
	}
	expiresAt := link.ExpiresAt
	if input.ExpiresAt != nil {
		expiresAt = *input.ExpiresAt
	}
	if expiresAt.IsZero() || !expiresAt.After(s.now()) {
		return DriveShareLink{}, fmt.Errorf("%w: share link expiry must be in the future", ErrDriveInvalidInput)
	}
	if policy.MaxShareLinkTTLHours > 0 && expiresAt.After(s.now().Add(time.Duration(policy.MaxShareLinkTTLHours)*time.Hour)) {
		return DriveShareLink{}, ErrDrivePolicyDenied
	}
	if err := s.authz.DeleteShareLinkTuple(ctx, link); err != nil {
		return DriveShareLink{}, err
	}
	updatedRow, err := s.queries.UpdateDriveShareLink(ctx, db.UpdateDriveShareLinkParams{
		CanDownload: canDownload,
		ExpiresAt:   pgTimestamp(expiresAt),
		PublicID:    linkID,
		TenantID:    input.TenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveShareLink{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveShareLink{}, fmt.Errorf("update drive share link: %w", err)
	}
	updated := driveShareLinkFromDB(updatedRow, resource)
	if err := s.authz.WriteShareLinkTuple(ctx, updated); err != nil {
		_, _ = s.queries.MarkDriveShareLinkPendingSync(context.Background(), db.MarkDriveShareLinkPendingSyncParams{ID: updated.ID, TenantID: updated.TenantID})
		s.auditFailed(ctx, actor, "drive.share_link.update", "drive_share_link", updated.PublicID, err, auditCtx)
		return DriveShareLink{}, err
	}
	s.recordAuditBestEffort(ctx, actor, auditCtx, "drive.share_link.update", "drive_share_link", updated.PublicID, map[string]any{
		"canDownload": updated.CanDownload,
		"expiresAt":   updated.ExpiresAt,
	})
	return updated, nil
}

func (s *DriveService) PublicShareLinkMetadata(ctx context.Context, token string) (DriveShareLink, *DriveFile, *DriveFolder, error) {
	link, file, folder, err := s.resolvePublicShareLink(ctx, token, false)
	if err != nil {
		return DriveShareLink{}, nil, nil, err
	}
	s.hydrateShareLinkPasswordState(ctx, &link)
	s.recordPublicLinkAudit(ctx, link, "drive.share_link.access", map[string]any{
		"resourceType": link.Resource.Type,
	})
	return link, file, folder, nil
}

func (s *DriveService) PublicShareLinkContent(ctx context.Context, token string) (DriveFileDownload, error) {
	return s.PublicShareLinkContentWithVerification(ctx, token, "")
}

func (s *DriveService) moveFolder(ctx context.Context, actor DriveActor, folder DriveFolder, parentPublicID string) (DriveFolder, error) {
	parentPublicID = strings.TrimSpace(parentPublicID)
	parentID := pgtype.Int8{}
	workspaceID := pgInt8(folder.WorkspaceID)
	var newParent *DriveFolder
	var newParentRef *DriveResourceRef
	if parentPublicID != "" && parentPublicID != "root" {
		parent, err := s.getDriveFolder(ctx, actor.TenantID, DriveResourceRef{Type: DriveResourceTypeFolder, PublicID: parentPublicID})
		if err != nil {
			return DriveFolder{}, err
		}
		if parent.ID == folder.ID {
			return DriveFolder{}, fmt.Errorf("%w: folder cannot be its own parent", ErrDriveInvalidInput)
		}
		isDescendant, err := s.queries.IsDriveFolderDescendant(ctx, db.IsDriveFolderDescendantParams{
			SourceFolderID:          folder.ID,
			TenantID:                actor.TenantID,
			CandidateParentFolderID: pgtype.Int8{Int64: parent.ID, Valid: true},
		})
		if err != nil {
			return DriveFolder{}, fmt.Errorf("check folder cycle: %w", err)
		}
		if isDescendant {
			return DriveFolder{}, fmt.Errorf("%w: folder move would create a cycle", ErrDriveInvalidInput)
		}
		if err := s.authz.CanEditFolder(ctx, actor, parent); err != nil {
			return DriveFolder{}, err
		}
		newParent = &parent
		workspaceID = pgInt8(parent.WorkspaceID)
		ref := parent.ResourceRef()
		newParentRef = &ref
		parentID = pgtype.Int8{Int64: parent.ID, Valid: true}
	}
	oldParent := folderParentRef(ctx, s, actor.TenantID, folder.ParentFolderID)
	if oldParent != nil {
		if err := s.authz.DeleteResourceParent(ctx, folder.ResourceRef(), *oldParent); err != nil {
			return DriveFolder{}, err
		}
	}
	if newParentRef != nil {
		if err := s.authz.WriteResourceParent(ctx, folder.ResourceRef(), *newParentRef); err != nil {
			if oldParent != nil {
				_ = s.authz.WriteResourceParent(context.Background(), folder.ResourceRef(), *oldParent)
			}
			return DriveFolder{}, err
		}
	}
	row, err := s.queries.MoveDriveFolder(ctx, db.MoveDriveFolderParams{
		ParentFolderID:     parentID,
		WorkspaceID:        workspaceID,
		InheritanceEnabled: newParent != nil,
		ID:                 folder.ID,
		TenantID:           actor.TenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		restoreFolderParent(context.Background(), s, folder.ResourceRef(), oldParent, newParentRef)
		return DriveFolder{}, ErrDriveNotFound
	}
	if err != nil {
		restoreFolderParent(context.Background(), s, folder.ResourceRef(), oldParent, newParentRef)
		return DriveFolder{}, fmt.Errorf("move drive folder: %w", err)
	}
	return driveFolderFromDB(row), nil
}

func (s *DriveService) moveFile(ctx context.Context, actor DriveActor, file DriveFile, parentPublicID string) (DriveFile, error) {
	parentPublicID = strings.TrimSpace(parentPublicID)
	parentID := pgtype.Int8{}
	workspaceID := pgInt8(file.WorkspaceID)
	var newParent *DriveFolder
	var newParentRef *DriveResourceRef
	if parentPublicID != "" && parentPublicID != "root" {
		parent, err := s.getDriveFolder(ctx, actor.TenantID, DriveResourceRef{Type: DriveResourceTypeFolder, PublicID: parentPublicID})
		if err != nil {
			return DriveFile{}, err
		}
		if err := s.authz.CanEditFolder(ctx, actor, parent); err != nil {
			return DriveFile{}, err
		}
		newParent = &parent
		workspaceID = pgInt8(parent.WorkspaceID)
		ref := parent.ResourceRef()
		newParentRef = &ref
		parentID = pgtype.Int8{Int64: parent.ID, Valid: true}
	}
	oldParent := fileParentRef(ctx, s, actor.TenantID, file.DriveFolderID)
	if oldParent != nil {
		if err := s.authz.DeleteResourceParent(ctx, file.ResourceRef(), *oldParent); err != nil {
			return DriveFile{}, err
		}
	}
	if newParentRef != nil {
		if err := s.authz.WriteResourceParent(ctx, file.ResourceRef(), *newParentRef); err != nil {
			if oldParent != nil {
				_ = s.authz.WriteResourceParent(context.Background(), file.ResourceRef(), *oldParent)
			}
			return DriveFile{}, err
		}
	}
	row, err := s.queries.MoveDriveFile(ctx, db.MoveDriveFileParams{
		DriveFolderID:      parentID,
		WorkspaceID:        workspaceID,
		InheritanceEnabled: newParent != nil,
		ID:                 file.ID,
		TenantID:           actor.TenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		restoreFolderParent(context.Background(), s, file.ResourceRef(), oldParent, newParentRef)
		return DriveFile{}, ErrDriveNotFound
	}
	if err != nil {
		restoreFolderParent(context.Background(), s, file.ResourceRef(), oldParent, newParentRef)
		return DriveFile{}, fmt.Errorf("move drive file: %w", err)
	}
	return driveFileFromDB(row), nil
}

func (s *DriveService) resolvePermissionResource(ctx context.Context, actor DriveActor, resource DriveResourceRef) (DriveResourceRef, *DriveResourceRef, bool, error) {
	switch resource.Type {
	case DriveResourceTypeFile:
		row, err := s.getDriveFileRow(ctx, actor.TenantID, resource)
		if err != nil {
			return DriveResourceRef{}, nil, false, err
		}
		file := driveFileFromDB(row)
		if err := s.authz.CanShareFile(ctx, actor, file); err != nil {
			return DriveResourceRef{}, nil, false, err
		}
		var parent *DriveResourceRef
		if file.DriveFolderID != nil {
			folder, err := s.getFolderByID(ctx, actor.TenantID, *file.DriveFolderID)
			if err != nil {
				return DriveResourceRef{}, nil, false, err
			}
			ref := folder.ResourceRef()
			parent = &ref
		}
		return file.ResourceRef(), parent, file.InheritanceEnabled, nil
	case DriveResourceTypeFolder:
		folder, err := s.getDriveFolder(ctx, actor.TenantID, resource)
		if err != nil {
			return DriveResourceRef{}, nil, false, err
		}
		if err := s.authz.CanShareFolder(ctx, actor, folder); err != nil {
			return DriveResourceRef{}, nil, false, err
		}
		var parent *DriveResourceRef
		if folder.ParentFolderID != nil {
			parentFolder, err := s.getFolderByID(ctx, actor.TenantID, *folder.ParentFolderID)
			if err != nil {
				return DriveResourceRef{}, nil, false, err
			}
			ref := parentFolder.ResourceRef()
			parent = &ref
		}
		return folder.ResourceRef(), parent, folder.InheritanceEnabled, nil
	default:
		return DriveResourceRef{}, nil, false, fmt.Errorf("%w: unsupported resource type", ErrDriveInvalidInput)
	}
}

func (s *DriveService) permissionsForResource(ctx context.Context, tenantID int64, resource DriveResourceRef, inheritedFromID string) ([]DrivePermission, error) {
	shares, err := s.queries.ListDriveResourceSharesByResource(ctx, db.ListDriveResourceSharesByResourceParams{
		TenantID:     tenantID,
		ResourceType: string(resource.Type),
		ResourceID:   resource.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("list drive shares: %w", err)
	}
	links, err := s.queries.ListDriveShareLinksByResource(ctx, db.ListDriveShareLinksByResourceParams{
		TenantID:     tenantID,
		ResourceType: string(resource.Type),
		ResourceID:   resource.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("list drive share links: %w", err)
	}
	items := make([]DrivePermission, 0, len(shares)+len(links))
	for _, row := range shares {
		subjectPublicID, err := s.resolveShareSubjectPublicID(ctx, tenantID, DriveShareSubjectType(row.SubjectType), row.SubjectID)
		if err != nil {
			subjectPublicID = ""
		}
		items = append(items, DrivePermission{
			Source:          permissionSource(inheritedFromID),
			Kind:            "share",
			PublicID:        row.PublicID.String(),
			Role:            DriveRole(row.Role),
			SubjectType:     row.SubjectType,
			SubjectID:       subjectPublicID,
			Status:          row.Status,
			InheritedFromID: inheritedFromID,
			CreatedAt:       row.CreatedAt.Time,
		})
	}
	for _, row := range links {
		canDownload := row.CanDownload
		expiresAt := row.ExpiresAt.Time
		items = append(items, DrivePermission{
			Source:          permissionSource(inheritedFromID),
			Kind:            "share_link",
			PublicID:        row.PublicID.String(),
			Role:            DriveRole(row.Role),
			CanDownload:     &canDownload,
			Status:          row.Status,
			ExpiresAt:       &expiresAt,
			InheritedFromID: inheritedFromID,
			CreatedAt:       row.CreatedAt.Time,
		})
	}
	return items, nil
}

func (s *DriveService) resolvePublicShareLink(ctx context.Context, token string, requireDownload bool) (DriveShareLink, *DriveFile, *DriveFolder, error) {
	if err := s.ensureConfigured(requireDownload); err != nil {
		return DriveShareLink{}, nil, nil, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return DriveShareLink{}, nil, nil, ErrDriveNotFound
	}
	tokenHash := driveShareLinkTokenHash(token)
	row, err := s.queries.LookupActiveDriveShareLinkByTokenHash(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveShareLink{}, nil, nil, ErrDriveNotFound
	}
	if err != nil {
		return DriveShareLink{}, nil, nil, fmt.Errorf("lookup drive share link: %w", err)
	}
	policy, err := s.drivePolicy(ctx, row.TenantID)
	if err != nil {
		return DriveShareLink{}, nil, nil, err
	}
	if !policy.LinkSharingEnabled || !policy.PublicLinksEnabled {
		return DriveShareLink{}, nil, nil, ErrDrivePolicyDenied
	}
	if requireDownload && !row.CanDownload {
		return DriveShareLink{}, nil, nil, ErrDrivePermissionDenied
	}
	link := driveShareLinkFromDB(row, DriveResourceRef{Type: DriveResourceType(row.ResourceType), ID: row.ResourceID, TenantID: row.TenantID})
	switch link.Resource.Type {
	case DriveResourceTypeFile:
		fileRow, err := s.getDriveFileRow(ctx, row.TenantID, link.Resource)
		if err != nil {
			return DriveShareLink{}, nil, nil, err
		}
		file := driveFileFromDB(fileRow)
		link.Resource = file.ResourceRef()
		if err := s.authz.CanViewWithShareLink(ctx, link); err != nil {
			return DriveShareLink{}, nil, nil, err
		}
		if requireDownload && !link.CanDownload {
			return DriveShareLink{}, nil, nil, ErrDrivePermissionDenied
		}
		return link, &file, nil, nil
	case DriveResourceTypeFolder:
		folder, err := s.getFolderByID(ctx, row.TenantID, row.ResourceID)
		if err != nil {
			return DriveShareLink{}, nil, nil, err
		}
		link.Resource = folder.ResourceRef()
		if err := s.authz.CanViewWithShareLink(ctx, link); err != nil {
			return DriveShareLink{}, nil, nil, err
		}
		if requireDownload {
			return DriveShareLink{}, nil, nil, ErrDriveInvalidInput
		}
		return link, nil, &folder, nil
	default:
		return DriveShareLink{}, nil, nil, ErrDriveNotFound
	}
}

func (s *DriveService) recordPublicLinkAudit(ctx context.Context, link DriveShareLink, action string, metadata map[string]any) {
	if s == nil || s.audit == nil {
		return
	}
	tenantID := link.TenantID
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["shareLinkId"] = link.PublicID
	s.audit.RecordBestEffort(ctx, AuditEventInput{
		AuditContext: AuditContext{
			ActorType: AuditActorSystem,
			TenantID:  &tenantID,
		},
		Action:     action,
		TargetType: "drive_share_link",
		TargetID:   link.PublicID,
		Metadata:   metadata,
	})
}

func (s *DriveService) getGroupByPublicID(ctx context.Context, tenantID int64, publicID string) (DriveGroup, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return DriveGroup{}, ErrDriveNotFound
	}
	row, err := s.queries.GetDriveGroupByPublicIDForTenant(ctx, db.GetDriveGroupByPublicIDForTenantParams{PublicID: parsed, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return DriveGroup{}, ErrDriveNotFound
	}
	if err != nil {
		return DriveGroup{}, fmt.Errorf("get drive group: %w", err)
	}
	return driveGroupFromDB(row), nil
}

func (s *DriveService) userIDForPublicID(ctx context.Context, tenantID int64, publicID string) (int64, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return 0, ErrDriveNotFound
	}
	user, err := s.queries.GetUserByPublicID(ctx, parsed)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrDriveNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get user: %w", err)
	}
	if _, err := s.ensureUserInTenant(ctx, tenantID, user.ID); err != nil {
		return 0, err
	}
	return user.ID, nil
}

func (s *DriveService) resolveShareSubjectID(ctx context.Context, tenantID int64, subjectType DriveShareSubjectType, subjectPublicID string) (int64, error) {
	switch subjectType {
	case DriveShareSubjectUser:
		return s.userIDForPublicID(ctx, tenantID, subjectPublicID)
	case DriveShareSubjectGroup:
		group, err := s.getGroupByPublicID(ctx, tenantID, subjectPublicID)
		if err != nil {
			return 0, err
		}
		return group.ID, nil
	default:
		return 0, fmt.Errorf("%w: unsupported share subject type", ErrDriveInvalidInput)
	}
}

type driveRestorePlacement struct {
	parentID     pgtype.Int8
	workspaceID  pgtype.Int8
	oldParent    *DriveResourceRef
	newParent    *DriveResourceRef
	oldWorkspace *DriveWorkspace
	newWorkspace *DriveWorkspace
}

func (s *DriveService) resolveDriveRestorePlacement(ctx context.Context, actor DriveActor, parentFolderPublicID *string, previousParentID, previousWorkspaceID pgtype.Int8) (driveRestorePlacement, error) {
	oldParent, err := s.driveFolderRefByIDIncludingDeleted(ctx, actor.TenantID, previousParentID)
	if err != nil {
		return driveRestorePlacement{}, err
	}
	oldWorkspace, err := s.driveWorkspaceByIDIncludingDeleted(ctx, actor.TenantID, previousWorkspaceID)
	if err != nil {
		return driveRestorePlacement{}, err
	}
	placement := driveRestorePlacement{
		oldParent:    oldParent,
		oldWorkspace: oldWorkspace,
	}

	if parentFolderPublicID != nil {
		parentPublicID := strings.TrimSpace(*parentFolderPublicID)
		if parentPublicID != "" && parentPublicID != "root" {
			parent, err := s.getDriveFolder(ctx, actor.TenantID, DriveResourceRef{Type: DriveResourceTypeFolder, PublicID: parentPublicID})
			if err != nil {
				return driveRestorePlacement{}, err
			}
			return s.driveRestorePlacementForParent(ctx, actor, parent, placement)
		}
		workspace, err := s.driveRestoreWorkspaceForRoot(ctx, actor, previousWorkspaceID)
		if err != nil {
			return driveRestorePlacement{}, err
		}
		placement.workspaceID = pgtype.Int8{Int64: workspace.ID, Valid: true}
		placement.newWorkspace = &workspace
		return placement, nil
	}

	if previousParentID.Valid {
		parent, err := s.getFolderByID(ctx, actor.TenantID, previousParentID.Int64)
		if err == nil {
			if err := s.authz.CanEditFolder(ctx, actor, parent); err == nil {
				return s.driveRestorePlacementForParent(ctx, actor, parent, placement)
			} else if !errors.Is(err, ErrDrivePermissionDenied) && !errors.Is(err, ErrDriveNotFound) {
				return driveRestorePlacement{}, err
			}
		} else if !errors.Is(err, ErrDriveNotFound) {
			return driveRestorePlacement{}, err
		}
	}

	workspace, err := s.driveRestoreWorkspaceForRoot(ctx, actor, previousWorkspaceID)
	if err != nil {
		return driveRestorePlacement{}, err
	}
	placement.workspaceID = pgtype.Int8{Int64: workspace.ID, Valid: true}
	placement.newWorkspace = &workspace
	return placement, nil
}

func (s *DriveService) driveRestorePlacementForParent(ctx context.Context, actor DriveActor, parent DriveFolder, placement driveRestorePlacement) (driveRestorePlacement, error) {
	if err := s.authz.CanEditFolder(ctx, actor, parent); err != nil {
		return driveRestorePlacement{}, err
	}
	workspace, err := s.driveRestoreWorkspaceForParent(ctx, actor, parent)
	if err != nil {
		return driveRestorePlacement{}, err
	}
	parentRef := parent.ResourceRef()
	placement.parentID = pgtype.Int8{Int64: parent.ID, Valid: true}
	placement.workspaceID = pgtype.Int8{Int64: workspace.ID, Valid: true}
	placement.newParent = &parentRef
	placement.newWorkspace = &workspace
	return placement, nil
}

func (s *DriveService) driveRestoreWorkspaceForParent(ctx context.Context, actor DriveActor, parent DriveFolder) (DriveWorkspace, error) {
	if parent.WorkspaceID != nil {
		return s.getWorkspaceByID(ctx, actor.TenantID, *parent.WorkspaceID)
	}
	return s.ensureDefaultWorkspace(ctx, actor.TenantID, actor)
}

func (s *DriveService) driveRestoreWorkspaceForRoot(ctx context.Context, actor DriveActor, previousWorkspaceID pgtype.Int8) (DriveWorkspace, error) {
	if previousWorkspaceID.Valid {
		workspace, err := s.getWorkspaceByID(ctx, actor.TenantID, previousWorkspaceID.Int64)
		if err == nil {
			return workspace, nil
		}
		if !errors.Is(err, ErrDriveNotFound) {
			return DriveWorkspace{}, err
		}
	}
	return s.ensureDefaultWorkspace(ctx, actor.TenantID, actor)
}

func (s *DriveService) applyDriveRestoreTuples(ctx context.Context, resource DriveResourceRef, placement driveRestorePlacement) error {
	if !sameDriveResourceRef(placement.oldParent, placement.newParent) {
		if placement.oldParent != nil {
			if err := s.authz.DeleteResourceParent(ctx, resource, *placement.oldParent); err != nil {
				return err
			}
		}
		if placement.newParent != nil {
			if err := s.authz.WriteResourceParent(ctx, resource, *placement.newParent); err != nil {
				return err
			}
		}
	}
	if !sameDriveWorkspace(placement.oldWorkspace, placement.newWorkspace) {
		if placement.oldWorkspace != nil {
			if err := s.authz.DeleteResourceWorkspace(ctx, resource, *placement.oldWorkspace); err != nil {
				return err
			}
		}
		if placement.newWorkspace != nil {
			if err := s.authz.WriteResourceWorkspace(ctx, resource, *placement.newWorkspace); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *DriveService) rollbackDriveRestoreTuples(ctx context.Context, resource DriveResourceRef, placement driveRestorePlacement) {
	if !sameDriveWorkspace(placement.oldWorkspace, placement.newWorkspace) {
		if placement.newWorkspace != nil {
			_ = s.authz.DeleteResourceWorkspace(ctx, resource, *placement.newWorkspace)
		}
		if placement.oldWorkspace != nil {
			_ = s.authz.WriteResourceWorkspace(ctx, resource, *placement.oldWorkspace)
		}
	}
	if !sameDriveResourceRef(placement.oldParent, placement.newParent) {
		if placement.newParent != nil {
			_ = s.authz.DeleteResourceParent(ctx, resource, *placement.newParent)
		}
		if placement.oldParent != nil {
			_ = s.authz.WriteResourceParent(ctx, resource, *placement.oldParent)
		}
	}
}

func (s *DriveService) driveFolderRefByIDIncludingDeleted(ctx context.Context, tenantID int64, folderID pgtype.Int8) (*DriveResourceRef, error) {
	if !folderID.Valid {
		return nil, nil
	}
	var publicID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT public_id FROM drive_folders WHERE id = $1 AND tenant_id = $2`, folderID.Int64, tenantID).Scan(&publicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get drive folder restore tuple ref: %w", err)
	}
	return &DriveResourceRef{Type: DriveResourceTypeFolder, ID: folderID.Int64, PublicID: publicID.String(), TenantID: tenantID}, nil
}

func (s *DriveService) driveWorkspaceByIDIncludingDeleted(ctx context.Context, tenantID int64, workspaceID pgtype.Int8) (*DriveWorkspace, error) {
	if !workspaceID.Valid {
		return nil, nil
	}
	var publicID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT public_id FROM drive_workspaces WHERE id = $1 AND tenant_id = $2`, workspaceID.Int64, tenantID).Scan(&publicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get drive workspace restore tuple ref: %w", err)
	}
	return &DriveWorkspace{ID: workspaceID.Int64, PublicID: publicID.String(), TenantID: tenantID}, nil
}

func sameDriveResourceRef(left, right *DriveResourceRef) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Type == right.Type && left.PublicID == right.PublicID
}

func sameDriveWorkspace(left, right *DriveWorkspace) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.PublicID == right.PublicID
}

func pgTimestampPtr(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgTimestamp(*value)
}

func permissionSource(inheritedFromID string) string {
	if strings.TrimSpace(inheritedFromID) == "" {
		return "direct"
	}
	return "inherited"
}

func driveShareLinkTokenHash(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func folderParentRef(ctx context.Context, s *DriveService, tenantID int64, parentID *int64) *DriveResourceRef {
	if parentID == nil {
		return nil
	}
	parent, err := s.getFolderByID(ctx, tenantID, *parentID)
	if err != nil {
		return nil
	}
	ref := parent.ResourceRef()
	return &ref
}

func fileParentRef(ctx context.Context, s *DriveService, tenantID int64, parentID *int64) *DriveResourceRef {
	return folderParentRef(ctx, s, tenantID, parentID)
}

func restoreFolderParent(ctx context.Context, s *DriveService, child DriveResourceRef, oldParent, newParent *DriveResourceRef) {
	if newParent != nil {
		_ = s.authz.DeleteResourceParent(ctx, child, *newParent)
	}
	if oldParent != nil {
		_ = s.authz.WriteResourceParent(ctx, child, *oldParent)
	}
}
