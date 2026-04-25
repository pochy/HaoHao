# P2 TODO 縦切り実装チュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **P2: 業務ドメインの縦切りを追加する** を、実装できる順番に分解したチュートリアルです。

P0 では operability を閉じ、P1 では tenant selector / machine client admin UI / integrations UX を整えました。P2 では、その土台の上に最初の業務ドメイン機能として **tenant 共有 TODO** を追加します。

このチュートリアルで扱う TODO は、古い `TUTORIAL.md` の user-only TODO ではありません。現在の HaoHao に合わせて、次の性質を持つ browser CRUD として実装します。

- active tenant が必須
- active tenant の tenant role `todo_user` が必須
- mutation は CSRF token が必須
- OpenAPI から generated SDK を作り、frontend は wrapper 経由で使う
- single binary の SPA fallback で `/todos` を表示できる

この文書は `TUTORIAL.md` / `TUTORIAL_SINGLE_BINARY.md` / `TUTORIAL_P1_ADMIN_UI.md` と同じように、対象ファイル、主要コード方針、確認コマンド、完了条件まで追える形にしています。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- migration は `0007_machine_clients` まで存在する
- `roles` には `todo_user` が存在する
- `tenant_memberships` と active tenant session が存在する
- `GET /api/v1/tenants` と `POST /api/v1/session/tenant` が存在する
- `frontend/src/components/TenantSelector.vue` が app header に接続済み
- `frontend/src/api/client.ts` が Cookie / CSRF / generated SDK の共通 transport を持っている
- `frontend/src/api/generated/*` は `openapi/openapi.yaml` から生成されている
- `make smoke-operability` は既存 server、既定では `http://127.0.0.1:8080`、に対して確認する

この P2 では、tenant 自体の CRUD 管理 UI は追加しません。tenant と membership は、既存の provider claim / SCIM / local seed を使って準備します。

M2M / external bearer 用の TODO API も追加しません。最初の縦切りでは browser session + Cookie + CSRF + tenant role の経路に限定します。

## 完成条件

このチュートリアルの完了条件は次です。

- `0008_todos` migration で tenant 共有 TODO table が追加される
- `db/queries/todos.sql` から sqlc 生成物が作られる
- `TodoService` が active tenant 単位で list/create/update/delete を扱う
- `GET /api/v1/todos` で active tenant の TODO 一覧を取得できる
- `POST /api/v1/todos` で TODO を作成できる
- `PATCH /api/v1/todos/{todoPublicId}` で title / completed を更新できる
- `DELETE /api/v1/todos/{todoPublicId}` で TODO を削除できる
- 未ログインは `401`
- active tenant が無い場合は `409`
- active tenant に `todo_user` tenant role が無い場合は `403`
- 空 title は `400`
- tenant 外または存在しない TODO は `404`
- `/todos` 画面で list/create/update/delete ができる
- active tenant を Acme / Beta で切り替えると TODO 一覧が混ざらない
- `npm --prefix frontend run build` が通る
- `go test ./backend/...` が通る
- `make binary` が通る
- single binary を `:8080` で起動した状態で `make smoke-operability` が通る

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | `db/migrations/0008_todos.*.sql` | tenant 共有 TODO schema を追加する |
| Step 2 | `db/queries/todos.sql` | sqlc 用 query を追加する |
| Step 3 | `backend/internal/service/todo_service.go` | domain service と validation を追加する |
| Step 4 | `backend/internal/api/todos.go` | Huma operation を追加する |
| Step 5 | backend wiring / OpenAPI | runtime と OpenAPI export に TodoService を接続する |
| Step 6 | `frontend/src/api/*`, `frontend/src/stores/*` | generated SDK wrapper と Pinia store を追加する |
| Step 7 | `frontend/src/views/*`, router, nav | `/todos` 画面と導線を追加する |
| Step 8 | local seed / browser smoke | Acme / Beta で tenant 分離を確認する |
| Step 9 | generated artifact / CI / binary smoke | 生成物 drift と配布形を確認する |

## Step 1. TODO schema / migration を追加する

### 1-1. up migration を追加する

