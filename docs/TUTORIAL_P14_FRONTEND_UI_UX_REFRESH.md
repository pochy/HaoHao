# P14: Frontend UI / UX Refresh 実装チュートリアル

## この文書の目的

この文書は、添付された billing 画面の UI/UX を分析し、HaoHao の Vue frontend をより作業用 SaaS らしい情報設計へ改善するための実装チュートリアルです。

目的は Metronic の見た目をそのまま複製することではありません。目的は、参考画像から次の設計原則を抽出し、HaoHao の既存 Vue / Pinia / Router / CSS 構成へ安全に落とすことです。

- persistent sidebar による安定した主導線
- top bar / breadcrumb / section tabs による現在地の明確化
- hero ではなく、ページタイトル、状態、主要 action をコンパクトに置く構成
- data card、metric tile、table、status badge を中心にした業務画面
- 余白、枠線、色、角丸、影を抑えた scan しやすい visual density
- icon button と明確な label を使った反復操作の効率化
- desktop / mobile の両方で layout shift や text overflow を起こさないこと

この文書は、既存の `docs/TUTORIAL.md`、`docs/TUTORIAL_SINGLE_BINARY.md`、`docs/TUTORIAL_OPENFGA_P5_UI_E2E.md` と同じように、「どのファイルを、どの順番で、何を確認しながら変更するか」を追える形にします。

## 参考画像の分析

### 画面構造

参考画像は、典型的な SaaS account / billing 画面です。大きく次の領域に分かれています。

| 領域 | 役割 | HaoHao に取り込む観点 |
| --- | --- | --- |
| 左 sidebar | product 全体の primary navigation | route が増えた HaoHao では pill nav より安定する |
| top bar | breadcrumb、utility icon、profile | active tenant、user、docs、notifications を置ける |
| section tabs | account 内の secondary navigation | Drive / Tenant Admin / Settings の内部移動に使える |
| page header | title、description、primary action | hero ではなく現在作業の context を示す |
| summary banner | trial / upgrade などの状態通知 | tenant status、support access、storage provider 状態に転用できる |
| plan card | plan、usage、quota、progress | entitlements、Drive storage、rate limit 表示に転用できる |
| two-column cards | payment、invoice、support | tenant admin、Drive permissions、signals summary に使える |
| table | invoice history | Customer Signals、Drive items、Machine clients に使える |

### 視覚上の特徴

- 背景はほぼ white / very light gray
- card は 8-12px 程度の角丸、薄い border、控えめな shadow
- brand color は active state と primary action に限定
- title は大きすぎず、card 内 text も compact
- table / metric / badge / progress が情報密度を作っている
- row action は button の text だけに頼らず、icon や menu を使っている
- page 全体に marketing hero や装飾背景を置いていない

### そのまま取り込まないもの

- Metronic logo、illustration、avatar、billing 文言、配色をそのまま使わない
- paid template の asset や独自 pattern をコピーしない
- billing 固有の UI を HaoHao の全画面へ無理に当てはめない
- 装飾用の hex pattern や illustration を primary UI にしない

参考画像は、layout、density、hierarchy の参考として扱います。

## HaoHao の現在地

現在の frontend は次の構成です。

- Vue 3 + Vite + TypeScript
- Pinia store
- Vue Router
- generated SDK + thin API wrapper
- global CSS: `frontend/src/style.css`
- root shell: `frontend/src/App.vue`
- main views: `frontend/src/views/*`
- reusable components: `frontend/src/components/*`

現在の UI は foundation tutorial の段階から発展してきたため、次の特徴があります。

- `App.vue` が中央寄せの single-column shell
- top navigation が pill link の折り返し
- page title `HaoHao` が大きく、作業画面より landing page に近い
- 背景が beige / radial gradient / grid overlay で、業務画面としては装飾が強い
- `panel` の角丸と shadow が大きく、nested card に見えやすい
- button が pill shape で、dense table action には横幅を取りやすい
- `window.prompt` を使う操作が残っている
- icon button / compact toolbar / section tabs が不足している

この P14 では、API や backend contract は変えません。frontend shell と UI component の整理を中心に進めます。

## 完成条件

このチュートリアルの完了条件は次です。

