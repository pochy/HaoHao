# 後続機能計画

## AI 向けメンテナンス方針

この文書は、今後も人間と AI が継続的に追記・更新する前提で管理します。AI がこの文書を編集する場合は、次のルールを守ります。

- まず現在の実装状態を確認し、推測だけで「実装済み」「未実装」「後続」を書き換えない。
- 新しい後続機能を追加する場合は、本文末尾にメモだけを足さず、`後続機能一覧`、`優先順位`、該当 `Phase`、`完了条件` を必要に応じて同時に更新する。
- 実装が完了した機能は、該当 Phase の説明を残したまま現在地を更新し、必要なら `v1 で実装済みの範囲` または後続の実装済み範囲として整理する。
- README へのリンク追加は、この文書を正式な実装フェーズとして扱うと決めたタイミングで行う。
- DB migration SQL、具体的な Go 関数名、生成物の差分をこの文書に細かく固定しすぎない。設計判断、必要な schema / API / UI / worker 変更、テスト観点、完了条件を中心に書く。
- `backend/internal/db/*`、`frontend/src/api/generated/*`、`openapi/*.yaml` のような生成物を手書き編集する前提の手順を書かない。生成が必要な場合は generator 経由で更新する前提にする。
- 破壊的操作、tenant 境界、raw DB / work DB の扱い、export file の扱いなど安全性に関わる前提を変える場合は、`共通設計ルール` と `やらないこと` も見直す。

追記する機能は、原則として次の粒度で書きます。

```md
## Phase N: 機能名

### 目的

なぜこの機能が必要か、v1 でやらなかった理由を書く。

### 主な変更

DB / API / backend worker / frontend UI / i18n / observability のうち、影響する領域を書く。

### 設計方針

採用する方式、採用しない方式、重要な制約を書く。

### 完了条件

ユーザー視点とテスト視点で、何ができれば完了かを書く。
```

ステータス表記が必要な場合は、次を使います。

| Status | 意味 |
| --- | --- |
| Proposed | 方向性だけがある。まだ実装計画として固定していない |
| Planned | 実装する前提で、Phase と完了条件が決まっている |
| In progress | 実装中。仕様変更があればこの文書も更新する |
| Implemented | 実装済み。現在地と確認コマンドを更新済み |
| Deferred | 意図的に後回しにしている |
| Superseded | 別方針に置き換わった。置き換え先を本文に残す |

## この文書の目的

この文書は、HaoHao の後続機能を継続的に管理し、実装へ落とすための計画です。現在の主な対象は、Dataset / SQL Studio の Work table v1 で意図的に後続へ回した機能です。

Work table v1 では、ClickHouse の `hh_t_<tenant>_work` 配下に作成された table を UI で確認し、管理レコード化し、作成元 Dataset と紐付け、rename / truncate / drop、正式 Dataset 化、CSV export まで扱えるようにしました。一方で、次の機能は v1 の複雑さを抑えるために対象外にしています。

- Parquet / JSON export
- scheduled export
- Dataset 化後の再同期 / 差分同期
- Work table の完全な lineage / dependency graph
- 大容量 export の streaming 最適化
- export retention / cleanup policy の詳細化

この文書では、これらを実装順、設計方針、DB / API / UI / worker への影響、完了条件まで分解します。

README へのリンク追加は、この文書を正式な後続機能計画として採用するタイミングで行います。

## 前提と現在地

この文書は、少なくとも次の状態にある前提で進めます。

- `/datasets` に Dataset list と Work tables セクションがある
- `/datasets/:datasetPublicId` に SQL / スキーマ / 履歴タブがある
- Work table は `dataset_work_tables` で管理できる
- Work table と作成元 Dataset は、自動または手動 link で紐付けられる
- Dataset 詳細のスキーマタブには、その Dataset に紐付く Work tables だけが表示される
- Work table の preview は query history に追加されない
- Work table の promote は raw DB へのスナップショットコピーとして扱う
- Work table の export は CSV の単発 export として扱う
- outbox worker、file storage、download endpoint の基本的な流れが使える

