# データクレンジング / 前処理パイプライン実装計画

## 目的

この文書は、`docs/data-pipeline-ui-and-implementation.md` で整理したデータクレンジング / 前処理カテゴリを、HaoHao の既存 Dataset / Work table / outbox / medallion catalog 基盤に合わせて実装するための計画です。

v1 は ClickHouse 優先で実装します。入力は active tenant の Dataset または managed Work table、出力は tenant work database 上の managed Work table とします。DuckDB / Parquet runtime、外部 S3 / DB input、LLM enrichment、任意 SQL node、backfill UI、multi-output fanout write は v1 では扱いません。

UI は独立ページ `/data-pipelines` として追加し、Vue Flow を使って DAG builder を提供します。Vue Flow は `@vue-flow/core` の nodes / edges model を使い、既存 `LineageFlowGraph.vue` と同じ style import / Controls / MiniMap パターンを再利用します。

## v1 の成功条件

- `data_pipeline_user` tenant role を持つユーザーが pipeline を作成、編集、Preview、手動実行、定期実行できる。
- pipeline definition は Vue Flow 互換の graph JSON として version 管理され、published version だけが run / schedule の対象になる。
- 実行結果は run / step 単位で status、error summary、error sample、row count、duration を追跡できる。
- output は managed Work table として登録され、既存 Dataset / Work table UI、lineage、medallion catalog と接続できる。
- すべての read / write query は `tenant_id` で tenant 境界を守る。

## 参照元

- `docs/data-pipeline-ui-and-implementation.md`
- Vue Flow GitHub: https://github.com/bcakmakoglu/vue-flow
- Vue Flow docs: https://vue-flow-docs.netlify.app/guide/
- 既存実装: `frontend/src/components/LineageFlowGraph.vue`
- 既存 scheduling 実装: `backend/internal/jobs/work_table_export_scheduler.go`
- 既存 async 実行実装: `backend/internal/service/outbox_handler.go`

## 全体アーキテクチャ

```text
Frontend /data-pipelines
  - Vue Flow DAG builder
  - node config inspector
  - preview / run / schedule panel
        |
        v
Backend API /api/v1/data-pipelines
  - graph validation
  - version publish
  - preview
  - manual run request
  - schedule CRUD
        |
        v
PostgreSQL metadata
  - pipeline definitions
  - versions
  - runs
  - run steps
  - schedules
        |
        v
Outbox + scheduler
  - data_pipeline.run_requested
  - due schedule claim
        |
        v
ClickHouse executor
  - read Dataset / Work table
  - compile structured config to safe SQL
  - write managed Work table
        |
        v
Medallion catalog / lineage / audit / metrics
```

## Graph Contract

Pipeline graph は Vue Flow 互換の `nodes` / `edges` JSON として保存します。UI と backend validation / compiler が同じ contract を扱います。

```json
{
  "nodes": [
    {
      "id": "input_1",
      "type": "pipelineStep",
      "position": { "x": 80, "y": 120 },
      "data": {
        "label": "Input",
        "stepType": "input",
        "config": {
          "sourceKind": "dataset",
          "datasetPublicId": "018f2f05-c6c9-7a49-b32d-04f4dd84ef4a"
        }
      }
    }
  ],
  "edges": [
    {
      "id": "edge_input_clean",
      "source": "input_1",
      "target": "clean_1"
    }
  ]
}
```

### Graph validation

保存時、Preview 時、Run 作成時に同じ validation を通します。

- node は最大 50、edge は最大 80。
- `id` は graph 内で一意。
- `stepType` は v1 node catalog のみ許可。
- input node は必ず 1 つ。
- output node は 1 つ以上。
- directed graph は acyclic。
- input から到達不能な executable node を許可しない。
- output へ到達しない executable node を許可しない。
- edge の `source` / `target` は存在する node id のみ。
- self-loop は拒否する。
- node config は step type ごとの schema で検証する。

## v1 Node Catalog

### `input`

入力 Dataset または managed Work table を指定します。

設定:

- `sourceKind`: `dataset` または `work_table`
- `datasetPublicId`: `sourceKind=dataset` の場合に必須
- `workTablePublicId`: `sourceKind=work_table` の場合に必須

実行:

- Dataset は `raw_database.raw_table` を read source にする。
- Work table は `database.table` を read source にする。
- public ID は API 層で tenant-scoped record に解決し、compiler には ClickHouse の fully qualified table ref だけを渡す。

### `profile`

Preview / run metadata 用の profiling step です。出力 row を変えない passthrough step として扱います。

設定:

- `sampleLimit`: default `1000`, max `10000`
- `columns`: 空なら全列

収集値:

- row count
- null count / null ratio
- unique estimate
- min / max
- top values

v1 では profile result は `data_pipeline_run_steps.metadata` に保存します。

### `clean`

欠損、重複、外れ値、不正値を扱います。

設定例:

```json
{
  "rules": [
    {
      "column": "price",
      "operation": "null_if",
      "condition": { "operator": "<", "value": 0 }
    },
    {
      "operation": "dedupe",
      "keys": ["product_id"],
      "keep": "latest",
      "orderBy": "updated_at"
    }
  ]
}
```

許可 operation:

- `drop_null_rows`
- `fill_null`
- `null_if`
- `clamp`
- `dedupe`
- `trim_control_chars`

### `normalize`

表記、型、スケールを揃えます。

許可 operation:

- string: `trim`, `lowercase`, `uppercase`, `normalize_spaces`, `remove_symbols`
- numeric: `cast_decimal`, `round`, `scale`
- date: `parse_date`, `to_date`, `timezone`
- category: `map_values`

### `validate`

データ品質 rule を評価します。Validation は hard error と warning を分けます。

設定:

- `rules[].column`
- `rules[].operator`
- `rules[].value`
- `rules[].severity`: `error` または `warning`
- `rules[].message`

許可 operator:

- `required`
- `type`
- `>=`, `>`, `<=`, `<`, `=`, `!=`
- `regex`
- `in`
- `unique`

実行:

- error rule に違反する row がある場合、default は run failed。
- warning rule は run を継続し、`data_pipeline_run_steps.error_sample` と `metadata.warningCount` に保存する。
- v1 では warning を除外する自動 filter はしない。

### `schema_mapping`

入力列を target schema に対応付けます。

設定:

- `mappings[].targetColumn`
- `mappings[].sourceColumn`
- `mappings[].cast`
- `mappings[].defaultValue`
- `mappings[].required`

実行:

- target column だけを select する。
- `required=true` かつ source/default がない mapping は validation error。

### `schema_completion`

足りない列を固定値、rule、他列からの生成で補完します。

許可 method:

- `literal`
- `case_when`
- `concat`
- `coalesce`
- `copy_column`

LLM / API / 外部マスタ補完は v1 では扱いません。

### `enrich_join`

Dataset または managed Work table と join して列を追加します。

設定:

- `rightSourceKind`: `dataset` または `work_table`
- `rightDatasetPublicId` / `rightWorkTablePublicId`
- `joinType`: v1 は `left` のみ
- `leftKeys`
- `rightKeys`
- `selectColumns`
- `unmatched`: `keep_null`

制約:

- right source も同一 tenant の record のみ。
- cross tenant / external DB join は禁止。
- join key count は 1 から 5。

### `transform`

分析 / 出力用に形を変えます。

許可 operation:

- `select_columns`
- `drop_columns`
- `rename_columns`
- `derive_column`
- `filter`
- `sort`
- `aggregate`

v1 aggregate:

- group by columns
- aggregate function allowlist: `count`, `sum`, `avg`, `min`, `max`
- window function は v1 では扱わない。

### `output`

結果を書き出す managed Work table を定義します。

設定:

- `displayName`
- `tableName`: optional。未指定なら `dp_<pipeline_public_id>_<run_public_id>` 系の安全な名前を生成する。
- `writeMode`: v1 は `replace` のみ
- `engine`: v1 は `MergeTree`
- `orderBy`: optional。未指定なら最初の 1 から 3 column を使う。

実行:

- stage table に書き込み、成功後に final table として登録する。
- `dataset_work_tables` に managed Work table として登録する。
- 既存 Work table preview / export / promote / lineage から参照できる状態にする。

## PostgreSQL Schema

実装では `db/schema.sql` と `db/queries/data_pipelines.sql` を追加し、`sqlc` 生成を行います。

### `data_pipelines`

