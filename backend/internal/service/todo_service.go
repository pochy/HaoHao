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
	"github.com/jackc/pgx/v5/pgxpool"
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
	pool    *pgxpool.Pool
	queries *db.Queries
	audit   AuditRecorder
}

func NewTodoService(pool *pgxpool.Pool, queries *db.Queries, audit AuditRecorder) *TodoService {
	return &TodoService{
		pool:    pool,
		queries: queries,
		audit:   audit,
	}
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

func (s *TodoService) Create(ctx context.Context, tenantID, userID int64, title string, auditCtx AuditContext) (Todo, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return Todo{}, fmt.Errorf("todo service is not configured")
	}
	if s.audit == nil {
		return Todo{}, fmt.Errorf("audit recorder is not configured")
	}

	normalizedTitle, err := normalizeTodoTitle(title)
	if err != nil {
		return Todo{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Todo{}, fmt.Errorf("begin todo create transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	row, err := qtx.CreateTodo(ctx, db.CreateTodoParams{
		TenantID:        tenantID,
		CreatedByUserID: userID,
		Title:           normalizedTitle,
	})
	if err != nil {
		return Todo{}, fmt.Errorf("create todo: %w", err)
	}
	item := todoFromDB(row)

	auditCtx.TenantID = &tenantID
	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "todo.create",
		TargetType:   "todo",
		TargetID:     item.PublicID,
		Metadata: map[string]any{
			"titleLength": len([]rune(normalizedTitle)),
		},
	}); err != nil {
		return Todo{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Todo{}, fmt.Errorf("commit todo create transaction: %w", err)
	}
	return item, nil
}

func (s *TodoService) Update(ctx context.Context, tenantID int64, publicID string, input TodoUpdateInput, auditCtx AuditContext) (Todo, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return Todo{}, fmt.Errorf("todo service is not configured")
	}
	if s.audit == nil {
		return Todo{}, fmt.Errorf("audit recorder is not configured")
	}
	if input.Title == nil && input.Completed == nil {
		return Todo{}, ErrInvalidTodoUpdate
	}

	parsedPublicID, err := parseTodoPublicID(publicID)
	if err != nil {
		return Todo{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Todo{}, fmt.Errorf("begin todo update transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	existing, err := qtx.GetTodoByPublicIDForTenant(ctx, db.GetTodoByPublicIDForTenantParams{
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

	row, err := qtx.UpdateTodoByPublicIDForTenant(ctx, db.UpdateTodoByPublicIDForTenantParams{
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
	item := todoFromDB(row)

	changedFields := make([]string, 0, 2)
	if input.Title != nil {
		changedFields = append(changedFields, "title")
	}
	if input.Completed != nil {
		changedFields = append(changedFields, "completed")
	}

	auditCtx.TenantID = &tenantID
	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "todo.update",
		TargetType:   "todo",
		TargetID:     item.PublicID,
		Metadata: map[string]any{
			"changedFields": changedFields,
		},
	}); err != nil {
		return Todo{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Todo{}, fmt.Errorf("commit todo update transaction: %w", err)
	}
	return item, nil
}

func (s *TodoService) Delete(ctx context.Context, tenantID int64, publicID string, auditCtx AuditContext) error {
	if s == nil || s.pool == nil || s.queries == nil {
		return fmt.Errorf("todo service is not configured")
	}
	if s.audit == nil {
		return fmt.Errorf("audit recorder is not configured")
	}

	parsedPublicID, err := parseTodoPublicID(publicID)
	if err != nil {
		return err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin todo delete transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	qtx := s.queries.WithTx(tx)
	affectedRows, err := qtx.DeleteTodoByPublicIDForTenant(ctx, db.DeleteTodoByPublicIDForTenantParams{
		PublicID: parsedPublicID,
		TenantID: tenantID,
	})
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	if affectedRows == 0 {
		return ErrTodoNotFound
	}

	auditCtx.TenantID = &tenantID
	if err := s.audit.RecordWithQueries(ctx, qtx, AuditEventInput{
		AuditContext: auditCtx,
		Action:       "todo.delete",
		TargetType:   "todo",
		TargetID:     parsedPublicID.String(),
	}); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit todo delete transaction: %w", err)
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