#### ファイル: `db/migrations/0008_todos.up.sql`

```sql
CREATE TABLE todos (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX todos_public_id_idx
    ON todos(public_id);

CREATE INDEX todos_tenant_id_created_at_idx
    ON todos(tenant_id, created_at DESC, id DESC);

CREATE INDEX todos_created_by_user_id_idx
    ON todos(created_by_user_id);
```

TODO は tenant 共有の業務データです。`tenant_id` を必須にし、一覧・更新・削除の query では必ず `tenant_id` で絞ります。

`created_by_user_id` は作成者の追跡用です。P2 の API response では必須表示しませんが、あとで監査表示や activity feed を追加できるように保持します。

### 1-2. down migration を追加する

#### ファイル: `db/migrations/0008_todos.down.sql`

```sql
DROP TABLE IF EXISTS todos;
```

### 1-3. schema snapshot を更新する

migration を追加したら、schema snapshot を更新します。

```bash
make db-schema
```

`db/schema.sql` は生成物ですが、この repository では tracked artifact として扱います。migration を足したら差分が出るのが正しい状態です。

## Step 2. sqlc query を追加する

#### ファイル: `db/queries/todos.sql`

```sql
-- name: ListTodosByTenantID :many
SELECT
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    title,
    completed,
    created_at,
    updated_at
FROM todos
WHERE tenant_id = $1
ORDER BY created_at DESC, id DESC;

-- name: GetTodoByPublicIDForTenant :one
SELECT
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    title,
    completed,
    created_at,
    updated_at
FROM todos
WHERE public_id = $1
  AND tenant_id = $2
LIMIT 1;

-- name: CreateTodo :one
INSERT INTO todos (
    tenant_id,
    created_by_user_id,
    title
) VALUES (
    $1,
    $2,
    $3
)
RETURNING
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    title,
    completed,
    created_at,
    updated_at;

-- name: UpdateTodoByPublicIDForTenant :one
UPDATE todos
SET
    title = $3,
    completed = $4,
    updated_at = now()
WHERE public_id = $1
  AND tenant_id = $2
RETURNING
    id,
    public_id,
    tenant_id,
    created_by_user_id,
    title,
    completed,
    created_at,
    updated_at;

-- name: DeleteTodoByPublicIDForTenant :execrows
DELETE FROM todos
WHERE public_id = $1
  AND tenant_id = $2;
```

query を追加したら sqlc を生成します。

```bash
cd backend && sqlc generate
```

ここで `backend/internal/db/todos.sql.go` が生成されます。

## Step 3. TodoService を追加する

#### ファイル: `backend/internal/service/todo_service.go`

`TodoService` は HTTP や Cookie を知らない domain service として作ります。認証済み user ID と active tenant ID は API 層から渡します。

公開する method は次の 4 つです。

```go
func (s *TodoService) List(ctx context.Context, tenantID int64) ([]Todo, error)
func (s *TodoService) Create(ctx context.Context, tenantID, userID int64, title string) (Todo, error)
func (s *TodoService) Update(ctx context.Context, tenantID int64, publicID string, input TodoUpdateInput) (Todo, error)
func (s *TodoService) Delete(ctx context.Context, tenantID int64, publicID string) error
```

主要な型は次の方針にします。

```go
var (
    ErrInvalidTodoTitle  = errors.New("invalid todo title")
    ErrInvalidTodoUpdate = errors.New("invalid todo update")
    ErrTodoNotFound      = errors.New("todo not found")
)

type Todo struct {
    PublicID  string
    Title     string
    Completed bool
    CreatedAt time.Time
    UpdatedAt time.Time
}

type TodoUpdateInput struct {
    Title     *string
    Completed *bool
}
```

validation は service に寄せます。

- create の title は `strings.TrimSpace` 後に空なら `ErrInvalidTodoTitle`
- update は `Title` と `Completed` が両方 nil なら `ErrInvalidTodoUpdate`
- update の `Title` が指定されていて trim 後に空なら `ErrInvalidTodoTitle`
- `publicID` が UUID として parse できない場合は `ErrTodoNotFound`
- update はまず `GetTodoByPublicIDForTenant` で既存値を読み、未指定 field は既存値を使って `UpdateTodoByPublicIDForTenant` に渡す
- delete の affected rows が 0 なら `ErrTodoNotFound`

