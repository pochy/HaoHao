package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeOpenFGAClient struct {
	checkAllowed bool
	checkErr     error
	writeErr     error
	deleteErr    error

	checkCalls  []OpenFGATuple
	writeCalls  [][]OpenFGATuple
	deleteCalls [][]OpenFGATuple
	listObjects []string
}

func (f *fakeOpenFGAClient) Check(ctx context.Context, tuple OpenFGATuple, context map[string]any) (bool, error) {
	f.checkCalls = append(f.checkCalls, tuple)
	if f.checkErr != nil {
		return false, f.checkErr
	}
	return f.checkAllowed, nil
}

func (f *fakeOpenFGAClient) BatchCheck(ctx context.Context, tuples []OpenFGATuple, context map[string]any) ([]bool, error) {
	result := make([]bool, len(tuples))
	for i := range result {
		result[i] = f.checkAllowed
	}
	return result, f.checkErr
}

func (f *fakeOpenFGAClient) ListObjects(ctx context.Context, user string, relation string, objectType string, context map[string]any) ([]string, error) {
	return f.listObjects, f.checkErr
}

func (f *fakeOpenFGAClient) WriteTuples(ctx context.Context, tuples []OpenFGATuple) error {
	f.writeCalls = append(f.writeCalls, tuples)
	return f.writeErr
}

func (f *fakeOpenFGAClient) DeleteTuples(ctx context.Context, tuples []OpenFGATuple) error {
	f.deleteCalls = append(f.deleteCalls, tuples)
	return f.deleteErr
}

func TestDriveAuthorizationFailClosedOnCheckError(t *testing.T) {
	client := &fakeOpenFGAClient{checkErr: errors.New("network down")}
	svc := NewDriveAuthorizationService(client, DriveAuthorizationConfig{Enabled: true, FailClosed: true})

	err := svc.CanViewFile(context.Background(), testDriveActor(), testDriveFile(false))
	if !errors.Is(err, ErrDriveAuthzUnavailable) {
		t.Fatalf("CanViewFile() error = %v, want ErrDriveAuthzUnavailable", err)
	}
}

func TestDriveAuthorizationDisabledFailClosed(t *testing.T) {
	svc := NewDriveAuthorizationService(nil, DriveAuthorizationConfig{Enabled: false, FailClosed: true})

	err := svc.CanViewFile(context.Background(), testDriveActor(), testDriveFile(false))
	if !errors.Is(err, ErrDriveAuthzUnavailable) {
		t.Fatalf("CanViewFile() error = %v, want ErrDriveAuthzUnavailable", err)
	}
}

func TestDriveAuthorizationLockedFileRejectsBeforeOpenFGA(t *testing.T) {
	client := &fakeOpenFGAClient{checkAllowed: true}
	svc := NewDriveAuthorizationService(client, DriveAuthorizationConfig{Enabled: true, FailClosed: true})

	err := svc.CanEditFile(context.Background(), testDriveActor(), testDriveFile(true))
	if !errors.Is(err, ErrDriveLocked) {
		t.Fatalf("CanEditFile() error = %v, want ErrDriveLocked", err)
	}
	if len(client.checkCalls) != 0 {
		t.Fatalf("OpenFGA Check calls = %d, want 0", len(client.checkCalls))
	}
}

func TestDriveAuthorizationPlatformAdminBypassesResourceChecks(t *testing.T) {
	client := &fakeOpenFGAClient{checkAllowed: false}
	svc := NewDriveAuthorizationService(client, DriveAuthorizationConfig{Enabled: true, FailClosed: true})
	actor := testDriveActor()
	actor.PlatformAdmin = true

	if err := svc.CanViewFile(context.Background(), actor, testDriveFile(false)); err != nil {
		t.Fatalf("CanViewFile() error = %v, want nil", err)
	}
	if err := svc.CanEditFile(context.Background(), actor, testDriveFile(false)); err != nil {
		t.Fatalf("CanEditFile() error = %v, want nil", err)
	}
	if err := svc.CanViewFolder(context.Background(), actor, testDriveFolder(false)); err != nil {
		t.Fatalf("CanViewFolder() error = %v, want nil", err)
	}
	if err := svc.CanShareFolder(context.Background(), actor, testDriveFolder(false)); err != nil {
		t.Fatalf("CanShareFolder() error = %v, want nil", err)
	}
	if len(client.checkCalls) != 0 {
		t.Fatalf("OpenFGA Check calls = %d, want 0", len(client.checkCalls))
	}
}

