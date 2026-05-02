package api

import (
	"context"
	"net/http"
	"strings"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type ExternalDriveFileInput struct {
	TenantID     string `header:"X-Tenant-ID" required:"true"`
	FilePublicID string `path:"fileId" format:"uuid"`
}

type ExternalDriveFolderChildrenInput struct {
	TenantID       string `header:"X-Tenant-ID" required:"true"`
	FolderPublicID string `path:"folderId"`
	Limit          int32  `query:"limit" default:"100"`
}

type ExternalDriveCreateFolderInput struct {
	TenantID string `header:"X-Tenant-ID" required:"true"`
	Body     CreateDriveFolderBody
}

type ExternalDriveShareInput struct {
	TenantID     string `header:"X-Tenant-ID" required:"true"`
	FilePublicID string `path:"fileId" format:"uuid"`
	Body         CreateDriveShareBody
}

func registerExternalDriveRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "getExternalDriveFileMetadata",
		Method:      http.MethodGet,
		Path:        "/api/external/v1/drive/files/{fileId}/metadata",
		Summary:     "external bearer Drive file metadata を返す",
		Tags:        []string{"external-drive"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *ExternalDriveFileInput) (*DriveFileOutput, error) {
		authCtx, tenant, err := requireExternalDriveUser(ctx, "drive:read")
		if err != nil {
			return nil, err
		}
		file, err := deps.DriveService.GetFile(ctx, tenant.ID, authCtx.User.ID, input.FilePublicID, externalDriveAuditContext(authCtx, tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveFileOutput{Body: toDriveFileBody(file)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "deleteExternalDriveFile",
		Method:      http.MethodDelete,
		Path:        "/api/external/v1/drive/files/{fileId}",
		Summary:     "external bearer Drive file を削除する",
		Tags:        []string{"external-drive"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *ExternalDriveFileInput) (*DriveNoContentOutput, error) {
		authCtx, tenant, err := requireExternalDriveUser(ctx, "drive:write")
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.DeleteFile(ctx, tenant.ID, authCtx.User.ID, input.FilePublicID, externalDriveAuditContext(authCtx, tenant.ID)); err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listExternalDriveFolderChildren",
		Method:      http.MethodGet,
		Path:        "/api/external/v1/drive/folders/{folderId}/children",
		Summary:     "external bearer Drive folder children を返す",
		Tags:        []string{"external-drive"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *ExternalDriveFolderChildrenInput) (*DriveItemListOutput, error) {
		authCtx, tenant, err := requireExternalDriveUser(ctx, "drive:read")
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListChildren(ctx, service.DriveListChildrenInput{
			TenantID:             tenant.ID,
			ActorUserID:          authCtx.User.ID,
			ParentFolderPublicID: input.FolderPublicID,
			Limit:                input.Limit,
		}, externalDriveAuditContext(authCtx, tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return driveItemListOutput(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createExternalDriveFolder",
		Method:      http.MethodPost,
		Path:        "/api/external/v1/drive/folders",
		Summary:     "external bearer Drive folder を作成する",
		Tags:        []string{"external-drive"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *ExternalDriveCreateFolderInput) (*DriveFolderOutput, error) {
		authCtx, tenant, err := requireExternalDriveUser(ctx, "drive:write")
		if err != nil {
			return nil, err
		}
		folder, err := deps.DriveService.CreateFolder(ctx, service.DriveCreateFolderInput{
			TenantID:             tenant.ID,
			ActorUserID:          authCtx.User.ID,
			WorkspacePublicID:    input.Body.WorkspacePublicID,
			ParentFolderPublicID: input.Body.ParentFolderPublicID,
			Name:                 input.Body.Name,
		}, externalDriveAuditContext(authCtx, tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveFolderOutput{Body: toDriveFolderBody(folder)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createExternalDriveFileShare",
		Method:      http.MethodPost,
		Path:        "/api/external/v1/drive/files/{fileId}/shares",
		Summary:     "external bearer Drive file share を作成する",
		Tags:        []string{"external-drive"},
		Security:    []map[string][]string{{"bearerAuth": {}}},
	}, func(ctx context.Context, input *ExternalDriveShareInput) (*DriveShareOutput, error) {
		authCtx, tenant, err := requireExternalDriveUser(ctx, "drive:share")
		if err != nil {
			return nil, err
		}
		share, err := deps.DriveService.CreateShare(ctx, service.DriveCreateShareInput{
			TenantID:        tenant.ID,
			ActorUserID:     authCtx.User.ID,
			Resource:        service.DriveResourceRef{Type: service.DriveResourceTypeFile, PublicID: input.FilePublicID, TenantID: tenant.ID},
			SubjectType:     service.DriveShareSubjectType(input.Body.SubjectType),
			SubjectPublicID: input.Body.SubjectPublicID,
			Role:            service.DriveRole(input.Body.Role),
		}, externalDriveAuditContext(authCtx, tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveShareOutput{Body: toDriveShareBody(share)}, nil
	})
}

func requireExternalDriveUser(ctx context.Context, scope string) (service.AuthContext, service.TenantAccess, error) {
	authCtx, ok := service.AuthContextFromContext(ctx)
	if !ok {
		return service.AuthContext{}, service.TenantAccess{}, huma.Error500InternalServerError("missing auth context")
	}
	if authCtx.User == nil {
		return service.AuthContext{}, service.TenantAccess{}, huma.Error403Forbidden("external Drive API requires a user bearer principal")
	}
	if authCtx.ActiveTenant == nil {
		return service.AuthContext{}, service.TenantAccess{}, huma.Error409Conflict("X-Tenant-ID is required")
	}
	if !hasScope(authCtx.Scopes, scope) {
		return service.AuthContext{}, service.TenantAccess{}, huma.Error403Forbidden(scope + " scope is required")
	}
	return authCtx, *authCtx.ActiveTenant, nil
}

func externalDriveAuditContext(authCtx service.AuthContext, tenantID int64) service.AuditContext {
	return service.AuditContext{
		ActorType:   service.AuditActorUser,
		ActorUserID: &authCtx.User.ID,
		TenantID:    &tenantID,
	}
}

func hasScope(scopes []string, scope string) bool {
	needle := strings.TrimSpace(scope)
	for _, item := range scopes {
		if strings.TrimSpace(item) == needle {
			return true
		}
	}
	return false
}
