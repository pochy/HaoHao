**ローカル開発 → 段階的導入の推奨順番（2026年時点）**
あなたの要件（メタデータフィルタ、日付・部門・タグ絞り込み、ACL、ハイブリッド検索、マルチテナント）を考慮した現実的なパスをまとめました。

### 推奨導入順（最もスムーズな流れ）
1. **Chroma（ローカル開発・PoC段階）**
   - **理由**: 最も簡単でセットアップが速い。Embeddedモード（Pythonプロセス内で動く）でDocker不要、ノートブックやローカルアプリで即開始可能。
   - 開発速度が最優先。メタデータフィルタは基本的に使え、フィルタ/検索ロジックを早い段階でテストできる。
   - 数万〜数十万ベクトルまで快適。
   - **移行しやすさ**: LangChain/LlamaIndexなどの抽象化レイヤーを使えば、後でDBを切り替えやすい。

2. **pgvector（ステージング / 小〜中規模本番）**
   - **理由**: Postgresをすでに使っている（またはこれから使う）場合に最適。ローカルでもDockerのpgvectorイメージで簡単に動く。
   - メタデータフィルタ・ACL（RLS）・SQL JOINが非常に強い。あなたの4要件にマッチしやすい。
   - 同一トランザクションで業務データとベクトルを管理可能。
   - **目安**: 〜500万ベクトル（快適ゾーン）。拡張（pgvectorscaleなど）でさらに伸ばせる。
   - Chromaからの移行も比較的スムーズ（特に抽象化ライブラリ使用時）。

3. **Qdrant（中規模〜大規模本番、または高性能が必要になった時）**
   - **理由**: 専用DBとしてフィルタリング性能が高く、ハイブリッド検索も強い。自前運用（Docker/K8s）やCloud版あり。
   - ローカルではDockerで本番に近い環境を再現しやすい（開発・本番のパリティが高い）。
   - マルチテナントや複雑フィルタがスケールしても安定。
   - Kubernetes運用を検討している場合に強い。

4. **オプション: Weaviate or Pinecone（さらにスケール/運用ゼロが必要時）**
   - Weaviate: ハイブリッド検索（BM25+Vector）が最優先の場合。
   - Pinecone: 完全マネージドで運用を最小化したいSaaS向け（コストは高め）。

### なぜこの順番が良いか？
- **Chroma → pgvector/Qdrant** の流れは、多くの比較記事で推奨されている「プロトタイピング → 実用化」の定番。Chromaでアイデア検証 → pgvectorで本格化（インフラ追加最小限）→ 必要に応じて専用DBへ。
- 早期に**抽象化ライブラリ**（LangChain, LlamaIndex, Haystackなど）を使うと、DB切り替え時のコード変更を最小限に抑えられる。
- 移行コストは意外とかかる（再インデックス、フィルタロジックの調整など）ので、最初から「将来的にスケールしそうな要件」を意識して設計を。

### 実践的なTips
- **ローカル環境**: Chroma（embedded） or pgvector（Docker） or Qdrant（Docker）で並行テスト可能。
- **データ移行**: ツール（MINGなど）やスクリプトでベクトル + メタデータをエクスポート/インポート。dual-write（並行書き込み）でゼロダウンタイムも検討。
- **テスト規模**: まずは実データの10-20%で各DBを試して、フィルタ性能・ACL挙動・ハイブリッド精度を比較。
- Postgres既存の場合: 最初からpgvectorスタートでもOK（ChromaはPoCのみ）。

あなたの状況をもう少し教えてください：
- Postgresはすでに使っているか？
- 想定ベクトル数（最終的にどれくらい？）
- チームの運用負荷（K8s運用できるか？マネージド希望か？）
- 主な言語（Python？）

これでより最適な順番・組み合わせを提案できます！




---



**Chroma → pgvector/Qdrant の流れに置き換えた場合の設計イメージ（あなたのデータ構造化パイプライン向け）**

元のRedis中心設計（PostgreSQL + Redis Search Vector Index + S3/MinIO）を尊重しつつ、**ローカル開発から段階的にスケール**する観点で置き換え・移行したバージョンをまとめます。このユースケース（カラム名+サンプル値+周辺コンテキストの意味検索、メタデータフィルタ重視、過去マッピング再利用）は**メタデータフィルタとSQL統合が強い**ので、pgvectorとの相性が非常に良いです。

### 推奨段階的導入順（Chroma → pgvector/Qdrant）

