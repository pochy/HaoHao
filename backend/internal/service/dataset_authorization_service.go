package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"example.com/haohao/backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	DataResourceDataset      = "dataset"
	DataResourceWorkTable    = "work_table"
	DataResourceDataPipeline = "data_pipeline"
	DataResourceScope        = "data_scope"

	DataActionView              = "can_view"
	DataActionUpdate            = "can_update"
	DataActionDelete            = "can_delete"
	DataActionManagePermissions = "can_manage_permissions"
	DataActionPreview           = "can_preview"
	DataActionQuery             = "can_query"
	DataActionExport            = "can_export"
	DataActionCreateDataset     = "can_create_dataset"
	DataActionCreateWorkTable   = "can_create_work_table"
	DataActionCreatePipeline    = "can_create_pipeline"
	DataActionSaveVersion       = "can_save_version"
	DataActionPublishVersion    = "can_publish_version"
	DataActionRun               = "can_run"
	DataActionManageSchedule    = "can_manage_schedule"

	datasetManagersSystemKey = "dataset_managers"
)

var (
	ErrDataPermissionDenied = errors.New("data resource permission denied")
	ErrDataAuthzUnavailable = errors.New("data resource authorization unavailable")
)

type DatasetPermissionGroup struct {
	ID              int64
	PublicID        string
	TenantID        int64
	Name            string
	Description     string
	SystemKey       string
	CreatedByUserID *int64
	CreatedAt       pgtype.Timestamptz
	UpdatedAt       pgtype.Timestamptz
}

type DatasetPermissionGroupMember struct {
	UserID      int64
	PublicID    string
	Email       string
	DisplayName string
	CreatedAt   pgtype.Timestamptz
}

type DatasetPermissionGrant struct {
	ID                   int64
	ResourceType         string
	ResourcePublicID     string
	SubjectType          string
	SubjectUserID        *int64
	SubjectUserPublicID  string
	SubjectUserEmail     string
	SubjectUserName      string
	SubjectGroupID       *int64
	SubjectGroupPublicID string
	SubjectGroupName     string
	Action               string
	CreatedByUserID      *int64
	CreatedAt            pgtype.Timestamptz
}

type DatasetAuthorizationService struct {
	queries    *db.Queries
	client     OpenFGAClient
	enabled    bool
	failClosed bool
}

func NewDatasetAuthorizationService(queries *db.Queries, client OpenFGAClient, enabled, failClosed bool) *DatasetAuthorizationService {
	return &DatasetAuthorizationService{queries: queries, client: client, enabled: enabled, failClosed: failClosed}
}

func (s *DatasetAuthorizationService) CheckResourceAction(ctx context.Context, tenantID, actorUserID int64, resourceType, resourcePublicID, action string) error {
	if strings.TrimSpace(resourcePublicID) == "" {
		return ErrDataPermissionDenied
	}
	user, err := s.openFGAUserForID(ctx, actorUserID)
	if err != nil {
		return err
	}
	return s.check(ctx, user, action, openFGADataResource(resourceType, resourcePublicID))
}

func (s *DatasetAuthorizationService) CheckScopeAction(ctx context.Context, tenantID, actorUserID int64, action string) error {
	user, err := s.openFGAUserForID(ctx, actorUserID)
	if err != nil {
		return err
	}
	if err := s.EnsureScopeManagerTuples(ctx, tenantID, actorUserID); err != nil {
		return err
	}
	scope, err := s.ensureScope(ctx, tenantID)
	if err != nil {
		return err
	}
	return s.check(ctx, user, action, openFGADataScope(scope.PublicID.String()))
}

func (s *DatasetAuthorizationService) FilterResourcePublicIDs(ctx context.Context, actorUserID int64, resourceType, action string, publicIDs []string) (map[string]bool, error) {
	allowed := make(map[string]bool, len(publicIDs))
	if len(publicIDs) == 0 {
		return allowed, nil
	}
	user, err := s.openFGAUserForID(ctx, actorUserID)
	if err != nil {
		return nil, err
	}
	tuples := make([]OpenFGATuple, 0, len(publicIDs))
	index := make([]string, 0, len(publicIDs))
	for _, publicID := range publicIDs {
		if strings.TrimSpace(publicID) == "" {
			continue
		}
		tuples = append(tuples, OpenFGATuple{User: user, Relation: action, Object: openFGADataResource(resourceType, publicID)})
		index = append(index, publicID)
	}
	checks, err := s.batchCheck(ctx, tuples)
	if err != nil {
		return nil, err
	}
	for i, ok := range checks {
		if ok {
			allowed[index[i]] = true
		}
	}
	return allowed, nil
}

