# データ構造化で扱う入力ファイル種別

## 概要

この文書は、HaoHao のデータパイプラインで構造化対象になりやすい入力ファイル種別を整理するためのメモです。

データ構造化では、入力ファイルをそのままテーブルとして読める場合と、テキスト抽出、OCR、表抽出、ネスト展開、スキーマ推定などを経由してからテーブル化する場合がある。CSV、JSON、Excel のような構造化・半構造化ファイルは比較的直接扱いやすい。一方で PDF、画像、Office 文書、HTML、メールのような非構造化ファイルは、抽出結果の根拠、ページ番号、座標、信頼度、元ファイルメタデータを残す設計が重要になる。

現時点の前提として、PDF、画像、CSV、JSON、Excel は対応済みの主要入力として扱う。今後追加を検討する場合は、HTML、XML、テキスト/Markdown、Word、PowerPoint、メール、ログ、Parquet/Avro/ORC を優先候補にする。

## 分類

入力ファイルは大きく次のカテゴリに分けられる。

| 分類 | 代表形式 | 構造化の特徴 |
| --- | --- | --- |
| 構造化データ | CSV, TSV, Parquet, Avro, ORC | 既に行列またはスキーマ付きデータとして扱える |
| 半構造化データ | JSON, XML, HTML | ネスト、属性、配列、DOM などを抽出定義でテーブル化する |
| 表計算・Office | Excel, Word, PowerPoint | 表、シート、段落、スライドなどの単位で抽出する |
| 文書・画像 | PDF, 画像 | OCR、ページ解析、表抽出、レイアウト解析が必要になる |
| メッセージ・ログ | EML, MSG, JSONL, NDJSON, `.log` | 時系列、スレッド、イベント、添付を構造化する |

## 対応済みの主要形式

### PDF

PDF は契約書、請求書、申請書、帳票、商品仕様書、報告書などでよく使われる。ネイティブテキストを持つ PDF と、スキャン画像だけの PDF で抽出方法が変わる。

- 主な用途: 帳票抽出、契約情報抽出、請求書明細抽出、報告書の項目抽出。
- 代表的な抽出単位: ファイル、ページ、段落、表、フィールド、座標付きテキスト。
- 典型的な出力列: `file_public_id`, `file_name`, `page_number`, `text`, `field_name`, `field_value`, `table_id`, `row_number`, `confidence`, `bbox_json`。
- 必要な処理: テキスト抽出、OCR、表領域検出、ヘッダー推定、ページまたぎ表の連結、信頼度判定。
- 注意点: ページ番号、座標、抽出エンジン、信頼度を残す。OCR とネイティブテキスト抽出の結果を混同しない。複数列レイアウトや脚注、印影、手書き文字は誤抽出しやすい。

候補 step:

- `extract_text`
- `extract_table`
- `extract_fields`
- `classify_document`
- `confidence_gate`

### 画像

画像はスキャン帳票、レシート、商品ラベル、検査結果、本人確認書類、スクリーンショットなどで使われる。基本的には OCR と画像前処理が入口になる。

- 主な用途: レシート/伝票 OCR、ラベル情報抽出、商品画像からの属性抽出、紙帳票のデジタル化。
- 代表的な抽出単位: 画像、領域、行テキスト、フィールド、表セル。
- 典型的な出力列: `file_public_id`, `file_name`, `image_width`, `image_height`, `text`, `bbox_json`, `confidence`, `field_name`, `field_value`。
- 必要な処理: OCR、回転補正、傾き補正、ノイズ除去、領域検出、文字方向判定。
- 注意点: 画像品質に強く依存する。抽出値だけでなく bbox と信頼度を残すとレビューしやすい。言語別 OCR、手書き文字、低解像度画像は別途評価が必要。

候補 step:

- `extract_text`
- `extract_fields`
- `extract_table`
- `detect_language_encoding`
- `confidence_gate`

### CSV / TSV

CSV と TSV は最も一般的なテーブル入力で、データベース出力、業務システム連携、集計データ、マスターデータでよく使われる。

- 主な用途: マスター投入、売上データ、ログ集計、商品一覧、顧客一覧。
- 代表的な抽出単位: ファイル、行、列。
- 典型的な出力列: 元ファイルの各列、`row_number`, `file_public_id`, `file_name`。
- 必要な処理: 区切り文字判定、ヘッダー有無判定、文字コード判定、型推定、欠損処理。
- 注意点: カンマを含む引用符、改行入りセル、Shift_JIS、BOM、列数不一致、ヘッダー重複を扱う必要がある。

候補 step:

- `input`
- `clean`
- `normalize`
- `validate`
- `schema_mapping`

