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

この文書は、HaoHao の後続機能を継続的に管理し、実装へ落とすための計画です。現在の主な対象は、Dataset / SQL Studio の Work table v1 で意図的に後続へ回した機能と、ローカル OSS だけで完結する Medallion Architecture 型の統合データ基盤です。

Work table v1 では、ClickHouse の `hh_t_<tenant>_work` 配下に作成された table を UI で確認し、管理レコード化し、作成元 Dataset と紐付け、rename / truncate / drop、正式 Dataset 化、CSV export まで扱えるようにしました。一方で、次の機能は v1 の複雑さを抑えるために対象外にしています。

- Parquet / JSON export
- scheduled export
- Dataset 化後の再同期 / 差分同期
- Work table の完全な lineage / dependency graph
- 大容量 export の streaming 最適化
- export retention / cleanup policy の詳細化
- Bronze / Silver / Gold の catalog と pipeline run 管理
- 画像 / PDF など非構造データから Silver artifact を作る統合 pipeline
- Gold table / data mart として明示 publish する workflow

この文書では、これらを実装順、設計方針、DB / API / UI / worker への影響、完了条件まで分解します。クラウド managed service は採用せず、PostgreSQL、ClickHouse、Drive / file storage、outbox worker、OCR / 商品抽出、必要に応じたローカル OSS の処理 runtime だけで閉じる方針にします。

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
- Drive / file storage は local filesystem または SeaweedFS S3-compatible driver を使える
- Drive OCR / 商品情報抽出は outbox worker と local runtime で非同期処理できる
- ClickHouse は tenant ごとに `hh_t_<tenant>_raw` / `hh_t_<tenant>_work` を使える

この文書では、Work table を正式 Dataset と同じ正本として扱うのではなく、引き続き `hh_t_<tenant>_work` 配下の作業成果物として扱います。正式 Dataset 化したものだけが raw DB 側の独立 Dataset になります。

Medallion Architecture では、既存の構成を次の論理層として扱います。

| Layer | 現在の HaoHao で近いもの | 役割 |
| --- | --- | --- |
| Bronze | Drive file、`file_objects`、local / SeaweedFS file storage、CSV raw table | 元ファイルと取り込み直後の raw data を保持する |
| Silver | OCR run、OCR page、商品抽出 item、将来の画像 / PDF 抽出 artifact | 非構造データや raw data を検索・分析可能な中間構造へ変換する |
| Gold | ClickHouse の publish 済み table、正式 Dataset、data mart | 業務分析や集計に使える curated data を提供する |

`hh_t_<tenant>_work` は引き続き分析者の作業領域として扱います。Medallion の Gold は、Work table から明示 publish された table / Dataset だけを対象にします。

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
| Medallion foundation / catalog | 未実装 | Bronze / Silver / Gold の asset、artifact、publish 状態、pipeline run を管理する |
| Silver pipeline for unstructured data | OCR / 商品抽出は個別実装済み | 画像 / PDF を中心に、Drive OCR と商品抽出を Silver pipeline として統合する |
| Gold publication / data mart | Work table / Dataset 化はある | Work table から Gold table / Gold Dataset へ明示 publish する |
| Local vector / search layer | Drive search と OCR text index はある | PostgreSQL full-text を優先し、必要に応じて pgvector / Qdrant を local OSS 候補にする |
| Video / audio pipeline | 未実装 | FFmpeg、whisper.cpp、local Whisper runtime を後続候補として扱う |

## 優先順位

実装順は次を基本にします。

| 優先 | Phase | 理由 |
| --- | --- | --- |
| 0 | CSV export 安定化 | v1 の基盤が不安定なまま format や schedule を増やさない |
| 1 | Export format 拡張 | 既存 export API / worker / file storage を自然に拡張できる |
| 2 | Scheduled export | 単発 export の成功 / 失敗 / download / cleanup が固まってから定期実行にする |
| 3 | Dataset 化の再同期 / 差分同期 | データ破壊や二重取り込みのリスクが高いため、同期方式を明確にしてから入れる |
| 4 | Lineage / dependency graph | SQL 解析と可視化が絡むため、基礎 metadata が蓄積されてから入れる |
| 5 | Medallion foundation / catalog | 既存 raw / work / Drive / OCR を壊さず、Bronze / Silver / Gold の共通語彙を固定する |
| 6 | Silver image / PDF pipeline v1 | 既存 OCR / 商品抽出を統合し、非構造データを分析可能な中間成果物にする |
| 7 | Gold publication / data mart | Work table の成果物を業務利用できる curated data として明示公開する |
| 8 | Local vector / search layer | Silver artifact を検索・RAG へ拡張する前に local OSS の境界を固定する |
| 9 | Video / audio pipeline | 画像 / PDF の pipeline が安定した後に重い media 処理を追加する |

