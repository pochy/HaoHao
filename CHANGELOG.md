# Changelog

このファイルは、repo の主要な構成変更を簡潔に追うための記録です。

## Unreleased

### Added

- monorepo の最小骨格を追加
- root に `go.work`, `Makefile`, `compose.yaml`, `README.md` を追加
- backend に Go + Gin + Huma の最小構成を追加
- browser API として `GET /api/v1/health`, `GET /api/v1/session` を追加
- OpenAPI 3.1 export コマンドを追加
- `openapi/openapi.yaml` を追加して artifact commit 前提を整備
- docs / OpenAPI 公開の認可差し込み点として stub middleware を追加
- frontend に Vue 3 + Vite + TypeScript + Pinia の最小構成を追加
- generated client の生成設定と生成済み artifact を追加
- transport wrapper を追加
- `credentials: 'include'` の既定化と CSRF header 付与 placeholder を追加
- `shared / features / pages` 構成の最小例を追加
- db に migration, schema snapshot, queries を追加
- `sqlc` 設定と生成済み Go コードを追加
- PostgreSQL / Redis の compose 起動定義を追加
- backend の最小テストを追加
- GitHub Issues の起票基盤として `TODO.md` のテンプレートから `#1`-`#24` を作成
- Issue 管理用の label を追加（`priority:*`, `area:*`, `track:phase-*`, `dependencies`）
- 5 フェーズ管理の milestone を追加（`M1`-`M5`）
- リリース軸の milestone を追加（`v0.1 Foundation`, `v0.2 Auth`, `v0.3 First Feature`）
- GitHub Project `HaoHao Roadmap TODO 1-5` を作成し、`#1`-`#24` を追加
- Project に `Priority`, `Area`, `Risk`, `Target Release` の custom field を追加
- Issue 作成品質統一のため `.github/ISSUE_TEMPLATE/` に Issue Forms を追加
- PR 品質統一のため `.github/pull_request_template.md` を追加
- レビュー責務明確化のため `.github/CODEOWNERS` を追加
- 脆弱性報告方針として `.github/SECURITY.md` を追加
- 依存更新自動化のため `.github/dependabot.yml` を追加
- セキュリティ解析のため `.github/workflows/codeql.yml` を追加
- `main` ブランチ保護を有効化（PR 必須、Code Owner review 必須、会話解決必須、force push 禁止）
- GitHub Discussions を有効化
- Dependabot security updates を有効化
- browser / external API の境界ルールを `docs/api-surface-boundary.md` と package comment で明文化
- browser API の `cookieAuth` 契約と external API の `bearerAuth` 契約を OpenAPI に追加
- external API の最小 endpoint として `GET /external/v1/health` を追加し、generated client / artifact に反映
- Zitadel 導入フェーズ、BFF 経由 OIDC 方針、ローカル起動順の設計メモを `CONCEPT.md` / `TODO.md` に追記
- `go.work.sum` を追加
- Issue 着手ルールとして `.cursor/rules/issue-kickoff-workflow.mdc`、Codex 用設定として `.codex/config.toml` と root の `AGENTS.md` を追加
- 生成物ガード用の GitHub Actions workflow として `.github/workflows/generated-artifacts.yml` を追加
- `sqlc vet` 用の CI 設定 `backend/sqlc.ci.yaml` を追加
- migration と `db/schema.sql` の更新漏れを検知する `scripts/check-schema-snapshot.sh` を追加
- OpenAPI lint 用に `@redocly/cli` と `frontend` の `lint:openapi` スクリプトを追加

### Changed

- OpenAPI docs の配信面に関する記述を `/docs`, `/openapi`, `/openapi.json`, `/openapi.yaml` 前提で整理
- CodeQL workflow を `workflow_dispatch`, `concurrency`, timeout, Node.js setup, frontend `npm ci/build` を含む構成に更新
- `actions/setup-go` を v6 に更新
- `Makefile` に `check-generated`, `openapi-lint`, `sqlc-vet` を追加し、生成物と契約の検証入口を統一
- `README.md` と `CONCEPT.md` に、`sqlc Cloud` は使わずローカル / GitHub Actions のみで artifact drift を検知する方針を追記
- `compose.yaml` の PostgreSQL volume mount を PostgreSQL 18 向けの `/var/lib/postgresql` に変更

### Removed

- 旧 PR metadata 自動化 workflow `.github/workflows/pr-auto-metadata.yml` を削除

### Notes

- 認証は stub であり、Zitadel 本接続は未実装
- session は bootstrap 用の最小応答のみで、Redis 連携は未実装
- external client 向け API は最小の health endpoint のみ実装済みで、本体機能は未実装
- frontend の UI は本番品質ではなく接続確認用
- Project の saved views は GitHub UI での手動作成が必要（CLI からは作成不可）
- `sqlc Cloud` と `SQLC_AUTH_TOKEN` は現時点の標準フローでは使わず、`make check-generated` / `make openapi-lint` / `make sqlc-vet` を基準に運用する
