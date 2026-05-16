# Data Pipeline Retrospective 2026-05-16

作成日: 2026-05-16

## 目的

この文書は、2026-05-13 から 2026-05-16 にかけて複数セッションで進めた HaoHao Data Pipeline 周辺実装の振り返りです。

別セッションで再開する agent / 開発者が、この文書だけでも次を把握できることを目標にします。

- 何が完了したか。
- なぜその順番で実装したか。
- どの問題が発生し、原因と対応が何だったか。
- どの UI / API / metadata が正本になったか。
- どの検証を通したか。
- 次に何をすべきか。

短い引き継ぎは `docs/DATA_PIPELINE_SESSION_HANDOFF.md`、全体計画は `docs/NEXT_IMPLEMENTATION_PLAN.md`、詳細な現状メモは `docs/data-pipeline-current-state.md` を参照します。本書はそれらを横断した「ここまでの会話と作業のまとめ」です。

## 全体結論

Data Pipeline は、Month 1 から Month 3 の v1 範囲については、実行・品質 metadata・失敗処理・主要 runtime node・SCD2 / Gold 運用導線まで到達しています。

現在の中心課題は、新しい node を増やすことではありません。次に重要なのは、すでに作れる Work table / Gold publication / SCD2 snapshot を、運用者が安全に理解し、失敗理由や履歴を追える状態にすることです。

現時点で「実装済み」と判断できる領域:

- `profile` / `validate` / `quality_report` / `confidence_gate` の run step metadata。
- `quarantine` / `human_review` / review item / Reviews tab。
- Drive text、JSON、Excel、product extraction から review queue への接続。
- Inspector の代表 false positive 修正と lightweight validation endpoint。
- `route_by_condition`、`union`、`partition_filter`、`watermark_filter`。
- typed output / `orderBy` / output `append`。
- `snapshot_scd2`、output `scd2_merge`、`rebuild_key_history`。
- SCD2 Work table preview controls、table-wide SCD2 summary、key history drilldown。
- Data Pipeline output から Gold publish。
- Gold detail の source Work table link、source SCD2 summary、source Data Pipeline run link、source output metadata summary、source quality summary。
- Gold detail から Data Pipeline Runs tab の該当 run/output row への deep link。
- Gold publish history row から Data Pipeline run/output へ戻る deep link v1。

現時点で「未完了」として残すべき領域:

- SCD2 削除検知 policy のうち `deleteDetection=close_current` v1。`mark_deleted` は未完了。
- 同一 key / 同一 `valid_from` に複数変更がある場合の `sameValidFromPolicy=reject` v1。高度な winner policy は未完了。
- composite key の SCD2 key history API / UI v1。
- Gold publish history と Data Pipeline run history の厳密履歴化。publish run 作成時点の source run/output 参照永続化まで完了済み。
- `validate` の行単位 status column と quarantine 連携。
- backend step catalog / generated contract への output schema 単一正本化。
- review item の担当者割当、修正値の再投入 run、review 履歴 UI。
- Data Pipeline / Gold / Drive / RAG の E2E coverage 拡張。

## なぜこの順で進めたか

当初の計画では LLM node や高度な AI node の候補もありました。しかし、LLM node を先に増やすと、confidence、evidence、review、metadata、失敗理由の受け皿が不足します。

そこで順番は次のようにしました。

1. Data Pipeline run step metadata を整える。
2. `profile` / `validate` で品質と失敗理由を保存する。
3. `quality_report` / `confidence_gate` / `quarantine` / `human_review` で低品質・低信頼データを分離する。
4. review item と Reviews tab で人手確認の導線を作る。
5. route / union / partition / watermark / typed output で実運用 graph を作れるようにする。
6. snapshot / SCD2 / append / SCD2 merge で履歴管理と増分に近い運用を可能にする。
7. Work table / Gold detail へ SCD2 summary、quality summary、source run link を接続し、運用者が結果を追えるようにする。

