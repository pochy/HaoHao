package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DriveFolderBody struct {
	PublicID           string     `json:"publicId" example:"018f2f05-c6c9-7a49-b32d-04f4dd84ef4a"`
	WorkspacePublicID  string     `json:"workspacePublicId,omitempty"`
	Name               string     `json:"name" example:"Project"`
	Description        string     `json:"description,omitempty"`
	InheritanceEnabled bool       `json:"inheritanceEnabled"`
	CreatedAt          time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt          time.Time  `json:"updatedAt" format:"date-time"`
	DeletedAt          *time.Time `json:"deletedAt,omitempty" format:"date-time"`
}

type DriveFileBody struct {
	PublicID           string     `json:"publicId" example:"018f2f05-c6c9-7a49-b32d-04f4dd84ef4a"`
	WorkspacePublicID  string     `json:"workspacePublicId,omitempty"`
	OriginalFilename   string     `json:"originalFilename" example:"spec.md"`
	Description        string     `json:"description,omitempty"`
	ContentType        string     `json:"contentType" example:"text/markdown"`
	ByteSize           int64      `json:"byteSize"`
	SHA256Hex          string     `json:"sha256Hex"`
	Status             string     `json:"status"`
	ScanStatus         string     `json:"scanStatus"`
	DLPBlocked         bool       `json:"dlpBlocked"`
	InheritanceEnabled bool       `json:"inheritanceEnabled"`
	Locked             bool       `json:"locked"`
	LockReason         string     `json:"lockReason,omitempty"`
	CreatedAt          time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt          time.Time  `json:"updatedAt" format:"date-time"`
	LockedAt           *time.Time `json:"lockedAt,omitempty" format:"date-time"`
	DeletedAt          *time.Time `json:"deletedAt,omitempty" format:"date-time"`
}

type DriveWorkspaceBody struct {
	PublicID          string         `json:"publicId"`
	Name              string         `json:"name"`
	StorageQuotaBytes *int64         `json:"storageQuotaBytes,omitempty"`
	PolicyOverride    map[string]any `json:"policyOverride,omitempty"`
	CreatedAt         time.Time      `json:"createdAt" format:"date-time"`
	UpdatedAt         time.Time      `json:"updatedAt" format:"date-time"`
}

type DriveItemBody struct {
	Type              string           `json:"type" example:"file"`
	Folder            *DriveFolderBody `json:"folder,omitempty"`
	File              *DriveFileBody   `json:"file,omitempty"`
	OwnedByMe         bool             `json:"ownedByMe"`
	SharedWithMe      bool             `json:"sharedWithMe"`
	StarredByMe       bool             `json:"starredByMe"`
	OwnerUserPublicID string           `json:"ownerUserPublicId,omitempty"`
	OwnerDisplayName  string           `json:"ownerDisplayName,omitempty"`
	ShareRole         string           `json:"shareRole,omitempty"`
	Source            string           `json:"source,omitempty" example:"upload"`
	Tags              []string         `json:"tags,omitempty"`
}

type DriveActivityBody struct {
	PublicID          string         `json:"publicId"`
	ResourceType      string         `json:"resourceType"`
	Action            string         `json:"action"`
	ActorUserPublicID string         `json:"actorUserPublicId,omitempty"`
	ActorDisplayName  string         `json:"actorDisplayName,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	CreatedAt         time.Time      `json:"createdAt" format:"date-time"`
}

type DriveStorageUsageBody struct {
	QuotaBytes     *int64 `json:"quotaBytes,omitempty"`
	UsedBytes      int64  `json:"usedBytes"`
	TrashBytes     int64  `json:"trashBytes"`
	FileCount      int64  `json:"fileCount"`
	TrashFileCount int64  `json:"trashFileCount"`
	StorageDriver  string `json:"storageDriver"`
}

type DriveFolderTreeNodeBody struct {
	PublicID string                    `json:"publicId"`
	Name     string                    `json:"name"`
	Children []DriveFolderTreeNodeBody `json:"children"`
}

type DriveFolderTreeBody struct {
	OwnedRoots  []DriveFolderTreeNodeBody `json:"ownedRoots"`
	SharedRoots []DriveFolderTreeNodeBody `json:"sharedRoots"`
}

type DriveShareTargetBody struct {
	Type        string `json:"type" example:"user"`
	PublicID    string `json:"publicId"`
	DisplayName string `json:"displayName"`
	Secondary   string `json:"secondary,omitempty"`
}

type DrivePermissionBody struct {
	Source          string     `json:"source" example:"direct"`
	Kind            string     `json:"kind" example:"share"`
	PublicID        string     `json:"publicId,omitempty"`
	Role            string     `json:"role" example:"viewer"`
	SubjectType     string     `json:"subjectType,omitempty" example:"user"`
	SubjectID       string     `json:"subjectId,omitempty"`
	CanDownload     *bool      `json:"canDownload,omitempty"`
	Status          string     `json:"status,omitempty"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty" format:"date-time"`
	InheritedFromID string     `json:"inheritedFromId,omitempty"`
	CreatedAt       time.Time  `json:"createdAt" format:"date-time"`
}

