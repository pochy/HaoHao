package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrFileTooLarge = errors.New("file is too large")
var ErrSignedURLUnsupported = errors.New("signed object URL is not supported by this storage driver")

type LocalFileStorage struct {
	root string
}

func NewLocalFileStorage(root string) *LocalFileStorage {
	root = strings.TrimSpace(root)
	if root == "" {
		root = ".data/files"
	}
	return &LocalFileStorage{root: root}
}

func (s *LocalFileStorage) Save(ctx context.Context, key string, reader io.Reader, maxBytes int64) (StoredFile, error) {
	return s.PutObject(ctx, key, reader, maxBytes, ObjectPutOptions{})
}

func (s *LocalFileStorage) PutObject(ctx context.Context, key string, reader io.Reader, maxBytes int64, _ ObjectPutOptions) (StoredFile, error) {
	if s == nil {
		return StoredFile{}, fmt.Errorf("file storage is not configured")
	}
	finalPath, err := s.pathForKey(key)
	if err != nil {
		return StoredFile{}, err
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o750); err != nil {
		return StoredFile{}, fmt.Errorf("create file storage directory: %w", err)
	}

	temp, err := os.CreateTemp(filepath.Dir(finalPath), ".upload-*")
	if err != nil {
		return StoredFile{}, fmt.Errorf("create temp file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	hasher := sha256.New()
	limit := maxBytes + 1
	if limit <= 1 {
		limit = 10*1024*1024 + 1
	}
	written, err := io.Copy(io.MultiWriter(temp, hasher), io.LimitReader(reader, limit))
	if closeErr := temp.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return StoredFile{}, fmt.Errorf("write file body: %w", err)
	}
	if maxBytes > 0 && written > maxBytes {
		return StoredFile{}, ErrFileTooLarge
	}

	select {
	case <-ctx.Done():
		return StoredFile{}, ctx.Err()
	default:
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		return StoredFile{}, fmt.Errorf("publish file body: %w", err)
	}

	return StoredFile{
		Driver:    FileStorageDriverLocal,
		Key:       key,
		Size:      written,
		SHA256Hex: hex.EncodeToString(hasher.Sum(nil)),
		ETag:      hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func (s *LocalFileStorage) StorageDriver() string {
	return FileStorageDriverLocal
}

func (s *LocalFileStorage) Open(ctx context.Context, key string) (FileReadCloser, error) {
	return s.GetObject(ctx, key)
}

func (s *LocalFileStorage) GetObject(ctx context.Context, key string) (FileReadCloser, error) {
	if s == nil {
		return FileReadCloser{}, fmt.Errorf("file storage is not configured")
	}
	path, err := s.pathForKey(key)
	if err != nil {
		return FileReadCloser{}, err
	}
	select {
	case <-ctx.Done():
		return FileReadCloser{}, ctx.Err()
	default:
	}
	file, err := os.Open(path)
	if err != nil {
		return FileReadCloser{}, fmt.Errorf("open file body: %w", err)
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return FileReadCloser{}, fmt.Errorf("stat file body: %w", err)
	}
	return FileReadCloser{ReadCloser: file, Size: stat.Size()}, nil
}

func (s *LocalFileStorage) Delete(ctx context.Context, key string) error {
	return s.DeleteObject(ctx, key)
}

func (s *LocalFileStorage) DeleteObject(ctx context.Context, key string) error {
	if s == nil {
		return nil
	}
	path, err := s.pathForKey(key)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete file body: %w", err)
	}
	return nil
}

func (s *LocalFileStorage) CreateDownloadURL(context.Context, string, time.Duration) (SignedObjectURL, error) {
	return SignedObjectURL{}, ErrSignedURLUnsupported
}

func (s *LocalFileStorage) CreateUploadURL(context.Context, string, time.Duration, string) (SignedObjectURL, error) {
	return SignedObjectURL{}, ErrSignedURLUnsupported
}

func (s *LocalFileStorage) HeadObject(ctx context.Context, key string) (ObjectHead, error) {
	if s == nil {
		return ObjectHead{}, fmt.Errorf("file storage is not configured")
	}
	path, err := s.pathForKey(key)
	if err != nil {
		return ObjectHead{}, err
	}
	select {
	case <-ctx.Done():
		return ObjectHead{}, ctx.Err()
	default:
	}
	stat, err := os.Stat(path)
	if err != nil {
		return ObjectHead{}, fmt.Errorf("head file body: %w", err)
	}
	return ObjectHead{
		Key:       key,
		Size:      stat.Size(),
		UpdatedAt: stat.ModTime(),
	}, nil
}

func (s *LocalFileStorage) pathForKey(key string) (string, error) {
	cleanKey := filepath.Clean(strings.TrimSpace(key))
	cleanKey = strings.TrimPrefix(cleanKey, string(filepath.Separator))
	if cleanKey == "." || strings.HasPrefix(cleanKey, ".."+string(filepath.Separator)) || cleanKey == ".." {
		return "", fmt.Errorf("invalid storage key")
	}
	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("resolve storage root: %w", err)
	}
	path := filepath.Join(root, cleanKey)
	if !strings.HasPrefix(path, root+string(filepath.Separator)) && path != root {
		return "", fmt.Errorf("invalid storage key")
	}
	return path, nil
}