```sql
CREATE TABLE public.data_pipelines (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id uuid DEFAULT uuidv7() NOT NULL UNIQUE,
    tenant_id bigint NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    created_by_user_id bigint REFERENCES public.users(id) ON DELETE SET NULL,
    updated_by_user_id bigint REFERENCES public.users(id) ON DELETE SET NULL,
    name text NOT NULL,
    description text DEFAULT '' NOT NULL,
    status text DEFAULT 'draft' NOT NULL,
    published_version_id bigint,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    archived_at timestamptz,
    CONSTRAINT data_pipelines_name_check CHECK (btrim(name) <> ''),
    CONSTRAINT data_pipelines_status_check CHECK (status IN ('draft', 'published', 'archived'))
);

CREATE INDEX data_pipelines_tenant_updated_idx
    ON public.data_pipelines(tenant_id, updated_at DESC, id DESC)
    WHERE archived_at IS NULL;
```

### `data_pipeline_versions`

```sql
CREATE TABLE public.data_pipeline_versions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id uuid DEFAULT uuidv7() NOT NULL UNIQUE,
    tenant_id bigint NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    pipeline_id bigint NOT NULL REFERENCES public.data_pipelines(id) ON DELETE CASCADE,
    version_number integer NOT NULL,
    status text DEFAULT 'draft' NOT NULL,
    graph jsonb NOT NULL,
    validation_summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by_user_id bigint REFERENCES public.users(id) ON DELETE SET NULL,
    published_by_user_id bigint REFERENCES public.users(id) ON DELETE SET NULL,
    created_at timestamptz DEFAULT now() NOT NULL,
    published_at timestamptz,
    CONSTRAINT data_pipeline_versions_status_check CHECK (status IN ('draft', 'published', 'archived')),
    CONSTRAINT data_pipeline_versions_graph_object_check CHECK (jsonb_typeof(graph) = 'object'),
    CONSTRAINT data_pipeline_versions_version_check CHECK (version_number > 0),
    UNIQUE (pipeline_id, version_number)
);

CREATE INDEX data_pipeline_versions_pipeline_created_idx
    ON public.data_pipeline_versions(pipeline_id, created_at DESC, id DESC);
```

### `data_pipeline_runs`

```sql
CREATE TABLE public.data_pipeline_runs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id uuid DEFAULT uuidv7() NOT NULL UNIQUE,
    tenant_id bigint NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    pipeline_id bigint NOT NULL REFERENCES public.data_pipelines(id) ON DELETE CASCADE,
    version_id bigint NOT NULL REFERENCES public.data_pipeline_versions(id) ON DELETE RESTRICT,
    schedule_id bigint,
    requested_by_user_id bigint REFERENCES public.users(id) ON DELETE SET NULL,
    trigger_kind text DEFAULT 'manual' NOT NULL,
    status text DEFAULT 'pending' NOT NULL,
    output_work_table_id bigint REFERENCES public.dataset_work_tables(id) ON DELETE SET NULL,
    outbox_event_id bigint REFERENCES public.outbox_events(id) ON DELETE SET NULL,
    row_count bigint DEFAULT 0 NOT NULL,
    error_summary text,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT data_pipeline_runs_trigger_kind_check CHECK (trigger_kind IN ('manual', 'scheduled')),
    CONSTRAINT data_pipeline_runs_status_check CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'skipped')),
    CONSTRAINT data_pipeline_runs_row_count_check CHECK (row_count >= 0)
);

CREATE INDEX data_pipeline_runs_pipeline_created_idx
    ON public.data_pipeline_runs(pipeline_id, created_at DESC, id DESC);

CREATE INDEX data_pipeline_runs_active_idx
    ON public.data_pipeline_runs(tenant_id, pipeline_id, created_at DESC, id DESC)
    WHERE status IN ('pending', 'processing');
```

### `data_pipeline_run_steps`

```sql
CREATE TABLE public.data_pipeline_run_steps (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id bigint NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    run_id bigint NOT NULL REFERENCES public.data_pipeline_runs(id) ON DELETE CASCADE,
    node_id text NOT NULL,
    step_type text NOT NULL,
    status text DEFAULT 'pending' NOT NULL,
    row_count bigint DEFAULT 0 NOT NULL,
    error_summary text,
    error_sample jsonb DEFAULT '[]'::jsonb NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT data_pipeline_run_steps_node_id_check CHECK (btrim(node_id) <> ''),
    CONSTRAINT data_pipeline_run_steps_status_check CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'skipped')),
    UNIQUE (run_id, node_id)
);

CREATE INDEX data_pipeline_run_steps_run_idx
    ON public.data_pipeline_run_steps(run_id, id);
```

