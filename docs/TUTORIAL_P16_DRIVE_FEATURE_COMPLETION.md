# P16: Drive 未実装機能 完成チュートリアル

## この文書の目的

この文書は、P15 の Drive / ファイル共有 UX refresh 後に残っている未実装機能を、実装へ落とすためのチュートリアルです。

P15 では Google Drive を参考に、Drive 画面の情報設計、左ナビ、grid / list、share dialog、details panel などを改善しました。一方で、次のような項目は UI placeholder または後続候補として残っています。

- Shared with me
- Starred
- Recent
- Storage usage
- Owner / Source filter
- full folder tree
- activity feed
- public folder browser
- preview / thumbnail
- share role update
- copy / bulk download
- permanent delete

この P16 では、それらを DB、sqlc、backend service、OpenAPI、frontend store、Drive UI、E2E まで順番に接続します。

優先順は、Shared with me、Starred、Recent / Activity、Storage、実 filter、folder tree、共有管理、public folder link、preview / thumbnail、一括操作、完全削除です。日常的に使う Drive 画面を先に成立させ、preview、bulk、chunked resumable upload のような重い処理は後半フェーズで扱います。

この文書は、次の既存チュートリアルの後続です。

- `docs/TUTORIAL.md`
- `docs/TUTORIAL_SINGLE_BINARY.md`
- `docs/TUTORIAL_OPENFGA_P5_UI_E2E.md`
- `docs/TUTORIAL_P15_DRIVE_SHARE_UX_REFRESH.md`
- `docs/TUTORIAL_FILE_SHARE.md`

## 前提と現在地

このチュートリアルは、少なくとも次の状態にある前提で進めます。

