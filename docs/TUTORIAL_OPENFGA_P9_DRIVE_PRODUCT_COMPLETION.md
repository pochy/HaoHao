# Phase 9: OpenFGA Drive プロダクト完成チュートリアル

## この文書の目的

この文書は、`TUTORIAL_OPENFGA_P8_DRIVE_PRODUCT_EXPANSION.md` の末尾で Phase 8 の外に置いた Drive product 課題を、実装順に落とし込むためのチュートリアルです。

対象は次の 7 項目です。

- native Office file co-authoring compatibility
- enterprise eDiscovery provider integration
- customer-managed HSM dedicated deployment
- on-premise storage gateway
- end-to-end encryption with zero-knowledge sharing
- AI-assisted document classification and summarization
- public marketplace integration for Drive apps

この文書では、これらを別 Phase や別 roadmap に残しません。各項目を、DB、backend service、API、OpenFGA、frontend、audit、smoke、rollback の単位まで分解します。

## この文書が前提にしている現在地

このチュートリアルは、Phase 1-8 が完了している状態から始めます。

- Drive file / folder / group / share link / workspace の基本認可は OpenFGA で判定している
- tenant boundary、resource state、tenant policy、plan、scan state、DLP state は OpenFGA check の前に DB で確認している
- object storage driver、signed URL、upload state、storage consistency check がある
- full text search、collaborative editing 境界、desktop sync、mobile offline sync、CMK、data residency、legal discovery、clean room が feature flag と smoke 付きで導入済み
- legal discovery と clean room は通常 Drive UX とは分離されている
- CMK は raw key material を扱わず、key unavailable / disabled / deleted を fail-closed として扱う
- `openfga/drive.fga` の authorization model は runtime 起動時に自動更新しない
- OpenFGA の tuple は DB から再構築できる派生 state として扱う
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make smoke-openfga` が通る

## 完成条件

このチュートリアルの完了条件は次です。

- Office provider adapter と local fake adapter があり、`.docx` / `.xlsx` / `.pptx` を Drive 上で共同編集できる
- Office editing session は DB guard と OpenFGA `can_view` / `can_edit` を通った場合だけ発行される
- provider webhook は権限 source にならず、revision / checksum / actor mapping を検証してから取り込まれる
- enterprise eDiscovery provider に legal hold / case export / chain of custody manifest を送信できる
- eDiscovery provider export は dedicated role、approval、audit、retention policy を通る
- tenant ごとの dedicated HSM endpoint / key binding / attestation / health check がある
- HSM unavailable、key disabled、key destroyed は fail-closed になり、plaintext key material は repository、DB、log、audit に残らない
- on-premise storage gateway は mTLS と signed manifest で backend と通信し、gateway から DB / OpenFGA へ直接接続しない
- E2EE file は server が plaintext と content encryption key を持たず、sharing は recipient key envelope で行う
- zero-knowledge mode では server-side search、AI summary、DLP content scan、provider preview が plaintext を要求しない形に制限される
- AI classification / summarization は tenant opt-in、DLP 後、response-time authorization、provider retention policy 付きで動く
- marketplace app は review / install / scope / approval / webhook / audit / uninstall まで実装される
- marketplace app の scope は OpenFGA 権限を拡張せず、request 時に DB guard と OpenFGA check を必ず通る
- frontend には Office edit、eDiscovery export、HSM status、gateway status、E2EE sharing、AI labels / summary、marketplace install / manage の画面がある
- `IMPL.md` に全 feature flag、provider adapter、env、rollback、smoke、operational drill が記録されている
- 全 smoke env を明示した確認コマンドが通る

## 実装順の全体像

| Step | 主題 | 主な対象ファイル | この Step の目的 |
| --- | --- | --- | --- |
| Step 1 | Office co-authoring | `drive_office_*`, `DriveOfficeService`, `DriveOfficeEditorView.vue` | Office provider と Drive permission を接続する |
| Step 2 | enterprise eDiscovery provider | `drive_ediscovery_*`, `DriveEDiscoveryService`, admin UI | legal hold / export を外部 provider へ送る |
| Step 3 | dedicated HSM deployment | `drive_hsm_*`, `HSMClient`, tenant security UI | tenant 専用 HSM key を Drive encryption に接続する |
| Step 4 | on-premise storage gateway | `drive_storage_gateways`, `DriveGatewayService`, gateway binary | customer network 内 storage を Drive driver として扱う |
| Step 5 | E2EE zero-knowledge sharing | `drive_e2ee_*`, WebCrypto, E2EE share UI | server が復号できない Drive file と共有を作る |
| Step 6 | AI classification / summarization | `drive_ai_*`, `DriveAIService`, AI panel | content 派生情報を policy と authz 付きで作る |
| Step 7 | public marketplace | `drive_marketplace_*`, app review / install UI | third-party Drive app の導入面を作る |
| Step 8 | final verification | smoke / E2E / runbook / `IMPL.md` | 全 feature を同じ品質 gate で閉じる |

## 推奨 PR 分割

各 PR は、DB migration、SQL、service、API、OpenAPI、frontend、audit、metrics、smoke、rollback note を同時に含めます。schema だけ、UI だけ、provider adapter だけの PR にはしません。

| PR | 対象 | merge 条件 |
| --- | --- | --- |
| 1 | Office co-authoring | local fake provider で共同編集 session と webhook smoke が通る |
| 2 | eDiscovery provider | legal hold export と manifest verification smoke が通る |
| 3 | dedicated HSM | fake HSM と unavailable / disabled key の fail-closed smoke が通る |
| 4 | on-prem gateway | fake gateway の upload / download / disconnect smoke が通る |
| 5 | E2EE zero-knowledge | browser E2EE upload / share / revoke / rotate smoke が通る |
| 6 | AI classification / summary | fake AI provider、DLP block、revoked access block の smoke が通る |
| 7 | marketplace | fake app install / webhook / uninstall / denied scope smoke が通る |
| 8 | final hardening | all flags enabled の integration smoke と operational drill が通る |

### この repository での最小実装メモ

今回の Phase 9 実装では、外部 SaaS / customer network / HSM / AI provider を直接導入せず、local fake と DB-backed MVP で provider 境界を固定します。

- Office: fake provider file / session / webhook を DB に持ち、`.docx` / `.xlsx` / `.pptx` の session 発行と revision webhook dedupe / stale reject を確認する。
- eDiscovery: Phase 8 の legal case / hold を provider export table に接続し、request user と approver を分けた fake manifest export にする。
- HSM: key material は扱わず、deployment / key / binding / health status と download fail-closed guard を実装する。
- on-prem gateway: gateway binary は作らず、gateway registration / object manifest / disconnect fail-closed guard を API と smoke で固定する。
- E2EE: WebCrypto UI は将来実装とし、server は public key / file key metadata / recipient envelope だけを保存する。smoke では ciphertext を opaque payload として扱う。
- AI: fake provider が deterministic summary / classification を返し、保存済み result の read 時にも source file の DB guard と OpenFGA check を必ず通す。
- marketplace: fake reviewed app を seed し、install approval、scope upper bound、OpenFGA check、uninstall を確認する。

このため、Phase 9 の frontend は tenant admin Drive policy 上の feature flag 露出までを merge gate とし、provider-specific rich UI と Playwright E2E は実 provider / browser crypto を固定する段階で追加します。通常確認は `RUN_DRIVE_*_SMOKE=1` の明示 smoke を正とします。

## 先に決める方針

### Permission source は増やさない

Phase 9 では外部 provider が増えますが、permission source は増やしません。

- DB: tenant、workspace、file state、policy、provider connection、feature flag、approval state
- OpenFGA: Drive resource relation、workspace relation、group relation、share relation
- provider: content editing、export transfer、storage operation、AI inference、app webhook delivery

provider の session、webhook、callback、cursor、manifest、app scope は、OpenFGA の代わりになりません。API は必ず DB guard を通してから OpenFGA `Check` または `BatchCheck` を行います。

### External provider は local fake を必ず持つ

CI では external SaaS、customer HSM、on-prem network、AI provider に依存しません。各 provider interface には local fake を実装します。

実 provider の確認は operational drill として分けます。

```bash
RUN_DRIVE_OFFICE_PROVIDER_DRILL=1 make smoke-openfga
RUN_DRIVE_EDISCOVERY_PROVIDER_DRILL=1 make smoke-openfga
RUN_DRIVE_HSM_PROVIDER_DRILL=1 make smoke-openfga
RUN_DRIVE_GATEWAY_PROVIDER_DRILL=1 make smoke-openfga
RUN_DRIVE_AI_PROVIDER_DRILL=1 make smoke-openfga
RUN_DRIVE_MARKETPLACE_PROVIDER_DRILL=1 make smoke-openfga
```

### Zero-knowledge mode は product capability を制限する

E2EE zero-knowledge file では、server が plaintext を読めません。そのため、次の機能は plaintext 前提で動かしません。

- server-side full text index
- server-side AI summary
- server-side DLP content scan
- Office provider preview / co-authoring
- legal discovery content export

これらを使う場合は、tenant / workspace / file の policy で明示的に non-zero-knowledge mode を選ばせます。zero-knowledge と表示する tenant では、server escrow や provider-side plaintext processing を混ぜません。

### Derived content は source file の権限に従う

AI summary、classification label、Office provider preview、eDiscovery export manifest、marketplace app generated artifact は、source file から派生した content として扱います。

保存するときだけではなく、読むときにも source file の DB guard と OpenFGA check を行います。source file の access が消えた actor には、古い summary や exported preview を返しません。

## Step 1. native Office file co-authoring compatibility を実装する

### 対象 subsystem

```text
db/migrations/0019_openfga_drive_enterprise_integrations.up.sql
db/migrations/0019_openfga_drive_enterprise_integrations.down.sql
db/queries/drive_office.sql
backend/internal/service/drive_office_service.go
backend/internal/service/drive_office_provider.go
backend/internal/service/drive_office_provider_fake.go
backend/internal/api/drive_office.go
frontend/src/views/DriveOfficeEditorView.vue
frontend/src/components/DriveOpenWithMenu.vue
frontend/src/stores/driveOffice.ts
scripts/smoke-openfga.sh
```

### DB を追加する

Office provider の状態は Drive file の権限 source ではありません。session、lock、provider revision、webhook dedupe、compatibility state を DB に持ちます。

追加する table の要点:

```sql
CREATE TABLE drive_office_provider_files (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  provider_file_id TEXT NOT NULL,
  compatibility_state TEXT NOT NULL,
  provider_revision TEXT NOT NULL,
  content_checksum TEXT,
  last_synced_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, file_object_id, provider),
  UNIQUE (provider, provider_file_id)
);

