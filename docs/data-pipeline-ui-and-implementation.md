データクレンジング／前処理はかなり広いので、実務で使う分類に整理して「抜けなく網羅」します。
（単なる列挙ではなく、設計観点で整理しています）

---

# 全体像（まず俯瞰）

データ前処理は大きくこの6カテゴリに分かれます：

```text
① クリーニング（Cleaning）
② 正規化・標準化（Normalization / Standardization）
③ バリデーション（Validation）
④ スキーマ操作（Schema系）
⑤ エンリッチメント（Enrichment）
⑥ 変換・特徴量生成（Transformation / Feature Engineering）
```

---

# ① クリーニング（Cleaning）

「汚れたデータを整える」

### 主な処理

* 欠損値処理（NULL / NaN）

  * 削除
  * 補完（平均・中央値・前後補完・モデル補完）
* 重複削除（deduplication）
* 外れ値処理
* 不正値の除去
* 型の修正（文字列→数値など）
* 不正フォーマット修正
* 空白・改行・制御文字の除去
* ノイズ除去（例：ログのゴミデータ）

👉 一番ベースになる処理

---

# ② 正規化・標準化（Normalization / Standardization）

「表記やスケールを揃える」

### 主な処理

* 表記揺れ統一

  * 全角／半角
  * 大文字／小文字
  * カナ統一（カタカナ・ひらがな）
* 日付フォーマット統一
* 通貨・単位の統一（円・ドルなど）
* コード体系の統一（ISOコードなど）
* スケーリング

  * Min-Max正規化
  * 標準化（z-score）
* カテゴリ値のマッピング（例：male / M → 男）

👉 「同じ意味なのに違う値」を揃える

---

# ③ バリデーション（Validation）

「正しいデータかチェックする」

### 主な処理

* 型チェック（int, stringなど）
* 必須項目チェック
* 範囲チェック（例：年齢0〜120）
* 一意性チェック（ID重複）
* 外部キー整合性チェック
* 正規表現チェック（メール、電話番号）
* ビジネスルール検証

  * 売上 >= 0
  * 日付が未来すぎない
* データ分布チェック（異常検知）

👉 CIや監視にも使う重要な部分

---

# ④ スキーマ操作（Schema系）

ここが質問の中心部分

## 4-1. スキーマ補完（Schema Completion）

不足しているカラムを補う

* 欠けている列の追加
* デフォルト値の設定
* AIによる補完（例：カテゴリ推定）

---

## 4-2. スキーマ変換（Schema Transformation）

構造そのものを変える

* カラム名変更
* 型変換
* ネスト → フラット
* JSON → テーブル
* ワイド ↔ ロング変換
* カラム分割／結合

---

## 4-3. スキーママッピング（Schema Mapping）

別スキーマとの対応付け

* source → target の列対応
* 型の対応
* ビジネス意味の対応

例：

```text
price → unit_price
customer_id → user_id
```

---

## 4-4. スキーマ進化対応（Schema Evolution）

変更に耐える

* カラム追加
* 型変更
* 後方互換性維持
* late binding（ビューで吸収）

---

# ⑤ エンリッチメント（Enrichment）

「データに価値を追加する」

### 主な処理

* 外部データ結合

  * マスターデータ（ユーザー情報など）
* API連携

  * 住所→緯度経度
* カテゴリ推定（AI）
* キーワード生成（AI）
* 属性追加（例：年齢→年代）
* ログデータにメタ情報付与
* クロールデータ結合

👉 プロダクト価値に直結する領域

---

# ⑥ 変換・特徴量生成（Transformation / Feature Engineering）

### 主な処理

* 集約（group by）
* ウィンドウ関数
* 時系列特徴量

  * 過去7日売上
* フラグ生成

  * 購入有無
* バケット化（binning）
* one-hot encoding
* embedding生成（AI）
* テキスト特徴量
* 正規化された数値生成

👉 ML・分析・ダッシュボード用

---

# 実務でよく追加される重要カテゴリ

## ⑦ データ統合（Integration）

* 複数ソース統合
* JOIN / UNION
* フォーマット統一

---

## ⑧ データ削減（Reduction）

* サンプリング
* フィルタリング
* カラム削減

---

## ⑨ データ匿名化・マスキング

* 個人情報削除
* ハッシュ化
* トークナイズ

---

## ⑩ データ品質管理（Data Quality）

* 品質スコア
* 異常検知
* 監視
* SLA管理