| 段階 | 推奨DB | 主な役割 | Redis設計との対応 | メリット・移行ポイント |
|------|--------|----------|------------------|------------------------|
| **1. ローカル開発 / PoC / MVP** | **Chroma** (Embeddedモード) | 標準スキーマ項目、過去マッピング例、入力カラムのembedding保存・検索 | Redis Vector Indexの代替 | インストール1行（pip install chromadb）、Docker不要。メタデータフィルタ（domain, schema_type, file_typeなど）が即使用可能。UIプロトタイプを最速で作れる。数万件まで快適。 |
| **2. ステージング / 小〜中規模本番** | **pgvector** (Postgres拡張) | 本格的な検索 + 正式データ連携 | Redis + PostgreSQLの統合版 | あなたの設計の「PostgreSQLに正式データ（mapping履歴、schema定義など）」を活かせる。**同一DB内でベクトル + 業務データ + RLS（ACL）**が可能。メタデータフィルタはSQLで最強（JOIN含む）。 |
| **3. 中〜大規模 / 高性能本番** | **Qdrant** (Self-host or Cloud) | 高負荷フィルタ検索 + ハイブリッド | Redis Searchのスケール版 | 複雑payloadフィルタ + hybrid（dense + sparse）が強い。マルチテナントもpayload/partitionで対応。K8s運用向き。 |

### 各段階での具体的な設計変更イメージ

**1. Chroma（ローカルMVP）**
- コレクションに `text`（カラム名 + サンプル値 + 周辺カラム + sheet_nameなど）、`embedding`、メタデータ（`domain`, `schema_type`, `file_type`, `language` など）を保存。
- フィルタ例: `where={"domain": "retail", "schema_type": "product"}`
- 過去マッピング例も同じコレクションに保存（`accepted_by_user: true` などのメタデータでフィルタ）。
- LangChain/LlamaIndexを使えば、後続のpgvector/Qdrantへの移行がコード変更最小限。
- **BGE-M3**などの多言語モデルはそのまま使用可能。

**2. pgvector（本番移行のメイン推奨）**
- Postgresの同じテーブル/スキーマにベクトル列を追加（または専用テーブル）。
- **強み発揮ポイント**:
  - メタデータフィルタ → SQL `WHERE` + `JOIN`（例: schema定義テーブルとJOINして説明文も参照）。
  - ACL → **Row-Level Security (RLS)** でtenant/domainごとの自動制御（アプリ側漏れ防止）。
  - 過去マッピング再利用 → 履歴テーブルとベクトルを同一トランザクションで管理。
  - ハイブリッド → `tsvector` + pg_trgm や ParadeDB拡張でBM25併用可能。
- HNSWインデックスで数百万件規模まで実用的（あなたのケースはスキーマ項目数自体がそれほど多くないはず）。
- Redisの`FT.SEARCH`相当はSQLで自然に書ける。

**3. Qdrant（さらにスケール時）**
- Payload filteringが優秀（indexedで高速pre-filter）。
- Hybrid search（dense + sparse/BM25）がネイティブで強い → カラム名表記ゆれ + 厳密キーワードを組み合わせやすい。
- マルチテナント: payload + partitionで効率的。
- Redisからの移行: ベクトル + payloadをエクスポート/インポート。

### 全体アーキテクチャの置き換えイメージ
- **PostgreSQL**: 正式データ（schema定義、mapping履歴、pipeline定義、採用ログ） ← **変更なし（むしろ強化）**
- **Vector Store**: Chroma（dev） → pgvector（本番） or Qdrant（高負荷時） ← **Redis Searchの置き換え**
- **S3/MinIO**: 元ファイル・抽出結果 ← **変更なし**
- **Embedding**: BGE-M3など ← **そのまま**
- **検索フロー**: 入力カラム → embedding → フィルタ付き類似検索 → top-k候補 → UI/LLM rerank → 採用

**移行のポイント**
- 抽象化ライブラリ（LangChain, LlamaIndex, Haystack）を使うとDBスイッチが楽。
- 最初は**dual write**（Chroma/pgvector両方に書き込み）で並行テスト。
- Embedding model変更時は全再index必須（Redisと同じ注意点）。

### この流れのメリット（Redis設計比）
- **開発速度**: Chromaで最速スタート。
- **運用簡素化**: pgvectorならRedisを追加せず、Postgresだけで完結（監視・バックアップ統一）。
- **要件適合**: メタデータフィルタ・ACL・複雑クエリでpgvectorがRedis Searchより優位な部分が多い。
- **スケーラビリティ**: 必要に応じてQdrantへシームレスに移行可能。

**Redisを残す選択肢**もあり（キャッシュや超高速semantic cacheとして併用）。ただ、**Chroma→pgvector**だけであなたのMVP（Schema Mapping Candidate Search）は十分実用レベルになります。

あなたの環境（Postgresはすでに使っている？想定データ量は？K8s運用予定？）を教えていただければ、さらに細かく調整した構成を提案します！



---



はい。データ構造化パイプライン文脈だと、ベクトル検索は **RAG** よりもむしろ、まずは **スキーマ推定・カラム対応付け・過去の処理例の再利用** に効きます。

結論から言うと、Redisでベクトル検索を使うなら、最初の実用ポイントはこのあたりです。

```txt
1. 新しいExcel/PDFのカラムを、既存の標準スキーマにマッピングする
2. 過去に似たファイル・似たシート・似たカラム構造を探す
3. 商品名・説明文・カテゴリなどから類似商品を探す
4. LLMに渡す前に、関連する過去の変換例・ルール・マッピング例を検索する
5. エンリッチメントやスキーマ補完の候補を絞り込む
```

