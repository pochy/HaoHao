# P17: 多言語対応実装チュートリアル

## この文書の目的

この文書は、HaoHao の Vue frontend に多言語対応を追加するための実装チュートリアルです。

目的は、画面上の英語と日本語の文字列を一度に置き換えることではありません。目的は、`vue-i18n` を使って次の実装基盤を作り、既存画面を壊さず段階的に翻訳できる状態にすることです。

- locale catalog を frontend source として管理する
- user が UI から locale を切り替えられる
- route title、sidebar、topbar、主要 form label を翻訳できる
- 日付、数値、容量などの表示を locale に合わせられる
- 既存 E2E を壊さず、新しい i18n E2E を追加できる
- single binary build に含めても同じ挙動になる

この文書は、`docs/TUTORIAL.md` と `docs/TUTORIAL_SINGLE_BINARY.md` と同じように、「どのファイルを、どの順番で、何を確認しながら変更するか」を追える形にします。

## 参考にする外部情報

このチュートリアルでは、Vue 3 向けの公式 i18n plugin として `vue-i18n` を使います。

- Repository: `https://github.com/intlify/vue-i18n`
- Documentation: `https://vue-i18n.intlify.dev/`

2026-04-27 時点では、GitHub repository は Vue 3 向け Vue I18n v11 を stable として案内しています。公式 installation guide も `npm install vue-i18n@11` を示しています。Vue I18n v9 以降では Legacy API mode は v11 で deprecated、v12 で削除予定のため、このチュートリアルでは Composition API mode を正本にします。

Composition API mode では `createI18n` に `legacy: false` を設定し、component では `useI18n()` を `<script setup>` の top level で呼び出します。

## 前提と現在地

このチュートリアルは、現在の HaoHao repository が少なくとも次の状態にある前提で進めます。

- frontend は Vue 3 + Vite + TypeScript
- frontend state は Pinia
- routing は Vue Router
- root shell は `frontend/src/App.vue`
- global shell component は `AppSidebar.vue` と `AppTopbar.vue`
- shared UI component は `PageHeader.vue`、`DataCard.vue`、`StatusBadge.vue` などに分かれている
- frontend build output は `backend/web/dist/`
- `make binary` は frontend build artifact を Go binary に embed する

この P17 では backend API contract は変えません。OpenAPI schema も generated SDK も i18n の対象にしません。

### やらないこと

- backend error message をこの段階で多言語化しない
- Huma docs / OpenAPI description をこの段階で多言語化しない
- DB に user locale preference を追加しない
- `frontend/src/api/generated/*` を手書き編集しない
- 自動翻訳を source of truth にしない
- UI に表示する user generated content を翻訳しない

user locale はまず browser localStorage に保存します。tenant / user profile に保存したい場合は、後続フェーズで DB、API、OpenAPI、generated SDK、frontend store の順に追加します。

## 完成条件

このチュートリアルの完了条件は次です。

- `frontend/package.json` に `vue-i18n` が追加されている
- `frontend/src/i18n/*` に locale、message、format、storage の正本がある
- `frontend/src/main.ts` で `app.use(i18n)` している
- `AppTopbar` に locale switcher がある
- selected locale が reload 後も維持される
- `document.documentElement.lang` が selected locale と同期する
- route breadcrumb、sidebar、auth status、主要 navigation が翻訳される
- 既存 Playwright E2E は default English のまま通る
- 日本語 locale に切り替える Playwright E2E がある
- `npm --prefix frontend run build` が通る
- `make binary` 後の embedded frontend でも locale switch が動く

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | `frontend/package.json`, `package-lock.json` | `vue-i18n` を追加する |
| Step 2 | `frontend/src/i18n/*` | locale catalog と format 定義を作る |
| Step 3 | `frontend/src/main.ts` | Vue app に i18n plugin を登録する |
| Step 4 | `LocaleSwitcher.vue` | user が locale を切り替えられる UI を作る |
| Step 5 | `AppTopbar.vue`, `AppSidebar.vue` | shell の固定文言を翻訳する |
| Step 6 | `router/index.ts` | route meta を translation key に寄せる |
| Step 7 | shared components / views | 主要 view を段階的に翻訳する |
| Step 8 | date / number utilities | `Intl` 直書きを i18n format に寄せる |
| Step 9 | stores / API error | frontend static error と backend error の境界を整理する |
| Step 10 | E2E | default locale と locale switching を検証する |
| Step 11 | build / binary | Vite build と single binary で確認する |

