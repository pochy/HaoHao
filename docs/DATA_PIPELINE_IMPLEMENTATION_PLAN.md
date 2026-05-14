# Data Pipeline Implementation Plan

作成日: 2026-05-13

## Summary

この文書は、次に Data Pipeline を実装するための実装方針です。再調査対象は次の 3 ファイルです。

- `docs/NEXT_IMPLEMENTATION_PLAN.md`
- `docs/data-pipeline-llm-node.md`
- `docs/data-pipeline-current-state.md`

結論は、LLM node を先に増やすのではなく、まず **Data Pipeline の品質、可観測性、説明可能性**を実装することです。`data_pipeline_run_steps.metadata` と `DataPipelineRunStepBody.metadata` を使う Month 1 の metadata 基盤は完了済みです。次は Month 2 の `quarantine` node で、失敗行・低信頼行を通常 output から分離します。

最初の実装単位は次です。

1. run step metadata contract を固定する。
2. structured / hybrid executor が node id ごとの row count と metadata を返す。
3. `profile` と `validate` を passthrough から実行可能 node にする。
4. `quality_report` と `confidence_gate` の行データ結果を run step metadata に集約する。
5. Data Pipeline detail の Runs tab で step metadata を読めるようにする。

## 2026-05-14 更新

上記の最初の実装単位は `b9d5c03 Enhance data pipeline run metadata` で実装済みです。

完了した内容:

- structured / hybrid executor が node ごとの row count と metadata を保存する。
- `profile` が row count、column count、null count / rate、unique count、min / max、top values を run step metadata に保存する。
- `validate` が rule count、failed rows、error count、warning count、rule 別 failed rows を run step metadata に保存する。
- `quality_report` と `confidence_gate` の summary を hybrid run step metadata に集約する。
- Data Pipeline detail の Runs tab で run output と run step metadata summary を表示する。
- Work table 一覧 API が columns を返すようになり、Inspector の上流列警告の誤検知を解消した。

Drive file input を含む pipeline の smoke 自動化は `0160678 Add data pipeline drive smoke` で実装済みです。

```bash
make smoke-data-pipeline
```

この smoke は demo login、tenant 選択、Drive workspace 自動選択、inline JSON upload、`drive_file` input、`json_extract`、`profile`、`validate`、`output`、run 完了、metadata 検証までを一括で確認します。

Month 1 の残タスクは `c54de44 Complete data pipeline metadata smoke` で完了済みです。

追加で完了した内容:

- `inputRows`、`samples`、`queryStats` を run step metadata に追加する。
- `validate.required` で空文字と空白のみの値を失敗扱いにする。
- Runs tab で profile / validation / quality / confidenceGate / queryStats の詳細を確認できるようにする。
- smoke を `json`、`excel`、`text` scenario に分ける。
- CI では live smoke ではなく、まず smoke script の syntax check を行う。

現在の確認コマンド:

```bash
go test ./backend/...
npm --prefix frontend run build
node --check scripts/smoke-data-pipeline.mjs
make smoke-data-pipeline-suite
```

Month 2 の最初の実装である `quarantine` node v1 は 2026-05-14 に実装済みです。`validate` / `confidence_gate` の失敗行や低信頼行を通常 output から分離し、別 Work table として確認できる入口ができました。次は `confidence_gate` / `quality_report` の失敗理由 metadata 強化に進みます。

## 調査結果

### NEXT_IMPLEMENTATION_PLAN.md から確認した方針

HaoHao の次の 6 か月は、機能数を増やすよりも、Drive / Dataset から作った Pipeline を品質検査、隔離、レビュー、スケジュール、Gold publish まで安全に運用できる状態へ仕上げる方針です。

特に Month 1 は次を完了条件にしています。

- `validate` と `profile` を passthrough から実行可能 node にする。
- `data_pipeline_run_steps.metadata` に row count、欠損率、失敗件数、warning、sample を保存する。
- Run detail UI に step metadata、品質 summary、重い step、warning を表示する。
- structured / hybrid の両方で run step metadata が保存される。

