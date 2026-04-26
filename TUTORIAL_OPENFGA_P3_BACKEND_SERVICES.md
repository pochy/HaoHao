# Phase 3: Backend service 実装チュートリアル

## この文書の目的

この文書は、OpenFGA client wrapper、`DriveAuthorizationService`、`DriveService` を backend service 層に追加する手順書です。

この Phase の中心は、API handler から OpenFGA SDK を直接呼ばせないことです。OpenFGA の object ID 組み立て、timeout、fail-closed、metrics、error mapping は `DriveAuthorizationService` に閉じ込めます。

## 完成条件

- OpenFGA SDK access が backend service 層に閉じている
- fake OpenFGA client で unit test できる
- DB tenant/resource state check の後に OpenFGA check が呼ばれる
- resource 作成時に owner tuple と parent tuple を write できる
- share 作成失敗時は `pending_sync` になり、access は許可されない
- share revoke は OpenFGA tuple delete 成功後に DB row を revoked にする
- inheritance stop は DB parent を残し、OpenFGA parent tuple を削除する
- locked/deleted/tenant policy は OpenFGA 許可より優先して拒否される

## Step 1. OpenFGA client interface を作る

### 対象ファイル

```text
backend/internal/service/openfga_client.go
backend/internal/service/openfga_client_test.go
```

### 実装方針

OpenFGA SDK の型を API 層や DriveService に広げません。HaoHao 内部用の小さな interface を作ります。

```go
type OpenFGATuple struct {
	User      string
	Relation  string
	Object    string
	Condition *OpenFGACondition
}

type OpenFGACondition struct {
	Name    string
	Context map[string]any
}

type OpenFGAClient interface {
	Check(ctx context.Context, tuple OpenFGATuple, context map[string]any) (bool, error)
	BatchCheck(ctx context.Context, tuples []OpenFGATuple, context map[string]any) ([]bool, error)
	ListObjects(ctx context.Context, user string, relation string, objectType string, context map[string]any) ([]string, error)
	WriteTuples(ctx context.Context, tuples []OpenFGATuple) error
	DeleteTuples(ctx context.Context, tuples []OpenFGATuple) error
}
```

実 SDK client はこの interface を満たす adapter にします。unit test では fake を注入します。

## Step 2. object ID helper を作る

### 対象ファイル

```text
backend/internal/service/drive_authz_ids.go
```

### 実装方針

ID の組み立てを散らさないよう、helper にまとめます。

```go
func openFGAUser(publicID string) string       { return "user:" + publicID }
func openFGAGroup(publicID string) string      { return "group:" + publicID }
func openFGAFolder(publicID string) string     { return "folder:" + publicID }
func openFGAFile(publicID string) string       { return "file:" + publicID }
func openFGAShareLink(publicID string) string  { return "share_link:" + publicID }
func openFGAGroupMember(publicID string) string { return "group:" + publicID + "#member" }
```

Zitadel subject はここに入れません。必ず `users.public_id` を使います。

## Step 3. `DriveAuthorizationService` を追加する

### 対象ファイル

```text
backend/internal/service/drive_authorization_service.go
backend/internal/service/drive_authorization_service_test.go
```

### 主な method

```text
CanViewFile(ctx, actor, file)
CanDownloadFile(ctx, actor, file)
CanEditFile(ctx, actor, file)
CanDeleteFile(ctx, actor, file)
CanShareFile(ctx, actor, file)
CanViewFolder(ctx, actor, folder)
CanEditFolder(ctx, actor, folder)
CanDeleteFolder(ctx, actor, folder)
CanShareFolder(ctx, actor, folder)
ListViewableFiles(ctx, actor)
ListViewableFolders(ctx, actor)
WriteResourceOwner(ctx, actor, resource)
WriteResourceParent(ctx, child, parent)
DeleteResourceParent(ctx, child, parent)
WriteShareTuple(ctx, share)
DeleteShareTuple(ctx, share)
WriteGroupMemberTuple(ctx, group, user)
DeleteGroupMemberTuple(ctx, group, user)
WriteShareLinkTuple(ctx, link)
DeleteShareLinkTuple(ctx, link)
```

### fail-closed

`OPENFGA_ENABLED=true` で OpenFGA call が error になった場合、permission は拒否します。

error mapping は service 層では typed error にします。

```go
var ErrDriveAuthzUnavailable = errors.New("drive authorization unavailable")
var ErrDrivePermissionDenied = errors.New("drive permission denied")
```

API 層では次に変換します。

- `ErrDriveAuthzUnavailable` -> `503`
- `ErrDrivePermissionDenied` -> `403`
- tenant mismatch / not found -> `404`

## Step 4. `DriveService` の型を作る

### 対象ファイル

```text
backend/internal/service/drive_service.go
backend/internal/service/drive_types.go
backend/internal/service/drive_errors.go
```

### 依存

```go
type DriveService struct {
	pool           *pgxpool.Pool
	queries        *db.Queries
	files          *FileService
	storage        FileStorage
	authz          *DriveAuthorizationService
	tenantSettings *TenantSettingsService
	audit          AuditRecorder
	now            func() time.Time
}
```