CREATE TABLE drive_office_edit_sessions (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
  actor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  provider_session_id TEXT NOT NULL,
  access_level TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drive_office_webhook_events (
  id BIGSERIAL PRIMARY KEY,
  provider TEXT NOT NULL,
  provider_event_id TEXT NOT NULL,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  file_object_id BIGINT REFERENCES file_objects(id) ON DELETE SET NULL,
  payload_hash TEXT NOT NULL,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at TIMESTAMPTZ,
  result TEXT,
  UNIQUE (provider, provider_event_id)
);
```

`file_objects` には、Office compatibility を検索しやすい最小列だけ追加します。

```sql
ALTER TABLE file_objects
  ADD COLUMN office_mime_family TEXT,
  ADD COLUMN office_coauthoring_enabled BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN office_last_revision TEXT;
```

### provider interface を追加する

#### ファイル: `backend/internal/service/drive_office_provider.go`

```go
package service

import "context"

type DriveOfficeProvider interface {
	CreateOrGetProviderFile(ctx context.Context, input DriveOfficeProviderFileInput) (DriveOfficeProviderFile, error)
	CreateEditSession(ctx context.Context, input DriveOfficeEditSessionInput) (DriveOfficeEditSession, error)
	RevokeEditSession(ctx context.Context, providerSessionID string) error
	FetchRevision(ctx context.Context, providerFileID string) (DriveOfficeRevision, error)
	VerifyWebhook(ctx context.Context, headers map[string]string, body []byte) (DriveOfficeWebhookEvent, error)
}
```

local fake は repository 内で完結させます。fake は provider file、session、revision、webhook event を memory または test DB に持ち、CI smoke で使います。

### API を追加する

browser API と provider webhook API を分けます。

| Route | 認証 | 目的 |
| --- | --- | --- |
| `POST /api/v1/drive/files/{fileId}/office/sessions` | browser session | edit / view session を作る |
| `DELETE /api/v1/drive/office/sessions/{sessionId}` | browser session | session を revoke する |
| `POST /api/office/webhooks/{provider}` | provider signature | provider revision event を受ける |

session 作成時の guard:

1. tenant feature `drive.office_coauthoring.enabled` を確認する
2. file が active で、upload / scan / DLP が完了していることを確認する
3. `.docx` / `.xlsx` / `.pptx` など provider が対応する MIME だけ許可する
4. view session は OpenFGA `can_view`
5. edit session は OpenFGA `can_edit`
6. E2EE zero-knowledge file では Office provider session を発行しない
7. session TTL は短くし、signed launch token は session TTL より長くしない

### webhook の取り込み

webhook は権限 source ではありません。次の順序で処理します。

1. provider signature を検証する
2. `provider_event_id` で dedupe する
3. `provider_file_id` から tenant / file を引く
4. provider revision が現在 revision より新しいことを確認する
5. checksum / size / MIME を検証する
6. Drive file revision と audit を更新する
7. outbox で search index、sync cursor、AI job、legal hold mirror を更新する

### frontend を追加する

- `DriveOpenWithMenu.vue`: Office 対応 file だけ `Open in Office` を表示する
- `DriveOfficeEditorView.vue`: provider launch URL を iframe で表示する場合でも、origin allowlist と CSP を固定する
- `driveOffice.ts`: session create / revoke / heartbeat を store にまとめる

通常 Drive viewer と Office editor は route を分けます。

```text
/drive/files/:fileId
/drive/files/:fileId/office
```

### audit / metrics

追加する audit action:

- `drive.office.session.create`
- `drive.office.session.revoke`
- `drive.office.webhook.accept`
- `drive.office.webhook.reject`
- `drive.office.revision.sync`

追加する metrics:

```text
drive_office_session_total{provider,result,access_level}
drive_office_webhook_total{provider,result}
drive_office_revision_sync_total{provider,result}
drive_office_session_active{provider}
```

### 確認コマンド

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
RUN_DRIVE_OFFICE_SMOKE=1 make smoke-openfga
RUN_DRIVE_OFFICE_WEBHOOK_SMOKE=1 make smoke-openfga
```

smoke では次を確認します。

- viewer は view session だけ作れる
- editor は edit session を作れる
- unshared user は session を作れない
- E2EE zero-knowledge file は Office session を作れない
- duplicated webhook は 1 回だけ処理される
- stale provider revision は reject される

## Step 2. enterprise eDiscovery provider integration を実装する

### 対象 subsystem

```text
db/migrations/0019_openfga_drive_enterprise_integrations.up.sql
db/queries/drive_ediscovery.sql
backend/internal/service/drive_ediscovery_service.go
backend/internal/service/drive_ediscovery_provider.go
backend/internal/service/drive_ediscovery_provider_fake.go
backend/internal/api/drive_ediscovery.go
frontend/src/views/admin/DriveEDiscoveryView.vue
frontend/src/stores/driveEDiscovery.ts
scripts/smoke-openfga.sh
```

### DB を追加する

Phase 8 の legal discovery workflow を外部 provider に接続します。case、hold、export job、manifest、provider delivery を正本として DB に持ちます。

```sql
CREATE TABLE drive_ediscovery_provider_connections (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  status TEXT NOT NULL,
  config_json JSONB NOT NULL DEFAULT '{}',
  encrypted_credentials BYTEA,
  created_by_user_id BIGINT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, provider)
);

CREATE TABLE drive_ediscovery_exports (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  case_public_id TEXT NOT NULL,
  provider_connection_id BIGINT NOT NULL REFERENCES drive_ediscovery_provider_connections(id),
  requested_by_user_id BIGINT NOT NULL REFERENCES users(id),
  approved_by_user_id BIGINT REFERENCES users(id),
  status TEXT NOT NULL,
  manifest_hash TEXT,
  provider_export_id TEXT,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drive_ediscovery_export_items (
  id BIGSERIAL PRIMARY KEY,
  export_id BIGINT NOT NULL REFERENCES drive_ediscovery_exports(id) ON DELETE CASCADE,
  file_object_id BIGINT NOT NULL REFERENCES file_objects(id),
  file_revision TEXT NOT NULL,
  content_sha256 TEXT NOT NULL,
  status TEXT NOT NULL,
  provider_item_id TEXT,
  UNIQUE (export_id, file_object_id, file_revision)
);
```

### service boundary

eDiscovery provider integration は通常 Drive download endpoint を再利用しません。専用 service で、legal discovery policy と approval を通してから export します。

処理順:

1. actor が `tenant_admin` と `drive_legal_discovery_admin` を持つことを DB role で確認する
2. tenant policy `drive.legal_discovery.provider_export.enabled` を確認する
3. case / hold が active であることを確認する
4. export request を `pending_approval` で作る
5. approver が request user と別人であることを確認する
6. export item を legal hold scope から固定する
7. item ごとに file state / residency / retention / E2EE mode を確認する
8. zero-knowledge E2EE file は plaintext export に含めない
9. manifest に file id、revision、sha256、size、timestamp、actor、case id を記録する
10. provider adapter へ manifest と content stream を送る
11. provider export id と manifest hash を DB に保存する
12. audit と metrics を記録する

### provider interface

#### ファイル: `backend/internal/service/drive_ediscovery_provider.go`

```go
package service

import "context"

type DriveEDiscoveryProvider interface {
	CreateExport(ctx context.Context, input DriveEDiscoveryCreateExportInput) (DriveEDiscoveryProviderExport, error)
	UploadItem(ctx context.Context, input DriveEDiscoveryUploadItemInput) (DriveEDiscoveryProviderItem, error)
	FinalizeExport(ctx context.Context, providerExportID string, manifest DriveEDiscoveryManifest) error
	GetExportStatus(ctx context.Context, providerExportID string) (DriveEDiscoveryProviderExportStatus, error)
}
```

provider credential は `backend/internal/auth/secret_box.go` と同じ方針で encrypted at rest にします。credential の raw 値は audit / log に出しません。

### admin UI

`DriveEDiscoveryView.vue` に次を追加します。

- provider connection status
- legal case / hold selector
- export request form
- approval queue
- manifest hash
- provider delivery status
- retry / cancel

UI は通常 Drive browser から分離します。file body を横断閲覧する画面ではなく、case export operation の画面として扱います。

### audit / metrics

追加する audit action:

- `drive.ediscovery.provider.connect`
- `drive.ediscovery.export.request`
- `drive.ediscovery.export.approve`
- `drive.ediscovery.export.item.upload`
- `drive.ediscovery.export.finalize`
- `drive.ediscovery.export.reject`

追加する metrics:

```text
drive_ediscovery_export_total{provider,result}
drive_ediscovery_export_items_total{provider,result}
drive_ediscovery_export_bytes_total{provider}
drive_ediscovery_provider_request_seconds{provider,operation,result}
```

### 確認コマンド

```bash
make gen
go test ./backend/...
RUN_DRIVE_EDISCOVERY_PROVIDER_SMOKE=1 make smoke-openfga
```

smoke では次を確認します。

- legal discovery admin だけ export request を作れる
- request user は自分の request を approve できない
- hold scope 外の file は export item に入らない
- zero-knowledge E2EE file は plaintext export されない
- manifest hash が provider fake 側と DB 側で一致する
- revoked / deleted file の扱いが legal hold policy と一致する

## Step 3. customer-managed HSM dedicated deployment を実装する

### 対象 subsystem

```text
db/migrations/0019_openfga_drive_enterprise_integrations.up.sql
db/queries/drive_hsm.sql
backend/internal/service/drive_hsm_service.go
backend/internal/service/drive_hsm_client.go
backend/internal/service/drive_hsm_client_fake.go
backend/internal/config/config.go
backend/internal/api/drive_hsm.go
frontend/src/views/admin/DriveSecurityKeysView.vue
frontend/src/stores/driveHSM.ts
scripts/smoke-openfga.sh
```

### DB を追加する

tenant 専用 HSM deployment は、CMK の provider option ではなく、tenant security posture の一部として扱います。

```sql
CREATE TABLE drive_hsm_deployments (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  endpoint_url TEXT NOT NULL,
  status TEXT NOT NULL,
  attestation_hash TEXT,
  health_status TEXT NOT NULL DEFAULT 'unknown',
  last_health_checked_at TIMESTAMPTZ,
  created_by_user_id BIGINT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, provider)
);

CREATE TABLE drive_hsm_keys (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  deployment_id BIGINT NOT NULL REFERENCES drive_hsm_deployments(id),
  key_ref TEXT NOT NULL,
  key_version TEXT NOT NULL,
  purpose TEXT NOT NULL,
  status TEXT NOT NULL,
  rotation_due_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, key_ref, key_version)
);

CREATE TABLE drive_hsm_key_bindings (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  workspace_id BIGINT,
  file_object_id BIGINT REFERENCES file_objects(id) ON DELETE CASCADE,
  hsm_key_id BIGINT NOT NULL REFERENCES drive_hsm_keys(id),
  binding_scope TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, binding_scope, workspace_id, file_object_id)
);
```

### config を追加する

#### ファイル: `backend/internal/config/config.go`

追加する env:

```text
DRIVE_HSM_ENABLED=false
DRIVE_HSM_PROVIDER=fake
DRIVE_HSM_ENDPOINT=
DRIVE_HSM_CLIENT_CERT_FILE=
DRIVE_HSM_CLIENT_KEY_FILE=
DRIVE_HSM_CA_FILE=
DRIVE_HSM_TIMEOUT=5s
DRIVE_HSM_FAIL_CLOSED=true
```

`DRIVE_HSM_FAIL_CLOSED=false` は local development のみ許可します。production profile では起動時に reject します。

### HSM client interface

#### ファイル: `backend/internal/service/drive_hsm_client.go`

```go
package service

import "context"

type DriveHSMClient interface {
	Health(ctx context.Context) (DriveHSMHealth, error)
	VerifyAttestation(ctx context.Context, input DriveHSMAttestationInput) (DriveHSMAttestation, error)
	WrapDataKey(ctx context.Context, input DriveHSMWrapDataKeyInput) (DriveHSMWrappedDataKey, error)
	UnwrapDataKey(ctx context.Context, input DriveHSMUnwrapDataKeyInput) (DriveHSMPlainDataKey, error)
	RotateKeyVersion(ctx context.Context, keyRef string) (DriveHSMKeyVersion, error)
}
```

`DriveHSMPlainDataKey` は request lifetime だけ memory に置きます。DB、audit、log、panic message に入れません。

### Drive encryption へ接続する

object write の順序:

1. tenant policy から encryption mode を解決する
2. dedicated HSM binding が必要な workspace / file か確認する
3. HSM health と key status を確認する
4. random data key を生成する
5. HSM で data key を wrap する
6. object storage へ encrypted content を書く
7. wrapped data key ref と key version を DB に保存する
8. audit を書く

object read の順序:

1. DB guard で tenant / workspace / file state を確認する
2. OpenFGA `can_download` を確認する
3. HSM deployment / key status を確認する
4. disabled / destroyed / unavailable なら fail-closed
5. HSM で data key を unwrap する
6. content stream を decrypt して返す

### admin UI

`DriveSecurityKeysView.vue` に次を追加します。

- HSM deployment status
- attestation hash
- key status / version
- rotation due date
- last health check
- fail-closed reason
- rotation request
- emergency disable

UI で key material を入力させません。customer-managed HSM の接続情報は endpoint と certificate reference だけ扱います。

### audit / metrics

追加する audit action:

- `drive.hsm.deployment.create`
- `drive.hsm.attestation.verify`
- `drive.hsm.key.bind`
- `drive.hsm.key.rotate`
- `drive.hsm.operation.denied`

追加する metrics:

```text
drive_hsm_request_seconds{provider,operation,result}
drive_hsm_health_status{provider,status}
drive_hsm_fail_closed_total{reason}
drive_hsm_key_rotation_total{result}
```

### 確認コマンド

```bash
make gen
go test ./backend/...
RUN_DRIVE_HSM_SMOKE=1 make smoke-openfga
RUN_DRIVE_HSM_FAIL_CLOSED_SMOKE=1 make smoke-openfga
```

smoke では次を確認します。

- HSM enabled tenant は fake HSM で upload / download できる
- HSM unavailable のとき download は fail-closed になる
- disabled key では signed URL / content response を返さない
- rotation 後の新規 upload は新 key version を使う
- old key version が active な間は既存 file を読める
- raw key material が DB と audit に残らない

## Step 4. on-premise storage gateway を実装する

### 対象 subsystem

```text
db/migrations/0019_openfga_drive_enterprise_integrations.up.sql
db/queries/drive_storage_gateways.sql
backend/internal/service/drive_gateway_service.go
backend/internal/service/drive_gateway_client.go
backend/internal/service/drive_gateway_client_fake.go
backend/internal/service/file_storage.go
backend/internal/api/drive_gateway.go
backend/cmd/drive-gateway/main.go
frontend/src/views/admin/DriveStorageGatewayView.vue
frontend/src/stores/driveStorageGateway.ts
scripts/smoke-openfga.sh
```

### DB を追加する

on-prem gateway は storage driver の 1 つです。gateway が DB / OpenFGA に直接触る設計にはしません。

```sql
CREATE TABLE drive_storage_gateways (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  workspace_id BIGINT,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  endpoint_url TEXT NOT NULL,
  certificate_fingerprint TEXT NOT NULL,
  last_seen_at TIMESTAMPTZ,
  created_by_user_id BIGINT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, name)
);

CREATE TABLE drive_gateway_objects (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  gateway_id BIGINT NOT NULL REFERENCES drive_storage_gateways(id),
  file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
  gateway_object_key TEXT NOT NULL,
  manifest_hash TEXT NOT NULL,
  replication_status TEXT NOT NULL,
  last_verified_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (gateway_id, gateway_object_key),
  UNIQUE (file_object_id)
);

CREATE TABLE drive_gateway_transfers (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  gateway_id BIGINT NOT NULL REFERENCES drive_storage_gateways(id),
  file_object_id BIGINT REFERENCES file_objects(id),
  direction TEXT NOT NULL,
  status TEXT NOT NULL,
  bytes_total BIGINT NOT NULL DEFAULT 0,
  bytes_transferred BIGINT NOT NULL DEFAULT 0,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`file_objects.storage_driver` には `onprem_gateway` を許可します。`storage_key` は service-generated で、user input を使いません。

### gateway protocol

backend と gateway の通信は mTLS で固定します。

backend から gateway:

- reserve object key
- upload chunk
- complete upload with manifest
- create download stream
- verify object manifest
- delete object
- heartbeat / status

gateway から backend:

- heartbeat
- transfer status callback
- object verification result

gateway から backend の callback は、mTLS identity、gateway id、signed payload、nonce、timestamp を検証します。

### storage driver に接続する

`FileStorage` に gateway driver を追加します。

```go
type GatewayFileStorage struct {
	client DriveGatewayClient
}
```

write path:

1. DB guard と OpenFGA `can_edit` / parent `can_edit` を確認する
2. tenant / workspace policy から gateway を選ぶ
3. upload state を `reserved` にする
4. gateway に service-generated key を予約する
5. chunk upload を行う
6. manifest hash、size、checksum を検証する
7. DB state を `active` にする
8. search / sync / AI / legal hold outbox を発行する

read path:

1. DB guard と OpenFGA `can_download` を確認する
2. gateway status が active であることを確認する
3. object manifest を必要に応じて verify する
4. gateway download stream を返す

gateway unavailable のとき、metadata は表示しても content download は fail-closed にします。

### gateway binary

`backend/cmd/drive-gateway/main.go` は customer network 内で動く最小 binary とします。

責務:

- mTLS server
- local disk / S3 compatible storage adapter
- manifest generation
- chunk checksum verification
- heartbeat callback

gateway binary は tenant DB credential、OpenFGA credential、application session secret を持ちません。

### admin UI

`DriveStorageGatewayView.vue` に次を追加します。

- gateway registration
- certificate fingerprint
- status / last seen
- workspace binding
- transfer list
- manifest verification
- disconnect / disable

### audit / metrics

追加する audit action:

- `drive.gateway.register`
- `drive.gateway.disable`
- `drive.gateway.upload.complete`
- `drive.gateway.download.start`
- `drive.gateway.manifest.verify`

追加する metrics:

```text
drive_gateway_transfer_total{direction,result}
drive_gateway_transfer_bytes_total{direction}
drive_gateway_heartbeat_age_seconds{gateway}
drive_gateway_manifest_verify_total{result}
```

### 確認コマンド

```bash
make gen
go test ./backend/...
RUN_DRIVE_GATEWAY_SMOKE=1 make smoke-openfga
RUN_DRIVE_GATEWAY_DISCONNECT_SMOKE=1 make smoke-openfga
```

smoke では次を確認します。

- active gateway に upload / download できる
- unshared user は gateway file を download できない
- gateway から DB / OpenFGA credential を要求しない
- manifest mismatch は active state にならない
- gateway disabled 後は new upload も download も fail-closed になる

## Step 5. end-to-end encryption with zero-knowledge sharing を実装する

### 対象 subsystem

```text
db/migrations/0019_openfga_drive_enterprise_integrations.up.sql
db/queries/drive_e2ee.sql
backend/internal/service/drive_e2ee_service.go
backend/internal/api/drive_e2ee.go
frontend/src/crypto/driveE2EE.ts
frontend/src/stores/driveE2EE.ts
frontend/src/components/DriveE2EEBanner.vue
frontend/src/components/DriveE2EEShareDialog.vue
frontend/src/views/DriveView.vue
scripts/smoke-openfga.sh
```

### DB を追加する

server は plaintext content encryption key を保存しません。保存するのは public key、wrapped key envelope、ciphertext metadata だけです。

```sql
CREATE TABLE drive_e2ee_user_keys (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key_algorithm TEXT NOT NULL,
  public_key_jwk JSONB NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  rotated_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX drive_e2ee_user_keys_one_active
  ON drive_e2ee_user_keys (tenant_id, user_id)
  WHERE status = 'active';

CREATE TABLE drive_e2ee_file_keys (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
  key_version INTEGER NOT NULL,
  encryption_algorithm TEXT NOT NULL,
  ciphertext_sha256 TEXT NOT NULL,
  encrypted_metadata JSONB NOT NULL DEFAULT '{}',
  created_by_user_id BIGINT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (file_object_id, key_version)
);

CREATE TABLE drive_e2ee_key_envelopes (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  file_key_id BIGINT NOT NULL REFERENCES drive_e2ee_file_keys(id) ON DELETE CASCADE,
  recipient_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  recipient_key_id BIGINT NOT NULL REFERENCES drive_e2ee_user_keys(id),
  wrapped_file_key BYTEA NOT NULL,
  wrap_algorithm TEXT NOT NULL,
  created_by_user_id BIGINT NOT NULL REFERENCES users(id),
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (file_key_id, recipient_user_id, recipient_key_id)
);
```

`file_objects` に encryption mode を追加します。

```sql
ALTER TABLE file_objects
  ADD COLUMN encryption_mode TEXT NOT NULL DEFAULT 'server_managed',
  ADD COLUMN e2ee_file_key_public_id TEXT;
```

### frontend crypto を実装する

#### ファイル: `frontend/src/crypto/driveE2EE.ts`

WebCrypto で次を実装します。

- user key pair generation
- public key registration
- file data key generation
- browser-side file encryption
- browser-side file decryption
- recipient public key による file key wrap
- local key rotation helper

private key の扱いは product policy として固定します。

- server へ private key を送らない
- browser storage に保存する場合は passphrase wrapping を必須にする
- recovery key を server escrow する mode は zero-knowledge と呼ばない
- enterprise recovery を入れる場合は `enterprise_escrow` と明示表示する

### backend API を追加する

| Route | 目的 |
| --- | --- |
| `POST /api/v1/drive/e2ee/user-keys` | user public key を登録する |
| `GET /api/v1/drive/e2ee/users/{userId}/public-key` | share recipient の public key を取得する |
| `POST /api/v1/drive/files/e2ee` | encrypted file metadata と ciphertext object を作る |
| `GET /api/v1/drive/files/{fileId}/e2ee/envelope` | actor 用 wrapped key envelope を返す |
| `POST /api/v1/drive/files/{fileId}/e2ee/envelopes` | share と同時に recipient envelope を追加する |
| `POST /api/v1/drive/files/{fileId}/e2ee/rotate` | file key version を rotate する |

download path:

1. DB guard で tenant / workspace / file state を確認する
2. OpenFGA `can_download` を確認する
3. actor 用 key envelope が active であることを確認する
4. ciphertext stream と wrapped key envelope を返す
5. browser が decrypt する

share path:

1. actor が OpenFGA `can_share` を持つことを確認する
2. recipient が active user で public key を持つことを確認する
3. browser が recipient 用 wrapped key envelope を作る
4. backend は share tuple と envelope を同じ transaction / outbox flow で保存する
5. audit に key material ではなく key version と recipient id だけ記録する

revoke path:

1. share を revoke する
2. recipient envelope を revoke する
3. strict revoke が必要な場合は file key を rotate する
4. new ciphertext / new envelopes を保存する
5. old key version を retired にする

### Zero-knowledge policy guard

server-side plaintext を必要とする機能は、E2EE zero-knowledge file では拒否します。

| 機能 | zero-knowledge file での扱い |
| --- | --- |
| full text search | encrypted metadata search だけ許可 |
| Office co-authoring |拒否 |
| AI summary | 拒否、または client-side summary のみ |
| DLP content scan | 拒否、または client-side scan result の署名付き提出のみ |
| eDiscovery plaintext export | 拒否 |
| public anonymous link | default disabled |
| marketplace app content read | 拒否、または client-side delegated decrypt のみ |

この表は service guard と UI の両方に反映します。

### audit / metrics

追加する audit action:

- `drive.e2ee.user_key.create`
- `drive.e2ee.file.create`
- `drive.e2ee.envelope.create`
- `drive.e2ee.envelope.revoke`
- `drive.e2ee.file_key.rotate`
- `drive.e2ee.policy.deny`

追加する metrics:

```text
drive_e2ee_files_total{mode}
drive_e2ee_envelope_total{operation,result}
drive_e2ee_key_rotation_total{result}
drive_e2ee_policy_denied_total{feature}
```

### 確認コマンド

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
RUN_DRIVE_E2EE_SMOKE=1 make smoke-openfga
RUN_DRIVE_E2EE_REVOKE_SMOKE=1 make smoke-openfga
```

smoke では次を確認します。

- server に plaintext file key が保存されない
- owner は encrypted upload / download / decrypt ができる
- shared recipient は envelope がある場合だけ decrypt できる
- OpenFGA share を消すと envelope があっても download できない
- envelope を消すと OpenFGA share があっても decrypt できない
- key rotation 後に revoked user は新 revision を decrypt できない
- Office / AI / eDiscovery plaintext export は zero-knowledge file を拒否する

## Step 6. AI-assisted document classification and summarization を実装する

### 対象 subsystem

```text
db/migrations/0019_openfga_drive_enterprise_integrations.up.sql
db/queries/drive_ai.sql
backend/internal/service/drive_ai_service.go
backend/internal/service/drive_ai_provider.go
backend/internal/service/drive_ai_provider_fake.go
backend/internal/jobs/drive_ai_worker.go
backend/internal/api/drive_ai.go
frontend/src/components/DriveAISummaryPanel.vue
frontend/src/components/DriveClassificationBadge.vue
frontend/src/views/admin/DriveAIPolicyView.vue
scripts/smoke-openfga.sh
```

### DB を追加する

AI result は source file の派生 content です。source file の authz なしに返しません。

```sql
CREATE TABLE drive_ai_jobs (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
  file_revision TEXT NOT NULL,
  job_type TEXT NOT NULL,
  provider TEXT NOT NULL,
  status TEXT NOT NULL,
  requested_by_user_id BIGINT REFERENCES users(id),
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (file_object_id, file_revision, job_type)
);

CREATE TABLE drive_ai_classifications (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
  file_revision TEXT NOT NULL,
  label TEXT NOT NULL,
  confidence NUMERIC(5,4) NOT NULL,
  provider TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (file_object_id, file_revision, label)
);

CREATE TABLE drive_ai_summaries (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
  file_revision TEXT NOT NULL,
  summary_text TEXT NOT NULL,
  provider TEXT NOT NULL,
  input_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (file_object_id, file_revision)
);
```

### provider interface

#### ファイル: `backend/internal/service/drive_ai_provider.go`

```go
package service

import "context"

type DriveAIProvider interface {
	Classify(ctx context.Context, input DriveAIClassifyInput) (DriveAIClassificationResult, error)
	Summarize(ctx context.Context, input DriveAISummarizeInput) (DriveAISummaryResult, error)
}
```

local fake は deterministic result を返します。CI smoke では external model に依存しません。

### job 実行順

AI job は upload 後すぐには走らせません。P7 / P8 の guard を通します。

1. tenant policy `drive.ai.enabled` を確認する
2. file が active であることを確認する
3. scan / DLP が clean または allowed であることを確認する
4. E2EE zero-knowledge file では server-side AI を拒否する
5. data residency と provider region が一致することを確認する
6. provider retention / training opt-out config を確認する
7. content stream を provider に渡す
8. result を保存する
9. audit と metrics を記録する

### response-time authorization

summary / classification を読む API では、保存済み result を返す前に毎回 source file を確認します。

| Route | 必須 check |
| --- | --- |
| `GET /api/v1/drive/files/{fileId}/ai/summary` | DB guard + OpenFGA `can_view` |
| `GET /api/v1/drive/files/{fileId}/ai/classifications` | DB guard + OpenFGA `can_view` |
| `POST /api/v1/drive/files/{fileId}/ai/jobs` | DB guard + OpenFGA `can_edit` または policy role |

source file access が消えた user には、古い summary も返しません。

### admin policy UI

`DriveAIPolicyView.vue` に次を追加します。

- tenant opt-in
- provider selection
- allowed region
- retention policy
- training opt-out status
- DLP block behavior
- E2EE handling
- manual reprocess

### audit / metrics

追加する audit action:

- `drive.ai.job.create`
- `drive.ai.classification.create`
- `drive.ai.summary.create`
- `drive.ai.policy.deny`
- `drive.ai.result.view`

追加する metrics:

```text
drive_ai_jobs_total{job_type,provider,result}
drive_ai_provider_request_seconds{provider,operation,result}
drive_ai_policy_denied_total{reason}
drive_ai_result_view_total{kind,result}
```

### 確認コマンド

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
RUN_DRIVE_AI_SMOKE=1 make smoke-openfga
RUN_DRIVE_AI_POLICY_SMOKE=1 make smoke-openfga
```

smoke では次を確認します。

- AI disabled tenant では job が作れない
- DLP blocked file は AI job が走らない
- zero-knowledge E2EE file は server-side summary を拒否する
- viewer は summary を読める
- unshared user は saved summary も読めない
- access revoke 後は old summary API が 403 / 404 になる
- fake provider result が UI に表示される

## Step 7. public marketplace integration for Drive apps を実装する

### 対象 subsystem

```text
db/migrations/0019_openfga_drive_enterprise_integrations.up.sql
db/queries/drive_marketplace.sql
backend/internal/service/drive_marketplace_service.go
backend/internal/service/drive_marketplace_webhook.go
backend/internal/api/drive_marketplace.go
backend/internal/api/drive_app_callbacks.go
frontend/src/views/DriveMarketplaceView.vue
frontend/src/views/admin/DriveMarketplaceAdminView.vue
frontend/src/components/DriveAppInstallDialog.vue
frontend/src/stores/driveMarketplace.ts
scripts/smoke-openfga.sh
```

### DB を追加する

marketplace app は tenant に install され、scope と approval に従って Drive API を呼びます。scope は OpenFGA permission を拡張しません。

```sql
CREATE TABLE drive_marketplace_apps (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  publisher_name TEXT NOT NULL,
  status TEXT NOT NULL,
  homepage_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drive_marketplace_app_versions (
  id BIGSERIAL PRIMARY KEY,
  app_id BIGINT NOT NULL REFERENCES drive_marketplace_apps(id) ON DELETE CASCADE,
  version TEXT NOT NULL,
  manifest_json JSONB NOT NULL,
  signature TEXT NOT NULL,
  review_status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (app_id, version)
);

CREATE TABLE drive_marketplace_installations (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  app_id BIGINT NOT NULL REFERENCES drive_marketplace_apps(id),
  app_version_id BIGINT NOT NULL REFERENCES drive_marketplace_app_versions(id),
  status TEXT NOT NULL,
  installed_by_user_id BIGINT NOT NULL REFERENCES users(id),
  approved_by_user_id BIGINT REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, app_id)
);

CREATE TABLE drive_marketplace_installation_scopes (
  id BIGSERIAL PRIMARY KEY,
  installation_id BIGINT NOT NULL REFERENCES drive_marketplace_installations(id) ON DELETE CASCADE,
  scope TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (installation_id, scope)
);

CREATE TABLE drive_app_webhook_deliveries (
  id BIGSERIAL PRIMARY KEY,
  public_id TEXT NOT NULL UNIQUE,
  tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  installation_id BIGINT NOT NULL REFERENCES drive_marketplace_installations(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  payload_hash TEXT NOT NULL,
  status TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### app manifest

app version manifest の最小形:

```json
{
  "name": "Example Drive App",
  "version": "1.0.0",
  "requestedScopes": [
    "drive.file.read",
    "drive.file.write",
    "drive.webhook.receive"
  ],
  "redirectUris": [
    "https://app.example.com/oauth/callback"
  ],
  "webhookUrl": "https://app.example.com/drive/webhook",
  "contentSecurity": {
    "iframeOrigins": ["https://app.example.com"]
  }
}
```

manifest は review 済み version だけ install 可能にします。signature verification を通らない version は API でも UI でも表示しません。

### authorization model

marketplace app は 2 つの権限を同時に満たす必要があります。

1. installation scope が operation を許可している
2. request actor が Drive resource に対して DB guard + OpenFGA permission を持っている

app scope は permission の上限です。OpenFGA permission の代わりではありません。

app initiated operation の actor は次のどちらかに固定します。

- delegated user: user session / OAuth grant の user として OpenFGA check
- app service account: tenant 内に明示 provisioning された service account user として OpenFGA check

service account を使う場合、通常 user と同じく Drive share / group membership を持たせます。marketplace install だけで全 Drive file を読める状態にしません。

### API を追加する

| Route | 認証 | 目的 |
| --- | --- | --- |
| `GET /api/v1/drive/marketplace/apps` | browser session | review 済み app 一覧 |
| `POST /api/v1/drive/marketplace/installations` | tenant admin | install request |
| `POST /api/v1/drive/marketplace/installations/{id}/approve` | tenant admin approver | install approve |
| `DELETE /api/v1/drive/marketplace/installations/{id}` | tenant admin | uninstall |
| `GET /api/v1/drive/marketplace/installations` | tenant admin | installed app 一覧 |
| `POST /api/v1/drive/apps/{installationId}/callbacks` | app signature | app callback |

Drive file API を marketplace app から呼ぶ場合は、browser API とは middleware を分けます。

```text
/api/v1/drive/*
/api/external/drive/*
/api/apps/drive/*
```

`/api/apps/drive/*` は app credential、installation status、scope、actor mapping、rate limit を確認してから DriveService を呼びます。

### webhook delivery

Drive event を app webhook に送る順序:

1. source event を outbox に入れる
2. installation status と scope を確認する
3. event payload を最小化する
4. sensitive metadata を policy に従って削る
5. delivery record を作る
6. signed request を送る
7. retry は exponential backoff
8. uninstall / disabled app は delivery を止める

webhook payload に file content や E2EE key envelope を入れません。

### frontend

追加する画面:

- `DriveMarketplaceView.vue`: app catalog、scope 表示、install request
- `DriveMarketplaceAdminView.vue`: approval、installed apps、disable、webhook delivery status
- `DriveAppInstallDialog.vue`: requested scope、publisher、version、review status を表示

install UI は scope を短い説明に変換して表示します。ただし UI 文言だけに頼らず、API 側で scope enforcement を必ず行います。

### audit / metrics

追加する audit action:

- `drive.marketplace.app.review`
- `drive.marketplace.install.request`
- `drive.marketplace.install.approve`
- `drive.marketplace.install.reject`
- `drive.marketplace.uninstall`
- `drive.marketplace.scope.deny`
- `drive.marketplace.webhook.deliver`

追加する metrics:

```text
drive_marketplace_install_total{app,result}
drive_marketplace_scope_denied_total{scope,operation}
drive_marketplace_webhook_delivery_total{app,result}
drive_marketplace_app_api_requests_total{app,operation,result}
```

### 確認コマンド

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
RUN_DRIVE_MARKETPLACE_SMOKE=1 make smoke-openfga
RUN_DRIVE_MARKETPLACE_SCOPE_SMOKE=1 make smoke-openfga
```

smoke では次を確認します。

- review 済み app だけ catalog に出る
- tenant admin だけ install request を作れる
- request user は自分の install request を approve できない
- scope にない operation は拒否される
- scope があっても OpenFGA permission がなければ拒否される
- uninstall 後は app API と webhook delivery が止まる
- E2EE key envelope は webhook payload に含まれない

## Step 8. smoke / E2E / operational verification を固定する

### `scripts/smoke-openfga.sh` を拡張する

既存の smoke は default path を軽く保ち、Phase 9 の各機能は env で明示実行します。

```bash
RUN_DRIVE_OFFICE_SMOKE=1 make smoke-openfga
RUN_DRIVE_EDISCOVERY_PROVIDER_SMOKE=1 make smoke-openfga
RUN_DRIVE_HSM_SMOKE=1 make smoke-openfga
RUN_DRIVE_GATEWAY_SMOKE=1 make smoke-openfga
RUN_DRIVE_E2EE_SMOKE=1 make smoke-openfga
RUN_DRIVE_AI_SMOKE=1 make smoke-openfga
RUN_DRIVE_MARKETPLACE_SMOKE=1 make smoke-openfga
```

high-risk failure path は別 env にします。

```bash
RUN_DRIVE_OFFICE_WEBHOOK_SMOKE=1 make smoke-openfga
RUN_DRIVE_HSM_FAIL_CLOSED_SMOKE=1 make smoke-openfga
RUN_DRIVE_GATEWAY_DISCONNECT_SMOKE=1 make smoke-openfga
RUN_DRIVE_E2EE_REVOKE_SMOKE=1 make smoke-openfga
RUN_DRIVE_AI_POLICY_SMOKE=1 make smoke-openfga
RUN_DRIVE_MARKETPLACE_SCOPE_SMOKE=1 make smoke-openfga
```

### E2E を追加する

frontend E2E では、少なくとも次の user flow を固定します。

- Office 対応 file を開き、edit session を開始して close する
- admin が eDiscovery export request を作り、別 admin が approve する
- admin が HSM status と fail-closed reason を確認する
- admin が on-prem gateway を登録し、Drive file を gateway workspace に upload する
- owner が E2EE file を upload し、recipient に share して revoke する
- viewer が AI summary を読み、revoke 後に読めなくなる
- tenant admin が marketplace app を install / approve / uninstall する

### Operational drill

実 provider を使う drill は CI から分けます。`IMPL.md` に provider、credential location、test tenant、rollback、expected audit を記録します。

| Drill | 確認内容 |
| --- | --- |
| Office provider | real provider session、webhook signature、revision sync |
| eDiscovery provider | real export、manifest hash、provider status polling |
| HSM | attestation、key wrap / unwrap、unavailable fail-closed |
| on-prem gateway | mTLS、network disconnect、manifest verification |
| AI provider | retention opt-out、region routing、DLP blocked denial |
| marketplace app | signed webhook、scope denial、uninstall cleanup |

## 最終確認コマンド

通常確認:

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
cd openfga && fga model test --tests drive.fga.yaml
make smoke-openfga
```

Phase 9 全機能確認:

```bash
RUN_DRIVE_OFFICE_SMOKE=1 make smoke-openfga
RUN_DRIVE_OFFICE_WEBHOOK_SMOKE=1 make smoke-openfga
RUN_DRIVE_EDISCOVERY_PROVIDER_SMOKE=1 make smoke-openfga
RUN_DRIVE_HSM_SMOKE=1 make smoke-openfga
RUN_DRIVE_HSM_FAIL_CLOSED_SMOKE=1 make smoke-openfga
RUN_DRIVE_GATEWAY_SMOKE=1 make smoke-openfga
RUN_DRIVE_GATEWAY_DISCONNECT_SMOKE=1 make smoke-openfga
RUN_DRIVE_E2EE_SMOKE=1 make smoke-openfga
RUN_DRIVE_E2EE_REVOKE_SMOKE=1 make smoke-openfga
RUN_DRIVE_AI_SMOKE=1 make smoke-openfga
RUN_DRIVE_AI_POLICY_SMOKE=1 make smoke-openfga
RUN_DRIVE_MARKETPLACE_SMOKE=1 make smoke-openfga
RUN_DRIVE_MARKETPLACE_SCOPE_SMOKE=1 make smoke-openfga
```

provider drill:

```bash
RUN_DRIVE_OFFICE_PROVIDER_DRILL=1 make smoke-openfga
RUN_DRIVE_EDISCOVERY_PROVIDER_DRILL=1 make smoke-openfga
RUN_DRIVE_HSM_PROVIDER_DRILL=1 make smoke-openfga
RUN_DRIVE_GATEWAY_PROVIDER_DRILL=1 make smoke-openfga
RUN_DRIVE_AI_PROVIDER_DRILL=1 make smoke-openfga
RUN_DRIVE_MARKETPLACE_PROVIDER_DRILL=1 make smoke-openfga
```

## 生成物として扱うファイル

次は生成物です。手で編集しません。

- `backend/internal/db/*.sql.go`
- `backend/internal/db/models.go`
- `db/schema.sql`
- `openapi/openapi.yaml`
- `frontend/src/api/generated/*`
- `backend/web/dist/*`

Phase 9 で手で編集する正本は次です。

- `db/migrations/0019_openfga_drive_enterprise_integrations.*.sql`
- `db/queries/drive_office.sql`
- `db/queries/drive_ediscovery.sql`
- `db/queries/drive_hsm.sql`
- `db/queries/drive_storage_gateways.sql`
- `db/queries/drive_e2ee.sql`
- `db/queries/drive_ai.sql`
- `db/queries/drive_marketplace.sql`
- `backend/internal/service/*`
- `backend/internal/api/*`
- `backend/internal/jobs/*`
- `frontend/src/views/*`
- `frontend/src/components/*`
- `frontend/src/stores/*`
- `frontend/src/crypto/*`
- `scripts/smoke-openfga.sh`
- `IMPL.md`

## ここまでで何ができているか

この文書を最後まで実装すると、Drive は次の状態になります。

- browser Drive、Office co-authoring、external app、sync client、on-prem gateway が同じ DB guard と OpenFGA relation に従う
- enterprise legal workflow は provider export まで閉じている
- customer-managed HSM と zero-knowledge E2EE を区別して運用できる
- server が plaintext を扱える mode と扱えない mode の product 表示が一致している
- AI と marketplace は source file の permission を超えて情報を返さない
- external provider に依存する機能は CI fake と real provider drill の両方で確認できる
- Phase 8 末尾の 7 項目はすべて実装対象に含まれ、未定義の backlog として残っていない
