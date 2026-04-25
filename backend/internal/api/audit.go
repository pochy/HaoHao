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

func sessionAuditContext(ctx context.Context, current service.CurrentSession, tenantID *int64) service.AuditContext {
	actorUserID := current.User.ID
	auditCtx := service.UserAuditContext(actorUserID, tenantID, auditRequest(ctx))
	if current.ActorUser != nil && current.SupportAccess != nil {
		auditCtx.ActorUserID = &current.ActorUser.ID
		auditCtx.SupportAccessID = &current.SupportAccess.ID
		auditCtx.ImpersonatedUserID = &current.User.ID
	}
	return auditCtx
}
