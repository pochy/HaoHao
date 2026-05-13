# Data Pipeline LLM Node Ideas

## 概要

この文書は、HaoHao Data Pipeline で AI / LLM を活用する node 候補を整理した提案メモです。ここに書く `llm_*` node は、現時点の backend catalog に実装済みの step type ではありません。現在の `classify_document`、`extract_fields`、`entity_resolution` などは主に rule / regex / dictionary based であり、この文書ではそれらを補完する将来案として LLM node を説明します。

HaoHao で LLM を使う価値が高いのは、曖昧な非構造化データを、根拠と confidence 付きで構造化する場面です。例えば請求書、注文書、契約書、メール、問い合わせ文、OCR text のように、入力形式や表現が揺れるデータでは、rule だけで全パターンを拾うのが難しくなります。

ただし、LLM の出力をそのまま正解扱いすると、誤抽出、幻覚、再現性低下、コスト増、監査困難につながります。そのため LLM node は「候補生成、抽出、分類、説明」に使い、最終制御は `confidence_gate`、`validate`、`quarantine`、`human_review` へ渡す設計にします。

## 基本方針

LLM node は、SQL や rule で安定して処理できることを置き換えるものではありません。ClickHouse の集計、join、filter、数値計算、regex で十分に表現できる抽出は、既存 node を優先します。LLM は「意味理解が必要」「入力形式が揺れる」「候補や理由を出したい」場合に使います。

基本方針は次の通りです。

- LLM の役割は、候補生成、分類、抽出、説明、review 補助に限定する。
- 出力は自由文ではなく、JSON schema に沿った structured output にする。
- すべての重要な値に confidence、evidence、failure reason を付ける。
- 低 confidence、根拠不足、schema validation 失敗は `human_review` に回せる形にする。
- LLM の raw response を業務データの正本にしない。
- model、prompt version、input policy、token usage、latency、provider error を run step metadata に残す。
- PII や secret を含む可能性がある入力は、送信前 redaction、tenant policy、監査ログの対象にする。
- 同じ入力と同じ prompt version でできるだけ再現性が出るように、temperature は低く保つ。

初心者向けに言うと、LLM node は「自動で決める機械」ではなく、「候補と理由を作る補助者」です。後続 node がそれを検査し、必要なら人に回すことで、運用で壊れにくい pipeline になります。

## 共通 contract

LLM node は node ごとに目的が違っても、共通の contract を持つと実装と UI が揃います。

### 共通 input

- `inputColumns`: LLM に渡す列。例: `text`、`normalized_text`、`layout_json`、`vendor`、`address`。
- `contextColumns`: 補助 context として渡す列。例: `file_name`、`page_number`、`document_type`。
- `promptVersion`: prompt template の version。結果の再現性と監査のために必須にする。
- `modelPolicy`: tenant または system で許可された model policy 名。直接 model 名を node に固定しすぎない。
- `outputPrefix`: 出力列名の prefix。複数 LLM node を置いた場合の列衝突を避ける。
- `confidenceThreshold`: 自動通過と review の境界。
- `maxInputChars`: 1 row あたり LLM に渡す最大文字数。
- `batchSize`: provider に送る batch size。rate limit と cost control に使う。
- `redactBeforeSend`: LLM 送信前に PII / secret らしい値を mask するか。
- `storeRawResponse`: raw response を保存するか。既定は false または管理者限定が安全。

### 共通 output columns

LLM node は、node 固有の値に加えて、次のような共通列を持つと後続 node が扱いやすくなります。列名は `outputPrefix` を付けてもよいです。

| column | 内容 |
| --- | --- |
| `llm_status` | `passed` / `needs_review` / `failed` / `skipped` |
| `llm_confidence` | 0.0 から 1.0 の総合 confidence |
| `llm_reason` | status や判断の短い理由 |
| `llm_evidence_json` | 根拠 text、page、source column、span、box id など |
| `llm_warnings_json` | 欠損、曖昧、schema mismatch、truncation など |
| `llm_result_json` | node 固有の structured output |

業務列として後続処理が読む値は通常列に展開します。例えば `llm_extract_fields` なら `invoice_number`、`invoice_date`、`total_amount` のような列です。一方、debug や監査向けの詳細は `llm_result_json` や run step metadata に寄せます。