---

## まず、ベクトル検索とは何か

通常の検索は、文字列が一致するかを見ます。

```txt
検索: "商品名"
ヒット: "商品名", "商品名称", "商品名_日本語"
```

一方、ベクトル検索は「意味が近いか」を見ます。

```txt
検索: "商品名"
ヒット:
- product_name
- item_name
- title
- 品名
- 商品名称
- 商品タイトル
```

つまり、**表記ゆれ・言語差・言い換えに強い検索**です。

データ構造化では、これがかなり重要です。なぜなら、ExcelやCSVのカラム名は会社ごとにバラバラだからです。

```txt
JAN
JANコード
商品コード
バーコード
GTIN
Product Code
```

これらを「標準スキーマのどの項目に近いか？」で探せるようになります。

---

## Redisでのベクトル検索の2つの選択肢

Redisには大きく2パターンあります。

### 1. Redis Search の Vector Index

こちらが実務向きです。

Redisはベクトルとメタデータを Hash または JSON に保存し、それに対して secondary index を作れます。Redis Search のベクトル検索では、テキスト・数値・地理情報・タグなどのメタデータフィルタと組み合わせた検索もできます。([Redis][1])

たとえば、

```txt
domain = retail
language = ja
schema_type = product
```

で絞り込んだうえで、意味的に近いスキーマ項目を検索できます。

### 2. Redis Vector Sets

Redis 8以降には Vector Sets というデータ型もあります。`VADD` でベクトル付き要素を追加し、`VSIM` で類似検索できます。Vector Sets はセットアップが軽く、プロトタイプやシンプルな類似検索に向いています。一方で、複雑なフィルタや全文検索との組み合わせが必要なら Redis Search の方が向いています。([Redis][2])

私なら、データ構造化パイプラインではまず **Redis Search の Vector Index** を選びます。

---

## データ構造化パイプラインで一番使えそうな用途

一番有効なのは、**カラムマッピング候補の検索**です。

たとえば、標準スキーマがこうだとします。

```json
{
  "target_column": "product_name",
  "description": "商品の正式名称。ECサイトやカタログに表示される商品名。",
  "aliases": ["商品名", "品名", "商品名称", "title", "item_name"]
}
```

新しいExcelにこういうカラムが来たとします。

```txt
品目名称
```

通常の文字列一致では `product_name` とマッチしにくいですが、ベクトル検索なら「意味が近い」と判断できます。

### 検索対象にするテキスト

重要なのは、**カラム名だけをベクトル化しないこと**です。

カラム名だけだと情報が少なすぎます。

```txt
悪い例:
"品目名称"
```

より良いのは、周辺情報も混ぜることです。

```txt
良い例:
Column name: 品目名称
Sheet name: 商品一覧
Sample values: カップラーメン, 緑茶 500ml, チョコレート
Neighbor columns: JANコード, メーカー名, 規格, 入数
File type: product master
```

これを embedding して検索します。

そうすると、単に「名前っぽい」だけでなく、**商品マスタの中の商品名らしい**という文脈も入ります。

---

## 具体的な流れ

```txt
1. 標準スキーマ項目を embedding してRedisに登録
2. 過去のマッピング例も embedding してRedisに登録
3. 新しいファイルをアップロード
4. シート名、カラム名、サンプル値、周辺カラムを抽出
5. 各カラムを embedding
6. Redisで近い標準スキーマ項目を検索
7. 上位候補をUIに表示
8. ユーザーが採用・修正
9. 採用結果を次回検索用の学習データとして保存
```

UIとしては、こんな感じです。

```txt
入力カラム: 品目名称

候補:
1. product_name       score: 0.91
2. product_title      score: 0.87
3. display_name       score: 0.82
4. category_name      score: 0.61
```

これにより、完全自動ではなく **AI-assisted schema mapping** にできます。

---

## Redisのインデックス例

たとえば、BGE-M3 のような 1024次元の embedding を使う場合は、Redis Search のインデックスはこういうイメージです。

```redis
FT.CREATE idx:schema_columns
ON HASH
PREFIX 1 schema_column:
SCHEMA
  text TEXT
  domain TAG
  schema_type TAG
  column_id TAG
  embedding VECTOR HNSW 6
    TYPE FLOAT32
    DIM 1024
    DISTANCE_METRIC COSINE
```

Redisのベクトルフィールドでは、`TYPE`、`DIM`、`DISTANCE_METRIC` を指定します。`DIM` は embedding の次元数で、検索時のベクトルも同じ次元である必要があります。距離指標としては `L2`、`IP`、`COSINE` がサポートされています。([Redis][1])

検索はこういうイメージです。

```redis
FT.SEARCH idx:schema_columns
  '(@domain:{retail} @schema_type:{product})=>[KNN 5 @embedding $vec AS score]'
  PARAMS 2 vec <embedding_bytes>
  SORTBY score
  RETURN 4 column_id text domain score
  DIALECT 2
```

