# Data Pipeline Session Handoff

作成日: 2026-05-16

## 目的

この文書は、2026-05-14 から 2026-05-16 にかけて進めた Data Pipeline 実装セッションの引き継ぎメモです。別セッションで再開するときに、これまで何を実装し、何を検証し、次に何をすべきかを短時間で把握できることを目的にします。

詳細な設計背景は次を参照します。

- 全体計画: `docs/NEXT_IMPLEMENTATION_PLAN.md`
- Data Pipeline 実装計画: `docs/DATA_PIPELINE_IMPLEMENTATION_PLAN.md`
- 現状メモ: `docs/data-pipeline-current-state.md`
- Month 2 詳細: `docs/DATA_PIPELINE_MONTH2_RELIABLE_FAILURE_HANDLING_PLAN.md`
- Inspector 列推論問題: `docs/DATA_PIPELINE_UI_COLUMN_INFERENCE.md`
- validation endpoint: `docs/DATA_PIPELINE_VALIDATION_ENDPOINT_PLAN.md`

## 現在の結論

Month 1 の品質 / 可観測性、Month 2 の信頼できる失敗処理、Month 3 の主要 runtime node は、v1 として実装済みです。

現在の次タスクは、機能をさらに横に増やすことではなく、運用 UI と説明性を強くすることです。優先候補は次です。

1. SCD2 / snapshot table の運用 UI。
2. Gold publish 完了後の lineage / quality summary 表示。
3. SCD2 削除検知と同一 `valid_from` 衝突時の policy。
4. backend step catalog / generated contract への output schema 単一正本化。

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

残課題:

- backend step catalog / generated contract へ output schema をさらに集約する。
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
- `rebuild_key_history` は対象 key 全体を再計算するため late row を扱えるが、削除検知や同一 `valid_from` の複数変更に対する業務 policy はまだない。

## Gold Publish 導線

既存の Gold publish API は Work table 起点です。Data Pipeline output は run 成功時に managed Work table として登録されるため、Data Pipeline 側では次を追加しました。

- run output metadata に `workTablePublicId`、database、table name、display name、write mode、SCD2 merge policy を保存する。
- Runs tab の output 行に `hh_t_*_work.table` と write mode を表示する。
- output 行の `Publish output to Gold` ボタンから既存 Gold publication API を呼ぶ。
- 成功後は Gold detail へ遷移する。

確認済み:

- `make smoke-data-pipeline-snapshot-merge` で metadata に `workTablePublicId` と `scd2MergePolicy=current_only` が入る。
- `make smoke-data-pipeline-snapshot-merge-backfill` で metadata に `scd2MergePolicy=rebuild_key_history` が入る。
- agent-browser で Runs tab の `Publish output to Gold` ボタンを押し、Gold detail へ遷移することを確認した。

残課題:

- Gold publish 完了後に Data Pipeline run / output へ戻る明確な導線。
- Gold detail に Data Pipeline source、run id、quality summary、SCD2 policy を表示する。
- Gold publish history と Data Pipeline run history の相互リンク。

## SCD2 / Snapshot Work Table UI

Data Pipeline の `snapshot_scd2` と output `writeMode=scd2_merge` により、ClickHouse 上の Work table には SCD Type 2 の履歴行が保存されます。これまでは Work table UI が列一覧と preview rows をそのまま表示するだけだったため、利用者は `is_current`、`valid_from`、`valid_to`、`change_hash` を目視で探して current / history の状態を判断する必要がありました。

2026-05-16 の追加対応で、`frontend/src/components/DatasetWorkTableBrowser.vue` は選択中 Work table の列に `valid_from`、`valid_to`、`is_current`、`change_hash` が揃っている場合に SCD2 snapshot table として検出します。検出時は Overview の preview セクションに次を表示します。

- SCD2 snapshot 検出バッジ。
- 現在読み込まれている preview rows の件数。
- preview 内の current rows 件数。
- preview 内の history rows 件数。
- 代表 key column 候補。現時点では `id`、`product_id`、`sku`、`file_public_id` の順で存在する列を表示する。
- preview rows に対する `All` / `Current` / `History` フィルタ。

この UI は backend API contract を増やさず、既存の Work table preview payload だけを使う最小実装です。したがって件数とフィルタは **テーブル全体ではなく、現在読み込まれている preview rows に対する値**です。大規模 table の全体 current/history 件数や key 単位履歴は、別途 backend query endpoint を追加する必要があります。

今後の拡張候補:

- Work table preview API に SCD2 summary mode を追加し、全体 row count、current/history row count、key count、latest valid_from を返す。
- key column を output metadata や lineage から取得し、候補推定ではなく正確に表示する。
- key value を指定して履歴 rows を時系列で表示する。
- Gold detail 側にも SCD2 policy、current row count、history row count、source pipeline run へのリンクを表示する。

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

最優先候補は `snapshot table の運用 UI` です。

実装案:

- Data Pipeline Runs tab または Work table detail で SCD2 table と判定できる場合、current rows / history rows / key history を切り替えて見られるようにする。
- `uniqueKeys`、`valid_from`、`valid_to`、`is_current`、`change_hash`、`scd2MergePolicy` を summary として表示する。
- key を指定して履歴を時系列表示する。
- `is_current=1` が key ごとに 1 件か、`valid_to` が連続しているかを validation / quality summary として表示する。

次点候補:

- Gold detail に Data Pipeline source / run / quality summary / SCD2 policy を表示する。
- SCD2 削除検知 policy を設計する。
- `validate` の行単位 status column と quarantine 連携を設計する。
- backend step catalog / generated contract への output schema 単一正本化を進める。
