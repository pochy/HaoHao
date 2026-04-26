package api

import (
	"context"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DriveFolderOutput struct {
	Body DriveFolderBody
}

type DriveItemListOutput struct {
	Body struct {
		Items []DriveItemBody `json:"items"`
	}
}

type CreateDriveFolderBody struct {
	Name                 string `json:"name" maxLength:"255"`
	WorkspacePublicID    string `json:"workspacePublicId,omitempty" format:"uuid"`
	ParentFolderPublicID string `json:"parentFolderPublicId,omitempty"`
}

type CreateDriveFolderInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          CreateDriveFolderBody
}

type GetDriveFolderInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	FolderPublicID string      `path:"folderPublicId"`
}

type UpdateDriveFolderBody struct {
	Name                 *string `json:"name,omitempty" maxLength:"255"`
	ParentFolderPublicID *string `json:"parentFolderPublicId,omitempty"`
}

type UpdateDriveFolderInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FolderPublicID string      `path:"folderPublicId"`
	Body           UpdateDriveFolderBody
}

type DeleteDriveFolderInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FolderPublicID string      `path:"folderPublicId"`
}

type ListDriveChildrenInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	FolderPublicID    string      `path:"folderPublicId"`
	WorkspacePublicID string      `query:"workspacePublicId" format:"uuid"`
	Limit             int32       `query:"limit" default:"100"`
}

type DriveInheritanceBody struct {
	Enabled bool `json:"enabled"`
}

type UpdateDriveFolderInheritanceInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FolderPublicID string      `path:"folderPublicId"`
	Body           DriveInheritanceBody
}

type ListDriveItemsInput struct {
	SessionCookie        http.Cookie `cookie:"SESSION_ID"`
	WorkspacePublicID    string      `query:"workspacePublicId" format:"uuid"`
	FolderPublicID       string      `query:"folderPublicId"`
	ParentFolderPublicID string      `query:"parentFolderPublicId"`
	Limit                int32       `query:"limit" default:"100"`
}

type SearchDriveItemsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Query         string      `query:"q"`
	ContentType   string      `query:"contentType"`
	Limit         int32       `query:"limit" default:"100"`
}

func registerDriveRoutes(api huma.API, deps Dependencies) {
	registerDriveWorkspaceRoutes(api, deps)
	registerDriveFolderRoutes(api, deps)
	registerDriveFileRoutes(api, deps)
	registerDriveShareRoutes(api, deps)
	registerDriveGroupRoutes(api, deps)
	registerDriveShareLinkRoutes(api, deps)
	registerDriveInvitationRoutes(api, deps)
	registerDrivePhase8Routes(api, deps)
}

func registerDriveFolderRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "createDriveFolder",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/folders",
		Summary:     "Drive folder を作成する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *CreateDriveFolderInput) (*DriveFolderOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		folder, err := deps.DriveService.CreateFolder(ctx, service.DriveCreateFolderInput{
			TenantID:             tenant.ID,
			ActorUserID:          current.User.ID,
			WorkspacePublicID:    input.Body.WorkspacePublicID,
			ParentFolderPublicID: input.Body.ParentFolderPublicID,
			Name:                 input.Body.Name,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveFolderOutput{Body: toDriveFolderBody(folder)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDriveFolder",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/folders/{folderPublicId}",
		Summary:     "Drive folder detail を返す",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *GetDriveFolderInput) (*DriveFolderOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		if input.FolderPublicID == "root" {
			now := serviceNow()
			return &DriveFolderOutput{Body: DriveFolderBody{
				PublicID:           "root",
				Name:               "Root",
				InheritanceEnabled: true,
				CreatedAt:          now,
				UpdatedAt:          now,
			}}, nil
		}
		folder, err := deps.DriveService.GetFolder(ctx, tenant.ID, current.User.ID, input.FolderPublicID, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveFolderOutput{Body: toDriveFolderBody(folder)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateDriveFolder",
		Method:      http.MethodPatch,
		Path:        "/api/v1/drive/folders/{folderPublicId}",
		Summary:     "Drive folder を更新する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *UpdateDriveFolderInput) (*DriveFolderOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		folder, err := deps.DriveService.UpdateFolder(ctx, service.DriveUpdateFolderInput{
			TenantID:             tenant.ID,
			ActorUserID:          current.User.ID,
			FolderPublicID:       input.FolderPublicID,
			Name:                 input.Body.Name,
			ParentFolderPublicID: input.Body.ParentFolderPublicID,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveFolderOutput{Body: toDriveFolderBody(folder)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDriveFolder",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/folders/{folderPublicId}",
		Summary:       "Drive folder を削除する",
		Tags:          []string{"drive"},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DeleteDriveFolderInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.DeleteFolder(ctx, tenant.ID, current.User.ID, input.FolderPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDriveFolderChildren",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/folders/{folderPublicId}/children",
		Summary:     "Drive folder children を返す",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDriveChildrenInput) (*DriveItemListOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListChildren(ctx, service.DriveListChildrenInput{
			TenantID:             tenant.ID,
			ActorUserID:          current.User.ID,
			WorkspacePublicID:    input.WorkspacePublicID,
			ParentFolderPublicID: input.FolderPublicID,
			Limit:                input.Limit,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return driveItemListOutput(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateDriveFolderInheritance",
		Method:      http.MethodPatch,
		Path:        "/api/v1/drive/folders/{folderPublicId}/inheritance",
		Summary:     "Drive folder inheritance を更新する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *UpdateDriveFolderInheritanceInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		ref := service.DriveResourceRef{Type: service.DriveResourceTypeFolder, PublicID: input.FolderPublicID, TenantID: tenant.ID}
		if input.Body.Enabled {
			err = deps.DriveService.ResumeInheritance(ctx, tenant.ID, current.User.ID, ref, sessionAuditContext(ctx, current, &tenant.ID))
		} else {
			err = deps.DriveService.StopInheritance(ctx, tenant.ID, current.User.ID, ref, sessionAuditContext(ctx, current, &tenant.ID))
		}
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDriveItems",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/items",
		Summary:     "Drive item 一覧を返す",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDriveItemsInput) (*DriveItemListOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		parentID := input.FolderPublicID
		if parentID == "" {
			parentID = input.ParentFolderPublicID
		}
		items, err := deps.DriveService.ListChildren(ctx, service.DriveListChildrenInput{
			TenantID:             tenant.ID,
			ActorUserID:          current.User.ID,
			WorkspacePublicID:    input.WorkspacePublicID,
			ParentFolderPublicID: parentID,
			Limit:                input.Limit,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return driveItemListOutput(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "searchDriveItems",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/search",
		Summary:     "Drive item を検索する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *SearchDriveItemsInput) (*DriveItemListOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.Search(ctx, service.DriveSearchInput{
			TenantID:    tenant.ID,
			ActorUserID: current.User.ID,
			Query:       input.Query,
			ContentType: input.ContentType,
			Limit:       input.Limit,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return driveItemListOutput(items), nil
	})
}

func driveItemListOutput(items []service.DriveItem) *DriveItemListOutput {
	out := &DriveItemListOutput{}
	out.Body.Items = make([]DriveItemBody, 0, len(items))
	for _, item := range items {
		out.Body.Items = append(out.Body.Items, toDriveItemBody(item))
	}
	return out
}

func serviceNow() time.Time {
	return time.Now().UTC()
}
