# Phase 4: Drive API / audit / smoke 実装チュートリアル

## この文書の目的

この文書は、`DriveService` を Huma/Gin の browser API に接続し、audit、metrics、readiness、smoke test まで追加する手順書です。

この Phase では Vue UI はまだ作りません。API と運用上の確認を先に完成させます。

## 完成条件

- `/api/v1/drive/...` が browser surface に追加される
- external surface / M2M surface / SCIM surface には Drive route が出ない
- raw upload/overwrite route は Cookie session + CSRF + active tenant を確認する
- raw download route は read operation として Cookie session + active tenant を確認する
- denied / failed operation が audit に残る
- OpenFGA metrics が低 cardinality label で記録される
- `OPENFGA_ENABLED=true` のとき `/readyz` が OpenFGA を確認する
- `scripts/smoke-openfga.sh` と `make smoke-openfga` がある

## Step 1. Drive API 型を追加する

### 対象ファイル

```text
backend/internal/api/drive_types.go
backend/internal/api/drive_folders.go
backend/internal/api/drive_files.go
backend/internal/api/drive_shares.go
backend/internal/api/drive_groups.go
backend/internal/api/drive_share_links.go
```

### 方針

既存 API と同じく Huma の input/output struct を明示します。public ID は path/query/body で扱い、DB internal ID は response に出しません。

共通 response body:

```text
DriveFolderBody
DriveFileBody
DriveItemBody
DrivePermissionBody
DriveShareBody
DriveGroupBody
DriveShareLinkBody
```

permissions response は direct と inherited を分けます。

```json
{
  "direct": [],
  "inherited": []
}
```

## Step 2. folder API を追加する

### endpoint

```text
GET    /api/v1/drive/folders/root
POST   /api/v1/drive/folders
GET    /api/v1/drive/folders/{folderPublicId}
PATCH  /api/v1/drive/folders/{folderPublicId}
DELETE /api/v1/drive/folders/{folderPublicId}
GET    /api/v1/drive/folders/{folderPublicId}/children
PATCH  /api/v1/drive/folders/{folderPublicId}/inheritance
```

### auth

mutating request は Cookie session + CSRF + active tenant を前提にします。read request は Cookie session + active tenant を前提にします。folder detail と children は `can_view`、rename/move/delete/inheritance はそれぞれ `can_edit` / `can_delete` / `can_share` を要求します。

children list は DB で tenant / parent / deleted を絞った後、OpenFGA `BatchCheck(can_view)` で不可視 item を除外します。

Gin は `/api/v1/drive/folders/root` と `/api/v1/drive/folders/{folderPublicId}` を別 route として同居させると wildcard conflict になります。runtime では `/api/v1/drive/folders/{folderPublicId}` handler が `folderPublicId == "root"` を特別扱いし、`/api/v1/drive/folders/root` URL を返します。OpenAPI 上は `/api/v1/drive/folders/{folderPublicId}` として表現します。

## Step 3. file API と raw route を追加する

### Huma endpoint

```text
GET    /api/v1/drive/files/{filePublicId}
PATCH  /api/v1/drive/files/{filePublicId}
DELETE /api/v1/drive/files/{filePublicId}
PATCH  /api/v1/drive/files/{filePublicId}/inheritance
GET    /api/v1/drive/items
GET    /api/v1/drive/search
```

### raw Gin route

既存 raw file route と同じ理由で、multipart upload と binary download は Gin route として登録します。

```text
POST /api/v1/drive/files
GET  /api/v1/drive/files/{filePublicId}/content
PUT  /api/v1/drive/files/{filePublicId}/content
```

download は `can_view` と `can_download` の両方を要求します。`can_download=false` の share link は metadata/preview 相当だけ許可し、content endpoint は拒否します。

overwrite は `can_edit` を要求し、locked/deleted/read-only file は拒否します。

download は read operation なので `X-CSRF-Token` は要求しません。upload / overwrite は mutating operation なので CSRF を必須にします。

## Step 4. share / group / link API を追加する

### permissions / shares

```text
GET    /api/v1/drive/files/{filePublicId}/permissions
POST   /api/v1/drive/files/{filePublicId}/shares
DELETE /api/v1/drive/files/{filePublicId}/shares/{sharePublicId}

GET    /api/v1/drive/folders/{folderPublicId}/permissions
POST   /api/v1/drive/folders/{folderPublicId}/shares
DELETE /api/v1/drive/folders/{folderPublicId}/shares/{sharePublicId}
```

### groups

```text
GET    /api/v1/drive/groups
POST   /api/v1/drive/groups
GET    /api/v1/drive/groups/{groupPublicId}
PATCH  /api/v1/drive/groups/{groupPublicId}
DELETE /api/v1/drive/groups/{groupPublicId}
POST   /api/v1/drive/groups/{groupPublicId}/members
DELETE /api/v1/drive/groups/{groupPublicId}/members/{userPublicId}
```

### share links

```text
POST   /api/v1/drive/files/{filePublicId}/share-links
POST   /api/v1/drive/folders/{folderPublicId}/share-links
PATCH  /api/v1/drive/share-links/{shareLinkPublicId}
DELETE /api/v1/drive/share-links/{shareLinkPublicId}
```

