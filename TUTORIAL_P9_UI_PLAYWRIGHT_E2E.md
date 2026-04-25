# P9 UI Playwright E2E 実装チュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **P7 UI Playwright E2E チュートリアル作成 / 実装** を、現在の HaoHao に実装できる順番へ分解したチュートリアルです。

P7 までで、Cookie session、CSRF、tenant 切り替え、Customer Signals、file upload、notifications、Tenant Admin settings、tenant data export、single binary SPA fallback までが入りました。P8 では OpenAPI を `full` / `browser` / `external` に分け、frontend generated SDK は `openapi/browser.yaml` 由来になりました。

この P9 では、browser の実挙動を Playwright で確認します。shell smoke は API と運用面に強い一方で、フォーム、route、generated SDK 呼び出し、Cookie / CSRF、SPA fallback、role 不足 UI、確認 dialog の回帰を直接見ません。P9 の目的は、P0-P8 で作った browser app の主要な縦線を、local single binary に対して再現できる E2E として固定することです。

この文書は `TUTORIAL.md` / `TUTORIAL_SINGLE_BINARY.md` / `TUTORIAL_P7_WEB_SERVICE_COMMON.md` / `TUTORIAL_P8_OPENAPI_SPLIT.md` と同じように、対象ファイル、主要コード方針、確認コマンド、完了条件まで追える形にしています。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- `frontend/package.json` に Vue / Pinia / Vue Router / `@hey-api/openapi-ts` がある
- `frontend/src/router/index.ts` に `/login`、`/customer-signals`、`/tenant-admin`、`/notifications` などの browser route がある
- `frontend/src/api/client.ts` が Cookie credentials、CSRF bootstrap、`Idempotency-Key` を扱っている
- `scripts/seed-demo-user.sql` が `demo@example.com` / `changeme123` と `acme` / `beta` tenant を作れる
- `Makefile` に `up`、`db-up`、`seed-demo-user`、`gen`、`frontend-build`、`binary`、既存 smoke target がある
- `make binary` で frontend を embed した `bin/haohao` が作れる
- `openapi/browser.yaml` と frontend generated SDK は P8 の browser surface 由来になっている
- CI では PostgreSQL service、Go、Node、frontend build、embedded binary build、generated drift check が動いている

この P9 では、backend API contract や DB schema は原則として増やしません。追加するのは Playwright の実行基盤、E2E seed、安定 selector、browser spec、Makefile / CI の実行入口です。

## 完成条件

このチュートリアルの完了条件は次です。