この文書では、Work table を正式 Dataset と同じ正本として扱うのではなく、引き続き `hh_t_<tenant>_work` 配下の作業成果物として扱います。正式 Dataset 化したものだけが raw DB 側の独立 Dataset になります。

## v1 で実装済みの範囲

v1 の責務は、Work table を「見える」「由来を管理できる」「安全に lifecycle 操作できる」「Dataset 化できる」「CSV で export できる」状態にすることです。

| 領域 | v1 の扱い |
| --- | --- |
| Work table 一覧 | active tenant の work DB を表示し、managed / unmanaged を区別する |
| 管理レコード | `dataset_work_tables` に public id、work database/table、status、row count、bytes、engine、source dataset を持つ |
| 由来管理 | scoped SQL の `CREATE TABLE` 成功時は自動登録し、既存 table は手動 register / link できる |
| Dataset 詳細 | 紐付いた Work tables だけをスキーマタブに表示する |
| lifecycle | managed table の rename / truncate / drop を扱う |
| promote | Work table を raw DB にコピーし、独立した Dataset として作成する |
| export | CSV の単発 export を作成し、ready 後に download できる |
| 安全性 | lifecycle / promote / export は managed table のみ対象にする |

v1 の promote はスナップショットコピーです。Promote 後に元 Work table を更新しても、作成済み Dataset へ自動反映しません。

v1 の export は CSV のみです。型保持や列指向分析用途、定期実行、保持期間管理はこの後続計画で扱います。

## 後続機能一覧

| 機能 | 現在地 | 後続で実装すること |
| --- | --- | --- |
| Export format 拡張 | CSV のみ | Parquet / JSON を追加し、format ごとの content type、拡張子、生成処理を持つ |
| Scheduled export | 単発 request のみ | schedule、次回実行時刻、重複実行防止、失敗管理、保持期間を追加する |
| Dataset 再同期 | Promote 時点の snapshot のみ | full refresh、append、key-based merge の同期方式を設計する |
| 差分同期 | 未実装 | primary key / cursor / watermark などの差分条件を扱う |
| Lineage | source dataset / query job の最小紐付け | Dataset、query job、Work table、promoted Dataset、export の関係を可視化する |
| Export cleanup | 明示的な後続対象 | expires_at、file object、download 可否、削除済み状態を整理する |
| 大容量 export | v1 では単純実装 | backend memory を抱え込まず、streaming / chunking に寄せる |

## 優先順位

実装順は次を基本にします。

| 優先 | Phase | 理由 |
| --- | --- | --- |
| 0 | CSV export 安定化 | v1 の基盤が不安定なまま format や schedule を増やさない |
| 1 | Export format 拡張 | 既存 export API / worker / file storage を自然に拡張できる |
| 2 | Scheduled export | 単発 export の成功 / 失敗 / download / cleanup が固まってから定期実行にする |
| 3 | Dataset 化の再同期 / 差分同期 | データ破壊や二重取り込みのリスクが高いため、同期方式を明確にしてから入れる |
| 4 | Lineage / dependency graph | SQL 解析と可視化が絡むため、基礎 metadata が蓄積されてから入れる |

短期の実装順は、`CSV export 安定化 -> format 拡張 -> scheduled export -> resync -> lineage` とします。

## Phase 0: CSV export 安定化

Dataset Work Table 後続機能の最初に、v1 CSV export の動作を確認し、後続 format / schedule の土台として使える状態にします。

### 目的

CSV export の request、worker 処理、file 保存、download、failed 状態を安定させます。ここで大きな設計漏れがある場合は、Parquet / JSON や scheduled export を追加する前に修正します。

### 主な確認対象

- export request が managed active Work table だけを対象にする
- unsupported status の Work table には export を作らない
- worker が processing / ready / failed を正しく更新する
- ready export だけ download できる
- failed export は `error_summary` を UI に出せる
- response body や log に raw SQL result、認証情報、CSRF token を出さない
- large table で backend memory 使用量が過剰にならない設計へ移行できる余地がある