## 手で書くファイルと生成物

### 手で書くファイル

```text
frontend/src/i18n/locales.ts
frontend/src/i18n/messages.ts
frontend/src/i18n/formats.ts
frontend/src/i18n/storage.ts
frontend/src/i18n/index.ts
frontend/src/components/LocaleSwitcher.vue
frontend/src/components/AppTopbar.vue
frontend/src/components/AppSidebar.vue
frontend/src/router/index.ts
frontend/src/views/*.vue
frontend/src/components/*.vue
frontend/src/stores/*.ts
e2e/i18n.spec.ts
```

### 更新されるが手書きしない生成物

```text
frontend/package-lock.json
backend/web/dist/*
```

`package-lock.json` は `npm install` が更新します。`backend/web/dist/*` は `npm --prefix frontend run build` が生成します。どちらも editor で手書き修正しません。

## Step 1. vue-i18n を追加する

### 対象ファイル

```text
frontend/package.json
frontend/package-lock.json
```

### コマンド

```bash
npm --prefix frontend install vue-i18n@11
```

### 確認コマンド

```bash
npm --prefix frontend ls vue-i18n
git diff -- frontend/package.json frontend/package-lock.json
```

`dependencies` に `vue-i18n` が入っていればよいです。Vite plugin はこの段階では追加しません。まずは TypeScript file の message catalog をそのまま bundle し、最小の移行面で始めます。

## Step 2. locale と message catalog を作る

### 対象ディレクトリ

```text
frontend/src/i18n/
```

### 方針

最初に対応する locale は `en` と `ja` にします。default locale は `en` にします。理由は、現在の UI と Playwright E2E が英語 label を多く参照しているためです。

日本語を default にする場合は、既存 E2E の role name / text assertion も同じ PR で更新します。

### 2-1. locale 定義

#### ファイル: `frontend/src/i18n/locales.ts`

```ts
export const supportedLocales = ['en', 'ja'] as const

export type AppLocale = typeof supportedLocales[number]

export const defaultLocale: AppLocale = 'en'

export function isAppLocale(value: string | null | undefined): value is AppLocale {
  return supportedLocales.includes(value as AppLocale)
}
```

### 2-2. localStorage

#### ファイル: `frontend/src/i18n/storage.ts`

```ts
import { defaultLocale, isAppLocale, type AppLocale } from './locales'

const localeStorageKey = 'haohao.locale'

export function loadPreferredLocale(): AppLocale {
  if (typeof window === 'undefined') {
    return defaultLocale
  }

  const savedLocale = window.localStorage.getItem(localeStorageKey)
  return isAppLocale(savedLocale) ? savedLocale : defaultLocale
}

export function savePreferredLocale(locale: AppLocale) {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(localeStorageKey, locale)
}
```

未保存時に `navigator.language` へ自動追従させないのは、既存 E2E と現在の英語 UI を安定させるためです。日本語を default にする場合は、既存 E2E の role name / text assertion も同じ PR で更新します。

### 2-3. message catalog

#### ファイル: `frontend/src/i18n/messages.ts`

