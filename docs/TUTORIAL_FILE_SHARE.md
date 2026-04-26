# ファイル共有サービスを実装に落とすチュートリアル

## この文書の目的

この文書は、`FILE_SHARE_SPEC.md` に書かれたファイル共有サービス要件を、現在の HaoHao repository で実装できる順番へ変換したチュートリアルです。

目的は仕様の再掲ではありません。目的は、**ファイル共有サービスを作るときに、どの基盤を使い、どの順番で実装し、どの確認を通してから次へ進むか**を迷わず追えるようにすることです。

この文書では次を守ります。

- `FILE_SHARE_SPEC.md` を要件の正本として扱う
- 認証、tenant、session、OpenAPI、single binary の既存基盤を作り直さない
- Drive metadata と policy は PostgreSQL を source of truth にする
- resource authorization は OpenFGA に閉じる
- API は Huma / OpenAPI から生成 client へ流す
- frontend build artifact と generated code は直接編集しない

## この文書が前提にしている現在地

この repository は、少なくとも次の状態にある前提で進めます。

- `docs/TUTORIAL.md` の foundation が実装済み
- `docs/TUTORIAL_SINGLE_BINARY.md` の単一バイナリ配信が実装済み
- local password login と Zitadel OIDC browser login がある
- PostgreSQL / Redis / OpenFGA を `compose.yaml` で起動できる
- tenant-aware auth、tenant admin、audit、metrics、readiness がある
- `file_objects` と local file storage の基盤がある
- OpenAPI は full / browser / external に分割済み
- `docs/TUTORIAL_OPENFGA.md` と OpenFGA Drive Phase 1-9 の実装手順がある
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary` が通る

未実装の外部 provider 連携は、最初から本物の vendor API に直結しません。Office、eDiscovery、HSM、on-prem gateway、AI、marketplace、object storage、virus scan などは、local fake / interface / policy guard を先に作り、production provider は別 PR で差し替えます。

## 完成条件

### MVP の完成条件

MVP では、`FILE_SHARE_SPEC.md` の「21. MVPで実装すべき機能」を browser API と Vue UI から一通り使える状態にします。

- tenant 内で Drive workspace / folder / file を作成できる
- file upload / download ができる
- folder 階層と breadcrumbs がある
- Owner / Editor / Viewer を扱える
- file / folder 単位で user share と group share ができる
- folder から child folder / file へ権限が継承される
- inheritance stop / resume ができる
- share link を作成、無効化できる
- share link に有効期限と download 禁止を設定できる
- soft delete、restore、complete delete の guard がある
- search が actor の閲覧権限で filter される
- `drive.*` audit event が残る
- OpenFGA 障害時は fail-closed になる
- Vue の Drive 画面、share dialog、permissions panel、group UI がある
- `make smoke-openfga` と `make e2e` が通る

### 拡張完了条件

拡張段階では、`FILE_SHARE_SPEC.md` の後続フェーズと enterprise 要件を product surface として扱います。

- 外部ユーザー共有、未登録メール招待、domain allow / deny、管理者承認を扱える
- password 付き share link を扱える
- admin UI で共有状態、policy、audit、break-glass を確認できる
- OpenFGA tuple drift を検出し、repair できる
- workspace、scan / DLP、plan enforcement、storage consistency がある
- full text search、collaboration lock、sync / mobile offline の土台がある
- CMK、data residency、legal discovery、clean room の境界がある
- Office、eDiscovery provider、HSM、on-prem gateway、E2EE、AI、marketplace は fake provider と guard 付きで運用確認できる
- single binary で API / UI / OpenAPI / public share route を返せる

## 仕様との対応表

| `FILE_SHARE_SPEC.md` | HaoHao での責務 | 主な参照チュートリアル |
| --- | --- | --- |
| 2. ユーザー・組織管理 | Zitadel、local login、tenant、membership、group mapping | `docs/TUTORIAL.md`, `docs/TUTORIAL_ZITADEL.md`, `docs/TUTORIAL_P5_TENANT_ADMIN_UI.md` |
| 3. ファイル管理 | `file_objects`、Drive upload/download、metadata、storage driver | `docs/TUTORIAL_OPENFGA_P2_DB_SQLC.md`, `docs/TUTORIAL_OPENFGA_P3_BACKEND_SERVICES.md`, `docs/TUTORIAL_OPENFGA_P4_API_AUDIT_SMOKE.md` |
| 4. フォルダ管理 | `drive_folders`、parent relation、cycle guard、workspace | `docs/TUTORIAL_OPENFGA_P2_DB_SQLC.md`, `docs/TUTORIAL_OPENFGA_P7_ADVANCED_DRIVE_OPERATIONS.md` |
| 5. 権限管理 | OpenFGA model、Owner / Editor / Viewer、inheritance | `docs/TUTORIAL_OPENFGA_P1_INFRA_MODEL.md`, `docs/TUTORIAL_OPENFGA_P3_BACKEND_SERVICES.md` |
| 6. 共有機能 | user share、group share、external invitation、share revoke | `docs/TUTORIAL_OPENFGA_P4_API_AUDIT_SMOKE.md`, `docs/TUTORIAL_OPENFGA_P6_REMAINING_TASKS.md` |
| 7. リンク共有 | share link、password、TTL、download guard、public route | `docs/TUTORIAL_OPENFGA_P4_API_AUDIT_SMOKE.md`, `docs/TUTORIAL_OPENFGA_P6_REMAINING_TASKS.md` |
| 8. プレビュー | provider 境界、derived content policy | `docs/TUTORIAL_OPENFGA_P9_DRIVE_PRODUCT_COMPLETION.md` |
| 9. バージョン管理 | file revision、Office compatibility、retention guard | `docs/TUTORIAL_OPENFGA_P7_ADVANCED_DRIVE_OPERATIONS.md`, `docs/TUTORIAL_OPENFGA_P9_DRIVE_PRODUCT_COMPLETION.md` |
| 10. ゴミ箱・復元 | soft delete、restore、complete delete、physical purge | `docs/TUTORIAL_P12_FILE_LIFECYCLE_PHYSICAL_DELETE.md`, `docs/TUTORIAL_OPENFGA_P6_REMAINING_TASKS.md` |
| 11. 検索・一覧 | Drive list/search、OpenFGA filter、search index | `docs/TUTORIAL_OPENFGA_P5_UI_E2E.md`, `docs/TUTORIAL_OPENFGA_P8_DRIVE_PRODUCT_EXPANSION.md` |
| 12. 監査ログ | `drive.*` audit、metadata scrub、admin audit UI | `docs/TUTORIAL_P3_AUDIT_LOG.md`, `docs/TUTORIAL_OPENFGA_P4_API_AUDIT_SMOKE.md` |
| 13. 組織ポリシー | tenant drive policy、share policy、download policy | `docs/TUTORIAL_OPENFGA_P2_DB_SQLC.md`, `docs/TUTORIAL_OPENFGA_P6_REMAINING_TASKS.md` |
| 14. 通知 | invitation、notification、outbox | `docs/TUTORIAL_P7_WEB_SERVICE_COMMON.md`, `docs/TUTORIAL_OPENFGA_P6_REMAINING_TASKS.md` |
| 15. コメント・共同作業 | lock、edit session、presence、collaboration provider | `docs/TUTORIAL_OPENFGA_P8_DRIVE_PRODUCT_EXPANSION.md` |
| 16. ストレージ管理 | local / object storage driver、quota、storage consistency | `docs/TUTORIAL_OPENFGA_P7_ADVANCED_DRIVE_OPERATIONS.md` |
| 17. セキュリティ | fail-closed、DLP、CMK、E2EE、zero-knowledge guard | `docs/TUTORIAL_OPENFGA_P8_DRIVE_PRODUCT_EXPANSION.md`, `docs/TUTORIAL_OPENFGA_P9_DRIVE_PRODUCT_COMPLETION.md` |
| 18. 管理者機能 | tenant admin UI、break-glass、legal discovery | `docs/TUTORIAL_OPENFGA_P6_REMAINING_TASKS.md`, `docs/TUTORIAL_OPENFGA_P8_DRIVE_PRODUCT_EXPANSION.md` |
| 19. API要件 | `/api/v1/drive/...`、public link、external Drive surface | `docs/TUTORIAL_OPENFGA_P4_API_AUDIT_SMOKE.md`, `docs/TUTORIAL_OPENFGA_P7_ADVANCED_DRIVE_OPERATIONS.md` |
| 20. 推奨アーキテクチャ | Zitadel / OpenFGA / PostgreSQL / storage / backend / frontend の分離 | `docs/TUTORIAL_OPENFGA.md` |

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | 既存基盤 | auth、tenant、file、OpenAPI、single binary の入口を確認する |
| Step 2 | DB / sqlc / migration | Drive metadata、share、link、policy、audit の正本を固める |
| Step 3 | OpenFGA model / bootstrap | resource authorization model を repo 管理し、明示 bootstrap する |
| Step 4 | backend service / API / audit | DriveService、DriveAuthorizationService、browser API、audit を接続する |
| Step 5 | frontend Drive UI | generated SDK wrapper から Drive browser と共有 UI を実装する |
| Step 6 | 共有拡張 | password link、外部共有、domain policy、group mapping を追加する |
| Step 7 | 検索 / 履歴 / ゴミ箱 / 管理 UI | search、revision、delete guard、admin UI、repair を追加する |
| Step 8 | product expansion / provider 境界 | sync、DLP、CMK、legal、Office、AI などを fake provider から始める |
| Step 9 | single binary / smoke / E2E | 1 バイナリで API / UI / OpenAPI / public route を確認する |

## Step 1. 既存基盤を確認する

最初に、ファイル共有を載せる土台が揃っていることを確認します。

### 対象

- `docs/TUTORIAL.md`
- `docs/TUTORIAL_SINGLE_BINARY.md`
- `docs/TUTORIAL_OPENFGA.md`
- `compose.yaml`
- `.env.example`
- `Makefile`
- `openapi/openapi.yaml`
- `openapi/browser.yaml`
- `openapi/external.yaml`

### 確認すること

- PostgreSQL、Redis、OpenFGA が local runtime にある
- backend は Cookie session / CSRF / tenant-aware auth を持つ
- OpenAPI artifact は full / browser / external に分かれている
- frontend は browser spec 由来の generated SDK を使う
- single binary build は `backend/web/dist/` を embed できる

### 実行コマンド

```bash
make db-up
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
```

ここで失敗した場合、Drive 実装へ進まず、foundation / single binary / OpenAPI split の崩れを先に直します。

## Step 2. DB / sqlc / migration を固める

ファイル共有の状態は PostgreSQL を source of truth にします。OpenFGA は relation graph を持ちますが、metadata store ではありません。

### 対象

- `db/migrations/*.sql`
- `db/queries/drive_*.sql`
- `db/schema.sql`
- `backend/sqlc.yaml`
- `backend/internal/db/*`

### 実装方針

MVP では次を DB に持たせます。

- Drive file metadata
- folder hierarchy
- group と group member
- user / group / external share
- share link と token hash
- tenant drive policy
- soft delete / restore / retention / legal hold guard
- search index と file revision

既存の `file_objects` は attachment / avatar / import / export でも使います。Drive API は `purpose = 'drive'` の row だけを扱い、既存 file flow を壊しません。

### 参照する手順

- `docs/TUTORIAL_OPENFGA_P2_DB_SQLC.md`
- `docs/TUTORIAL_OPENFGA_P6_REMAINING_TASKS.md`
- `docs/TUTORIAL_OPENFGA_P7_ADVANCED_DRIVE_OPERATIONS.md`
- `docs/TUTORIAL_OPENFGA_P8_DRIVE_PRODUCT_EXPANSION.md`
- `docs/TUTORIAL_OPENFGA_P9_DRIVE_PRODUCT_COMPLETION.md`

### 確認コマンド

```bash
make db-up
make db-schema
make gen
go test ./backend/...
```

`db/schema.sql` と `backend/internal/db/*` は生成物です。直接直さず、migration と query を直して再生成します。

## Step 3. OpenFGA model / bootstrap を用意する

Drive の resource authorization は OpenFGA に閉じます。Zitadel claim や global role に、file / folder 単位の permission を持たせません。

### 対象

- `openfga/drive.fga`
- `openfga/drive.fga.yaml`
- `scripts/openfga-bootstrap.sh`
- `.env.example`
- `backend/internal/config/*`

### 実装方針

OpenFGA model は次の resource を扱います。

- `user`
- `group`
- `share_link`
- `workspace`
- `clean_room`
- `folder`
- `file`

object ID は DB の `public_id` から作ります。tenant ID は OpenFGA object ID に入れません。すべての Drive API は OpenFGA check の前に DB で active tenant と resource tenant を確認します。

authorization model を変更した場合、runtime 起動時に自動更新しません。必ず model test、bootstrap、`.env` の `OPENFGA_AUTHORIZATION_MODEL_ID` 更新を明示的に行います。

### 参照する手順

- `docs/TUTORIAL_OPENFGA_P1_INFRA_MODEL.md`
- `docs/TUTORIAL_OPENFGA.md`

### 確認コマンド

```bash
make openfga-bootstrap
make test-openfga-model
```

OpenFGA を有効にする runtime では `.env` に次が必要です。

```dotenv
OPENFGA_ENABLED=true
OPENFGA_API_URL=http://127.0.0.1:8088
OPENFGA_STORE_ID=...
OPENFGA_AUTHORIZATION_MODEL_ID=...
OPENFGA_FAIL_CLOSED=true
```

## Step 4. backend service / API / audit を接続する

DB と OpenFGA の境界が決まったら、backend service で順序を固定します。

### 対象

- `backend/internal/service/drive_*.go`
- `backend/internal/service/openfga*.go`
- `backend/internal/api/drive*.go`
- `backend/internal/api/register.go`
- `backend/internal/app/app.go`
- `scripts/smoke-openfga.sh`

### 実装方針

`DriveService` は DB state、tenant policy、file body、OpenFGA tuple の順序を管理します。`DriveAuthorizationService` は OpenFGA SDK を service 層に閉じ込め、API handler や frontend に OpenFGA の詳細を漏らしません。

操作の基本順序は次です。

1. actor の tenant membership と active tenant を確認する
2. resource が同じ tenant に存在することを DB で確認する
3. delete / retention / locked / policy guard を評価する
4. OpenFGA で action に必要な relation を check する
5. DB mutation と audit を transaction に入れる
6. tuple write / delete を行い、失敗時は pending sync または compensation する
7. audit metadata から secret、raw token、storage key を除外する

browser API は `/api/v1/drive/...` に置きます。public share link は login 前でも使う route として分け、external bearer / M2M Drive API は browser surface に混ぜません。

### 参照する手順

- `docs/TUTORIAL_OPENFGA_P3_BACKEND_SERVICES.md`
- `docs/TUTORIAL_OPENFGA_P4_API_AUDIT_SMOKE.md`
- `docs/TUTORIAL_OPENFGA_P7_ADVANCED_DRIVE_OPERATIONS.md`

### 確認コマンド

```bash
make gen
go test ./backend/...
make smoke-openfga
```

## Step 5. frontend Drive UI を実装する

UI は generated SDK を直接 view から呼ばず、Drive 用 API wrapper と Pinia store を挟みます。

### 対象

- `frontend/src/api/*`
- `frontend/src/stores/*`
- `frontend/src/views/*`
- `frontend/src/components/Drive*.vue`
- `frontend/src/router/index.ts`

### 実装方針

MVP UI では次を扱います。

- Drive 一覧
- folder 作成
- file upload / download
- rename / move / delete / restore
- breadcrumbs
- share dialog
- permissions panel
- share link 作成 / 無効化
- group 管理
- search
- tenant admin の policy / audit 入口

UI は SaaS 業務画面として、密度があり、繰り返し操作しやすい構成にします。ファイル共有サービスの第一画面は landing page ではなく、Drive browser です。

### 参照する手順

- `docs/TUTORIAL_OPENFGA_P5_UI_E2E.md`
- `docs/TUTORIAL_SINGLE_BINARY.md`

### 確認コマンド

```bash
make gen
npm --prefix frontend run build
make e2e
```

## Step 6. 共有リンク、外部共有、グループ共有を拡張する

MVP の共有が動いたら、`FILE_SHARE_SPEC.md` の共有要件を production 向けに広げます。

### 対象

- tenant drive policy
- external invitation
- domain allow / deny
- password protected share link
- Drive group external mapping
- share state admin UI
- notification / outbox

### 実装方針

外部共有は tenant membership を増やしません。未登録メールは invitation として保存し、受諾時に local user へ解決します。share link の raw token と password は保存せず、hash だけを保存します。

password 付き link、anonymous editor link、外部 domain share は default safe にします。tenant policy が許可していない場合は、UI で選択できても backend が必ず拒否します。

### 参照する手順

- `docs/TUTORIAL_OPENFGA_P6_REMAINING_TASKS.md`
- `docs/TUTORIAL_P7_WEB_SERVICE_COMMON.md`

### 確認コマンド

```bash
RUN_DRIVE_PASSWORD_LINK_SMOKE=1 \
RUN_OPENFGA_EXTERNAL_SHARE_SMOKE=1 \
RUN_DRIVE_ADMIN_CONTENT_ACCESS_SMOKE=1 \
RUN_OPENFGA_PUBLIC_EDITOR_LINK_SMOKE=1 \
make smoke-openfga
```

## Step 7. 検索、履歴、ゴミ箱、管理 UI を追加する

共有サービスは、upload と share だけでは運用できません。検索、履歴、削除、管理者監査、repair の導線を追加します。

### 対象

- Drive search index / search job
- file revision
- soft delete / restore / complete delete
- retention / legal hold
- scan / DLP state
- storage metadata
- OpenFGA tuple drift detection / repair
- tenant admin Drive UI

### 実装方針

検索結果は DB の候補取得だけで返さず、actor が閲覧できる resource だけに絞ります。retention / legal hold 対象は complete delete できません。admin の break-glass は通常の Drive UI とは分け、audit と理由入力を必須にします。

Trash / restore は通常一覧と分けます。通常の `/api/v1/drive/items` は `deleted_at IS NULL` の item だけを返し、削除済み item は `/api/v1/drive/trash` から返します。復元は `/api/v1/drive/files/{filePublicId}/restore` と `/api/v1/drive/folders/{folderPublicId}/restore` で行い、OpenFGA の owner 権限を確認したうえで、元 parent が有効なら元の場所へ、無効なら workspace root へ戻します。

### 参照する手順

- `docs/TUTORIAL_OPENFGA_P6_REMAINING_TASKS.md`
- `docs/TUTORIAL_OPENFGA_P7_ADVANCED_DRIVE_OPERATIONS.md`
- `docs/TUTORIAL_OPENFGA_P8_DRIVE_PRODUCT_EXPANSION.md`

### 確認コマンド

```bash
RUN_OPENFGA_WORKSPACE_SMOKE=1 \
RUN_DRIVE_SCAN_SMOKE=1 \
RUN_DRIVE_PLAN_ENFORCEMENT_SMOKE=1 \
RUN_DRIVE_DRIFT_SMOKE=1 \
RUN_DRIVE_STORAGE_CONSISTENCY_SMOKE=1 \
RUN_DRIVE_SEARCH_SMOKE=1 \
make smoke-openfga
```

## Step 8. product expansion と provider 境界を追加する

本格 SaaS の機能は、provider interface と local fake を先に作ってから本物の連携へ進みます。

### 対象

- collaborative editing
- desktop sync
- mobile offline
- CMK / HSM
- data residency
- legal discovery
- clean room
- Office co-authoring
- eDiscovery provider
- on-prem gateway
- E2EE
- AI classification / summarization
- marketplace

### 実装方針

provider 連携は permission source を増やしません。derived content、AI summary、Office preview、marketplace app の出力は、source file の OpenFGA authorization に従います。

zero-knowledge / E2EE を有効にする場合、server-side search、preview、AI summary など、server が plaintext を必要とする機能を policy で制限します。

### 参照する手順

- `docs/TUTORIAL_OPENFGA_P8_DRIVE_PRODUCT_EXPANSION.md`
- `docs/TUTORIAL_OPENFGA_P9_DRIVE_PRODUCT_COMPLETION.md`

### 確認コマンド

```bash
RUN_DRIVE_COLLAB_SMOKE=1 \
RUN_DRIVE_DESKTOP_SYNC_SMOKE=1 \
RUN_DRIVE_MOBILE_OFFLINE_SMOKE=1 \
RUN_DRIVE_CMK_SMOKE=1 \
RUN_DRIVE_RESIDENCY_SMOKE=1 \
RUN_DRIVE_LEGAL_DISCOVERY_SMOKE=1 \
RUN_DRIVE_CLEAN_ROOM_SMOKE=1 \
RUN_DRIVE_OFFICE_SMOKE=1 \
RUN_DRIVE_OFFICE_WEBHOOK_SMOKE=1 \
RUN_DRIVE_EDISCOVERY_PROVIDER_SMOKE=1 \
RUN_DRIVE_HSM_SMOKE=1 \
RUN_DRIVE_HSM_FAIL_CLOSED_SMOKE=1 \
RUN_DRIVE_GATEWAY_SMOKE=1 \
RUN_DRIVE_GATEWAY_DISCONNECT_SMOKE=1 \
RUN_DRIVE_AI_SMOKE=1 \
RUN_DRIVE_AI_POLICY_SMOKE=1 \
RUN_DRIVE_E2EE_SMOKE=1 \
RUN_DRIVE_E2EE_REVOKE_SMOKE=1 \
RUN_DRIVE_MARKETPLACE_SMOKE=1 \
RUN_DRIVE_MARKETPLACE_SCOPE_SMOKE=1 \
make smoke-openfga
```

## Step 9. single binary / smoke / E2E で確認する

最後に、開発サーバーではなく配布形に近い binary で確認します。

### 対象

- `backend/web/dist/*`
- `bin/haohao`
- `/api/v1/drive/...`
- `/share/:token` または public share route
- `/openapi.yaml`
- `/docs`
- `/drive`

### 実行コマンド

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
make test-openfga-model
make smoke-openfga
make e2e
```

OpenFGA enabled の binary smoke では、`.env` の store/model ID が現在の `openfga/drive.fga` と一致していることを先に確認します。

```bash
make openfga-bootstrap
```

起動例:

```bash
HTTP_PORT=18080 \
APP_BASE_URL=http://127.0.0.1:18080 \
FRONTEND_BASE_URL=http://127.0.0.1:18080 \
RATE_LIMIT_ENABLED=false \
./bin/haohao
```

別 terminal で確認します。

```bash
curl -i http://127.0.0.1:18080/readyz
curl -i http://127.0.0.1:18080/openapi.yaml
curl -i http://127.0.0.1:18080/drive
curl -i http://127.0.0.1:18080/api/v1/session
```

## 最終確認コマンド

ファイル共有の通常確認は次です。

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
make test-openfga-model
make smoke-openfga
make e2e
```

P6-P9 まで含めた product surface の smoke は次です。

```bash
RUN_DRIVE_PASSWORD_LINK_SMOKE=1 \
RUN_OPENFGA_EXTERNAL_SHARE_SMOKE=1 \
RUN_DRIVE_ADMIN_CONTENT_ACCESS_SMOKE=1 \
RUN_OPENFGA_PUBLIC_EDITOR_LINK_SMOKE=1 \
RUN_OPENFGA_WORKSPACE_SMOKE=1 \
RUN_DRIVE_SCAN_SMOKE=1 \
RUN_DRIVE_PLAN_ENFORCEMENT_SMOKE=1 \
RUN_DRIVE_DRIFT_SMOKE=1 \
RUN_DRIVE_STORAGE_CONSISTENCY_SMOKE=1 \
RUN_DRIVE_SEARCH_SMOKE=1 \
RUN_DRIVE_COLLAB_SMOKE=1 \
RUN_DRIVE_DESKTOP_SYNC_SMOKE=1 \
RUN_DRIVE_MOBILE_OFFLINE_SMOKE=1 \
RUN_DRIVE_CMK_SMOKE=1 \
RUN_DRIVE_RESIDENCY_SMOKE=1 \
RUN_DRIVE_LEGAL_DISCOVERY_SMOKE=1 \
RUN_DRIVE_CLEAN_ROOM_SMOKE=1 \
RUN_DRIVE_OFFICE_SMOKE=1 \
RUN_DRIVE_OFFICE_WEBHOOK_SMOKE=1 \
RUN_DRIVE_EDISCOVERY_PROVIDER_SMOKE=1 \
RUN_DRIVE_HSM_SMOKE=1 \
RUN_DRIVE_HSM_FAIL_CLOSED_SMOKE=1 \
RUN_DRIVE_GATEWAY_SMOKE=1 \
RUN_DRIVE_GATEWAY_DISCONNECT_SMOKE=1 \
RUN_DRIVE_AI_SMOKE=1 \
RUN_DRIVE_AI_POLICY_SMOKE=1 \
RUN_DRIVE_E2EE_SMOKE=1 \
RUN_DRIVE_E2EE_REVOKE_SMOKE=1 \
RUN_DRIVE_MARKETPLACE_SMOKE=1 \
RUN_DRIVE_MARKETPLACE_SCOPE_SMOKE=1 \
make smoke-openfga
```

rate limit が smoke の大量リクエストを妨げる場合は、一時的に `RATE_LIMIT_ENABLED=false` で binary を起動して確認します。

## 生成物として扱うファイル

このチュートリアルで増える、または更新される生成物は次です。

- `db/schema.sql`
- `backend/internal/db/*`
- `openapi/openapi.yaml`
- `openapi/browser.yaml`
- `openapi/external.yaml`
- `frontend/src/api/generated/*`
- `backend/web/dist/*`
- `bin/haohao`

これらは source ではありません。問題があれば生成物を直接直さず、次を直します。

- DB schema が違う: `db/migrations/*.sql`
- sqlc type / query が違う: `db/queries/*.sql` または `backend/sqlc.yaml`
- OpenAPI が違う: `backend/internal/api/*.go`
- frontend SDK が違う: `openapi/browser.yaml` の元になる API 定義
- UI build output が違う: `frontend/src/*` または `frontend/vite.config.ts`
- binary が違う: `backend/frontend*.go`、`backend/cmd/main/main.go`、`Makefile`

## 途中で迷わないための判断基準

### tenant 境界は DB で確認する

OpenFGA object ID に tenant ID を入れません。tenant mismatch は `404 Not Found` として扱い、存在を隠します。

### OpenFGA は resource authorization だけを担当する

login、global role、tenant role、active tenant、M2M verifier は既存 auth / authz 基盤が担当します。OpenFGA は file / folder / workspace / clean room / group / share link の relation に限定します。

### secret は audit と log に残さない

share link raw token、password、storage key、provider credential、webhook secret、idempotency key raw value は audit metadata と log に入れません。

### provider は fake から始める

外部 provider は production credential より先に interface、fake implementation、smoke、policy guard を作ります。これにより、CI と local binary で product boundary を検証できます。

## ここまでで何ができているか

ここまで終えると、`FILE_SHARE_SPEC.md` の要件は HaoHao の既存基盤に沿って実装順へ分解されます。

- 認証と tenant 管理は既存基盤を使う
- Drive metadata は PostgreSQL / sqlc を正本にする
- file / folder / share / link の resource authorization は OpenFGA に寄せる
- backend API は Huma / OpenAPI split から generated SDK へ流す
- Vue UI は Drive browser を中心に作る
- single binary で API / docs / OpenAPI / SPA を確認できる
- advanced provider 連携は local fake と policy guard から安全に増やせる
