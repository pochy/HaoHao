package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidTodoTitle  = errors.New("invalid todo title")
	ErrInvalidTodoUpdate = errors.New("invalid todo update")
	ErrTodoNotFound      = errors.New("todo not found")
)

const maxTodoTitleLength = 200

type Todo struct {
	PublicID  string
	Title     string
	Completed bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TodoUpdateInput struct {
	Title     *string
	Completed *bool
}

type TodoService struct {
	queries *db.Queries
}

func NewTodoService(queries *db.Queries) *TodoService {
	return &TodoService{queries: queries}
}

func (s *TodoService) List(ctx context.Context, tenantID int64) ([]Todo, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("todo service is not configured")
	}

	rows, err := s.queries.ListTodosByTenantID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}

	items := make([]Todo, 0, len(rows))
	for _, row := range rows {
		items = append(items, todoFromDB(row))
	}
	return items, nil
}

func (s *TodoService) Create(ctx context.Context, tenantID, userID int64, title string) (Todo, error) {
	if s == nil || s.queries == nil {
		return Todo{}, fmt.Errorf("todo service is not configured")
	}

	normalizedTitle, err := normalizeTodoTitle(title)
	if err != nil {
		return Todo{}, err
	}

	row, err := s.queries.CreateTodo(ctx, db.CreateTodoParams{
		TenantID:        tenantID,
		CreatedByUserID: userID,
		Title:           normalizedTitle,
	})
	if err != nil {
		return Todo{}, fmt.Errorf("create todo: %w", err)
	}
	return todoFromDB(row), nil
}

func (s *TodoService) Update(ctx context.Context, tenantID int64, publicID string, input TodoUpdateInput) (Todo, error) {
	if s == nil || s.queries == nil {
		return Todo{}, fmt.Errorf("todo service is not configured")
	}
	if input.Title == nil && input.Completed == nil {
		return Todo{}, ErrInvalidTodoUpdate
	}

	parsedPublicID, err := parseTodoPublicID(publicID)
	if err != nil {
		return Todo{}, err
	}

	existing, err := s.queries.GetTodoByPublicIDForTenant(ctx, db.GetTodoByPublicIDForTenantParams{
		PublicID: parsedPublicID,
		TenantID: tenantID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Todo{}, ErrTodoNotFound
	}
	if err != nil {
		return Todo{}, fmt.Errorf("get todo before update: %w", err)
	}

	title := existing.Title
	if input.Title != nil {
		title, err = normalizeTodoTitle(*input.Title)
		if err != nil {
			return Todo{}, err
		}
	}

	completed := existing.Completed
	if input.Completed != nil {
		completed = *input.Completed
	}

	row, err := s.queries.UpdateTodoByPublicIDForTenant(ctx, db.UpdateTodoByPublicIDForTenantParams{
		PublicID:  parsedPublicID,
		TenantID:  tenantID,
		Title:     title,
		Completed: completed,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Todo{}, ErrTodoNotFound
	}
	if err != nil {
		return Todo{}, fmt.Errorf("update todo: %w", err)
	}
	return todoFromDB(row), nil
}

func (s *TodoService) Delete(ctx context.Context, tenantID int64, publicID string) error {
	if s == nil || s.queries == nil {
		return fmt.Errorf("todo service is not configured")
	}

	parsedPublicID, err := parseTodoPublicID(publicID)
	if err != nil {
		return err
	}

	affectedRows, err := s.queries.DeleteTodoByPublicIDForTenant(ctx, db.DeleteTodoByPublicIDForTenantParams{
		PublicID: parsedPublicID,
		TenantID: tenantID,
	})
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	if affectedRows == 0 {
		return ErrTodoNotFound
	}
	return nil
}

func normalizeTodoTitle(title string) (string, error) {
	normalized := strings.TrimSpace(title)
	if normalized == "" || len([]rune(normalized)) > maxTodoTitleLength {
		return "", ErrInvalidTodoTitle
	}
	return normalized, nil
}

func parseTodoPublicID(publicID string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(publicID))
	if err != nil {
		return uuid.Nil, ErrTodoNotFound
	}
	return parsed, nil
}

func todoFromDB(row db.Todo) Todo {
	return Todo{
		PublicID:  row.PublicID.String(),
		Title:     row.Title,
		Completed: row.Completed,
		CreatedAt: timestamptzTime(row.CreatedAt),
		UpdatedAt: timestamptzTime(row.UpdatedAt),
	}
}
