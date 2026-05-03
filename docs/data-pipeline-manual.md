# データパイプライン利用マニュアル

この文書は、HaoHao の `/data-pipelines` 画面でデータクレンジング / 前処理パイプラインを作成、Preview、実行、定期実行するための利用者向けマニュアルです。

## 対象者

- active tenant 上の Dataset または managed Work table を入力にして、クレンジング済みの managed Work table を作りたいユーザー。
- データ加工の流れを DAG として可視化し、処理定義を version 管理したいユーザー。
- 手動実行または定期実行の結果を Runs / medallion catalog で追跡したいユーザー。

## 前提条件

- active tenant が選択されていること。
- tenant role `data_pipeline_user` が付与されていること。
- 入力として使う Dataset または managed Work table が存在すること。
- Preview / Run には ClickHouse の tenant sandbox が利用できること。

権限が不足している場合、画面には `Data pipeline role required` が表示されます。tenant admin に `data_pipeline_user` role の付与を依頼してください。

## 画面構成

`/data-pipelines` は次の領域で構成されています。

| 領域 | 用途 |
| --- | --- |
| 左サイドバー | 新規 pipeline 作成、既存 pipeline 選択、node palette、利用可能な Dataset / Work table の確認 |
| 中央 canvas | Vue Flow による DAG builder。node の追加、配置、接続を行う |
| 右 Inspector | 選択 node の label と config を編集する |
| 下部 Preview / Runs / Schedule panel | Preview、手動 Run、Run 履歴更新、定期実行 schedule の追加 / 無効化を行う |
| 画面上部 action | pipeline 名 / 説明の編集、Save、Publish、Refresh |

## 基本ワークフロー

1. active tenant を選択します。
2. `/data-pipelines` を開きます。
3. 左サイドバーの `New pipeline` で名前と説明を入力し、`Create` を押します。
4. 初期 graph には `Input -> Output` が作成されます。
5. `Input` node を選択し、右 Inspector で source を設定します。
6. 左サイドバーの `Palette` から必要な処理 node を追加します。
7. canvas 上で node を接続し、処理順を作ります。
8. 各 node を選択して Inspector の `Config UI` または `Config JSON` を編集します。
9. 下部 `Preview` で選択 node までの結果を確認します。
10. 必要に応じて画面上部の `Save` / `Publish` を押します。
11. 下部 `Run` で手動実行します。
12. 定期実行したい場合は `Schedule` で頻度と時刻を設定して `Add` を押します。

## Pipeline の作成と選択

### 新規作成

左サイドバーの `New pipeline` で次を入力します。

- `Name`: pipeline の表示名。
- `Description`: 任意の説明。

`Create` を押すと、左の `Pipelines` 一覧に追加され、中央 canvas に初期 graph が表示されます。

### 既存 pipeline の選択

左サイドバーの `Pipelines` から対象 pipeline を選択します。選択すると、その pipeline の最新 version graph、run 履歴、schedule が読み込まれます。

### 名前と説明の変更

中央上部の `Name` / `Description` を編集し、`Save` を押します。`Save` は pipeline metadata と現在の graph version を保存します。

## DAG Builder の使い方

### Node の追加

左サイドバーの `Palette` から node を押すと、canvas に追加されます。

v1 の node catalog:

- `Input`
- `Profile`
- `Clean`
- `Normalize`
- `Validate`
- `Schema Mapping`
- `Schema Completion`
- `Enrich Join`
- `Transform`
- `Output`

### Node の接続

canvas 上の node 右側 handle から、次の node 左側 handle へ接続します。

基本形:

```text
Input -> Clean -> Normalize -> Transform -> Output
```

Join を含む例:

```text
Input -> Clean -> Enrich Join -> Schema Mapping -> Output
```

### Graph の制約

保存、Preview、Run 時に graph validation が行われます。

- `Input` は必ず 1 つ。
- `Output` は 1 つ以上。
- cycle は不可。
- node は最大 50。
- edge は最大 80。
- Input から到達できない node は不可。
- Output へ到達しない executable node は不可。
- edge は存在する node 同士を接続する必要があります。

## Inspector の使い方

右 Inspector では、選択 node の設定を編集します。

### Label

node の表示名です。canvas 上の node label に反映されます。