func (s *DatasetAuthorizationService) EnsureResourceOwnerTuples(ctx context.Context, tenantID, ownerUserID int64, resourceType, resourcePublicID string) error {
	scope, err := s.ensureScope(ctx, tenantID)
	if err != nil {
		return err
	}
	managers, err := s.ensureManagersGroup(ctx, tenantID, ownerUserID)
	if err != nil {
		return err
	}
	tuples := []OpenFGATuple{
		{User: openFGADataScope(scope.PublicID.String()), Relation: "scope", Object: openFGADataResource(resourceType, resourcePublicID)},
		{User: openFGADataGroupMember(managers.PublicID.String()), Relation: "owner", Object: openFGADataResource(resourceType, resourcePublicID)},
	}
	if ownerUserID > 0 {
		user, err := s.openFGAUserForID(ctx, ownerUserID)
		if err != nil {
			return err
		}
		tuples = append(tuples, OpenFGATuple{User: user, Relation: "owner", Object: openFGADataResource(resourceType, resourcePublicID)})
	}
	return s.writeTuples(ctx, tuples)
}

func (s *DatasetAuthorizationService) EnsureScopeManagerTuples(ctx context.Context, tenantID, actorUserID int64) error {
	scope, err := s.ensureScope(ctx, tenantID)
	if err != nil {
		return err
	}
	managers, err := s.ensureManagersGroup(ctx, tenantID, actorUserID)
	if err != nil {
		return err
	}
	if err := s.queries.AddTenantAdminsToDatasetManagersGroup(ctx, db.AddTenantAdminsToDatasetManagersGroupParams{
		GroupID:       managers.ID,
		TenantID:      tenantID,
		AddedByUserID: pgtype.Int8{Int64: actorUserID, Valid: actorUserID > 0},
	}); err != nil {
		return fmt.Errorf("add tenant admins to dataset managers group: %w", err)
	}
	members, err := s.listGroupMembers(ctx, managers.ID)
	if err != nil {
		return err
	}
	return s.writeTuples(ctx, dataScopeManagerTuples(scope.PublicID.String(), managers.PublicID.String(), members))
}

func (s *DatasetAuthorizationService) RepairTenantTuples(ctx context.Context, tenantID, actorUserID int64) error {
	if err := s.EnsureScopeManagerTuples(ctx, tenantID, actorUserID); err != nil {
		return err
	}
	rows, err := s.queries.ListTenantDataResourcesForPermissionBackfill(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("list tenant data resources for permission backfill: %w", err)
	}
	for _, row := range rows {
		ownerID := actorUserID
		if row.CreatedByUserID.Valid {
			ownerID = row.CreatedByUserID.Int64
		}
		if err := s.EnsureResourceOwnerTuples(ctx, tenantID, ownerID, row.ResourceType, row.PublicID.String()); err != nil {
			return err
		}
	}
	return nil
}

func (s *DatasetAuthorizationService) ListGroups(ctx context.Context, tenantID int64, limit int32) ([]DatasetPermissionGroup, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if err := s.RepairTenantTuples(ctx, tenantID, 0); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListDatasetPermissionGroups(ctx, db.ListDatasetPermissionGroupsParams{TenantID: tenantID, LimitCount: limit})
	if err != nil {
		return nil, fmt.Errorf("list dataset permission groups: %w", err)
	}
	items := make([]DatasetPermissionGroup, 0, len(rows))
	for _, row := range rows {
		items = append(items, datasetPermissionGroupFromDB(row))
	}
	return items, nil
}