### `data_pipeline_schedules`

```sql
CREATE TABLE public.data_pipeline_schedules (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id uuid DEFAULT uuidv7() NOT NULL UNIQUE,
    tenant_id bigint NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    pipeline_id bigint NOT NULL REFERENCES public.data_pipelines(id) ON DELETE CASCADE,
    version_id bigint NOT NULL REFERENCES public.data_pipeline_versions(id) ON DELETE RESTRICT,
    created_by_user_id bigint REFERENCES public.users(id) ON DELETE SET NULL,
    frequency text NOT NULL,
    timezone text NOT NULL,
    run_time text NOT NULL,
    weekday smallint,
    month_day smallint,
    enabled boolean DEFAULT true NOT NULL,
    next_run_at timestamptz NOT NULL,
    last_run_at timestamptz,
    last_status text,
    last_error_summary text,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT data_pipeline_schedules_frequency_check CHECK (frequency IN ('daily', 'weekly', 'monthly')),
    CONSTRAINT data_pipeline_schedules_frequency_shape_check CHECK (
        (frequency = 'daily' AND weekday IS NULL AND month_day IS NULL)
        OR (frequency = 'weekly' AND weekday IS NOT NULL AND month_day IS NULL)
        OR (frequency = 'monthly' AND weekday IS NULL AND month_day IS NOT NULL)
    ),
    CONSTRAINT data_pipeline_schedules_run_time_check CHECK (run_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    CONSTRAINT data_pipeline_schedules_weekday_check CHECK (weekday IS NULL OR weekday BETWEEN 1 AND 7),
    CONSTRAINT data_pipeline_schedules_month_day_check CHECK (month_day IS NULL OR month_day BETWEEN 1 AND 28)
);

CREATE INDEX data_pipeline_schedules_due_idx
    ON public.data_pipeline_schedules(next_run_at, id)
    WHERE enabled;

CREATE INDEX data_pipeline_schedules_pipeline_idx
    ON public.data_pipeline_schedules(pipeline_id, updated_at DESC, id DESC);
```

### medallion 変更

`medallion_pipeline_runs_pipeline_type_check` に `data_pipeline` を追加します。service constant も追加します。

```go
const MedallionPipelineDataPipeline = "data_pipeline"
```

Run 記録:

- `PipelineType`: `data_pipeline`
- `SourceResourceKind`: input source に応じて `dataset` または `work_table`
- `TargetResourceKind`: `work_table`
- `Runtime`: `clickhouse`
- `TriggerKind`: `manual` または `scheduled`

## SQLC Queries

`db/queries/data_pipelines.sql` を追加します。主な query は次の通りです。

- `CreateDataPipeline`
- `ListDataPipelines`
- `GetDataPipelineForTenant`
- `UpdateDataPipeline`
- `ArchiveDataPipeline`
- `CreateDataPipelineVersion`
- `GetDataPipelineVersionForTenant`
- `ListDataPipelineVersions`
- `PublishDataPipelineVersion`
- `SetDataPipelinePublishedVersion`
- `CreateDataPipelineRun`
- `GetDataPipelineRunForTenant`
- `ListDataPipelineRuns`
- `MarkDataPipelineRunProcessing`
- `CompleteDataPipelineRun`
- `FailDataPipelineRun`
- `CreateDataPipelineRunStep`
- `MarkDataPipelineRunStepProcessing`
- `CompleteDataPipelineRunStep`
- `FailDataPipelineRunStep`
- `CreateDataPipelineSchedule`
- `ListDataPipelineSchedules`
- `UpdateDataPipelineSchedule`
- `DisableDataPipelineSchedule`
- `ClaimDueDataPipelineSchedules`
- `CountActiveDataPipelineRunsForSchedule`
- `MarkDataPipelineScheduleCreated`
- `MarkDataPipelineScheduleSkipped`
- `MarkDataPipelineScheduleFailed`

