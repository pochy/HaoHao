package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

var externalIDFilterRE = regexp.MustCompile(`(?i)^\s*externalId\s+eq\s+"([^"]+)"\s*$`)

type SCIMUserBody struct {
	Schemas     []string        `json:"schemas,omitempty"`
	ID          string          `json:"id,omitempty" format:"uuid"`
	ExternalID  string          `json:"externalId,omitempty"`
	UserName    string          `json:"userName,omitempty" format:"email"`
	DisplayName string          `json:"displayName,omitempty"`
	Active      *bool           `json:"active,omitempty"`
	Groups      []SCIMGroupBody `json:"groups,omitempty"`
}

type SCIMGroupBody struct {
	Value string `json:"value"`
}

type SCIMListResponseBody struct {
	Schemas      []string       `json:"schemas"`
	TotalResults int            `json:"totalResults"`
	StartIndex   int32          `json:"startIndex"`
	ItemsPerPage int            `json:"itemsPerPage"`
	Resources    []SCIMUserBody `json:"Resources"`
}

type SCIMUserInput struct {
	Body SCIMUserBody
}

type SCIMUserByIDInput struct {
	ID string `path:"id" format:"uuid"`
}

type SCIMListUsersInput struct {
	Filter     string `query:"filter"`
	StartIndex int32  `query:"startIndex" minimum:"1" default:"1"`
	Count      int32  `query:"count" minimum:"1" maximum:"100" default:"100"`
}

type SCIMReplaceUserInput struct {
	ID   string `path:"id" format:"uuid"`
	Body SCIMUserBody
}

type SCIMPatchInput struct {
	ID   string `path:"id" format:"uuid"`
	Body struct {
		Schemas    []string             `json:"schemas,omitempty"`
		Operations []SCIMPatchOperation `json:"Operations"`
	}
}

type SCIMPatchOperation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

type SCIMUserOutput struct {
	Body SCIMUserBody
}

type SCIMListUsersOutput struct {
	Body SCIMListResponseBody
}

type SCIMDeleteUserOutput struct{}

