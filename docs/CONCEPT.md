# コンセプト

## 結論

このシステムは、**OpenAPI 3.1 優先 + Monorepo + 単一バイナリ配信**を基本方針にする。

- フロントエンド: `Vue 3 + Vite + TypeScript + Pinia + vue-router + vue-i18n`
- API 契約: `Huma` で生成する `OpenAPI 3.1`（`full` / `browser` / `external` の 3 surface に分割）
- バックエンド: `Go + Gin + Huma`
- データ層: `PostgreSQL 18 + pgx + pgxpool + sqlc` を主軸に、`ClickHouse`(分析層) と S3 互換 (`SeaweedFS`) を併用
- 認可: `OpenFGA`（細粒度パーミッション、特に Drive ドメイン）
- 認証: `BFF + HttpOnly Cookie`（browser）/ `bearer token`（external / M2M / SCIM）/ `Zitadel` を IdP に採用
- 配信: `Go バイナリに Vue の dist と markdown docs を埋め込んで配信`
- 可観測性: 構造化ログ + Prometheus メトリクス + OpenTelemetry トレース

この構成の狙いは次の 5 点。

- OpenAPI 3.1 を現実的に運用できるようにする
- 実装と API 契約のドリフトを最小化する
- SQL を主役にして PostgreSQL の機能を素直に使い、分析・全文検索・大量ファイルは別ストアに逃がす
- ブラウザ用と外部クライアント用の API 表面を最初から別契約として扱う
- 開発時は分離し、本番時は単一バイナリ配信に寄せる

## 基本方針

### 1. Go の型から OpenAPI 3.1 を生成する

API の定義は hand-written な YAML ではなく、Go 側の Huma operation 定義と input / output struct を正として管理する。

- 設計: Go の型とタグで API の入出力、バリデーション、説明を定義する
- 生成: Huma から OpenAPI 3.1 の spec と API docs を生成する
- 再利用: 生成した spec から TypeScript client / types を生成する
- 運用: OpenAPI 3.1 を唯一の公開契約として扱う

これにより、OpenAPI 3.1 を使いながら、仕様更新と実装更新を同じ場所で管理できる。

### 2. OpenAPI surface を 3 つに分ける

外部クライアント、ブラウザ SPA、運用ドキュメント用途を、最初から別契約として扱う。`backend/cmd/openapi` を `-surface=full|browser|external` で実行し、3 ファイルを生成する。

- `openapi/openapi.yaml` (`full`): 全 operation を含む、運用・docs 配信向け
- `openapi/browser.yaml` (`browser`): SPA から呼び出す operation のみ。`SessionAuth` (Cookie + CSRF) スキーム前提
- `openapi/external.yaml` (`external`): 外部クライアント向け operation のみ。`BearerAuth`（OAuth2 / Zitadel 発行 JWT）前提

frontend の生成 client は `browser.yaml` を入力にし、external 用 SDK は `external.yaml` を別配布する。両者を混在させない。

### 3. Monorepo

フロント、バック、生成した OpenAPI artifact、DB スキーマ、マイグレーション、OpenFGA モデル、ops 設定、CI を 1 リポジトリで管理する。

メリットは次の通り。

- 変更の追跡がしやすい
- API 変更・OpenFGA モデル変更・migration を同じ PR で扱える
- CI を一元管理できる
- レビュー時に影響範囲を把握しやすい

### 4. 単一バイナリ配信

本番では Vue のビルド成果物を Go 側に埋め込み、API と SPA を 1 つのバイナリで配信する。markdown docs (`docs/`, runbook 類) もビルドタグで埋め込み、`/docs` 配下から認証付きで読める。

- 開発時: `frontend` と `backend` を別プロセスで起動する。`backend` は `Air` でホットリロードする
- 本番時: Go バイナリ 1 つで `/api/*`、OpenAPI、markdown docs、SPA 静的配信をまとめて扱う
- frontend の build 出力先は `backend/web/dist/` に揃える
- 埋め込みは `embed_frontend` ビルドタグで切り替え（`frontend_embed.go` / `frontend_stub.go`）

これにより、運用とデプロイの複雑さを下げられる。

## アーキテクチャ