短期の実装順は、`CSV export 安定化 -> format 拡張 -> scheduled export -> resync -> lineage` とします。その後、`Medallion foundation -> Silver image/PDF pipeline -> Gold publication -> local vector/search -> video/audio` の順で進めます。

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

## Phase 5: Medallion foundation / catalog

### 目的

既存の Drive、file storage、Dataset、ClickHouse raw / work DB、OCR / 商品抽出を、Bronze / Silver / Gold の共通モデルで説明・管理できるようにします。

この Phase では、大きな data copy や既存 table の破壊的移行は行いません。まず catalog と pipeline run の語彙を固定し、後続 Phase が同じ状態管理を使えるようにします。

### Medallion layer 方針

| Layer | 対象 | 方針 |
| --- | --- | --- |
| Bronze | Drive file、`file_objects`、CSV raw table | 元ファイルと取り込み直後の raw data を正本として保持する |
| Silver | OCR text、OCR page、商品抽出 item、画像 / PDF 抽出 artifact | 非構造データや raw table を分析可能な中間構造へ変換する |
| Gold | publish 済み ClickHouse table、Gold Dataset、data mart | BI / SQL 集計 / 業務利用に出せる curated data として扱う |

既存の `hh_t_<tenant>_raw` / `hh_t_<tenant>_work` は壊しません。必要になった時点で `hh_t_<tenant>_silver` / `hh_t_<tenant>_gold` を追加できるようにしますが、Phase 5 の初期実装では論理 layer と catalog を優先します。

`hh_t_<tenant>_work` は Gold ではなく、分析者の作業領域です。Gold に入るのは、publish workflow を通った table / Dataset だけです。

### 主な変更

- Bronze / Silver / Gold asset を表す catalog table を追加する
- pipeline run の status、source、target、runtime、started / completed、error summary を保存する
- Dataset、Work table、Drive file、OCR run、product extraction item を catalog に紐付けられるようにする
- tenant ごとの layer boundary と source file 権限を維持する
- UI は最初から graph editor にせず、Dataset / Drive detail に layer badge と pipeline history を表示する

### Catalog 方針

Catalog は storage の正本ではなく、既存 resource を横断して追跡する管理台帳です。file body は file storage、Drive metadata は PostgreSQL、raw / work data は ClickHouse、OCR 結果は既存 OCR table を正本にします。

Catalog には少なくとも次を持たせます。

- layer: `bronze`, `silver`, `gold`
- resource kind: `drive_file`, `dataset`, `work_table`, `ocr_run`, `product_extraction`, `gold_table`
- tenant、source resource、target resource
- status: `active`, `building`, `failed`, `archived`
- schema / column summary、row count、byte size は取得できる範囲で保持する
- created by / updated by、created at / updated at

### 完了条件

- Drive CSV、画像、PDF が Bronze asset として catalog で追跡される
- 既存 Dataset と Work table が layer 付きで表示できる
- OCR run / product extraction が Silver artifact として source file に紐付く
- pipeline run の成功 / 失敗 / retry 対象が DB で追える
- 既存 raw / work table を破壊せずに導入できる
- `go test ./backend/...` と `cd frontend && npm run build` が通る

## Phase 6: Silver image / PDF pipeline v1

### 目的

Drive に保存された画像 / PDF を、OCR text、page layout、商品・販促情報などの分析可能な Silver artifact に変換します。

既存の Drive OCR / 商品抽出を Silver pipeline の初期実装として位置づけます。動画 / 音声はこの Phase では扱わず、Phase 9 に回します。

### 対象 runtime

初期実装で許可する runtime は、ローカルで実行できる OSS に限定します。

