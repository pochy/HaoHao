package api

import (
	"context"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type TenantAdminDriveShareStateBody struct {
	PublicID         string    `json:"publicId"`
	ResourceType     string    `json:"resourceType"`
	ResourcePublicID string    `json:"resourcePublicId"`
	ResourceName     string    `json:"resourceName"`
	SubjectType      string    `json:"subjectType"`
	SubjectPublicID  string    `json:"subjectPublicId"`
	Role             string    `json:"role"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt        time.Time `json:"updatedAt" format:"date-time"`
}

type TenantAdminDriveShareLinkStateBody struct {
	PublicID         string    `json:"publicId"`
	ResourceType     string    `json:"resourceType"`
	ResourcePublicID string    `json:"resourcePublicId"`
	ResourceName     string    `json:"resourceName"`
	CanDownload      bool      `json:"canDownload"`
	PasswordRequired bool      `json:"passwordRequired"`
	Status           string    `json:"status"`
	ExpiresAt        time.Time `json:"expiresAt" format:"date-time"`
	CreatedAt        time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt        time.Time `json:"updatedAt" format:"date-time"`
}

type TenantAdminDriveAuditEventBody struct {
	PublicID   string         `json:"publicId"`
	ActorType  string         `json:"actorType"`
	Action     string         `json:"action"`
	TargetType string         `json:"targetType"`
	TargetID   string         `json:"targetId"`
	Metadata   map[string]any `json:"metadata"`
	OccurredAt time.Time      `json:"occurredAt" format:"date-time"`
}

type TenantAdminDriveSyncItemBody struct {
	Kind     string `json:"kind"`
	PublicID string `json:"publicId"`
	Status   string `json:"status"`
	Action   string `json:"action"`
	Error    string `json:"error,omitempty"`
}

type TenantAdminDriveOperationsHealthBody struct {
	TenantID                int64     `json:"tenantId"`
	WorkspaceCount          int64     `json:"workspaceCount"`
	MissingWorkspaceCount   int64     `json:"missingWorkspaceCount"`
	OpenFGADriftCount       int       `json:"openfgaDriftCount"`
	StorageMissingCount     int       `json:"storageMissingCount"`
	StorageOrphanCheckState string    `json:"storageOrphanCheckState"`
	CheckedAt               time.Time `json:"checkedAt" format:"date-time"`
}

type TenantAdminDriveAdminContentSessionBody struct {
	PublicID       string    `json:"publicId"`
	ReasonCategory string    `json:"reasonCategory"`
	ExpiresAt      time.Time `json:"expiresAt" format:"date-time"`
	CreatedAt      time.Time `json:"createdAt" format:"date-time"`
}

type TenantAdminDriveAdminContentSessionInputBody struct {
	Reason         string `json:"reason" maxLength:"2000"`
	ReasonCategory string `json:"reasonCategory,omitempty" enum:"manual,incident,legal,security"`
}

type TenantAdminDriveOperationsHealthOutput struct {
	Body TenantAdminDriveOperationsHealthBody
}

type TenantAdminDriveAdminContentSessionOutput struct {
	Body TenantAdminDriveAdminContentSessionBody
}

type TenantAdminDriveSyncOutput struct {
	Body struct {
		DryRun bool                           `json:"dryRun"`
		Items  []TenantAdminDriveSyncItemBody `json:"items"`
	}
}

type TenantAdminDriveSharesOutput struct {
	Body struct {
		Items []TenantAdminDriveShareStateBody `json:"items"`
	}
}

type TenantAdminDriveShareLinksOutput struct {
	Body struct {
		Items []TenantAdminDriveShareLinkStateBody `json:"items"`
	}
}

type TenantAdminDriveInvitationsOutput struct {
	Body struct {
		Items []DriveShareInvitationBody `json:"items"`
	}
}

type TenantAdminDriveAuditOutput struct {
	Body struct {
		Items []TenantAdminDriveAuditEventBody `json:"items"`
	}
}

type TenantAdminDriveBySlugInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Limit         int32       `query:"limit" default:"100"`
}

type TenantAdminDriveMutationInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
}

type TenantAdminDriveApprovalInput struct {
	SessionCookie      http.Cookie `cookie:"SESSION_ID"`
	CSRFToken          string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug         string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	InvitationPublicID string      `path:"invitationPublicId" format:"uuid"`
}

type TenantAdminDriveAdminContentSessionInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	Body          TenantAdminDriveAdminContentSessionInputBody
}

type TenantAdminDriveAdminContentFileInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug" minLength:"3" maxLength:"64"`
	FilePublicID  string      `path:"filePublicId" format:"uuid"`
}

func registerTenantAdminDriveRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listTenantAdminDriveShares",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/shares",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "tenant admin 用 Drive share state を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveBySlugInput) (*TenantAdminDriveSharesOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListAdminDriveShares(ctx, tenant.ID)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		out := &TenantAdminDriveSharesOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toTenantAdminDriveShareStateBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listTenantAdminDriveShareLinks",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/share-links",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "tenant admin 用 Drive share link state を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveBySlugInput) (*TenantAdminDriveShareLinksOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListAdminDriveShareLinks(ctx, tenant.ID)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		out := &TenantAdminDriveShareLinksOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toTenantAdminDriveShareLinkStateBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listTenantAdminDriveInvitations",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/invitations",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "tenant admin 用 Drive invitations を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveBySlugInput) (*TenantAdminDriveInvitationsOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListAdminDriveInvitations(ctx, tenant.ID)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		out := &TenantAdminDriveInvitationsOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDriveShareInvitationBody(item, false))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listTenantAdminDriveAuditEvents",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/audit-events",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "tenant admin 用 Drive audit events を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveBySlugInput) (*TenantAdminDriveAuditOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListAdminDriveAuditEvents(ctx, tenant.ID, input.Limit)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		out := &TenantAdminDriveAuditOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toTenantAdminDriveAuditEventBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listTenantAdminDriveShareApprovals",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/share-approvals",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "tenant admin 用 Drive share approvals を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveBySlugInput) (*TenantAdminDriveInvitationsOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListShareApprovals(ctx, tenant.ID)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		out := &TenantAdminDriveInvitationsOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDriveShareInvitationBody(item, false))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "approveTenantAdminDriveShareApproval",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/share-approvals/{invitationPublicId}/approve",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "Drive external share approval を承認する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveApprovalInput) (*TenantAdminNoContentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.ApproveShareInvitation(ctx, tenant.ID, current.User.ID, input.InvitationPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &TenantAdminNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "rejectTenantAdminDriveShareApproval",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/share-approvals/{invitationPublicId}/reject",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "Drive external share approval を拒否する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveApprovalInput) (*TenantAdminNoContentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.RejectShareInvitation(ctx, tenant.ID, current.User.ID, input.InvitationPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &TenantAdminNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getTenantAdminDriveOpenFGADrift",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/openfga-sync/drift",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "Drive OpenFGA drift dry-run を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveBySlugInput) (*TenantAdminDriveSyncOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		result, err := deps.DriveService.OpenFGADrift(ctx, tenant.ID)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return toTenantAdminDriveSyncOutput(result), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "repairTenantAdminDriveOpenFGASync",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/openfga-sync/repair",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "Drive OpenFGA pending sync を修復する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveMutationInput) (*TenantAdminDriveSyncOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		result, err := deps.DriveService.RepairOpenFGASync(ctx, tenant.ID, current.User.ID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return toTenantAdminDriveSyncOutput(result), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getTenantAdminDriveOperationsHealth",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/operations/health",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "Drive / OpenFGA / storage operations health を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveBySlugInput) (*TenantAdminDriveOperationsHealthOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		health, err := deps.DriveService.DriveOperationsHealth(ctx, tenant.ID)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &TenantAdminDriveOperationsHealthOutput{Body: toTenantAdminDriveOperationsHealthBody(health)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "checkTenantAdminDriveOperationsDrift",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/operations/drift-check",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "Drive OpenFGA drift dry-run を operation endpoint から実行する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveMutationInput) (*TenantAdminDriveSyncOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		result, err := deps.DriveService.OpenFGADrift(ctx, tenant.ID)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return toTenantAdminDriveSyncOutput(result), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "repairTenantAdminDriveOperations",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/operations/repair",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "Drive OpenFGA pending sync repair を operation endpoint から実行する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveMutationInput) (*TenantAdminDriveSyncOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		result, err := deps.DriveService.RepairOpenFGASync(ctx, tenant.ID, current.User.ID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return toTenantAdminDriveSyncOutput(result), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "startTenantAdminDriveContentAccessSession",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/content-access-sessions",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "Drive admin content break-glass session を開始する",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveAdminContentSessionInput) (*TenantAdminDriveAdminContentSessionOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		session, err := deps.DriveService.StartAdminContentAccessSession(ctx, service.DriveStartAdminContentAccessInput{
			TenantID:       tenant.ID,
			ActorUserID:    current.User.ID,
			Reason:         input.Body.Reason,
			ReasonCategory: input.Body.ReasonCategory,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &TenantAdminDriveAdminContentSessionOutput{Body: toTenantAdminDriveAdminContentSessionBody(session)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "endTenantAdminDriveContentAccessSession",
		Method:        http.MethodDelete,
		Path:          "/api/v1/admin/tenants/{tenantSlug}/drive/content-access-sessions/current",
		Tags:          []string{"tenant-admin-drive"},
		Summary:       "Drive admin content break-glass session を終了する",
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveMutationInput) (*TenantAdminNoContentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.EndAdminContentAccessSession(ctx, tenant.ID, current.User.ID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &TenantAdminNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getTenantAdminDriveFileMetadata",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/tenants/{tenantSlug}/drive/files/{filePublicId}/metadata",
		Tags:        []string{"tenant-admin-drive"},
		Summary:     "break-glass session 中に Drive file metadata を返す",
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *TenantAdminDriveAdminContentFileInput) (*DriveFileOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		file, err := deps.DriveService.GetAdminDriveFileMetadata(ctx, tenant.ID, current.User.ID, input.FilePublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveFileOutput{Body: toDriveFileBody(file)}, nil
	})
}

func toTenantAdminDriveShareStateBody(item service.DriveAdminShareState) TenantAdminDriveShareStateBody {
	return TenantAdminDriveShareStateBody{
		PublicID:         item.PublicID,
		ResourceType:     item.ResourceType,
		ResourcePublicID: item.ResourcePublicID,
		ResourceName:     item.ResourceName,
		SubjectType:      item.SubjectType,
		SubjectPublicID:  item.SubjectPublicID,
		Role:             item.Role,
		Status:           item.Status,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func toTenantAdminDriveShareLinkStateBody(item service.DriveAdminShareLinkState) TenantAdminDriveShareLinkStateBody {
	return TenantAdminDriveShareLinkStateBody{
		PublicID:         item.PublicID,
		ResourceType:     item.ResourceType,
		ResourcePublicID: item.ResourcePublicID,
		ResourceName:     item.ResourceName,
		CanDownload:      item.CanDownload,
		PasswordRequired: item.PasswordRequired,
		Status:           item.Status,
		ExpiresAt:        item.ExpiresAt,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func toTenantAdminDriveAuditEventBody(item service.DriveAdminAuditEvent) TenantAdminDriveAuditEventBody {
	return TenantAdminDriveAuditEventBody{
		PublicID:   item.PublicID,
		ActorType:  item.ActorType,
		Action:     item.Action,
		TargetType: item.TargetType,
		TargetID:   item.TargetID,
		Metadata:   item.Metadata,
		OccurredAt: item.OccurredAt,
	}
}

func toTenantAdminDriveOperationsHealthBody(item service.DriveOperationsHealth) TenantAdminDriveOperationsHealthBody {
	return TenantAdminDriveOperationsHealthBody{
		TenantID:                item.TenantID,
		WorkspaceCount:          item.WorkspaceCount,
		MissingWorkspaceCount:   item.MissingWorkspaceCount,
		OpenFGADriftCount:       item.OpenFGADriftCount,
		StorageMissingCount:     item.StorageMissingCount,
		StorageOrphanCheckState: item.StorageOrphanCheckState,
		CheckedAt:               item.CheckedAt,
	}
}

func toTenantAdminDriveAdminContentSessionBody(item service.DriveAdminContentAccessSession) TenantAdminDriveAdminContentSessionBody {
	return TenantAdminDriveAdminContentSessionBody{
		PublicID:       item.PublicID,
		ReasonCategory: item.ReasonCategory,
		ExpiresAt:      item.ExpiresAt,
		CreatedAt:      item.CreatedAt,
	}
}

func toTenantAdminDriveSyncOutput(result service.DriveOpenFGASyncResult) *TenantAdminDriveSyncOutput {
	out := &TenantAdminDriveSyncOutput{}
	out.Body.DryRun = result.DryRun
	for _, item := range result.Items {
		out.Body.Items = append(out.Body.Items, TenantAdminDriveSyncItemBody{
			Kind:     item.Kind,
			PublicID: item.PublicID,
			Status:   item.Status,
			Action:   item.Action,
			Error:    item.Error,
		})
	}
	return out
}