`ClaimDueDataPipelineSchedules` は既存 work table export schedule と同じく `FOR UPDATE SKIP LOCKED` を使います。

```sql
-- name: ClaimDueDataPipelineSchedules :many
SELECT *
FROM data_pipeline_schedules
WHERE enabled
  AND next_run_at <= sqlc.arg(now)::timestamptz
ORDER BY next_run_at, id
LIMIT sqlc.arg(batch_limit)
FOR UPDATE SKIP LOCKED;
```

## Backend Service

`backend/internal/service/data_pipeline_service.go` を追加します。

主要 type:

- `DataPipeline`
- `DataPipelineVersion`
- `DataPipelineRun`
- `DataPipelineRunStep`
- `DataPipelineSchedule`
- `DataPipelineGraph`
- `DataPipelineNode`
- `DataPipelineEdge`
- `DataPipelinePreview`

主要 method:

```go
func (s *DataPipelineService) List(ctx context.Context, tenantID int64, limit int32) ([]DataPipeline, error)
func (s *DataPipelineService) Create(ctx context.Context, tenantID, userID int64, input DataPipelineInput, auditCtx AuditContext) (DataPipeline, error)
func (s *DataPipelineService) Get(ctx context.Context, tenantID int64, publicID string) (DataPipelineDetail, error)
func (s *DataPipelineService) SaveDraftVersion(ctx context.Context, tenantID, userID int64, pipelinePublicID string, graph DataPipelineGraph, auditCtx AuditContext) (DataPipelineVersion, error)
func (s *DataPipelineService) PublishVersion(ctx context.Context, tenantID, userID int64, versionPublicID string, auditCtx AuditContext) (DataPipelineVersion, error)
func (s *DataPipelineService) Preview(ctx context.Context, tenantID int64, versionPublicID, nodeID string, limit int32) (DataPipelinePreview, error)
func (s *DataPipelineService) RequestRun(ctx context.Context, tenantID int64, userID *int64, versionPublicID string, triggerKind string, scheduleID *int64, auditCtx AuditContext) (DataPipelineRun, error)
func (s *DataPipelineService) HandleRunRequested(ctx context.Context, tenantID, runID, outboxEventID int64) error
func (s *DataPipelineService) RunDueSchedules(ctx context.Context, now time.Time, batchSize int32) (DataPipelineScheduleRunSummary, error)
```

`RequestRun` は transaction 内で run を作成し、outbox event を enqueue します。

```json
{
  "tenantId": 1,
  "runId": 123,
  "pipelinePublicId": "018f2f05-c6c9-7a49-b32d-04f4dd84ef4a",
  "versionPublicId": "018f2f05-c6c9-7a49-b32d-04f4dd84ef4b"
}
```

## SQL Compiler

Compiler は raw user SQL を保存・実行しません。構造化 config だけから ClickHouse SQL を生成します。

実装ファイル:

- `backend/internal/service/data_pipeline_graph.go`
- `backend/internal/service/data_pipeline_compile.go`
- `backend/internal/service/data_pipeline_execute.go`
- `backend/internal/service/data_pipeline_schedule.go`

### Safety rules

- ClickHouse identifier は既存 `quoteCHIdent` 相当の helper を使う。
- column / table / database name は metadata から解決したものだけ許可する。
- user input の identifier は allowlist regex と metadata 照合を通す。
- literal は query string に直結せず、型ごとの escape / bind 相当 helper を使う。
- operation / function / cast / operator は enum allowlist のみ。
- external function pattern、system database、tenant 外 database は拒否する。
- Preview / Run は既存 `querySettings()` を使い、max seconds / memory / rows / threads を適用する。

### Intermediate model

各 node は upstream relation から SQL fragment を生成し、compiler は topological order で CTE を作ります。

```sql
WITH
step_input_1 AS (
  SELECT *
  FROM `hh_t_1_raw`.`ds_...`
),
step_clean_1 AS (
  SELECT
    *,
    if(`price` < 0, NULL, `price`) AS `price`
  FROM step_input_1
)
SELECT *
FROM step_clean_1
LIMIT 100
```

Run は最終 output ごとに `CREATE TABLE ... AS SELECT` または `INSERT INTO stage SELECT` を使い、成功後に managed Work table として登録します。

## API

