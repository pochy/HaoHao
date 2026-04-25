# P1 管理 UI を補完するチュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **P1: 管理 UI を補完する** を、実装できる順番に分解したチュートリアルです。

P0 では request logging、health/readiness、provisioning scheduler、smoke、runbook を入れて、既存 backend surface を運用しやすくしました。P1 では、新しい backend API を増やす前に、すでに存在する tenant / machine client / integrations / docs の surface を browser から自然に操作できるようにします。

このチュートリアルで扱う P1 は次の 4 点です。

- tenant selector
- machine client admin UI
- integrations UX の整理
- docs link hardening

`TUTORIAL.md` や `TUTORIAL_SINGLE_BINARY.md` と同じく、どのファイルを触り、どの順で確認するかを追える形にします。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- `frontend` は Vue 3 + Vite + TypeScript + Pinia + Vue Router で動いている
- `frontend/src/api/generated/*` は `openapi/openapi.yaml` から生成されている
- `frontend/src/api/client.ts` が generated SDK の共通 fetch / CSRF / Cookie 設定を持っている
- `frontend/src/stores/session.ts` が login / logout / current session を管理している
- `frontend/src/views/HomeView.vue` と `frontend/src/views/IntegrationsView.vue` が存在する
- `GET /api/v1/tenants` と `POST /api/v1/session/tenant` が backend に存在する
- `GET /api/v1/machine-clients`、`POST /api/v1/machine-clients`、`GET /api/v1/machine-clients/{id}`、`PUT /api/v1/machine-clients/{id}`、`DELETE /api/v1/machine-clients/{id}` が backend に存在する
- machine client API は browser session + CSRF + global role `machine_client_admin` で保護されている
- integrations API は active tenant がない場合に `409` を返す
- P0 により `make smoke-operability` と `RUNBOOK_OPERABILITY.md` が存在する

この文書では、backend API contract は原則変えません。OpenAPI に既にある endpoint を frontend から使い切ることを優先します。どうしても API の情報が足りないと判明した場合だけ、その時点で別 P1.5 として backend contract 変更を切り出します。

## 確認時に重要な前提

この P1 は login 方法と server の起動方法で見える結果が変わります。実装前に次を固定しておくと、動作確認で迷いません。

- local password login で確認する場合の user は `demo@example.com`
- Zitadel login で確認する場合の user は、Zitadel callback 後に local DB に作られた email
- Step 8 の local seed は `demo@example.com` 用なので、Zitadel login user には同じ role / tenant membership が自動では付きません
- tenant selector はログイン中 user の `tenant_memberships` だけを表示する
- machine client admin は tenant role ではなく global role `machine_client_admin` を見る
- role / membership を DB で変えた後は、画面 reload または `Refresh` で API を再取得する
- local login と Zitadel login を切り替えるときは、古い browser session が残らないように logout するか private window を使う
- Zitadel login では再ログイン時に provider claim / group から local role が再同期されることがある
- frontend dev server は `http://127.0.0.1:5173`、single binary / smoke の基準 URL は `http://127.0.0.1:8080`
- `make smoke-operability` は server を起動せず、既に `:8080` で動いている process に対して確認する
- macOS / zsh で `diff` が `delta` に alias されている場合があるため、OpenAPI drift の確認は `/usr/bin/diff -u` または `git diff --no-index` を使う

## 完成条件

このチュートリアルの完了条件は次です。

- app header から active tenant が分かる
- `GET /api/v1/tenants` の結果で tenant 一覧を表示できる
- `POST /api/v1/session/tenant` で active tenant を切り替えられる
- tenant を切り替えた後、tenant 依存の integrations 表示が再読込される
- `/machine-clients` で machine client 一覧を表示できる
- `/machine-clients/new` で machine client を作成できる
- `/machine-clients/:id` で detail / update / disable ができる
- `machine_client_admin` role がない user では、machine client 画面が破綻せず 403 用 UI を出す
- integrations 画面で active tenant、connect / verify / revoke の状態、last error、callback result が読みやすく表示される
- docs link は、docs access が拒否された場合に JSON error 画面へ飛ばず、frontend 上で理由を表示する
- `npm --prefix frontend run build` が通る
- `go test ./backend/...` が通る
- `make binary` が通る
- single binary を `:8080` で起動した状態で `make smoke-operability` が通る

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | `frontend/src/api/*` | tenant / machine client / docs の API wrapper を作る |
| Step 2 | `frontend/src/stores/*` | tenant と machine client の state を Pinia に寄せる |
| Step 3 | `frontend/src/components/TenantSelector.vue`, `frontend/src/App.vue` | tenant selector を app shell に接続する |
| Step 4 | `frontend/src/router/index.ts`, `frontend/src/views/*` | machine client admin 画面を追加する |
| Step 5 | `frontend/src/views/IntegrationsView.vue` | active tenant 前提の UX に整理する |
| Step 6 | `frontend/src/components/DocsLink.vue`, `frontend/src/views/HomeView.vue` | docs access の失敗を画面内で扱う |
| Step 7 | `frontend/src/style.css` | 管理 UI 用の共通 style を足す |
| Step 8 | local / binary / Docker | 動作確認を固定する |

## Step 1. frontend API wrapper を追加する

まず、generated SDK を view から直接呼ばないように、既存の `frontend/src/api/session.ts` / `frontend/src/api/integrations.ts` と同じ層に wrapper を追加します。

### 1-1. tenant API wrapper を追加する

#### ファイル: `frontend/src/api/tenants.ts`

```ts
import type { ListTenantsBody, TenantBody } from './generated/types.gen'
import { listTenants, selectTenant } from './generated/sdk.gen'
import { readCookie } from './client'

export async function fetchTenants(): Promise<ListTenantsBody> {
  const data = await listTenants({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as ListTenantsBody

  return {
    ...data,
    items: data.items ?? [],
  }
}

export async function switchActiveTenant(tenantSlug: string): Promise<TenantBody> {
  const data = await selectTenant({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    body: {
      tenantSlug,
    },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { activeTenant: TenantBody }

  return data.activeTenant
}
```

`frontend/src/api/client.ts` は mutating request の CSRF を自動補完できますが、既存 wrapper と揃えるために `X-CSRF-Token` を明示します。明示しても cookie が無い初回 request では共通 fetch が `/api/v1/csrf` を取りに行けます。

