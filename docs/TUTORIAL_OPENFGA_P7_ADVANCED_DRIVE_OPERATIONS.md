# Phase 7: OpenFGA Drive 高度運用チュートリアル

## この文書の目的

この文書は、`TUTORIAL_OPENFGA_P6_REMAINING_TASKS.md` の末尾で Phase 6 でも残した Drive/OpenFGA 周辺課題を、次に実装できる粒度へ分解したチュートリアルです。

Phase 1-6 では、Drive の基本認可、共有拡張、管理 UI、外部共有、password link、pending sync repair、OpenFGA drift 検出までを扱いました。Phase 7 では、より影響範囲が広い運用課題を扱います。

この Phase の目的は、危険な機能をまとめて有効化することではありません。目的は、**採用判断が必要な機能を feature flag / policy / audit / test とセットで実装できる順番へ落とすこと**です。

特に次の 2 つは、実装前に採用判断を必ず挟みます。

- tenant admin によるファイル本文の横断閲覧
- 匿名 Editor link

どちらも Drive/OpenFGA の権限境界を大きく変えるため、初期状態では disabled にします。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- `TUTORIAL_OPENFGA_P1_INFRA_MODEL.md` から `TUTORIAL_OPENFGA_P6_REMAINING_TASKS.md` までが実装済み
- Drive file / folder / group / share link の resource authorization は OpenFGA で判定している
- tenant boundary、resource state、tenant policy は OpenFGA check の前に DB で確認している
- tenant admin UI は Drive policy、Drive audit、共有状態、repair 操作を扱える
- external share、invitation、password share link、approval flow が DB / API / UI / audit として存在する
- `pending_sync` repair と OpenFGA tuple drift detection が運用できる
- `tenant_admin` は、明示的に共有されていない Drive file body を閲覧できない
- OpenFGA authorization model は runtime 起動時に自動更新されない
- `make smoke-openfga` と OpenFGA enabled E2E が通る

この Phase 7 でも、Zitadel / AuthzService / tenant-aware auth の担当境界は変えません。

## この Phase でやること / やらないこと

やること:

```text
高リスク機能を default disabled の policy と audit 付きで設計する
tenant admin 本文横断閲覧を break-glass workflow として分離する
匿名 Editor link を採用判断付きの model / guard に落とす
workspace を folder/file 継承の上位 resource として導入する
file body を object storage driver に閉じ込める
scan / DLP / plan enforcement を DriveService の guard に接続する
HA/DR と OpenFGA / DB / storage の整合性 runbook を repo に持つ
external bearer / M2M Drive API を browser API から分離する
```

やらないこと:

```text
危険機能を migration だけで既存 tenant に暗黙有効化する
tenant_admin role だけで file body を横断閲覧可能にする
anonymous editor link を password / TTL / revision なしで production enabled にする
storage key、signed URL、scan finding 本文、token raw value を audit / log / metrics に出す
検索、共同編集、sync client、CMK、data residency、legal discovery、clean room をこの Phase に混ぜる
```

Phase 7 の各 Step は、実装前に短い採用判断を `IMPL.md` へ残します。少なくとも default policy、feature flag、rollback 方法、追加 audit event、smoke env を書きます。

## 完成条件

Phase 7 backlog を一通り実装した状態の完成条件は次です。

- tenant admin のファイル本文横断閲覧は、採用可否、必要 role、policy、audit、UI 導線、緊急時運用が明文化されている
- 匿名 Editor link は default disabled のまま、採用する場合の authorization model / tenant policy / rate limit / audit / abuse guard が決まっている
- workspace resource が Drive metadata と OpenFGA model の両方で第一級 resource として扱える
- object storage driver と signed URL の境界ができ、DB は metadata、object storage は body の source of truth として整理されている
- virus scan / content inspection / DLP が upload / download / share flow に割り込める
- billing / subscription / plan enforcement が Drive policy と storage / share / link 制限へ接続されている
- multi-region / HA / DR で PostgreSQL、OpenFGA、object storage の復旧順序と整合性確認が定義されている
- external bearer / M2M Drive API の全 endpoint 実装計画が browser API と分離されている
- smoke / E2E / operational verification が高度運用機能を明示 env 付きで確認できる