tenant 外の TODO を触ろうとした場合も `ErrTodoNotFound` にします。存在確認を tenant 抜きで行わないことで、他 tenant の TODO の存在を漏らしません。

## Step 4. Huma API を追加する

#### ファイル: `backend/internal/api/todos.go`

API は browser session 用の Huma operation として追加します。endpoint は次の 4 つです。

| Method | Path | 目的 |
| --- | --- | --- |
| `GET` | `/api/v1/todos` | active tenant の TODO 一覧 |
| `POST` | `/api/v1/todos` | TODO 作成 |
| `PATCH` | `/api/v1/todos/{todoPublicId}` | TODO 更新 |
| `DELETE` | `/api/v1/todos/{todoPublicId}` | TODO 削除 |

request / response model は次の方針にします。

```go
type TodoBody struct {
    PublicID  string    `json:"publicId" format:"uuid"`
    Title     string    `json:"title"`
    Completed bool      `json:"completed"`
    CreatedAt time.Time `json:"createdAt" format:"date-time"`
    UpdatedAt time.Time `json:"updatedAt" format:"date-time"`
}

type TodoListBody struct {
    Items []TodoBody `json:"items"`
}

type CreateTodoBody struct {
    Title string `json:"title" maxLength:"200"`
}

type UpdateTodoBody struct {
    Title     *string `json:"title,omitempty" maxLength:"200"`
    Completed *bool   `json:"completed,omitempty"`
}
```

`CreateTodoBody.Title` には `minLength:"1"` を付けません。空文字や whitespace-only title は `TodoService` の validation まで通し、`400 invalid todo title` として返します。Huma schema validation で先に落とすと `400` ではなく request validation error になり、完成条件とずれます。

### 4-1. authorization helper を作る

`todos.go` には TODO 専用の helper を置きます。

```go
func requireTodoTenant(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, service.TenantAccess, error)
```

この helper は次を行います。

- `csrfToken == ""` の read request は `currentSessionAuthContext` を使う
- mutation は `currentSessionAuthContextWithCSRF` を使う
- `deps.TodoService == nil` なら `503`
- `authCtx.ActiveTenant == nil` なら `409 active tenant is required`
- `authCtx.ActiveTenant.Roles` に `todo_user` がなければ `403 todo_user tenant role is required`

`todo_user` は global role ではなく tenant role として扱います。つまり `authCtx.HasRole("todo_user")` だけで許可しません。

### 4-2. HTTP error mapping を追加する

既存の `toHTTPError()` は session 系の error を扱っています。TODO 専用 error は `todos.go` 側に次のような helper を作り、operation 内で使います。

```go
func toTodoHTTPError(err error) error {
    switch {
    case errors.Is(err, service.ErrInvalidTodoTitle):
        return huma.Error400BadRequest("invalid todo title")
    case errors.Is(err, service.ErrInvalidTodoUpdate):
        return huma.Error400BadRequest("invalid todo update")
    case errors.Is(err, service.ErrTodoNotFound):
        return huma.Error404NotFound("todo not found")
    default:
        return toHTTPError(err)
    }
}
```

### 4-3. operation registration を追加する

`registerTodoRoutes(api, deps)` を作り、operation tag は `todos` にします。

mutation endpoint は `X-CSRF-Token` header を必須にします。

```go
type CreateTodoInput struct {
    SessionCookie http.Cookie `cookie:"SESSION_ID"`
    CSRFToken     string      `header:"X-CSRF-Token" required:"true"`
    Body          CreateTodoBody
}
```

`PATCH` は `DefaultStatus: http.StatusOK`、`DELETE` は `DefaultStatus: http.StatusNoContent` にします。

## Step 5. runtime / OpenAPI wiring を更新する

### 5-1. API dependencies に TodoService を追加する

#### ファイル: `backend/internal/api/register.go`

`Dependencies` に `TodoService *service.TodoService` を追加し、`Register()` で `registerTodoRoutes(api, deps)` を呼びます。

