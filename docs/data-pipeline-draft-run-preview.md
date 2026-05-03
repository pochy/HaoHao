# Data Pipeline Draft Run Preview

## 概要

Draft Run Preview は、Drive OCR などの非構造化データ処理を含むデータパイプラインで、選択したノードまでを一時的に materialize してプレビューする仕組み。

通常の構造化データ向けプレビューは SQL の `WITH` と `SELECT ... LIMIT` で完結できる。一方、Drive OCR を含むパイプラインは OCR 実行、抽出結果の保存、後続ノード向けの中間テーブル作成が必要になるため、従来の lightweight preview では扱えなかった。

今回の変更では、Drive OCR パイプラインでもパイプライン Run や Work table を永続化せず、選択ノードまでの中間結果だけを一時テーブルとして作成し、結果返却後に削除する。

## 目的

- Drive OCR ノードを含むパイプラインでも、ブラウザ上で選択ノードの出力を確認できるようにする。
- 「Run を実行しないと結果が見られない」状態を避け、パイプライン設計中の試行錯誤を短くする。
- プレビュー用データを通常の Work table や本実行の中間テーブルと明確に区別する。
- プレビュー結果はアドホックな一時データとして扱い、永続化しない。

## 対象になるグラフ

フロントエンドでは、次のいずれかを含むグラフを Draft Run Preview 対象として判定する。

- `input` ノードで `sourceKind = drive_file`
- 手動プレビュー対象の処理ノード

Drive OCR 系の非構造化処理は、主に次のような流れを想定している。

```text
Drive file input
  -> extract_text
  -> classify_document
  -> extract_fields
  -> output
```

## UI 挙動

対象グラフでは、Preview パネルのボタン表示が `Draft Run Preview` に切り替わる。

Draft Run Preview 対象グラフでは、ノードを選択すると、その選択ノードまでの preview を debounce 付きで自動実行する。これにより、`input`、`extract_text`、`classify_document`、`extract_fields`、`confidence_gate` など、各ノードの出力を選択操作だけで確認できる。

Preview パネルの `Draft Run Preview` ボタンは残している。自動プレビューが失敗した場合、キャッシュがない場合、または明示的に再確認したい場合に手動実行できるようにするため。

通常の構造化データプレビューでは、既存の auto preview 制御を維持している。LLM、API、外部連携などの手動プレビュー対象ノードは、従来どおり自動実行しない。Draft Run Preview 対象グラフだけは、選択ノードごとの出力確認を優先して自動実行する。

Preview パネルには、選択ノードまで一時データを materialize し、結果返却後に削除する旨を表示する。OCR キャッシュは再利用または作成される場合がある。

同じ graph signature と同じ node ID の preview 結果は frontend store でキャッシュする。グラフ構造やノード設定が変わらない限り、同じノードを再選択しても不要な preview API 呼び出しは行わない。

## API

既存のプレビュー API を使う。

- 保存済み version のプレビュー: `POST /api/v1/data-pipeline-versions/{versionPublicId}/preview`
- 未保存 draft graph のプレビュー: `POST /api/v1/data-pipelines/{pipelinePublicId}/preview`

リクエストは従来どおり、選択ノード ID と件数上限を渡す。

```json
{
  "nodeId": "extract_fields",
  "limit": 100
}
```

draft graph のプレビューでは、リクエストボディに現在の graph も含める。これにより、保存前のノード追加、設定変更、接続変更をそのままプレビューできる。

## 認可と実行ユーザー

Drive OCR プレビューでは、OCR 実行に actor user が必要になる。

そのため API 層では `requireDataPipelineTenant` の戻り値から現在ユーザーを取得し、`DataPipelineService.Preview` / `PreviewDraft` に `actorUserID` を渡すようにした。

actor user が取得できない場合、Drive OCR プレビューは `ErrInvalidDataPipelineGraph` として失敗する。これは OCR 実行の監査、権限、キャッシュ作成理由を明確にするため。

## バックエンド処理

プレビュー実行時の分岐は `previewGraph` に集約している。

```text
previewGraph
  -> validateDataPipelineGraph
  -> dataPipelineGraphNeedsHybrid
      -> true:  previewHybridGraph
      -> false: compilePreviewSelect
```