func TestDriveAuthorizationPlatformAdminFiltersTenantResourcesWithoutOpenFGA(t *testing.T) {
	client := &fakeOpenFGAClient{checkAllowed: false}
	svc := NewDriveAuthorizationService(client, DriveAuthorizationConfig{Enabled: true, FailClosed: true})
	actor := testDriveActor()
	actor.PlatformAdmin = true

	files, err := svc.FilterViewableFiles(context.Background(), actor, []DriveFile{
		testDriveFile(false),
		{ID: 3, PublicID: "other-tenant-file", TenantID: 11},
	})
	if err != nil {
		t.Fatalf("FilterViewableFiles() error = %v, want nil", err)
	}
	if len(files) != 1 || files[0].PublicID != "file-public-id" {
		t.Fatalf("FilterViewableFiles() = %#v, want only same-tenant file", files)
	}

	folders, err := svc.FilterViewableFolders(context.Background(), actor, []DriveFolder{
		testDriveFolder(false),
		{ID: 4, PublicID: "other-tenant-folder", TenantID: 11},
	})
	if err != nil {
		t.Fatalf("FilterViewableFolders() error = %v, want nil", err)
	}
	if len(folders) != 1 || folders[0].PublicID != "folder-public-id" {
		t.Fatalf("FilterViewableFolders() = %#v, want only same-tenant folder", folders)
	}
}

func TestDriveAuthorizationWriteShareTupleForGroup(t *testing.T) {
	client := &fakeOpenFGAClient{}
	svc := NewDriveAuthorizationService(client, DriveAuthorizationConfig{Enabled: true, FailClosed: true})

	share := DriveShare{
		Resource:        DriveResourceRef{Type: DriveResourceTypeFolder, PublicID: "folder-public-id"},
		SubjectType:     DriveShareSubjectGroup,
		SubjectPublicID: "group-public-id",
		Role:            DriveRoleEditor,
	}
	if err := svc.WriteShareTuple(context.Background(), share); err != nil {
		t.Fatalf("WriteShareTuple() error = %v", err)
	}
	if len(client.writeCalls) != 1 || len(client.writeCalls[0]) != 1 {
		t.Fatalf("write calls = %#v, want one tuple", client.writeCalls)
	}
	got := client.writeCalls[0][0]
	if got.User != "group:group-public-id#member" || got.Relation != "editor" || got.Object != "folder:folder-public-id" {
		t.Fatalf("tuple = %#v", got)
	}
}

func TestDriveAuthorizationShareLinkTupleHasExpiryCondition(t *testing.T) {
	client := &fakeOpenFGAClient{}
	svc := NewDriveAuthorizationService(client, DriveAuthorizationConfig{Enabled: true, FailClosed: true})
	expiresAt := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	link := DriveShareLink{
		PublicID:  "link-public-id",
		Resource:  DriveResourceRef{Type: DriveResourceTypeFile, PublicID: "file-public-id"},
		ExpiresAt: expiresAt,
	}
	if err := svc.WriteShareLinkTuple(context.Background(), link); err != nil {
		t.Fatalf("WriteShareLinkTuple() error = %v", err)
	}
	got := client.writeCalls[0][0]
	if got.User != "share_link:link-public-id" || got.Relation != "viewer" || got.Object != "file:file-public-id" {
		t.Fatalf("tuple = %#v", got)
	}
	if got.Condition == nil || got.Condition.Name != "not_expired" {
		t.Fatalf("condition = %#v, want not_expired", got.Condition)
	}
	if got.Condition.Context["expires_at"] != expiresAt {
		t.Fatalf("expires_at context = %#v, want %#v", got.Condition.Context["expires_at"], expiresAt)
	}
}

func testDriveActor() DriveActor {
	return DriveActor{UserID: 1, PublicID: "user-public-id", TenantID: 10}
}

func testDriveFile(locked bool) DriveFile {
	file := DriveFile{ID: 2, PublicID: "file-public-id", TenantID: 10}
	if locked {
		now := time.Now()
		file.LockedAt = &now
	}
	return file
}

func testDriveFolder(deleted bool) DriveFolder {
	folder := DriveFolder{ID: 3, PublicID: "folder-public-id", TenantID: 10}
	if deleted {
		now := time.Now()
		folder.DeletedAt = &now
	}
	return folder
}