| レイヤー       | 技術                                                          | 役割                                              |
| -------------- | ------------------------------------------------------------- | ------------------------------------------------- |
| フロントエンド | Vue 3 + Vite + TypeScript + Pinia + vue-router + vue-i18n     | UI、画面状態管理、API 呼び出し、多言語表示        |
| API 契約       | Huma が生成する OpenAPI 3.1（full / browser / external）       | バックエンドとフロントエンド・外部の共通契約      |
| バックエンド   | Go + Gin + Huma                                               | 認証、認可、業務ロジック、API 提供、OpenAPI 生成  |
| 認可エンジン   | OpenFGA                                                       | テナント・Drive・データセットの細粒度パーミッション |
| データ層       | PostgreSQL 18 + `pgx` + `sqlc`                                | トランザクショナル永続化、業務クエリ              |
| 分析層         | ClickHouse                                                    | データセット分析、テナント単位のクエリ実行        |
| ファイルストア | ローカル FS / S3 互換（SeaweedFS）                            | Drive ファイル、エクスポート成果物                |
| キャッシュ     | Redis                                                         | セッション、idempotency、レート制限、login state  |
| 認証基盤       | Zitadel（OIDC + Management API）                              | ユーザー、組織、SCIM provisioning                  |
| 可観測性       | 構造化ログ + Prometheus + OpenTelemetry (OTLP HTTP)            | ログ・メトリクス・分散トレース                    |
| 配信           | Go 単一バイナリ                                               | API、OpenAPI、docs、SPA の配信                    |

### 責務分離の原則

- Vue は UI と画面制御に集中する
- Huma operation は request / response、バリデーション、OpenAPI metadata、surface 分類に集中する
- service は業務ルールとトランザクション制御を担う
- repository (`sqlc` 生成) は SQL 実行に集中する
- OpenFGA は「誰が何をできるか」を一元管理し、service から `Check` / `ListObjects` で問い合わせる
- ジョブ (`jobs/`) はスケジューラ・outbox・provisioning reconcile などのバックグラウンド処理を担う
- platform (`platform/`) は postgres / redis / clickhouse / tracing / metrics / readiness / migration check の横断インフラを束ねる
- OpenAPI artifact は frontend と外部連携向けの公開契約として扱う

Huma operation に業務ロジックを書き込まない、Vue に Go の内部型を直接持ち込まない、認可判断を operation 層でアドホックに書かない（service + OpenFGA に寄せる）、という 3 点を強く守る。

## ディレクトリ構成

```text
HaoHao/
├── docs/                         # 設計・チュートリアル・runbook の markdown
├── openapi/                      # 生成された OpenAPI artifact（3 surface）
│   ├── openapi.yaml
│   ├── browser.yaml
│   └── external.yaml
├── go.work                       # repo root から ./backend を扱う Go workspace
├── frontend/
│   ├── src/
│   │   ├── components/           # 横断 UI コンポーネント
│   │   ├── views/                # ページ単位の view
│   │   ├── stores/               # Pinia store
│   │   ├── api/                  # transport wrapper + 生成 client (api/generated/)
│   │   ├── i18n/                 # vue-i18n 翻訳
│   │   ├── invitations/          # 招待フロー UI
│   │   ├── tenant-admin/         # テナント管理 UI 共通
│   │   ├── router/
│   │   └── utils/
│   ├── vite.config.ts
│   └── package.json
├── backend/
│   ├── cmd/
│   │   ├── main/                 # アプリ本体エントリ
│   │   └── openapi/              # OpenAPI 生成 CLI（-surface フラグ）
│   ├── internal/
│   │   ├── app/                  # Huma + Gin の組み立て、health, openapi handler
│   │   ├── api/                  # Huma operation 登録・request/response model
│   │   ├── service/              # 業務ロジック（Drive, dataset, pipeline, tenant…）
│   │   ├── db/                   # sqlc 生成コード
│   │   ├── auth/                 # OIDC / bearer / M2M / session / delegated OAuth
│   │   ├── middleware/           # CORS, rate limit, body limit, request id, tracing, recovery, security headers, markdown docs
│   │   ├── platform/             # postgres, redis, clickhouse, logger, metrics, tracing, readiness, migration check
│   │   ├── jobs/                 # outbox, scheduler, provisioning reconcile, data lifecycle, work table export
│   │   └── config/               # 環境変数読み込み・dotenv
│   ├── web/
│   │   └── dist/                 # frontend build 成果物（埋め込み対象）
│   ├── frontend.go / frontend_embed.go / frontend_stub.go   # SPA 配信と埋め込みタグ切替
│   ├── docs_embed.go / docs_stub.go                          # markdown docs の埋め込みタグ切替
│   ├── go.mod
│   └── sqlc.yaml
├── db/
│   ├── migrations/               # golang-migrate 用 versioned SQL（約 80 ファイル / 42 step）
│   ├── queries/                  # sqlc 入力 SQL
│   └── schema.sql                # migration 適用後のスナップショット（自動生成、編集禁止）
├── openfga/                      # OpenFGA model (drive.fga) と test (drive.fga.yaml)
├── e2e/                          # Playwright E2E（access fallback / browser journey / drive / i18n）
├── ops/
│   └── prometheus/               # Prometheus 設定
├── dev/
│   ├── zitadel/                  # 開発用 Zitadel docker-compose
│   └── clickhouse/               # ClickHouse local config (config.d/)
├── scripts/                      # gen.sh / smoke-*.sh / seed-*.sql / e2e-single-binary.sh / openfga-bootstrap.sh
├── compose.yaml                  # 開発依存サービス（postgres / redis / clickhouse / openfga / seaweedfs）
├── docker/Dockerfile             # 本番イメージ（多段階ビルド）
├── .air.toml                     # backend ホットリロード設定
└── Makefile
```

