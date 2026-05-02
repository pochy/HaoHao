# データ依存グラフ / Lineage

## 目的

データ依存グラフは、Dataset、SQL 実行、Work table、export、sync などの関係を「どのデータから、どの成果物が作られたか」として追跡するための読み取り専用ビューです。

HaoHao では、データの出所確認、変更前の影響確認、失敗した処理の原因調査をしやすくするために使います。

## グラフの考え方

依存グラフの矢印は、常に元データから派生成果物へ向けます。

```text
Dataset
  -> dataset query job
  -> Work table
  -> promoted Dataset
  -> Work table export

Export schedule
  -> scheduled Work table export

Work table
  -> Dataset sync job
  -> synced Dataset
```

主な node は次の通りです。

- `dataset`: 正式 Dataset。
- `dataset_query_job`: SQL Studio などで実行された query job。
- `dataset_work_table`: query job から作られた managed Work table。
- `dataset_work_table_export`: Work table から作られた export file。
- `dataset_work_table_export_schedule`: Work table export の定期実行設定。
- `dataset_sync_job`: Work table 由来 Dataset の再同期 job。

主な edge は次の通りです。

- `query_input`: Dataset が query job の入力になった。
- `query_created_work_table`: query job が Work table を作った。
- `source_dataset`: Work table の元 Dataset。
- `promoted_dataset`: Work table が正式 Dataset に昇格された。
- `work_table_export`: Work table から export が作られた。
- `export_schedule`: Work table に export schedule が設定された。
- `scheduled_export_run`: schedule から export run が作られた。
- `dataset_sync_source`: Work table が Dataset sync の入力になった。
- `dataset_sync_target`: sync job が Dataset を更新した。

## 使い方

Dataset detail では、対象 Dataset の上流と下流を確認します。

- 上流: どの Work table や query job から作られた Dataset か。
- 下流: その Dataset を元に作られた Work table や sync job があるか。
- 変更前確認: Dataset schema や削除方針を変える前に、影響を受ける成果物を確認する。

Work table detail では、作業テーブルの作成元と成果物を確認します。

- 作成元: どの Dataset と query job から作られたか。
- 成果物: promoted Dataset、manual export、scheduled export、sync job があるか。
- 調査: export や sync が失敗したときに、どの Work table と schedule に紐づく処理かを追う。

UI では、重い graph editor ではなく、compact graph と timeline で表示します。巨大化を避けるため、v1 は深さ最大 2、history 最大 100 件を上限にします。

## v1 の範囲

v1 は既存 metadata を正本にした read-only lineage です。

- SQL parser は使いません。
- column-level lineage は扱いません。
- 手動で edge を追加・編集する graph editor は提供しません。
- lineage 専用の正本 table は持たず、既存 Dataset / Work table / export / schedule / sync metadata から組み立てます。
- tenant 境界と既存の認可条件を維持します。
- response には内部 numeric id、storage path、ClickHouse credential、raw SQL error detail を含めません。

将来、SQL parser による query input 推定、column-level lineage、手動注釈、より広い graph traversal が必要になった場合は、別 Phase として追加します。
