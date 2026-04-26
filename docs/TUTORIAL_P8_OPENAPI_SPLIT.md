# P8 OpenAPI 分割実装チュートリアル

## この文書の目的

この文書は、`deep-research-report.md` の **`openapi/openapi.yaml` の適切な分割** を、現在の HaoHao に実装できる順番へ分解したチュートリアルです。

P7 までで、browser session API、tenant admin、Customer Signals、file、notification、invitation、tenant settings、data export、external bearer、SCIM、M2M が同じ `openapi/openapi.yaml` に載るようになりました。単一 spec は drift check や全体 docs には便利ですが、frontend generated SDK、外部公開 API、machine client API、運用 docs の責務が混ざり始めています。

このチュートリアルの目的は、OpenAPI を手で分割することではありません。Huma の operation 登録元を single source of truth としたまま、利用者と security boundary に沿って次の 3 つの artifact を生成できるようにします。

- `openapi/openapi.yaml`: full canonical spec
- `openapi/browser.yaml`: frontend generated SDK 用の browser spec
- `openapi/external.yaml`: external bearer / M2M / SCIM 用の external spec

OpenAPI Initiative の Best Practices では、OpenAPI Description を source control に置くこと、single source of truth を保つこと、大きくなった API は toolchain に合う粒度で分割すること、operation tags で整理することが推奨されています。この P8 では、Huma の登録処理を正本にし、生成された YAML は tracked artifact として CI で drift を検知する形にします。

この文書は `TUTORIAL.md` / `TUTORIAL_SINGLE_BINARY.md` / `TUTORIAL_P7_WEB_SERVICE_COMMON.md` と同じように、対象ファイル、主要コード方針、確認コマンド、完了条件まで追える形にしています。

## この文書が前提にしている現在地

このチュートリアルを始める前の repository は、少なくとも次の状態にある前提で進めます。

- `backend/internal/app/app.go` が Gin / Huma app を組み立てている
- `backend/internal/api/register.go` が Huma operation をまとめて登録している
- `backend/cmd/openapi/main.go` が `application.API.OpenAPI().YAML()` を stdout に出している
- `Makefile` の `openapi` target が `openapi/openapi.yaml` を生成している
- `scripts/gen.sh` が sqlc、OpenAPI、frontend generated SDK をまとめて生成している
- `frontend/openapi-ts.config.ts` が `../openapi/openapi.yaml` を入力にしている
- CI の generated drift check が `openapi/openapi.yaml` と `frontend/src/api/generated` を確認している
- single binary runtime の `/openapi.yaml` は full spec を返している

この P8 では、API contract 自体は増やしません。DB migration、service、frontend UI も追加しません。変更するのは、OpenAPI の生成単位、generated SDK の入力、CI / release artifact の扱いです。

## 完成条件

このチュートリアルの完了条件は次です。

