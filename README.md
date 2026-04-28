# HaoHao

OpenAPI 3.1 優先 + Monorepo + 単一バイナリ配信を基本方針とした、Go/Huma + Vue + PostgreSQL/sqlc + Redis 構成のアプリケーションです。

## 主要ドキュメント

- [設計方針](CONCEPT.md)
- [実装状況](IMPL.md)
- [調査レポート](deep-research-report.md)

## チュートリアル手順

1. [基礎](TUTORIAL.md)
2. [Zitadel認証](TUTORIAL_ZITADEL.md)
3. [単一バイナリ配信](TUTORIAL_SINGLE_BINARY.md)
4. [運用可能性](TUTORIAL_P0_OPERABILITY.md)
5. [管理 UI](TUTORIAL_P1_ADMIN_UI.md)
6. [シンプルTODO機能](TUTORIAL_P2_TODO.md)
7. [監査ログ](TUTORIAL_P3_AUDIT_LOG.md)
8. [可観測性](TUTORIAL_P4_OBSERVABILITY.md)
9. [テナント管理 UI](TUTORIAL_P5_TENANT_ADMIN_UI.md)
10. [ドメイン拡張](TUTORIAL_P6_DOMAIN_EXPANSION.md)
11. [Webサービス共通機能](TUTORIAL_P7_WEB_SERVICE_COMMON.md)
12. [OpenAPI 分割](TUTORIAL_P8_OPENAPI_SPLIT.md)
13. [UI Playwright E2E](TUTORIAL_P9_UI_PLAYWRIGHT_E2E.md)
14. [横断拡張](TUTORIAL_P10_CROSS_CUTTING_EXTENSIONS.md)
15. [テナント単位レート制限（ランタイム反映）](TUTORIAL_P11_TENANT_RATE_LIMIT_RUNTIME.md)
16. [ファイルライフサイクル物理削除](TUTORIAL_P12_FILE_LIFECYCLE_PHYSICAL_DELETE.md)

## クイックスタート

- 必要環境: Go 1.26.0 / Node.js 22 / Docker / GNU Make / sqlc / golang-migrate / Air
- 初回のみ Air をインストール: `go install github.com/air-verse/air@latest`
- 依存サービスを起動: `make up`
- マイグレーションを適用: `make db-up`
- 生成物を更新（sqlc + OpenAPI + frontend SDK）: `make gen`
- バックエンドをホットリロード起動: `make backend-dev`
- フロントエンドを起動: `make frontend-dev`

詳細な手順は [TUTORIAL.md](TUTORIAL.md) を参照してください。

## バックエンド開発サーバー

`make backend-dev` は `.env` を読み込んだうえで Air を起動し、`backend` 配下の Go ソース変更時にバックエンドを自動で再ビルド・再起動します。Air の監視設定は `.air.toml` にあります。

ホットリロードを使わず従来どおり起動したい場合は、次のどちらかを使ってください。

```bash
make backend-run
go run ./backend/cmd/main
```

`air` コマンドが見つからない場合は、次を実行してください。

```bash
go install github.com/air-verse/air@latest
export PATH=$PATH:$(go env GOPATH)/bin
```

## Runbook

- [運用Runbook](RUNBOOK_OPERABILITY.md)
- [可観測性Runbook](RUNBOOK_OBSERVABILITY.md)
- [デプロイRunbook](RUNBOOK_DEPLOYMENT.md)
- [Drive 商品情報抽出 Python / GiNZA / SudachiPy Runbook](docs/RUNBOOK_DRIVE_PRODUCT_EXTRACTION_NLP.md)

## OpenFGA

### 仕様・計画

- [ファイル共有仕様](DRIVE_OPENFGA_PERMISSIONS_SPEC.md)
- [OpenFGA 実装計画](OPENFGA_IMPLEMENTATION_PLAN.md)

### 手順

1. [概要](TUTORIAL_OPENFGA.md)
2. [P1: インフラとモデル](TUTORIAL_OPENFGA_P1_INFRA_MODEL.md)
3. [P2: DBとsqlc](TUTORIAL_OPENFGA_P2_DB_SQLC.md)
4. [P3: バックエンドサービス](TUTORIAL_OPENFGA_P3_BACKEND_SERVICES.md)
5. [P4: API・監査・スモークテスト](TUTORIAL_OPENFGA_P4_API_AUDIT_SMOKE.md)
6. [P5: UIとE2E](TUTORIAL_OPENFGA_P5_UI_E2E.md)
