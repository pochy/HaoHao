# P18: テナント詳細ページ分割チュートリアル

## この文書の目的

この文書は、現在 1 つの画面に集約されている tenant detail 管理 UI を、用途・目的ごとのサブページへ分割するための実装チュートリアルです。

対象は `frontend/src/views/TenantAdminTenantDetailView.vue` です。この view は tenant 基本設定、membership、invitation、common settings、Drive policy、Drive share state、OpenFGA sync、entitlements、support access、webhook、export、import をすべて同じファイルと同じページに持っています。機能は増えていますが、変更理由が異なる領域が同居しているため、次の問題が起きやすくなっています。

- 1 つの Vue file が大きくなり、どの変更がどの機能に影響するか読みづらい
- Drive policy のような重い form が、membership や invitation の修正にも巻き込まれる
- 初期表示で必要のない API まで同時に load しやすい
- E2E が長い 1 ページ前提になり、対象機能を絞って検証しづらい
- 画面上でも「どこで何を管理するか」が分かりづらい

この P18 では、Drive ページの local navigation を参考にしながら、tenant detail に左サイドバーを追加し、サブページ単位で view を分けます。あわせて、サイドバーは Drive 専用ではない共通 component として作り、他の管理画面でも再利用できる形にします。

この文書は、次の既存チュートリアルと同じように、「どのファイルを、どの順番で、何を確認しながら変更するか」を追える形にします。

- `docs/TUTORIAL.md`
- `docs/TUTORIAL_SINGLE_BINARY.md`
- `docs/TUTORIAL_P5_TENANT_ADMIN_UI.md`
- `docs/TUTORIAL_P16_DRIVE_FEATURE_COMPLETION.md`
- `docs/TUTORIAL_P17_I18N.md`

## 前提と現在地

このチュートリアルは、現在の HaoHao repository が少なくとも次の状態にある前提で進めます。

- frontend は Vue 3 + Vite + TypeScript
- routing は Vue Router
- state は Pinia
- i18n は `vue-i18n` Composition API mode
- `/tenant-admin` は tenant 一覧
- `/tenant-admin/new` は tenant 作成
- `/tenant-admin/:tenantSlug` は tenant detail
- `TenantAdminTenantDetailView.vue` に tenant detail の主要機能が集約されている
- Drive 画面には `DriveWorkspaceLayout.vue` と `DriveSideNav.vue` がある
- `make gen`、`npm --prefix frontend run build`、`make e2e` が標準確認コマンドとして使える

この P18 では backend API contract は原則変更しません。既存 API をサブページから呼び分ける frontend refactor として進めます。

### やらないこと

- tenant admin API の path を変更しない
- OpenAPI schema を手書き編集しない
- `frontend/src/api/generated/*` を手書き編集しない
- Drive の file body や tenant data export の download 仕様を変えない
- role / permission model をこの段階で変更しない
- すべての tenant admin 機能を 1 回で高度化しない

今回の目的は、情報設計と実装単位を分けることです。機能追加ではなく、既存機能を管理しやすい画面構造に移すことを優先します。

## 完成条件

このチュートリアルの完了条件は次です。

- `/tenant-admin/:tenantSlug` が既存 deep link として残り、overview サブページへ誘導される
- tenant detail 配下に用途別サブページがある
- tenant detail の左側に sidebar が表示される
- sidebar に各サブページへの link が表示される
- active subpage が sidebar 上で分かる
- sidebar component は Drive 専用ではない共通 component として使える
- `TenantAdminTenantDetailView.vue` の巨大な script / template が分割される
- 各サブページは必要な store action と form state だけを持つ
- 既存の tenant 作成後遷移、tenant 一覧からの遷移、`/tenant-admin/acme` 直打ちが壊れない
- i18n message catalog に tenant admin sidebar / subpage label がある
- desktop / tablet / mobile で sidebar と本文が重ならない
- `npm --prefix frontend run build` が通る
- `make e2e` が通る
- `make binary` 後の embedded frontend でも tenant detail subpage route が SPA fallback される

## 分割後の画面構成

最初の分割単位は、現行画面にある管理目的をそのまま反映します。

| Subpage | URL | 主な責務 |
| --- | --- | --- |
| Overview | `/tenant-admin/:tenantSlug/overview` | slug、display name、active、active member count、updated at、deactivate |
| Members | `/tenant-admin/:tenantSlug/members` | tenant role grant、membership 一覧、local role revoke |
| Invitations | `/tenant-admin/:tenantSlug/invitations` | invitation 作成、pending invite 一覧、revoke |
| Settings | `/tenant-admin/:tenantSlug/settings` | file quota、browser API rate limit、notification |
| Drive Policy | `/tenant-admin/:tenantSlug/drive-policy` | Drive feature / sharing / scan / plan / residency policy |
| Drive Operations | `/tenant-admin/:tenantSlug/drive-operations` | share state、share link、approval、OpenFGA sync、Drive audit |
| Entitlements | `/tenant-admin/:tenantSlug/entitlements` | feature gate 一覧と更新 |
| Support | `/tenant-admin/:tenantSlug/support` | support access session 開始 |
| Webhooks | `/tenant-admin/:tenantSlug/webhooks` | outbound webhook 作成、一覧 |
| Data | `/tenant-admin/:tenantSlug/data` | tenant data export、CSV export、Customer Signals CSV import |