### 1-2. machine client API wrapper を追加する

#### ファイル: `frontend/src/api/machine-clients.ts`

```ts
import { readCookie } from './client'
import {
  createMachineClient,
  deleteMachineClient,
  getMachineClient,
  listMachineClients,
  updateMachineClient,
} from './generated/sdk.gen'
import type {
  MachineClientBody,
  MachineClientRequestBody,
} from './generated/types.gen'

export async function fetchMachineClients(): Promise<MachineClientBody[]> {
  const data = await listMachineClients({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items: MachineClientBody[] | null }

  return data.items ?? []
}

export async function fetchMachineClient(id: number): Promise<MachineClientBody> {
  return getMachineClient({
    path: { id },
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<MachineClientBody>
}

export async function createMachineClientFromForm(
  body: MachineClientRequestBody,
): Promise<MachineClientBody> {
  return createMachineClient({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<MachineClientBody>
}

export async function updateMachineClientFromForm(
  id: number,
  body: MachineClientRequestBody,
): Promise<MachineClientBody> {
  return updateMachineClient({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    path: { id },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<MachineClientBody>
}

export async function disableMachineClient(id: number): Promise<void> {
  await deleteMachineClient({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    path: { id },
    responseStyle: 'data',
    throwOnError: true,
  })
}
```

backend の `DELETE /api/v1/machine-clients/{id}` は物理削除ではなく無効化です。UI 上の文言も `Delete` ではなく `Disable` にします。

### 1-3. docs access check wrapper を追加する

#### ファイル: `frontend/src/api/docs.ts`

```ts
export async function checkDocsAccess(): Promise<void> {
  const response = await fetch('/docs', {
    method: 'GET',
    credentials: 'include',
    headers: {
      Accept: 'text/html',
    },
  })

  if (response.ok) {
    return
  }

  if (response.status === 401) {
    throw new Error('Login is required to open docs.')
  }
  if (response.status === 403) {
    throw new Error('docs_reader role is required to open docs.')
  }

  throw new Error(`Docs are unavailable: HTTP ${response.status}`)
}
```

`/docs` を直接 `target="_blank"` で開くと、docs auth が有効な環境では browser が JSON error をそのまま表示する可能性があります。frontend で先に access check し、失敗時は現在画面の中で理由を出します。

### 1-4. API error helper を補強する

#### ファイル: `frontend/src/api/client.ts`

generated client は失敗時に `Error` instance ではなく Problem JSON を throw することがあります。403 を UI state に分けるため、既存の `toApiErrorMessage()` と同じ場所に status helper を置きます。

```ts
type ProblemLike = Partial<Pick<ErrorModel, 'detail' | 'title'>> & {
  message?: string
  status?: number
}

export function toApiErrorStatus(error: unknown): number | undefined {
  if (error && typeof error === 'object' && 'status' in error) {
    const status = (error as ProblemLike).status
    return typeof status === 'number' ? status : undefined
  }

  return undefined
}

export function isApiForbidden(error: unknown): boolean {
  if (toApiErrorStatus(error) === 403) {
    return true
  }

  const message = toApiErrorMessage(error)
  return /forbidden|machine_client_admin|docs_reader/i.test(message)
}
```

## Step 2. Pinia store を追加する

API wrapper の次に、view から使う state を Pinia に寄せます。tenant は複数 view で共有し、machine client は管理画面の中で使います。

### 2-1. tenant store を追加する

#### ファイル: `frontend/src/stores/tenants.ts`

```ts
import { defineStore } from 'pinia'

import { toApiErrorMessage } from '../api/client'
import { fetchTenants, switchActiveTenant } from '../api/tenants'
import type { TenantBody } from '../api/generated/types.gen'

type TenantStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'error'

export const useTenantStore = defineStore('tenants', {
  state: () => ({
    status: 'idle' as TenantStatus,
    items: [] as TenantBody[],
    activeTenant: null as TenantBody | null,
    defaultTenant: null as TenantBody | null,
    errorMessage: '',
    switchingSlug: '',
  }),

  getters: {
    hasMultipleTenants: (state) => state.items.length > 1,
  },

  actions: {
    async load() {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        const data = await fetchTenants()
        this.items = data.items ?? []
        this.activeTenant = data.activeTenant ?? this.items.find((item) => item.selected) ?? null
        this.defaultTenant = data.defaultTenant ?? this.items.find((item) => item.default) ?? null
        this.status = this.items.length > 0 ? 'ready' : 'empty'
      } catch (error) {
        this.items = []
        this.activeTenant = null
        this.defaultTenant = null
        this.status = 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async select(tenantSlug: string) {
      if (!tenantSlug || tenantSlug === this.activeTenant?.slug) {
        return
      }

      this.switchingSlug = tenantSlug
      this.errorMessage = ''

      try {
        const activeTenant = await switchActiveTenant(tenantSlug)
        this.activeTenant = activeTenant
        this.items = this.items.map((item) => ({
          ...item,
          selected: item.slug === activeTenant.slug,
        }))
      } catch (error) {
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.switchingSlug = ''
      }
    },

    reset() {
      this.status = 'idle'
      this.items = []
      this.activeTenant = null
      this.defaultTenant = null
      this.errorMessage = ''
      this.switchingSlug = ''
    },
  },
})
```

tenant store は login session とは分けます。`GET /api/v1/session` の `UserResponse` は user identity の最小情報であり、tenant 一覧は `GET /api/v1/tenants` が正本です。

### 2-2. machine client store を追加する

#### ファイル: `frontend/src/stores/machine-clients.ts`