Redis Search の KNN クエリでは、メタデータで pre-filter してからベクトル検索できます。公式例でも、`(*)=>[KNN ...]` の `(*)` 部分がフィルタで、ここを条件に置き換えられます。([Redis][3])

---

## HNSW / FLAT の使い分け

Redisでは主に `FLAT` と `HNSW` を意識すると良いです。

### FLAT

全件を正確に見る方式です。

```txt
メリット:
- 精度が高い
- 小規模ならわかりやすい

デメリット:
- 件数が増えると遅い
```

Redis公式ドキュメントでも、`FLAT` は小さいデータセット、または検索レイテンシより完全な検索精度を重視する場合に向くと説明されています。([Redis][1])

### HNSW

近似最近傍検索です。

```txt
メリット:
- 大規模でも速い
- 実用上かなり使いやすい

デメリット:
- 近似なので、理論上は最適解を取り逃す可能性がある
- メモリ使用量やパラメータ調整が必要
```

Redis公式ドキュメントでは、HNSWは大規模データセットや検索性能・スケーラビリティを重視する場合に向くとされています。`M`、`EF_CONSTRUCTION`、`EF_RUNTIME` などで精度・メモリ・レイテンシのトレードオフを調整します。([Redis][1])

データ構造化パイプラインなら、最初はこうで良いと思います。

```txt
PoC / 小規模: FLAT
実運用 / 件数が増える: HNSW
```

---

## 何をベクトル化すべきか

データ構造化パイプラインでは、全部をベクトル化する必要はありません。

### 1. 標準スキーマ

これはかなり有効です。

```txt
target_column: product_name
description: 商品の正式名称
aliases: 商品名, 品名, 商品名称, item name, product title
examples: 明治ミルクチョコレート, 綾鷹 525ml
```

### 2. 入力カラム

これも有効です。

```txt
source_column: 品目名称
sheet_name: 商品一覧
sample_values: 緑茶, 缶コーヒー, チョコレート
neighbor_columns: JAN, メーカー, 価格
```

### 3. 過去のマッピング例

かなり重要です。

```txt
source_column: 品名
target_column: product_name
domain: retail
accepted_by_user: true
```

ユーザーが過去に採用したマッピングを検索できると、精度が上がります。

### 4. 変換ルール

これも使えます。

```txt
rule_name: price_normalization
description: "税込価格・税抜価格・円表記を数値に正規化する"
examples: "¥1,200" -> 1200
```

### 5. 商品説明・カテゴリ説明

エンリッチメントやカテゴリ推定に使えます。

```txt
商品説明: "辛口のカップ麺..."
カテゴリ: "食品 > 麺類 > インスタント麺"
```

---

## ベクトル検索に向いていないもの

ここはかなり重要です。

ベクトル検索は「意味の近さ」には強いですが、**正確な一致**には弱いです。

向いていないもの：

```txt
JANコード
SKU
型番
郵便番号
電話番号
日付
価格
数量
ID
```

たとえば、

```txt
4901234567890
4901234567891
```

この2つは意味的には近いかもしれませんが、実務上は別の商品です。

なので、こういうものはベクトル検索ではなく、

```txt
完全一致
正規表現
辞書
マスタ参照
SQL JOIN
バリデーションルール
```

で扱うべきです。

おすすめはハイブリッドです。

```txt
意味的な候補出し: ベクトル検索
厳密な確認: ルール / SQL / マスタ / バリデーション
最終判断: confidence score + UI確認
```

---

## Redisを使ったおすすめ設計

```txt
PostgreSQL:
  - pipeline定義
  - schema定義
  - mapping履歴
  - 実行履歴
  - 採用/却下された候補

S3 / MinIO:
  - 元ファイル
  - 抽出済みデータ
  - 構造化結果

Redis:
  - ベクトルインデックス
  - 直近のschema候補
  - 過去マッピング例の高速検索
  - LLMに渡すcontext候補
  - 一時的な類似検索キャッシュ
```

Redisに全部を保存するのではなく、**検索用インデックスとして使う**のが安全です。

正式なデータは PostgreSQL / S3 側に置き、Redisには検索しやすい形でコピーする感じです。

---

## Embeddingモデルの選び方

日本語・英語・中国語が混ざる可能性があるなら、多言語対応の embedding model が必要です。

たとえば BGE-M3 は、多言語・長文・複数の検索方式に対応する embedding model として紹介されており、100以上の言語、最大8192トークン、dense retrieval / sparse retrieval / multi-vector retrieval をサポートします。Hugging Face上の説明では、次元数は1024です。([bge-model.com][4])

ただし、モデル選定はベンチマークだけで決めない方がいいです。Sentence Transformersのドキュメントでも、リーダーボード上で良いモデルが自分のタスクでも良いとは限らず、実験が重要だと説明されています。([sbert.net][5])

データ構造化パイプラインなら、評価用データを作るのが一番です。

