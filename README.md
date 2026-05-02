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

`make backend-dev` は先に `make db-up` で DB migration を適用し、`.env` を読み込んだうえで Air を起動します。`backend` 配下の Go ソース変更時にバックエンドを自動で再ビルド・再起動します。Air の監視設定は `.air.toml` にあります。

起動時は `DB_MIGRATION_CHECK_MODE=warn|fail|off` で DB migration drift を検知できます。既定は `warn` で起動を継続し、ローカル開発や CI では `fail` を推奨します。

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

## バックエンドログの確認

バックエンドログは当面 stdout/stderr の同じ structured log stream に出します。ログの種類は `log_type` で分かれます。

- `access`: HTTP access log。`method`, `path`, `status`, `latency_ms`, `request_id` を見る。
- `application_error`: API handler が未分類 error を 500 に丸める直前の root cause。通常の 500 調査はまずこれを見る。
- `panic`: panic recovery log。`stack` に stack trace が入る。
- `migration_check`: 起動時 DB migration drift check。DB が migration に追いついていない、dirty、または `schema_migrations` が読めない場合に出る。

ログをそのまま見るには、バックエンドを起動している terminal を確認します。

```bash
make backend-dev
# or
make backend-run
```

`jq` が使える場合は、`log_type` で絞り込めます。

```bash
make backend-dev 2>&1 | jq 'select(.log_type == "application_error")'
make backend-dev 2>&1 | jq 'select(.log_type == "panic")'
make backend-dev 2>&1 | jq 'select(.log_type == "migration_check")'
```

`jq` がない場合は、文字列検索でも最低限確認できます。

```bash
make backend-dev 2>&1 | grep '"log_type":"application_error"'
make backend-dev 2>&1 | grep '"log_type":"panic"'
```

500 response と root cause を突き合わせるときは、client error や access log に出ている `request_id` を使います。

```bash
make backend-dev 2>&1 | jq 'select(.request_id == "7eadc09c19f3b12ac27da345730552ea")'
```

`application_error` には `operation`, `error`, `error_type` が出ます。Postgres error の場合は可能な範囲で `sqlstate`, `severity`, `table`, `column`, `constraint` も出ます。request body, Cookie, Authorization header, CSRF token, raw SQL result はログに出しません。

stack trace が出るのは `panic` のときだけです。DB schema mismatch や外部サービス失敗のような通常の returned error は `application_error` に root cause を出し、stack trace は出しません。

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
