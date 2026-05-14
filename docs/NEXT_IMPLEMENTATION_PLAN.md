# HaoHao 次期実装計画

作成日: 2026-05-13

## Summary

HaoHao は、基盤 SaaS から **Drive + Dataset + Data Pipeline + Local Search/RAG のローカル完結データ基盤**へ重心が移っています。次の 6 か月は、機能数を増やすよりも、Drive / Dataset から作った Pipeline を品質検査、隔離、レビュー、スケジュール、Gold publish まで安全に運用できる状態へ仕上げます。

現状確認では、次が通過済みです。

```bash
go test ./backend/...
npm --prefix frontend run build
```

frontend build では Monaco / Data Pipeline detail 周辺を中心に大きい chunk の警告があります。機能 blocker ではありませんが、Month 6 の運用品質改善で扱います。

## この文書だけで引き継ぐための前提

この文書は、別セッションや別エージェントが最初に読む「次に何をやるべきか」の入口です。詳細な実装仕様や過去のチュートリアルは各 docs に残っていますが、次の判断に必要な現状、課題、根拠、ファイル位置はこの文書に集約します。

この調査で確認した重要な結論:

- 既存の foundation は作り直さない。OpenAPI 3.1、Monorepo、Go/Huma/Gin、Vue/Vite、PostgreSQL/sqlc、Redis、OpenFGA、ClickHouse、SeaweedFS、単一バイナリ配信はすでに実装済みの前提で進める。
- 今後の中心は、Drive、Dataset、Data Pipeline、Medallion、Gold、Local Search、RAG です。
- Data Pipeline は多くの node が catalog / UI に存在しますが、すべてが同じ深さで実行・監視できるわけではありません。Month 1 の品質・可観測性の土台は `b9d5c03`、`0160678`、`c54de44` で完了済みです。
- `DataPipelineRunStepBody.metadata` は API contract として使われ、`profile`、`validate`、`quality_report`、`confidence_gate`、`inputRows`、`samples`、`queryStats`、Runs tab 詳細、Data Pipeline smoke suite まで実装済みです。
- Drive OCR、商品抽出、Local Search、pgvector、Drive RAG は主要な足場が動いています。次は broad な機能追加ではなく、評価、追跡、失敗理由、運用導線を強くします。
- frontend は機能がかなり増え、Data Pipeline detail と Monaco editor の chunk が大きくなっています。運用前に code splitting を改善する余地があります。
- AI coding 改善は未実装の提案段階です。`docs/HARNESS_ENGINEERING_ADOPTION_ANALYSIS.md` に方針はありますが、`docs/AGENT_KNOWLEDGE_INDEX.md` や RAG/OpenFGA/OpenAPI/frontend 用 skill はまだありません。

## 調査根拠

この計画は、次の repo 実態を確認して作成しました。

確認した主要 docs:

- `README.md`
- `docs/CONCEPT.md`
- `IMPL.md`
- `FUTURE_FEATURES.md`
- `docs/data-pipeline-current-state.md`
- `docs/data-pipeline-llm-node.md`
- `docs/VECTOR_SEARCH_RAG_IMPLEMENTATION_PLAN.md`
- `docs/DRIVE_AGENTIC_RAG_IMPLEMENTATION_PLAN.md`
- `docs/DATA_LINEAGE_DEPENDENCY_GRAPH.md`
- `docs/HARNESS_ENGINEERING_ADOPTION_ANALYSIS.md`
- `docs/TUTORIAL_P16_DRIVE_FEATURE_COMPLETION.md`
- `docs/TUTORIAL_P19_DRIVE_LOCAL_OCR_PRODUCT_EXTRACTION.md`

確認した主要コード領域:

- `backend/internal/service/data_pipeline_*.go`
- `backend/internal/api/data_pipelines.go`
- `frontend/src/views/DataPipelinesView.vue`
- `frontend/src/views/DataPipelineDetailView.vue`
- `frontend/src/components/DataPipeline*.vue`
- `backend/internal/service/drive_*.go`
- `backend/internal/service/local_search_service.go`
- `backend/internal/service/drive_rag_service.go`
- `backend/internal/service/dataset_lineage*.go`
- `backend/internal/service/dataset_gold_service.go`
- `backend/internal/api/register.go`
- `backend/cmd/main/main.go`
- `.github/workflows/ci.yml`
- `Makefile`

確認時点の repo 規模:

- tracked files: 約 807
- `backend/internal` の Go files: 約 274
- `backend/internal` の Go test files: 約 58
- `frontend/src` の Vue files: 約 100
- Playwright spec files: 5
- DB migration up files: 47

確認したコマンド:

```bash
go test ./backend/...
npm --prefix frontend run build
git status --short
git diff --check -- docs/NEXT_IMPLEMENTATION_PLAN.md
```

確認時点の作業ツリー:

- `.tool-versions` が削除状態。これは既存のユーザー変更として扱い、この計画作成では触っていません。
- `docs/NEXT_IMPLEMENTATION_PLAN.md` はこの計画書として追加した新規ファイルです。

## 現在のアーキテクチャ

HaoHao の基本方針は **OpenAPI 3.1 優先 + Monorepo + 単一バイナリ配信**です。

主要技術:

- Frontend: Vue 3、Vite、TypeScript、Pinia、vue-router、vue-i18n、Vue Flow、Monaco、lucide-vue-next
- Backend: Go、Gin、Huma
- API contract: Huma から OpenAPI 3.1 を生成し、`full` / `browser` / `external` の 3 surface に分割
- DB: PostgreSQL 18、pgx、sqlc
- Analytics: ClickHouse
- Auth: local password login と Zitadel OIDC
- Session / rate limit: Redis
- Authorization: OpenFGA、主に Drive / Dataset resource authorization
- File storage: local filesystem と SeaweedFS S3-compatible
- Search / RAG: PostgreSQL full-text、pgvector、local embedding / generation runtime
- Distribution: Vue dist と docs を Go binary に embed する単一バイナリ

主要 runtime wiring:

- `backend/cmd/main/main.go` が PostgreSQL、Redis、ClickHouse、file storage、OpenFGA、Drive、OCR、Dataset、Medallion、Data Pipeline、Local Search、Realtime、Outbox、Schedulers を組み立てます。
- `backend/internal/api/register.go` が browser / external surface に応じて Huma operation を登録します。
- frontend generated client は `openapi/browser.yaml` を正本にします。
- `scripts/gen.sh` / `make gen` は sqlc、OpenAPI、frontend SDK をまとめて更新します。

## 現在実装済みの主要領域

Foundation:

- Cookie session、CSRF、local login、Zitadel login、external bearer、M2M、SCIM、delegated OAuth が実装済み。
- Tenant、tenant admin、membership、role、support access、rate limit runtime override が実装済み。
- Structured logs、health/readiness、Prometheus metrics、OpenTelemetry tracing、alert rules、runbook がある。
- Outbox、idempotency、notifications、invitations、webhooks、file lifecycle purge がある。

Drive:

- File/folder CRUD、workspace、trash/restore、sharing、share links、external invitations、groups、OpenFGA authorization がある。
- Shared、Starred、Recent、Storage、Trash、Search の route / UI は存在します。
- Drive OCR、OCR pages、product extraction、OCR status UI、product extraction table が存在します。
- Local Search / semantic / hybrid search と RAG panel が入っています。
- Tenant Admin Drive Policy で OCR / RAG / local runtime 関連の policy を扱えます。

Dataset / Work table / Gold:

- Drive file から Dataset import ができます。
- ClickHouse raw / work database を tenant ごとに使います。
- Work table management、rename、truncate、drop、promote、export、scheduled export、sync job、Gold publication が実装されています。
- Dataset lineage v1/v2 があり、metadata lineage、parser/manual change set、parse run の足場があります。

Data Pipeline:

- `/data-pipelines` と `/data-pipelines/:pipelinePublicId` があり、Vue Flow graph builder、inspector、preview、run、schedule を扱います。
- Graph は node / edge の DAG として保存され、published version に対して run / schedule できます。
- structured path は ClickHouse SQL CTE に compile します。
- Drive OCR、JSON、Excel、extract 系 node を含む場合は hybrid path で ClickHouse の中間 table に materialize します。
- Run は outbox event `data_pipeline.run_requested` で非同期実行されます。
- `data_pipeline_run_outputs` により複数 output node に対応しています。
- schema mapping candidate、mapping example、tenant shared evidence、pgvector-backed search document rebuild が実装済みです。

Local Search / RAG:

- `local_search_documents` と `local_search_embeddings` があり、pgvector 1024 dimension 前提です。
- resource kind は Drive file、OCR run、product extraction、Gold publication、schema column、mapping example を扱います。
- Tenant policy で local search vector と Drive RAG を明示有効化します。
- Drive RAG は query planning、multi-query retrieval、rerank、sufficiency retry、citation guard、fallback answer を持っています。
- 標準検証 model は文書上 `text-embedding-mxbai-embed-large-v1` が推奨されています。`ruri-v3-310m` は LM Studio routing が安定せず標準から外されています。

