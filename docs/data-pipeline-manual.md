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

データパイプライン画面は、一覧ページと詳細 / 編集ページに分かれています。

### 一覧ページ

`/data-pipelines` は pipeline の作成と一覧確認を行うページです。

| 領域 | 用途 |
| --- | --- |
| 新規 pipeline 作成 | pipeline の名前と説明を入力して作成する |
| Pipeline 一覧 | 既存 pipeline の名前、説明、status、更新日時を確認し、詳細 / 編集ページへ移動する |

### 詳細 / 編集ページ

`/data-pipelines/{pipelinePublicId}` は pipeline の編集、Preview、Run、Schedule を行うページです。

| 領域 | 用途 |
| --- | --- |
| 左サイドバー | node palette、利用可能な Dataset / Work table の確認 |
| 中央 canvas | Vue Flow による DAG builder。node の追加、配置、接続を行う |
| 右 Inspector | 選択 node の label と config を編集する |
| 下部 Preview / Runs / Schedules panel | Preview、Run 履歴、schedule 一覧をタブで確認する |
| 画面上部 action | pipeline 名 / 説明の編集、Save、Publish、Refresh、Run、Schedule 設定 |

## 基本ワークフロー

1. active tenant を選択します。
2. `/data-pipelines` を開きます。
3. 一覧ページの `New pipeline` で名前と説明を入力し、`Create` を押します。
4. 作成後、詳細 / 編集ページへ移動します。
5. 初期 graph には `Input -> Output` が作成されます。
6. `Input` node を選択し、右 Inspector で source を設定します。
7. 左サイドバーの `Palette` から必要な処理 node を追加します。
8. canvas 上で node を接続し、処理順を作ります。
9. 各 node を選択して Inspector の `Config UI` または `Config JSON` を編集します。
10. 下部 `Preview` で選択 node までの結果を確認します。
11. 必要に応じて画面上部の `Save` / `Publish` を押します。
12. 画面上部の `Run` で手動実行します。
13. 定期実行したい場合は画面上部の `Schedule` で頻度と時刻を設定して `Add` を押します。

## Pipeline の作成と選択

### 新規作成

一覧ページの `New pipeline` で次を入力します。

- `Name`: pipeline の表示名。
- `Description`: 任意の説明。

`Create` を押すと pipeline が作成され、詳細 / 編集ページへ移動します。詳細 / 編集ページでは初期 graph として `Input -> Output` が表示されます。

### 既存 pipeline の選択

一覧ページの `Pipelines` から対象 pipeline を選択します。選択すると詳細 / 編集ページへ移動し、その pipeline の最新 version graph、run 履歴、schedule が読み込まれます。

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

## Preview と Run の違い

Preview は途中確認、Run は本番実行です。

| 項目 | Preview | Run |
| --- | --- | --- |
| 対象 | 選択中の node まで | Output node まで |
| 実行内容 | `SELECT ... LIMIT 100` | 最終結果を ClickHouse table として作成 |
| version | 必要に応じて draft version を自動保存 | 必要に応じて draft 保存 + publish |
| 出力 Work table | 作らない | 作る / 置き換える |
| Run 履歴 | 残らない | `Runs` に残る |
| Medallion 記録 | 残らない | 残る |
| 用途 | 設定や列変換が正しいか確認 | 加工済みデータを正式に生成 |

Preview は「この node までの結果が想定通りか」を見るための読み取り専用確認です。Run は pipeline 全体を実行して、Output node の設定に従って managed Work table を作成します。

### 大規模データの場合

1 億行のような大規模データでは、Preview と Run の処理範囲を分けて考えてください。

Preview は表示結果を最大 100 行に制限します。ただし、必ず 100 行分だけ読んで終わるわけではありません。単純な `Input -> Clean -> Preview` のような graph では ClickHouse が `LIMIT 100` で早めに止められることがあります。一方で、`aggregate`、`sort`、`dedupe`、`join` などを含む場合は、正しい 100 行の結果を作るために、入力側の 1 億行を広く scan することがあります。