Month 2 では `quality_report`、`confidence_gate`、`quarantine`、`human_review` を信頼できる失敗処理へつなげる計画です。したがって、Month 1 の metadata 基盤が先行しないと Month 2 の実装は安定しません。

### data-pipeline-current-state.md から確認した現状

Data Pipeline は graph JSON、DAG validation、preview、run、schedule、structured path、hybrid path、複数 output をすでに持っています。

PostgreSQL 側の主要 table は次です。

- `data_pipelines`
- `data_pipeline_versions`
- `data_pipeline_runs`
- `data_pipeline_run_steps`
- `data_pipeline_run_outputs`
- `data_pipeline_schedules`

metadata の保存先の分担は次の設計が正です。

- `data_pipeline_run_steps.metadata`: node 単位の実行結果 summary。
- `data_pipeline_run_outputs.metadata`: output node 単位の最終成果物 summary。
- `data_pipeline_versions.validation_summary`: run 前の graph validation summary。
- `data_pipeline_runs`: run 全体の lifecycle と代表 output。
- ClickHouse の行データ列: 後続 node が行単位で使う判定や注釈。

現在 backend catalog に存在する step type は多いですが、catalog にあることと、すべての実行経路で同じ深さまで実装されていることは別です。`profile` と `validate` は行や列を変更しない passthrough node のまま、run step metadata に実測 summary を保存する実装へ更新済みです。

推奨実装順は次です。

1. `validate` を実行可能化し、失敗行、warning、metadata を保存する。
2. `profile` を実行可能化し、row count、欠損率、型推定、前回差分を保存する。
3. `union` を追加する。
4. `quarantine` を追加する。
5. `route_by_condition` を追加する。
6. `partition_filter` / `watermark_filter` を追加する。
7. `snapshot_scd2` を追加する。

ただしこの文書では、最初の実装範囲を `metadata`、`profile`、`validate`、`quality_report`、`confidence_gate`、UI 表示に絞ります。

### data-pipeline-llm-node.md から確認した方針

LLM node の推奨順は次です。

1. `llm_extract_fields`
2. `llm_classify_document`
3. `llm_schema_mapping`
4. `llm_entity_resolution`
5. `llm_table_repair`
6. `llm_quality_explain`
7. `llm_review_assist`
8. `llm_summarize`

ただし、LLM node も単体で賢く見せるのではなく、confidence、evidence、metadata、review 連携を先に設計する方針です。

LLM node の共通 metadata は次を想定します。

- `model_policy`
- `model`
- `prompt_version`
- `input_rows`
- `completed_rows`
- `needs_review_rows`
- `failed_rows`
- `avg_confidence`
- `token_usage`
- `latency_ms`
- `provider_errors`
- `warnings`

この内容は将来の LLM node 実装に残します。今回の優先実装は、LLM node を受け止められる run step metadata と review / confidence の土台作りです。

## コード確認結果

### Backend

確認した主なファイル:

- `backend/internal/service/data_pipeline_graph.go`
- `backend/internal/service/data_pipeline_compile.go`
- `backend/internal/service/data_pipeline_unstructured.go`
- `backend/internal/service/data_pipeline_service.go`
- `backend/internal/db/data_pipelines.sql.go`
- `backend/internal/api/data_pipelines.go`

確認した事実:

- `dataPipelineStepCatalog` には `profile`、`validate`、`quality_report`、`confidence_gate` が存在します。
- `data_pipeline_compile.go` の `compileNode` では `DataPipelineStepProfile`、`DataPipelineStepValidate`、`DataPipelineStepOutput` は行データ上は `passThrough` です。run executor 側で `profile` / `validate` metadata を収集します。
- `CompleteDataPipelineRunStep` は `row_count` と `metadata` を更新できます。
- `HandleRunRequested` は node id ごとの row count / metadata を `CompleteDataPipelineRunStep` に渡します。
- `executeRun` は structured path、`executeHybridRun` は hybrid path を担当し、node id ごとの metadata を返します。
- `materializeQualityReport` は `quality_report_json`、`missing_rate_json`、`validation_summary_json` を ClickHouse 行データへ追加します。
- `materializeConfidenceGate` は `gate_score` と `gate_status` を ClickHouse 行データへ追加します。

### Frontend