```ts
import { defineStore } from 'pinia'

import { isApiForbidden, toApiErrorMessage } from '../api/client'
import {
  createMachineClientFromForm,
  disableMachineClient,
  fetchMachineClient,
  fetchMachineClients,
  updateMachineClientFromForm,
} from '../api/machine-clients'
import type {
  MachineClientBody,
  MachineClientRequestBody,
} from '../api/generated/types.gen'

type AdminStatus = 'idle' | 'loading' | 'ready' | 'forbidden' | 'error'

export const useMachineClientStore = defineStore('machineClients', {
  state: () => ({
    status: 'idle' as AdminStatus,
    items: [] as MachineClientBody[],
    current: null as MachineClientBody | null,
    errorMessage: '',
    saving: false,
  }),

  actions: {
    async loadList() {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        this.items = await fetchMachineClients()
        this.status = 'ready'
      } catch (error) {
        this.items = []
        this.status = isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async loadOne(id: number) {
      this.status = 'loading'
      this.errorMessage = ''

      try {
        this.current = await fetchMachineClient(id)
        this.status = 'ready'
      } catch (error) {
        this.current = null
        this.status = isApiForbidden(error) ? 'forbidden' : 'error'
        this.errorMessage = toApiErrorMessage(error)
      }
    },

    async create(body: MachineClientRequestBody) {
      this.saving = true
      this.errorMessage = ''
      try {
        return await createMachineClientFromForm(body)
      } catch (error) {
        if (isApiForbidden(error)) {
          this.status = 'forbidden'
        }
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async update(id: number, body: MachineClientRequestBody) {
      this.saving = true
      this.errorMessage = ''
      try {
        this.current = await updateMachineClientFromForm(id, body)
        return this.current
      } catch (error) {
        if (isApiForbidden(error)) {
          this.status = 'forbidden'
        }
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },

    async disable(id: number) {
      this.saving = true
      this.errorMessage = ''
      try {
        await disableMachineClient(id)
        if (this.current?.id === id) {
          this.current = { ...this.current, active: false }
        }
        this.items = this.items.map((item) => (
          item.id === id ? { ...item, active: false } : item
        ))
      } catch (error) {
        if (isApiForbidden(error)) {
          this.status = 'forbidden'
        }
        this.errorMessage = toApiErrorMessage(error)
        throw error
      } finally {
        this.saving = false
      }
    },
  },
})
```

generated client の error shape は将来変わる可能性があります。`Error` instance だけを見ず、Problem JSON の `status` も見ることで、403 を専用 state に分けられます。

## Step 3. tenant selector を app shell に接続する

tenant selector は全画面で参照される context なので、`App.vue` の header に置きます。

### 3-1. TenantSelector component を追加する

#### ファイル: `frontend/src/components/TenantSelector.vue`

```vue
<script setup lang="ts">
import { computed, onMounted } from 'vue'

import { useTenantStore } from '../stores/tenants'

const tenantStore = useTenantStore()

const selectedSlug = computed(() => tenantStore.activeTenant?.slug ?? '')
const disabled = computed(() => (
  tenantStore.status === 'loading' ||
  tenantStore.status === 'empty' ||
  Boolean(tenantStore.switchingSlug)
))

async function onChange(event: Event) {
  const target = event.target as HTMLSelectElement
  await tenantStore.select(target.value)
}

onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
})
</script>

<template>
  <div class="tenant-selector">
    <label class="field-label" for="tenant-selector">Active tenant</label>
    <select
      id="tenant-selector"
      class="field-input"
      :disabled="disabled"
      :value="selectedSlug"
      @change="onChange"
    >
      <option v-if="tenantStore.status === 'loading'" value="">
        Loading tenants
      </option>
      <option v-else-if="tenantStore.status === 'empty'" value="">
        No tenant
      </option>
      <option
        v-for="tenant in tenantStore.items"
        :key="tenant.slug"
        :value="tenant.slug"
      >
        {{ tenant.displayName }} / {{ tenant.slug }}
      </option>
    </select>
    <p v-if="tenantStore.errorMessage" class="error-message">
      {{ tenantStore.errorMessage }}
    </p>
  </div>
</template>

<style scoped>
.tenant-selector {
  display: grid;
  gap: 8px;
  min-width: 240px;
}
</style>
```

`select` は管理画面として十分に分かりやすく、keyboard 操作も自然です。tenant 数が極端に増えるまでは custom combobox を作りません。

### 3-2. App.vue に置く

#### ファイル: `frontend/src/App.vue`

`TenantSelector` を import し、認証済みのときだけ header に表示します。

```ts
import TenantSelector from './components/TenantSelector.vue'
```

template の `identity-card` の近くに置きます。

```vue
<div class="header-tools">
  <TenantSelector v-if="sessionStore.status === 'authenticated'" />

  <div class="identity-card">
    <span class="identity-label">Current identity</span>
    <strong>{{ displayName }}</strong>
    <span class="identity-status">{{ statusLabel }}</span>
  </div>
</div>
```

style は header 内で横並び、mobile では縦積みにします。

```css
.header-tools {
  display: flex;
  align-items: end;
  gap: 16px;
}

@media (max-width: 720px) {
  .header-tools {
    align-items: stretch;
    flex-direction: column;
  }
}
```

### 3-3. logout 時に tenant state を戻す

#### ファイル: `frontend/src/stores/session.ts`

logout 後に tenant state が残ると、次の user で古い tenant が一瞬見えます。`tenants` store に `reset()` action を足し、logout 成功時に呼びます。

#### ファイル: `frontend/src/stores/tenants.ts`

```ts
reset() {
  this.status = 'idle'
  this.items = []
  this.activeTenant = null
  this.defaultTenant = null
  this.errorMessage = ''
  this.switchingSlug = ''
},
```

#### ファイル: `frontend/src/stores/session.ts`

```ts
import { useTenantStore } from './tenants'
```

logout 成功後:

```ts
const tenantStore = useTenantStore()
tenantStore.reset()
```

## Step 4. machine client admin 画面を追加する

machine client admin は global role `machine_client_admin` が必要です。frontend は role の正本ではありません。nav の表示制御は補助にとどめ、実際の許可判定は backend の 403 を UI で扱います。

### 4-1. router を追加する

#### ファイル: `frontend/src/router/index.ts`

view を import します。

```ts
import MachineClientDetailView from '../views/MachineClientDetailView.vue'
import MachineClientFormView from '../views/MachineClientFormView.vue'
import MachineClientsView from '../views/MachineClientsView.vue'
```

routes に追加します。

```ts
{
  path: '/machine-clients',
  name: 'machine-clients',
  component: MachineClientsView,
  meta: { requiresAuth: true },
},
{
  path: '/machine-clients/new',
  name: 'machine-client-new',
  component: MachineClientFormView,
  meta: { requiresAuth: true },
},
{
  path: '/machine-clients/:id',
  name: 'machine-client-detail',
  component: MachineClientDetailView,
  meta: { requiresAuth: true },
},
```

