package api

import (
	"context"
	"net/http"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type ListDriveTrashInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Limit         int32       `query:"limit" default:"100"`
}

type RestoreDriveResourceBody struct {
	ParentFolderPublicID *string `json:"parentFolderPublicId,omitempty"`
}

type RestoreDriveFileInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId"`
	Body          RestoreDriveResourceBody
}

type RestoreDriveFolderInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FolderPublicID string      `path:"folderPublicId"`
	Body           RestoreDriveResourceBody
}

func registerDriveTrashRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listDriveTrashItems",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/trash",
		Summary:     "Drive trash item 一覧を返す",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDriveTrashInput) (*DriveItemListOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListTrash(ctx, service.DriveListTrashInput{
			TenantID:    tenant.ID,
			ActorUserID: current.User.ID,
			Limit:       input.Limit,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return driveItemListOutput(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "restoreDriveFile",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/files/{filePublicId}/restore",
		Summary:     "Drive file を trash から復元する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *RestoreDriveFileInput) (*DriveFileOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		file, err := deps.DriveService.RestoreFile(ctx, service.DriveRestoreResourceInput{
			TenantID:             tenant.ID,
			ActorUserID:          current.User.ID,
			ResourcePublicID:     input.FilePublicID,
			ParentFolderPublicID: input.Body.ParentFolderPublicID,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveFileOutput{Body: toDriveFileBody(file)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "restoreDriveFolder",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/folders/{folderPublicId}/restore",
		Summary:     "Drive folder を trash から復元する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *RestoreDriveFolderInput) (*DriveFolderOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		folder, err := deps.DriveService.RestoreFolder(ctx, service.DriveRestoreResourceInput{
			TenantID:             tenant.ID,
			ActorUserID:          current.User.ID,
			ResourcePublicID:     input.FolderPublicID,
			ParentFolderPublicID: input.Body.ParentFolderPublicID,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveFolderOutput{Body: toDriveFolderBody(folder)}, nil
	})
}