### 共通 run step metadata

`data_pipeline_run_steps.metadata` には、行ごとではなく node 実行全体の summary を保存します。

| metadata | 内容 |
| --- | --- |
| `model_policy` | 使用した model policy |
| `model` | 実際に使った model 名 |
| `prompt_version` | prompt template version |
| `input_rows` | 対象行数 |
| `completed_rows` | 成功行数 |
| `needs_review_rows` | review 行数 |
| `failed_rows` | 失敗行数 |
| `skipped_rows` | 入力不足などで skipped の行数 |
| `avg_confidence` | 平均 confidence |
| `low_confidence_samples` | 低 confidence の sample。ただし個人情報は mask する |
| `token_usage` | prompt / completion token の集計 |
| `latency_ms` | total / p50 / p95 など |
| `provider_errors` | rate limit、timeout、schema validation error など |
| `warnings` | truncation、redaction、fallback 使用など |

行ごとの判定は ClickHouse の列に残し、run detail や監視で見たい集計は `data_pipeline_run_steps.metadata` に残す、という分担を守ります。

### 共通 failure handling

LLM node は外部 provider、local runtime、prompt、schema validation など複数の失敗点を持ちます。失敗時の扱いは node config で選べるようにしつつ、既定は安全側に倒します。

- provider timeout: 行を `failed` または `needs_review` にし、run 全体を即失敗にしない mode を用意する。
- structured output parse 失敗: raw response を通常列に出さず、`llm_warnings_json` に理由を残す。
- confidence 不足: 値は候補として残し、`confidence_gate` または `human_review` に渡す。
- input truncation: `llm_warnings_json` と metadata に `truncated=true` を残す。
- PII redaction 後に判断不能: `needs_review` にする。

## 推奨 LLM node

### `llm_extract_fields`

目的:

請求書、注文書、契約書、申込書、メール本文、OCR text から、指定した field を抽出します。HaoHao で最初に実装する LLM node として最も効果が高い候補です。

既存 node との違い:

既存の `extract_fields` は regex / rule based で、定型 text には強い一方、表現の揺れや位置の揺れには弱くなります。`llm_extract_fields` は「請求番号」「支払期限」「取引先名」「合計金額」のような意味を指定し、文脈から値を候補として抽出します。

入力:

- `text` または `normalized_text`
- 必要に応じて `layout_json`、`boxes_json`、`page_number`
- `document_type`
- field 定義。例: name、description、type、required、format、examples

出力列:

- field ごとの値列。例: `invoice_number`、`invoice_date`、`total_amount`
- field ごとの confidence。例: `invoice_number_confidence`
- field ごとの evidence。例: `invoice_number_evidence`
- `fields_json`
- `llm_status`
- `llm_confidence`
- `llm_reason`
- `llm_warnings_json`

run step metadata:

- field ごとの抽出成功率
- required field の欠損数
- confidence 分布
- low confidence sample
- provider / schema validation error 件数
- token usage と latency

使う場面:

- OCR 済み請求書から請求番号、支払期限、合計金額、取引先名を抽出する。
- 注文メールから注文番号、商品名、数量、希望納期を抽出する。
- 契約書から契約相手、契約開始日、終了日、自動更新有無を抽出する。

後続 node との接続:

- `confidence_gate`: field confidence が低い行を review に回す。
- `schema_mapping`: 抽出列を標準 schema に合わせる。
- `validate`: 金額、日付、required field を検査する。
- `human_review`: 低 confidence や required 欠損を人手確認する。
- `output`: 確認済みの抽出結果を Work table に保存する。

実装時の注意:

LLM はもっともらしい値を作る可能性があります。必ず evidence text を返させ、evidence が入力 text 内に存在するかを検査します。金額や日付は LLM の値をそのまま信じず、後続の型変換や validation で確認します。

### `llm_classify_document`

目的:

文書や row を、請求書、注文書、契約書、問い合わせ、障害報告などの業務カテゴリへ分類します。

既存 node との違い:

既存の `classify_document` は keyword / regex / priority による rule based 分類です。`llm_classify_document` は、keyword が明示されない文書や、複数カテゴリにまたがる文書でも、文脈から分類候補と理由を返します。

入力:

- `text` または `normalized_text`
- `file_name`
- `mime_type`
- 既存の OCR confidence
- class 定義。例: label、description、examples、negative examples

出力列:

- `document_type`
- `document_type_confidence`
- `document_type_reason`
- `document_type_candidates_json`
- `llm_status`
- `llm_warnings_json`

run step metadata:

- class ごとの件数
- low confidence 件数
- unknown / ambiguous 件数
- top confusion pairs
- prompt version と model

使う場面:

- Drive に置かれた混在文書を請求書、注文書、契約書に分ける。
- 文書種別ごとに後続の抽出 field 定義を変える。
- 問い合わせ文を請求、解約、障害、その他に分類する。

後続 node との接続:

- `route_by_condition`: `document_type` で branch を分ける。
- `llm_extract_fields`: 文書種別ごとに field 定義を変える。
- `confidence_gate`: ambiguous な分類を review へ回す。
- `human_review`: unknown や低 confidence 文書を確認する。

実装時の注意:

分類 class は自由生成させず、必ず候補集合から選ばせます。候補に当てはまらない場合は `unknown` または `needs_review` を返す設計にします。分類理由は短くし、監査や UI で読める形にします。

### `llm_schema_mapping`

目的:

入力列や抽出 field を、社内標準 schema の target column に対応付ける候補を作ります。

既存 node との違い:

既存の `schema_mapping` は、ユーザーが指定した mapping を実行する node です。`schema_inference` は型や欠損傾向を推定します。`llm_schema_mapping` は、列名、sample 値、説明、既存 mapping example を使って、mapping 候補と理由を生成します。

入力:

- source column 名
- source column の sample values
- `schema_inference_json`
- target schema 定義。例: column、type、description、required
- 過去の mapping examples

出力列:

- `schema_mapping_candidates_json`
- `schema_mapping_confidence`
- `schema_mapping_reason`
- `schema_mapping_status`
- 必要なら candidate ごとの `source_column`、`target_column`、`cast`、`default` 候補

run step metadata:

- source column 数
- high confidence mapping 件数
- ambiguous mapping 件数
- unmapped source / target 件数
- target schema coverage

使う場面:

- 取引先ごとに列名が違う Excel / CSV を標準 schema にそろえる。
- OCR / JSON 抽出後の field を分析用 schema に対応付ける。
- 過去の mapping example から新しい input の mapping 候補を作る。

後続 node との接続:

- `schema_mapping`: high confidence 候補をユーザー承認後に config 化する。
- `schema_completion`: 不足列の default / coalesce 候補を作る。
- `human_review`: ambiguous mapping を人が選ぶ。
- `quality_report`: mapping 後の欠損率や型変換失敗を確認する。

実装時の注意:

自動で mapping を確定しすぎると、列の意味を取り違えたまま後続処理が進みます。v1 は「候補生成」に留め、UI で承認して `schema_mapping` config に反映する流れが安全です。sample values は PII を含む可能性があるため、mask と件数制限が必要です。

### `llm_entity_resolution`

目的:

会社名、住所、取引先名、商品名、部署名などの表記ゆれを、既知 entity 候補へ説明付きで対応付けます。

既存 node との違い:

既存の `entity_resolution` は dictionary の name / alias に対する完全一致や包含一致を中心にしています。`llm_entity_resolution` は、住所、略称、法人格、部署名、文脈を使って「同じ可能性が高い」候補を出します。

入力:

- 照合したい列。例: `vendor`、`customer_name`、`address`
- 正規化済み列。例: `vendor_normalized`
- candidate dictionary。例: entity id、canonical name、aliases、address、phone
- 補助列。例: 郵便番号、電話番号、メールドメイン

出力列:

- `<prefix>_entity_id`
- `<prefix>_canonical_name`
- `<prefix>_match_score`
- `<prefix>_match_method`
- `<prefix>_match_reason`
- `<prefix>_match_evidence_json`
- `<prefix>_candidates_json`
- `llm_status`

run step metadata:

- matched / unmatched / needs_review 件数
- score 分布
- candidate dictionary 件数
- ambiguous group 件数
- high risk auto match 件数

使う場面:

- OCR で抽出した取引先名を vendor master に寄せる。
- 表記ゆれのある会社名を同一法人として扱う候補を作る。
- 商品名の略称や型番表記を商品マスタに近づける。