`requiresRole` guard はこの時点では入れません。current browser session の `GET /api/v1/session` は global roles を返していないため、frontend だけで正確に判断できません。role 不足は machine client API の 403 で判定します。

### 4-2. App navigation に link を追加する

#### ファイル: `frontend/src/App.vue`

```vue
<RouterLink to="/machine-clients">Machine Clients</RouterLink>
```

nav link は常に出して構いません。権限不足の user がクリックした場合は、machine client view で「`machine_client_admin` role が必要」と表示します。権限があるかどうかを frontend で推測して隠すより、backend の判定結果をそのまま見せる方が混乱しません。

### 4-3. access denied component を追加する

#### ファイル: `frontend/src/components/AdminAccessDenied.vue`

```vue
<template>
  <section class="panel stack">
    <span class="status-pill danger">Forbidden</span>
    <h2>Machine client admin role required</h2>
    <p>
      この画面を使うには global role <code>machine_client_admin</code> が必要です。
    </p>
  </section>
</template>
```

同じ 403 表示を list / detail / form で使い回します。

### 4-4. machine client list view を追加する

#### ファイル: `frontend/src/views/MachineClientsView.vue`

この view は list と空状態を担当します。

実装方針:

- `onMounted()` で `machineClientStore.loadList()` を呼ぶ
- `status === 'forbidden'` なら `AdminAccessDenied` を出す
- `items.length === 0` なら empty state を出す
- active / inactive を視覚的に分ける
- detail は `/machine-clients/:id` へ遷移する
- create は `/machine-clients/new` へ遷移する

template の骨格:

```vue
<template>
  <AdminAccessDenied v-if="store.status === 'forbidden'" />

  <section v-else class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">M2M</span>
        <h2>Machine Clients</h2>
      </div>
      <div class="action-row">
        <button class="secondary-button" :disabled="store.status === 'loading'" @click="store.loadList()">
          Refresh
        </button>
        <RouterLink class="primary-button link-button" to="/machine-clients/new">
          New
        </RouterLink>
      </div>
    </div>

    <p v-if="store.errorMessage" class="error-message">{{ store.errorMessage }}</p>

    <div v-if="store.items.length > 0" class="admin-table">
      <!-- provider, client id, display name, default tenant, scopes, active, updated at -->
    </div>

    <p v-else-if="store.status === 'ready'">
      Machine client はまだ登録されていません。
    </p>
  </section>
</template>
```

table 表示は native `<table>` でも CSS grid でも構いません。管理 UI では一覧性が重要なので、カードを何重にも重ねず、列が比較しやすい構造にします。

### 4-5. machine client form view を追加する

#### ファイル: `frontend/src/views/MachineClientFormView.vue`

作成 form は `MachineClientRequestBody` に合わせます。

入力項目:

- `provider`
  - default: `zitadel`
- `providerClientId`
  - required
- `displayName`
  - required
- `defaultTenantId`
  - optional
  - tenant store の `items` から select
- `allowedScopes`
  - textarea または input
  - UI では whitespace / comma separated を許可し、submit 前に `string[]` へ正規化
- `active`
  - checkbox
  - default: true

scope parser は view 内 helper で十分です。

```ts
function parseScopes(value: string): string[] {
  return value
    .split(/[\s,]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}
```

submit では `store.create(body)` を呼び、成功したら detail へ遷移します。

```ts
const created = await store.create({
  provider: provider.value || 'zitadel',
  providerClientId: providerClientId.value,
  displayName: displayName.value,
  defaultTenantId: defaultTenantId.value ? Number(defaultTenantId.value) : undefined,
  allowedScopes: parseScopes(allowedScopes.value),
  active: active.value,
})

await router.push({ name: 'machine-client-detail', params: { id: created.id } })
```

form 表示時にも permission を早めに確定させます。`machine_client_admin` が無い user で空の作成 form を見せ続けないように、`onMounted()` で tenant list を読み込んだ後、`store.loadList()` を呼んで 403 を `AdminAccessDenied` に流します。

```ts
onMounted(async () => {
  if (tenantStore.status === 'idle') {
    await tenantStore.load()
  }
  await store.loadList()
})
```

### 4-6. machine client detail view を追加する

#### ファイル: `frontend/src/views/MachineClientDetailView.vue`

detail view は `GET`、`PUT`、`DELETE` を担当します。

実装方針:

- route param `id` を number に変換する
- `onMounted()` で `store.loadOne(id)` を呼ぶ
- `status === 'forbidden'` なら `AdminAccessDenied` を出す
- `store.current` から form 初期値を作る
- save は `store.update(id, body)` を呼ぶ
- disable は確認後に `store.disable(id)` を呼ぶ
- disable 後は detail 上の `active` を false にし、戻る link を表示する

disable button の文言:

```vue
<button
  class="secondary-button danger-button"
  :disabled="store.saving || !store.current?.active"
  type="button"
  @click="disableCurrent"
>
  Disable
</button>
```

`providerClientId` は provider 側の識別子です。誤更新の影響が大きいため、detail 画面では保存前に変更差分を見える形にします。最小実装では、保存 button 近くに current value を表示するだけで構いません。

default tenant の select は、現在の user が見える tenant list を基本にします。ただし既存 machine client の `defaultTenant` が tenant list に含まれない場合でも、detail 表示で既存値が消えないように current value を option に残します。

```ts
const tenantOptions = computed<TenantBody[]>(() => {
  const currentTenant = store.current?.defaultTenant
  if (!currentTenant || tenantStore.items.some((tenant) => tenant.id === currentTenant.id)) {
    return tenantStore.items
  }

  return [currentTenant, ...tenantStore.items]
})
```

## Step 5. integrations UX を整理する

既存の `IntegrationsView.vue` は connect / verify / revoke を持っています。P1 では active tenant と callback / error の見え方を補強します。

### 5-1. tenant store を使う

#### ファイル: `frontend/src/views/IntegrationsView.vue`

`useTenantStore()` を import します。

```ts
import { watch } from 'vue'
import { useTenantStore } from '../stores/tenants'
```

setup 内:

```ts
const tenantStore = useTenantStore()
```

`onMounted()` で tenant を先に読みます。

