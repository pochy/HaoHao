# HaoHao

`CONCEPT.md` を正として組んだ、実装可能な最小骨格です。

- Monorepo
- backend: Go + Gin + Huma
- frontend: Vue 3 + Vite + TypeScript + Pinia
- OpenAPI: 3.1 only
- DB: PostgreSQL + sqlc
- session store: Redis
- auth target: Zitadel

現時点の目的は、業務実装の前に「起動・生成・接続の導線」を揃えることです。

## 現在の範囲

- browser 向け API の最小 BFF 骨格
- `GET /api/v1/health`
- `GET /api/v1/session`
- `GET /external/v1/health`
- Huma からの OpenAPI 3.1 export
- raw OpenAPI endpoint (`/openapi.json`, `/openapi.yaml`) と docs placeholder (`/docs`)
- OpenAPI artifact の commit
- generated TypeScript client の commit
- Vue から generated client を呼ぶ最小経路
- PostgreSQL / Redis の compose 起動
- sqlc の最小生成

まだ本実装していないもの:

- Zitadel 本接続
- Redis を使った本物のセッション管理
- `Stoplight Elements` / `Scalar` / `Swagger UI` を切り替え可能な `/openapi`
- external client 向け API 本体
- 業務ロジック
- 本番レベルの認証・認可

## ディレクトリ

```text
.
├── backend/
│   ├── cmd/
│   │   ├── openapi/
│   │   └── server/
│   ├── internal/
│   │   ├── api/
│   │   ├── app/
│   │   ├── config/
│   │   ├── db/
│   │   ├── middleware/
│   │   └── service/
│   ├── sqlc.yaml
│   └── web/dist/
├── db/
│   ├── migrations/
│   ├── queries/
│   └── schema.sql
├── frontend/
│   ├── public/
│   ├── src/
│   │   ├── api/
│   │   │   └── generated/
│   │   ├── features/
│   │   ├── pages/
│   │   └── shared/
│   └── vite.config.ts
├── openapi/
│   └── openapi.yaml
├── compose.yaml
├── go.work
└── Makefile
```

## 必要環境

- Go 1.26+
- Node.js 20+
- npm
- Docker + Docker Compose
- GNU Make

## 起動手順

### 1. frontend 依存を入れる

```bash
npm --prefix frontend install
```

### 2. PostgreSQL / Redis を起動する

```bash
make compose-up
```