後続 node との接続:

- `confidence_gate`: score が低い match を review へ回す。
- `human_review`: ambiguous match を人が承認する。
- `enrich_join`: entity id 確定後に master data を付与する。
- `deduplicate`: 同一 entity 候補を group 化する。

実装時の注意:

誤結合は業務影響が大きいため、自動採用 threshold を高くします。中間帯は必ず `human_review` に回します。LLM には candidate 集合の中から選ばせ、存在しない entity id を生成させないようにします。

### `llm_table_repair`

目的:

OCR / PDF から抽出した表の header、列ずれ、セル欠損、複数ページ連結を補正する候補を作ります。

既存 node との違い:

既存の `extract_table` は delimiter based の簡易表抽出です。`llm_table_repair` は、`extract_table` や OCR layout の結果を入力にして、壊れた表を業務上扱いやすい行列へ補正する補助 node です。

入力:

- `source_text`
- `row_json`
- `layout_json`
- `boxes_json`
- `page_number`
- 期待 header 定義または target schema

出力列:

- `repaired_row_json`
- `repaired_columns_json`
- `table_repair_confidence`
- `table_repair_reason`
- `cell_warnings_json`
- `llm_status`

run step metadata:

- repair 対象 table 数
- repaired row 数
- low confidence cell 件数
- header 推定成功率
- page merge 件数
- repair failure reason 件数

使う場面:

- 請求書明細の列が OCR でずれた場合。
- PDF の罫線表から header が取れなかった場合。
- 複数ページにまたがる明細表を 1 つの table として扱いたい場合。

後続 node との接続:

- `schema_mapping`: repaired row を標準明細 schema に合わせる。
- `confidence_gate`: 低 confidence cell を review へ回す。
- `human_review`: 表補正結果を人が確認する。
- `quality_report`: 明細件数や欠損率を確認する。

実装時の注意:

表補正は LLM が過剰に推測しやすい領域です。存在しない行や値を作らせないため、evidence cell / source span を必須にします。合計金額との整合性など、数値検査は後続の rule / validate で行います。

### `llm_summarize`

目的:

長文 document、複数 row、review 対象の理由を短く要約します。

既存 node との違い:

既存 node は主に値の抽出や構造化を行います。`llm_summarize` は、UI や review、run detail で人が理解しやすくするための説明を作ります。

入力:

- `text`
- `fields_json`
- `quality_report_json`
- `review_reason_json`
- group key。例: file、customer、document_type

出力列:

- `summary`
- `summary_confidence`
- `summary_evidence_json`
- `summary_warnings_json`
- `llm_status`

run step metadata:

- summarized row / group 件数
- skipped 件数
- average summary length
- low confidence 件数
- token usage

使う場面:

- 契約書や問い合わせ本文を review 用に短く要約する。
- 抽出結果の差分や警告を人が読みやすい形にする。
- document group ごとに処理内容をまとめる。

後続 node との接続:

- `human_review`: review item の説明として使う。
- `output`: summary を Work table に残す。
- `quality_report`: summary 生成対象の欠損や長さを監視する。

実装時の注意:

要約は原文の代替ではありません。重要な判断には evidence や source link を残します。要約に個人情報が混ざる可能性があるため、表示権限と redaction を考慮します。

### `llm_quality_explain`

目的:

`quality_report`、`profile`、`validate`、`confidence_gate` の結果を、人が理解しやすい説明に変換します。

既存 node との違い:

既存の品質 node は件数、欠損率、warning などの structured data を作ります。`llm_quality_explain` は、その structured data をもとに「何が問題か」「どこを見ればよいか」を説明します。

入力:

- `quality_report_json`
- `missing_rate_json`
- `validation_summary_json`
- `gate_status`
- run step metadata の summary

出力列:

- `quality_explanation`
- `quality_risk_level`
- `quality_recommended_action`
- `quality_explanation_evidence_json`
- `llm_status`

run step metadata:

- risk level 件数
- explanation 生成件数
- warning category 件数
- model / prompt version

使う場面:

- schedule run で欠損率が急に悪化した理由を説明する。
- validation error が多いときに、初心者でも次の確認箇所が分かるようにする。
- run detail UI に品質 summary を表示する。

後続 node との接続:

- `human_review`: 品質異常の review reason にする。
- `quarantine`: risk level に応じて隔離対象を選ぶ。
- notification / alert: 将来的に運用通知へつなげる。

実装時の注意:

LLM は原因を断定しすぎる可能性があります。入力 metadata から推測できる範囲だけを説明させ、原因が不明な場合は「可能性」として表現します。数値は LLM に計算させず、既存 metadata の値をそのまま渡します。

### `llm_review_assist`

目的:

`human_review` に回った行について、確認すべき点、修正候補、判断理由を作ります。

既存 node との違い:

既存の `human_review` は review 用の注釈列を作る node です。`llm_review_assist` は、review item を処理する人のために、なぜ確認が必要か、どの値が怪しいか、候補は何かを説明します。

入力:

- `review_reason_json`
- `llm_result_json`
- `evidence_json`
- low confidence field
- source text / row values

出力列:

- `review_assist_summary`
- `review_suggested_values_json`
- `review_checklist_json`
- `review_risk_level`
- `review_assist_confidence`
- `llm_status`

run step metadata:

- review assist 生成件数
- suggested correction 件数
- high risk 件数
- low confidence 件数
- provider error 件数

使う場面:

- 請求書の支払期限や金額が低 confidence のときに、確認ポイントを出す。
- entity resolution の候補が複数あるときに、どこを比べるべきか説明する。
- table repair の低 confidence cell を人が確認しやすくする。

後続 node との接続:

- `human_review`: review queue の説明として使う。
- `sample_compare`: 修正前後の差分を確認する。
- `output`: 承認済みの値だけを最終出力へ渡す。

実装時の注意:

`llm_review_assist` は承認や差し戻しを自動実行しません。あくまで review 補助です。提案値を自動採用する場合は、別途 approval workflow、監査ログ、差し戻し可能な再投入設計が必要です。

## 典型 workflow

### 請求書 OCR から構造化する workflow

```text
input(drive_file)
  -> extract_text
  -> llm_classify_document
  -> route_by_condition(document_type = invoice)
  -> llm_extract_fields
  -> confidence_gate
  -> human_review
  -> schema_mapping
  -> validate
  -> output
```

この workflow では、LLM は文書分類と field 抽出を担当します。低 confidence の値は `confidence_gate` で止め、`human_review` に回します。最終的な schema 整備と検査は `schema_mapping` と `validate` が担当します。

### 取引先名を master data に寄せる workflow

```text
input(work_table)
  -> canonicalize
  -> llm_entity_resolution
  -> confidence_gate
  -> human_review
  -> enrich_join
  -> output
```

この workflow では、`canonicalize` で表記ゆれを減らした後、LLM が entity 候補を作ります。高 confidence のみ自動で進め、中間帯は人手確認します。確定した entity id で `enrich_join` し、master data の属性を付けます。

### 初回取り込み schema を整える workflow

```text
input(dataset)
  -> profile
  -> schema_inference
  -> llm_schema_mapping
  -> human_review
  -> schema_mapping
  -> quality_report
  -> output
```

この workflow では、LLM は mapping 候補を作るだけです。ユーザーが承認した mapping を `schema_mapping` config として使い、`quality_report` で結果を確認します。

## 実装優先順

最初に入れるなら、効果が大きく既存 node と接続しやすい順に進めるのが安全です。

1. `llm_extract_fields`: OCR / text extraction の価値を直接高められる。`confidence_gate` と `human_review` に接続しやすい。
2. `llm_classify_document`: 文書種別ごとに後続処理を分けられる。`route_by_condition` と相性がよい。
3. `llm_schema_mapping`: 初回取り込み時の設定負荷を下げる。自動採用ではなく候補生成に留めやすい。
4. `llm_entity_resolution`: 業務データ統合に効くが誤結合リスクが大きいため、review 前提で入れる。
5. `llm_table_repair`: OCR / PDF 表に効くが、layout と evidence 設計が必要なので後回しにする。
6. `llm_quality_explain`: run detail / UI の理解を助ける。品質 metadata が整ってから入れる。
7. `llm_review_assist`: review queue が本格化してから価値が高くなる。
8. `llm_summarize`: 便利だが、抽出や分類より優先度は低い。

