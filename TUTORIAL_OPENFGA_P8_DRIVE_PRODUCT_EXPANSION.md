# Phase 8: OpenFGA Drive プロダクト拡張チュートリアル

## この文書の目的

この文書は、`TUTORIAL_OPENFGA_P7_ADVANCED_DRIVE_OPERATIONS.md` の末尾で Phase 7 でも残した Drive product backlog を、次に実装できる順番へ分解したチュートリアルです。

Phase 1-7 では、Drive の基本認可、外部共有、管理 UI、整合性 repair、workspace、object storage、DLP、billing、HA/DR、M2M Drive API までを扱いました。Phase 8 では、その上に載る product / compliance / client ecosystem の機能を扱います。

この Phase の目的は、Drive を単なる file browser から、検索、共同編集、sync client、暗号鍵管理、data residency、legal discovery、cross-tenant collaboration まで扱える product surface に広げることです。

ただし、Phase 8 でも OpenFGA の責務は変えません。OpenFGA は resource relation と permission check を扱い、検索 index、編集 session、sync state、encryption key、legal hold workflow、data clean room policy の source of truth にはしません。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- `TUTORIAL_OPENFGA_P1_INFRA_MODEL.md` から `TUTORIAL_OPENFGA_P7_ADVANCED_DRIVE_OPERATIONS.md` までが実装済み
- Drive file / folder / group / share link / workspace の resource authorization は OpenFGA で判定している
- tenant boundary、workspace boundary、resource state、tenant policy、plan enforcement は OpenFGA check の前に DB で確認している
- object storage driver と signed URL の境界がある
- scan / inspection / DLP の state が file lifecycle に入っている
- billing / subscription / plan enforcement が Drive policy と接続されている
- HA/DR と OpenFGA tuple drift / storage consistency の operational check がある
- browser API、public link API、external bearer API、M2M API の入口が分離されている
- tenant admin の本文横断閲覧や匿名 Editor link のような危険機能は default disabled で管理されている
- `make smoke-openfga` と OpenFGA enabled E2E が通る

この Phase 8 でも、Zitadel / AuthzService / tenant-aware auth の担当境界は変えません。

## この Phase でやること / やらないこと

やること:

```text
検索、共同編集、sync client、CMK、data residency、legal discovery、clean room を個別 backlog として分ける
各 subsystem の source of truth を明示し、Drive permission source を増やさない
検索 index / sync delta / legal export / clean room export の前に DB guard と OpenFGA check を通す
外部 service 依存は adapter 境界、feature flag、env-gated smoke とセットで入れる
高リスク workflow は dedicated role、approval、audit、retention を持つ専用 UI に分ける
```

やらないこと:

```text
検索 index や collaboration service の state を permission の正本にする
sync device token を user permission の代わりに使う
KMS key material、raw storage key、share token、legal export secret を DB / audit / log に残す
legal discovery や clean room を通常 Drive browser / share dialog に混ぜる
Phase 7 の workspace / storage / DLP / external API 境界が固まる前に Phase 8 機能を実装する
```

Phase 8 は product expansion なので、Step 間の依存が強くありません。検索だけ、CMK だけ、legal discovery だけ、という単位で着手して構いません。ただし、各 Step は feature flag、tenant policy、audit、smoke env、rollback 方針を同じ PR に含めます。

## 完成条件

Phase 8 backlog を一通り実装した状態の完成条件は次です。

- full text search / content indexing が Drive permission filter と scan / DLP state を守って検索結果を返せる
- collaborative editing engine の採用境界が決まり、edit session、revision、lock、presence、conflict handling が Drive authz と接続されている
- desktop sync client の device registration、delta sync、conflict resolution、remote wipe、rate limit、audit が定義されている
- mobile offline sync が local cache、offline change queue、token refresh、remote revoke、lost device 対応を持つ
- customer managed keys が tenant / workspace / file encryption scope、key rotation、key loss、audit と接続されている
- per-region data residency UI が tenant / workspace policy、object storage region、index region、OpenFGA / DB placement 方針を明示できる
- advanced legal discovery workflow が legal hold、case、export、chain of custody、audit を扱える
- cross-tenant data clean room が通常共有と分離された policy / workspace / audit / export guard を持つ
- smoke / E2E / operational verification が Phase 8 の高リスク機能を明示 env 付きで確認できる

