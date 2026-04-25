package service

import (
	"context"
	"io"
)

type StoredFile struct {
	Key       string
	Size      int64
	SHA256Hex string
}

type FileReadCloser struct {
	io.ReadCloser
	Size int64
}

type FileStorage interface {
	Save(context.Context, string, io.Reader, int64) (StoredFile, error)
	Open(context.Context, string) (FileReadCloser, error)
	Delete(context.Context, string) error
}