```ts
onMounted(async () => {
  if (tenantStore.status === 'idle' || tenantStore.status === 'loading') {
    await tenantStore.load()
  }
  await loadIntegrations()
  clearCallbackQuery()
})
```

ここで tenant load を待たずに `loadIntegrations()` を先に呼ぶと、header の `TenantSelector` が読み込み中の一瞬だけ active tenant が空になり、`Integration を操作するには active tenant が必要です。` が一時表示されます。integrations は active tenant 依存なので、必ず tenant load の後に読みます。

active tenant が切り替わったら integrations を再読込します。

```ts
watch(
  () => tenantStore.activeTenant?.slug,
  async (slug, previous) => {
    if (slug && slug !== previous) {
      verifyResult.value = null
      await loadIntegrations()
    }
  },
)
```

### 5-2. active tenant required を UI にする

`GET /api/v1/integrations` は active tenant がない場合に `409` を返します。単に error message を出すだけでなく、tenant selector を使う理由が分かる表示にします。

template の header 近くに active tenant を出します。

```vue
<p v-if="tenantStore.activeTenant">
  Active tenant: <strong>{{ tenantStore.activeTenant.displayName }}</strong>
</p>
<p v-else class="error-message">
  Integration を操作するには active tenant が必要です。
</p>
```

connect / verify / revoke button は active tenant がない場合に disable します。

```vue
:disabled="!tenantStore.activeTenant || busyResource === item.resourceServer"
```

### 5-3. callback result を明示する

既存の `callbackMessage` は `connected` / `error` を見ています。P1 では error code を文言に反映します。

```ts
const callbackMessage = computed(() => {
  if (route.query.connected) {
    return `${route.query.connected} integration connected.`
  }
  if (route.query.error === 'missing_session') {
    return 'Integration callback failed because the browser session was missing.'
  }
  if (route.query.error) {
    return `Integration callback failed: ${route.query.error}`
  }
  return ''
})
```

`clearCallbackQuery()` は message が見える前に消さないように、表示後に消すか、`replace` する前に local ref へ退避します。最小実装では `const callbackNotice = ref(callbackMessage.value)` を持ち、`onMounted()` の先頭で退避してから query を消します。

### 5-4. verify result と last error を分ける

`lastErrorCode` は過去の状態、`verifyResult` は今回の確認結果です。UI 上では同じ場所に混ぜず、一覧には `Last error`、下部には `Access Check` を出します。

既存 view の方針は維持し、次だけ追加します。

- `item.revokedAt` があれば `Revoked` を表示する
- `verifyResult.connected === false` なら warning message を出す
- `accessExpiresAt` が過去なら `Expired` 表示にする

## Step 6. docs link hardening を入れる

Home 画面の `Open Docs` link は、docs auth が有効な環境で raw error を見せやすい場所です。button component にして access check を挟みます。

### 6-1. DocsLink component を追加する

#### ファイル: `frontend/src/components/DocsLink.vue`

```vue
<script setup lang="ts">
import { ref } from 'vue'

import { checkDocsAccess } from '../api/docs'

const checking = ref(false)
const errorMessage = ref('')

async function openDocs() {
  checking.value = true
  errorMessage.value = ''

  try {
    await checkDocsAccess()
    window.open('/docs', '_blank', 'noreferrer')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Docs are unavailable.'
  } finally {
    checking.value = false
  }
}
</script>

<template>
  <div class="docs-link-wrapper">
    <button class="secondary-button" :disabled="checking" type="button" @click="openDocs">
      {{ checking ? 'Checking...' : 'Open Docs' }}
    </button>
    <p v-if="errorMessage" class="error-message">
      {{ errorMessage }}
    </p>
  </div>
</template>

<style scoped>
.docs-link-wrapper {
  display: inline-grid;
  gap: 8px;
}
</style>
```

### 6-2. HomeView の link を置き換える

#### ファイル: `frontend/src/views/HomeView.vue`

```ts
import DocsLink from '../components/DocsLink.vue'
```

既存の `<a class="secondary-button docs-link" href="/docs" ...>` を次に置き換えます。

```vue
<DocsLink />
```

`docs-link` 用の scoped style が不要になれば削除します。

## Step 7. 共通 style を追加する

管理 UI は、一覧、form、metadata、status の見え方が揃っていると使いやすくなります。既存の `panel` / `stack` / `field` / `field-input` / `action-row` を活かし、足りない utility だけ追加します。

#### ファイル: `frontend/src/style.css`

```css
.section-header {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 16px;
}

.link-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  text-decoration: none;
}

.admin-table {
  overflow-x: auto;
}

.admin-table table {
  width: 100%;
  border-collapse: collapse;
}

.admin-table th,
.admin-table td {
  padding: 12px;
  border-bottom: 1px solid var(--border);
  text-align: left;
  vertical-align: top;
}

.admin-table th {
  color: var(--muted);
  font-size: 0.78rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.status-pill.danger {
  color: var(--danger);
  background: rgba(174, 45, 42, 0.1);
}

.danger-button {
  color: var(--danger);
}

@media (max-width: 720px) {
  .section-header {
    align-items: stretch;
    flex-direction: column;
  }
}
```

`IntegrationsView.vue` に同名 class が scoped style として既にある場合は、重複を整理します。共通化する class は `style.css` に残し、view 固有の見た目だけ scoped style に残します。

## Step 8. local 確認用データを準備する

UI を確認するには、ログイン中 user に tenant membership と global role `machine_client_admin` が必要です。これは開発専用 seed として扱い、migration には入れません。

### 8-1. local password login 用の seed

まず DB と seed を用意します。

```bash
make up
make db-up
make seed-demo-user
```

demo user に P1 確認用 role / tenant を付与します。

```bash
docker-compose exec -T postgres psql -U haohao -d haohao <<'SQL'
INSERT INTO roles (code)
VALUES ('machine_client_admin'), ('docs_reader'), ('todo_user')
ON CONFLICT (code) DO NOTHING;

INSERT INTO tenants (slug, display_name)
VALUES
  ('acme', 'Acme'),
  ('beta', 'Beta')
ON CONFLICT (slug) DO UPDATE
SET display_name = EXCLUDED.display_name,
    active = true,
    updated_at = now();

UPDATE users
SET default_tenant_id = (SELECT id FROM tenants WHERE slug = 'acme'),
    deactivated_at = NULL
WHERE email = 'demo@example.com';

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code IN ('machine_client_admin', 'docs_reader')
WHERE u.email = 'demo@example.com'
ON CONFLICT DO NOTHING;

INSERT INTO tenant_memberships (user_id, tenant_id, role_id, source)
SELECT u.id, t.id, r.id, 'local_override'
FROM users u
JOIN tenants t ON t.slug IN ('acme', 'beta')
JOIN roles r ON r.code IN ('todo_user', 'docs_reader')
WHERE u.email = 'demo@example.com'
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET active = true,
    updated_at = now();
SQL
```