## 実装順の全体像

| Step | 主題 | 目的 |
| --- | --- | --- |
| Step 1 | full text search / content indexing | permission filter と DLP state を守った検索基盤を作る |
| Step 2 | collaborative editing engine | 共同編集の採用境界と Drive authz 接続を固定する |
| Step 3 | desktop sync client | device / delta / conflict / remote wipe の API 境界を作る |
| Step 4 | mobile offline sync | offline cache と revoke を両立する mobile sync policy を作る |
| Step 5 | customer managed keys | encryption key ownership と key lifecycle を tenant policy に接続する |
| Step 6 | per-region data residency UI | tenant / workspace の region policy を管理 UI で扱う |
| Step 7 | advanced legal discovery workflow | legal hold と discovery export を audit 付きで運用化する |
| Step 8 | cross-tenant data clean room | 通常共有と分離した共同分析領域を設計する |
| Step 9 | smoke / E2E / operational verification | 高リスク product 機能の確認手順を固定する |

## 先に決める方針

### Phase 8 は product surface を広げるが permission source は増やさない

検索 engine、collaboration service、sync client、KMS、legal discovery tool が増えても、Drive resource permission の source of truth は増やしません。

- OpenFGA: relation と permission check
- PostgreSQL: tenant / workspace / resource metadata、policy、state、audit
- object storage: file body と object version
- index service: searchable text と metadata index
- collaboration service: live editing session と revision state
- KMS: encryption key material または external key reference

検索結果、sync delta、legal export、data clean room export は、すべて DB guard と OpenFGA check または permission snapshot を通して返します。

### content indexing は scan / DLP 後に走らせる

感染、blocked、未検査の file body を index しません。

index job は次の条件を満たす file だけを対象にします。

- `purpose = 'drive'`
- `deleted_at is null`
- scan state が `clean` または policy 上 `skipped` 許可
- DLP state が `allowed`
- object version と metadata version が一致

検索 index に raw storage key、share link token、private KMS key id を保存しません。

### client sync は browser API と別 surface にする

desktop / mobile sync client は browser API と違い、長時間 token、device identity、offline queue、bulk delta を扱います。

そのため、sync API は専用 route / OpenAPI spec / rate limit / audit actor type に分けます。

```text
browser API: cookie session + CSRF
sync API: device-bound bearer token + M2M-style scope
public link API: token hash lookup + public rate limit
admin API: tenant admin role + tenant policy guard
```

### CMK と data residency は後戻りが難しい

customer managed keys と data residency は、一度有効化すると migration / disaster recovery / support operation に大きく影響します。

初期は plan-gated / tenant-gated / workspace-gated で default disabled にします。既存 tenant の data placement や encryption mode を migration で暗黙に変えません。

### legal discovery と data clean room は通常 Drive UX に混ぜない

legal discovery と cross-tenant data clean room は、通常の folder browser / share dialog とは別導線にします。

通常共有より強い export、bulk access、cross-tenant processing を扱うため、専用 role、case / room state、理由入力、approval、audit、retention を必須にします。

## 推奨 PR 分割

Phase 8 は 1 Step 1 PR を基本にします。外部 provider を使う Step では、provider adapter と local fake implementation を同じ PR に入れ、CI は local fake で通します。

| PR | 含める Step | merge gate |
| --- | --- | --- |
| 1 | Step 1 | permission-filtered search と index rebuild smoke が通る |
| 2 | Step 2 | lock-based edit / revision が通り、provider-backed realtime は disabled |
| 3 | Step 3 | device token hash、delta cursor、remote revoke が通る |
| 4 | Step 4 | offline replay が server-side permission 再判定を通る |
| 5 | Step 5 | key unavailable 時に download / index / export が fail-closed する |
| 6 | Step 6 | region policy dry-run と placement validation が通る |
| 7 | Step 7 | legal case / hold / export が dedicated role と audit を持つ |
| 8 | Step 8-9 | clean room export denial と operational drill が env-gated で動く |