```ts
import type { AppLocale } from './locales'

type MessageSchema = {
  app: {
    name: string
    tagline: string
  }
  common: {
    loading: string
    save: string
    cancel: string
    delete: string
    refresh: string
  }
  nav: {
    groups: {
      workspace: string
      work: string
      admin: string
      public: string
      authentication: string
    }
    items: {
      session: string
      notifications: string
      signals: string
      drive: string
      todos: string
      tenants: string
      machineClients: string
      integrations: string
      driveGroups: string
    }
  }
  auth: {
    guest: string
    signIn: string
    logout: string
    status: {
      authenticated: string
      anonymous: string
      loading: string
      idle: string
    }
  }
  settings: {
    locale: string
    localeNames: {
      en: string
      ja: string
    }
  }
  routes: {
    login: string
    invitation: string
    signalDetail: string
    driveFolder: string
    driveSearch: string
    driveShared: string
    driveStarred: string
    driveRecent: string
    driveStorage: string
    driveTrash: string
    publicDriveLink: string
    newTenant: string
    tenantDetail: string
    newMachineClient: string
    machineClientDetail: string
  }
}

const en: MessageSchema = {
  app: {
    name: 'HaoHao',
    tagline: 'Workspace OS',
  },
  common: {
    loading: 'Loading...',
    save: 'Save',
    cancel: 'Cancel',
    delete: 'Delete',
    refresh: 'Refresh',
  },
  nav: {
    groups: {
      workspace: 'Workspace',
      work: 'Work',
      admin: 'Admin',
      public: 'Public',
      authentication: 'Authentication',
    },
    items: {
      session: 'Session',
      notifications: 'Notifications',
      signals: 'Signals',
      drive: 'Drive',
      todos: 'TODO',
      tenants: 'Tenants',
      machineClients: 'Machine Clients',
      integrations: 'Integrations',
      driveGroups: 'Drive groups',
    },
  },
  auth: {
    guest: 'Guest',
    signIn: 'Sign in',
    logout: 'Logout',
    status: {
      authenticated: 'Authenticated',
      anonymous: 'Anonymous',
      loading: 'Checking',
      idle: 'Idle',
    },
  },
  settings: {
    locale: 'Language',
    localeNames: {
      en: 'English',
      ja: 'Japanese',
    },
  },
  routes: {
    login: 'Login',
    invitation: 'Invitation',
    signalDetail: 'Signal Detail',
    driveFolder: 'Drive Folder',
    driveSearch: 'Drive Search',
    driveShared: 'Shared with me',
    driveStarred: 'Starred',
    driveRecent: 'Recent Drive',
    driveStorage: 'Drive Storage',
    driveTrash: 'Drive Trash',
    publicDriveLink: 'Public Drive Link',
    newTenant: 'New Tenant',
    tenantDetail: 'Tenant Detail',
    newMachineClient: 'New Machine Client',
    machineClientDetail: 'Machine Client Detail',
  },
}

const ja: MessageSchema = {
  app: {
    name: 'HaoHao',
    tagline: 'ワークスペース OS',
  },
  common: {
    loading: '読み込み中...',
    save: '保存',
    cancel: 'キャンセル',
    delete: '削除',
    refresh: '更新',
  },
  nav: {
    groups: {
      workspace: 'ワークスペース',
      work: '業務',
      admin: '管理',
      public: '公開',
      authentication: '認証',
    },
    items: {
      session: 'セッション',
      notifications: '通知',
      signals: 'シグナル',
      drive: 'ドライブ',
      todos: 'TODO',
      tenants: 'テナント',
      machineClients: 'マシンクライアント',
      integrations: '連携',
      driveGroups: 'Drive グループ',
    },
  },
  auth: {
    guest: 'ゲスト',
    signIn: 'サインイン',
    logout: 'ログアウト',
    status: {
      authenticated: '認証済み',
      anonymous: '未認証',
      loading: '確認中',
      idle: '待機中',
    },
  },
  settings: {
    locale: '言語',
    localeNames: {
      en: '英語',
      ja: '日本語',
    },
  },
  routes: {
    login: 'ログイン',
    invitation: '招待',
    signalDetail: 'シグナル詳細',
    driveFolder: 'Drive フォルダ',
    driveSearch: 'Drive 検索',
    driveShared: '共有アイテム',
    driveStarred: 'スター付き',
    driveRecent: '最近の Drive',
    driveStorage: 'Drive ストレージ',
    driveTrash: 'Drive ゴミ箱',
    publicDriveLink: '公開 Drive リンク',
    newTenant: '新規テナント',
    tenantDetail: 'テナント詳細',
    newMachineClient: '新規マシンクライアント',
    machineClientDetail: 'マシンクライアント詳細',
  },
}

export const messages: Record<AppLocale, MessageSchema> = {
  en,
  ja,
}
```

