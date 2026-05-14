# Data Pipeline UI Column Inference Notes

作成日: 2026-05-14

## Summary

Data Pipeline Inspector の「設定済みの列が上流ステップの出力にありません」警告で、実行時には存在する列が UI だけで存在しないと判定される false positive が発生した。この文書は、問題、背景、原因、対応済み内容、検証結果、今後の恒久対策をまとめる。

結論は、backend runtime の出力列と frontend Inspector の静的列推論が別々に実装されていたことが根本原因である。直近では `DataPipelineInspector.vue` の node 別推論を backend の materializer と合わせて修正した。次にやるべきことは、step output schema を backend catalog か generated contract として単一正本化し、frontend がそこから列候補を作る形へ寄せること。

## Problem

Data Pipeline detail の Inspector は、選択 node の設定が上流 step の列を参照しているかを確認し、存在しない列があれば次の警告を出す。

```text
設定済みの列が上流ステップの出力にありません: ...
Configured columns are not available from the upstream step: ...
```

この警告は本来、`orderBy`、`scoreColumns`、`reasonColumns`、`columns`、mapping source などで本当に存在しない列を参照したときに、run 前に設定ミスを見つけるためのもの。しかし今回、backend 実行では成功している pipeline で UI だけが誤警告を出した。

確認した代表事例:

- `http://localhost:5173/data-pipelines/019e24b7-06bd-7ff9-aed5-c9e496b5f86b`
  - `quality_report.columns = ["text", "confidence"]`
  - 直前の `extract_text` は実行時に `text` と `confidence` を出力する。
  - UI は `extract_text` を passthrough 相当と推論していたため、`text, confidence` を上流にない列として警告した。
- `http://localhost:5173/data-pipelines/019e24b7-498d-7334-9c27-c24142133e73`
  - `output.orderBy = ["file_public_id"]`
  - 上流の `input` は Drive JSON で `file_public_id` を出し、`json_extract.includeSourceColumns=true`、`schema_mapping.includeSourceColumns=true`、`human_review` は上流列を保持する。
  - UI は `schema_mapping` の出力を target columns のみと推論していたため、`file_public_id` を上流にない列として警告した。

どちらも run は `completed` であり、backend / DB / ClickHouse の runtime failure ではなかった。問題は frontend Inspector の静的な列候補計算だけにあった。

## Background

Data Pipeline には大きく 2 つの実行経路がある。

- structured path
  - Dataset / Work table を ClickHouse SQL CTE に compile して実行する。
  - 主に `input`, `clean`, `normalize`, `schema_mapping`, `schema_completion`, `join`, `enrich_join`, `transform`, `output` が対象。
- hybrid path
  - Drive file input、OCR、JSON / Excel、extract 系 node を含む場合に使う。
  - node ごとに ClickHouse 中間 table を materialize し、後続 node がその table を読む。

Runtime では、各 materializer が実際の出力列を決める。たとえば:

- `extract_text`
  - `file_public_id`, `ocr_run_public_id`, `page_number`, `text`, `confidence`, `layout_json`, `boxes_json`
- `quality_report`
  - 上流列に `quality_report_json`, `missing_rate_json`, `validation_summary_json` を追加する。
- `confidence_gate`
  - 上流列に `gate_score`, `gate_status` または設定済み `statusColumn`, `gate_reason` を追加する。
- `schema_mapping`
  - `includeSourceColumns=true` なら上流列を保持し、target columns と `schema_mapping_confidence`, `schema_mapping_status`, `schema_mapping_reason`, `schema_mapping_json` を追加する。
  - `includeSourceColumns=false` なら target columns と schema mapping metadata columns を中心に出す。
- `human_review`
  - 上流列に `review_status`, `review_queue`, `review_reason_json` を追加する。
- `product_extraction`
  - `includeSourceColumns=true` なら Drive file metadata を保持し、product extraction の各列を追加する。

一方、frontend の Inspector は run を待たずに warning を出す必要があるため、graph config だけから「この node の出力列」を推論している。入口は `frontend/src/components/DataPipelineInspector.vue` の `columnsForNodeOutput()` である。