### この repository での最小実装メモ

今回の Phase 8 実装では、外部 provider をまだ固定しない機能を local fake / DB-backed MVP として閉じます。

- search: Postgres full text search を index service 境界の内側で使う。検索結果は DB guard、scan/DLP guard、OpenFGA check の後に返す。
- collaboration: CRDT / OT provider は入れず、lock-based edit session と revision save までを実装する。
- sync: desktop / mobile client の実 binary は作らず、device-bound token、delta cursor、remote wipe、offline replay API を固定する。
- CMK / residency: 実 KMS や multi-region object store は導入せず、tenant policy、key availability、region placement policy、fail-closed guard を先に実装する。
- legal discovery / clean room: 通常 Drive 共有とは別の tenant admin API と dedicated role で扱い、raw export は default deny にする。

このため、Phase 8 の E2E env は将来の provider-backed UI を追加するときの予約です。この repository では `RUN_DRIVE_*_SMOKE=1` の明示 smoke と `npm --prefix frontend run build` を merge gate にします。

## Step 1. full text search / content indexing を追加する

### 対象 subsystem

```text
db/migrations
db/queries
backend/internal/service/drive_search_service.go
backend/internal/service/drive_index_job.go
backend/internal/api/drive_search.go
backend/internal/platform/metrics.go
frontend/src/views/DriveSearchView.vue
frontend/src/components/drive/DriveSearchResults.vue
ops/worker
E2E
```

### 実装方針

Drive file の本文検索は、DB の `LIKE` ではなく index service を前提にします。

最小構成では Postgres full text index から始めても構いませんが、service 境界は将来の OpenSearch / Meilisearch / Typesense などへ差し替えられる形にします。

追加する DB state の例:

```text
drive_index_jobs
drive_search_documents
drive_search_index_versions
```

`drive_search_documents` は検索用 metadata と extracted text を持ちますが、storage key、share link raw token、password hash、KMS secret は持ちません。

index job は upload / new object version / rename / move / scan clean / DLP allowed の event から非同期に起動します。

### API / DB / UI / authz の境界

検索 API は次の順で処理します。

1. 認証、active tenant、workspace access を確認する
2. query、file type、owner、updated range、shared state の filter を正規化する
3. index service から candidate resource IDs を取得する
4. DB で tenant / workspace / deleted / scan / DLP state を再確認する
5. OpenFGA `BatchCheck(can_view)` で閲覧可能 resource だけ残す
6. response には snippet と metadata だけ返し、content download は既存 download endpoint に任せる

検索 UI は Drive browser の補助として実装し、検索結果から直接 file body を表示しません。preview / download は既存の permission guard を通る導線にします。

検索結果の permission は index 時点の snapshot を信頼しません。index には candidate を返すための metadata だけを置き、response 直前の DB guard と OpenFGA `BatchCheck` を必須にします。large result set では上限件数ごとに chunk し、permission-filter 後に不足した分だけ追加 candidate を取り直します。

### audit / metrics / security 注意点

- `drive.search.query` audit は query raw text を保存しない。必要なら hash または redacted summary にする
- snippet は permission filter 後の file だけ返す
- index lag、failed extraction、permission-filtered result count を metrics に出す
- tenant をまたいだ index shard を使う場合も response 前に DB tenant guard を必ず通す
- DLP blocked file は index から削除または非表示にする

### 確認コマンド

```bash
go test ./backend/internal/service -run 'DriveSearch|DriveIndex'
go test ./backend/internal/api -run DriveSearch
RUN_DRIVE_SEARCH_SMOKE=1 make smoke-openfga
npm --prefix frontend run build
RUN_DRIVE_SEARCH_E2E=1 make e2e
```

## Step 2. collaborative editing engine の境界を決める

### 対象 subsystem

