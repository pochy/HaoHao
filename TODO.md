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

## 次に進む場所

今の skeleton から着手するなら、まずは次を優先します。

1. browser API と external API の境界、および認証方式の分離を明確にする
2. `make gen` を前提に OpenAPI / client / sqlc の CI 差分検知を入れる
3. auth / session / CSRF の stub を本実装へ置き換える
4. docs / OpenAPI endpoint の認証を stub から実装に切り替える
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

Issue 起票テンプレート:

#### Issue 1-1. browser API と external API の責務を文書で固定する

タイトル:

- `browser API と external API の責務境界を文書化する`

背景:

- 今の skeleton には `backend/internal/api/browser/` と `backend/internal/api/external/` があるが、どの責務をどちらへ置くかはまだ文章で固定されていない
- この状態で auth 実装を進めると Cookie session と bearer token の境界で手戻りが出やすい

作業内容:

- `CONCEPT.md` と現行実装を見て browser API が担う責務を箇条書きにする
- external API が担う責務を箇条書きにする
- 同一 resource を両方に出す場合の path / schema / auth の分離方針を書く
- `README.md` に短い運用説明を追加する
- 必要なら `backend/internal/api/browser/` と `backend/internal/api/external/` に package comment を追加する

完了条件:

- `README.md` または `CONCEPT.md` だけ見れば両 API の責務差分を説明できる
- browser と external を混在させる実装例が消えている

#### Issue 1-2. browser 用 Cookie auth の security scheme を backend と OpenAPI に追加する

タイトル:

- `browser API 向け Cookie auth の security scheme を追加する`

背景:

- browser API は最終的に BFF + Cookie session を使う前提だが、OpenAPI 上ではまだその契約が十分に表現されていない

作業内容:

- Huma の config / operation で browser 用 security scheme を定義する
- Cookie 名と説明を OpenAPI に反映する
- browser 側 endpoint に browser 用 security を付与する
- `make gen` を実行して `openapi/openapi.yaml` を更新する
- frontend generated client の差分が妥当か確認する

完了条件:

- OpenAPI で browser API に Cookie auth が見える
- browser 側 operation が external 用 security scheme と混在していない

#### Issue 1-3. external 用 bearer token の security scheme を backend と OpenAPI に追加する

タイトル:

- `external API 向け bearer token の security scheme を追加する`

背景:

- external client 向け API は browser とは別経路・別認証を前提にしているため、契約上も分離が必要

作業内容:

- Huma の config / operation で bearer token 用 security scheme を定義する
- external API 用 operation にだけ bearer token を付与する
- OpenAPI export を更新して security scheme の差分を確認する
- `README.md` に external API の利用前提を短く追記する

完了条件:

- OpenAPI で bearer token 用 security scheme が確認できる
- browser 用 Cookie auth と external 用 bearer token が operation 単位で分離されている

#### Issue 1-4. external API 用の最小 endpoint を追加して分離を確認する

タイトル:

- `external API 用の最小 endpoint を追加して browser API と分離する`

背景:

- security scheme だけ分けても、実際の endpoint がないと routing / package / OpenAPI の分離を確認しづらい

作業内容:

- `backend/internal/api/external/v1/` に最小 endpoint を 1 本追加する
- `/api/v1/external/...` のような path を決めて実装する
- browser 側 package と依存が混ざらないようにする
- `openapi/openapi.yaml` を再生成して external path を確認する
- `README.md` に確認用 URL を追記する

完了条件:

- external API の path が browser API と別 namespace で公開される
- OpenAPI と実装の両方で external API の存在が確認できる

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

Issue 起票テンプレート:

#### Issue 2-1. `make gen` を実行する CI workflow を追加する

タイトル:

- `CI で make gen を実行する workflow を追加する`

背景:

- この repo は OpenAPI artifact と generated client を commit する前提なので、CI でも同じ生成入口を使う必要がある

作業内容:

- `.github/workflows/` に generation 用 workflow を追加する
- Go / Node / 必要な generator のセットアップを入れる
- `make gen` だけで generation が完結するよう Makefile を見直す
- ローカルと CI で同じコマンドを使えることを確認する

完了条件:

- CI 上で `make gen` が成功する
- README の開発フローと CI の実行内容が一致する

#### Issue 2-2. `make gen` 実行後の差分検知を CI に追加する

タイトル:

- `make gen 実行後の git diff 差分検知を CI に追加する`

背景:

- generator を回しても更新物を commit し忘れると、契約と実装がずれたまま merge される

作業内容:

- `make gen` 実行後に `git diff --exit-code` 相当のチェックを入れる
- 差分があった場合の失敗メッセージを分かりやすくする
- OpenAPI / frontend generated client / sqlc generated code の更新漏れで落ちることを確認する

完了条件:

- 生成物の未 commit が CI で検知される
- 開発者が `make gen` を叩けば CI を再現できる

#### Issue 2-3. OpenAPI artifact の validate / lint を CI に追加する

タイトル:

- `OpenAPI artifact の validate と lint を CI に追加する`

背景:

- OpenAPI 3.1 を唯一の公開契約にするなら、export できるだけでは不十分で、artifact 自体の整合性チェックが必要

作業内容:

- OpenAPI validate / lint に使う最小ツールを選定する
- workflow に validate / lint を追加する
- `openapi/openapi.yaml` が壊れたときに CI が落ちることを確認する
- 必要なら `make openapi-check` のような補助ターゲットを追加する

完了条件:

- OpenAPI artifact が不正な YAML / 不正な契約なら CI が失敗する
- ローカルでも同じ検証を回せる

#### Issue 2-4. `sqlc generate`, `verify`, `vet` を CI に追加する

タイトル:

- `sqlc generate / verify / vet を CI に追加する`

背景:

- DB query と生成コードの整合性は `sqlc` に強く依存するため、CI での標準チェックが必要

作業内容:

- CI で `sqlc generate`, `sqlc verify`, `sqlc vet` を実行する
- `backend/sqlc.yaml` と `db/schema.sql` の参照関係を整理する
- query の壊れ方が CI で検知されることを確認する
- 必要なら `make sqlc-check` を追加する

完了条件:

- query / schema の不整合が CI で検知される
- `make gen` と `sqlc` の検証フローが README と一致する

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

Issue 起票テンプレート:

#### Issue 3-1. Redis を使う session store を実装する

タイトル:

- `Redis ベースの session store を実装する`

背景:

- 現在の session は placeholder で、永続性や失効管理がない
- 最終方針では Redis を session store として使う

作業内容:

- Redis 接続設定を `backend/internal/config/` に追加する
- session store の interface と Redis 実装を `backend/internal/service/` 配下に追加する
- session の保存項目、TTL、失効ルールをコードに落とす
- compose の Redis と接続できることを確認する

完了条件:

- backend が Redis を使って session を保存・取得・削除できる
- session store の置き換え点が service 層に閉じている

#### Issue 3-2. `SESSION_ID` cookie の発行・再発行・削除を実装する

タイトル:

- `SESSION_ID cookie の lifecycle を実装する`

背景:

- browser 認証の本体は Cookie session なので、cookie 属性と再発行の扱いを早めに固定する必要がある

作業内容:

- `SESSION_ID` cookie 名と属性を config に切り出す
- login 完了時の発行、refresh 時の再発行、logout 時の削除を実装する
- `HttpOnly`, `Secure`, `SameSite`, `Path` の設定を環境に応じて制御する
- local 開発で動く設定と本番想定設定の差を README に書く

完了条件:

- browser から session cookie が期待通り付与される
- logout 後に cookie が削除される

#### Issue 3-3. `GET /api/v1/session` を bootstrap endpoint として本実装する

タイトル:

- `GET /api/v1/session を session bootstrap の本実装に置き換える`

背景:

- frontend は初期描画時に session 情報を必要とするが、現在の endpoint は匿名 stub を返すだけ

作業内容:

- Redis session から現在ユーザー情報を引けるようにする
- 未ログイン時とログイン済み時でレスポンス形を整理する
- 必要なら auth mode や権限情報の最小表現を schema に追加する
- generated client を再生成して frontend 側の受け口を合わせる

完了条件:

- frontend 初期化時に本物の session 状態が取得できる
- stub 固有の値に依存した UI が消える

#### Issue 3-4. `XSRF-TOKEN` の発行と CSRF middleware を実装する

タイトル:

- `XSRF-TOKEN 発行と CSRF middleware を実装する`

背景:

- transport wrapper には placeholder があるが、state changing request を守る実処理はまだない

作業内容:

- `XSRF-TOKEN` cookie の発行タイミングを決めて実装する
- `X-CSRF-Token` header との照合 middleware を追加する
- `POST`, `PUT`, `PATCH`, `DELETE` に適用する
- 失敗時のレスポンス形式を problem details に寄せる

完了条件:

- 更新系 request で CSRF token が必須になる
- token 不一致時に想定通り拒否される

#### Issue 3-5. login / logout / session refresh の最小 endpoint を追加する

タイトル:

- `login / logout / session refresh の最小 endpoint を追加する`

背景:

- 本番では Zitadel を使う予定だが、今の skeleton でも session lifecycle を通す API は必要

作業内容:

- browser API に login / logout / session refresh の最小 endpoint を追加する
- 今回は Zitadel 連携を行わず、差し込み点が分かる placeholder に留める
- session store と cookie lifecycle に接続する
- OpenAPI と generated client を更新する

完了条件:

- login / refresh / logout の 3 経路が API として存在する
- 将来の Zitadel 差し込み位置がコード上で明確になる

#### Issue 3-6. frontend の transport wrapper と session store を本実装に合わせて更新する

タイトル:

- `frontend の transport wrapper と session store を auth 本実装に合わせて更新する`

背景:

- backend 側の session / CSRF が本実装になると、frontend 側も bootstrap とエラー処理を合わせる必要がある

作業内容:

- `frontend/src/shared/lib/http/transport.ts` で CSRF header 付与を本実装に置き換える
- auth error / CSRF error の共通処理を整理する
- `frontend/src/features/session/` の store / adapter を session bootstrap 仕様に合わせる
- login / logout / refresh の呼び出し経路を追加する

完了条件:

- frontend から Cookie session と CSRF を前提にした通信ができる
- session bootstrap と auth error 処理が一箇所にまとまる

### 4. docs / OpenAPI endpoint の認証を stub から実装に切り替える

何を決めるか:

- docs / OpenAPI を誰に見せるか決める
- browser の session で見せるか、専用 role / scope で見せるか決める
- 本番と開発で公開条件を分けるか決める

何をやるか:

- `DOCS_BEARER_TOKEN` の暫定ロジックを外す
- docs / openapi へのアクセス制御を実際の auth context に接続する
- docs 閲覧権限の判定を middleware に実装する
- `/docs`, `/openapi.yaml`, `/openapi.json` で共通の認可判定を使う
- unauthorized / forbidden のレスポンスを揃える
- `README.md` に開発時と本番時の docs 公開条件を書く

どこを触るか:

- `backend/internal/middleware/docs_auth.go`
- `backend/internal/app/app.go`
- 必要なら auth context を扱う service / middleware
- `README.md`

完了条件:

- docs / OpenAPI が本番で無条件公開されない
- 開発時の確認方法と本番時の公開条件が文書で揃う

Issue 起票テンプレート:

#### Issue 4-1. docs / OpenAPI 閲覧権限の方針を決めて文書化する

タイトル:

- `docs / OpenAPI の閲覧権限方針を文書化する`

背景:

- docs / OpenAPI は認証付き公開を想定しているが、誰に見せるかの方針がまだ明文化されていない

作業内容:

- 開発環境と本番環境での公開条件を整理する
- browser session で見せるのか、専用 role / scope が必要か決める
- `README.md` または別設計メモに確認方法を書く

完了条件:

- docs / OpenAPI を誰が見られるかを文章で説明できる
- 実装時に迷わない判断基準が残る

#### Issue 4-2. `DOCS_BEARER_TOKEN` の暫定ロジックを廃止する

タイトル:

- `DOCS_BEARER_TOKEN ベースの暫定認証を廃止する`

背景:

- 現状の docs 認証は placeholder であり、最終構成にそのまま残すべきではない

作業内容:

- `backend/internal/middleware/docs_auth.go` の暫定ロジックを削除する
- 実際の auth context へつなぐための依存に置き換える
- 開発用の確認手段が必要なら別の明示的な dev-only 経路にする

完了条件:

- docs 認証の実装が暫定 bearer token に依存しない
- placeholder の痕跡が main flow から外れる

#### Issue 4-3. docs / OpenAPI 用の認可 middleware を auth context ベースで実装する

タイトル:

- `docs / OpenAPI 用の認可 middleware を auth context ベースで実装する`

背景:

- docs と OpenAPI の公開条件は、session / role / scope などの auth context で判定できる必要がある

作業内容:

- auth context を取り出す middleware / helper を整理する
- docs 閲覧権限の判定を middleware に実装する
- `app.go` から `/docs`, `/openapi.yaml`, `/openapi.json` に共通適用する
- unauthorized / forbidden の差を明確にする

完了条件:

- docs / OpenAPI へのアクセス制御が auth context ベースで動く
- path ごとに認可ロジックが分散しない

#### Issue 4-4. docs / OpenAPI の認可結果とレスポンスを揃える

タイトル:

- `/docs /openapi.yaml /openapi.json の認可結果とレスポンスを揃える`

背景:

- 同じ閲覧権限で守るなら、path ごとに挙動がばらつくと運用で混乱する