---

# まとめ（完全一覧）

かなり網羅的にまとめると：

```text
■ Cleaning
欠損補完 / 重複削除 / 外れ値処理 / ノイズ除去

■ Normalization
表記統一 / 単位統一 / スケーリング

■ Validation
型チェック / 範囲 / 一意性 / ビジネスルール

■ Schema系
スキーマ補完
スキーマ変換
スキーママッピング
スキーマ進化

■ Enrichment
外部結合 / AI補完 / 属性追加

■ Transformation
集約 / 特徴量生成 / 時系列処理

■ その他
データ統合
データ削減
匿名化
品質管理
```

---


以下の2軸で設計するとよいです。

```text
ノーコードUI = ユーザーが処理を組み立てる場所
DuckDB / ClickHouse = 実際に処理を実行する場所
Parquet = 入出力の標準フォーマット
```

DuckDBはParquet / JSON / S3などを直接SQLで読める分析DBで、ローカル・軽量ジョブ・前処理に向いています。([DuckDB][1])
ClickHouseは大規模集計、Materialized View、MergeTree系エンジンによる高速分析に向いています。([ClickHouse][2])

---

# 1. ノーコードUIにどう落とすか

## 基本形

UIは「データ処理パイプラインビルダー」にします。

```text
Input
  ↓
Profile
  ↓
Clean
  ↓
Normalize
  ↓
Validate
  ↓
Schema Mapping / Completion
  ↓
Enrich
  ↓
Transform
  ↓
Output
```

ユーザーにはSQLを書かせず、各ステップをカード形式で追加させます。

---

# UIの主要ステップ

## ① Input

データソースを選ぶ画面です。

### UI項目

```text
入力タイプ
- CSV
- JSON
- JSONL
- Parquet
- Excel
- S3 / GCS / MinIO
- PostgreSQL
- MySQL
- ClickHouse
```

### 設定項目

```text
ファイルパス
文字コード
区切り文字
ヘッダー有無
日付フォーマット
サンプル行数
```

### 裏側の実装

DuckDBならこういうSQLになります。

```sql
SELECT *
FROM read_parquet('s3://bucket/path/*.parquet');
```

またはCSVなら、

```sql
SELECT *
FROM read_csv_auto('input.csv');
```

---

## ② Profile

最初にデータを自動分析するステップです。

### UIで見せる内容

```text
カラム一覧
型推定
NULL率
ユニーク数
最小値 / 最大値
平均 / 中央値
上位カテゴリ
サンプル値
異常っぽい値
```

### 目的

ユーザーが「このデータは何か」を理解できるようにします。

### 例

```text
price
- 推定型: number
- NULL率: 3.2%
- 最小: -100
- 最大: 999999
- 異常候補: -100
```

ここで「priceに負数がある」と分かるので、次のCleaningやValidationに進めます。

---

## ③ Cleaning

汚れたデータを修正するステップです。

### UI項目

```text
欠損値処理
- そのまま
- 行を削除
- 固定値で補完
- 平均 / 中央値 / 最頻値で補完
- 前方補完
- 後方補完

重複処理
- 完全一致で削除
- キー指定で削除
- 最新レコードを残す

外れ値処理
- 削除
- NULL化
- 上限 / 下限に丸める
```

### UI例

```text
Column: price
Rule: price < 0 を NULL にする
```

### DuckDB実装例

```sql
SELECT
  *,
  CASE
    WHEN price < 0 THEN NULL
    ELSE price
  END AS price_cleaned
FROM input;
```

---

## ④ Normalize / Standardize

表記・単位・フォーマットを揃えるステップです。

### UI項目

```text
文字列
- trim
- lowercase
- uppercase
- 全角半角変換
- 空白正規化
- 記号除去

日付
- YYYY-MM-DDに統一
- タイムゾーン変換

数値
- 単位変換
- 通貨変換
- 小数丸め

カテゴリ
- 値マッピング
```

### UI例

```text
"Male", "M", "male" → "male"
"Female", "F", "female" → "female"
```

### DuckDB実装例

```sql
SELECT
  lower(trim(gender)) AS gender_normalized
FROM input;
```

---

## ⑤ Validation

データがルールを満たしているか確認するステップです。

### UI項目

```text
必須チェック
型チェック
範囲チェック
正規表現チェック
一意性チェック
参照整合性チェック
ビジネスルール
```

### UI例

