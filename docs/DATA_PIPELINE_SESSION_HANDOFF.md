# Data Pipeline Session Handoff

作成日: 2026-05-16

## 目的

この文書は、2026-05-14 から 2026-05-16 にかけて進めた Data Pipeline 実装セッションの引き継ぎメモです。別セッションで再開するときに、これまで何を実装し、何を検証し、次に何をすべきかを短時間で把握できることを目的にします。

詳細な設計背景は次を参照します。

- 全体計画: `docs/NEXT_IMPLEMENTATION_PLAN.md`
- Data Pipeline 実装計画: `docs/DATA_PIPELINE_IMPLEMENTATION_PLAN.md`
- 現状メモ: `docs/data-pipeline-current-state.md`
- セッション横断の振り返り: `docs/DATA_PIPELINE_RETROSPECTIVE_2026-05-16.md`
- Month 2 詳細: `docs/DATA_PIPELINE_MONTH2_RELIABLE_FAILURE_HANDLING_PLAN.md`
- Inspector 列推論問題: `docs/DATA_PIPELINE_UI_COLUMN_INFERENCE.md`
- validation endpoint: `docs/DATA_PIPELINE_VALIDATION_ENDPOINT_PLAN.md`

## 現在の結論

Month 1 の品質 / 可観測性、Month 2 の信頼できる失敗処理、Month 3 の主要 runtime node は、v1 として実装済みです。2026-05-16 時点の詳細な振り返り、問題、原因、対応、未完了領域、次タスクは `docs/DATA_PIPELINE_RETROSPECTIVE_2026-05-16.md` に集約しました。

現在の次タスクは、機能をさらに横に増やすことではなく、運用 UI と説明性を強くすることです。優先候補は次です。

1. Frontend palette は backend step catalog contract 取得へ移行済み。残る集約対象は output schema inference の完全 generated/API contract 化。
2. `mark_deleted` / winner policy などの高度 SCD2 policy 検討。
3. Month 3 optional hardening の残確認。既存 Gold publish run 行の source run/output 参照 backfill は `0050_backfill_dataset_gold_publish_run_source_refs` で migration 化し、2026-05-16 の local DB 検証で version 50 / dirty=false、既存 publish run 4/4 件の補完まで確認済み。

## 実装済みの流れ

### Month 1: Metadata / Profile / Validate

主な成果:

- structured / hybrid executor が node ごとの row count と metadata を保存する。
- `profile` が row count、column count、null count / rate、unique count、min / max、top values を metadata に保存する。
- `validate` が required、regex、range、in、unique の失敗件数、warning、sample を metadata に保存する。
- Runs tab で step metadata summary と detail dialog を確認できる。
- `queryStats`、`inputRows`、`samples` を保存する。
- JSON / Excel / text scenario の smoke を整備した。

代表検証:

```bash
go test ./backend/...
npm --prefix frontend run build
node --check scripts/smoke-data-pipeline.mjs
make smoke-data-pipeline-suite
```

### Month 2: Reliable Failure Handling

主な成果:

- `quality_report` と `confidence_gate` の metadata / warning / low confidence sample を強化した。
- `quarantine` node v1 を追加した。
- `human_review(createReviewItems=true)` が永続 review item を作成する。
- Data Pipeline detail に Reviews tab を追加した。
- review item の list / detail / transition / comment API を追加した。
- review item detail は pipeline `can_view`、transition / comment は pipeline `can_update` を service 層で確認する。
- Drive text、`extract_fields`、`extract_table`、Drive JSON + `schema_mapping`、Drive product extraction の低信頼結果を review queue へ接続した。
- Drive file detail から source file に紐づく review item / pipeline run へ戻れるようにした。

代表 smoke:

```bash
make smoke-data-pipeline-quarantine
make smoke-data-pipeline-review
make smoke-data-pipeline-field-review
make smoke-data-pipeline-table-review
make smoke-data-pipeline-schema-mapping-review
make smoke-data-pipeline-product-review
```

残課題:

- review item の担当者割当。
- review で修正した値の再投入 run。
- review 履歴 UI。
- `validate` の行単位 status column 化。

### Inspector / Validation Endpoint

