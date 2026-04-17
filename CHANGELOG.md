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

### Notes

- 認証は stub であり、Zitadel 本接続は未実装
- session は bootstrap 用の最小応答のみで、Redis 連携は未実装
- external client 向け API は予約のみ
- frontend の UI は本番品質ではなく接続確認用