`/tenant-admin/:tenantSlug` は互換性のため残し、`/tenant-admin/:tenantSlug/overview` へ redirect します。

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | inventory | 現行 detail view の責務を棚卸しする |
| Step 2 | `SectionSideNav.vue` | Drive 風 sidebar を共通 component として作る |
| Step 3 | tenant admin section 定義 | sidebar link と route name を 1 か所に集約する |
| Step 4 | router | tenant detail を nested route にする |
| Step 5 | shell view | header、sidebar、共通 loading/error を親 view に移す |
| Step 6 | subpage views | 現行 template / state / action を用途別 view に分ける |
| Step 7 | store load の整理 | サブページが必要な data だけを load する |
| Step 8 | CSS / responsive | Drive layout を参考に tenant detail layout を整える |
| Step 9 | i18n | sidebar / heading / button label を message catalog に移す |
| Step 10 | E2E | 旧 URL、sidebar navigation、主要操作を検証する |
| Step 11 | build / binary | Vite build と single binary route fallback を確認する |

## Step 1. 現行 detail view の責務を棚卸しする

### 対象ファイル

```text
frontend/src/views/TenantAdminTenantDetailView.vue
frontend/src/stores/tenant-admin.ts
frontend/src/stores/tenant-common.ts
frontend/src/api/tenant-admin.ts
frontend/src/api/tenant-settings.ts
frontend/src/api/tenant-invitations.ts
frontend/src/api/tenant-data-exports.ts
frontend/src/api/customer-signal-imports.ts
frontend/src/api/webhooks.ts
frontend/src/api/entitlements.ts
frontend/src/router/index.ts
e2e/browser-journey.spec.ts
```

### 確認コマンド

```bash
rg -n "Tenant Detail|Grant Tenant Role|Drive Policy|Drive Admin|Entitlements|Support Access|Outbound Webhooks|Tenant Data Exports|Customer Signals CSV" frontend/src/views/TenantAdminTenantDetailView.vue
rg -n "tenant-admin|tenant-invitation|tenant-request-export" frontend/src e2e
rg -n "/api/v1/admin/tenants/\\{tenantSlug\\}" openapi/openapi.yaml
```

### 分類する責務

`TenantAdminTenantDetailView.vue` の state と action を、次の単位へ分けます。

| 現行 state / action | 移動先 |
| --- | --- |
| `displayName`, `active`, `saveSettings`, `askDeactivate` | Overview |
| `grantUserEmail`, `grantRoleCode`, `grantRole`, `askRevoke` | Members |
| `invitationEmail`, `invitationRoleCode`, `createInvitation`, `revokeInvitation` | Invitations |
| `fileQuotaBytes`, `browserRateLimit`, `notificationsEnabled` | Settings |
| `drive*Enabled`, `drive*Limit`, `saveCommonSettings` の Drive 部分 | Drive Policy |
| `approveDriveInvitation`, `rejectDriveInvitation`, `repairDriveSync` | Drive Operations |
| `saveEntitlements` | Entitlements |
| `supportUserPublicId`, `supportReason`, `startSupportAccess` | Support |
| `webhookName`, `webhookUrl`, `webhookEvents`, `createWebhook` | Webhooks |
| `requestExport`, `requestCSVExport`, `importFile`, `uploadImportCSV` | Data |

この棚卸しでは、まだ code を動かしません。先に「どの subpage がどの state を持つか」を固定します。

## Step 2. 共通 sidebar component を作る

Drive の `DriveSideNav.vue` は Drive 専用の folder tree、upload、storage summary を含んでいます。そのまま tenant admin に流用すると責務が混ざります。

ここでは、link navigation だけを共通化した component を新しく作ります。Drive で将来使う場合も、folder tree や storage summary は slot として外側から渡せる形にします。

### 対象ファイル

```text
frontend/src/components/SectionSideNav.vue
frontend/src/components/section-side-nav.ts
```

### component interface

`<script setup>` から named export すると扱いづらいため、再利用する型は `.ts` ファイルに分けます。

#### ファイル: `frontend/src/components/section-side-nav.ts`

```ts
import type { Component } from 'vue'
import type { RouteLocationRaw } from 'vue-router'

export type SectionSideNavItem = {
  key: string
  label: string
  description?: string
  to: RouteLocationRaw
  icon?: Component
  disabled?: boolean
  badge?: string | number
}
```

### 実装例

#### ファイル: `frontend/src/components/SectionSideNav.vue`