`backend/internal/api/data_pipelines.go` を追加し、`register.go` から登録します。OpenAPI tag は `Data & Datasets` を使うか、必要なら `Data Pipelines` tag を追加します。

### Endpoint

| Method | Path | Operation ID | Purpose |
| --- | --- | --- | --- |
| GET | `/api/v1/data-pipelines` | `listDataPipelines` | pipeline list |
| POST | `/api/v1/data-pipelines` | `createDataPipeline` | pipeline create |
| GET | `/api/v1/data-pipelines/{pipelinePublicId}` | `getDataPipeline` | detail |
| PATCH | `/api/v1/data-pipelines/{pipelinePublicId}` | `updateDataPipeline` | name / description update |
| POST | `/api/v1/data-pipelines/{pipelinePublicId}/versions` | `saveDataPipelineVersion` | draft graph save |
| POST | `/api/v1/data-pipeline-versions/{versionPublicId}/publish` | `publishDataPipelineVersion` | publish |
| POST | `/api/v1/data-pipeline-versions/{versionPublicId}/preview` | `previewDataPipelineVersion` | selected node preview |
| GET | `/api/v1/data-pipelines/{pipelinePublicId}/runs` | `listDataPipelineRuns` | run history |
| POST | `/api/v1/data-pipeline-versions/{versionPublicId}/runs` | `createDataPipelineRun` | manual run |
| GET | `/api/v1/data-pipelines/{pipelinePublicId}/schedules` | `listDataPipelineSchedules` | schedule list |
| POST | `/api/v1/data-pipelines/{pipelinePublicId}/schedules` | `createDataPipelineSchedule` | schedule create |
| PATCH | `/api/v1/data-pipeline-schedules/{schedulePublicId}` | `updateDataPipelineSchedule` | schedule update |
| DELETE | `/api/v1/data-pipeline-schedules/{schedulePublicId}` | `disableDataPipelineSchedule` | schedule disable |

### Auth / role

`requireDataPipelineTenant` helper を追加します。

```go
func requireDataPipelineTenant(ctx context.Context, deps Dependencies, sessionID, csrfToken string) (service.CurrentSession, service.TenantAccess, error) {
    if deps.DataPipelineService == nil {
        return service.CurrentSession{}, service.TenantAccess{}, huma.Error503ServiceUnavailable("data pipeline service is not configured")
    }
    return requireActiveTenantRole(ctx, deps, sessionID, csrfToken, "data_pipeline_user", "data pipeline service")
}
```

GET は CSRF なし、mutation は CSRF 必須。create / run は `Idempotency-Key` に対応します。

## Scheduler / Outbox

### Outbox handler

`DefaultOutboxHandler` に `*DataPipelineService` を追加し、event を処理します。

```go
case "data_pipeline.run_requested":
    var payload struct {
        TenantID int64 `json:"tenantId"`
        RunID    int64 `json:"runId"`
    }
    ...
    return h.dataPipelines.HandleRunRequested(ctx, payload.TenantID, payload.RunID, event.ID)
```

### Schedule job

`backend/internal/jobs/data_pipeline_scheduler.go` を追加します。構造は `WorkTableExportScheduleJob` と同じです。

Config:

- `DATA_PIPELINE_SCHEDULER_ENABLED`: default `true`
- `DATA_PIPELINE_SCHEDULER_INTERVAL`: default `1m`
- `DATA_PIPELINE_SCHEDULER_TIMEOUT`: default `30s`
- `DATA_PIPELINE_SCHEDULER_BATCH_SIZE`: default `20`
- `DATA_PIPELINE_SCHEDULER_RUN_ON_STARTUP`: default `true`

Due schedule 処理:

1. `ClaimDueDataPipelineSchedules` で due schedule を claim。
2. published version が存在しない場合は schedule disabled。
3. 同じ schedule の active run がある場合は skipped にして next run を更新。
4. active run がなければ run を作成し、outbox event を enqueue。
5. next run を daily / weekly / monthly rule で更新。

## Frontend

### Routing / navigation

- `frontend/src/router/index.ts` に `/data-pipelines` route を追加する。
- `frontend/src/App.vue` の `wideMain` に `/data-pipelines` を追加する。
- `frontend/src/components/AppSidebar.vue` の Work group に `Data Pipelines` を追加する。icon は `Workflow` または `GitBranch` from `lucide-vue-next`。
- i18n は `nav.items.dataPipelines`, `routes.dataPipelines`, `dataPipelines.*` を追加する。

