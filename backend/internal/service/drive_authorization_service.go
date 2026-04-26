package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type DriveAuthorizationConfig struct {
	Enabled    bool
	FailClosed bool
	Now        func() time.Time
	Metrics    DriveAuthzMetrics
}

type DriveAuthorizationService struct {
	client     OpenFGAClient
	enabled    bool
	failClosed bool
	now        func() time.Time
	metrics    DriveAuthzMetrics
}

type DriveAuthzMetrics interface {
	IncDriveAuthzDenied(operation, resourceType string)
}

func NewDriveAuthorizationService(client OpenFGAClient, cfg DriveAuthorizationConfig) *DriveAuthorizationService {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &DriveAuthorizationService{
		client:     client,
		enabled:    cfg.Enabled,
		failClosed: cfg.FailClosed,
		now:        now,
		metrics:    cfg.Metrics,
	}
}

func (s *DriveAuthorizationService) CanViewFile(ctx context.Context, actor DriveActor, file DriveFile) error {
	if err := validateDriveActorResource(actor, file.ResourceRef(), file.DeletedAt); err != nil {
		return err
	}
	return s.checkResource(ctx, openFGAUser(actor.PublicID), "can_view", openFGAFile(file.PublicID), "file", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanDownloadFile(ctx context.Context, actor DriveActor, file DriveFile) error {
	if err := validateDriveActorResource(actor, file.ResourceRef(), file.DeletedAt); err != nil {
		return err
	}
	return s.checkResource(ctx, openFGAUser(actor.PublicID), "can_download", openFGAFile(file.PublicID), "file", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanEditFile(ctx context.Context, actor DriveActor, file DriveFile) error {
	if err := validateDriveActorResource(actor, file.ResourceRef(), file.DeletedAt); err != nil {
		return err
	}
	if file.LockedAt != nil {
		return ErrDriveLocked
	}
	return s.checkResource(ctx, openFGAUser(actor.PublicID), "can_edit", openFGAFile(file.PublicID), "file", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanDeleteFile(ctx context.Context, actor DriveActor, file DriveFile) error {
	if err := validateDriveActorResource(actor, file.ResourceRef(), file.DeletedAt); err != nil {
		return err
	}
	if file.LockedAt != nil {
		return ErrDriveLocked
	}
	return s.checkResource(ctx, openFGAUser(actor.PublicID), "can_delete", openFGAFile(file.PublicID), "file", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanShareFile(ctx context.Context, actor DriveActor, file DriveFile) error {
	if err := validateDriveActorResource(actor, file.ResourceRef(), file.DeletedAt); err != nil {
		return err
	}
	if file.LockedAt != nil {
		return ErrDriveLocked
	}
	return s.checkResource(ctx, openFGAUser(actor.PublicID), "can_share", openFGAFile(file.PublicID), "file", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanViewFolder(ctx context.Context, actor DriveActor, folder DriveFolder) error {
	if err := validateDriveActorResource(actor, folder.ResourceRef(), folder.DeletedAt); err != nil {
		return err
	}
	return s.checkResource(ctx, openFGAUser(actor.PublicID), "can_view", openFGAFolder(folder.PublicID), "folder", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanEditFolder(ctx context.Context, actor DriveActor, folder DriveFolder) error {
	if err := validateDriveActorResource(actor, folder.ResourceRef(), folder.DeletedAt); err != nil {
		return err
	}
	return s.checkResource(ctx, openFGAUser(actor.PublicID), "can_edit", openFGAFolder(folder.PublicID), "folder", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanDeleteFolder(ctx context.Context, actor DriveActor, folder DriveFolder) error {
	if err := validateDriveActorResource(actor, folder.ResourceRef(), folder.DeletedAt); err != nil {
		return err
	}
	return s.checkResource(ctx, openFGAUser(actor.PublicID), "can_delete", openFGAFolder(folder.PublicID), "folder", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanShareFolder(ctx context.Context, actor DriveActor, folder DriveFolder) error {
	if err := validateDriveActorResource(actor, folder.ResourceRef(), folder.DeletedAt); err != nil {
		return err
	}
	return s.checkResource(ctx, openFGAUser(actor.PublicID), "can_share", openFGAFolder(folder.PublicID), "folder", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanViewWithShareLink(ctx context.Context, link DriveShareLink) error {
	if link.Status != "active" || !link.ExpiresAt.After(s.now()) {
		s.recordDenied("can_view", "share_link")
		return ErrDrivePermissionDenied
	}
	return s.checkResource(ctx, openFGAShareLink(link.PublicID), "can_view", openFGAResourceObject(link.Resource), "share_link", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanEditWithShareLink(ctx context.Context, link DriveShareLink) error {
	if link.Status != "active" || !link.ExpiresAt.After(s.now()) || link.Role != DriveRoleEditor {
		s.recordDenied("can_edit", "share_link")
		return ErrDrivePermissionDenied
	}
	return s.checkResource(ctx, openFGAShareLink(link.PublicID), "can_edit", openFGAResourceObject(link.Resource), "share_link", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanViewCleanRoom(ctx context.Context, actor DriveActor, cleanRoomPublicID string) error {
	if actor.UserID <= 0 || actor.TenantID <= 0 || strings.TrimSpace(actor.PublicID) == "" || strings.TrimSpace(cleanRoomPublicID) == "" {
		return ErrDrivePermissionDenied
	}
	return s.checkResource(ctx, openFGAUser(actor.PublicID), "can_view", openFGACleanRoom(cleanRoomPublicID), "clean_room", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanSubmitCleanRoomDataset(ctx context.Context, actor DriveActor, cleanRoomPublicID string) error {
	if actor.UserID <= 0 || actor.TenantID <= 0 || strings.TrimSpace(actor.PublicID) == "" || strings.TrimSpace(cleanRoomPublicID) == "" {
		return ErrDrivePermissionDenied
	}
	return s.checkResource(ctx, openFGAUser(actor.PublicID), "can_submit_dataset", openFGACleanRoom(cleanRoomPublicID), "clean_room", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CanApproveCleanRoomExport(ctx context.Context, actor DriveActor, cleanRoomPublicID string) error {
	if actor.UserID <= 0 || actor.TenantID <= 0 || strings.TrimSpace(actor.PublicID) == "" || strings.TrimSpace(cleanRoomPublicID) == "" {
		return ErrDrivePermissionDenied
	}
	return s.checkResource(ctx, openFGAUser(actor.PublicID), "can_approve_export", openFGACleanRoom(cleanRoomPublicID), "clean_room", s.currentTimeContext())
}

func (s *DriveAuthorizationService) CheckShareTuple(ctx context.Context, share DriveShare) error {
	tuple := shareTuple(share)
	return s.checkResource(ctx, tuple.User, tuple.Relation, tuple.Object, string(share.Resource.Type), s.currentTimeContext())
}

func (s *DriveAuthorizationService) CheckShareLinkTuple(ctx context.Context, link DriveShareLink) error {
	tuple := shareLinkTuple(link)
	return s.checkResource(ctx, tuple.User, tuple.Relation, tuple.Object, "share_link", s.currentTimeContext())
}

func (s *DriveAuthorizationService) FilterViewableFiles(ctx context.Context, actor DriveActor, files []DriveFile) ([]DriveFile, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if actor.UserID <= 0 || actor.TenantID <= 0 || strings.TrimSpace(actor.PublicID) == "" {
		s.recordDenied("can_view", "file")
		return nil, ErrDrivePermissionDenied
	}
	tuples := make([]OpenFGATuple, 0, len(files))
	indexes := make([]int, 0, len(files))
	for i, file := range files {
		if validateDriveActorResource(actor, file.ResourceRef(), file.DeletedAt) != nil {
			continue
		}
		tuples = append(tuples, OpenFGATuple{
			User:     openFGAUser(actor.PublicID),
			Relation: "can_view",
			Object:   openFGAFile(file.PublicID),
		})
		indexes = append(indexes, i)
	}
	allowed, err := s.batchCheck(ctx, tuples, s.currentTimeContext())
	if err != nil {
		s.recordDenied("can_view", "file")
		return nil, err
	}
	items := make([]DriveFile, 0, len(files))
	for i, ok := range allowed {
		if ok {
			items = append(items, files[indexes[i]])
		}
	}
	return items, nil
}

func (s *DriveAuthorizationService) FilterViewableFolders(ctx context.Context, actor DriveActor, folders []DriveFolder) ([]DriveFolder, error) {
	if len(folders) == 0 {
		return nil, nil
	}
	if actor.UserID <= 0 || actor.TenantID <= 0 || strings.TrimSpace(actor.PublicID) == "" {
		s.recordDenied("can_view", "folder")
		return nil, ErrDrivePermissionDenied
	}
	tuples := make([]OpenFGATuple, 0, len(folders))
	indexes := make([]int, 0, len(folders))
	for i, folder := range folders {
		if validateDriveActorResource(actor, folder.ResourceRef(), folder.DeletedAt) != nil {
			continue
		}
		tuples = append(tuples, OpenFGATuple{
			User:     openFGAUser(actor.PublicID),
			Relation: "can_view",
			Object:   openFGAFolder(folder.PublicID),
		})
		indexes = append(indexes, i)
	}
	allowed, err := s.batchCheck(ctx, tuples, s.currentTimeContext())
	if err != nil {
		s.recordDenied("can_view", "folder")
		return nil, err
	}
	items := make([]DriveFolder, 0, len(folders))
	for i, ok := range allowed {
		if ok {
			items = append(items, folders[indexes[i]])
		}
	}
	return items, nil
}

func (s *DriveAuthorizationService) ListViewableFiles(ctx context.Context, actor DriveActor) ([]string, error) {
	if actor.TenantID <= 0 || strings.TrimSpace(actor.PublicID) == "" {
		return nil, ErrDrivePermissionDenied
	}
	objects, err := s.listObjects(ctx, openFGAUser(actor.PublicID), "can_view", "file", s.currentTimeContext())
	if err != nil {
		return nil, err
	}
	return stripOpenFGAObjectPrefixes(objects), nil
}

func (s *DriveAuthorizationService) ListViewableFolders(ctx context.Context, actor DriveActor) ([]string, error) {
	if actor.TenantID <= 0 || strings.TrimSpace(actor.PublicID) == "" {
		return nil, ErrDrivePermissionDenied
	}
	objects, err := s.listObjects(ctx, openFGAUser(actor.PublicID), "can_view", "folder", s.currentTimeContext())
	if err != nil {
		return nil, err
	}
	return stripOpenFGAObjectPrefixes(objects), nil
}

func (s *DriveAuthorizationService) WriteResourceOwner(ctx context.Context, actor DriveActor, resource DriveResourceRef) error {
	return s.writeTuples(ctx, []OpenFGATuple{{
		User:     openFGAUser(actor.PublicID),
		Relation: "owner",
		Object:   openFGAResourceObject(resource),
	}})
}

func (s *DriveAuthorizationService) WriteResourceParent(ctx context.Context, child, parent DriveResourceRef) error {
	return s.writeTuples(ctx, []OpenFGATuple{parentTuple(child, parent)})
}

func (s *DriveAuthorizationService) WriteResourceWorkspace(ctx context.Context, resource DriveResourceRef, workspace DriveWorkspace) error {
	return s.writeTuples(ctx, []OpenFGATuple{workspaceTuple(resource, workspace.ResourceRef())})
}

func (s *DriveAuthorizationService) DeleteResourceWorkspace(ctx context.Context, resource DriveResourceRef, workspace DriveWorkspace) error {
	return s.deleteTuples(ctx, []OpenFGATuple{workspaceTuple(resource, workspace.ResourceRef())})
}

func (s *DriveAuthorizationService) DeleteResourceParent(ctx context.Context, child, parent DriveResourceRef) error {
	return s.deleteTuples(ctx, []OpenFGATuple{parentTuple(child, parent)})
}

func (s *DriveAuthorizationService) DeleteResourceOwner(ctx context.Context, actor DriveActor, resource DriveResourceRef) error {
	return s.deleteTuples(ctx, []OpenFGATuple{{
		User:     openFGAUser(actor.PublicID),
		Relation: "owner",
		Object:   openFGAResourceObject(resource),
	}})
}

func (s *DriveAuthorizationService) WriteResourceCreateTuples(ctx context.Context, actor DriveActor, resource DriveResourceRef, parent *DriveResourceRef) error {
	tuples := []OpenFGATuple{{
		User:     openFGAUser(actor.PublicID),
		Relation: "owner",
		Object:   openFGAResourceObject(resource),
	}}
	if parent != nil {
		tuples = append(tuples, parentTuple(resource, *parent))
	}
	return s.writeTuples(ctx, tuples)
}

func (s *DriveAuthorizationService) WriteWorkspaceOwner(ctx context.Context, actor DriveActor, workspace DriveWorkspace) error {
	return s.writeTuples(ctx, []OpenFGATuple{{
		User:     openFGAUser(actor.PublicID),
		Relation: "owner",
		Object:   openFGAWorkspace(workspace.PublicID),
	}})
}

func (s *DriveAuthorizationService) WriteResourceCreateTuplesWithWorkspace(ctx context.Context, actor DriveActor, resource DriveResourceRef, parent *DriveResourceRef, workspace *DriveWorkspace) error {
	tuples := []OpenFGATuple{{
		User:     openFGAUser(actor.PublicID),
		Relation: "owner",
		Object:   openFGAResourceObject(resource),
	}}
	if parent != nil {
		tuples = append(tuples, parentTuple(resource, *parent))
	}
	if workspace != nil {
		tuples = append(tuples, workspaceTuple(resource, workspace.ResourceRef()))
	}
	return s.writeTuples(ctx, tuples)
}

func (s *DriveAuthorizationService) WriteShareTuple(ctx context.Context, share DriveShare) error {
	return s.writeTuples(ctx, []OpenFGATuple{shareTuple(share)})
}

func (s *DriveAuthorizationService) DeleteShareTuple(ctx context.Context, share DriveShare) error {
	return s.deleteTuples(ctx, []OpenFGATuple{shareTuple(share)})
}

func (s *DriveAuthorizationService) WriteGroupMemberTuple(ctx context.Context, group DriveGroup, userPublicID string) error {
	return s.writeTuples(ctx, []OpenFGATuple{{
		User:     openFGAUser(userPublicID),
		Relation: "member",
		Object:   openFGAGroup(group.PublicID),
	}})
}

func (s *DriveAuthorizationService) DeleteGroupMemberTuple(ctx context.Context, group DriveGroup, userPublicID string) error {
	return s.deleteTuples(ctx, []OpenFGATuple{{
		User:     openFGAUser(userPublicID),
		Relation: "member",
		Object:   openFGAGroup(group.PublicID),
	}})
}

func (s *DriveAuthorizationService) WriteShareLinkTuple(ctx context.Context, link DriveShareLink) error {
	return s.writeTuples(ctx, []OpenFGATuple{shareLinkTuple(link)})
}

func (s *DriveAuthorizationService) DeleteShareLinkTuple(ctx context.Context, link DriveShareLink) error {
	return s.deleteTuples(ctx, []OpenFGATuple{shareLinkTuple(link)})
}

func (s *DriveAuthorizationService) WriteCleanRoomTuple(ctx context.Context, cleanRoomPublicID, userPublicID, role string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "owner", "participant", "reviewer":
	default:
		return ErrDriveInvalidInput
	}
	if strings.TrimSpace(cleanRoomPublicID) == "" || strings.TrimSpace(userPublicID) == "" {
		return ErrDriveInvalidInput
	}
	return s.writeTuples(ctx, []OpenFGATuple{{
		User:     openFGAUser(userPublicID),
		Relation: role,
		Object:   openFGACleanRoom(cleanRoomPublicID),
	}})
}

func (s *DriveAuthorizationService) check(ctx context.Context, user, relation, object string, contextMap map[string]any) error {
	if err := s.ensureReady(); err != nil {
		if !s.failClosed {
			return nil
		}
		return err
	}
	allowed, err := s.client.Check(ctx, OpenFGATuple{User: user, Relation: relation, Object: object}, contextMap)
	if err != nil {
		if s.failClosed {
			return fmt.Errorf("%w: %v", ErrDriveAuthzUnavailable, err)
		}
		return nil
	}
	if !allowed {
		return ErrDrivePermissionDenied
	}
	return nil
}

func (s *DriveAuthorizationService) checkResource(ctx context.Context, user, relation, object, resourceType string, contextMap map[string]any) error {
	if err := s.check(ctx, user, relation, object, contextMap); err != nil {
		s.recordDenied(relation, resourceType)
		return err
	}
	return nil
}

func (s *DriveAuthorizationService) batchCheck(ctx context.Context, tuples []OpenFGATuple, contextMap map[string]any) ([]bool, error) {
	if len(tuples) == 0 {
		return nil, nil
	}
	if err := s.ensureReady(); err != nil {
		if !s.failClosed {
			return make([]bool, len(tuples)), nil
		}
		return nil, err
	}
	allowed, err := s.client.BatchCheck(ctx, tuples, contextMap)
	if err != nil {
		if s.failClosed {
			return nil, fmt.Errorf("%w: %v", ErrDriveAuthzUnavailable, err)
		}
		return make([]bool, len(tuples)), nil
	}
	if len(allowed) != len(tuples) {
		resized := make([]bool, len(tuples))
		copy(resized, allowed)
		allowed = resized
	}
	return allowed, nil
}

func (s *DriveAuthorizationService) listObjects(ctx context.Context, user, relation, objectType string, contextMap map[string]any) ([]string, error) {
	if err := s.ensureReady(); err != nil {
		if !s.failClosed {
			return nil, nil
		}
		return nil, err
	}
	objects, err := s.client.ListObjects(ctx, user, relation, objectType, contextMap)
	if err != nil {
		if s.failClosed {
			return nil, fmt.Errorf("%w: %v", ErrDriveAuthzUnavailable, err)
		}
		return nil, nil
	}
	return objects, nil
}

func (s *DriveAuthorizationService) writeTuples(ctx context.Context, tuples []OpenFGATuple) error {
	if err := s.ensureReady(); err != nil {
		if !s.failClosed {
			return nil
		}
		return err
	}
	if err := s.client.WriteTuples(ctx, tuples); err != nil {
		if s.failClosed {
			return fmt.Errorf("%w: %v", ErrDriveAuthzUnavailable, err)
		}
	}
	return nil
}

func (s *DriveAuthorizationService) deleteTuples(ctx context.Context, tuples []OpenFGATuple) error {
	if err := s.ensureReady(); err != nil {
		if !s.failClosed {
			return nil
		}
		return err
	}
	if err := s.client.DeleteTuples(ctx, tuples); err != nil {
		if s.failClosed {
			return fmt.Errorf("%w: %v", ErrDriveAuthzUnavailable, err)
		}
	}
	return nil
}

func (s *DriveAuthorizationService) ensureReady() error {
	if s == nil || !s.enabled || s.client == nil {
		return ErrDriveAuthzUnavailable
	}
	return nil
}

func (s *DriveAuthorizationService) currentTimeContext() map[string]any {
	return map[string]any{
		"current_time": s.now().UTC(),
	}
}

func validateDriveActorResource(actor DriveActor, resource DriveResourceRef, deletedAt *time.Time) error {
	if actor.UserID <= 0 || actor.TenantID <= 0 || strings.TrimSpace(actor.PublicID) == "" {
		return ErrDrivePermissionDenied
	}
	if resource.TenantID <= 0 || actor.TenantID != resource.TenantID {
		return ErrDriveNotFound
	}
	if strings.TrimSpace(resource.PublicID) == "" || deletedAt != nil {
		return ErrDriveNotFound
	}
	return nil
}

func openFGAResourceObject(resource DriveResourceRef) string {
	switch resource.Type {
	case DriveResourceTypeFile:
		return openFGAFile(resource.PublicID)
	case DriveResourceTypeFolder:
		return openFGAFolder(resource.PublicID)
	case DriveResourceTypeCleanRoom:
		return openFGACleanRoom(resource.PublicID)
	default:
		return openFGAObject(string(resource.Type), resource.PublicID)
	}
}

func parentTuple(child, parent DriveResourceRef) OpenFGATuple {
	return OpenFGATuple{
		User:     openFGAResourceObject(parent),
		Relation: "parent",
		Object:   openFGAResourceObject(child),
	}
}

func shareTuple(share DriveShare) OpenFGATuple {
	user := openFGAUser(share.SubjectPublicID)
	if share.SubjectType == DriveShareSubjectGroup {
		user = openFGAGroupMember(share.SubjectPublicID)
	}
	return OpenFGATuple{
		User:     user,
		Relation: string(share.Role),
		Object:   openFGAResourceObject(share.Resource),
	}
}

func shareLinkTuple(link DriveShareLink) OpenFGATuple {
	role := link.Role
	if role == "" {
		role = DriveRoleViewer
	}
	return OpenFGATuple{
		User:     openFGAShareLink(link.PublicID),
		Relation: string(role),
		Object:   openFGAResourceObject(link.Resource),
		Condition: &OpenFGACondition{
			Name: "not_expired",
			Context: map[string]any{
				"expires_at": link.ExpiresAt.UTC(),
			},
		},
	}
}

func workspaceTuple(resource DriveResourceRef, workspace DriveResourceRef) OpenFGATuple {
	return OpenFGATuple{
		User:     openFGAResourceObject(workspace),
		Relation: "workspace",
		Object:   openFGAResourceObject(resource),
	}
}

func stripOpenFGAObjectPrefixes(objects []string) []string {
	result := make([]string, 0, len(objects))
	for _, object := range objects {
		result = append(result, stripOpenFGAObjectPrefix(object))
	}
	return result
}

func isDrivePermissionError(err error) bool {
	return errors.Is(err, ErrDrivePermissionDenied) || errors.Is(err, ErrDriveAuthzUnavailable) || errors.Is(err, ErrDriveLocked)
}

func (s *DriveAuthorizationService) recordDenied(operation, resourceType string) {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.IncDriveAuthzDenied(operation, resourceType)
}