### JSON

JSON は API レスポンス、設定ファイル、イベントデータ、ネストしたマスターデータでよく使われる。配列やオブジェクトを JSON path で抽出してテーブル化する。

- 主な用途: API データ構造化、商品カタログ、イベントログ、ネストした参照データ。
- 代表的な抽出単位: ファイル、record path に一致するオブジェクト、配列要素、ネストフィールド。
- 典型的な出力列: 抽出定義の列、`record_path`, `row_number`, `raw_record_json`, `file_public_id`, `file_name`。
- 必要な処理: record path 指定、field path 指定、配列 join、デフォルト値、raw record 保持。
- 注意点: ルートが配列かオブジェクトかで record path が変わる。深いネスト、可変スキーマ、配列の展開粒度を設計する必要がある。

候補 step:

- `input` の JSON 抽出設定
- `json_extract`
- `schema_inference`
- `schema_mapping`
- `validate`

### Excel

Excel は業務現場で非常に多く、複数シート、結合セル、タイトル行、注記、ピボット風レイアウトなどが含まれることが多い。

- 主な用途: 台帳、見積、受発注、棚卸、商品一覧、会計資料、管理表。
- 代表的な抽出単位: workbook、sheet、range、table、row、cell。
- 典型的な出力列: シート内の各列、`sheet_name`, `sheet_index`, `row_number`, `file_public_id`, `file_name`。
- 必要な処理: シート選択、ヘッダー行指定、範囲指定、空行処理、セル型解釈、日付シリアル値変換。
- 注意点: 表の開始位置が固定でないケース、複数表が同一シートにあるケース、結合セル、数式、非表示行/列を扱う方針が必要。

候補 step:

- `input` の spreadsheet 設定
- `excel_extract`
- `clean`
- `normalize`
- `schema_mapping`
- `validate`

## 追加優先度が高い形式

### HTML / Web ページ

HTML は DOM 構造を持つため、PDF より安定して抽出できる場合がある。EC 商品ページ、FAQ、ニュース、ヘルプページ、社内 Wiki、公開データ表でよく使われる。

- 主な用途: Web 表、商品ページ、ニュース記事、FAQ、ドキュメントページの構造化。
- 抽出単位: URL、DOM node、table、heading section、link、metadata。
- 典型的な出力列: `url`, `title`, `heading`, `text`, `table_id`, `row_number`, `selector`, `link_url`, `extracted_at`。
- 実装候補: `html_extract`。
- 設定項目候補: CSS selector、XPath、抽出対象属性、table 抽出、本文抽出モード、リンク追跡有無。
- 注意点: 動的レンダリング、ログイン、ページネーション、robots/policy、文字化け、広告やナビゲーションの除外が必要になる。

優先度は高い。理由は、DOM という構造を利用でき、業務データや公開情報の取り込みに使いやすいため。

### XML

XML は官公庁、金融、医療、出版、古い基幹システム連携でまだ多い。JSON と同様にネスト構造をテーブル化する需要がある。

- 主な用途: 公開データ、業界標準データ、設定、古い API 連携、文書構造データ。
- 抽出単位: document、element、attribute、namespace、repeated node。
- 典型的な出力列: XPath に対応する抽出列、`record_path`, `row_number`, `raw_record_xml`, `file_public_id`。
- 実装候補: `xml_extract`。
- 設定項目候補: record XPath、field XPath、namespace mapping、属性抽出、テキスト正規化。
- 注意点: namespace、属性と本文の扱い、同名要素の繰り返し、DTD/外部 entity の安全性に注意する。

JSON 抽出と近い UI/実装モデルを流用しやすいため、追加候補として扱いやすい。

### テキスト / Markdown

プレーンテキストと Markdown は、議事録、ログ、仕様書、README、ナレッジ文書、チャットエクスポートなどでよく使われる。

- 主な用途: 文書チャンク化、見出し単位の分割、キー値抽出、箇条書き抽出、要約前処理。
- 抽出単位: ファイル、見出し section、段落、行、コードブロック、キー値。
- 典型的な出力列: `file_public_id`, `file_name`, `section_path`, `heading`, `line_number`, `text`, `chunk_index`。
- 実装候補: `text_extract`, `markdown_extract`。
- 設定項目候補: chunk size、overlap、見出し分割、front matter 抽出、コードブロック含有有無。
- 注意点: 自由形式なので、構造化ルールを過信しない。Markdown は見出し階層や front matter を構造として活用できる。

LLM を使った抽出や検索インデックス作成の前処理として重要度が高い。

### Word / DOCX