func (s *DatasetAuthorizationService) CreateGroup(ctx context.Context, tenantID, actorUserID int64, name, description string) (DatasetPermissionGroup, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return DatasetPermissionGroup{}, ErrInvalidDatasetInput
	}
	row, err := s.queries.CreateDatasetPermissionGroup(ctx, db.CreateDatasetPermissionGroupParams{
		TenantID:        tenantID,
		Name:            name,
		Description:     description,
		CreatedByUserID: pgtype.Int8{Int64: actorUserID, Valid: actorUserID > 0},
	})
	if err != nil {
		return DatasetPermissionGroup{}, fmt.Errorf("create dataset permission group: %w", err)
	}
	return datasetPermissionGroupFromDB(row), nil
}

func (s *DatasetAuthorizationService) GetGroup(ctx context.Context, tenantID int64, groupPublicID string) (DatasetPermissionGroup, []DatasetPermissionGroupMember, error) {
	group, err := s.getGroupRow(ctx, tenantID, groupPublicID)
	if err != nil {
		return DatasetPermissionGroup{}, nil, err
	}
	members, err := s.listGroupMembers(ctx, group.ID)
	if err != nil {
		return DatasetPermissionGroup{}, nil, err
	}
	return datasetPermissionGroupFromDB(group), members, nil
}

func (s *DatasetAuthorizationService) UpdateGroup(ctx context.Context, tenantID int64, groupPublicID, name, description string) (DatasetPermissionGroup, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(groupPublicID))
	if err != nil {
		return DatasetPermissionGroup{}, ErrDatasetNotFound
	}
	row, err := s.queries.UpdateDatasetPermissionGroup(ctx, db.UpdateDatasetPermissionGroupParams{
		TenantID: tenantID, PublicID: parsed, Name: strings.TrimSpace(name), Description: strings.TrimSpace(description),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DatasetPermissionGroup{}, ErrDatasetNotFound
	}
	if err != nil {
		return DatasetPermissionGroup{}, fmt.Errorf("update dataset permission group: %w", err)
	}
	return datasetPermissionGroupFromDB(row), nil
}

func (s *DatasetAuthorizationService) AddGroupMember(ctx context.Context, tenantID, actorUserID int64, groupPublicID, userPublicID string) error {
	group, err := s.getGroupRow(ctx, tenantID, groupPublicID)
	if err != nil {
		return err
	}
	parsedUserID, err := uuid.Parse(strings.TrimSpace(userPublicID))
	if err != nil {
		return ErrInvalidDatasetInput
	}
	user, err := s.queries.GetUserByPublicID(ctx, parsedUserID)
	if err != nil {
		return ErrDatasetNotFound
	}
	if _, err := s.queries.AddDatasetPermissionGroupMember(ctx, db.AddDatasetPermissionGroupMemberParams{
		GroupID: group.ID, UserID: user.ID, AddedByUserID: pgtype.Int8{Int64: actorUserID, Valid: actorUserID > 0},
	}); err != nil {
		return fmt.Errorf("add dataset permission group member: %w", err)
	}
	return s.writeTuples(ctx, []OpenFGATuple{{User: openFGAUser(user.PublicID.String()), Relation: "member", Object: openFGADataGroup(group.PublicID.String())}})
}

func (s *DatasetAuthorizationService) RemoveGroupMember(ctx context.Context, tenantID int64, groupPublicID, userPublicID string) error {
	group, err := s.getGroupRow(ctx, tenantID, groupPublicID)
	if err != nil {
		return err
	}
	parsedUserID, err := uuid.Parse(strings.TrimSpace(userPublicID))
	if err != nil {
		return ErrInvalidDatasetInput
	}
	user, err := s.queries.GetUserByPublicID(ctx, parsedUserID)
	if err != nil {
		return ErrDatasetNotFound
	}
	if err := s.deleteTuples(ctx, []OpenFGATuple{{User: openFGAUser(user.PublicID.String()), Relation: "member", Object: openFGADataGroup(group.PublicID.String())}}); err != nil {
		return err
	}
	if _, err := s.queries.RemoveDatasetPermissionGroupMember(ctx, db.RemoveDatasetPermissionGroupMemberParams{GroupID: group.ID, UserID: user.ID}); errors.Is(err, pgx.ErrNoRows) {
		return ErrDatasetNotFound
	} else if err != nil {
		return fmt.Errorf("remove dataset permission group member: %w", err)
	}
	return nil
}