| 用途 | 優先候補 | 備考 |
| --- | --- | --- |
| PDF text extraction | Poppler `pdftotext` | born-digital PDF を優先処理する |
| OCR | Tesseract | 日本語は `jpn` / `jpn_vert` traineddata を運用側で配置する |
| 画像前処理 | ImageMagick | 必要な場合だけ resize / normalize に使う |
| 構造化抽出 | rules、Python helper、GiNZA、SudachiPy | 外部 API に送らず local process として実行する |
| local LLM 抽出 | Ollama と OSS model | tenant policy で明示有効化し、local URL だけを許可する |

### 主な変更

- Drive upload 後の同期処理ではなく、outbox worker で pipeline run を実行する
- OCR / 商品抽出の結果を Silver artifact として catalog に登録する
- OCR text、page、layout、boxes、商品抽出 item を source file の権限で保護する
- unsupported media、DLP blocked、infected / blocked、zero-knowledge E2EE file は skipped として記録する
- Silver artifact を ClickHouse に投影する場合は、source OCR run と schema version を保存する

### API / UI

Browser API は既存 Drive OCR / product extraction endpoint を維持し、Medallion catalog の読み取り endpoint を追加します。UI は Drive detail に Bronze / Silver 状態、最新 pipeline run、抽出 item、再実行 action を表示します。

Pipeline 再実行は file revision / content hash / engine / extractor を見て冪等に扱います。設定変更で再実行する場合は、新しい pipeline run として履歴を残します。

### 完了条件

- PDF / PNG / JPEG / TIFF / WebP が Silver pipeline の対象になる
- OCR / 商品抽出結果が Silver artifact として catalog に登録される
- Drive detail から pipeline status、page text、抽出 item が確認できる
- source file の閲覧権限なしに Silver artifact を読めない
- 失敗 / skipped / dependency unavailable が安全な error summary として表示される
- `go test ./backend/...` と `cd frontend && npm run build` が通る

## Phase 7: Gold publication / data mart

### 目的

Work table や Silver artifact を、業務分析に使える Gold table / Gold Dataset / data mart として明示 publish できるようにします。

Gold は、単に `hh_t_<tenant>_work` に存在する table ではありません。source、pipeline run、schema、row count、更新時刻、公開状態を追跡し、利用者に「分析に使ってよい」と示せる curated data だけを Gold と呼びます。

### 主な変更

- Work table から Gold table / Gold Dataset を publish する request API を追加する
- Gold catalog に source Work table、source Dataset、pipeline run、schema version、row count、published by、published at を保存する
- Gold table は ClickHouse 上で SQL 集計できる形にする
- Gold Dataset detail に source lineage、schema、last publish status、refresh policy を表示する
- publish / unpublish / archive は audit と active tenant role を必須にする

### Gold table 方針

最初は full publish を優先します。既存 Gold table を直接 truncate せず、新しい table を作成して検証後に publish pointer を切り替える方式を検討します。失敗時は既存 Gold を読み続けられる状態にします。

Data mart は Gold table のうち、特定用途向けに表示名、owner、説明、主要指標、更新頻度を持つものとして扱います。最初から外部 BI connector は実装しません。

### 完了条件

- managed Work table から Gold table / Gold Dataset を publish できる
- Gold は ClickHouse SQL で集計できる
- Gold detail で source、schema、row count、publish history が分かる
- publish 失敗時に既存 Gold が壊れない
- unpublish / archive 後の download / query 可否が policy に従う
- `go test ./backend/...` と `cd frontend && npm run build` が通る

## Phase 8: Local vector / search layer

### 目的

Silver artifact と Gold Dataset を、ローカル完結の検索 / 類似検索 / RAG 用途へ広げられるようにします。

初期実装では、既存の Drive search と PostgreSQL full-text を優先します。ベクトル検索が必要になった場合だけ、local OSS の `pgvector` または Qdrant を候補にします。

### 設計方針

- 外部 embedding API やクラウド AI 検索は使わない
- source file の権限を持たないユーザーに OCR text、embedding、検索結果を返さない
- embedding は source text / model / dimension / content hash / created at を保存する
- model は local runtime で明示設定し、tenant policy で enable する
- search index の再作成は outbox / background job で扱う

### 候補 runtime