この構造自体は妥当だが、runtime の出力列 contract と UI の推論が別々に hardcode されているため、どちらかだけを更新すると差分が生まれる。

## Root Cause

根本原因は、step output schema の正本がひとつではないこと。

具体的な原因:

- `columnsForNodeOutput()` が、backend materializer が列を追加する一部 node を default passthrough として扱っていた。
- `extract_text` の実出力列を Inspector が知らず、Drive file input の `file_public_id`, `file_name`, `mime_type`, `file_revision` だけを上流列として見ていた。
- `schema_mapping` は hybrid path で `includeSourceColumns=true` の場合に上流列を保持するが、UI は mapping の `targetColumn` だけを返していた。
- `human_review` が追加する `review_status`, `review_queue`, `review_reason_json` を UI が推論していなかった。
- `product_extraction`, `extract_fields`, `extract_table`, `classify_document`, `deduplicate`, `canonicalize`, `redact_pii`, `detect_language_encoding`, `schema_inference`, `entity_resolution`, `unit_conversion`, `relationship_extraction`, `sample_compare` なども、runtime が追加する列と UI 推論が完全には揃っていなかった。
- `detect_language_encoding` の configured column warning は `config.column` を見ていたが、UI/backend の実設定は `textColumn` である。

この手の bug は、run smoke だけでは見つけにくい。run は成功しているため、backend smoke は緑になる。UI の static warning を見る browser check、または UI 推論列と representative graph config を照合する test が必要になる。

## Implemented Fix

対応済みコミット:

- `4757732 Fix data pipeline inspector column inference`
- `bfcbb4a Align data pipeline inspector column inference`

対応内容:

- `DataPipelineInspector.vue` の `columnsForNodeOutput()` に node 別の出力列推論を追加した。
- `extract_text` の実出力列を推論するようにした。
- `quality_report` の追加列 `quality_report_json`, `missing_rate_json`, `validation_summary_json` を推論するようにした。
- `confidence_gate` の `gate_score`, `gate_status`, `gate_reason` を推論するようにした。
- `schema_mapping.includeSourceColumns=true` の場合に上流列を保持するようにした。
- `human_review` の `review_status`, `review_queue`, `review_reason_json` を推論するようにした。
- `product_extraction` は `includeSourceColumns` に従って product extraction columns と上流列を推論するようにした。
- `extract_fields`, `extract_table`, `classify_document`, `deduplicate`, `canonicalize`, `redact_pii`, `detect_language_encoding`, `schema_inference`, `entity_resolution`, `unit_conversion`, `relationship_extraction`, `sample_compare` の追加列を推論するようにした。
- `detect_language_encoding` の参照列 warning を `config.textColumn` に合わせた。
- `frontend/src/utils/data-pipeline-step-output-schema.ts` を追加し、runtime step の出力列推論を Vue component から純粋関数の contract module へ切り出した。
- `DataPipelineInspector.vue` は `input`, `transform`, `join`, `enrich_join` のような graph / source selection 依存の推論だけを保持し、それ以外の step output schema は共有 module を参照するようにした。

今回の修正は frontend の warning と preview の列候補を backend runtime に近づけるもの。backend materializer、DB schema、保存済み graph は変更していない。

### Step Output Schema Contract Module

今回追加した `frontend/src/utils/data-pipeline-step-output-schema.ts` は、backend runtime の出力列 contract を frontend で再利用しやすくするための中間段階である。

目的:

- `DataPipelineInspector.vue` に node ごとの出力列 hardcode が集中する状態を解消する。
- UI warning、Preview の列候補、将来の form helper が同じ推論関数を使えるようにする。
- 将来的に backend catalog / generated contract へ移行するとき、置き換える境界を明確にする。

現時点で module が扱う step:

- passthrough: `profile`, `clean`, `normalize`, `validate`, `output`, `quarantine`
- extractor / unstructured: `extract_text`, `json_extract`, `excel_extract`, `classify_document`, `extract_fields`, `extract_table`, `product_extraction`
- quality / failure handling: `quality_report`, `confidence_gate`, `deduplicate`, `sample_compare`, `human_review`
- normalization / enrichment: `canonicalize`, `redact_pii`, `detect_language_encoding`, `schema_inference`, `entity_resolution`, `unit_conversion`, `relationship_extraction`
- structured helper: `schema_mapping`, `schema_completion`