message key は英語文そのものにしません。`nav.items.drive` のように、UI 上の意味に対して安定した key を付けます。

### 2-4. 日付と数値 format

#### ファイル: `frontend/src/i18n/formats.ts`

```ts
export const datetimeFormats = {
  en: {
    short: {
      year: 'numeric',
      month: 'short',
      day: '2-digit',
    },
    long: {
      year: 'numeric',
      month: 'short',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    },
  },
  ja: {
    short: {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    },
    long: {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    },
  },
} as const

export const numberFormats = {
  en: {
    integer: {
      maximumFractionDigits: 0,
    },
    percent: {
      style: 'percent',
      maximumFractionDigits: 1,
    },
  },
  ja: {
    integer: {
      maximumFractionDigits: 0,
    },
    percent: {
      style: 'percent',
      maximumFractionDigits: 1,
    },
  },
} as const
```

容量表示は `KB` / `MB` のような binary unit を使うため、`numberFormats` だけでは完結しません。`formatDriveSize` のような utility は残しつつ、数値部分だけ `n()` に寄せます。

### 2-5. i18n instance

#### ファイル: `frontend/src/i18n/index.ts`

```ts
import { createI18n } from 'vue-i18n'

import { datetimeFormats, numberFormats } from './formats'
import { defaultLocale, type AppLocale } from './locales'
import { messages } from './messages'
import { loadPreferredLocale, savePreferredLocale } from './storage'

const initialLocale = loadPreferredLocale()
document.documentElement.lang = initialLocale

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: initialLocale,
  fallbackLocale: defaultLocale,
  messages,
  datetimeFormats,
  numberFormats,
  missingWarn: import.meta.env.DEV,
  fallbackWarn: import.meta.env.DEV,
})

export function setI18nLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  savePreferredLocale(locale)
  document.documentElement.lang = locale
}
```

`legacy: false` が Composition API mode の入口です。`globalInjection: true` により template 内では `$t()` も使えますが、新規実装では `<script setup>` の top level で `useI18n()` を呼び、`t()` / `d()` / `n()` を明示して使います。

## Step 3. Vue app に i18n を登録する

### 対象ファイル

```text
frontend/src/main.ts
```

### 変更例

```ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import { i18n } from './i18n'
import router from './router'
import './style.css'

const app = createApp(App)

app.use(createPinia())
app.use(i18n)
app.use(router)
app.mount('#app')
```

plugin registration は `mount` より前に行います。

### 確認コマンド

```bash
npm --prefix frontend run build
```

この時点では UI がまだ翻訳されていなくてもかまいません。まず i18n plugin が build に入ることを確認します。

## Step 4. LocaleSwitcher を作る

### 対象ファイル

```text
frontend/src/components/LocaleSwitcher.vue
frontend/src/style.css
```

### component

#### ファイル: `frontend/src/components/LocaleSwitcher.vue`

```vue
<script setup lang="ts">
import { Languages } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import { setI18nLocale } from '../i18n'
import { isAppLocale, supportedLocales } from '../i18n/locales'

const { locale, t } = useI18n({ useScope: 'global' })

function changeLocale(event: Event) {
  const nextLocale = (event.target as HTMLSelectElement).value
  if (isAppLocale(nextLocale)) {
    setI18nLocale(nextLocale)
  }
}
</script>

<template>
  <label class="locale-switcher">
    <Languages :size="16" stroke-width="1.8" aria-hidden="true" />
    <span class="sr-only">{{ t('settings.locale') }}</span>
    <select
      data-testid="locale-switcher"
      class="locale-switcher-select"
      :aria-label="t('settings.locale')"
      :value="locale"
      @change="changeLocale"
    >
      <option v-for="item in supportedLocales" :key="item" :value="item">
        {{ t(`settings.localeNames.${item}`) }}
      </option>
    </select>
  </label>
</template>
```

### CSS

