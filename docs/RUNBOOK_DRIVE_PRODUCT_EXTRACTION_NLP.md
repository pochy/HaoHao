# Drive 商品情報抽出: Python / GiNZA / SudachiPy Runbook

この文書の識別子は `RUNBOOK_DRIVE_PRODUCT_EXTRACTION_NLP` です。

## 目的

Drive OCR の構造化抽出で、LLM を使わない Python 系の商品情報抽出を運用するための手順です。対象 extractor は次の3つです。

| extractor | 依存 | 用途 |
| --- | --- | --- |
| `python` | Python 標準ライブラリのみ | regex と軽量 token 抽出で動作確認したい場合 |
| `ginza` | `spacy` / `ginza` / `ja_ginza` | GiNZA の名詞句・固有表現を商品名候補に使う場合 |
| `sudachipy` | `sudachipy` | 日本語形態素解析を軽く使いたい場合。大きいOCRではまずこれを推奨 |

Go 本体は Python package を import しません。抽出時に `python3 backend/internal/service/scripts/drive_product_extraction_nlp.py` を起動し、OCR全文・ページ・rules設定・modeをJSON stdinで渡します。

## 前提

- repo root で作業する。
- DB migration `0025_drive_ocr_python_nlp_extractors` が適用済みである。
- backend process の `PATH` で、必要なpackageが入った `python3` が先に見つかる。
- GiNZA / SudachiPy はアプリが自動インストールしない。運用側で事前に配置する。
- 依存不足時に `rules` へ silent fallback しない。Runtime Status は unavailable になり、抽出実行時は明示的に失敗する。

## 初期セットアップ

repo root に venv を作ります。

```bash
python3 -m venv venv
source venv/bin/activate
python3 -m pip install --upgrade pip
python3 -m pip install spacy ja_ginza sudachipy
```

最小構成だけ確認したい場合は `python` mode だけなら追加 package は不要です。`ginza` / `sudachipy` を選ぶ場合は上記 package を入れてください。

DB migration を適用します。

```bash
make db-up
```

constraint に3値が入っていることを確認します。

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select version, dirty from schema_migrations order by version desc limit 1; select pg_get_constraintdef(oid) from pg_constraint where conname='drive_ocr_runs_structured_extractor_check';\""
```

`python`、`ginza`、`sudachipy` が `drive_ocr_runs_structured_extractor_check` に含まれていればOKです。

## helper の単体確認

backend を起動する前に、helper を直接確認します。

```bash
PATH="$PWD/venv/bin:$PATH" python3 backend/internal/service/scripts/drive_product_extraction_nlp.py --check python
PATH="$PWD/venv/bin:$PATH" python3 backend/internal/service/scripts/drive_product_extraction_nlp.py --check ginza
PATH="$PWD/venv/bin:$PATH" python3 backend/internal/service/scripts/drive_product_extraction_nlp.py --check sudachipy
```

期待値:

```text
python helper 3.x.x
ginza helper available
sudachipy helper available
```

抽出の最小サンプル:

```bash
printf '%s' '{"mode":"sudachipy","text":"商品名: テスト商品\nJANコード: 4901111111111\n価格: 1,280円\n内容量: 500g","rules":{"candidateScoreThreshold":4,"maxBlockRunes":3000,"contextWindowRunes":800,"priceExtractionEnabled":true},"limits":{"maxItems":5}}' \
  | PATH="$PWD/venv/bin:$PATH" python3 backend/internal/service/scripts/drive_product_extraction_nlp.py extract
