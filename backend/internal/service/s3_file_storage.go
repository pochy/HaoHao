package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3FileStorageConfig struct {
	Driver          string
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
}

type S3FileStorage struct {
	client  *s3.Client
	presign *s3.PresignClient
	driver  string
	bucket  string
}

func NewS3FileStorage(ctx context.Context, cfg S3FileStorageConfig) (*S3FileStorage, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" {
		driver = FileStorageDriverSeaweedS3
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, fmt.Errorf("FILE_S3_BUCKET is required")
	}
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")

	loadOptions := []func(*awscfg.LoadOptions) error{
		awscfg.WithRegion(region),
	}
	if strings.TrimSpace(cfg.AccessKeyID) != "" || strings.TrimSpace(cfg.SecretAccessKey) != "" {
		loadOptions = append(loadOptions, awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(cfg.AccessKeyID),
			strings.TrimSpace(cfg.SecretAccessKey),
			"",
		)))
	}
	awsConfig, err := awscfg.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load s3 config: %w", err)
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		if endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
		options.UsePathStyle = cfg.ForcePathStyle
	})

	return &S3FileStorage{
		client:  client,
		presign: s3.NewPresignClient(client),
		driver:  driver,
		bucket:  bucket,
	}, nil
}

func (s *S3FileStorage) StorageDriver() string {
	if s == nil || strings.TrimSpace(s.driver) == "" {
		return FileStorageDriverSeaweedS3
	}
	return s.driver
}

func (s *S3FileStorage) Save(ctx context.Context, key string, reader io.Reader, maxBytes int64) (StoredFile, error) {
	return s.PutObject(ctx, key, reader, maxBytes, ObjectPutOptions{})
}

func (s *S3FileStorage) PutObject(ctx context.Context, key string, reader io.Reader, maxBytes int64, options ObjectPutOptions) (StoredFile, error) {
	if s == nil || s.client == nil {
		return StoredFile{}, fmt.Errorf("s3 file storage is not configured")
	}
	if reader == nil {
		return StoredFile{}, fmt.Errorf("file body is required")
	}
	objectKey, err := normalizeS3ObjectKey(key)
	if err != nil {
		return StoredFile{}, err
	}

	temp, err := os.CreateTemp("", "haohao-s3-upload-*")
	if err != nil {
		return StoredFile{}, fmt.Errorf("create s3 upload temp file: %w", err)
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
	if err != nil {
		_ = temp.Close()
		return StoredFile{}, fmt.Errorf("write s3 upload temp file: %w", err)
	}
	if maxBytes > 0 && written > maxBytes {
		_ = temp.Close()
		return StoredFile{}, ErrFileTooLarge
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		_ = temp.Close()
		return StoredFile{}, fmt.Errorf("rewind s3 upload temp file: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(objectKey),
		Body:          temp,
		ContentLength: aws.Int64(written),
	}
	if contentType := strings.TrimSpace(options.ContentType); contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	if len(options.Metadata) > 0 {
		input.Metadata = options.Metadata
	}
	output, err := s.client.PutObject(ctx, input)
	closeErr := temp.Close()
	if err != nil {
		return StoredFile{}, fmt.Errorf("put s3 object: %w", err)
	}
	if closeErr != nil {
		return StoredFile{}, fmt.Errorf("close s3 upload temp file: %w", closeErr)
	}

	return StoredFile{
		Driver:    s.StorageDriver(),
		Bucket:    s.bucket,
		Key:       objectKey,
		Size:      written,
		SHA256Hex: hex.EncodeToString(hasher.Sum(nil)),
		ETag:      cleanS3ETag(aws.ToString(output.ETag)),
	}, nil
}

func (s *S3FileStorage) Open(ctx context.Context, key string) (FileReadCloser, error) {
	return s.GetObject(ctx, key)
}

func (s *S3FileStorage) GetObject(ctx context.Context, key string) (FileReadCloser, error) {
	if s == nil || s.client == nil {
		return FileReadCloser{}, fmt.Errorf("s3 file storage is not configured")
	}
	objectKey, err := normalizeS3ObjectKey(key)
	if err != nil {
		return FileReadCloser{}, err
	}
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return FileReadCloser{}, fmt.Errorf("get s3 object: %w", err)
	}
	return FileReadCloser{
		ReadCloser: output.Body,
		Size:       aws.ToInt64(output.ContentLength),
	}, nil
}

func (s *S3FileStorage) Delete(ctx context.Context, key string) error {
	return s.DeleteObject(ctx, key)
}

func (s *S3FileStorage) DeleteObject(ctx context.Context, key string) error {
	if s == nil || s.client == nil {
		return nil
	}
	objectKey, err := normalizeS3ObjectKey(key)
	if err != nil {
		return err
	}
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	}); err != nil {
		return fmt.Errorf("delete s3 object: %w", err)
	}
	return nil
}