- root layout が persistent sidebar + topbar + content area になる
- `/`, `/drive`, `/customer-signals`, `/tenant-admin`, `/machine-clients` が同じ shell で表示される
- mobile では sidebar が content を押し潰さず、top navigation または drawer 相当の compact navigation に切り替わる
- active route と active tenant が常に分かる
- page header が各 view の title、description、primary action、secondary actions を整理して表示する
- `panel` は業務用 card として 8-12px 程度の角丸、薄い border、控えめな shadow に寄る
- background gradient、decorative grid、large display serif は通常の app shell から外す
- table / list の row height、action column、badge、empty / loading / error state が安定する
- destructive action は `ConfirmActionDialog` または同等の dialog を通す
- icon-only button には必ず `aria-label` がある
- text overflow が desktop / mobile で崩れない
- `npm --prefix frontend run build` と `make e2e` が通る
- `make binary` で embedded frontend も崩れない

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | design inventory | 既存 class と view の使われ方を棚卸しする |
| Step 2 | design tokens | 背景、色、角丸、影、typography を業務 UI 向けに整理する |
| Step 3 | app shell | `App.vue` を sidebar + topbar layout に変える |
| Step 4 | shared components | page header、tabs、badge、card、icon button を作る |
| Step 5 | navigation mapping | route を primary / secondary navigation に分類する |
| Step 6 | key views | Home、Drive、Signals、Tenant Admin を順に refresh する |
| Step 7 | dialogs / prompts | `window.prompt` を form/dialog へ置き換える |
| Step 8 | responsive QA | desktop / tablet / mobile で崩れを直す |
| Step 9 | E2E / binary | build、E2E、single binary で確認する |

## Step 1. design inventory を作る

### 対象ファイル

```text
frontend/src/App.vue
frontend/src/style.css
frontend/src/views/*
frontend/src/components/*
```

### 確認コマンド

```bash
rg -n "panel|status-pill|primary-button|secondary-button|app-shell|app-nav|window.prompt|aria-label" frontend/src
rg -n "border-radius|gradient|backdrop-filter|font-display|letter-spacing" frontend/src/style.css frontend/src/App.vue
```

### 見るポイント

- `panel` を card として使っている箇所
- destructive action に確認 dialog があるか
- `window.prompt` で入力を取っている箇所
- action button が多すぎて table row が広がっている箇所
- route が増えたことで primary navigation が読みにくくなっている箇所
- mobile で grid が 1 column になった時に action が長くなりすぎる箇所

inventory は新規 doc にしてもよいですが、最初の実装では issue / PR description にまとめるだけでも十分です。

## Step 2. design tokens を業務 UI 向けに整理する

### 対象ファイル

```text
frontend/src/style.css
```

### 方針

HaoHao は Tailwind ではなく global CSS を使っています。この P14 では新しく Tailwind を導入しません。既存 CSS の変数と utility class を整理して進めます。

変更する方向:

- background は white / neutral gray 中心にする
- accent は active route、primary action、focus ring に限定する
- semantic color は success / warning / danger を token 化する
- card radius は `8px` から `12px` を基準にする
- page-level card に大きな blur / backdrop-filter を使わない
- body background の radial gradient と decorative grid は外す
- serif display font を通常 app shell の heading / button から外す
- numeric data には `font-variant-numeric: tabular-nums` を使う

### 例

```css
:root {
  --bg: #f8fafc;
  --surface: #ffffff;
  --surface-muted: #f1f5f9;
  --text: #334155;
  --text-strong: #0f172a;
  --muted: #64748b;
  --accent: #2563eb;
  --accent-strong: #1d4ed8;
  --success: #15803d;
  --warning: #b45309;
  --danger: #b91c1c;
  --border: #e2e8f0;
  --border-strong: #cbd5e1;
  --shadow-sm: 0 1px 2px rgba(15, 23, 42, 0.06);
  --radius-sm: 8px;
  --radius-md: 10px;
  --radius-lg: 12px;
  --font-sans:
    Inter, "Avenir Next", "Segoe UI", "Hiragino Sans",
    "Hiragino Kaku Gothic ProN", sans-serif;
}
```

注意:

- accent を画面全体に広げない
- purple / purple-blue gradient に寄せない
- beige / cream / sand の一色調に戻さない
- `letter-spacing` は必要な small label 以外では増やさない
- `h-screen` は使わず、必要なら `min-height: 100dvh` を使う

## Step 3. App shell を sidebar + topbar に変える

