# P15: Google Drive 参考の Drive / ファイル共有 UX 改善チュートリアル

## この文書の目的

この文書は、Google Drive の画面構造を参考にして、HaoHao の Drive / ファイル共有 UX を改善するための実装チュートリアルです。

目的は Google Drive の見た目、ロゴ、配色、文言、アイコン、アセットを複製することではありません。目的は、添付画像から次の情報設計と操作導線を抽出し、既存の HaoHao frontend に安全に落とすことです。

- Drive 画面内で完結する左ナビゲーション
- Drive 専用検索と filter chips
- grid / list 表示切替
- folder card、file thumbnail、file type icon、共有状態の見える化
- item ごとの三点メニューと keyboard 操作
- share dialog の簡素化、link copy、current access 表示
- details / activity / permissions を見る右パネル

この P15 は、`docs/TUTORIAL_FILE_SHARE.md`、`docs/TUTORIAL_OPENFGA_P5_UI_E2E.md`、`docs/TUTORIAL_P14_FRONTEND_UI_UX_REFRESH.md` の後続フェーズです。P14 で作った global app shell は維持し、Drive 画面の内側だけを Google Drive 風の作業 UI に寄せます。

この文書では backend API、DB schema、OpenAPI contract は原則変更しません。既存 API だけでは実現できない UX は、後続実装候補として明示します。

## 参考画像の分析

### 画面構造

添付画像の Google Drive 画面は、大きく次の領域に分かれています。

| 領域 | 役割 | HaoHao に取り込む観点 |
| --- | --- | --- |
| Drive header | logo、検索、utility actions、account | P14 の topbar を残しつつ、Drive 内検索は content 側に置く |
| 左 Drive nav | New、Home、My Drive、folder tree、Shared、Recent、Starred、Trash、Storage | Drive 内 navigation と folder tree を AppSidebar から分離する |
| main title | My Drive と dropdown | current workspace / current folder の切替に使う |
| filter chips | 種類、ユーザー、最終更新、ソース | search / list query の条件を compact に見せる |
| sort row | 更新日時、sort direction | list/grid 共通の sort state を見せる |
| folder grid | folder card と三点メニュー | folder を table row ではなく scan しやすい card で見せる |
| file grid | thumbnail preview、file icon、file name、三点メニュー | file type と preview を先に認識できるようにする |
| view toggle | list / grid toggle、info button | user preference と details panel の入口にする |
| right rail | calendar / keep など | HaoHao では details / activity / permissions panel に置き換える |

### UX 上の特徴

- primary action は左上の `New` に集約されている
- folder tree は現在地の理解と移動を速くしている
- filter chips は高度検索画面へ行く前の軽い絞り込みになっている
- grid view では folder と file が視覚的に分かれている
- item action は row/button ではなく三点メニューに集約されている
- details panel は常時表示ではなく、必要時だけ右側に出す
- 画面全体は白と淡い neutral gray が中心で、装飾より可読性を優先している

### そのまま取り込まないもの

- Google Drive の logo、brand color、favicon、Google アプリ rail は使わない
- 画像内の日本語文言をそのまま複製しない
- Google 独自の icon shape や proprietary asset は使わない
- P14 の global app shell を Google Drive 風に置き換えない
- backend authorization を UI state だけで代替しない

参考画像は、情報設計、density、操作導線の参考として扱います。

## HaoHao Drive の現在地

現在の Drive UI は P5 と P14 の成果として、次の構成になっています。

- `frontend/src/views/DriveView.vue`
- `frontend/src/components/DriveToolbar.vue`
- `frontend/src/components/DriveItemTable.vue`
- `frontend/src/components/DriveBreadcrumbs.vue`
- `frontend/src/components/DriveShareDialog.vue`
- `frontend/src/components/DrivePermissionsPanel.vue`
- `frontend/src/stores/drive.ts`
- `frontend/src/api/drive.ts`
- `e2e/drive.spec.ts`

現在できていること:

- workspace / folder / file の一覧
- folder 作成
- file upload / download / overwrite
- rename / move / soft delete / restore
- Drive search
- user share / group share / external invitation / share link 作成
- share link 無効化
- permissions panel の direct / inherited 表示
- public share link view
- OpenFGA 有効環境での Drive E2E