### Files

- `frontend/src/views/DataPipelinesView.vue`
- `frontend/src/components/DataPipelineFlowBuilder.vue`
- `frontend/src/components/DataPipelineNode.vue`
- `frontend/src/components/DataPipelineInspector.vue`
- `frontend/src/components/DataPipelinePreviewPanel.vue`
- `frontend/src/stores/data-pipelines.ts`
- `frontend/src/api/data-pipelines.ts`

### Layout

```text
/data-pipelines

Page header
  - title
  - active tenant
  - refresh
  - new pipeline

Main
  Left: pipeline list + node palette
  Center: Vue Flow canvas
  Right: selected node config inspector
  Bottom: preview / validation / run history / schedules
```

### Vue Flow usage

Use the existing import pattern:

```ts
import { VueFlow, type Connection } from '@vue-flow/core'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'
import '@vue-flow/minimap/dist/style.css'
```

UI behavior:

- Palette button adds node with stable `id` and default config.
- `onConnect` creates edge after local validation.
- Selecting node opens inspector.
- Save draft posts graph to backend.
- Publish validates and locks run target version.
- Preview posts selected node id and shows columns / rows / warnings.
- Run creates manual run for published version.
- Schedule panel creates daily / weekly / monthly schedule.

Do not build a marketing landing page. The first screen is the actual builder / management UI.

## Observability / Audit

Audit actions:

- `data_pipeline.create`
- `data_pipeline.update`
- `data_pipeline.version.save`
- `data_pipeline.version.publish`
- `data_pipeline.run.request`
- `data_pipeline.run.complete`
- `data_pipeline.schedule.create`
- `data_pipeline.schedule.update`
- `data_pipeline.schedule.disable`

Metrics:

- run duration by status / trigger
- run count by status / trigger
- step count by step type / status
- scheduler claimed / created / skipped / failed / disabled
- preview duration / failure count

Do not include tenant slug, user email, public ID, SQL text, file path, raw row values, or error sample in metric labels.

## Testing Plan

### Backend unit tests

- graph validation: missing input, multiple input, no output, cycle, orphan node, too many nodes / edges.
- config validation for each step type.
- unsafe identifier rejection.
- SQL compiler allowlist: operators, casts, functions.
- tenant boundary: cannot reference Dataset / Work table from another tenant.
- schedule next run: daily / weekly / monthly, invalid timezone, invalid weekday / month day.
- active run skip for schedule.

### Service / job tests

- manual run creates pending run and outbox event.
- outbox handler marks run processing / completed.
- failed ClickHouse execution records run and step failure.
- schedule claim uses due enabled schedules only.
- schedule disables invalid or unpublished pipeline.
- medallion run is recorded on processing / completed / failed.

### API tests

- `403` without `data_pipeline_user`.
- `409` without active tenant.
- `400` invalid graph.
- create / list / detail / save version / publish.
- preview success and invalid selected node.
- manual run idempotency replay.
- schedule create / update / disable.

### Frontend tests

- `/data-pipelines` renders for authorized tenant.
- role denied state renders for missing role.
- add node, connect edge, edit config, save draft.
- publish validation errors show next to builder action.
- preview selected node renders table.
- manual run appears in run history.
- schedule create / disable flow.
- tenant switch resets selected pipeline and reloads list.

### Verification commands

```bash
go test ./backend/...
cd frontend && npm run build
cd frontend && npm run openapi-ts
cd frontend && npm run e2e -- data-pipelines
```

## Rollout

1. Add role / schema / sqlc queries behind backend tests.
2. Add service validation and compiler without enabling UI.
3. Add API and OpenAPI generated client.
4. Add frontend builder and route.
5. Add scheduler and outbox handling.
6. Add medallion integration and observability.
7. Run smoke tests with one Dataset input and one Work table output.

## Future Phases

- DuckDB runtime for file / Parquet preprocessing.
- External source input: S3 / GCS / PostgreSQL / MySQL.
- Parquet output and file export integration.
- LLM / API enrichment node.
- Backfill UI and historical schedule replay.
- Column-level lineage from pipeline graph.
- Reusable pipeline templates.
- Multi-output graph execution.