#### ファイル: `frontend/src/style.css`

```css
.locale-switcher {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 34px;
  padding: 0 8px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: #fff;
  color: var(--text);
}

.locale-switcher-select {
  min-width: 92px;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
```

既に `.sr-only` 相当の utility がある場合は重複追加せず、既存 class を使います。

## Step 5. topbar と sidebar を翻訳する

### 対象ファイル

```text
frontend/src/components/AppTopbar.vue
frontend/src/components/AppSidebar.vue
```

### AppTopbar の方針

`route.meta.title` / `route.meta.group` は既存 route との互換として残し、`titleKey` / `groupKey` があれば translation key を優先します。

#### 変更例

```ts
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import LocaleSwitcher from './LocaleSwitcher.vue'

const { t } = useI18n()
const route = useRoute()

const routeLabel = computed(() => {
  const key = route.meta.titleKey
  return typeof key === 'string' ? t(key) : String(route.meta.title ?? t('app.name'))
})

const routeGroup = computed(() => {
  const key = route.meta.groupKey
  return typeof key === 'string' ? t(key) : String(route.meta.group ?? t('nav.groups.workspace'))
})
```

auth status は switch の戻り値を直接 English にしないで、key に変換します。

```ts
const statusLabel = computed(() => {
  switch (sessionStore.status) {
    case 'authenticated':
      return t('auth.status.authenticated')
    case 'anonymous':
      return t('auth.status.anonymous')
    case 'loading':
      return t('auth.status.loading')
    default:
      return t('auth.status.idle')
  }
})
```

template の topbar action に locale switcher を追加します。

```vue
<LocaleSwitcher />
```

### AppSidebar の方針

navigation definition は label ではなく label key を持つ形にします。

```ts
type NavigationItem = {
  to: string
  labelKey: string
  icon: Component
}

type NavigationGroup = {
  labelKey: string
  items: NavigationItem[]
}

const navigationGroups: NavigationGroup[] = [
  {
    labelKey: 'nav.groups.workspace',
    items: [
      { to: '/', labelKey: 'nav.items.session', icon: Home },
      { to: '/notifications', labelKey: 'nav.items.notifications', icon: Bell },
    ],
  },
]
```

template では `t()` を使います。

```vue
<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <strong>{{ t('app.name') }}</strong>
  <span>{{ t('app.tagline') }}</span>

  <section v-for="group in navigationGroups" :key="group.labelKey" class="sidebar-group">
    <h2>{{ t(group.labelKey) }}</h2>
    <RouterLink v-for="item in group.items" :key="item.to" class="sidebar-link" :to="item.to">
      <component :is="item.icon" class="sidebar-link-icon" :size="17" stroke-width="1.8" aria-hidden="true" />
      <span>{{ t(item.labelKey) }}</span>
    </RouterLink>
  </section>
</template>
```

## Step 6. route meta を translation key に寄せる

### 対象ファイル

```text
frontend/src/router/index.ts
```

### 型追加

Vue Router の `RouteMeta` に i18n key を追加します。

```ts
declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    title?: string
    group?: string
    titleKey?: string
    groupKey?: string
  }
}
```

### route meta の変更例

```ts
{
  path: '/',
  name: 'home',
  component: HomeView,
  meta: {
    requiresAuth: true,
    title: 'Session',
    group: 'Workspace',
    titleKey: 'nav.items.session',
    groupKey: 'nav.groups.workspace',
  },
},
{
  path: '/login',
  name: 'login',
  component: LoginView,
  meta: {
    title: 'Login',
    group: 'Authentication',
    titleKey: 'routes.login',
    groupKey: 'nav.groups.authentication',
  },
},
```

既存の `title` / `group` は一度に消さなくてよいです。まず `titleKey` / `groupKey` を追加し、AppTopbar 側が key を優先する形にすれば、未移行 route も壊れません。

## Step 7. 主要 view を段階的に翻訳する

### 対象ファイル

