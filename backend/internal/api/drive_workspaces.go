package api

import (
	"context"
	"encoding/json"
	"net/http"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DriveWorkspaceOutput struct {
	Body DriveWorkspaceBody
}

type DriveWorkspaceListOutput struct {
	Body struct {
		Items []DriveWorkspaceBody `json:"items"`
	}
}

type CreateDriveWorkspaceBody struct {
	Name              string         `json:"name" maxLength:"255"`
	StorageQuotaBytes *int64         `json:"storageQuotaBytes,omitempty"`
	PolicyOverride    map[string]any `json:"policyOverride,omitempty"`
}

type CreateDriveWorkspaceInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          CreateDriveWorkspaceBody
}

type ListDriveWorkspacesInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Limit         int32       `query:"limit" default:"100"`
}

type GetDriveWorkspaceInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	WorkspacePublicID string      `path:"workspacePublicId" format:"uuid"`
}

type UpdateDriveWorkspaceInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token" required:"true"`
	WorkspacePublicID string      `path:"workspacePublicId" format:"uuid"`
	Body              CreateDriveWorkspaceBody
}

type DeleteDriveWorkspaceInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token" required:"true"`
	WorkspacePublicID string      `path:"workspacePublicId" format:"uuid"`
}

func registerDriveWorkspaceRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listDriveWorkspaces",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/workspaces",
		Summary:     "Drive workspace 一覧を返す",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDriveWorkspacesInput) (*DriveWorkspaceListOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListWorkspaces(ctx, tenant.ID, current.User.ID, input.Limit)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		out := &DriveWorkspaceListOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDriveWorkspaceBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDriveWorkspace",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/workspaces",
		Summary:     "Drive workspace を作成する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *CreateDriveWorkspaceInput) (*DriveWorkspaceOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		workspace, err := deps.DriveService.CreateWorkspace(ctx, service.DriveCreateWorkspaceInput{
			TenantID:           tenant.ID,
			ActorUserID:        current.User.ID,
			Name:               input.Body.Name,
			StorageQuotaBytes:  input.Body.StorageQuotaBytes,
			PolicyOverrideJSON: drivePolicyOverrideJSON(input.Body.PolicyOverride),
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveWorkspaceOutput{Body: toDriveWorkspaceBody(workspace)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateDriveWorkspace",
		Method:      http.MethodPatch,
		Path:        "/api/v1/drive/workspaces/{workspacePublicId}",
		Summary:     "Drive workspace を更新する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *UpdateDriveWorkspaceInput) (*DriveWorkspaceOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		workspace, err := deps.DriveService.UpdateWorkspace(ctx, service.DriveUpdateWorkspaceInput{
			TenantID:           tenant.ID,
			ActorUserID:        current.User.ID,
			WorkspacePublicID:  input.WorkspacePublicID,
			Name:               input.Body.Name,
			StorageQuotaBytes:  input.Body.StorageQuotaBytes,
			PolicyOverrideJSON: drivePolicyOverrideJSON(input.Body.PolicyOverride),
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveWorkspaceOutput{Body: toDriveWorkspaceBody(workspace)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDriveWorkspace",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/workspaces/{workspacePublicId}",
		Summary:       "Drive workspace を削除する",
		Tags:          []string{"drive"},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DeleteDriveWorkspaceInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.DeleteWorkspace(ctx, tenant.ID, current.User.ID, input.WorkspacePublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})
}

func drivePolicyOverrideJSON(value map[string]any) []byte {
	if len(value) == 0 {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}