```txt
入力カラム: 品名
正解: product_name

入力カラム: JANコード
正解: jan_code

入力カラム: 定価
正解: list_price

入力カラム: メーカー
正解: manufacturer_name
```

これを100〜500件くらい作って、

```txt
top-1 accuracy
top-3 accuracy
top-5 accuracy
誤マッピング率
人間が修正した割合
```

を見るのが良いです。

---

## 実務では「ベクトル検索だけ」にしない

かなり大事です。

精度を上げるなら、こういう多段構成が良いです。

```txt
1. ルールベース
   - JAN, SKU, price, date などを先に判定

2. ベクトル検索
   - カラム名 + サンプル値 + 周辺カラムで候補取得

3. メタデータフィルタ
   - domain, category, file_type, organization で絞る

4. LLM / reranker
   - 上位候補を再評価

5. UI確認
   - confidenceが低いものだけ人間に確認
```

BGE-M3の公式説明でも、検索パイプラインでは hybrid retrieval と reranking の組み合わせが推奨されています。dense embeddingだけでなく、BM25的な lexical matching や reranker を組み合わせる設計は、データ構造化にもかなり合います。([Hugging Face][6])

---

## 具体的なユースケース別の向き不向き

| ユースケース            | ベクトル検索の相性 | コメント              |
| ----------------- | --------: | ----------------- |
| カラム名から標準スキーマ候補を出す |        高い | 最初にやる価値あり         |
| シート名から抽出対象を推定する   |       中〜高 | サンプル行も混ぜると良い      |
| 商品名から類似商品を探す      |        高い | ただしJANがあるならJAN優先  |
| PDF本文から関連ルールを探す   |        高い | RAG的に使える          |
| JAN/SKU/型番の照合     |        低い | 完全一致・正規化・マスタ参照が良い |
| 価格・数量・日付の判定       |       低〜中 | ベクトルよりルールが良い      |
| 過去のマッピング例の再利用     |        高い | パイプライン改善にかなり効く    |

---

## 私なら最初に作るMVP

最初からRAGまで広げず、まずはこれが良いです。

```txt
Schema Mapping Candidate Search
```

### 保存するもの

```json
{
  "id": "schema:product:product_name",
  "text": "product_name 商品名 品名 商品名称 item name product title ECサイトに表示される商品の正式名称",
  "domain": "retail",
  "schema_type": "product",
  "target_column": "product_name",
  "embedding": [0.01, -0.03, ...]
}
```

### 検索するもの

```json
{
  "source_column": "品目名称",
  "sheet_name": "商品一覧",
  "sample_values": ["カップラーメン", "緑茶 500ml", "チョコレート"],
  "neighbor_columns": ["JANコード", "メーカー名", "規格", "入数"]
}
```

### 返すもの

```json
[
  {
    "target_column": "product_name",
    "score": 0.91,
    "reason": "カラム名とサンプル値が商品名に近い"
  },
  {
    "target_column": "display_name",
    "score": 0.84,
    "reason": "表示名としても解釈可能"
  }
]
```

このMVPはかなり実用的です。
しかも、ユーザーが採用・却下した結果を保存していけば、後で改善できます。

---

## 注意点

ベクトル検索を入れるときの落とし穴はこのあたりです。

```txt
1. カラム名だけをembeddingしてしまう
2. JANやSKUのようなIDまで意味検索してしまう
3. tenant / domain / file_type でフィルタしない
4. scoreだけで自動確定してしまう
5. embedding modelを変えたのに再indexしない
6. 過去の誤ったマッピング例まで検索対象にしてしまう
7. Redisを正式な保存先にしてしまう
```

特に `embedding modelを変えたら再indexが必要` は重要です。
別モデルのベクトル同士は同じ空間にいないので、基本的に混ぜない方がいいです。

---

## まとめ

データ構造化パイプラインでのベクトル検索は、RAGよりもまず **スキーママッピング補助** に使うのが一番効果が出やすいです。

おすすめ構成はこれです。

```txt
Redis Search:
  - schema候補検索
  - 過去マッピング例検索
  - 類似ファイル/類似シート検索
  - LLM用context検索

PostgreSQL:
  - 正式なschema定義
  - mapping履歴
  - 採用/却下ログ
  - pipeline実行履歴

S3 / MinIO:
  - 元ファイル
  - 抽出結果
  - 構造化結果
```

そして最初に作るなら、

```txt
カラム名 + サンプル値 + 周辺カラム
↓
embedding
↓
Redis Search
↓
標準スキーマ候補 top-k
↓
UIで確認
```

が一番現実的です。

[1]: https://redis.io/docs/latest/develop/ai/search-and-query/vectors/ "Vector search concepts | Docs"
[2]: https://redis.io/tutorials/howtos/vector-sets-basics/ "Redis Vector Sets Tutorial: Similarity Search Getting Started Guide"
[3]: https://redis.io/docs/latest/develop/ai/search-and-query/query/vector-search/ "Vector search | Docs"
[4]: https://bge-model.com/bge/bge_m3.html "BGE-M3 — BGE  documentation"
[5]: https://www.sbert.net/docs/sentence_transformer/pretrained_models.html "Pretrained Models — Sentence Transformers  documentation"
[6]: https://huggingface.co/BAAI/bge-m3 "BAAI/bge-m3 · Hugging Face"