```text
frontend/src/views/HomeView.vue
frontend/src/views/LoginView.vue
frontend/src/views/NotificationsView.vue
frontend/src/views/TodosView.vue
frontend/src/views/CustomerSignalsView.vue
frontend/src/views/DriveView.vue
frontend/src/views/TenantAdminTenantsView.vue
frontend/src/views/MachineClientsView.vue
frontend/src/views/IntegrationsView.vue
```

### 進め方

最初に shell と login / home を翻訳し、その後に業務 view を 1 画面ずつ進めます。

1. `PageHeader` の `eyebrow` / `title` / `description`
2. form label
3. button label
4. empty / loading / error state
5. table header
6. status badge
7. success toast / notice message
8. dialog title / confirm message

### `<script setup>` の基本形

```vue
<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <PageHeader
    :eyebrow="t('nav.groups.workspace')"
    :title="t('nav.items.session')"
    :description="t('session.description')"
  />
</template>
```

`useI18n()` は `<script setup>` の top level で呼びます。event handler や `computed` の内側で初めて呼ばないようにします。

### enum / status の扱い

backend から返る enum value はそのまま business value として扱い、表示境界で翻訳します。

```ts
const priorityLabelKeys = {
  low: 'signals.priority.low',
  medium: 'signals.priority.medium',
  high: 'signals.priority.high',
  urgent: 'signals.priority.urgent',
} as const
```

template:

```vue
{{ t(priorityLabelKeys[signal.priority]) }}
```

backend enum から translation key を文字列連結で作る場合は、未定義 value の fallback を必ず用意します。API contract 外の値が来たときに raw key を画面へ出さないためです。

## Step 8. 日付、数値、容量を locale に合わせる

### 対象ファイル

```text
frontend/src/utils/driveItems.ts
frontend/src/views/*.vue
frontend/src/components/*.vue
```

### 方針

component 内の日付と数値は `useI18n()` の `d()` / `n()` を使います。

```ts
const { d, n } = useI18n()
```

例:

```vue
<time :datetime="item.updatedAt">{{ d(new Date(item.updatedAt), 'long') }}</time>
<span>{{ n(totalCount, 'integer') }}</span>
```

utility 関数で日付を format している箇所は、次のどちらかに寄せます。

- component に format を移して `d()` / `n()` を使う
- `locale` を引数で受け取る pure utility にする

既存の `formatDriveSize(value)` は容量単位を持つため、そのまま `d()` / `n()` には置き換えません。必要なら次のように数値部分だけ分離します。

```ts
function driveSizeParts(value: number) {
  if (value < 1024) {
    return { value, unit: 'B' }
  }
  if (value < 1024 * 1024) {
    return { value: value / 1024, unit: 'KB' }
  }
  return { value: value / 1024 / 1024, unit: 'MB' }
}
```

template:

```vue
<span>{{ n(size.value, 'integer') }} {{ size.unit }}</span>
```

## Step 9. frontend error と backend error の境界を決める

### 対象ファイル

```text
frontend/src/stores/*.ts
frontend/src/api/client.ts
frontend/src/views/*.vue
```

### 方針

frontend が作っている static error は translation key に置き換えます。

例:

```ts
this.errorMessageKey = 'todos.errors.titleRequired'
```

template:

```vue
<p v-if="store.errorMessageKey" class="error-message">
  {{ t(store.errorMessageKey) }}
</p>
```

一方、backend から返る error message はこの段階では raw message として表示してよいです。backend error を翻訳する場合は、message ではなく stable error code を API response に追加してから frontend で key に map します。

後続で backend error code を入れる場合の順番は次です。

1. Go error response に `code` を追加する
2. Huma / OpenAPI を更新する
3. `make gen` で generated SDK を更新する
4. frontend の `toApiErrorMessage` を `toApiErrorKey` へ拡張する
5. view で `t(errorKey)` を使う

## Step 10. E2E を追加する

### 対象ファイル

```text
e2e/i18n.spec.ts
e2e/fixtures/auth.ts
```

既存 E2E は default `en` の label を前提にしています。default locale を `en` に保つ限り、大きく更新する必要はありません。

locale switching の専用 E2E を追加します。

#### ファイル: `e2e/i18n.spec.ts`