```vue
<script setup lang="ts">
import type { SectionSideNavItem } from './section-side-nav'

defineProps<{
  ariaLabel: string
  heading?: string
  items: SectionSideNavItem[]
}>()
</script>

<template>
  <aside class="section-side-nav" :aria-label="ariaLabel">
    <slot name="before" />

    <div v-if="heading" class="section-side-heading">
      {{ heading }}
    </div>

    <nav class="section-local-nav">
      <template v-for="item in items" :key="item.key">
        <span
          v-if="item.disabled"
          class="section-local-link muted"
          aria-disabled="true"
        >
          <component :is="item.icon" v-if="item.icon" :size="17" stroke-width="1.9" aria-hidden="true" />
          <span>
            <strong>{{ item.label }}</strong>
            <small v-if="item.description">{{ item.description }}</small>
          </span>
          <em v-if="item.badge !== undefined">{{ item.badge }}</em>
        </span>

        <RouterLink
          v-else
          class="section-local-link"
          :to="item.to"
        >
          <component :is="item.icon" v-if="item.icon" :size="17" stroke-width="1.9" aria-hidden="true" />
          <span>
            <strong>{{ item.label }}</strong>
            <small v-if="item.description">{{ item.description }}</small>
          </span>
          <em v-if="item.badge !== undefined">{{ item.badge }}</em>
        </RouterLink>
      </template>
    </nav>

    <slot name="after" />
  </aside>
</template>
```

`RouterLink` は current route と一致すると自動で `router-link-active` / `router-link-exact-active` class を付けます。CSS 側では `.router-link-active` も active state として扱います。

## Step 3. tenant admin section 定義を作る

sidebar の link、route name、label、icon を各 view に散らすと、サブページ追加時に更新漏れが起きます。tenant admin detail 用の section 定義を 1 ファイルに集約します。

### 対象ファイル

```text
frontend/src/tenant-admin/sections.ts
```

`frontend/src/tenant-admin/` がまだ無い場合は、このタイミングで作ります。view でも component でも store でもない、tenant admin feature 固有の定義を置く場所です。

### 実装例

```ts
import {
  Database,
  Gauge,
  KeyRound,
  LifeBuoy,
  MailPlus,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  Users,
  Webhook,
} from 'lucide-vue-next'
import type { Component } from 'vue'
import type { RouteLocationRaw } from 'vue-router'

export type TenantAdminSectionKey =
  | 'overview'
  | 'members'
  | 'invitations'
  | 'settings'
  | 'drive-policy'
  | 'drive-operations'
  | 'entitlements'
  | 'support'
  | 'webhooks'
  | 'data'

export type TenantAdminSection = {
  key: TenantAdminSectionKey
  routeName: string
  labelKey: string
  descriptionKey: string
  icon: Component
}

export const tenantAdminSections: TenantAdminSection[] = [
  {
    key: 'overview',
    routeName: 'tenant-admin-detail-overview',
    labelKey: 'tenantAdmin.sections.overview',
    descriptionKey: 'tenantAdmin.sectionDescriptions.overview',
    icon: Gauge,
  },
  {
    key: 'members',
    routeName: 'tenant-admin-detail-members',
    labelKey: 'tenantAdmin.sections.members',
    descriptionKey: 'tenantAdmin.sectionDescriptions.members',
    icon: Users,
  },
  {
    key: 'invitations',
    routeName: 'tenant-admin-detail-invitations',
    labelKey: 'tenantAdmin.sections.invitations',
    descriptionKey: 'tenantAdmin.sectionDescriptions.invitations',
    icon: MailPlus,
  },
  {
    key: 'settings',
    routeName: 'tenant-admin-detail-settings',
    labelKey: 'tenantAdmin.sections.settings',
    descriptionKey: 'tenantAdmin.sectionDescriptions.settings',
    icon: Settings,
  },
  {
    key: 'drive-policy',
    routeName: 'tenant-admin-detail-drive-policy',
    labelKey: 'tenantAdmin.sections.drivePolicy',
    descriptionKey: 'tenantAdmin.sectionDescriptions.drivePolicy',
    icon: SlidersHorizontal,
  },
  {
    key: 'drive-operations',
    routeName: 'tenant-admin-detail-drive-operations',
    labelKey: 'tenantAdmin.sections.driveOperations',
    descriptionKey: 'tenantAdmin.sectionDescriptions.driveOperations',
    icon: ShieldCheck,
  },
  {
    key: 'entitlements',
    routeName: 'tenant-admin-detail-entitlements',
    labelKey: 'tenantAdmin.sections.entitlements',
    descriptionKey: 'tenantAdmin.sectionDescriptions.entitlements',
    icon: KeyRound,
  },
  {
    key: 'support',
    routeName: 'tenant-admin-detail-support',
    labelKey: 'tenantAdmin.sections.support',
    descriptionKey: 'tenantAdmin.sectionDescriptions.support',
    icon: LifeBuoy,
  },
  {
    key: 'webhooks',
    routeName: 'tenant-admin-detail-webhooks',
    labelKey: 'tenantAdmin.sections.webhooks',
    descriptionKey: 'tenantAdmin.sectionDescriptions.webhooks',
    icon: Webhook,
  },
  {
    key: 'data',
    routeName: 'tenant-admin-detail-data',
    labelKey: 'tenantAdmin.sections.data',
    descriptionKey: 'tenantAdmin.sectionDescriptions.data',
    icon: Database,
  },
]

export function tenantAdminSectionTo(tenantSlug: string, section: TenantAdminSection): RouteLocationRaw {
  return {
    name: section.routeName,
    params: { tenantSlug },
  }
}
```

