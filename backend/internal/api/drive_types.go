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
	PublicID           string    `json:"publicId" example:"018f2f05-c6c9-7a49-b32d-04f4dd84ef4a"`
	Name               string    `json:"name" example:"Project"`
	InheritanceEnabled bool      `json:"inheritanceEnabled"`
	CreatedAt          time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt          time.Time `json:"updatedAt" format:"date-time"`
}

type DriveFileBody struct {
	PublicID           string     `json:"publicId" example:"018f2f05-c6c9-7a49-b32d-04f4dd84ef4a"`
	OriginalFilename   string     `json:"originalFilename" example:"spec.md"`
	ContentType        string     `json:"contentType" example:"text/markdown"`
	ByteSize           int64      `json:"byteSize"`
	SHA256Hex          string     `json:"sha256Hex"`
	Status             string     `json:"status"`
	InheritanceEnabled bool       `json:"inheritanceEnabled"`
	Locked             bool       `json:"locked"`
	LockReason         string     `json:"lockReason,omitempty"`
	CreatedAt          time.Time  `json:"createdAt" format:"date-time"`
	UpdatedAt          time.Time  `json:"updatedAt" format:"date-time"`
	LockedAt           *time.Time `json:"lockedAt,omitempty" format:"date-time"`
}

type DriveItemBody struct {
	Type   string           `json:"type" example:"file"`
	Folder *DriveFolderBody `json:"folder,omitempty"`
	File   *DriveFileBody   `json:"file,omitempty"`
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
		Name:               item.Name,
		InheritanceEnabled: item.InheritanceEnabled,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func toDriveFileBody(item service.DriveFile) DriveFileBody {
	return DriveFileBody{
		PublicID:           item.PublicID,
		OriginalFilename:   item.OriginalFilename,
		ContentType:        item.ContentType,
		ByteSize:           item.ByteSize,
		SHA256Hex:          item.SHA256Hex,
		Status:             item.Status,
		InheritanceEnabled: item.InheritanceEnabled,
		Locked:             item.LockedAt != nil,
		LockReason:         item.LockReason,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
		LockedAt:           item.LockedAt,
	}
}

func toDriveItemBody(item service.DriveItem) DriveItemBody {
	out := DriveItemBody{Type: string(item.Type)}
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