raw token は create response で一度だけ返します。list/detail response には token hash も raw token も含めません。

## Step 5. public share link API を追加する

### endpoint

```text
GET /api/public/drive/share-links/{token}
GET /api/public/drive/share-links/{token}/content
```

public endpoint は Cookie session を要求しません。ただし token hash lookup、status、expires_at、tenant policy、resource deleted を必ず確認します。

OpenFGA check では `share_link:<public_id>` を subject として `can_view` / `can_download` を確認します。

## Step 6. route registration を browser surface に限定する

### 対象ファイル

```text
backend/internal/api/register.go
backend/internal/app/openapi.go
backend/internal/api/files.go
backend/cmd/main/main.go
```

### 方針

`SurfaceBrowser` だけで Drive route を登録します。

```go
case SurfaceBrowser:
	registerDriveRoutes(api, deps)
```

external surface には追加しません。OpenAPI artifact は次の状態にします。

- `openapi/browser.yaml`: Drive API を含む
- `openapi/external.yaml`: Drive API を含まない
- `openapi/openapi.yaml`: full surface として含む

## Step 7. error mapping を追加する

Drive service error を API response に変換します。

```text
ErrDriveAuthzUnavailable -> 503
ErrDrivePermissionDenied -> 403
ErrDriveNotFound         -> 404
tenant mismatch         -> ErrDriveNotFound -> 404
ErrDriveLocked           -> 409
ErrInvalidDriveInput     -> 400
ErrDrivePolicyDenied     -> 403
```

OpenFGA timeout / unavailable は denied と区別し、`503` にします。OpenFGA check が false の場合は `403` です。tenant mismatch は OpenFGA に到達する前に DB tenant check / `validateDriveActorResource` で `ErrDriveNotFound` に寄せます。

## Step 8. audit event を追加する

### event

```text
drive.file.create
drive.file.view
drive.file.download
drive.file.update
drive.file.move
drive.file.delete
drive.folder.create
drive.folder.view
drive.folder.update
drive.folder.delete
drive.share.create
drive.share.revoke
drive.share_link.create
drive.share_link.update
drive.share_link.disable
drive.share_link.access
drive.group.create
drive.group.update
drive.group.delete
drive.group_member.add
drive.group_member.remove
drive.authz.denied
```

### audit に入れない値

- share link raw token
- token hash
- storage key
- OpenFGA API token
- password / secret
- raw idempotency key

metadata は resource type、resource public ID、role、subject public ID、result など低感度の値にします。

public share link token は URL path に含まれるため、request log は raw `Request.URL.Path` ではなく Gin route template の `FullPath()` を記録します。未マッチ route は `unmatched` に丸めます。

## Step 9. metrics / readiness を追加する

### metrics

```text
haohao_openfga_requests_total{operation,result}
haohao_openfga_request_duration_seconds{operation,result}
haohao_drive_authz_denied_total{operation,resource_type}
```

`operation` は次だけにします。

```text
openfga_check
openfga_write
openfga_delete
openfga_list_objects
```

tenant ID、user ID、file ID、folder ID、link token は label に入れません。

### readiness

`OPENFGA_ENABLED=true` の場合、`/readyz` で OpenFGA API 疎通を確認します。OpenFGA が落ちている場合は readiness fail です。

既存 session が有効でも OpenFGA が落ちていれば Drive protected operation は fail-closed です。SDK client wrapper は `haohao_openfga_requests_total{operation,result}` と `haohao_openfga_request_duration_seconds{operation,result}` を記録し、authorization denied は service layer で `haohao_drive_authz_denied_total{operation,resource_type}` に記録します。

## Step 10. smoke script を追加する

### 対象ファイル

```text
scripts/smoke-openfga.sh
Makefile
```

### Make target

```makefile
smoke-openfga:
	bash scripts/smoke-openfga.sh
```

`scripts/smoke-openfga.sh` は local password login 前提の開発用 smoke です。`.env` を読み込み、`DATABASE_URL` で owner / viewer / editor の smoke user と tenant membership を準備します。`AUTH_MODE=zitadel` のまま起動している場合、local password login は `501` になるため、smoke 実行時は backend を `AUTH_MODE=local` と `ENABLE_LOCAL_PASSWORD_LOGIN=true` で起動します。

### smoke scenario

```text
1. owner user が folder を作る
2. owner user が file を upload する
3. 別 user は download できない
4. owner が別 user に viewer share する
5. viewer は download できる
6. viewer は delete できない
7. editor share user は file rename / overwrite できる
8. DB 上で locked にした file は editor でも overwrite / delete できない
9. owner が group を作り member を追加する
10. group に folder viewer を付与し child file を閲覧できる
11. search 結果には閲覧可能 item だけが出る
12. share link を作成し public metadata を取得できる
13. can_download=false link で content download が拒否される
14. share link disable 後に public access が拒否される
15. audit と OpenFGA metrics が出ていることを確認する
```

## Phase 4 の完了確認

```bash
make gen
go test ./backend/...
OPENFGA_ENABLED=true make smoke-openfga
```

OpenAPI 確認:

```bash
rg "/api/v1/drive" openapi/browser.yaml openapi/openapi.yaml
! rg "/api/v1/drive" openapi/external.yaml
```