### 追加検討

CSV export が現在 memory buffer 中心の場合、Phase 1 の前に streaming writer へ寄せるかを判断します。Parquet は format library の都合で一時 file が必要になる可能性があるため、export worker の内部 I/O を「memory 前提」に固定しないようにします。

### 完了条件

- CSV export の request / worker / download が手動確認できる
- failed export が UI と application log で原因を追える
- export file の purpose、content type、filename が確認できる
- large table 対応の設計メモが Phase 1 に引き継がれている

## Phase 1: Export format 拡張

### 目的

CSV に加えて、Parquet / JSON を選んで export できるようにします。

CSV は人間が確認しやすい一方で、型情報や大容量分析用途には弱いです。Parquet は分析基盤連携、JSON は API 連携や lightweight exchange に向きます。

### 主な変更

- `dataset_work_table_exports.format` に `parquet`, `json` を追加する
- export request API で format を受け取れるようにする
- unsupported format は 400 を返す
- format ごとに file extension、content type、download filename を分ける
- worker は format ごとの writer を選ぶ
- UI の Work table detail に format selector を追加する
- export list に format を表示する

### Format 方針

| Format | 用途 | content type | 備考 |
| --- | --- | --- | --- |
| CSV | spreadsheet / 目視確認 | `text/csv` | v1 互換として維持する |
| JSON | API 連携 / lightweight exchange | `application/json` | 最初は JSON Lines ではなく配列 JSON を基本にするか、実装前に決める |
| Parquet | 分析基盤連携 | `application/vnd.apache.parquet` | ClickHouse 型から Parquet 型への mapping を明示する |

JSON は、通常 JSON 配列と JSON Lines のどちらを採用するかを実装前に決めます。大容量 table では JSON Lines の方が streaming しやすいため、この Phase の実装時は JSON Lines を優先候補にします。ただし UI 表示名は `JSON` とし、詳細には `newline-delimited JSON` と表示してもよいです。

Parquet は ClickHouse の型をどこまで保持するかが重要です。最初は scalar 型を主対象にし、Nested / Array / Map / Tuple は JSON string へ fallback するか、unsupported として明示的に failed にします。

### API / UI

export request は既存の Work table export endpoint を拡張します。新しい endpoint family は増やさず、既存 request body に `format` を追加する方針にします。

UI は Work table detail の exports 領域に format selector を追加します。既定は `csv` のままにし、既存の操作を壊しません。

### 失敗時の扱い

format writer が対応できない型、file storage への保存失敗、ClickHouse query 失敗は export を `failed` にします。client response に内部 detail は出さず、UI には安全な `error_summary` を表示します。

### 完了条件

- CSV / JSON / Parquet の export request が作れる
- ready export を format ごとの拡張子で download できる
- unsupported format は 400 になる
- writer が未対応型を安全に failed へ落とせる
- format ごとの unit test と worker test がある
- `go test ./backend/...` と `cd frontend && npm run build` が通る

## Phase 2: Scheduled export

### 目的

Work table の export を手動 request だけでなく、定期実行できるようにします。

Scheduled export は単発 export の上位機能です。単発 export の成功 / 失敗 / download / cleanup が安定してから実装します。

### 主な変更

- Work table ごとの export schedule を保存する
- schedule には format、frequency、timezone、next run、enabled、retention を持たせる
- scheduler は due schedule を outbox event に変換する
- 同じ schedule が同時に複数実行されないようにする
- 実行ごとの export record は既存 export と同じ list に表示する
- UI は Work table detail の exports 領域に schedule 管理を追加する

### Schedule 方針

最初の frequency は次に絞ります。

| Frequency | 例 | 備考 |
| --- | --- | --- |
| daily | 毎日 03:00 | timezone 必須 |
| weekly | 毎週月曜 03:00 | weekday と timezone 必須 |
| monthly | 毎月 1 日 03:00 | month day と timezone 必須 |