- `openapi/openapi.yaml` は full canonical spec として残る
- `openapi/browser.yaml` が生成される
- `openapi/browser.yaml` は Cookie session / CSRF 前提の `/api/v1/*` browser API を含む
- `openapi/browser.yaml` には `/api/external/*`、`/api/m2m/*`、`/api/scim/*` が含まれない
- `openapi/browser.yaml` には `bearerAuth`、`m2mBearerAuth` が含まれない
- `openapi/external.yaml` が生成される
- `openapi/external.yaml` は `/api/external/*`、`/api/m2m/*`、`/api/scim/*` を含む
- `openapi/external.yaml` には `/api/v1/*` が含まれない
- `openapi/external.yaml` には `cookieAuth` が含まれない
- `/openapi.yaml` runtime route は互換性維持のため full canonical spec を返す
- `frontend/openapi-ts.config.ts` は `../openapi/browser.yaml` を入力にする
- `make gen` が sqlc、3 つの OpenAPI spec、frontend generated SDK を更新する
- CI が `openapi/openapi.yaml`、`openapi/browser.yaml`、`openapi/external.yaml`、`frontend/src/api/generated` の drift を検知する
- release workflow が少なくとも full spec を載せ、必要に応じて browser / external spec も release asset に載せる
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary` が通る

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | 現状確認 | full spec、openapi command、frontend SDK、CI drift の役割を確認する |
| Step 2 | surface 方針 | path / security boundary / tag の分類を固定する |
| Step 3 | `backend/internal/api` | `SurfaceFull` / `SurfaceBrowser` / `SurfaceExternal` と `RegisterSurface` を追加する |
| Step 4 | `backend/internal/app`, `backend/cmd/openapi` | OpenAPI export 専用 helper と `-surface` option を追加する |
| Step 5 | `openapi/*.yaml`, `Makefile`, `scripts/gen.sh` | full / browser / external spec を生成対象にする |
| Step 6 | `frontend/openapi-ts.config.ts` | generated SDK の入力を browser spec に切り替える |
| Step 7 | `.github/workflows/*` | CI drift check、OpenAPI validate、release asset を更新する |
| Step 8 | tests / boundary check | surface の混入を unit test と grep check で検知する |

## 先に決める方針

### 正本は Huma operation 登録に置く

`openapi/openapi.yaml`、`openapi/browser.yaml`、`openapi/external.yaml` は tracked artifact ですが、手で編集しません。

正本は次です。

- Huma operation registration: `backend/internal/api/*.go`
- Huma config / security scheme: `backend/internal/app`
- generation command: `backend/cmd/openapi`
- frontend SDK generator config: `frontend/openapi-ts.config.ts`

YAML を手で切り出すと、full spec と subset spec のどちらが正しいのかが曖昧になります。OpenAPI Best Practices の single source of truth に合わせ、Go 側の登録単位から audience 別 spec を生成します。

### full spec は残す

`openapi/openapi.yaml` は削除しません。

理由は次です。

- runtime `/openapi.yaml` の互換性を維持できる
- 全体 docs と release artifact の入口として使える
- browser / external のどちらかに入れ忘れた operation を発見しやすい
- CI の drift check で従来と同じ full contract を見られる

### browser spec は frontend SDK 専用にする

`openapi/browser.yaml` は Vue frontend の generated SDK 入力です。

含めるのは Cookie session / CSRF 前提の browser API です。external bearer API、M2M runtime API、SCIM provisioning API は含めません。これにより、frontend generated SDK に external client 用の operation や security scheme が混ざることを防ぎます。

### external spec は browser Cookie に依存しない API にする

`openapi/external.yaml` は external bearer、M2M、SCIM のような browser Cookie に依存しない API をまとめます。

現状の delegated integration route は `/api/v1/integrations/*` であり、Cookie session と active tenant を前提にした browser flow です。そのため、P8 では browser spec に残します。将来、外部 client が browser Cookie なしで delegated grant を扱う API を追加した時点で、その新 API を external spec に入れます。

### YAML の `$ref` 分割はまだしない

OpenAPI の大規模化対策には、path hierarchy に沿った YAML file 分割もあります。しかし HaoHao は code-first で Huma から spec を生成しています。

P8 では、`paths/users.yaml` のような手書き `$ref` 分割は行いません。まずは audience 別 artifact を作り、toolchain が扱う YAML の大きさと責務を分けます。将来、外部公開 API が大きくなり design-first 寄りに移す場合だけ、YAML module 分割を別チュートリアルで扱います。

## Step 1. 現状の OpenAPI 生成経路を確認する

まず、現在の生成経路を確認します。

```bash
sed -n '1,120p' backend/cmd/openapi/main.go
sed -n '45,60p' Makefile
sed -n '1,40p' scripts/gen.sh
sed -n '1,20p' frontend/openapi-ts.config.ts
```

現在の役割は次です。

| ファイル | 現在の役割 |
| --- | --- |
| `backend/internal/api/register.go` | すべての Huma operation を 1 つの API に登録する |
| `backend/internal/app/app.go` | runtime app と Huma config を組み立てる |
| `backend/cmd/openapi/main.go` | full spec を stdout に出す |
| `Makefile` | `go run ./backend/cmd/openapi > openapi/openapi.yaml` を実行する |
| `scripts/gen.sh` | `openapi/openapi.yaml` 生成後に frontend SDK を生成する |
| `frontend/openapi-ts.config.ts` | `../openapi/openapi.yaml` から SDK を生成する |
| `.github/workflows/ci.yml` | generated drift と OpenAPI 3.1 validation を行う |

この状態だと frontend generated SDK は full spec を見ます。そのため、browser から呼ばない external / SCIM / M2M operation まで SDK に入ります。

Step 1 ではコードを変更しません。現在の境界を把握するだけです。

## Step 2. OpenAPI surface 方針を固定する

P8 の surface は次の 3 つにします。

#### `full`

`full` は従来の `openapi/openapi.yaml` と同じです。すべての Huma operation を含めます。

用途は次です。

- `/openapi.yaml` runtime route
- `/docs` の全体 docs
- release artifact
- 全体 contract の drift check

#### `browser`

`browser` は frontend generated SDK 用です。

含める operation は、browser Cookie session と CSRF を前提にした `/api/v1/*` API です。

| path / tag | 含める理由 |
| --- | --- |
| `/api/v1/auth/settings`, `/api/v1/auth/login`, `/api/v1/auth/callback` | browser login flow |
| `/api/v1/login`, `/api/v1/logout`, `/api/v1/session`, `/api/v1/session/refresh`, `/api/v1/csrf` | Cookie session / CSRF |
| `/api/v1/tenants`, `/api/v1/session/tenant` | active tenant selection |
| `/api/v1/admin/tenants*` | tenant admin UI |
| `/api/v1/customer-signals*` | browser 業務 UI |
| `/api/v1/todos*` | browser TODO UI |
| `/api/v1/notifications*` | in-app notification UI |
| `/api/v1/invitations/accept`, `/api/v1/admin/tenants/*/invitations*` | invitation UI |
| `/api/v1/files*` | file metadata UI |
| `/api/v1/admin/tenants/*/settings` | tenant settings UI |
| `/api/v1/admin/tenants/*/exports*` | tenant data export UI |
| `/api/v1/machine-clients*` | browser から machine client を管理する UI |
| `/api/v1/integrations*` | delegated integration は現状 browser Cookie flow |

`browser` に含めないものは次です。

- `/api/external/*`
- `/api/m2m/*`
- `/api/scim/*`
- `bearerAuth`
- `m2mBearerAuth`

#### `external`

`external` は browser Cookie に依存しない API 用です。

| path / tag | 含める理由 |
| --- | --- |
| `/api/external/*` | external bearer API |
| `/api/m2m/*` | machine client runtime API |
| `/api/scim/*` | SCIM provisioning API |

`external` に含めないものは次です。

- `/api/v1/*`
- `cookieAuth`

## Step 3. `backend/internal/api` の route 登録を surface 対応にする

Huma operation の登録元を分けられるように、`backend/internal/api/register.go` に surface を追加します。

#### ファイル: `backend/internal/api/register.go`

```go
type Surface string

const (
	SurfaceFull     Surface = "full"
	SurfaceBrowser  Surface = "browser"
	SurfaceExternal Surface = "external"
)

func (s Surface) Valid() bool {
	return s == SurfaceFull || s == SurfaceBrowser || s == SurfaceExternal
}

func Register(api huma.API, deps Dependencies) {
	RegisterSurface(api, deps, SurfaceFull)
}

func RegisterSurface(api huma.API, deps Dependencies, surface Surface) {
	if surface == "" {
		surface = SurfaceFull
	}

	if includeBrowser(surface) {
		registerAuthSettingsRoute(api, deps)
		registerOIDCRoutes(api, deps)
		registerSessionRoutes(api, deps)
		registerIntegrationRoutes(api, deps)
		registerTenantRoutes(api, deps)
		registerTenantAdminRoutes(api, deps)
		registerCustomerSignalRoutes(api, deps)
		registerTodoRoutes(api, deps)
		registerNotificationRoutes(api, deps)
		registerTenantInvitationRoutes(api, deps)
		registerFileRoutes(api, deps)
		registerTenantSettingsRoutes(api, deps)
		registerTenantDataExportRoutes(api, deps)
		registerMachineClientRoutes(api, deps)
	}

	if includeExternal(surface) {
		registerExternalRoutes(api, deps)
		registerM2MRoutes(api, deps)
		registerSCIMRoutes(api, deps)
	}
}

func includeBrowser(surface Surface) bool {
	return surface == SurfaceFull || surface == SurfaceBrowser
}

func includeExternal(surface Surface) bool {
	return surface == SurfaceFull || surface == SurfaceExternal
}
```

重要なのは、既存の `Register(api, deps)` を残すことです。runtime 側の呼び出しを壊さず、OpenAPI export だけが `RegisterSurface` を使えるようにします。

`registerTenantInvitationRoutes` には browser 管理 API と accept API が同居しています。どちらも `/api/v1/*` で Cookie session 前提なので、P8 では browser に含めます。

raw Gin route は別扱いです。たとえば file upload / download の raw route が Huma operation ではない場合、それらは現在の OpenAPI には出ません。P8 は「Huma に出ている operation の分割」を扱います。raw route を OpenAPI に載せたい場合は、別途 Huma operation 化するか、OpenAPI への明示追加を別 step にします。

## Step 4. OpenAPI export 用 helper と `-surface` option を追加する

現在の `backend/cmd/openapi/main.go` は runtime app を作り、その `API.OpenAPI()` をそのまま出しています。P8 では、OpenAPI export 専用に surface を指定できる helper を作ります。

### 4-1. Huma config を共有する

runtime app と export helper が同じ `info`、OpenAPI version、security scheme を使えるように、Huma config の作成を関数に切り出します。

#### ファイル: `backend/internal/app/openapi.go`

```go
package app

import (
	backendapi "example.com/haohao/backend/internal/api"
	"example.com/haohao/backend/internal/auth"
	"example.com/haohao/backend/internal/config"

	"github.com/danielgtaylor/huma/v2"
)

func humaConfigForSurface(cfg config.Config, surface backendapi.Surface) huma.Config {
	humaConfig := huma.DefaultConfig(cfg.AppName, cfg.AppVersion)

	securitySchemes := map[string]*huma.SecurityScheme{}
	if surface == backendapi.SurfaceFull || surface == backendapi.SurfaceBrowser {
		securitySchemes["cookieAuth"] = &huma.SecurityScheme{
			Type: "apiKey",
			In:   "cookie",
			Name: auth.SessionCookieName,
		}
	}
	if surface == backendapi.SurfaceFull || surface == backendapi.SurfaceExternal {
		securitySchemes["bearerAuth"] = &huma.SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		}
		securitySchemes["m2mBearerAuth"] = &huma.SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		}
	}

	humaConfig.Components.SecuritySchemes = securitySchemes
	return humaConfig
}
```

`backend/internal/app/app.go` の runtime 側もこの helper を使うようにします。

#### ファイル: `backend/internal/app/app.go`

```go
humaConfig := humaConfigForSurface(cfg, backendapi.SurfaceFull)
api := humagin.New(router, humaConfig)
```

runtime は従来どおり full surface です。これにより `/docs` と `/openapi.yaml` の互換性を維持します。

### 4-2. export 専用 app を作る

OpenAPI export では middleware、DB、Redis、frontend route は不要です。Huma API だけを作り、指定 surface の operation を登録します。

#### ファイル: `backend/internal/app/openapi.go`

```go
func NewOpenAPIExport(cfg config.Config, surface backendapi.Surface) (*huma.OpenAPI, error) {
	if !surface.Valid() {
		return nil, fmt.Errorf("invalid OpenAPI surface %q", surface)
	}

	router := gin.New()
	api := humagin.New(router, humaConfigForSurface(cfg, surface))
	backendapi.RegisterSurface(api, openAPIExportDependencies(cfg), surface)

	return api.OpenAPI(), nil
}
```

実際の import は既存 package 名に合わせます。`backendapi` alias、`fmt`、`gin`、`humagin` が必要です。

この helper は OpenAPI export 専用です。runtime app を置き換えるものではありません。

`openAPIExportDependencies` は、既存 `cmd/openapi` が持っていた nil DB の service stub を `backend/internal/app` 側に移した helper です。OpenAPI 生成に必要なのは operation の schema と registration であり、DB connection は不要です。

### 4-3. `cmd/openapi` に `-surface` を追加する

#### ファイル: `backend/cmd/openapi/main.go`

```go
surfaceFlag := flag.String("surface", string(backendapi.SurfaceFull), "OpenAPI surface: full, browser, external")
flag.Parse()

surface := backendapi.Surface(*surfaceFlag)
spec, err := app.NewOpenAPIExport(cfg, surface)
if err != nil {
	log.Fatal(err)
}

yamlBytes, err := spec.YAML()
if err != nil {
	log.Fatal(err)
}

fmt.Print(string(yamlBytes))
```

既存互換として、引数なしの次のコマンドは full spec を出し続けます。

```bash
go run ./backend/cmd/openapi
```

新しい使い方は次です。

```bash
go run ./backend/cmd/openapi -surface=full
go run ./backend/cmd/openapi -surface=browser
go run ./backend/cmd/openapi -surface=external
```

既存互換のため、`-surface` を指定しない場合は `SurfaceFull` を default にします。

## Step 5. 3 つの OpenAPI artifact を生成する

### 5-1. Makefile を更新する

#### ファイル: `Makefile`

```makefile
openapi:
	mkdir -p openapi
	go run ./backend/cmd/openapi -surface=full > openapi/openapi.yaml
	go run ./backend/cmd/openapi -surface=browser > openapi/browser.yaml
	go run ./backend/cmd/openapi -surface=external > openapi/external.yaml
```

`openapi/openapi.yaml` というファイル名は維持します。既存の README、docs、release asset、runtime `/openapi.yaml` との関係を壊さないためです。

### 5-2. `scripts/gen.sh` を更新する

#### ファイル: `scripts/gen.sh`

```bash
mkdir -p openapi

(
  cd backend
  sqlc generate
)

go run ./backend/cmd/openapi -surface=full > openapi/openapi.yaml
go run ./backend/cmd/openapi -surface=browser > openapi/browser.yaml
go run ./backend/cmd/openapi -surface=external > openapi/external.yaml

(
  cd frontend
  npm run openapi-ts
)
```

順番は OpenAPI 生成を frontend SDK 生成より前にします。Step 6 で `openapi-ts` の入力を `browser.yaml` に変えるためです。

### 5-3. 生成物を初回生成する

```bash
make gen
```

生成後に次を確認します。

```bash
test -s openapi/openapi.yaml
test -s openapi/browser.yaml
test -s openapi/external.yaml
grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/openapi.yaml
grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/browser.yaml
grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/external.yaml
```

## Step 6. frontend generated SDK の入力を browser spec に切り替える

frontend generated SDK は browser API だけを見るようにします。

#### ファイル: `frontend/openapi-ts.config.ts`

```ts
import { defineConfig } from '@hey-api/openapi-ts'

export default defineConfig({
  input: '../openapi/browser.yaml',
  output: 'src/api/generated',
})
```

この変更後、frontend SDK には `/api/external/*`、`/api/m2m/*`、`/api/scim/*` の operation が出なくなります。

確認します。

```bash
make gen
rg "External|SCIM|M2M|getExternalMe|getM2MSelf|scim" frontend/src/api/generated
```

この `rg` は match しないことを期待します。ただし schema 名や error message に偶然文字列が残る場合は、operation name と path の混入を優先して確認します。

より直接的には次を確認します。

```bash
! rg "/api/external/|/api/m2m/|/api/scim/" openapi/browser.yaml
! rg "bearerAuth|m2mBearerAuth" openapi/browser.yaml
```

## Step 7. CI と release workflow を更新する

### 7-1. generated drift check を更新する

#### ファイル: `.github/workflows/ci.yml`

```yaml
- name: Generated drift
  run: |
    make gen
    git diff --exit-code -- openapi/openapi.yaml openapi/browser.yaml openapi/external.yaml frontend/src/api/generated backend/internal/db
```

3 つの spec を tracked artifact として扱います。`openapi/browser.yaml` と `openapi/external.yaml` も commit 対象です。

### 7-2. OpenAPI validate を更新する

#### ファイル: `.github/workflows/ci.yml`

```yaml
- name: OpenAPI validate
  run: |
    test -s openapi/openapi.yaml
    test -s openapi/browser.yaml
    test -s openapi/external.yaml
    grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/openapi.yaml
    grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/browser.yaml
    grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/external.yaml
```

### 7-3. boundary check を CI に追加する

最初は grep ベースで十分です。YAML parser を増やさず、混入を明確に検知します。

#### ファイル: `.github/workflows/ci.yml`

```yaml
- name: OpenAPI surface boundary
  run: |
    ! grep -Eq '/api/(external|m2m|scim)/' openapi/browser.yaml
    ! grep -Eq 'bearerAuth|m2mBearerAuth' openapi/browser.yaml
    ! grep -Eq '/api/v1/' openapi/external.yaml
    ! grep -Eq 'cookieAuth' openapi/external.yaml
```

`grep -E` の pattern は実際の YAML に合わせます。SCIM path が `/api/scim/v2/Users` のように出る場合は、`/api/scim/` を検知対象にします。

### 7-4. release asset を更新する

#### ファイル: `.github/workflows/release.yml`

Validate step を 3 spec に拡張します。

```yaml
- name: Validate OpenAPI artifacts
  run: |
    test -s openapi/openapi.yaml
    test -s openapi/browser.yaml
    test -s openapi/external.yaml
    grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/openapi.yaml
    grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/browser.yaml
    grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/external.yaml
```

release asset には 3 spec を載せます。

```yaml
files: |
  openapi/openapi.yaml
  openapi/browser.yaml
  openapi/external.yaml
  dist/haohao-linux-amd64.tar.gz
```

external client の利用者に渡す artifact は `openapi/external.yaml` です。frontend SDK 再生成用に渡す artifact は `openapi/browser.yaml` です。全体 docs と互換性用途では `openapi/openapi.yaml` を使います。

## Step 8. regression test と boundary check を追加する

grep check は CI 上でわかりやすい一方、route registration の意図までは守れません。Go test で surface ごとの spec を直接確認します。

#### ファイル: `backend/internal/app/openapi_test.go`

```go
func TestOpenAPISurfaces(t *testing.T) {
	cfg := config.Config{
		AppName:    "HaoHao",
		AppVersion: "test",
		SCIMBasePath: "/api/scim/v2",
	}

	tests := []struct {
		name       string
		surface    backendapi.Surface
		wantPaths  []string
		blockPaths []string
		wantSchemes []string
		blockSchemes []string
	}{
		{
			name:        "browser",
			surface:     backendapi.SurfaceBrowser,
			wantPaths:   []string{"/api/v1/session", "/api/v1/customer-signals"},
			blockPaths:  []string{"/api/external/v1/me", "/api/m2m/v1/self", "/api/scim/v2/Users"},
			wantSchemes: []string{"cookieAuth"},
			blockSchemes: []string{"bearerAuth", "m2mBearerAuth"},
		},
		{
			name:        "external",
			surface:     backendapi.SurfaceExternal,
			wantPaths:   []string{"/api/external/v1/me", "/api/m2m/v1/self", "/api/scim/v2/Users"},
			blockPaths:  []string{"/api/v1/session", "/api/v1/customer-signals"},
			wantSchemes: []string{"bearerAuth", "m2mBearerAuth"},
			blockSchemes: []string{"cookieAuth"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := NewOpenAPIExport(cfg, tt.surface)
			if err != nil {
				t.Fatal(err)
			}

			for _, path := range tt.wantPaths {
				if _, ok := spec.Paths[path]; !ok {
					t.Fatalf("missing path %s", path)
				}
			}
			for _, path := range tt.blockPaths {
				if _, ok := spec.Paths[path]; ok {
					t.Fatalf("unexpected path %s", path)
				}
			}
			for _, scheme := range tt.wantSchemes {
				if spec.Components.SecuritySchemes[scheme] == nil {
					t.Fatalf("missing security scheme %s", scheme)
				}
			}
			for _, scheme := range tt.blockSchemes {
				if spec.Components.SecuritySchemes[scheme] != nil {
					t.Fatalf("unexpected security scheme %s", scheme)
				}
			}
		})
	}
}
```

この実装では nil DB の service stub を `openAPIExportDependencies` として `backend/internal/app` 側に寄せ、command と test が同じ export helper を使うようにします。

最低限、次を test で守ります。

- browser surface は `/api/v1/*` を含む
- browser surface は external / M2M / SCIM path を含まない
- browser surface は `cookieAuth` だけを持つ
- external surface は external / M2M / SCIM path を含む
- external surface は `/api/v1/*` を含まない
- external surface は `bearerAuth` / `m2mBearerAuth` を持ち、`cookieAuth` を持たない

## 生成と確認

### 生成物を更新する

```bash
make gen
```

期待する生成物は次です。

- `openapi/openapi.yaml`
- `openapi/browser.yaml`
- `openapi/external.yaml`
- `frontend/src/api/generated/*`
- `backend/internal/db/*`

`backend/internal/db/*` は sqlc 由来です。P8 では DB query を変えないため、通常は差分なしです。

### boundary check

```bash
test -s openapi/openapi.yaml
test -s openapi/browser.yaml
test -s openapi/external.yaml

grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/openapi.yaml
grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/browser.yaml
grep -Eq '^openapi: "?(3\.1|3\.1\.)' openapi/external.yaml

! rg '/api/external/|/api/m2m/|/api/scim/' openapi/browser.yaml
! rg 'bearerAuth|m2mBearerAuth' openapi/browser.yaml
! rg '/api/v1/' openapi/external.yaml
! rg 'cookieAuth' openapi/external.yaml
```

`! rg` は shell によって見え方が変わることがあります。CI では `grep -Eq` と `! grep -Eq` に寄せると安定します。

### backend / frontend

```bash
go test ./backend/...
npm --prefix frontend run build
make binary
```

frontend build は `browser.yaml` から生成された SDK で通る必要があります。ここで external operation を直接 import していた frontend code があれば、P8 の目的に反するので削除します。

### generated drift

```bash
git diff --exit-code -- openapi/openapi.yaml openapi/browser.yaml openapi/external.yaml frontend/src/api/generated backend/internal/db
```

差分がある場合は、生成物を commit 対象に含めます。generated SDK や OpenAPI YAML を手で直して差分を消してはいけません。

### runtime smoke

P8 後も runtime `/openapi.yaml` は full canonical spec を返します。

既存 server を起動します。

```bash
make backend-dev
```

別 terminal で確認します。

```bash
curl -fsS http://127.0.0.1:8080/openapi.yaml -o /tmp/haohao-runtime-openapi.yaml
grep -q "openapi: 3.1.0" /tmp/haohao-runtime-openapi.yaml
grep -q "/api/external/v1/me:" /tmp/haohao-runtime-openapi.yaml
grep -q "/api/v1/session:" /tmp/haohao-runtime-openapi.yaml
```

single binary でも確認します。

```bash
make binary
./bin/haohao
```

別 terminal で同じ `curl` を実行します。

## 実装時の注意点

### generated spec を手で編集しない

`openapi/openapi.yaml`、`openapi/browser.yaml`、`openapi/external.yaml` は source control に置きますが、手書きの正本ではありません。

修正したい場合は次のどれかを直します。

- Huma operation の `Path`、`Tags`、`Security`
- `RegisterSurface` の分類
- `humaConfigForSurface` の security scheme
- `cmd/openapi` の generation option

### `Register` の既存互換を壊さない

runtime app は既存の `backendapi.Register(api, deps)` を使っています。P8 ではこの関数を消さず、内部で `RegisterSurface(..., SurfaceFull)` を呼ぶ形にします。

これにより、runtime behavior と `/docs`、`/openapi.yaml` は従来通り full surface になります。

### security scheme は surface ごとに最小化する

browser spec に `bearerAuth` が残っていると、frontend SDK 利用者から見ると外部 API も browser から呼べるように見えます。

external spec に `cookieAuth` が残っていると、外部 client 向け docs に browser Cookie session の前提が混ざります。

security scheme は operation の混入と同じくらい重要な境界です。

### path prefix だけに頼りすぎない

P8 の分類は基本的に path prefix で十分です。ただし、HaoHao では delegated integration のように「名前は外部連携だが browser Cookie flow」の API があります。

そのため、実装では `registerExternalRoutes`、`registerM2MRoutes`、`registerSCIMRoutes` を external group として明示的に分けます。単純に `/api/v1` 以外を external とするだけでは、将来の例外に弱くなります。

### tags は docs の整理に使う

分割後も tags は残します。`browser.yaml` では `session`、`tenants`、`tenant-admin`、`customer-signals` などで docs を整理できます。`external.yaml` では `external`、`m2m`、`scim` が docs の主な分類になります。

operation tag を消して分割だけに頼ると、docs UI で探しにくくなります。

### raw Gin route は OpenAPI に出ない

Huma に登録していない route は OpenAPI に出ません。P8 は「現在 OpenAPI に出ている Huma operation」を audience 別に分ける作業です。

raw Gin route を OpenAPI に載せる必要がある場合は、その route を Huma operation として表現できるかを先に検討します。Huma で表現しにくい file streaming などは、別途 docs または custom OpenAPI injection を設計します。

### release artifact の意味を明確にする

release に 3 spec を載せる場合、利用者向けの意味を README や release note で明確にします。

- `openapi/openapi.yaml`: internal full docs / compatibility
- `openapi/browser.yaml`: frontend SDK generation
- `openapi/external.yaml`: external client integration

外部連携先に full spec を渡すと、browser-only API まで公開 contract のように見えてしまいます。外部連携には `external.yaml` を渡します。

## よくある詰まりどころ

### `browser.yaml` に `bearerAuth` が残る

`humaConfigForSurface` が surface を見ずにすべての security scheme を入れている可能性があります。

`SurfaceBrowser` の場合は `cookieAuth` だけを入れます。`bearerAuth` と `m2mBearerAuth` は `SurfaceFull` または `SurfaceExternal` の場合だけ入れます。

### `external.yaml` に `/api/v1/session` が残る

`RegisterSurface` で `registerSessionRoutes` が external 側にも呼ばれている可能性があります。

`registerSessionRoutes`、`registerTenantRoutes`、`registerTenantAdminRoutes` など `/api/v1/*` の browser API は `includeBrowser(surface)` の中だけで呼びます。

### frontend SDK から external operation が消えない

`frontend/openapi-ts.config.ts` がまだ `../openapi/openapi.yaml` を見ている可能性があります。

`input` を `../openapi/browser.yaml` に変更し、`make gen` を実行します。generated SDK を手で消すのではなく、generator の入力を変えて再生成します。

### `/openapi.yaml` が browser spec になってしまう

runtime app が `SurfaceBrowser` で Huma config / operation registration されている可能性があります。

runtime は `SurfaceFull` のままにします。surface を切り替えるのは `backend/cmd/openapi -surface=...` と export helper だけです。

### SCIM base path が test と runtime でずれる

SCIM path は `cfg.SCIMBasePath` に依存します。OpenAPI export 用の `config.Load()` が `.env` を読むため、local `.env` の値によって generated YAML が揺れないようにします。

もし drift が起きる場合は、OpenAPI export で使う default config を固定するか、CI と local `.env.example` の `SCIM_BASE_PATH` を揃えます。

## 最終確認チェックリスト

- [ ] `backend/internal/api` に `SurfaceFull` / `SurfaceBrowser` / `SurfaceExternal` がある
- [ ] `backend/internal/api.Register` は full surface の既存互換 wrapper として残っている
- [ ] `backend/internal/api.RegisterSurface` が browser group と external group を明示的に分けている
- [ ] runtime app は `SurfaceFull` を使っている
- [ ] OpenAPI export helper は surface を受け取れる
- [ ] `backend/cmd/openapi` は `-surface=full|browser|external` を受け取れる
- [ ] `go run ./backend/cmd/openapi` は従来通り full spec を stdout に出す
- [ ] `Makefile` の `openapi` target が 3 spec を生成する
- [ ] `scripts/gen.sh` が 3 spec を生成してから frontend SDK を生成する
- [ ] `frontend/openapi-ts.config.ts` は `../openapi/browser.yaml` を入力にしている
- [ ] CI の generated drift check に 3 spec が含まれている
- [ ] CI の OpenAPI validate が 3 spec を確認している
- [ ] CI に browser / external の boundary check がある
- [ ] release workflow が必要な OpenAPI artifact を upload している
- [ ] `openapi/browser.yaml` に `/api/external/`、`/api/m2m/`、`/api/scim/` がない
- [ ] `openapi/browser.yaml` に `bearerAuth`、`m2mBearerAuth` がない
- [ ] `openapi/external.yaml` に `/api/v1/` がない
- [ ] `openapi/external.yaml` に `cookieAuth` がない
- [ ] `make gen` が通る
- [ ] `go test ./backend/...` が通る
- [ ] `npm --prefix frontend run build` が通る
- [ ] `make binary` が通る
- [ ] runtime `/openapi.yaml` は full canonical spec を返す

## ここまでで何ができているか

P8 が終わると、HaoHao の OpenAPI artifact は利用者別に分かれます。

frontend は `browser.yaml` だけを見て SDK を生成するため、external bearer、M2M、SCIM の operation が browser app に混ざりません。

外部連携先には `external.yaml` を渡せるため、Cookie session 前提の browser API を公開 contract と誤解されにくくなります。

一方で `openapi/openapi.yaml` は full canonical spec として残るため、runtime `/openapi.yaml`、全体 docs、release asset、drift check の互換性は維持できます。

この状態にした上で、次の段階では P7 で増えた browser flow を Playwright E2E で確認すると、Login、tenant 選択、Customer Signals、file upload、Tenant Admin settings / export までの回帰検知を frontend SDK の境界込みで固められます。
