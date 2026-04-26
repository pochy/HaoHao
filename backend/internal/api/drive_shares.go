package api

import (
	"context"
	"net/http"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DrivePermissionsOutput struct {
	Body DrivePermissionsBody
}

type DriveShareOutput struct {
	Body DriveShareBody
}

type GetDriveFilePermissionsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	FilePublicID  string      `path:"filePublicId"`
}

type GetDriveFolderPermissionsInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	FolderPublicID string      `path:"folderPublicId"`
}

type CreateDriveShareBody struct {
	SubjectType     string `json:"subjectType" enum:"user,group"`
	SubjectPublicID string `json:"subjectPublicId"`
	Role            string `json:"role" enum:"owner,editor,viewer"`
}

type CreateDriveFileShareInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId"`
	Body          CreateDriveShareBody
}

type CreateDriveFolderShareInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FolderPublicID string      `path:"folderPublicId"`
	Body           CreateDriveShareBody
}

type DeleteDriveFileShareInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId"`
	SharePublicID string      `path:"sharePublicId"`
}

type DeleteDriveFolderShareInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FolderPublicID string      `path:"folderPublicId"`
	SharePublicID  string      `path:"sharePublicId"`
}

func registerDriveShareRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getDriveFilePermissions",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/files/{filePublicId}/permissions",
		Summary:     "Drive file permissions を返す",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *GetDriveFilePermissionsInput) (*DrivePermissionsOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		permissions, err := deps.DriveService.ListPermissions(ctx, tenant.ID, current.User.ID, service.DriveResourceRef{Type: service.DriveResourceTypeFile, PublicID: input.FilePublicID, TenantID: tenant.ID}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DrivePermissionsOutput{Body: toDrivePermissionsBody(permissions)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDriveFolderPermissions",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/folders/{folderPublicId}/permissions",
		Summary:     "Drive folder permissions を返す",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *GetDriveFolderPermissionsInput) (*DrivePermissionsOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		permissions, err := deps.DriveService.ListPermissions(ctx, tenant.ID, current.User.ID, service.DriveResourceRef{Type: service.DriveResourceTypeFolder, PublicID: input.FolderPublicID, TenantID: tenant.ID}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DrivePermissionsOutput{Body: toDrivePermissionsBody(permissions)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDriveFileShare",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/files/{filePublicId}/shares",
		Summary:     "Drive file share を作成する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *CreateDriveFileShareInput) (*DriveShareOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		share, err := deps.DriveService.CreateShare(ctx, service.DriveCreateShareInput{
			TenantID:        tenant.ID,
			ActorUserID:     current.User.ID,
			Resource:        service.DriveResourceRef{Type: service.DriveResourceTypeFile, PublicID: input.FilePublicID, TenantID: tenant.ID},
			SubjectType:     service.DriveShareSubjectType(input.Body.SubjectType),
			SubjectPublicID: input.Body.SubjectPublicID,
			Role:            service.DriveRole(input.Body.Role),
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveShareOutput{Body: toDriveShareBody(share)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDriveFolderShare",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/folders/{folderPublicId}/shares",
		Summary:     "Drive folder share を作成する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *CreateDriveFolderShareInput) (*DriveShareOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		share, err := deps.DriveService.CreateShare(ctx, service.DriveCreateShareInput{
			TenantID:        tenant.ID,
			ActorUserID:     current.User.ID,
			Resource:        service.DriveResourceRef{Type: service.DriveResourceTypeFolder, PublicID: input.FolderPublicID, TenantID: tenant.ID},
			SubjectType:     service.DriveShareSubjectType(input.Body.SubjectType),
			SubjectPublicID: input.Body.SubjectPublicID,
			Role:            service.DriveRole(input.Body.Role),
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveShareOutput{Body: toDriveShareBody(share)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDriveFileShare",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/files/{filePublicId}/shares/{sharePublicId}",
		Summary:       "Drive file share を解除する",
		Tags:          []string{"drive"},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DeleteDriveFileShareInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.RevokeShare(ctx, service.DriveRevokeShareInput{TenantID: tenant.ID, ActorUserID: current.User.ID, ShareID: input.SharePublicID}, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDriveFolderShare",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/folders/{folderPublicId}/shares/{sharePublicId}",
		Summary:       "Drive folder share を解除する",
		Tags:          []string{"drive"},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DeleteDriveFolderShareInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.RevokeShare(ctx, service.DriveRevokeShareInput{TenantID: tenant.ID, ActorUserID: current.User.ID, ShareID: input.SharePublicID}, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})
}