- `frontend/package.json` に Playwright dependency と `e2e` script がある
- repository root に `playwright.config.ts` がある
- repository root に `e2e/` directory がある
- E2E は local single binary `bin/haohao` に対して走る
- E2E 用 seed が `demo@example.com` と role 不足確認用 user を用意する
- browser journey で login、tenant 選択、Customer Signals 作成、file upload、notifications 表示、Tenant Admin settings 更新、tenant data export request を確認する
- boundary spec で SPA fallback と role 不足 UI を確認する
- `Makefile` に `e2e` target がある
- CI が Chromium Playwright E2E を少なくとも 1 回実行する
- Playwright report / trace / screenshot が失敗時に確認できる
- shell smoke は残り、Playwright は smoke を置き換えない
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary`、`make e2e` が通る

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | 現状確認 | router、seed、binary、CI、P8 browser SDK の位置を確認する |
| Step 2 | E2E 境界 | 何を Playwright で見るか、何を shell smoke に残すかを決める |
| Step 3 | `frontend/package.json` | Playwright dependency と npm scripts を追加する |
| Step 4 | `playwright.config.ts` | root config、base URL、report、Chromium project を固定する |
| Step 5 | `scripts/*`, seed SQL | single binary 起動、migration、seed、cleanup を shell に分離する |
| Step 6 | frontend selectors | 画面の意味を変えずに E2E 用の安定 selector を足す |
| Step 7 | `e2e/fixtures/*` | login、tenant 選択、test data 作成を helper 化する |
| Step 8 | `e2e/browser-journey.spec.ts` | primary browser flow を 1 本通す |
| Step 9 | `e2e/access-and-fallback.spec.ts` | role 不足 UI と SPA fallback を確認する |
| Step 10 | `Makefile`, CI | `make e2e` と CI の Chromium 実行を追加する |
| Step 11 | verification | build、unit、binary、E2E、drift check を実行する |

## 先に決める方針

### Playwright は browser behavior に集中する

P9 で Playwright に持たせる責務は次です。

- user が見て操作する route
- form 入力と submit
- Cookie session と CSRF を含む browser fetch
- frontend generated SDK 経由の API 呼び出し
- file input を使った upload
- tenant selector と active tenant の反映
- role 不足時の UI
- single binary の SPA fallback

次は shell smoke / Go test に残します。

- API response body の細かい schema 確認
- metrics の series 名確認
- OpenAPI surface boundary
- DB migration / sqlc / generated drift
- backup / restore smoke
- external bearer / M2M / SCIM の non-browser API

### 実行対象は local single binary にする

Playwright は Vite dev server ではなく、`make binary` で作った `bin/haohao` に対して実行します。

理由は次です。

- single binary の `/`, `/login`, `/customer-signals` 直接アクセスを確認できる
- embedded frontend と backend API が同一 origin になり、Cookie / CSRF の本番寄り挙動を見られる
- `/assets/*` の 404 と SPA fallback の境界を確認できる
- CI と release artifact に近い形で browser flow を通せる

frontend 開発中の高速確認として Vite dev server に対して Playwright を当てる拡張は後で追加できます。P9 初期版では、回帰検知の価値が高い single binary を正にします。

### E2E は deterministic seed を使う

Playwright の login は local password login を使います。Zitadel OIDC の本物の redirect flow は、identity provider の availability や test tenant の状態に依存するため、P9 初期版の必須 E2E には入れません。

E2E 用 user は次に分けます。

| user | 目的 |
| --- | --- |
| `demo@example.com` | full browser journey。tenant admin、customer_signal_user、todo_user を持つ |
| `limited@example.com` | role 不足 UI。tenant_admin と customer_signal_user を持たない |

`demo@example.com` は既存の `scripts/seed-demo-user.sql` を使います。`limited@example.com` は P9 で追加する `scripts/seed-e2e-users.sql` に置きます。

### selector は visible text 優先、曖昧な箇所だけ `data-testid`

Playwright はできるだけ `getByRole`、`getByLabel`、`getByText` を使います。user が実際に見ている UI と test がずれにくいためです。

ただし、Tenant Admin detail のように `Email`、`Role`、`Save` が複数ある画面では、テストの安定性を上げるために `data-testid` を足します。`data-testid` は画面に表示されない属性なので、UI copy の都合で無理な文言を入れません。

### E2E data は一意な名前にする

Customer Signal title、upload file name、invitation email などは test run ごとに suffix を付けます。

```ts
const runId = `p9-${Date.now()}-${testInfo.workerIndex}`
```

DB を毎回 truncate しなくても衝突しないようにします。CI で clean database を使う場合でも、この作法を守ります。

## Step 1. 現状の browser surface と実行経路を確認する

まず、現在の route、login、seed、binary build を確認します。

```bash
sed -n '1,150p' frontend/src/router/index.ts
sed -n '1,140p' frontend/src/views/LoginView.vue
sed -n '1,120p' frontend/src/api/client.ts
sed -n '1,120p' scripts/seed-demo-user.sql
sed -n '1,90p' Makefile
sed -n '1,120p' .github/workflows/ci.yml
```

確認する点は次です。

- `/login` は local password login を持つ
- authenticated route は router guard で `/login` に戻る
- frontend API client は `credentials: 'include'` で動く
- mutating request は CSRF token と `Idempotency-Key` を付ける
- `demo@example.com` は `acme` と `beta` tenant membership を持つ
- `demo@example.com` は `tenant_admin` を持つ
- `make binary` は frontend build 後に embedded binary を作る
- CI はまだ Playwright browser を install / run していない

この Step ではコードを変更しません。P9 の E2E がどこを通るべきかを把握するだけです。

## Step 2. E2E 対象を固定する

P9 初期版で見る browser flow は次にします。

| 領域 | Playwright で確認すること | 既存 smoke との関係 |
| --- | --- | --- |
| Login | `demo@example.com` で login し、identity が authenticated になる | API login smoke の browser 版 |
| Tenant | `acme` / `beta` を selector で切り替えられる | tenant API smoke の browser 版 |
| Customer Signals | form から signal を作り、detail を開ける | P6 / P7 smoke の UI 版 |
| File upload | detail 画面で `type=file` から attachment を upload できる | multipart / CSRF の browser 版 |
| Notifications | invitation 作成後、notification center に item が出る | notification API smoke の UI 版 |
| Tenant settings | file quota / browser rate limit / notification setting を保存できる | tenant settings API smoke の UI 版 |
| Tenant export | data export request を作り、list に出る | data export API smoke の UI 版 |
| Role boundary | role 不足 user で `AdminAccessDenied` が出る | authorization API の UI 版 |
| SPA fallback | direct route access が embedded SPA として動く | `TUTORIAL_SINGLE_BINARY.md` の browser 版 |

P9 初期版で見ないものは次です。

- external bearer / M2M / SCIM
- Zitadel 本物 OIDC redirect
- visual regression screenshot の pixel 比較
- mobile viewport の網羅
- cross-browser matrix
- long-running outbox retry / dead-letter

最初から範囲を広げすぎると E2E が不安定になります。P9 は Chromium 1 project、主要 happy path、role 不足、fallback に絞ります。

## Step 3. Playwright dependency と npm scripts を追加する

Playwright は frontend の dev dependency として追加します。repository root に package がないため、既存の Node 管理場所である `frontend/package.json` に置きます。

```bash
npm --prefix frontend install -D @playwright/test
```

#### ファイル: `frontend/package.json`

`scripts` に次を追加します。

```json
{
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc -b && vite build",
    "preview": "vite preview",
    "openapi-ts": "openapi-ts",
    "e2e": "NODE_PATH=./node_modules playwright test -c ../playwright.config.ts",
    "e2e:ui": "NODE_PATH=./node_modules playwright test -c ../playwright.config.ts --ui",
    "e2e:install": "playwright install chromium"
  }
}
```

`playwright.config.ts` と `e2e/` は repository root に置く一方で、Playwright package は `frontend/node_modules` に入るため、`NODE_PATH=./node_modules` を script に付けて root 側の config / spec から `@playwright/test` を解決できるようにします。

CI では browser dependency も必要になるため、後続 Step で次を実行します。

```bash
npm --prefix frontend exec -- playwright install --with-deps chromium
```

local macOS では次だけで十分な場合があります。

```bash
npm --prefix frontend run e2e:install
```

## Step 4. root `playwright.config.ts` を追加する

Playwright config は repository root に置きます。E2E spec も root の `e2e/` に置くためです。

#### ファイル: `playwright.config.ts`

```ts
import { defineConfig, devices } from '@playwright/test'

const baseURL = process.env.E2E_BASE_URL ?? 'http://127.0.0.1:18080'

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  expect: {
    timeout: 10_000,
  },
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  outputDir: 'test-results',
  reporter: process.env.CI
    ? [
        ['list'],
        ['html', { outputFolder: 'playwright-report', open: 'never' }],
      ]
    : [['list']],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
```

`webServer` はここには入れません。P9 では server 起動、migration、seed、cleanup を shell script に分離します。Playwright config は browser test の設定だけを持ちます。

Playwright の失敗時 artifact は repository に入れないため、root `.gitignore` に次も追加します。

```gitignore
playwright-report
test-results
```

## Step 5. E2E seed と single binary runner を追加する

Playwright spec に DB setup や process 起動を埋め込むと、test code が browser 操作以外の責務を持ち始めます。P9 では shell script に分離します。

追加するファイルは次です。

- `scripts/seed-e2e-users.sql`: role 不足確認用 user を作る
- `scripts/e2e-single-binary.sh`: migration、seed、binary 起動、Playwright 実行、cleanup を行う

### 5-1. E2E 用 limited user seed を追加する

#### ファイル: `scripts/seed-e2e-users.sql`

```sql
-- Development-only E2E seed. Do not run this in production.
INSERT INTO users (email, display_name, password_hash)
VALUES (
    'limited@example.com',
    'Limited User',
    crypt('changeme123', gen_salt('bf'))
)
ON CONFLICT (email) DO UPDATE
SET
    display_name = EXCLUDED.display_name,
    password_hash = EXCLUDED.password_hash,
    deactivated_at = NULL,
    updated_at = now();

INSERT INTO roles (code)
VALUES
    ('todo_user'),
    ('docs_reader'),
    ('customer_signal_user'),
    ('tenant_admin')
ON CONFLICT (code) DO NOTHING;

INSERT INTO tenants (slug, display_name)
VALUES ('acme', 'Acme')
ON CONFLICT (slug) DO UPDATE
SET
    display_name = EXCLUDED.display_name,
    active = true,
    updated_at = now();

UPDATE users
SET
    default_tenant_id = (SELECT id FROM tenants WHERE slug = 'acme'),
    updated_at = now()
WHERE email = 'limited@example.com';

DELETE FROM user_roles
WHERE user_id = (SELECT id FROM users WHERE email = 'limited@example.com');

DELETE FROM tenant_memberships
WHERE user_id = (SELECT id FROM users WHERE email = 'limited@example.com');

INSERT INTO tenant_memberships (user_id, tenant_id, role_id, source)
SELECT u.id, t.id, r.id, 'local_override'
FROM users u
JOIN tenants t ON t.slug = 'acme'
JOIN roles r ON r.code IN ('todo_user', 'docs_reader')
WHERE u.email = 'limited@example.com'
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET
    active = true,
    updated_at = now();
```

`limited@example.com` は `tenant_admin` と `customer_signal_user` を持ちません。そのため、Tenant Admin と Customer Signals の role 不足 UI を確認できます。一方で `todo_user` は持たせておくと、login と tenant selection は正常に動く user として使えます。

### 5-2. single binary E2E runner を追加する

#### ファイル: `scripts/e2e-single-binary.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${E2E_HTTP_PORT:-18080}"
BASE_URL="${E2E_BASE_URL:-http://127.0.0.1:${PORT}}"
DATABASE_URL="${DATABASE_URL:-postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable}"
REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"
FILE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/haohao-e2e-files.XXXXXX")"
LOG_FILE="$(mktemp "${TMPDIR:-/tmp}/haohao-e2e-server.XXXXXX")"
SERVER_PID=""

cleanup() {
  local status=$?
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  rm -rf "$FILE_DIR"
  if [[ "$status" -eq 0 ]]; then
    rm -f "$LOG_FILE"
  else
    echo "E2E server log retained: $LOG_FILE" >&2
  fi
  exit "$status"
}
trap cleanup EXIT

cd "$ROOT_DIR"

if [[ ! -x ./bin/haohao ]]; then
  echo "bin/haohao is missing. Run make binary first." >&2
  exit 1
fi

for attempt in {1..60}; do
  if psql "$DATABASE_URL" -c 'select 1' >/dev/null 2>&1; then
    break
  fi
  if [[ "$attempt" == "60" ]]; then
    echo "database is not reachable: $DATABASE_URL" >&2
    exit 1
  fi
  sleep 1
done

migrate -path db/migrations -database "$DATABASE_URL" up
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/seed-demo-user.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/seed-e2e-users.sql

HTTP_PORT="$PORT" \
APP_BASE_URL="$BASE_URL" \
FRONTEND_BASE_URL="$BASE_URL" \
DATABASE_URL="$DATABASE_URL" \
REDIS_ADDR="$REDIS_ADDR" \
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
COOKIE_SECURE=false \
DOCS_AUTH_REQUIRED=false \
RATE_LIMIT_ENABLED=false \
FILE_LOCAL_DIR="$FILE_DIR" \
OUTBOX_WORKER_INTERVAL=200ms \
OUTBOX_WORKER_TIMEOUT=2s \
DATA_LIFECYCLE_ENABLED=false \
./bin/haohao >"$LOG_FILE" 2>&1 &
SERVER_PID="$!"

for attempt in {1..80}; do
  if curl -fsS "$BASE_URL/readyz" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    echo "E2E server exited early. Log: $LOG_FILE" >&2
    sed -n '1,160p' "$LOG_FILE" >&2 || true
    exit 1
  fi
  if [[ "$attempt" == "80" ]]; then
    echo "E2E server did not become ready. Log: $LOG_FILE" >&2
    sed -n '1,160p' "$LOG_FILE" >&2 || true
    exit 1
  fi
  sleep 0.25
done

E2E_BASE_URL="$BASE_URL" npm --prefix frontend run e2e -- --project=chromium
```

この script は `.env` を直接 source しません。`bin/haohao` 側の `config.Load()` が `.env` を読むためです。script で指定する `HTTP_PORT`、`APP_BASE_URL`、`DATABASE_URL` などは environment variable として渡すので、`.env` より優先されます。

local で動かすには PostgreSQL と Redis が必要です。

```bash
make up
make e2e
```

## Step 6. E2E 用の安定 selector を足す

まずは visible label で選べる箇所はそのまま使います。曖昧になりやすい箇所だけ `data-testid` を足します。

### 6-1. Login form

#### ファイル: `frontend/src/views/LoginView.vue`

```vue
<input
  v-model="email"
  data-testid="login-email"
  class="field-input"
  type="email"
  required
  autocomplete="username"
/>

<input
  v-model="password"
  data-testid="login-password"
  class="field-input"
  type="password"
  required
  minlength="8"
  autocomplete="current-password"
/>
```

### 6-2. Identity と tenant selector

#### ファイル: `frontend/src/App.vue`

```vue
<span class="identity-status" data-testid="identity-status">
  {{ statusLabel }}
</span>
```

#### ファイル: `frontend/src/components/TenantSelector.vue`

```vue
<select
  id="tenant-selector"
  data-testid="tenant-selector"
  class="field-input tenant-select"
  :disabled="disabled"
  :value="selectedSlug"
  @change="onChange"
>
```

### 6-3. Tenant Admin detail の曖昧な form

Tenant Admin detail には複数の `Email`、`Role`、`Save` があるため、E2E が使う field と button にだけ test id を足します。

#### ファイル: `frontend/src/views/TenantAdminTenantDetailView.vue`

```vue
<input
  v-model="invitationEmail"
  data-testid="tenant-invitation-email"
  class="field-input"
  autocomplete="email"
  type="email"
  required
>

<select
  v-model="invitationRoleCode"
  data-testid="tenant-invitation-role"
  class="field-input"
>

<input
  v-model.number="fileQuotaBytes"
  data-testid="tenant-file-quota"
  class="field-input"
  min="0"
  type="number"
>

<input
  v-model.number="browserRateLimit"
  data-testid="tenant-browser-rate-limit"
  class="field-input"
  min="1"
  type="number"
>

<button
  data-testid="tenant-request-export"
  class="primary-button compact-button"
  type="button"
  :disabled="commonStore.saving"
  @click="requestExport"
>
  Request export
</button>
```

`data-testid` は必要になった場所にだけ足します。画面上の文言を E2E の都合で増やさないことが重要です。

## Step 7. Playwright fixtures を追加する

login、tenant 選択、run id 生成は複数 spec で使うので helper に分けます。

#### ファイル: `e2e/fixtures/auth.ts`

```ts
import { expect, type Page } from '@playwright/test'

export const demoUser = {
  email: 'demo@example.com',
  password: 'changeme123',
}

export const limitedUser = {
  email: 'limited@example.com',
  password: 'changeme123',
}

export async function login(page: Page, user = demoUser) {
  await page.goto('/login')
  await page.getByTestId('login-email').fill(user.email)
  await page.getByTestId('login-password').fill(user.password)
  await page.getByRole('button', { name: 'Sign in' }).click()

  await expect(page.getByTestId('identity-status')).toHaveText('Authenticated')
}

export async function selectTenant(page: Page, slug: string) {
  await page.getByTestId('tenant-selector').selectOption(slug)
  await expect(page.getByTestId('tenant-selector')).toHaveValue(slug)
}
```

#### ファイル: `e2e/fixtures/run-id.ts`

```ts
import type { TestInfo } from '@playwright/test'

export function runId(testInfo: TestInfo) {
  return `p9-${Date.now()}-${testInfo.workerIndex}`
}
```

helper は browser 操作の共通部だけに留めます。DB 直接操作や backend API setup を helper に混ぜると、E2E が何を保証しているのか分かりにくくなります。

## Step 8. primary browser journey spec を追加する

最初の spec は、P7 で増えた browser flow を 1 本にまとめます。stateful な画面遷移を扱うため、初期版では `serial` にします。

#### ファイル: `e2e/browser-journey.spec.ts`

```ts
import { expect, test } from '@playwright/test'
import { writeFile } from 'node:fs/promises'

import { login, selectTenant } from './fixtures/auth'
import { runId } from './fixtures/run-id'

test.describe.serial('P9 browser journey', () => {
  test('login, tenant, signals, files, settings, export, notifications', async ({ page }, testInfo) => {
    const id = runId(testInfo)
    const signalTitle = `P9 signal ${id}`
    const uploadPath = testInfo.outputPath(`attachment-${id}.txt`)

    await login(page)
    await selectTenant(page, 'beta')
    await selectTenant(page, 'acme')

    await page.getByRole('link', { name: 'Signals' }).click()
    await expect(page.getByRole('heading', { name: 'Signals' })).toBeVisible()

    await page.getByRole('textbox', { name: 'Customer', exact: true }).fill('Acme')
    await page.getByRole('textbox', { name: 'Title', exact: true }).fill(signalTitle)
    await page.getByRole('textbox', { name: 'Details', exact: true }).fill(`Created by Playwright ${id}`)
    await page.getByRole('button', { name: 'Add Signal' }).click()

    await expect(page.getByRole('link', { name: signalTitle })).toBeVisible()
    await page.getByRole('link', { name: signalTitle }).click()
    await expect(page.getByRole('heading', { name: 'Signal Detail' })).toBeVisible()

    await writeFile(uploadPath, `hello from ${id}\n`)
    await page.getByLabel('File').setInputFiles(uploadPath)
    await page.getByRole('button', { name: 'Upload' }).click()
    await expect(page.getByText(`attachment-${id}.txt`)).toBeVisible()

    await page.goto('/tenant-admin/acme')
    await expect(page.getByRole('heading', { name: 'Tenant Detail' })).toBeVisible()

    await page.getByTestId('tenant-file-quota').fill('104857600')
    await page.getByTestId('tenant-browser-rate-limit').fill('120')
    await page.getByRole('button', { name: 'Save common settings' }).click()
    await expect(page.getByText('Tenant common settings を更新しました。')).toBeVisible()

    await page.getByTestId('tenant-invitation-email').fill('demo@example.com')
    await page.getByTestId('tenant-invitation-role').selectOption('todo_user')
    await page.getByRole('button', { name: 'Invite' }).click()
    await expect(page.getByText('Invitation を作成しました')).toBeVisible()

    await page.getByTestId('tenant-request-export').click()
    await expect(page.getByText('Tenant data export を request しました。')).toBeVisible()
    await expect(page.getByText(/json \/ (processing|ready)/i).first()).toBeVisible()

    await page.getByRole('link', { name: 'Notifications' }).click()
    await expect(page.getByRole('heading', { name: 'Notification Center' })).toBeVisible()
    await expect(page.getByText('Tenant invitation').first()).toBeVisible()
  })
})
```

この spec は「画面で主要 flow が通ること」を見るものです。API response の全 field や DB の詳細状態は確認しません。

## Step 9. access boundary と SPA fallback spec を追加する

primary journey とは別に、壊れやすい境界だけを小さく確認します。

#### ファイル: `e2e/access-and-fallback.spec.ts`

```ts
import { expect, test } from '@playwright/test'

import { limitedUser, login } from './fixtures/auth'

test('limited user sees role-specific access denied UI', async ({ page }) => {
  await login(page, limitedUser)

  await page.goto('/tenant-admin')
  await expect(page.getByText('Tenant admin role required')).toBeVisible()
  await expect(page.getByText('tenant_admin')).toBeVisible()

  await page.goto('/customer-signals')
  await expect(page.getByText('Customer Signal role required')).toBeVisible()
  await expect(page.getByText('customer_signal_user')).toBeVisible()

  await page.goto('/todos')
  await expect(page.getByRole('heading', { name: 'TODO' })).toBeVisible()
})

test('single binary keeps SPA fallback separate from API and assets', async ({ page, request }) => {
  await page.goto('/customer-signals')
  await expect(page.getByRole('heading', { name: 'Login' })).toBeVisible()

  const missingAsset = await request.get('/assets/definitely-missing-p9.js')
  expect(missingAsset.status()).toBe(404)

  const missingAPI = await request.get('/api/definitely-missing-p9')
  expect(missingAPI.status()).toBe(404)
})
```

direct route の `/customer-signals` は embedded SPA の `index.html` から Vue router が起動し、未認証なので Login に誘導されます。一方で `/assets/*` と `/api/*` は SPA fallback されず 404 になります。

## Step 10. Makefile と CI に E2E を追加する

### 10-1. Makefile target を追加する

#### ファイル: `Makefile`

```makefile
e2e: binary
	bash scripts/e2e-single-binary.sh
```

`binary` に依存させることで、E2E は常に embedded frontend を含む binary に対して動きます。

local では次の順番で確認します。

```bash
make up
npm --prefix frontend run e2e:install
make e2e
```

### 10-2. CI に Redis service と Playwright 実行を追加する

#### ファイル: `.github/workflows/ci.yml`

CI の service に Redis を追加します。

```yaml
services:
  postgres:
    image: postgres:18
    env:
      POSTGRES_DB: haohao
      POSTGRES_USER: haohao
      POSTGRES_PASSWORD: haohao
    ports:
      - 5432:5432
    options: >-
      --health-cmd "pg_isready -U haohao -d haohao"
      --health-interval 5s
      --health-timeout 5s
      --health-retries 20
  redis:
    image: redis:7.4
    ports:
      - 6379:6379
    options: >-
      --health-cmd "redis-cli ping"
      --health-interval 5s
      --health-timeout 5s
      --health-retries 20
```

frontend dependencies install 後に Playwright browser を install します。

```yaml
- name: Install Playwright browsers
  run: npm --prefix frontend exec -- playwright install --with-deps chromium
```

embedded binary build の後に E2E を実行します。

```yaml
- name: UI E2E
  run: make e2e
```

失敗時に report と trace を upload します。

```yaml
- name: Upload Playwright report
  if: failure()
  uses: actions/upload-artifact@v4
  with:
    name: playwright-report
    path: |
      playwright-report
      test-results
```

CI では `make e2e` が `make binary` を再実行します。build 時間が気になる場合は、後で `e2e-run` target を切り出して、CI では事前に作った binary を再利用してもよいです。P9 初期版では単純さを優先します。

## Step 11. 確認コマンド

実装後は次を実行します。

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
make e2e
git diff --check
git diff --exit-code -- openapi/openapi.yaml openapi/browser.yaml openapi/external.yaml frontend/src/api/generated backend/internal/db
```

CI と同じ browser install を local で確認したい場合は次も実行します。

```bash
npm --prefix frontend exec -- playwright install chromium
```

E2E だけを再実行したい場合は、PostgreSQL と Redis を起動したまま次を使います。

```bash
make e2e
```

port を変えたい場合は次です。

```bash
E2E_HTTP_PORT=18081 make e2e
```

## 実装時の注意点

### generated SDK は手で編集しない

P8 以降、frontend generated SDK は `openapi/browser.yaml` 由来です。P9 で SDK に足りない operation が見つかった場合でも、`frontend/src/api/generated/*` を手で直しません。

正しい順番は次です。

1. Huma route / OpenAPI surface を直す
2. `make gen` を実行する
3. frontend API wrapper を直す
4. Playwright を直す

### Playwright のために production UI copy を歪めない

E2E の selector は visible role / label を優先します。曖昧な箇所だけ `data-testid` を使います。

button text や heading を test の都合だけで増やすと、UI と test の責務が逆転します。画面に見えない `data-testid` で補助する方が安全です。

### E2E は stateful なので並列数を絞る

P9 初期版は local / CI ともに `workers: 1` にします。Customer Signal 作成、tenant settings 更新、invitation、export request は DB state を更新するため、最初から parallel にしません。

将来 spec が増えたら、test data prefix と isolated tenant を使って parallel 化します。

### role 不足 user は明示的に seed する

既存の `demo@example.com` は多くの role を持つため、role 不足 UI の確認には向きません。`limited@example.com` のように「どの role を持たないか」が明確な user を用意します。

E2E のために production authz を bypass する flag は追加しません。role 不足 UI は実際の authz error から表示される必要があります。

### Playwright は shell smoke を置き換えない

`smoke-common-services` は API / metrics / outbox / idempotency を速く確認できます。Playwright は browser でしか見えない壊れ方を拾うための追加 layer です。

P9 後も次は残します。

```bash
make smoke-operability
make smoke-observability
make smoke-tenant-admin
make smoke-customer-signals
make smoke-common-services
make smoke-backup-restore
```

### CI の flake は trace から原因を固定する

E2E が失敗したら、まず `playwright-report` と `test-results` の trace / screenshot を見ます。timeout を雑に伸ばす前に、次を確認します。

- app server が `/readyz` まで到達しているか
- seed が完了しているか
- selector が曖昧で別の element を掴んでいないか
- outbox worker の非同期完了を待ちすぎていないか
- test data が前回 run と衝突していないか

## 最終確認チェックリスト

- [ ] `frontend/package.json` に `@playwright/test` と `e2e` scripts がある
- [ ] `playwright.config.ts` が root にある
- [ ] `e2e/fixtures/auth.ts` と `e2e/fixtures/run-id.ts` がある
- [ ] `e2e/browser-journey.spec.ts` が login から notifications まで通す
- [ ] `e2e/access-and-fallback.spec.ts` が role 不足 UI と SPA fallback を見る
- [ ] `scripts/seed-e2e-users.sql` が limited user を作る
- [ ] `scripts/e2e-single-binary.sh` が migration、seed、server 起動、Playwright 実行、cleanup を持つ
- [ ] `.gitignore` が `playwright-report` と `test-results` を ignore する
- [ ] `Makefile` に `e2e` target がある
- [ ] CI に Redis service、Playwright install、`make e2e`、失敗時 artifact upload がある
- [ ] `make gen` が通る
- [ ] `go test ./backend/...` が通る
- [ ] `npm --prefix frontend run build` が通る
- [ ] `make binary` が通る
- [ ] `make e2e` が通る
- [ ] generated SDK と OpenAPI YAML を手で編集していない