```go
type Dependencies struct {
    SessionService *service.SessionService
    TodoService    *service.TodoService
    // 既存 fields...
}

func Register(api huma.API, deps Dependencies) {
    registerAuthSettingsRoute(api, deps)
    registerOIDCRoutes(api, deps)
    registerSessionRoutes(api, deps)
    registerExternalRoutes(api, deps)
    registerIntegrationRoutes(api, deps)
    registerTenantRoutes(api, deps)
    registerTodoRoutes(api, deps)
    registerMachineClientRoutes(api, deps)
    registerM2MRoutes(api, deps)
    registerSCIMRoutes(api, deps)
}
```

### 5-2. App constructor に TodoService を通す

#### ファイル: `backend/internal/app/app.go`

`app.New(...)` の引数に `todoService *service.TodoService` を追加し、`backendapi.Dependencies` へ渡します。

この変更は runtime と OpenAPI export の両方に効きます。`cmd/main` だけに追加して `cmd/openapi` を忘れると、runtime では動くのに `openapi/openapi.yaml` に `/api/v1/todos` が出ない状態になります。

### 5-3. runtime entrypoint に TodoService を作る

#### ファイル: `backend/cmd/main/main.go`

`queries := db.New(pool)` の後に `todoService := service.NewTodoService(queries)` を作り、`app.New(...)` に渡します。

### 5-4. OpenAPI export entrypoint に TodoService を作る

#### ファイル: `backend/cmd/openapi/main.go`

OpenAPI export でも `TodoService` を `app.New(...)` に渡します。

`backend/cmd/openapi/main.go` は DB に接続しません。既存の pattern と同じく nil pool / nil redis で Huma operation schema だけ作れるようにし、`TodoService` も `service.NewTodoService(nil)` のように nil queries を許容する形で渡します。

operation 登録時に DB query を実行しない限り、OpenAPI export は動きます。runtime では `backend/cmd/main/main.go` で `queries := db.New(pool)` から作った `TodoService` を渡します。

### 5-5. single binary SPA fallback の確認

`/todos` は frontend route です。`backend/frontend.go` 側で reserved path に入れません。

既存の SPA fallback が `/machine-clients` と同じ扱いなら、`/todos` は追加実装なしで `index.html` を返せます。もし frontend fallback test が明示的な route list を持っている場合は、`/todos` も test case に追加します。

## Step 6. frontend API wrapper / Pinia store を追加する

### 6-1. generated SDK を再生成する

backend API を追加した後、OpenAPI と generated SDK を更新します。

```bash
go run ./backend/cmd/openapi > /tmp/haohao-openapi.yaml
/usr/bin/diff -u openapi/openapi.yaml /tmp/haohao-openapi.yaml
cp /tmp/haohao-openapi.yaml openapi/openapi.yaml
npm --prefix frontend run openapi-ts
```

macOS / zsh で `diff` が `delta` に alias されている場合があります。この repository の確認では `/usr/bin/diff -u` を使います。

### 6-2. TODO API wrapper を追加する

#### ファイル: `frontend/src/api/todos.ts`

generated SDK を view から直接呼ばず、wrapper を作ります。

```ts
import { readCookie } from './client'
import {
  createTodo,
  deleteTodo,
  listTodos,
  updateTodo,
} from './generated/sdk.gen'
import type { CreateTodoBody, TodoBody, UpdateTodoBody } from './generated/types.gen'

export async function fetchTodos(): Promise<TodoBody[]> {
  const data = await listTodos({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: TodoBody[] | null }

  return data.items ?? []
}

export async function createTodoItem(body: CreateTodoBody): Promise<TodoBody> {
  return createTodo({
    headers: { 'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '' },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TodoBody>
}

export async function updateTodoItem(todoPublicId: string, body: UpdateTodoBody): Promise<TodoBody> {
  return updateTodo({
    path: { todoPublicId },
    headers: { 'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '' },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TodoBody>
}

export async function deleteTodoItem(todoPublicId: string) {
  return deleteTodo({
    path: { todoPublicId },
    headers: { 'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '' },
    throwOnError: true,
  })
}
```

