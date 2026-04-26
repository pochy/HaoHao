package service

import (
	"context"
	"io"
	"time"
)

type StoredFile struct {
	Key       string
	Size      int64
	SHA256Hex string
	ETag      string
}

type FileReadCloser struct {
	io.ReadCloser
	Size int64
}

type ObjectPutOptions struct {
	ContentType string
	Metadata    map[string]string
}

type ObjectHead struct {
	Key         string
	Size        int64
	ContentType string
	ETag        string
	UpdatedAt   time.Time
}

type SignedObjectURL struct {
	URL       string
	ExpiresAt time.Time
}

type FileStorage interface {
	Save(context.Context, string, io.Reader, int64) (StoredFile, error)
	Open(context.Context, string) (FileReadCloser, error)
	Delete(context.Context, string) error
	PutObject(context.Context, string, io.Reader, int64, ObjectPutOptions) (StoredFile, error)
	GetObject(context.Context, string) (FileReadCloser, error)
	DeleteObject(context.Context, string) error
	CreateDownloadURL(context.Context, string, time.Duration) (SignedObjectURL, error)
	CreateUploadURL(context.Context, string, time.Duration, string) (SignedObjectURL, error)
	HeadObject(context.Context, string) (ObjectHead, error)
}