Word は契約書、報告書、申請書、議事録、マニュアルでよく使われる。段落、見出し、表、コメント、変更履歴などが構造化対象になる。

- 主な用途: 契約項目抽出、文書分類、表抽出、レビューコメント抽出、章単位の構造化。
- 抽出単位: document、heading、paragraph、table、comment、section。
- 典型的な出力列: `file_public_id`, `file_name`, `heading_path`, `paragraph_index`, `text`, `table_id`, `row_number`, `comment_text`。
- 実装候補: `docx_extract`。
- 設定項目候補: 段落抽出、表抽出、コメント抽出、変更履歴含有有無、見出し階層保持。
- 注意点: 表と本文が混在する。変更履歴、コメント、ヘッダー/フッター、脚注を抽出対象にするか設計する必要がある。

契約書や申請書を扱うなら優先度は高い。

### PowerPoint / PPTX

PowerPoint は営業資料、提案資料、調査レポート、研修資料でよく使われる。文章量は少なくても、スライド構造や図表に意味がある。

- 主な用途: 提案資料の要点抽出、スライド単位の分類、表や箇条書きの構造化。
- 抽出単位: deck、slide、shape、text box、table、speaker note。
- 典型的な出力列: `file_public_id`, `file_name`, `slide_number`, `shape_type`, `text`, `speaker_note`, `table_id`。
- 実装候補: `pptx_extract`。
- 設定項目候補: speaker note 抽出、非表示スライド含有有無、図表内テキスト抽出、スライド画像化。
- 注意点: 図として貼られた表や画像内文字は OCR が必要。スライドの読み順を正しく推定する必要がある。

ナレッジ化や営業資料分析では有用だが、表形式データの直接取り込みとしては DOCX より優先度は下がる。

### メール / EML / MSG

メールは問い合わせ、受発注、サポート、契約交渉、アラート通知でよく使われる。本文だけでなく、送受信者、日時、スレッド、添付が重要になる。

- 主な用途: 問い合わせ分類、注文情報抽出、サポート履歴構造化、添付ファイル連携。
- 抽出単位: message、thread、body、header、attachment。
- 典型的な出力列: `message_id`, `thread_id`, `from`, `to`, `cc`, `subject`, `sent_at`, `body_text`, `attachment_file_public_id`。
- 実装候補: `email_extract`。
- 設定項目候補: HTML/plain text 優先度、引用除去、署名除去、添付抽出、スレッド grouping。
- 注意点: 個人情報が多い。引用文や署名を除外しないと重複が増える。添付ファイルを別 pipeline に渡せる設計が必要。

業務自動化での価値は高いが、PII 対応と監査設計を先に整える必要がある。

## 追加優先度が中程度の形式

### ログファイル

ログはアプリケーションログ、アクセスログ、監査ログ、ジョブログなどで使われる。形式が固定されていれば構造化しやすいが、自由文ログはパースルールが必要になる。

- 主な用途: 障害分析、監査、利用状況分析、イベント集計。
- 対象形式: `.log`, JSONL, NDJSON, Apache/Nginx log, syslog, application log。
- 抽出単位: 行、イベント、スタックトレース、JSON object。
- 典型的な出力列: `timestamp`, `level`, `service`, `event_name`, `message`, `request_id`, `user_id`, `attributes_json`。
- 実装候補: `log_extract`, `jsonl_extract`。
- 設定項目候補: 行パターン、timestamp format、multiline stack trace、JSONL mode、正規表現 named capture。
- 注意点: 機密情報や token が混ざりやすい。巨大ファイルを streaming で扱う必要がある。

運用分析の需要がある場合は優先度が上がる。

### Parquet / Avro / ORC

Parquet、Avro、ORC はデータ基盤で使われるスキーマ付き形式で、既に構造化済みであることが多い。

- 主な用途: DWH/データレイク連携、分析データ取り込み、外部データセット取り込み。
- 抽出単位: ファイル、row group、record。
- 典型的な出力列: ファイル内スキーマの各列、`file_public_id`, `row_number`。
- 実装候補: `columnar_file_input` または `parquet_input`。
- 設定項目候補: schema projection、partition columns、compression、nested column flattening。
- 注意点: 大容量前提なのでメモリに全展開しない。型、timestamp、decimal、nested column の扱いを明確にする。

分析基盤連携が増えるなら重要だが、非構造化データの構造化という観点では優先度は少し下がる。

### YAML

YAML は設定ファイル、Kubernetes manifest、CI 設定、ドキュメント front matter でよく使われる。