## Step 4. router を nested route にする

### 対象ファイル

```text
frontend/src/router/index.ts
frontend/src/views/TenantAdminTenantShellView.vue
frontend/src/views/tenant-admin/TenantAdminTenantOverviewView.vue
frontend/src/views/tenant-admin/TenantAdminTenantMembersView.vue
frontend/src/views/tenant-admin/TenantAdminTenantInvitationsView.vue
frontend/src/views/tenant-admin/TenantAdminTenantSettingsView.vue
frontend/src/views/tenant-admin/TenantAdminTenantDrivePolicyView.vue
frontend/src/views/tenant-admin/TenantAdminTenantDriveOperationsView.vue
frontend/src/views/tenant-admin/TenantAdminTenantEntitlementsView.vue
frontend/src/views/tenant-admin/TenantAdminTenantSupportView.vue
frontend/src/views/tenant-admin/TenantAdminTenantWebhooksView.vue
frontend/src/views/tenant-admin/TenantAdminTenantDataView.vue
```

### route 方針

既存の `/tenant-admin/:tenantSlug` は壊さず、空 child route として overview に redirect します。

```ts
{
  path: '/tenant-admin/:tenantSlug',
  component: TenantAdminTenantShellView,
  meta: {
    requiresAuth: true,
    title: 'Tenant Detail',
    group: 'Admin',
    titleKey: 'routes.tenantDetail',
    groupKey: 'nav.groups.admin',
  },
  children: [
    {
      path: '',
      name: 'tenant-admin-detail',
      redirect: (to) => ({
        name: 'tenant-admin-detail-overview',
        params: to.params,
      }),
    },
    {
      path: 'overview',
      name: 'tenant-admin-detail-overview',
      component: TenantAdminTenantOverviewView,
      meta: { titleKey: 'tenantAdmin.sections.overview' },
    },
    {
      path: 'members',
      name: 'tenant-admin-detail-members',
      component: TenantAdminTenantMembersView,
      meta: { titleKey: 'tenantAdmin.sections.members' },
    },
    {
      path: 'invitations',
      name: 'tenant-admin-detail-invitations',
      component: TenantAdminTenantInvitationsView,
      meta: { titleKey: 'tenantAdmin.sections.invitations' },
    },
    {
      path: 'settings',
      name: 'tenant-admin-detail-settings',
      component: TenantAdminTenantSettingsView,
      meta: { titleKey: 'tenantAdmin.sections.settings' },
    },
    {
      path: 'drive-policy',
      name: 'tenant-admin-detail-drive-policy',
      component: TenantAdminTenantDrivePolicyView,
      meta: { titleKey: 'tenantAdmin.sections.drivePolicy' },
    },
    {
      path: 'drive-operations',
      name: 'tenant-admin-detail-drive-operations',
      component: TenantAdminTenantDriveOperationsView,
      meta: { titleKey: 'tenantAdmin.sections.driveOperations' },
    },
    {
      path: 'entitlements',
      name: 'tenant-admin-detail-entitlements',
      component: TenantAdminTenantEntitlementsView,
      meta: { titleKey: 'tenantAdmin.sections.entitlements' },
    },
    {
      path: 'support',
      name: 'tenant-admin-detail-support',
      component: TenantAdminTenantSupportView,
      meta: { titleKey: 'tenantAdmin.sections.support' },
    },
    {
      path: 'webhooks',
      name: 'tenant-admin-detail-webhooks',
      component: TenantAdminTenantWebhooksView,
      meta: { titleKey: 'tenantAdmin.sections.webhooks' },
    },
    {
      path: 'data',
      name: 'tenant-admin-detail-data',
      component: TenantAdminTenantDataView,
      meta: { titleKey: 'tenantAdmin.sections.data' },
    },
  ],
}
```

tenant 一覧と tenant 作成画面からの遷移は、既存名 `tenant-admin-detail` を使い続けても overview へ redirect されます。ただし、意図が明確な code にするなら次のように overview route name へ更新します。

```ts
router.push({
  name: 'tenant-admin-detail-overview',
  params: { tenantSlug: created.slug },
})
```

## Step 5. shell view を作る

親 view は、tenant detail 全体に共通する layout だけを担当します。

### 対象ファイル

