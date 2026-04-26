package service

import (
	"io"
	"time"

	db "example.com/haohao/backend/internal/db"
)

type DriveResourceType string

const (
	DriveResourceTypeFile   DriveResourceType = "file"
	DriveResourceTypeFolder DriveResourceType = "folder"
)

type DriveRole string

const (
	DriveRoleOwner  DriveRole = "owner"
	DriveRoleEditor DriveRole = "editor"
	DriveRoleViewer DriveRole = "viewer"
)

type DriveShareSubjectType string

const (
	DriveShareSubjectUser  DriveShareSubjectType = "user"
	DriveShareSubjectGroup DriveShareSubjectType = "group"
)

type DriveActor struct {
	UserID   int64
	PublicID string
	TenantID int64
}

type DriveResourceRef struct {
	Type     DriveResourceType
	ID       int64
	PublicID string
	TenantID int64
}

type DriveFolder struct {
	ID                 int64
	PublicID           string
	TenantID           int64
	ParentFolderID     *int64
	Name               string
	CreatedByUserID    int64
	InheritanceEnabled bool
	DeletedAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type DriveFile struct {
	ID                 int64
	PublicID           string
	TenantID           int64
	UploadedByUserID   *int64
	DriveFolderID      *int64
	OriginalFilename   string
	ContentType        string
	ByteSize           int64
	SHA256Hex          string
	StorageDriver      string
	StorageKey         string
	Status             string
	InheritanceEnabled bool
	LockedAt           *time.Time
	LockReason         string
	DeletedAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type DriveGroup struct {
	ID              int64
	PublicID        string
	TenantID        int64
	Name            string
	Description     string
	CreatedByUserID int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DriveShare struct {
	ID              int64
	PublicID        string
	TenantID        int64
	Resource        DriveResourceRef
	SubjectType     DriveShareSubjectType
	SubjectID       int64
	SubjectPublicID string
	Role            DriveRole
	Status          string
	CreatedByUserID int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DriveShareLink struct {
	ID              int64
	PublicID        string
	TenantID        int64
	Resource        DriveResourceRef
	Role            DriveRole
	CanDownload     bool
	ExpiresAt       time.Time
	Status          string
	CreatedByUserID int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	RawToken        string
}

type DriveCreateFolderInput struct {
	TenantID       int64
	ActorUserID    int64
	ParentFolderID *int64
	Name           string
}

type DriveUploadFileInput struct {
	TenantID       int64
	ActorUserID    int64
	ParentFolderID *int64
	Filename       string
	ContentType    string
	Body           io.Reader
}

type DriveCreateShareInput struct {
	TenantID    int64
	ActorUserID int64
	Resource    DriveResourceRef
	SubjectType DriveShareSubjectType
	SubjectID   int64
	Role        DriveRole
}

type DriveRevokeShareInput struct {
	TenantID    int64
	ActorUserID int64
	ShareID     string
}

type DriveCreateShareLinkInput struct {
	TenantID    int64
	ActorUserID int64
	Resource    DriveResourceRef
	CanDownload bool
	ExpiresAt   time.Time
}

type DriveDisableShareLinkInput struct {
	TenantID    int64
	ActorUserID int64
	ShareLinkID string
}

func (f DriveFolder) ResourceRef() DriveResourceRef {
	return DriveResourceRef{Type: DriveResourceTypeFolder, ID: f.ID, PublicID: f.PublicID, TenantID: f.TenantID}
}

func (f DriveFile) ResourceRef() DriveResourceRef {
	return DriveResourceRef{Type: DriveResourceTypeFile, ID: f.ID, PublicID: f.PublicID, TenantID: f.TenantID}
}

func driveFolderFromDB(row db.DriveFolder) DriveFolder {
	return DriveFolder{
		ID:                 row.ID,
		PublicID:           row.PublicID.String(),
		TenantID:           row.TenantID,
		ParentFolderID:     optionalPgInt8(row.ParentFolderID),
		Name:               row.Name,
		CreatedByUserID:    row.CreatedByUserID,
		InheritanceEnabled: row.InheritanceEnabled,
		DeletedAt:          optionalPgTime(row.DeletedAt),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}
}

func driveFileFromDB(row db.FileObject) DriveFile {
	return DriveFile{
		ID:                 row.ID,
		PublicID:           row.PublicID.String(),
		TenantID:           row.TenantID,
		UploadedByUserID:   optionalPgInt8(row.UploadedByUserID),
		DriveFolderID:      optionalPgInt8(row.DriveFolderID),
		OriginalFilename:   row.OriginalFilename,
		ContentType:        row.ContentType,
		ByteSize:           row.ByteSize,
		SHA256Hex:          row.Sha256Hex,
		StorageDriver:      row.StorageDriver,
		StorageKey:         row.StorageKey,
		Status:             row.Status,
		InheritanceEnabled: row.InheritanceEnabled,
		LockedAt:           optionalPgTime(row.LockedAt),
		LockReason:         optionalText(row.LockReason),
		DeletedAt:          optionalPgTime(row.DeletedAt),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}
}

func driveGroupFromDB(row db.DriveGroup) DriveGroup {
	return DriveGroup{
		ID:              row.ID,
		PublicID:        row.PublicID.String(),
		TenantID:        row.TenantID,
		Name:            row.Name,
		Description:     row.Description,
		CreatedByUserID: row.CreatedByUserID,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}
}