### Config UI

node type ごとのフォームです。source select、operation select、column list、mapping、condition などを UI で編集できます。

### Config JSON

同じ設定を JSON として直接編集できます。

Config UI と Config JSON は同期します。

- UI で変更すると JSON が更新されます。
- valid な JSON を入力すると UI に反映されます。
- invalid JSON の場合は error が表示され、graph には反映されません。

## Node 別設定

### Input

入力 source を指定します。

| 項目 | 説明 |
| --- | --- |
| `sourceKind` | `dataset` または `work_table` |
| `datasetPublicId` | Dataset を使う場合の public ID |
| `workTablePublicId` | managed Work table を使う場合の public ID |

UI では `Source kind` と `Dataset` / `Work table` select で選択できます。

JSON 例:

```json
{
  "sourceKind": "dataset",
  "datasetPublicId": "00000000-0000-0000-0000-000000000000"
}
```

### Profile

データを変更しない確認用 node です。v1 では passthrough として扱われます。

### Clean

欠損、重複、外れ値、不正値を処理します。

主な operation:

| operation | 用途 |
| --- | --- |
| `drop_null_rows` | 指定列が NULL の行を除外 |
| `fill_null` | NULL を指定値で補完 |
| `null_if` | 条件に一致する値を NULL に変換 |
| `clamp` | 数値を min / max の範囲に丸める |
| `trim_control_chars` | 制御文字を除去 |
| `dedupe` | key による重複除去 |

JSON 例:

```json
{
  "rules": [
    {
      "operation": "drop_null_rows",
      "columns": ["customer_id", "email"]
    },
    {
      "operation": "dedupe",
      "keys": ["customer_id"],
      "orderBy": "updated_at"
    }
  ]
}
```

### Normalize

表記、型、スケールを揃えます。

主な operation:

| operation | 用途 |
| --- | --- |
| `trim` | 前後空白を除去 |
| `lowercase` | 小文字化 |
| `uppercase` | 大文字化 |
| `normalize_spaces` | 連続空白を 1 つに正規化 |
| `remove_symbols` | 記号を除去 |
| `cast_decimal` | Decimal へ変換 |
| `round` | 数値丸め |
| `scale` | 数値に係数を掛ける |
| `parse_date` | 日時文字列を datetime に変換 |
| `to_date` | 日付へ変換 |
| `map_values` | カテゴリ値を mapping |

JSON 例:

```json
{
  "rules": [
    {
      "operation": "lowercase",
      "column": "email"
    },
    {
      "operation": "map_values",
      "column": "gender",
      "values": {
        "M": "male",
        "F": "female"
      }
    }
  ]
}
```

### Validate

品質チェック用の node です。v1 の SQL compiler では output を変更しない passthrough として扱われます。将来の品質 rule / error sample 拡張に向けて config を保存できます。

UI では column、operator、value を設定できます。

### Schema Mapping

入力列を target schema に対応付け、target column だけを出力します。

| 項目 | 説明 |
| --- | --- |
| `sourceColumn` | 元の列名 |
| `targetColumn` | 出力列名 |
| `cast` | `string`, `int64`, `float64`, `decimal`, `date`, `datetime` など |
| `defaultValue` | source がない場合の既定値 |
| `required` | true の場合、source/default がない mapping は保存 / 実行時 error |

JSON 例:

```json
{
  "mappings": [
    {
      "sourceColumn": "customer_id",
      "targetColumn": "id",
      "cast": "string",
      "required": true
    },
    {
      "sourceColumn": "amount",
      "targetColumn": "total_amount",
      "cast": "decimal"
    }
  ]
}
```

### Schema Completion

足りない列を固定値、他列コピー、coalesce、concat で補完します。

主な method:

- `literal`
- `copy_column`
- `coalesce`
- `concat`
- `case_when` は v1 では `defaultValue` を使う簡易扱いです。

JSON 例:

```json
{
  "rules": [
    {
      "targetColumn": "source_system",
      "method": "literal",
      "value": "haohao"
    },
    {
      "targetColumn": "display_name",
      "method": "concat",
      "sourceColumns": ["first_name", "last_name"]
    }
  ]
}
```

### Enrich Join

右側 source と left join し、追加列を取り込みます。