この順番により、将来の `llm_extract_fields`、`llm_schema_mapping`、`llm_quality_explain` などは、既存 metadata / review / quality UI に載せやすくなります。

## 主要コミットの流れ

直近の重要コミットは次の通りです。

```text
5e68517 Deep link Gold source pipeline run
0425ab0 Show Gold source quality summary
803d77f Show Gold source output metadata
fd5840c Link Gold detail to source pipeline
1a161d4 Link pipeline outputs to Gold publications
022fc09 Link Gold detail to source work table
df870f2 Show SCD2 summary on Gold detail
2a5fce5 Use SCD2 output metadata for key history
5bdbc86 Add SCD2 key history drilldown
70ab736 Add SCD2 work table summary
77b95a5 Add SCD2 work table preview controls
ca7000b Document data pipeline handoff state
87d7fda Add SCD2 key history rebuild policy
0930240 Link data pipeline outputs to Gold publish
0eed7b9 Add SCD2 merge mode for data pipeline outputs
5911bc2 Add append mode for data pipeline outputs
9ba00a3 Add data pipeline SCD2 snapshot node
5c5dd87 Carry forward data pipeline watermarks
e08dba2 Add typed data pipeline outputs
41fe0df Add data pipeline partition filters
b081bc3 Add data pipeline union node
bd02d1d Add data pipeline route node
0512628 Add data pipeline graph issues summary
07fd6fc Show data pipeline validation warnings on graph nodes
a1ca8e8 Add data pipeline validation smoke
b058330 Add data pipeline draft validation endpoint
23ec2c6 Add data pipeline preview output schemas
1aa7735 Extract data pipeline step output schema
eae7b3b Document data pipeline column inference follow-up
bfcbb4a Align data pipeline inspector column inference
```

読み方:

- `bfcbb4a` から `b058330` までは、Inspector / validation endpoint / schema warning の土台。
- `bd02d1d` から `5c5dd87` までは、Month 3 の graph runtime。
- `9ba00a3` から `87d7fda` までは、snapshot / append / SCD2 merge / key history rebuild。
- `77b95a5` から `5bdbc86` までは、Work table 上の SCD2 運用 UI。
- `2a5fce5` から `5e68517` までは、Gold / Data Pipeline / Work table を相互に追跡する導線。

## 発生した問題と対応

### Inspector の列不足 false positive

現象:

- `extract_text -> quality_report` で、UI が `text, confidence` を不足列と表示した。
- `schema_mapping(includeSourceColumns=true) -> human_review -> output` で、UI が `file_public_id` を不足列と表示した。
- run 自体は成功しており、backend runtime には列が存在した。

原因:

- Inspector が graph config だけから frontend 側で静的推論していた。
- hybrid executor / materializer が実際に出す列と、frontend fallback 推論がずれていた。

対応:

- `DataPipelineInspector.vue` の fallback 推論を代表 node について backend に合わせた。
- preview API に selected subgraph の `outputSchemas` を追加した。
- lightweight validation endpoint を追加し、preview 実行なしで backend schema / missing-column warning を返すようにした。
- Inspector は validation result を primary source とし、local inference は fallback にした。

残課題:

- backend step catalog / generated contract へ output schema をさらに集約する。
- node 追加時は runtime 出力、validation endpoint、Inspector fallback、smoke を同時に確認する。

### SCD2 merge の再実行 idempotency

現象:

- `snapshot_scd2` stage table は過去 row と current row の両方を持つ。
- `current_only` merge で stage 過去 row まで差分候補にすると、同一データ再実行でも過去 row が再追加される可能性があった。
- current row が古い `valid_from` で誤って close されるリスクがあった。

原因:

- `snapshot_scd2` は入力内履歴を整形する node であり、stage には履歴全体が入る。
- 一方、`scd2_merge current_only` は「今の current state との差分を追加する」policy であり、履歴 row を差分候補にしてはいけない。

対応:

- `current_only` では stage 側の `is_current=1` の行だけを差分候補にした。
- `rebuild_key_history` policy を追加し、late arriving data / key 単位 backfill は別 policy として扱った。

