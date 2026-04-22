# `CONCEPT.md` を実装へ落とし込むチュートリアル提案

## この文書の目的

この文書は、`CONCEPT.md` に書かれた方針を、実際に手を動かして組み立てていくためのチュートリアルに変換したものです。

目的は要約ではありません。目的は、**「今どのファイルを作るべきか」「そのファイルには何を書くべきか」「なぜその順番なのか」**を、迷わず追えるようにすることです。

特にこのチュートリアルでは、次の 3 点を重視します。

- 1 ファイルずつ進められること
- 各ファイルの役割が明確であること
- 生成物と手書きファイルの境界がはっきりしていること

## 最初に作る機能

最初の縦切り機能として、**セッション確認とログイン導線**を題材にします。

具体的には、最低限次の API と画面が動くところまでを最初のゴールにします。

- `GET /api/v1/session`
- `POST /api/v1/login`
- `POST /api/v1/logout`
- `frontend` から Cookie 認証でそれらを呼び出す

この題材を選ぶ理由は単純です。`CONCEPT.md` の核になっている要素がほぼ全部入っているからです。

- Huma による OpenAPI 3.1 生成
- `sqlc` による SQL ベースのデータアクセス
- BFF + HttpOnly Cookie
- CSRF 対策
- 生成 client を使う Vue 側の接続
- 最終的な単一バイナリ配信

つまり、この最初の縦切り機能が作れれば、以後の業務機能は同じ型で増やしていけます。

## 完成イメージ

最終的な構成は、`CONCEPT.md` にある次の形をベースにします。

```text
my-enterprise-app/
├── docs/
├── openapi/
│   └── openapi.yaml
├── go.work
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   ├── views/
│   │   ├── composables/
│   │   ├── stores/
│   │   └── api/
│   ├── vite.config.ts
│   └── package.json
├── backend/
│   ├── cmd/
│   ├── internal/
│   │   ├── api/
│   │   ├── service/
│   │   ├── db/
│   │   ├── auth/
│   │   ├── config/
│   │   └── middleware/
│   ├── go.mod
│   ├── sqlc.yaml
│   ├── web/
│   │   └── dist/
│   └── embed.go
├── db/
│   ├── migrations/
│   ├── queries/
│   └── schema.sql
├── compose.yaml
├── docker/
│   └── Dockerfile
├── scripts/
│   └── gen.sh
├── .github/workflows/
└── Makefile
```

## このチュートリアルの読み方

この文書は、上から順に読んで、そのまま順番にファイルを作っていく前提で書いています。

進め方のルールは次の通りです。

1. まず「土台になるファイル」を作る
2. 次に「DB と SQL の正本」を作る
3. その後に「Go 側の API 契約と実装」を作る
4. 生成物を出力する
5. 最後に「Vue 側から生成 client を使って接続する」

この順番を崩すと、途中で次の問題が起きやすくなります。

- フロントの型が先にできてしまい、OpenAPI とずれる
- API 実装が先に進み、DB 設計が後追いになる
- 生成物の出所が曖昧になり、手書き修正が入り込む

## 実装順の全体像

| フェーズ | 主な対象ファイル | このフェーズの目的 |
| --- | --- | --- |
| 1 | `go.work`, `backend/go.mod`, `frontend/package.json`, `compose.yaml`, `Makefile`, `scripts/gen.sh` | リポジトリ全体の実行土台を作る |
| 2 | `db/migrations/*`, `db/schema.sql`, `db/queries/*`, `backend/sqlc.yaml` | DB と SQL を正本として固める |
| 3 | `backend/internal/*`, `backend/cmd/*`, `openapi/openapi.yaml` | Huma で API 契約と実装を結ぶ |
| 4 | `frontend/vite.config.ts`, `frontend/src/*` | 生成 client を transport wrapper 経由で使う |
| 5 | `backend/embed.go`, `docker/Dockerfile`, `.github/workflows/ci.yml` | 単一バイナリ配信と CI で仕上げる |