現在の UX 上の不足:

- Drive 内の左ナビと folder tree がない
- toolbar が create / upload / search に偏り、filter / sort / view state が分かりにくい
- folder と file が同じ table に並び、Google Drive 型の scan しやすさが不足している
- grid / list の表示切替がない
- share dialog が user / group / external / link の form 群になっており、初見で現在の共有状態を把握しにくい
- link copy が作成直後の input 表示に寄っており、明確な copy action と state が弱い
- details / activity / permissions を画面右側で確認する導線がない
- item action が常時表示の icon button 群で、狭い viewport では情報密度が上がりすぎる

P15 はこれらの UX 不足を改善します。authorization、audit、OpenFGA check は既存 backend を正本にし、UI は操作しやすさと状態表示に集中します。

## 完成条件

このチュートリアルの完了条件は次です。

- `/drive` に Drive 専用 layout があり、P14 の app shell 内で表示される
- Drive 左ナビに My Drive、Shared with me、Recent、Starred、Trash、Storage、folder tree がある
- 左ナビの `New` から folder 作成と file upload を開始できる
- current workspace / current folder / breadcrumbs が画面上部で分かる
- Drive 専用 search box と filter chips がある
- type / owner / modified / source の filter chip が UI state として表現される
- sort key と sort direction が grid / list の両方に効く
- grid / list 表示を切り替えられる
- folder は compact card、file は thumbnail 付き card または list row として表示される
- item action は三点メニューに集約され、keyboard と pointer の両方で使える
- destructive action は既存の `ConfirmActionDialog` を通る
- share dialog は current access、people / groups、link sharing を最初に見せる
- share link は create 後に copy button と copied state を表示する
- permissions は direct / inherited / link / external を区別して表示する
- details panel で metadata、activity、permissions summary を切り替えられる
- loading / empty / forbidden / error state が Drive layout 内で崩れない
- desktop / tablet / mobile で text overflow と layout overlap がない
- `npm --prefix frontend run build`、`go test ./backend/...`、`make e2e`、`make binary` が通る

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | Drive UX inventory | 既存 Drive UI と API で使える情報を棚卸しする |
| Step 2 | Drive 専用 layout | P14 shell の内側に Drive workspace layout を作る |
| Step 3 | 左ナビ / folder tree | Drive 内 navigation と folder 移動を分離する |
| Step 4 | 検索 / filter / sort | Drive 上部の操作面を検索と絞り込み中心にする |
| Step 5 | grid / list view | folder card、file card、list row を実装する |
| Step 6 | item menu / accessibility | 三点メニューと keyboard 操作を安全に作る |
| Step 7 | share UX | share dialog と permissions panel を整理する |
| Step 8 | right details panel | details / activity / permissions の補助面を作る |
| Step 9 | responsive / E2E / binary | desktop/mobile、E2E、single binary を確認する |

## Step 1. Drive UX inventory を作る

### 対象ファイル

```text
frontend/src/views/DriveView.vue
frontend/src/components/DriveToolbar.vue
frontend/src/components/DriveItemTable.vue
frontend/src/components/DriveShareDialog.vue
frontend/src/components/DrivePermissionsPanel.vue
frontend/src/stores/drive.ts
frontend/src/api/drive.ts
e2e/drive.spec.ts
```

### 確認コマンド

```bash
rg -n "DriveView|DriveToolbar|DriveItemTable|DriveShareDialog|DrivePermissionsPanel" frontend/src
rg -n "search|permissions|share|trash|workspace|folder|file" frontend/src/stores/drive.ts frontend/src/api/drive.ts
rg -n "Drive UI|share link|grid|list|permission" e2e
```

### 見るポイント

- 既存 API response に owner、updatedAt、byteSize、status、locked、share state がどこまで含まれるか
- frontend store に filter / sort / view mode を置けるか
- public ID 入力による move を folder picker に置き換える余地があるか
- share dialog が一画面に詰め込みすぎていないか
- E2E で維持すべき既存 journey が何か

inventory の結果、API が不足していてもこの P15 では backend をすぐ拡張しません。まず UI state と既存情報で改善できる範囲を実装し、不足は「後続実装候補」に分けます。

