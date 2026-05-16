# Agent Knowledge Index

作成日: 2026-05-16

## 目的

この文書は、HaoHao repository を別セッションの AI agent / 開発者が扱うときの入口です。docs が多いため、作業タイプごとに最初に読むべき文書、見るべきコード、最低限の検証コマンドをまとめます。

原則:

- まず `git status --short` を確認する。
- 既存のユーザー変更や未追跡ファイルは勝手に削除しない。
- GitHub 情報が必要な場合は `gh` を優先する。
- Data Pipeline / Drive / Dataset / Gold は DB、API、frontend、ClickHouse、authorization が絡むため、単一ファイルだけで判断しない。

## 共通で読むもの

最初に読む:

- `AGENTS.md`
- `docs/NEXT_IMPLEMENTATION_PLAN.md`
- `docs/DATA_PIPELINE_RETROSPECTIVE_2026-05-16.md`
- `docs/DATA_PIPELINE_SESSION_HANDOFF.md`

共通コマンド:

```bash
git status --short
go test ./backend/...
npm --prefix frontend run build
git diff --check
```

OpenAPI / generated type を触った場合:

```bash
make gen
go test ./backend/internal/api
npm --prefix frontend run build
```

## Data Pipeline Runtime を触る場合

読む docs:

- `docs/data-pipeline-current-state.md`
- `docs/DATA_PIPELINE_IMPLEMENTATION_PLAN.md`
- `docs/DATA_PIPELINE_RETROSPECTIVE_2026-05-16.md`
- `docs/DATA_PIPELINE_VALIDATION_ENDPOINT_PLAN.md`

見るコード:

- `backend/internal/service/data_pipeline_graph.go`
- `backend/internal/service/data_pipeline_compile.go`
- `backend/internal/service/data_pipeline_unstructured.go`
- `backend/internal/service/data_pipeline_service.go`
- `backend/internal/service/data_pipeline_output_schema.go`
- `db/queries/data_pipelines.sql`
- `db/migrations/0040_data_pipelines.up.sql`
- `db/migrations/0041_data_pipeline_run_outputs.up.sql`

検証:

```bash
go test ./backend/internal/service
go test ./backend/...
make smoke-data-pipeline-suite
```

注意:

- `data_pipeline_run_steps.metadata` は step 単位の実行結果 summary。
- `data_pipeline_run_outputs.metadata` は output 成果物の summary。
- ClickHouse の行データ列は後続 node が読む判定や注釈。
- metadata と行データ列を混同しない。

## Data Pipeline UI / Inspector を触る場合

読む docs:

- `docs/DATA_PIPELINE_UI_COLUMN_INFERENCE.md`
- `docs/DATA_PIPELINE_VALIDATION_ENDPOINT_PLAN.md`
- `docs/DATA_PIPELINE_RETROSPECTIVE_2026-05-16.md`

見るコード:

- `frontend/src/views/DataPipelineDetailView.vue`
- `frontend/src/components/DataPipelineFlowBuilder.vue`
- `frontend/src/components/DataPipelineInspector.vue`
- `frontend/src/components/DataPipelinePreviewPanel.vue`
- `frontend/src/stores/data-pipelines.ts`
- `frontend/src/api/data-pipelines.ts`
- `frontend/src/i18n/messages.ts`

検証:

```bash
npm --prefix frontend run build
git diff --check
```

注意:

- Inspector の missing-column warning は backend validation result を primary source にする。
- frontend fallback 推論は endpoint が取得できない場合の保険。
- 新しい node / 出力列を追加したら、runtime 出力、validation endpoint、Inspector fallback、smoke を同時に確認する。
- `TestInferOutputSchemasCoversEveryCatalogStep` は `dataPipelineStepCatalog` の全 step type が backend `inferOutputSchemas` で non-empty schema を返すことを確認する。step 追加時にこのテストが落ちた場合は、frontend fallback より先に backend schema 推論を更新する。

## SCD2 / Snapshot / Work table を触る場合

読む docs:

- `docs/DATA_PIPELINE_RETROSPECTIVE_2026-05-16.md`
- `docs/DATA_PIPELINE_SESSION_HANDOFF.md`
- `docs/data-pipeline-current-state.md`

見るコード:

- `backend/internal/service/data_pipeline_compile.go`
- `backend/internal/service/data_pipeline_service.go`
- `backend/internal/service/dataset_service.go`
- `backend/internal/api/datasets.go`
- `frontend/src/components/DatasetWorkTableBrowser.vue`
- `frontend/src/views/DatasetGoldDetailView.vue`

検証:

```bash
go test ./backend/internal/service ./backend/internal/api
make smoke-data-pipeline-snapshot-scd2
make smoke-data-pipeline-snapshot-append
make smoke-data-pipeline-snapshot-merge
make smoke-data-pipeline-snapshot-merge-backfill
npm --prefix frontend run build
```

注意:

- `current_only` merge は stage 側の `is_current=1` だけを見る。
- `rebuild_key_history` は stage に含まれる key の履歴全体を再計算する。
- `deleteDetection=close_current` と `sameValidFromPolicy=reject` は backend / smoke / Output 設定 UI まで実装済み。
- composite key drilldown は `scd2Summary.keyColumns` と `scd2-history?keyColumns=...&keyValues=...` で実装済み。

## Gold / Dataset / Lineage を触る場合

読む docs:

- `docs/DATA_PIPELINE_RETROSPECTIVE_2026-05-16.md`
- `docs/data-pipeline-current-state.md`
- `docs/DATA_LINEAGE_DEPENDENCY_GRAPH.md`

見るコード:

- `backend/internal/service/dataset_gold_service.go`
- `backend/internal/api/dataset_gold_publications.go`
- `backend/internal/service/dataset_service.go`
- `backend/internal/service/medallion_catalog_service.go`
- `frontend/src/views/DatasetGoldDetailView.vue`
- `frontend/src/views/DatasetsView.vue`
- `frontend/src/components/DatasetWorkTableBrowser.vue`

検証:

```bash
go test ./backend/internal/service ./backend/internal/api
npm --prefix frontend run build
```

注意:

- Gold publication は Work table 起点。
- Data Pipeline output は managed Work table として登録されるため、Gold とは Work table を介してつながる。
- Gold detail は `sourceScd2Summary`、`sourceDataPipelineRun`、`sourceDataPipelineRun.qualitySummary` を表示する。
- Gold publish history の各 run row も `sourceDataPipelineRun` を返し、UI から同期元 Data Pipeline detail の該当 `runPublicId` / `outputNodeId` へ戻れる。
- `dataset_gold_publish_runs.source_data_pipeline_run_id` / `source_data_pipeline_run_output_id` を追加済み。新規 Gold publish run は作成時点の source Data Pipeline run/output を保存し、表示時は保存済み output を優先する。既存行や参照欠落時だけ `source_work_table_id` から最新 completed output へ fallback する。
- Data Pipeline output row は `latestGoldPublication` から Gold detail へ進める。

## Drive / OCR / Product Extraction を触る場合

読む docs:

- `docs/TUTORIAL_P19_DRIVE_LOCAL_OCR_PRODUCT_EXTRACTION.md`
- `docs/TUTORIAL_OPENFGA_P9_DRIVE_PRODUCT_COMPLETION.md`
- `docs/RUNBOOK_DRIVE_PADDLEOCR.md`
- `docs/RUNBOOK_DRIVE_LMSTUDIO_OCR.md`

使う skill:

- `.agents/skills/haohao-drive-debug`

見るコード:

- `backend/internal/service/drive_*.go`
- `backend/internal/service/drive_ocr_service.go`
- `backend/internal/service/drive_product_extraction*.go`
- `frontend/src/views/DriveView.vue`
- `frontend/src/components/DriveFileDetailPage.vue`

検証:

```bash
go test ./backend/internal/service ./backend/internal/api
npm --prefix frontend run build
```

注意:

- Drive visibility 問題は DB row、workspace/folder、API response、authorization filtering、frontend filters の順で見る。
- OpenFGA / Dataset resource permission と DB tenant guard を混同しない。

## Local Search / RAG を触る場合

読む docs:

- `docs/VECTOR_SEARCH_RAG_IMPLEMENTATION_PLAN.md`
- `docs/DRIVE_AGENTIC_RAG_IMPLEMENTATION_PLAN.md`
- `docs/RUNBOOK_VECTOR_SEARCH_RAG.md`
- `docs/RUNBOOK_OPENWEBUI_RURI_EMBEDDINGS.md`

見るコード:

- `backend/internal/service/local_search_service.go`
- `backend/internal/service/drive_rag_service.go`
- `backend/internal/service/tenant_settings_service.go`
- `frontend/src/views/DriveView.vue`
- `frontend/src/views/tenant-admin/TenantAdminTenantDrivePolicyView.vue`

注意:

- RAG smoke は local model latency と outbox indexing readiness に左右される。
- policy disabled、runtime unavailable、index 未完了、citation 不足を分けて見る。
- 外部 cloud AI / OCR API は既定で使わない。local runtime と tenant policy を優先する。

## OpenFGA / Authorization を触る場合

読む docs:

- `docs/DRIVE_OPENFGA_PERMISSIONS_SPEC.md`
- `docs/OPENFGA_IMPLEMENTATION_PLAN.md`
- `docs/runbooks/drive-openfga-dr.md`

見るコード:

- `backend/internal/service/authorization*.go`
- `backend/internal/service/dataset_authorization_service.go`
- `backend/internal/service/drive_authorization_service.go`
- `backend/internal/service/openfga*.go`

注意:

- tenant-aware table は必ず `tenant_id` で絞る。
- tenant 外 resource は原則 404 として隠す。
- Drive file body、OCR text、embedding、RAG context、product extraction は source file 権限なしに読めない前提。

## DB / sqlc / migration を触る場合

読む docs:

- `docs/RUNBOOK_OPERABILITY.md`
- `docs/data-pipeline-current-state.md`

使う skill:

- `.agents/skills/haohao-db-dev`
- `.agents/skills/supabase-postgres-best-practices`

見る場所:

- `db/migrations/*.sql`
- `db/queries/*.sql`
- `backend/internal/db/*.go`

検証:

```bash
make gen
go test ./backend/...
```

注意:

- migration を追加したら down migration も確認する。
- sqlc query を変えたら generated code と API / service call site を同時に確認する。

## Frontend visual / browser check を触る場合

読む docs:

- `docs/TUTORIAL_P9_UI_PLAYWRIGHT_E2E.md`
- `docs/DATA_PIPELINE_RETROSPECTIVE_2026-05-16.md`

検証:

```bash
npm --prefix frontend run build
```

browser 確認:

- local target は `agent-browser` を使う。
- `make up`
- `make backend-dev`
- `make frontend-dev`

注意:

- local login / OIDC mode の違いで browser smoke が失敗することがある。
- UI smoke は API smoke より session state に依存する。
- Data Pipeline detail / Gold detail / Work table preview は、できれば固定 sample public ID を使って確認する。

## 現在の代表未完了リスト

Data Pipeline:

- `mark_deleted` policy。
- `latest_ingested_wins` などの高度 winner policy。
- backend step catalog / generated output schema contract。
- `validate` status column と quarantine 連携。

Gold / Lineage:

- Gold publish history と Data Pipeline run history の相互リンクは、publish history row から source run/output への deep link と publish run 作成時点の source run/output ID 永続化まで完了。既存行の backfill は必要に応じて行う。
- source run step detail を Gold detail から直接見るかどうかの設計。

Review:

- review item assignee。
- review 修正値の再投入 run。
- review 履歴 UI。

AI coding:

- `haohao-data-pipeline-debug` skill。
- `haohao-openapi-gen` skill。
- smoke 結果を artifact / docs へ残す仕組み。
- local browser smoke session の標準化。