| 用途 | 候補 | 採用タイミング |
| --- | --- | --- |
| full-text | PostgreSQL full-text、既存 Drive search | 初期実装で優先 |
| vector in Postgres | `pgvector` | PostgreSQL 内で運用を閉じたい場合 |
| vector DB | Qdrant | 大きい embedding index や filter search が必要になった場合 |
| local embedding | Ollama など OSS local model runtime | tenant policy と model 配置が固まった後 |

### 完了条件

- OCR text / Silver artifact が既存 search で見つかる
- vector feature は disabled by default で導入できる
- local embedding runtime が未設定でも通常検索が壊れない
- source file 権限と tenant boundary が検索結果に反映される
- index rebuild の進捗と失敗が追える
- `go test ./backend/...` と `cd frontend && npm run build` が通る

## Phase 9: Video / audio pipeline

### 目的

画像 / PDF の Silver pipeline が安定した後に、動画 / 音声を Bronze から Silver / Gold へ変換できるようにします。

この Phase は deferred follow-up です。初期の Medallion 実装には含めません。

### 対象候補

| Media | local OSS 候補 | Silver artifact |
| --- | --- | --- |
| 動画 | FFmpeg | metadata、thumbnail、scene frame、duration、codec、frame sample |
| 音声 | FFmpeg、whisper.cpp、local Whisper runtime | transcript、segment、speaker-like interval、language |
| 動画内音声 | FFmpeg + local Whisper | time-coded transcript |

### 設計方針

- file upload request 内で動画 / 音声解析を同期実行しない
- large media は tenant policy の max bytes / duration / timeout で制限する
- frame extraction、transcription、embedding は別 pipeline run として履歴を残す
- raw media body は Bronze として保持し、Silver artifact は source media の権限で保護する
- GPU 前提にはしない。GPU 対応は local runtime option として後続にする

### 完了条件

- video / audio は initial Medallion release の対象外として UI で過剰に露出しない
- 実装開始前に runtime dependency、timeout、storage footprint、tenant quota の設計が更新されている
- FFmpeg / whisper.cpp / local Whisper runtime の導入確認手順が runbook 化されている
- source media の権限なしに transcript / frame artifact を読めない

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
- Bronze / Silver / Gold の catalog は既存 resource の正本を置き換えない
- Silver artifact は source file の authorization と tenant boundary を必ず継承する
- Gold publish は明示 action と audit event を必須にする
- local runtime の未設定や依存不足は `failed` または `skipped` として記録し、同期 request を失敗させない
- S3-compatible storage は SeaweedFS など local OSS runtime に限定する

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
- AWS / Google Cloud / Azure などの cloud managed service への依存
- BigQuery / Snowflake / Databricks など外部 warehouse / lakehouse への publish
- AWS Rekognition / Google Vision AI など managed AI API への file body / OCR text 送信
- 外部 embedding API や cloud vector search への依存
- OSS ではない local desktop / model runtime を Medallion runtime として採用すること
- Video / audio pipeline を画像 / PDF の Silver pipeline より先に実装する

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

Medallion 関連のドキュメント更新では、次も確認します。

- 章構成が既存の Phase 0-4 と矛盾していない
- cloud managed service を採用対象として書いていない
- generated file を手書き編集する前提を書いていない
- local OSS runtime の dependency と fallback が明示されている

## 完了条件

この文書で扱う Dataset Work Table と Medallion 統合の後続機能全体の完了条件は次です。

- CSV export の v1 flow が安定している
- CSV / JSON / Parquet export を選択できる
- scheduled export を作成、無効化、履歴確認できる
- retention により古い export の download 可否が制御される
- Work table 由来 Dataset を full refresh で再同期できる
- 再同期失敗時に既存 Dataset が壊れない
- Dataset detail と Work table detail で主要 lineage が確認できる
- Lineage 初期版は parser に依存せず、保存済み metadata だけで動く
- Drive CSV、画像、PDF が Bronze asset として追跡される
- OCR / 抽出結果が Silver artifact として権限付きで参照できる
- Silver から Gold table / Dataset へ publish できる
- Gold は ClickHouse で SQL 集計できる
- tenant 境界、source file 権限、audit、outbox retry が維持される
- すべての runtime は local OSS だけで完結する
- `./scripts/gen.sh`、`go test ./backend/...`、`cd frontend && npm run build` が通る