作業内容:

- 3 つの path で同じ認可判定を使う
- 401 / 403 / 404 をどう返すか方針を決めて実装する
- 開発時の確認手順を README に追記する
- smoke check を追加する

完了条件:

- `/docs`, `/openapi.yaml`, `/openapi.json` で認可挙動が揃う
- README の確認手順どおりに動く

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

Issue 起票テンプレート:

#### Issue 5-1. 最初に通す業務機能を 1 つ決める

タイトル:

- `最初に縦通しする業務機能を 1 つ決める`

背景:

- skeleton の次段階では、抽象設計よりも DB から UI まで貫通する最小機能を 1 本作るほうが有効

作業内容:

- `CONCEPT.md` の範囲で最小に切れる業務機能候補を列挙する
- table / query / API / UI が最小で済む候補を選ぶ
- 選定理由を `TODO.md` か `README.md` に短く残す

完了条件:

- 次に作る機能が 1 つに決まっている
- なぜその機能を最初に選んだか説明できる

#### Issue 5-2. 対象機能の migration と `db/schema.sql` を追加する

タイトル:

- `対象機能の migration と schema.sql を追加する`

背景:

- 縦通し実装の最初の土台は DB スキーマであり、sqlc 生成にも直結する

作業内容:

- `db/migrations/` に新規 migration を追加する
- `db/schema.sql` を更新する
- `bigint` と `uuidv7()` の使い分けを対象集約で決める
- `timestamptz` / UTC 運用を守る

完了条件:

- 対象機能のテーブル構造が migration と schema に反映される
- `sqlc` 生成の前提が揃う

#### Issue 5-3. `db/queries/` と sqlc generated code を追加する

タイトル:

- `対象機能の query と sqlc generated code を追加する`

背景:

- repository 層の基盤は handwritten SQL + sqlc の生成コードで揃える前提

作業内容:

- `db/queries/` に対象機能の CRUD または必要最小 query を追加する
- `make gen` で `backend/internal/db/` を再生成する
- generated code の API が service 層で使いやすいか確認する
- 必要なら query 名や戻り値を見直す

完了条件:

- 対象機能に必要な query が sqlc generated code として使える
- query 変更が `make gen` に集約される

#### Issue 5-4. service と Huma operation を追加する

タイトル:

- `対象機能の service と Huma operation を追加する`

背景:

- DB だけ増やしても frontend には届かないため、service と API を最小で接続する必要がある

作業内容:

- `backend/internal/service/` に対象機能の service を追加する
- generated db code を service から呼ぶ
- `backend/internal/api/browser/` に Huma operation を追加する
- error を problem details へ寄せる
- OpenAPI export を更新する

完了条件:

- DB -> service -> browser API が一通りつながる
- OpenAPI から対象機能の endpoint が見える

#### Issue 5-5. frontend feature と page を追加する

タイトル:

- `対象機能の frontend feature と page を追加する`

背景:

- generated client を実際に使う経路を作らないと、frontend 側の最小構成が検証できない

作業内容:

- `frontend/src/features/` に対象機能の API adapter / store / UI を追加する
- generated client を transport wrapper 経由で呼ぶ
- `frontend/src/pages/` に表示入口を追加する
- 画面から対象機能の取得または更新ができるようにする

完了条件:

- UI から対象機能の API を呼べる
- feature / pages 構成の分離が守られている

#### Issue 5-6. 最低限の test と smoke check を追加する

タイトル:

- `対象機能の最低限の test と smoke check を追加する`

背景:

- 最初の縦通し機能では、以後の実装テンプレートになる最小テストもセットで作っておく価値がある

作業内容:

- backend 側で service または handler の最小 test を追加する
- query が壊れていないことを確認するチェックを入れる
- frontend 側で最低限の表示確認または store test を追加する
- README か TODO に手動 smoke check 手順を残す

完了条件:

- 対象機能に最低限の自動確認が付く
- 次の機能実装時に流用できる検証パターンができる

## High Priority

- Zitadel を使う前提で browser 向け認証フローの境界を決める
- browser API と external client API の責務と認証方式を分離する
- Redis を使った session store を実装する
- CSRF token の発行・検証・更新を placeholder から本実装に置き換える
- docs / OpenAPI endpoint の認証を stub から実装に切り替える
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
- docs / OpenAPI endpoint の公開条件と配布ルートを整理する
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
- docs / OpenAPI を認証付きで配信する本番導線を固める
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
- Cookie auth / CSRF / docs auth の smoke test を追加する
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
- ID 戦略の判断基準を文章化する
- browser API と external API の使い分けを明文化する