問題:

- UI の Inspector は graph config だけから上流列を静的推論していた。
- backend runtime が実際に出す列と UI 推論がずれると、run は成功するのに UI だけで「設定済みの列が上流ステップの出力にありません」と表示された。

実際に発生した false positive:

- `extract_text -> quality_report` で `text, confidence` が不足扱いになった。
- `schema_mapping(includeSourceColumns=true) -> human_review -> output` で `file_public_id` が不足扱いになった。

対応:

- `DataPipelineInspector.vue` の fallback 推論を backend materializer に合わせた。
- preview API が selected subgraph の `outputSchemas` を返すようにした。
- 軽量 validation endpoint が preview 実行なしで graph 全体の output schema と missing-column warnings を返すようにした。
- Inspector は validation result を primary source とし、local inference は fallback として残した。
- `dataPipelineStepCatalog` の全 step type が backend `inferOutputSchemas` で non-empty schema を返すことを `TestInferOutputSchemasCoversEveryCatalogStep` で固定した。今後 step を追加して backend schema 推論を忘れると service test が落ちる。
- 保存済み pipeline では validation endpoint を missing-column warning の正本とし、validation 未取得時は Inspector の local fallback warning を表示しない。local fallback は列候補生成と pipelinePublicId がない一時状態向けに残す。

残課題:

- frontend palette / node catalog は `/api/v1/data-pipelines/step-catalog` から `type`、`category`、`order`、`labelKey` を取得する。`DataPipelineFlowBuilder.vue` のカテゴリ分けと auto layout ordering はこの contract を使う。frontend の `data-pipeline-step-output-schema.ts` は列候補生成の fallback として残っているため、次は output schema inference も generated/API contract へさらに寄せる。
- 新しい node / 出力列を追加するたびに、backend runtime 出力、validation endpoint、Inspector fallback、smoke を同時に確認する。

### Month 3: Runtime Node / Output

主な成果:

- `route_by_condition` v1。
- `union` v1。
- `partition_filter` v1。
- `watermark_filter` v1。
- `watermarkSource=previous_success` による前回成功 run からの watermark 引き継ぎ。
- output `columns` による列選択、リネーム、型変換。
- output `orderBy` による ClickHouse table の primary sort key 指定。
- `snapshot_scd2` v1。
- output `writeMode=append`。
- output `writeMode=scd2_merge`。
- `scd2MergePolicy=rebuild_key_history`。
- Data Pipeline output から Gold publish への最小 UI 導線。

代表 smoke:

```bash
make smoke-data-pipeline-route
make smoke-data-pipeline-union
make smoke-data-pipeline-partition
make smoke-data-pipeline-typed-output
make smoke-data-pipeline-watermark-previous
make smoke-data-pipeline-snapshot-scd2
make smoke-data-pipeline-snapshot-append
make smoke-data-pipeline-snapshot-merge
make smoke-data-pipeline-snapshot-merge-backfill
```

## SCD2 の現状

`snapshot_scd2` は、入力内に含まれる履歴行を SCD Type 2 形式に整える node です。

主な config:

- `uniqueKeys`: entity を識別する key。
- `updatedAtColumn`: version の開始時刻として使う列。
- `watchedColumns`: `change_hash` に含める列。
- `validFromColumn`: 既定 `valid_from`。
- `validToColumn`: 既定 `valid_to`。
- `isCurrentColumn`: 既定 `is_current`。
- `changeHashColumn`: 既定 `change_hash`。

`writeMode=scd2_merge` は、output node で既存 snapshot table と今回 run の stage table を merge します。

`scd2MergePolicy=current_only`:

- 既定 policy。
- stage 側の `is_current=1` の行だけを差分候補にする。
- 既存 current row と stage current row を `uniqueKeys` と `change_hash` で比較する。
- 変更がなければ追加しない。
- 変更があれば既存 current row の `valid_to` を新 row の `valid_from` で閉じ、新 current row を追加する。
- 同一データ再実行では行数を増やさない。

`scd2MergePolicy=rebuild_key_history`:

- late arriving data / key 単位 backfill 用。
- stage に含まれる key だけを対象 key とする。
- 対象 key の既存 snapshot row と stage row を結合する。
- `uniqueKeys`、`valid_from`、`change_hash` で重複を除く。
- `valid_from` 昇順で `valid_to` / `is_current` を再計算する。
- 例: `draft(2026-05-01) -> ready(2026-05-03)` に `review(2026-05-02)` が遅延到着すると、`draft(2026-05-01..2026-05-02) -> review(2026-05-02..2026-05-03) -> ready(current)` になる。
- 同じ late file を再実行しても重複 version は増えない。

実装時に見つかった注意点:

- `snapshot_scd2` stage table には過去 row と current row の両方が含まれる。
- `current_only` merge で stage 過去 row まで差分候補にすると、同一データ再実行でも過去 row が再追加され、current row が古い `valid_from` で誤って close される。
- そのため `current_only` では stage 側の `is_current=1` だけを見る。
- `rebuild_key_history` は対象 key 全体を再計算するため late row を扱える。
- `deleteDetection=close_current` は backend / smoke まで実装済み。
- 同一 key / 同一 `valid_from` の複数変更は、既定の `sameValidFromPolicy=reject` で run を failed にする。

## Gold Publish 導線

既存の Gold publish API は Work table 起点です。Data Pipeline output は run 成功時に managed Work table として登録されるため、Data Pipeline 側では次を追加しました。

- run output metadata に `workTablePublicId`、database、table name、display name、write mode を保存する。`writeMode=scd2_merge` の場合は `scd2MergePolicy`、`scd2UniqueKeys`、`validFromColumn`、`validToColumn`、`isCurrentColumn`、`changeHashColumn` も保存する。
- Runs tab の output 行に `hh_t_*_work.table` と write mode を表示する。
- output 行の `Publish output to Gold` ボタンから既存 Gold publication API を呼ぶ。
- 成功後は Gold detail へ遷移する。
- Data Pipeline Runs tab の output 行から、その Work table を source に持つ最新 Gold publication detail へ移動できる。
- Gold detail から同期元 Work table と同期元 Data Pipeline detail へ戻れる。同期元 Pipeline は `dataset_gold_publications.source_work_table_id` から `data_pipeline_run_outputs.output_work_table_id` を逆引きし、最新 completed output の pipeline public ID、pipeline name、run public ID、output node ID、output row count、completed_at、output metadata summary を detail API に返す。

確認済み:

- `make smoke-data-pipeline-snapshot-merge` で metadata に `workTablePublicId`、`scd2MergePolicy=current_only`、`scd2UniqueKeys=["id"]` が入る。
- `make smoke-data-pipeline-snapshot-merge-backfill` で metadata に `scd2MergePolicy=rebuild_key_history`、`scd2UniqueKeys=["id"]` が入る。
- agent-browser で Runs tab の `Publish output to Gold` ボタンを押し、Gold detail へ遷移することを確認した。
- Gold detail API は `sourceDataPipelineRun` を返し、UI は `同期元 Pipeline` として pipeline detail link、run public ID、output node ID、write mode、SCD2 merge policy、unique keys、output row count を表示する。
- Gold detail API は source run の `data_pipeline_run_steps.metadata` も集約し、step count、warning count、failed rows、review item count、validation errors / warnings、confidence gate pass / needs-review rows、quarantined rows、quality rows / columns を `sourceDataPipelineRun.qualitySummary` として返す。UI は `同期元品質サマリー` panel で表示する。
- Gold detail の同期元 Pipeline link は `runPublicId` / `outputNodeId` query を付ける。Data Pipeline detail は query がある場合に Runs tab を開き、該当 run / output row をハイライトして中央へスクロールする。

残課題:

- Gold detail の quality summary は `sourceDataPipelineRun.qualitySummary` として表示済み。SCD2 の row summary は `sourceScd2Summary` として表示済みで、Data Pipeline source / run id / SCD2 merge policy は `sourceDataPipelineRun` として表示済み。
- Gold publish history と Data Pipeline run history の相互リンク。Gold detail の publish history row は `sourceDataPipelineRun` を表示し、source Data Pipeline detail の `runPublicId` / `outputNodeId` へ deep link できる。
- `dataset_gold_publish_runs.source_data_pipeline_run_id` / `source_data_pipeline_run_output_id` を追加済み。新規 publish run は作成時点の source Data Pipeline run/output を保存し、表示時は保存済み output を優先する。
- 既存行や参照欠落時は `source_work_table_id` から最新 completed output を fallback 表示する。さらに `0050_backfill_dataset_gold_publish_run_source_refs` で、migration 実行時に `source_work_table_id` と publish run 作成時刻から最も近い completed Data Pipeline output を best-effort 補完する。
- 2026-05-16 local DB 検証では `make db-up` が 0049 / 0050 を適用し、`schema_migrations.version=50, dirty=false` になった。`dataset_gold_publish_runs` は `source_data_pipeline_run_id` / `source_data_pipeline_run_output_id` が 4/4 件で補完済み。続けて `make smoke-data-pipeline-snapshot-merge` と `make smoke-data-pipeline-snapshot-merge-delete` が成功した。

## SCD2 / Snapshot Work Table UI

Data Pipeline の `snapshot_scd2` と output `writeMode=scd2_merge` により、ClickHouse 上の Work table には SCD Type 2 の履歴行が保存されます。これまでは Work table UI が列一覧と preview rows をそのまま表示するだけだったため、利用者は `is_current`、`valid_from`、`valid_to`、`change_hash` を目視で探して current / history の状態を判断する必要がありました。

2026-05-16 の追加対応で、`frontend/src/components/DatasetWorkTableBrowser.vue` は選択中 Work table の列に `valid_from`、`valid_to`、`is_current`、`change_hash` が揃っている場合に SCD2 snapshot table として検出します。検出時は Overview の preview セクションに次を表示します。

- SCD2 snapshot 検出バッジ。
- 現在読み込まれている preview rows の件数。
- preview 内の current rows 件数。
- preview 内の history rows 件数。
- key columns。Data Pipeline の `scd2_merge` output から作成された managed Work table では run output metadata の `scd2UniqueKeys` 全体を優先する。互換用に `keyColumn` には先頭 key を返す。metadata がない古い run や手動 Work table では `id`、`product_id`、`sku`、`file_public_id` の順で存在する列を fallback 表示する。
- preview rows に対する `All` / `Current` / `History` フィルタ。

この UI の最初の版は backend API contract を増やさず、既存の Work table preview payload だけを使っていました。その後、Work table preview API の response に `scd2Summary` を追加し、SCD2 table の場合は ClickHouse table 全体に対する summary を返すようにしました。

`scd2Summary` の内容:

- `detected`: SCD2 列が揃っているか。
- `totalRows`: table 全体の行数。
- `currentRows`: table 全体の current rows。
- `historyRows`: table 全体の history rows。
- `keyColumn`: key column。managed Work table では最新の completed run output metadata に保存された `scd2UniqueKeys[0]` を実テーブル列と照合して使う。metadata がない場合は `id`、`product_id`、`sku`、`file_public_id` の順で fallback 検出する。
- `keyColumns`: composite key columns。managed Work table では最新の completed run output metadata に保存された `scd2UniqueKeys` 全体を実テーブル列と照合して使う。
- `keyCount`: key column が検出できた場合の distinct key 数。
- `earliestValidAt`: `valid_from` の最小値。
- `latestValidAt`: `valid_from` の最大値。

Key 単位履歴 drilldown:

- `GET /api/v1/dataset-work-tables/{workTablePublicId}/scd2-history?key=...&limit=100` を追加した。
- composite key 用に `GET /api/v1/dataset-work-tables/{workTablePublicId}/scd2-history?keyColumns=tenant_id,product_id&keyValues=1,P001&limit=100` も追加した。
- preview と同じく Work table の `can_preview` 権限を要求する。
- SCD2 列が揃っていない table、または key column を解決できない table では invalid input として扱う。
- key columns は summary と同じく run output metadata の `scd2UniqueKeys` を優先し、metadata がない場合だけ `id`、`product_id`、`sku`、`file_public_id` から推定する。
- query は `toString(key_column) = key` で絞り込み、`valid_from ASC` で返す。
- Work table UI では SCD2 panel に key 入力欄を出し、指定 key の履歴 rows を時系列で表示する。

