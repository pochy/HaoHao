package jobs

import (
	"context"
	"fmt"

	db "example.com/haohao/backend/internal/db"
	"example.com/haohao/backend/internal/service"
)

type ProvisioningReconcileJob struct {
	queries           *db.Queries
	sessionService    *service.SessionService
	delegationService *service.DelegationService
}

func NewProvisioningReconcileJob(queries *db.Queries, sessionService *service.SessionService, delegationService *service.DelegationService) *ProvisioningReconcileJob {
	return &ProvisioningReconcileJob{
		queries:           queries,
		sessionService:    sessionService,
		delegationService: delegationService,
	}
}

func (j *ProvisioningReconcileJob) RunOnce(ctx context.Context) error {
	if j == nil || j.queries == nil {
		return nil
	}

	users, err := j.queries.ListDeactivatedUsersWithActiveGrants(ctx)
	if err != nil {
		return fmt.Errorf("list deactivated users with active grants: %w", err)
	}

	var failed int32
	for _, user := range users {
		if j.sessionService != nil {
			if err := j.sessionService.DeleteUserSessions(ctx, user.ID); err != nil {
				failed++
				continue
			}
		}
		if j.delegationService != nil {
			if err := j.delegationService.DeleteAllGrantsForUser(ctx, user.ID); err != nil {
				failed++
				continue
			}
		}
		if err := j.queries.DeleteOAuthUserGrantsByUserID(ctx, user.ID); err != nil {
			failed++
		}
	}

	if err := j.queries.UpsertProvisioningSyncState(ctx, db.UpsertProvisioningSyncStateParams{
		Source:      "scim",
		FailedCount: failed,
	}); err != nil {
		return fmt.Errorf("update provisioning reconcile state: %w", err)
	}

	return nil
}