残課題:

- stage に存在しない既存 current key を削除として扱うかどうか。
- 同一 key / 同一 `valid_from` で `change_hash` が異なる複数 row が来た場合の policy。

### Hybrid output Work table の権限不足

現象:

- Drive JSON / OCR / extract 系を含む hybrid run の output が Runs tab には表示される。
- しかし Work table preview API が `403 data resource permission denied` になるケースがあった。

原因:

- structured output path は Work table 登録後に owner tuple を作成していた。
- hybrid output path は管理レコードだけ作り、owner tuple を付与していなかった。

対応:

- `data_pipeline_unstructured.go` で hybrid output 登録後にも `EnsureResourceOwnerTuples` を呼ぶように修正した。

### Gold detail の説明性不足

現象:

- Data Pipeline output を Gold publication にできても、Gold detail から「どの pipeline / run / output で作られたか」が分かりにくかった。
- SCD2 table なのか、current/history が何件あるか、source run の品質状態がどうかも Gold detail だけでは追いづらかった。

対応:

- Gold detail に `sourceScd2Summary` を追加した。
- Gold detail から source Work table へ戻る link を追加した。
- Data Pipeline run output response に `latestGoldPublication` を追加し、Runs tab から Gold detail へ進めるようにした。
- Gold detail に `sourceDataPipelineRun` を追加し、source pipeline、run、output node、write mode、SCD2 merge policy、unique keys、row count を表示した。
- `sourceDataPipelineRun.qualitySummary` を追加し、source run の step metadata から warning、failed rows、review items、validation errors / warnings、confidence gate、quarantine、quality row/column count を集約した。
- Gold detail の source pipeline link に `runPublicId` / `outputNodeId` query を付け、Data Pipeline detail は Runs tab を開いて該当 row をハイライトするようにした。
- Gold publish history row にも `sourceDataPipelineRun` を返し、各 publish run row から Data Pipeline detail の該当 run/output へ戻れるようにした。
- `dataset_gold_publish_runs.source_data_pipeline_run_id` / `source_data_pipeline_run_output_id` を追加し、新規 Gold publish run 作成時点の source Data Pipeline run/output を保存するようにした。
- 表示時は保存済み `source_data_pipeline_run_output_id` を優先し、既存行や参照欠落時のみ `source_work_table_id` から最新 completed output を fallback 表示する。

残課題:

- 既存 Gold publish run 行の source run/output 参照 backfill は `0050_backfill_dataset_gold_publish_run_source_refs` で migration 化した。`source_work_table_id` と publish run 作成時刻から、同一 tenant / Work table の completed Data Pipeline output を best-effort で選び、`source_data_pipeline_run_id` と `source_data_pipeline_run_output_id` を補完する。
- source run step の詳細 dialog を Gold detail から直接開くかどうかは未設計。

## 現在の API / metadata 正本

### run step metadata

保存先:

- `data_pipeline_run_steps.metadata`
- `DataPipelineRunStepBody.metadata`

用途:

- node 単位の実行結果 summary。
- Runs tab の metadata summary / detail dialog。
- Gold detail の source quality summary。

主な key:

- `inputRows`
- `outputRows`
- `warningCount`
- `warnings`
- `failedRows`
- `samples`
- `profile`
- `validation`
- `quality`
- `confidenceGate`
- `queryStats`
- `reviewItemCount`
- `quarantinedRows`
- `passedRows`
- `routeCounts`
- `watermarkFilter`
- `snapshotSCD2`

### run output metadata

保存先:

- `data_pipeline_run_outputs.metadata`
- `DataPipelineRunOutputBody.metadata`

用途:

- output Work table と write mode の説明。
- Gold publish の input。
- Work table / Gold の source tracing。
- SCD2 key column inference。

主な key:

- `workTablePublicId`
- `database`
- `table`
- `displayName`
- `writeMode`
- `scd2MergePolicy`
- `scd2UniqueKeys`
- `validFromColumn`
- `validToColumn`
- `isCurrentColumn`
- `changeHashColumn`