### 対象ファイル

```text
frontend/src/App.vue
frontend/src/components/AppSidebar.vue
frontend/src/components/AppTopbar.vue
frontend/src/components/TenantSelector.vue
frontend/src/style.css
```

### 追加する component

```text
frontend/src/components/AppSidebar.vue
frontend/src/components/AppTopbar.vue
```

### App shell の責務

- primary navigation を固定位置で表示する
- current user と session status を topbar に表示する
- active tenant selector を topbar に表示する
- breadcrumb 相当の current route label を表示する
- content width を page 種別に応じて広く使えるようにする
- mobile では sidebar を compact navigation にする

### navigation group

最初は次の分類にします。

```ts
const navigationGroups = [
  {
    label: 'Workspace',
    items: [
      { to: '/', label: 'Session' },
      { to: '/notifications', label: 'Notifications' },
    ],
  },
  {
    label: 'Work',
    items: [
      { to: '/customer-signals', label: 'Signals' },
      { to: '/drive', label: 'Drive' },
      { to: '/todos', label: 'TODO' },
    ],
  },
  {
    label: 'Admin',
    items: [
      { to: '/tenant-admin', label: 'Tenants' },
      { to: '/machine-clients', label: 'Machine Clients' },
      { to: '/integrations', label: 'Integrations' },
    ],
  },
]
```

この分類は backend role とは別です。UI の primary navigation として、route の見通しをよくするための分類です。

### layout skeleton

```vue
<template>
  <div class="app-layout">
    <AppSidebar class="app-sidebar" />
    <div class="app-workspace">
      <SupportAccessBanner />
      <AppTopbar />
      <main class="app-content">
        <RouterView />
      </main>
    </div>
  </div>
</template>
```

### CSS 方針

```css
.app-layout {
  min-height: 100dvh;
  display: grid;
  grid-template-columns: 264px minmax(0, 1fr);
  background: var(--bg);
}

.app-sidebar {
  position: sticky;
  top: 0;
  height: 100dvh;
  border-right: 1px solid var(--border);
  background: var(--surface);
}

.app-workspace {
  min-width: 0;
}

.app-content {
  width: min(1280px, calc(100vw - 320px));
  margin: 0 auto;
  padding: 24px;
}
```

mobile では 1 column にします。

```css
@media (max-width: 900px) {
  .app-layout {
    grid-template-columns: 1fr;
  }

  .app-sidebar {
    position: static;
    height: auto;
  }

  .app-content {
    width: 100%;
    padding: 16px;
  }
}
```

## Step 4. shared UI component を作る

### 対象ファイル

```text
frontend/src/components/PageHeader.vue
frontend/src/components/SectionTabs.vue
frontend/src/components/DataCard.vue
frontend/src/components/MetricTile.vue
frontend/src/components/StatusBadge.vue
frontend/src/components/IconButton.vue
frontend/src/components/EmptyState.vue
frontend/src/components/SkeletonBlock.vue
frontend/src/style.css
```

### component 方針

Vue component は薄く作ります。domain data の取得や API 呼び出しは view / store に残します。

#### `PageHeader.vue`

責務:

- page title
- short description
- optional status badge
- primary action slot
- secondary action slot

使用例:

```vue
<PageHeader
  title="Drive"
  description="Files, folders, sharing, and workspace storage."
>
  <template #actions>
    <button class="primary-button" type="button">Upload</button>
  </template>
</PageHeader>
```

#### `SectionTabs.vue`

責務:

- 同一 domain 内の secondary navigation
- `RouterLink` の active state
- 横 overflow 時に scroll できること

Drive では次を tabs にします。

```text
Browser
Search
Trash
Groups
```

Tenant Admin では次を tabs にします。

```text
Overview
Members
Invitations
Settings
Drive
OpenFGA
Entitlements
Support
Webhooks
Export / Import
```

#### `IconButton.vue`

責務:

- icon-only action の fixed size
- `aria-label` 必須
- disabled / danger / active state

アイコンが必要な場合は `lucide-vue-next` を導入します。

```bash
npm --prefix frontend install lucide-vue-next
```

導入した場合は `frontend/package.json` と `frontend/package-lock.json` を commit 対象にします。既存 component では text button から始めてもよいですが、table row action は icon button 化した方が密度が上がります。

## Step 5. page header と section tabs を route に接続する