## 実装順の全体像

| Step | 主題 | 目的 |
| --- | --- | --- |
| Step 1 | tenant admin によるファイル本文横断閲覧 | 採用判断と break-glass guard を先に固定する |
| Step 2 | 匿名 Editor link | default disabled のまま、採用時のリスク制御を設計する |
| Step 3 | workspace resource | tenant と folder の間に workspace 境界を導入する |
| Step 4 | object storage driver / signed URL | file body を pluggable storage へ移し、download/upload の境界を作る |
| Step 5 | virus scan / content inspection / DLP | content lifecycle に検査状態を入れる |
| Step 6 | billing / subscription / plan enforcement | plan に応じて storage、share、link、scan を制限する |
| Step 7 | multi-region / HA / DR | DB、OpenFGA、object storage の復旧と drift 確認を運用化する |
| Step 8 | external bearer / M2M Drive API | browser surface と分けて全 Drive endpoint を external API 化する |
| Step 9 | smoke / E2E / operational verification | 高度運用機能の確認手順を固定する |

## 先に決める方針

### Phase 7 は default safe で始める

Phase 7 の機能は、誤ると data exposure、権限昇格、tenant 間漏洩、監査不能につながります。

そのため、追加する policy はすべて default disabled にします。migration や seed で既存 tenant の挙動を広げません。

### OpenFGA は resource relation、DB は状態と policy を持つ

Phase 7 でも、OpenFGA に storage state、scan state、billing state、approval state を持たせません。

- OpenFGA: `can_view`、`can_download`、`can_edit`、`can_share` のような relation 判定
- PostgreSQL: tenant policy、workspace state、object metadata、scan result、billing plan、audit
- object storage: file body

API は必ず DB guard を通してから OpenFGA `Check` または signed URL 発行へ進みます。

### tenant admin 本文閲覧は通常の admin UI に混ぜない

tenant admin が共有状態や audit を調査できることと、ファイル本文を閲覧できることは別権限です。

本文横断閲覧を採用する場合も、通常の tenant admin UI には body preview / download 導線を置きません。専用の break-glass 画面、追加 role、理由入力、時間制限、二重確認、audit を必須にします。

### 匿名 Editor link は実装しても default disabled

匿名 Editor link は abuse と data destruction のリスクが高いため、Phase 7 で設計しても production default は disabled にします。

有効化する場合は tenant policy、最大 TTL、password 必須、rate limit、編集可能 MIME type、revision retention、rollback UI、abuse metrics を同時に入れます。

### public / external / M2M surface を混ぜない

Drive の browser API、public link API、external bearer API、M2M API は entrypoint と auth middleware を分けます。

同じ service を再利用しても、HTTP surface、OpenAPI spec、CSRF 要否、rate limit、audit actor type は明示的に分けます。

## 推奨 PR 分割

Phase 7 は subsystem の境界が大きいため、各 Step を原則 1 PR にします。例外は、Step 4 object storage と Step 5 scan/DLP のように file lifecycle が密接な場合だけです。

| PR | 含める Step | merge gate |
| --- | --- | --- |
| 1 | Step 1 | break-glass disabled のまま denial / audit / UI 非表示が確認できる |
| 2 | Step 2 | model test と service guard があるが production default は disabled |
| 3 | Step 3 | workspace migration、model bootstrap、UI selector、E2E が揃う |
| 4 | Step 4 | storage driver contract、upload complete guard、signed URL leak check が通る |
| 5 | Step 5 | scan/DLP state が download/share guard に入る |
| 6 | Step 6 | effective Drive policy が plan 上限を超えない |
| 7 | Step 7 | DR runbook、dry-run drift、storage consistency check がある |
| 8 | Step 8-9 | external API smoke と operational verification が env-gated で動く |

## Step 1. tenant admin によるファイル本文横断閲覧の採用判断と guard 設計

### 対象 subsystem