### Go workspace 方針

- `backend/` を独立した Go module とし、repo root に `go.work` を置く（`use ./backend`）
- `Makefile` と CI は repo root から `go run ./backend/cmd/main` / `go run ./backend/cmd/openapi` を実行する前提でそろえる
- 単一 module へ寄せるなら、ディレクトリ構成だけでなく `Makefile`、CI、生成手順、`.air.toml` の `cmd` も同時に合わせて変更する

## 技術選定

### フロントエンド

`Vue 3 + Vite + TypeScript + Pinia` を採用する。

- Vue 3: 保守性と学習コストのバランスがよい
- Vite: 開発体験が軽い
- TypeScript: API 契約と接続しやすい
- Pinia: Vue 標準寄りで扱いやすい
- vue-i18n: 多言語対応（日本語/英語）
- vue-router: SPA ルーティング
- 補助ライブラリ: `reka-ui`(UI primitive)、`lucide-vue-next`(アイコン)、`@vue-flow/*` + `elkjs`(データパイプラインのノードベースエディタ)、`virtua`(仮想スクロール)

frontend のディレクトリは views ベース + ドメインごとの store / api 分割を取る。

- `components/`: 横断的な UI コンポーネント
- `views/`: ページ entry
- `stores/`: ドメイン単位の Pinia store
- `api/`: transport wrapper と `api/generated/` の生成済み client
- `tenant-admin/`, `invitations/`: 大きなドメイン UI を views 外に切り出した置き場
- `i18n/`: 翻訳

### バックエンド

`Go + Gin + Huma` を採用する。

- Go: 単一バイナリ、並行処理、運用容易性が強い
- Gin: 既存の middleware 資産を活かしやすい
- Huma: OpenAPI 3.1 生成、request validation、docs 生成を一体で扱える

既存の Gin middleware を使い続けながら、API の入出力定義と OpenAPI 生成を Huma に寄せる。Huma adapter は `humagin` を使用する。

### データ層

- 第一データストア: `PostgreSQL 18 + pgx + pgxpool + sqlc`。業務トランザクション、認可状態、メタデータ、outbox、idempotency
- 分析層: `ClickHouse`。データセットへの SQL 分析クエリ。テナントごとの実行ユーザー/ロールで分離し、メモリ・行数・スレッド・実行秒数の上限を設定する
- 全文検索: 当面は PostgreSQL 上の `local_search` レイヤで足りる範囲をカバーし、要件が伸びた場合に外部検索エンジンを検討する
- ファイルストア: 開発はローカル FS、本番は S3 互換（SeaweedFS をデフォルト想定）。`FILE_STORAGE_DRIVER` で切り替える
- キャッシュ / セッション: `Redis`

ORM 中心ではなく、**SQL を設計資産として管理する**方針を取る。

### 認可: OpenFGA

ファイル共有・データセット共有のような関係ベース認可は OpenFGA を使う。

- モデル定義は `openfga/drive.fga`、テストは `drive.fga.yaml`
- backend からは `service/openfga_client.go` 経由で `Check` / `ListObjects` を呼ぶ
- migration と OpenFGA 側のモデル更新は同じ PR で進める
- `OPENFGA_FAIL_CLOSED=true` を既定にし、OpenFGA への到達不能を許可とみなさない
- 開発用の OpenFGA は専用 PostgreSQL と一緒に `compose.yaml` で起動する

### 可観測性

- ログ: 構造化 JSON（`platform/logger.go`）。`log_type` で `access` / `application_error` / `panic` / `migration_check` などを分ける
- メトリクス: `prometheus/client_golang` で `/metrics` を公開（`METRICS_ENABLED=true` のとき）
- トレース: OpenTelemetry。`otelgin` で gin に instrument し、OTLP HTTP で出す。`OTEL_TRACES_SAMPLER_RATIO` でサンプリングを制御
- 起動時 self-check: `migration_check`（DB が migration に追従しているか）、`readiness`（postgres / redis / clickhouse / 必要なら zitadel）

## PostgreSQL 18 の活用方針

PostgreSQL 18 の新機能は活用するが、流行りで採用するのではなく用途を限定する。

### 設計判断として使うもの

- `UUIDv7`: 外部公開 ID や分散環境向けの主キー候補
- 仮想生成カラム: 表示補助や検索補助に限定して使う
- テンポラルコンストレイント (`WITHOUT OVERLAPS`): 予約・期間管理など重複排除が必要なテーブル設計で使う

### 使えるなら活用するが前提にはしないもの

- Async I/O: 重い読み込み系ワークロードで恩恵がある。`io_method` で `worker` / `io_uring` / `sync` を選択できる
- `pg_upgrade` 時の統計保持: 本番アップグレードの安定化に有効