---


# 上記はRedis想定ですが、ローカル開発から段階的にスケールする観点で置き換え・移行（Chroma → pgvector/Qdrant）する場合どうなりますか？

移行を考えるなら、最初から「VectorStore抽象化」と「再インデックス可能な元データ」を分けておくのが一番大事です。Chroma・pgvector・Qdrantを、段階ごとの役割で整理します。


結論としては、**Chroma → pgvector → Qdrant** はかなり自然な段階移行です。
ただし、最初から **「Vector DBは派生インデックス」** として扱う設計にしておくのが重要です。

つまり、こうです。

```txt id="0mfc5q"
正本:
  PostgreSQL / S3 / JSONL / Parquet
  - text
  - metadata
  - embedding
  - embedding_model
  - embedding_version
  - content_hash

派生インデックス:
  Chroma
  pgvector
  Qdrant
  Redis Vector Search
```

Vector DB側だけを正本にすると、後で移行がつらくなります。

---

## おすすめの段階移行

```txt id="s0m59x"
Phase 0: ローカルPoC
  Chroma

Phase 1: MVP / 小〜中規模プロダクト
  pgvector

Phase 2: ベクトル検索が本格的に重要になる
  Qdrant
```

私なら、データ構造化パイプラインではこう考えます。

| フェーズ    | 採用候補     | 向いている状況                                  |
| ------- | -------- | ---------------------------------------- |
| ローカル検証  | Chroma   | notebook / Python / 小規模PoC               |
| 初期プロダクト | pgvector | 既にPostgreSQLがあり、schema/mapping履歴とJOINしたい |
| 本格運用    | Qdrant   | ベクトル数・QPS・フィルタ検索・低レイテンシが重要               |
| キャッシュ寄り | Redis    | 一時的な高速検索・短期インデックス                        |

---

## Phase 0: Chroma

Chromaは、**試す速さ**が強いです。
Pythonでは in-memory client や PersistentClient を使えます。PersistentClient はローカルディスクに保存し、起動時に再読み込みできます。JS/TSクライアントの場合は、基本的にChroma serverへ接続する形になります。([docs.trychroma.com][1])

Chromaは `documents`、`embeddings`、`metadatas` を追加できます。自前でembeddingを作った場合は、それを渡せばChroma側で再embeddingせず保存できます。([docs.trychroma.com][2])

### Chromaでやること

```txt id="7lqc0x"
- どのテキストをembeddingするか検証
- カラム名 + サンプル値 + 周辺カラムの効果を検証
- top-k候補のUIを検証
- embedding modelを比較
- score thresholdを試す
```

### Chromaで長く抱えすぎない方がいいもの

```txt id="ac1jgd"
- 正式なmapping履歴
- tenantごとの権限管理
- 本番の監査ログ
- 大量データの長期保存
- パイプライン実行履歴
```

Chromaから移行する可能性があるなら、Chromaにだけデータを持たせず、必ず正本を別に持つべきです。

---

## Phase 1: pgvector

MVP段階では、私は **pgvectorが一番バランスが良い**と思います。

理由は、データ構造化パイプラインではベクトル検索だけで完結しないからです。

たとえば、検索時にこういう条件が欲しくなります。

```txt id="a71bzd"
tenant_id = xxx
domain = retail
schema_type = product
file_type = excel
language = ja
accepted_by_user = true
deleted_at IS NULL
```

pgvectorなら、ベクトルと通常の業務データをPostgreSQL内で一緒に扱えます。pgvectorはPostgreSQL上でベクトル類似検索を行う拡張で、exact / approximate nearest neighbor search、L2、inner product、cosine distanceなどをサポートし、PostgresのACID、PITR、JOINなども使えます。([GitHub][3])

### pgvectorが特に向いている用途

```txt id="k6s8v7"
- schema mapping候補検索
- 過去mapping履歴検索
- tenant / domain / schema_typeで絞った検索
- UIで採用/却下された候補の保存
- PostgreSQL上のpipeline_run / step_runとのJOIN
```

### テーブル設計イメージ

```sql id="n1phq3"
CREATE TABLE vector_items (
  id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL,
  namespace text NOT NULL,
  item_type text NOT NULL,

  source_id text NOT NULL,
  text text NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}',

  embedding_model text NOT NULL,
  embedding_version text NOT NULL,
  embedding vector(1024) NOT NULL,

  content_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
```

たとえば BGE-M3 のような1024次元のモデルなら `vector(1024)` で扱えます。pgvectorの `vector` 型は最大2,000次元、`halfvec` は最大4,000次元に対応しています。([GitHub][3])

### インデックス例

```sql id="fv3qzz"
CREATE INDEX vector_items_embedding_hnsw_idx
ON vector_items
USING hnsw (embedding vector_cosine_ops);

CREATE INDEX vector_items_filter_idx
ON vector_items (tenant_id, namespace, item_type);
```

