package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"example.com/haohao/backend/internal/platform"
	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestInternalHTTPErrorLogsApplicationError(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	ctx := platform.ContextWithRequestMetadata(context.Background(), platform.RequestMetadata{RequestID: "req-app"})
	root := &pgconn.PgError{
		Code:           "42703",
		Severity:       "ERROR",
		Message:        "column dataset_id does not exist",
		TableName:      "dataset_query_jobs",
		ColumnName:     "dataset_id",
		ConstraintName: "dataset_query_jobs_dataset_id_fkey",
	}

	err := internalHTTPError(ctx, Dependencies{Logger: logger}, "listDatasetScopedQueryJobs", fmt.Errorf("list dataset query jobs: %w", root))

	var statusErr huma.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error does not implement huma.StatusError: %T", err)
	}
	if statusErr.GetStatus() != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", statusErr.GetStatus(), http.StatusInternalServerError)
	}
	if strings.Contains(err.Error(), "dataset_id") {
		t.Fatalf("client error leaked internal detail: %q", err.Error())
	}

	logLine := output.String()
	for _, want := range []string{
		`"msg":"application error"`,
		`"log_type":"application_error"`,
		`"request_id":"req-app"`,
		`"operation":"listDatasetScopedQueryJobs"`,
		`"error_type":"*pgconn.PgError"`,
		`"sqlstate":"42703"`,
		`"severity":"ERROR"`,
		`"table":"dataset_query_jobs"`,
		`"column":"dataset_id"`,
		`"constraint":"dataset_query_jobs_dataset_id_fkey"`,
	} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log line %q does not contain %q", logLine, want)
		}
	}
}

func TestToHTTPErrorWithLogDoesNotLogMappedDomainError(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	ctx := platform.ContextWithRequestMetadata(context.Background(), platform.RequestMetadata{RequestID: "req-auth"})

	err := toHTTPErrorWithLog(ctx, Dependencies{Logger: logger}, "getSession", service.ErrUnauthorized)

	var statusErr huma.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error does not implement huma.StatusError: %T", err)
	}
	if statusErr.GetStatus() != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", statusErr.GetStatus(), http.StatusUnauthorized)
	}
	if output.Len() != 0 {
		t.Fatalf("mapped domain error should not create application_error log, got %q", output.String())
	}
}