### 対象ファイル

```text
frontend/src/router/index.ts
frontend/src/App.vue
frontend/src/components/AppTopbar.vue
frontend/src/components/PageHeader.vue
frontend/src/components/SectionTabs.vue
```

### route meta を増やす

`router/index.ts` の route meta に label と group を追加します。

```ts
{
  path: '/drive',
  name: 'drive',
  component: DriveView,
  meta: {
    requiresAuth: true,
    label: 'Drive',
    group: 'Work',
  },
}
```

TypeScript の `RouteMeta` 型拡張が必要なら、`frontend/src/router/types.ts` または `frontend/src/env.d.ts` に追加します。

```ts
declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    label?: string
    group?: string
  }
}
```

### topbar 表示

`AppTopbar.vue` は現在 route の `meta.label` と active tenant を表示します。

```text
Work / Drive
Tenant: Acme / acme
User: Demo User
```

参考画像の breadcrumb は page context を示すためのものです。HaoHao では deep hierarchy が増えるまでは、`group / label` の 2 階層で十分です。

## Step 6. HomeView を dashboard ではなく account overview に変える

### 対象ファイル

```text
frontend/src/views/HomeView.vue
frontend/src/components/PageHeader.vue
frontend/src/components/DataCard.vue
frontend/src/components/MetricTile.vue
frontend/src/style.css
```

### 方針

現在の Home は tutorial verification の意味が強く、production UI としては説明文が多いです。P14 では、session / tenant / quick links を compact に見せる account overview へ変えます。

表示する内容:

- current user
- active tenant
- session status
- CSRF/session refresh action
- quick links: Drive、Signals、Tenant Admin、Docs
- support access banner は shell 側に残す

参考画像の billing plan card を、HaoHao では account / tenant summary card として使います。

注意:

- tutorial 説明文を UI から減らす
- JSON dump は collapsible details または developer-only card に移す
- `Logout` は topbar user menu または account card action に置く

## Step 7. DriveView を作業画面として再構成する

### 対象ファイル

```text
frontend/src/views/DriveView.vue
frontend/src/components/DriveToolbar.vue
frontend/src/components/DriveItemTable.vue
frontend/src/components/DriveBreadcrumbs.vue
frontend/src/components/DriveShareDialog.vue
frontend/src/components/DrivePermissionsPanel.vue
frontend/src/components/PageHeader.vue
frontend/src/components/SectionTabs.vue
frontend/src/components/IconButton.vue
frontend/src/style.css
```

### 目標 layout

Drive は Google Drive clone の主画面なので、card-heavy ではなく table/list 中心にします。

上から次の順に置きます。

1. `PageHeader`: title、workspace selector、primary upload action
2. `SectionTabs`: Browser / Search / Trash / Groups
3. Storage / workspace summary row
4. Breadcrumb + toolbar
5. File/folder table
6. Share dialog / permissions panel

### 改善する点

- `window.prompt` による rename / move / workspace create を dialog form に置き換える
- upload / create folder / search を toolbar にまとめる
- row action は fixed action column にし、hover で layout が動かないようにする
- action が多い場合は icon button + overflow menu 相当に分ける
- file/folder name は `truncate` 相当の CSS で 1 行に収める
- trash mode では restore / permanent delete など、状態に合う action だけ出す
- locked file の action は disabled で見せるが、API でも拒否させる

### table density

Drive table は次の column を基本にします。

```text
Name
Type
Owner
Updated
Size
Sharing
Actions
```

`Actions` は幅を固定します。

```css
.data-table-actions {
  width: 168px;
  white-space: nowrap;
}
```

## Step 8. Customer Signals を filter + list layout に寄せる

### 対象ファイル

```text
frontend/src/views/CustomerSignalsView.vue
frontend/src/views/CustomerSignalDetailView.vue
frontend/src/components/PageHeader.vue
frontend/src/components/DataCard.vue
frontend/src/components/MetricTile.vue
frontend/src/style.css
```

### 方針

Signals は CRM / support workflow の作業画面です。参考画像の invoice table のように、scan しやすい table/list と compact filters を優先します。

改善内容:

- PageHeader に `Create Signal` primary action を置く
- open / urgent / planned などの count を metric tile にする
- search / status / priority / source を toolbar row にまとめる
- saved filters は horizontal chips または side panel にする
- signal list は title、customer、status、priority、source、updated を揃える
- create form は常時大きく出すのではなく、dialog または collapsible section にする