Inspector に残している step:

- `input`
  - dataset / work table / drive file / manifest / source config に依存するため、画面側の `datasets`, `workTables`, `driveManifest` が必要。
- `transform`
  - `select_columns`, `drop_columns`, `rename_columns`, `aggregate` など、既存 UI helper と密接に結びついている。
- `join`
  - left / right の複数 upstream branch を同時に見る必要がある。
- `enrich_join`
  - primary upstream と right source selection を同時に見る必要がある。

この module はまだ backend 由来の generated schema ではない。そのため「単一正本」の最終形ではなく、frontend 内の重複を減らすための v1 contract である。最終的には backend が node ごとの `inferredOutputColumns` を返し、frontend はそれを優先して表示する形にする。

### Backend Preview Output Schemas

次の段階として、preview API が selected subgraph の node 別 output schema を返すようにした。

追加した backend contract:

- `service.DataPipelineNodeOutputSchema`
  - `nodeId`
  - `stepType`
  - `columns`
  - `warnings`
- `service.DataPipelinePreview.OutputSchemas`
- API response `DataPipelinePreviewBody.outputSchemas`

対象 endpoint:

- `POST /api/v1/data-pipeline-versions/{versionPublicId}/preview`
- `POST /api/v1/data-pipelines/{pipelinePublicId}/preview`

挙動:

- preview 対象 node までの subgraph を作る。
- backend が topological order で各 node の output columns を推論する。
- preview 実行結果の `columns` / `previewRows` と同じ response に `outputSchemas` を同梱する。
- frontend `DataPipelineInspector.vue` は、`props.preview.outputSchemas` に対象 node の schema があればそれを優先し、なければ `data-pipeline-step-output-schema.ts` の local fallback を使う。

現時点の役割分担:

- backend `outputSchemas`
  - preview 済み subgraph についての優先 contract。
  - dataset / work table input は backend service で実際の source columns を解決できる。
  - Drive input、extract 系、quality / review 系、transform / join 系の代表的な列推論を持つ。
- frontend local fallback
  - preview 前の即時 UI feedback を維持する。
  - backend preview が未実行、または選択 graph が変わって preview cache が無効になった場合に使う。

この段階で完全に frontend hardcode を消してはいない。理由は、Inspector の warning は preview 実行前にも必要であり、preview API を毎 keystroke で呼ぶ設計にはしていないため。今後は軽量な validation-only endpoint を追加し、preview 実行なしで backend schema / warning を取得できるようにする。

実装上の注意:

- `outputSchemas` は selected subgraph のみを返す。graph 全体ではない。
- `input`, `enrich_join` の dataset / work table source columns は backend の DatasetService で解決する。
- `drive_file` input は UI と同じく file metadata columns、spreadsheet mode、json mode を静的に推論する。
- `schema_mapping` は frontend fallback と同じく、structured / hybrid の差異で false positive を増やさないため、v1 では target columns と `includeSourceColumns` を中心に扱う。
- OpenAPI (`openapi/openapi.yaml`, `openapi/browser.yaml`) と generated frontend SDK (`frontend/src/api/generated/*`) も更新対象。

## Verification

実施済みの確認:

```bash
npm --prefix frontend run build
go test ./backend/internal/service ./backend/internal/api
make smoke-data-pipeline-quarantine
make smoke-data-pipeline-field-review
make smoke-data-pipeline-product-review
git diff --check
```

ブラウザ確認:

- `019e24b7-06bd-7ff9-aed5-c9e496b5f86b`
  - `Quality report` 選択時に `text, confidence` の上流列警告が出ないことを確認した。
- `019e24b7-498d-7334-9c27-c24142133e73`
  - `Output` の `orderBy=file_public_id` に対する上流列警告が出ないことを確認した。
- product review smoke で作成された pipeline
  - product extraction / confidence gate / human review / output の画面で上流列警告が出ないことを確認した。
- `019e2564-2509-7295-8cec-94641b452dc9`
  - product review smoke で作成された pipeline。
  - `Product extraction` と `Confidence gate` の Inspector / Preview で `product_confidence` と gate 追加列が上流列として扱われ、上流列警告が出ないことを確認した。