Run は基本的に Output node まで全件処理します。入力が 1 億行なら、pipeline 全体をそのデータに対して実行し、結果を managed Work table として作成します。ただし出力行数は処理内容によって変わります。`filter` で絞れば減り、`aggregate` すれば集計結果だけになり、`normalize` や `clean` 中心なら入力に近い行数になります。

| 操作 | 1 億行データでの扱い |
| --- | --- |
| Preview | 表示は最大 100 行。ただし処理内容によっては大きく scan する |
| Run | Output まで全件処理し、managed Work table を作る |

大規模データでは、まず Input や前段 node を Preview し、重い `join` / `aggregate` / `sort` は設定が固まってから Run してください。

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

画面上部の `Run` ボタンで手動実行します。実行履歴は下部 `Runs` tab で確認します。

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

画面上部の `Schedule` で定期実行を設定します。作成済み schedule は下部 `Schedules` tab で確認します。

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

- Schedule は作成時点の published version に紐付きます。現在の UI は必要に応じて draft を保存し、未 publish の場合は publish してから schedule を作成します。
- 別の version を publish した後も、既存 schedule の `versionId` は自動では差し替わりません。scheduler は pipeline の現在の published version と schedule の version が一致しない場合、その schedule を無効化します。pipeline を更新して publish した後は、必要に応じて schedule を作り直してください。
- schedule worker は due schedule を claim し、前回 run がまだ `pending` / `processing` の場合は新しい run を skip します。
- schedule を止めるには schedule 一覧の trash icon を押します。削除ではなく disabled になります。

## 出力 Work table

Run が成功すると、Output node の config に基づいて managed Work table が作成または置き換えられます。

- `displayName` が Work table の表示名になります。
- `tableName` が指定されていればその table 名を使います。
- `tableName` が未指定の場合は run public ID から `dp_...` 形式で自動生成されます。
- 出力は Dataset / Work table 画面、lineage、medallion catalog から追跡できます。

## 内部処理と仕組み

この章では、画面操作の裏側でどの API、DB table、service、ClickHouse query、job が動くかを説明します。

### 全体アーキテクチャ

データパイプラインは、主に次の層で構成されています。

| 層 | 主な責務 |
| --- | --- |
| Frontend | `/data-pipelines` 一覧、`/data-pipelines/{pipelinePublicId}` 詳細 / 編集、Vue Flow canvas、Inspector、Preview / Runs / Schedules panel |
| API | `/api/v1/data-pipelines` 配下の CRUD、version save / publish、preview、run request、schedule 操作 |
| Service | graph validation、version 管理、ClickHouse SQL compile、Preview 実行、Run request、schedule claim、medallion 記録 |
| PostgreSQL | pipeline metadata、version graph、run、run step、schedule、outbox event を tenant-scoped table に保存 |
| ClickHouse | Preview SELECT、Run の stage table 作成、最終 managed Work table 作成 |
| Outbox / Jobs | `data_pipeline.run_requested` event の非同期処理、due schedule の claim と run 作成 |

重要な点は、ユーザーが任意 SQL を保存・実行するのではなく、Vue Flow graph の structured config から backend が allowlist ベースで ClickHouse SQL を生成することです。

### Tenant と権限

すべての操作は active tenant に紐付きます。

- API は session cookie から現在のユーザーを確認します。
- active tenant がない場合、操作は失敗します。
- active tenant で `data_pipeline_user` role が必要です。
- mutation API は CSRF token を要求します。
- create / run request は `Idempotency-Key` に対応しています。
- PostgreSQL query は `tenant_id` 条件を必須にして、他 tenant の pipeline / version / run / schedule を参照しません。
- ClickHouse 実行時も tenant sandbox / tenant work database を使います。

### 主要 table

Data Pipeline v1 で使う PostgreSQL table は次の通りです。

| table | 保存内容 |
| --- | --- |
| `data_pipelines` | pipeline の名前、説明、status、published version への参照 |
| `data_pipeline_versions` | 保存された graph JSON、version number、validation summary、publish 状態 |
| `data_pipeline_runs` | manual / scheduled run の状態、出力 work table、row count、error |
| `data_pipeline_run_steps` | run 内の node ごとの状態、row count、error sample、metadata |
| `data_pipeline_schedules` | schedule の頻度、timezone、次回実行時刻、last status |
| `outbox_events` | `data_pipeline.run_requested` event を非同期 worker に渡すための event |
| `medallion_pipeline_runs` | medallion catalog 上の pipeline run 履歴 |

