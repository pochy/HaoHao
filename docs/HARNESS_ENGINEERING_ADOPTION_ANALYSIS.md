# Harness Engineering Adoption Analysis for HaoHao

## Executive Summary

OpenAI の Harness Engineering 記事は、エージェントにコードを書かせること自体ではなく、エージェントが安全に作業できる環境、知識、検証ループ、制御システムを設計することを主題にしている。HaoHao は monorepo、OpenAPI 生成、sqlc、Playwright E2E、smoke scripts、runbook、repo-local skills をすでに持っており、この考え方を採用しやすい。

ただし、HaoHao で採るべき形は「手書きコード禁止」や「完全自律マージ」ではない。既存の人間主導レビューとローカル検証を維持しながら、エージェントがより少ない探索で正しい作業を行えるように、知識の入口、領域別スキル、機械的チェック、UI/API/DB/observability の検証ハーネスを段階的に整備するのが現実的である。

推奨する採用方針は次の通り。

- `AGENTS.md` は短い地図として維持し、詳細は `docs/` と `.agents/skills/` に分散する。
- 繰り返し発生するレビュー指摘は、ドキュメントではなくテスト、lint、smoke、生成物 drift check に昇格する。
- Drive、RAG、OpenFGA、DB、frontend、observability のような領域ごとに、最短の調査コマンドと検証手順を repo に保存する。
- エージェントがアプリ、API、DB、ログ、メトリクスを読んで自己検証できる流れを標準化する。

## OpenAI Article Takeaways

