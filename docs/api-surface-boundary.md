# API Surface Boundary

Issue `#1` の合意内容として、browser API と external API の責務境界を固定する。

## Scope

- 対象: `backend/internal/api/browser/` と `backend/internal/api/external/` の責務定義
- 対象: 同一 resource を両 API surface に公開する場合の分離方針
- 非対象: 認証機能の本実装、endpoint 追加、OpenAPI の大規模再設計

## Responsibility Split

### Browser API (`backend/internal/api/browser/`)

- 同一オリジンの画面向け BFF として扱う
- 認証は Cookie session を前提にする
- state-changing request は CSRF 対策を前提にする
- 画面 UX に合わせたレスポンス最適化を許容する

### External API (`backend/internal/api/external/`)

- 非ブラウザ client 向け API surface として扱う
- 認証は bearer token 系を前提に設計する
- browser API と認証方式や middleware 前提を共有しない
- SDK/連携利用を前提に、外部公開しやすい契約を優先する

## Boundary Rules

### Path Separation

- browser と external は同一 path 空間を共有しない
- 同一 resource 名を扱っても、endpoint は API surface ごとに分離する

### Schema Separation

- 同一 resource でも request/response schema は API surface ごとに分離する
- browser の DTO と external の DTO は同名にしないか、名前で用途を明示する
- 一方の変更を他方へ自動伝播しない

### Auth Separation

- browser: Cookie session + CSRF header
- external: bearer token (or OAuth2)
- OpenAPI security scheme は browser/external で分離定義する

## Decision Checklist

新しい endpoint を追加する前に、次を順に確認する。

1. この endpoint の一次利用者は browser か external client か
2. 必要な認証方式は Cookie session か bearer token か
3. path は既存の別 surface と混在していないか
4. schema は別 surface と独立して変更可能か
5. 同一 resource の重複公開が必要なら、意図的に分離設計したか

## Do / Don't

Do:

- API surface を先に決めてから handler と schema を配置する
- 同一 resource を両方で出すときは path/schema/auth を全て分離する
- browser 側実装では BFF 視点の payload 最適化を明示する

Don't:

- browser 用 endpoint を external 用 path に追加しない
- Cookie session と bearer token の前提を同一 handler で混在させない
- browser/external で同一 DTO を安易に共用しない