operationId は Huma 側で `listTodos`, `createTodo`, `updateTodo`, `deleteTodo` にします。名前がずれると generated SDK の import 名も変わるため、Step 4 の operationId とこの wrapper を合わせます。

### 6-3. API error helper を補強する

#### ファイル: `frontend/src/api/client.ts`

既存の `isApiForbidden()` は machine client / docs を主に見ています。TODO の 403 UI でも使えるように、message 判定に `todo_user` を加えます。

```ts
return /forbidden|machine_client_admin|docs_reader|todo_user/i.test(message)
```

### 6-4. TODO store を追加する

#### ファイル: `frontend/src/stores/todos.ts`

Pinia store は view の状態を薄く保つために追加します。

state は次を持ちます。

- `items`
- `status`: `idle | loading | ready | empty | error | forbidden`
- `errorMessage`
- `creating`
- `updatingPublicId`
- `deletingPublicId`

actions は次を持ちます。

- `load()`
- `create(title: string)`
- `toggle(todoPublicId: string, completed: boolean)`
- `rename(todoPublicId: string, title: string)`
- `remove(todoPublicId: string)`
- `reset()`

`load()` の catch では `toApiErrorStatus(error) === 403` または `isApiForbidden(error)` の場合に `status = 'forbidden'` にします。`409 active tenant is required` は `error` として表示し、画面側では tenant selector の選択を促します。

create / update / delete 成功後は store 内の `items` を更新します。tenant を切り替えた場合は view 側の watcher で `load()` し直すため、store に tenant slug を持たせる必要はありません。

## Step 7. TODO 画面と navigation を追加する

### 7-1. router に `/todos` を追加する

#### ファイル: `frontend/src/router/index.ts`

```ts
import TodosView from '../views/TodosView.vue'

{
  path: '/todos',
  name: 'todos',
  component: TodosView,
  meta: { requiresAuth: true },
}
```

### 7-2. App navigation に TODO を追加する

#### ファイル: `frontend/src/App.vue`

`Machine Clients` と同じ nav に追加します。

```vue
<RouterLink to="/todos">TODO</RouterLink>
```

### 7-3. HomeView には導線だけを追加する

#### ファイル: `frontend/src/views/HomeView.vue`

TODO 本体は `TodosView.vue` に分離します。Home は session verification の画面として残し、action row に `/todos` への link を追加する程度にします。

```vue
<RouterLink class="secondary-link" to="/todos">Open TODO</RouterLink>
```

既存 style に `secondary-link` が無い場合は、`secondary-button` と同じ見た目で anchor 用 class を追加します。

### 7-4. TODO view を追加する

#### ファイル: `frontend/src/views/TodosView.vue`

画面は operational tool として、軽い CRUD に集中させます。大きな landing page にはしません。

期待する UI state は次です。

- active tenant 名を表示する
- title input と Add button で TODO を作成する
- loading 中は list の高さが大きく崩れないようにする
- empty state は短く表示する
- 403 は `TODO role required` と `この画面を使うには active tenant の todo_user role が必要です。` を表示する
- 各 TODO は checkbox、title、createdAt、delete button を持つ
- title rename は inline input または prompt ではなく、小さな form として扱う

tenant 切り替えへの追従は view 側で行います。

```ts
watch(
  () => tenantStore.activeTenant?.slug,
  async (slug) => {
    todoStore.reset()
    if (slug) {
      await todoStore.load()
    }
  },
  { immediate: true },
)
```

`tenantStore.status === 'idle'` の場合は、view mount 時に `tenantStore.load()` を呼んでから TODO を load します。P1 の app shell ですでに tenant selector が load していても、TODO view 自体が direct access に耐えるようにします。

### 7-5. style を追加する

#### ファイル: `frontend/src/style.css`

既存の panel / stack / action-row を再利用し、TODO 専用には次程度を追加します。

- `.todo-form`
- `.todo-list`
- `.todo-item`
- `.todo-title-row`
- `.todo-meta`
- `.inline-edit-form`

button や card の style を新設しすぎず、P1 の visual language に寄せます。mobile では todo item の action が折り返しても text が重ならないようにします。