pgvectorはデフォルトではexact nearest neighbor searchで、HNSWやIVFFlatのapproximate indexを追加すると速度と引き換えにrecallのトレードオフが発生します。HNSWはIVFFlatより速度・recallのバランスが良い一方、ビルド時間とメモリ使用量が増えます。([GitHub][3])

### pgvectorの注意点

pgvectorでフィルタ付きapproximate searchを使う場合、フィルタがindex scan後に適用されることがあります。そのため、条件に合う行が少ないと結果数が足りなくなる場合があります。対策として、通常のB-tree index、partial index、partitioning、`hnsw.ef_search` の調整、iterative scanなどを検討します。([GitHub][3])

つまり、pgvectorは便利ですが、**高カーディナリティのフィルタが多い大規模検索**では、専用Vector DBの方が扱いやすくなります。

---

## Phase 2: Qdrant

Qdrantは、ベクトル検索が本格的にプロダクトの中核になってきたら有力です。

Qdrantはclient-server型で、Python、TypeScript、Rust、Go、.NET、Javaなどの公式クライアントがあり、HTTP/gRPCでも利用できます。データはcollection単位で管理され、各pointはvectorとpayload metadataを持ちます。([Qdrant][4])

### Qdrantに移行したくなるタイミング

```txt id="4ur4fp"
- vector数がかなり増えてきた
- 検索QPSが増えた
- PostgreSQLの負荷とVector検索の負荷を分離したい
- payload filter込みの検索性能が重要
- 複数embeddingを1レコードに持ちたい
- horizontal scaling / replicationが必要
- vector searchを独立したサービスとして運用したい
```

Qdrantはcollection作成時にvector sizeとdistance metricを指定します。payload indexを作ると、フィルタ条件をsemantic search中に効率よく扱える設計になっています。スケール面では、sharding、replication、on-disk vector、payload indexなどの選択肢があります。([Qdrant][4])

ローカル開発もDockerで簡単に始められ、REST APIは6333、gRPCは6334で使えます。([Qdrant][5])

### Qdrantのcollection設計

データ構造化パイプラインなら、まずはcollectionを細かく分けすぎない方が良いです。

```txt id="8uy5ix"
collection: structured_data_embeddings

payload:
  tenant_id
  namespace
  item_type
  domain
  schema_type
  source_id
  embedding_model
  embedding_version
```

Qdrantのドキュメントでも、基本的には単一collection + payload-based partitioning、つまりpayloadによるマルチテナンシーが多くの場合に推奨されています。完全な分離が必要な場合は複数collectionも選択肢になります。([Qdrant][6])

---

## Chroma → pgvector/Qdrant の移行で重要なこと

一番大事なのは、**Chromaから直接移行する設計にしない**ことです。

悪い例：

```txt id="m2drh6"
Chromaにだけdocuments/metadatas/embeddingsがある
↓
必要になったらChromaからexportして移行
```

良い例：

```txt id="3i00x0"
PostgreSQL/S3に正本がある
↓
Chromaはローカル検証用index
↓
pgvector/Qdrantへ再index
```

Chromaからembeddingを取り出すこと自体は可能です。Chromaはデフォルトではembeddingを返さないため、必要な場合は `include=["embeddings", "documents", "metadatas", "distances"]` を指定します。([docs.trychroma.com][7])
ただし、移行の基本戦略としては **exportではなくrebuild index** の方が安全です。

---

## 抽象化レイヤーを先に作る

最初からこのようなinterfaceを切っておくと、Chroma → pgvector → Qdrant の移行が楽になります。

```ts id="f0fozv"
export type VectorItem = {
  id: string;
  namespace: string;
  itemType: string;
  text: string;
  embedding: number[];
  metadata: Record<string, unknown>;
  embeddingModel: string;
  embeddingVersion: string;
};

export type VectorSearchQuery = {
  namespace: string;
  itemType?: string;
  embedding: number[];
  topK: number;
  filter?: Record<string, unknown>;
};

export type VectorSearchResult = {
  id: string;
  score: number;
  text?: string;
  metadata: Record<string, unknown>;
};

export interface VectorStore {
  upsert(items: VectorItem[]): Promise<void>;
  search(query: VectorSearchQuery): Promise<VectorSearchResult[]>;
  delete(ids: string[]): Promise<void>;
}
```

実装だけ差し替えます。

```txt id="697ksi"
ChromaVectorStore
PgVectorStore
QdrantVectorStore
RedisVectorStore
```

アプリケーション側は、どのVector DBを使っているか知らなくてよい形にします。

---

## ID設計を固定する

移行で壊れやすいのはIDです。

おすすめは、意味のある安定IDです。

```txt id="wum7an"
schema_column:{tenant_id}:{schema_id}:{column_id}:{embedding_model_version}

mapping_example:{tenant_id}:{mapping_id}:{embedding_model_version}

file_column:{tenant_id}:{file_id}:{sheet_name}:{column_name}:{embedding_model_version}
```