`graph` は `data_pipeline_versions.graph` に JSONB として保存されます。中身は Vue Flow 互換の `nodes` / `edges` です。

```json
{
  "nodes": [
    {
      "id": "input_1",
      "type": "pipelineStep",
      "position": { "x": 60, "y": 120 },
      "data": {
        "label": "Input",
        "stepType": "input",
        "config": {
          "sourceKind": "dataset",
          "datasetPublicId": "..."
        }
      }
    }
  ],
  "edges": [
    { "id": "edge_input_output", "source": "input_1", "target": "output_1" }
  ]
}
```

### API の役割

主な API は次の通りです。

| 操作 | API | 内部で呼ばれる service |
| --- | --- | --- |
| 一覧 | `GET /api/v1/data-pipelines` | `DataPipelineService.List` |
| 作成 | `POST /api/v1/data-pipelines` | `DataPipelineService.Create` |
| 詳細 | `GET /api/v1/data-pipelines/{pipelinePublicId}` | `DataPipelineService.Get` |
| 名前 / 説明更新 | `PATCH /api/v1/data-pipelines/{pipelinePublicId}` | `DataPipelineService.Update` |
| graph 保存 | `POST /api/v1/data-pipelines/{pipelinePublicId}/versions` | `DataPipelineService.SaveDraftVersion` |
| publish | `POST /api/v1/data-pipeline-versions/{versionPublicId}/publish` | `DataPipelineService.PublishVersion` |
| preview | `POST /api/v1/data-pipeline-versions/{versionPublicId}/preview` | `DataPipelineService.Preview` |
| run request | `POST /api/v1/data-pipeline-versions/{versionPublicId}/runs` | `DataPipelineService.RequestRun` |
| run 一覧 | `GET /api/v1/data-pipelines/{pipelinePublicId}/runs` | `DataPipelineService.ListRuns` |
| schedule 作成 | `POST /api/v1/data-pipelines/{pipelinePublicId}/schedules` | `DataPipelineService.CreateSchedule` |
| schedule 更新 | `PATCH /api/v1/data-pipeline-schedules/{schedulePublicId}` | `DataPipelineService.UpdateSchedule` |
| schedule 無効化 | `DELETE /api/v1/data-pipeline-schedules/{schedulePublicId}` | `DataPipelineService.DisableSchedule` |

詳細 API は pipeline 本体だけでなく、最新 versions、published version、直近 runs、schedules をまとめて返します。詳細 / 編集ページはこのレスポンスを元に canvas、Runs、Schedules を復元します。

### Frontend の状態管理

Frontend は Pinia の data pipeline store で次の状態を持ちます。

| state | 用途 |
| --- | --- |
| `items` | 一覧ページの pipeline list |
| `selectedPublicId` | 詳細表示中の pipeline public ID |
| `detail` | 詳細 API の結果 |
| `draftGraph` | canvas / Inspector で編集中の graph |
| `selectedNodeId` | Preview / Inspector の対象 node |
| `preview` | Preview API の結果 |
| `runs` | Run 履歴 |
| `schedules` | Schedule 一覧 |

一覧ページでは `store.load(false)` で list だけを取得します。詳細 / 編集ページでは list 取得後に `store.loadDetail(pipelinePublicId)` を呼び、対象 pipeline の graph / run / schedule を読み込みます。

Preview、Run、Schedule 作成の前には、frontend が `ensureDraftVersion()` を呼びます。最新 version の graph と `draftGraph` が同じなら既存 version を使い、違う場合は新しい draft version を保存します。これにより、ユーザーが明示的に `Save` を押し忘れても Preview / Run / Schedule は現在の canvas 内容に基づいて動きます。

### Graph validation

`Save`、`Publish`、`Preview`、`Run` の入口で backend は graph validation を行います。

