package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type DriveShareInvitationBody struct {
	PublicID           string    `json:"publicId"`
	ResourceType       string    `json:"resourceType"`
	ResourcePublicID   string    `json:"resourcePublicId,omitempty"`
	InviteeEmailDomain string    `json:"inviteeEmailDomain"`
	MaskedInviteeEmail string    `json:"maskedInviteeEmail,omitempty"`
	Role               string    `json:"role"`
	Status             string    `json:"status"`
	ExpiresAt          time.Time `json:"expiresAt" format:"date-time"`
	CreatedAt          time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt          time.Time `json:"updatedAt" format:"date-time"`
	AcceptToken        string    `json:"acceptToken,omitempty"`
}

type DriveShareInvitationOutput struct {
	Body DriveShareInvitationBody
}

type DriveShareInvitationListOutput struct {
	Body struct {
		Items []DriveShareInvitationBody `json:"items"`
	}
}

type CreateDriveShareInvitationBody struct {
	InviteeEmail        string     `json:"inviteeEmail" format:"email"`
	InviteeUserPublicID string     `json:"inviteeUserPublicId,omitempty"`
	Role                string     `json:"role" enum:"owner,editor,viewer"`
	ExpiresAt           *time.Time `json:"expiresAt,omitempty" format:"date-time"`
}

type CreateDriveFileInvitationInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	FilePublicID  string      `path:"filePublicId"`
	Body          CreateDriveShareInvitationBody
}

type CreateDriveFolderInvitationInput struct {
	SessionCookie  http.Cookie `cookie:"SESSION_ID"`
	CSRFToken      string      `header:"X-CSRF-Token" required:"true"`
	FolderPublicID string      `path:"folderPublicId"`
	Body           CreateDriveShareInvitationBody
}

type ListDriveShareInvitationsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type AcceptDriveShareInvitationBody struct {
	AcceptToken string `json:"acceptToken"`
}

type AcceptDriveShareInvitationInput struct {
	SessionCookie      http.Cookie `cookie:"SESSION_ID"`
	CSRFToken          string      `header:"X-CSRF-Token" required:"true"`
	InvitationPublicID string      `path:"invitationPublicId"`
	Body               AcceptDriveShareInvitationBody
}

type RevokeDriveShareInvitationInput struct {
	SessionCookie      http.Cookie `cookie:"SESSION_ID"`
	CSRFToken          string      `header:"X-CSRF-Token" required:"true"`
	InvitationPublicID string      `path:"invitationPublicId"`
}

func registerDriveInvitationRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "createDriveFileShareInvitation",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/files/{filePublicId}/invitations",
		Summary:     "Drive file external share invitation を作成する",
		Tags:        []string{DocTagDriveSharingPermissions},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *CreateDriveFileInvitationInput) (*DriveShareInvitationOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CreateShareInvitation(ctx, service.DriveCreateShareInvitationInput{
			TenantID:            tenant.ID,
			ActorUserID:         current.User.ID,
			Resource:            service.DriveResourceRef{Type: service.DriveResourceTypeFile, PublicID: input.FilePublicID, TenantID: tenant.ID},
			InviteeEmail:        input.Body.InviteeEmail,
			InviteeUserPublicID: input.Body.InviteeUserPublicID,
			Role:                service.DriveRole(input.Body.Role),
			ExpiresAt:           optionalTime(input.Body.ExpiresAt),
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveShareInvitationOutput{Body: toDriveShareInvitationBody(item, true)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDriveFolderShareInvitation",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/folders/{folderPublicId}/invitations",
		Summary:     "Drive folder external share invitation を作成する",
		Tags:        []string{DocTagDriveSharingPermissions},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *CreateDriveFolderInvitationInput) (*DriveShareInvitationOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		item, err := deps.DriveService.CreateShareInvitation(ctx, service.DriveCreateShareInvitationInput{
			TenantID:            tenant.ID,
			ActorUserID:         current.User.ID,
			Resource:            service.DriveResourceRef{Type: service.DriveResourceTypeFolder, PublicID: input.FolderPublicID, TenantID: tenant.ID},
			InviteeEmail:        input.Body.InviteeEmail,
			InviteeUserPublicID: input.Body.InviteeUserPublicID,
			Role:                service.DriveRole(input.Body.Role),
			ExpiresAt:           optionalTime(input.Body.ExpiresAt),
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveShareInvitationOutput{Body: toDriveShareInvitationBody(item, true)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "listDriveShareInvitations",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/invitations",
		Summary:     "ログイン user 宛の Drive invitation を返す",
		Tags:        []string{DocTagDriveSharingPermissions},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDriveShareInvitationsInput) (*DriveShareInvitationListOutput, error) {
		current, _, err := currentSessionAuthContext(ctx, deps, input.SessionCookie.Value)
		if err != nil {
			return nil, toSessionHTTPError(err)
		}
		items, err := deps.DriveService.ListShareInvitationsForUser(ctx, current.User.ID)
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		out := &DriveShareInvitationListOutput{}
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toDriveShareInvitationBody(item, false))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "acceptDriveShareInvitation",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/invitations/{invitationPublicId}/accept",
		Summary:     "Drive invitation を受諾する",
		Tags:        []string{DocTagDriveSharingPermissions},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *AcceptDriveShareInvitationInput) (*DriveShareOutput, error) {
		current, _, err := currentSessionAuthContextWithCSRF(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, toSessionHTTPError(err)
		}
		share, err := deps.DriveService.AcceptShareInvitation(ctx, service.DriveAcceptShareInvitationInput{
			ActorUserID:        current.User.ID,
			InvitationPublicID: input.InvitationPublicID,
			AcceptToken:        input.Body.AcceptToken,
		}, sessionAuditContext(ctx, current, nil))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveShareOutput{Body: toDriveShareBody(share)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "revokeDriveShareInvitation",
		Method:        http.MethodPost,
		Path:          "/api/v1/drive/invitations/{invitationPublicId}/revoke",
		Summary:       "Drive invitation を revoke する",
		Tags:          []string{DocTagDriveSharingPermissions},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *RevokeDriveShareInvitationInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		err = deps.DriveService.RevokeShareInvitation(ctx, service.DriveRevokeShareInvitationInput{
			TenantID:           tenant.ID,
			ActorUserID:        current.User.ID,
			InvitationPublicID: input.InvitationPublicID,
		}, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPErrorWithLog(ctx, deps, "", err)
		}
		return &DriveNoContentOutput{}, nil
	})
}

func toDriveShareInvitationBody(item service.DriveShareInvitation, includeToken bool) DriveShareInvitationBody {
	body := DriveShareInvitationBody{
		PublicID:           item.PublicID,
		ResourceType:       string(item.Resource.Type),
		ResourcePublicID:   item.Resource.PublicID,
		InviteeEmailDomain: item.InviteeEmailDomain,
		MaskedInviteeEmail: item.MaskedInviteeEmail,
		Role:               string(item.Role),
		Status:             item.Status,
		ExpiresAt:          item.ExpiresAt,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
	if includeToken {
		body.AcceptToken = item.RawAcceptToken
	}
	return body
}

func toSessionHTTPError(err error) error {
	var statusErr huma.StatusError
	if errors.As(err, &statusErr) {
		return err
	}
	return toHTTPError(err)
}
