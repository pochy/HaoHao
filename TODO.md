# TODO

このファイルは、現在の最小骨格から次に進める作業を整理するためのメモです。

## GitHub 運用整備（実施済み）

- `Issue 1-1` から `Issue 5-6` までを GitHub Issue `#1`-`#24` として起票済み
- label を運用用に整理済み（`priority:*`, `area:*`, `track:phase-*`, `dependencies`）
- milestone を整備済み
  - フェーズ管理: `M1`-`M5`
  - リリース管理: `v0.1 Foundation`, `v0.2 Auth`, `v0.3 First Feature`
- GitHub Project `HaoHao Roadmap TODO 1-5` を作成し、`#1`-`#24` を追加済み
- Project custom field `Priority`, `Area`, `Risk`, `Target Release` を作成済み
- `.github/ISSUE_TEMPLATE/`, `.github/pull_request_template.md`, `.github/CODEOWNERS`, `.github/SECURITY.md` を追加済み
- `main` の branch protection を設定済み（PR 必須、Code Owner review 必須、force push 禁止、`ci-codeql` 必須）

## 先に差し込む: OpenAPI docs 導線

`CONCEPT.md` に合わせて、OpenAPI docs のパスと役割を早めに固定する。

- `/docs`: Huma built-in docs route
- `/openapi`: OpenAPI Documents (`Stoplight Elements` / `Scalar` / `Swagger UI`)
- `/openapi.json`: raw JSON
- `/openapi.yaml`: raw YAML

最初にやること:

- `backend/internal/app/app.go`, `README.md`, `TODO.md` の表記をこの 4 本にそろえる
- `/openapi` の既定 renderer を `Stoplight Elements` として実装する
- `Scalar`, `Swagger UI`, `none` を config で切り替えられるようにする
- `/docs`, `/openapi`, `/openapi.json`, `/openapi.yaml` に同じ auth policy を適用する
- local verification と smoke test に `/openapi` を追加する

## 次に進む場所

今の skeleton から着手するなら、まずは次を優先します。

1. browser API と external API の境界、および認証方式の分離を明確にする
2. `make gen` を前提に OpenAPI / client / sqlc の CI 差分検知を入れる
3. auth / session / CSRF の stub を本実装へ置き換える
4. `/docs`, `/openapi`, raw spec の導線と認証を stub から実装に切り替える
5. DB query と service を増やして業務機能を 1 本通す

### 1. browser API と external API の境界、および認証方式の分離を明確にする

何を決めるか:

- browser 向け API で扱う責務を決める
- external client 向け API で扱う責務を決める
- browser 側は Cookie session、external 側は bearer token を使う方針を明文化する
- 同じ resource を両方に出す場合、path と schema を共通化するか分けるか決める

何をやるか:

- `backend/internal/api/browser/` と `backend/internal/api/external/` の責務コメントを具体化する
- browser 側にだけ置く endpoint と external 側にだけ置く endpoint の例を 1 つずつ決める
- Huma config / operation に browser 用 security と external 用 security の定義を入れる
- `openapi/openapi.yaml` 上で security scheme が分かれて見える状態にする
- `README.md` に browser API と external API の使い分けを短く追記する

どこを触るか:

- `backend/internal/app/app.go`
- `backend/internal/api/browser/`
- `backend/internal/api/external/`
- `openapi/openapi.yaml`
- `README.md`

完了条件:

- browser 向けと external 向けの境界を説明できる
- OpenAPI 上で security scheme の分離が確認できる
- 以後 auth 実装を進めても API surface の手戻りが出にくい

対応 Issue:

- [#1 browser API と external API の責務境界を文書化する](https://github.com/pochy/HaoHao/issues/1)
- [#2 browser API 向け Cookie auth の security scheme を追加する](https://github.com/pochy/HaoHao/issues/2)
- [#3 external API 向け bearer token の security scheme を追加する](https://github.com/pochy/HaoHao/issues/3)
- [#4 external API 用の最小 endpoint を追加して browser API と分離する](https://github.com/pochy/HaoHao/issues/4)

### 2. `make gen` を前提に OpenAPI / client / sqlc の CI 差分検知を入れる

何を決めるか:

- generated artifact を commit 対象とする運用を CI でどう守るか決める
- `make gen` を唯一の生成入口にするか決める

何をやるか:

- CI で `make gen` を実行する workflow を追加する
- 実行後に `git diff --exit-code` 相当で差分検知する
- OpenAPI artifact の validate / lint を追加する
- `sqlc generate`, `sqlc verify`, `sqlc vet` を CI に追加する
- frontend generated client の更新漏れを CI で検知する
- `db/schema.sql` の更新漏れを CI で検知する

どこを触るか:

- `.github/workflows/`
- `Makefile`
- 必要なら補助 script

完了条件:

- API / query / schema を変えたのに生成物を更新し忘れた PR が CI で落ちる
- `make gen` を叩けばローカルと CI の結果が揃う

対応 Issue:

- [#5 CI で make gen を実行する workflow を追加する](https://github.com/pochy/HaoHao/issues/5)
- [#6 make gen 実行後の git diff 差分検知を CI に追加する](https://github.com/pochy/HaoHao/issues/6)
- [#7 OpenAPI artifact の validate と lint を CI に追加する](https://github.com/pochy/HaoHao/issues/7)
- [#8 sqlc generate / verify / vet を CI に追加する](https://github.com/pochy/HaoHao/issues/8)

### 3. auth / session / CSRF の stub を本実装へ置き換える

何を決めるか:

- login 完了後に何を session に保存するか決める
- session TTL、refresh 方針、logout 時の失効方針を決める
- CSRF token をいつ発行し、いつ更新するか決める

何をやるか:

- session store を Redis に接続する
- `SESSION_ID` cookie の発行・再発行・削除を実装する
- `XSRF-TOKEN` cookie の発行と `X-CSRF-Token` header の検証を実装する
- state changing request に対して CSRF middleware を有効にする
- `GET /api/v1/session` を bootstrap endpoint として整備する
- login / logout / session refresh の最小 endpoint を追加する
- transport wrapper で auth error / CSRF error の扱いを共通化する

どこを触るか:

- `backend/internal/service/`
- `backend/internal/middleware/`
- `backend/internal/api/browser/`
- `backend/internal/config/`
- `frontend/src/shared/lib/http/transport.ts`
- `frontend/src/features/session/`

完了条件:

- browser から Cookie session で認証状態を維持できる
- `POST`, `PUT`, `PATCH`, `DELETE` で CSRF 検証が通る
- logout で session と cookie が失効する

対応 Issue:

- [#9 Redis ベースの session store を実装する](https://github.com/pochy/HaoHao/issues/9)
- [#10 SESSION_ID cookie の lifecycle を実装する](https://github.com/pochy/HaoHao/issues/10)
- [#11 GET /api/v1/session を session bootstrap の本実装に置き換える](https://github.com/pochy/HaoHao/issues/11)
- [#12 XSRF-TOKEN 発行と CSRF middleware を実装する](https://github.com/pochy/HaoHao/issues/12)
- [#13 login / logout / session refresh の最小 endpoint を追加する](https://github.com/pochy/HaoHao/issues/13)
- [#14 frontend の transport wrapper と session store を auth 本実装に合わせて更新する](https://github.com/pochy/HaoHao/issues/14)

### 4. `/docs`, `/openapi`, raw spec の導線と認証を stub から実装に切り替える

何を決めるか:

- `/docs` と `/openapi` の役割分担を決める
- `/openapi` の既定 renderer を `Stoplight Elements` にする前提で、`Scalar` / `Swagger UI` の切り替え方法を決める
- docs / OpenAPI を誰に見せるか決める
- browser の session で見せるか、専用 role / scope で見せるか決める
- 本番と開発で公開条件を分けるか決める

何をやるか:

- `DOCS_BEARER_TOKEN` の暫定ロジックを外す
- `/openapi` route を追加し、`Stoplight Elements` / `Scalar` / `Swagger UI` を切り替えられるようにする
- docs / openapi へのアクセス制御を実際の auth context に接続する
- docs 閲覧権限の判定を middleware に実装する
- `/docs`, `/openapi`, `/openapi.yaml`, `/openapi.json` で共通の認可判定を使う
- unauthorized / forbidden のレスポンスを揃える
- `README.md` に開発時と本番時の docs 公開条件を書く

どこを触るか:

- `backend/internal/middleware/docs_auth.go`
- `backend/internal/app/app.go`
- 必要なら auth context を扱う service / middleware
- `README.md`

完了条件:

- `/docs`, `/openapi`, raw spec が本番で無条件公開されない
- `/openapi` で `Stoplight Elements` を既定に docs UI が見える
- 開発時の確認方法と本番時の公開条件が文書で揃う

対応 Issue:

- [#15 docs / OpenAPI の閲覧権限方針を文書化する](https://github.com/pochy/HaoHao/issues/15)
- [#16 DOCS_BEARER_TOKEN ベースの暫定認証を廃止する](https://github.com/pochy/HaoHao/issues/16)
- [#17 docs / OpenAPI 用の認可 middleware を auth context ベースで実装する](https://github.com/pochy/HaoHao/issues/17)
- [#18 /docs /openapi.yaml /openapi.json の認可結果とレスポンスを揃える](https://github.com/pochy/HaoHao/issues/18)

### 5. DB query と service を増やして業務機能を 1 本通す

何を決めるか:

- 最初に通す業務機能を 1 つ決める
- その機能で必要な table, query, endpoint, page を最小単位で切る

何をやるか:

- 対象機能の migration を追加する
- `db/schema.sql` を更新する
- `db/queries/` に sqlc 用 query を追加する
- `make gen` で `backend/internal/db/` を再生成する
- `service` から sqlc generated code を呼ぶ
- Huma operation を追加して `/api/v1/...` に接続する
- frontend feature を追加して generated client 経由で呼ぶ
- page からその feature を表示する
- 最低限の test と smoke check を追加する

どこを触るか:

- `db/migrations/`
- `db/schema.sql`
- `db/queries/`
- `backend/internal/db/`
- `backend/internal/service/`
- `backend/internal/api/browser/`
- `frontend/src/features/`
- `frontend/src/pages/`

完了条件:

- 1 つの業務機能が DB -> service -> API -> generated client -> UI まで通る
- その変更に対して `make gen` と最低限のテストが回る

対応 Issue:

- [#19 最初に縦通しする業務機能を 1 つ決める](https://github.com/pochy/HaoHao/issues/19)
- [#20 対象機能の migration と schema.sql を追加する](https://github.com/pochy/HaoHao/issues/20)
- [#21 対象機能の query と sqlc generated code を追加する](https://github.com/pochy/HaoHao/issues/21)
- [#22 対象機能の service と Huma operation を追加する](https://github.com/pochy/HaoHao/issues/22)
- [#23 対象機能の frontend feature と page を追加する](https://github.com/pochy/HaoHao/issues/23)
- [#24 対象機能の最低限の test と smoke check を追加する](https://github.com/pochy/HaoHao/issues/24)

## High Priority

- Zitadel を使う前提で browser 向け認証フローの境界を決める
- browser API と external client API の責務と認証方式を分離する
- Redis を使った session store を実装する
- CSRF token の発行・検証・更新を placeholder から本実装に置き換える
- `/docs`, `/openapi`, raw spec の導線と認証を stub から実装に切り替える
- backend に config validation を追加する
- `make gen` の生成結果を CI で差分検知できるようにする

## Auth / Security

- `SESSION_ID` cookie の属性を本実装する
- `HttpOnly`, `Secure`, `SameSite`, domain, path の方針を config に落とす
- login / logout / session refresh の導線を追加する
- `XSRF-TOKEN` cookie と `X-CSRF-Token` header の照合を実装する
- browser 向け Cookie auth と external client 向け bearer token の OpenAPI security scheme を分離する
- CORS の許可 Origin を config で制御する
- CSP の最小構成を追加する
- `v-html` を使わない方針を frontend 側のルールとして明示する
- 認可の最終判断を operation ではなく service 側で行う形にそろえる

## API / Backend

- `service` 層と `repository` 層の責務分離を広げる
- domain error と problem details のマッピング方針を実装する
- browser API の operation ごとに security を明示する
- external client 向け API surface を追加する
- `/api/v1` の versioning ルールを実装方針として固定する
- request logging と request ID の導線を整える
- `pprof` とアプリケーションメトリクスの差し込み点を追加する
- static asset 配信と API 配信の責務境界を整理する

## OpenAPI / Contract

- OpenAPI 3.1 を唯一の公開契約として運用するルールを明文化する
- OpenAPI artifact 更新手順を `make gen` 前提で固定する
- OpenAPI export を CI で必須チェックにする
- OpenAPI artifact の lint / validate を追加する
- generated client の差分検知を CI に追加する
- browser 向け契約と external client 向け契約の分離方針を固める
- `/docs`, `/openapi`, raw spec の公開条件と配布ルートを整理する
- GitHub Release / release asset での固定版配布を整備する

## Database

- migration の運用ルールを決める
- migration から `db/schema.sql` を再生成する流れを整える
- sqlc 用 query を追加し、repository 実装へ接続する
- `sqlc generate`, `verify`, `vet` を標準フローに組み込む
- `uuidv7()` 利用テーブルと `bigint` 利用テーブルの基準を明文化する
- PostgreSQL 18 前提の初期 extension / settings を必要に応じて整理する
- `pgxpool` の初期化と lifecycle を追加する
- `timestamptz` + UTC 運用をコードとテーブル設計に固定する
- 必要箇所で Row-Level Security の適用可否を検討する

## Frontend

- 画面 routing を導入する
- generated client のエラーハンドリングを transport wrapper に寄せる
- problem details を UI 用エラー表現へ変換する共通層を作る
- feature 単位で API adapter / store / UI の分離を進める
- session bootstrap と auth state の扱いを整理する
- token refresh や auth error 処理を wrapper 側で吸収する
- browser 向け API 呼び出しを transport wrapper 経由に統一する
- 本番レベルの UI ではなくても画面 entry と routing 責務を整理する

## Runtime / Delivery

- `frontend` build を `backend/web/dist/` に出す本番フローを確定する
- 静的 asset の正本を `frontend/public/` に寄せる運用を固定する
- `backend/embed.go` 経由の単一バイナリ配信を smoke test で検証する
- SPA fallback と存在しない asset の 404 を検証する
- `/docs`, `/openapi`, raw spec を認証付きで配信する本番導線を固める
- Dockerfile を追加する
- frontend build と Go build の多段ビルドを追加する
- `backend/web/dist/` の placeholder と build 生成物の扱いを整理する
- 将来的な CDN / object storage 退避の切り出し条件を整理する

## Testing

- `make test` を追加する
- backend の unit test を増やす
- OpenAPI export の契約テストを追加する
- generated client の差分テストを追加する
- migration を適用した実 PostgreSQL に対する sqlc query テストを追加する
- Cookie auth / CSRF / docs auth (`/docs`, `/openapi`) の smoke test を追加する
- SPA fallback / embed 配信の smoke test を追加する
- ローカルと CI で再現しやすい DB テスト実行方式を決める

## CI / Tooling

- backend / frontend / generation の CI を追加する
- `make gen` を CI でも使えるように整える
- OpenAPI artifact と generated client の更新漏れ検知を追加する
- `db/schema.sql` の更新漏れ検知を追加する
- lint / format の最小構成を追加する
- release build の最小パイプラインを追加する

## Observability / Operations

- structured log の形式を決める
- request ID を全リクエストに通す
- Prometheus 向け metrics の導線を追加する
- OpenTelemetry を後付けできる構造にする
- PostgreSQL の重い query を追える運用メモを追加する
- upgrade / backup / restore の最低限の運用方針を整理する

## Documentation

- `README.md` に環境変数一覧を追加する
- API versioning と breaking change ルールを明文化する
- auth / session / CSRF の設計メモを追加する
- OpenAPI artifact 更新ルールを明文化する
- `/docs`, `/openapi`, raw spec の使い分けを明文化する
- ID 戦略の判断基準を文章化する
- browser API と external API の使い分けを明文化する