## Step 8. seed / local 動作確認を追加する

### 8-1. local password login 用 seed を更新する

P2 smoke では `demo@example.com` を Acme / Beta の両方に所属させます。`todo_user` は tenant role として付与します。

この repository では、毎回手動 SQL を流さなくてよいように `scripts/seed-demo-user.sql` に組み込みます。

#### ファイル: `scripts/seed-demo-user.sql`

既存の demo user 作成 SQL の後ろに次を追加します。

```sql
INSERT INTO roles (code)
VALUES
  ('docs_reader'),
  ('machine_client_admin'),
  ('todo_user')
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
SET
  default_tenant_id = (SELECT id FROM tenants WHERE slug = 'acme'),
  updated_at = now()
WHERE email = 'demo@example.com';

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code = 'machine_client_admin'
WHERE u.email = 'demo@example.com'
ON CONFLICT (user_id, role_id) DO NOTHING;

INSERT INTO tenant_memberships (user_id, tenant_id, role_id, source)
SELECT u.id, t.id, r.id, 'local_override'
FROM users u
JOIN tenants t ON t.slug IN ('acme', 'beta')
JOIN roles r ON r.code IN ('todo_user', 'docs_reader')
WHERE u.email = 'demo@example.com'
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET active = true,
    updated_at = now();
```

更新後は次を実行します。

```bash
make db-up
make seed-demo-user
```

Zitadel login user で確認する場合は、`demo@example.com` の代わりに Zitadel callback 後に local DB に作られた email を使います。

### 8-2. single binary で browser smoke をする

配布形に近い確認は `8080` の single binary で行います。

root の `.env` が `AUTH_MODE=zitadel` の場合、`demo@example.com` の password login は `501 password login is disabled for the current auth mode` になります。demo login で P2 TODO を確認する時だけ、起動コマンドで local mode を上書きします。

terminal 1:

```bash
make binary
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
APP_BASE_URL=http://127.0.0.1:8080 \
./bin/haohao
```

terminal 2:

```bash
make smoke-operability
curl -i http://127.0.0.1:8080/api/v1/todos
curl -i http://127.0.0.1:8080/todos
```

期待値:

- `make smoke-operability` は成功する
- 未ログインの `GET /api/v1/todos` は `401 application/problem+json`
- `GET /todos` は `200 text/html` で `index.html` を返す

`/todos` が HTML を返すのは正常です。`/todos` は backend API ではなく SPA route なので、curl では TODO 画面そのものではなく `index.html` が見えます。実際の画面確認は browser で行います。

browser:

- `http://127.0.0.1:8080/login` を開く
- `demo@example.com` / `changeme123` で login
- header の tenant selector で `Acme / acme` を選ぶ
- `/todos` を開く
- `Acme todo 1` を作成する
- completed を toggle する
- title を更新する
- TODO を削除する
- tenant selector で `Beta / beta` を選ぶ
- `Beta` の一覧に `Acme` の TODO が出ないことを確認する
- `Beta todo 1` を作成する
- tenant selector で `Acme / acme` に戻り、`Beta todo 1` が出ないことを確認する

### 8-3. dev server で browser smoke をする

terminal 1:

```bash
make backend-dev
```

terminal 2:

```bash
npm --prefix frontend run dev
```

browser:

- `http://127.0.0.1:5173/login` で login
- header の tenant selector で `Acme` を選ぶ
- `/todos` を開く
- `Acme todo 1` を作成する
- completed を toggle する
- title を更新する
- TODO を削除する
- tenant selector で `Beta` を選ぶ
- `Beta` の一覧に `Acme` の TODO が出ないことを確認する
- `Beta todo 1` を作成する
- tenant selector で `Acme` に戻り、`Beta todo 1` が出ないことを確認する

### 8-4. 403 UI を確認する

一時的に `beta` の `todo_user` role を外します。

