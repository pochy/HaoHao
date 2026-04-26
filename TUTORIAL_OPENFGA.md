# OpenFGA Drive 実装チュートリアル

## この文書の目的

この文書は、`OPENFGA_IMPLEMENTATION_PLAN.md` を、現在の HaoHao repository に実装できる順番へ分解したチュートリアルです。

目的は設計の再説明ではありません。目的は、**どの順番で、どのファイルを追加し、どの確認を通してから次へ進むか**を迷わず追えるようにすることです。

このチュートリアルでは次を守ります。

- 既存の Zitadel / AuthzService / tenant-aware auth を置き換えない
- OpenFGA は Drive file / folder / group / share link の resource authorization に限定する
- PostgreSQL を Drive metadata と policy の source of truth にする
- OpenFGA authorization model は runtime 起動時に自動更新しない
- OpenFGA 障害時は fail-closed にする
- browser surface から始め、external bearer / M2M / SCIM surface には初期導入しない

## この文書が前提にしている現在地

この repository は、少なくとも次の状態にある前提で進めます。

- PostgreSQL と Redis を `compose.yaml` で起動できる
- `users`、`tenants`、`tenant_memberships` が存在する
- local login と Zitadel OIDC browser login が存在する
- `AuthzService` が global role / tenant role / active tenant を解決できる
- Cookie session / CSRF の browser API 基盤がある
- `FileService` と `file_objects` による tenant-aware file upload/download/soft delete がある
- `AuditService`、tenant settings、metrics/readiness がある
- OpenAPI full/browser/external split がある
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary` が既存機能で通る

## 完成条件

OpenFGA Drive 導入の完了条件は次です。

- `compose.yaml` で OpenFGA と専用 PostgreSQL を起動できる
- `openfga/drive.fga` を repo で管理している
- bootstrap script で OpenFGA store / authorization model を登録できる
- backend config に OpenFGA 設定が入り、`OPENFGA_ENABLED=true` で接続できる
- Drive file / folder / group / share link 用 DB schema と sqlc query がある
- `DriveAuthorizationService` が OpenFGA SDK を service 層に閉じ込めている
- `DriveService` が DB state、tenant policy、OpenFGA check/write/delete を正しい順序で扱う
- `/api/v1/drive/...` が browser API として追加される
- `tenant_admin` は shared policy と audit を扱えるが、ファイル本文閲覧権限は持たない
- Vue に Drive browser、share dialog、permissions panel、group/share link UI がある
- audit に `drive.*` event が残る
- metrics/readiness が OpenFGA を観測できる
- `make smoke-openfga` と `make e2e` が通る

## Phase と主題の対応表

OpenFGA Drive は範囲が広いため、フェーズ別ファイルに分割します。

| Phase | 文書 | 主題 |
| --- | --- | --- |
| Phase 1 | `TUTORIAL_OPENFGA_P1_INFRA_MODEL.md` | OpenFGA compose、config、model、bootstrap |
| Phase 2 | `TUTORIAL_OPENFGA_P2_DB_SQLC.md` | Drive DB schema、sqlc query、tenant policy |
| Phase 3 | `TUTORIAL_OPENFGA_P3_BACKEND_SERVICES.md` | OpenFGA client wrapper、DriveAuthorizationService、DriveService |
| Phase 4 | `TUTORIAL_OPENFGA_P4_API_AUDIT_SMOKE.md` | Drive API、audit、metrics、readiness、smoke |
| Phase 5 | `TUTORIAL_OPENFGA_P5_UI_E2E.md` | Vue Drive UI、tenant admin 導線、E2E |

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | `compose.yaml`, `.env.example`, `backend/internal/config/*` | OpenFGA を local runtime と backend config に接続できるようにする |
| Step 2 | `openfga/drive.fga`, `scripts/openfga-bootstrap.sh` | authorization model を repo 管理し、明示 bootstrap できるようにする |
| Step 3 | `db/migrations/*`, `db/queries/*`, `db/schema.sql` | Drive metadata、share、group、link、policy を DB の正本にする |
| Step 4 | `backend/internal/service/openfga*`, `backend/internal/service/drive_*` | OpenFGA access と Drive domain logic を service 層に閉じる |
| Step 5 | `backend/internal/api/drive*.go`, `backend/internal/api/register.go` | browser API と raw upload/download route を追加する |
| Step 6 | audit / metrics / readiness / smoke | 運用時に壊れ方を見えるようにする |
| Step 7 | `frontend/src/api/*`, `frontend/src/stores/*`, `frontend/src/views/*` | Drive UI を generated SDK wrapper 経由で実装する |
| Step 8 | tests / E2E / final build | fail-closed、継承、共有、リンク、UI を確認する |

## 先に決める方針

### Zitadel は resource permission を持たない

Zitadel は user authentication、OIDC login、bearer token issuer、provider claim、SCIM/M2M の入口を担当します。

Drive file / folder の Owner / Editor / Viewer、folder inheritance、user/group/share link 共有は OpenFGA と HaoHao DB の担当です。Zitadel claim に `drive:folder:*` のような resource permission を入れません。

### OpenFGA user は local user public ID に正規化する

OpenFGA tuple には Zitadel subject を直接入れません。

```text
Zitadel subject
  -> user_identities(provider='zitadel', subject)
  -> users.public_id
  -> OpenFGA user:<users.public_id>
```

これにより、local password user と Zitadel user を同じ Drive permission model で扱えます。

### tenant 境界は DB で先に確認する

OpenFGA object ID には tenant ID を埋め込みません。

すべての Drive API は OpenFGA check の前に DB で次を確認します。

- active tenant が存在する
- actor が active tenant の member である
- resource / group / share / link の `tenant_id` が active tenant と一致する
- resource が deleted ではない
- locked / read-only / retention / tenant policy に反していない

tenant mismatch は `404 Not Found` を返して存在を隠します。

### 生成物と手書きファイルを分ける

手書きする主な正本:

- `openfga/drive.fga`
- `scripts/openfga-bootstrap.sh`
- `db/migrations/*.sql`
- `db/queries/*.sql`
- `backend/internal/service/*.go`
- `backend/internal/api/*.go`
- `frontend/src/api/*.ts`
- `frontend/src/stores/*.ts`
- `frontend/src/views/*.vue`
- `frontend/src/components/*.vue`
- `scripts/smoke-openfga.sh`

生成物:

- `db/schema.sql`
- `backend/internal/db/*`
- `openapi/openapi.yaml`
- `openapi/browser.yaml`
- `openapi/external.yaml`
- `frontend/src/api/generated/*`
- `backend/web/dist/*`

生成物は直接編集しません。`make gen` または既存の生成コマンドで更新します。

## 最終確認コマンド

全フェーズ完了後は次を通します。

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
make smoke-openfga
make e2e
```

OpenFGA を有効にした runtime 確認では `.env` に次が入っていることを確認します。

```dotenv
OPENFGA_ENABLED=true
OPENFGA_API_URL=http://127.0.0.1:8088
OPENFGA_STORE_ID=...
OPENFGA_AUTHORIZATION_MODEL_ID=...
OPENFGA_FAIL_CLOSED=true
```

## このチュートリアルで初期導入しないもの

次は `DRIVE_OPENFGA_PERMISSIONS_SPEC.md` の要件として残しますが、初期 OpenFGA Drive 導入では実装しません。

- 外部ユーザーへの直接共有
- 未登録メールアドレス招待
- パスワード付き share link
- domain allow / deny
- 管理者承認フロー
- explicit deny
- Owner 移譲
- 完全削除
- restore
- legal hold / retention policy UI
- Commenter / Uploader role
- workspace table の新設
- external bearer / M2M からの Drive 操作