## Step 2. Drive 専用 layout を定義する

### 対象ファイル

```text
frontend/src/views/DriveView.vue
frontend/src/components/DriveWorkspaceLayout.vue
frontend/src/style.css
```

### 追加する component

```text
frontend/src/components/DriveWorkspaceLayout.vue
```

### 方針

P14 の `AppSidebar` と `AppTopbar` は app 全体の navigation として残します。P15 では `DriveView.vue` の内側に Drive 専用 layout を追加します。

Drive layout は次の領域を持ちます。

- left rail: Drive navigation、folder tree、storage summary
- header: current workspace、current folder title、breadcrumbs、view toggle、details toggle
- command bar: search、filter chips、sort
- content: grid / list
- right panel: details、activity、permissions

### 注意

- `h-screen` は使わず、必要なら `min-height: 100dvh` または親 layout の minmax を使う
- fixed element は `safe-area-inset-*` を考慮する
- Drive layout 内に card を入れ子にしすぎない
- background gradient、blur、decorative blob は使わない
- accent color は active navigation、focus ring、primary action に限定する

## Step 3. Drive 左ナビと folder tree を作る

### 対象ファイル

```text
frontend/src/components/DriveSideNav.vue
frontend/src/components/DriveFolderTree.vue
frontend/src/stores/drive.ts
frontend/src/views/DriveView.vue
```

### 追加する component

```text
frontend/src/components/DriveSideNav.vue
frontend/src/components/DriveFolderTree.vue
```

### navigation

左ナビには次を置きます。

```text
New
My Drive
Shared with me
Recent
Starred
Trash
Storage
```

初期実装で backend API がないものは、次の扱いにします。

| Navigation | 初期実装 | 後続候補 |
| --- | --- | --- |
| My Drive | 既存 `/drive` と workspace / folder list を表示 | workspace switcher の改善 |
| Shared with me | UI route と empty state を用意 | 自分に共有された item API |
| Recent | UI route と sort by updatedAt の client 表示 | recent API |
| Starred | UI route と empty state を用意 | starred metadata API |
| Trash | 既存 `/drive/trash` を表示 | complete delete UI |
| Storage | quota summary の placeholder | tenant quota / SeaweedFS usage API |

folder tree は最初から全階層を eager load しません。現在の workspace root と開いた folder の children だけを使う lazy tree にします。

### UX

- folder tree の row は icon、name、expand button を持つ
- active folder は left nav と breadcrumb の両方で分かる
- folder name は `truncate` 相当で省略し、layout を押し広げない
- empty folder tree には `New folder` の明確な action を出す

## Step 4. 検索 / filter / sort toolbar を作る

### 対象ファイル

```text
frontend/src/components/DriveCommandBar.vue
frontend/src/components/DriveFilterChips.vue
frontend/src/stores/drive.ts
frontend/src/views/DriveView.vue
```

### 追加する component

```text
frontend/src/components/DriveCommandBar.vue
frontend/src/components/DriveFilterChips.vue
```

### UI state

store には UI state として次を追加します。

```text
viewMode: "grid" | "list"
query: string
typeFilter: "all" | "folder" | "document" | "image" | "archive" | "other"
ownerFilter: "all" | "owned_by_me" | "shared_with_me"
modifiedFilter: "any" | "today" | "last_7_days" | "last_30_days"
sourceFilter: "all" | "uploaded" | "external"
sortKey: "updated_at" | "name" | "size" | "type"
sortDirection: "asc" | "desc"
```

既存 API が query / content type 以外の filter を受け取れない場合、初期実装では client-side filter と sort にします。大規模 folder 向け server-side filter は後続実装候補にします。

### UX

- search input は Drive content の上部に置く
- filter は button または select を chip 風に表示する
- active filter は clear できる
- sort direction は icon button で切り替える
- search / filter error は command bar の近くに出す
- input paste はブロックしない

## Step 5. grid / list item view を作る

### 対象ファイル

