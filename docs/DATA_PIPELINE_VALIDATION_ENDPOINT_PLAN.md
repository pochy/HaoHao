# Data Pipeline Lightweight Validation Endpoint Plan

作成日: 2026-05-14

実装日: 2026-05-14

## Implementation Status

この計画の初期実装は完了した。

実装済みの内容は次の通り。

- backend に `DataPipelineGraphValidation` と `DataPipelineNodeWarning` を追加した。
- `POST /api/v1/data-pipelines/{pipelinePublicId}/validate` を追加した。
- validation endpoint は draft graph を受け取り、preview 実行なしで次を返す。
  - `validationSummary`
  - graph 全体の `outputSchemas`
  - node ごとの `nodeWarnings`
- backend の既存 `inferOutputSchemas` を validation endpoint でも再利用するようにした。
- `orderBy`、`columns`、`leftKeys`、`rightKeys`、`scoreColumns`、`reasonColumns` など、Inspector が参照する代表 config column refs を backend 側でも検査するようにした。
- frontend store に graph signature 単位の validation cache と debounce 付き auto validation を追加した。
- `DataPipelineInspector.vue` は validation result がある場合、preview / local 推論より validation result を優先する。
- Huma OpenAPI と frontend generated SDK を `make gen` で更新した。
- 代表的な false positive である `extract_text -> quality_report` の `text, confidence` が warning にならないことを backend unit test で固定した。

検証済みの内容は次の通り。

- `go test ./backend/internal/service ./backend/internal/api`
- `GOCACHE=/private/tmp/haohao-go-build go test ./...`
- `npm --prefix frontend run build`
- `GOCACHE=/private/tmp/haohao-go-build make gen`
- `git diff --check`

残っている作業は、実データを使ったブラウザ上の回帰確認と、validation result を画面上でより明示的に表示する UX 改善である。現時点では Inspector warning の primary source として validation result を使うところまで実装済みで、validation summary 自体を専用 UI として表示するところまでは含めていない。

2026-05-16 追記:

- `route_by_condition`、`union`、`partition_filter`、`watermark_filter`、typed output、`snapshot_scd2`、output `append` / `scd2_merge` 追加後も、validation endpoint と Inspector の warning contract は維持している。
- output `writeMode=scd2_merge` では `uniqueKeys` を上流列参照として検査対象に含めた。
- `orderBy` は最終出力列に対する設定なので、上流列不足 warning には含めない方針を維持している。
- 今後の UX 改善では、graph issue summary に SCD2 / output / Gold publish に関係する設定不足を grouped warning として表示すると、Inspector を開かなくても問題を見つけやすくなる。

## Summary

Data Pipeline Inspector の列警告を、frontend の local 推論中心から backend validation contract 中心へ移すため、preview 実行なしで graph validation、node output schema、missing-column warnings を返す軽量 validation endpoint を追加する。

現在は `DataPipelineInspector.vue` が graph config だけから上流列を推論し、`orderBy`、`columns`、`scoreColumns`、`reasonColumns` などの設定が上流列に存在するかを警告している。短期対応として frontend に step output schema module を追加し、さらに preview API が selected subgraph の `outputSchemas` を返すようになった。しかし、preview API は実データを読む重い操作であり、Inspector の即時警告に必要な validation source としてはまだ不十分である。

次の実装では、draft graph を backend に渡すだけで、実行や preview を行わずに次を返す。

- graph validation summary
- graph 全体の node output schemas
- node ごとの missing-column warnings
- frontend Inspector が優先表示する warning payload

この endpoint により、runtime、preview、Inspector warning の列 contract を backend に寄せ、`file_public_id`、`text`、`confidence`、`product_confidence` のような runtime では存在する列が UI だけ不足扱いになる false positive を再発しにくくする。

## Problem

### 表面化した問題

Data Pipeline detail の Inspector では、設定済み列が上流 step の出力に存在しない場合に、次の警告を表示する。

```text
設定済みの列が上流ステップの出力にありません: ...
Configured columns are not available from the upstream step: ...
```

この警告は、run 前に設定ミスを見つけるために必要である。一方で、過去に次の false positive が発生した。

- `extract_text -> quality_report`
  - `quality_report.columns = ["text", "confidence"]`
  - runtime の `extract_text` は `text` と `confidence` を出力する。
  - frontend は `extract_text` の追加列を知らず、UI だけが `text, confidence` を不足扱いにした。