---

## フェーズ 1: まずリポジトリの土台を作る

### 1. `go.work`

- 役割: repo root から `./backend` を Go workspace として扱えるようにする
- この段階で決めること: root から `go run ./backend/...` を実行する運用にするかどうか
- 先に作る理由: `Makefile` と CI の基準点になるから

`CONCEPT.md` では、repo root に `go.work` を置いて `backend/` を独立 module として扱う方針でした。この方針は実務上かなり有効です。理由は、フロントエンドとバックエンドを monorepo に置きつつ、Go のモジュール境界は明確に保てるからです。

最初は次の最小形で十分です。

```go
go 1.24.0

use ./backend
```

このファイルを最初に置いておくと、以後のコマンド例をすべて repo root 基準で統一できます。

### 2. `backend/go.mod`

- 役割: Go 側の依存関係と module 名を定義する
- この段階で決めること: module path、Go のバージョン、主要ライブラリ
- 先に作る理由: これがないと `go.work` と `Makefile` の意味が出ないから

このファイルでは、最初から大量の依存を入れなくて構いません。まずは次を含む最小構成を目標にしてください。

- `gin`
- `huma`
- `humagin`
- `pgx/v5`
- `pgxpool`

`sqlc` 生成コードが入ることも見越して、PostgreSQL 周りのランタイムは早めに揃えておくと後で迷いません。

完了条件は、`go mod tidy` が通ることです。

### 3. `frontend/package.json`

- 役割: フロントエンドの依存と npm scripts を定義する
- この段階で決めること: Vue 周辺の初期構成と、ローカル開発・build・typecheck の入口
- 先に作る理由: フロントエンドも root から操作する前提を早めに固定したいから

`CONCEPT.md` の方針通り、ここでは次を採用します。

- `vue`
- `vite`
- `typescript`
- `pinia`
- `vue-router`
- `@hey-api/openapi-ts`

scripts は最低限次があると進めやすいです。

- `dev`
- `build`
- `typecheck`
- `lint`

この段階では、UI の実装よりも「生成 client を受け入れられるプロジェクトの器」を用意する意識が重要です。

### 4. `compose.yaml`

- 役割: ローカル開発用の依存サービスを起動する
- この段階で決めること: PostgreSQL 18 と Redis をどの名前・ポートで扱うか
- 先に作る理由: DB とセッションストアの接続先を後からぶらさないため

`CONCEPT.md` では、初期推奨構成として PostgreSQL 18 と Redis を置く方針でした。このファイルは、単にコンテナを立てるためのものではありません。**チーム全体のローカル前提を固定するファイル**です。

最低限、次を決めます。

- PostgreSQL の image と version
- DB 名、ユーザー、パスワード
- Redis のポート
- 永続ボリュームの有無

このファイルができたら、`docker compose up -d` で依存サービスが立ち上がる状態にします。

### 5. `Makefile`

- 役割: repo root で実行する標準コマンドを集約する
- この段階で決めること: 開発・生成・テスト・build の正式な入口
- 先に作る理由: 人によってコマンドがばらけるのを防ぐため

`Makefile` は便利コマンド集ではなく、**運用ルールを固定するファイル**です。特にこの構成では `make gen` が重要です。OpenAPI、TypeScript client、`sqlc` 生成の順番を統一する必要があるからです。

最初は次のようなターゲットがあると十分です。

```make
gen:
	./scripts/gen.sh

backend-dev:
	go run ./backend/cmd/main.go

frontend-dev:
	cd frontend && npm run dev

test:
	cd backend && go test ./...
	cd frontend && npm run typecheck
```

`make` の価値は短さではありません。**チーム全員が同じ順序で同じ処理を走らせること**にあります。

### 6. `scripts/gen.sh`

- 役割: 生成処理の順番を 1 か所に固定する
- この段階で決めること: 何を、どの順番で生成するか
- 先に作る理由: `make gen` と CI が同じ処理を共有できるから