構造化データのみのグラフは従来どおり `compilePreviewSelect` を使う。Drive OCR など hybrid materialize が必要なグラフだけ `previewHybridGraph` に進む。

## Hybrid Preview の流れ

`previewHybridGraph` は次の順で動く。

1. actor user と limit を検証する。
2. `runKey = "preview:" + uuid` を作る。
3. `executeHybridGraph` を選択ノード ID 付きで呼ぶ。
4. 選択ノードまでの各ノードを ClickHouse の一時テーブルへ materialize する。
5. 選択ノードの materialized relation に対して `SELECT * ... LIMIT n` を実行する。
6. 行とカラムを `DataPipelinePreview` として返す。
7. `defer` で preview 用一時テーブルを削除する。

本実行の `executeHybridRun` とは違い、Dataset Work table の作成、Run 成果物としての保存、最終 output table の永続化は行わない。

## 一時テーブルの命名

Draft Run Preview の一時テーブルは、通常の中間テーブルと区別できるように `__dp_preview_` prefix を使う。

```text
__dp_preview_<preview_id>_<node_cte_name>
```

例:

```text
__dp_preview_019decef123456789_extract_text
__dp_preview_019decef123456789_extract_fields
```

通常実行の中間テーブルは `__dp_node_...` 系の prefix を使う。preview 用テーブルは `__dp_preview_...` で検索できるため、運用時に残存確認しやすい。

## Session Temporary Table を使わなかった理由

当初の選択肢として、ClickHouse の session temporary table を使う案もあった。session temporary table は session が閉じれば自動で消えるため、preview の「永続化しない」という性質とは相性がよく見える。

ただし今回の実装では、通常の work database 上に `__dp_preview_...` prefix の通常テーブルを作り、処理後に明示的に `DROP TABLE IF EXISTS` する方式を採用した。

主な理由は次の通り。

### 既存の hybrid 実行基盤を流用しやすい

Drive OCR を含む hybrid pipeline は、各ノードの出力を `dataPipelineMaterializedRelation` として扱う。これは `Database`、`Table`、`Columns` を持ち、後続ノードが `database.table` として参照する前提になっている。

通常の work database 上に preview table を作る方式なら、本実行で使っている `executeHybridGraph`、`materializeHybridNode`、各 materialize 関数の構造をほぼそのまま使える。

session temporary table にすると、`database.table` 参照、DDL、後続 SELECT の扱いを別経路にする必要が出やすい。preview 専用分岐が増えるほど、本実行と preview の結果がズレるリスクも上がる。

### 複数ノードの materialize 依存を扱いやすい

Draft Run Preview は単一 SELECT ではなく、選択ノードまでのノードを順番に materialize する。

```text
input
  -> extract_text
  -> classify_document
  -> extract_fields
```

この場合、後続ノードは前段ノードの materialized result を読む。通常テーブルであれば、前段の出力を `hh_t_<tenant_id>_work.__dp_preview_...` として安定して参照できる。

session temporary table を使う場合、作成、参照、削除まで同一 ClickHouse session に閉じ込める必要がある。今の実装でも同一 connection 内で処理しているが、将来 materialize 処理が分割されたり、OCR や抽出処理の周辺で接続管理が変わった場合に制約が強くなる。

### 接続管理の制約を弱くできる

ClickHouse の session temporary table は session に紐づくため、同じ session で作成した temporary table を同じ session で読む必要がある。

通常テーブルであれば、materialize、preview SELECT、cleanup の間で同じ connection を維持する実装に依存しすぎない。実際には今回も同じ service flow の中で処理しているが、通常テーブルの方が `database.table` として参照できるため、既存の接続管理や helper 関数との相性がよい。

preview のためだけに ClickHouse session lifecycle を強く意識する実装にすると、将来のリファクタリングや並列化で壊れやすくなる。

### 障害時の調査と cleanup がしやすい

session temporary table は session 終了時に消えるため、正常系の cleanup は楽になる。一方で、問題が起きたときに `system.tables` から preview 用の残存テーブルを確認し、どの prefix のデータが残っているかを調べる運用はしづらい。

