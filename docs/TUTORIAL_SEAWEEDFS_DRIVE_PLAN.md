# SeaweedFS を使った Drive Storage 導入チュートリアル

## この文書の目的

この文書は、`FILE_SHARE_SPEC.md` と `docs/TUTORIAL_FILE_SHARE.md` の Google Drive クローン要件に対して、SeaweedFS を file body storage として導入するための実装チュートリアル兼 runbook です。

この文書は「これから作る計画」だけではなく、現在の実装状態、確認済みのコマンド、未完了の production / direct upload 領域を区別して扱います。

SeaweedFS は HaoHao の metadata store や authorization store にはしません。HaoHao では引き続き次の責務分離を守ります。

| 領域 | Source of truth |
| --- | --- |
| 認証、SSO、MFA、ユーザー入口 | Zitadel / local auth |
| tenant、workspace、folder、file metadata、共有、リンク、監査、policy | PostgreSQL / sqlc |
| file / folder / workspace / group / share link authorization | OpenFGA |
| file body、object head、signed URL、physical purge | SeaweedFS S3-compatible storage |
| browser UI、Drive 操作導線 | Vue / generated SDK |

## 現在の実装状態

このセッションで完了した範囲は次です。

| Phase | 状態 | 内容 |
| --- | --- | --- |
| Phase 0 | 完了 | Docker Compose optional profile の `seaweedfs`、Make target、`.env.example` を追加 |
| Phase 1 | 完了 | `FILE_STORAGE_DRIVER=local|seaweedfs_s3`、S3-compatible `FileStorage` driver、driver unit test を追加 |
| Phase 2 | 完了 | Drive upload / overwrite / public editor overwrite / collaboration save を SeaweedFS S3 driver に接続 |
| Phase 5 | 完了 | `smoke-file-purge` を `local` と `seaweedfs_s3` の両方で検証できるように拡張 |
| Phase 3, 4, 6, 7, 8 | 未完了 | direct upload API、大容量 upload、本番 multi-component 構成、local storage からの移行など |

実装済みの主要ファイル:

```text
compose.yaml
Makefile
.env.example
backend/internal/config/config.go
backend/internal/service/file_storage.go
backend/internal/service/s3_file_storage.go
backend/internal/service/s3_file_storage_test.go
backend/internal/service/drive_service.go
backend/internal/service/drive_service_api.go
backend/internal/service/drive_phase7_service.go
backend/internal/service/drive_phase8_service.go
backend/internal/service/file_service.go
db/queries/file_objects.sql
db/queries/drive_files.sql
scripts/smoke-openfga.sh
scripts/smoke-file-purge.sh
```

現在の重要な制約:

- Drive API は backend proxied stream のままです。browser が SeaweedFS credential を持つ direct upload はまだ実装していません。
- `CreateDownloadURL` / `CreateUploadURL` は `FileStorage` driver level で presign 可能ですが、公開 API と tenant policy guard 付きの signed URL flow は Phase 3 で実装します。
- `weed mini` は local dev / smoke 専用です。本番では Master / Volume / Filer / S3 Gateway を分けます。
- `storage_key` は user input から作りません。filename、folder path、email、tenant slug/name を key に含めません。

## SeaweedFS 採用判断

公式 README と Wiki では、開発用途は `weed mini -dir=/data` で Master / Volume / Filer / S3 / WebDAV / Admin UI を単一プロセス起動できます。一方、Wiki は `weed mini` を learning / development / testing 用と明記し、本番は Master、Volume、Filer、S3 Gateway を分ける multi-component setup を使う前提です。

HaoHao ではこの差をそのまま採用します。

- local dev: Docker Compose profile の `seaweedfs` で `weed mini` を起動する
- backend integration: SeaweedFS の S3-compatible endpoint を `FileStorage` の S3 driver から使う
- production: `weed mini` ではなく Master / Volume / Filer / S3 Gateway の分割構成にする
- app metadata: SeaweedFS Filer metadata ではなく PostgreSQL を正本にする
- permission: SeaweedFS ACL ではなく HaoHao API + OpenFGA check を正本にする

参考:

- https://github.com/seaweedfs/seaweedfs
- https://github.com/seaweedfs/seaweedfs/wiki/Quick-Start-with-weed-mini
- https://github.com/seaweedfs/seaweedfs/wiki/Getting-Started

## Local Runtime

`compose.yaml` に SeaweedFS の optional profile を追加します。通常の `make up` では起動せず、Drive storage を検証するときだけ明示起動します。

```bash
make seaweedfs-config
make seaweedfs-up
make seaweedfs-logs
make seaweedfs-down
```

