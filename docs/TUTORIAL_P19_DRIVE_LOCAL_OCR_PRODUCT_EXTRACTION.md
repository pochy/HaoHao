# P19: Drive ローカル OCR / 商品情報抽出チュートリアル

この文書の識別子は `TUTORIAL_P19_DRIVE_LOCAL_OCR_PRODUCT_EXTRACTION` です。

Python / GiNZA / SudachiPy を使った非LLMの商品情報抽出の環境構築・運用手順は、実装チュートリアルではなく runbook として `docs/RUNBOOK_DRIVE_PRODUCT_EXTRACTION_NLP.md` にまとめています。

## この文書の目的

この文書は、HaoHao 内蔵 Drive に **ローカル OCR と商品・販促情報抽出** を追加するための実装チュートリアルです。

対象は、Drive にアップロードされた PDF / 画像から OCR テキストを作り、さらに商品・販促単位の中間構造化データを保存する流れです。

この文書の目的は、OCR 技術の比較表を作ることではありません。目的は、既存の Drive / outbox / tenant settings / search index の形に沿って、次を迷わず実装できる順番に落とすことです。

- どの設定を tenant Drive policy に追加するか
- どの DB table を正本として追加するか
- upload / overwrite 後にどこで非同期ジョブを投入するか
- OCR worker と provider 境界をどこに置くか
- OCR 結果を Drive 検索と UI にどう反映するか
- local-only 実行をどう守るか

この P19 では、最初の実装を `Tesseract + Poppler + rules` に寄せます。Docling、PaddleOCR、Ollama は同じ interface の optional mode として追加できる形にし、default path を重くしません。

この文書は、次の既存チュートリアルの後続です。

- `docs/TUTORIAL.md`
- `docs/TUTORIAL_SINGLE_BINARY.md`
- `docs/TUTORIAL_P16_DRIVE_FEATURE_COMPLETION.md`
- `docs/TUTORIAL_P17_I18N.md`
- `docs/TUTORIAL_P18_TENANT_DETAIL_SUBPAGES.md`
- `docs/TUTORIAL_OPENFGA_P9_DRIVE_PRODUCT_COMPLETION.md`

## 前提と現在地

このチュートリアルは、現在の HaoHao repository が少なくとも次の状態にある前提で進めます。