- `schema_mapping(includeSourceColumns=true) -> human_review -> output`
  - `output.orderBy = ["file_public_id"]`
  - runtime では Drive input / JSON extract / schema mapping / human review を通って `file_public_id` が保持される。
  - frontend は `schema_mapping` の出力を target columns のみと推論し、UI だけが `file_public_id` を不足扱いにした。
- `product_extraction -> confidence_gate -> human_review -> output`
  - `confidence_gate.scoreColumns = ["product_confidence"]`
  - runtime では `product_extraction` が `product_confidence` を出す。
  - frontend が product extraction の出力列を知らないと、低信頼 review workflow の UI が誤警告を出す。

これらは run failure ではない。backend 実行は成功し、ClickHouse 中間 table には対象列が存在していた。問題は、frontend の静的推論と backend runtime の出力列 contract がずれていたことにある。

### なぜ preview outputSchemas だけでは足りないか

`23ec2c6 Add data pipeline preview output schemas` で、preview API は selected subgraph の `outputSchemas` を返すようになった。これにより、preview 実行後の Inspector は backend が推論した schema を優先できる。

しかし preview は次の理由で軽量 validation の代替にならない。

- preview は実データを読むため、Drive OCR / JSON / Excel / product extraction を含む graph では重い。
- auto preview が無効な node では、ユーザーが明示的に preview するまで backend schema が取れない。
- Inspector の警告は config editing 中に即時表示したい。
- preview は selected node までの subgraph だけを対象にする。保存・公開前の graph 全体 validation には別 contract が必要。
- preview failure と graph setting warning は意味が違う。重い preview が失敗しても、設定上の column warning だけは返せるべきである。

したがって、preview とは別に、実データを読まない validation-only endpoint が必要である。

## Background

### 現在の frontend 側

現在の frontend は主に次の 2 層で列を推論している。

- `frontend/src/utils/data-pipeline-step-output-schema.ts`
  - step type と config、upstream columns から output columns を返す pure helper。
  - `extract_text`、`product_extraction`、`confidence_gate`、`human_review` など代表 step の出力列を扱う。
- `frontend/src/components/DataPipelineInspector.vue`
  - `input`, `transform`, `join`, `enrich_join` のように graph / source selection 依存の推論を持つ。
  - `configuredPrimaryColumnRefs()` で step config が参照する列を抽出する。
  - `selectedMissingColumnWarnings()` で上流列と比較して warning を作る。
  - `props.preview.outputSchemas` がある場合は preview 由来の backend schema を優先する。

この構成は、preview 前の即時 feedback を保つためには有効である。ただし frontend に runtime contract の写しを持つため、backend materializer / compiler が変わるたびに追随が必要になる。

### 現在の backend 側

backend は既に次を持つ。

- `backend/internal/service/data_pipeline_graph.go`
  - graph validation、step catalog、topological order。
- `backend/internal/service/data_pipeline_output_schema.go`
  - preview subgraph 用の output schema inference。
  - `DataPipelineNodeOutputSchema` を返す。
- `backend/internal/service/data_pipeline_service.go`
  - `Preview` / `PreviewDraft` / `previewGraph`。
  - preview response に `OutputSchemas` を含める。
- `backend/internal/api/data_pipelines.go`
  - `DataPipelinePreviewBody.outputSchemas` を API response として返す。

この backend 推論は、preview subgraph 用としては動作確認済みである。次の実装では、同じ output schema inference を validation endpoint からも使う。

### 生成物と正本

API schema の正本は backend Huma registration である。変更後は次を更新する。

```bash
GOCACHE=/private/tmp/haohao-go-build make gen
```

`make gen` は次を実行する。

- `sqlc generate`
- `openapi/openapi.yaml`
- `openapi/browser.yaml`
- `openapi/external.yaml`
- `frontend/src/api/generated/*`

sqlc query を変更しない場合でも、Huma response type を変えるため OpenAPI と frontend generated SDK は更新対象になる。

## Root Cause

根本原因は、Data Pipeline の「設定時に見える列」と「実行時に存在する列」を返す単一の backend contract がまだないこと。

より具体的には次の通り。