```text
email は必須
email はメール形式
price は 0 以上
product_id は一意
```

### 出力設計

Validationは2種類に分けるとよいです。

```text
Hard Error
- 処理を止める

Warning
- 処理は続けるが警告を出す
```

### DuckDB実装例

```sql
SELECT *
FROM input
WHERE price < 0 OR price IS NULL;
```

結果が1件以上あればエラー、という判定にできます。

---

## ⑥ Schema Mapping

入力データをターゲットスキーマに対応付けるステップです。

### UI項目

```text
Target Column
Source Column
変換関数
必須かどうか
型
デフォルト値
```

### UI例

```text
target: product_name ← source: name
target: unit_price   ← source: price
target: jan_code     ← source: barcode
```

### UIとして重要な機能

```text
自動マッピング候補
信頼度スコア
手動修正
未マッピング列の警告
必須列の不足表示
```

### DuckDB実装例

```sql
SELECT
  name AS product_name,
  price::DECIMAL(18,2) AS unit_price,
  barcode AS jan_code
FROM input;
```

---

## ⑦ Schema Completion

足りないカラムを補完するステップです。

### 補完パターン

```text
固定値で補完
ルールで補完
他カラムから生成
外部マスタから補完
AIで補完
```

### UI例

```text
target_category がない
→ product_name と description からAI推定する
```

### 設定項目

```text
補完対象カラム
参照する入力カラム
補完方法
信頼度しきい値
失敗時の扱い
```

### DuckDB実装例

ルール補完ならSQLでできます。

```sql
SELECT
  *,
  CASE
    WHEN product_name ILIKE '%coffee%' THEN 'coffee'
    WHEN product_name ILIKE '%tea%' THEN 'tea'
    ELSE 'unknown'
  END AS category
FROM input;
```

AI補完の場合は、DuckDBだけで完結させるより、Python / API / LLM処理を挟む方が自然です。

---

## ⑧ Enrichment

外部データを使って情報を追加するステップです。

### UI項目

```text
結合元
- マスターテーブル
- API
- クロールデータ
- LLM
- ベクトル検索

JOINキー
追加するカラム
一致しない場合の扱い
```

### UI例

```text
JANコードで商品マスタとJOINして、
brand / category / manufacturer を追加する
```

### DuckDB実装例

```sql
SELECT
  p.*,
  m.brand,
  m.category,
  m.manufacturer
FROM products p
LEFT JOIN master_products m
  ON p.jan_code = m.jan_code;
```

---

## ⑨ Transform

分析・出力用に形を変えるステップです。

### UI項目

```text
カラム追加
カラム削除
カラム結合
カラム分割
集約
ピボット
アンピボット
ソート
フィルタ
```

### UI例

```text
日別売上を作る
```

### DuckDB実装例

```sql
SELECT
  date_trunc('day', purchased_at) AS day,
  sum(amount) AS revenue
FROM input
GROUP BY 1;
```

---

## ⑩ Output

結果を書き出すステップです。

### UI項目

```text
出力形式
- Parquet
- CSV
- JSONL
- ClickHouse table
- PostgreSQL table

出力先
パーティション
上書き / 追記
圧縮形式
```

### DuckDB実装例

```sql
COPY (
  SELECT * FROM final_result
)
TO 'out/products_cleaned.parquet'
(FORMAT 'parquet');
```

---

# UI設計の重要ポイント

## 初心者向けは「推奨設定」を出す

最初から全部の設定を見せると難しくなります。

```text
Basic
- よく使う設定だけ

Advanced
- SQL式
- NULLの詳細扱い
- 型変換
- エラー時の挙動
```

---

## 各ステップにPreviewを付ける

ノーコードUIではPreviewが非常に重要です。

```text
Before
After
Diff
Error rows
Warning rows
```

例：

```text
Before: "  Coffee "
After:  "coffee"
```

---

## 各ステップは内部的にJSONとして保存する

UIはノーコードでも、内部表現はJSONにするとよいです。

```json
{
  "type": "normalize",
  "column": "product_name",
  "operations": ["trim", "lowercase"]
}
```

これをバックエンド側でSQLやPython処理に変換します。

---

# 2. DuckDB / ClickHouseでどう実装するか

## 役割分担

基本はこうです。