`docker compose` を使っている環境では、上の `docker-compose` を `docker compose` に置き換えます。

### 8-2. Zitadel login で確認する場合の seed

Zitadel login で UI smoke をする場合、Step 8-1 の `demo@example.com` seed だけでは足りません。まず一度 Zitadel login を完了し、local DB に user が作られている状態にします。その後、ログイン中 user の email を確認します。

```bash
docker-compose exec -T postgres psql -U haohao -d haohao <<'SQL'
SELECT email, display_name, deactivated_at
FROM users
ORDER BY email;
SQL
```

対象 email を差し替えて、P1 確認用の global role と `acme` / `beta` membership を付与します。

```bash
TARGET_EMAIL='zitadel-admin@zitadel.localhost'

docker-compose exec -T postgres psql -U haohao -d haohao \
  -v target_email="$TARGET_EMAIL" <<'SQL'
INSERT INTO roles (code)
VALUES ('machine_client_admin'), ('docs_reader'), ('todo_user')
ON CONFLICT (code) DO NOTHING;

INSERT INTO tenants (slug, display_name)
VALUES
  ('acme', 'Acme'),
  ('beta', 'Beta')
ON CONFLICT (slug) DO UPDATE
SET display_name = EXCLUDED.display_name,
    active = true,
    updated_at = now();

UPDATE users
SET default_tenant_id = (SELECT id FROM tenants WHERE slug = 'acme'),
    deactivated_at = NULL
WHERE email = :'target_email';

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code IN ('machine_client_admin', 'docs_reader')
WHERE u.email = :'target_email'
ON CONFLICT DO NOTHING;

INSERT INTO tenant_memberships (user_id, tenant_id, role_id, source)
SELECT u.id, t.id, r.id, 'local_override'
FROM users u
JOIN tenants t ON t.slug IN ('acme', 'beta')
JOIN roles r ON r.code IN ('todo_user', 'docs_reader')
WHERE u.email = :'target_email'
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET active = true,
    updated_at = now();
SQL
```

Zitadel 経由の本来の確認では、local DB への手動付与ではなく、Zitadel 側の claim / group または SCIM provisioning から同じ role / membership が同期されていることを確認します。

### 8-3. seed 後の状態を確認する

browser で使う user と seed 対象の email が一致していることを確認します。

```bash
TARGET_EMAIL='demo@example.com'

docker-compose exec -T postgres psql -U haohao -d haohao \
  -v target_email="$TARGET_EMAIL" <<'SQL'
SELECT u.email, COALESCE(string_agg(DISTINCT r.code, ', ' ORDER BY r.code), '(none)') AS global_roles
FROM users u
LEFT JOIN user_roles ur ON ur.user_id = u.id
LEFT JOIN roles r ON r.id = ur.role_id
WHERE u.email = :'target_email'
GROUP BY u.email;

SELECT u.email, t.slug, COALESCE(string_agg(r.code, ', ' ORDER BY r.code), '(none)') AS tenant_roles
FROM users u
LEFT JOIN tenant_memberships tm ON tm.user_id = u.id AND tm.active = true
LEFT JOIN tenants t ON t.id = tm.tenant_id
LEFT JOIN roles r ON r.id = tm.role_id
WHERE u.email = :'target_email'
GROUP BY u.email, t.slug
ORDER BY t.slug;
SQL
```

期待値:

- `global_roles` に `machine_client_admin` が含まれる
- `tenant_roles` に `acme` と `beta` が出る
- browser の `GET /api/v1/session` に出る `user.email` と `TARGET_EMAIL` が一致する

### 8-4. dev server で browser smoke を始める

local password login で UI smoke をする場合は、backend を local auth で起動します。

```bash
bash -lc 'set -a; source .env; export AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=true; set +a; go run ./backend/cmd/main'
```

別 terminal で frontend を起動します。

```bash
npm --prefix frontend run dev
```

browser で `http://127.0.0.1:5173/login` を開き、`demo@example.com / changeme123` で login します。

Zitadel login で確認する場合も frontend dev server は `http://127.0.0.1:5173` です。ただし確認対象 user は `demo@example.com` ではなく、Zitadel から戻ってきた local user の email です。

## Step 9. 確認すること

### 9-1. frontend build

```bash
npm --prefix frontend run build
```

期待値:

- `vue-tsc -b` が通る
- Vite build が通る
- `backend/web/dist/` が生成される

### 9-2. backend test

P1 は frontend 中心ですが、OpenAPI / generated SDK に依存するため backend test も通します。

```bash
go test ./backend/...
```

期待値:

- 既存 backend test がすべて通る

### 9-3. generated SDK drift

この P1 では OpenAPI contract を変えない前提です。SDK 再生成で想定外差分が出ないことを確認します。

```bash
go run ./backend/cmd/openapi > /tmp/haohao-openapi.yaml
/usr/bin/diff -u openapi/openapi.yaml /tmp/haohao-openapi.yaml
npm --prefix frontend run openapi-ts
git diff -- frontend/src/api/generated openapi/openapi.yaml
```

期待値:

- OpenAPI diff がない
- generated SDK に想定外差分がない

もし差分が出る場合は、P1 UI とは別に API contract drift として確認します。

macOS / zsh で `diff` が `delta` などに alias されている場合、`diff -u` が `unexpected argument '-u'` で失敗します。その場合は上のように `/usr/bin/diff -u` を使うか、次で確認します。

```bash
git diff --no-index -- openapi/openapi.yaml /tmp/haohao-openapi.yaml
```

### 9-4. browser smoke

local login と Zitadel login を行き来している場合は、先に logout するか private window で始めます。古い session が残っていると、DB で `demo@example.com` を変更しても、実際の browser は Zitadel user として API を呼び続けます。