```ts
import { expect, test } from '@playwright/test'

import { login } from './fixtures/auth'

test('user can switch locale and keep it after reload', async ({ page }) => {
  await login(page)

  await page.getByTestId('locale-switcher').selectOption('ja')
  await expect(page.getByRole('link', { name: '通知' })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('lang', 'ja')

  await page.reload()
  await expect(page.getByTestId('locale-switcher')).toHaveValue('ja')
  await expect(page.getByRole('link', { name: '通知' })).toBeVisible()

  await page.getByTestId('locale-switcher').selectOption('en')
  await expect(page.getByRole('link', { name: 'Notifications' })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('lang', 'en')
})
```

`getByRole` の name を使う既存 E2E は accessibility label の検証として有効です。ただし i18n 対応後は locale に依存します。locale を切り替える test では `data-testid="locale-switcher"` のような安定 selector と、翻訳後の role name の両方を使います。

### 確認コマンド

```bash
npm --prefix frontend run build
make e2e
```

OpenFGA が必要な Drive E2E は既存の skip 条件に従います。

## Step 11. single binary で確認する

`docs/TUTORIAL_SINGLE_BINARY.md` の前提どおり、production build は `backend/web/dist/` に出力され、それを Go binary に embed します。

### 確認コマンド

```bash
npm --prefix frontend run build
make binary
./bin/haohao
```

browser で次を確認します。

- `http://127.0.0.1:8080/` で SPA が開く
- locale switcher が表示される
- `ja` に切り替えると sidebar / topbar が日本語になる
- reload 後も selected locale が維持される
- `/docs` と `/openapi.yaml` は SPA fallback されない

dev server と embedded binary は origin が異なります。

- dev: `http://127.0.0.1:5173`
- embedded: `http://127.0.0.1:8080`

localStorage は origin ごとに分かれるため、dev server で選んだ locale が embedded binary 側に自動では引き継がれないことに注意します。

## Step 12. 移行 checklist

### 最初の PR

- `vue-i18n@11` を追加する
- `frontend/src/i18n/*` を追加する
- `main.ts` に `app.use(i18n)` を追加する
- `LocaleSwitcher.vue` を追加する
- `AppTopbar.vue` と `AppSidebar.vue` を翻訳する
- route meta に `titleKey` / `groupKey` を追加する
- `e2e/i18n.spec.ts` を追加する
- `npm --prefix frontend run build` を通す

### 後続 PR

- `HomeView.vue` と `LoginView.vue`
- `TodosView.vue`
- `NotificationsView.vue`
- `CustomerSignalsView.vue`
- `DriveView.vue` と Drive components
- tenant admin / machine clients / integrations
- stores の static error
- date / number / size formatting
- backend error code を使った error translation
- user profile locale preference

## 失敗時の見方

### `useI18n` が使えない

`createI18n` に `legacy: false` があるか確認します。Composition API mode を使うにはこの設定が必要です。

### template で key がそのまま表示される

message catalog に key がありません。`missingWarn` / `fallbackWarn` は dev で警告を出すため、browser console を確認します。

### Playwright の既存 test が落ちる

default locale が `en` のままか確認します。日本語 default に変えた場合は、role name を参照している E2E を同じ PR で更新します。

### Japanese text が button からはみ出す

翻訳後の文言は英語より長くなることがあります。固定幅 button / card / table cell は `min-width`、`max-width`、`white-space`、`text-overflow` を見直します。i18n 対応では文字列だけでなく layout の検証も必要です。

### embedded binary で古い文言が出る

`make binary` の前に `npm --prefix frontend run build` が実行されているか確認します。single binary は build 時点の `backend/web/dist/` を embed します。

## 完了確認

最後に次を通します。

```bash
npm --prefix frontend run build
make e2e
make binary
git diff --check -- docs/TUTORIAL_P17_I18N.md
```

このチュートリアルを最後まで進めると、HaoHao は `en` / `ja` の UI 切替を持ち、既存機能と single binary 配信を保ったまま、画面単位で翻訳を増やせる状態になります。
