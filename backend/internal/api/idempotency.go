package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

func beginIdempotency(ctx context.Context, deps Dependencies, key, method, path string, userID int64, tenantID *int64, body any) (service.IdempotencyAttempt, error) {
	if deps.IdempotencyService == nil || key == "" {
		return service.IdempotencyAttempt{}, nil
	}
	scope := fmt.Sprintf("%s:%s:user:%d", method, path, userID)
	if tenantID != nil {
		scope = fmt.Sprintf("%s:tenant:%d", scope, *tenantID)
	}
	return deps.IdempotencyService.Begin(ctx, service.IdempotencyInput{
		Key:         key,
		Method:      method,
		Path:        path,
		Scope:       scope,
		TenantID:    tenantID,
		ActorUserID: &userID,
		RequestBody: body,
	})
}

func completeIdempotency(ctx context.Context, deps Dependencies, attempt service.IdempotencyAttempt, status int, body any) error {
	if deps.IdempotencyService == nil {
		return nil
	}
	return deps.IdempotencyService.Complete(ctx, attempt, status, body)
}

func replayIdempotencyBody[T any](attempt service.IdempotencyAttempt) (T, error) {
	var body T
	if err := json.Unmarshal(attempt.Body, &body); err != nil {
		return body, huma.Error500InternalServerError("stored idempotency response is invalid")
	}
	return body, nil
}

func toIdempotencyHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrIdempotencyConflict):
		return huma.Error409Conflict("idempotency key was used with a different request")
	case errors.Is(err, service.ErrIdempotencyInProgress):
		return huma.Error409Conflict("idempotency key is already processing")
	case errors.Is(err, service.ErrIdempotencyFailed):
		return huma.Error409Conflict("idempotency key is associated with a failed request")
	default:
		return huma.Error500InternalServerError("internal server error")
	}
}

func idempotencyStatus(attempt service.IdempotencyAttempt, fallback int) int {
	if attempt.StatusCode != 0 {
		return attempt.StatusCode
	}
	if fallback == 0 {
		return http.StatusOK
	}
	return fallback
}
