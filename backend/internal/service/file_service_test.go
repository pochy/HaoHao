package service

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeUploadInputRejectsDrivePurpose(t *testing.T) {
	svc := NewFileService(nil, nil, nil, nil, nil, 0, nil, nil)

	_, err := svc.normalizeUploadInput(FileUploadInput{
		TenantID:         1,
		UserID:           1,
		Purpose:          "drive",
		OriginalFilename: "document.txt",
		ContentType:      "text/plain",
		Body:             strings.NewReader("body"),
	})
	if !errors.Is(err, ErrInvalidFileInput) {
		t.Fatalf("normalizeUploadInput() error = %v, want ErrInvalidFileInput", err)
	}
}