```text
backend/internal/service/drive_collaboration_service.go
backend/internal/api/drive_collaboration.go
backend/internal/service/drive_revision_service.go
db/migrations
frontend/src/views/DriveEditorView.vue
frontend/src/components/drive/DrivePresenceList.vue
frontend/src/components/drive/DriveRevisionHistory.vue
openfga/drive.fga
E2E
```

### 実装方針

共同編集は、最初から汎用 CRDT / OT engine を自作しません。text document、markdown、office document など file type ごとに採用 engine を明示します。

初期は次のように段階を分けます。

```text
level 0: no collaborative editing
level 1: lock-based edit + revision history
level 2: provider-backed realtime collaboration
level 3: offline collaborative merge
```

Phase 8 では level 1 を最小実装にし、level 2 以降は provider / engine の境界を作ってから進めます。

追加する DB state の例:

```text
drive_file_revisions
drive_edit_sessions
drive_edit_locks
drive_presence_sessions
```

lock は lease として扱います。`drive_edit_locks` には owner、expires_at、base_revision、last_heartbeat_at を持たせ、期限切れ lock は cleanup job で解除できます。save 時は lock の存在だけで許可せず、current revision、resource state、OpenFGA `can_edit` を再確認します。

### API / DB / UI / authz の境界

編集開始時は `can_edit` を OpenFGA で確認します。編集中も長時間 session を信頼し続けず、heartbeat または save 時に resource state と permission を再確認します。

編集保存は次を守ります。

1. file が deleted / locked / legal hold / retention blocked でない
2. scan / DLP policy が編集後 content に適用される
3. user が `can_edit` を持つ
4. expected revision と current revision が一致する、または conflict resolution path に入る
5. object storage に new version を保存する
6. DB revision row と audit を残す
7. index / scan job を enqueue する

UI は Drive browser と分離した editor route にします。Viewer は editor route へ入れても read-only 表示にし、save / lock / collaboration control は表示しません。

### audit / metrics / security 注意点

- `drive.file.edit_session.started`、`drive.file.revision.created`、`drive.file.conflict.detected` を audit に追加する
- revision diff に sensitive content を audit へ保存しない
- presence data は短期 retention にする
- anonymous Editor link が disabled の場合、public editor route を出さない
- lock timeout、stale session cleanup、revision count / storage growth を metrics に出す

### 確認コマンド

```bash
go test ./backend/internal/service -run 'DriveCollaboration|DriveRevision'
go test ./backend/internal/api -run DriveCollaboration
RUN_DRIVE_COLLAB_SMOKE=1 make smoke-openfga
RUN_DRIVE_COLLAB_E2E=1 make e2e
```

## Step 3. desktop sync client の土台を作る

### 対象 subsystem

```text
backend/internal/api/drive_sync.go
backend/internal/service/drive_sync_service.go
backend/internal/service/drive_device_service.go
db/migrations
openapi/external
cmd/drive-sync-smoke
docs/drive-sync.md
ops/rate-limit
```

### 実装方針

desktop sync client は browser session と違い、device identity と delta cursor を持ちます。

追加する DB state の例:

```text
drive_devices
drive_sync_cursors
drive_sync_events
drive_sync_conflicts
drive_remote_wipe_requests
```

sync API は次の endpoint から始めます。

```text
POST /api/v1/drive-sync/devices/register
POST /api/v1/drive-sync/devices/:deviceId/heartbeat
GET  /api/v1/drive-sync/delta
POST /api/v1/drive-sync/upload
POST /api/v1/drive-sync/conflicts/:conflictId/resolve
POST /api/v1/drive-sync/devices/:deviceId/revoke
```

delta event は DB の metadata change と object version change から作ります。OpenFGA tuple の変化も、アクセス喪失 event として client に伝えます。

delta cursor は monotonic で再開可能にします。client が持つ cursor は tamper-proof な opaque value にし、server では `drive_sync_cursors` と照合します。cursor が古すぎる場合は full resync を要求し、権限喪失済み resource は tombstone または remote-delete event として返します。

### API / DB / UI / authz の境界

device registration は Zitadel login 済み user に紐づけますが、file permission は device ではなく user に対して判定します。

delta response は次を守ります。