```text
tenant settings / role seed
backend admin API
DriveService guard
Drive audit
tenant admin UI
OpenAPI browser spec
E2E
```

### 実装方針

まず、本文横断閲覧を採用するかどうかを product policy として明示します。

初期実装は次の状態にします。

```json
{
  "drive": {
    "adminContentAccessMode": "disabled"
  }
}
```

採用する場合だけ、次の mode を追加します。

```text
disabled
break_glass
```

`tenant_admin` だけでは本文閲覧を許可しません。本文閲覧には追加 role を要求します。

```text
drive_content_admin
```

この role は tenant 管理全般の role ではなく、本文アクセス専用の高権限 role として扱います。

### API / DB / UI の境界

DB は tenant settings に `adminContentAccessMode` を保存します。必要であれば、break-glass session を別 table に保存します。

```text
drive_admin_content_access_sessions
```

最小列:

```text
id
tenant_id
actor_user_id
reason
reason_category
expires_at
created_at
ended_at
```

Admin API は通常の Drive download endpoint を再利用しません。専用 route を切ります。

```text
POST /api/v1/admin/tenants/{tenantSlug}/drive/content-access-sessions
DELETE /api/v1/admin/tenants/{tenantSlug}/drive/content-access-sessions/current
GET /api/v1/admin/tenants/{tenantSlug}/drive/files/{fileId}/metadata
GET /api/v1/admin/tenants/{tenantSlug}/drive/files/{fileId}/content
```

通常の `GET /api/v1/drive/files/{fileId}/content` は引き続き OpenFGA `can_download` を要求します。

Admin content endpoint は OpenFGA の resource permission を bypass する可能性があるため、次を DB で必ず確認します。

- actor が `tenant_admin` と `drive_content_admin` を持つ
- tenant policy が `break_glass`
- active な break-glass session がある
- session reason が空ではない
- file の `tenant_id` が対象 tenant と一致する
- file が deleted / retention blocked / legal hold restricted ではない

UI は通常の Drive browser に入れません。tenant admin 側に専用画面を置き、metadata と audit context を見せた後に content access を明示操作で開始します。

break-glass session は短命にします。推奨 default は 15 分以内です。session 中も file ごとに再認可し、signed URL を返す場合は session expiry より長い TTL を発行しません。二重承認が必要な tenant では、session start と content access を別 actor が承認する mode を追加できるよう table / service 境界を残します。

### audit / metrics / security 注意点

audit event:

```text
drive.admin_content_access.session_started
drive.admin_content_access.session_ended
drive.admin_content_access.metadata_viewed
drive.admin_content_access.content_viewed
drive.admin_content_access.denied
```

metadata には reason length、session id、file public id、result、policy decision を入れます。file body、storage key、signed URL は入れません。

metrics:

```text
drive_admin_content_access_total{result,reason}
drive_admin_content_access_session_active
```

`reason` label に自由入力を入れません。`manual`、`incident`、`legal` のような低 cardinality 値だけにします。

