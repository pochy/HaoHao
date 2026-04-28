package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/haohao/backend/internal/platform"
	"example.com/haohao/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func TestDriveProblemFromCodedInvalidInput(t *testing.T) {
	ctx := platform.ContextWithRequestMetadata(context.Background(), platform.RequestMetadata{RequestID: "req-123"})
	err := service.NewDriveCodedError(service.ErrDriveInvalidInput, service.DriveErrorFilenameRequired, "Filename is required.")

	problem := driveProblemFromError(ctx, err)

	if problem.Status != http.StatusBadRequest {
		t.Fatalf("Status = %d, want %d", problem.Status, http.StatusBadRequest)
	}
	if problem.Detail != "Filename is required." {
		t.Fatalf("Detail = %q", problem.Detail)
	}
	if problem.Type != "urn:haohao:error:drive.filename_required" {
		t.Fatalf("Type = %q", problem.Type)
	}
	if problem.Instance != "urn:haohao:request:req-123" {
		t.Fatalf("Instance = %q", problem.Instance)
	}
}

func TestDriveProblemFromFileTooLarge(t *testing.T) {
	err := service.NewDriveCodedError(service.ErrInvalidFileInput, service.DriveErrorFileTooLarge, "File exceeds the Drive upload limit of 10 MB.")

	problem := driveProblemFromError(context.Background(), err)

	if problem.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("Status = %d, want %d", problem.Status, http.StatusRequestEntityTooLarge)
	}
	if problem.Type != "urn:haohao:error:drive.file_too_large" {
		t.Fatalf("Type = %q", problem.Type)
	}
	if problem.Detail != "File exceeds the Drive upload limit of 10 MB." {
		t.Fatalf("Detail = %q", problem.Detail)
	}
}

func TestDriveProblemFromInternalErrorDoesNotLeakDetail(t *testing.T) {
	ctx := platform.ContextWithRequestMetadata(context.Background(), platform.RequestMetadata{RequestID: "req-500"})

	problem := driveProblemFromError(ctx, errors.New("database password leaked"))

	if problem.Status != http.StatusInternalServerError {
		t.Fatalf("Status = %d, want %d", problem.Status, http.StatusInternalServerError)
	}
	if strings.Contains(problem.Detail, "database password leaked") {
		t.Fatalf("Detail leaked internal error: %q", problem.Detail)
	}
	if problem.Instance != "urn:haohao:request:req-500" {
		t.Fatalf("Instance = %q", problem.Instance)
	}
}

func TestWriteRawDriveErrorWritesProblemAndLogFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx := platform.ContextWithRequestMetadata(context.Background(), platform.RequestMetadata{RequestID: "req-raw"})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/drive/files", nil).WithContext(ctx)
	err := service.NewDriveCodedError(service.ErrInvalidFileInput, service.DriveErrorFileTooLarge, "File exceeds the Drive upload limit of 10 MB.")

	writeRawDriveError(c, err)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
	var body driveProblem
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Type != "urn:haohao:error:drive.file_too_large" {
		t.Fatalf("body.Type = %q", body.Type)
	}
	if body.Instance != "urn:haohao:request:req-raw" {
		t.Fatalf("body.Instance = %q", body.Instance)
	}
	if value, _ := c.Get("error_type"); value != body.Type {
		t.Fatalf("error_type = %#v, want %q", value, body.Type)
	}
	if value, _ := c.Get("error_detail"); value != body.Detail {
		t.Fatalf("error_detail = %#v, want %q", value, body.Detail)
	}
}