func (s *DatasetAuthorizationService) ListPermissionGrants(ctx context.Context, tenantID int64, resourceType, resourcePublicID string) ([]DatasetPermissionGrant, error) {
	if err := validateDataResourceTarget(resourceType, resourcePublicID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListDatasetPermissionGrants(ctx, db.ListDatasetPermissionGrantsParams{
		TenantID:         tenantID,
		ResourceType:     resourceType,
		ResourcePublicID: pgDataPermissionUUID(resourcePublicID, resourceType != DataResourceScope),
	})
	if err != nil {
		return nil, fmt.Errorf("list dataset permission grants: %w", err)
	}
	items := make([]DatasetPermissionGrant, 0, len(rows))
	for _, row := range rows {
		items = append(items, datasetPermissionGrantFromDB(row))
	}
	return items, nil
}

func (s *DatasetAuthorizationService) PutPermissionGrants(ctx context.Context, tenantID, actorUserID int64, resourceType, resourcePublicID, subjectType, subjectPublicID string, actions []string) error {
	if err := validateDataResourceTarget(resourceType, resourcePublicID); err != nil {
		return err
	}
	user, subjectUserID, subjectGroupID, err := s.openFGASubjectWithIDs(ctx, tenantID, subjectType, subjectPublicID)
	if err != nil {
		return err
	}
	object := openFGADataResource(resourceType, resourcePublicID)
	if resourceType == DataResourceScope {
		scope, err := s.ensureScope(ctx, tenantID)
		if err != nil {
			return err
		}
		object = openFGADataScope(scope.PublicID.String())
	}
	existing, err := s.ListPermissionGrants(ctx, tenantID, resourceType, resourcePublicID)
	if err != nil {
		return err
	}
	deleteTuples := make([]OpenFGATuple, 0)
	for _, grant := range existing {
		if grant.SubjectType != strings.TrimSpace(subjectType) {
			continue
		}
		if subjectUserID.Valid && (grant.SubjectUserID == nil || *grant.SubjectUserID != subjectUserID.Int64) {
			continue
		}
		if subjectGroupID.Valid && (grant.SubjectGroupID == nil || *grant.SubjectGroupID != subjectGroupID.Int64) {
			continue
		}
		relations, err := relationsForDataAction(resourceType, grant.Action)
		if err != nil {
			return err
		}
		for _, relation := range relations {
			deleteTuples = append(deleteTuples, OpenFGATuple{User: user, Relation: relation, Object: object})
		}
	}
	if err := s.deleteTuples(ctx, deleteTuples); err != nil {
		return err
	}
	if err := s.queries.RevokeDatasetPermissionGrantsForSubject(ctx, db.RevokeDatasetPermissionGrantsForSubjectParams{
		TenantID: tenantID, ResourceType: resourceType, ResourcePublicID: pgDataPermissionUUID(resourcePublicID, resourceType != DataResourceScope), SubjectType: subjectType, SubjectUserID: subjectUserID, SubjectGroupID: subjectGroupID,
	}); err != nil {
		return fmt.Errorf("revoke dataset permission grants: %w", err)
	}
	tuples := make([]OpenFGATuple, 0, len(actions))
	for _, action := range actions {
		relations, err := relationsForDataAction(resourceType, action)
		if err != nil {
			return err
		}
		if _, err := s.queries.CreateDatasetPermissionGrant(ctx, db.CreateDatasetPermissionGrantParams{
			TenantID: tenantID, ResourceType: resourceType, ResourcePublicID: pgDataPermissionUUID(resourcePublicID, resourceType != DataResourceScope), SubjectType: subjectType, SubjectUserID: subjectUserID, SubjectGroupID: subjectGroupID, Action: action, CreatedByUserID: pgtype.Int8{Int64: actorUserID, Valid: actorUserID > 0},
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("create dataset permission grant: %w", err)
		}
		for _, relation := range relations {
			tuples = append(tuples, OpenFGATuple{User: user, Relation: relation, Object: object})
		}
	}
	return s.writeTuples(ctx, tuples)
}

func (s *DatasetAuthorizationService) WritePermissionTuples(ctx context.Context, tenantID int64, resourceType, resourcePublicID, subjectType, subjectPublicID string, actions []string) error {
	return s.PutPermissionGrants(ctx, tenantID, 0, resourceType, resourcePublicID, subjectType, subjectPublicID, actions)
}

func (s *DatasetAuthorizationService) ensureScope(ctx context.Context, tenantID int64) (db.TenantDataAccessScope, error) {
	if s == nil || s.queries == nil {
		return db.TenantDataAccessScope{}, fmt.Errorf("dataset authorization service is not configured")
	}
	scope, err := s.queries.EnsureTenantDataAccessScope(ctx, tenantID)
	if err != nil {
		return db.TenantDataAccessScope{}, fmt.Errorf("ensure tenant data access scope: %w", err)
	}
	return scope, nil
}

func (s *DatasetAuthorizationService) ensureManagersGroup(ctx context.Context, tenantID, actorUserID int64) (db.DatasetPermissionGroup, error) {
	group, err := s.queries.EnsureDatasetManagersGroup(ctx, db.EnsureDatasetManagersGroupParams{
		TenantID: tenantID, CreatedByUserID: pgtype.Int8{Int64: actorUserID, Valid: actorUserID > 0},
	})
	if err != nil {
		return db.DatasetPermissionGroup{}, fmt.Errorf("ensure dataset managers group: %w", err)
	}
	return group, nil
}

func (s *DatasetAuthorizationService) getGroupRow(ctx context.Context, tenantID int64, groupPublicID string) (db.DatasetPermissionGroup, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(groupPublicID))
	if err != nil {
		return db.DatasetPermissionGroup{}, ErrDatasetNotFound
	}
	group, err := s.queries.GetDatasetPermissionGroupByPublicIDForTenant(ctx, db.GetDatasetPermissionGroupByPublicIDForTenantParams{TenantID: tenantID, PublicID: parsed})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.DatasetPermissionGroup{}, ErrDatasetNotFound
	}
	if err != nil {
		return db.DatasetPermissionGroup{}, fmt.Errorf("get dataset permission group: %w", err)
	}
	return group, nil
}

