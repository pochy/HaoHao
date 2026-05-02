
あなたはシニアソフトウェアエンジニアです。
LLMを使わず、VRAM 4GB以下の環境でも動作する「商品情報抽出ツール」を開発してください。

## 目的

約14万文字程度の長文テキストから、商品情報を自動抽出するツールを作成してください。

GPUが使える場合でもVRAM 4GB以内で動作することを前提にしてください。
基本的にはCPU実行で問題ありません。

## 想定入力

入力はプレーンテキストです。

ただし、元データは以下のようなものから抽出されたテキストである可能性があります。

- ECサイトの商品一覧
- 商品カタログ
- PDFから抽出したテキスト
- HTMLからタグを除去したテキスト
- 会社情報、利用規約、送料、返品情報などが混ざった長文

文字数は最大で約14万文字程度を想定してください。

## 抽出したい商品情報

以下の項目を抽出してください。

```ts
type Product = {
  productName: string | null;
  brand: string | null;
  manufacturer: string | null;
  price: number | null;
  janCode: string | null;
  sku: string | null;
  modelNumber: string | null;
  category: string | null;
  description: string | null;
  size: string | null;
  color: string | null;
  capacity: string | null;
  sourceText: string;
  confidence: number;
};
````

## 技術方針

LLMは使わず、以下のような軽量な手法を組み合わせてください。

### 必須

* 正規表現
* キーワードベースの抽出
* ブロック分割
* 商品候補スコアリング
* 重複排除
* JSON出力

### 可能なら使用

* SudachiPy
* GiNZA
* spaCy
* scikit-learn
* TF-IDF + LinearSVC / LogisticRegression
* CRF / sklearn-crfsuite

ただし、初期実装では学習済みモデルや教師データがなくても動くようにしてください。

## 初期実装の優先方針

まずは以下の構成で実装してください。

```txt
1. テキスト前処理
2. ブロック分割
3. 商品候補ブロックのスコアリング
4. 正規表現による項目抽出
5. GiNZA / SudachiPy が利用可能なら名詞句抽出
6. 商品単位への整形
7. 重複排除
8. JSON出力
```

## 前処理

以下を実装してください。

* 改行コードの正規化
* 連続空白の正規化
* 全角数字を半角数字へ変換
* 全角英数字を半角英数字へ変換
* 連続する空行の削減
* HTMLタグが残っている場合は除去
* 不要になりやすい文言の除外候補を判定

除外候補の例:

```txt
会社概要
お問い合わせ
利用規約
プライバシーポリシー
返品
送料
配送
特定商取引法
ログイン
会員登録
カート
お気に入り
レビュー一覧
FAQ
```

ただし、完全に削除すると商品説明を失う可能性があるため、まずはスコアリングでマイナス評価にしてください。

## ブロック分割

14万文字を一度に処理しないでください。

以下の単位でブロック分割してください。

* 空行区切り
* 見出しらしき行
* JANコード周辺
* 価格表記周辺
* 「商品名」「品名」「製品名」「ブランド」「メーカー」などのキーワード周辺

1ブロックは目安として500〜3000文字程度にしてください。

長すぎるブロックはさらに分割してください。

## 商品候補スコアリング

各ブロックに対して商品情報らしさのスコアを付けてください。

加点例:

```txt
+5: 13桁JANコードがある
+4: 価格表記がある
+4: 「商品名」「品名」「製品名」がある
+3: 「ブランド」「メーカー」がある
+3: 「内容量」「容量」「サイズ」「カラー」がある
+3: 「型番」「品番」「SKU」がある
+2: g / kg / ml / L / cm / mm などの単位がある
+2: 商品説明らしい文がある
+1: 名詞句が多い
```

減点例:

```txt
-5: 利用規約
-5: プライバシーポリシー
-4: 会社概要
-4: お問い合わせ
-3: 送料
-3: 返品
-3: 会員登録
-3: ログイン
```

一定以上のスコアのブロックのみ商品候補として扱ってください。

スコア閾値は設定で変更できるようにしてください。

## 正規表現による抽出

最低限、以下を抽出してください。

### JANコード

13桁数字を抽出してください。

```txt
4901234567890
```

ただし、周辺に「JAN」「JANコード」「バーコード」などがある場合は信頼度を上げてください。

### 価格

以下のような表記を抽出してください。

```txt
1,280円
¥1,280
税込 1,280円
価格：1280円
本体価格 980円
```

数値は `number` に変換してください。

### SKU / 型番 / 品番

以下のようなラベルに対応してください。

```txt
SKU
型番
品番
商品コード
管理番号
製品番号
Model
Model No.
```

### 内容量 / 容量 / サイズ / カラー

以下のような表記を抽出してください。

```txt
内容量：500g
容量：350ml
サイズ：M
カラー：ブラック
色：ホワイト
```

## 商品名抽出

商品名は難しいので、以下の優先順位で抽出してください。

1. 「商品名」「品名」「製品名」などのラベルの値
2. JANや価格の近くにある見出し行
3. ブロック先頭付近の名詞句
4. ブランド名 + 名詞句 + 容量/サイズ表現の組み合わせ

推測しすぎないでください。
商品名が取れない場合は `null` にしてください。

## ブランド / メーカー抽出

以下のラベルを優先してください。

```txt
ブランド
メーカー
製造元
販売元
発売元
```

ラベルがない場合は、ブロック先頭の固有名詞候補をブランド候補として扱ってもよいですが、confidenceは低めにしてください。

## description 抽出

商品説明らしい段落を `description` に入れてください。

ただし、以下は説明文から除外してください。

* 送料
* 返品
* 配送
* 会社概要
* 利用規約
* レビュー一覧
* ナビゲーション文言

## confidence

各商品に `confidence` を付けてください。

0〜1 の数値にしてください。

例:

```txt
JANあり: +0.25
価格あり: +0.15
商品名あり: +0.25
ブランドあり: +0.10
内容量/サイズあり: +0.10
商品候補スコアが高い: +0.15
```

最大は1.0に丸めてください。

## 重複排除

以下の優先順位で同一商品を判定してください。

1. janCode が同じ
2. sku が同じ
3. modelNumber が同じ
4. productName + brand がほぼ同じ
5. productName + price がほぼ同じ

重複した場合は、confidenceが高い情報を優先してマージしてください。

nullではない値を優先してください。

## 出力形式

JSONで出力してください。

```json
{
  "products": [
    {
      "productName": "AGF ブレンディ スティック カフェオレ 100本",
      "brand": "AGF",
      "manufacturer": null,
      "price": 1280,
      "janCode": "4901111111111",
      "sku": null,
      "modelNumber": null,
      "category": null,
      "description": "まろやかな味わいのスティックタイプのカフェオレです。",
      "size": null,
      "color": null,
      "capacity": "100本",
      "sourceText": "...",
      "confidence": 0.92
    }
  ],
  "stats": {
    "inputCharacters": 140000,
    "blocks": 120,
    "candidateBlocks": 28,
    "products": 12
  }
}
```

## 実装要件

CLIツールとして実装してください。

例:

```bash
python extract_products.py input.txt --output products.json
```

または Node.js / TypeScript の場合:

```bash
pnpm extract input.txt --output products.json
```

## 設定可能にする項目

以下は設定ファイルまたはCLIオプションで変更できるようにしてください。

```txt
スコア閾値
最大ブロックサイズ
JAN周辺の抽出文字数
価格抽出の有効/無効
GiNZA/SudachiPy使用の有効/無効
出力形式
```

## テスト

最低限、以下のテストを作成してください。

1. JANコードを抽出できる
2. 価格を数値として抽出できる
3. SKU / 型番を抽出できる
4. 内容量を抽出できる
5. 商品名ラベルから商品名を抽出できる
6. 会社概要や利用規約ブロックのスコアが低くなる
7. 商品候補ブロックのスコアが高くなる
8. 同じJANの商品が重複排除される
9. null値の商品情報をマージできる
10. 14万文字程度の入力でも処理できる

## 実装時の注意

* LLM APIは使わないでください。
* OpenAI API、Claude API、Gemini APIなどは使わないでください。
* 大規模言語モデルは使わないでください。
* VRAM 4GB以内で動作する構成にしてください。
* 可能な限りCPUだけで動作するようにしてください。
* ルールベース部分を先に完成させてください。
* ML分類器やCRFは後から追加できる設計にしてください。
* 抽出ロジックは関数ごとに分離してください。
* テストしやすい構造にしてください。
* 抽出結果には必ず sourceText を含めて、後から人間が確認できるようにしてください。

## 推奨ディレクトリ構成

Pythonの場合:

```txt
product_extractor/
  __init__.py
  cli.py
  preprocess.py
  block_splitter.py
  scoring.py
  regex_extractors.py
  product_builder.py
  deduplication.py
  schema.py
  config.py
tests/
  test_preprocess.py
  test_block_splitter.py
  test_scoring.py
  test_regex_extractors.py
  test_deduplication.py
sample/
  input.txt
  output.json
```

TypeScriptの場合:

```txt
src/
  cli.ts
  preprocess.ts
  blockSplitter.ts
  scoring.ts
  regexExtractors.ts
  productBuilder.ts
  deduplication.ts
  schema.ts
  config.ts
tests/
  preprocess.test.ts
  blockSplitter.test.ts
  scoring.test.ts
  regexExtractors.test.ts
  deduplication.test.ts
sample/
  input.txt
  output.json
```

## まず実装してほしいもの

最初のMVPとして、以下を完成させてください。

1. CLI
2. テキスト前処理
3. ブロック分割
4. 商品候補スコアリング
5. JAN / 価格 / SKU / 型番 / 内容量 / サイズ / カラーの抽出
6. 商品名ラベルからの商品名抽出
7. JSON出力
8. 重複排除
9. 基本テスト

CRF、TF-IDF分類器、GiNZA/SudachiPy連携は、MVP完成後に拡張できるように設計してください。