```text
frontend/src/views/TenantAdminTenantShellView.vue
```

### 親 view の責務

- route param から `tenantSlug` を読む
- tenant summary / memberships を load する
- `tenant_admin` 不足時の access denied を表示する
- header に tenant 名、slug、active 状態を表示する
- left sidebar を表示する
- child route を `<RouterView />` で表示する

### 実装例

```vue
<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import AdminAccessDenied from '../components/AdminAccessDenied.vue'
import SectionSideNav from '../components/SectionSideNav.vue'
import type { SectionSideNavItem } from '../components/section-side-nav'
import { useTenantAdminStore } from '../stores/tenant-admin'
import { tenantAdminSections, tenantAdminSectionTo } from '../tenant-admin/sections'

const route = useRoute()
const store = useTenantAdminStore()
const { t } = useI18n()

const tenantSlug = computed(() => {
  const raw = route.params.tenantSlug
  return Array.isArray(raw) ? raw[0] : raw ?? ''
})

const tenant = computed(() => store.current?.tenant ?? null)
const navItems = computed<SectionSideNavItem[]>(() => (
  tenantAdminSections.map((section) => ({
    key: section.key,
    label: t(section.labelKey),
    description: t(section.descriptionKey),
    icon: section.icon,
    to: tenantAdminSectionTo(tenantSlug.value, section),
  }))
))

async function loadTenant() {
  if (!tenantSlug.value) {
    return
  }
  await store.loadOne(tenantSlug.value)
}

onMounted(loadTenant)

watch(
  () => tenantSlug.value,
  loadTenant,
)
</script>

<template>
  <AdminAccessDenied
    v-if="store.status === 'forbidden'"
    :title="t('access.adminRequiredTitle')"
    :message="t('tenantAdmin.accessDenied')"
    role-label="tenant_admin"
  />

  <section v-else class="tenant-detail-layout">
    <SectionSideNav
      :aria-label="t('tenantAdmin.navigation')"
      :heading="tenant?.displayName ?? tenantSlug"
      :items="navItems"
    >
      <template #before>
        <RouterLink class="secondary-button link-button" to="/tenant-admin">
          {{ t('common.back') }}
        </RouterLink>
      </template>
    </SectionSideNav>

    <main class="tenant-detail-main">
      <header class="tenant-detail-header">
        <div>
          <span class="status-pill">Tenant Admin</span>
          <h1>{{ tenant?.displayName ?? tenantSlug }}</h1>
          <p v-if="tenant" class="cell-subtle">{{ tenant.slug }}</p>
        </div>
        <span v-if="tenant" :class="['status-pill', tenant.active ? '' : 'danger']">
          {{ tenant.active ? t('common.active') : t('common.inactive') }}
        </span>
      </header>

      <p v-if="store.status === 'loading'">{{ t('common.loading') }}</p>
      <p v-if="store.errorMessage" class="error-message">{{ store.errorMessage }}</p>

      <RouterView v-if="tenant" />
    </main>
  </section>
</template>
```

child view は同じ Pinia store を読むため、親から props で tenant を渡さなくても動きます。props drilling を避け、各 subpage は必要な store だけ import します。

## Step 6. subpage views に分割する

### ディレクトリ

```text
frontend/src/views/tenant-admin/
```

### Overview

#### ファイル

```text
frontend/src/views/tenant-admin/TenantAdminTenantOverviewView.vue
```

移すもの:

- `displayName`
- `active`
- `canSaveSettings`
- `formatDate`
- `saveSettings`
- `askDeactivate`
- `ConfirmActionDialog` の deactivate 部分

Overview は tenant 自体の基本設定だけを扱います。membership grant や Drive policy を置かないことが重要です。

### Members

#### ファイル

```text
frontend/src/views/tenant-admin/TenantAdminTenantMembersView.vue
```

移すもの:

- `tenantRoleOptions`
- `memberships`
- `grantUserEmail`
- `grantRoleCode`
- `canGrantRole`
- `grantRole`
- `askRevoke`
- `roleSourceClass`
- `userLabel`
- `ConfirmActionDialog` の revoke 部分

`tenantRoleOptions` は将来増えるため、view 内定数ではなく次のような tenant admin feature 定義へ逃がしてもよいです。

```text
frontend/src/tenant-admin/roles.ts
```

```ts
export const tenantRoleOptions = ['customer_signal_user', 'docs_reader', 'todo_user'] as const
```

### Invitations

#### ファイル

```text
frontend/src/views/tenant-admin/TenantAdminTenantInvitationsView.vue
```

移すもの:

- `invitationEmail`
- `invitationRoleCode`
- `canInvite`
- `createInvitation`
- `revokeInvitation`
- invitation list

この view は `useTenantCommonStore()` を使います。既存 store の `load(tenantSlug)` は settings / exports / imports / entitlements / webhooks もまとめて読むため、最初の分割では許容してもよいです。ただし、次の Step 7 で page-specific load に寄せます。