確認した主なファイル:

- `frontend/src/views/DataPipelineDetailView.vue`
- `frontend/src/components/DataPipelinePreviewPanel.vue`
- `frontend/src/components/DataPipelineInspector.vue`
- `frontend/src/components/DataPipelineFlowBuilder.vue`
- `frontend/src/stores/data-pipelines.ts`
- `frontend/src/api/generated/types.gen.ts`

確認した事実:

- `DataPipelineRunStepBody` には `metadata: Record<string, unknown>` がすでにあります。
- `DataPipelineRunBody` には `steps` と `outputs` が含まれます。
- `DataPipelinePreviewPanel` の Runs tab は run、output、run steps、metadata summary、metadata detail dialog を表示します。
- `DataPipelineInspector` には `validate`、`confidence_gate`、`quality_report` の設定 UI が存在します。
- `DataPipelineFlowBuilder` の palette にも `profile`、`validate`、`confidence_gate`、`quality_report` が存在します。

## 実装方針

### 1. run step metadata contract を固定する

DB migration は追加しません。既存の `data_pipeline_run_steps.metadata` と `DataPipelineRunStepBody.metadata` を使います。

metadata は JSON object とし、v1 では次の stable key を使います。

| key | 内容 |
| --- | --- |
| `inputRows` | node 入力行数 |
| `outputRows` | node 出力行数 |
| `failedRows` | validation / gate / runtime 上の失敗行数 |
| `warningCount` | warning 件数 |
| `warnings` | UI に表示できる短い warning 配列 |
| `samples` | mask 済み sample。大量保存しない |
| `profile` | profile node の summary |
| `validation` | validate node の summary |
| `quality` | quality_report node の summary |
| `confidenceGate` | confidence_gate node の summary |
| `queryStats` | ClickHouse query id、elapsed、read rows など。v1 では取れる範囲でよい |

命名は frontend generated type に合わせ、JSON key は camelCase にします。既存 LLM node 文書の snake_case は、将来 provider / model 系 metadata を入れるときに backend 内部名として残してもよいですが、browser API で安定表示する key は camelCase に寄せます。

保存先の分担:

- run detail / monitoring 用 summary は `data_pipeline_run_steps.metadata`。
- 後続 node が行単位で読む値は ClickHouse の列。
- output table や Work table の情報は `data_pipeline_run_outputs.metadata`。
- raw prompt / raw response / PII sample は既定では保存しない。

### 2. executor の返り値を node metadata 対応にする

`dataPipelineRunOutputResult` だけでは node 単位 metadata が表現しづらいため、run 実行結果に node id ごとの summary を持たせる構造を追加します。

推奨する内部型:

```go
type dataPipelineRunNodeResult struct {
    NodeID   string
    StepType string
    RowCount int64
    Metadata map[string]any
}

type dataPipelineRunExecutionResult struct {
    Outputs []dataPipelineRunOutputResult
    Nodes   map[string]dataPipelineRunNodeResult
}
```

実装では、既存の `executeRun` / `executeHybridRun` の呼び出し側を大きく壊さないように、先に helper を追加して段階移行します。

`HandleRunRequested` の変更方針:

- run step 作成と processing mark は現状維持。
- `executeRun` / `executeHybridRun` から node result を受け取る。
- 成功時は各 node の `RowCount` と `Metadata` を `CompleteDataPipelineRunStep` に渡す。
- node result がない node は、後方互換 fallback として output total row count と最小 metadata を入れる。
- output 失敗時は現状どおり output を fail し、run step には失敗 summary を入れる。

### 3. structured path の metadata 収集

structured path は ClickHouse SQL CTE で実行されるため、node ごとの実データを取るには、対象 node までの compiled SQL を使って summary query を追加実行します。

v1 の方針:

- 全 node に対して最低限 `outputRows` を取る。
- `profile` node では target columns の profile query を実行する。
- `validate` node では configured rule を SQL expression に変換し、失敗件数と sample を取る。
- preview の挙動は変えない。run 後 metadata のみ追加する。

実装候補:

- `compileSelectWithOptions(ctx, tenantID, graph, node.ID, true)` を使い、node までの `SELECT` を取得する。
- その SQL を subquery として `count()` や column summary を実行する。
- 出力用 table 作成とは別 query になるため、v1 では correctness を優先し、過度な最適化はしない。

注意点:

- `output` node は passthrough のままでもよいが、metadata には `outputRows` と output table 情報を入れる。
- `join` / `enrich_join` の warning は v1 では必須にしない。ただし後続で未マッチ件数、row count 増減、key null、列衝突を入れる余地を残す。
- query cost が大きくなりすぎる場合に備え、profile 対象 column 数と top values 件数は上限を設ける。

### 4. hybrid path の metadata 収集

hybrid path は node ごとに ClickHouse 中間 table を materialize するため、structured path より node metadata を取りやすいです。

v1 の方針:

- `executeHybridGraph` が relation だけでなく node result map を返す。
- `materializeHybridNode` 実行後に、作成済み table に対して `count()` を取り `outputRows` とする。
- node 固有 summary は materializer ごとに取れる範囲で追加する。

node 固有 summary:

- `quality_report`: `quality_report` 関数の結果を `metadata.quality` に入れる。
- `confidence_gate`: threshold、scoreColumns、pass / needs_review 件数、最低 score、平均 scoreを `metadata.confidenceGate` に入れる。
- `human_review`: review 対象件数、queue 名、理由 column を入れる。
- `schema_inference`: field count、confidence、推定 column summary を入れる。

v1 では ClickHouse 行データ列の互換性を維持します。既存の `quality_report_json`、`missing_rate_json`、`validation_summary_json`、`gate_score`、`gate_status` は削除しません。

### 5. profile node を実行可能化する

`profile` は行や列を変更しません。出力 relation は passthrough のままにし、run step metadata に summary を保存します。

v1 config:

- `columns`: 対象 columns。未指定なら上限つきで全 column。
- `topValuesLimit`: default 10、max 20。
- `sampleLimit`: default 5、max 20。

v1 metadata:

```json
{
  "profile": {
    "rowCount": 1234,
    "columnCount": 12,
    "columns": [
      {
        "name": "amount",
        "nullCount": 10,
        "nullRate": 0.0081,
        "uniqueCount": 532,
        "min": "0",
        "max": "9999",
        "topValues": [
          { "value": "100", "count": 32 }
        ]
      }
    ]
  }
}
```

数値 / 日付の型判定は ClickHouse の値を string 化して返してよいです。v1 では型推定の完全性より、UI とテストで安定して読める summary を優先します。

### 6. validate node を実行可能化する

`validate` も v1 では行や列を変更しません。run step metadata に validation summary を保存します。行を止める、run を fail させる、quarantine へ分岐する処理は次フェーズに回します。

v1 rule:

- `required`
- `regex`
- `range`
- `in`
- `unique`

想定 config:

```json
{
  "rules": [
    {
      "column": "email",
      "operator": "required",
      "severity": "error"
    },
    {
      "column": "amount",
      "operator": "range",
      "min": 0,
      "max": 100000,
      "severity": "warning"
    }
  ]
}
```

v1 metadata:

```json
{
  "validation": {
    "ruleCount": 2,
    "failedRows": 7,
    "errorCount": 5,
    "warningCount": 2,
    "rules": [
      {
        "column": "email",
        "operator": "required",
        "severity": "error",
        "failedRows": 5
      }
    ],
    "samples": [
      {
        "rowNumber": 12,
        "column": "email",
        "reason": "required"
      }
    ]
  }
}
```

`unique` は対象 column の duplicate count を返します。複合 key は v1 では対象外でもよいですが、実装する場合は `columns` 配列で明示します。

### 7. quality_report と confidence_gate を metadata に接続する

`quality_report`:

- 現在の行データ列追加は維持する。
- `qualityReport(rows, targetColumns)` の結果を `metadata.quality` に入れる。
- `rowCount`、`columnCount`、`missingRate`、`warningCount` を top-level stable key にも反映する。

`confidence_gate`:

- 現在の `gate_score`、`gate_status` は維持する。
- metadata には `threshold`、`scoreColumns`、`passRows`、`needsReviewRows`、`failedRows`、`minScore`、`avgScore` を入れる。
- score column 欠損、数値変換失敗、threshold 未指定は warning にする。

