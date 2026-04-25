package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	db "example.com/haohao/backend/internal/db"

	"github.com/jackc/pgx/v5"
)

var (
	ErrEntitlementNotFound     = errors.New("entitlement not found")
	ErrInvalidEntitlementInput = errors.New("invalid entitlement input")
)

type Entitlement struct {
	FeatureCode string
	DisplayName string
	Description string
	Enabled     bool
	LimitValue  map[string]any
	Source      string
	UpdatedAt   time.Time
}

type EntitlementUpdateInput struct {
	FeatureCode string
	Enabled     bool
	LimitValue  map[string]any
}

type EntitlementService struct {
	queries *db.Queries
	audit   AuditRecorder
}

func NewEntitlementService(queries *db.Queries, audit AuditRecorder) *EntitlementService {
	return &EntitlementService{queries: queries, audit: audit}
}

func (s *EntitlementService) List(ctx context.Context, tenantID int64) ([]Entitlement, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("entitlement service is not configured")
	}
	rows, err := s.queries.ListTenantEntitlements(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list entitlements: %w", err)
	}
	items := make([]Entitlement, 0, len(rows))
	for _, row := range rows {
		items = append(items, entitlementFromListRow(row))
	}
	return items, nil
}

func (s *EntitlementService) IsEnabled(ctx context.Context, tenantID int64, featureCode string) (bool, error) {
	if s == nil || s.queries == nil {
		return false, nil
	}
	return s.IsEnabledWithQueries(ctx, s.queries, tenantID, featureCode)
}

func (s *EntitlementService) IsEnabledWithQueries(ctx context.Context, queries *db.Queries, tenantID int64, featureCode string) (bool, error) {
	if queries == nil {
		return false, nil
	}
	code := normalizeFeatureCode(featureCode)
	if code == "" {
		return false, ErrInvalidEntitlementInput
	}
	row, err := queries.GetTenantEntitlement(ctx, db.GetTenantEntitlementParams{
		TenantID: tenantID,
		Code:     code,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrEntitlementNotFound
	}
	if err != nil {
		return false, fmt.Errorf("get entitlement: %w", err)
	}
	return row.Enabled, nil
}

func (s *EntitlementService) Update(ctx context.Context, tenantID int64, inputs []EntitlementUpdateInput, auditCtx AuditContext) ([]Entitlement, error) {
	if s == nil || s.queries == nil {
		return nil, fmt.Errorf("entitlement service is not configured")
	}
	if len(inputs) == 0 {
		return s.List(ctx, tenantID)
	}
	changed := make([]string, 0, len(inputs))
	for _, input := range inputs {
		code := normalizeFeatureCode(input.FeatureCode)
		if code == "" {
			return nil, ErrInvalidEntitlementInput
		}
		limitValue := input.LimitValue
		if limitValue == nil {
			limitValue = map[string]any{}
		}
		payload, err := json.Marshal(limitValue)
		if err != nil {
			return nil, ErrInvalidEntitlementInput
		}
		if _, err := s.queries.UpsertTenantEntitlement(ctx, db.UpsertTenantEntitlementParams{
			TenantID:    tenantID,
			FeatureCode: code,
			Enabled:     input.Enabled,
			LimitValue:  payload,
			Source:      "manual",
		}); err != nil {
			return nil, fmt.Errorf("upsert entitlement: %w", err)
		}
		changed = append(changed, code)
	}
	if s.audit != nil {
		auditCtx.TenantID = &tenantID
		s.audit.RecordBestEffort(ctx, AuditEventInput{
			AuditContext: auditCtx,
			Action:       "tenant_entitlement.update",
			TargetType:   "tenant",
			TargetID:     fmt.Sprintf("%d", tenantID),
			Metadata: map[string]any{
				"featureCodes": changed,
			},
		})
	}
	return s.List(ctx, tenantID)
}

func normalizeFeatureCode(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func entitlementFromListRow(row db.ListTenantEntitlementsRow) Entitlement {
	return Entitlement{
		FeatureCode: row.Code,
		DisplayName: row.DisplayName,
		Description: row.Description,
		Enabled:     row.Enabled,
		LimitValue:  jsonObject(row.LimitValue),
		Source:      row.Source,
		UpdatedAt:   timestamptzTime(row.UpdatedAt),
	}
}

func jsonObject(payload []byte) map[string]any {
	if len(payload) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}