```text
frontend/src/components/DriveItemGrid.vue
frontend/src/components/DriveItemList.vue
frontend/src/components/DriveItemCard.vue
frontend/src/components/DriveFileThumbnail.vue
frontend/src/components/DriveFileTypeIcon.vue
frontend/src/views/DriveView.vue
frontend/src/style.css
```

### 追加する component

```text
frontend/src/components/DriveItemGrid.vue
frontend/src/components/DriveItemList.vue
frontend/src/components/DriveItemCard.vue
frontend/src/components/DriveFileThumbnail.vue
frontend/src/components/DriveFileTypeIcon.vue
```

### 表示方針

grid view:

- 上段に folder card を表示する
- 下段に file card を表示する
- file card は thumbnail area、file type icon、name、updatedAt、shared/locked state を持つ
- thumbnail がない file は MIME type に応じた icon placeholder を出す

list view:

- table ではなく dense list row として実装してもよい
- name、owner または source、updatedAt、size、state、actions を固定順で表示する
- action column は hover で layout shift しない幅を確保する

### 注意

- card の radius は P14 の token に合わせる
- thumbnail area は `aspect-ratio` で安定させる
- 長い file name は 2 line までに制限する
- numeric data は tabular nums を使う
- loading は structural skeleton にする
- empty state には `Upload file` または `New folder` の一つ以上の明確な action を置く

## Step 6. item action menu と keyboard / accessibility を実装する

### 対象ファイル

```text
frontend/src/components/DriveItemMenu.vue
frontend/src/components/IconButton.vue
frontend/src/components/ConfirmActionDialog.vue
frontend/src/views/DriveView.vue
```

### 追加する component

```text
frontend/src/components/DriveItemMenu.vue
```

### menu action

三点メニューには次を入れます。

```text
Open
Download
Share
Rename
Move
Replace
Delete
Restore
Details
```

item type と mode によって表示を制御します。

| 状態 | 表示する action |
| --- | --- |
| folder | Open、Share、Rename、Move、Delete、Details |
| file | Download、Share、Rename、Move、Replace、Delete、Details |
| locked file | Download、Details を残し、編集系は disabled |
| trash | Restore、Details |
| public link view | Download は `canDownload=true` の時だけ |

### accessibility

- icon-only button には必ず `aria-label` を付ける
- menu trigger は keyboard で focus できる
- Escape で menu を閉じる
- destructive action は menu から直接実行せず `ConfirmActionDialog` を開く
- focus trap や roving focus が必要な menu は、既存 primitive または native control で実装する
- 独自 keyboard behavior を増やす場合は E2E で確認する

HaoHao は現在 Vue / global CSS 構成です。React 向け primitive は導入せず、既存 component と native semantics を優先します。

## Step 7. share dialog / permissions panel を改善する

### 対象ファイル

```text
frontend/src/components/DriveShareDialog.vue
frontend/src/components/DrivePermissionsPanel.vue
frontend/src/components/DriveShareAccessSummary.vue
frontend/src/components/DriveShareLinkSection.vue
frontend/src/components/DrivePeopleAccessList.vue
frontend/src/views/DriveView.vue
```

### 追加する component

```text
frontend/src/components/DriveShareAccessSummary.vue
frontend/src/components/DriveShareLinkSection.vue
frontend/src/components/DrivePeopleAccessList.vue
```

### share dialog の構成

上から次の順で表示します。

```text
1. item name と file/folder type
2. Current access summary
3. Add people / groups
4. Link sharing
5. People and groups access list
6. Inherited permissions
7. External invitations
```

### link sharing

- link が未作成なら `Create link`
- link 作成後は URL、copy button、copied state を表示
- raw token は既存方針どおり作成直後だけ表示する
- password、expiresAt、canDownload、role を同じ section にまとめる
- `canDownload=false` は「完全な情報漏えい防止ではない」注意書きを短く添える

### permissions panel

表示分類は次にします。

```text
Owner
Direct people
Groups
Share links
External invitations
Inherited
```

direct と inherited は必ず分けます。OpenFGA / backend が最終判定であり、UI は補助表示に留めます。

## Step 8. right details panel を作る

### 対象ファイル

```text
frontend/src/components/DriveDetailsPanel.vue
frontend/src/components/DriveActivityPanel.vue
frontend/src/components/DrivePermissionsSummary.vue
frontend/src/views/DriveView.vue
```