この PostgreSQL は、`db/schema.sql` を流し込めば後段の `make sqlc-vet` に使えます。`sqlc Cloud` や `SQLC_AUTH_TOKEN` は不要です（方針の詳細は「[CI ガード](#ci-ガード)」）。

### 3. artifact を生成する

```bash
make gen
```

これは次をまとめて実行します。

- `openapi/openapi.yaml` の再生成
- `frontend/src/api/generated/` の再生成
- `backend/internal/db/` の sqlc 再生成

### 4. backend を起動する

```bash
make backend
```

### 5. frontend を起動する

```bash
make frontend
```

## 開発フロー

この repo では、手書きの YAML や手書きの client を正本にしません。

- API 契約の正本: backend の Huma operation と request / response struct
- DB 契約の正本: `db/migrations/`
- OpenAPI artifact: `openapi/openapi.yaml`
- frontend client artifact: `frontend/src/api/generated/`
- sqlc artifact: `backend/internal/db/`

直接編集する場所と、生成される場所をまず分けて考えてください。

### 直接編集する場所

- backend の operation: `backend/internal/api/`
- backend の業務ロジック: `backend/internal/service/`
- backend の middleware / config: `backend/internal/middleware/`, `backend/internal/config/`
- DB migration: `db/migrations/`
- DB query: `db/queries/`
- schema snapshot: `db/schema.sql`
- frontend の静的 asset: `frontend/public/`
- frontend の feature / page / shared: `frontend/src/features/`, `frontend/src/pages/`, `frontend/src/shared/`

### 生成される場所

- OpenAPI artifact: `openapi/openapi.yaml`
- generated client: `frontend/src/api/generated/`
- sqlc generated code: `backend/internal/db/`

生成物は直接編集せず、元データを直してから `make gen` で更新します。

### 変更の基本順序

業務実装を追加する場合は、原則として次の順で進めます。

1. `db/migrations/` を追加または更新する
2. `db/schema.sql` を現行状態に合わせて更新する
3. `db/queries/` に sqlc 用の query を追加する
4. backend の Huma operation と request / response model を追加する
5. backend の `service` と必要なら `repository` 接続を追加する
6. `make gen` を実行して OpenAPI / client / sqlc を更新する
7. frontend で generated client を feature 経由で使う
8. ローカルで backend / frontend の接続を確認する
9. 生成物を含めて commit する

この順番にしておくと、実装・OpenAPI artifact・frontend client のずれを最小化できます。

### 具体例: 新しい browser API を追加する

たとえば browser 向けに `GET /api/v1/me` を追加する場合は、次の流れです。

1. DB が必要なら migration を追加する

```bash
touch db/migrations/002_add_profile.sql
```

2. `db/schema.sql` を更新する

- `db/schema.sql` は current schema の snapshot です
- 手元の migration 適用結果に合わせて更新します
- `sqlc` はこのファイルを入力にします

3. `db/queries/` に query を追加する

```sql
-- name: GetCurrentUser :one
SELECT ...
```

4. backend に operation を追加する

- `backend/internal/api/browser/v1/` に handler を置く
- request / response struct をそのファイルか近い場所に置く
- path は `/api/v1/...` を使う
- Huma の operation 定義から OpenAPI 3.1 が生成されます

5. backend の service を追加する

- operation には業務ロジックを書き込みすぎない
- 認可、業務ルール、repository 呼び出しは `backend/internal/service/` に寄せる

6. `make gen` を実行する

```bash
make gen
```

これで次が更新されます。

- `openapi/openapi.yaml`
- `frontend/src/api/generated/`
- `backend/internal/db/`

7. frontend 側で generated client を feature 経由で使う

- `frontend/src/features/<feature>/api/` に API adapter を置く
- `frontend/src/features/<feature>/model/` に Pinia store か state 処理を置く
- page から直接 generated file をばらばらに呼ばない
- HTTP の共通挙動は `frontend/src/shared/lib/http/transport.ts` に寄せる

8. page から feature を使う

- 画面 entry は `frontend/src/pages/`
- 共通 UI や util は `frontend/src/shared/`

### 具体例: frontend のみを変更する

API 契約を変えず、画面だけ変える場合は次の流れです。

1. `make frontend` で Vite を起動する
2. `frontend/src/features/`, `frontend/src/pages/`, `frontend/src/shared/` を編集する
3. generated client が必要なら既存の `frontend/src/api/generated/` を参照する
4. backend の API 契約を変えていないなら `make gen` は不要

### 具体例: backend のみを変更する

OpenAPI に影響しない内部実装だけを変える場合は次の流れです。

1. `make backend` を起動する
2. `backend/internal/service/` や `backend/internal/middleware/` を編集する
3. request / response や operation 定義を変えていないなら `make gen` は不要
4. Huma operation の入力・出力を変えたら必ず `make gen` を実行する

### `make gen` を実行すべきタイミング

次のどれかを変えたら `make gen` を実行します。

- Huma operation の path / method / request / response
- backend の API model
- `db/queries/`
- `db/schema.sql`

逆に、次だけなら通常は不要です。

- frontend の見た目だけの変更
- backend の内部ロジックだけの変更
- README や TODO などの文書変更

### ローカル確認の標準手順

backend と frontend を触ったら、最低限ここまでは確認します。

1. 依存サービスを起動する

```bash
make compose-up
```

2. 生成物を更新する

```bash
make gen
```

3. backend を起動する

```bash
make backend
```

4. 別ターミナルで frontend を起動する

```bash
make frontend
```

5. API と spec を確認する

```bash
curl http://localhost:8080/api/v1/health
curl -i http://localhost:8080/api/v1/session
curl -I http://localhost:8080/openapi.json
curl -I http://localhost:8080/openapi.yaml
```

6. 画面を確認する

- `http://localhost:5173` を開く
- Vite proxy 経由で `/api/v1/health` と `/api/v1/session` が呼べることを確認する
- `http://localhost:8080/docs` で built-in docs route が見えることを確認する
- `/openapi` は `Stoplight Elements` / `Scalar` / `Swagger UI` を切り替え可能な docs route として後続実装する

7. backend のテストを実行する

```bash
cd backend && go test ./...
```

8. frontend の build を確認する

```bash
make build-frontend
```

9. OpenAPI artifact を lint する

```bash
make openapi-lint
```

10. sqlc の query lint を確認する

```bash
PGPASSWORD=haohao psql -h localhost -p 5432 -U haohao -d haohao -f db/schema.sql
POSTGRESQL_SERVER_URI=postgresql://haohao:haohao@localhost:5432/haohao?sslmode=disable make sqlc-vet
```

`make compose-up` 済みなら、上の接続先をそのまま使えます。fresh な DB では先に `db/schema.sql` を流し込んでから `make sqlc-vet` を実行します。ローカルは compose の PostgreSQL、CI は GitHub Actions の service container を使います。

### commit 前チェック

commit 前には少なくとも次を確認します。

- `make gen` 実行後の生成物が最新である
- `openapi/openapi.yaml` が API 変更を反映している
- `frontend/src/api/generated/` が更新されている
- `backend/internal/db/` が query / schema 変更を反映している
- `make check-generated` が通る
- `make openapi-lint` が通る
- `make sqlc-vet` が通る
- backend のテストが通る
- frontend が build できる

### 開発時の判断ルール

- OpenAPI 3.0.3 には落とさない。3.1 のみを使う
- browser 向け API は `/api/v1`
- browser 向けと external client 向けは同じ API surface に混ぜない
- generated file を直接編集しない
- transport wrapper を経由せずに browser から直接 `fetch` を散らさない
- 静的 asset の正本は `frontend/public/` に置く
- frontend の build 出力先は `backend/web/dist/` を維持する
- DB の時間型は `timestamptz` + UTC 前提で扱う

## 主な URL

- frontend: `http://localhost:5173`
- backend: `http://localhost:8080`
- health: `http://localhost:8080/api/v1/health`
- session: `http://localhost:8080/api/v1/session`
- external health: `http://localhost:8080/external/v1/health`
- docs built-in route: `http://localhost:8080/docs`
- OpenAPI Documents: `http://localhost:8080/openapi`
- OpenAPI JSON: `http://localhost:8080/openapi.json`
- OpenAPI YAML: `http://localhost:8080/openapi.yaml`

`/openapi` は設計上の目標パスです。現状の skeleton では `/docs`, `/openapi.json`, `/openapi.yaml` を先に持っています。

開発時は Vite の proxy で `/api`, `/docs`, `/openapi`, `/openapi.json`, `/openapi.yaml` を backend に流す想定です。

## Make ターゲット

```bash
make gen
make check-generated
make openapi-lint
POSTGRESQL_SERVER_URI=postgresql://haohao:haohao@localhost:5432/haohao?sslmode=disable make sqlc-vet
make backend
make frontend
make build-frontend
make compose-up
make compose-down
```

`make build-frontend` は `frontend` の build を `backend/web/dist/` に出力します。backend はこの成果物を embed して配信できる前提です。

`make check-generated` は `make gen` を実行した上で、`openapi/openapi.yaml`, `frontend/src/api/generated/`, `backend/internal/db/` に差分が残っていないことを確認します。

`make sqlc-vet` は `backend/sqlc.ci.yaml` を使って `sqlc/db-prepare` を実行します。`POSTGRESQL_SERVER_URI` には、`db/schema.sql` を流し込み済みの PostgreSQL を渡してください。

## CI ガード

**生成物・契約のドリフト防止**（Issue #4 では external のスモークより、OpenAPI / client / sqlc の整合を CI で落とすことを優先した）。

### Issue #4 での主眼

Issue #4 のうち、ここで優先したのは **external API の存在確認そのものではなく、生成物と契約の更新漏れを CI で落とすこと**です。external 向け endpoint はすでに `main` に入っているため、このターンで追加の価値が大きいのは **OpenAPI / frontend generated client / sqlc 生成物のドリフト防止**だと判断したためです。

### 背景と目的

Huma から OpenAPI を出し、そこから frontend client を生成し、DB 側は `sqlc` でコード生成する構成では、正本と生成物がずれるとレビューでも実行時でも壊れやすくなります。

`sqlc verify` は sqlc Cloud 前提であり、この repo の初期方針としては重いので採用しません。**sqlc Cloud と `SQLC_AUTH_TOKEN` は使わず、ローカル環境と GitHub Actions だけで再現できるチェック**に寄せています。

### 構成（何が追加されたか）

- **Makefile**: `make check-generated`, `make openapi-lint`, `make sqlc-vet` を追加済み（生成物検証・OpenAPI lint・PostgreSQL 18 向け `sqlc vet` の入口）
- **`.github/workflows/generated-artifacts.yml`**: PR / push で次を実行する
  - `make check-generated`
  - OpenAPI lint（`make openapi-lint` と同等の目的）
  - `db/migrations/` 変更がある場合、`scripts/check-schema-snapshot.sh` により `db/schema.sql` の更新漏れを検知
  - PostgreSQL 18 の service に `db/schema.sql` を流し込んだうえで `make sqlc-vet`
- **`backend/sqlc.ci.yaml`**: `POSTGRESQL_SERVER_URI` を受け取る CI 用の `sqlc vet` 設定（ローカル用 `backend/sqlc.yaml` と役割を分離）
- **`scripts/check-schema-snapshot.sh`**: migration と `db/schema.sql` の整合を検知

### 期待する結果

- API / query / schema を変えたのに `make gen` や `db/schema.sql` を更新し忘れた PR が CI で落ちる
- OpenAPI artifact が壊れていれば CI で落ちる
- SQL が PostgreSQL 18 上で成立しなければ CI で落ちる
- ローカルでも Actions でも、ほぼ同じ Makefile 経由で確認できる

### この変更を確認したコマンド（例）

次まで通過することを確認しています。

- `make check-generated`
- `make openapi-lint`
- `POSTGRESQL_SERVER_URI=... make sqlc-vet`（または README 記載の接続先）
- `cd backend && go test ./...`
- `make build-frontend`

commit 前の一覧は「[commit 前チェック](#commit-前チェック)」と合わせて使ってください。

### 備考（今回の公式な変更範囲に含めないもの）

`compose.yaml` の PostgreSQL 18 向けボリュームパス修正は、手元では見えていても **今回の「生成物ガード」変更としては正式には含めていません**。ドキュメント上は参考情報としてのみ区別してください。

## GitHub 運用

この repo では、設計ドキュメントと実装差分だけでなく、Issue/PR 運用も同じ粒度で管理します。

- Issue は `TODO.md` の起票テンプレートを基準に作成する
- フェーズ進行は milestone `M1`-`M5` で管理する
- リリース進行は milestone `v0.1 Foundation`, `v0.2 Auth`, `v0.3 First Feature` で管理する
- GitHub Project `HaoHao Roadmap TODO 1-5` で横断管理する
- Project のカスタムフィールド `Priority`, `Area`, `Risk`, `Target Release` を使う
- PR は `.github/pull_request_template.md` を必須テンプレートとして使う
- Issue は `.github/ISSUE_TEMPLATE/` の form から起票する
- レビュー責務は `.github/CODEOWNERS` に従う

## ブランチ保護ルール

`main` には次を適用する前提です。

- Pull Request 経由でのみマージする
- Code Owner review を必須にする
- 1 件以上の Approve を必須にする
- stale review を自動 dismiss する
- 会話解決（conversation resolution）を必須にする
- force push と branch 削除を禁止する
- required status check として `ci-codeql` を通す

## 実装メモ

### backend

- Huma の docs / spec 公開は [docs auth stub](backend/internal/middleware/docs_auth.go) で保護差し込み点だけ作っています
- OpenAPI docs の目標パス構成は `/docs` built-in route + `/openapi` custom route + `/openapi.json` / `/openapi.yaml`
- browser API は `/api/v1`
- external client 向け API は `backend/internal/api/external/` に予約しています
- external client 向け API は `/external/v1` 配下とし、OpenAPI 上も bearer token 前提で browser API と分離します

### frontend

- 構成は `shared / features / pages`
- `frontend/public/` が favicon などの静的 asset の正本
- generated client は `frontend/src/api/generated/`
- transport wrapper は `frontend/src/shared/lib/http/transport.ts`
- `credentials: 'include'` を既定化
- state changing request で `X-CSRF-Token` を付ける placeholder を実装

### DB

- migration 正本は `db/migrations/`
- `db/schema.sql` は現行スナップショット
- sqlc 設定は `backend/sqlc.yaml`
- ID の最小例として `bigint` 主キー + `uuidv7()` 公開 ID を採用

## docs / auth の扱い

OpenAPI docs の目標パス構成は次です。

- `/docs`: Huma built-in docs route
- `/openapi`: OpenAPI Documents (`Stoplight Elements` / `Scalar` / `Swagger UI`)
- `/openapi.json`: raw JSON
- `/openapi.yaml`: raw YAML

`/docs`, `/openapi`, `/openapi.yaml`, `/openapi.json` は最終的に認証付き公開を想定しています。今は stub / placeholder 段階です。

- 現状の skeleton で先にあるのは `/docs`, `/openapi.json`, `/openapi.yaml`
- `/openapi` は docs renderer を切り替え可能な custom route として追加予定

- `DOCS_BEARER_TOKEN` 未設定: 開発用に通す
- `DOCS_BEARER_TOKEN` 設定: `Authorization: Bearer <token>` を要求