- graph validation は node / edge / DAG / output config の構造チェックが中心で、node output schema や config column reference の妥当性までは返していない。
- preview API は backend schema を返すようになったが、実データを読む重い操作であり、編集時 validation の primary source にはできない。
- frontend local fallback は preview 前の UX を支えるが、backend materializer / compiler と二重管理になる。
- `configuredPrimaryColumnRefs()` 相当の「この step config はどの上流列を参照しているか」という知識が frontend に閉じている。
- runtime materializer が追加する列と、Inspector が候補に出す列が別々に更新されると false positive が再発する。

この状態では、Data Pipeline node が増えるほど次のコストが増える。

- backend runtime 出力列の変更
- frontend output schema helper の変更
- frontend warning key extraction の変更
- docs / smoke / browser check の変更

軽量 validation endpoint は、この二重管理を減らすための次の段階である。

## Current State

対応済み:

- `DataPipelineInspector.vue` の代表 node 出力列推論を runtime に合わせた。
- `frontend/src/utils/data-pipeline-step-output-schema.ts` を追加し、frontend 内の step output schema を component から切り出した。
- preview API が selected subgraph の `outputSchemas` を返すようになった。
- Inspector は preview `outputSchemas` があればそれを優先する。
- field review / product review smoke で代表 workflow の false positive が出ないことを確認した。
- `docs/DATA_PIPELINE_UI_COLUMN_INFERENCE.md` に問題、原因、対応、今後を記録した。

未完了:

- preview 実行なしで backend schema を取得する endpoint がない。
- backend は missing-column warning を返していない。
- frontend は warning 表示の primary source としてまだ local inference を使う。
- `configuredPrimaryColumnRefs()` 相当の config reference extraction が backend にない。
- validation result の cache / debounce / stale handling が frontend store にない。

## Implementation Plan

### 1. Backend service に validation result contract を追加する

追加する service-level type:

```go
type DataPipelineGraphValidationResult struct {
    ValidationSummary DataPipelineValidationSummary
    OutputSchemas     []DataPipelineNodeOutputSchema
    NodeWarnings      []DataPipelineNodeWarning
}

type DataPipelineNodeWarning struct {
    NodeID     string
    StepType   string
    Code       string
    Severity   string
    Message    string
    Columns    []string
    ConfigKeys []string
}
```

初期実装で使う warning code:

| code | severity | 意味 |
| --- | --- | --- |
| `missing_upstream_columns` | `warning` | selected node config が参照する primary upstream columns が存在しない |
| `missing_right_upstream_columns` | `warning` | join node の right keys が right upstream columns に存在しない |

初期実装では、warning は column existence に絞る。品質 warning、missing rate、confidence threshold、row count anomaly は runtime / preview / run metadata の責務として残す。

### 2. Backend に graph 全体の output schema inference を追加する

既存の `inferOutputSchemas(ctx, tenantID, graph)` を validation endpoint でも使う。

方針:

- preview subgraph ではなく、draft graph 全体を対象にする。
- topological order で node ごとの output columns を計算する。
- graph validation が DAG と node existence で失敗する場合でも、返せる範囲の validation summary を返す。
- source 解決が必要な `input` / `enrich_join` は DatasetService / WorkTable metadata を読む。
- Drive file input は実ファイルの中身を読まず、config から metadata columns / spreadsheet columns / JSON fields を推論する。

注意:

- `inferOutputSchemas` が graph structural error で失敗する場合、API は 200 で `validationSummary.valid=false` を返す設計にする。transport error と graph validation error を混ぜない。
- dataset / work table public id が存在しない場合は、`validationSummary.errors` に source resolution error を含める。
- permission error は 403 とし、validation summary にはしない。

### 3. Backend に configured column reference extraction を追加する

frontend の `configuredPrimaryColumnRefs()` と同等の処理を backend に実装する。

初期対象:

| step | primary refs |
| --- | --- |
| `json_extract` | `sourceColumn` |
| `excel_extract` | `sourceFileColumn` |
| `clean` | `rules[].column`, `rules[].columns`, `rules[].keys`, `rules[].orderBy` |
| `normalize` | `rules[].column` |
| `validate` | `rules[].column` |
| `schema_mapping` | `mappings[].sourceColumn` |
| `schema_completion` | `rules[].sourceColumn`, `rules[].sourceColumns` |
| `join` | `leftKeys`; right refs は `rightKeys` |
| `transform` | operation ごとの `columns`, `renames`, `conditions[].column`, `sorts[].column`, `groupBy`, `aggregations[].column` |
| `schema_inference` | `columns` |
| `quality_report` | `columns` |
| `deduplicate` | `columns` |
| `redact_pii` | `columns` |
| `quarantine` | `statusColumn` |
| `canonicalize` | `rules[].column` |
| `classify_document` | `textColumn` |
| `relationship_extraction` | `textColumn` |
| `detect_language_encoding` | `textColumn` |
| `unit_conversion` | `rules[].valueColumn`, `rules[].unitColumn` |
| `sample_compare` | `pairs[].beforeColumn`, `pairs[].afterColumn` |
| `output` | `orderBy` |

