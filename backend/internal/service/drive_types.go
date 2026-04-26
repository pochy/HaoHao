package service

import (
	"encoding/json"
	"io"
	"time"

	db "example.com/haohao/backend/internal/db"
)

type DriveResourceType string

const (
	DriveResourceTypeFile      DriveResourceType = "file"
	DriveResourceTypeFolder    DriveResourceType = "folder"
	DriveResourceTypeWorkspace DriveResourceType = "workspace"
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
	WorkspaceID        *int64
	WorkspacePublicID  string
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
	WorkspaceID        *int64
	WorkspacePublicID  string
	UploadedByUserID   *int64
	DriveFolderID      *int64
	OriginalFilename   string
	ContentType        string
	ByteSize           int64
	SHA256Hex          string
	StorageDriver      string
	StorageKey         string
	Status             string
	ScanStatus         string
	ScanReason         string
	ScanEngine         string
	ScannedAt          *time.Time
	DLPBlocked         bool
	UploadState        string
	InheritanceEnabled bool
	LockedAt           *time.Time
	LockReason         string
	DeletedAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type DriveWorkspace struct {
	ID                int64
	PublicID          string
	TenantID          int64
	Name              string
	RootFolderID      *int64
	CreatedByUserID   *int64
	StorageQuotaBytes *int64
	PolicyOverride    map[string]any
	DeletedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
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
	ID               int64
	PublicID         string
	TenantID         int64
	Resource         DriveResourceRef
	Role             DriveRole
	CanDownload      bool
	PasswordRequired bool
	ExpiresAt        time.Time
	Status           string
	CreatedByUserID  int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	RawToken         string
}

type DriveShareInvitation struct {
	ID                 int64
	PublicID           string
	TenantID           int64
	Resource           DriveResourceRef
	InviteeEmailDomain string
	MaskedInviteeEmail string
	InviteeUserID      *int64
	Role               DriveRole
	Status             string
	ExpiresAt          time.Time
	ApprovedByUserID   *int64
	ApprovedAt         *time.Time
	AcceptedAt         *time.Time
	CreatedByUserID    int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	RawAcceptToken     string
}

type DriveAdminShareState struct {
	PublicID         string
	ResourceType     string
	ResourcePublicID string
	ResourceName     string
	SubjectType      string
	SubjectPublicID  string
	Role             string
	Status           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type DriveAdminShareLinkState struct {
	PublicID         string
	ResourceType     string
	ResourcePublicID string
	ResourceName     string
	CanDownload      bool
	PasswordRequired bool
	Status           string
	ExpiresAt        time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type DriveAdminAuditEvent struct {
	PublicID   string
	ActorType  string
	Action     string
	TargetType string
	TargetID   string
	Metadata   map[string]any
	OccurredAt time.Time
}

type DriveOpenFGASyncItem struct {
	Kind     string
	PublicID string
	Status   string
	Action   string
	Error    string
}

type DriveOpenFGASyncResult struct {
	DryRun bool
	Items  []DriveOpenFGASyncItem
}

type DriveItemType string

const (
	DriveItemTypeFolder DriveItemType = "folder"
	DriveItemTypeFile   DriveItemType = "file"
)

type DriveItem struct {
	Type   DriveItemType
	Folder *DriveFolder
	File   *DriveFile
}

type DrivePermission struct {
	Source          string
	Kind            string
	PublicID        string
	Role            DriveRole
	SubjectType     string
	SubjectID       string
	CanDownload     *bool
	Status          string
	ExpiresAt       *time.Time
	InheritedFromID string
	CreatedAt       time.Time
}

type DrivePermissions struct {
	Direct    []DrivePermission
	Inherited []DrivePermission
}

type DriveFileDownload struct {
	File DriveFile
	Body FileReadCloser
}

type DriveAdminContentAccessSession struct {
	ID             int64
	PublicID       string
	TenantID       int64
	ActorUserID    int64
	Reason         string
	ReasonCategory string
	ExpiresAt      time.Time
	EndedAt        *time.Time
	CreatedAt      time.Time
}

type DriveOperationsHealth struct {
	TenantID                int64
	WorkspaceCount          int64
	MissingWorkspaceCount   int64
	OpenFGADriftCount       int
	StorageMissingCount     int
	StorageOrphanCheckState string
	CheckedAt               time.Time
}

type DriveCreateFolderInput struct {
	TenantID             int64
	ActorUserID          int64
	WorkspacePublicID    string
	ParentFolderID       *int64
	ParentFolderPublicID string
	Name                 string
}

type DriveUpdateFolderInput struct {
	TenantID             int64
	ActorUserID          int64
	FolderPublicID       string
	Name                 *string
	ParentFolderPublicID *string
}

type DriveUploadFileInput struct {
	TenantID             int64
	ActorUserID          int64
	WorkspacePublicID    string
	ParentFolderID       *int64
	ParentFolderPublicID string
	Filename             string
	ContentType          string
	Body                 io.Reader
}

type DriveUpdateFileInput struct {
	TenantID             int64
	ActorUserID          int64
	FilePublicID         string
	Filename             *string
	ParentFolderPublicID *string
}

type DriveOverwriteFileInput struct {
	TenantID     int64
	ActorUserID  int64
	FilePublicID string
	Filename     string
	ContentType  string
	Body         io.Reader
}

type DriveListChildrenInput struct {
	TenantID             int64
	ActorUserID          int64
	WorkspacePublicID    string
	ParentFolderPublicID string
	Limit                int32
}

type DriveSearchInput struct {
	TenantID      int64
	ActorUserID   int64
	Query         string
	ContentType   string
	UpdatedAfter  *time.Time
	UpdatedBefore *time.Time
	Limit         int32
}

type DriveCreateShareInput struct {
	TenantID        int64
	ActorUserID     int64
	Resource        DriveResourceRef
	SubjectType     DriveShareSubjectType
	SubjectID       int64
	SubjectPublicID string
	Role            DriveRole
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
	Role        DriveRole
	CanDownload bool
	ExpiresAt   time.Time
	Password    string
}

type DriveCreateWorkspaceInput struct {
	TenantID           int64
	ActorUserID        int64
	Name               string
	StorageQuotaBytes  *int64
	PolicyOverrideJSON []byte
}

type DriveUpdateWorkspaceInput struct {
	TenantID           int64
	ActorUserID        int64
	WorkspacePublicID  string
	Name               string
	StorageQuotaBytes  *int64
	PolicyOverrideJSON []byte
}

type DriveStartAdminContentAccessInput struct {
	TenantID       int64
	ActorUserID    int64
	Reason         string
	ReasonCategory string
	TTL            time.Duration
}

type DrivePublicEditorOverwriteInput struct {
	Token              string
	VerificationCookie string
	Filename           string
	ContentType        string
	Body               io.Reader
}

type DriveCreateShareInvitationInput struct {
	TenantID            int64
	ActorUserID         int64
	Resource            DriveResourceRef
	InviteeEmail        string
	InviteeUserPublicID string
	Role                DriveRole
	ExpiresAt           time.Time
}

type DriveAcceptShareInvitationInput struct {
	ActorUserID        int64
	InvitationPublicID string
	AcceptToken        string
}

type DriveRevokeShareInvitationInput struct {
	TenantID           int64
	ActorUserID        int64
	InvitationPublicID string
}

type DriveUpdateShareLinkInput struct {
	TenantID    int64
	ActorUserID int64
	ShareLinkID string
	CanDownload *bool
	ExpiresAt   *time.Time
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

func (w DriveWorkspace) ResourceRef() DriveResourceRef {
	return DriveResourceRef{Type: DriveResourceTypeWorkspace, ID: w.ID, PublicID: w.PublicID, TenantID: w.TenantID}
}

func driveFolderFromDB(row db.DriveFolder) DriveFolder {
	return DriveFolder{
		ID:                 row.ID,
		PublicID:           row.PublicID.String(),
		TenantID:           row.TenantID,
		WorkspaceID:        optionalPgInt8(row.WorkspaceID),
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
		WorkspaceID:        optionalPgInt8(row.WorkspaceID),
		UploadedByUserID:   optionalPgInt8(row.UploadedByUserID),
		DriveFolderID:      optionalPgInt8(row.DriveFolderID),
		OriginalFilename:   row.OriginalFilename,
		ContentType:        row.ContentType,
		ByteSize:           row.ByteSize,
		SHA256Hex:          row.Sha256Hex,
		StorageDriver:      row.StorageDriver,
		StorageKey:         row.StorageKey,
		Status:             row.Status,
		ScanStatus:         row.ScanStatus,
		ScanReason:         optionalText(row.ScanReason),
		ScanEngine:         optionalText(row.ScanEngine),
		ScannedAt:          optionalPgTime(row.ScannedAt),
		DLPBlocked:         row.DlpBlocked,
		UploadState:        row.UploadState,
		InheritanceEnabled: row.InheritanceEnabled,
		LockedAt:           optionalPgTime(row.LockedAt),
		LockReason:         optionalText(row.LockReason),
		DeletedAt:          optionalPgTime(row.DeletedAt),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}
}

func driveWorkspaceFromDB(row db.DriveWorkspace) DriveWorkspace {
	override := map[string]any{}
	if len(row.PolicyOverride) > 0 {
		_ = json.Unmarshal(row.PolicyOverride, &override)
	}
	return DriveWorkspace{
		ID:                row.ID,
		PublicID:          row.PublicID.String(),
		TenantID:          row.TenantID,
		Name:              row.Name,
		RootFolderID:      optionalPgInt8(row.RootFolderID),
		CreatedByUserID:   optionalPgInt8(row.CreatedByUserID),
		StorageQuotaBytes: optionalPgInt8(row.StorageQuotaBytes),
		PolicyOverride:    override,
		DeletedAt:         optionalPgTime(row.DeletedAt),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
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
