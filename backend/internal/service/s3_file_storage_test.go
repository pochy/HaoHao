package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestS3FileStorageObjectLifecycle(t *testing.T) {
	var (
		mu      sync.Mutex
		body    []byte
		deleted bool
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/haohao-drive-dev/tenants/1/files/test.txt" {
			t.Errorf("unexpected request path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read put body: %v", err)
				http.Error(w, "read body", http.StatusInternalServerError)
				return
			}
			mu.Lock()
			body = data
			deleted = false
			mu.Unlock()
			w.Header().Set("ETag", `"test-etag"`)
			w.WriteHeader(http.StatusOK)
		case http.MethodHead:
			mu.Lock()
			exists := body != nil && !deleted
			size := len(body)
			mu.Unlock()
			if !exists {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(size))
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("ETag", `"test-etag"`)
			w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			mu.Lock()
			data := append([]byte(nil), body...)
			exists := body != nil && !deleted
			mu.Unlock()
			if !exists {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("ETag", `"test-etag"`)
			_, _ = w.Write(data)
		case http.MethodDelete:
			mu.Lock()
			deleted = true
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	storage, err := NewS3FileStorage(context.Background(), S3FileStorageConfig{
		Endpoint:        server.URL,
		Region:          "us-east-1",
		Bucket:          "haohao-drive-dev",
		AccessKeyID:     "test",
		SecretAccessKey: "test",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("NewS3FileStorage() error = %v", err)
	}

	stored, err := storage.PutObject(context.Background(), "tenants/1/files/test.txt", strings.NewReader("hello"), 1024, ObjectPutOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}
	if stored.Driver != FileStorageDriverSeaweedS3 || stored.Bucket != "haohao-drive-dev" || stored.Key != "tenants/1/files/test.txt" {
		t.Fatalf("stored metadata = %+v", stored)
	}
	if stored.Size != 5 || stored.SHA256Hex == "" || stored.ETag != "test-etag" {
		t.Fatalf("stored object = %+v", stored)
	}

	head, err := storage.HeadObject(context.Background(), stored.Key)
	if err != nil {
		t.Fatalf("HeadObject() error = %v", err)
	}
	if head.Size != 5 || head.ContentType != "text/plain" || head.ETag != "test-etag" {
		t.Fatalf("HeadObject() = %+v", head)
	}

	got, err := storage.GetObject(context.Background(), stored.Key)
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	data, err := io.ReadAll(got)
	if closeErr := got.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("read object body: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("GetObject() body = %q", data)
	}

	if err := storage.DeleteObject(context.Background(), stored.Key); err != nil {
		t.Fatalf("DeleteObject() error = %v", err)
	}
}

func TestS3FileStorageRejectsOversizedObjectBeforeUpload(t *testing.T) {
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	storage, err := NewS3FileStorage(context.Background(), S3FileStorageConfig{
		Endpoint:        server.URL,
		Region:          "us-east-1",
		Bucket:          "haohao-drive-dev",
		AccessKeyID:     "test",
		SecretAccessKey: "test",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("NewS3FileStorage() error = %v", err)
	}
	_, err = storage.PutObject(context.Background(), "tenants/1/files/too-large.txt", strings.NewReader("hello"), 4, ObjectPutOptions{})
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("PutObject() error = %v, want ErrFileTooLarge", err)
	}
	if requested {
		t.Fatalf("S3 endpoint was called for oversized object")
	}
}

func TestS3FileStoragePresignedURLs(t *testing.T) {
	storage, err := NewS3FileStorage(context.Background(), S3FileStorageConfig{
		Endpoint:        "http://127.0.0.1:8333",
		Region:          "us-east-1",
		Bucket:          "haohao-drive-dev",
		AccessKeyID:     "test",
		SecretAccessKey: "test",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("NewS3FileStorage() error = %v", err)
	}

	download, err := storage.CreateDownloadURL(context.Background(), "tenants/1/files/test.txt", time.Minute)
	if err != nil {
		t.Fatalf("CreateDownloadURL() error = %v", err)
	}
	assertPresignedURL(t, download.URL)

	upload, err := storage.CreateUploadURL(context.Background(), "tenants/1/files/test.txt", time.Minute, "text/plain")
	if err != nil {
		t.Fatalf("CreateUploadURL() error = %v", err)
	}
	assertPresignedURL(t, upload.URL)
}

func assertPresignedURL(t *testing.T, rawURL string) {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse presigned URL: %v", err)
	}
	if parsed.Query().Get("X-Amz-Signature") == "" {
		t.Fatalf("missing X-Amz-Signature in %s", rawURL)
	}
	if !strings.Contains(parsed.Path, "/haohao-drive-dev/tenants/1/files/test.txt") {
		t.Fatalf("unexpected presigned path: %s", parsed.Path)
	}
}