- `019e25dd-d395-73f6-a0fb-89093478bd3e`
  - product review smoke で作成された pipeline。
  - version preview API を `nodeId=output` で呼び、`outputSchemas` が 5 node 分返ることを確認した。
  - output schema に `file_public_id`, `product_confidence`, `gate_score`, `gate_status`, `review_status`, `review_queue` が含まれることを確認した。

補足:

- `quality_report` の `confidence missing rate 1.0000 exceeded 0.0000` は別問題である。これは列が存在しないという意味ではなく、列は存在するが値が空または欠損しているという品質 warning であり、想定される runtime metadata である。
- agent-browser では `Smoke output` node のクリックが選択切り替えに至らなかったため、今回のブラウザ確認は product extraction / confidence gate まで。Output は pass-through contract として module 側に定義されており、前回修正済みの `file_public_id` propagation を保持する。
- `make gen` は local `sqlc` binary が PATH に無く失敗したため、今回の API 変更では `GOCACHE=/private/tmp/haohao-go-build go run ./backend/cmd/openapi ...` と `npm --prefix frontend run openapi-ts` を個別に実行した。

## Operational Guidance

同じ種類の問題を調査するときは、次の順序で見る。

1. 対象 pipeline の graph config を確認する。
2. 該当 step の backend materializer / compiler が実際に返す `Columns` を確認する。
3. `DataPipelineInspector.vue` の `columnsForNodeOutput()` が同じ列を返すか確認する。
4. 警告対象の設定 field が `configuredPrimaryColumnRefs()` で正しい config key を見ているか確認する。
5. run が失敗しているのか、UI warning だけなのかを分けて判断する。

判断基準:

- run step が `completed` で output まで成功している場合、UI warning は false positive の可能性が高い。
- backend materializer が `dataPipelineRequireColumn()` で失敗している場合、実設定ミスの可能性が高い。
- warning が `missing rate` や `lowConfidenceSamples` 由来なら、列存在ではなくデータ品質の問題として扱う。

## Future Plan

短期:

- `frontend/src/utils/data-pipeline-step-output-schema.ts` の step output schema を維持する。
- 新しい node または出力列を増やす場合、backend materializer / compiler と frontend step output schema module を同じ PR で更新する。
- representative smoke graph で、`orderBy`, `columns`, `scoreColumns`, `reasonColumns`, `statusColumn`, `sourceFileColumn` が UI 推論上も存在することを確認する。

中期:

- 2026-05-14 に、preview 実行なしで `outputSchemas` と missing-column warnings を返す軽量 validation endpoint を追加した。詳細は `docs/DATA_PIPELINE_VALIDATION_ENDPOINT_PLAN.md` を参照する。
- backend の `dataPipelineStepCatalog` または近い場所に output schema metadata を持たせる。
- frontend は `data-pipeline-step-output-schema.ts` の手書き contract を endpoint 未取得時の fallback に寄せ、最終的には generated contract または API から取得した step schema を使う。
- static output schema だけでは表現できない config-dependent columns は、次のような resolver として定義する。
  - passthrough upstream columns を保持するか。
  - config のどの field が output column name になるか。
  - fixed columns は何か。
  - mode によって column set が変わるか。

長期:

- graph validation / preview API が、node ごとの inferred output schema と warning を返せるようにする。
- frontend は local inference ではなく backend validation result を優先表示する。
- UI、runtime、docs、smoke が同じ schema contract を参照し、step 追加時の二重実装をなくす。

## New Node Checklist

Data Pipeline node を追加または変更するときは、最低限次を確認する。

- Backend catalog に step type がある。
- structured compiler または hybrid materializer のどちらで実行されるかが明確。
- 出力列が明文化されている。
- `DataPipelineInspector.vue` の列推論が runtime と一致している。
- Inspector の config field が正しい上流列 key を検証している。
- Preview / Run で代表 graph が動く。
- `orderBy`, `scoreColumns`, `reasonColumns`, `columns` など downstream が参照する列が false positive にならない。
- docs の node 説明と smoke scenario が更新されている。