func (s *S3FileStorage) CreateDownloadURL(ctx context.Context, key string, ttl time.Duration) (SignedObjectURL, error) {
	if s == nil || s.presign == nil {
		return SignedObjectURL{}, fmt.Errorf("s3 file storage is not configured")
	}
	objectKey, err := normalizeS3ObjectKey(key)
	if err != nil {
		return SignedObjectURL{}, err
	}
	ttl = normalizeSignedURLTTL(ttl)
	output, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return SignedObjectURL{}, fmt.Errorf("presign s3 download URL: %w", err)
	}
	return SignedObjectURL{URL: output.URL, ExpiresAt: time.Now().Add(ttl)}, nil
}

func (s *S3FileStorage) CreateUploadURL(ctx context.Context, key string, ttl time.Duration, contentType string) (SignedObjectURL, error) {
	if s == nil || s.presign == nil {
		return SignedObjectURL{}, fmt.Errorf("s3 file storage is not configured")
	}
	objectKey, err := normalizeS3ObjectKey(key)
	if err != nil {
		return SignedObjectURL{}, err
	}
	ttl = normalizeSignedURLTTL(ttl)
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	}
	if strings.TrimSpace(contentType) != "" {
		input.ContentType = aws.String(strings.TrimSpace(contentType))
	}
	output, err := s.presign.PresignPutObject(ctx, input, s3.WithPresignExpires(ttl))
	if err != nil {
		return SignedObjectURL{}, fmt.Errorf("presign s3 upload URL: %w", err)
	}
	return SignedObjectURL{URL: output.URL, ExpiresAt: time.Now().Add(ttl)}, nil
}

func (s *S3FileStorage) HeadObject(ctx context.Context, key string) (ObjectHead, error) {
	if s == nil || s.client == nil {
		return ObjectHead{}, fmt.Errorf("s3 file storage is not configured")
	}
	objectKey, err := normalizeS3ObjectKey(key)
	if err != nil {
		return ObjectHead{}, err
	}
	output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return ObjectHead{}, fmt.Errorf("head s3 object: %w", err)
	}
	updatedAt := time.Time{}
	if output.LastModified != nil {
		updatedAt = *output.LastModified
	}
	return ObjectHead{
		Key:         objectKey,
		Size:        aws.ToInt64(output.ContentLength),
		ContentType: aws.ToString(output.ContentType),
		ETag:        cleanS3ETag(aws.ToString(output.ETag)),
		UpdatedAt:   updatedAt,
	}, nil
}

func normalizeS3ObjectKey(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		return "", fmt.Errorf("invalid storage key")
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != trimmed {
		return "", fmt.Errorf("invalid storage key")
	}
	return cleaned, nil
}

func normalizeSignedURLTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return 15 * time.Minute
	}
	if ttl > 24*time.Hour {
		return 24 * time.Hour
	}
	return ttl
}

func cleanS3ETag(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"`)
}