注意点:

- summary の件数は table 全体を対象にする。
- `All` / `Current` / `History` filter は、引き続き現在読み込まれている preview rows だけを対象にする。
- key column は output metadata からではなく列名候補で推定しているため、業務 key が別名の場合は `keyColumn` / `keyCount` が空または 0 になる。
- key 単位履歴 drilldown も同じ key 推定に依存する。

実装中に見つかった権限問題:

- structured output path は Data Pipeline が作成した Work table に owner tuple を付与していた。
- hybrid output path は Work table 管理レコードを作るだけで owner tuple を付与していなかった。
- そのため Drive JSON / OCR / extract 系を含む hybrid run の output は Runs tab には表示される一方、`/api/v1/dataset-work-tables/{workTablePublicId}/preview` が `403 data resource permission denied` になることがあった。
- `data_pipeline_unstructured.go` で hybrid output 登録後にも `EnsureResourceOwnerTuples` を呼ぶように修正した。

今後の拡張候補:

- key columns は output metadata の `scd2UniqueKeys` を優先し、composite key の UI / API contract まで完了済み。
- Gold detail 側には `sourceScd2Summary` として current row count、history row count、key count、key column、`valid_from` range を表示済み。同期元 Work table への deep link、同期元 Data Pipeline detail への run/output deep link、Data Pipeline Runs tab の output 行から Gold detail へ進む link も追加済み。`sourceDataPipelineRun` には write mode、SCD2 merge policy、unique keys、output row count、quality summary も表示済み。

## 主要コミット

直近の関連コミット:

- `87d7fda Add SCD2 key history rebuild policy`
- `0930240 Link data pipeline outputs to Gold publish`
- `0eed7b9 Add SCD2 merge mode for data pipeline outputs`
- `5911bc2 Add append mode for data pipeline outputs`
- `9ba00a3 Add data pipeline SCD2 snapshot node`
- `5c5dd87 Carry forward data pipeline watermarks`
- `e08dba2 Add typed data pipeline outputs`
- `41fe0df Add data pipeline partition filters`
- `b081bc3 Add data pipeline union node`
- `bd02d1d Add data pipeline route node`
- `b058330 Add data pipeline draft validation endpoint`
- `23ec2c6 Add data pipeline preview output schemas`
- `bfcbb4a Align data pipeline inspector column inference`
- `4757732 Fix data pipeline inspector column inference`
- `2a5fce5 Use SCD2 output metadata for key history`
- `df870f2 Show SCD2 summary on Gold detail`
- `022fc09 Link Gold detail to source work table`
- `1a161d4 Link pipeline outputs to Gold publications`
- `fd5840c Link Gold detail to source pipeline`
- `803d77f Show Gold source output metadata`
- `0425ab0 Show Gold source quality summary`
- `5e68517 Deep link Gold source pipeline run`

## 検証環境メモ

前提:

```bash
make up
make backend-dev
make frontend-dev
```

よく使う検証:

```bash
go test ./backend/internal/service ./backend/internal/api
npm --prefix frontend run build
node --check scripts/smoke-data-pipeline.mjs
make smoke-data-pipeline-snapshot-merge
make smoke-data-pipeline-snapshot-merge-backfill
```

ClickHouse で SCD2 結果を見る例:

```bash
docker exec haohao-clickhouse clickhouse-client --query \
  "SELECT id, name, status, toString(valid_from), toString(valid_to), is_current, change_hash
   FROM hh_t_1_work.<output_table>
   ORDER BY id, valid_from, is_current
   FORMAT PrettyCompact"
```

## 次にやること

最優先候補は output schema inference の generated/API contract 化です。Frontend palette / node catalog の backend contract 化と Gold publish history の厳密履歴化は完了しました。

実装案:

- `mark_deleted`、`latest_ingested_wins`、`highest_source_priority_wins` のような高度 policy を採用するかは業務要件に合わせて別途決める。

次点候補:

- `validate` の行単位 status column と quarantine 連携を設計する。
- output schema inference の generated/API contract 化を進める。palette / node catalog は backend contract に寄せ済み。
