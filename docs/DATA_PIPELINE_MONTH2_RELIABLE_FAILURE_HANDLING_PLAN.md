# Data Pipeline Month 2: Reliable Failure Handling Plan

作成日: 2026-05-14

## Summary

Month 2 の目的は、Data Pipeline の失敗行、低信頼行、レビュー待ち行を通常 output から分離し、理由と履歴を追える状態にすることです。Month 1 で run step metadata、`profile`、`validate`、`quality_report`、`confidence_gate` の土台は完了済みで、Month 2 の最初の `quarantine` node v1 も実装済みです。次は失敗理由の強化と review queue 化へ進みます。

この文書は Month 2 の実装を別セッションで開始するための詳細計画です。最初の PR は `quarantine` node v1 に限定し、その後に失敗理由の強化、`human_review` の queue 化、Drive / OCR / extraction 連携へ進みます。

## Current State

完了済み:

- structured / hybrid executor が node ごとの `rowCount` と `metadata` を保存する。
- `DataPipelineRunStepBody.metadata` は browser API で `Record<string, unknown>` として公開済み。
- `profile` は row count、column count、null count / rate、unique count、min / max、top values を保存する。
- `validate` は required、regex、range、in、unique の失敗件数、warning、samples を保存する。
- `quality_report` は `quality_report_json`、`missing_rate_json`、`validation_summary_json` を行データへ追加し、run step metadata に `quality` summary と missing rate warning を保存する。
- `confidence_gate` は `gate_score`、`gate_status`、`gate_reason` を行データへ追加し、metadata に threshold、score columns、pass / needs review 件数、低信頼 sample を保存する。
- `quarantine` node v1 は backend catalog、hybrid executor、frontend type、palette、Inspector、Runs metadata summary、smoke suite に実装済み。
- `quarantine` は `statusColumn` と `matchValues` で行を分離し、`outputMode=quarantine_only` / `pass_only` で別 Work table へ出力できる。
- `human_review` は `review_status`、`review_queue`、`review_reason_json` を行データへ追加できる。ただし永続的な review item / queue ではない。
- Data Pipeline smoke suite は JSON、Excel、text、quarantine scenario で run step metadata と quarantine output 分離を検証する。

未実装:

- `validate` の失敗行は metadata に集計されるが、後続 node が行単位で読める validation status 列はまだない。
- `human_review` は注釈列のみで、担当者、承認、差し戻し、コメント、修正履歴、再投入 run を持つ queue ではない。
- Drive OCR / extraction / schema mapping / product extraction の低信頼結果から review queue へ到達する導線は実装済み。schema mapping は `schema_mapping_confidence`、product extraction は `product_confidence` / `product_extraction_reason` を `confidence_gate` / `human_review` に渡す smoke まで追加済み。

重要ファイル:

- `backend/internal/service/data_pipeline_graph.go`
- `backend/internal/service/data_pipeline_unstructured.go`
- `backend/internal/service/data_pipeline_compile.go`
- `frontend/src/api/data-pipelines.ts`
- `frontend/src/components/DataPipelineFlowBuilder.vue`
- `frontend/src/components/DataPipelineInspector.vue`
- `frontend/src/components/DataPipelinePreviewPanel.vue`
- `frontend/src/i18n/messages.ts`
- `scripts/smoke-data-pipeline.mjs`

## Completion Criteria

Month 2 は次を満たしたら完了です。

- low confidence / validation error の理由を run detail で説明できる。
- `quarantine` output が通常 output と別 Work table として作成され、Runs tab から確認できる。
- `human_review` が永続 review item / queue の入口になる。
- review item は tenant boundary、CSRF、audit を満たす。
- Drive OCR / extraction / schema mapping の低信頼結果から review flow へ到達できる。
- smoke で `confidence_gate -> quarantine -> output` の分離を検証できる。

## Implementation Phases

### Phase 1: `quarantine` node v1

状態: 完了済み。

完了コミット前の確認:

- `/Users/knakajima/.asdf/installs/golang/1.23.0/go/bin/go test ./backend/...`
- `npm --prefix frontend run build`
- `node --check scripts/smoke-data-pipeline.mjs`
- `make smoke-data-pipeline-suite`

最初の PR は `quarantine` node v1 に限定します。review item / queue は作りません。

Backend:

- `DataPipelineStepQuarantine = "quarantine"` を追加し、step catalog に登録する。
- `dataPipelineUnstructuredStep` に `quarantine` を追加し、hybrid path で実行する。
- `materializeQuarantine` を追加する。
- upstream column `statusColumn` を読み、`matchValues` に一致する行を quarantine 対象にする。
- `outputMode = quarantine_only` は一致行だけを出力する。
- `outputMode = pass_only` は非一致行だけを出力する。
- `mode` は v1 では `filter` 扱いにする。将来の `annotate` / `split` の予約名として残す。
- metadata に `quarantinedRows`、`passedRows`、`statusColumn`、`matchValues`、`outputMode` を保存する。

Config v1:

| key | default | 内容 |
| --- | --- | --- |
| `statusColumn` | `gate_status` | 判定に使う列 |
| `matchValues` | `["needs_review"]` | quarantine 対象とする値 |
| `outputMode` | `quarantine_only` | `quarantine_only` または `pass_only` |
| `mode` | `filter` | v1 では filter のみ |

Frontend:

- `DataPipelineStepType` に `quarantine` を追加する。
- palette の quality category に `quarantine` を追加する。
- default config は `statusColumn: "gate_status"`, `matchValues: ["needs_review"]`, `outputMode: "quarantine_only"`, `mode: "filter"` にする。
- Inspector に `statusColumn`、`matchValues`、`outputMode` を編集する最小 UI を追加する。
- Runs tab の metadata summary で `quarantinedRows` と `passedRows` を読めるようにする。
- i18n に英語 / 日本語の label、description、option label を追加する。

Graph design:

- v1 では node 自体に 2 つの出力 port は作らない。
- 通常 output と quarantine output を両方作る場合は、`confidence_gate` から 2 本の branch を伸ばし、片方に `quarantine(outputMode=pass_only)`、もう片方に `quarantine(outputMode=quarantine_only)` を置く。
- 既存の `data_pipeline_run_outputs` により、2 つの output node を別 Work table として保存する。

Smoke scenario:

- `drive_file -> extract_text -> quality_report -> confidence_gate` まで既存 text scenario と同じ流れにする。
- `confidence_gate -> quarantine_pass(outputMode=pass_only) -> output_pass`
- `confidence_gate -> quarantine_review(outputMode=quarantine_only) -> output_quarantine`
- `gate_status = needs_review` の行が quarantine output に入り、pass output から除外されることを検証する。

### Phase 2: 失敗理由の強化

状態: 完了済み。

完了コミット前の確認:

- `/Users/knakajima/.asdf/installs/golang/1.23.0/go/bin/go test ./backend/...`
- `npm --prefix frontend run build`
- `node --check scripts/smoke-data-pipeline.mjs`
- `make smoke-data-pipeline-suite`

Phase 1 の後に、run detail と後続 node が失敗理由をより明確に読めるようにします。

`confidence_gate`:

- `gate_reason` 列を追加する。
- score column 欠損、数値変換不能、threshold 未満を reason として区別する。
- metadata に `lowConfidenceSamples` を最大 5 件保存する。sample は既存方針どおり mask 済み、件数上限つきにする。
- metadata に `minScore`、`avgScore`、`scoreColumns`、`threshold`、`passRows`、`needsReviewRows` を維持する。

`quality_report`:

- metadata に missing rate threshold 超過列を `warnings` として保存する。
- `warningCount` は threshold 超過または欠損率ありの列数として UI に出せる形にする。
- 将来の前回 run 比較に備え、metadata key は `quality.rowCount`、`quality.columnCount`、`quality.missingRate`、`quality.summary` を維持する。

`validate`:

- v1 の行データは passthrough のまま維持する。
- 将来の quarantine 連携用に、行単位の `validation_status` と `validation_errors_json` を追加する案を設計に残す。
- 実装する場合は structured / hybrid の両方で同じ列 contract にする。

### Phase 3: `human_review` queue 化

`human_review` を注釈列から永続 review queue の入口へ拡張します。この phase では DB migration、API、UI が必要です。

Status: 実装済み。2026-05-14 に review item 永続化、list / detail / transition / comment API、Data Pipeline detail 下部 panel の Reviews tab を追加しました。`human_review` は既存の注釈列挙動を維持しつつ、`createReviewItems: true` の場合だけ `needs_review` 行から tenant-scoped review item を作成します。

DB model 方針:

- `data_pipeline_review_items` を追加する。
- tenant、pipeline、version、run、step、source output / source row、reason、status、assignee、created by、updated by を持つ。
- status は `open`、`approved`、`rejected`、`needs_changes`、`closed` を想定する。
- raw row 全体を無制限に保存しない。v1 は masked snapshot と source reference を保存する。

API 方針:

- browser session + CSRF 必須にする。
- tenant 外 review item は 404 として扱う。
- list / detail / transition / comment を最小 endpoint とする。
- status transition は audit log を残す。