- P15 の Drive layout が実装済み
- `/drive` に Drive 専用 left nav、command bar、grid / list、details panel がある
- OpenFGA Drive P1-P9 が実装済み
- `drive_resource_shares`、`drive_share_links`、`drive_external_invitations` が存在する
- soft delete / restore が実装済み
- SeaweedFS S3-compatible storage driver は導入済み
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary`、`make e2e` が標準確認コマンドとして使える

この P16 では、OpenFGA を authorization の正本として維持します。DB は listing、metadata、UX state、activity、storage usage の正本です。

### やらないこと

- Google Drive のロゴ、文言、配色、アイコン、アセットを複製しない
- authorization を frontend state だけで代替しない
- `backend/internal/db/*`、`frontend/src/api/generated/*`、`openapi/openapi.yaml` を手書き編集しない
- `backend/web/dist/*` を source として編集しない
- preview / thumbnail generation で plaintext が必要な処理を、E2EE file に無条件で適用しない

## P15 後に残っている未実装機能

P15 後の主な未実装は次です。

| 機能 | 現在地 | P16 での扱い |
| --- | --- | --- |
| Shared with me | 左ナビはあるが disabled / empty | share metadata と OpenFGA check を通して list API を作る |
| Starred | 左ナビはあるが disabled | file/folder 共通の starred metadata を作る |
| Recent | Search への導線のみ | activity / last viewed を保存して recent API を作る |
| Storage | quota placeholder | DB byte size を正本に usage API を作る |
| Owner / Source filter | UI state 中心 | API query と response metadata に接続する |
| folder tree | current children 中心 | actor が見える folder tree API を作る |
| Activity panel | placeholder | audit / activity feed API を接続する |
| Public folder link | metadata 表示のみ | public children browsing を追加する |
| Preview / thumbnail | MIME placeholder | image / PDF / text から段階的に接続する |
| Share management | create / delete 中心 | role update、target search、owner transfer を追加する |
| Upload / metadata | 単一 upload 中心 | multi-upload、drag and drop、progress、retry、description、tags を追加する |
| Bulk / copy | 未実装 | copy、archive download、multi-upload を追加する |
| Permanent delete | API / UI 不足 | trash からの完全削除を retention-aware にする |

## 完成条件

このチュートリアルの完了条件は次です。

- `/drive/shared` で自分に共有された file / folder が表示される
- `/drive/starred` で star 済み file / folder が表示される
- `/drive/recent` で最近開いた、更新した、共有された item が表示される
- Drive 左ナビの Shared with me、Starred、Recent、Storage が disabled ではない
- Storage summary が quota、used、trash、file count を表示する
- Owner / Source filter が backend query に接続される
- folder tree が root 直下だけでなく階層移動に使え、owned folder tree と shared folder roots を区別して表示できる
- details panel の Activity が実データを表示する
- share dialog で user / group search、role update、current access 確認ができる
- public folder link で folder children を閲覧できる
- thumbnail / preview は image、PDF、text の最小対応がある
- multi-upload、drag and drop、progress、retry が使える
- description と tags を file / folder metadata として扱える
- copy、folder archive download、permanent delete の API と UI がある
- destructive action は確認 dialog を通る
- desktop / tablet / mobile で Drive layout が重ならない
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary`、`make e2e`、`make smoke-openfga` が通る

## 仕様との対応表

| 仕様 / 既存文書 | P16 で実装する領域 |
| --- | --- |
| `FILE_SHARE_SPEC.md` の file listing | Shared with me、Starred、Recent、filter、sort、folder tree |
| `FILE_SHARE_SPEC.md` の sharing | share target search、role update、owner transfer、public folder link |
| `FILE_SHARE_SPEC.md` の storage | storage usage、trash bytes、quota display、SeaweedFS drift は health 側 |
| `FILE_SHARE_SPEC.md` の preview | image / PDF / text preview、thumbnail metadata、E2EE policy |
| `docs/TUTORIAL_FILE_SHARE.md` | Drive 全体の file sharing 実装順 |
| `docs/TUTORIAL_OPENFGA_P5_UI_E2E.md` | UI から OpenFGA authorization を確認する E2E |
| `docs/TUTORIAL_P15_DRIVE_SHARE_UX_REFRESH.md` | P16 で接続する placeholder / follow-up 一覧 |
| `docs/TUTORIAL_SINGLE_BINARY.md` | build 後に embedded frontend で同じ画面を確認する |

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | inventory | P15 placeholder と既存 API / DB を棚卸しする |
| Step 2 | DB / sqlc metadata | starred、activity、preview、usage 用 metadata を追加する |
| Step 3 | Shared with me | 自分に共有された item list を実装する |
| Step 4 | Starred | file / folder 共通 star を実装する |
| Step 5 | Recent / Activity | recent list と details panel activity を接続する |
| Step 6 | Storage usage | quota / used / trash bytes を返す |
| Step 7 | filters / folder tree | query filter と folder tree API を実装する |
| Step 8 | share management | target search、role update、owner transfer を実装する |
| Step 9 | public folder / preview | public folder browsing と thumbnail / preview を追加する |
| Step 10 | upload / metadata / bulk / copy / permanent delete | Drive の日常操作を補完する |
| Step 11 | frontend wiring | routes、store、components、empty/error state を接続する |
| Step 12 | E2E / smoke / binary | OpenFGA、browser、single binary で確認する |

## Step 1. inventory を作る

### 対象ファイル

```text
docs/TUTORIAL_P15_DRIVE_SHARE_UX_REFRESH.md
FILE_SHARE_SPEC.md
frontend/src/views/DriveView.vue
frontend/src/components/DriveSideNav.vue
frontend/src/components/DriveCommandBar.vue
frontend/src/components/DriveDetailsPanel.vue
frontend/src/components/DriveShareDialog.vue
frontend/src/stores/drive.ts
frontend/src/api/drive.ts
backend/internal/api/drive_*.go
backend/internal/service/drive_*.go
db/migrations/*
db/queries/drive_*.sql
e2e/drive.spec.ts
```

### 確認コマンド

```bash
rg -n "Shared with me|Starred|Recent|Storage|placeholder|disabled|Activity|thumbnail|public folder" docs frontend/src backend/internal db e2e
rg -n "shared_with_me|star|favorite|recent|quota|preview|thumbnail|permanent|bulk|copy|owner transfer" FILE_SHARE_SPEC.md docs frontend/src backend/internal db
rg -n "/api/v1/drive|operationId:" backend/internal/api openapi/openapi.yaml frontend/src/api/drive.ts
```

### 見るポイント

- UI に表示されているが API に接続されていない項目
- DB にはあるが service / API / frontend に出ていない項目
- OpenFGA tuple はあるが list API がない項目
- E2E にまだない user journey
- generated file と手書き file の境界

inventory の結果は、PR description または実装メモにまとめます。専用ファイルを増やす必要はありません。

## Step 2. DB / sqlc metadata を追加する

### 対象ファイル

```text
db/migrations/0021_drive_feature_completion.up.sql
db/migrations/0021_drive_feature_completion.down.sql
db/queries/drive_items.sql
db/queries/drive_activity.sql
db/queries/drive_starred.sql
db/queries/drive_storage.sql
db/queries/drive_tags.sql
backend/sqlc.yaml
```

### 追加する DB 領域

file / folder 共通 item metadata は、resource type と resource id を持つ形で扱います。

```text
drive_starred_items
drive_item_activities
drive_file_previews
drive_item_tags
```

`drive_starred_items` は actor 単位の UX state です。

- tenant id
- user id
- resource type: `file` / `folder`
- resource id
- created at
- unique tenant + user + resource

`drive_item_activities` は recent と details panel の正本です。

- tenant id
- actor user id
- resource type
- resource id
- action: `viewed` / `downloaded` / `uploaded` / `updated` / `renamed` / `moved` / `shared` / `unshared` / `deleted` / `restored`
- metadata jsonb
- created at

`drive_file_previews` は preview / thumbnail の job state です。

- tenant id
- file id
- status: `pending` / `ready` / `failed` / `skipped`
- thumbnail storage key
- preview storage key
- content type
- error code
- generated at

upload / overwrite 完了後は thumbnail generation job を enqueue し、job が成功したら `ready`、対象外または policy により処理しない場合は `skipped`、失敗時は `failed` にします。

`drive_item_tags` は file / folder 共通の tag metadata です。

- tenant id
- resource type: `file` / `folder`
- resource id
- tag
- created by user id
- created at
- unique tenant + resource + normalized tag

file / folder の description は、既存 file / folder metadata の update API に追加します。tags は `drive_item_tags` を正本にし、list / search / details response では tag 配列として返します。

### 注意

- file / folder の authorization は DB flag ではなく OpenFGA check を通す
- E2EE file は preview status を `skipped` にする
- storage usage は SeaweedFS API に問い合わせるのではなく DB の byte size を正本にする
- SeaweedFS 側の実容量確認は health / drift check として別扱いにする
- sqlc generated code は `make gen` で更新する

### 確認コマンド

```bash
make gen
go test ./backend/internal/db ./backend/internal/service
```

この Step の確認では、migration up/down、`db/schema.sql`、`backend/internal/db/*`、`openapi/openapi.yaml`、`frontend/src/api/generated/*` の差分が正本由来になっていることも確認します。

## Step 3. Shared with me を実装する

### 追加 API

```text
GET /api/v1/drive/shared-with-me
```

### `DriveItemBody` response 方針

既存の `DriveItemBody` を拡張し、次を含めます。

```text
ownedByMe
sharedWithMe
ownerUserPublicId
ownerDisplayName
shareRole
source
starredByMe
description
tags
```

### service 方針

Shared with me は、次を統合して返します。

- current actor に直接 share された file / folder
- actor が所属する group に share された file / folder
- accepted external invitation により actor がアクセスできる file / folder

一覧を返す直前に OpenFGA check を通し、削除済み item、期限切れ invitation、revoked share は除外します。

### frontend 方針

- `/drive/shared` route を追加する
- `DriveSideNav` の Shared with me を disabled から router link に変える
- `DriveView` は route mode に応じて `loadSharedWithMe` を呼ぶ
- empty state は「共有されたファイルはありません」とし、primary action は置かない

### E2E

- user A が folder と file を作る
- user A が user B に viewer share する
- user B で login し、Shared with me に item が出る
- user B は viewer 操作だけでき、rename / delete はできない

## Step 4. Starred を実装する

### 追加 API

```text
GET /api/v1/drive/starred
POST /api/v1/drive/files/{filePublicId}/star
DELETE /api/v1/drive/files/{filePublicId}/star
POST /api/v1/drive/folders/{folderPublicId}/star
DELETE /api/v1/drive/folders/{folderPublicId}/star
```

### service 方針

- star は user personal metadata
- viewer 以上の権限がある item だけ star できる
- 権限を失った item は Starred 一覧に出さない
- item が soft delete されたら Starred 一覧には出さない
- item restore 後、まだ権限があれば Starred に戻ってよい

### frontend 方針

- item menu に `Add to starred` / `Remove from starred` を追加する
- grid / list item に star state を表示する
- `/drive/starred` route を追加する
- optimistic update は行ってよいが、失敗時は元に戻して action 位置に error を出す

### E2E

- My Drive の file を star する
- Starred view に出る
- unstar すると Starred view から消える
- shared item も権限があれば star できる

## Step 5. Recent / Activity を実装する

### 追加 API

```text
GET /api/v1/drive/recent
GET /api/v1/drive/files/{filePublicId}/activity
GET /api/v1/drive/folders/{folderPublicId}/activity
```

### 記録する action

```text
viewed
downloaded
uploaded
updated
renamed
moved
shared
unshared
deleted
restored
previewed
```

### service 方針

- API の user-facing action 完了後に activity を best effort で記録する
- failed request は activity に入れない
- recent list は actor がアクセス可能な item だけ返す
- repeated viewed event は短時間に増えすぎないように debounce する
- audit log は compliance 用、activity は UX 用として分ける

### frontend 方針

- `/drive/recent` route を追加する
- details panel の Activity tab を placeholder から API 接続に変える
- activity item は action、actor、time、target を compact に表示する
- loading skeleton と empty state を入れる

### E2E

- file を開く / download する
- Recent view に表示される
- details panel Activity に download event が表示される

## Step 6. Storage usage を実装する

### 追加 API

```text
GET /api/v1/drive/storage
```

### response

```json
{
  "quotaBytes": 10737418240,
  "usedBytes": 123456789,
  "trashBytes": 12345,
  "fileCount": 42,
  "trashFileCount": 3,
  "storageDriver": "seaweedfs_s3"
}
```

### service 方針

- `usedBytes` は active file body の byte size 合計
- `trashBytes` は trashed file body の byte size 合計
- quota は workspace / tenant settings 由来
- storage usage は DB の file byte size を正本にする
- SeaweedFS の object-level drift は operations health 側で扱う

### frontend 方針

- `DriveSideNav` の Storage placeholder を usage bar に置き換える
- quota が未設定の場合は used と file count だけ表示する
- Storage route は最初は summary panel だけでよい

### E2E

- file upload 後に used bytes が増える
- trash 後に trash bytes が増える
- restore 後に trash bytes が戻る

## Step 7. filters / folder tree を実装する

### 追加 / 拡張 API

```text
GET /api/v1/drive/items
GET /api/v1/drive/search
GET /api/v1/drive/folder-tree
```

### query

```text
type=all|file|folder
owner=all|me|shared_with_me|user:{publicId}
source=all|upload|external|generated|sync
sort=name|updated_at|size
direction=asc|desc
limit
cursor
```

### service 方針

- list / search の filter は backend で処理する
- cursor pagination を前提にし、offset 前提の UI にしない
- folder tree は actor が閲覧可能な folder だけ返す
- response は actor が owner の owned folder tree と、共有で見えている shared folder roots を分ける
- shared folder roots は My Drive tree とは別の root として返してよい

### frontend 方針

- `DriveCommandBar` の Owner / Source filter を API query に接続する
- filter state は route query に反映する
- folder tree は expand / collapse state を持つ
- mobile では folder tree を collapsible panel にする

### E2E

- type=file で folder が消える
- owner=shared_with_me で shared item だけ出る
- folder tree から child folder に移動できる

## Step 8. share management を実装する

### 追加 API

```text
GET /api/v1/drive/share-targets?q=
PATCH /api/v1/drive/files/{filePublicId}/shares/{sharePublicId}
PATCH /api/v1/drive/folders/{folderPublicId}/shares/{sharePublicId}
POST /api/v1/drive/files/{filePublicId}/owner-transfer
POST /api/v1/drive/folders/{folderPublicId}/owner-transfer
```

### service 方針

- share target search は tenant users と drive groups を返す
- share role update は owner / editor / viewer の遷移を検証する
- owner transfer は current owner のみ許可する
- owner transfer は OpenFGA owner tuple と DB ownership metadata を同じ transaction 境界で扱う
- external invitation は既存 policy を維持する

### frontend 方針

- `DriveShareDialog` の User public ID 直接入力を target search に置き換える
- people / groups の current access を role selector 付きで表示する
- owner transfer は destructive 相当として確認 dialog を通す
- link copy は copied state を表示し続ける

### E2E

- user search で share target を選択できる
- viewer から editor へ role update できる
- share revoke 後、Shared with me から消える

## Step 9. public folder link / preview / thumbnail を実装する

### 追加 API

```text
GET /api/public/drive/share-links/{token}/children
GET /api/v1/drive/files/{filePublicId}/thumbnail
GET /api/v1/drive/files/{filePublicId}/preview
```

### public folder 方針

- password、expiry、disabled、role は既存 share link policy を再利用する
- public folder children は folder viewer role 以上で許可する
- public folder browser では owner / internal user id を出さない
- public link token をログに出さない

### preview / thumbnail 方針

最初の対応範囲は次に限定します。

- image: thumbnail と preview
- PDF: preview metadata と browser render
- text: sanitized text preview

Office file は既存 Office session / provider 境界に接続します。video、audio、AI summary はこの Step の後続拡張です。

E2EE file、policy disabled file、unsupported MIME type は `skipped` として扱います。

### frontend 方針

- `DriveFileThumbnail` を MIME placeholder から thumbnail URL 対応にする
- public share view に folder browser を追加する
- preview がない場合は既存 file type icon を維持する

### E2E

- public folder link で children が見える
- password protected public folder link が password 入力後に開く
- image upload 後に thumbnail placeholder から thumbnail 表示へ変わる

## Step 10. upload / metadata / bulk / copy / permanent delete を実装する

### 追加 API

```text
POST /api/v1/drive/files/{filePublicId}/copy
POST /api/v1/drive/folders/{folderPublicId}/copy
POST /api/v1/drive/downloads/archive
DELETE /api/v1/drive/files/{filePublicId}/permanent
DELETE /api/v1/drive/folders/{folderPublicId}/permanent
```

### upload 方針

- 最初は existing upload API を使って multi-upload と progress UI を作る
- 小さいファイルは既存 upload API を並列実行する
- drag and drop は current folder に upload する
- failed upload は retry できる
- 大容量・中断再開は chunked resumable upload として P16 後半フェーズまたは別 PR に分離する

### metadata 方針

- file / folder update API に `description` を追加する
- tags は `drive_item_tags` で file / folder 共通管理にする
- list / grid / details panel では description、tags、owner、source、shared state、starred state、locked state を表示できる response にする
- tags の入力は paste を妨げず、重複と空文字を backend で正規化する

### copy 方針

- file copy は storage object を新しい key に複製する
- folder copy は descendants を含む recursive copy
- copied item は requester が owner
- share / star / activity は copy 先に継承しない

### archive download 方針

- folder / multiple items を ZIP として stream する
- archive は authorization check 後に生成する
- 大容量 archive は後続で async export に切り替える余地を残す

### permanent delete 方針

- trash 内 item のみ permanent delete 可能
- owner のみ許可
- legal hold / retention / active descendant がある場合は拒否
- storage object delete は idempotent に扱う

### frontend 方針

- item menu に Star、Copy、Download ZIP、Move、Details、Share、Delete permanently を整理して追加する
- Thumbnail、shared state、starred state、locked state、source、owner を grid / list の両方で表示する
- permanent delete は確認 dialog を必須にする
- multi-upload progress は command area または bottom queue に出す

### E2E

- drag and drop / file input で複数 file を upload できる
- description と tags を編集できる
- file copy が同じ folder に作られる
- folder ZIP download が開始できる
- trash から permanent delete すると restore できない

## Step 11. frontend routes / store / UI wiring を行う

### 対象ファイル

```text
frontend/src/router/index.ts
frontend/src/stores/drive.ts
frontend/src/api/drive.ts
frontend/src/views/DriveView.vue
frontend/src/views/PublicDriveShareView.vue
frontend/src/components/DriveSideNav.vue
frontend/src/components/DriveCommandBar.vue
frontend/src/components/DriveDetailsPanel.vue
frontend/src/components/DriveItemMenu.vue
frontend/src/components/DriveShareDialog.vue
frontend/src/components/DriveFileThumbnail.vue
frontend/src/style.css
```

### route

```text
/drive
/drive/folders/:folderPublicId
/drive/shared
/drive/starred
/drive/recent
/drive/search
/drive/trash
/drive/storage
```

### store

`drive.ts` に次の action を追加します。

```text
loadSharedWithMe
loadStarred
loadRecent
loadStorageUsage
toggleStar
loadFolderTree
loadActivity
searchShareTargets
updateShareRole
transferOwner
copyItem
downloadArchive
deletePermanently
updateItemMetadata
```

### UI rules

- icon-only button には `aria-label` を付ける
- destructive action は確認 dialog を通す
- error は action を行った場所の近くに出す
- loading は structural skeleton を優先する
- dense UI では text overflow に `truncate` 相当の class を使う
- animation は必須でなければ追加しない
- accent color は active nav、primary action、focus に限定する

## Step 12. E2E / smoke / single binary で確認する

### 追加する E2E scenario

```text
Shared with me
Starred
Recent / Activity
Storage usage
Owner / Source filter
folder tree navigation
share target search and role update
public folder link
thumbnail / preview fallback
multi-upload progress
copy and archive download
permanent delete
mobile Drive layout
```

### backend test scenario

```text
Shared with me: user share、group share、revoked share、trashed item、folder share root
Starred: star / unstar、deleted item、permission loss
Recent / Activity: view、download、update、share、delete、restore
Storage: upload、update、delete、restore、permanent delete
Public folder link: password、expiry、disabled、folder children、download denial
Permanent delete: owner only、retention block、legal hold block、non-trashed block、active descendants block
```

### frontend test scenario

```text
shared / starred / recent / storage routes の empty / loading / error / success state
filter query sync
grid / list toggle
folder tree expand / collapse
share target search
role update
owner transfer confirmation
drag and drop multi-upload progress and retry
```

### 確認コマンド

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
make e2e
make smoke-openfga
```

`make gen` 後は、generated OpenAPI と frontend SDK の差分が API 変更と一致していることを確認します。single binary では Drive UI、API、OpenAPI static artifacts が同じ binary から返ることを確認します。

OpenFGA E2E は環境変数が必要な場合があります。既存の `docs/TUTORIAL_OPENFGA_P5_UI_E2E.md` に従い、OpenFGA server、store、model、tenant seed を揃えてから実行します。

## 実装単位と前提

P16 は差分が大きいため、1 commit でまとめず複数 commit に分けます。推奨順は次です。

```text
1. backend foundation: migration、sqlc query、service type
2. API / codegen: Huma endpoint、OpenAPI、frontend SDK
3. frontend wiring: routes、store、Drive UI 接続
4. E2E: Shared with me、Starred、Recent、Storage を先に固定
5. preview / bulk: thumbnail、public folder、copy、archive、permanent delete
```

OpenFGA は authorization の正本として維持し、DB は listing、metadata、UX state の正本にします。resumable upload は大きいため、最初は multi-upload、progress、retry を完成条件にし、chunked resumable upload は P16 後半フェーズまたは別 PR で実装します。

## 生成物と直接編集しないファイル

次のファイルは生成物です。手書きで直しません。

```text
db/schema.sql
backend/internal/db/*
openapi/openapi.yaml
frontend/src/api/generated/*
backend/web/dist/*
bin/haohao
```

生成物を更新する場合は、必ず正本を直してから次のコマンドで再生成します。

```bash
make gen
npm --prefix frontend run build
make binary
```

## 最終確認チェックリスト

- [ ] Shared with me が disabled ではなく、共有 item を表示する
- [ ] Starred が disabled ではなく、star / unstar できる
- [ ] Recent が Search への alias ではなく recent list を表示する
- [ ] Storage が placeholder ではなく usage を表示する
- [ ] Owner / Source filter が API query に反映される
- [ ] folder tree で階層移動できる
- [ ] details panel の Activity が実データを表示する
- [ ] share dialog で target search と role update ができる
- [ ] public folder link で children を閲覧できる
- [ ] thumbnail / preview が image、PDF、text で動く
- [ ] multi-upload と retry が使える
- [ ] description と tags が file / folder metadata として使える
- [ ] copy、archive download、permanent delete が使える
- [ ] generated OpenAPI / frontend SDK の差分が API 変更と一致している
- [ ] `make gen` が通る
- [ ] `go test ./backend/...` が通る
- [ ] `npm --prefix frontend run build` が通る
- [ ] `make binary` が通る
- [ ] `make e2e` が通る
- [ ] `make smoke-openfga` が通る