func (s *DatasetAuthorizationService) listGroupMembers(ctx context.Context, groupID int64) ([]DatasetPermissionGroupMember, error) {
	rows, err := s.queries.ListDatasetPermissionGroupMembers(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("list dataset permission group members: %w", err)
	}
	items := make([]DatasetPermissionGroupMember, 0, len(rows))
	for _, row := range rows {
		items = append(items, DatasetPermissionGroupMember{UserID: row.UserID, PublicID: row.UserPublicID.String(), Email: row.Email, DisplayName: row.DisplayName, CreatedAt: row.CreatedAt})
	}
	return items, nil
}

func (s *DatasetAuthorizationService) openFGAUserForID(ctx context.Context, userID int64) (string, error) {
	if userID <= 0 || s == nil || s.queries == nil {
		return "", ErrDataPermissionDenied
	}
	user, err := s.queries.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrDataPermissionDenied
	}
	if err != nil {
		return "", fmt.Errorf("load data permission actor: %w", err)
	}
	return openFGAUser(user.PublicID.String()), nil
}

func (s *DatasetAuthorizationService) openFGASubject(ctx context.Context, tenantID int64, subjectType, subjectPublicID string) (string, error) {
	subject, _, _, err := s.openFGASubjectWithIDs(ctx, tenantID, subjectType, subjectPublicID)
	return subject, err
}

