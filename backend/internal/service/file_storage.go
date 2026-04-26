package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	FileStorageDriverLocal     = "local"
	FileStorageDriverSeaweedS3 = "seaweedfs_s3"
)

type StoredFile struct {
	Driver    string
	Bucket    string
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

type FileStorageConfig struct {
	Driver            string
	LocalDir          string
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3ForcePathStyle  bool
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

func NewFileStorage(ctx context.Context, cfg FileStorageConfig) (FileStorage, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" {
		driver = FileStorageDriverLocal
	}
	switch driver {
	case FileStorageDriverLocal:
		return NewLocalFileStorage(cfg.LocalDir), nil
	case FileStorageDriverSeaweedS3:
		return NewS3FileStorage(ctx, S3FileStorageConfig{
			Driver:          FileStorageDriverSeaweedS3,
			Endpoint:        cfg.S3Endpoint,
			Region:          cfg.S3Region,
			Bucket:          cfg.S3Bucket,
			AccessKeyID:     cfg.S3AccessKeyID,
			SecretAccessKey: cfg.S3SecretAccessKey,
			ForcePathStyle:  cfg.S3ForcePathStyle,
		})
	default:
		return nil, fmt.Errorf("unsupported file storage driver %q", cfg.Driver)
	}
}

func FileStorageDriverName(storage FileStorage) string {
	type driverNamer interface {
		StorageDriver() string
	}
	if storage != nil {
		if named, ok := storage.(driverNamer); ok {
			if driver := strings.TrimSpace(named.StorageDriver()); driver != "" {
				return driver
			}
		}
	}
	return FileStorageDriverLocal
}

func storageDriverForStoredFile(storage FileStorage, stored StoredFile) string {
	if driver := strings.TrimSpace(stored.Driver); driver != "" {
		return driver
	}
	return FileStorageDriverName(storage)
}