## 現在の課題

### Data Pipeline の課題

最重要課題は、処理結果の信頼性と説明可能性です。

- structured compiler では `profile`、`validate`、`output` は行データ上 passthrough です。run executor は `profile` / `validate` の実測 summary を metadata に保存します。
- `quality_report` や `confidence_gate` は hybrid path で行データ列を追加し、run step metadata summary も保存します。
- `HandleRunRequested` は node ごとの実測 metadata を step completion に渡す実装へ進んでいます。
- join / enrich_join は行数爆発、未マッチ、key null、列衝突などの warning を UI で十分説明できていません。
- `human_review` は `createReviewItems=true` の場合に永続 review item を作成できます。Drive text / `extract_fields` / `extract_table`、Drive JSON / `schema_mapping`、Drive product extraction の低信頼 reason は review queue へ接続済みです。Drive file detail から source file に紐づく review item / pipeline run へ戻る導線も追加済みです。
- `quarantine` v1 は実装済みです。`union`、`route_by_condition`、`partition_filter`、`watermark_filter`、`snapshot_scd2` はまだ実装されていません。

次に着手すべき理由:

- 既存 UI / DB / API は metadata を受ける余地があります。
- ここを強くすると、OCR、LLM/RAG、schema mapping、Gold publish などの後続機能が安全になります。
- 新しい node を増やす前に、失敗理由と品質を追えるようにする方が運用価値が高いです。

### Drive / RAG の課題

- RAG の主要足場は動いていますが、broad smoke は local model latency と outbox/index readiness に左右されます。
- Retrieval trace は service 内にありますが、通常 UI / admin debug の導線はまだ限定的です。
- policy disabled、runtime unavailable、index 未完了、citation 不足などの原因を、ユーザーや運用者が一目で分かる表示にする余地があります。
- Drive の日常操作はかなり実装されていますが、P16 の文書には preview / thumbnail、bulk、copy、permanent delete、storage usage などの完成観点が残っています。現コードには route / UI があるものの、全導線が同じ成熟度とは限りません。

### Gold / Lineage の課題

- Metadata lineage と parser/manual lineage change set の両方が存在します。
- 今後は、Dataset detail / Gold detail / Job detail で、source kind、confidence、publish history、parser result を混同せず見せる必要があります。
- Gold publication は「work table がある」だけではなく、業務利用に出せる curated data として owner、説明、更新頻度、主要指標を持たせる必要があります。

### Frontend / UX の課題

- `DataPipelineDetailView`、Monaco、Vue Flow、ELK worker などが build chunk を大きくしています。
- Data Pipeline、Drive、Dataset、Gold、Jobs などの操作面は広がっていますが、E2E spec は 5 本で、主要導線すべてを守れてはいません。
- UI は実装済みでも、失敗理由、非同期 job の進捗、policy による無効化理由の説明が不足しやすい領域があります。

### AI Coding / Agent 運用の課題

- `AGENTS.md` は短い入口として良い状態ですが、docs が多いため、どの作業で何を読むべきかの index がありません。
- repo-local skills は `haohao-db-dev` と `haohao-drive-debug` が中心です。RAG、OpenFGA、OpenAPI generation、frontend debug 用の skill がありません。
- 繰り返し発生するレビュー指摘を docs ではなく test / lint / smoke に昇格する仕組みがまだ薄いです。
- agent が UI、DB、API、logs、metrics を一連の検証ループとして確認する標準手順が未整備です。

## 重要ファイルマップ

Data Pipeline:

- `backend/internal/service/data_pipeline_service.go`: pipeline / version / run / schedule の service 中心。
- `backend/internal/service/data_pipeline_graph.go`: step catalog、graph validation、topological order。
- `backend/internal/service/data_pipeline_compile.go`: structured graph の ClickHouse SQL compiler。
- `backend/internal/service/data_pipeline_unstructured.go`: Drive OCR / extract 系を含む hybrid executor。
- `backend/internal/service/data_pipeline_json.go`: JSON input / extract。
- `backend/internal/service/data_pipeline_spreadsheet.go`: Excel / spreadsheet extract。
- `backend/internal/api/data_pipelines.go`: browser API。
- `backend/internal/jobs/data_pipeline_scheduler.go`: due schedule から run を作る job。
- `frontend/src/views/DataPipelinesView.vue`: pipeline list。
- `frontend/src/views/DataPipelineDetailView.vue`: pipeline detail。
- `frontend/src/components/DataPipelineFlowBuilder.vue`: graph builder。
- `frontend/src/components/DataPipelineInspector.vue`: node inspector。
- `frontend/src/components/DataPipelinePreviewPanel.vue`: preview panel。
- `frontend/src/stores/data-pipelines.ts`: frontend state。