### 追加する component

```text
frontend/src/components/DriveDetailsPanel.vue
frontend/src/components/DriveActivityPanel.vue
frontend/src/components/DrivePermissionsSummary.vue
```

### tabs

right panel は次の tabs を持ちます。

```text
Details
Activity
Permissions
```

初期実装:

- Details は既存 item metadata から表示する
- Activity は audit / history API がない場合、empty state と後続候補を表示する
- Permissions は `DrivePermissionsPanel` の summary 版を表示する

### UX

- details panel は details toggle で開閉する
- selected item がない場合は current folder の details を表示する
- mobile では right panel を inline section または modal 相当にする
- panel 内の action は share dialog や item menu と重複しすぎないよう、summary と入口に留める

## Step 9. responsive / E2E / single binary を確認する

### responsive QA

最低限、次の viewport を確認します。

```text
1440x900 desktop
1024x768 tablet
390x844 mobile
```

見るポイント:

- Drive 左ナビが content を押し潰さない
- mobile で folder tree と right panel が重ならない
- filter chips が折り返しても main content を隠さない
- grid card の file name が overflow しない
- item menu と share dialog が viewport 外にはみ出さない
- public share link view が app shell なしでも崩れない

### E2E で追加する scenario

`e2e/drive.spec.ts` に次を追加します。

- grid view で folder を作成し、folder card から移動できる
- list view に切り替えて、同じ item が見える
- search query と type filter を適用し、clear できる
- item menu から rename / share / delete を実行できる
- share dialog で link を作成し、copy button が copied state になる
- permissions summary に direct と inherited が分かれて表示される
- details panel を開閉できる
- mobile viewport で Drive menu、filter、share dialog が操作できる

OpenFGA を必要とする journey は、既存と同じく `E2E_OPENFGA_ENABLED=true` の条件付きにします。

## 確認コマンド

実装中の基本確認:

```bash
rg --files docs frontend/src e2e
rg -n "DriveWorkspaceLayout|DriveSideNav|DriveCommandBar|DriveItemGrid|DriveItemMenu|DriveDetailsPanel" frontend/src
npm --prefix frontend run build
go test ./backend/...
make e2e
make binary
```

OpenFGA 実体を使う Drive 縦通し確認:

```bash
make smoke-openfga
E2E_OPENFGA_ENABLED=true make e2e
```

Markdown だけを変更した場合の確認:

```bash
rg --files docs frontend/src e2e
git diff --check -- docs/TUTORIAL_P15_DRIVE_SHARE_UX_REFRESH.md
```

## 生成物と直接編集しないファイル

次は生成物または build artifact です。直接編集しません。

```text
db/schema.sql
backend/internal/db/*
openapi/openapi.yaml
openapi/browser.yaml
openapi/external.yaml
frontend/src/api/generated/*
backend/web/dist/*
bin/haohao
```

OpenAPI / generated SDK が必要になった場合は、Huma schema、API handler、OpenAPI split、`make gen` の流れで更新します。generated file を手で直して UX を合わせることはしません。

## 後続実装候補

この P15 の初期実装で backend/API を変更しない場合、次は placeholder または client-side state になります。

| UX | 初期扱い | 後続で必要な API / schema |
| --- | --- | --- |
| Shared with me | empty state | actor に共有された item の list API |
| Recent | updatedAt sort の client 表示 | recent activity / last viewed API |
| Starred | empty state | starred metadata |
| Storage | placeholder | tenant quota、usage、SeaweedFS usage |
| Activity panel | empty state | drive audit / activity feed API |
| File thumbnail | MIME placeholder | preview / thumbnail provider |
| Owner filter | client-side 可能な範囲 | owner metadata を含む list response |
| Source filter | placeholder | source / provider metadata |
| Folder picker move | current tree だけ | folder search / tree API |

後続 API を追加する場合は、`docs/TUTORIAL_FILE_SHARE.md` と `docs/TUTORIAL_OPENFGA_P7_ADVANCED_DRIVE_OPERATIONS.md` の順序に戻り、DB、sqlc、backend service、OpenAPI、generated SDK、frontend の順で進めます。