- 主な用途: 設定棚卸し、manifest 解析、ドキュメント metadata 抽出。
- 抽出単位: document、mapping、sequence、field。
- 典型的な出力列: path 抽出列、`record_path`, `raw_record_yaml`, `file_public_id`。
- 実装候補: `yaml_extract`。
- 設定項目候補: record path、field path、multi-document YAML、anchor/alias の扱い。
- 注意点: YAML は仕様が広く、anchor、alias、複数 document、型推定の癖がある。安全な parser を使う。

JSON/XML と同じ半構造化ファイルとして扱える。

## 実装優先度

次に追加するなら、以下の順を推奨する。

1. HTML / Web ページ
2. XML
3. テキスト / Markdown
4. Word / DOCX
5. メール / EML / MSG
6. PowerPoint / PPTX
7. ログ / JSONL / NDJSON
8. Parquet / Avro / ORC
9. YAML

この順序の理由は、構造化ニーズの多さ、既存の JSON/Excel/PDF/画像対応との実装類似性、業務データとしての出現頻度を考慮したため。HTML と XML は JSON 抽出に近い path/selector ベースの UI にしやすい。テキスト/Markdown と DOCX は非構造化処理の入口として使いやすい。メールは価値が高いが、PII と添付連携を丁寧に設計する必要がある。

## step 設計の方針

入力形式ごとの抽出は、`input` node にすべて詰め込むのではなく、独立した extract step として追加できる設計が望ましい。

既に `json_extract` と `excel_extract` を別機能として扱う方針があるため、今後の候補も同じように `html_extract`、`xml_extract`、`docx_extract`、`email_extract` のような step として追加するのが自然。

一方で、CSV や spreadsheet table のように input 時点でテーブルとして読めるものは `input` node の設定で十分な場合がある。判断基準は次の通り。

- `input` node に向いている: ファイルを読むだけで行列データになる、設定が少ない、抽出ロジックが汎用的。
- extract step に向いている: 抽出定義が複雑、複数の出力粒度がある、抽出結果の根拠や信頼度を持つ、後で別の入力 source にも再利用したい。

## 共通で持たせたいメタデータ列

ファイル由来の構造化データでは、元データへの追跡性を保つために以下の列を共通候補にする。

| 列 | 用途 |
| --- | --- |
| `file_public_id` | 元 Drive file への参照 |
| `file_name` | レビューや検索で使う表示名 |
| `mime_type` | 入力形式の判定 |
| `file_revision` | 再実行時の差分や監査 |
| `row_number` | 入力内の行番号または抽出順 |
| `page_number` | PDF/画像/Office 文書のページ参照 |
| `sheet_name` | Excel のシート参照 |
| `slide_number` | PowerPoint のスライド参照 |
| `record_path` | JSON/XML/YAML の record 位置 |
| `source_path` | DOM selector、XPath、JSON path などの抽出元 |
| `bbox_json` | OCR や文書レイアウトの座標 |
| `confidence` | OCR/抽出/分類の信頼度 |
| `raw_record_json` | 半構造化 record の原文保持 |

これらは常に出力に含める必要はないが、プレビュー、レビュー、再処理、監査、品質評価では有用。UI では「抽出元メタデータ列を含める」のように切り替え可能にする。

## UI 設定で共通化したい項目

抽出 step の設定 UI は、形式ごとの違いを出しつつも、以下は共通パターンにできる。

- 抽出対象: ファイル列、URL 列、Drive file、添付ファイルなど。
- record path / selector: JSON path、XPath、CSS selector、sheet/range など。
- fields editor: 出力列名、抽出 path、配列 join、デフォルト値、型。
- max rows / max records: Preview や安全な実行制限。
- raw record の保持: 原文 JSON/XML/HTML snippet などを残すか。
- metadata columns: file、page、row、path、confidence などを含めるか。
- error handling: 抽出失敗時に行を落とすか、エラー列として残すか。

JSON、XML、YAML は field path editor を共通化しやすい。HTML は CSS selector/XPath editor、Excel は sheet/range editor、DOCX/PPTX/PDF/画像は文書構造と OCR 設定を中心にする。

## 注意点

ファイル構造化は、単に値を取り出すだけではなく、取り出した値を信頼できる状態にすることが重要。

- 元ファイルへの参照を残す。
- 抽出元の位置を残す。
- 自動抽出の信頼度を残す。
- 失敗、未抽出、複数候補を明示する。
- 大容量ファイルは streaming または chunking を前提にする。
- PII や機密情報が preview、ログ、LLM 入力に漏れないようにする。
- 抽出 step の設定変更時に再実行・差分確認できるようにする。

特に PDF、画像、メール、Office 文書は誤抽出が避けられないため、`confidence_gate`、`human_review`、`quality_report` と組み合わせて運用する前提で設計する。