`DriveService` は既存 `FileService` / `FileStorage` を再利用します。ただし Drive file は `purpose='drive'` と `drive_folder_id` を必須にし、既存 attachment/import/export flow と混ぜません。

## Step 5. resource 作成 flow を実装する

### folder create

順序:

```text
1. active tenant と actor を受け取る
2. parent folder がある場合は tenant と can_edit を確認する
3. DB transaction で drive_folders row を作成する
4. audit drive.folder.create を transaction 内で記録する
5. commit
6. OpenFGA owner tuple を write する
7. parent がある場合は OpenFGA parent tuple を write する
8. tuple write 失敗時は resource を使用不可状態にし、audit failed を残す
```

### file upload

順序:

```text
1. parent folder の tenant と can_edit を確認する
2. tenant file quota / drive policy を確認する
3. FileStorage に body を保存する
4. DB transaction で file_objects purpose='drive' row を作成する
5. audit drive.file.create を transaction 内で記録する
6. commit
7. OpenFGA owner tuple と parent tuple を write する
8. tuple write 失敗時は file を使用不可状態にし、audit failed を残す
```

storage save と DB commit の間で失敗した場合は、既存 FileService の cleanup 方針に合わせます。

## Step 6. check ordering を実装する

すべての Drive operation は次の順にします。

```text
1. 認証済み user または有効な share link token を確認する
2. active tenant を確認する
3. resource tenant を DB で確認する
4. deleted を拒否する
5. locked / retention / read-only を確認する
6. tenant policy を確認する
7. OpenFGA Check / BatchCheck / ListObjects を呼ぶ
8. denied / failed を audit する
```

locked file は `can_view` だけ許可し、edit / overwrite / rename / move / delete / share change は OpenFGA の許可結果より前に拒否します。

## Step 7. share / group / link flow を実装する

### user/group share create

```text
1. actor の can_share を確認する
2. subject user/group が同一 tenant に属することを確認する
3. DB に active share row を作成する
4. audit drive.share.create を記録する
5. OpenFGA tuple を write する
6. tuple write 失敗時は share row を pending_sync に更新する
```

`pending_sync` は access を許可しません。OpenFGA check が true にならないためです。

### share revoke

```text
1. actor の can_share を確認する
2. OpenFGA tuple delete を実行する
3. delete 成功後に DB share row を revoked にする
4. audit drive.share.revoke を記録する
```

revoke は「OpenFGA から先に消す」順序を守ります。DB だけ revoked で tuple が残る状態を避けます。

### group member add/remove

```text
add:
1. group tenant と user tenant membership を確認する
2. DB member row を active にする
3. OpenFGA group#member tuple を write する
4. audit drive.group_member.add

remove:
1. DB member row を deleted にする前に OpenFGA tuple delete を実行する
2. delete 成功後に DB row を deleted にする
3. audit drive.group_member.remove
```

### share link create/disable

```text
create:
1. tenant drive policy を確認する
2. actor の can_share を確認する
3. raw token を生成し、hash だけ保存する
4. DB share link row を active で作成する
5. OpenFGA share_link viewer tuple を condition 付きで write する
6. raw token は response で一度だけ返す

disable:
1. actor の can_share を確認する
2. OpenFGA tuple delete を実行する
3. DB share link row を disabled にする
4. audit drive.share_link.disable
```

## Step 8. inheritance stop を実装する

継承停止では DB の親子関係を残します。UI の folder tree と一覧表示には parent が必要だからです。

```text
1. actor の can_share を確認する
2. DB inheritance_enabled=false に更新する
3. OpenFGA parent tuple を delete する
4. audit を記録する
```

再開する場合は逆に `inheritance_enabled=true` と parent tuple write を行います。

## Step 9. backend wiring に接続する

### 対象ファイル

```text
backend/cmd/main/main.go
backend/internal/app/app.go
backend/internal/api/register.go
```

### wiring 順

```text
config.Load()
PostgreSQL / Redis
db.Queries
AuditService
AuthzService
TenantSettingsService
FileStorage
FileService
OpenFGAClient
DriveAuthorizationService
DriveService
app.New(...)
```

`backend/internal/api.Dependencies` に `DriveService` を追加します。route 登録は Phase 4 で行います。

## Step 10. unit test を追加する

### 確認観点

- OpenFGA error は fail-closed
- OpenFGA disabled でも `OPENFGA_FAIL_CLOSED=true` なら拒否する
- locked file は editor でも edit/delete/share 不可
- deleted resource は OpenFGA check 前に拒否される
- tenant mismatch は not found 扱い
- share 作成時の tuple write 失敗は `pending_sync`
- share revoke は tuple delete 成功後に DB revoked
- group member add/remove が tuple write/delete を呼ぶ
- `can_download=false` share link は metadata だけ許可する

## Phase 3 の完了確認

```bash
go test ./backend/internal/service
go test ./backend/internal/app ./backend/cmd/main
```

この時点では API route が未接続でも構いません。service の fail-closed と tuple ordering を先に固めます。