### 特に設定不要で恩恵を受けられるもの

- ページチェックサム: PostgreSQL 18 からデフォルトで有効化されており、データ整合性の検知が標準で機能する

### ID 戦略

集約ごとに `bigint` と `uuidv7` を使い分ける。

- コア業務テーブルや高頻度 join / 集計が中心の内部テーブル: `bigint GENERATED ALWAYS AS IDENTITY`
- 外部公開 API の境界、外部連携対象、分散構成を意識する集約: `uuid DEFAULT uuidv7()`
- 外部に見せる必要がない巨大テーブルまで一律に `uuidv7` にはしない

## 契約生成とクライアント生成

生成フローは `scripts/gen.sh` (`make gen`) に集約している。

```bash
# 1) sqlc 生成
cd backend && sqlc generate

# 2) OpenAPI 3.1 を 3 surface 分エクスポート
go run ./backend/cmd/openapi -surface=full     > openapi/openapi.yaml
go run ./backend/cmd/openapi -surface=browser  > openapi/browser.yaml
go run ./backend/cmd/openapi -surface=external > openapi/external.yaml

# 3) frontend 用 TypeScript client を生成
cd frontend && npm run openapi-ts
```

- TypeScript client: `@hey-api/openapi-ts`。frontend は `browser.yaml` から生成する
- 画面から生成 client を直接ばらばらに呼ばず、`frontend/src/api/` 配下の transport wrapper 経由で使う
  - `baseURL` は同一オリジン前提で `/api` に寄せる
  - `credentials: "include"` を既定にする
  - `POST` / `PUT` / `PATCH` / `DELETE` では `XSRF-TOKEN` Cookie を読んで `X-CSRF-Token` を自動付与する
  - problem details を UI 向けのエラー表現に寄せる場所を 1 か所にする
- OpenAPI artifact は repo に含めて PR で差分レビューする
- 本番の docs / OpenAPI endpoint は認証付きで公開する（`DOCS_AUTH_REQUIRED=true`）
- 固定版の補助配布経路として GitHub Release / release asset を持つ
- OpenAPI 3.0.3 へのダウングレード出力は持たず、3.1 を唯一の公開契約とする

## API バージョニングと surface 分離

- 既定は URL path versioning。`/api/v1` を使う
- 互換性を壊す変更は `/api/v2` のように新しい path prefix を切る
- 後方互換な追加変更は同一バージョン内で進める
- ブラウザ向け (`/api/v1/...`、`SessionAuth`) と外部クライアント向け (`/api/v1/external/...`、`BearerAuth`) は OpenAPI surface を分け、混在させない
- SCIM provisioning は `/api/scim/v2` に独立させ、`SCIM_BEARER_AUDIENCE` / `SCIM_REQUIRED_SCOPE` で保護する
- header / media type によるバージョニングは特殊要件のときだけ検討する

## リクエスト処理の流れ

### Vue

- transport wrapper 経由で生成 client を呼ぶ
- state-changing request では CSRF ヘッダーを自動付与する
- DTO を画面表示用に薄く整形する
- 表示する

### Huma operation

- path / query / header / body を受ける
- Huma が入力変換とバリデーションを行う
- 認証済みコンテキスト（session / bearer / m2m）を確認する
- service に委譲する
- response struct を返す

### Go service

- 認可確認（必要なら OpenFGA に問い合わせる）
- 業務バリデーション
- repository 呼び出し
- トランザクション制御
- outbox に副作用イベントを書き、ジョブが後段で取り出して webhook / notification / 検索インデックスに反映する

### Repository / sqlc

- SQL 実行
- 結果を Go 型として返す

### バックグラウンドジョブ

`backend/internal/jobs/` にまとめる。

- `outbox_worker`: outbox テーブルから取り出し、webhook 配信や notification を行う
- `scheduler`: 全体のスケジューラ
- `provisioning_reconcile`: SCIM/Zitadel と内部ユーザー/組織の同期
- `data_lifecycle`: outbox / notification / 削除済みファイルの保持期間整理
- `work_table_export_scheduler`: データセット work table のエクスポートスケジュール
- `data_pipeline_scheduler`: データパイプラインの定期実行

## 開発フロー

1. `db/migrations/` を追加または更新する
2. migration を適用した DB から `db/schema.sql` を再生成し、`db/queries/` を更新する
3. 認可モデルが変わるなら `openfga/drive.fga` と test を更新する
4. `backend/internal/api/` の operation と input / output model を更新する。`Tags` で surface (`browser` / `external`) を意識する
5. `make gen` を実行する（sqlc、OpenAPI 3 surface、frontend client）
6. export された OpenAPI artifact を PR 上で frontend と一緒にレビューする
7. `service` と API 実装を進める
8. フロントで transport wrapper 経由の生成済み client を使って接続する
9. 単体テストとローカル動作確認、必要なら `scripts/smoke-*.sh` を実行する