この段階では `quarantine` node は追加しません。`confidence_gate` の結果を metadata と行データの両方で安定させた後に、低信頼行を別 output に分ける実装へ進みます。

### 8. Run detail UI を追加する

`DataPipelinePreviewPanel` の Runs tab を拡張し、run の下に outputs だけでなく steps を表示します。

UI v1:

- run 行: status、trigger、rows、created、error。
- output 行: output node、status、rows、work table id、error。
- step 行: step type、node id、status、row count、warning count、error。
- step detail: metadata の主要 summary を compact に表示する。

表示する metadata:

- `warnings`
- `profile.rowCount` / `profile.columnCount`
- `validation.failedRows` / `validation.errorCount` / `validation.warningCount`
- `quality` の row count / missing rate summary
- `confidenceGate.threshold` / `passRows` / `needsReviewRows`

大量 JSON をそのまま table に出さないでください。必要な場合は、既存 preview cell dialog と同じように detail dialog で JSON 表示します。

## 6か月実装順

### Month 1: 品質・可観測性の土台

- run step metadata contract を実装する。
- structured / hybrid executor から node result を返す。
- `profile` と `validate` を metadata-producing node にする。
- Runs tab に step metadata summary を表示する。

状態: 完了済み。

完了条件:

- structured / hybrid の両方で `data_pipeline_run_steps.metadata` が空ではなくなる。
- `profile` が row count、null count、unique count、min/max、top values を返す。
- `validate` が required、regex、range、in、unique の結果を返す。
- frontend で run step ごとの warning / validation / profile を確認できる。

### Month 2: 失敗処理と人手確認

詳細計画: `docs/DATA_PIPELINE_MONTH2_RELIABLE_FAILURE_HANDLING_PLAN.md`

- `quality_report` と `confidence_gate` の metadata 保存を強化する。
- `human_review` を review item / queue の入口へ拡張する。
- `quarantine` node を追加し、低品質行や低信頼行を通常 output と分離する。

完了条件:

- low confidence / validation error を run detail で説明できる。
- quarantine output を Work table として確認できる。
- review 対象行の tenant boundary、audit、CSRF 方針が決まっている。

### Month 3: 分岐と増分処理

- `union` を追加する。
- `route_by_condition` を追加する。
- `partition_filter` / `watermark_filter` を追加する。

完了条件:

- 複数 source の縦結合ができる。
- 文書種別や品質状態で branch を分けられる。
- schedule run が処理対象範囲を絞れる。

### Month 4: LLM node の v1

- `llm_extract_fields` を最初に追加する。
- tenant policy、model policy、prompt version、confidence、evidence、metadata を必須にする。
- 自動採用ではなく、`confidence_gate` と `human_review` に渡せる結果を出す。

完了条件:

- OCR / text extraction の後に `llm_extract_fields` を置ける。
- model / prompt / token / latency / provider error が metadata に残る。
- 低信頼結果を review に回せる。

### Month 5: Gold / Lineage 連携

- Pipeline output から Gold publish までの品質 summary を接続する。
- lineage に pipeline run、output node、source kind、confidence を表示する。
- publish 失敗時に既存 Gold を壊さない運用を維持する。

完了条件:

- Gold detail で source、schema、row count、publish history、quality summary が読める。
- lineage parser / manual / metadata の source kind が混同されない。

### Month 6: リリース品質

- Playwright E2E を Pipeline happy path に拡張する。
- `DataPipelineDetailView`、Monaco、Vue Flow 周辺の巨大 chunk 警告を lazy load で改善する。
- local runtime、ClickHouse、pgvector、OpenFGA、SeaweedFS の runbook と smoke を整える。

完了条件:

- `go test ./backend/...` と `npm --prefix frontend run build` が通る。
- 主要 Pipeline 導線の E2E がある。
- runbook から原因調査、rollback、DR drill へ進める。

## Public Interfaces

新規 public endpoint は追加しません。

既存 API の `DataPipelineRunStepBody.metadata` を使い、次の key を browser API として安定扱いします。