このスクリプトには `set -euo pipefail` を入れて、途中失敗を必ず拾えるようにしてください。

処理順は `CONCEPT.md` の意図に沿って、次が基本です。

1. Huma から OpenAPI を export する
2. OpenAPI から TypeScript client を生成する
3. `sqlc` で Go コードを生成する

たとえば次のイメージです。

```bash
#!/usr/bin/env bash
set -euo pipefail

mkdir -p openapi
go run ./backend/cmd/openapi/main.go > openapi/openapi.yaml
cd frontend && npx @hey-api/openapi-ts -i ../openapi/openapi.yaml -o src/api/generated
cd ../backend && sqlc generate
```

このファイルを作る段階では、まだ全部動かなくて構いません。大事なのは、**生成物の流れを先に文章ではなくファイルで固定すること**です。

### フェーズ 1 の完了条件

- `go.work` と `backend/go.mod` があり、Go 側の依存が解決できる
- `frontend/package.json` があり、`npm install` が通る
- `compose.yaml` で PostgreSQL と Redis が起動できる
- `Makefile` と `scripts/gen.sh` があり、標準コマンドの入口が決まっている

---

## フェーズ 2: DB と SQL を先に固める

### 7. `db/migrations/0001_init.up.sql`

- 役割: 初期スキーマを作る正本
- この段階で決めること: テーブル、キー戦略、時刻カラム、制約
- 先に作る理由: `CONCEPT.md` の通り、アプリコードより先にスキーマ変更手順を整えるため

最初の migration では、題材にしているセッション導線に必要な最小テーブルだけ作れば十分です。

候補は次の通りです。

- `users`
- `sessions`

ここで `CONCEPT.md` の ID 戦略を反映させます。たとえば内部 join が多い `users.id` は `bigint`、外部に見せる識別子 `users.public_id` は `uuidv7()` といった形です。

このファイルで意識すべき点は次です。

- `created_at`, `updated_at` は `timestamptz`
- セッションの期限切れ用に `expires_at` を持つ
- 一意制約や外部キーは最初から明示する

重要なのは、**あとで使う SQL を想像しながら書くこと**です。ORM で隠す前提ではないので、migration 時点でテーブルの責務をかなり明確にしておきます。

### 8. `db/migrations/0001_init.down.sql`

- 役割: rollback 用の逆操作を書く
- この段階で決めること: 依存関係を壊さずに巻き戻せる順番
- 先に作る理由: migration の片方向化を防ぐため

このファイルは軽視されやすいですが、開発初期ほど重要です。理由は、初期のスキーマは何度も作り直すからです。

書き方の基本は単純です。

- 依存の深いテーブルから先に消す
- `up.sql` と対称になるように書く

完了条件は、空の DB に `up` を入れてから `down` できれいに戻せることです。

### 9. `db/schema.sql`

- 役割: 現在スキーマのスナップショット
- この段階で決めること: 手書きしない運用を徹底すること
- 先に作る理由: `sqlc` とレビューがこのファイルに依存するため

`CONCEPT.md` にもある通り、このファイルは migration 適用後の DB から再生成した結果を置く場所です。**手で編集しない**ことが重要です。

このファイルの役割は 2 つあります。

- `sqlc` の入力
- PR でのスキーマ差分レビュー

もしこのファイルを手で直し始めると、migration と実 DB と `sqlc` 入力の 3 つがずれます。ここは最初から運用ルールを強く決めておいてください。

### 10. `db/queries/session.sql`

- 役割: `sqlc` が読むアプリケーション用 SQL を置く
- この段階で決めること: どの操作を repository の公開 API にするか
- 先に作る理由: service を書く前に、DB に何を頼めるかを明確にするため

最初の機能に必要な SQL は多くありません。たとえば次のような単位です。

- セッション ID から現在ユーザーを引く
- セッションを作る
- セッションを削除する
- ログイン名やメールからユーザーを引く