Dataset / Gold / Lineage:

- `backend/internal/service/dataset_service.go`
- `backend/internal/service/dataset_gold_service.go`
- `backend/internal/service/dataset_lineage.go`
- `backend/internal/service/dataset_lineage_v2.go`
- `backend/internal/api/datasets.go`
- `backend/internal/api/dataset_gold_publications.go`
- `frontend/src/views/DatasetsView.vue`
- `frontend/src/views/DatasetDetailView.vue`
- `frontend/src/views/DatasetGoldDetailView.vue`
- `frontend/src/components/LineageCompactGraph.vue`
- `frontend/src/components/LineageFlowGraph.vue`
- `frontend/src/components/LineageTimeline.vue`

Drive / OCR / RAG:

- `backend/internal/service/drive_service.go`
- `backend/internal/service/drive_service_api.go`
- `backend/internal/service/drive_ocr_service.go`
- `backend/internal/service/drive_ocr_provider.go`
- `backend/internal/service/drive_product_extraction*.go`
- `backend/internal/service/local_search_service.go`
- `backend/internal/service/drive_rag_service.go`
- `backend/internal/api/drive_*.go`
- `frontend/src/views/DriveView.vue`
- `frontend/src/components/Drive*.vue`
- `frontend/src/stores/drive.ts`

Infrastructure / generation:

- `Makefile`
- `.github/workflows/ci.yml`
- `scripts/gen.sh`
- `backend/cmd/main/main.go`
- `backend/cmd/openapi/main.go`
- `backend/internal/api/register.go`
- `backend/internal/app/openapi.go`
- `db/migrations/*`
- `db/queries/*`
- `openapi/*.yaml`
- `frontend/src/api/generated/*`

AI coding / agent:

- `AGENTS.md`
- `.agents/skills/haohao-db-dev/SKILL.md`
- `.agents/skills/haohao-drive-debug/SKILL.md`
- `docs/HARNESS_ENGINEERING_ADOPTION_ANALYSIS.md`

## 実装時の制約

- `backend/internal/db/*`、`frontend/src/api/generated/*`、`openapi/*.yaml` は手書き編集しません。必ず generator 経由で更新します。
- DB schema 変更時は migration、sqlc、`db/schema.sql` を一貫して更新します。
- tenant-aware table は `tenant_id` で必ず絞ります。tenant 外 resource は原則 404 として隠します。
- Drive / Dataset の resource authorization は DB guard と OpenFGA / DatasetAuthorizationService の境界を崩しません。
- file body、OCR text、embedding、RAG context、product extraction は source file の権限なしに読めない前提です。
- destructive action は confirm dialog、CSRF、audit を必須にします。
- metrics label に tenant id、user id、email、public id、storage key、raw idempotency key、webhook URL を入れません。
- audit / log に secret、token、raw signature、share link raw token、storage key を入れません。
- external cloud AI / OCR API は既定では使いません。local runtime と tenant policy を優先します。

## 最初に読むべき既存ドキュメント

別セッションでこの計画を実装する場合、まずこの文書を読み、その後に作業領域に応じて次だけを読めば十分です。

Data Pipeline を触る場合:

- `docs/data-pipeline-current-state.md`
- `docs/data-pipeline.md`
- `docs/data-pipeline-draft-run-preview.md`
- `docs/data-pipeline-llm-node.md`

Drive / OCR / RAG を触る場合:

- `docs/TUTORIAL_P19_DRIVE_LOCAL_OCR_PRODUCT_EXTRACTION.md`
- `docs/VECTOR_SEARCH_RAG_IMPLEMENTATION_PLAN.md`
- `docs/DRIVE_AGENTIC_RAG_IMPLEMENTATION_PLAN.md`
- `docs/RUNBOOK_DRIVE_PADDLEOCR.md`
- `docs/RUNBOOK_DRIVE_LMSTUDIO_OCR.md`
- `docs/RUNBOOK_VECTOR_SEARCH_RAG.md`

Dataset / Gold / Lineage を触る場合:

- `FUTURE_FEATURES.md`
- `docs/DATA_LINEAGE_DEPENDENCY_GRAPH.md`
- `docs/data-pipeline-current-state.md` の ClickHouse / lineage 関連節

AI coding / agent 改善を触る場合:

- `docs/HARNESS_ENGINEERING_ADOPTION_ANALYSIS.md`
- `AGENTS.md`
- `.agents/skills/haohao-db-dev/SKILL.md`
- `.agents/skills/haohao-drive-debug/SKILL.md`

## 6か月実装計画

### Month 1: Pipeline 品質・可観測性の土台

- `validate` と `profile` を passthrough から実行可能 node にする。
- `data_pipeline_run_steps.metadata` に row count、欠損率、失敗件数、warning、sample を保存する。
- ClickHouse `system.query_log` と pipeline run / step を query id で紐付ける。
- Run detail UI に step metadata、品質 summary、重い step、warning を表示する。

状態: 完了済み。

完了コミット:

- `b9d5c03 Enhance data pipeline run metadata`
- `0160678 Add data pipeline drive smoke`
- `c54de44 Complete data pipeline metadata smoke`

実装済み:

- structured / hybrid の node-level run step metadata。
- `profile` / `validate` / `quality_report` / `confidence_gate` summary。
- `inputRows`、validation `samples`、`queryStats`。
- Runs tab の metadata summary / detail。
- `json` / `excel` / `text` の Data Pipeline smoke suite。

完了条件:

- structured / hybrid の両方で run step metadata が保存される。
- `profile` が row count、null count、unique count、min / max、top values の最小 summary を返す。
- `validate` が required、regex、range、in、unique の最小 rule を評価し、warning / error summary を返す。
- Data Pipeline detail で各 step の品質情報を確認できる。

### Month 2: 信頼できる失敗処理

詳細計画: `docs/DATA_PIPELINE_MONTH2_RELIABLE_FAILURE_HANDLING_PLAN.md`

- `quality_report` と `confidence_gate` の metadata 保存を強化する。
- `quarantine` node を追加し、失敗行・低信頼行を通常 output と分離する。
- `human_review` は review item / queue の入口まで完了済み。Drive text / `extract_fields` / `extract_table`、Drive JSON / `schema_mapping`、Drive product extraction の低信頼結果と Drive file detail からの review 導線も接続済み。
- review item detail は対象 pipeline の `can_view`、transition / comment は対象 pipeline の `can_update` を service 層で確認します。
- 低信頼 OCR / 抽出 / schema mapping の確認フローを Drive と Pipeline UI に接続する。

次にやること:

- `quarantine` node v1 は完了済み。
- `confidence_gate` / `quality_report` の失敗理由 metadata 強化も完了済み。
- 次は review item 権限 test と Month 3 の graph runtime 強化へ進む。

完了条件:

- `quarantine` output が通常 output と別 Work table として作成される。
- review item は tenant boundary、CSRF、audit を満たす。
- Drive OCR / product extraction の低信頼結果から review flow へ到達できる。

### Month 3: 実運用向け Pipeline 拡張

- `union` と `route_by_condition` を追加し、複数入力統合と条件分岐を扱う。
- `partition_filter` / `watermark_filter` を追加し、schedule run と backfill を全量処理から切り離す。
- output node の `orderBy` と型付き output を UI で明示し、ClickHouse の性能設計に使えるようにする。
- `snapshot_scd2` を master data 用 v1 として追加する。

完了条件:

- 複数 source の縦結合、文書種別別の分岐、低信頼行の分岐が graph 上で表現できる。
- schedule run が日付範囲または watermark に基づいて処理対象を絞れる。
- SCD2 output が `valid_from`、`valid_to`、`is_current`、`change_hash` を持つ。

### Month 4: Local Search / RAG の製品化

- RAG smoke を quick gate と broad gate に分け、CI / 手動評価の役割を固定する。
- Drive RAG の retrieval trace を tenant admin / debug UI で読めるようにする。
- schema mapping evidence の共有、却下、rebuild 状態を、運用者が迷わず管理できる形に整理する。
- LM Studio / local embedding の timeout、batch size、index readiness を runbook 化する。

完了条件:

- quick RAG smoke が短時間で regression を検出できる。
- broad RAG smoke は長時間・任意実行として citation coverage、fact coverage、latency を記録する。
- RAG policy disabled、index 未完了、generation runtime unavailable の原因が UI / job detail で追える。

### Month 5: Gold / Lineage / Data Mart