```

`{"items":[...]}` が返れば helper は動作しています。

## backend の起動

backend は `python3` を外部コマンドとして起動します。venv を有効化した状態、または `PATH` を明示した状態で起動してください。

```bash
source venv/bin/activate
make backend-dev
```

または:

```bash
PATH="$PWD/venv/bin:$PATH" make backend-dev
```

起動済み backend の `PATH` を確認したい場合:

```bash
lsof -nP -iTCP:8080 -sTCP:LISTEN
ps eww -p "$(lsof -tiTCP:8080 -sTCP:LISTEN | head -1)" | tr ' ' '\n' | rg '^PATH='
```

`/path/to/repo/venv/bin` が先頭付近に出ればOKです。

helper の場所を差し替えたい場合は `HAOHAO_DRIVE_PRODUCT_NLP_HELPER` を使えます。

```bash
export HAOHAO_DRIVE_PRODUCT_NLP_HELPER="$PWD/backend/internal/service/scripts/drive_product_extraction_nlp.py"
PATH="$PWD/venv/bin:$PATH" make backend-dev
```

通常は設定不要です。

## 管理画面での設定

Tenant Admin の Drive Policy で設定します。

1. OCR を有効化する。
2. Structured extraction を有効化する。
3. Structured extractor で `Non-LLM Python` から選ぶ。
4. 初回運用では `SudachiPy` を推奨する。
5. 必要に応じて `Rules` 調整値を変更する。

推奨初期値:

```json
{
  "candidateScoreThreshold": 4,
  "maxBlockRunes": 3000,
  "contextWindowRunes": 800,
  "priceExtractionEnabled": true
}
```

カタログ全体などOCRが広く、会社概要・利用規約・説明文まで拾う場合は `candidateScoreThreshold` を上げます。実例では `16` まで上げると JAN / 価格 / 型番付き候補に寄せられました。

Runtime Status の `Local runtime` に `python`、`ginza`、`sudachipy` が表示されます。選択中の extractor は `enabled / available` になることを確認してください。

## 商品情報抽出の実行

通常は Drive file detail 画面から実行します。

1. 対象ファイルを開く。
2. OCR が `completed` であることを確認する。
3. 商品情報抽出セクションで再実行ボタンを押す。
4. 抽出結果テーブルを確認する。

API では次の endpoint を使います。

```text
POST /api/v1/drive/files/{filePublicId}/product-extractions/jobs
GET  /api/v1/drive/files/{filePublicId}/product-extractions
```

POST は session cookie と CSRF token が必要です。手動確認ではUIからの実行を推奨します。

## 既存OCRから作り直す時の挙動

商品情報抽出の再実行は、最新の完了済み OCR run と OCR pages を使います。ファイル本体の再OCRは不要です。

ただし、OCR run の `structured_extractor` はOCR作成時点の値です。UIの「商品情報抽出エンジン」表示もこの値を参照します。既存OCRを別extractorで再利用して結果だけ入れ替える場合は、次のどちらかを選びます。

- UI/APIの通常フローで、最新OCR run の extractor と同じ方式で再抽出する。
- 検証用にDBでOCR runをコピーし、`structured_extractor` を新しい方式にした完了runを作る。

後者は開発環境向けです。本番運用では通常フローを優先してください。

## 出力される attributes

Python系 helper は `drive_product_extraction_items.attributes` に次の情報を保存します。

| key | 意味 |
| --- | --- |
| `extractor` | `python` / `ginza` / `sudachipy` |
| `pythonHelper` | helper filename |
| `schemaVersion` | attributes schema version |
| `nlpEngine` | NLP engine名 |
| `nounPhrases` | NLP / regex で拾った名詞句候補 |
| `rulesScore` | candidate block score |
| `rulesCandidateThreshold` | 実行時の閾値 |
| `nameDerivedFrom` | `label` / `row` / `nearbyHeading` / `nearby` / `nounPhrase` / `model` / `sku` / `janCode` |
| `capacity` / `size` / `color` | 抽出できた補助属性 |

DB schema は変更せず、補助情報は `attributes` に入れます。

## 手元での実データ確認

対象ファイルの抽出結果をDBで確認する例です。

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select i.name, i.brand, i.model, i.jan_code, i.price, i.attributes->>'extractor' as extractor, i.attributes->>'nlpEngine' as nlp_engine, i.confidence from drive_product_extraction_items i join file_objects f on f.id=i.file_object_id where f.public_id='<filePublicId>' order by i.created_at desc, i.id;\""
```