このファイルを設計するときのポイントは、**service が欲しい操作単位で SQL を切る**ことです。テーブル単位で雑に並べないほうが、あとで読みやすくなります。

### 11. `backend/sqlc.yaml`

- 役割: `sqlc` の入力と出力先を定義する
- この段階で決めること: `schema.sql`、`queries`、生成先 package
- 先に作る理由: DB 設計から Go コードへの橋を作るため

このファイルでは、少なくとも次を決めます。

- schema の場所
- queries の場所
- 出力先を `backend/internal/db/` にすること
- `sql_package` を `pgx/v5` 系に寄せること

イメージは次のようになります。

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "../db/schema.sql"
    queries: "../db/queries"
    gen:
      go:
        package: "db"
        out: "internal/db"
        sql_package: "pgx/v5"
```

完了条件は、`cd backend && sqlc generate && sqlc vet && sqlc verify` が通ることです。

### フェーズ 2 の完了条件

- migration の `up` と `down` が動く
- `db/schema.sql` が migration 由来で再生成されている
- `db/queries/*.sql` が最小機能を表現できている
- `backend/sqlc.yaml` から `sqlc generate` が通る

---

## フェーズ 3: Go 側で API 契約と実装をつなぐ

### 12. `backend/internal/config/config.go`

- 役割: 環境変数と起動設定を集約する
- この段階で決めること: DB 接続、Redis 接続、Cookie 名、公開 URL、環境種別
- 先に作る理由: 後続のファイルで設定読み込み方法をばらけさせないため

`CONCEPT.md` では、設定は `backend/internal/config/` に集約し、起動時にバリデーションする方針でした。このファイルは、その原則を具体化する最初の要です。

最低限、次は入れてください。

- `DATABASE_URL`
- `REDIS_ADDR`
- `SESSION_COOKIE_NAME`
- `XSRF_COOKIE_NAME`
- `APP_ENV`
- `FRONTEND_ORIGIN`

このファイルのポイントは、**値を取ることより、欠けていたら即落とすこと**です。

### 13. `backend/internal/db/pool.go`

- 役割: `pgxpool` の初期化とライフサイクル管理を行う
- この段階で決めること: 接続タイムアウト、ping、close の責務
- 先に作る理由: service や repository が接続方法を意識しないようにするため

ここでは「グローバル変数をどこでも触れるようにする」のではなく、**アプリケーションの入口で作り、依存として渡す**構成にしてください。

最低限の責務は次です。

- `config` から接続文字列を受け取る
- `pgxpool.New` する
- 起動時に `Ping` する
- 終了時に `Close` できるようにする

### 14. `backend/internal/auth/session.go`

- 役割: セッション Cookie とセッション識別の扱いを集約する
- この段階で決めること: Cookie 名、TTL、セッション ID の表現、検証の入口
- 先に作る理由: handler や service が生の Cookie を直接触らないようにするため

`CONCEPT.md` の認証方針は BFF + HttpOnly Cookie です。つまり、ブラウザに機密トークンを持たせないことが前提です。このファイルでは、その前提をコードの境界として表現します。

このファイルで少なくとも整理したいのは次です。

- セッション Cookie の発行
- Cookie からセッション ID を取り出す処理
- 有効期限の扱い
- セッション失効時の共通処理

最初から Redis 実装まで一気に書かなくても構いません。ただし、interface を切っておくと後で差し替えやすくなります。

### 15. `backend/internal/auth/csrf.go`

- 役割: `XSRF-TOKEN` Cookie と `X-CSRF-Token` ヘッダー検証を集約する
- この段階で決めること: state-changing request の判定と照合ルール
- 先に作る理由: セキュリティの重要ロジックを各 handler に散らさないため

`CONCEPT.md` では、`SameSite=Lax` に加えて cookie-to-header 方式を採用していました。ここは必ず共通化してください。

このファイルの責務は明確です。

- トークンを発行する
- Cookie に載せる
- `POST`, `PUT`, `PATCH`, `DELETE` でヘッダーと照合する

この責務が分離されていると、フロントエンド側の transport wrapper と綺麗に対応します。

### 16. `backend/internal/middleware/request_context.go`

- 役割: リクエスト単位の共通処理をまとめる
- この段階で決めること: request ID、ログ、認証済みコンテキストの入れ方
- 先に作る理由: Huma operation の責務を細く保つため

`CONCEPT.md` では、operation は入出力定義と validation に集中し、業務ロジックを書き込まない方針でした。このファイルはその方針を守るための補助線です。

ここで入れたいものは次です。

- request ID 発行
- 構造化ログの基本項目
- セッションからユーザー情報を読み取って context に入れる処理

この段階で全部作り込まなくてもよいですが、少なくとも **共通処理を handler に書かない** という型だけは先に作ってください。

### 17. `backend/internal/service/session_service.go`

- 役割: セッション関連の業務ルールを担当する
- この段階で決めること: login、logout、current session の責務分離
- 先に作る理由: API 層が DB と認証の詳細を直接知らないようにするため

このファイルでは、少なくとも次のメソッドをイメージして設計すると進めやすいです。

- `GetCurrentSession`
- `Login`
- `Logout`

ここで重要なのは、**service が SQL を書かないこと**ではありません。正確には、**service は SQL の文字列や HTTP の都合を持たないこと**が重要です。

service の責務は次です。

- 認可確認
- 業務バリデーション
- repository 呼び出し
- 必要ならトランザクション制御

### 18. `backend/internal/api/register.go`

- 役割: Huma operation の登録を 1 か所に集約する
- この段階で決めること: server 起動時と OpenAPI export 時に同じ登録処理を使うこと
- 先に作る理由: 実装と spec export のドリフトを防ぐため

このファイルは地味ですが重要です。`cmd/main.go` と `cmd/openapi/main.go` の両方が、同じ operation 登録関数を呼ぶ構成にしてください。

そうしておくと、次の状態を避けられます。

- サーバーでは出ている endpoint が export されない
- export 専用コードだけ別の設定を読んでしまう

### 19. `backend/internal/api/session.go`

- 役割: Huma の request / response struct と operation 定義を書く
- この段階で決めること: API 契約、validation、security metadata、エラー変換
- 先に作る理由: ここが OpenAPI 3.1 の正本になるため

このファイルは、`CONCEPT.md` で最も強く重視されていた部分です。hand-written YAML ではなく、**Go の型とタグが契約の正本**になります。

このファイルでやることは次です。

- `GET /api/v1/session` の output struct を定義する
- `POST /api/v1/login` の input / output struct を定義する
- `POST /api/v1/logout` を定義する
- 認証が必要な operation に security scheme を付ける
- service が返す domain error を problem details に変換する

ここで大切なのは、operation に業務ロジックを埋め込まないことです。operation は「HTTP と service の接着剤」にとどめます。

### 20. `backend/cmd/main.go`

- 役割: バックエンドの起動入口
- この段階で決めること: Gin、Huma、middleware、docs、静的配信の組み立て
- 先に作る理由: ローカルで API を立ち上げて挙動確認するため

このファイルでは、少なくとも次の流れが見えるようにしてください。

1. `config` を読む
2. DB / Redis などの依存を初期化する
3. Gin router を作る
4. Huma を載せる
5. `internal/api/register.go` で operation を登録する
6. docs と OpenAPI endpoint を出す
7. 本番では静的ファイルも返せる構成にする

この段階では、`embed.FS` での SPA 配信まで完全でなくても構いません。まずは API と docs が立ち上がることを優先してください。

### 21. `backend/cmd/openapi/main.go`

- 役割: サーバーを起動せずに OpenAPI YAML を stdout に出す
- この段階で決めること: CI でも使える export 導線にすること
- 先に作る理由: `make gen` の最初の一歩になるため

`CONCEPT.md` でも、サーバー起動中の endpoint を叩く方法より、起動不要な custom command のほうが扱いやすいと書かれていました。このファイルはまさにそのために作ります。

理想形は、`cmd/main.go` とほぼ同じ API 登録処理を使い、最後だけ「listen するか、spec を出力するか」が違う状態です。

### 22. `openapi/openapi.yaml`

- 役割: 公開契約としてコミットする OpenAPI artifact
- この段階で決めること: 手書きせず、必ず生成する運用を守ること
- 先に作る理由: フロントエンド生成物と PR レビューがこれを前提に進むため

このファイルは成果物ですが、手を抜いてはいけないファイルです。理由は、**このファイルがフロントエンドと外部公開契約の中間地点になるから**です。

ここで必ず確認してください。

- path が `/api/v1/...` になっているか
- security scheme が反映されているか
- request / response の schema が期待通りか

### フェーズ 3 の完了条件

- `go run ./backend/cmd/main.go` で API が起動する
- docs / OpenAPI endpoint が確認できる
- `go run ./backend/cmd/openapi/main.go > openapi/openapi.yaml` が通る
- `openapi/openapi.yaml` に session 系 endpoint が出力される

---

## フェーズ 4: Vue 側を生成 client につなぐ

### 23. `frontend/vite.config.ts`

- 役割: 開発 proxy と本番 build 出力先を決める
- この段階で決めること: `/api` の転送先と `backend/web/dist/` への出力
- 先に作る理由: 開発時の接続と本番時の配信先を最初から揃えるため

`CONCEPT.md` の重要なポイントの 1 つが、開発時はフロントとバックを分け、本番では Go バイナリに埋め込むことでした。このファイルは、その 2 つの環境をつなぐ接点です。

最低限の設定は次です。

- `server.proxy` で `/api` を Go 側へ流す
- `build.outDir` を `../backend/web/dist` にする

このファイルが正しくないと、開発時の接続と本番 build の行き先がずれて、後からまとめて壊れます。

### 24. `frontend/src/main.ts`

- 役割: Vue アプリの起動入口
- この段階で決めること: Pinia と router の初期化順
- 先に作る理由: store や route を作っても mount 入口がないと確認できないため

このファイルは薄く保ってください。責務はあくまで bootstrapping です。

ここでやることは次の程度で十分です。

- `createApp`
- `createPinia`
- `router` の適用
- `App.vue` の mount

### 25. `frontend/src/router/index.ts`

- 役割: 画面遷移の定義をまとめる
- この段階で決めること: public route と protected route の分離
- 先に作る理由: session store と画面の責務境界を明確にするため

最初の段階では、次の 2 画面があれば十分です。

- ログイン画面
- ログイン後ホーム画面

ルーティングで大事なのは、**認可判定を router だけに閉じ込めないこと**です。最終判断は store と API 応答に寄せて、router guard は薄く保ったほうが保守しやすいです。

### 26. `frontend/src/App.vue`

- 役割: 画面全体のシェルを定義する
- この段階で決めること: グローバルな loading、error、layout の置き場所
- 先に作る理由: 各画面の責務を細くするため

このファイルは UI を派手にする場所ではありません。最初の段階では「全体の枠」だけを作る意識で十分です。

役割は次です。

- `RouterView` を置く
- グローバルな通知やエラー表示の置き場を確保する

### 27. `frontend/src/api/generated/*`

- 役割: OpenAPI から生成された TypeScript client と型
- この段階で決めること: 手書き修正をしない運用
- 先に作る理由: Vue 側の API 呼び出しを契約ベースに切り替えるため

このディレクトリは「読むことはあるが書かない場所」です。ここに修正を入れたくなったら、直すべき場所は `backend/internal/api/*` か `openapi/openapi.yaml` です。

ここは生成物として次のルールを強く守ってください。

- 手で直さない
- 差分が出たら PR でレビューする
- 呼び出しは wrapper 経由に限定する

### 28. `frontend/src/api/client.ts`

- 役割: 生成 client をそのまま使わず、transport wrapper で包む
- この段階で決めること: Cookie 認証、CSRF 付与、エラー変換、base URL
- 先に作る理由: `CONCEPT.md` が最も強く要求していた frontend 側の責務分離だから

このファイルは非常に重要です。画面から生成 client を直接呼ばせない理由は、認証・CSRF・エラーハンドリングの共通処理を 1 か所に集めるためです。

ここでやるべきことは次です。

- `credentials: "include"` を既定にする
- `XSRF-TOKEN` Cookie を読む
- `POST`, `PUT`, `PATCH`, `DELETE` に `X-CSRF-Token` を付ける
- problem details を UI 向けエラーに変換する

この wrapper があると、Vue の各画面は「どの API を呼ぶか」だけに集中できます。

### 29. `frontend/src/stores/session.ts`

- 役割: セッション状態を Pinia にまとめる
- この段階で決めること: 未認証、確認中、認証済みの状態管理
- 先に作る理由: 複数画面でセッション確認処理を重複させないため

この store には、最低限次の責務を持たせると分かりやすいです。

- 初回 bootstrap で `GET /api/v1/session` を呼ぶ
- `login` を呼ぶ
- `logout` を呼ぶ
- 現在ユーザーの基本情報を保持する

ここで大切なのは、**画面から API 詳細を追い出すこと**です。画面は store を呼び、store が wrapper 経由で API を叩く流れにすると責務が綺麗に分かれます。

### 30. `frontend/src/views/LoginView.vue`

- 役割: ログイン導線の最小 UI
- この段階で決めること: 入力フォーム、送信状態、エラー表示
- 先に作る理由: session store の入口を最初に確認できるため

最初のログイン画面は簡素で構いません。重要なのは見た目ではなく、次の流れが確認できることです。

- 入力を受け取る
- store の `login` を呼ぶ
- 成功時に遷移する
- 失敗時にエラーを出す

### 31. `frontend/src/views/HomeView.vue`

- 役割: 認証後の状態確認画面
- この段階で決めること: 現在ユーザー情報の表示と logout 導線
- 先に作る理由: ログイン後のセッション状態を最も簡単に観察できるから

この画面は「本番のホーム画面」を完成させるためのものではありません。最初は、現在ユーザーの情報とログアウト導線があれば十分です。

ここで確認したいのは次です。

- Cookie 認証で `GET /api/v1/session` が通ること
- logout 後に未認証状態へ戻れること

### フェーズ 4 の完了条件

- `npm run dev` でフロントが起動する
- Vite の proxy 経由で `/api/v1/session` が呼べる
- 生成 client を直接ではなく wrapper 経由で使っている
- login / logout / current session の一連の導線が画面から確認できる

---

## フェーズ 5: 配信と CI まで閉じる

### 32. `backend/embed.go`

- 役割: `frontend` の build 結果を Go バイナリへ埋め込む
- この段階で決めること: どのディレクトリを embed 対象にするか
- 先に作る理由: `CONCEPT.md` の「単一バイナリ配信」を最後に実現するため

ここでは `backend/web/dist/` を `//go:embed` の対象にします。

イメージは次の通りです。

```go
package backend

import "embed"

//go:embed web/dist/*
var Frontend embed.FS
```

このファイル単体では完成しません。`cmd/main.go` 側で、静的アセットの存在確認、SPA fallback、`/api/*` の除外を正しく処理する必要があります。

### 33. `docker/Dockerfile`

- 役割: フロント build と Go build をまとめた本番イメージを作る
- この段階で決めること: 多段階ビルド、Node と Go の責務分離
- 先に作る理由: 開発構成と本番構成の差を最小化するため

このファイルでは次の流れを表現します。

1. Node で `frontend` を build する
2. build 結果を `backend/web/dist/` に置く
3. Go バイナリを build する
4. 実行用の小さいイメージへ入れる

このとき大事なのは、**ローカルで行っていた build 手順と Docker 内の build 手順を一致させること**です。

### 34. `.github/workflows/ci.yml`

- 役割: 生成漏れ、migration 漏れ、型崩れ、build 崩れを自動検知する
- この段階で決めること: 何を必須チェックにするか
- 先に作る理由: この構成は生成物が多いため、CI がないとすぐずれるため

`CONCEPT.md` の方針をそのまま CI のチェック項目に落とすと、最低限次が必要です。

- OpenAPI export
- OpenAPI artifact の validate
- 生成 client の更新漏れチェック
- migration 実行確認
- `sqlc generate`
- `sqlc verify`
- `sqlc vet`
- Go test
- Frontend typecheck / build
- Cookie 認証と CSRF の smoke test
- 埋め込み済み SPA の routing smoke test

このファイルの重要点は、テストの多さではありません。**「生成物がコミットされたか」を CI が必ず見張ること**です。

### フェーズ 5 の完了条件

- `frontend` を build すると `backend/web/dist/` に出力される
- Go バイナリ 1 つで API と SPA を返せる
- Docker build が通る
- CI で生成漏れと build 崩れを検知できる

---

## 途中で迷わないための判断基準

実装中に「この処理はどこに置くべきか」で迷ったら、次の基準で判断してください。

- HTTP の入出力、validation、OpenAPI metadata なら `backend/internal/api/`
- 業務ルールなら `backend/internal/service/`
- SQL の形に落ちるなら `db/queries/` と `backend/internal/db/`
- 認証、Cookie、CSRF の横断処理なら `backend/internal/auth/`
- フロントの API 接続共通処理なら `frontend/src/api/client.ts`
- 画面状態なら `frontend/src/stores/`
- 画面表示そのものなら `frontend/src/views/`

この境界を守ると、`CONCEPT.md` の中心思想である「責務分離」と「契約ドリフトの最小化」が自然に実装へ反映されます。

## 生成物として扱うファイル

次のファイルは原則として手で編集しません。

- `openapi/openapi.yaml`
- `frontend/src/api/generated/*`
- `backend/internal/db/*` の `sqlc` 生成ファイル
- `db/schema.sql`

これらに差分が出たときは、「このファイルを直す」のではなく「何を正本として変えるべきか」を考えてください。

- OpenAPI が変なら `backend/internal/api/*`
- SQL 生成コードが変なら `db/queries/*` か `backend/sqlc.yaml`
- schema が変なら `db/migrations/*`

## この順番を勧める理由

このチュートリアルは、見た目のわかりやすさよりも、後戻りの少なさを優先して順番を決めています。

まず DB と SQL を先に置くのは、`sqlc` を使う構成では SQL が設計資産だからです。次に Go 側で Huma operation を作るのは、OpenAPI 3.1 を公開契約として正しく出すためです。最後に Vue 側をつなぐのは、生成 client を受け取る側としてフロントを扱うと責務がぶれにくいからです。

逆に、先に画面から作り始めると、最初は速く見えても後で次のような揺れが出ます。

- path や payload 名が変わるたびに画面と API を両方手修正する
- generated client と wrapper の責務が混ざる
- OpenAPI artifact がレビューされない

## 最後に: 最小の 1 周目で目指す状態

最初の 1 周目では、全部を完成させる必要はありません。まずは次の状態を目指してください。

- PostgreSQL と Redis が `compose.yaml` で起動する
- migration と `sqlc` 生成が通る
- Huma で `session`, `login`, `logout` の OpenAPI が出る
- Vue から generated client + wrapper 経由で session API を呼べる
- frontend を build して Go から配信できる

ここまで到達すれば、このリポジトリは「概念だけの設計書」ではなく、**次の機能を同じ型で増やせる実装基盤**になります。

その後は、同じ順番で機能を増やしてください。

1. migration を追加する
2. `db/queries/*` を増やす
3. service を増やす
4. Huma operation を追加する
5. OpenAPI を再生成する
6. frontend store / view を増やす

この反復が安定して回るなら、`CONCEPT.md` の設計は正しく実装へ変換できています。
