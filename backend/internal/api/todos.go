package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"example.com/haohao/backend/internal/service"

	"github.com/danielgtaylor/huma/v2"
)

type TodoBody struct {
	PublicID  string    `json:"publicId" format:"uuid" example:"018f2f05-c6c9-7a49-b32d-04f4dd84ef4a"`
	Title     string    `json:"title" example:"Follow up with customer"`
	Completed bool      `json:"completed" example:"false"`
	CreatedAt time.Time `json:"createdAt" format:"date-time"`
	UpdatedAt time.Time `json:"updatedAt" format:"date-time"`
}

type TodoListBody struct {
	Items []TodoBody `json:"items"`
}

type ListTodosInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
}

type TodoListOutput struct {
	Body TodoListBody
}

type CreateTodoBody struct {
	Title string `json:"title" maxLength:"200" example:"Follow up with customer"`
}

type CreateTodoInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	Body          CreateTodoBody
}

type TodoOutput struct {
	Body TodoBody
}

type UpdateTodoBody struct {
	Title     *string `json:"title,omitempty" maxLength:"200" example:"Follow up with customer"`
	Completed *bool   `json:"completed,omitempty" example:"true"`
}

type UpdateTodoInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TodoPublicID  string      `path:"todoPublicId"`
	Body          UpdateTodoBody
}

type DeleteTodoInput struct {
	SessionCookie http.Cookie `cookie:"SESSION_ID"`
	CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
	TodoPublicID  string      `path:"todoPublicId"`
}

type DeleteTodoOutput struct{}

func registerTodoRoutes(api huma.API, deps Dependencies) {
	huma.Register(api, huma.Operation{
		OperationID: "listTodos",
		Method:      http.MethodGet,
		Path:        "/api/v1/todos",
		Summary:     "active tenant の TODO 一覧を返す",
		Tags:        []string{"todos"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *ListTodosInput) (*TodoListOutput, error) {
		_, tenant, err := requireTodoTenant(ctx, deps, input.SessionCookie.Value, "")
		if err != nil {
			return nil, err
		}

		items, err := deps.TodoService.List(ctx, tenant.ID)
		if err != nil {
			return nil, toTodoHTTPError(err)
		}

		out := &TodoListOutput{}
		out.Body.Items = make([]TodoBody, 0, len(items))
		for _, item := range items {
			out.Body.Items = append(out.Body.Items, toTodoBody(item))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "createTodo",
		Method:      http.MethodPost,
		Path:        "/api/v1/todos",
		Summary:     "active tenant に TODO を作成する",
		Tags:        []string{"todos"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *CreateTodoInput) (*TodoOutput, error) {
		current, tenant, err := requireTodoTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}

		item, err := deps.TodoService.Create(ctx, tenant.ID, current.User.ID, input.Body.Title, userAuditContext(ctx, current.User.ID, &tenant.ID))
		if err != nil {
			return nil, toTodoHTTPError(err)
		}
		return &TodoOutput{Body: toTodoBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "updateTodo",
		Method:      http.MethodPatch,
		Path:        "/api/v1/todos/{todoPublicId}",
		Summary:     "active tenant の TODO を更新する",
		Tags:        []string{"todos"},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *UpdateTodoInput) (*TodoOutput, error) {
		current, tenant, err := requireTodoTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}

		item, err := deps.TodoService.Update(ctx, tenant.ID, input.TodoPublicID, service.TodoUpdateInput{
			Title:     input.Body.Title,
			Completed: input.Body.Completed,
		}, userAuditContext(ctx, current.User.ID, &tenant.ID))
		if err != nil {
			return nil, toTodoHTTPError(err)
		}
		return &TodoOutput{Body: toTodoBody(item)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteTodo",
		Method:        http.MethodDelete,
		Path:          "/api/v1/todos/{todoPublicId}",
		Summary:       "active tenant の TODO を削除する",
		Tags:          []string{"todos"},
		DefaultStatus: http.StatusNoContent,
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, func(ctx context.Context, input *DeleteTodoInput) (*DeleteTodoOutput, error) {
		current, tenant, err := requireTodoTenant(ctx, deps, input.SessionCookie.Value, input.CSRFToken)
		if err != nil {
			return nil, err
		}

		if err := deps.TodoService.Delete(ctx, tenant.ID, input.TodoPublicID, userAuditContext(ctx, current.User.ID, &tenant.ID)); err != nil {
			return nil, toTodoHTTPError(err)
		}
		return &DeleteTodoOutput{}, nil
	})
}

func requireTodoTenant(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, service.TenantAccess, error) {
	if deps.TodoService == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error503ServiceUnavailable("todo service is not configured")
	}

	var current service.CurrentSession
	var authCtx service.AuthContext
	var err error
	if csrfToken == "" {
		current, authCtx, err = currentSessionAuthContext(ctx, deps, sessionID)
	} else {
		current, authCtx, err = currentSessionAuthContextWithCSRF(ctx, deps, sessionID, csrfToken)
	}
	if err != nil {
		var statusErr huma.StatusError
		if errors.As(err, &statusErr) {
			return service.CurrentSession{}, service.TenantAccess{}, err
		}
		return service.CurrentSession{}, service.TenantAccess{}, toHTTPError(err)
	}
	if authCtx.ActiveTenant == nil {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error409Conflict("active tenant is required")
	}
	if !tenantHasRole(*authCtx.ActiveTenant, "todo_user") {
		return service.CurrentSession{}, service.TenantAccess{}, huma.Error403Forbidden("todo_user tenant role is required")
	}
	return current, *authCtx.ActiveTenant, nil
}

func tenantHasRole(tenant service.TenantAccess, role string) bool {
	needle := strings.ToLower(strings.TrimSpace(role))
	if needle == "" {
		return true
	}
	for _, item := range tenant.Roles {
		if strings.ToLower(strings.TrimSpace(item)) == needle {
			return true
		}
	}
	return false
}

func toTodoBody(item service.Todo) TodoBody {
	return TodoBody{
		PublicID:  item.PublicID,
		Title:     item.Title,
		Completed: item.Completed,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func toTodoHTTPError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidTodoTitle):
		return huma.Error400BadRequest("invalid todo title")
	case errors.Is(err, service.ErrInvalidTodoUpdate):
		return huma.Error400BadRequest("invalid todo update")
	case errors.Is(err, service.ErrTodoNotFound):
		return huma.Error404NotFound("todo not found")
	default:
		return toHTTPError(err)
	}
}