- 既存の Gold publication と lineage v2 を、Dataset detail / Gold detail / Job detail で一貫して読めるようにする。
- parser / manual lineage change set の承認、差し戻し、公開履歴を UI で扱う。
- Gold table は full publish を既定にし、失敗時に既存 Gold を壊さない pointer / swap 方式を維持する。
- Data mart v1 として owner、説明、主要指標、更新頻度、source lineage を持たせる。

完了条件:

- Gold detail で source、schema、row count、publish history、lineage が分かる。
- lineage parser の結果は metadata / manual edge と混同せず、source kind と confidence を表示する。
- publish 失敗時に既存 Gold を読み続けられる。

### Month 6: リリース・運用品質

- E2E を Drive、Pipeline、Gold、RAG、Jobs の主要 happy path に拡張する。
- frontend build の巨大 chunk 警告、特に Monaco / Pipeline detail 周辺を lazy load で改善する。
- backup / restore、OpenFGA、pgvector、ClickHouse、SeaweedFS のローカル DR drill を標準確認にする。
- 本番前チェックリストを deployment runbook に統合する。

完了条件:

- 主要ユーザー導線の Playwright E2E が揃う。
- `make e2e` と主要 smoke が、どの機能リスクを守っているか分かる。
- deployment runbook から本番前確認、rollback、DR drill へ迷わず到達できる。

## Public Interfaces

- 既存の `DataPipelineRunStepBody.metadata` を正規化し、`rowCount`, `warningCount`, `failedRowCount`, `quality`, `profile`, `samples`, `queryStats` を安定キーとして使う。
- 新規 step type は `union`, `quarantine`, `route_by_condition`, `partition_filter`, `watermark_filter`, `snapshot_scd2`。
- Review queue は browser API として追加し、tenant boundary、CSRF、audit を必須にする。
- RAG retrieval trace は通常回答 API を壊さず、admin / debug 用の opt-in 表示にする。

## Test Plan

- 毎 PR 標準:
  - `go test ./backend/...`
  - `npm --prefix frontend run build`
  - `make gen` 後の drift check
- Pipeline:
  - structured / hybrid の両方で step metadata、quarantine、review、schedule、watermark を unit / service test で確認する。
- UI:
  - Playwright で Pipeline 作成、preview、run、schedule、quarantine、review、Gold publish、RAG panel を確認する。
- データ品質:
  - fixture dataset で validate / profile / quality_report の expected metadata を固定する。
- RAG:
  - `make validate-vector-rag-eval`
  - quick smoke
  - 任意の broad smoke

## Future Plan

半年後は、外部連携を増やすより先に、運用済み Pipeline の再実行、差分同期、レビュー済みデータの再投入、Gold refresh policy を強化します。その後に、次を検討します。

- Qdrant など外部 Vector DB
- 動画 / 音声 pipeline
- email / HTML / XML / log extract
- BI connector
- 課金
- HA 構成

## AI Coding Improvements

HaoHao は monorepo、OpenAPI 生成、sqlc、Playwright E2E、smoke scripts、runbook、repo-local skills をすでに持っています。次は AI coding を「実装代行」ではなく、エージェントが迷わず検証可能に作業するための repo 内インフラとして整えます。

優先実装:

- `docs/AGENT_KNOWLEDGE_INDEX.md` を追加し、主要 docs、skills、smoke、検証コマンドへの入口を 1 ページにまとめる。
- `.agents/skills` に次を追加する。
  - `haohao-rag-debug`
  - `haohao-openfga-debug`
  - `haohao-openapi-gen`
  - `haohao-frontend-debug`
- 繰り返しレビューされる規約は docs ではなく、custom lint、structure test、smoke に昇格する。
- agent が UI、ログ、DB、metrics を自分で読む標準ループを docs / skills に固定する。
- `AGENTS.md` は短い索引のまま維持し、詳細は docs / skills に逃がす。

参考:

- `docs/HARNESS_ENGINEERING_ADOPTION_ANALYSIS.md`
- OpenAI Harness engineering: https://openai.com/index/harness-engineering

## Assumptions

- 優先領域は B2B SaaS 基盤ではなく、データ基盤、Drive、Pipeline、Local AI / RAG とする。
- 外部クラウド AI / API は既定では使わず、local runtime と tenant policy を優先する。
- `backend/internal/db/*`、`frontend/src/api/generated/*`、`openapi/*.yaml` は generator 経由で更新する。
- destructive action は confirm dialog、audit、tenant boundary を必須にする。