```bash
docker-compose exec -T postgres psql -U haohao -d haohao <<'SQL'
UPDATE tenant_memberships tm
SET active = false,
    updated_at = now()
FROM users u, tenants t, roles r
WHERE tm.user_id = u.id
  AND tm.tenant_id = t.id
  AND tm.role_id = r.id
  AND u.email = 'demo@example.com'
  AND t.slug = 'beta'
  AND r.code = 'todo_user';
SQL
```

browser で `Beta` を選び、`/todos` を reload します。

期待値:

- 画面が blank にならない
- `TODO role required` が表示される
- `Acme` に戻すと TODO が再度表示できる

確認後に戻します。

```bash
docker-compose exec -T postgres psql -U haohao -d haohao <<'SQL'
UPDATE tenant_memberships tm
SET active = true,
    updated_at = now()
FROM users u, tenants t, roles r
WHERE tm.user_id = u.id
  AND tm.tenant_id = t.id
  AND tm.role_id = r.id
  AND u.email = 'demo@example.com'
  AND t.slug = 'beta'
  AND r.code = 'todo_user';
SQL
```

## Step 9. generated artifact / CI / binary smoke を確認する

実装後は次を実行します。

```bash
make db-up
cd backend && sqlc generate
go run ./backend/cmd/openapi > /tmp/haohao-openapi.yaml
/usr/bin/diff -u openapi/openapi.yaml /tmp/haohao-openapi.yaml
npm --prefix frontend run openapi-ts
go test ./backend/...
npm --prefix frontend run build
make binary
```

OpenAPI drift が意図した `/api/v1/todos` だけなら、tracked artifact を更新します。

```bash
cp /tmp/haohao-openapi.yaml openapi/openapi.yaml
npm --prefix frontend run openapi-ts
```

single binary smoke:

```bash
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
APP_BASE_URL=http://127.0.0.1:8080 \
./bin/haohao
```

別 terminal:

```bash
make smoke-operability
curl -i http://127.0.0.1:8080/todos
curl -i http://127.0.0.1:8080/api/v1/todos
```

期待値:

- `make smoke-operability` は成功する
- `/todos` は HTML を返す
- 未ログインの `/api/v1/todos` は `401 application/problem+json`
- `/openapi.yaml` に `/api/v1/todos` が含まれる

root の `.env` が `AUTH_MODE=zitadel` のままでも `/todos` と未ログイン API の確認はできます。ただし `demo@example.com` / `changeme123` で browser login する場合は、上記のように `AUTH_MODE=local` と `ENABLE_LOCAL_PASSWORD_LOGIN=true` を起動時に上書きしてください。

Docker image の build も確認します。

```bash
docker build -t haohao:dev -f docker/Dockerfile .
```

CI に追加する確認は、まず既存の generated drift / build / test で十分です。browser login を必要とする TODO smoke は live dependency が絡むため、最初は local runbook 扱いにします。

## Troubleshooting

### `/todos` が 404 になる

frontend route が router に追加されているか確認します。

single binary でだけ 404 になる場合は、`make binary` の前に `npm --prefix frontend run build` が走っているか、`backend/frontend.go` の reserved path に `/todos` を入れていないか確認します。

### `curl /todos` が HTML を返す

正常です。`/todos` は Vue Router の SPA route です。backend は `index.html` を返し、browser が JavaScript を読み込んで TODO 画面を描画します。

curl では次のように見えれば OK です。

```text
HTTP/1.1 200 OK
Content-Type: text/html; charset=utf-8
```

### 未ログインの `/api/v1/todos` が 401 になる

正常です。P2 TODO API は browser session + Cookie 前提です。未ログインの curl では次の response が期待値です。

```text
HTTP/1.1 401 Unauthorized
Content-Type: application/problem+json
```

body の `detail` が `missing or expired session` なら、session が無いことを正しく拒否できています。

### local demo login が 501 になる

root の `.env` が `AUTH_MODE=zitadel` の場合、password login は無効です。`demo@example.com` で確認する時は single binary を次のように起動します。

```bash
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
APP_BASE_URL=http://127.0.0.1:8080 \
./bin/haohao
```

Zitadel login で確認する場合は、Zitadel callback 後に作られた local user に tenant membership と `todo_user` tenant role を付与します。

### `/api/v1/todos` が OpenAPI に出ない