1. device token が active
2. user が active
3. tenant / workspace が active
4. resource が deleted でない、または tombstone event として返す
5. OpenFGA `BatchCheck(can_view)` を通す
6. download URL は `can_download` を通したものだけ返す

desktop sync は bulk operation になりやすいため、browser API と同じ endpoint を使い回しません。

### audit / metrics / security 注意点

- device public ID、device name、last IP、last user agent を audit metadata に残す
- raw device token は DB では hash、log / audit には出さない
- stolen device の revoke と remote wipe request を tenant admin UI から実行できるようにする
- sync delta size、conflict count、denied item count、remote wipe pending count を metrics に出す
- rate limit は user、device、tenant の 3 軸でかける

### 確認コマンド

```bash
go test ./backend/internal/service -run 'DriveSync|DriveDevice'
go test ./backend/internal/api -run DriveSync
RUN_DRIVE_DESKTOP_SYNC_SMOKE=1 make smoke-openfga
```

## Step 4. mobile offline sync を設計する

### 対象 subsystem

```text
backend/internal/api/drive_mobile_sync.go
backend/internal/service/drive_mobile_sync_service.go
backend/internal/service/drive_device_service.go
db/migrations
docs/mobile-offline-sync.md
mobile client spec
```

### 実装方針

mobile offline sync は desktop sync よりも device loss、token refresh、OS storage protection、background execution の制約が強くなります。

desktop sync と同じ server-side primitive を使いつつ、mobile 専用 policy を追加します。

```text
offlineCacheAllowed
offlineCacheMaxBytes
offlineCacheMaxDays
mobileDownloadRequiresBiometric
mobileRemoteWipeRequired
```

offline change queue は upload / rename / move / delete / comment など operation type ごとに扱います。conflict 発生時は server を勝たせるのか、user merge を求めるのかを resource type ごとに決めます。

### API / DB / UI / authz の境界

mobile client が offline 中に操作できるのは、最後に permission snapshot を取得した resource だけです。

online 復帰時は snapshot を信頼せず、server が再判定します。

1. device token と user state を確認する
2. offline operation の base revision を確認する
3. DB resource state を確認する
4. OpenFGA `Check(can_edit)` または `Check(can_delete)` を行う
5. policy 上 offline edit が許可されているか確認する
6. 成功なら revision / object version を作る
7. 失敗なら conflict / denied operation として client に返す

### audit / metrics / security 注意点

- offline cache policy は tenant admin と user device settings の両方に見えるようにする
- mobile lost device revoke は immediate deny にする。client が次回接続したとき remote wipe command を返す
- offline operation の raw content を audit に入れない
- failed replay、conflict、permission changed since snapshot を metrics に出す
- biometric required など OS 依存機能は backend policy と client capability を分けて扱う

### 確認コマンド

```bash
go test ./backend/internal/service -run 'DriveMobile|DriveOffline'
go test ./backend/internal/api -run DriveMobile
RUN_DRIVE_MOBILE_OFFLINE_SMOKE=1 make smoke-openfga
```

## Step 5. customer managed keys を導入する

### 対象 subsystem

```text
backend/internal/service/drive_encryption_service.go
backend/internal/service/kms_service.go
backend/internal/api/tenant_drive_security.go
db/migrations
object storage driver
tenant admin UI
ops/key-rotation
E2E
```

### 実装方針

customer managed keys は、tenant または workspace が暗号鍵の control を持つ機能です。

初期実装では actual key material を HaoHao DB に保存しません。DB には external KMS key reference、encryption mode、key version、last verified state を保存します。

追加する DB state の例:

```text
drive_encryption_policies
drive_kms_keys
drive_object_key_versions
drive_key_rotation_jobs
```

encryption scope は最初に固定します。

```text
service_managed
tenant_managed
workspace_managed
file_managed
```

MVP は `service_managed` と `tenant_managed` に絞るのが現実的です。

key loss policy も最初に固定します。customer managed key が disabled / unavailable / deleted になった場合、HaoHao は復号を代行できない前提で fail-closed にします。UI には復旧に必要な external KMS 側の action を表示しますが、key material の upload や copy は受け付けません。