| 項目 | 説明 |
| --- | --- |
| `rightSourceKind` | `dataset` または `work_table` |
| `rightDatasetPublicId` | 右側 Dataset public ID |
| `rightWorkTablePublicId` | 右側 Work table public ID |
| `joinType` | v1 は `left` のみ |
| `leftKeys` | 左側 key 列 |
| `rightKeys` | 右側 key 列 |
| `selectColumns` | 右側から追加する列 |

JSON 例:

```json
{
  "rightSourceKind": "work_table",
  "rightWorkTablePublicId": "00000000-0000-0000-0000-000000000000",
  "joinType": "left",
  "leftKeys": ["customer_id"],
  "rightKeys": ["id"],
  "selectColumns": ["segment", "region"]
}
```

### Transform

列選択、列削除、rename、filter、sort、aggregate を行います。

#### select_columns

```json
{
  "operation": "select_columns",
  "columns": ["customer_id", "email", "segment"]
}
```

#### drop_columns

```json
{
  "operation": "drop_columns",
  "columns": ["raw_payload"]
}
```

#### rename_columns

```json
{
  "operation": "rename_columns",
  "renames": {
    "customer_id": "id",
    "email_address": "email"
  }
}
```

#### filter

```json
{
  "operation": "filter",
  "conditions": [
    {
      "column": "amount",
      "operator": ">",
      "value": 0
    }
  ]
}
```

#### sort

```json
{
  "operation": "sort",
  "sorts": [
    {
      "column": "updated_at",
      "direction": "DESC"
    }
  ]
}
```

#### aggregate

```json
{
  "operation": "aggregate",
  "groupBy": ["segment"],
  "aggregations": [
    {
      "function": "sum",
      "column": "amount",
      "alias": "total_amount"
    },
    {
      "function": "count",
      "alias": "row_count"
    }
  ]
}
```

### Output

最終結果を managed Work table として登録します。

| 項目 | 説明 |
| --- | --- |
| `displayName` | Work table の表示名 |
| `tableName` | 任意。指定しない場合は run public ID から自動生成 |
| `writeMode` | v1 は `replace` |
| `engine` | v1 は `MergeTree` |

JSON 例:

```json
{
  "displayName": "Customer cleansing output",
  "tableName": "customer_cleansed",
  "writeMode": "replace",
  "engine": "MergeTree"
}
```

`tableName` は ClickHouse identifier として安全な名前だけ利用できます。英字または `_` で始め、英数字と `_` を使ってください。

## Save と Publish

### Save

画面上部の `Save` は現在の graph を新しい version として保存します。

保存時に graph validation が行われます。validation error がある場合は画面に error が表示されます。

### Publish

画面上部の `Publish` は最新 version を published version にします。

published version は次の用途で使われます。

- 手動 Run の対象 version。
- Schedule の対象 version。
- Run / schedule 履歴の追跡。

現在の UI では、`Preview` は必要に応じて draft を自動保存します。`Run` は必要に応じて draft を自動保存し、published でなければ publish してから実行要求を作成します。明示的に version を固定したい場合は、先に `Save` と `Publish` を実行してください。

## Preview

下部 `Preview` の `Preview` ボタンは、選択中の node までの処理結果を最大 100 行で確認します。

使い方:

1. canvas で確認したい node を選択します。
2. Inspector の config を編集します。
3. `Preview` を押します。
4. Preview table に結果が表示されます。

注意:

- Preview は選択 node までの SQL を read-only `SELECT ... LIMIT 100` として実行します。
- Preview 対象 node を選択していない場合は実行できません。
- config JSON が invalid な場合、graph に反映されず、Preview も期待通りの内容になりません。
- 入力 source が未設定の場合、backend で source 解決 error になります。

## 手動 Run

下部 `Runs` の `Run` ボタンで手動実行します。

Run の流れ:

1. 現在の draft graph が保存されます。
2. 必要に応じて最新 version が publish されます。
3. `data_pipeline.run_requested` outbox event が作成されます。
4. outbox handler が ClickHouse で SQL を実行します。
5. 結果は tenant work database の managed Work table として登録されます。
6. Runs table に status、trigger、row count、created time、error が表示されます。

Run status:

| status | 説明 |
| --- | --- |
| `pending` | 実行要求作成済み |
| `processing` | outbox handler が処理中 |
| `completed` | 出力 Work table 登録完了 |
| `failed` | 実行失敗。Error 列を確認 |

`Refresh` を押すと Run 履歴を再取得します。active run がある場合は画面側でも定期的に自動更新します。

## Schedule

下部 `Schedule` で定期実行を設定します。

設定項目:

| 項目 | 説明 |
| --- | --- |
| `Frequency` | `Daily`, `Weekly`, `Monthly` |
| `Timezone` | 例: `Asia/Tokyo` |
| `Run time` | `HH:MM` 形式。例: `03:00` |
| `Weekday` | weekly の場合に使用。1 から 7 |
| `Month day` | monthly の場合に使用。1 から 28 |

`Add` を押すと、現在の published version に対して schedule が作成されます。

注意:

- Schedule は published version に紐付きます。未 publish の場合は、先に `Publish` または `Run` を実行してください。
- schedule worker は due schedule を claim し、前回 run がまだ `pending` / `processing` の場合は新しい run を skip します。
- schedule を止めるには schedule 一覧の trash icon を押します。削除ではなく disabled になります。

## 出力 Work table

Run が成功すると、Output node の config に基づいて managed Work table が作成または置き換えられます。

- `displayName` が Work table の表示名になります。
- `tableName` が指定されていればその table 名を使います。
- `tableName` が未指定の場合は run public ID から `dp_...` 形式で自動生成されます。
- 出力は Dataset / Work table 画面、lineage、medallion catalog から追跡できます。

## よくあるエラーと対処

### `Data pipeline role required`

`data_pipeline_user` role がありません。tenant admin に role 付与を依頼してください。

### `sourceKind must be dataset or work_table`

Input または Enrich Join の source kind が空、または不正です。Inspector で Dataset / Work table を選択してください。

### `unknown column ...`

config に指定した列が upstream columns に存在しません。入力 source の列名、または前段 node の Schema Mapping / Transform による列名変更を確認してください。

### `unsafe identifier ...`

出力 table 名、target column、alias などに ClickHouse identifier として安全でない文字が含まれています。英字、数字、`_` を使い、先頭は英字または `_` にしてください。

### `run requires exactly one output node in v1`

v1 の Run は Output node 1 つだけを対象にします。Preview は複数 Output graph でも選択 node まで確認できますが、Run する graph は Output を 1 つにしてください。

### `data pipeline version is not published`

Schedule 作成や Run 対象 version が published ではありません。`Publish` を押すか、`Run` を押して自動 publish を行ってください。

### Preview / Run が古い設定で動く

Config JSON が invalid だと draft graph に反映されません。Inspector の error を解消してから Preview / Run してください。

## v1 の制約

- runtime は ClickHouse 優先です。
- 入力は active tenant の Dataset または managed Work table です。
- 出力は tenant work database の managed Work table です。
- DuckDB / Parquet runtime は未対応です。
- 外部 S3 / 外部 DB input は未対応です。
- 任意 SQL node はありません。structured config から allowlist ベースで SQL を生成します。
- LLM enrichment は未対応です。
- multi-output fanout write は未対応です。
- backfill UI は未対応です。
- `Profile` / `Validate` は v1 では主に定義保存 / passthrough 用です。厳密な品質検査や profile metadata の拡張は後続フェーズです。

## 推奨パターン

### まず最小 graph で確認する

最初は次の graph で Input / Output が動くことを確認してください。

```text
Input -> Output
```

Input source を選び、Output display name を設定して Preview / Run します。

### 加工は少しずつ追加する

一度に多くの node を追加せず、1 node 追加するたびに Preview してください。

```text
Input -> Clean -> Preview
Input -> Clean -> Normalize -> Preview
Input -> Clean -> Normalize -> Output -> Run
```

### Schema Mapping は後段に置く

Schema Mapping は target column だけを出力するため、後続 node では元の列名が使えなくなります。列名変更後に filter / aggregate を行う場合は、変更後の列名を使ってください。

### Run 前に Output を確認する

Run では最終 Output node までの結果が Work table に書き込まれます。Run 前に Output node を選択して Preview し、列と値を確認してください。
