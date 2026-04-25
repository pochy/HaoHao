package api

import (
	"context"

	"example.com/haohao/backend/internal/platform"
	"example.com/haohao/backend/internal/service"
)

func auditRequest(ctx context.Context) service.AuditRequest {
	metadata := platform.RequestMetadataFromContext(ctx)
	return service.AuditRequest{
		RequestID: metadata.RequestID,
		ClientIP:  metadata.ClientIP,
		UserAgent: metadata.UserAgent,
	}
}

func userAuditContext(ctx context.Context, userID int64, tenantID *int64) service.AuditContext {
	return service.UserAuditContext(userID, tenantID, auditRequest(ctx))
}