browser で次を確認します。

- login 後、header に tenant selector が出る
- `Acme` / `Beta` を切り替えられる
- tenant 切り替え後、Integrations が再読込される
- `/machine-clients` を開ける
- machine client を作成できる
- 作成後に detail へ遷移する
- detail で display name / scopes / active を更新できる
- `Disable` で inactive になる
- `machine_client_admin` role を外した user では `/machine-clients` が 403 用 UI を表示する
- `Open Docs` は許可されていれば new tab で開く
- docs access が拒否された場合は、現在画面に error message が出る

role 不足 UI を確認する場合は、ログイン中の user から role を外します。

まず現在の session user を確認します。Home の session JSON、または browser devtools の `GET /api/v1/session` response に出る `user.email` が対象です。local login なら通常は `demo@example.com`、Zitadel login なら `zitadel-admin@zitadel.localhost` など実際にログインしている email を使います。

必要なら DB 側の global role も確認します。

```bash
docker-compose exec -T postgres psql -U haohao -d haohao <<'SQL'
SELECT u.email, COALESCE(string_agg(r.code, ', ' ORDER BY r.code), '(none)') AS global_roles
FROM users u
LEFT JOIN user_roles ur ON ur.user_id = u.id
LEFT JOIN roles r ON r.id = ur.role_id
GROUP BY u.email
ORDER BY u.email;
SQL
```

対象 email を差し替えて `machine_client_admin` を外します。

```bash
TARGET_EMAIL='demo@example.com' # Zitadel login なら GET /api/v1/session の user.email に差し替える

docker-compose exec -T postgres psql -U haohao -d haohao \
  -v target_email="$TARGET_EMAIL" <<'SQL'
DELETE FROM user_roles ur
USING users u, roles r
WHERE ur.user_id = u.id
  AND ur.role_id = r.id
  AND u.email = :'target_email'
  AND r.code = 'machine_client_admin';
SQL
```

削除後、`/machine-clients` を reload するか `Refresh` を押します。既に読み込み済みの一覧は API を再取得するまで画面に残ります。

Zitadel login の場合、logout / login し直すと Zitadel 側の claim / group から local role が再同期され、`machine_client_admin` が戻ることがあります。403 UI を継続確認したい場合は、local DB だけでなく Zitadel 側の role claim も外します。

確認後、必要なら role を戻します。

```bash
TARGET_EMAIL='demo@example.com' # Zitadel login なら GET /api/v1/session の user.email に差し替える

docker-compose exec -T postgres psql -U haohao -d haohao \
  -v target_email="$TARGET_EMAIL" <<'SQL'
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code = 'machine_client_admin'
WHERE u.email = :'target_email'
ON CONFLICT DO NOTHING;
SQL
```

### 9-5. single binary smoke

```bash
make binary
APP_BASE_URL=http://127.0.0.1:8080 \
FRONTEND_BASE_URL=http://127.0.0.1:8080 \
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:8080/login \
./bin/haohao
```

別 terminal:

```bash
make smoke-operability
```

browser では `http://127.0.0.1:8080/` を開きます。embedded frontend でも次が動くことを確認します。

- `/`
- `/integrations`
- `/machine-clients`
- `/machine-clients/new`
- `/machine-clients/:id`
- `/docs`

`make smoke-operability` は server を起動しません。既に `:8080` で動いている process に対して smoke を実行します。`go run ./backend/cmd/main` など build tag なしの dev backend が `FRONTEND_BASE_URL=http://127.0.0.1:5173` のまま動いていると、callback redirect が Vite dev server になり smoke は失敗します。

その場合は現在の process を止めてから、上の `./bin/haohao` を起動し直します。

```bash
lsof -nP -iTCP:8080 -sTCP:LISTEN
kill <PID>
```

### 9-6. Docker smoke

```bash
docker build -t haohao:dev -f docker/Dockerfile .
```

Docker runtime の smoke は `RUNBOOK_OPERABILITY.md` の Docker deploy 手順に沿って確認します。live dependency が必要なため、CI へ入れるのは local / staging で安定してからにします。

## Troubleshooting

### tenant selector が空になる

`GET /api/v1/tenants` が空を返しています。ログイン中 user に tenant membership が入っているか確認します。

```bash
TARGET_EMAIL='demo@example.com'

docker-compose exec -T postgres psql -U haohao -d haohao \
  -v target_email="$TARGET_EMAIL" <<'SQL'
SELECT u.email, t.slug, r.code, tm.source, tm.active
FROM tenant_memberships tm
JOIN users u ON u.id = tm.user_id
JOIN tenants t ON t.id = tm.tenant_id
JOIN roles r ON r.id = tm.role_id
WHERE u.email = :'target_email'
ORDER BY t.slug, r.code;
SQL
```

Zitadel login で確認している場合は、`TARGET_EMAIL` を `GET /api/v1/session` に出る email に差し替えます。

### make smoke-operability が 5173 redirect で失敗する

`make smoke-operability` は `BASE_URL` の既定値 `http://127.0.0.1:8080` に対して smoke を実行します。server は起動しません。

`callback redirected to Vite dev server: http://127.0.0.1:5173/...` のように失敗する場合、`:8080` で build tag なしの dev backend が動いていて、`FRONTEND_BASE_URL=http://127.0.0.1:5173` を使っています。single binary smoke では、現在の `:8080` process を止めてから次で起動し直します。

```bash
make binary
APP_BASE_URL=http://127.0.0.1:8080 \
FRONTEND_BASE_URL=http://127.0.0.1:8080 \
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://127.0.0.1:8080/login \
./bin/haohao
```

別 terminal で実行します。

```bash
make smoke-operability
```

### tenant selector に Acme しか出ない

tenant selector は、ログイン中 user の `tenant_memberships` だけを表示します。Step 8 の seed は `demo@example.com` 用です。Zitadel login で `zitadel-admin@zitadel.localhost` など別 user として入っている場合、その user に `beta` membership が無ければ Acme しか出ません。

まず user ごとの membership を確認します。

```bash
docker-compose exec -T postgres psql -U haohao -d haohao <<'SQL'
SELECT u.email, t.slug, r.code, tm.source, tm.active
FROM users u
LEFT JOIN tenant_memberships tm ON tm.user_id = u.id
LEFT JOIN tenants t ON t.id = tm.tenant_id
LEFT JOIN roles r ON r.id = tm.role_id
ORDER BY u.email, t.slug, r.code, tm.source;
SQL
```