```text
DuckDB
- ファイル読み込み
- 前処理
- クレンジング
- スキーマ変換
- 小〜中規模JOIN
- Parquet出力
- ローカル / バッチ処理

ClickHouse
- 大規模データ分析
- 高速集計
- ダッシュボード
- ロールアップ
- 重複排除
- 長期分析テーブル
```

---

# 推奨アーキテクチャ

```text
[User Upload / S3]
        ↓
[DuckDB Preprocessing Job]
        ↓
[Cleaned Parquet]
        ↓
[ClickHouse]
        ↓
[Dashboard / API / Export]
```

または小規模なら、

```text
[Parquet]
   ↓
[DuckDB]
   ↓
[Parquet / CSV]
```

だけでも十分です。

---

# DuckDBで実装する処理

## ① Profiling

```sql
SELECT
  count(*) AS total_rows,
  count(*) FILTER (WHERE price IS NULL) AS null_price,
  min(price) AS min_price,
  max(price) AS max_price,
  avg(price) AS avg_price
FROM read_parquet('input/*.parquet');
```

---

## ② Cleaning

```sql
CREATE OR REPLACE TABLE cleaned AS
SELECT
  product_id,
  trim(product_name) AS product_name,
  CASE
    WHEN price < 0 THEN NULL
    ELSE price
  END AS price,
  purchased_at
FROM read_parquet('input/*.parquet');
```

---

## ③ Normalization

```sql
CREATE OR REPLACE TABLE normalized AS
SELECT
  *,
  lower(trim(category)) AS category_normalized,
  try_cast(price AS DECIMAL(18,2)) AS price_decimal
FROM cleaned;
```

---

## ④ Validation

エラー行を別ファイルに出す設計が便利です。

```sql
COPY (
  SELECT *
  FROM normalized
  WHERE price_decimal IS NULL
     OR price_decimal < 0
)
TO 'out/error_rows.parquet'
(FORMAT 'parquet');
```

正常データだけ出す。

```sql
COPY (
  SELECT *
  FROM normalized
  WHERE price_decimal IS NOT NULL
    AND price_decimal >= 0
)
TO 'out/valid_rows.parquet'
(FORMAT 'parquet');
```

---

## ⑤ Schema Mapping

```sql
COPY (
  SELECT
    product_id::VARCHAR AS id,
    product_name AS name,
    price_decimal AS unit_price,
    category_normalized AS category
  FROM normalized
)
TO 'out/mapped_products.parquet'
(FORMAT 'parquet');
```

---

## ⑥ Schema Completion

```sql
COPY (
  SELECT
    *,
    CASE
      WHEN category IS NULL AND name ILIKE '%coffee%' THEN 'coffee'
      WHEN category IS NULL AND name ILIKE '%tea%' THEN 'tea'
      ELSE category
    END AS completed_category
  FROM read_parquet('out/mapped_products.parquet')
)
TO 'out/completed_products.parquet'
(FORMAT 'parquet');
```

---

## ⑦ Enrichment

```sql
COPY (
  SELECT
    p.*,
    m.brand,
    m.manufacturer
  FROM read_parquet('out/completed_products.parquet') p
  LEFT JOIN read_parquet('master/products.parquet') m
    ON p.id = m.product_id
)
TO 'out/enriched_products.parquet'
(FORMAT 'parquet');
```

---

# ClickHouseで実装する処理

## ① 生データ投入先

```sql
CREATE TABLE products_raw
(
  id String,
  name String,
  unit_price Decimal(18,2),
  category String,
  brand String,
  updated_at DateTime
)
ENGINE = MergeTree
ORDER BY (id, updated_at);
```

---

## ② Parquetから投入

```sql
INSERT INTO products_raw
SELECT *
FROM s3(
  'https://bucket.s3.amazonaws.com/out/enriched_products/*.parquet',
  'AWS_KEY',
  'AWS_SECRET',
  'Parquet'
);
```

---

## ③ 重複排除したい場合

`ReplacingMergeTree` はバックグラウンドマージで重複を整理する仕組みです。ただし、ClickHouse公式ドキュメントでは、重複排除はマージ時に起きるため、即時に重複が消える保証はないと説明されています。([ClickHouse][3])

```sql
CREATE TABLE products_latest
(
  id String,
  name String,
  unit_price Decimal(18,2),
  category String,
  brand String,
  version UInt64,
  updated_at DateTime
)
ENGINE = ReplacingMergeTree(version)
ORDER BY id;
```