注意:

- status / priority は semantic badge にする
- long title / body preview は line clamp する
- empty state は `Create Signal` または `Clear filters` の 1 action に絞る

## Step 9. Tenant Admin を section tabs に分割する

### 対象ファイル

```text
frontend/src/views/TenantAdminTenantDetailView.vue
frontend/src/components/PageHeader.vue
frontend/src/components/SectionTabs.vue
frontend/src/components/DataCard.vue
frontend/src/style.css
```

### 現状の課題

`TenantAdminTenantDetailView.vue` は membership、settings、Drive policy、OpenFGA、entitlements、support、webhooks、export/import などが 1 view に集まっています。機能としては正しいですが、スクロール距離が長く、現在地が分かりにくくなります。

### 改善方針

最初の P14 では route を増やさず、同一 view 内の `SectionTabs` と anchor section で整理します。

```text
Overview
Members
Invitations
Settings
Drive
OpenFGA
Entitlements
Support
Webhooks
Export / Import
```

後続で必要になったら、次のように route 分割します。

```text
/tenant-admin/:tenantSlug
/tenant-admin/:tenantSlug/members
/tenant-admin/:tenantSlug/settings
/tenant-admin/:tenantSlug/drive
/tenant-admin/:tenantSlug/integrations
```

## Step 10. dialogs と destructive action を整理する

### 対象ファイル

```text
frontend/src/components/ConfirmActionDialog.vue
frontend/src/components/*
frontend/src/views/*
```

### 方針

削除、revoke、disable、reject、restore など、取り消しにくい操作は `ConfirmActionDialog` を通します。

`window.prompt` を使っている操作は、専用 dialog form に置き換えます。

優先して置き換えるもの:

- Drive workspace create
- Drive rename
- Drive move
- Drive share subject input
- Tenant role grant
- Webhook create

native `<dialog>` を使い続けてよいですが、次を守ります。

- close button に `aria-label`
- initial focus が自然な input または cancel button に当たる
- error message は dialog 内の action 近くに表示
- submit 中は button disabled
- destructive confirm は danger style

## Step 11. icon と action density を改善する

### 対象ファイル

```text
frontend/package.json
frontend/package-lock.json
frontend/src/components/IconButton.vue
frontend/src/components/DriveItemTable.vue
frontend/src/views/*
```

### 方針

参考画像では utility action が icon 化され、text button は主要 action に絞られています。HaoHao でも table row の繰り返し action から icon button 化します。

推奨 icon:

```text
RefreshCw
Upload
FolderPlus
Download
Share2
Pencil
MoveRight
ArchiveRestore
Trash2
MoreHorizontal
Settings
Bell
Search
User
```

導入する場合:

```bash
npm --prefix frontend install lucide-vue-next
```

注意:

- icon-only button には必ず `aria-label`
- 不明瞭な icon には `title` または tooltip 相当の text を用意する
- icon size は `16px` または `18px` を標準にする
- row action の button は square size にする
- text button は primary action と destructive confirmation に残す

## Step 12. responsive layout を確認する

### 対象 viewport

```text
1440 x 1000
1280 x 800
1024 x 768
390 x 844
```

### 確認する画面

```text
/
/drive
/drive/search
/drive/trash
/drive/groups
/customer-signals
/tenant-admin
/machine-clients
/notifications
```

### 確認観点

- sidebar が content を潰していない
- mobile で nav が操作可能
- page header の action が折り返しても読める
- table が横 overflow する場合、画面全体ではなく table container だけ scroll する
- button text が枠からはみ出さない
- long file name / customer name / tenant slug が崩れない
- dialog が `100dvh` 内に収まる
- focus ring が見える
- loading / empty / forbidden / error state が同じ場所に出る

## Step 13. E2E を更新する

### 対象ファイル

```text
e2e/access-and-fallback.spec.ts
e2e/browser-journey.spec.ts
e2e/drive.spec.ts
```

### 方針

E2E では visual の細部ではなく、主要導線が壊れていないことを確認します。

確認する導線:

- login 後に app shell が表示される
- primary navigation から Drive / Signals / Tenant Admin へ移動できる
- active route の nav item が分かる
- tenant selector が topbar に残っている
- Drive の upload / share / public link 導線が維持されている
- SPA fallback と API / assets の切り分けが維持されている