Zitadel login 中の user に `beta` を追加して UI を確認するだけなら、対象 email を差し替えて次を実行します。

```bash
TARGET_EMAIL='zitadel-admin@zitadel.localhost'

docker-compose exec -T postgres psql -U haohao -d haohao \
  -v target_email="$TARGET_EMAIL" <<'SQL'
INSERT INTO roles (code)
VALUES ('docs_reader'), ('todo_user')
ON CONFLICT (code) DO NOTHING;

INSERT INTO tenants (slug, display_name)
VALUES ('beta', 'Beta')
ON CONFLICT (slug) DO UPDATE
SET display_name = EXCLUDED.display_name,
    active = true,
    updated_at = now();

INSERT INTO tenant_memberships (user_id, tenant_id, role_id, source)
SELECT u.id, t.id, r.id, 'local_override'
FROM users u
JOIN tenants t ON t.slug = 'beta'
JOIN roles r ON r.code IN ('todo_user', 'docs_reader')
WHERE u.email = :'target_email'
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET active = true,
    updated_at = now();
SQL
```

本来の Zitadel 経由の確認では、Zitadel 側の `groups` claim または SCIM groups に次のような値を入れて、login / provisioning で同期させます。

```text
tenant:beta:todo_user
tenant:beta:docs_reader
```

### integrations が 409 になる

active tenant が session に入っていません。tenant selector で tenant を選択し直してください。それでも直らない場合は、`POST /api/v1/session/tenant` が CSRF error になっていないか browser devtools の Network で確認します。

### machine client 画面が 403 になる

ログイン中の user に global role `machine_client_admin` がありません。tenant membership の `tenant:<slug>:...` ではなく、`user_roles` に入る global role が必要です。

まず user ごとの global role を確認します。

```bash
docker-compose exec -T postgres psql -U haohao -d haohao <<'SQL'
SELECT u.email, COALESCE(string_agg(r.code, ', ' ORDER BY r.code), '(none)') AS global_roles
FROM users u
LEFT JOIN user_roles ur ON ur.user_id = u.id
LEFT JOIN roles r ON r.id = ur.role_id
GROUP BY u.email
ORDER BY u.email;
SQL
```

Zitadel login 中の user に local 確認用で `machine_client_admin` を付与する場合は、対象 email を差し替えて次を実行します。

```bash
TARGET_EMAIL='zitadel-admin@zitadel.localhost'

docker-compose exec -T postgres psql -U haohao -d haohao \
  -v target_email="$TARGET_EMAIL" <<'SQL'
INSERT INTO roles (code)
VALUES ('machine_client_admin')
ON CONFLICT (code) DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code = 'machine_client_admin'
WHERE u.email = :'target_email'
ON CONFLICT DO NOTHING;
SQL
```

Zitadel 経由の本来の確認では、Zitadel 側の user claim / group に `machine_client_admin` が入り、local role sync されていることを確認します。

### role を外しても machine client 一覧が表示される

削除した user とブラウザでログイン中の user が一致しているか確認します。`demo@example.com` から role を外しても、ブラウザが `zitadel-admin@zitadel.localhost` でログインしている場合は一覧が表示されます。

現在の実装では、machine client API は request ごとに DB の `user_roles` を読みます。role を外した後は `/machine-clients` を reload するか `Refresh` を押して再取得します。既に表示済みの table は、再取得するまで消えません。

Zitadel login では、再ログイン時に provider claim / group から global role が local DB に再同期されることがあります。local DB から削除しても再ログイン後に戻る場合は、Zitadel 側の role claim も確認してください。

### machine client 作成が 400 になる

`providerClientId` と `displayName` は空にできません。`defaultTenantId` は実在する tenant の ID だけを指定します。`allowedScopes` は `string[]` なので、submit 前に whitespace / comma separated 文字列から配列へ変換してください。

### docs link が 403 になる

docs auth が有効な環境では `docs_reader` role が必要です。P1 の目的は、この 403 を raw JSON として見せず、frontend 上の自然な error message にすることです。

### build は通るが embedded binary で route が 404 になる

`make binary` の前に frontend build が実行されているか確認します。`TUTORIAL_SINGLE_BINARY.md` の前提どおり、embedded binary は `backend/web/dist/` を build 時に取り込みます。

## 完了チェックリスト

- [ ] `frontend/src/api/tenants.ts` を追加した
- [ ] `frontend/src/api/machine-clients.ts` を追加した
- [ ] `frontend/src/api/docs.ts` を追加した
- [ ] `frontend/src/api/client.ts` に Problem JSON 用の status helper を追加した
- [ ] `frontend/src/stores/tenants.ts` を追加した
- [ ] `frontend/src/stores/machine-clients.ts` を追加した
- [ ] `frontend/src/components/TenantSelector.vue` を追加した
- [ ] `frontend/src/components/AdminAccessDenied.vue` を追加した
- [ ] `frontend/src/components/DocsLink.vue` を追加した
- [ ] `frontend/src/App.vue` に tenant selector と machine client nav を追加した
- [ ] `frontend/src/router/index.ts` に machine client routes を追加した
- [ ] `frontend/src/views/MachineClientsView.vue` を追加した
- [ ] `frontend/src/views/MachineClientFormView.vue` を追加した
- [ ] `frontend/src/views/MachineClientDetailView.vue` を追加した
- [ ] `frontend/src/views/IntegrationsView.vue` が tenant 切り替えに追従する
- [ ] `frontend/src/views/HomeView.vue` の docs link を `DocsLink` に置き換えた
- [ ] `frontend/src/style.css` に管理 UI の共通 style を追加した
- [ ] seed 後の DB 確認で `machine_client_admin` と `acme` / `beta` membership が確認できる
- [ ] browser smoke のログイン user と seed 対象 email が一致している
- [ ] `npm --prefix frontend run build` が通る
- [ ] `go test ./backend/...` が通る
- [ ] `make binary` が通る
- [ ] single binary を `:8080` で起動した状態で `make smoke-operability` が通る
- [ ] browser smoke で tenant selector / integrations / machine client admin / docs link を確認した