`registerTodoRoutes(api, deps)` が `backend/internal/api/register.go` から呼ばれているか確認します。

runtime では動くが OpenAPI に出ない場合は、`backend/cmd/openapi/main.go` の `app.New(...)` に `TodoService` を渡していない可能性があります。

### `todo_user` があるのに 403 になる

P2 の TODO は global role ではなく active tenant の tenant role を見ます。

次の query で、ログイン中 user と active tenant の membership を確認します。

```bash
docker-compose exec -T postgres psql -U haohao -d haohao <<'SQL'
SELECT
  u.email,
  t.slug,
  r.code,
  tm.source,
  tm.active
FROM tenant_memberships tm
JOIN users u ON u.id = tm.user_id
JOIN tenants t ON t.id = tm.tenant_id
JOIN roles r ON r.id = tm.role_id
WHERE u.email = 'demo@example.com'
ORDER BY t.slug, r.code, tm.source;
SQL
```

`user_roles` に `todo_user` があるだけでは、P2 TODO API は許可しません。

### active tenant が無い

tenant selector が空の場合、ログイン中 user に `tenant_memberships` がありません。

local password login で確認するなら Step 8-1 の seed を流します。Zitadel login で確認するなら、Zitadel callback 後に作られた local user の email を使って membership を付与します。

### tenant を切り替えても TODO が混ざる

backend query が `tenant_id` で絞れているか確認します。

- list は `WHERE tenant_id = $1`
- get/update/delete は `WHERE public_id = $1 AND tenant_id = $2`

frontend 側では `tenantStore.activeTenant?.slug` を watch し、切り替え時に `todoStore.reset()` と `todoStore.load()` を実行します。

### PATCH が 400 になる

`title` と `completed` が両方未指定の場合は `400 invalid todo update` にします。

`title` を指定した場合、trim 後に空文字なら `400 invalid todo title` にします。

### CSRF で 403 になる

mutation wrapper が `X-CSRF-Token` を渡しているか確認します。

`frontend/src/api/client.ts` は mutating request 前に `/api/v1/csrf` を呼べますが、wrapper 側でも既存 pattern に合わせて `readCookie('XSRF-TOKEN')` を明示します。

## 完了チェックリスト

- [ ] `db/migrations/0008_todos.up.sql` / `.down.sql` を追加した
- [ ] `db/schema.sql` を更新した
- [ ] `db/queries/todos.sql` を追加した
- [ ] `cd backend && sqlc generate` を実行した
- [ ] `backend/internal/service/todo_service.go` を追加した
- [ ] `backend/internal/api/todos.go` を追加した
- [ ] `backend/internal/api/register.go` に `TodoService` と route registration を追加した
- [ ] `backend/internal/app/app.go` / `backend/cmd/main/main.go` / `backend/cmd/openapi/main.go` に TodoService wiring を追加した
- [ ] `openapi/openapi.yaml` を更新した
- [ ] `npm --prefix frontend run openapi-ts` を実行した
- [ ] `frontend/src/api/todos.ts` を追加した
- [ ] `frontend/src/stores/todos.ts` を追加した
- [ ] `frontend/src/router/index.ts` に `/todos` を追加した
- [ ] `frontend/src/App.vue` に `TODO` navigation を追加した
- [ ] `frontend/src/views/HomeView.vue` に TODO への導線を追加した
- [ ] `frontend/src/views/TodosView.vue` を追加した
- [ ] `frontend/src/style.css` に TODO UI style を追加した
- [ ] `scripts/seed-demo-user.sql` に Acme / Beta と `todo_user` membership を追加した
- [ ] `go test ./backend/...` が通った
- [ ] `npm --prefix frontend run build` が通った
- [ ] `make binary` が通った
- [ ] `make smoke-operability` が通った
- [ ] 未ログインの `GET /api/v1/todos` が `401 application/problem+json` を返すことを確認した
- [ ] `GET /todos` が `200 text/html` で SPA fallback されることを確認した
- [ ] Acme / Beta で TODO が混ざらないことを browser で確認した
- [ ] `todo_user` を外した tenant で 403 UI を確認した
