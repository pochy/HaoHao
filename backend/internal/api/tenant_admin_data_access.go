package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type TenantAdminDataAccessGroupBody struct {
	PublicID        string                            `json:"publicId" format:"uuid"`
	Name            string                            `json:"name"`
	Description     string                            `json:"description"`
	SystemKey       string                            `json:"systemKey,omitempty"`
	CreatedByUserID *int64                            `json:"createdByUserId,omitempty"`
	CreatedAt       time.Time                         `json:"createdAt" format:"date-time"`
	UpdatedAt       time.Time                         `json:"updatedAt" format:"date-time"`
	Members         []TenantAdminDataAccessMemberBody `json:"members,omitempty"`
}

type TenantAdminDataAccessMemberBody struct {
	UserPublicID string    `json:"userPublicId" format:"uuid"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"displayName"`
	CreatedAt    time.Time `json:"createdAt" format:"date-time"`
}

type TenantAdminDataAccessGrantBody struct {
	SubjectType          string    `json:"subjectType" enum:"user,group"`
	SubjectUserPublicID  string    `json:"subjectUserPublicId,omitempty" format:"uuid"`
	SubjectUserEmail     string    `json:"subjectUserEmail,omitempty"`
	SubjectUserName      string    `json:"subjectUserName,omitempty"`
	SubjectGroupPublicID string    `json:"subjectGroupPublicId,omitempty" format:"uuid"`
	SubjectGroupName     string    `json:"subjectGroupName,omitempty"`
	Action               string    `json:"action"`
	CreatedAt            time.Time `json:"createdAt" format:"date-time"`
}

type TenantAdminDataAccessGroupListOutput struct {
	Body struct {
		Items []TenantAdminDataAccessGroupBody `json:"items"`
	}
}

type TenantAdminDataAccessGroupOutput struct {
	Body TenantAdminDataAccessGroupBody
}

type TenantAdminDataAccessPermissionListOutput struct {
	Body struct {
		Items []TenantAdminDataAccessGrantBody `json:"items"`
	}
}

type TenantAdminDataAccessNoContentOutput struct{}

type TenantAdminDataAccessGroupCreateBody struct {
	Name        string `json:"name" maxLength:"160"`
	Description string `json:"description,omitempty" maxLength:"2000"`
}

type TenantAdminDataAccessGroupUpdateBody struct {
	Name        string `json:"name" maxLength:"160"`
	Description string `json:"description,omitempty" maxLength:"2000"`
}

type TenantAdminDataAccessMemberWriteBody struct {
	UserPublicID string `json:"userPublicId" format:"uuid"`
}

type TenantAdminDataAccessPermissionWriteBody struct {
	SubjectType     string   `json:"subjectType" enum:"user,group"`
	SubjectPublicID string   `json:"subjectPublicId" format:"uuid"`
	Actions         []string `json:"actions"`
}

type TenantAdminDataAccessGroupListInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug"`
	Limit         int32       `query:"limit" minimum:"1" maximum:"200" default:"100"`
}

type TenantAdminDataAccessGroupCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug"`
	Body          TenantAdminDataAccessGroupCreateBody
}

type TenantAdminDataAccessGroupInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug"`
	GroupPublicID string      `path:"groupPublicId" format:"uuid"`
}

type TenantAdminDataAccessGroupUpdateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug"`
	GroupPublicID string      `path:"groupPublicId" format:"uuid"`
	Body          TenantAdminDataAccessGroupUpdateBody
}

type TenantAdminDataAccessMemberCreateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug"`
	GroupPublicID string      `path:"groupPublicId" format:"uuid"`
	Body          TenantAdminDataAccessMemberWriteBody
}

type TenantAdminDataAccessMemberDeleteInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug"`
	GroupPublicID string      `path:"groupPublicId" format:"uuid"`
	UserPublicID  string      `path:"userPublicId" format:"uuid"`
}

type TenantAdminDataAccessResourcePermissionInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	TenantSlug       string      `path:"tenantSlug"`
	ResourceType     string      `path:"resourceType" enum:"dataset,work_table,data_pipeline"`
	ResourcePublicID string      `path:"resourcePublicId" format:"uuid"`
}

type TenantAdminDataAccessResourcePermissionUpdateInput struct {
	SessionCookie    http.Cookie `cookie:"SESSION_ID"`
	CSRFToken        string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug       string      `path:"tenantSlug"`
	ResourceType     string      `path:"resourceType" enum:"dataset,work_table,data_pipeline"`
	ResourcePublicID string      `path:"resourcePublicId" format:"uuid"`
	Body             TenantAdminDataAccessPermissionWriteBody
}

type TenantAdminDataAccessScopePermissionInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	TenantSlug    string      `path:"tenantSlug"`
}

type TenantAdminDataAccessScopePermissionUpdateInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TenantSlug    string      `path:"tenantSlug"`
	Body          TenantAdminDataAccessPermissionWriteBody
}

func registerTenantAdminDataAccessRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{OperationID: "listTenantAdminDataAccessGroups", Method: http.MethodGet, Path: "/api/v1/admin/tenants/{tenantSlug}/data-access/groups", Tags: []string{DocTagTenantAdministration}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDataAccessGroupListInput) (*TenantAdminDataAccessGroupListOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		authz, err := requireDatasetAuthz(deps)
		if err != nil {
			return nil, err
		}
		groups, err := authz.ListGroups(ctx, tenant.ID, input.Limit)
		if err != nil {
			return nil, toDatasetDataAccessHTTPError(ctx, deps, "list tenant admin data access groups", err)
		}
		out := &TenantAdminDataAccessGroupListOutput{}
		for _, group := range groups {
			out.Body.Items = append(out.Body.Items, toTenantAdminDataAccessGroupBody(group, nil))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{OperationID: "createTenantAdminDataAccessGroup", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/data-access/groups", Tags: []string{DocTagTenantAdministration}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDataAccessGroupCreateInput) (*TenantAdminDataAccessGroupOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		group, err := deps.DatasetAuthorizationService.CreateGroup(ctx, tenant.ID, current.User.ID, input.Body.Name, input.Body.Description)
		if err != nil {
			return nil, toDatasetDataAccessHTTPError(ctx, deps, "create tenant admin data access group", err)
		}
		return &TenantAdminDataAccessGroupOutput{Body: toTenantAdminDataAccessGroupBody(group, nil)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "getTenantAdminDataAccessGroup", Method: http.MethodGet, Path: "/api/v1/admin/tenants/{tenantSlug}/data-access/groups/{groupPublicId}", Tags: []string{DocTagTenantAdministration}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDataAccessGroupInput) (*TenantAdminDataAccessGroupOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		group, members, err := deps.DatasetAuthorizationService.GetGroup(ctx, tenant.ID, input.GroupPublicID)
		if err != nil {
			return nil, toDatasetDataAccessHTTPError(ctx, deps, "get tenant admin data access group", err)
		}
		return &TenantAdminDataAccessGroupOutput{Body: toTenantAdminDataAccessGroupBody(group, members)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "updateTenantAdminDataAccessGroup", Method: http.MethodPatch, Path: "/api/v1/admin/tenants/{tenantSlug}/data-access/groups/{groupPublicId}", Tags: []string{DocTagTenantAdministration}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDataAccessGroupUpdateInput) (*TenantAdminDataAccessGroupOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		group, err := deps.DatasetAuthorizationService.UpdateGroup(ctx, tenant.ID, input.GroupPublicID, input.Body.Name, input.Body.Description)
		if err != nil {
			return nil, toDatasetDataAccessHTTPError(ctx, deps, "update tenant admin data access group", err)
		}
		return &TenantAdminDataAccessGroupOutput{Body: toTenantAdminDataAccessGroupBody(group, nil)}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "addTenantAdminDataAccessGroupMember", Method: http.MethodPost, Path: "/api/v1/admin/tenants/{tenantSlug}/data-access/groups/{groupPublicId}/members", Tags: []string{DocTagTenantAdministration}, DefaultStatus: http.StatusNoContent, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDataAccessMemberCreateInput) (*TenantAdminDataAccessNoContentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.DatasetAuthorizationService.AddGroupMember(ctx, tenant.ID, current.User.ID, input.GroupPublicID, input.Body.UserPublicID); err != nil {
			return nil, toDatasetDataAccessHTTPError(ctx, deps, "add tenant admin data access group member", err)
		}
		return &TenantAdminDataAccessNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "removeTenantAdminDataAccessGroupMember", Method: http.MethodDelete, Path: "/api/v1/admin/tenants/{tenantSlug}/data-access/groups/{groupPublicId}/members/{userPublicId}", Tags: []string{DocTagTenantAdministration}, DefaultStatus: http.StatusNoContent, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDataAccessMemberDeleteInput) (*TenantAdminDataAccessNoContentOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.DatasetAuthorizationService.RemoveGroupMember(ctx, tenant.ID, input.GroupPublicID, input.UserPublicID); err != nil {
			return nil, toDatasetDataAccessHTTPError(ctx, deps, "remove tenant admin data access group member", err)
		}
		return &TenantAdminDataAccessNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "listTenantAdminDataAccessResourcePermissions", Method: http.MethodGet, Path: "/api/v1/admin/tenants/{tenantSlug}/data-access/{resourceType}/{resourcePublicId}/permissions", Tags: []string{DocTagTenantAdministration}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDataAccessResourcePermissionInput) (*TenantAdminDataAccessPermissionListOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		return listTenantAdminDataAccessPermissions(ctx, deps, tenant.ID, input.ResourceType, input.ResourcePublicID)
	})

	huma.Register(api, huma.Operation{OperationID: "putTenantAdminDataAccessResourcePermissions", Method: http.MethodPut, Path: "/api/v1/admin/tenants/{tenantSlug}/data-access/{resourceType}/{resourcePublicId}/permissions", Tags: []string{DocTagTenantAdministration}, DefaultStatus: http.StatusNoContent, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDataAccessResourcePermissionUpdateInput) (*TenantAdminDataAccessNoContentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.DatasetAuthorizationService.PutPermissionGrants(ctx, tenant.ID, current.User.ID, input.ResourceType, input.ResourcePublicID, input.Body.SubjectType, input.Body.SubjectPublicID, input.Body.Actions); err != nil {
			return nil, toDatasetDataAccessHTTPError(ctx, deps, "put tenant admin data access resource permissions", err)
		}
		return &TenantAdminDataAccessNoContentOutput{}, nil
	})

	huma.Register(api, huma.Operation{OperationID: "listTenantAdminDataAccessScopePermissions", Method: http.MethodGet, Path: "/api/v1/admin/tenants/{tenantSlug}/data-access/scope/permissions", Tags: []string{DocTagTenantAdministration}, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDataAccessScopePermissionInput) (*TenantAdminDataAccessPermissionListOutput, error) {
		_, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, "", input.TenantSlug)
		if err != nil {
			return nil, err
		}
		return listTenantAdminDataAccessPermissions(ctx, deps, tenant.ID, service.DataResourceScope, "")
	})

	huma.Register(api, huma.Operation{OperationID: "putTenantAdminDataAccessScopePermissions", Method: http.MethodPut, Path: "/api/v1/admin/tenants/{tenantSlug}/data-access/scope/permissions", Tags: []string{DocTagTenantAdministration}, DefaultStatus: http.StatusNoContent, Security: []map[string][]string{{"cookieAuth": {}}}}, func(ctx context.Context, input *TenantAdminDataAccessScopePermissionUpdateInput) (*TenantAdminDataAccessNoContentOutput, error) {
		current, tenant, err := requireAdminTenantID(ctx, deps, input.SessionCookie.Value, input.CSRFToken, input.TenantSlug)
		if err != nil {
			return nil, err
		}
		if err := deps.DatasetAuthorizationService.PutPermissionGrants(ctx, tenant.ID, current.User.ID, service.DataResourceScope, "", input.Body.SubjectType, input.Body.SubjectPublicID, input.Body.Actions); err != nil {
			return nil, toDatasetDataAccessHTTPError(ctx, deps, "put tenant admin data access scope permissions", err)
		}
		return &TenantAdminDataAccessNoContentOutput{}, nil
	})
}

func listTenantAdminDataAccessPermissions(ctx context.Context, deps Dependencies, tenantID int64, resourceType, resourcePublicID string) (*TenantAdminDataAccessPermissionListOutput, error) {
	authz, err := requireDatasetAuthz(deps)
	if err != nil {
		return nil, err
	}
	grants, err := authz.ListPermissionGrants(ctx, tenantID, resourceType, resourcePublicID)
	if err != nil {
		return nil, toDatasetDataAccessHTTPError(ctx, deps, "list tenant admin data access permissions", err)
	}
	out := &TenantAdminDataAccessPermissionListOutput{}
	for _, grant := range grants {
		out.Body.Items = append(out.Body.Items, toTenantAdminDataAccessGrantBody(grant))
	}
	return out, nil
}

func requireDatasetAuthz(deps Dependencies) (*service.DatasetAuthorizationService, error) {
	if deps.DatasetAuthorizationService == nil {
		return nil, huma.Error503ServiceUnavailable("dataset authorization service is not configured")
	}
	return deps.DatasetAuthorizationService, nil
}

func toTenantAdminDataAccessGroupBody(group service.DatasetPermissionGroup, members []service.DatasetPermissionGroupMember) TenantAdminDataAccessGroupBody {
	body := TenantAdminDataAccessGroupBody{
		PublicID:        group.PublicID,
		Name:            group.Name,
		Description:     group.Description,
		SystemKey:       group.SystemKey,
		CreatedByUserID: group.CreatedByUserID,
		CreatedAt:       group.CreatedAt.Time,
		UpdatedAt:       group.UpdatedAt.Time,
	}
	for _, member := range members {
		body.Members = append(body.Members, TenantAdminDataAccessMemberBody{
			UserPublicID: member.PublicID,
			Email:        member.Email,
			DisplayName:  member.DisplayName,
			CreatedAt:    member.CreatedAt.Time,
		})
	}
	return body
}

func toTenantAdminDataAccessGrantBody(grant service.DatasetPermissionGrant) TenantAdminDataAccessGrantBody {
	return TenantAdminDataAccessGrantBody{
		SubjectType:          grant.SubjectType,
		SubjectUserPublicID:  grant.SubjectUserPublicID,
		SubjectUserEmail:     grant.SubjectUserEmail,
		SubjectUserName:      grant.SubjectUserName,
		SubjectGroupPublicID: grant.SubjectGroupPublicID,
		SubjectGroupName:     grant.SubjectGroupName,
		Action:               grant.Action,
		CreatedAt:            grant.CreatedAt.Time,
	}
}

func toDatasetDataAccessHTTPError(ctx context.Context, deps Dependencies, operation string, err error) error {
	switch {
	case errors.Is(err, service.ErrDataPermissionDenied):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, service.ErrDataAuthzUnavailable):
		return dataAccessAuthorizationUnavailableHTTPError(ctx, deps, operation, err)
	default:
		return toDatasetHTTPError(ctx, deps, operation, err)
	}
}
