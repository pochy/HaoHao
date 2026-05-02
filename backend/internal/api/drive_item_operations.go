package api

import (
	"context"
	"fmt"
	"net/http"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DriveItemCollectionInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Limit         int32       `query:"limit" default:"100"`
}

type DriveActivityListOutput struct {
	Body struct {
		Items []DriveActivityBody `json:"items"`
	}
}

type DriveStorageUsageOutput struct {
	Body DriveStorageUsageBody
}

type DriveFolderTreeInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Limit         int32       `query:"limit" default:"500"`
}

type DriveFolderTreeOutput struct {
	Body DriveFolderTreeBody
}

type DriveShareTargetsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Query         string      `query:"q"`
	Limit         int32       `query:"limit" default:"20"`
}

type DriveShareTargetsOutput struct {
	Body struct {
		Items []DriveShareTargetBody `json:"items"`
	}
}

type DriveFileStarInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId"`
}

type DriveFolderStarInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FolderPublicID string      `path:"folderPublicId"`
}

type DriveFileActivityInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	FilePublicID  string      `path:"filePublicId"`
	Limit         int32       `query:"limit" default:"50"`
}

type DriveFolderActivityInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	FolderPublicID string      `path:"folderPublicId"`
	Limit          int32       `query:"limit" default:"50"`
}

type DriveCopyBody struct {
	ParentFolderPublicID string `json:"parentFolderPublicId,omitempty"`
	Name                 string `json:"name,omitempty" maxLength:"255"`
}

type DriveFileCopyInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId"`
	Body          DriveCopyBody
}

type DriveFolderCopyInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FolderPublicID string      `path:"folderPublicId"`
	Body           DriveCopyBody
}

type DriveOwnerTransferBody struct {
	NewOwnerUserPublicID      string `json:"newOwnerUserPublicId" format:"uuid"`
	RevokePreviousOwnerAccess bool   `json:"revokePreviousOwnerAccess"`
}

type DriveFileOwnerTransferInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId"`
	Body          DriveOwnerTransferBody
}

type DriveFolderOwnerTransferInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FolderPublicID string      `path:"folderPublicId"`
	Body           DriveOwnerTransferBody
}

type DriveFilePermanentDeleteInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId"`
}

type DriveFolderPermanentDeleteInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FolderPublicID string      `path:"folderPublicId"`
}

type DriveArchiveItemBody struct {
	Type     string `json:"type" enum:"file,folder"`
	PublicID string `json:"publicId" format:"uuid"`
}

type DriveArchiveBody struct {
	Items    []DriveArchiveItemBody `json:"items" minItems:"1" maxItems:"100"`
	Filename string                 `json:"filename,omitempty" maxLength:"255"`
}

type DriveArchiveInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          DriveArchiveBody
}

type DriveArchiveOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	Body               []byte
}

func registerDriveItemOperationsRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listDriveSharedWithMe",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/shared-with-me",
		Summary:     "自分に共有された Drive item を返す",
		Tags:        []string{DocTagDriveSharingPermissions},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveItemCollectionInput) (*DriveItemListOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListSharedWithMe(ctx, tenant.ID, current.User.ID, input.Limit, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return driveItemListOutput(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDriveStarred",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/starred",
		Summary:     "Starred Drive item を返す",
		Tags:        []string{DocTagDriveFilesFolders},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveItemCollectionInput) (*DriveItemListOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListStarred(ctx, tenant.ID, current.User.ID, input.Limit, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return driveItemListOutput(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDriveRecent",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/recent",
		Summary:     "Recent Drive item を返す",
		Tags:        []string{DocTagDriveFilesFolders},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveItemCollectionInput) (*DriveItemListOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		items, err := deps.DriveService.ListRecent(ctx, tenant.ID, current.User.ID, input.Limit, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return driveItemListOutput(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDriveStorageUsage",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/storage",
		Summary:     "Drive storage usage を返す",
		Tags:        []string{DocTagDriveFilesFolders},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveItemCollectionInput) (*DriveStorageUsageOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		usage, err := deps.DriveService.GetStorageUsage(ctx, tenant.ID, current.User.ID)
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveStorageUsageOutput{Body: toDriveStorageUsageBody(usage)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDriveFolderTree",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/folder-tree",
		Summary:     "Drive folder tree を返す",
		Tags:        []string{DocTagDriveFilesFolders},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFolderTreeInput) (*DriveFolderTreeOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		tree, err := deps.DriveService.ListFolderTree(ctx, tenant.ID, current.User.ID, input.Limit, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveFolderTreeOutput{Body: toDriveFolderTreeBody(tree)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDriveShareTargets",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/share-targets",
		Summary:     "Drive share target candidates を返す",
		Tags:        []string{DocTagDriveSharingPermissions},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveShareTargetsInput) (*DriveShareTargetsOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		targets, err := deps.DriveService.ListShareTargets(ctx, tenant.ID, current.User.ID, input.Query, input.Limit)
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		out := &DriveShareTargetsOutput{}
		out.Body.Items = make([]DriveShareTargetBody, 0, len(targets))
		for _, target := range targets {
			out.Body.Items = append(out.Body.Items, toDriveShareTargetBody(target))
		}
		return out, nil
	})

	registerDriveStarRoutes(api, deps)
	registerDriveActivityRoutes(api, deps)
	registerDriveItemMutationRoutes(api, deps)
}

func registerDriveStarRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID:   "starDriveFile",
		Method:        http.MethodPost,
		Path:          "/api/v1/drive/files/{filePublicId}/star",
		Summary:       "Drive file に star を付ける",
		Tags:          []string{DocTagDriveFilesFolders},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFileStarInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		err = deps.DriveService.StarResource(ctx, tenant.ID, current.User.ID, service.DriveResourceRef{Type: service.DriveResourceTypeFile, PublicID: input.FilePublicID, TenantID: tenant.ID}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "unstarDriveFile",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/files/{filePublicId}/star",
		Summary:       "Drive file の star を外す",
		Tags:          []string{DocTagDriveFilesFolders},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFileStarInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		err = deps.DriveService.UnstarResource(ctx, tenant.ID, current.User.ID, service.DriveResourceRef{Type: service.DriveResourceTypeFile, PublicID: input.FilePublicID, TenantID: tenant.ID}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "starDriveFolder",
		Method:        http.MethodPost,
		Path:          "/api/v1/drive/folders/{folderPublicId}/star",
		Summary:       "Drive folder に star を付ける",
		Tags:          []string{DocTagDriveFilesFolders},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFolderStarInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		err = deps.DriveService.StarResource(ctx, tenant.ID, current.User.ID, service.DriveResourceRef{Type: service.DriveResourceTypeFolder, PublicID: input.FolderPublicID, TenantID: tenant.ID}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "unstarDriveFolder",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/folders/{folderPublicId}/star",
		Summary:       "Drive folder の star を外す",
		Tags:          []string{DocTagDriveFilesFolders},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFolderStarInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		err = deps.DriveService.UnstarResource(ctx, tenant.ID, current.User.ID, service.DriveResourceRef{Type: service.DriveResourceTypeFolder, PublicID: input.FolderPublicID, TenantID: tenant.ID}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})
}

func registerDriveActivityRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listDriveFileActivity",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/files/{filePublicId}/activity",
		Summary:     "Drive file activity を返す",
		Tags:        []string{DocTagDriveFilesFolders},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFileActivityInput) (*DriveActivityListOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		activities, err := deps.DriveService.ListActivity(ctx, tenant.ID, current.User.ID, service.DriveResourceRef{Type: service.DriveResourceTypeFile, PublicID: input.FilePublicID, TenantID: tenant.ID}, input.Limit, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return driveActivityListOutput(activities), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDriveFolderActivity",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/folders/{folderPublicId}/activity",
		Summary:     "Drive folder activity を返す",
		Tags:        []string{DocTagDriveFilesFolders},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFolderActivityInput) (*DriveActivityListOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		activities, err := deps.DriveService.ListActivity(ctx, tenant.ID, current.User.ID, service.DriveResourceRef{Type: service.DriveResourceTypeFolder, PublicID: input.FolderPublicID, TenantID: tenant.ID}, input.Limit, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return driveActivityListOutput(activities), nil
	})
}

func registerDriveItemMutationRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "downloadDriveArchive",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/downloads/archive",
		Summary:     "Drive items を ZIP archive として download する",
		Tags:        []string{DocTagDriveFilesFolders},
		Security:    []map[string][]string{{"cookieAuth": {}}},
		Responses: map[string]*huma.Response{
			"200": {
				Description: "ZIP archive",
				Content: map[string]*huma.MediaType{
					"application/zip": {
						Schema: &huma.Schema{Type: "string", Format: "binary"},
					},
				},
			},
		},
	}, func(ctx context.Context, input *DriveArchiveInput) (*DriveArchiveOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		items := make([]service.DriveArchiveItemInput, 0, len(input.Body.Items))
		for _, item := range input.Body.Items {
			items = append(items, service.DriveArchiveItemInput{
				Type:     service.DriveResourceType(item.Type),
				PublicID: item.PublicID,
			})
		}
		download, err := deps.DriveService.DownloadArchive(ctx, service.DriveArchiveInput{
			TenantID:    tenant.ID,
			ActorUserID: current.User.ID,
			Filename:    input.Body.Filename,
			Items:       items,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveArchiveOutput{
			ContentType:        download.ContentType,
			ContentDisposition: fmt.Sprintf("attachment; filename=%q", download.Filename),
			Body:               download.Body,
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "copyDriveFile",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/files/{filePublicId}/copy",
		Summary:     "Drive file を copy する",
		Tags:        []string{DocTagDriveFilesFolders},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFileCopyInput) (*DriveItemOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CopyResource(ctx, service.DriveCopyResourceInput{
			TenantID:             tenant.ID,
			ActorUserID:          current.User.ID,
			Resource:             service.DriveResourceRef{Type: service.DriveResourceTypeFile, PublicID: input.FilePublicID, TenantID: tenant.ID},
			ParentFolderPublicID: input.Body.ParentFolderPublicID,
			Name:                 input.Body.Name,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveItemOutput{Body: toDriveItemBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "copyDriveFolder",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/folders/{folderPublicId}/copy",
		Summary:     "Drive folder を copy する",
		Tags:        []string{DocTagDriveFilesFolders},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFolderCopyInput) (*DriveItemOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CopyResource(ctx, service.DriveCopyResourceInput{
			TenantID:             tenant.ID,
			ActorUserID:          current.User.ID,
			Resource:             service.DriveResourceRef{Type: service.DriveResourceTypeFolder, PublicID: input.FolderPublicID, TenantID: tenant.ID},
			ParentFolderPublicID: input.Body.ParentFolderPublicID,
			Name:                 input.Body.Name,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveItemOutput{Body: toDriveItemBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "transferDriveFileOwner",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/files/{filePublicId}/owner-transfer",
		Summary:     "Drive file owner を移譲する",
		Tags:        []string{DocTagDriveSharingPermissions},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFileOwnerTransferInput) (*DriveItemOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.TransferOwner(ctx, service.DriveOwnerTransferInput{
			TenantID:                  tenant.ID,
			ActorUserID:               current.User.ID,
			Resource:                  service.DriveResourceRef{Type: service.DriveResourceTypeFile, PublicID: input.FilePublicID, TenantID: tenant.ID},
			NewOwnerUserPublicID:      input.Body.NewOwnerUserPublicID,
			RevokePreviousOwnerAccess: input.Body.RevokePreviousOwnerAccess,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveItemOutput{Body: toDriveItemBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "transferDriveFolderOwner",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/folders/{folderPublicId}/owner-transfer",
		Summary:     "Drive folder owner を移譲する",
		Tags:        []string{DocTagDriveSharingPermissions},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFolderOwnerTransferInput) (*DriveItemOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.TransferOwner(ctx, service.DriveOwnerTransferInput{
			TenantID:                  tenant.ID,
			ActorUserID:               current.User.ID,
			Resource:                  service.DriveResourceRef{Type: service.DriveResourceTypeFolder, PublicID: input.FolderPublicID, TenantID: tenant.ID},
			NewOwnerUserPublicID:      input.Body.NewOwnerUserPublicID,
			RevokePreviousOwnerAccess: input.Body.RevokePreviousOwnerAccess,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveItemOutput{Body: toDriveItemBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "permanentlyDeleteDriveFile",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/files/{filePublicId}/permanent",
		Summary:       "Drive file を完全削除する",
		Tags:          []string{DocTagDriveFilesFolders},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFilePermanentDeleteInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		err = deps.DriveService.PermanentlyDeleteResource(ctx, service.DrivePermanentDeleteInput{
			TenantID:    tenant.ID,
			ActorUserID: current.User.ID,
			Resource:    service.DriveResourceRef{Type: service.DriveResourceTypeFile, PublicID: input.FilePublicID, TenantID: tenant.ID},
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "permanentlyDeleteDriveFolder",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/folders/{folderPublicId}/permanent",
		Summary:       "Drive folder を完全削除する",
		Tags:          []string{DocTagDriveFilesFolders},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DriveFolderPermanentDeleteInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		err = deps.DriveService.PermanentlyDeleteResource(ctx, service.DrivePermanentDeleteInput{
			TenantID:    tenant.ID,
			ActorUserID: current.User.ID,
			Resource:    service.DriveResourceRef{Type: service.DriveResourceTypeFolder, PublicID: input.FolderPublicID, TenantID: tenant.ID},
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})
}

func driveActivityListOutput(items []service.DriveActivity) *DriveActivityListOutput {
	out := &DriveActivityListOutput{}
	out.Body.Items = make([]DriveActivityBody, 0, len(items))
	for _, item := range items {
		out.Body.Items = append(out.Body.Items, toDriveActivityBody(item))
	}
	return out
}
