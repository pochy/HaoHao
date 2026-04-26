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