Cron expression を直接入力させる方式は v1 では採用しません。UI validation と運用上の説明が難しく、tenant user 向けには固定 option の方が安全です。

### 重複実行防止

schedule worker は、同一 schedule の `pending` / `processing` export が存在する場合、新しい export を作らない方針にします。前回実行が詰まっている状態で export を積み増すと、storage と ClickHouse 負荷を増やすためです。

### Retention / cleanup

schedule ごとに保持件数または保持日数を設定できるようにします。最初は保持日数を優先し、期限切れ export は `deleted` status にして download を拒否します。file object の物理削除は既存 file lifecycle の方針に合わせます。

### UI

Work table detail の exports 領域に次を追加します。

- schedule list
- create / update / disable schedule
- next run 表示
- last run result 表示
- retention 表示

Schedule の削除は物理削除ではなく disable を基本にします。履歴として過去 export を残すためです。

### 完了条件

- daily / weekly / monthly schedule を作成できる
- scheduler が due schedule から export を作成する
- 同一 schedule の重複実行が防止される
- failed schedule run が UI に表示される
- retention に従って古い export が download 不可になる
- schedule の enable / disable ができる
- `go test ./backend/...` と `cd frontend && npm run build` が通る

## Phase 3: Dataset 化の再同期 / 差分同期

### 目的

Work table を promote した Dataset に対して、後から Work table の変更を反映できるようにします。

現行 promote は raw DB へのスナップショットコピーです。これは元 Work table の drop / truncate / rename から Dataset を守るための安全な初期仕様です。一方で、Work table を継続的に更新し、それを Dataset として再利用したい場合は、再同期または差分同期が必要になります。

### 同期方式

実装前に、少なくとも次の 3 方式を比較します。

| 方式 | 内容 | 向いている用途 | 主なリスク |
| --- | --- | --- | --- |
| full refresh | promoted Dataset の raw table を全量作り直す | 小中規模 table、単純な再生成 | 実行中の Dataset 読み取りとの整合性 |
| append | Work table の新規行だけを追加する | event / log 系 table | 重複防止に key または watermark が必要 |
| key-based merge | primary key 相当で upsert / delete 反映する | master data / summary table | key 設計と ClickHouse engine 依存が強い |

Phase 3 の最初は full refresh を優先します。理由は、Work table と Dataset の整合性を説明しやすく、append / merge より誤更新のリスクが低いためです。

### 主な変更

- promoted Dataset が source Work table を参照できる metadata を持つ
- Dataset detail に source Work table と last sync status を表示する
- resync request API を追加する
- resync は outbox worker で非同期実行する
- sync history を保存する
- sync 成功時に row count、columns、imported_at 相当を更新する
- sync 失敗時に Dataset を壊さず、失敗履歴を残す

### 安全な full refresh

Full refresh は既存 raw table を直接 truncate しない方針にします。新しい raw table を一時名で作成し、コピー成功後に Dataset metadata を差し替えるか、rename swap できる設計にします。

実行途中で失敗した場合、既存 Dataset は引き続き古い snapshot を読める状態を維持します。

### Append / merge を後ろに回す理由

Append / merge は、重複行、削除反映、primary key、watermark、ClickHouse engine の選定が絡みます。Dataset の意味を壊しやすいため、Phase 3 の初回では full refresh を実装し、その後に append / merge を追加します。

### UI

Promoted Dataset detail に次を表示します。

- source Work table
- last synced at
- last sync status
- sync history
- resync action

Resync は破壊的ではありませんが、Dataset の内容が変わるため confirm dialog を通します。

### 完了条件

- Work table 由来の Dataset だけ resync できる
- full refresh が成功すると Dataset preview / schema が更新される
- full refresh 失敗時に既存 Dataset が壊れない
- sync history で成功 / 失敗を確認できる
- Dataset detail に last sync status が表示される
- append / merge は未実装として UI に出さない
- `go test ./backend/...` と `cd frontend && npm run build` が通る

## Phase 4: Lineage / dependency graph

### 目的

Dataset、query job、Work table、promoted Dataset、export の関係を UI と API で追えるようにします。