selector は text 依存だけにせず、必要に応じて `data-testid` を足します。

```vue
<nav data-testid="app-sidebar" aria-label="Primary">
```

## 最終確認コマンド

実装後は次を実行します。

```bash
npm --prefix frontend run build
go test ./backend/...
make binary
make e2e
git diff --check
```

Drive / OpenFGA を触った場合は次も確認します。

```bash
make test-openfga-model
make smoke-openfga
```

SeaweedFS storage driver を使う環境では、必要に応じて次も確認します。

```bash
make seaweedfs-up
make seaweedfs-config

FILE_STORAGE_DRIVER=seaweedfs_s3 \
FILE_S3_ENDPOINT=http://127.0.0.1:8333 \
FILE_S3_REGION=us-east-1 \
FILE_S3_BUCKET=haohao-drive-dev \
FILE_S3_ACCESS_KEY_ID=haohao \
FILE_S3_SECRET_ACCESS_KEY=haohao-dev-secret \
FILE_S3_FORCE_PATH_STYLE=true \
make smoke-file-purge
```

## 生成物と手書きファイルの境界

手で編集してよい source:

```text
frontend/src/App.vue
frontend/src/style.css
frontend/src/components/*
frontend/src/views/*
frontend/src/router/index.ts
frontend/package.json
frontend/package-lock.json
e2e/*
```

直接編集しない生成物 / build artifact:

```text
backend/web/dist/*
bin/haohao
frontend/src/api/generated/*
openapi/openapi.yaml
```

`frontend/src/api/generated/*` が変わる必要がある場合は、backend API contract を変更した別フェーズとして扱い、`make gen` で生成します。P14 の UI refresh だけでは generated SDK を直接編集しません。

## 実装時の注意

- landing page や hero を作らない
- UI 内に「この機能はこう使う」という説明文を増やしすぎない
- page section を大きな floating card として積みすぎない
- nested card を避ける
- button は主要 action に絞り、繰り返し action は icon button へ寄せる
- icon-only button には必ず `aria-label`
- destructive action は `ConfirmActionDialog`
- input error は action の近くに表示
- gradient、glow、decorative blur を primary visual にしない
- animation は必要になるまで入れない
- text は container 内に収める
- table / toolbar / action column は stable width を持たせる
- mobile で horizontal scroll が必要な場合は table container に閉じ込める

## Phase 分割

一度に全画面を変えると E2E と visual regression の原因が追いにくくなります。次の順に小さく commit します。

### Phase A. Shell and tokens

- `App.vue`
- `AppSidebar.vue`
- `AppTopbar.vue`
- `style.css`
- route meta

確認:

```bash
npm --prefix frontend run build
make e2e
```

### Phase B. Shared components

- `PageHeader.vue`
- `SectionTabs.vue`
- `DataCard.vue`
- `MetricTile.vue`
- `StatusBadge.vue`
- `IconButton.vue`
- `EmptyState.vue`

確認:

```bash
npm --prefix frontend run build
```

### Phase C. Drive refresh

- `DriveView.vue`
- `DriveToolbar.vue`
- `DriveItemTable.vue`
- `DriveShareDialog.vue`
- `DrivePermissionsPanel.vue`
- Drive E2E

確認:

```bash
npm --prefix frontend run build
make e2e
make smoke-openfga
```

### Phase D. Admin / Signals refresh

- `CustomerSignalsView.vue`
- `CustomerSignalDetailView.vue`
- `TenantAdminTenantDetailView.vue`
- `MachineClientsView.vue`
- browser journey E2E

確認:

```bash
npm --prefix frontend run build
make e2e
```

### Phase E. Single binary final check

```bash
make binary
make e2e
```

## 完了後の期待状態

P14 完了後の HaoHao frontend は、現在の tutorial build らしい画面から、日常的に使う SaaS admin / Drive app らしい画面に寄ります。

期待する変化:

- 左 navigation で機能が迷子にならない
- topbar で tenant / user / current route が分かる
- page title と action が毎画面で同じ位置にある
- card は情報のまとまりだけに使われる
- table と list が scan しやすくなる
- Drive の row action が密になり、操作対象が分かりやすくなる
- mobile でもナビゲーションと主要 action が破綻しない
- E2E と single binary build が維持される