ただし、Qdrantではpoint IDとしてUUIDや64-bit integerを使うのが自然です。Qdrantのpointは64-bit integerまたはUUIDで識別されます。([Qdrant][4])

なので、外部IDと内部IDを分けても良いです。

```txt id="0y6e75"
id: UUID
external_key: schema_column:...
```

---

## embedding model/versionを必ず持つ

これはかなり重要です。

```txt id="uevv71"
embedding_model = bge-m3
embedding_version = 2026-05-01
embedding_dim = 1024
distance_metric = cosine
```

embedding modelを変えたら、基本的に既存ベクトルと混ぜない方が良いです。
同じtextでも、モデルが違えばベクトル空間が違うからです。

移行時はこうします。

```txt id="9u3cvg"
1. 新しいembedding_model_versionを作る
2. 正本データから再embedding
3. 新しいindexにupsert
4. shadow searchで品質比較
5. 問題なければ切り替え
```

---

## 移行手順

実務ではこの流れが安全です。

```txt id="tijmkv"
1. VectorItemの共通スキーマを決める
2. Chroma/pgvector/Qdrantに依存しないVectorStore interfaceを作る
3. 正本テーブル or JSONLにtext/metadata/embeddingを保存する
4. ChromaでPoC
5. pgvectorへbackfill
6. dual writeする
7. shadow readで検索結果を比較する
8. feature flagでread先を切り替える
9. 問題なければChromaを停止
10. 将来必要になったらQdrantへ同じ手順で移行
```

### shadow readの比較

```txt id="77ofxx"
query: "品目名称"

Chroma top-5:
  product_name
  display_name
  category_name

pgvector top-5:
  product_name
  item_name
  display_name

比較:
  top-1一致率
  top-3 overlap
  score差分
  人間評価
```

単純にscoreだけ比較するのは危険です。
Chroma、pgvector、Qdrantでscoreのスケールや距離の扱いが違う可能性があるため、**順位と正解率**を見る方が良いです。

---

## データ構造化パイプラインでの現実的な選択

あなたの用途だと、最初に作るべきなのはたぶんこれです。

```txt id="vzipmx"
Schema Mapping Candidate Search
```

この用途なら、最初はpgvectorで十分な可能性が高いです。

理由は、スキーマ定義・mapping履歴・pipeline定義・ユーザーの採用/却下ログがPostgreSQLにあるはずだからです。

```txt id="ozqh6o"
source column:
  品目名称

query text:
  Column name: 品目名称
  Sheet name: 商品一覧
  Sample values: カップラーメン, 緑茶, チョコレート
  Neighbor columns: JANコード, メーカー名, 内容量

search target:
  schema columns
  accepted mapping examples
  transformation rules
```

この段階では、Qdrantよりpgvectorの方が実装・運用が軽いです。

---

## 判断基準

### Chromaで十分

```txt id="eyqedj"
- PoC
- notebook中心
- ローカル検証
- 数千〜数万件程度の実験
- 本番運用前
```

### pgvectorが良い

```txt id="hg06ze"
- 既にPostgreSQLを使っている
- vector検索と業務テーブルをJOINしたい
- tenant_id / domain / statusで絞りたい
- MVPとして早く出したい
- 運用コンポーネントを増やしたくない
```

### Qdrantが良い

```txt id="p6shq2"
- vector検索が主機能になった
- vector数が増えた
- 高QPS / 低レイテンシが必要
- payload filteringが重要
- Postgresから負荷を分離したい
- 水平スケールしたい
```

---

## 私のおすすめ構成

最初からQdrantに行くより、私はこの順が良いと思います。

```txt id="k9g600"
Local:
  Chroma or pgvector docker

MVP:
  PostgreSQL + pgvector

Scale:
  PostgreSQL + Qdrant

Always:
  VectorStore interface
  Reindex job
  正本データ
  embedding_model_version
```

特に、既にPostgreSQLを使う前提なら、**Chromaを飛ばして最初からpgvectorでも良い**です。

```txt id="l5ctpr"
個人PoC / notebook: Chroma
WebアプリMVP: pgvector
検索基盤として成長: Qdrant
```

この切り分けが一番きれいです。

[1]: https://docs.trychroma.com/docs/run-chroma/clients "Chroma Clients - Chroma Docs"
[2]: https://docs.trychroma.com/docs/collections/add-data "Adding Data to Chroma Collections - Chroma Docs"
[3]: https://github.com/pgvector/pgvector "GitHub - pgvector/pgvector: Open-source vector similarity search for Postgres · GitHub"
[4]: https://qdrant.tech/documentation/overview/ "Overview - Qdrant"
[5]: https://qdrant.tech/documentation/quickstart/ "Local Quickstart - Qdrant"
[6]: https://qdrant.tech/documentation/manage-data/collections/ "Collections - Qdrant"
[7]: https://docs.trychroma.com/docs/overview/troubleshooting "Troubleshooting - Chroma Docs"