### Work table SCD2 summary

API:

- Work table preview response の `scd2Summary`
- `GET /api/v1/dataset-work-tables/{workTablePublicId}/scd2-history?key=...&limit=100`
- `GET /api/v1/dataset-work-tables/{workTablePublicId}/scd2-history?keyColumns=...&keyValues=...&limit=100`

用途:

- Work table UI の SCD2 panel。
- Gold detail の `sourceScd2Summary`。

現状:

- key columns は Data Pipeline output metadata の `scd2UniqueKeys` を優先する。互換用に `keyColumn` には先頭 key を返す。
- metadata がない場合は `id`、`product_id`、`sku`、`file_public_id` の順で fallback。

制限:

- composite key は `keyColumns` / `keyValues` で history drilldown できる。
- key history API は単一 key value 前提。

### Gold source tracing

API:

- `DatasetGoldPublicationBody.sourceScd2Summary`
- `DatasetGoldPublicationBody.sourceDataPipelineRun`
- `DatasetGoldSourcePipelineRunBody.qualitySummary`

用途:

- Gold detail で source Work table、source Pipeline、source run、source output、SCD2 summary、quality summary を表示する。
- Gold detail から Data Pipeline detail の該当 run/output row へ戻る。

## UI 導線の現在地

### Data Pipeline detail

Runs tab でできること:

- run / output / step を一覧表示。
- output Work table label と write mode を表示。
- output から Gold publish。
- output に最新 Gold publication があれば Gold detail へ移動。
- step metadata summary と detail dialog を表示。
- `runPublicId` / `outputNodeId` query があれば Runs tab を開き、該当 row をハイライトする。

### Work table UI

SCD2 table の場合にできること:

- SCD2 検出バッジ。
- table 全体の total/current/history/key count。
- key column と `valid_from` range。
- preview row の All / Current / History filter。
- key value を指定した履歴 drilldown。

### Gold detail

できること:

- Gold overview。
- source Work table link。
- source Pipeline / run / output link。
- source output metadata summary。
- source quality summary。
- source SCD2 summary。
- Gold schema。
- Gold preview。
- publish history。
- medallion catalog panel。

## 検証済みコマンド

この一連の作業で繰り返し使った代表コマンド:

```bash
go test ./backend/internal/service ./backend/internal/api
go test ./backend/...
npm --prefix frontend run build
git diff --check
make gen
make smoke-data-pipeline-suite
make smoke-data-pipeline-route
make smoke-data-pipeline-union
make smoke-data-pipeline-partition
make smoke-data-pipeline-typed-output
make smoke-data-pipeline-watermark-previous
make smoke-data-pipeline-snapshot-scd2
make smoke-data-pipeline-snapshot-append
make smoke-data-pipeline-snapshot-merge
make smoke-data-pipeline-snapshot-merge-backfill
make smoke-data-pipeline-quarantine
make smoke-data-pipeline-review
make smoke-data-pipeline-field-review
make smoke-data-pipeline-table-review
make smoke-data-pipeline-schema-mapping-review
make smoke-data-pipeline-product-review
```

UI 確認で使った前提:

```bash
make up
make backend-dev
make frontend-dev
```

注意:

- `npm --prefix frontend run build` は Monaco / Data Pipeline detail / SQL editor 周辺の large chunk warning を出す。現時点では blocker ではない。
- `make gen` は Go build cache に触るため、sandbox 環境では権限が必要になることがある。
- local backend が起動していないと API / browser smoke は `ECONNREFUSED` になる。

## 別セッションで再開する手順