`extract_fields`, `extract_table`, `product_extraction`, `confidence_gate`, `human_review` などは、初期実装では backend に既存 frontend と同じ参照 key を持たせる。`confidence_gate.scoreColumns` や `human_review.reasonColumns` が存在する場合は対象に含める。

### 4. Missing-column warning resolver を追加する

resolver の流れ:

1. graph を topological order で読む。
2. node ごとの upstream ids を作る。
3. output schema inference の結果から `columnsByNodeID` を作る。
4. node config が参照する primary columns を抽出する。
5. primary upstream columns と比較し、存在しない列を `missing_upstream_columns` として返す。
6. `join` は left / right を分けて比較し、right missing は `missing_right_upstream_columns` にする。
7. warning message は backend では英語固定または code 中心にし、frontend i18n で表示文言を組み立てる。

重複排除:

- columns は trim して空文字を除外する。
- 同じ column が複数 config key から出ても 1 回だけ warning に含める。
- `configKeys` には warning の原因になった config path を入れる。例: `["columns"]`, `["scoreColumns"]`, `["rules[].column"]`。

### 5. API endpoint を追加する

追加 endpoint:

```http
POST /api/v1/data-pipelines/{pipelinePublicId}/validate
```

request body:

```json
{
  "graph": {
    "nodes": [],
    "edges": []
  }
}
```

response body:

```json
{
  "validationSummary": {
    "valid": true,
    "errors": []
  },
  "outputSchemas": [
    {
      "nodeId": "extract_text",
      "stepType": "extract_text",
      "columns": ["file_public_id", "text", "confidence"],
      "warnings": []
    }
  ],
  "nodeWarnings": [
    {
      "nodeId": "quality_report",
      "stepType": "quality_report",
      "code": "missing_upstream_columns",
      "severity": "warning",
      "message": "Configured columns are not available from the upstream step.",
      "columns": ["amount"],
      "configKeys": ["columns"]
    }
  ]
}
```

API rules:

- tenant / session / CSRF は preview draft endpoint と同じ。
- pipeline permission は `DataActionPreview` ではなく、設計検証に相当する既存 action を確認する。既存に適切な action がなければ `DataActionPreview` を使う。
- graph が空、unsupported step、cycle などは 200 + `validationSummary.valid=false` とする。
- authorization failure、tenant missing、pipeline not found は既存 error handling に従う。
- OpenAPI / generated SDK を `make gen` で更新する。

### 6. Frontend store に validation cache を追加する

`frontend/src/stores/data-pipelines.ts` に draft validation state を追加する。

追加 state:

- `validationByGraphSignature`
- `validationLoading`
- `validationError`
- `selectedValidation`

挙動:

- draft graph が変わったら debounce して validation API を呼ぶ。
- debounce は既存 auto preview より軽く、300-500ms 程度を既定にする。
- 同じ graph signature の result は cache する。
- request 中に graph が変わった場合は、古い result を現在 graph に適用しない。
- API failure 時は local fallback warning を維持し、画面全体を blocker にしない。

### 7. Inspector warning 表示を backend 優先にする

`DataPipelineInspector.vue` の warning source priority:

1. validation endpoint の `nodeWarnings`。
2. preview `outputSchemas` を使った local missing-column check。
3. frontend local output schema fallback を使った local missing-column check。

初期実装では、backend warning が存在する場合は同じ node の local warning を表示しない。backend warning が未取得、loading 中、または API error の場合だけ local warning に戻す。

表示文言:

- `code=missing_upstream_columns` は既存 `dataPipelines.missingUpstreamColumns` を使う。
- `code=missing_right_upstream_columns` は既存 `dataPipelines.missingRightUpstreamColumns` を使う。
- unknown code は backend `message` を表示する。

### 8. Preview との役割分担を明確にする