### API / DB / UI / authz の境界

upload 時は tenant / workspace policy から encryption mode と key reference を解決し、object storage driver へ渡します。

download 時は次を確認します。

1. user が `can_view` と必要なら `can_download` を持つ
2. file state が download 可能
3. key state が active
4. key version が object version と整合している
5. signed URL または stream response に encryption context を渡す

tenant admin UI は key reference の登録、検証、rotation request、disable を扱います。key secret は表示しません。

### audit / metrics / security 注意点

- key id は masked 表示にし、secret / credential は audit に入れない
- key unavailable の場合は fail-closed。download / preview / index / legal export を拒否する
- rotation は resumable job にし、途中失敗時に old key と new key の state を明確にする
- key deletion は immediate ではなく scheduled disable / grace period を置く
- KMS latency、key verify failure、rotation progress を metrics に出す

### 確認コマンド

```bash
go test ./backend/internal/service -run 'DriveEncryption|KMS'
go test ./backend/internal/api -run TenantDriveSecurity
RUN_DRIVE_CMK_SMOKE=1 make smoke-openfga
RUN_DRIVE_CMK_E2E=1 make e2e
```

## Step 6. per-region data residency UI を追加する

### 対象 subsystem

```text
backend/internal/api/tenant_drive_residency.go
backend/internal/service/drive_residency_service.go
db/migrations
object storage driver
index service
tenant admin UI
ops/region-migration
```

### 実装方針

data residency は tenant / workspace / object / index / backup / audit の配置を制御する機能です。

初期は、region を storage bucket の選択だけに閉じず、どの subsystem がどの region policy に従うかを明示します。

追加する DB state の例:

```text
drive_region_policies
drive_workspace_region_overrides
drive_region_migration_jobs
drive_region_placement_events
```

policy の例:

```json
{
  "primaryRegion": "ap-northeast-1",
  "allowedRegions": ["ap-northeast-1"],
  "replicationMode": "none",
  "indexRegion": "same_as_primary",
  "backupRegion": "same_jurisdiction"
}
```

### API / DB / UI / authz の境界

tenant admin UI は region policy の表示と変更 request を扱います。既存 data を移す操作は即時反映ではなく migration job にします。

upload 時は policy に従って storage region を決めます。search index、preview cache、scan sandbox、legal export も region policy を参照します。

OpenFGA tuple は global store のままにするか、region ごとに分けるかを運用設計として明示します。Phase 8 の標準は、OpenFGA store は環境単位 1 つを維持し、tenant / workspace region は DB と storage placement で制御します。

### audit / metrics / security 注意点

- region policy change は `drive.residency.policy.updated` として audit に残す
- region migration は dry-run、approval、progress、rollback plan を持つ
- forbidden region への object write は service guard で止める
- index / preview / scan temporary file も policy 対象にする
- region mismatch count、migration lag、placement failure を metrics に出す

### 確認コマンド

```bash
go test ./backend/internal/service -run DriveResidency
go test ./backend/internal/api -run TenantDriveResidency
RUN_DRIVE_RESIDENCY_SMOKE=1 make smoke-openfga
RUN_DRIVE_RESIDENCY_E2E=1 make e2e
```

## Step 7. advanced legal discovery workflow を実装する

### 対象 subsystem

```text
backend/internal/service/drive_legal_discovery_service.go
backend/internal/api/drive_legal_discovery.go
backend/internal/service/drive_export_service.go
db/migrations
tenant admin UI
ops/export-worker
E2E
```

### 実装方針

legal discovery は通常の download / share と別の bulk access workflow です。

case を作り、対象範囲、legal hold、reviewer、export request、approval、chain of custody を持たせます。

追加する DB state の例:

```text
drive_legal_cases
drive_legal_case_resources
drive_legal_holds
drive_legal_exports
drive_legal_export_items
drive_chain_of_custody_events
```

legal hold は delete / complete delete / key rotation / region migration / retention expiry より優先される禁止条件として service guard に入れます。

### API / DB / UI / authz の境界