1. `git status --short` で作業ツリーを見る。
2. `docs/DATA_PIPELINE_SESSION_HANDOFF.md` を読む。
3. 本書の「現時点の API / metadata 正本」と「次にやること」を読む。
4. 触る領域に応じて次を読む。
   - Data Pipeline 実行: `docs/data-pipeline-current-state.md`
   - 全体計画: `docs/NEXT_IMPLEMENTATION_PLAN.md`
   - Inspector 警告: `docs/DATA_PIPELINE_UI_COLUMN_INFERENCE.md`
   - validation endpoint: `docs/DATA_PIPELINE_VALIDATION_ENDPOINT_PLAN.md`
   - Month 2: `docs/DATA_PIPELINE_MONTH2_RELIABLE_FAILURE_HANDLING_PLAN.md`
5. 実装後は最低限次を確認する。

```bash
go test ./backend/internal/service ./backend/internal/api
npm --prefix frontend run build
git diff --check
```

backend API / OpenAPI / generated type を触った場合:

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
```

SCD2 / Gold / Work table を触った場合:

```bash
make smoke-data-pipeline-snapshot-merge
make smoke-data-pipeline-snapshot-merge-backfill
```

## 次にやること

### 1. SCD2 削除検知 policy

問題:

- 現在の `scd2_merge` は stage に現れた key の変更を扱う。
- stage に現れなかった既存 current key を「削除」とみなすかは当初未定義でした。
- source system によっては、差分 input なのか full snapshot input なのかが違うため、単純に missing key = deleted とすると危険。

2026-05-16 に実装した v1:

- 既定は削除検知なしのまま維持する。
- `writeMode=scd2_merge` かつ `scd2MergePolicy=current_only` の output node に、明示 opt-in の `deleteDetection=close_current` を追加した。
- `close_current` では、今回 run の current snapshot に存在しない既存 current key を close し、既存行の `valid_to` に今回 snapshot の最大 `valid_from` を設定し、`is_current=0` にする。
- 再実行時は、すでに close 済みの行は current row ではないため重複 close されない。
- output metadata に `deleteDetection` を保存する。
- smoke として `make smoke-data-pipeline-snapshot-merge-delete` を追加した。

残り:

- `deleteDetection=mark_deleted` は未実装。
- Data Pipeline Output 設定 UI から `deleteDetection` を選べる導線は実装済み。
- full snapshot input であることを UI 上で明示させる説明 / guard は未実装。

### 2. 同一 key / 同一 `valid_from` policy

問題:

- `rebuild_key_history` では `uniqueKeys`、`valid_from`、`change_hash` で重複排除している。
- 同一 key / 同一 `valid_from` に異なる `change_hash` が来た場合、どちらを採用すべきかが当初未定義でした。

2026-05-16 に実装した v1:

- `writeMode=scd2_merge` の既定 policy として `sameValidFromPolicy=reject` を追加した。
- promote 前に stage table を検査し、同一 `uniqueKeys` / `valid_from` group に複数の `change_hash` がある場合は run を failed にする。
- 既存 table がない初回 snapshot 作成でも検出される。
- output metadata に `sameValidFromPolicy` を保存する。
- smoke として `make smoke-data-pipeline-snapshot-merge-conflict` を追加した。

残り:

- `latest_ingested_wins`、`highest_source_priority_wins` のような winner policy は未実装。
- conflict sample を専用 metadata として保存するところまでは未実装。現状は run / output の `errorSummary` で原因を追える。
- UI から `sameValidFromPolicy` を選択する導線は実装済み。

### 3. Composite key SCD2 history

当初の問題:

- Work table SCD2 summary / key history は `scd2UniqueKeys[0]` を代表 key として扱っていた。
- 複合 key の履歴を正しく指定・表示する API / UI がなかった。

2026-05-16 に実装した v1:

- `scd2Summary` に `keyColumns []string` を追加した。
- key history API は従来の `?key=...` に加えて `?keyColumns=tenant_id,product_id&keyValues=1,P001` を受ける。
- managed Work table では run output metadata の `scd2UniqueKeys` 全体を使って key count と history query を行う。
- UI は composite key の場合、key column ごとの input を表示する。
- 単一 key table では従来 UI を維持する。
- smoke として `make smoke-data-pipeline-snapshot-merge-composite` を追加した。

### 4. Gold publish history と Data Pipeline run history の相互リンク

問題:

- Gold detail から source run/output へ戻れるようになった。
- Data Pipeline output から最新 Gold publication へ進めるようになった。
- publish history の各 run からも Data Pipeline run history へ戻る必要がある。

対応:

- v1 として `DatasetGoldPublishRunBody.sourceDataPipelineRun` を追加した。
- service 層では `dataset_gold_publish_runs.source_work_table_id` から、その Work table を最後に作った completed Data Pipeline run output を逆引きする。
- API は publication detail と同じ `DatasetGoldSourcePipelineRunBody` を publish run row にも返す。
- UI は Gold detail の publish history table に同期元 Pipeline 列を追加し、source pipeline detail の `runPublicId` / `outputNodeId` へ deep link できるようにした。
- その後、`dataset_gold_publish_runs` に `source_data_pipeline_run_id` と `source_data_pipeline_run_output_id` を追加し、新規 publish run 作成時点で解決できた source run/output を保存するようにした。
- hydrate 時は保存済み output ID を優先して source run を取得し、既存行や参照欠落時のみ Work table 逆引きへ fallback する。

残課題:

- migration 前の既存行は `0050_backfill_dataset_gold_publish_run_source_refs` 適用時に best-effort backfill される。ただし、該当 Work table を出力した completed Data Pipeline run が存在しない古い Gold publish run は nullable のまま残るため、表示側 fallback は引き続き必要。

## AI coding 改善として残すこと

このセッションでは、会話と作業量が大きくなったため、docs の分散と再開コストが問題になりました。今後の AI coding 改善として次を推奨します。

### 1. Agent knowledge index

`docs/AGENT_KNOWLEDGE_INDEX.md` を追加済みです。作業タイプ別に読むべき docs / files / commands を 1 ページにまとめ、別セッションが「どの文書から読むか」で迷わないようにしました。

現在の分類:

- Data Pipeline runtime を触る場合。
- Data Pipeline UI を触る場合。
- Drive / RAG を触る場合。
- OpenFGA / authorization を触る場合。
- OpenAPI / generated types を触る場合。
- DB migration / sqlc を触る場合。

### 2. Skills の追加

既存:

- `.agents/skills/haohao-db-dev`
- `.agents/skills/haohao-drive-debug`
- `.agents/skills/supabase-postgres-best-practices`

追加候補:

- `haohao-data-pipeline-debug`
- `haohao-openapi-gen`
- `haohao-rag-debug`
- `haohao-openfga-debug`
- `haohao-frontend-ui-check`

特に `haohao-data-pipeline-debug` は、graph validation、run outputs、run steps metadata、ClickHouse output table、Work table preview、Gold publication を一連で確認する手順を持つべきです。

### 3. Smoke 結果の docs 反映

smoke script が成功した場合、最低限の検証 summary を docs または temp artifact に出す仕組みがあると、次セッションで「何を確認済みか」を探しやすくなります。

候補:

- `artifacts/smoke/data-pipeline/latest.json`
- `docs/DATA_PIPELINE_VERIFICATION_LOG.md`

### 4. Browser smoke の安定化

agent-browser で local login session が切れる場合がありました。UI smoke は API smoke より状態依存が強いため、次を整えるとよいです。

- local login 用の固定 helper。
- session state の保存先。
- Gold detail / Data Pipeline detail / Work table preview の smoke navigation script。
- backend が OIDC mode か local login mode かを smoke 開始時に明示チェックする。

## 現時点の判断

Month 1、Month 2、Month 3 の v1 実装は、計画上は完了と見なせます。

ただし「運用品質として完成」ではありません。これからの実装は、主に次の方向です。

- SCD2 の edge case policy を明確にする。
- 複合 key / 削除検知 / 同一時刻 conflict を扱う。
- Gold / Data Pipeline / Work table の lineage と history をさらに一貫させる。
- E2E / smoke / docs / skills を整備し、AI agent が迷わず検証できる状態にする。