validation の主な目的は、DAG として実行可能な形か、そして backend compiler が安全に処理できる形かを確認することです。

チェック内容:

- node が存在すること。
- node 数が 50 以下であること。
- edge 数が 80 以下であること。
- node ID が空でないこと。
- node ID が重複しないこと。
- step type が v1 catalog に存在すること。
- `input` がちょうど 1 つ存在すること。
- `output` が 1 つ以上存在すること。
- edge の source / target が存在する node を指すこと。
- self-loop がないこと。
- graph が acyclic であること。
- すべての node が input から到達可能であること。
- すべての node がいずれかの output に到達可能であること。
- input 以外の node に upstream edge があること。

加えて、SQL compile 時には v1 runtime の制約として、通常の処理 node は upstream edge がちょうど 1 つである必要があります。複数 upstream の DAG merge は v1 では任意には扱いません。Join は `enrich_join` node の config で右側 source を指定する方式です。

### Version の保存と publish

`Save` は現在の graph を新しい draft version として保存します。

内部処理:

1. `data_pipelines` を `tenant_id + public_id` で取得します。
2. graph validation を実行します。
3. graph と validation summary を JSONB に encode します。
4. `data_pipeline_versions` に新しい row を insert します。
5. `version_number` は同じ pipeline の最大 version number + 1 で採番されます。
6. audit に `data_pipeline.version.save` を記録します。

`Publish` は version を pipeline の current published version にします。

内部処理:

1. version を `tenant_id + versionPublicId` で取得します。
2. graph を decode して再 validation します。
3. transaction を開始します。
4. 対象 version の status を `published` にし、`published_at` を設定します。
5. 同じ pipeline の他の published version を `archived` にします。
6. `data_pipelines.published_version_id` を対象 version に更新し、pipeline status を `published` にします。
7. transaction を commit します。
8. audit に `data_pipeline.version.publish` を記録します。

このため、manual run と schedule は「現在 publish されている version」を基準に実行されます。

### SQL compiler の考え方

Data Pipeline v1 は raw SQL node を持ちません。backend は graph と各 node の structured config を読み、ClickHouse SQL を生成します。

基本方針:

- node を topological order で並べます。
- 各 node を ClickHouse の CTE に変換します。
- CTE 名は node ID から `step_...` 形式で安全な名前に正規化します。
- column、target column、alias、table name などは allowlist / identifier check を通します。
- 文字列、数値、bool、NULL は backend の literal builder で SQL literal に変換します。
- user input をそのまま SQL 断片として実行しません。
- Dataset / Work table の source は backend が tenant 内の resource として解決します。

Preview の SQL は、選択 node までの CTE を作り、最後に `SELECT * FROM step_selected LIMIT 100` の形で実行されます。

Run の SQL は、Output node までの CTE を作り、ClickHouse の `CREATE TABLE ... AS SELECT ...` に使われます。

概念的には次のような SQL が生成されます。

```sql
WITH
`step_input_1` AS (
  SELECT *
  FROM `tenant_raw_db`.`source_table`
),
`step_clean_1` AS (
  SELECT
    ifNull(`name`, 'unknown') AS `name`,
    `email` AS `email`
  FROM `step_input_1`
  WHERE isNotNull(`email`)
),
`step_output_1` AS (
  SELECT *
  FROM `step_clean_1`
)
SELECT *
FROM `step_output_1`
LIMIT 100
```

実際の SQL は graph と config に応じて backend が生成します。

### Node ごとの compile

各 node は次のように SQL へ変換されます。

| node | compile 内容 |
| --- | --- |
| `input` | Dataset または managed Work table の ClickHouse table を `SELECT *` |
| `profile` | v1 では passthrough |
| `clean` | NULL 除外、NULL 補完、NULL 化、clamp、制御文字除去、dedupe を `SELECT` / `WHERE` / window function に変換 |
| `normalize` | trim、lower / upper、space 正規化、記号除去、decimal / date 変換、round、scale、値 mapping を expression に変換 |
| `validate` | v1 では passthrough。将来の品質検査 metadata 用の config を保存 |
| `schema_mapping` | mapping に従って target column だけを `SELECT expr AS target` |
| `schema_completion` | literal、copy_column、coalesce、concat などで新しい列を追加 |
| `enrich_join` | 右側 Dataset / Work table を解決し、left join して指定列を追加 |
| `transform` | select / drop / rename / filter / sort / aggregate を SQL に変換 |
| `output` | v1 では passthrough。Run 時の出力 table 設定として使う |