### Settings

#### ファイル

```text
frontend/src/views/tenant-admin/TenantAdminTenantSettingsView.vue
```

移すもの:

- `fileQuotaBytes`
- `browserRateLimit`
- `notificationsEnabled`
- `syncCommonForm` の common settings 部分
- `saveCommonSettings` の common settings 部分

Drive policy はここに残しません。Drive policy は設定項目が多く、別目的の subpage として独立させます。

### Drive Policy

#### ファイル

```text
frontend/src/views/tenant-admin/TenantAdminTenantDrivePolicyView.vue
```

移すもの:

- `driveExternalSharingEnabled`
- `driveRequireApproval`
- `drivePublicLinksEnabled`
- `drivePasswordLinksEnabled`
- `driveRequireLinkPassword`
- `driveAllowedDomains`
- `driveBlockedDomains`
- `driveMaxLinkTTLHours`
- `driveViewerDownloadEnabled`
- `driveExternalDownloadEnabled`
- `driveAdminContentAccessMode`
- `driveAnonymousEditorLinksEnabled`
- `driveContentScanEnabled`
- `driveDlpEnabled`
- `drivePlanCode`
- `driveMaxFileSizeBytes`
- `driveMaxWorkspaceCount`
- `driveSearchEnabled`
- `driveCollaborationEnabled`
- `driveSyncEnabled`
- `driveCmkEnabled`
- `driveDataResidencyEnabled`
- `driveLegalDiscoveryEnabled`
- `driveE2eeEnabled`
- `driveAiEnabled`
- `driveEncryptionMode`
- `drivePrimaryRegion`
- `driveAllowedRegions`
- `drivePolicyRows`
- `domainList`
- Drive 部分の `syncCommonForm`
- Drive 部分の `saveCommonSettings`

この view はさらに重くなりやすいため、form section component に分けます。

```text
frontend/src/components/tenant-admin/DriveSharingPolicyForm.vue
frontend/src/components/tenant-admin/DriveSecurityPolicyForm.vue
frontend/src/components/tenant-admin/DrivePlanPolicyForm.vue
frontend/src/components/tenant-admin/DriveResidencyPolicyForm.vue
```

最初から過度に細かくしすぎる必要はありません。目安は「1 component が 150-220 行を超え、form の目的が 2 つ以上混ざったら分ける」です。

### Drive Operations

#### ファイル

```text
frontend/src/views/tenant-admin/TenantAdminTenantDriveOperationsView.vue
```

移すもの:

- `store.driveHealth`
- `store.driveApprovals`
- `store.driveShares`
- `store.driveShareLinks`
- `store.driveSync`
- `store.driveAuditEvents`
- `approveDriveInvitation`
- `rejectDriveInvitation`
- `repairDriveSync`
- `store.loadDriveState(tenant.slug)`

Drive Operations は policy ではなく運用状態を見る画面です。share approval、OpenFGA drift、audit event など、operator が確認・修復する情報だけを置きます。

### Entitlements

#### ファイル

```text
frontend/src/views/tenant-admin/TenantAdminTenantEntitlementsView.vue
```

移すもの:

- `commonStore.entitlements`
- `saveEntitlements`

### Support

#### ファイル

```text
frontend/src/views/tenant-admin/TenantAdminTenantSupportView.vue
```

移すもの:

- `supportUserPublicId`
- `supportReason`
- `startSupportAccess`

### Webhooks

#### ファイル

```text
frontend/src/views/tenant-admin/TenantAdminTenantWebhooksView.vue
```

移すもの:

- `webhookName`
- `webhookUrl`
- `webhookEvents`
- `createWebhook`
- webhook list

### Data

#### ファイル

```text
frontend/src/views/tenant-admin/TenantAdminTenantDataView.vue
```

移すもの:

- `requestExport`
- `requestCSVExport`
- `importFile`
- `uploadImportCSV`
- `onImportFileChange`
- export list
- import list

Export と Import はどちらも tenant data lifecycle の操作です。最初の分割では 1 つの Data subpage にまとめます。将来項目が増える場合は `/data/exports` と `/data/imports` にさらに分けます。

## Step 7. store load を整理する

現行の `loadCurrent()` は次をまとめて実行しています。

```ts
await store.loadOne(tenantSlug.value)
await store.loadDriveState(tenantSlug.value)
await commonStore.load(tenantSlug.value)
```

サブページ化後は、全ページで Drive state や export list を読む必要はありません。

### 最初の移行

最初の移行では、親 shell が `store.loadOne()` だけを実行します。

```ts
await store.loadOne(tenantSlug.value)
```

各 subpage が必要になった data だけを load します。