この順番にすると、実装・OpenAPI artifact・frontend client がずれにくい。

## 開発環境と本番環境

### 開発環境

開発時はフロントとバックを分ける。

- `make up` で `compose.yaml` を起動（postgres / redis / clickhouse / openfga / seaweedfs はオプション profile）
- `make db-up` で migration を適用
- `make backend-dev` で `Air` を使って backend をホットリロード起動（DB migration drift を `DB_MIGRATION_CHECK_MODE` で検知）
- `make frontend-dev` で Vite dev server 起動
- `vite.config.ts` の proxy で `/api` を Go に流す
- Huma の docs と spec は `/docs`、`/openapi.json`、`/openapi.yaml` で確認する
- `make zitadel-up` で開発用 Zitadel を起動できる（`AUTH_MODE=local` のままでも動く）

`compose.yaml` で起動するもの。

- PostgreSQL 18（業務 DB）
- Redis 7（セッション・idempotency・rate limit）
- ClickHouse（dataset 分析）
- OpenFGA + 専用 PostgreSQL（認可）
- SeaweedFS（profile=`seaweedfs`、開発で S3 を試したいときに起動）
- 別途 `dev/zitadel/docker-compose.yml` で Zitadel

### 本番環境

本番では Vue の build 成果物を `backend/web/dist/` に出力し、Go に埋め込む。

1. `frontend` を build し、出力先を `backend/web/dist/` に揃える
2. `backend/frontend_embed.go` で `//go:embed` により成果物を埋め込む（`embed_frontend` build tag）
3. markdown docs も `backend/docs_embed.go` で埋め込み、`/docs/...` から認証付きで読めるようにする
4. `/api/*` は API に渡す
5. OpenAPI と docs は認証付きで公開する
6. 静的ファイルが存在する場合はそのまま返す
7. それ以外の `GET` / `HEAD` で HTML を要求するリクエストだけ `index.html` を返す
8. 存在しないアセットは 404 にする

```bash
# Makefile からの本番ビルド
make binary           # frontend build → embed_frontend タグ付きで CGO_ENABLED=0 ビルド
make binary-package   # tar.gz パッケージ
make binary-size-check
```

ビルドは `-trimpath -ldflags "-s -w -buildid="` + `-tags "embed_frontend nomsgpack"` で再現性とサイズを優先する。

Docker では多段階ビルドを採用し、Node で frontend を build した後に Go バイナリを作る。

静的アセット配信の負荷が将来ボトルネックになった場合は、`backend/web/dist/` の成果物を CDN や object storage に逃がし、Go を API 専用サーバーとして分離できるようにしておく。

## 設定管理

環境差分と秘密情報は `backend/internal/config/` に集約する。

- DB 接続先（PostgreSQL / ClickHouse）
- Redis 接続先
- Cookie の `Secure`、`SameSite`、domain 設定
- CORS の許可 Origin（browser）、external / SCIM 用の許可 Origin / audience
- 認証関連: `AUTH_MODE`、Zitadel の issuer / client / management token / organization、`ENABLE_LOCAL_PASSWORD_LOGIN`
- 委譲 OAuth: `DOWNSTREAM_TOKEN_ENCRYPTION_KEY`, `DOWNSTREAM_REFRESH_TOKEN_TTL`, `DOWNSTREAM_ACCESS_TOKEN_SKEW`
- Webhook 暗号化: `WEBHOOK_SECRET_ENCRYPTION_KEY`
- OpenFGA: `OPENFGA_ENABLED`, store / model id, timeout, fail-closed
- ファイル: `FILE_STORAGE_DRIVER`, `FILE_LOCAL_DIR`, `FILE_S3_*`, `FILE_MAX_BYTES`, `FILE_ALLOWED_MIME_TYPES`
- レート制限: `RATE_LIMIT_*`（テナント単位の上書きはランタイムで反映）
- Outbox / data lifecycle / SCIM reconcile の各 worker フラグと間隔
- 可観測性: `OTEL_*`, `METRICS_*`, `LOG_LEVEL`, `LOG_FORMAT`, `READINESS_*`
- 各種 secret と feature flag

設定の読み込み元は環境変数を基本とし、起動時にバリデーションして不足を早期に検知する。`config/dotenv.go` で `.env` を読み込めるようにしている。

## 時刻の扱い

日時の扱いは最初に統一ルールを決めておく。

- DB では `timestamptz` を使い、UTC で保存する
- Go 内部の `time.Time` も UTC 基準で比較・計算する
- API では RFC 3339 形式でやり取りする
- JST など利用者向けのタイムゾーン変換と表示整形は frontend で行う
- 日付のみを扱う項目は、時刻付き値に安易に寄せず `date` 相当として別扱いする

## 認証・セキュリティ

### 認証方針

API surface ごとに認証方式を分ける。