legal discovery の access は通常の `tenant_admin` だけでは許可しません。

専用 role の例:

```text
legal_admin
legal_reviewer
legal_exporter
```

export は次の順で処理します。

1. case が active
2. actor が legal role を持つ
3. tenant policy が legal discovery を許可している
4. 対象 resource が case scope に入っている
5. file が legal hold または discovery scope に入っている
6. KMS / residency / DLP policy が export を許可する
7. export package を作り、chain of custody event を残す

OpenFGA の通常 `can_view` を bypass するかどうかは明示的に決めます。初期は `can_view` を bypass しない設計にし、break-glass legal export は別 policy と approval を必須にします。

### audit / metrics / security 注意点

- legal export の file content、raw storage key、KMS secret は audit に入れない
- case 作成、scope 変更、hold 設定、export 作成、download、revoke を全て audit に残す
- export package は短命 URL、password / encryption、download count limit を持つ
- legal hold 対象の complete delete は必ず拒否する
- export job duration、item count、denied count、chain event count を metrics に出す

### 確認コマンド

```bash
go test ./backend/internal/service -run 'DriveLegal|DriveExport'
go test ./backend/internal/api -run DriveLegal
RUN_DRIVE_LEGAL_DISCOVERY_SMOKE=1 make smoke-openfga
RUN_DRIVE_LEGAL_DISCOVERY_E2E=1 make e2e
```

## Step 8. cross-tenant data clean room を設計する

### 対象 subsystem

```text
backend/internal/service/drive_clean_room_service.go
backend/internal/api/drive_clean_room.go
db/migrations
openfga/drive.fga
tenant admin UI
external API
ops/clean-room-worker
E2E
```

### 実装方針

cross-tenant data clean room は通常の external share と別物です。

目的は、tenant A と tenant B が raw file を相互共有せず、制限された workspace / processing policy / export approval の中で共同分析することです。

追加する DB state の例:

```text
drive_clean_rooms
drive_clean_room_participants
drive_clean_room_datasets
drive_clean_room_jobs
drive_clean_room_exports
drive_clean_room_policy_decisions
```

OpenFGA model は、通常 Drive resource と clean room resource を混ぜすぎないようにします。

例:

```fga
type clean_room
  relations
    define owner: [user]
    define participant: [user, group#member]
    define reviewer: [user, group#member]
    define can_view: owner or participant or reviewer
    define can_submit_dataset: owner or participant
    define can_approve_export: owner or reviewer
```

Drive file を clean room dataset に追加する時点では、元 file の `can_view` / `can_share` と tenant policy を確認します。clean room の participant に元 file の通常 Drive permission を自動付与しません。

### API / DB / UI / authz の境界

clean room は専用 UI にします。通常 Drive folder tree の中に cross-tenant room を混ぜません。

dataset submit flow:

1. actor の tenant と active tenant を確認する
2. target clean room が active
3. actor が clean room participant
4. source file が actor tenant の resource
5. actor が source file の `can_share` または policy 上 submit 可能な role を持つ
6. DLP / residency / KMS policy が clean room への投入を許可する
7. dataset row を作り、必要なら derived object を別 storage namespace に作る

export flow:

1. clean room job result が ready
2. export reviewer approval がある
3. participant tenant policy が export を許可する
4. DLP と aggregation threshold を満たす
5. export package を作り audit を残す

### audit / metrics / security 注意点

- cross-tenant actor は audit で `actorTenantId` と `resourceTenantId` を分けて保存する
- raw dataset export は default disabled にする
- aggregation threshold、row count threshold、k-anonymity など product policy を明示する
- clean room job sandbox は tenant isolation を持つ
- submitted dataset count、export denied count、cross-tenant policy decision を metrics に出す

### 確認コマンド

```bash
go test ./backend/internal/service -run DriveCleanRoom
go test ./backend/internal/api -run DriveCleanRoom
cd openfga && fga model test --tests drive.fga.yaml
RUN_DRIVE_CLEAN_ROOM_SMOKE=1 make smoke-openfga
RUN_DRIVE_CLEAN_ROOM_E2E=1 make e2e
```