type DrivePermissionsBody struct {
	Direct    []DrivePermissionBody `json:"direct"`
	Inherited []DrivePermissionBody `json:"inherited"`
}

type DriveShareBody struct {
	PublicID         string    `json:"publicId"`
	ResourceType     string    `json:"resourceType" example:"file"`
	ResourcePublicID string    `json:"resourcePublicId"`
	SubjectType      string    `json:"subjectType" example:"user"`
	SubjectPublicID  string    `json:"subjectPublicId"`
	Role             string    `json:"role" example:"viewer"`
	Status           string    `json:"status"`
	CreatedByUserID  int64     `json:"createdByUserId"`
	CreatedAt        time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt        time.Time `json:"updatedAt" format:"date-time"`
}

type DriveGroupBody struct {
	PublicID    string    `json:"publicId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Members     []string  `json:"members,omitempty"`
	CreatedAt   time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt   time.Time `json:"updatedAt" format:"date-time"`
}

type DriveShareLinkBody struct {
	PublicID         string    `json:"publicId"`
	ResourceType     string    `json:"resourceType" example:"file"`
	ResourcePublicID string    `json:"resourcePublicId"`
	Role             string    `json:"role" example:"viewer"`
	CanDownload      bool      `json:"canDownload"`
	PasswordRequired bool      `json:"passwordRequired"`
	ExpiresAt        time.Time `json:"expiresAt" format:"date-time"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt        time.Time `json:"updatedAt" format:"date-time"`
	Token            string    `json:"token,omitempty"`
}

type DriveNoContentOutput struct{}

func requireDriveTenant(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, service.TenantAccess, error) {
	if deps.DriveService == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error503ServiceUnavailable("drive service is not configured")
	}
	return requireActiveTenantRole(ctx, deps, sessionID, csrfToken, "", "drive service")
}

func toDriveHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrDriveInvalidInput), errors.Is(err, service.ErrInvalidFileInput):
		return huma.Error400BadRequest("invalid drive input")
	case errors.Is(err, service.ErrDriveAuthzUnavailable):
		return huma.Error503ServiceUnavailable("drive authorization unavailable")
	case errors.Is(err, service.ErrDrivePermissionDenied):
		return huma.Error403Forbidden("drive permission denied")
	case errors.Is(err, service.ErrDrivePolicyDenied):
		return huma.Error403Forbidden("drive policy denied")
	case errors.Is(err, service.ErrDriveLocked):
		return huma.Error409Conflict("drive resource is locked")
	case errors.Is(err, service.ErrDriveNotFound):
		return huma.Error404NotFound("drive resource not found")
	case errors.Is(err, service.ErrFileQuotaExceeded):
		return huma.Error409Conflict("file quota exceeded")
	default:
		return toHTTPError(err)
	}
}