validation endpoint:

- 実データを読まない。
- graph / config の整合性を見る。
- output schema と column reference warning を返す。
- editing UX と save / publish 前 validation に使う。

preview endpoint:

- 実データを読む。
- selected node までの preview rows を返す。
- run しないと分からない data quality / sample / runtime shape を確認する。
- `outputSchemas` は引き続き返すが、Inspector の primary source ではなく validation result の補助とする。

## Acceptance Criteria

実装完了条件:

- draft graph validation endpoint が追加され、OpenAPI / generated SDK に反映されている。
- endpoint は preview 実行なしで graph 全体の `outputSchemas` を返す。
- endpoint は missing-column warnings を node ごとに返す。
- `extract_text -> quality_report(columns=text,confidence)` で warning が出ない。
- `schema_mapping(includeSourceColumns=true) -> human_review -> output(orderBy=file_public_id)` で warning が出ない。
- `product_extraction -> confidence_gate(scoreColumns=product_confidence)` で warning が出ない。
- 実在しない列を設定した場合は backend warning が出る。
- frontend Inspector は backend warning を優先表示し、validation 未取得時は local fallback を使う。
- preview / run / smoke の既存挙動を壊さない。

## Test Plan

Backend unit tests:

- `inferOutputSchemas` の既存 test を維持する。
- `validateGraphForInspector` 相当の test を追加する。
- `extract_text -> quality_report` で `text`, `confidence` が missing にならない。
- `product_extraction -> confidence_gate` で `product_confidence` が missing にならない。
- 存在しない `quality_report.columns=["does_not_exist"]` は `missing_upstream_columns` になる。
- `join.leftKeys` / `join.rightKeys` を左右別に検証する。

API tests:

- draft graph validation endpoint が 200 を返す。
- invalid graph は 200 + `valid=false` を返す。
- unauthorized / tenant missing は既存 error response を返す。

Frontend checks:

- `npm --prefix frontend run build`
- validation result がある場合、Inspector local warning より backend warning が優先される。
- validation loading / error 時に local warning fallback が残る。

Smoke:

```bash
go test ./...
npm --prefix frontend run build
make smoke-data-pipeline-field-review
make smoke-data-pipeline-product-review
```

`go test ./...` は repo root ではなく `backend/` 配下で実行する。repo root は Go module ではない。

Generation:

```bash
GOCACHE=/private/tmp/haohao-go-build make gen
```

`sqlc` が PATH に存在することを前提にする。`sqlc` が見えない場合は生成前に環境を直す。

## Rollout Notes

- endpoint は browser API として追加する。external API には出さない。
- frontend は validation endpoint failure を non-blocking に扱う。
- save / publish の既存 validation を即座に置き換えない。まず Inspector warning と draft UX に使う。
- endpoint response は additive なので、既存 preview API の互換性には影響しない。
- warning code は将来増える前提で、frontend は unknown code を落とさず message 表示する。

## Future Plan

次の段階:

1. validation endpoint を save / publish 前の validation summary に統合する。
2. frontend local output schema fallback を縮小し、backend validation result を primary source にする。
3. `DataPipelineStepCatalog` を単なる step type list ではなく、output schema / config refs / UI metadata を持つ catalog に拡張する。
4. node 追加時の checklist に、backend output schema、backend config refs、validation warning test を必須化する。
5. warning を column existence 以外へ拡張する。
   - join key null
   - join row explosion risk
   - schema mapping required target missing
   - quality report threshold mismatch
   - confidence gate no score columns
6. validation result を Runs / Reviews / Schedules とは別に、design-time diagnostics として UI に表示する。

長期的には、UI、runtime、preview、validation、docs、smoke が同じ schema contract を参照する状態にする。frontend local inference は offline / loading fallback として最小限だけ残す。

## Implementation Order

推奨順:

1. backend service type と validation method を追加する。
2. backend configured column refs extractor を追加する。
3. missing-column warning resolver を追加する。
4. API endpoint / body type を追加する。
5. backend unit / API test を追加する。
6. `make gen` で OpenAPI / generated SDK を更新する。
7. frontend API wrapper / store state / debounce validation を追加する。
8. Inspector を backend warning 優先に切り替える。
9. docs と smoke を更新する。
10. build / test / smoke / browser check を実行する。

この順序なら、backend contract を先に固定し、frontend は生成済み型に合わせて実装できる。