func (s *DatasetAuthorizationService) openFGASubjectWithIDs(ctx context.Context, tenantID int64, subjectType, subjectPublicID string) (string, pgtype.Int8, pgtype.Int8, error) {
	switch strings.TrimSpace(subjectType) {
	case "user":
		parsed, err := uuid.Parse(strings.TrimSpace(subjectPublicID))
		if err != nil {
			return "", pgtype.Int8{}, pgtype.Int8{}, ErrInvalidDatasetInput
		}
		user, err := s.queries.GetUserByPublicID(ctx, parsed)
		if err != nil || user.ID <= 0 {
			return "", pgtype.Int8{}, pgtype.Int8{}, ErrDatasetNotFound
		}
		return openFGAUser(user.PublicID.String()), pgtype.Int8{Int64: user.ID, Valid: true}, pgtype.Int8{}, nil
	case "group":
		group, err := s.getGroupRow(ctx, tenantID, subjectPublicID)
		if err != nil {
			return "", pgtype.Int8{}, pgtype.Int8{}, err
		}
		return openFGADataGroupMember(group.PublicID.String()), pgtype.Int8{}, pgtype.Int8{Int64: group.ID, Valid: true}, nil
	default:
		return "", pgtype.Int8{}, pgtype.Int8{}, ErrInvalidDatasetInput
	}
}

func (s *DatasetAuthorizationService) check(ctx context.Context, user, relation, object string) error {
	if err := s.ensureReady(); err != nil {
		if !s.failClosed {
			return nil
		}
		return err
	}
	allowed, err := s.client.Check(ctx, OpenFGATuple{User: user, Relation: relation, Object: object}, nil)
	if err != nil {
		if s.failClosed {
			return fmt.Errorf("%w: %v", ErrDataAuthzUnavailable, err)
		}
		return nil
	}
	if !allowed {
		return ErrDataPermissionDenied
	}
	return nil
}

func (s *DatasetAuthorizationService) batchCheck(ctx context.Context, tuples []OpenFGATuple) ([]bool, error) {
	if len(tuples) == 0 {
		return nil, nil
	}
	if err := s.ensureReady(); err != nil {
		if !s.failClosed {
			return make([]bool, len(tuples)), nil
		}
		return nil, err
	}
	allowed, err := s.client.BatchCheck(ctx, tuples, nil)
	if err != nil {
		if s.failClosed {
			return nil, fmt.Errorf("%w: %v", ErrDataAuthzUnavailable, err)
		}
		return make([]bool, len(tuples)), nil
	}
	return allowed, nil
}

func (s *DatasetAuthorizationService) writeTuples(ctx context.Context, tuples []OpenFGATuple) error {
	if len(tuples) == 0 {
		return nil
	}
	if err := s.ensureReady(); err != nil {
		if !s.failClosed {
			return nil
		}
		return err
	}
	if err := s.client.WriteTuples(ctx, tuples); err != nil && s.failClosed {
		return fmt.Errorf("%w: %v", ErrDataAuthzUnavailable, err)
	}
	return nil
}

func (s *DatasetAuthorizationService) deleteTuples(ctx context.Context, tuples []OpenFGATuple) error {
	if len(tuples) == 0 {
		return nil
	}
	if err := s.ensureReady(); err != nil {
		if !s.failClosed {
			return nil
		}
		return err
	}
	if err := s.client.DeleteTuples(ctx, tuples); err != nil && s.failClosed {
		return fmt.Errorf("%w: %v", ErrDataAuthzUnavailable, err)
	}
	return nil
}

func (s *DatasetAuthorizationService) ensureReady() error {
	if s == nil || !s.enabled || s.client == nil {
		return ErrDataAuthzUnavailable
	}
	return nil
}

func relationForDataAction(resourceType, action string) (string, error) {
	relations, err := relationsForDataAction(resourceType, action)
	if err != nil {
		return "", err
	}
	if len(relations) != 1 {
		return "", ErrInvalidDatasetInput
	}
	return relations[0], nil
}

func relationsForDataAction(resourceType, action string) ([]string, error) {
	if resourceType == DataResourceScope && action == DataActionView {
		return []string{"dataset_viewer", "work_table_viewer", "pipeline_viewer"}, nil
	}
	switch action {
	case DataActionView:
		return []string{"viewer"}, nil
	case DataActionUpdate, DataActionSaveVersion:
		return []string{"updater"}, nil
	case DataActionDelete:
		return []string{"deleter"}, nil
	case DataActionManagePermissions:
		return []string{"permission_manager"}, nil
	case DataActionPreview:
		return []string{"viewer"}, nil
	case DataActionQuery:
		return []string{"query_runner"}, nil
	case DataActionExport:
		return []string{"exporter"}, nil
	case DataActionCreateDataset:
		return []string{"dataset_creator"}, nil
	case DataActionCreateWorkTable:
		return []string{"work_table_creator"}, nil
	case DataActionCreatePipeline:
		return []string{"pipeline_creator"}, nil
	case DataActionPublishVersion:
		return []string{"publisher"}, nil
	case DataActionRun:
		return []string{"runner"}, nil
	case DataActionManageSchedule:
		return []string{"scheduler"}, nil
	default:
		return nil, ErrInvalidDatasetInput
	}
}