- Browser: **BFF + HttpOnly Cookie**
- External client: **OAuth2 Bearer Token**（Zitadel が発行）
- M2M: **JWT Bearer Token**（`M2M_EXPECTED_AUDIENCE` / `M2M_REQUIRED_SCOPE_PREFIX` で検証）
- SCIM provisioning: **JWT Bearer Token**（`SCIM_BEARER_AUDIENCE` / `SCIM_REQUIRED_SCOPE`）

ブラウザに秘密情報を持たせない。`localStorage` に JWT を置かない。`HttpOnly`, `Secure`, `SameSite` 属性付き Cookie を使う。

### 推奨フロー（browser）

1. Vue からログイン要求を送る
2. Go が Zitadel と OIDC で本人確認するか、ローカルパスワード（`ENABLE_LOCAL_PASSWORD_LOGIN=true` の開発時のみ）で認証する
3. セッション ID を Cookie に設定する（Redis にセッション本体を保管）
4. 以後の API リクエストでは Cookie を検証する
5. 必要に応じて downstream OAuth で Zitadel の access / refresh token を暗号化保存し、外部 API 呼び出しに使う

### CSRF 方針

CSRF 防御は `SameSite=Lax` に加えて、cookie-to-header 方式の CSRF トークンを既定にする。

- セッション Cookie は `SESSION_ID` として `HttpOnly`, `Secure`, `SameSite=Lax` で保持する
- BFF は別途 `XSRF-TOKEN` Cookie を発行する。この Cookie は JavaScript から読めるが、認証情報は入れない
- Vue は `POST`, `PUT`, `PATCH`, `DELETE` のたびに `XSRF-TOKEN` を読み、`X-CSRF-Token` ヘッダーとして送る
- Go は state-changing request で `X-CSRF-Token` を必須とし、Cookie 内の値と照合する
- `GET`, `HEAD`, `OPTIONS` は CSRF トークン検証の対象外にする

### CSRF トークンの発行と更新

- `XSRF-TOKEN` はログイン成功時とセッション再発行時に払い出す
- SPA 初回ロード時にトークンが未取得なら、`/api/v1/session` のような bootstrap endpoint で取得する
- トークンは every request ではローテーションしない。ログイン、ログアウト、セッション更新、権限変更のタイミングで更新する
- frontend は token refresh を個別画面で処理せず、transport wrapper で吸収する

### frontend との接続ルール

生成 client と Cookie 認証を確実に両立させるため、ブラウザからの API 呼び出しは 1 つの transport wrapper に集約する。

- `credentials: "include"` を必須にする
- `XSRF-TOKEN` Cookie の読み出しと `X-CSRF-Token` ヘッダー付与を wrapper で共通化する
- 認証切れ、CSRF エラー、problem details の変換を wrapper で吸収する

### 外部クライアントと SCIM

- external API (`/api/v1/external/...`) は browser API と OpenAPI surface を分け、Bearer Token 認証のみを許可する
- `EXTERNAL_EXPECTED_AUDIENCE` / `EXTERNAL_REQUIRED_SCOPE_PREFIX` / `EXTERNAL_REQUIRED_ROLE` で検証ポリシーを制御する
- SCIM provisioning は `/api/scim/v2` 配下の独立した API surface として扱い、`SCIM_RECONCILE_*` で Zitadel との突き合わせジョブを動かす

### セッションストア

セッションの保存先は Redis を採用する。

- TTL 管理、セッション失効、複数インスタンス運用を単純にしやすい
- login state、idempotency key、rate limit カウンタも Redis に置く
- 開発時は `miniredis` でテストできるようにしている

### 認証基盤

OSS / self-hosted の **Zitadel** を採用する。

- 認証基盤の制御性を確保しやすい
- browser 向け Cookie セッションと external client 向け token 発行を整理しやすい
- Management API 経由でユーザー作成・招待・組織管理を行い、内部の `users` / `tenants` テーブルと SCIM reconcile で同期する
- 代わりに、可用性、アップグレード、監視、バックアップは自前で運用する前提になる

開発時は `AUTH_MODE=local` でローカルパスワードログインに切り替えられる（`ENABLE_LOCAL_PASSWORD_LOGIN=true`）。

### エラーハンドリング

API エラーは Huma の標準エラー形式（RFC 9457 problem details）をベースに統一する。

- バリデーションエラーは Huma の入力検証で返す
- 業務エラーは service で分類し、operation で適切な HTTP エラーに変換する
- frontend は HTTP ステータスと problem details の内容でハンドリングする

Go 側の実装では、業務エラーを機械可読な code を持つ domain error として表現する。

- service は `code`、利用者向け message、内部 cause を持つ domain error を返す
- operation は `code` を見て HTTP status と problem details に変換する
- frontend は文言ではなく `code` を基準に分岐する
- 業務判定のために ad-hoc な文字列比較や散在した sentinel error を増やさない

### ログ

ログは Go の構造化 JSON ログを前提に統一する。`log_type` で種類を分ける。

