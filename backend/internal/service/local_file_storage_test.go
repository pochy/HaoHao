package service

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestLocalFileStorageDeleteIsIdempotent(t *testing.T) {
	ctx := context.Background()
	storage := NewLocalFileStorage(t.TempDir())

	stored, err := storage.Save(ctx, "tenants/1/files/test.txt", strings.NewReader("hello"), 1024)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	path, err := storage.pathForKey(stored.Key)
	if err != nil {
		t.Fatalf("pathForKey() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stored file stat error = %v", err)
	}

	if err := storage.Delete(ctx, stored.Key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stored file still exists after delete: %v", err)
	}
	if err := storage.Delete(ctx, stored.Key); err != nil {
		t.Fatalf("second Delete() error = %v", err)
	}
}
