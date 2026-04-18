# TODO

このファイルは、現状の foundation から次に進める作業を、完了済みと未着手に分けて整理するためのメモです。

## 現在地

- GitHub 運用整備は完了
- Phase 1 の API 境界整理と認証方式分離は完了
- Phase 2 の generated artifact / OpenAPI / sqlc の CI ガードは完了
- `v0.1 Foundation` は完了
- 次の主対象は Phase 3 の auth / session / CSRF 本実装

生成物の扱いは固定する。

- 正本:
  - API 契約は backend の Huma operation / request / response
  - DB 契約は `db/migrations/`, `db/schema.sql`, `db/queries/`
- 生成物:
  - `openapi/openapi.yaml`
  - `frontend/src/api/generated/`
  - `backend/internal/db/`
- generated file は直接編集せず、正本を更新して `make gen` を実行する

## #9 開始前提（固定済み）

`v0.2 Auth` の開始条件として、local Zitadel 前提を次で固定した。

- 接続先: `local`
- 起動方式: standalone の `compose.auth.yaml`
- canonical 文書: `docs/auth-local-zitadel.md`
- seed 方式: repo-managed script `scripts/zitadel/seed-local.sh`
- issuer URL: `http://localhost:8081`
- OIDC application: `haohao-browser-local`
- test user: `haohao.dev@zitadel.localhost`
- redirect URI: `http://localhost:8080/auth/callback`
- post-logout redirect URI: `http://localhost:8080/auth/logout/callback`
- backend auth env: `.env.auth`
- session contract:
  - lookup unit: `SESSION_ID`
  - Redis key: `haohao:session:{session_id}`
  - payload: `user_id`, `zitadel_subject`, `roles`, `created_at`, `expires_at`, `csrf_secret`
  - TTL: absolute `8h`
  - idle timeout: なし
  - logout: server-side invalidation で `delete(session_id)`

`#9` 着手前の受け入れ基準は次で固定した。

- `make compose-auth-up` で local Zitadel が `http://localhost:8081` に上がる
- `make compose-auth-seed` で `haohao-browser-local` と test user が再現でき、`.env.auth` が生成される
- `curl http://localhost:8081/.well-known/openid-configuration` が成功する
- Redis session store の責務が `backend/internal/service/` に閉じている
- `#10`-`#12` に残す責務と `#9` に入れる責務が混ざっていない

## 完了済み

### GitHub 運用整備

- `Issue 1-1` から `Issue 5-6` までを GitHub Issue `#1`-`#24` として起票済み
- label を運用用に整理済み（`priority:*`, `area:*`, `track:phase-*`, `dependencies`）
- milestone を整備済み
  - フェーズ管理: `M1`-`M5`
  - リリース管理: `v0.1 Foundation`, `v0.2 Auth`, `v0.3 First Feature`
- GitHub Project `HaoHao Roadmap TODO 1-5` を作成し、`#1`-`#24` を追加済み
- Project custom field `Priority`, `Area`, `Risk`, `Target Release` を作成済み
- `.github/ISSUE_TEMPLATE/`, `.github/pull_request_template.md`, `.github/CODEOWNERS`, `.github/SECURITY.md` を追加済み
- `main` の branch protection を設定済み（PR 必須、Code Owner review 必須、force push 禁止、`ci-codeql` 必須）

### Phase 1. browser API / external API の境界整理（完了）

- browser 向け API と external client 向け API の責務境界を文書化済み
- browser 側は Cookie auth、external 側は bearer token を OpenAPI 上で分離済み
- external API の最小 endpoint として `GET /external/v1/health` を追加済み

対応 Issue:

- [#1 browser API と external API の責務境界を文書化する](https://github.com/pochy/HaoHao/issues/1)
- [#2 browser API 向け Cookie auth の security scheme を追加する](https://github.com/pochy/HaoHao/issues/2)
- [#3 external API 向け bearer token の security scheme を追加する](https://github.com/pochy/HaoHao/issues/3)
- [#4 external API 用の最小 endpoint を追加して browser API と分離する](https://github.com/pochy/HaoHao/issues/4)

### Phase 2. generated artifact / 契約ドリフトの CI ガード（完了）

- `make gen` を共通の生成入口として固定済み
- CI で `make check-generated` を実行し、OpenAPI / frontend generated client / sqlc generated code の差分検知を追加済み
- OpenAPI lint / validate を CI に追加済み
- `db/migrations/` 変更時の `db/schema.sql` 更新漏れ検知を追加済み
- `sqlc generate`, `sqlc compile --no-remote`, `sqlc vet --no-remote` を `make sqlc-check` に統合済み
- `sqlc verify` は project ID を要求するため、local / GitHub Actions-only の標準フローには含めない方針を固定済み

対応 Issue:

- [#5 CI で make gen を実行する workflow を追加する](https://github.com/pochy/HaoHao/issues/5)
- [#6 make gen 実行後の git diff 差分検知を CI に追加する](https://github.com/pochy/HaoHao/issues/6)
- [#7 OpenAPI artifact の validate と lint を CI に追加する](https://github.com/pochy/HaoHao/issues/7)
- [#8 sqlc generate / compile / vet を CI に追加する](https://github.com/pochy/HaoHao/issues/8)

## 事前に固定しておく前提

### OpenAPI docs 導線

この repo で docs 配信面として扱う path は、次の 4 本で固定する。

- `/docs`: Huma built-in docs route
- `/openapi`: OpenAPI Documents 用の renderer route
- `/openapi.json`: raw JSON
- `/openapi.yaml`: raw YAML

この 4 本について、Phase 4 の実装に入る前に次を固定する。

- `/docs`, `/openapi`, `/openapi.json`, `/openapi.yaml` には同じ auth policy を適用する
- `/openapi` の既定 renderer は `Stoplight Elements` とする
- `Scalar`, `Swagger UI`, `none` は config で切り替えられる構成にする
- local verification と smoke test の対象に `/openapi` を含める

### リリース milestone の定義

`v0.1 Foundation`

- 範囲: `M1` と `M2`
- 中身: stub auth のまま、browser / external API の境界、OpenAPI artifact、generated client、sqlc 生成導線、frontend 接続、CI のドリフト検知を固める
- 完了条件: backend / frontend / PostgreSQL / Redis が Zitadel なしで起動できる。browser / external API の境界が固定されている。`make gen`, `make check-generated`, `make openapi-lint`, `make sqlc-check` が local と GitHub Actions で再現できる
- 次へ進む条件: Zitadel 接続先が `local` か `shared dev` か決まっている。issuer URL、OIDC application、test user、redirect URI、logout URI、必要な env var が文書化されている
  この前提は `local` / `compose.auth.yaml` / `docs/auth-local-zitadel.md` / `.env.auth` で固定済み

`v0.2 Auth`

- 範囲: `M3` と `M4`
- 中身: Zitadel を接続し、browser auth / session / CSRF と docs / OpenAPI の閲覧制御を stub から実装へ置き換える
- 対象 Issue: `#9`-`#18`
- 完了条件: browser が BFF 経由で Zitadel Hosted Login に遷移し、callback 後に Redis-backed session を持てる。`GET /api/v1/session` が実認証状態を返す。state-changing request で CSRF 検証が有効。`/docs`, `/openapi`, `/openapi.json`, `/openapi.yaml` が同じ non-stub auth policy で保護される
- 次へ進む条件: auth / session / CSRF / docs auth の前提作業が残っていない。以後の機能開発で auth stub を前提にしなくてよい状態になっている

`v0.3 First Feature`

- 範囲: `M5`
- 中身: 最初の業務機能を 1 つ選び、DB -> service -> API -> generated client -> UI まで縦通しする
- 対象 Issue: `#19`-`#24`
- 完了条件: 対象機能の migration、`db/schema.sql`、query、sqlc generated code、service、Huma operation、frontend feature、page、最低限の test / smoke check がそろい、auth ありの通常フローで動く
- 次へ進む条件: `v0.3` 完了時点で、次の release milestone を別途定義する。現時点では `v0.4` 以降は未定義とする

### auth 実装前に決めること

以下は「いつか決める」ではなく、対応する Issue の Plan を開始する前に決めて文書へ固定する。`#9` については local Zitadel 前提で固定済み。

#### `#9` の Plan を始める前に固定する事項

- Zitadel 接続先を `local` か `shared dev` のどちらにするか
- issuer URL
- browser login 用 OIDC application の client ID / secret
- redirect URI
- post-logout redirect URI
- project / organization / app role / scope のどれを使うか
- test user の準備方法
- backend に渡す env var 名

この段階で必要な手順:

- `local` を選ぶなら、auth 用 compose profile か `compose.auth.yaml` を作る方針を決める
- `shared dev` を選ぶなら、誰が application / user / secret を管理するか決める
- backend から issuer discovery endpoint に到達できることを確認する
- login 用 application が Authorization Code Flow と logout redirect を扱えることを確認する

#### `#9` の実装を始める前に固定する事項

- Redis に保存する session payload
- session key の命名方針
- `SESSION_ID` cookie に紐づく session lookup の単位
- login 後に session に保存する最小情報
- logout 時に Redis から何を削除するか

session payload は少なくとも次のどれを保存するかを明示してから実装する。

- internal user ID
- Zitadel subject (`sub`)
- session 作成時刻
- session 有効期限
- CSRF token との関連付けに使う値
- docs auth や role 判定に必要な最小 claim

#### `#10` と `#11` の実装を始める前に固定する事項

- session TTL
- idle timeout を使うか
- session refresh を行う条件
- refresh 時に `SESSION_ID` を再発行するか
- logout を server-side invalidation にするか

この段階で決まっていない場合は、`SESSION_ID` cookie lifecycle を実装してはいけない。

#### `#12` の実装を始める前に固定する事項

- CSRF 対策を `cookie-to-header` にすること
- `XSRF-TOKEN` をいつ発行するか
- `XSRF-TOKEN` をいつ rotate するか
- どの HTTP method を CSRF 対象にするか
- token mismatch / missing 時のレスポンス仕様

#### `#15` の実装を始める前に固定する事項

- docs / OpenAPI を誰に見せるか
- auth context のどの claim / role / scope を閲覧権限判定に使うか
- `/docs`, `/openapi`, `/openapi.json`, `/openapi.yaml` を完全に同一 policy にすること

### Zitadel 導入方式の選び方

第一候補は `local` で、第二候補は `shared dev` とする。理由は、local の方が再現性と単独開発時の進行速度を確保しやすいため。

`local` を選ぶ条件:

- auth 実装を他人の環境準備に依存せず進めたい
- OIDC application、test user、role を自分で閉じて管理したい
- compose profile か別 compose file を repo に足してもよい

`shared dev` を選ぶ条件:

- すでに安定運用中の tenant があり、issuer と app 情報がすぐ使える
- test user / role / secret の払い出し手順が既にある
- 共有環境障害で local 開発が止まるリスクを許容できる

### skeleton 開発と auth 開発の境界

- `v0.1 Foundation` では `compose.yaml` は PostgreSQL と Redis だけを起動する
- Zitadel は foundation の必須依存にしない
- `v0.2 Auth` に入ったら、local か shared dev のいずれか 1 つに接続先を固定する
- browser login は BFF 経由の OIDC Authorization Code Flow を使い、browser に access token / refresh token を直接保持させない

## 推奨着手順

1. auth 実装前提を固定し、Zitadel 接続先と env var を文書化する
2. Phase 3: auth / session / CSRF の stub を本実装へ置き換える
3. Phase 4: `/docs`, `/openapi`, raw spec の導線と認証を stub から実装に切り替える
4. Phase 5: DB query と service を増やして業務機能を 1 本通す

## Phase 3. auth / session / CSRF の stub を本実装へ置き換える

対象範囲:

- `backend/internal/service/`
- `backend/internal/middleware/`
- `backend/internal/api/browser/`
- `backend/internal/config/`
- auth 用 compose 定義か補助ドキュメント
- `frontend/src/shared/lib/http/transport.ts`
- `frontend/src/features/session/`

非対象範囲:

- external client API の本体機能追加
- docs/OpenAPI の認可本実装
- browser に token を直接保持させる実装

固定する境界:

- browser 向け認証は Cookie session
- external client 向け認証は bearer token
- session store は Redis
- state changing request は CSRF 対象
- 認可や session lifecycle の判断は operation ではなく service / middleware 側に寄せる

主な作業:

- auth 開発用の Zitadel 接続手順を文書化する
- issuer URL、client ID、client secret、redirect URI などを config に通す
- login start / callback / logout callback を追加する
- Zitadel の `sub` と `db.app_users.zitadel_subject` を対応付ける
- `SESSION_ID` cookie の発行・再発行・削除を実装する
- `XSRF-TOKEN` cookie の発行と `X-CSRF-Token` header の検証を実装する
- `GET /api/v1/session` を bootstrap endpoint として整備する
- frontend の transport wrapper と session store を auth 本実装に合わせて更新する

完了条件:

- browser から Cookie session で認証状態を維持できる
- ローカルで Zitadel を使った login から session 発行まで確認できる
- `POST`, `PUT`, `PATCH`, `DELETE` で CSRF 検証が通る
- logout で session と cookie が失効する

対応 Issue:

- [#9 Redis ベースの session store を実装する](https://github.com/pochy/HaoHao/issues/9)
- [#10 SESSION_ID cookie の lifecycle を実装する](https://github.com/pochy/HaoHao/issues/10)
- [#11 GET /api/v1/session を session bootstrap の本実装に置き換える](https://github.com/pochy/HaoHao/issues/11)
- [#12 XSRF-TOKEN 発行と CSRF middleware を実装する](https://github.com/pochy/HaoHao/issues/12)
- [#13 login / logout / session refresh の最小 endpoint を追加する](https://github.com/pochy/HaoHao/issues/13)
- [#14 frontend の transport wrapper と session store を auth 本実装に合わせて更新する](https://github.com/pochy/HaoHao/issues/14)

## Phase 4. docs / OpenAPI / raw spec の導線と認証を stub から実装に切り替える

対象範囲:

- `backend/internal/middleware/docs_auth.go`
- `backend/internal/app/app.go`
- 必要なら auth context を扱う service / middleware
- `README.md`

非対象範囲:

- API 契約そのものの大幅な再設計
- frontend 業務画面の追加

固定する境界:

- `/docs`, `/openapi`, `/openapi.yaml`, `/openapi.json` は共通の認可判定を使う
- docs/OpenAPI は本番で無条件公開しない
- renderer 切り替えと認可判定は別責務に分ける

主な作業:

- `DOCS_BEARER_TOKEN` の暫定ロジックを外す
- `/openapi` route を追加し、renderer を切り替えられるようにする
- docs / openapi へのアクセス制御を auth context に接続する
- unauthorized / forbidden のレスポンスを揃える
- `README.md` に開発時と本番時の docs 公開条件を書く

完了条件:

- `/docs`, `/openapi`, raw spec が本番で無条件公開されない
- `/openapi` で既定 renderer の docs UI が見える
- 開発時の確認方法と本番時の公開条件が文書で揃う

対応 Issue:

- [#15 docs / OpenAPI の閲覧権限方針を文書化する](https://github.com/pochy/HaoHao/issues/15)
- [#16 DOCS_BEARER_TOKEN ベースの暫定認証を廃止する](https://github.com/pochy/HaoHao/issues/16)
- [#17 docs / OpenAPI 用の認可 middleware を auth context ベースで実装する](https://github.com/pochy/HaoHao/issues/17)
- [#18 /docs /openapi.yaml /openapi.json の認可結果とレスポンスを揃える](https://github.com/pochy/HaoHao/issues/18)

## Phase 5. DB query と service を増やして業務機能を 1 本通す

対象範囲:

- `db/migrations/`
- `db/schema.sql`
- `db/queries/`
- `backend/internal/db/`
- `backend/internal/service/`
- `backend/internal/api/browser/`
- `frontend/src/features/`
- `frontend/src/pages/`

非対象範囲:

- 複数機能を同時に通す拡張
- external API の本格展開

固定する境界:

- まずは 1 機能だけを DB -> service -> API -> generated client -> UI まで縦通しする
- schema / query / Huma operation を変えたら `make gen` を実行する
- generated client の直接利用は feature 経由に寄せる

主な作業:

- 最初に通す業務機能を 1 つ決める
- migration と `db/schema.sql` を更新する
- `db/queries/` と sqlc generated code を追加する
- service と Huma operation を追加する
- frontend feature と page を追加する
- 最低限の test と smoke check を追加する

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

## 横断 backlog

### Backend / Platform

- backend に config validation を追加する
- `service` 層と `repository` 層の責務分離を広げる
- domain error と problem details のマッピング方針を実装する
- request logging と request ID の導線を整える
- `pprof` とアプリケーションメトリクスの差し込み点を追加する
- static asset 配信と API 配信の責務境界を整理する
- `pgxpool` の初期化と lifecycle を追加する

### Frontend

- 画面 routing を導入する
- generated client のエラーハンドリングを transport wrapper に寄せる
- problem details を UI 用エラー表現へ変換する共通層を作る
- feature 単位で API adapter / store / UI の分離を進める
- browser 向け API 呼び出しを transport wrapper 経由に統一する

### Contract / Database

- migration の運用ルールを決める
- migration から `db/schema.sql` を再生成する流れを整える
- `uuidv7()` 利用テーブルと `bigint` 利用テーブルの基準を明文化する
- PostgreSQL 18 前提の初期 extension / settings を必要に応じて整理する
- `timestamptz` + UTC 運用をコードとテーブル設計に固定する
- 必要箇所で Row-Level Security の適用可否を検討する
- GitHub Release / release asset での固定版配布を整備する

### Runtime / Delivery / Testing

- `make test` を追加する
- backend の unit test を増やす
- OpenAPI export の契約テストを追加する
- migration を適用した実 PostgreSQL に対する sqlc query テストを追加する
- Cookie auth / CSRF / docs auth (`/docs`, `/openapi`) の smoke test を追加する
- SPA fallback / embed 配信の smoke test を追加する
- ローカルと CI で再現しやすい DB テスト実行方式を決める
- Dockerfile を追加する
- frontend build と Go build の多段ビルドを追加する
- `backend/web/dist/` の placeholder と build 生成物の扱いを整理する
- 将来的な CDN / object storage 退避の切り出し条件を整理する
- release build の最小パイプラインを追加する

### Documentation / Operations

- ローカル開発環境のセットアップ手順を foundation 用と auth 用で分けて書く
- Zitadel 導入手順と必要な env var を追加する
- `README.md` に環境変数一覧を追加する
- API versioning と breaking change ルールを明文化する
- auth / session / CSRF の設計メモを追加する
- OpenAPI artifact 更新ルールを明文化する
- `/docs`, `/openapi`, raw spec の使い分けを明文化する
- ID 戦略の判断基準を文章化する
- browser API と external API の使い分けを明文化する
- structured log の形式を決める
- OpenTelemetry を後付けできる構造にする
- PostgreSQL の重い query を追える運用メモを追加する
- upgrade / backup / restore の最低限の運用方針を整理する