### 確認コマンド

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
RUN_DRIVE_ADMIN_CONTENT_ACCESS_SMOKE=1 make smoke-openfga
```

manual browser 確認:

- `tenant_admin` だけでは本文を開けない
- `drive_content_admin` があっても policy disabled なら開けない
- break-glass session がないと開けない
- session 中の content access が audit に残る
- 通常 Drive UI には本文横断閲覧導線が出ない

## Step 2. 匿名 Editor link のリスク評価と disabled 方針

### 対象 subsystem

```text
openfga/drive.fga
openfga/drive.fga.yaml
drive_share_links
tenant settings
public share link API
DriveService edit guard
revision / audit
rate limit
```

### 実装方針

匿名 Editor link は default disabled にします。Phase 7 では、採用する場合の条件を明文化し、model test と service guard を用意できる状態にします。

tenant policy の候補:

```json
{
  "drive": {
    "anonymousEditorLinksEnabled": false,
    "anonymousEditorLinksRequirePassword": true,
    "anonymousEditorLinkMaxTTLMinutes": 60
  }
}
```

匿名 Editor link を有効化する場合でも、対象 resource は file に限定して始めます。folder に対する匿名 Editor link は、配下全体への影響が大きいため別途設計に残します。

### API / DB / UI の境界

DB の `drive_share_links` に editor link を表す permission level を追加する場合、既存 viewer link と同じ token だけで編集できる状態にしません。

必要な条件:

- link が active
- expires_at が必須
- password required
- target file が locked / deleted / read-only ではない
- tenant policy が anonymous editor link を許可している
- MIME type が editable allowlist に入っている
- rate limit と abuse guard を通る

public API は viewer content endpoint と分けます。

```text
GET /api/public/drive/share-links/{token}
GET /api/public/drive/share-links/{token}/content
PUT /api/public/drive/share-links/{token}/content
```

`PUT` は CSRF cookie session ではなく public token + password verification + rate limit を使います。anonymous actor として audit します。

OpenFGA model に `share_link` を editor relation へ入れる場合は、必ず model test を追加します。

```text
share_link with not_expired can_edit file
expired share_link cannot edit file
viewer link cannot edit file
disabled tenant policy blocks service before OpenFGA
```

### audit / metrics / security 注意点

audit event:

```text
drive.share_link.editor_created
drive.share_link.editor_accessed
drive.share_link.editor_updated_content
drive.share_link.editor_denied
drive.share_link.editor_disabled
```

anonymous edit は rollback 前提で扱います。file revision または object version を保存できない状態では、匿名 Editor link を production enabled にしません。

metrics:

```text
drive_public_editor_link_requests_total{result,operation}
drive_public_editor_link_rate_limited_total
drive_public_editor_link_content_bytes_total
```

token、IP address、password result detail を high cardinality label に入れません。

### 確認コマンド

```bash
cd openfga && fga model test --tests drive.fga.yaml
go test ./backend/...
RUN_OPENFGA_PUBLIC_EDITOR_LINK_SMOKE=1 make smoke-openfga
```

manual 確認:

- default policy では editor link を作成できない
- password なしでは editor link を作成できない
- expired link は edit できない
- locked file は edit できない
- edit 後に audit と revision が残る

## Step 3. workspace resource の本格導入

### 対象 subsystem

```text
db/migrations
db/queries
openfga/drive.fga
DriveService
tenant admin UI
Drive browser UI
smoke / E2E
```

### 実装方針

Phase 1-6 では tenant と folder/file の間に workspace resource を本格導入していません。Phase 7 では、tenant 配下に複数 workspace を持てるようにします。

workspace は次を表します。

- Drive root folder のまとまり
- tenant 内の管理単位
- storage quota と policy override の単位
- 将来の project / department / external collaboration unit

OpenFGA model は tenant boundary を担当しませんが、workspace relation は folder/file inheritance の上位に置けます。

候補:

```fga
type workspace
  relations
    define owner: [user, group#member]
    define editor: [user, group#member] or owner
    define viewer: [user, group#member] or editor

type folder
  relations
    define workspace: [workspace]
    define owner: [user, group#member] or owner from parent or owner from workspace
    define editor: [user, group#member] or owner or editor from parent or editor from workspace
    define viewer: [user, group#member, share_link with not_expired] or editor or viewer from parent or viewer from workspace
```

authorization model を変えるため、runtime 自動更新は禁止です。model test と明示 bootstrap で `OPENFGA_AUTHORIZATION_MODEL_ID` を更新します。

### API / DB / UI の境界

DB table:

```text
drive_workspaces
```

最小列:

```text
id
public_id
tenant_id
name
root_folder_id
created_by_user_id
created_at
updated_at
deleted_at
```

`drive_folders` と `file_objects(purpose='drive')` には `workspace_id` を追加します。

既存 tenant には migration で default workspace を 1 つ作り、既存 root folder / drive file をそこへ紐づけます。この backfill は再実行可能にし、`workspace_id IS NULL` の Drive resource が残っていたら startup readiness ではなく smoke / migration check で検出します。service 層では Phase 7 以降、Drive resource に `workspace_id` がない状態を invalid state として扱います。

API:

```text
GET /api/v1/drive/workspaces
POST /api/v1/drive/workspaces
GET /api/v1/drive/workspaces/{workspaceId}
PATCH /api/v1/drive/workspaces/{workspaceId}
DELETE /api/v1/drive/workspaces/{workspaceId}
GET /api/v1/drive/workspaces/{workspaceId}/children
```

UI:

- Drive sidebar に workspace selector を追加する
- root folder は workspace ごとに分ける
- tenant admin UI では workspace list、owner、quota、policy override を表示する

### audit / metrics / security 注意点

audit event:

```text
drive.workspace.created
drive.workspace.updated
drive.workspace.deleted
drive.workspace.permission_changed
```

tenant mismatch は引き続き `404` にします。workspace id を指定した API でも、active tenant と workspace tenant が一致しない場合は存在を隠します。

### 確認コマンド

```bash
make gen
cd openfga && fga model test --tests drive.fga.yaml
go test ./backend/...
npm --prefix frontend run build
RUN_OPENFGA_WORKSPACE_SMOKE=1 make smoke-openfga
```

E2E:

- workspace A の owner は A root を見られる
- workspace B の viewer は A root を見られない
- workspace share から folder/file に viewer が継承される
- workspace deleted 後は children が表示されない

## Step 4. object storage driver と signed URL の本格導入

### 対象 subsystem

```text
backend/internal/service/file_service.go
backend/internal/service/drive_service.go
storage driver package
file_objects
tenant settings
download / upload API
single binary static serving
```

### 実装方針

DB は file metadata と authorization state の source of truth とし、file body は storage driver に閉じ込めます。

storage driver interface は最初に固定します。

```text
PutObject(ctx, key, reader, size, contentType, metadata)
GetObject(ctx, key)
DeleteObject(ctx, key)
CreateDownloadURL(ctx, key, ttl)
CreateUploadURL(ctx, key, ttl, contentType)
HeadObject(ctx, key)
```

local dev は filesystem または existing DB backed storage を使えます。production は S3 compatible object storage を想定します。

### API / DB / UI の境界

`file_objects` は body を持つ代わりに storage metadata を持ちます。

```text
storage_driver
storage_bucket
storage_key
storage_version
content_sha256
size_bytes
content_type
etag
```

signed URL を返す場合でも、API は必ず先に DB guard と OpenFGA check を行います。

download flow:

```text
authenticate
active tenant
resource tenant
deleted / locked / policy
OpenFGA can_view + can_download
storage object exists
short TTL signed URL or proxied stream
audit
```

upload flow:

```text
authenticate
active tenant
folder tenant
OpenFGA can_edit parent folder
size / content type / plan policy
object key reserve
signed upload URL or proxied upload
complete callback validates object
DB row active
owner / parent tuple
audit
```

upload は `reserved` / `uploading` / `active` のような state を分けます。signed upload URL を発行しただけでは list / search / share 対象にしません。complete callback では object size、content type、checksum、tenant/workspace、reserved key、uploader を再確認し、失敗時は orphan cleanup job の対象にします。

object key は user 入力から作りません。tenant public id、workspace public id、file public id、object version などから service が生成し、path traversal や推測可能な連番を避けます。

### audit / metrics / security 注意点

storage key と signed URL は audit / log / metrics に出しません。

audit event:

```text
drive.file.upload_url_created
drive.file.upload_completed
drive.file.download_url_created
drive.file.storage_mismatch_detected
```

metrics:

```text
drive_storage_operation_seconds{operation,result,driver}
drive_storage_bytes_total{operation,driver}
```

bucket、key、file id は label にしません。

### 確認コマンド

```bash
go test ./backend/...
RUN_DRIVE_STORAGE_DRIVER_SMOKE=1 make smoke-openfga
npm --prefix frontend run build
make binary
```

manual 確認:

- viewer download disabled の file は signed URL が発行されない
- signed URL TTL が短い
- storage key が response / audit / log に出ない
- upload complete 前の DB row は list に出ない、または pending として扱われる

## Step 5. virus scan / content inspection / DLP の挿入点設計

### 対象 subsystem

```text
file_objects
drive scan job
upload complete flow
download guard
share guard
tenant policy
admin UI
audit / metrics
```

### 実装方針

scan は upload 後に非同期で実行します。scan が完了するまでの扱いは tenant policy で決めます。

policy 候補:

```json
{
  "drive": {
    "contentScanEnabled": false,
    "blockDownloadUntilScanComplete": true,
    "blockShareUntilScanComplete": true,
    "dlpEnabled": false
  }
}
```

DB state:

```text
scan_status: pending / clean / infected / blocked / failed / skipped
scan_reason
scanned_at
scan_engine
```

`infected` または `blocked` の file は、Owner でも download / share / public link access を拒否します。

### API / DB / UI の境界

upload complete 後:

```text
file row active
scan_status pending
scan job enqueue
```

download 前:

```text
if scan required and not clean -> deny
then OpenFGA can_view / can_download
then storage access
```

share 前:

```text
if policy blocks sharing unscanned file -> deny before tuple write
```

tenant admin UI:

- scan status filter
- blocked file list
- rescan action
- DLP finding summary
- body preview は出さない

### audit / metrics / security 注意点

audit event:

```text
drive.file.scan_enqueued
drive.file.scan_completed
drive.file.scan_failed
drive.file.dlp_blocked
drive.file.download_denied_scan
drive.share.denied_scan
```

DLP finding の詳細本文や検出文字列は audit に入れません。分類と件数だけにします。

metrics:

```text
drive_content_scan_total{result,engine}
drive_content_scan_duration_seconds{engine,result}
drive_content_scan_queue_depth
```

### 確認コマンド

```bash
go test ./backend/...
RUN_DRIVE_SCAN_SMOKE=1 make smoke-openfga
```

manual 確認:

- scan pending の file は policy に従って download / share が止まる
- clean 後に download できる
- blocked file は Owner でも download できない
- tenant admin は scan status を見られるが本文は見られない

## Step 6. billing / subscription / plan enforcement と Drive policy の接続

### 対象 subsystem

```text
tenant settings
billing / plan service
DriveService policy guard
share link service
storage quota job
tenant admin UI
metrics
```

### 実装方針

Drive の上限値は tenant policy に直接ばら撒かず、plan から解決した effective policy として扱います。

plan で制限する候補:

```text
storage quota
max file size
max workspace count
max external share count
max public link count
password link availability
DLP availability
anonymous editor link availability
M2M Drive API availability
```

tenant settings は plan の override を持てますが、plan 上限を超える override は保存時に拒否します。

### API / DB / UI の境界

`DrivePolicyService` のような解決層を置きます。

```text
tenant settings
subscription plan
feature flags
runtime config
-> effective Drive policy
```

DriveService は raw tenant settings ではなく effective policy を参照します。

tenant admin UI:

- current plan
- used storage
- external share usage
- public link usage
- disabled because of plan の表示

billing UI が未実装でも、Drive 側は `plan_code` と effective policy を読めるようにします。

### audit / metrics / security 注意点

audit event:

```text
drive.policy.enforcement_denied
drive.quota.exceeded
drive.plan_feature.denied
```

metrics:

```text
drive_storage_usage_bytes{plan}
drive_quota_denied_total{reason,plan}
drive_plan_feature_denied_total{feature,plan}
```

tenant id を label に入れません。

### 確認コマンド

```bash
go test ./backend/...
RUN_DRIVE_PLAN_ENFORCEMENT_SMOKE=1 make smoke-openfga
npm --prefix frontend run build
```

確認 scenario:

- free plan では public link count 上限を超えると作成できない
- plan が password link を許可しない場合は UI が disabled になる
- quota 超過時は upload URL が発行されない
- audit に policy decision が残る

## Step 7. multi-region / HA / DR と OpenFGA / DB / object storage の整合性設計

### 対象 subsystem

```text
ops docs
readiness / liveness
OpenFGA bootstrap
drift detection job
storage consistency checker
backup / restore scripts
runbook
alerts
```

### 実装方針

HA/DR は feature code だけで完結しません。runbook と検証手順を repo に持ちます。

復旧順序は次に固定します。

```text
PostgreSQL restore
object storage availability check
OpenFGA store/model availability check
OpenFGA tuple drift dry-run
pending_sync repair dry-run
Drive smoke read-only
Drive mutation smoke
```

OpenFGA は DB の派生 state として扱います。DB と OpenFGA がずれた場合、DB を source of truth にして repair します。ただし、repair は dry-run と diff audit を先に出します。

### API / DB / UI の境界

admin API:

```text
GET /api/v1/admin/tenants/{tenantSlug}/drive/operations/health
POST /api/v1/admin/tenants/{tenantSlug}/drive/operations/drift-check
POST /api/v1/admin/tenants/{tenantSlug}/drive/operations/repair
```

operator runbook:

```text
docs/runbooks/drive-openfga-dr.md
```

CI または scheduled job:

```text
drive-openfga-drift-check
drive-storage-orphan-check
drive-storage-missing-object-check
```

### audit / metrics / security 注意点

audit event:

```text
drive.operations.drift_check_started
drive.operations.drift_check_completed
drive.operations.repair_started
drive.operations.repair_completed
drive.operations.storage_consistency_failed
```

metrics:

```text
drive_openfga_drift_detected_total{kind}
drive_openfga_repair_total{result}
drive_storage_consistency_errors_total{kind}
```

DR runbook には secret、token、bucket key を書きません。

### 確認コマンド

```bash
go test ./backend/...
RUN_DRIVE_DRIFT_SMOKE=1 make smoke-openfga
RUN_DRIVE_STORAGE_CONSISTENCY_SMOKE=1 make smoke-openfga
```

operational drill:

- OpenFGA tuple を意図的に欠落させて drift check が検出する
- dry-run では変更されない
- repair 後に expected tuple が戻る
- storage orphan を検出できる

## Step 8. external bearer / M2M Drive API の全 endpoint 実装計画

### 対象 subsystem

```text
backend external API
OpenAPI external spec
AuthzService bearer / M2M validation
DriveService
rate limit
audit
SDK docs
smoke
```

### 実装方針

browser API と external bearer / M2M API を混ぜません。external API は bearer token、scope、tenant resolution、actor type を明示的に扱います。

scope:

```text
drive:read
drive:write
drive:share
drive:admin
```

actor type:

```text
user_bearer
machine_client
```

M2M は user の代わりに client principal として audit します。OpenFGA user tuple に machine client を混ぜるかどうかは、authorization model 変更として扱います。

初期方針:

- user bearer: user identity を `user:<public_id>` として OpenFGA check
- machine client: service account user を明示 provisioning してから OpenFGA check

### API / DB / UI の境界

external API:

```text
GET /api/external/v1/drive/files/{fileId}/metadata
GET /api/external/v1/drive/files/{fileId}/content
POST /api/external/v1/drive/files
DELETE /api/external/v1/drive/files/{fileId}
GET /api/external/v1/drive/folders/{folderId}/children
POST /api/external/v1/drive/folders
POST /api/external/v1/drive/files/{fileId}/shares
```

tenant resolution:

- explicit tenant slug/header is required
- token must be allowed for tenant
- active tenant cookie is ignored

CSRF は不要です。代わりに bearer validation、scope check、rate limit、audit actor type を必須にします。

### audit / metrics / security 注意点

audit event は browser と同じ `drive.*` を使えますが、metadata に actor type と client id public identifier を入れます。

token raw value は一切残しません。

metrics:

```text
drive_external_api_requests_total{operation,result,actor_type}
drive_external_api_rate_limited_total{operation}
```

### 確認コマンド

```bash
make gen
go test ./backend/...
RUN_OPENFGA_EXTERNAL_API_SMOKE=1 make smoke-openfga
```

smoke scenario:

- scope なし bearer は拒否
- wrong tenant header は `404` または `403`
- `drive:read` は download できるが share 作成できない
- service account user に OpenFGA viewer tuple がある場合だけ read できる
- audit actor type が `machine_client` になる

## Step 9. smoke / E2E / operational verification

### 対象 subsystem

```text
scripts/smoke-openfga.sh
frontend E2E
openfga model tests
ops runbooks
CI jobs
```

### 実装方針

Phase 7 の確認は default CI に全部入れません。高度運用機能は外部 service、object storage、scanner、billing plan、M2M token が必要になるため、明示 env 付き smoke と operational drill に分けます。

default CI:

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
cd openfga && fga model test --tests drive.fga.yaml
make smoke-openfga
```

明示 smoke:

```bash
RUN_DRIVE_ADMIN_CONTENT_ACCESS_SMOKE=1 make smoke-openfga
RUN_OPENFGA_PUBLIC_EDITOR_LINK_SMOKE=1 make smoke-openfga
RUN_OPENFGA_WORKSPACE_SMOKE=1 make smoke-openfga
RUN_DRIVE_STORAGE_DRIVER_SMOKE=1 make smoke-openfga
RUN_DRIVE_SCAN_SMOKE=1 make smoke-openfga
RUN_DRIVE_PLAN_ENFORCEMENT_SMOKE=1 make smoke-openfga
RUN_DRIVE_DRIFT_SMOKE=1 make smoke-openfga
RUN_OPENFGA_EXTERNAL_API_SMOKE=1 make smoke-openfga
```

E2E は UI があるものに限定します。

```bash
E2E_OPENFGA_ENABLED=true E2E_DRIVE_WORKSPACE_ENABLED=true make e2e
E2E_OPENFGA_ENABLED=true E2E_DRIVE_ADMIN_OPERATIONS_ENABLED=true make e2e
```

### audit / metrics / security 注意点

smoke と E2E は token raw value、password、signed URL、storage key を stdout に出しません。

log assertion は public id、status code、audit event type、low sensitivity metadata だけを対象にします。

### 最終確認コマンド

Phase 7 の通常確認:

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
cd openfga && fga model test --tests drive.fga.yaml
make smoke-openfga
```

Phase 7 の高度運用確認:

```bash
RUN_DRIVE_ADMIN_CONTENT_ACCESS_SMOKE=1 make smoke-openfga
RUN_OPENFGA_PUBLIC_EDITOR_LINK_SMOKE=1 make smoke-openfga
RUN_OPENFGA_WORKSPACE_SMOKE=1 make smoke-openfga
RUN_DRIVE_STORAGE_DRIVER_SMOKE=1 make smoke-openfga
RUN_DRIVE_SCAN_SMOKE=1 make smoke-openfga
RUN_DRIVE_PLAN_ENFORCEMENT_SMOKE=1 make smoke-openfga
RUN_DRIVE_DRIFT_SMOKE=1 make smoke-openfga
RUN_OPENFGA_EXTERNAL_API_SMOKE=1 make smoke-openfga
```

各 Step の完了時は `IMPL.md` に、default disabled のまま残した policy、追加した role、OpenFGA model id 更新の有無、operational runbook、明示 smoke env を追記します。特に break-glass、匿名 Editor link、M2M API は「採用判断」と「有効化条件」を分けて記録します。

## この Phase でも残すもの

Phase 7 でも、次は別 Phase または別設計として残します。

- full text search / content indexing
- collaborative editing engine
- desktop sync client
- mobile offline sync
- customer managed keys
- per-region data residency UI
- advanced legal discovery workflow
- cross-tenant data clean room

これらは Drive product として重要ですが、Phase 7 の中心である「高度な管理閲覧、workspace、storage、inspection、plan、HA/DR、M2M API」とは分けて扱います。

次に扱う場合は、`TUTORIAL_OPENFGA_P8_DRIVE_PRODUCT_EXPANSION.md` に沿って、検索、共同編集、sync client、CMK、data residency、legal discovery、data clean room を個別の product backlog として実装します。
