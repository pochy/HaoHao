package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
)

const dataAccessAuthorizationUnavailableMessage = "data access authorization is not ready: register the latest OpenFGA model and update OPENFGA_AUTHORIZATION_MODEL_ID"

func dataAccessAuthorizationUnavailableHTTPError(ctx context.Context, deps Dependencies, operation string, err error) error {
	logApplicationError(ctx, deps.Logger, operation, err)
	return huma.Error503ServiceUnavailable(dataAccessAuthorizationUnavailableMessage)
}
