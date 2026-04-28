package service

import "errors"

var (
	ErrDriveAuthzUnavailable = errors.New("drive authorization unavailable")
	ErrDrivePermissionDenied = errors.New("drive permission denied")
	ErrDriveNotFound         = errors.New("drive resource not found")
	ErrDriveInvalidInput     = errors.New("invalid drive input")
	ErrDriveLocked           = errors.New("drive resource is locked")
	ErrDrivePolicyDenied     = errors.New("drive policy denied")
)

const (
	DriveErrorFileRequired         = "drive.file_required"
	DriveErrorInvalidMultipart     = "drive.invalid_multipart"
	DriveErrorFilenameRequired     = "drive.filename_required"
	DriveErrorFileTooLarge         = "drive.file_too_large"
	DriveErrorWorkspaceNotFound    = "drive.workspace_not_found"
	DriveErrorParentFolderNotFound = "drive.parent_folder_not_found"
	DriveErrorPermissionDenied     = "drive.permission_denied"
	DriveErrorPolicyDenied         = "drive.policy_denied"
	DriveErrorQuotaExceeded        = "drive.quota_exceeded"
)

type DriveCodedError struct {
	Code   string
	Detail string
	Err    error
}

func NewDriveCodedError(err error, code, detail string) error {
	return DriveCodedError{Code: code, Detail: detail, Err: err}
}

func (e DriveCodedError) Error() string {
	if e.Detail != "" {
		return e.Detail
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e DriveCodedError) Unwrap() error {
	return e.Err
}

func DriveErrorCodeOf(err error) (string, bool) {
	var coded DriveCodedError
	if errors.As(err, &coded) && coded.Code != "" {
		return coded.Code, true
	}
	return "", false
}

func DriveErrorDetailOf(err error) (string, bool) {
	var coded DriveCodedError
	if errors.As(err, &coded) && coded.Detail != "" {
		return coded.Detail, true
	}
	return "", false
}