- `inputRows`
- `outputRows`
- `failedRows`
- `warningCount`
- `warnings`
- `samples`
- `profile`
- `validation`
- `quality`
- `confidenceGate`
- `queryStats`

OpenAPI / frontend generated client は、型が `Record<string, unknown>` のままであれば更新不要です。もし説明や schema example を追加する場合は、Huma type comment / API body definition を更新し、`make gen` で generated files を更新します。

DB schema は変更しません。

## Test Plan

### Backend

- `go test ./backend/...`
- `backend/internal/service/data_pipeline_graph_test.go`
  - `profile` / `validate` の graph validation が既存制約を壊さないこと。
- `backend/internal/service/data_pipeline_unstructured_test.go`
  - `quality_report` metadata が保存用 summary と行データ列の両方に反映されること。
  - `confidence_gate` metadata が threshold、scoreColumns、pass / needs_review 件数を返すこと。
- structured compiler tests
  - `profile` が passthrough relation を維持しつつ metadata query を作れること。
  - `validate` の required / regex / range / in / unique が失敗件数を返すこと。
- service tests
  - `HandleRunRequested` が各 step に node 固有 row count / metadata を渡すこと。

### Frontend

- `npm --prefix frontend run build`
- Runs tab で steps が表示されること。
- metadata が空でも UI が壊れないこと。
- profile / validation / quality / confidenceGate の一部 key が欠けても fallback 表示になること。
- 長い JSON は table を壊さず detail dialog で見られること。

### Integration / Smoke

- structured pipeline: input -> profile -> validate -> output。
- hybrid pipeline: drive_file -> extract_text -> quality_report -> confidence_gate -> output。
- multiple output pipeline: node metadata と output metadata が混同されないこと。
- failed output: run / output / step の error summary が確認できること。

## 実装時の注意

- `.tool-versions` の削除差分は既存ユーザー変更として扱い、触らない。
- `backend/internal/db/*`、`frontend/src/api/generated/*`、`openapi/*.yaml` は generator 経由で更新する。
- destructive action は confirm dialog、audit、tenant boundary を必須にする。
- ClickHouse 行データに個人情報 sample を広げすぎない。metadata sample は mask 済み、件数上限つきにする。
- LLM node では raw prompt / raw response を既定保存しない。
- まずは運用品質を上げる。新規 node 数を増やすのは metadata / validation / review の後にする。

## 次の最小実装タスク

次の PR は Month 2 の最初の実装として、`quarantine` node に絞ります。

状態: 完了済み。

目的:

- `validate` の failed row と `confidence_gate` の `needs_review` row を通常 output から分離する。
- quarantine 側も Work table として登録し、run outputs / Runs tab から確認できるようにする。
- まずは hybrid path の `confidence_gate.gate_status = needs_review` を対象にする。

最小実装範囲:

1. backend catalog に `quarantine` step type を追加する。
2. hybrid executor に `quarantine` materializer を追加する。
3. config は v1 では `mode`, `statusColumn`, `matchValues`, `outputMode` に絞る。
4. `outputMode = quarantine_only` は一致行だけを出力する。
5. `outputMode = pass_only` は非一致行だけを出力する。
6. run step metadata に `quarantinedRows`, `passedRows`, `statusColumn`, `matchValues` を保存する。
7. Inspector / palette に最小 UI を追加する。
8. smoke に `confidence_gate -> quarantine -> output` scenario を追加する。
9. `go test ./backend/...`、`npm --prefix frontend run build`、`make smoke-data-pipeline-suite` を通す。

この PR では review item / queue は作りません。`quarantine` output を Work table として確認できるところまでを完了条件にします。

実装済み:

- backend catalog に `quarantine` step type を追加した。
- hybrid executor に `quarantine` materializer を追加した。
- config v1 は `mode`, `statusColumn`, `matchValues`, `outputMode` に絞った。
- `outputMode=quarantine_only` / `pass_only` に対応した。
- run step metadata に `quarantinedRows`, `passedRows`, `statusColumn`, `matchValues`, `outputMode` を保存する。
- Inspector / palette に最小 UI を追加した。
- smoke suite に quarantine scenario を追加した。