func registerSCIMRoutes(api huma.API, deps Dependencies) {
	if deps.SCIMBasePath == "" {
		return
	}
	usersPath := deps.SCIMBasePath + "/Users"

	huma.Register(api, huma.Operation{
		OperationID: "scimCreateUser",
		Method:      http.MethodPost,
		Path:        usersPath,
		Summary:     "SCIM user を作成または upsert する",
		Tags:        []string{"scim"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *SCIMUserInput) (*SCIMUserOutput, error) {
		if deps.ProvisioningService == nil {
			return nil, huma.Error503ServiceUnavailable("scim provisioning is not configured")
		}
		user, err := deps.ProvisioningService.UpsertUser(ctx, provisionedInputFromSCIM(input.Body))
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		return &SCIMUserOutput{Body: scimUserFromProvisioned(user)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "scimListUsers",
		Method:      http.MethodGet,
		Path:        usersPath,
		Summary:     "SCIM user を list する",
		Tags:        []string{"scim"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *SCIMListUsersInput) (*SCIMListUsersOutput, error) {
		if deps.ProvisioningService == nil {
			return nil, huma.Error503ServiceUnavailable("scim provisioning is not configured")
		}
		if externalID := externalIDFromFilter(input.Filter); externalID != "" {
			user, err := deps.ProvisioningService.GetUserByExternalID(ctx, externalID)
			if err == nil {
				body := scimUserFromProvisioned(user)
				return &SCIMListUsersOutput{Body: scimList(input.StartIndex, []SCIMUserBody{body})}, nil
			}
			if errors.Is(err, service.ErrUnauthorized) {
				return &SCIMListUsersOutput{Body: scimList(input.StartIndex, nil)}, nil
			}
			if !errors.Is(err, service.ErrInvalidSCIMUser) {
				return nil, toSCIMHTTPError(err)
			}
		}

		users, err := deps.ProvisioningService.ListUsers(ctx, input.StartIndex, input.Count)
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		resources := make([]SCIMUserBody, 0, len(users))
		for _, user := range users {
			resources = append(resources, scimUserFromProvisioned(user))
		}
		return &SCIMListUsersOutput{Body: scimList(input.StartIndex, resources)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "scimGetUser",
		Method:      http.MethodGet,
		Path:        usersPath + "/{id}",
		Summary:     "SCIM user を取得する",
		Tags:        []string{"scim"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *SCIMUserByIDInput) (*SCIMUserOutput, error) {
		if deps.ProvisioningService == nil {
			return nil, huma.Error503ServiceUnavailable("scim provisioning is not configured")
		}
		user, err := deps.ProvisioningService.GetUser(ctx, input.ID)
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		return &SCIMUserOutput{Body: scimUserFromProvisioned(user)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "scimReplaceUser",
		Method:      http.MethodPut,
		Path:        usersPath + "/{id}",
		Summary:     "SCIM user を置換する",
		Tags:        []string{"scim"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *SCIMReplaceUserInput) (*SCIMUserOutput, error) {
		if deps.ProvisioningService == nil {
			return nil, huma.Error503ServiceUnavailable("scim provisioning is not configured")
		}
		existing, err := deps.ProvisioningService.GetUser(ctx, input.ID)
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		body := input.Body
		if strings.TrimSpace(body.ExternalID) == "" {
			body.ExternalID = existing.ExternalID
		}
		user, err := deps.ProvisioningService.UpsertUser(ctx, provisionedInputFromSCIM(body))
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		return &SCIMUserOutput{Body: scimUserFromProvisioned(user)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "scimPatchUser",
		Method:      http.MethodPatch,
		Path:        usersPath + "/{id}",
		Summary:     "SCIM user を patch する",
		Tags:        []string{"scim"},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *SCIMPatchInput) (*SCIMUserOutput, error) {
		if deps.ProvisioningService == nil {
			return nil, huma.Error503ServiceUnavailable("scim provisioning is not configured")
		}
		existing, err := deps.ProvisioningService.GetUser(ctx, input.ID)
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		next := SCIMUserBody{
			ExternalID:  existing.ExternalID,
			UserName:    existing.UserName,
			DisplayName: existing.DisplayName,
			Active:      &existing.Active,
		}
		for _, op := range input.Body.Operations {
			if err := applySCIMPatch(&next, op); err != nil {
				return nil, toSCIMHTTPError(err)
			}
		}
		user, err := deps.ProvisioningService.UpsertUser(ctx, provisionedInputFromSCIM(next))
		if err != nil {
			return nil, toSCIMHTTPError(err)
		}
		return &SCIMUserOutput{Body: scimUserFromProvisioned(user)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "scimDeleteUser",
		Method:        http.MethodDelete,
		Path:          usersPath + "/{id}",
		Summary:       "SCIM user を deactivate する",
		Tags:          []string{"scim"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *SCIMUserByIDInput) (*SCIMDeleteUserOutput, error) {
		if deps.ProvisioningService == nil {
			return nil, huma.Error503ServiceUnavailable("scim provisioning is not configured")
		}
		if _, err := deps.ProvisioningService.DeactivateUser(ctx, input.ID); err != nil {
			return nil, toSCIMHTTPError(err)
		}
		return &SCIMDeleteUserOutput{}, nil
	})
}

func provisionedInputFromSCIM(body SCIMUserBody) service.ProvisionedUserInput {
	active := true
	if body.Active != nil {
		active = *body.Active
	}
	groups := make([]string, 0, len(body.Groups))
	for _, group := range body.Groups {
		if strings.TrimSpace(group.Value) != "" {
			groups = append(groups, strings.TrimSpace(group.Value))
		}
	}
	if body.Groups == nil {
		groups = nil
	}
	return service.ProvisionedUserInput{
		ExternalID:  body.ExternalID,
		UserName:    body.UserName,
		DisplayName: body.DisplayName,
		Active:      active,
		Groups:      groups,
	}
}

func scimUserFromProvisioned(user service.ProvisionedUser) SCIMUserBody {
	active := user.Active
	return SCIMUserBody{
		Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		ID:          user.PublicID,
		ExternalID:  user.ExternalID,
		UserName:    user.UserName,
		DisplayName: user.DisplayName,
		Active:      &active,
	}
}

func scimList(startIndex int32, resources []SCIMUserBody) SCIMListResponseBody {
	if startIndex < 1 {
		startIndex = 1
	}
	return SCIMListResponseBody{
		Schemas:      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		TotalResults: len(resources),
		StartIndex:   startIndex,
		ItemsPerPage: len(resources),
		Resources:    resources,
	}
}

func applySCIMPatch(body *SCIMUserBody, op SCIMPatchOperation) error {
	if strings.ToLower(strings.TrimSpace(op.Op)) != "replace" {
		return service.ErrInvalidSCIMUser
	}
	path := strings.ToLower(strings.TrimSpace(op.Path))
	switch path {
	case "active":
		var active bool
		if err := json.Unmarshal(op.Value, &active); err != nil {
			return service.ErrInvalidSCIMUser
		}
		body.Active = &active
	case "displayname":
		var value string
		if err := json.Unmarshal(op.Value, &value); err != nil {
			return service.ErrInvalidSCIMUser
		}
		body.DisplayName = value
	case "username":
		var value string
		if err := json.Unmarshal(op.Value, &value); err != nil {
			return service.ErrInvalidSCIMUser
		}
		body.UserName = value
	case "groups":
		var groups []SCIMGroupBody
		if err := json.Unmarshal(op.Value, &groups); err != nil {
			return service.ErrInvalidSCIMUser
		}
		body.Groups = groups
	default:
		return service.ErrInvalidSCIMUser
	}
	return nil
}

func externalIDFromFilter(filter string) string {
	match := externalIDFilterRE.FindStringSubmatch(filter)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func toSCIMHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidSCIMUser):
		return huma.Error400BadRequest("invalid scim user")
	case errors.Is(err, service.ErrUnauthorized):
		return huma.Error404NotFound("scim user not found")
	default:
		return huma.Error500InternalServerError(fmt.Sprintf("scim operation failed: %v", err))
	}
}