local endpoint:

| 用途 | URL |
| --- | --- |
| Master UI | `http://127.0.0.1:9333` |
| Volume Server | `http://127.0.0.1:9340` |
| Filer UI | `http://127.0.0.1:8888` |
| S3 Endpoint | `http://127.0.0.1:8333` |
| WebDAV | `http://127.0.0.1:7333` |
| Admin UI | `http://127.0.0.1:23646` |

local S3 credential:

```dotenv
SEAWEEDFS_ACCESS_KEY=haohao
SEAWEEDFS_SECRET_KEY=haohao-secret
SEAWEEDFS_S3_ENDPOINT=http://127.0.0.1:8333
SEAWEEDFS_BUCKET=haohao-drive-dev
FILE_STORAGE_DRIVER=seaweedfs_s3
FILE_S3_ENDPOINT=http://127.0.0.1:8333
FILE_S3_REGION=us-east-1
FILE_S3_BUCKET=haohao-drive-dev
FILE_S3_ACCESS_KEY_ID=haohao
FILE_S3_SECRET_ACCESS_KEY=haohao-secret
FILE_S3_FORCE_PATH_STYLE=true
```

bucket 作成と疎通確認:

```bash
AWS_ACCESS_KEY_ID=haohao \
AWS_SECRET_ACCESS_KEY=haohao-secret \
AWS_DEFAULT_REGION=us-east-1 \
aws --endpoint-url http://127.0.0.1:8333 s3 mb s3://haohao-drive-dev

AWS_ACCESS_KEY_ID=haohao \
AWS_SECRET_ACCESS_KEY=haohao-secret \
AWS_DEFAULT_REGION=us-east-1 \
aws --endpoint-url http://127.0.0.1:8333 s3api head-bucket --bucket haohao-drive-dev

curl -fsS 'http://127.0.0.1:9333/cluster/status?pretty=y'
curl -fsSI http://127.0.0.1:8888/
```

## ローカル検証手順

`.env` が `AUTH_MODE=zitadel` になっている環境でも、storage smoke は local login 前提で起動します。smoke 用の一時 backend は `AUTH_MODE=local ENABLE_LOCAL_PASSWORD_LOGIN=true` を明示してください。

SeaweedFS driver で Drive API を検証する例:

```bash
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
HTTP_PORT=18080 \
APP_BASE_URL=http://127.0.0.1:18080 \
FILE_STORAGE_DRIVER=seaweedfs_s3 \
FILE_S3_ENDPOINT=http://127.0.0.1:8333 \
FILE_S3_REGION=us-east-1 \
FILE_S3_BUCKET=haohao-drive-dev \
FILE_S3_ACCESS_KEY_ID=haohao \
FILE_S3_SECRET_ACCESS_KEY=haohao-secret \
FILE_S3_FORCE_PATH_STYLE=true \
./bin/haohao
```

別 terminal で実行:

```bash
BASE_URL=http://127.0.0.1:18080 \
FILE_STORAGE_DRIVER=seaweedfs_s3 \
FILE_S3_ENDPOINT=http://127.0.0.1:8333 \
FILE_S3_REGION=us-east-1 \
FILE_S3_BUCKET=haohao-drive-dev \
FILE_S3_ACCESS_KEY_ID=haohao \
FILE_S3_SECRET_ACCESS_KEY=haohao-secret \
FILE_S3_FORCE_PATH_STYLE=true \
RUN_DRIVE_STORAGE_DRIVER_SMOKE=1 \
RUN_DRIVE_STORAGE_CONSISTENCY_SMOKE=1 \
make smoke-openfga
```

file body purge を SeaweedFS driver で検証:

```bash
FILE_STORAGE_DRIVER=seaweedfs_s3 \
FILE_S3_ENDPOINT=http://127.0.0.1:8333 \
FILE_S3_REGION=us-east-1 \
FILE_S3_BUCKET=haohao-drive-dev \
FILE_S3_ACCESS_KEY_ID=haohao \
FILE_S3_SECRET_ACCESS_KEY=haohao-secret \
FILE_S3_FORCE_PATH_STYLE=true \
make smoke-file-purge
```

local storage fallback も残すため、次も通る必要があります。

```bash
make smoke-file-purge
```

## Google Drive クローン化への実装手順

### Phase 0. SeaweedFS runtime を固定する

目的:

- local dev で SeaweedFS を optional dependency として起動できる
- S3 endpoint、credential、bucket 名を `.env.example` に明示する
- CI や通常開発の `make up` に余計な dependency を増やさない

完了条件:

- `make seaweedfs-config` が通る
- `make seaweedfs-up` で S3 endpoint が起動する
- bucket 作成と `aws s3 ls` が通る

状態: 完了。

### Phase 1. S3-compatible `FileStorage` driver を追加する

対象:

```text
backend/internal/config/config.go
backend/cmd/main/main.go
backend/internal/service/file_storage.go
backend/internal/service/s3_file_storage.go
backend/internal/service/s3_file_storage_test.go
```

方針:

- `FILE_STORAGE_DRIVER=local|seaweedfs_s3` を追加する
- local driver は既存挙動を維持する
- SeaweedFS driver は AWS S3 compatible API だけを使う
- `storage_driver` は `seaweedfs_s3`、`storage_bucket` は bucket 名、`storage_key` は service-generated key を保存する
- S3 access key / secret / signed URL は audit、log、API response、metrics label に出さない
- `FILE_S3_*` は `SEAWEEDFS_*` を fallback として読めるようにする

必要な config:

```dotenv
FILE_STORAGE_DRIVER=seaweedfs_s3
FILE_S3_ENDPOINT=http://127.0.0.1:8333
FILE_S3_REGION=us-east-1
FILE_S3_BUCKET=haohao-drive-dev
FILE_S3_ACCESS_KEY_ID=haohao
FILE_S3_SECRET_ACCESS_KEY=haohao-secret
FILE_S3_FORCE_PATH_STYLE=true
```

実装順:

1. config に S3 設定を追加する
2. `NewFileStorage(cfg)` factory を作る
3. `S3FileStorage.PutObject/GetObject/DeleteObject/HeadObject` を実装する
4. `CreateDownloadURL/CreateUploadURL` は presign を返す
5. S3 driver test は fake HTTP server か local SeaweedFS integration flag で分ける

確認:

```bash
go test ./backend/internal/service -run 'S3FileStorage|LocalFileStorage'
go test ./backend/...
```

実装メモ:

- `backend/internal/service/s3_file_storage.go` に S3-compatible driver を追加済み
- `FILE_STORAGE_DRIVER=seaweedfs_s3` は SeaweedFS の S3 endpoint、bucket、credential を使う
- `CreateDownloadURL` / `CreateUploadURL` は driver level の presign を返す
- S3 driver test は `httptest.Server` を使い、通常 unit test で SeaweedFS container を必須にしない
- oversized object は S3 endpoint に送る前に `ErrFileTooLarge` で止める

### Phase 2. Drive upload / download を SeaweedFS に接続する

対象:

```text
backend/internal/service/drive_service.go
backend/internal/service/drive_service_api.go
backend/internal/service/drive_phase7_service.go
backend/internal/service/drive_phase8_service.go
scripts/smoke-openfga.sh
```

方針:

- API は必ず HaoHao backend を通す
- upload 前に tenant、folder、quota、content-type、OpenFGA `can_edit` を確認する
- download 前に tenant、deleted、scan/DLP、download policy、OpenFGA `can_view/can_download` を確認する
- MVP は proxied stream でよい
- signed URL は Phase 3 で導入し、発行前 guard は必ず backend で行う

保存 key:

```text
tenants/{tenant_id}/workspaces/{workspace_public_id}/files/{generated_file_id}/v{revision}/body
```

key は user input から作りません。filename、folder path、email、tenant name は入れません。

確認:

```bash
FILE_STORAGE_DRIVER=seaweedfs_s3 \
RUN_DRIVE_STORAGE_DRIVER_SMOKE=1 \
make smoke-openfga

go test ./backend/...
npm --prefix frontend run build
```

実装メモ:

- Drive upload / overwrite / public editor overwrite / collaboration save は `PutObject` 経由
- `storage_driver`、`storage_bucket`、`storage_key`、`content_sha256`、`etag` を DB に保存する
- `RUN_DRIVE_STORAGE_DRIVER_SMOKE=1` では DB metadata と SeaweedFS object head を確認する
- `RUN_DRIVE_STORAGE_CONSISTENCY_SMOKE=1` も同じ storage metadata / object head 検証に参加する
- download、trash、restore、share link は既存の HaoHao backend と OpenFGA guard を経由する

### Phase 3. signed URL と direct upload を追加する

目的:

- 大きな file download で backend を詰まらせない
- upload は reserved row / complete callback で整合性を保つ

API 設計:

```text
POST /api/v1/drive/files/upload-urls
POST /api/v1/drive/files/{filePublicId}/upload-complete
POST /api/v1/drive/files/{filePublicId}/download-url
```

guard:

- signed URL 発行は短 TTL
- upload complete 前は list/search/share 対象にしない
- complete callback で `HeadObject`、size、content type、checksum、tenant/workspace/key を再検証する
- orphan object cleanup job を用意する

確認:

```bash
RUN_DRIVE_SIGNED_URL_SMOKE=1 make smoke-openfga
go test ./backend/...
```

状態: 未完了。driver level の presign はありますが、公開 API、reserved row、complete callback、policy guard はまだありません。

### Phase 4. 大容量 upload と versioning を整える

Google Drive クローンとしては、通常 upload だけでなく大容量 upload、再試行、履歴が必要です。

方針:

- resumable upload session を DB に保存する
- chunk upload state は DB、body は SeaweedFS multipart upload に閉じる
- file revision は DB の正本にし、SeaweedFS object version だけに依存しない
- overwrite は新 object key を作り、旧 key は retention / purge job で消す

確認:

```bash
RUN_DRIVE_LARGE_UPLOAD_SMOKE=1 \
RUN_DRIVE_REVISION_SMOKE=1 \
make smoke-openfga
```

状態: 未完了。

### Phase 5. lifecycle、trash、physical purge を SeaweedFS 対応にする

対象:

```text
backend/internal/jobs/data_lifecycle.go
backend/internal/service/file_service.go
db/queries/file_objects.sql
docs/TUTORIAL_P12_FILE_LIFECYCLE_PHYSICAL_DELETE.md
```

方針:

- soft delete / restore は PostgreSQL metadata だけを更新する
- complete delete は retention / legal hold / DLP / policy を確認する
- physical purge は実行中 storage driver に合わせて `local` または `seaweedfs_s3` を claim する
- `DeleteObject` は missing object を成功扱いにする
- purge 成功後も `file_objects` row は tombstone として残す

確認:

```bash
FILE_STORAGE_DRIVER=seaweedfs_s3 make smoke-file-purge
RUN_DRIVE_STORAGE_CONSISTENCY_SMOKE=1 make smoke-openfga
```

実装メモ:

- `scripts/smoke-file-purge.sh` は `FILE_STORAGE_DRIVER=local` と `FILE_STORAGE_DRIVER=seaweedfs_s3` の両方に対応済み
- `ClaimDeletedFileObjectsForPurge` は起動中の storage driver だけを claim する
- SeaweedFS purge smoke は upload 後に `aws s3api head-object` が成功し、purge 後に同じ `head-object` が失敗することを確認する

状態: 完了。

### Phase 6. 検索、preview、AI、Office との境界を固定する

SeaweedFS は file body の保存先です。検索 index、preview、Office co-authoring、AI summary は別 subsystem として、元 file の authorization と policy に従わせます。

方針:

- text extraction job は backend が `GetObject` した content から index を作る
- preview / thumbnail は derived object として別 key に保存する
- derived object は source file の OpenFGA check を通らない限り返さない
- E2EE 有効 file は server-side search / preview / AI を disabled にする

確認:

```bash
RUN_DRIVE_SEARCH_SMOKE=1 \
RUN_DRIVE_OFFICE_SMOKE=1 \
RUN_DRIVE_AI_POLICY_SMOKE=1 \
RUN_DRIVE_E2EE_SMOKE=1 \
make smoke-openfga
```

状態: 方針固定済み。SeaweedFS 導入に伴う追加実装はまだありません。

### Phase 7. production SeaweedFS 構成に進む

`weed mini` は local dev 専用です。本番では次の構成へ分けます。

```text
SeaweedFS master x 3
SeaweedFS volume server x N
SeaweedFS filer x 2+
SeaweedFS S3 gateway x 2+
Filer metadata store
backup / replication / erasure coding policy
TLS / network policy / private endpoint
```

HaoHao production checklist:

- S3 gateway は private network に置き、public internet へ直接出さない
- browser は SeaweedFS に直接 credential を持たない
- signed URL を使う場合も TTL は短く、scope は object key 単位にする
- bucket lifecycle と HaoHao retention policy の衝突を避ける
- backup restore drill は PostgreSQL、OpenFGA、SeaweedFS object を同じ時点へ戻せることを確認する

確認:

```bash
make smoke-backup-restore
RUN_DRIVE_STORAGE_CONSISTENCY_SMOKE=1 make smoke-openfga
```

状態: 未完了。本番 runbook、TLS、network policy、replication、backup/restore drill は別作業です。

### Phase 8. local storage から SeaweedFS へ移行する

移行は overwrite ではなく copy + verify + switch で行います。

手順:

1. `file_objects.storage_driver = 'local'` の active object を列挙する
2. local body を SeaweedFS key へ copy する
3. size / sha256 / etag を検証する
4. DB row の `storage_driver/storage_bucket/storage_key/storage_version/content_sha256/etag` を transaction で更新する
5. old local body は retention 期間後に purge する
6. migration audit と metrics を残す

rollback:

- DB row に previous storage metadata を残す
- cutover 中は download が old/new の両方を試せる read-through mode を短期間だけ用意する

確認:

```bash
RUN_DRIVE_STORAGE_MIGRATION_SMOKE=1 make smoke-openfga
go test ./backend/...
```

状態: 未完了。read-through mode と migration audit はまだありません。

## 完成条件

### 現在満たしている条件

- SeaweedFS local runtime が optional profile で起動できる
- bucket 作成、S3 head、Master/Filer の疎通確認ができる
- `FILE_STORAGE_DRIVER=seaweedfs_s3` で Drive upload/download/share link/trash/restore が通る
- storage key は service-generated で、user input を含めない
- `storage_driver`、`storage_bucket`、`storage_key`、`content_sha256`、`etag` を DB に保存する
- physical purge と storage consistency smoke が SeaweedFS object を対象にできる
- local storage fallback の `make smoke-file-purge` も通る
- single binary でも API / UI / OpenAPI / public share route は変わらず動く

### 残っている条件

- signed URL は OpenFGA と tenant policy の後にだけ発行される公開 API として実装する
- direct upload は reserved row / complete callback / orphan cleanup まで含めて実装する
- 大容量 upload、multipart upload、revision retention を実装する
- production runbook に SeaweedFS backup / restore / replication / DR 手順を追加する
- local storage から SeaweedFS への copy + verify + switch migration を実装する

## 確認済みコマンド

`make smoke-openfga` は backend を起動しません。SeaweedFS driver で検証する場合は、先に `18080` など空いている port で `./bin/haohao` を起動してから実行します。`make smoke-file-purge` と `make e2e` は script 側で一時 server を起動します。

```bash
make seaweedfs-config
make seaweedfs-up
make db-up
make gen
go test ./backend/...
npm --prefix frontend run build
make binary
make test-openfga-model
AUTH_MODE=local \
ENABLE_LOCAL_PASSWORD_LOGIN=true \
HTTP_PORT=18080 \
APP_BASE_URL=http://127.0.0.1:18080 \
FILE_STORAGE_DRIVER=seaweedfs_s3 \
FILE_S3_ENDPOINT=http://127.0.0.1:8333 \
FILE_S3_REGION=us-east-1 \
FILE_S3_BUCKET=haohao-drive-dev \
FILE_S3_ACCESS_KEY_ID=haohao \
FILE_S3_SECRET_ACCESS_KEY=haohao-secret \
FILE_S3_FORCE_PATH_STYLE=true \
./bin/haohao
BASE_URL=http://127.0.0.1:18080 \
FILE_STORAGE_DRIVER=seaweedfs_s3 \
FILE_S3_ENDPOINT=http://127.0.0.1:8333 \
FILE_S3_REGION=us-east-1 \
FILE_S3_BUCKET=haohao-drive-dev \
FILE_S3_ACCESS_KEY_ID=haohao \
FILE_S3_SECRET_ACCESS_KEY=haohao-secret \
FILE_S3_FORCE_PATH_STYLE=true \
RUN_DRIVE_STORAGE_DRIVER_SMOKE=1 \
RUN_DRIVE_STORAGE_CONSISTENCY_SMOKE=1 \
make smoke-openfga
FILE_STORAGE_DRIVER=seaweedfs_s3 \
FILE_S3_ENDPOINT=http://127.0.0.1:8333 \
FILE_S3_REGION=us-east-1 \
FILE_S3_BUCKET=haohao-drive-dev \
FILE_S3_ACCESS_KEY_ID=haohao \
FILE_S3_SECRET_ACCESS_KEY=haohao-secret \
FILE_S3_FORCE_PATH_STYLE=true \
make smoke-file-purge
make smoke-file-purge
make e2e
git diff --check
```

`make e2e` は現状 `3 passed / 1 skipped` です。Drive UI E2E は既存設定で skip されます。

## 直接編集しない生成物

次は生成物です。S3 driver や API を追加したあとも、元ファイルを直して regenerate します。

- `db/schema.sql`
- `backend/internal/db/*`
- `openapi/openapi.yaml`
- `openapi/browser.yaml`
- `openapi/external.yaml`
- `frontend/src/api/generated/*`
- `backend/web/dist/*`
- `bin/haohao`