| Subpage | load |
| --- | --- |
| Overview | 親 shell の `store.loadOne()` |
| Members | 親 shell の `store.loadOne()` |
| Invitations | `commonStore.load(tenantSlug)` または invitation 専用 action |
| Settings | `commonStore.load(tenantSlug)` または settings 専用 action |
| Drive Policy | `commonStore.load(tenantSlug)` |
| Drive Operations | `store.loadDriveState(tenantSlug)` |
| Entitlements | `commonStore.load(tenantSlug)` または entitlement 専用 action |
| Webhooks | `commonStore.load(tenantSlug)` または webhook 専用 action |
| Data | `commonStore.load(tenantSlug)` または export/import 専用 action |

### 次の改善

`useTenantCommonStore().load()` は便利ですが、サブページ分割後は読み込み範囲が広すぎます。必要に応じて次の action を追加します。

```ts
async loadSettings(tenantSlug: string)
async loadInvitations(tenantSlug: string)
async loadExports(tenantSlug: string)
async loadImports(tenantSlug: string)
async loadEntitlements(tenantSlug: string)
async loadWebhooks(tenantSlug: string)
```

API wrapper は既に分かれているため、backend や OpenAPI を変えずに store action だけ細分化できます。

## Step 8. CSS と responsive layout を追加する

Drive layout の考え方は使いますが、class 名は Drive 専用にしません。共通 sidebar 用に generic class を追加します。

### 対象ファイル

```text
frontend/src/style.css
```

### 追加する class

```css
.tenant-detail-layout {
  display: grid;
  grid-template-columns: 230px minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}

.tenant-detail-main {
  display: grid;
  gap: 14px;
  min-width: 0;
}

.tenant-detail-header,
.section-side-nav {
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: #fff;
  box-shadow: var(--shadow);
}

.tenant-detail-header {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
}

.section-side-nav {
  position: sticky;
  top: 88px;
  display: grid;
  gap: 14px;
  max-height: calc(100dvh - 112px);
  overflow: auto;
  padding: 14px;
}

.section-local-nav {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.section-side-heading {
  color: var(--muted);
  font-size: 0.8rem;
  font-weight: 700;
}

.section-local-link {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 9px;
  min-height: 40px;
  padding: 7px 10px;
  overflow: hidden;
  border-radius: var(--radius-sm);
  color: var(--text);
  text-decoration: none;
}

.section-local-link:hover {
  background: var(--surface-muted);
}

.section-local-link.router-link-active,
.section-local-link.router-link-exact-active {
  color: var(--accent-strong);
  background: #eff6ff;
}

.section-local-link span {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.section-local-link strong,
.section-local-link small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.section-local-link small {
  color: var(--muted);
  font-size: 0.78rem;
  font-weight: 500;
}
```

mobile では Drive と同じく 1 column にし、sidebar navigation を横スクロールできるようにします。

```css
@media (max-width: 860px) {
  .tenant-detail-layout {
    grid-template-columns: 1fr;
  }

  .section-side-nav {
    position: static;
    max-height: none;
  }

  .section-local-nav {
    display: flex;
    gap: 6px;
    overflow-x: auto;
    padding-bottom: 2px;
  }

  .section-local-link {
    flex: 0 0 auto;
    width: min(76vw, 240px);
  }

  .tenant-detail-header {
    align-items: stretch;
    flex-direction: column;
  }
}
```

## Step 9. i18n message を追加する

### 対象ファイル

```text
frontend/src/i18n/messages.ts
```

`P17` で入れた message catalog に tenant admin detail 用の key を追加します。

```ts
tenantAdmin: {
  navigation: 'Tenant detail navigation',
  accessDenied: 'This page requires the tenant_admin global role.',
  sections: {
    overview: 'Overview',
    members: 'Members',
    invitations: 'Invitations',
    settings: 'Settings',
    drivePolicy: 'Drive Policy',
    driveOperations: 'Drive Operations',
    entitlements: 'Entitlements',
    support: 'Support',
    webhooks: 'Webhooks',
    data: 'Data',
  },
  sectionDescriptions: {
    overview: 'Tenant identity and status',
    members: 'Roles and memberships',
    invitations: 'Pending user invites',
    settings: 'Quota and common settings',
    drivePolicy: 'Drive feature controls',
    driveOperations: 'Sharing, sync, and audit',
    entitlements: 'Feature gates',
    support: 'Support access sessions',
    webhooks: 'Outbound integrations',
    data: 'Exports and imports',
  },
}
```

日本語側も同じ key を持たせます。

```ts
tenantAdmin: {
  navigation: 'テナント詳細ナビゲーション',
  accessDenied: 'この画面を使うには global role tenant_admin が必要です。',
  sections: {
    overview: '概要',
    members: 'メンバー',
    invitations: '招待',
    settings: '設定',
    drivePolicy: 'Drive ポリシー',
    driveOperations: 'Drive 運用',
    entitlements: '権限',
    support: 'サポート',
    webhooks: 'Webhook',
    data: 'データ',
  },
  sectionDescriptions: {
    overview: 'テナント名と状態',
    members: 'ロールとメンバーシップ',
    invitations: '保留中のユーザー招待',
    settings: 'クォータと共通設定',
    drivePolicy: 'Drive 機能制御',
    driveOperations: '共有、同期、監査',
    entitlements: '機能ゲート',
    support: 'サポートアクセス',
    webhooks: '外部連携',
    data: 'エクスポートとインポート',
  },
}
```