column の存在確認は upstream columns を使って行われます。例えば Schema Mapping で `customer_id` を `id` に変えた後、後続 node で `customer_id` を参照すると `unknown column` になります。

### Identifier と安全性

SQL compiler は identifier と operation を制限します。

- table name、target column、alias は `^[A-Za-z_][A-Za-z0-9_]{0,127}$` に一致する必要があります。
- ClickHouse identifier は quote されます。
- cast は `string`, `int64`, `float64`, `decimal`, `date`, `datetime` などの allowlist のみです。
- condition operator は `required`, `=`, `!=`, `>`, `>=`, `<`, `<=`, `in`, `regex` などの allowlist のみです。
- aggregate function は `count`, `sum`, `avg`, `min`, `max` のみです。
- `enrich_join` の join type は v1 では `left` のみです。
- `in` operator は value array が必要です。
- unsupported operation / method / cast / operator は validation error または compile error になります。

この設計により、pipeline 定義は JSON として保存されますが、任意 SQL 実行にはなりません。

### Preview の内部処理

Preview は、選択中 node までの結果を確認するための軽量実行です。

Frontend の流れ:

1. 選択 node ID を決めます。未選択の場合は Output node、なければ先頭 node を候補にします。
2. 選択 node が孤立している場合、frontend は input から選択 node、選択 node から output への edge を補助的に追加します。
3. `ensureDraftVersion()` で現在の draft graph を version として保存します。
4. `POST /api/v1/data-pipeline-versions/{versionPublicId}/preview` を呼びます。

Backend の流れ:

1. version を `tenant_id + versionPublicId` で取得します。
2. graph JSON を decode します。
3. graph validation を実行します。
4. selected node までの ClickHouse SELECT を compile します。
5. tenant sandbox を準備します。
6. tenant 用 ClickHouse connection を開きます。
7. `SELECT ... LIMIT 100` を実行します。
8. ClickHouse rows を `columns` と `previewRows` に変換して返します。

Preview は run row や output Work table を作りません。エラーは API response として返り、画面上の error に表示されます。

### Manual Run の内部処理

Manual Run は、UI 上は `Run` ボタン 1 つですが、内部では request 作成と実行処理が分離されています。

Frontend の流れ:

1. `ensureDraftVersion()` で現在の graph を保存します。
2. その version が published でない場合、frontend は publish API を呼びます。
3. `POST /api/v1/data-pipeline-versions/{versionPublicId}/runs` を呼びます。

Backend の request 作成:

1. version を取得します。
2. version status が `published` であることを確認します。
3. pipeline の `published_version_id` がその version を指していることを確認します。
4. graph validation を実行します。
5. `data_pipeline_runs` に `pending` run を作成します。
6. `outbox_events` に `data_pipeline.run_requested` event を作成します。
7. run に `outbox_event_id` を保存します。
8. audit に `data_pipeline.run.request` を記録します。

この時点で API response は返ります。実際の ClickHouse 実行は outbox worker が非同期で行います。

Outbox handler の実行処理:

1. `data_pipeline.run_requested` event を受け取ります。
2. run を `processing` に更新します。
3. graph の全 node に対して `data_pipeline_run_steps` を作成し、step を `processing` にします。
4. graph を Output node まで compile します。v1 Run は Output node がちょうど 1 つ必要です。
5. tenant sandbox と tenant ClickHouse connection を準備します。
6. tenant work database に stage table を作ります。
7. stage table を target table に rename します。
8. target table を managed Work table として登録します。
9. 全 run step を `completed` にし、row count を保存します。
10. run を `completed` にし、`output_work_table_id` と row count を保存します。
11. medallion pipeline run を `completed` として記録します。
12. audit に `data_pipeline.run.complete` を記録します。

失敗した場合:

1. 全 run step を `failed` にします。
2. `error_sample` に `[{ "error": "..." }]` 形式で error を保存します。
3. run を `failed` にし、`error_summary` を保存します。
4. medallion pipeline run を `failed` として記録します。
5. outbox worker 側の retry / dead handling に従って event が扱われます。

現在の v1 実装では、成功時の step row count は各 node 個別の中間件数ではなく、最終 output Work table の row count が入ります。中間 step ごとの正確な row count / profile metadata は後続拡張対象です。

### Run 時の ClickHouse table 作成

Run では直接 target table を作らず、stage table を経由します。

処理の流れ:

1. target database は tenant work database です。
2. target table は Output node の `tableName` を使います。
3. `tableName` が空、または安全な identifier でない場合は run public ID から `dp_...` を自動生成します。
4. stage table 名は `__dp_stage_` + run public ID から作ります。
5. 既存 stage table を drop します。
6. `CREATE TABLE target_db.stage ENGINE = MergeTree ORDER BY tuple() AS {compiled SELECT}` を実行します。
7. 既存 target table を drop します。
8. stage table を target table に rename します。
9. `registerDatasetWorkTableForRef` で managed Work table として登録します。

つまり、Run 成功後に Dataset / Work table 側から見える出力は、ClickHouse table と PostgreSQL 上の managed Work table metadata の両方が揃った状態です。

### Schedule の内部処理

Schedule は `data_pipeline_schedules` に保存され、`DataPipelineScheduleJob` が定期的に due schedule を処理します。

Schedule 作成時:

1. pipeline を `tenant_id + public_id` で取得します。
2. pipeline に `published_version_id` があることを確認します。
3. frequency、timezone、run time、weekday / month day を正規化します。
4. `next_run_at` を計算します。
5. `data_pipeline_schedules` に insert します。
6. audit に `data_pipeline.schedule.create` を記録します。

Scheduler job:

1. 設定された interval で起動します。起動時に一度実行する設定もあります。
2. 同じ job が重複実行されないよう atomic flag で制御します。
3. transaction を開始します。
4. `enabled = true` かつ `next_run_at <= now` の schedule を claim します。
5. claim query は `FOR UPDATE SKIP LOCKED` を使うため、複数 worker がいても同じ schedule を同時処理しにくい設計です。
6. 次回 `next_run_at` を計算します。
7. pipeline の current published version と schedule の `version_id` が一致するか確認します。
8. 同じ schedule の active run がある場合は run を作らず `skipped` として次回時刻へ進めます。
9. 問題なければ `scheduled` trigger の run と outbox event を作成します。
10. schedule の `last_run_at`, `last_status`, `last_run_id`, `next_run_at` を更新します。
11. transaction を commit します。

実際の ClickHouse 実行は manual run と同じく outbox handler が処理します。

### Medallion catalog への記録

Run が完了または失敗すると、Data Pipeline は medallion pipeline run として記録されます。

記録内容:

| 項目 | 内容 |
| --- | --- |
| `pipeline_type` | `data_pipeline` |
| `runtime` | `clickhouse` |
| `trigger_kind` | `manual` または `scheduled` |
| `source_resource_kind` | 入力 source が Dataset なら `dataset`、Work table なら `work_table` |
| `source_resource_public_id` | Input node の source public ID |
| `target_resource_kind` | 成功時は `work_table` |
| `target_resource_public_id` | 出力 Work table の public ID |
| `status` | `completed` または `failed` |
| `metadata.versionPublicId` | 実行した version public ID |

この記録により、Dataset / Work table / medallion catalog から「どの pipeline run がどの source からどの output を作ったか」を追跡できます。

### 失敗時に見る場所

失敗時は次の順に確認します。

1. 詳細 / 編集ページの error message。
2. Runs tab の `Error`。
3. `data_pipeline_runs.error_summary`。
4. `data_pipeline_run_steps.error_summary` と `error_sample`。
5. outbox worker log。
6. medallion pipeline run の status / error summary。

Preview 失敗は run を作らないため、`data_pipeline_runs` には残りません。Run 失敗は run / run step / medallion に記録されます。

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