HaoHao の次の課題は、node の数を増やすことだけではなく、処理結果を信頼、監視、説明できるようにすることです。そのため LLM node も、単体で賢く見せるより、confidence、evidence、metadata、review 連携を先に設計します。

## LLM に任せすぎない方がよい処理

LLM を使うべきではない、または LLM だけに任せるべきではない処理もあります。

| 処理 | LLM に任せすぎない理由 |
| --- | --- |
| `join` の最終判定 | 行数爆発、未マッチ、重複 key は SQL と metadata で検査する方が安定する |
| `validate` の最終判定 | required、range、regex、unique などは deterministic rule が向いている |
| 数値計算、集計、合計検算 | ClickHouse の計算結果を正とし、LLM には説明だけさせる |
| PII redaction の唯一手段 | 漏れが許されないため、rule based redaction と権限設計を併用する |
| deduplicate の自動 survivor 決定 | 誤統合の影響が大きく、survivor rule と review が必要 |
| output table の schema 確定 | LLM は候補生成に留め、最終 schema は config と validation で固定する |

LLM は「判断材料を増やす」ために使い、「監査できない最終判断」を増やさないようにします。

## metadata / row column の保存方針

LLM node は保存する情報が多くなりやすいため、保存先を明確に分けます。

| 保存先 | 置くもの |
| --- | --- |
| ClickHouse の行データ列 | 後続 node が読む値、confidence、status、reason、evidence JSON |
| `data_pipeline_run_steps.metadata` | node 単位の summary、model、prompt version、token usage、latency、error count |
| `data_pipeline_run_outputs.metadata` | 最終 output に関する補足。LLM node の詳細は通常ここには置かない |
| audit log | 誰が LLM node を含む pipeline を実行したか、どの tenant policy で許可されたか |
| provider log / local runtime log | provider 通信や runtime debug。ただし機密データの保存は制限する |

行ごとの `llm_result_json` にすべてを詰め込むと、後続 node は使いやすい一方で、ClickHouse の行サイズや preview 表示が重くなります。v1 では、後続処理に必要な最小限の値を通常列へ出し、詳細は JSON、集計は run step metadata に分けるのが扱いやすいです。

LLM node の metadata には、最低限次を残します。

- `model_policy`
- `model`
- `prompt_version`
- `input_rows`
- `completed_rows`
- `needs_review_rows`
- `failed_rows`
- `avg_confidence`
- `token_usage`
- `latency_ms`
- `provider_errors`
- `warnings`

個人情報を含む sample や raw prompt / raw response は、既定では保存しないか、管理者限定、短期 TTL、redaction 済みにします。

## セキュリティ、監査、コストの注意点

LLM node は外部送信、local runtime、prompt、モデル選択、コスト管理が絡むため、通常の SQL node より運用上の注意点が多くなります。

セキュリティ:

- tenant policy で LLM 利用可否、利用可能 model policy、外部送信可否を制御する。
- PII / secret を含む可能性がある列は `redact_pii` 後に渡すか、local runtime のみ許可する。
- prompt injection を考慮し、入力 text に「指示」が含まれていても system prompt や schema を上書きさせない。
- raw response や prompt を保存する場合は、権限、TTL、masking を決める。

監査:

- pipeline version、node id、prompt version、model policy、実行 user、tenant を追えるようにする。
- LLM が生成した値と、人が承認した値を区別する。
- review で修正された場合は、修正前、修正後、担当者、理由を残す。
- provider error や timeout は run step metadata に集約し、運用で見られるようにする。

コスト:

- `maxInputChars`、`batchSize`、`maxRows` を node config または tenant policy で制限する。
- preview では sample rows のみ LLM 実行し、本番 run と区別する。
- 同じ file revision、prompt version、model policy、input hash の結果は cache できる余地がある。
- token usage と latency を metadata に保存し、node ごとのコストを見える化する。

品質:

- LLM 出力は JSON schema validation を通す。
- 型、日付、金額、required は後続の `validate` で検査する。
- confidence threshold は業務影響に応じて高めに設定する。
- 自動処理と人手確認の境界を `confidence_gate` で明示する。

この設計にすると、LLM を使って便利にするだけでなく、失敗したときに「どの node が、どの model / prompt で、何を根拠に、どの confidence で出した結果か」を追えるようになります。