func openFGADataResource(resourceType, publicID string) string {
	return openFGAObject(resourceType, publicID)
}

func openFGADataScope(publicID string) string {
	return openFGAObject(DataResourceScope, publicID)
}

func openFGADataGroup(publicID string) string {
	return openFGAObject("dataset_group", publicID)
}

func openFGADataGroupMember(publicID string) string {
	return openFGADataGroup(publicID) + "#member"
}

func dataScopeManagerTuples(scopePublicID, groupPublicID string, members []DatasetPermissionGroupMember) []OpenFGATuple {
	groupMember := openFGADataGroupMember(groupPublicID)
	scope := openFGADataScope(scopePublicID)
	tuples := []OpenFGATuple{
		{User: groupMember, Relation: "owner", Object: scope},
		{User: groupMember, Relation: "dataset_creator", Object: scope},
		{User: groupMember, Relation: "work_table_creator", Object: scope},
		{User: groupMember, Relation: "pipeline_creator", Object: scope},
	}
	for _, member := range members {
		if strings.TrimSpace(member.PublicID) == "" {
			continue
		}
		tuples = append(tuples, OpenFGATuple{User: openFGAUser(member.PublicID), Relation: "member", Object: openFGADataGroup(groupPublicID)})
	}
	return tuples
}

func validateDataResourceTarget(resourceType, resourcePublicID string) error {
	switch resourceType {
	case DataResourceScope:
		return nil
	case DataResourceDataset, DataResourceWorkTable, DataResourceDataPipeline:
		if _, err := uuid.Parse(strings.TrimSpace(resourcePublicID)); err != nil {
			return ErrInvalidDatasetInput
		}
		return nil
	default:
		return ErrInvalidDatasetInput
	}
}

func pgDataPermissionUUID(value string, valid bool) pgtype.UUID {
	if !valid {
		return pgtype.UUID{}
	}
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

func datasetPermissionGroupFromDB(row db.DatasetPermissionGroup) DatasetPermissionGroup {
	return DatasetPermissionGroup{
		ID:              row.ID,
		PublicID:        row.PublicID.String(),
		TenantID:        row.TenantID,
		Name:            row.Name,
		Description:     row.Description,
		SystemKey:       pgTextString(row.SystemKey),
		CreatedByUserID: optionalPgInt8(row.CreatedByUserID),
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func pgTextString(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func datasetPermissionGrantFromDB(row db.ListDatasetPermissionGrantsRow) DatasetPermissionGrant {
	item := DatasetPermissionGrant{
		ID:                   row.ID,
		ResourceType:         row.ResourceType,
		ResourcePublicID:     dataPermissionUUIDString(row.ResourcePublicID),
		SubjectType:          row.SubjectType,
		SubjectUserID:        optionalPgInt8(row.SubjectUserID),
		SubjectUserPublicID:  dataPermissionUUIDString(row.SubjectUserPublicID),
		SubjectUserEmail:     pgTextString(row.SubjectUserEmail),
		SubjectUserName:      pgTextString(row.SubjectUserDisplayName),
		SubjectGroupID:       optionalPgInt8(row.SubjectGroupID),
		SubjectGroupPublicID: dataPermissionUUIDString(row.SubjectGroupPublicID),
		SubjectGroupName:     pgTextString(row.SubjectGroupName),
		Action:               row.Action,
		CreatedByUserID:      optionalPgInt8(row.CreatedByUserID),
		CreatedAt:            row.CreatedAt,
	}
	return item
}

func dataPermissionUUIDString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}