`__dp_preview_...` prefix の通常テーブルであれば、次のように残存確認できる。

```sql
SELECT database, name, total_rows, total_bytes
FROM system.tables
WHERE database = 'hh_t_<tenant_id>_work'
  AND name LIKE '__dp_preview_%';
```

障害や中断で cleanup が完走しなかった場合でも、prefix で対象を絞って手動削除や cleanup job の対象にできる。

### 本実行との差分を小さくできる

今回の目的は、Drive OCR pipeline を「本実行に近い処理」で選択ノードまで試せるようにすること。

そのため preview だけ別の table 種別、別の参照規則、別の materialize 実装にすると、preview では成功するが run では失敗する、またはその逆のような差分が生まれやすい。

通常テーブルを使うことで、本実行との差分を次の範囲に抑えている。

- table prefix が `__dp_preview_...`
- selected node までで処理を止める
- Dataset Work table を作らない
- Data Pipeline Run の成果物として保存しない
- preview rows を返した後に table を削除する
- OCR request の `Reason` が `data_pipeline_preview`

### 採用しなかった trade-off

session temporary table の最大の利点は、session 終了時に自動削除されること。この点では通常テーブルより安全に見える。

ただし今回の実装では、`defer` による `dropHybridTables`、materialize 途中のエラー時 cleanup、`__dp_preview_...` prefix による識別で、通常系の削除と障害時の調査を両立する方針にした。

今後、ClickHouse session lifecycle を明示的に管理する preview executor を作る場合や、hybrid materialize が完全に単一 connection 内で閉じる設計に整理できた場合は、session temporary table を再検討してもよい。

## 削除方針

Preview 用の materialized table は `previewHybridGraph` の `defer` で削除する。

削除処理は `dropHybridTables` に集約している。対象 tenant の work database を開き、`DROP TABLE IF EXISTS` で preview 実行中に作成されたテーブルを削除する。

選択ノードが存在しない場合や materialize 途中でエラーになった場合も、`executeHybridGraph` 側で作成済みテーブルを削除する。

ただし、プロセス異常終了や ClickHouse 接続断など、`defer` が完走できない障害では一時テーブルが残る可能性がある。その場合は `__dp_preview_%` の prefix で残存確認し、手動削除または将来の cleanup job で回収する。

## OCR キャッシュ

Draft Run Preview は preview 用の中間テーブルを永続化しないが、Drive OCR のキャッシュは既存仕様どおり再利用または作成される場合がある。

理由は、OCR 自体が高コスト処理であり、同じ Drive file に対する再プレビューや本実行で結果を再利用できる方が実用的だから。

Preview 由来の OCR request には `Reason = data_pipeline_preview` を渡す。通常のパイプライン実行では `Reason = data_pipeline` を使う。これにより、監査やログ上で preview 起点の OCR と本実行起点の OCR を区別できる。

## Limit

Preview limit は次の扱いにしている。

- `limit <= 0` の場合は `100`
- 最大値を超える場合も `100`
- 実際の上限は `datasetPreviewRowLimit`

プレビューは設計確認用であり、大量データの materialize 結果を全件返す用途ではない。

## フロントエンド実装

主な変更点は次の通り。

- `isDataPipelinePreviewSupported(graph)` は graph があれば true を返す。
- `isDataPipelineDraftRunPreviewGraph(graph)` を追加し、Drive file input や手動プレビュー対象ノードを含むか判定する。
- Draft Run Preview 対象グラフでは、手動プレビュー対象ノードでも `selectedAutoPreviewKey` を返す。これにより、ノード選択時に各ノードの Draft Run Preview が自動実行される。
- 通常の構造化グラフでは、従来どおり `isDataPipelineAutoPreviewEnabled` が false のノードは自動プレビューしない。
- `previewSelected` から Drive OCR プレビュー不可のブロックを削除し、手動プレビュー実行を許可する。
- `previewSelected({ automatic: true })` は、Draft Run Preview 対象グラフの場合に限り、`extract_text`、`classify_document`、`extract_fields` などの manual preview step でも実行する。
- preview cache は `nodeId` と `graphPreviewSignature` で管理し、同一グラフ・同一ノードの重複実行を避ける。
- Preview パネルへ `draftRunPreview` prop を追加し、ボタン文言と説明文を切り替える。