最新OCR run の extractor を確認する例です。

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select r.public_id, r.structured_extractor, r.status, r.page_count, length(r.extracted_text) as text_len, r.created_at from drive_ocr_runs r join file_objects f on f.id=r.file_object_id where f.public_id='<filePublicId>' order by r.created_at desc limit 5;\""
```

## 注意点

- backend 起動時の `PATH` が重要です。shellで `venv` をactivateしていても、既に起動済みのbackendには反映されません。再起動してください。
- `.air.toml` は `.py` 変更でGo binaryを再ビルドしません。ただし helper は抽出実行時に毎回 `python3` で起動されるため、helperファイルの変更は次回抽出に反映されます。
- `ginza` は大きいOCR全文で処理時間が長くなることがあります。50ページ級のカタログではまず `sudachipy` を使い、必要なページや閾値を絞ってから `ginza` を試してください。
- `python` mode は依存が少ない反面、日本語の名詞句補強は弱いです。疎通確認やfallback的な軽量検証に向きます。
- `rules` への自動fallbackはありません。依存不足は `unavailable` として見える状態にし、実行時も失敗させます。
- `ollama` / `lmstudio` のURL・モデル設定はPython系では使いません。
- helper は外部ネットワークを使いません。model / dictionary の自動downloadもしません。
- `db/migrations/0025_drive_ocr_python_nlp_extractors.down.sql` は `python` / `ginza` / `sudachipy` の既存runを `rules` に戻してからconstraintを戻します。

## トラブルシューティング

### Runtime Status が unavailable

まず backend process の `PATH` を確認します。

```bash
ps eww -p "$(lsof -tiTCP:8080 -sTCP:LISTEN | head -1)" | tr ' ' '\n' | rg '^PATH='
```

次に同じPATHで helper check を実行します。

```bash
PATH="$PWD/venv/bin:$PATH" python3 backend/internal/service/scripts/drive_product_extraction_nlp.py --check sudachipy
PATH="$PWD/venv/bin:$PATH" python3 backend/internal/service/scripts/drive_product_extraction_nlp.py --check ginza
```

`python3` が system Python を指している場合は、`PATH="$PWD/venv/bin:$PATH" make backend-dev` で再起動してください。

### `structured_extractor` のDB保存で constraint error

DB migration 25 が未適用です。

```bash
make db-up
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select version, dirty from schema_migrations order by version desc limit 1;\""
```

`version=25` 以上になっていることを確認してください。

### GiNZA が `compound_splitter.split_mode` で落ちる

`ja_ginza` 5.2.0 と `spacy` 3.8系の組み合わせで、通常の `spacy.load("ja_ginza")` が次のように落ちることがあります。

```text
Config error for 'compound_splitter'
compound_splitter -> split_mode at root: None is not <class 'str'>
```

repo内helperは `split_mode: "C"` を明示して再ロードする互換処理を持っています。helperが古い場合は最新の `backend/internal/service/scripts/drive_product_extraction_nlp.py` に更新してください。

### GiNZA が遅い

長いOCR全文をGiNZAに渡すと、model load と解析に時間がかかります。まず次を試してください。

- `sudachipy` を使う。
- `candidateScoreThreshold` を上げる。
- OCR対象ページ数を減らす。
- 商品表のあるページだけで helper を単体検証する。

### ノイズ商品が多い

rules score 閾値を上げます。

```json
{
  "candidateScoreThreshold": 16
}
```

特に総合カタログでは会社概要、特集見出し、説明文、保証サービスが商品候補として拾われやすいため、JAN・価格・型番が揃う候補に寄せる調整が必要です。

### 商品表で1件しか出ない

OCRが「1行に複数の商品名/JAN、次行に複数の型番/価格」の形になる場合があります。helperは代表的な横並び表を `name + JAN` と `model + price` の順番で対応付けます。期待した商品が出ない場合は、該当ページのOCRテキストを確認してください。

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select page_number, raw_text from drive_ocr_pages where ocr_run_id=<ocrRunId> and raw_text ilike '%<型番またはJAN>%';\""
```

OCR textの並びが崩れている場合は、rulesだけで完全に復元するのは難しいため、ページ範囲、OCR engine、またはextractorを見直します。

### UIに古いextractor名が出る

商品情報抽出ステータスのengine表示は最新OCR runの `structured_extractor` を見ます。既存OCR runを別extractorで再利用してitemsだけ差し替えた場合、表示が古いままになることがあります。開発検証ではOCR runをコピーして新extractorの完了runを作るか、対象extractorでOCR jobから作り直してください。

### `__pycache__` ができる

helper check や `py_compile` で `backend/internal/service/scripts/__pycache__` が生成されます。repoに含めないでください。

```bash
rm -rf backend/internal/service/scripts/__pycache__
```

## 推奨確認コマンド

変更後の基本確認です。

```bash
go test ./backend/internal/service -run 'PythonNLP|DriveOCRPolicy|DriveProduct|LocalCommand|Migration'
go test ./backend/...
npm --prefix frontend run build
git diff --check
```