func driveStatusCode(err error) int {
	switch {
	case errors.Is(err, service.ErrDriveInvalidInput), errors.Is(err, service.ErrInvalidFileInput):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrDriveAuthzUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, service.ErrDrivePermissionDenied), errors.Is(err, service.ErrDrivePolicyDenied):
		return http.StatusForbidden
	case errors.Is(err, service.ErrDriveLocked), errors.Is(err, service.ErrFileQuotaExceeded):
		return http.StatusConflict
	case errors.Is(err, service.ErrDriveNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func toDriveFolderBody(item service.DriveFolder) DriveFolderBody {
	return DriveFolderBody{
		PublicID:           item.PublicID,
		WorkspacePublicID:  item.WorkspacePublicID,
		Name:               item.Name,
		Description:        item.Description,
		InheritanceEnabled: item.InheritanceEnabled,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
		DeletedAt:          item.DeletedAt,
	}
}

func toDriveFileBody(item service.DriveFile) DriveFileBody {
	return DriveFileBody{
		PublicID:           item.PublicID,
		WorkspacePublicID:  item.WorkspacePublicID,
		OriginalFilename:   item.OriginalFilename,
		Description:        item.Description,
		ContentType:        item.ContentType,
		ByteSize:           item.ByteSize,
		SHA256Hex:          item.SHA256Hex,
		Status:             item.Status,
		ScanStatus:         item.ScanStatus,
		DLPBlocked:         item.DLPBlocked,
		InheritanceEnabled: item.InheritanceEnabled,
		Locked:             item.LockedAt != nil,
		LockReason:         item.LockReason,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
		LockedAt:           item.LockedAt,
		DeletedAt:          item.DeletedAt,
	}
}

func toDriveWorkspaceBody(item service.DriveWorkspace) DriveWorkspaceBody {
	return DriveWorkspaceBody{
		PublicID:          item.PublicID,
		Name:              item.Name,
		StorageQuotaBytes: item.StorageQuotaBytes,
		PolicyOverride:    item.PolicyOverride,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func toDriveItemBody(item service.DriveItem) DriveItemBody {
	out := DriveItemBody{
		Type:              string(item.Type),
		OwnedByMe:         item.Metadata.OwnedByMe,
		SharedWithMe:      item.Metadata.SharedWithMe,
		StarredByMe:       item.Metadata.StarredByMe,
		OwnerUserPublicID: item.Metadata.OwnerUserPublicID,
		OwnerDisplayName:  item.Metadata.OwnerDisplayName,
		ShareRole:         item.Metadata.ShareRole,
		Source:            item.Metadata.Source,
		Tags:              item.Metadata.Tags,
	}
	if item.Folder != nil {
		folder := toDriveFolderBody(*item.Folder)
		out.Folder = &folder
	}
	if item.File != nil {
		file := toDriveFileBody(*item.File)
		out.File = &file
	}
	return out
}

func toDriveActivityBody(item service.DriveActivity) DriveActivityBody {
	return DriveActivityBody{
		PublicID:          item.PublicID,
		ResourceType:      string(item.ResourceType),
		Action:            item.Action,
		ActorUserPublicID: item.ActorUserPublicID,
		ActorDisplayName:  item.ActorDisplayName,
		Metadata:          item.Metadata,
		CreatedAt:         item.CreatedAt,
	}
}

func toDriveStorageUsageBody(item service.DriveStorageUsage) DriveStorageUsageBody {
	return DriveStorageUsageBody{
		QuotaBytes:     item.QuotaBytes,
		UsedBytes:      item.UsedBytes,
		TrashBytes:     item.TrashBytes,
		FileCount:      item.FileCount,
		TrashFileCount: item.TrashFileCount,
		StorageDriver:  item.StorageDriver,
	}
}

func toDriveFolderTreeBody(item service.DriveFolderTree) DriveFolderTreeBody {
	return DriveFolderTreeBody{
		OwnedRoots:  toDriveFolderTreeNodeBodies(item.OwnedRoots),
		SharedRoots: toDriveFolderTreeNodeBodies(item.SharedRoots),
	}
}

func toDriveFolderTreeNodeBodies(items []service.DriveFolderTreeNode) []DriveFolderTreeNodeBody {
	out := make([]DriveFolderTreeNodeBody, 0, len(items))
	for _, item := range items {
		out = append(out, DriveFolderTreeNodeBody{
			PublicID: item.PublicID,
			Name:     item.Name,
			Children: toDriveFolderTreeNodeBodies(item.Children),
		})
	}
	return out
}

func toDriveShareTargetBody(item service.DriveShareTarget) DriveShareTargetBody {
	return DriveShareTargetBody{
		Type:        item.Type,
		PublicID:    item.PublicID,
		DisplayName: item.DisplayName,
		Secondary:   item.Secondary,
	}
}

func toDrivePermissionBody(item service.DrivePermission) DrivePermissionBody {
	return DrivePermissionBody{
		Source:          item.Source,
		Kind:            item.Kind,
		PublicID:        item.PublicID,
		Role:            string(item.Role),
		SubjectType:     item.SubjectType,
		SubjectID:       item.SubjectID,
		CanDownload:     item.CanDownload,
		Status:          item.Status,
		ExpiresAt:       item.ExpiresAt,
		InheritedFromID: item.InheritedFromID,
		CreatedAt:       item.CreatedAt,
	}
}

func toDrivePermissionsBody(items service.DrivePermissions) DrivePermissionsBody {
	out := DrivePermissionsBody{
		Direct:    make([]DrivePermissionBody, 0, len(items.Direct)),
		Inherited: make([]DrivePermissionBody, 0, len(items.Inherited)),
	}
	for _, item := range items.Direct {
		out.Direct = append(out.Direct, toDrivePermissionBody(item))
	}
	for _, item := range items.Inherited {
		out.Inherited = append(out.Inherited, toDrivePermissionBody(item))
	}
	return out
}

func toDriveShareBody(item service.DriveShare) DriveShareBody {
	return DriveShareBody{
		PublicID:         item.PublicID,
		ResourceType:     string(item.Resource.Type),
		ResourcePublicID: item.Resource.PublicID,
		SubjectType:      string(item.SubjectType),
		SubjectPublicID:  item.SubjectPublicID,
		Role:             string(item.Role),
		Status:           item.Status,
		CreatedByUserID:  item.CreatedByUserID,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func toDriveGroupBody(item service.DriveGroup, members []string) DriveGroupBody {
	return DriveGroupBody{
		PublicID:    item.PublicID,
		Name:        item.Name,
		Description: item.Description,
		Members:     members,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toDriveShareLinkBody(item service.DriveShareLink, includeToken bool) DriveShareLinkBody {
	out := DriveShareLinkBody{
		PublicID:         item.PublicID,
		ResourceType:     string(item.Resource.Type),
		ResourcePublicID: item.Resource.PublicID,
		Role:             string(item.Role),
		CanDownload:      item.CanDownload,
		PasswordRequired: item.PasswordRequired,
		ExpiresAt:        item.ExpiresAt,
		Status:           item.Status,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
	if includeToken {
		out.Token = item.RawToken
	}
	return out
}