- `access`: HTTP access log。`method`, `path`, `status`, `latency_ms`, `request_id`
- `application_error`: handler が未分類 error を 500 に丸める直前の root cause。Postgres error は `sqlstate`, `severity`, `table`, `column`, `constraint` も付ける
- `panic`: panic recovery log（stack trace 付き）
- `migration_check`: 起動時 DB migration drift check

request body / Cookie / Authorization header / CSRF token / raw SQL result はログに出さない。stack trace を出すのは `panic` のときだけにする。

### 監視・オブザーバビリティ

- メトリクス: `prometheus/client_golang` で `/metrics` を公開（`METRICS_ENABLED=true`）
- トレース: OpenTelemetry。`otelgin` で gin に instrument し、OTLP HTTP exporter で送る。`OTEL_TRACES_SAMPLER_RATIO` でサンプリング率を制御
- 起動 readiness: postgres / redis / clickhouse / 必要なら zitadel に対する readiness check
- DB: `EXPLAIN ANALYZE` と PostgreSQL の標準統計で重いクエリを追えるようにする
- pprof: 必要に応じて導線を持つ

### セキュリティ対策

| 項目          | 方針                                                                                                                                                                              |
| ------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| XSS           | `v-html` を原則禁止し、CSP を `SECURITY_CSP` で設定する                                                                                                                            |
| CSRF          | `SameSite=Lax` + cookie-to-header の CSRF トークン。state-changing request では `X-CSRF-Token` を必須にする                                                                        |
| CORS          | browser / external で別管理。許可 Origin を明示し、`*` を使わない                                                                                                                  |
| SQL Injection | `sqlc` とパラメータ化クエリで防ぐ                                                                                                                                                  |
| 入力検証      | Huma と Go 側の両方で検証する                                                                                                                                                      |
| 権限制御      | operation ではなく service で最終判断する。Drive 系は OpenFGA に集約する                                                                                                            |
| Body サイズ   | `MAX_REQUEST_BODY_BYTES` / `DATASET_MAX_UPLOAD_BYTES` で middleware が制限する                                                                                                      |
| Rate limit    | login / browser API / external API ごとに異なる上限を持ち、テナント単位の上書きをランタイムで反映する                                                                              |
| Header        | `SECURITY_HEADERS_ENABLED`、HSTS、`X-Content-Type-Options`、`X-Frame-Options` などを middleware で付与する                                                                          |
| Webhook 機密   | webhook secret は `WEBHOOK_SECRET_ENCRYPTION_KEY` で encrypt-at-rest。`WEBHOOK_SECRET_KEY_VERSION` で鍵ローテーションを表現する                                                      |
| Token 機密    | downstream refresh token / SCIM provisioning token も同様に encrypt-at-rest                                                                                                         |

### Huma でのセキュリティ定義例

Cookie 認証スキームと Bearer 認証スキームは Huma の config / operation で登録し、その結果が OpenAPI に反映される。

```go
config := huma.DefaultConfig("HaoHao API", "1.0.0")
config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
    "SessionAuth": {
        Type: "apiKey",
        In:   "cookie",
        Name: "SESSION_ID",
    },
    "BearerAuth": {
        Type:         "http",
        Scheme:       "bearer",
        BearerFormat: "JWT",
    },
}
```

### DB 側の防御

- Row-Level Security を必要な箇所で使う
- 機微データは暗号化を検討する
- DB 接続ユーザーは最小権限にする
- ClickHouse はテナントごとの実行ユーザー / ロール、メモリ・行数・スレッド・実行秒数の上限で隔離する

## マイグレーション戦略

- `golang-migrate` 用の versioned SQL を `db/migrations/` に置く（現在 42 step）
- `db/migrations/` を正本とし、`db/schema.sql` は migration 適用後の DB から `make db-schema` で再生成する
- `sqlc` は再生成された `db/schema.sql` と `db/queries/` を入力にする
- CI では migration からスキーマを再構築し、`db/schema.sql` の更新漏れを検知する
- backend 起動時にも `migration_check` で DB が migration に追従しているかをチェック（`DB_MIGRATION_CHECK_MODE=warn|fail|off`、ローカル/CI は `fail` 推奨）
- スキーマ差分管理や drift 検知が将来必要になったら `Atlas` 併用を検討する

重要なのは、アプリケーションコードより前にスキーマ変更手順を整備すること。

## ページネーション方針

ページネーションは一律ではなく、用途ごとに `cursor` と `offset` を使い分ける。

- external client 向け API や更新頻度の高い一覧は `cursor` を使う
- 管理画面や総件数表示、任意ページジャンプが必要な画面は `offset` を使う
- 同じ資源でも browser 向け画面と external client 向け API で方式が異なることは許容する
- カーソル生成は `service/cursor.go` に共通化する

## テスト戦略

最低限、次の 5 層を用意する。