v1 では、Work table の source dataset と created query job までを保存します。これは最小の由来管理です。Phase 4 では、この metadata を起点に、ユーザーが「この table は何から作られ、どこで使われたか」を確認できるようにします。

### 表示したい関係

- Dataset から実行された query job
- query job が作成した Work table
- Work table の source Dataset
- Work table から promote された Dataset
- Work table から作成された export
- Dataset 化後の resync history

UI では、最初から複雑な graph editor にしません。Dataset detail と Work table detail に lineage section を追加し、一次関係を list / timeline として見せます。

### SQL parser の扱い

完全な SQL lineage 解析は後続扱いにします。

ClickHouse SQL では WITH、JOIN、UNION、subquery、database/table alias、quoted identifier、function table、view などがあり、正確な依存関係抽出は単純な文字列解析では危険です。Phase 4 の初回は、アプリが確実に保存している metadata を正本にします。

SQL parser を導入する場合は、次を明示的に設計します。

- parser library の選定
- ClickHouse dialect 対応範囲
- parser failure 時の fallback
- 複数 Dataset / Work table 参照時の表現
- lineage confidence の表示

### API / UI

Lineage API は、最初は resource 起点の read-only endpoint にします。

- Dataset 起点 lineage
- Work table 起点 lineage
- query job 起点 lineage

UI は次の順で追加します。

1. Work table detail に source Dataset / created query / promoted Dataset / exports を表示
2. Dataset detail に linked Work tables / promoted source / sync history を表示
3. 必要になったら graph view を追加

### 完了条件

- Dataset detail から関連 Work table と promoted source が分かる
- Work table detail から source Dataset、created query job、promoted Dataset、exports が分かる
- SQL parser に頼らない metadata lineage が read-only API で取れる
- parser が未実装でも lineage UI が破綻しない
- `go test ./backend/...` と `cd frontend && npm run build` が通る

## 共通設計ルール

この文書の各 Phase では、次のルールを維持します。

- Work table は active tenant の work DB だけを対象にする
- raw DB、system DB、他 tenant DB への lifecycle / export / sync を拒否する
- destructive action は confirm dialog を通す
- mutating endpoint は CSRF と active tenant role を必須にする
- generated files は手書き編集しない
- OpenAPI と frontend SDK は generator 経由で更新する
- response body に内部 error detail、raw SQL result、認証情報を出さない
- application error は `application_error` log で request id と紐付ける
- Work table preview / export / lineage read は query history に追加しない
- managed table だけを lifecycle / promote / export / schedule / sync の対象にする

## やらないこと

この文書では、次は対象外にします。

- Work table を Dataset list に混ぜる
- unmanaged table に対する destructive action
- Work table を raw DB の正本として直接扱う
- Cross-tenant の Work table 参照
- SQL parser による完全 lineage を最初から必須にする
- Cron expression を tenant user に直接入力させる scheduled export
- Append / merge sync を full refresh より先に実装する
- Export file の外部共有や公開リンク
- BI tool connector や external warehouse への直接転送

## 確認コマンド

各 Phase の実装では、少なくとも次を確認します。

```bash
./scripts/gen.sh
go test ./backend/...
cd frontend && npm run build
```

ドキュメントだけを更新した場合は、次を確認します。

```bash
git diff --check -- FUTURE_FEATURES.md
```

## 完了条件

この文書で扱う Dataset Work Table 後続機能全体の完了条件は次です。

- CSV export の v1 flow が安定している
- CSV / JSON / Parquet export を選択できる
- scheduled export を作成、無効化、履歴確認できる
- retention により古い export の download 可否が制御される
- Work table 由来 Dataset を full refresh で再同期できる
- 再同期失敗時に既存 Dataset が壊れない
- Dataset detail と Work table detail で主要 lineage が確認できる
- Lineage 初期版は parser に依存せず、保存済み metadata だけで動く
- `./scripts/gen.sh`、`go test ./backend/...`、`cd frontend && npm run build` が通る