関連ファイル:

- `frontend/src/api/data-pipelines.ts`
- `frontend/src/stores/data-pipelines.ts`
- `frontend/src/views/DataPipelineDetailView.vue`
- `frontend/src/components/DataPipelinePreviewPanel.vue`
- `frontend/src/i18n/messages.ts`

## バックエンド実装

主な変更点は次の通り。

- `Preview` / `PreviewDraft` / `previewGraph` が `actorUserID` を受け取る。
- `dataPipelineGraphNeedsHybrid(graph)` が true の場合、エラーではなく `previewHybridGraph` を呼ぶ。
- `previewHybridGraph` を追加し、一時 materialize、SELECT、cleanup を行う。
- `executeHybridGraph` が選択ノード ID と OCR reason を受け取る。
- `materializeExtractText` が OCR request に `Reason` を渡せるようになった。
- `dataPipelineHybridTablePrefix` が `preview:` runKey を `__dp_preview_...` prefix に変換する。
- 選択ノードが見つからない場合、作成済み一時テーブルを削除してからエラーにする。

関連ファイル:

- `backend/internal/api/data_pipelines.go`
- `backend/internal/service/data_pipeline_service.go`
- `backend/internal/service/data_pipeline_unstructured.go`

## 従来エラーとの関係

以前は Drive OCR ノードを含むグラフで preview を実行すると、次のエラーを返していた。

```text
invalid data pipeline graph: preview for Drive OCR pipeline nodes is not supported in v1; run the pipeline to materialize results
```

この制限は、Drive OCR ノードが SQL だけではプレビューできず、中間テーブルの materialize を必要とするためだった。

今回の実装により、Drive OCR グラフは `previewHybridGraph` 経由で一時 materialize できるため、このエラーは通常発生しない。

## 確認方法

バックエンドの確認:

```bash
GOCACHE=/Users/pochy/Projects/HaoHao/.cache/go-build GOMODCACHE=/Users/pochy/Projects/HaoHao/.cache/go-mod go test ./backend/internal/service ./backend/internal/api
```

フロントエンドの確認:

```bash
npm --prefix frontend run build
```

ブラウザでの確認:

1. Drive file input を含むデータパイプラインを開く。
2. `extract_text`、`classify_document`、`extract_fields` などのノードを選択する。
3. Preview パネルのボタンが `Draft Run Preview` になっていることを確認する。
4. ボタンを押して、選択ノードの preview rows が返ることを確認する。
5. 通常の Run 一覧や Work table が増えていないことを確認する。

ClickHouse の一時テーブル残存確認:

```sql
SELECT count()
FROM system.tables
WHERE database = 'hh_t_<tenant_id>_work'
  AND name LIKE '__dp_preview_%';
```

正常終了後は `0` になる。

## 実装時に確認した結果

実装後、Drive OCR パイプラインの `extract_fields` ノードに対して Draft Run Preview を実行し、1 行のプレビュー結果が返ることを確認した。

返却された主なカラム:

- `file_public_id`
- `ocr_run_public_id`
- `page_number`
- `text`
- `confidence`
- `document_type`
- `vendor`
- `invoice_no`
- `document_date`
- `total_jpy`
- `fields_json`
- `evidence_json`
- `field_confidence`

実行後、ClickHouse の `__dp_preview_%` テーブルが残っていないことも確認した。

## 今後の改善候補

- プロセス異常終了時に残った `__dp_preview_%` テーブルを掃除する cleanup job を追加する。
- Preview 実行履歴を永続化せずに、直近の UI セッション内だけで表示できる lightweight log を追加する。
- Draft Run Preview の実行中に、どのノードを materialize しているかを UI に表示する。
- OCR キャッシュを新規作成した場合と再利用した場合を Preview パネルに表示する。
- Preview 用の timeout、最大ファイル数、最大ページ数を UI と API の両方で明示する。
- `__dp_preview_%` の残存数を運用メトリクスとして監視する。