参照元: [ハーネスエンジニアリング：エージェントファーストの世界における Codex の活用](https://openai.com/ja-JP/index/harness-engineering/)（2026-02-11）

記事の重要な学びは、HaoHao では次のように解釈できる。

- 人間の主業務は、実装そのものから「意図、環境、フィードバックループ、制約」を設計することへ移る。
- エージェントは、repo 内で読めるものだけを実質的な知識として扱える。Slack、口頭判断、暗黙知は、Markdown、テスト、schema、scripts、skills に変換しない限り存在しない。
- 大きな `AGENTS.md` はコンテキストを圧迫する。短い入口と、必要に応じて読める設計文書・runbook・skills の組み合わせがよい。
- アーキテクチャ方針は文章だけでは守れない。重要な境界や好みは、構造テスト、custom lint、CI、生成物 drift check で機械的に強制する。
- UI、ログ、メトリクス、トレースをエージェントが直接読めると、再現、修正、検証のループを人間の観察に依存しにくくできる。
- エージェント生成コードは既存パターンを増幅する。悪いパターンが広がる前に、定期的な「ガベージコレクション」と小さなリファクタリングを回す必要がある。

## HaoHao Current Readiness

HaoHao は Harness Engineering の土台をすでに多く持っている。

- `docs/CONCEPT.md` に、OpenAPI 3.1 優先、monorepo、単一バイナリ配信、責務分離、OpenFGA、PostgreSQL/sqlc、observability の基本方針がある。
- `.github/workflows/ci.yml` は Go test、frontend build、binary build、Playwright E2E、generated drift、DB schema drift、OpenAPI validate、Docker build、whitespace check を実行している。
- `Makefile` には `make gen`, `make sqlc`, `make openapi`, `make smoke-*`, `make e2e` があり、エージェントが実行できる検証単位が揃っている。
- `docs/RUNBOOK_OPERABILITY.md` と `docs/RUNBOOK_OBSERVABILITY.md` は、運用確認と障害切り分けを Markdown として repo に保存している。
- `.agents/skills/haohao-db-dev` と `.agents/skills/haohao-drive-debug` に、DB/Drive 調査の狭い手順が保存されている。
- 直近の Drive 不具合調査では、`ORDER BY` と `LIMIT` の見落としを skill / `AGENTS.md` に反映済みで、レビュー指摘や失敗を repo の知識へ戻す流れが始まっている。

不足している点も明確である。

- `docs/` が多い一方で、エージェント向けの索引がない。新しい作業でどの runbook / plan / tutorial を読むべきかを毎回探索している。
- アーキテクチャ境界の多くは文章化されているが、機械的に検査できる custom lint / structure test はまだ限定的である。
- UI 検証は Playwright E2E があるが、エージェントが「特定画面を起動してスクリーンショットやコンソールログで検証する」標準手順はまだ薄い。
- Observability runbook はあるが、ローカルでエージェントが LogQL / PromQL / trace 相当の情報を読むハーネスは記事ほど整っていない。
- skills はまだ Drive / DB 中心で、RAG、OpenFGA、frontend、OpenAPI generation、smoke triage などの領域別入口が足りない。

## Recommended Adoption Model

HaoHao では、Harness Engineering を「エージェントに任せる範囲を増やす」ではなく、「エージェントが迷わず検証可能に作業するための repo 内インフラを増やす」と定義する。

採用モデルは次の 4 段階にする。

### Phase 1: Knowledge Map

- `AGENTS.md` は現在の短さを維持し、詳細ルールを追加しすぎない。
- `docs/AGENT_KNOWLEDGE_INDEX.md` を追加し、主要領域ごとに読むべき文書、skill、検証コマンドをまとめる。
- 既存の長い tutorial は廃止せず、索引から必要なものへ誘導する。
- 新しい設計判断や調査結果は、チャットだけで終わらせず `docs/` または `.agents/skills/` に戻す。

### Phase 2: Domain Skills

- `haohao-drive-debug` と同じ粒度で、次の skills を追加する。
  - `haohao-rag-debug`: Drive RAG / vector search / LM Studio smoke / retrieval evaluation。
  - `haohao-openfga-debug`: model test、permission drift、share / workspace / folder authorization。
  - `haohao-openapi-gen`: Huma operation、OpenAPI surfaces、frontend generated SDK drift。
  - `haohao-frontend-debug`: Vue store、router、i18n、Playwright、build error triage。
- 各 skill は「まず読むファイル」「最短の `rg`」「DB/API確認」「検証コマンド」「避けるべき広すぎる探索」を含める。
- セッション後に skill が実態と違っていた場合は、修正を作業の一部として扱う。

### Phase 3: Mechanical Guardrails

- 既存 CI の drift check を維持し、繰り返し発生する逸脱を小さな検査へ昇格する。
- 候補:
  - API surface の混入検査を script 化する。
  - Huma operation に業務ロジックを入れないための構造チェックを追加する。
  - Drive / OpenFGA / RAG の重要 invariant を unit test または smoke に追加する。
  - docs / skills のリンク切れと必須見出しを確認する軽量 check を追加する。
- custom lint を増やす場合は、エージェントが修正しやすいエラーメッセージにする。

### Phase 4: Agent-Readable Verification Harness

- ローカル起動、API smoke、Playwright、ログ確認、DB確認を「作業完了の標準ループ」として docs に固定する。
- UI 作業では、スクリーンショット、console error、network error、主要 DOM 状態を確認する手順を標準化する。
- RAG / OCR / Drive のような非同期処理では、job status、index coverage、検索結果、citation を確認する smoke を整備する。
- Observability は、まず既存の structured log と Prometheus metrics をエージェントが最短で確認できるコマンド集から始める。LogQL / PromQL スタックの常時ローカル化は、必要性が高まってからでよい。

## Risks and Guardrails

- `AGENTS.md` 肥大化: 入口は短く保ち、詳細は docs / skills に移す。
- 誤った手順の固定化: skill は成功した調査ルートだけでなく、適用条件と失敗時の次手も書く。
- 自律化しすぎ: production 影響、migration、権限、データ削除、外部サービス設定は人間承認を維持する。
- 検証コスト増: すべてを default CI に入れず、fast CI、env-gated smoke、manual drill に分ける。
- docs 腐敗: 実装と衝突する情報は、次の作業で直す。特に `docs/AGENT_KNOWLEDGE_INDEX.md` と skills は短く保つ。

## Acceptance Criteria

Harness Engineering 採用の初期成功基準は次の通り。

- 新しいエージェントが、`AGENTS.md` から 1 回のジャンプで対象領域の docs / skill / 検証コマンドに到達できる。
- Drive / RAG / OpenFGA / frontend / OpenAPI の典型調査で、広すぎる `rg` や巨大 JSON 出力を使う回数が減る。
- PR ごとに、変更領域に応じた最小検証コマンドが docs または skill から明確に分かる。
- 同じ種類のレビュー指摘が 2 回以上出た場合、次は docs ではなく test / lint / smoke / script への昇格を検討する。
- エージェントが作業後に「何を検証し、何を repo の知識に戻すべきか」を明示できる。

## Recommended Next Steps

1. `docs/AGENT_KNOWLEDGE_INDEX.md` を追加し、既存 docs / skills / smoke の入口を 1 ページにまとめる。
2. RAG 作業が続いているため、次の skill は `haohao-rag-debug` を優先する。
3. `make` target、CI step、smoke script、docs の対応表を索引化する。
4. 直近 3-5 件の不具合修正を振り返り、機械的チェックへ昇格できるものを 1 つだけ選んで追加する。
5. UI 作業用に、Playwright screenshot / console / network error を確認する最短手順を `docs/` または skill に保存する。

