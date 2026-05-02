package api

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"

	"example.com/haohao/backend/internal/platform"
	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/trace"
)

func toHTTPErrorWithLog(ctx context.Context, deps Dependencies, operation string, err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		return huma.Error401Unauthorized("invalid credentials")
	case errors.Is(err, service.ErrUnauthorized):
		return huma.Error401Unauthorized("missing or expired session")
	case errors.Is(err, service.ErrInvalidCSRFToken):
		return huma.Error403Forbidden("invalid csrf token")
	case errors.Is(err, service.ErrAuthModeUnsupported):
		return huma.Error501NotImplemented("password login is disabled for the current auth mode")
	default:
		return internalHTTPError(ctx, deps, operation, err)
	}
}

func internalHTTPError(ctx context.Context, deps Dependencies, operation string, err error) error {
	logApplicationError(ctx, deps.Logger, operation, err)
	return huma.Error500InternalServerError("internal server error")
}

func logApplicationError(ctx context.Context, logger *slog.Logger, operation string, err error) {
	if logger == nil {
		logger = slog.Default()
	}
	if ctx == nil {
		ctx = context.Background()
	}

	metadata := platform.RequestMetadataFromContext(ctx)
	attrs := []any{
		"log_type", "application_error",
		"request_id", metadata.RequestID,
		"operation", normalizedOperation(operation),
		"error", safeErrorString(err),
		"error_type", errorType(err),
	}
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		attrs = append(attrs,
			"trace_id", spanContext.TraceID().String(),
			"span_id", spanContext.SpanID().String(),
		)
	}
	if pgAttrs := postgresErrorAttrs(err); len(pgAttrs) > 0 {
		attrs = append(attrs, pgAttrs...)
	}

	logger.ErrorContext(ctx, "application error", attrs...)
}

func normalizedOperation(operation string) string {
	operation = strings.TrimSpace(operation)
	if operation == "" {
		return "unknown"
	}
	return operation
}

func safeErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func errorType(err error) string {
	if err == nil {
		return "<nil>"
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil {
		return reflect.TypeOf(pgErr).String()
	}
	for {
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		next := unwrapper.Unwrap()
		if next == nil {
			break
		}
		err = next
	}
	errType := reflect.TypeOf(err)
	if errType == nil {
		return "<nil>"
	}
	return errType.String()
}

func postgresErrorAttrs(err error) []any {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr == nil {
		return nil
	}

	attrs := []any{
		"sqlstate", pgErr.Code,
		"severity", pgErr.Severity,
	}
	if pgErr.TableName != "" {
		attrs = append(attrs, "table", pgErr.TableName)
	}
	if pgErr.ColumnName != "" {
		attrs = append(attrs, "column", pgErr.ColumnName)
	}
	if pgErr.ConstraintName != "" {
		attrs = append(attrs, "constraint", pgErr.ConstraintName)
	}
	return attrs
}
