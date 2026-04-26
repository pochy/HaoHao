package api

import (
	"context"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DriveShareLinkOutput struct {
	Body DriveShareLinkBody
}

type PublicDriveShareLinkOutput struct {
	Body struct {
		Link   DriveShareLinkBody `json:"link"`
		Folder *DriveFolderBody   `json:"folder,omitempty"`
		File   *DriveFileBody     `json:"file,omitempty"`
	}
}

type CreateDriveShareLinkBody struct {
	CanDownload bool       `json:"canDownload"`
	Role        string     `json:"role,omitempty" enum:"viewer,editor"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty" format:"date-time"`
	Password    string     `json:"password,omitempty" maxLength:"256"`
}

type CreateDriveFileShareLinkInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId"`
	Body          CreateDriveShareLinkBody
}

type CreateDriveFolderShareLinkInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FolderPublicID string      `path:"folderPublicId"`
	Body           CreateDriveShareLinkBody
}

type UpdateDriveShareLinkBody struct {
	CanDownload *bool      `json:"canDownload,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty" format:"date-time"`
}

type UpdateDriveShareLinkInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token" required:"true"`
	ShareLinkPublicID string      `path:"shareLinkPublicId"`
	Body              UpdateDriveShareLinkBody
}

type DeleteDriveShareLinkInput struct {
	SessionCookie     http.Cookie `cookie:"SESSION_ID"`
	CSRFToken         string      `header:"X-CSRF-Token" required:"true"`
	ShareLinkPublicID string      `path:"shareLinkPublicId"`
}

type PublicDriveShareLinkInput struct {
	Token string `path:"token"`
}

type PublicDriveShareLinkChildrenInput struct {
	Token              string      `path:"token"`
	VerificationCookie http.Cookie `cookie:"DRIVE_SHARE_LINK_VERIFICATION" required:"false"`
	Limit              int32       `query:"limit" default:"100"`
}

func registerDriveShareLinkRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "createDriveFileShareLink",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/files/{filePublicId}/share-links",
		Summary:     "Drive file share link を作成する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *CreateDriveFileShareLinkInput) (*DriveShareLinkOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		link, err := deps.DriveService.CreateShareLink(ctx, service.DriveCreateShareLinkInput{
			TenantID:    tenant.ID,
			ActorUserID: current.User.ID,
			Resource:    service.DriveResourceRef{Type: service.DriveResourceTypeFile, PublicID: input.FilePublicID, TenantID: tenant.ID},
			Role:        service.DriveRole(input.Body.Role),
			CanDownload: input.Body.CanDownload,
			ExpiresAt:   optionalTime(input.Body.ExpiresAt),
			Password:    input.Body.Password,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveShareLinkOutput{Body: toDriveShareLinkBody(link, true)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDriveFolderShareLink",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/folders/{folderPublicId}/share-links",
		Summary:     "Drive folder share link を作成する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *CreateDriveFolderShareLinkInput) (*DriveShareLinkOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		link, err := deps.DriveService.CreateShareLink(ctx, service.DriveCreateShareLinkInput{
			TenantID:    tenant.ID,
			ActorUserID: current.User.ID,
			Resource:    service.DriveResourceRef{Type: service.DriveResourceTypeFolder, PublicID: input.FolderPublicID, TenantID: tenant.ID},
			Role:        service.DriveRole(input.Body.Role),
			CanDownload: input.Body.CanDownload,
			ExpiresAt:   optionalTime(input.Body.ExpiresAt),
			Password:    input.Body.Password,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveShareLinkOutput{Body: toDriveShareLinkBody(link, true)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateDriveShareLink",
		Method:      http.MethodPatch,
		Path:        "/api/v1/drive/share-links/{shareLinkPublicId}",
		Summary:     "Drive share link を更新する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *UpdateDriveShareLinkInput) (*DriveShareLinkOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		link, err := deps.DriveService.UpdateShareLink(ctx, service.DriveUpdateShareLinkInput{
			TenantID:    tenant.ID,
			ActorUserID: current.User.ID,
			ShareLinkID: input.ShareLinkPublicID,
			CanDownload: input.Body.CanDownload,
			ExpiresAt:   input.Body.ExpiresAt,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveShareLinkOutput{Body: toDriveShareLinkBody(link, false)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDriveShareLink",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/share-links/{shareLinkPublicId}",
		Summary:       "Drive share link を無効化する",
		Tags:          []string{"drive"},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DeleteDriveShareLinkInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.DisableShareLink(ctx, service.DriveDisableShareLinkInput{TenantID: tenant.ID, ActorUserID: current.User.ID, ShareLinkID: input.ShareLinkPublicID}, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getPublicDriveShareLink",
		Method:      http.MethodGet,
		Path:        "/api/public/drive/share-links/{token}",
		Summary:     "public Drive share link metadata を返す",
		Tags:        []string{"drive-public"},
	}, func(ctx context.Context, input *PublicDriveShareLinkInput) (*PublicDriveShareLinkOutput, error) {
		if deps.DriveService == nil {
			return nil, huma.Error503ServiceUnavailable("drive service is not configured")
		}
		link, file, folder, err := deps.DriveService.PublicShareLinkMetadata(ctx, input.Token)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		out := &PublicDriveShareLinkOutput{}
		out.Body.Link = toDriveShareLinkBody(link, false)
		if file != nil {
			body := toDriveFileBody(*file)
			out.Body.File = &body
		}
		if folder != nil {
			body := toDriveFolderBody(*folder)
			out.Body.Folder = &body
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listPublicDriveShareLinkChildren",
		Method:      http.MethodGet,
		Path:        "/api/public/drive/share-links/{token}/children",
		Summary:     "public Drive folder share link children を返す",
		Tags:        []string{"drive-public"},
	}, func(ctx context.Context, input *PublicDriveShareLinkChildrenInput) (*DriveItemListOutput, error) {
		if deps.DriveService == nil {
			return nil, huma.Error503ServiceUnavailable("drive service is not configured")
		}
		items, err := deps.DriveService.PublicShareLinkFolderChildrenWithVerification(ctx, input.Token, input.VerificationCookie.Value, input.Limit)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return driveItemListOutput(items), nil
	})
}

func optionalTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