- 契約テスト: OpenAPI 3.1 の export と lint。3 surface すべてを対象にする
- 生成物テスト: OpenAPI artifact と frontend client の差分検知
- データ層テスト: migration を適用した実 PostgreSQL に対して `sqlc` のクエリを検証する
- アプリケーションテスト: Go test（service / middleware / jobs / auth に `*_test.go` 多数）と frontend の typecheck / build (`vue-tsc`)
- 結合 / E2E テスト: Playwright で `e2e/` 配下にログイン、Cookie 認証、CSRF、Drive、i18n、access fallback、埋め込み配信の smoke を持つ。さらに `scripts/smoke-*.sh` でドメイン別 smoke（operability / observability / openfga / rate-limit-runtime / file-purge / customer-signals / common-services / tenant-admin / backup-restore / p10）を回す

データ層テストは `miniredis` などで Redis を再現しつつ、必要に応じて実 PostgreSQL コンテナを起動する方針。

## CI で最低限やること

- OpenAPI 3.1 を 3 surface 分 export
- OpenAPI artifact の lint / validate
- 生成 client の更新漏れチェック
- migration 実行確認 + `db/schema.sql` の再生成漏れチェック
- `sqlc -f backend/sqlc.yaml generate` と `sqlc verify` / `sqlc vet`
- `golangci-lint`
- Go test
- Frontend lint / typecheck (`vue-tsc`) / build
- OpenFGA model のテスト (`fga model test --tests openfga/drive.fga.yaml`)
- Cookie 認証と CSRF の smoke test
- 埋め込み済み SPA のルーティング smoke test (`scripts/e2e-single-binary.sh`)
- バイナリサイズ閾値チェック (`make binary-size-check`)

この文書では、生成した OpenAPI artifact と frontend client をコミット対象とする。したがって、**CI で差分検知する**ことは必須。

## この構成の利点

- OpenAPI 3.1 を現実的に運用しやすい（surface 分割で browser と external のドリフトを分離）
- API 実装と公開契約のドリフトを小さくできる
- 型不整合による手戻りが減る
- 認可は OpenFGA に集約され、Drive のような関係ベース権限も追える
- SQL を隠蔽しすぎず、PostgreSQL の性能を活かせる。分析は ClickHouse、ファイルは S3 互換に逃がせる
- 単一バイナリ配信で運用が単純になる

## 採用済みの判断（このドキュメントの既定）

実装を進める前に決めるべき項目と、本リポジトリでの既定値。

- OpenAPI artifact をリポジトリに含める: **含める**
- OpenAPI 3.1 を唯一の公開契約として扱う: **そうする**
- OpenAPI surface 分割: **`full` / `browser` / `external` の 3 種類**
- docs / OpenAPI endpoint を本番で公開するか: **認証付きで公開**（`DOCS_AUTH_REQUIRED`）
- OpenAPI artifact の固定版配布: **GitHub Release / release asset**
- ID 戦略: **集約ごとに `bigint` と `uuidv7` を使い分ける**
- 認証基盤: **OSS / self-hosted の Zitadel**（開発時は `AUTH_MODE=local` を許容）
- external client 向け API: **browser とは別 OpenAPI surface + Bearer token**
- 認可: **OpenFGA（Drive / dataset の関係ベース権限）**
- API バージョニング: **URL prefix の `/api/v1`**（SCIM は `/api/scim/v2` で別 surface）
- ページネーション: **`cursor` / `offset` を使い分け**
- frontend 構成: **`views/` + `components/` + `stores/` + `api/`**（`tenant-admin/` などドメイン UI を切り出し可）
- セッションストア: **Redis**
- マイグレーション: **`golang-migrate`**
- ファイルストア: **ローカル FS（開発）/ S3 互換 SeaweedFS（本番想定）**
- 分析層: **ClickHouse**
- バックグラウンドジョブ: **outbox + scheduler + provisioning reconcile + data lifecycle + work table export + data pipeline scheduler**
- 可観測性: **構造化 JSON ログ + Prometheus + OpenTelemetry (OTLP HTTP)**
- 配信: **Vue build を Go バイナリに `embed_frontend` タグで埋め込む**

この構成なら、OpenAPI 3.1・開発速度・保守性・運用性のバランスがよい。

## 参照した公式情報

- Huma OpenAPI generation: https://huma.rocks/features/openapi-generation/
- Huma router adapter / `humagin`: https://huma.rocks/features/bring-your-own-router/
- Huma validation: https://huma.rocks/features/request-validation/
- Huma errors / RFC 9457: https://huma.rocks/features/response-errors/
- Huma CLI / spec export: https://huma.rocks/features/cli/
- `@hey-api/openapi-ts` の OpenAPI 3.1 対応: https://github.com/hey-api/openapi-ts
- OpenFGA: https://openfga.dev/
- Zitadel: https://zitadel.com/docs
- ClickHouse: https://clickhouse.com/docs
- SeaweedFS: https://github.com/seaweedfs/seaweedfs