## Step 9. smoke / E2E / operational verification を固定する

### 対象 subsystem

```text
Makefile
scripts/smoke-openfga.sh
e2e
ops/runbooks
ops/prometheus
ops/grafana
docs
```

### 実装方針

Phase 8 の機能は検索 engine、collaboration engine、sync client、KMS、region placement、legal export、clean room など外部依存が多いため、default CI に全部を入れません。

通常確認、env-gated smoke、operational drill に分けます。

### 通常確認

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
cd openfga && fga model test --tests drive.fga.yaml
make smoke-openfga
```

### Phase 8 の明示 smoke

```bash
RUN_DRIVE_SEARCH_SMOKE=1 make smoke-openfga
RUN_DRIVE_COLLAB_SMOKE=1 make smoke-openfga
RUN_DRIVE_DESKTOP_SYNC_SMOKE=1 make smoke-openfga
RUN_DRIVE_MOBILE_OFFLINE_SMOKE=1 make smoke-openfga
RUN_DRIVE_CMK_SMOKE=1 make smoke-openfga
RUN_DRIVE_RESIDENCY_SMOKE=1 make smoke-openfga
RUN_DRIVE_LEGAL_DISCOVERY_SMOKE=1 make smoke-openfga
RUN_DRIVE_CLEAN_ROOM_SMOKE=1 make smoke-openfga
```

### E2E

```bash
RUN_DRIVE_SEARCH_E2E=1 make e2e
RUN_DRIVE_COLLAB_E2E=1 make e2e
RUN_DRIVE_CMK_E2E=1 make e2e
RUN_DRIVE_RESIDENCY_E2E=1 make e2e
RUN_DRIVE_LEGAL_DISCOVERY_E2E=1 make e2e
RUN_DRIVE_CLEAN_ROOM_E2E=1 make e2e
```

### operational drill

```text
search index rebuild drill
collaboration session failover drill
desktop sync remote wipe drill
mobile lost device drill
KMS key unavailable drill
region migration dry-run drill
legal export chain-of-custody drill
clean room export denial drill
```

## 最終確認コマンド

Phase 8 の通常確認:

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
cd openfga && fga model test --tests drive.fga.yaml
make smoke-openfga
```

Phase 8 の高リスク機能確認:

```bash
RUN_DRIVE_SEARCH_SMOKE=1 make smoke-openfga
RUN_DRIVE_COLLAB_SMOKE=1 make smoke-openfga
RUN_DRIVE_DESKTOP_SYNC_SMOKE=1 make smoke-openfga
RUN_DRIVE_MOBILE_OFFLINE_SMOKE=1 make smoke-openfga
RUN_DRIVE_CMK_SMOKE=1 make smoke-openfga
RUN_DRIVE_RESIDENCY_SMOKE=1 make smoke-openfga
RUN_DRIVE_LEGAL_DISCOVERY_SMOKE=1 make smoke-openfga
RUN_DRIVE_CLEAN_ROOM_SMOKE=1 make smoke-openfga
```

各 Step の完了時は `IMPL.md` に、導入した provider / adapter、default policy、明示 smoke env、外部依存の local fake、rollback 手順を追記します。Phase 8 の機能は外部 service に依存しやすいため、「CI で fake を使う確認」と「実 provider を使う operational drill」を分けて記録します。

## Phase 9 で扱うもの

Phase 8 の中心である「検索、共同編集、sync client、CMK、data residency、legal discovery、clean room」から外した次の product 課題は、`TUTORIAL_OPENFGA_P9_DRIVE_PRODUCT_COMPLETION.md` で実装手順まで扱います。

- native Office file co-authoring compatibility
- enterprise eDiscovery provider integration
- customer-managed HSM dedicated deployment
- on-premise storage gateway
- end-to-end encryption with zero-knowledge sharing
- AI-assisted document classification and summarization
- public marketplace integration for Drive apps

Phase 9 では、これらを provider fake、DB guard、OpenFGA check、frontend、audit、smoke、operational drill まで含めて閉じます。