この段階で、現行 `TenantAdminTenantDetailView.vue` に残っている日本語と英語混在の文字列も、移動先 subpage で message key に寄せます。

## Step 10. E2E を更新する

### 対象ファイル

```text
e2e/browser-journey.spec.ts
e2e/i18n.spec.ts
```

既存 E2E は `/tenant-admin/acme` で `Tenant Detail` heading を見ています。サブページ化後は redirect と sidebar navigation を確認します。

### browser journey の確認例

```ts
await page.goto('/tenant-admin/acme')
await expect(page).toHaveURL(/\/tenant-admin\/acme\/overview$/)
await expect(page.getByRole('navigation', { name: 'Tenant detail navigation' })).toBeVisible()
await expect(page.getByRole('link', { name: /Members/ })).toBeVisible()

await page.getByRole('link', { name: /Invitations/ }).click()
await expect(page).toHaveURL(/\/tenant-admin\/acme\/invitations$/)
await page.getByTestId('tenant-invitation-email').fill('demo@example.com')
await page.getByTestId('tenant-invitation-role').selectOption('todo_user')
```

### i18n E2E の確認例

日本語 locale に切り替えた状態で、sidebar label が日本語になることを確認します。

```ts
await page.goto('/tenant-admin/acme')
await page.getByTestId('locale-switcher').selectOption('ja')
await expect(page.getByRole('link', { name: /メンバー/ })).toBeVisible()
await expect(page.getByRole('link', { name: /Drive ポリシー/ })).toBeVisible()
```

## Step 11. build / binary で確認する

### frontend build

```bash
npm --prefix frontend run build
```

確認すること:

- Vue Router の nested route import が解決できる
- `section-side-nav.ts` の共通型を各 component / view から import できる
- `lucide-vue-next` icon import が tree-shake される
- i18n message key の型崩れがない

### E2E

```bash
make e2e
```

確認すること:

- `/tenant-admin/acme` が overview に redirect される
- tenant 一覧から detail に遷移できる
- tenant 作成後に overview に遷移できる
- Invitations subpage で既存 invitation test が通る
- sidebar link の active state が表示される
- mobile viewport でも sidebar と本文が重ならない

### single binary

```bash
make binary
```

binary 起動後、次の route が browser reload でも `index.html` に fallback されることを確認します。

```text
/tenant-admin/acme/overview
/tenant-admin/acme/members
/tenant-admin/acme/drive-policy
/tenant-admin/acme/data
```

`docs/TUTORIAL_SINGLE_BINARY.md` の方針どおり、`/api/*`、`/docs`、`/openapi.yaml` は SPA fallback させません。tenant admin の subpage route は拡張子なしの frontend route なので fallback 対象です。

## 実装時の注意点

### API contract を変えない

今回の分割は frontend information architecture の改善です。backend path や OpenAPI schema を変える必要はありません。

### 親 shell に form state を集めない

親 shell は layout と tenant context だけを持ちます。form state を親に残すと、ファイルは分かれても責務は分かれません。

### `TenantAdminTenantDetailView.vue` は最終的に削除する

新しい shell と subpage に移行できたら、旧 `TenantAdminTenantDetailView.vue` は残しません。互換性は router redirect で担保します。

### Drive policy と Drive operations を混ぜない

Drive policy は「設定」です。Drive operations は「現在状態の確認と修復」です。operator の導線が違うため、別 subpage にします。

### sidebar は tenant admin 専用にしない

`TenantAdminSideNav.vue` だけを作ると、次の管理画面で同じ実装を繰り返すことになります。link list と slot だけを持つ `SectionSideNav.vue` を共通 component にし、tenant admin 固有の section 定義は `frontend/src/tenant-admin/sections.ts` に置きます。

### E2E selector は subpage に寄せる

`Tenant Detail` という大きな heading だけを見る test は、分割後の意図を検証しません。sidebar navigation、URL、対象 form の `data-testid` を確認します。

## 完了チェックリスト

- `SectionSideNav.vue` が追加されている
- tenant admin section 定義が 1 ファイルに集約されている
- `/tenant-admin/:tenantSlug` が overview へ redirect される
- overview / members / invitations / settings / drive-policy / drive-operations / entitlements / support / webhooks / data の route がある
- shell view が sidebar と `<RouterView />` を表示している
- 旧 `TenantAdminTenantDetailView.vue` の責務が subpage に移動している
- sidebar label が i18n message catalog から表示される
- existing tenant admin journey E2E が新 URL 構成で通る
- `npm --prefix frontend run build` が通る
- `make e2e` が通る
- `make binary` 後に nested frontend route を reload できる