Pipeline 連携:

- `human_review` node に `createReviewItems: boolean` を追加する。
- 既定は false とし、注釈列だけの既存挙動を維持する。
- true の場合、`review_status = needs_review` の行から review item を作る。
- duplicate run で同じ item が重複しすぎないよう、pipeline run id + step id + row fingerprint を idempotency key として扱う。

UI 方針:

- Data Pipeline detail に review items の入口を追加する。
- review item detail では reason、source row snapshot、run / step、承認状態を表示する。
- v1 では修正値の再投入は行わない。承認 / 差し戻し / comment までに留める。

実装結果:

- `0048_data_pipeline_review_items` migration で `data_pipeline_review_items` と `data_pipeline_review_item_comments` を追加した。
- `human_review(createReviewItems=true)` は `review_status = needs_review` の行を review item draft として run result に渡し、run 完了処理で PostgreSQL に upsert する。
- source fingerprint は `node_id + source snapshot` の SHA-1 を使い、`tenant_id + run_id + node_id + source_fingerprint` で同一 run 内の重複を防ぐ。
- review item は既定 1000 件まで作成し、`reviewItemLimit` config で最大 10000 件まで調整できる。超過時は step metadata に `review_item_limit_exceeded` warning を残す。
- API は `GET /api/v1/data-pipelines/{pipelinePublicId}/review-items`、`GET /api/v1/data-pipeline-review-items/{reviewItemPublicId}`、`POST /api/v1/data-pipeline-review-items/{reviewItemPublicId}/transition`、`POST /api/v1/data-pipeline-review-items/{reviewItemPublicId}/comments` を追加した。
- review item detail は対象 pipeline の `can_view`、transition / comment は対象 pipeline の `can_update` を service 層で確認する。
- Frontend は API client / store / Data Pipeline detail 下部 panel の Reviews tab を追加した。detail 画面ロード時と run refresh 時に open review items を取得する。

### Phase 4: Drive / OCR / extraction 連携

Drive から入った低信頼データが、Pipeline 上で隔離と review に届く導線を作ります。

Status: 完了。2026-05-14 に `extract_fields` / `extract_table` / `schema_mapping` / `product_extraction` の信頼度 metadata と、Drive text / JSON / product extraction -> `confidence_gate` -> `human_review(createReviewItems=true)` の smoke を追加した。Reviews tab では source snapshot から Drive file / OCR run の trace を表示し、Drive file detail から `source_snapshot.file_public_id` に紐づく review item / pipeline run へ戻る導線も追加済み。

対象:

- OCR confidence が低い page / file。
- `extract_fields` の field confidence が低い行。
- schema mapping candidate の confidence が低い列または mapping。
- product extraction の低信頼 result。

代表 workflow:

```text
drive_file
  -> extract_text
  -> quality_report
  -> confidence_gate
  -> quarantine(outputMode=pass_only)
  -> output(normal)

confidence_gate
  -> quarantine(outputMode=quarantine_only)
  -> human_review(createReviewItems=true)
  -> output(review_candidates)
```

UI 方針:

- Pipeline Runs tab から normal output と quarantine output を区別して表示する。
- Drive file detail / OCR result 側から該当 pipeline run または review item へ遷移できるようにする。Drive file detail 側は `GET /api/v1/drive/files/{filePublicId}/data-pipeline-review-items` と UI section を実装済み。
- 低信頼理由は `gate_reason`、`review_reason_json`、run step metadata の順に表示する。

実装結果:

- `extract_fields` の run step metadata に `fieldExtraction` を追加した。
- `fieldExtraction` には field count、row count、average confidence、low confidence rows、missing required rows、field 別 extracted / missing rows、low confidence sample を保存する。
- Drive file detail は related pipeline reviews section を表示し、review item の status、pipeline name、node / queue、reason、Data Pipeline detail へのリンクを表示する。
- `field_review` / `table_review` smoke は Drive file scoped review item API が pipeline / run public ID を返すことまで検証する。
- `field_confidence` を `confidence_gate.scoreColumns` に渡す smoke `field_review` を追加した。
- `field_review` smoke は required field 欠落を `field_confidence = 0.6667` として検出し、review item の source snapshot に抽出列、`field_confidence`、`gate_status`、`gate_reason` が残ることを検証する。
- `extract_table` の run step metadata に `tableExtraction` を追加した。
- `tableExtraction` には table count、row count、expected column count、average confidence、low confidence rows、missing cell rows、low confidence sample を保存する。
- `table_confidence` を `confidence_gate.scoreColumns` に渡す smoke `table_review` を追加した。
- `table_review` smoke は欠損 cell を `table_confidence = 0.7500` として検出し、review item の source snapshot に `row_json`、`table_confidence`、`table_missing_cell_count`、`gate_status`、`gate_reason` が残ることを検証する。
- `schema_mapping` の hybrid path は `schema_mapping_confidence`、`schema_mapping_status`、`schema_mapping_reason`、`schema_mapping_json` を出力し、run step metadata に `schemaMapping` summary を保存する。
- `schema_mapping_review` smoke は低信頼 mapping を `schema_mapping_confidence = 0.8200` として検出し、review item の source snapshot に mapping JSON、mapped columns、`gate_status`、`gate_reason` が残ることを検証する。
- `product_extraction` node は既存の Drive product extraction item を `product_*` columns へ展開し、`product_confidence`、`product_extraction_status`、`product_extraction_reason`、証跡 JSON を出力する。
- `product_review` smoke は OCR / product extraction job を完了させた後、`product_extraction` -> `confidence_gate` -> `human_review(createReviewItems=true)` を実行し、confidence missing / low confidence item が review item と Drive file detail link に残ることを検証する。
- `human_review` Inspector に `createReviewItems`、`queue`、`reviewItemLimit` を追加した。

## Public Interfaces

Phase 1 で追加する public contract:

- step type: `quarantine`
- config keys: `statusColumn`, `matchValues`, `outputMode`, `mode`
- metadata keys: `quarantinedRows`, `passedRows`, `statusColumn`, `matchValues`, `outputMode`

既存 API の `DataPipelineRunStepBody.metadata` は `Record<string, unknown>` のまま使うため、OpenAPI schema の大きな変更は不要です。frontend の手書き `DataPipelineStepType` は更新します。

Phase 3 で追加する public contract:

- review item list / detail / transition / comment API。
- review item body は tenant 境界、audit、CSRF を前提に設計する。
- generated API client を更新する場合は `make gen` 経由で行う。

## Test Plan

Phase 1:

- `/Users/knakajima/.asdf/installs/golang/1.23.0/go/bin/go test ./backend/...`
- `npm --prefix frontend run build`
- `node --check scripts/smoke-data-pipeline.mjs`
- `make smoke-data-pipeline-review`
- `make smoke-data-pipeline-field-review`
- `make smoke-data-pipeline-table-review`
- `make smoke-data-pipeline-suite`
- `git diff --check`

Backend test 観点:

- `quarantine` が graph validation で有効な step type として扱われる。
- `statusColumn` が upstream にない場合は分かりやすい error になる。
- `quarantine_only` は match rows だけを出力する。
- `pass_only` は non-match rows だけを出力する。
- metadata の `quarantinedRows` と `passedRows` が upstream 全体に対する件数になる。

Frontend test 観点:

- palette から `quarantine` を追加できる。
- Inspector で config を編集できる。
- `matchValues` は comma separated input で配列として保存される。
- Runs tab は metadata key が欠けても壊れない。

Smoke test 観点:

- text scenario に quarantine branch を追加する。
- pass output と quarantine output が別 Work table として登録される。
- quarantine output の row count が `confidenceGate.needsReviewRows` と一致する。

Phase 3:

- review item API は tenant 外 item を 404 にする。
- transition API は CSRF と audit を必須にする。
- duplicate run / retry で review item が重複作成されすぎない。

## Implementation Constraints

- `backend/internal/db/*`、`frontend/src/api/generated/*`、`openapi/*.yaml` は generator 経由で更新する。
- review item の DB migration を追加するまでは、Phase 1 で DB schema を変更しない。
- quarantine table には機密情報が残りやすいため、`redact_pii` と組み合わせる設計を UI / docs で明示する。
- audit / logs / metrics に tenant id、user id、email、public id、storage key、raw row、raw token を入れない。
- LLM node は Month 4 の範囲。Month 2 では LLM の低信頼結果を受け止める failure handling の土台だけを作る。

## Next Implementation Task

次に実装するのは review item 権限 test の補強と、Month 3 の graph runtime 強化です。

最小完了条件:

1. product extraction / schema mapping の低信頼 review smoke を suite で維持する。
2. review item の pipeline view / update / Drive file view 権限 test を追加する。
3. Drive 由来の run / review item を source file から追跡できる smoke を維持する。
4. review item detail から source run / step / Drive file を追跡できる UI 情報をさらに増やす。
5. tenant boundary、CSRF、audit、review item transition の自動 test を追加する。
