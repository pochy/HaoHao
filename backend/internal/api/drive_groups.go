package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type DriveGroupOutput struct {
	Body DriveGroupBody
}

type DriveGroupListOutput struct {
	Body struct {
		Items []DriveGroupBody `json:"items"`
	}
}

type ListDriveGroupsInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	Limit         int32       `query:"limit" default:"100"`
}

type CreateDriveGroupBody struct {
	Name        string `json:"name" maxLength:"255"`
	Description string `json:"description,omitempty" maxLength:"2000"`
}

type CreateDriveGroupInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          CreateDriveGroupBody
}

type GetDriveGroupInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	GroupPublicID string      `path:"groupPublicId"`
}

type UpdateDriveGroupInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	GroupPublicID string      `path:"groupPublicId"`
	Body          CreateDriveGroupBody
}

type DeleteDriveGroupInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	GroupPublicID string      `path:"groupPublicId"`
}

type AddDriveGroupMemberBody struct {
	UserPublicID string `json:"userPublicId"`
}

type AddDriveGroupMemberInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	GroupPublicID string      `path:"groupPublicId"`
	Body          AddDriveGroupMemberBody
}

type DeleteDriveGroupMemberInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	GroupPublicID string      `path:"groupPublicId"`
	UserPublicID  string      `path:"userPublicId"`
}

func registerDriveGroupRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listDriveGroups",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/groups",
		Summary:     "Drive groups を返す",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *ListDriveGroupsInput) (*DriveGroupListOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		groups, err := deps.DriveService.ListGroups(ctx, tenant.ID, current.User.ID, input.Limit)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		out := &DriveGroupListOutput{}
		out.Body.Items = make([]DriveGroupBody, 0, len(groups))
		for _, group := range groups {
			out.Body.Items = append(out.Body.Items, toDriveGroupBody(group, nil))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createDriveGroup",
		Method:      http.MethodPost,
		Path:        "/api/v1/drive/groups",
		Summary:     "Drive group を作成する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *CreateDriveGroupInput) (*DriveGroupOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		group, err := deps.DriveService.CreateGroup(ctx, tenant.ID, current.User.ID, input.Body.Name, input.Body.Description, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveGroupOutput{Body: toDriveGroupBody(group, nil)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "getDriveGroup",
		Method:      http.MethodGet,
		Path:        "/api/v1/drive/groups/{groupPublicId}",
		Summary:     "Drive group detail を返す",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *GetDriveGroupInput) (*DriveGroupOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}
		group, members, err := deps.DriveService.GetGroup(ctx, tenant.ID, current.User.ID, input.GroupPublicID)
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveGroupOutput{Body: toDriveGroupBody(group, members)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateDriveGroup",
		Method:      http.MethodPatch,
		Path:        "/api/v1/drive/groups/{groupPublicId}",
		Summary:     "Drive group を更新する",
		Tags:        []string{"drive"},
		Security:    []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *UpdateDriveGroupInput) (*DriveGroupOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		group, err := deps.DriveService.UpdateGroup(ctx, tenant.ID, current.User.ID, input.GroupPublicID, input.Body.Name, input.Body.Description, sessionAuditContext(ctx, current, &tenant.ID))
		if err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveGroupOutput{Body: toDriveGroupBody(group, nil)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDriveGroup",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/groups/{groupPublicId}",
		Summary:       "Drive group を削除する",
		Tags:          []string{"drive"},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DeleteDriveGroupInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.DeleteGroup(ctx, tenant.ID, current.User.ID, input.GroupPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "addDriveGroupMember",
		Method:        http.MethodPost,
		Path:          "/api/v1/drive/groups/{groupPublicId}/members",
		Summary:       "Drive group member を追加する",
		Tags:          []string{"drive"},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *AddDriveGroupMemberInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.AddGroupMemberByPublicID(ctx, tenant.ID, current.User.ID, input.GroupPublicID, input.Body.UserPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDriveGroupMember",
		Method:        http.MethodDelete,
		Path:          "/api/v1/drive/groups/{groupPublicId}/members/{userPublicId}",
		Summary:       "Drive group member を削除する",
		Tags:          []string{"drive"},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"cookieAuth": {}}},
	}, func(ctx context.Context, input *DeleteDriveGroupMemberInput) (*DriveNoContentOutput, error) {
		current, tenant, err := requireDriveTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}
		if err := deps.DriveService.RemoveGroupMemberByPublicID(ctx, tenant.ID, current.User.ID, input.GroupPublicID, input.UserPublicID, sessionAuditContext(ctx, current, &tenant.ID)); err != nil {
			return nil, toDriveHTTPError(err)
		}
		return &DriveNoContentOutput{}, nil
	})
}
