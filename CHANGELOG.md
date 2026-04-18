# Changelog

このファイルは repo の主要な変更を時系列で追うための記録です。
形式は `Keep a Changelog` に寄せ、バージョン運用が始まるまでは日付単位でまとめます。

## [Unreleased]

### Changed

- changelog を過去の git 履歴ベースで再構成し、日付ごとに整理

## [2026-04-18]

### Added

- monorepo の最小骨格として `backend`, `frontend`, `openapi`, `db`, `compose.yaml`, `Makefile`, `README.md`, `TODO.md`, `CHANGELOG.md` を追加
- backend の最小構成として Gin + Huma ベースの server / OpenAPI export / docs auth stub / embed 配信を追加
- browser API の最小構成として `GET /api/v1/health`, `GET /api/v1/session` と対応する generated client / `sqlc` 生成物を追加
- frontend の最小構成として Vue 3 + Vite + TypeScript + Pinia、`shared / features / pages`、transport wrapper を追加
- PostgreSQL / Redis の compose 起動定義、`db/migrations`, `db/schema.sql`, `db/queries`, `backend/sqlc.yaml` を追加
- GitHub governance / security 基盤として Issue Forms, PR template, CODEOWNERS, SECURITY.md, Dependabot, CodeQL を追加
- browser / external API の境界ルールを `docs/api-surface-boundary.md` と package comment で明文化
- browser API の `cookieAuth` 契約と external API の `bearerAuth` 契約を OpenAPI に追加
- external API の最小 endpoint として `GET /external/v1/health` を追加
- Zitadel 導入フェーズ、BFF 経由 OIDC 方針、ローカル起動順に関する設計メモを `CONCEPT.md` / `TODO.md` に追加
- `go.work.sum`, `.cursor/rules/issue-kickoff-workflow.mdc`, `.codex/config.toml`, root の `AGENTS.md` を追加
- generated artifact guard 用の GitHub Actions workflow、`sqlc vet` 用 CI 設定、schema snapshot guard script を追加
- OpenAPI lint 用に `@redocly/cli` と `frontend` の `lint:openapi` スクリプトを追加

### Changed

- `CONCEPT.md` にアーキテクチャ図、request flow、開発フロー図を追加
- OpenAPI docs の配信面に関する記述を `/docs`, `/openapi`, `/openapi.json`, `/openapi.yaml` 前提で整理
- CodeQL workflow を `workflow_dispatch`, `concurrency`, timeout, Node.js setup, frontend `npm ci/build` を含む構成に更新
- GitHub project 運用ドキュメントと TODO の issue 参照を整理
- `actions/setup-go` を v6 に更新
- `actions/checkout` を v6 に更新
- `github/codeql-action` を v4 に更新
- `github.com/jackc/pgx/v5` を `v5.9.1` に更新
- `vite` を `8.0.8` に更新
- `Makefile` に `check-generated`, `openapi-lint`, `sqlc-vet` を追加し、生成物と契約の検証入口を統一
- `Makefile` と GitHub Actions を `make sqlc-load-schema` / `make sqlc-check` に揃え、`sqlc generate`, `sqlc compile`, `sqlc vet` を local / GitHub Actions-only で再現できるようにした
- `README.md` と `CONCEPT.md` に、`sqlc Cloud` を使わずローカル / GitHub Actions のみで artifact drift を検知する方針を追記
- `compose.yaml` の PostgreSQL volume mount を PostgreSQL 18 向けの `/var/lib/postgresql` に変更

### Removed

- 旧 PR metadata 自動化 workflow `.github/workflows/pr-auto-metadata.yml` を削除

### Security

- 認証付き開発を見据えた CodeQL / governance 基盤を追加し、browser / external API の認証境界を OpenAPI 契約で分離

### Notes

- 認証は stub のままで、Zitadel 本接続は未実装
- session は bootstrap 用の最小応答のみで、Redis 連携は未実装
- external client 向け API は最小の health endpoint のみ実装済みで、本体機能は未実装
- frontend の UI は本番品質ではなく接続確認用
- `sqlc Cloud` と `SQLC_AUTH_TOKEN` は標準フローでは使わず、`make check-generated` / `make openapi-lint` / `make sqlc-load-schema` / `make sqlc-check` を基準に運用する

## [2026-04-17]

### Added

- システム全体の設計方針をまとめた `CONCEPT.md` を追加