- Drive file / folder / share / search / preview の基本機能がある
- upload は `DriveService.UploadFile`、本文更新は `DriveService.OverwriteFile` を通る
- Drive 検索は `drive_search_documents` に indexed text を保存している
- Drive AI は Phase 9 の fake provider による summary / classification として存在する
- tenant Drive policy は `tenant_settings.features.drive` に JSON として保存している
- outbox は `outbox_events` と `DefaultOutboxHandler` の event type dispatch で処理している
- zero-knowledge E2EE file では server が plaintext を読めない
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build`、`make binary` が標準確認コマンドとして使える

ローカル OCR runtime は次を想定します。

| 領域 | default | optional |
| --- | --- | --- |
| PDF text extraction | `pdftotext` | - |
| PDF rasterize | `pdftoppm -r 300` | ImageMagick fallback |
| OCR | `tesseract` | Docling / PaddleOCR |
| structured extraction | rules | Ollama / Docling |

ローカル環境に `tesseract`、Poppler、ImageMagick があっても、日本語 OCR には `jpn`、必要に応じて `jpn_vert` の traineddata が必要です。Docling、PaddleOCR、Ollama 連携は別途 runtime と model を事前配置します。

### やらないこと

- upload request の中で同期 OCR を実行しない
- 実行時に外部 OCR / LLM API へ file body や OCR text を送信しない
- runtime 中に model を自動 download しない
- Drive AI fake provider を OCR の実装として流用しない
- OpenFGA の authorization model を OCR のために広げない
- `openapi/openapi.yaml`、`frontend/src/api/generated/*`、`backend/internal/db/*` を手書き編集しない
- DLP blocked、infected / blocked、zero-knowledge E2EE file を OCR しない
- OCR 結果を source file の権限なしで読める別 resource として扱わない

OCR / 商品抽出結果は source file から派生した content です。保存済み結果を読むときも、source file の DB guard と Drive authorization check を必ず通します。

## 完成条件

このチュートリアルの完了条件は次です。

- tenant Drive policy に `features.drive.ocr` が保存できる
- 管理画面の Drive policy で OCR engine、抽出方式、言語、上限ページ、timeout、Ollama 設定を変更できる
- `drive_ocr_runs`、`drive_ocr_pages`、`drive_product_extraction_items` が migration と sqlc query で管理される
- Drive upload / overwrite の DB commit 後に `drive.ocr.requested` outbox event が enqueue される
- outbox worker が OCR service に dispatch し、成功 / 失敗 / skip を DB に残す
- PDF / PNG / JPEG / TIFF / WebP だけが OCR 対象になる
- deleted、DLP blocked、infected / blocked、zero-knowledge E2EE file は `skipped` になる
- born-digital PDF は `pdftotext` を優先し、文字量が少ない page だけ画像化して OCR する
- default mode では `Tesseract + Poppler + rules` だけで完了する
- optional mode として Docling / PaddleOCR / Ollama を選べる
- `ollamaBaseURL` は `127.0.0.1` / `localhost` のみ許可される
- OCR 全文が `drive_search_documents.extracted_text` に反映され、Drive 検索で見つかる
- Browser API から OCR status / page text / product extraction を取得できる
- Admin API から local OCR dependency status を取得できる
- Drive details / preview で OCR 状態、再実行、抽出商品一覧が見える
- `make gen`、`go test ./backend/...`、`npm --prefix frontend run build` が通る
- sample PDF / image を使った manual smoke が外部ネットワークなしで完了する

## 手で書くファイルと生成物

### 手で書くファイル

```text
db/migrations/0022_drive_local_ocr_product_extraction.up.sql
db/migrations/0022_drive_local_ocr_product_extraction.down.sql
db/queries/drive_ocr.sql
backend/internal/service/tenant_settings_service.go
backend/internal/service/tenant_settings_service_test.go
backend/internal/service/drive_ocr_service.go
backend/internal/service/drive_ocr_provider.go
backend/internal/service/drive_ocr_tesseract.go
backend/internal/service/drive_product_extraction.go
backend/internal/service/outbox_handler.go
backend/internal/service/drive_service.go
backend/internal/service/drive_service_api.go
backend/internal/api/drive_ocr.go
backend/internal/api/tenant_admin_drive_ocr.go
backend/cmd/main/main.go
backend/internal/app/openapi.go
frontend/src/api/drive.ts
frontend/src/api/tenant-admin.ts
frontend/src/stores/drive.ts
frontend/src/tenant-admin/detail-context.ts
frontend/src/views/tenant-admin/TenantAdminTenantDrivePolicyView.vue
frontend/src/components/DriveDetailsPanel.vue
frontend/src/i18n/messages.ts
e2e/drive.spec.ts
```

### 更新されるが手書きしない生成物

```text
db/schema.sql
backend/internal/db/*
openapi/openapi.yaml
openapi/browser.yaml
openapi/external.yaml
frontend/src/api/generated/*
backend/web/dist/*
```

生成物は `make gen`、frontend build、OpenAPI export から作ります。editor で直接直しません。

## 実装順の全体像

| Step | 対象 | 目的 |
| --- | --- | --- |
| Step 1 | inventory | Drive upload / overwrite / search / AI / policy の差し込み口を確認する |
| Step 2 | tenant policy | `features.drive.ocr` の型、default、validation を追加する |
| Step 3 | DB / sqlc | OCR run、page、product extraction の正本を作る |
| Step 4 | service boundary | OCR provider と structured extractor の interface を固定する |
| Step 5 | enqueue | upload / overwrite commit 後に `drive.ocr.requested` を投入する |
| Step 6 | worker | outbox handler から OCR service を呼ぶ |
| Step 7 | default OCR | Tesseract / Poppler / rules の local 実行を実装する |
| Step 8 | optional engines | Docling / PaddleOCR / Ollama を選択式にする |
| Step 9 | search index | OCR 全文を `drive_search_documents` に反映する |
| Step 10 | API | Browser API と Admin status API を追加する |
| Step 11 | frontend | Drive details と tenant admin Drive policy に接続する |
| Step 12 | verification | unit / service / DB / frontend / manual smoke を通す |

## Step 1. 差し込み口を棚卸しする

### 対象ファイル

```text
backend/internal/service/drive_service.go
backend/internal/service/drive_service_api.go
backend/internal/service/drive_collaboration_governance_service.go
backend/internal/service/drive_enterprise_integrations_service.go
backend/internal/service/outbox_handler.go
backend/internal/service/tenant_settings_service.go
backend/internal/api/drive_files.go
backend/internal/api/drive_office_ai_security.go
backend/internal/api/tenant_settings.go
db/queries/drive_search.sql
frontend/src/components/DriveDetailsPanel.vue
frontend/src/views/tenant-admin/TenantAdminTenantDrivePolicyView.vue
frontend/src/tenant-admin/detail-context.ts
```

### 確認コマンド

```bash
rg -n "UploadFile|OverwriteFile|CreateAIJob|drive_search_documents|UpsertDriveSearchDocument|DefaultOutboxHandler|features.drive|DrivePolicy" backend db frontend/src
rg -n "drive_ai|drive_index_jobs|drive_search_documents|outbox_events" db/migrations db/schema.sql
rg -n "DriveDetailsPanel|Drive policy|aiEnabled|searchEnabled" frontend/src
```

### 見るポイント

- upload / overwrite は request body 処理後に DB commit している
- `drive_search_documents` は検索用の派生 state であり、file body の正本ではない
- Drive AI summary / classification は fake provider で同期処理している
- OCR は Drive AI とは別の非同期 pipeline にする
- tenant Drive policy は `features.drive` の JSON normalize / save に統合する
- outbox handler は unknown event を error にするため、`drive.ocr.requested` を明示 dispatch する

この Step ではまだ code を変えません。どこに変更を入れるかを PR description または実装メモにまとめます。

## Step 2. tenant Drive policy に OCR 設定を追加する

### 対象ファイル

```text
backend/internal/service/tenant_settings_service.go
backend/internal/service/tenant_settings_service_test.go
frontend/src/tenant-admin/detail-context.ts
frontend/src/views/tenant-admin/TenantAdminTenantDrivePolicyView.vue
frontend/src/i18n/messages.ts
```

### policy shape

`features.drive.ocr` は、既存 `features.drive` の nested object として保存します。

```json
{
  "drive": {
    "ocr": {
      "enabled": false,
      "ocrEngine": "tesseract",
      "ocrLanguages": ["jpn", "eng"],
      "structuredExtractionEnabled": false,
      "structuredExtractor": "rules",
      "maxPages": 20,
      "timeoutSecondsPerPage": 30,
      "ollamaBaseURL": "http://127.0.0.1:11434",
      "ollamaModel": ""
    }
  }
}
```

### backend validation

`DrivePolicy` に OCR 用の nested struct を追加します。

```go
type DriveOCRPolicy struct {
	Enabled                     bool
	OCREngine                   string
	OCRLanguages                []string
	StructuredExtractionEnabled bool
	StructuredExtractor         string
	MaxPages                    int
	TimeoutSecondsPerPage       int
	OllamaBaseURL               string
	OllamaModel                 string
}
```

default は次にします。

| key | default | 理由 |
| --- | --- | --- |
| `enabled` | `false` | 既存 tenant の upload 挙動を変えない |
| `ocrEngine` | `tesseract` | 最小 runtime で始める |
| `ocrLanguages` | `["jpn", "eng"]` | 日本語資料と英数字商品コードを同時に読む |
| `structuredExtractionEnabled` | `false` | まず OCR text 保存だけでも価値がある |
| `structuredExtractor` | `rules` | 外部 API なし、predictable |
| `maxPages` | `20` | upload 後 job の runaway を避ける |
| `timeoutSecondsPerPage` | `30` | page 単位で停止できる |
| `ollamaBaseURL` | `http://127.0.0.1:11434` | local-only を強制しやすい |
| `ollamaModel` | `""` | 未設定なら Ollama extraction は使えない |

validation は次を守ります。

- `ocrEngine` は `tesseract` / `docling` / `paddleocr`
- `structuredExtractor` は `rules` / `ollama` / `docling`
- `ocrLanguages` は空にしない
- language code は英数字、underscore、hyphen だけに正規化する
- `maxPages` は `1..200`
- `timeoutSecondsPerPage` は `1..300`
- `ollamaBaseURL` は `http://127.0.0.1:*`、`http://localhost:*` だけを許可する
- `structuredExtractor=ollama` かつ structured extraction enabled の場合は `ollamaModel` を必須にする

### frontend

tenant admin Drive policy page に OCR section を追加します。

追加する control は次です。

- OCR enabled checkbox
- OCR engine select: `tesseract`, `docling`, `paddleocr`
- languages input: comma separated `jpn, eng`
- structured extraction enabled checkbox
- structured extractor select: `rules`, `ollama`, `docling`
- max pages number input
- timeout seconds per page number input
- Ollama base URL input
- Ollama model input

`saveCommonSettings` では既存 `features.drive` を壊さず、`ocr` nested object を含めて保存します。

## Step 3. DB / sqlc を追加する

### 対象ファイル

```text
db/migrations/0022_drive_local_ocr_product_extraction.up.sql
db/migrations/0022_drive_local_ocr_product_extraction.down.sql
db/queries/drive_ocr.sql
backend/sqlc.yaml
```

### `drive_ocr_runs`

file revision / hash ごとの OCR 実行状態を保存します。

```sql
CREATE TABLE drive_ocr_runs (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    file_revision TEXT NOT NULL DEFAULT '1',
    content_sha256 TEXT,
    engine TEXT NOT NULL,
    languages TEXT[] NOT NULL DEFAULT ARRAY['jpn', 'eng'],
    structured_extractor TEXT NOT NULL DEFAULT 'rules',
    status TEXT NOT NULL DEFAULT 'pending',
    reason TEXT NOT NULL DEFAULT 'upload',
    page_count INTEGER NOT NULL DEFAULT 0,
    processed_page_count INTEGER NOT NULL DEFAULT 0,
    average_confidence NUMERIC(5,4),
    extracted_text TEXT NOT NULL DEFAULT '',
    error_code TEXT,
    error_message TEXT,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_ocr_runs_engine_check CHECK (engine IN ('tesseract', 'docling', 'paddleocr')),
    CONSTRAINT drive_ocr_runs_structured_extractor_check CHECK (structured_extractor IN ('rules', 'ollama', 'docling')),
    CONSTRAINT drive_ocr_runs_status_check CHECK (status IN ('pending', 'running', 'completed', 'failed', 'skipped'))
);
```

必要な index:

- unique: `(file_object_id, file_revision, content_sha256, engine, structured_extractor)`
- unique: `public_id`
- partial pending: `(tenant_id, created_at, id) WHERE status IN ('pending', 'running')`
- list: `(tenant_id, file_object_id, created_at DESC)`
- status: `(tenant_id, status, created_at DESC)`

### `drive_ocr_pages`

page 単位の raw text、confidence、layout / box 情報を保存します。

```sql
CREATE TABLE drive_ocr_pages (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    ocr_run_id BIGINT NOT NULL REFERENCES drive_ocr_runs(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    page_number INTEGER NOT NULL,
    raw_text TEXT NOT NULL DEFAULT '',
    average_confidence NUMERIC(5,4),
    layout_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    boxes_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_ocr_pages_page_number_check CHECK (page_number > 0)
);
```

必要な index:

- unique: `(ocr_run_id, page_number)`
- list: `(tenant_id, file_object_id, page_number)`

### `drive_product_extraction_items`

商品 / 販促候補の中間データを保存します。

```sql
CREATE TABLE drive_product_extraction_items (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    ocr_run_id BIGINT NOT NULL REFERENCES drive_ocr_runs(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    item_type TEXT NOT NULL,
    name TEXT NOT NULL,
    brand TEXT,
    manufacturer TEXT,
    model TEXT,
    sku TEXT,
    jan_code TEXT,
    category TEXT,
    description TEXT,
    price JSONB NOT NULL DEFAULT '{}'::jsonb,
    promotion JSONB NOT NULL DEFAULT '{}'::jsonb,
    availability JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_text TEXT NOT NULL DEFAULT '',
    evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    confidence NUMERIC(5,4),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_product_extraction_items_item_type_check CHECK (item_type IN ('product', 'promotion', 'bundle', 'unknown'))
);
```

必要な index:

- unique: `public_id`
- list: `(tenant_id, file_object_id, created_at DESC)`
- run: `(ocr_run_id, id)`
- optional lookup: `(tenant_id, jan_code) WHERE jan_code IS NOT NULL`

### down migration

down migration は依存順を逆にします。

```sql
DROP TABLE IF EXISTS drive_product_extraction_items;
DROP TABLE IF EXISTS drive_ocr_pages;
DROP TABLE IF EXISTS drive_ocr_runs;
```

### sqlc query

`db/queries/drive_ocr.sql` に最低限次を追加します。

- `CreateDriveOCRRun`
- `GetDriveOCRRunByPublicID`
- `GetLatestDriveOCRRunForFile`
- `ListDriveOCRPages`
- `ListDriveProductExtractionItems`
- `MarkDriveOCRRunRunning`
- `MarkDriveOCRRunCompleted`
- `MarkDriveOCRRunFailed`
- `MarkDriveOCRRunSkipped`
- `UpsertDriveOCRPage`
- `DeleteDriveProductExtractionItemsForRun`
- `CreateDriveProductExtractionItem`

## Step 4. OCR service boundary を作る

### 対象ファイル

```text
backend/internal/service/drive_ocr_service.go
backend/internal/service/drive_ocr_provider.go
backend/internal/service/drive_product_extraction.go
backend/internal/service/drive_types.go
```

### provider interface

OCR provider は file body を受け取り、page 単位の結果を返します。

```go
type DriveOCRProvider interface {
	Name() string
	Check(ctx context.Context) DriveOCRDependencyStatus
	Extract(ctx context.Context, input DriveOCRProviderInput) (DriveOCRProviderResult, error)
}
```

`DriveOCRProviderInput` は次を持ちます。

- tenant id
- file public id
- content type
- original filename
- storage body reader
- policy
- timeout per page
- max pages

`DriveOCRProviderResult` は次を持ちます。

- pages
- full text
- average confidence
- page count
- layout / boxes JSON
- warnings

### structured extractor interface

商品抽出は OCR provider とは分けます。

```go
type DriveProductExtractor interface {
	Name() string
	ExtractProducts(ctx context.Context, input DriveProductExtractionInput) (DriveProductExtractionResult, error)
}
```

default の `rules` extractor は正規表現、辞書、価格 / 期間 parser で堅く抽出します。Ollama / Docling は同じ interface に差し込む optional 実装にします。

### 中間 schema v1

document metadata:

```json
{
  "documentKind": "flyer",
  "issuerName": "Example Store",
  "validFrom": "2026-04-01",
  "validUntil": "2026-04-30",
  "currency": "JPY",
  "language": "ja",
  "confidence": 0.88
}
```

item:

```json
{
  "itemType": "product",
  "name": "商品名",
  "brand": "",
  "manufacturer": "",
  "model": "",
  "sku": "",
  "janCode": "",
  "category": "",
  "description": "",
  "price": {
    "amount": 198,
    "currency": "JPY",
    "taxIncluded": true
  },
  "promotion": {
    "label": "特価",
    "validFrom": "2026-04-01",
    "validUntil": "2026-04-07"
  },
  "availability": {
    "inStock": true,
    "limitedQuantity": false
  },
  "sourceText": "OCR text around item",
  "evidence": [
    {
      "pageNumber": 1,
      "text": "source span",
      "box": {"x": 0, "y": 0, "w": 100, "h": 40}
    }
  ],
  "attributes": {},
  "confidence": 0.77
}
```

DB には schema version を列として持たず、`attributes` に `schemaVersion: 1` を入れます。将来 v2 が必要になった場合は、列追加または dedicated version column を別 migration で追加します。

## Step 5. upload / overwrite 後に enqueue する

### 対象ファイル

```text
backend/internal/service/drive_service.go
backend/internal/service/drive_service_api.go
backend/internal/service/outbox_service.go
```

### 方針

OCR は upload / overwrite の request path では実行しません。DB commit 後に outbox event を enqueue します。

payload は file public id ではなく DB id も含めます。ただし外部 API に出す値ではないため、outbox 内部 payload として扱います。

```json
{
  "tenantId": 1,
  "fileObjectId": 123,
  "filePublicId": "uuid",
  "actorUserId": 456,
  "reason": "upload"
}
```

`reason` は次にします。

- `upload`
- `overwrite`
- `manual`
- `rebuild`

### idempotency

同じ file revision / content hash / engine / structured extractor で OCR run がある場合は、既存 run を使います。

- `completed`: 新しい run を作らず既存結果を返す
- `pending` / `running`: duplicate enqueue を避ける
- `failed`: manual retry では新しい run を作る
- `skipped`: policy / file state が変わった manual retry では再評価する

## Step 6. outbox worker に接続する

### 対象ファイル

```text
backend/internal/service/outbox_handler.go
backend/cmd/main/main.go
backend/internal/app/openapi.go
```

`DefaultOutboxHandler` に OCR service を extra dependency として受け取れるようにします。

```go
type DefaultOutboxHandler struct {
	// existing fields
	driveOCR *DriveOCRService
}
```

dispatch に `drive.ocr.requested` を追加します。

```go
case "drive.ocr.requested":
	var payload struct {
		TenantID     int64  `json:"tenantId"`
		FileObjectID int64  `json:"fileObjectId"`
		FilePublicID string `json:"filePublicId"`
		ActorUserID  int64  `json:"actorUserId"`
		Reason       string `json:"reason"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	if h.driveOCR == nil {
		return nil
	}
	return h.driveOCR.HandleRequested(ctx, payload.TenantID, payload.FileObjectID, payload.ActorUserID, payload.Reason, event.ID)
```

OpenAPI export 用 dependency では OCR service を nil-safe にします。OpenAPI generation は actual OCR runtime に依存させません。

## Step 7. default OCR を実装する

### 対象ファイル

```text
backend/internal/service/drive_ocr_tesseract.go
backend/internal/service/drive_ocr_commands.go
backend/internal/service/drive_product_extraction.go
```

### 対象 MIME type

対象は次だけです。

| MIME type | 処理 |
| --- | --- |
| `application/pdf` | `pdftotext` 優先、必要 page だけ rasterize |
| `image/png` | OCR |
| `image/jpeg` | OCR |
| `image/tiff` | OCR |
| `image/webp` | ImageMagick で変換して OCR |

その他は `skipped` にします。

### skip 条件

worker は処理開始時に file state を再読込し、次なら `skipped` にします。

- file が存在しない
- tenant が一致しない
- deleted
- scan status が `infected` / `blocked`
- `dlp_blocked = true`
- encryption mode が zero-knowledge
- tenant policy の OCR が disabled
- MIME type が対象外

### PDF flow

PDF は最初に `pdftotext` を実行します。

```bash
pdftotext -layout -f 1 -l "${maxPages}" input.pdf output.txt
```

page ごとの文字量が十分なら、その page は born-digital text として保存します。文字量が少ない page だけ `pdftoppm` で画像化します。

```bash
pdftoppm -r 300 -png -f "${page}" -l "${page}" input.pdf page
```

画像化した page は Tesseract に渡します。

```bash
tesseract page.png stdout -l jpn+eng --psm 6 tsv
```

### Tesseract language

`ocrLanguages` は Tesseract CLI では `jpn+eng` のように `+` で結合します。

日本語資料では `jpn` を必須にし、縦書き資料が対象なら運用側で `jpn_vert` を追加します。

dependency status API では次を返します。

- `tesseract` binary の有無
- `tesseract --version`
- `tesseract --list-langs` に含まれる language
- `pdftotext` / `pdftoppm` の有無
- ImageMagick `magick` または `convert` の有無

### rules extraction

rules extractor は最初の実装では次だけを狙います。

- JAN code: 8 桁 / 13 桁
- 価格: `¥198`、`198円`、`税込198円`
- 期間: `4/1-4/7`、`2026年4月1日から`
- 商品名候補: 価格行の近傍 text
- promotion label: `特価`、`セール`、`割引`、`ポイント`

精度よりも、失敗時に OCR text が残ることを優先します。rules で抽出できなかった場合も OCR run は `completed` にします。

## Step 8. optional engines を追加する

### Docling

Docling mode は local helper process として実行します。

- input file path を渡す
- output は JSON
- layout / table / read order を `drive_ocr_pages.layout_json` に保存する
- structured extraction を Docling に寄せる場合は `drive_product_extraction_items` に変換して保存する

Docling package / model は runtime 起動時に download しません。運用で事前配置します。

### PaddleOCR

PaddleOCR mode は日本語精度、表、読み順を優先した optional mode です。

- `backend/internal/service/scripts/drive_ocr_paddleocr.py` を local Python process として実行する
- PaddleOCR 3.x の Python API `PaddleOCR(...).predict(...)` を使い、旧CLI引数 `--image_dir` / `--use_angle_cls` には依存しない
- output JSON を page layout / boxes に保存する
- `HAOHAO_DRIVE_PADDLEOCR_PYTHON` で Python runtime を指定できる
- helper failure は OCR run `failed` にし、stderr summary を `error_message` に保存する

runtime setup は `docs/RUNBOOK_DRIVE_PADDLEOCR.md` に分離します。

### Ollama structured extraction

Ollama は OCR そのものではなく structured extraction に使います。

入力:

- OCR text
- layout text
- JSON Schema
- language / currency hints

出力:

- JSON Schema に合う product extraction items

制約:

- `ollamaBaseURL` は localhost / loopback のみ
- file body を Ollama に直接渡さない
- model がない場合は extraction を `failed` ではなく structured extraction 部分だけ skipped にし、OCR run は raw OCR result を残す
- schema validation に失敗したら product items は保存せず、run の warning / error summary に残す

## Step 9. Drive search index に反映する

### 対象ファイル

```text
backend/internal/service/drive_collaboration_governance_service.go
backend/internal/service/drive_ocr_service.go
db/queries/drive_search.sql
```

OCR completed 後に、OCR 全文を `drive_search_documents.extracted_text` に反映します。

既存 file metadata / text extraction と競合しないように、検索用 text は次の順で作ります。

1. 既存の file title
2. born-digital text または OCR text
3. OCR page snippets

`UpsertDriveSearchDocument` は既存 query を使い、`extracted_text` に OCR 全文、`snippet` に先頭部分、`content_sha256` に file hash、`object_updated_at` に file updated at を渡します。

検索 index 更新が失敗しても OCR run の raw result は残します。ただし worker は error を返し、outbox retry で再実行できるようにします。duplicate run は idempotent に処理します。

## Step 10. Browser API / Admin API を追加する

### 対象ファイル

```text
backend/internal/api/drive_ocr.go
backend/internal/api/tenant_admin_drive_ocr.go
backend/internal/api/drive_types.go
backend/internal/api/register.go
```

### Browser API

#### `POST /api/v1/drive/files/{filePublicId}/ocr/jobs`

手動再実行します。

条件:

- session + CSRF 必須
- source file に `CanEditFile` 相当の権限が必要
- tenant policy の OCR が enabled
- zero-knowledge / DLP blocked / infected は request は受けても run を `skipped` にする

response:

```json
{
  "publicId": "uuid",
  "filePublicId": "uuid",
  "engine": "tesseract",
  "status": "pending",
  "createdAt": "2026-04-28T00:00:00Z"
}
```

#### `GET /api/v1/drive/files/{filePublicId}/ocr`

最新 OCR run と page summary を返します。

条件:

- source file に `CanViewFile` 相当の権限が必要

response:

```json
{
  "run": {
    "publicId": "uuid",
    "filePublicId": "uuid",
    "engine": "tesseract",
    "status": "completed",
    "pageCount": 2,
    "processedPageCount": 2,
    "averageConfidence": 0.91,
    "errorCode": "",
    "errorMessage": "",
    "createdAt": "2026-04-28T00:00:00Z",
    "completedAt": "2026-04-28T00:00:10Z"
  },
  "pages": [
    {
      "pageNumber": 1,
      "rawText": "OCR text",
      "averageConfidence": 0.92
    }
  ]
}
```

#### `GET /api/v1/drive/files/{filePublicId}/product-extractions`

商品 / 販促候補を返します。

条件:

- source file に `CanViewFile` 相当の権限が必要

response:

```json
{
  "items": [
    {
      "publicId": "uuid",
      "itemType": "product",
      "name": "商品名",
      "janCode": "4900000000000",
      "price": {"amount": 198, "currency": "JPY"},
      "sourceText": "OCR source",
      "confidence": 0.77
    }
  ]
}
```

### Admin API

#### `GET /api/v1/admin/tenants/{tenantSlug}/drive/ocr/status`

local runtime dependency を返します。

response:

```json
{
  "enabled": true,
  "ocrEngine": "tesseract",
  "structuredExtractor": "rules",
  "dependencies": [
    {"name": "tesseract", "available": true, "version": "5.5.2"},
    {"name": "pdftotext", "available": true, "version": ""},
    {"name": "pdftoppm", "available": true, "version": ""},
    {"name": "jpn.traineddata", "available": true, "version": ""}
  ],
  "ollama": {
    "configured": false,
    "reachable": false,
    "modelAvailable": false
  }
}
```

この endpoint は dependency status を返すだけです。file body や OCR text は返しません。

## Step 11. frontend に接続する

### 対象ファイル

```text
frontend/src/api/drive.ts
frontend/src/api/tenant-admin.ts
frontend/src/stores/drive.ts
frontend/src/components/DriveDetailsPanel.vue
frontend/src/views/tenant-admin/TenantAdminTenantDrivePolicyView.vue
frontend/src/tenant-admin/detail-context.ts
frontend/src/i18n/messages.ts
```

### Drive details

`DriveDetailsPanel` の tab を増やすか、details tab 内に OCR section を追加します。

表示する情報:

- OCR status
- engine
- page count
- confidence
- latest completed time
- error / skipped reason
- manual rerun button
- product / promotion candidate list

manual rerun button は selected item が file の場合だけ表示します。busy state 中は disable します。

### Drive preview

preview dialog では OCR text を本文 preview と混ぜません。OCR text は details panel の OCR section から見る形にし、元 file preview と派生 text の境界を保ちます。

### tenant admin Drive policy

Drive policy page に OCR section を追加します。

UI は既存 form の密度に合わせ、巨大な説明文は置きません。必要な入力だけを配置します。

追加する i18n key は `tenantAdmin.fields.*`、`tenantAdmin.policy.*`、`drive.ocr*` に分けます。

## Step 12. 検証する

### unit

```bash
go test ./backend/internal/service -run 'TestDriveOCR|TestDrivePolicy|TestTenantSettings'
go test ./backend/internal/api -run 'TestDriveOCR'
```

見る内容:

- Drive OCR policy default
- invalid engine / extractor
- language normalization
- `ollamaBaseURL` loopback validation
- supported MIME type 判定
- Tesseract / Poppler command argument generation
- product intermediate schema validation

### service

```bash
go test ./backend/internal/service -run 'TestDriveOCRService'
```

見る内容:

- upload 後 enqueue
- overwrite 後 enqueue
- fake OCR provider success
- provider failure
- unsupported MIME type skip
- DLP blocked skip
- zero-knowledge skip
- same hash idempotency
- search index reflection

### DB / sqlc / OpenAPI

```bash
make gen
git diff -- db/schema.sql backend/internal/db openapi frontend/src/api/generated
```

見る内容:

- `db/schema.sql` に `drive_ocr_runs` / `drive_ocr_pages` / `drive_product_extraction_items` が入っている
- generated db code に `drive_ocr.sql` query が入っている
- OpenAPI に Browser API / Admin API が出ている
- generated SDK に OCR endpoint が出ている

### frontend

```bash
npm --prefix frontend run build
```

見る内容:

- tenant admin Drive policy が build できる
- generated SDK wrapper の型が合う
- details panel が selected file / folder / no selection の各状態で崩れない

### manual smoke

local runtime を確認します。

```bash
command -v tesseract
tesseract --version
tesseract --list-langs
command -v pdftotext
command -v pdftoppm
command -v magick || command -v convert
```

PaddleOCR mode を確認する場合は Python runtime も確認します。

```bash
export HAOHAO_DRIVE_PADDLEOCR_PYTHON="$PWD/.venv-paddleocr/bin/python"
$HAOHAO_DRIVE_PADDLEOCR_PYTHON backend/internal/service/scripts/drive_ocr_paddleocr.py --check
```

sample を使って確認します。

```bash
RUN_DRIVE_OCR_SMOKE=1 make smoke-openfga
```

manual では次を確認します。

- text PDF は `pdftotext` だけで completed になる
- scanned PDF は rasterize + OCR で completed になる
- PNG / JPEG は OCR completed になる
- unsupported file は skipped になる
- OCR text が Drive search で検索できる
- product extraction item が details panel に表示される
- network を切った状態でも default mode が完了する

## Rollback / 運用メモ

### rollback

OCR は派生 state なので、機能停止は policy から始めます。

1. tenant Drive policy の `features.drive.ocr.enabled` を `false` にする
2. outbox worker を再起動して新規 OCR 処理を止める
3. pending / running の `drive_ocr_runs` を `skipped` または `failed` に更新する
4. 必要なら `drive_search_documents.extracted_text` から OCR 由来 text を rebuild で落とす
5. schema rollback が必要な場合だけ `0022` down migration を実行する

OCR table は file object に cascade するため、file delete / purge と整合します。

### local-only の運用

- Tesseract / Poppler / ImageMagick は OS package として管理する
- `jpn` / `jpn_vert` traineddata は deploy artifact または image に含める
- Docling / PaddleOCR / Ollama model は runtime 起動時に download しない
- Ollama は loopback URL だけを許可する
- OCR helper の stderr に file content を出さない
- OCR text / extracted product は source file と同じ tenant boundary で扱う

### observability

metrics label に tenant id、file id、OCR text、商品名は入れません。

推奨 label:

- engine
- extractor
- status
- reason
- content type family

log には run public id、file public id、engine、status、duration だけを出します。raw OCR text、商品名、JAN code、価格は log に出しません。

### 後続候補

- OCR queue 専用 worker interval / batch size の config 分離
- page image cache の保存期間管理
- OCR result の admin bulk rebuild
- product extraction item から catalog candidate への昇格 flow
- Docling / PaddleOCR helper の container 化
- Ollama model availability drill