```sql
INSERT INTO products_latest
SELECT
  id,
  name,
  unit_price,
  category,
  brand,
  toUnixTimestamp(updated_at) AS version,
  updated_at
FROM products_raw;
```

厳密に最新だけ見たいクエリでは、以下のように集約する方が安全です。

```sql
SELECT
  id,
  argMax(name, version) AS name,
  argMax(unit_price, version) AS unit_price,
  argMax(category, version) AS category,
  argMax(brand, version) AS brand
FROM products_latest
GROUP BY id;
```

---

## ④ 集計テーブル

`AggregatingMergeTree` は、同じソートキーの集計状態をマージでき、行数を大きく減らせるケースに向いています。([ClickHouse][2])

```sql
CREATE TABLE sales_by_day
(
  day Date,
  revenue AggregateFunction(sum, Decimal(18,2)),
  orders AggregateFunction(count)
)
ENGINE = AggregatingMergeTree()
ORDER BY day;
```

```sql
CREATE MATERIALIZED VIEW mv_sales_by_day
TO sales_by_day AS
SELECT
  toDate(updated_at) AS day,
  sumState(unit_price) AS revenue,
  countState() AS orders
FROM products_raw
GROUP BY day;
```

読むとき。

```sql
SELECT
  day,
  sumMerge(revenue) AS revenue,
  countMerge(orders) AS orders
FROM sales_by_day
GROUP BY day
ORDER BY day;
```

---

# 実装の全体フロー

## UIで作った設定

```json
{
  "steps": [
    {
      "type": "input",
      "format": "parquet",
      "path": "s3://bucket/raw/products/*.parquet"
    },
    {
      "type": "normalize",
      "columns": {
        "name": ["trim"],
        "category": ["trim", "lowercase"]
      }
    },
    {
      "type": "validate",
      "rules": [
        {
          "column": "unit_price",
          "operator": ">=",
          "value": 0,
          "severity": "error"
        }
      ]
    },
    {
      "type": "schema_mapping",
      "mappings": {
        "id": "product_id",
        "name": "product_name",
        "unit_price": "price"
      }
    },
    {
      "type": "output",
      "format": "parquet",
      "path": "s3://bucket/clean/products/"
    }
  ]
}
```

---

## バックエンドでSQL生成

上記JSONを受け取り、DuckDB SQLに変換します。

```sql
COPY (
  SELECT
    product_id AS id,
    trim(product_name) AS name,
    price::DECIMAL(18,2) AS unit_price
  FROM read_parquet('s3://bucket/raw/products/*.parquet')
  WHERE price >= 0
)
TO 's3://bucket/clean/products/'
(FORMAT 'parquet');
```

---

# どの処理をどちらでやるべきか

| 処理                     | DuckDB | ClickHouse |
| ------------------------ | -----: | ---------: |
| ファイル読み込み         |      ◎ |          ○ |
| CSV/JSON/Parquet前処理   |      ◎ |          △ |
| クレンジング             |      ◎ |          ○ |
| スキーマ変換             |      ◎ |          ○ |
| スキーマ補完             |      ◎ |          △ |
| AIエンリッチメント前処理 |      ◎ |          △ |
| 大規模JOIN               |      ○ |          ◎ |
| 高速集計                 |      ○ |          ◎ |
| ダッシュボード           |      △ |          ◎ |
| 継続的ロールアップ       |      △ |          ◎ |
| CIデータ品質テスト       |      ◎ |          ○ |

---

# おすすめ構成

最初のMVPならこれで十分です。

```text
Frontend
- Pipeline Builder
- Data Preview
- Validation Result
- Step Config UI

Backend
- Pipeline JSON保存
- DuckDB SQL生成
- Job実行
- Parquet出力

Storage
- MinIO / S3
- Parquet

Analytics
- 必要になったらClickHouse
```

最初からClickHouse前提にしすぎず、

```text
DuckDB + Parquetで前処理
↓
必要な集計だけClickHouseへ
```

が一番現実的です。

[1]: https://duckdb.org/?utm_source=chatgpt.com "DuckDB – An in-process SQL OLAP database management ..."
[2]: https://clickhouse.com/docs/engines/table-engines/mergetree-family/aggregatingmergetree?utm_source=chatgpt.com "AggregatingMergeTree table engine | ClickHouse Docs"
[3]: https://clickhouse.com/docs/engines/table-engines/mergetree-family/replacingmergetree?utm_source=chatgpt.com "ReplacingMergeTree table engine | ClickHouse Docs"
