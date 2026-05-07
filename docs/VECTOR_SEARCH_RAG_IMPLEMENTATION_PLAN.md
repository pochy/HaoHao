# Vector Search / Vector DB / RAG 導入計画

## 目的

この文書は、`docs/VectorDB.md` の方針を HaoHao の現状実装に合わせて具体化し、ベクトル検索、Vector DB、RAG を段階的に導入するための実装計画をまとめる。

最初から RAG 全体を広げず、まず Data Pipeline の `schema_mapping` を AI-assisted にする。次に Drive / OCR / Gold table の検索を semantic / hybrid search に拡張し、最後に permission-filtered RAG を追加する。

## 実装状況

2026-05-08 時点で、Phase 1 と Phase 2 の基盤実装は完了している。Phase 3 は schema mapping candidate の backend MVP、seed/rebuild 導線、inspector UI の候補取得/適用、preview sample values 連携、pgvector を使う vector/hybrid ranking 接続、private accepted/rejected evidence capture、bulk mapping example rebuild、tenant-shared evidence backend flow、tenant admin evidence 管理 UI、tenant admin local search rebuild/status UI まで実装済み。Phase 4 は Drive search の `mode=keyword|semantic|hybrid` API、generated frontend types、Drive 検索バーの mode toggle、tenant vector policy 配下の semantic / hybrid `drive_file` search、OpenFGA filter、低スコア semantic hit filter まで実装済み。Phase 5 は tenant policy で明示有効化する Drive RAG API、structured citation guard、RAG answer panel、tenant admin RAG policy UI、LM Studio-backed starter smoke まで完了している。

このセッションでは、計画作成、別 AI レビューの妥当性判定、計画改善、Phase 1-5 の段階実装、複数回の動作検証、検証結果を受けた計画更新を繰り返した。現在の到達点は「local search / pgvector / tenant policy / schema mapping evidence / Drive semantic search / permission-filtered RAG API / frontend RAG controls の主要足場が動く」状態である。starter evaluation dataset と acceptance gate は作成済みで、LM Studio `text-embedding-mxbai-embed-large-v1` による in-memory harness 評価は通過した。実 API / 実 pgvector index 経由の LM Studio smoke も、Drive 用 semantic query expansion と resource-specific score threshold を入れた後に通過した。RAG の broad smoke は実行可能な足場まで作り、実 backend path の retrieval / indexing 挙動を観察できる状態になったが、残る失敗は主に LM Studio embedding latency と outbox/indexing readiness の評価環境差分であり、導入計画の必須実装 blocker ではない。`ruri-v3-310m` は試行したが、LM Studio `/v1/embeddings` が `No models loaded` を返す状態が再現し、CLI からも安定して local model key として load できなかったため、この計画の標準 embedding model から外す。以後の標準は `text-embedding-mxbai-embed-large-v1` とする。

完了済み:

- `compose.yaml` の app Postgres image を `pgvector/pgvector:0.8.2-pg18-trixie` に変更した。
- `db/migrations/0044_local_search_pgvector.*.sql` を追加し、`vector` extension、`local_search_embeddings.embedding vector(1024)`、denormalized filter column、HNSW index、`schema_column` / `mapping_example` resource kind を導入した。
- `db/schema.sql` と sqlc generated code を更新した。
- `db/queries/local_search.sql` に embedding upsert、document embedding delete、pending document list、tenant/resource/model/status filter 付き cosine search query を追加した。
- `backend/internal/service/embedding_types.go`、`local_embedding_provider.go`、`vector_store.go` を追加し、`EmbeddingProvider` / `VectorStore` と pgvector 実装、Ollama / LM Studio embedding 呼び出しを入れた。
- `LocalSearchService.HandleEmbeddingRequested` は `skipped` 固定ではなく、tenant policy 確認、chunking、local embedding runtime 呼び出し、pgvector upsert、job 完了更新を行う。
- `UpsertLocalSearchDocument` 後に、vector enabled tenant では embedding job を best-effort enqueue する。
- Tenant settings validation は v1 仕様として `vectorEnabled=true` なら `dimension=1024` 固定、`runtimeURL` 必須、localhost / 127.0.0.1 制限を強制する。
- OpenAPI / frontend generated types は再生成済み。
- `db/migrations/0045_schema_mapping_candidates.*.sql` を追加し、`data_pipeline_schema_columns` と `data_pipeline_mapping_examples` を導入した。
- `POST /api/v1/data-pipelines/schema-mapping/candidates` を追加した。
- `DataPipelineService.SchemaMappingCandidates` を追加し、tenant guard、pipeline/version view authorization、schema column keyword search、accepted/rejected evidence count、strict hint scoring を実装した。
- `DataPipelineService.RebuildSchemaMappingSearchDocuments` と `LocalSearchService.UpsertDocument` を追加し、schema column を `local_search_documents` へ同期する足場を入れた。
- `samples/schema-mapping/invoice-columns.json` と `scripts/seed-schema-mapping-columns.mjs` を追加し、invoice domain の標準 schema column を再現可能に seed できるようにした。
- tenant admin API `POST /api/v1/admin/tenants/{tenantSlug}/data-pipelines/schema-mapping/search-documents/rebuild` を追加し、schema column を `local_search_documents` へ materialize できるようにした。
- `DataPipelineInspector` の `schema_mapping` inspector に domain / schemaType、候補取得、候補表示、target column 適用 UI を追加した。
- `DataPipelineInspector` に selected preview を渡し、候補取得時に preview rows から最大 5 件の sample values を API request に含めるようにした。
- `LocalSearchService.SearchSemantic` を追加し、tenant vector policy が有効で embedding が存在する場合は `schema_column` を pgvector cosine search で取得できるようにした。
- `DataPipelineService.SchemaMappingCandidates` は keyword 候補と vector 候補を merge し、`matchMethod` を `keyword` / `vector` / `hybrid` で返す。vector runtime が未設定、disabled、または embedding search が失敗した場合は keyword ranking に fallback する。
- `POST /api/v1/data-pipelines/schema-mapping/examples` を追加し、候補の採用/却下履歴を `data_pipeline_mapping_examples` に private evidence として upsert できるようにした。採用履歴は `mapping_example` local search document にも同期する。
- `DataPipelineInspector` は候補を適用したときに accepted mapping example を記録する。
- tenant admin rebuild API は `schema_column` だけでなく既存 `mapping_example` も bulk materialize する。response は互換用の `indexed` に加えて `schemaColumnsIndexed` と `mappingExamplesIndexed` を返す。
- tenant admin API `PATCH /api/v1/admin/tenants/{tenantSlug}/data-pipelines/schema-mapping/examples/{mappingExamplePublicId}/sharing` を追加し、mapping example の `sharedScope` を `private` / `tenant` に切り替えられるようにした。`tenant` にした evidence だけが pipeline 横断で candidate ranking に効く。
- tenant admin API `GET /api/v1/admin/tenants/{tenantSlug}/data-pipelines/schema-mapping/examples` を追加し、mapping example evidence を pipeline、source/target column、decision、shared scope、materialized status 付きで一覧できるようにした。
- Tenant Admin の Data 画面に schema mapping evidence 管理 UI を追加した。一覧、検索、`private` / `tenant` filter、accepted/rejected filter、共有/非公開切替、schema mapping search documents rebuild を同じ画面から実行できる。
- Tenant Admin の Data 画面に Local Search Index の状態表示と rebuild 導線を追加した。最新 10 件の local search index job を status、reason、resource kind、indexed/skipped、failed、completed time 付きで確認し、同じ画面から full rebuild を enqueue できる。
- `GET /api/v1/drive/search/documents` に `mode=keyword|semantic|hybrid` を追加した。未指定時は従来互換の `keyword`、`semantic` は `local_search_embeddings` の `drive_file` vector hit、`hybrid` は keyword candidates と vector hit を merge する。
- Drive search UI に search mode toggle を追加し、route query と generated frontend API types に `mode` を通すようにした。
- Drive semantic search は resource kind 別の score threshold を使う。Drive file は実 API smoke の raw score に合わせて `0.51`、schema column は評価 harness の starter gate と分けて `0.78`、その他は既存 fallback `0.05` とする。
- Drive semantic query には日本語の業務同義語を限定的に拡張する。例: `支払期限` は `振込期限`、`支払期日`、`入金期限` を semantic query に含める。
- `samples/evaluation/schema-mapping-invoice-ja.json` と `samples/evaluation/drive-rag-retrieval-ja.json` を追加し、schema mapping と Drive/RAG retrieval の starter evaluation dataset を定義した。
- `scripts/validate-vector-rag-evaluation-datasets.mjs` と `make validate-vector-rag-eval` を追加し、dataset の gate、ID、target column、citation / forbidden document reference を機械的に検証できるようにした。
- `scripts/evaluate-vector-rag-retrieval.mjs` と `make eval-vector-rag-retrieval` を追加し、starter dataset を embedding provider で評価できる harness を入れた。`HAOHAO_EVAL_EMBEDDING_RUNTIME=ollama|lmstudio` で実 local model を使い、default の `fake` runtime は script path / metric calculation の検証用に使う。
- `features.drive.rag` tenant policy、local generation provider、`DriveService.QueryRAG`、`POST /api/v1/drive/rag/query` を追加し、RAG は明示的に有効化された tenant と local generation runtime だけで動くようにした。
- RAG response は structured JSON の `claims[]` を検証し、今回 permission-filter を通った context の `citationId` を参照しない claim を除外する。citation が残らない回答は block する。
- `scripts/smoke-lmstudio-drive-rag.mjs` と `make smoke-lmstudio-drive-rag` を追加し、LM Studio embedding + local generation model で Drive RAG の実 API smoke を実行できるようにした。
- `DriveService.QueryRAG` は RAG 用に検索候補を最大 20 件まで広げ、質問語と file name / snippet / local search matches の lexical overlap で context 候補を rerank する。`TP-2026-0412` のようなハイフン付き identifier は分割しすぎないように扱う。
- `scripts/smoke-lmstudio-drive-rag.mjs` の broad mode は、readiness polling と RAG query で同じ marker 付き query を使うように修正した。これにより、開発 tenant の既存 file と broad smoke 用 upload file の混在を減らして実 backend path を評価できる。
- `scripts/openai-compatible-proxy.mjs` を追加し、tenant settings の localhost runtime URL 制約を保ったまま、`127.0.0.1:11234 -> 192.168.1.28:1234` の LM Studio proxy 経由で検証できるようにした。

検証済み:

- `make gen`
- `go test ./backend/...`
- `npm run build`
- 一時 pgvector Postgres で migration chain `up -> down 1 -> up 1`
- 通常ローカル DB に 0045 migration 適用、`schema_migrations.version=45`、`dirty=false`
- 現実寄りの schema mapping 課題として、`請求No` を `請求書番号 / invoice_no` に寄せる疑似 embedding 検索を実施し、tenant/resource/model/status filter と HNSW index 作成を確認した。
- HTTP API smoke として、仕入請求書 CSV の source columns `請求No`、`支払先`、`税込合計`、`社内メモ` を `POST /api/v1/data-pipelines/schema-mapping/candidates` に投入した。期待どおり `invoice_number`、`vendor_name`、`total_amount` が返り、`社内メモ` は候補なしになった。
- 同 smoke で `columns=[]` と CSRF header なしは Huma validation により `422` で拒否されることを確認した。
- schema mapping candidate ranking の pgvector 接続後に `go test ./backend/internal/service` と `go test ./backend/...` が通ることを確認した。
- mapping example capture API 追加後に `make gen`、`go test ./backend/internal/service`、`go test ./backend/...`、`npm run build` が通ることを確認した。
- `data_pipeline_mapping_examples` の version 有無それぞれの `ON CONFLICT` query は、ローカル Postgres で parse / conflict target validation まで通ることを確認した。確認用 insert は FK で失敗しており、永続データは作成していない。
- 実 API smoke として、`demo@example.com` で `acme` tenant にログインし、一時 pipeline を作成したうえで `請求No -> invoice_number` の accepted mapping example を記録した。記録後、candidate API の `acceptedEvidence` が `0` から `1` に増え、DB 上で `data_pipeline_mapping_examples.shared_scope = private` と `local_search_documents.resource_kind = mapping_example` が作成されることを確認した。
- 同じ mapping example を再送しても duplicate row は作られず、invalid decision と CSRF なし request は `422` で拒否されることを確認した。一時作成した pipeline / mapping example / `mapping_example` local search document は検証後に削除済み。
- bulk `mapping_example` rebuild 追加後に `make gen`、`go test ./backend/internal/service`、`go test ./backend/internal/api`、`go test ./backend/...`、`npm run build` が通ることを確認した。
- bulk `mapping_example` rebuild 用の DB query はローカル Postgres で実行確認済み。`acme` tenant には検証後の mapping example が残っていないため 0 rows だったが、query shape と join は実 DB で通った。
- tenant-shared evidence API 追加後に `make gen`、`go test ./backend/internal/service`、`go test ./backend/internal/api`、`go test ./backend/...`、`npm run build` が通ることを確認した。
- tenant-shared evidence の get / update SQL はローカル Postgres で実行確認済み。検証用 mapping example は削除済みのため 0 rows / `UPDATE 0` だったが、query shape は実 DB で通った。
- tenant-shared evidence の実 API smoke として、source pipeline で作成した `請求No -> invoice_number` の private accepted example が別 pipeline の候補 ranking に効かないこと、admin API で `sharedScope=tenant` に昇格すると別 pipeline の `acceptedEvidence` が `0 -> 1` になり score が `1.800000011920929 -> 1.880000011920929` に上がること、`private` に戻すと `acceptedEvidence=0` に戻ることを確認した。無効な `sharedScope=workspace` は `422`。検証用 pipeline / mapping example / `mapping_example` local search document は削除済み。
- tenant admin evidence 管理 UI 追加後に `make gen`、`go test ./backend/internal/service ./backend/internal/api`、`go test ./backend/...`、`npm --prefix frontend run build` が通ることを確認した。
- tenant admin evidence 管理 UI の pointer-click regression として `e2e/schema-mapping-evidence.spec.ts` を追加し、`Share`、`Make private`、`Rebuild search docs` が Playwright の通常 click で通ることを確認した。agent-browser の通常 click では一部 button の UI 更新が安定しなかったため、この regression を実クリック系の継続検証に使う。
- tenant admin local search rebuild/status UI 追加後に `npm --prefix frontend run build` が通ることを確認した。
- `e2e/schema-mapping-evidence.spec.ts` を拡張し、`Rebuild local search` click、queue 成功 message、`queued/admin_rebuild` job row 表示も通常 Playwright click で確認するようにした。`E2E_BASE_URL=http://127.0.0.1:5174 npm --prefix frontend run e2e -- ../e2e/schema-mapping-evidence.spec.ts` は、一時 local-login backend `18080` と Vite `5174` で通過した。
- Drive search mode API / UI 追加後に `make gen`、`go test ./backend/internal/service`、`go test ./backend/...`、`npm --prefix frontend run build` が通ることを確認した。
- Drive semantic / hybrid search の実 API smoke として、一時 local-login backend と fake Ollama-compatible embedding runtime を起動し、`acme` tenant の vector policy を一時有効化した。現実寄りの請求書 file に `振込期限: 2026-06-30`、`税込合計: 128000円`、`支払先: Acme Supplies` を入れ、別 file に無関係な週次 sales note を入れて local search rebuild を実行した。`mode=keyword&q=支払期限` は 0 件、`mode=semantic&q=支払期限` と `mode=hybrid&q=支払期限` は請求書 file のみを返し、note は返らないことを確認した。未指定 mode は従来どおり keyword として `q=振込期限` で請求書 file を返した。
- LM Studio `ruri-v3-310m` 指定の実 API smoke では、tenant policy / embedding row は `model='ruri-v3-310m'`、dimension `1024` として保存された。最初の smoke では `mode=semantic&q=支払期限` が請求書 file を返したものの、無関係なプロダクト週報も同時に返し、週報が上位になるケースを確認した。raw score は `支払期限` 単体で週報 `0.471`、請求書 `0.447` だった。この検証は後に LM Studio 側の model routing 不整合が判明したため、採用モデルの評価としては扱わない。
- 上記を受けて、Drive semantic query に `支払期限 振込期限 支払期日 入金期限` のような限定的な業務語彙 expansion を追加した。raw score は週報 `0.497`、請求書 `0.521` に改善したため、Drive threshold を `0.51` に設定した。
- 修正後の real API / real pgvector smoke は ruri 実験でも一度通過した。`mode=keyword&q=支払期限` は 0 件、`mode=semantic&q=支払期限` と `mode=hybrid&q=支払期限` は請求書 file のみを返し、無関係なプロダクト週報を除外した。未指定 mode は従来どおり keyword として `q=振込期限` で請求書 file を返した。検証用 tenant settings は元に戻し、一時 upload file は soft delete 済み。以後は同じ smoke script の既定 model を `text-embedding-mxbai-embed-large-v1` に戻して検証する。
- 同 smoke で pgvector の `ORDER BY embedding <=> query LIMIT n` が低スコア候補を拾いすぎる問題を検出したため、`SearchSemantic` に `localSearchMinSemanticScore=0.05` の下限 filter を追加し、0 score 近傍の noise を除外するよう修正した。検証用 tenant settings は元に戻し、一時 upload file は soft delete 済み。
- `node scripts/validate-vector-rag-evaluation-datasets.mjs` で starter evaluation dataset を検証した。schema mapping は 30 cases / no-candidate 4 cases / target columns 10、Drive/RAG retrieval は 8 documents / 12 queries / denied 2 queries で、ID 参照と gate 定義が通ることを確認した。
- `make eval-vector-rag-retrieval` を default fake runtime で実行し、schema mapping / Drive/RAG retrieval の metric calculation、gate 判定、failure reporting が動くことを確認した。fake runtime は実精度評価用ではないため `ok=false` は許容し、`HAOHAO_EVAL_ENFORCE_GATES=1` は実 local embedding model での評価時に使う。
- RAG API 追加後に `make gen`、`go test ./backend/internal/service`、`go test ./backend/internal/api`、`go test ./backend/...`、`npm --prefix frontend run build` が通ることを確認した。
- LM Studio-backed RAG 実 API smoke は `HAOHAO_EVAL_EMBEDDING_MODEL=text-embedding-mxbai-embed-large-v1`、`HAOHAO_RAG_GENERATION_MODEL=qwen/qwen3.5-9b` で通過した。現実寄りの請求書 file に `支払期限: 2026-06-30` と `税込合計: 128000円` を入れ、無関係な note も upload したうえで、回答が `2026年6月30日` と `128,000円` を含み、citation が請求書 file を指し、no-citation guard で block されないことを確認した。
- 2026-05-07 の最終確認で、`http://192.168.1.28:1234/v1/models` に `text-embedding-mxbai-embed-large-v1` が表示され、`/v1/embeddings` は同 model 名で 1024 次元 embedding を返した。
- RAG answer panel / tenant admin RAG policy UI の browser smoke を agent-browser で実施した。一時 local-login backend `18080` と Vite `5174` を起動し、`/drive/search?q=支払期限` で RAG panel が表示され、検索語が質問欄に引き継がれ、submit が `POST /api/v1/drive/rag/query` を呼び、tenant policy disabled 時は `drive policy denied` を画面に表示することを確認した。さらに `/tenant-admin/acme/drive-policy` で `Drive RAG enabled`、generation runtime `LM Studio`、runtime URL、model、context budget controls が表示・編集でき、Current Policy に `Enabled / runtime lmstudio` と反映されることを確認した。UI smoke 中に古い `bin/haohao` では RAG endpoint が `404` になることも確認したため、ブラウザ検証は現在ソースの `go run ./backend/cmd/main` で実施した。
- Broad RAG smoke の実行足場として、`scripts/smoke-lmstudio-drive-rag.mjs` に `HAOHAO_SMOKE_RAG_BROAD=1` mode を追加した。既存の Drive/RAG evaluation dataset から viewable documents を実 Drive に upload し、実 local search / pgvector index の到達を query ごとに polling したうえで、複数 RAG query の citation coverage、answer fact coverage、no-citation answer rate、denied block rate、p50/p95 latency を集計する。デフォルト broad set は 12 queries で、`HAOHAO_SMOKE_RAG_QUERY_IDS` または `HAOHAO_SMOKE_RAG_QUERY_LIMIT` で対象を変更できる。2026-05-07 の追加時点では、`192.168.1.28:1234` と `127.0.0.1:1234` の LM Studio endpoint がどちらも接続不可だったため、実 broad run は未実施。
- LM Studio が復旧した後、`text-embedding-mxbai-embed-large-v1` + `qwen/qwen3.5-9b` で broad smoke を実行した。tenant settings は runtime URL を localhost / 127.0.0.1 に制限するため、`scripts/openai-compatible-proxy.mjs` を追加して `127.0.0.1:11234 -> 192.168.1.28:1234` 経由で検証した。初期 run では RAG endpoint まで到達したが、期待文書が RAG context に入らず citation coverage / answer fact coverage が不足した。これを受けて RAG 用候補取得数拡張、lexical rerank、context budget env、broad smoke marker、marker query の RAG 実行反映を追加した。
- 修正後の LM Studio-backed 2 query broad smoke では、青葉商事の請求書ケースが期待文書 citation と `2026-06-30` fact coverage を満たすことを確認した。一方、`TP-2026-0412` ケースは LM Studio embedding / outbox indexing の完了前に expected document が readiness に入らない run が残り、全体 gate は未通過だった。`OUTBOX_WORKER_TIMEOUT=60s` と `OUTBOX_WORKER_BATCH_SIZE=1` では indexing は安定化したが、12 documents broad smoke では LM Studio embedding latency が支配的で、検証時間内に全期待文書が揃わないことがある。この残課題は実装 blocker ではなく、評価環境 / smoke 分割の運用課題として扱う。

検証時の注意:

- `db/schema.sql` を fresh DB に直接流す確認では、今回の `local_search` / pgvector 部分は作成できたが、既存 schema dump の後半で `public.uuidv7()` / 一部 FK 順序に起因するエラーが出た。現時点の DB 適用確認は migration chain を正とする。
- `go mod tidy` は sandbox / network 制約で完走確認できていない。`github.com/pgvector/pgvector-go` は direct dependency として `backend/go.mod` に追加済みで、backend 全体 test は通っている。
- Schema mapping candidate API の最初の smoke では、source column、sample values、neighbor columns をすべて keyword query に混ぜたため、Postgres full-text search が候補を返せないケースがあった。現在は source column を keyword 主検索語にし、sample values / neighbor columns は semantic query text に含めて vector/hybrid ranking で使う。
- Schema mapping candidate の実 API smoke は local embedding runtime なしで実施したため、candidate response の `matchMethod` は `keyword` 経路を確認した。一方、評価 harness は LM Studio の `text-embedding-mxbai-embed-large-v1` 指定で starter dataset の schema mapping gate を通過済み。
- Drive semantic / hybrid smoke は fake Ollama-compatible runtime と LM Studio-backed real API / real pgvector index の両方で semantic 経路まで確認した。評価 harness は LM Studio の `text-embedding-mxbai-embed-large-v1` 指定で Drive/RAG retrieval gate を通過済み。
- `ruri-v3-310m` は一時的に検証したが、LM Studio の model routing が安定せず、`/v1/embeddings` が `No models loaded` を返す状態が再現したため、この計画では採用しない。既定の smoke script と今後の検証は `text-embedding-mxbai-embed-large-v1` を使う。

任意の追跡課題:

- RAG の real API broad smoke を quick gate と broad gate に分ける。quick gate は選択 query に必要な documents だけを upload して実装 regression を短時間で見る。broad gate は 12+ query / 16 documents の end-to-end latency と indexing readiness を観測する任意の長時間評価にする。
- LM Studio 実行時の outbox worker 推奨設定を runbook に追記する。現時点の実測では `OUTBOX_WORKER_BATCH_SIZE=1`、`OUTBOX_WORKER_TIMEOUT=60s` 以上が broad smoke には安定しやすい。

## 現状調査

HaoHao には、Vector/RAG 導入に使える足場がすでにある。

- PostgreSQL は metadata、tenant boundary、job state の正本として使われている。
- Redis は導入済みだが `redis:7.4` で、現状は session、rate limit、realtime などに使われている。RediSearch / Redis Stack 前提ではない。
- ClickHouse は Dataset / Work table / Data Pipeline の実行基盤として使われている。
- Drive file body は `FileStorage` abstraction の背後にあり、local / SeaweedFS S3-compatible storage に対応している。
- OpenFGA は Drive authorization の正本として使われ、検索結果は最終レスポンス前に permission filter される。
- `local_search_documents` は Drive file、OCR run、product extraction、Gold table を横断する Postgres full-text index として実装済み。
- `local_search_embeddings` は `vector(1024)` 化済みで、tenant/resource/model/status filter 用 column と HNSW index を持つ。
- `local_search.embedding_requested` event は outbox handler まで配線済みで、現在は local embedding runtime を呼び出して pgvector に upsert する。
- Tenant settings には `localSearch.vectorEnabled`、`embeddingRuntime`、`runtimeURL`、`model`、`dimension` がすでにある。
- Data Pipeline には `schema_mapping`、`schema_inference`、`entity_resolution`、`canonicalize`、`deduplicate` などの step catalog と UI entry がある。
- 非構造化 Data Pipeline は `extract_text -> classify_document -> extract_fields -> output` の hybrid materialize flow を持つ。

このため、導入方針は「新しい検索基盤を別途作る」ではなく、`local_search` を semantic index service に拡張する。

## 採用方針

### v1 Vector DB

v1 は `pgvector` を採用する。

理由:

- 既存の tenant metadata、Drive / Dataset / Pipeline metadata、job state が PostgreSQL にある。
- metadata filter、ACL 前の candidate filter、JOIN、rebuild 対象抽出を SQL で扱える。
- `local_search_documents` / `local_search_embeddings` の既存足場を活かせる。
- Redis を RediSearch 用に Redis Stack へ置き換えるより、運用差分が小さい。
- Vector DB は派生 index として扱い、Postgres 正本から再生成できる。

Redis は v1 では Vector DB にしない。将来、semantic cache や query result cache として併用する余地は残す。

Qdrant は v2+ の移行先として扱う。目安は、pgvector の検索 latency、reindex 時間、tenant 分離、filter selectivity、運用コストが要件を満たさなくなった時点。

### Embedding model

v1 の標準候補は多言語対応の `bge-m3` とし、dimension は 1024 を想定する。

ただし model は tenant setting で固定し、別 model / 別 dimension の embedding を同じ検索空間で混ぜない。model、dimension、chunking policy を変えた場合は再indexする。

local development では次を許可する。

- `ollama`
- `lmstudio`
- unit test 用 `fake`

remote embedding provider は v1 では扱わない。tenant settings の既存 validation と同様、runtime URL は localhost / 127.0.0.1 に制限する。

## Phase 1. pgvector 基盤

Status: implemented in `0044_local_search_pgvector`.

### DB migration

次の migration を追加した。

- `CREATE EXTENSION IF NOT EXISTS vector`
- `local_search_documents.resource_kind` と `local_search_index_jobs.resource_kind` の check constraint に次を追加:
  - `schema_column`
  - `mapping_example`
- `local_search_embeddings` を pgvector 対応へ拡張:
  - `tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE`
  - `resource_kind TEXT NOT NULL`
  - `resource_id BIGINT NOT NULL`
  - `resource_public_id UUID NOT NULL`
  - `embedding vector(1024)`
  - `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`
  - `indexed_at TIMESTAMPTZ`

pgvector は column dimension が型に含まれるため、v1 では標準 dimension を 1024 に固定する。tenant setting の `dimension` は `1024` 以外を拒否する。将来 multi-model を同時運用する場合は、model/dimension 別 table または partition を検討する。

Local development の `compose.yaml` は app Postgres service を `pgvector/pgvector:0.8.2-pg18-trixie` に切り替え済み。Docker Hub 上で tag の存在を確認し、一時 Postgres container で `vector` extension が利用可能なことを確認した。

本番環境でも migration 前に pgvector extension が利用可能であることを readiness / deployment runbook に追加する。

今後別の Postgres base image へ戻す場合は、次のどちらかを必ず実施する。

- 推奨: Postgres service image を pgvector 対応 image に切り替え、`make up` で `CREATE EXTENSION vector` が通ることを smoke test する。image は `pgvector/pgvector:pg18` 系を候補にするが、採用時点で存在する tag を確認して digest / tag を固定する。
- 代替: local Postgres image に pgvector package を install する Dockerfile を追加し、compose はその image を使う。

Index:

- `local_search_embeddings(tenant_id, resource_kind, model, status, indexed_at DESC)`
- `local_search_embeddings(document_id, chunk_ordinal)`
- `local_search_embeddings USING hnsw (embedding vector_cosine_ops) WHERE status = 'completed' AND embedding IS NOT NULL`

小規模 tenant の PoC では exact scan でもよいが、migration は HNSW 前提で作る。

pgvector の ANN index は metadata filter の selectivity が高いと recall が落ちるため、v1 の検索実装では次を必須にする。

- vector search は `tenant_id`、`resource_kind`、`model`、`status` filter を必ず適用する。
- `hnsw.ef_search` と候補取得数は API `limit` より十分大きくし、permission filter 後の候補不足を避ける。
- filter 後の候補数が閾値未満の場合は exact scan fallback を行う。
- pgvector version が対応している場合は `hnsw.iterative_scan` の利用可否を migration / runtime smoke test で確認する。
- tenant / model / resource_kind ごとの row 数と latency を見て、必要になった時点で model/resource_kind 別 partial index または partition を追加する。

既存 `local_search_embeddings` の unique constraint は `UNIQUE (document_id, chunk_ordinal, model, content_hash)` を維持する。`tenant_id`、`resource_kind`、`resource_id`、`resource_public_id` は document 由来の denormalized filter column とし、同じ chunk/model/content_hash の重複 embedding は作らない。document 削除時は既存 `ON DELETE CASCADE` で embedding も削除する。

schema mapping 用の `schema_column` / `mapping_example` も、まず `local_search_documents` に canonical document を作り、embedding は document に従属させる。resource 系 column は検索 filter と rebuild 対象抽出のために持つが、正本 ID は `local_search_documents.resource_kind/resource_id` とする。

既存 table からの migration 手順:

1. `local_search_embeddings` に nullable で `tenant_id`、`resource_kind`、`resource_id`、`resource_public_id`、`metadata`、`indexed_at` を追加する。
2. `document_id` で `local_search_documents` を join し、追加列を backfill する。
3. 既存 `embedding DOUBLE PRECISION[]` は v1 では再利用しない。既存 row があれば `status='pending'`、`embedding=NULL`、`error_summary=NULL` に戻し、embedding job で再生成する。
4. `embedding_v2 vector(1024)` を追加する。
5. 旧 `embedding DOUBLE PRECISION[]` を drop し、`embedding_v2` を `embedding` に rename する。既存値を使わない方針なので direct cast には依存しない。
6. backfill 後に `tenant_id`、`resource_kind`、`resource_id`、`resource_public_id` を `NOT NULL` にする。
7. HNSW partial index を作成する。本番では `CREATE INDEX CONCURRENTLY` を使い、初回 build の時間と write amplification を rollout runbook に入れる。
8. down migration では HNSW index、追加 column、resource kind check constraint、`vector` extension 依存を戻す手順を明示する。ただし extension drop は同一 DB の他 object が使っていない場合だけ行う。

service 側では `localSearchResourceKindValid` と tenant admin local search job の API enum に `schema_column` / `mapping_example` を追加済み。`pipeline_rule` は v1 では resource kind に追加しない。変換ルール検索を扱う場合は、別 phase で正本 table と rebuild flow を定義してから追加する。

tenant settings の validation は Phase 1 で変更済み。v1 では `vectorEnabled=true` の場合 `dimension=1024` のみ許可し、`embeddingRuntime != none`、`runtimeURL`、`model` を必須にする。runtime URL は既存 OCR policy と同じく localhost / 127.0.0.1 に制限する。`vectorEnabled=false` の場合は `dimension=0` または `1024` を許可する。

### sqlc query

`db/queries/local_search.sql` に次を追加済み。

- embedding upsert
- document ごとの embedding delete
- stale / pending embedding list
- tenant + resource kind + model + vector cosine search
- full-text rank と vector rank を同時に返す hybrid candidate query は Phase 4 で追加する

検索 query は必ず `tenant_id`、`resource_kind`、`model`、`status = 'completed'` で絞る。

## Phase 2. Embedding job

Status: implemented for local runtime embedding and pgvector upsert. Accuracy evaluation and production hardening remain.

### Backend interface

`backend/internal/service` に次の interface を追加する。

```go
type EmbeddingProvider interface {
	Embed(ctx context.Context, input EmbeddingRequest) (EmbeddingResult, error)
}

type VectorStore interface {
	UpsertEmbedding(ctx context.Context, input VectorUpsertInput) error
	Search(ctx context.Context, input VectorSearchInput) ([]VectorSearchHit, error)
	DeleteForDocument(ctx context.Context, tenantID, documentID int64) error
}
```

v1 の `VectorStore` は pgvector 実装だけにする。Qdrant は同じ interface で後から追加する。

配置は `backend/internal/service/embedding_types.go` に `EmbeddingProvider`、`EmbeddingRequest`、`EmbeddingResult` と chunking 関連 type を置き、`backend/internal/service/vector_store.go` に `VectorStore` と pgvector 入出力 type を置いた。Ollama / LM Studio の local runtime provider は `local_embedding_provider.go` に置いた。

### Job flow

`LocalSearchService.HandleEmbeddingRequested` を実処理に変更済み。

1. tenant policy を読む。
2. `vectorEnabled=false` または `embeddingRuntime=none` なら `skipped`。
3. 対象 `local_search_documents` を取得する。
4. `title + body_text + metadata` を chunking する。
5. chunk ごとに `content_hash` を計算する。
6. 現実装では document 単位で既存 embedding を削除して再 upsert する。`model + dimension + content_hash` が同じ場合の chunk 単位 reuse は次の最適化として残す。
7. provider で embedding を生成する。
8. `local_search_embeddings` に upsert する。
9. job status、indexed/skipped/failed count、last_error を更新する。

v1 では `dimension=1024` 固定なので、再利用条件の `dimension` は validation guard として扱う。複数 dimension を同時保存しないため、unique constraint に `dimension` は追加しない。将来 multi-dimension を扱う場合は table / partition を分ける。

Chunking default:

- `maxChunkRunes`: 1600
- `overlapRunes`: 200
- `maxChunksPerDocument`: 32

schema mapping 用の短文 document は chunk しない。

chunk しない document も `local_search_embeddings.chunk_ordinal` は `0` 固定にする。`chunk_ordinal` は現行 schema と unique constraint に合わせて `NOT NULL` のままとし、`NULL` は使わない。

### Rebuild

現実装では個別 index flow の `UpsertLocalSearchDocument` 後に embedding job を best-effort enqueue する。

既存 `RequestRebuild` は resource rebuild の中で各 `UpsertLocalSearchDocument` を通るため、vector enabled tenant では embedding job が enqueue される。将来、大量 rebuild 時の job storm を避けるため、batch enqueue / dedupe / rate limit を追加する。

## Phase 3. Schema Mapping Candidate Search

Status: backend candidate API, seed/rebuild flow, inspector UI, preview sample values, vector/hybrid ranking, private accepted/rejected evidence capture, bulk mapping example rebuild, tenant-shared evidence API, tenant admin evidence management UI, and tenant admin local search rebuild/status UI are implemented. Browser smoke for the new admin UI is done, and a Playwright regression now covers pointer-click Local Search rebuild / Share / Make private / Rebuild behavior.

最初の実用 MVP は `Schema Mapping Candidate Search` とする。

### Source of truth

Vector index は派生データなので、schema mapping 用の正本 table を追加する。これは `0045_schema_mapping_candidates` で実装済み。

Phase 1 と Phase 3 の migration は分けた。Phase 1 は pgvector 化、既存 `local_search` table の constraint / column 変更、tenant settings validation までに限定した。Phase 3 で `data_pipeline_schema_columns` と `data_pipeline_mapping_examples` を追加し、その後に `schema_column` / `mapping_example` document rebuild を実装する。

#### `data_pipeline_schema_columns`

標準スキーマ項目の正本。

主な column:

- `tenant_id`
- `public_id`
- `domain`
- `schema_type`
- `target_column`
- `description`
- `aliases JSONB`
- `examples JSONB`
- `language`
- `version`
- `archived_at`

Unique key:

- `(tenant_id, domain, schema_type, target_column, version)`

Implemented in `0045_schema_mapping_candidates.up.sql`.

#### `data_pipeline_mapping_examples`

採用/却下された mapping 履歴の正本。

主な column:

- `tenant_id`
- `public_id`
- `pipeline_id`
- `version_id`
- `schema_column_id`
- `source_column`
- `sheet_name`
- `sample_values JSONB`
- `neighbor_columns JSONB`
- `decision` (`accepted`, `rejected`)
- `decided_by_user_id`
- `decided_at`

Constraint / relationship:

- `tenant_id` は `tenants(id)` に `ON DELETE CASCADE`。
- `pipeline_id` は `data_pipelines(id)` に `ON DELETE CASCADE`。
- `version_id` は `data_pipeline_versions(id)` に `ON DELETE SET NULL` とし、過去の採用/却下履歴は version 削除後も残す。
- `schema_column_id` は `data_pipeline_schema_columns(id)` に `ON DELETE RESTRICT`。schema column を廃止する場合は `archived_at` を使う。
- `decided_by_user_id` は `tenant_users(id)` に `ON DELETE SET NULL`。
- `pipeline_id`、`version_id`、`schema_column_id`、`decided_by_user_id` は同一 `tenant_id` に属することを service layer で検証し、可能な範囲で composite FK / trigger でも守る。
- 重複防止は `(tenant_id, pipeline_id, version_id, source_column, schema_column_id, decision)` の unique index を基本にする。`version_id IS NULL` の履歴は別 partial unique index で扱う。

Implemented in `0045_schema_mapping_candidates.up.sql` with private / tenant shared scope columns. The current API records private evidence from the schema mapping inspector and counts accepted/rejected evidence by tenant, schema column, and optional pipeline scope. Tenant admins can explicitly promote an example to `shared_scope='tenant'`; only private evidence from the current pipeline and tenant-shared evidence are counted by the candidate API.

Index rebuild はこの 2 table から `local_search_documents` の `schema_column` / `mapping_example` resource を再生成する。現在は `schema_column` と `mapping_example` の bulk rebuild service method、tenant admin API、Tenant Admin Data 画面からの rebuild 起動導線まで実装済み。新規/更新された `mapping_example` は記録時にも `local_search_documents` へ同期する。

### Index 対象

`local_search_documents` に次の resource を追加する。

#### `schema_column`

標準スキーマ項目を表す。

Index text:

```text
target_column
description
aliases
examples
domain
schema_type
```

Metadata:

- `domain`
- `schemaType`
- `targetColumn`
- `language`
- `version`

#### `mapping_example`

過去に採用/却下された mapping 例を表す。

Index text:

```text
source_column
sheet_name
sample_values
neighbor_columns
target_column
decision
```

Metadata:

- `domain`
- `schemaType`
- `targetColumn`
- `sourceColumn`
- `accepted`
- `pipelinePublicId`
- `versionPublicId`

採用済み mapping は ranking で加点する。却下済み mapping は候補抑制または negative evidence として扱う。

### API

追加 API:

```text
POST /api/v1/data-pipelines/schema-mapping/candidates
```

Request:

```json
{
  "pipelinePublicId": "...",
  "versionPublicId": "...",
  "domain": "retail",
  "schemaType": "product",
  "columns": [
    {
      "sourceColumn": "品目名称",
      "sheetName": "商品一覧",
      "sampleValues": ["緑茶 500ml", "缶コーヒー", "チョコレート"],
      "neighborColumns": ["JANコード", "メーカー名", "規格", "入数"]
    }
  ],
  "limit": 5
}
```

API flow:

1. `pipelinePublicId` / `versionPublicId` から対象 Data Pipeline を解決する。
2. tenant guard と Data Pipeline の閲覧/編集 authorization を通す。判定は既存の Data Pipeline authorization を使い、OpenFGA relation が存在する resource では `can_view` / `can_edit` を併用する。
3. `domain` / `schemaType` が対象 pipeline の許可 domain と矛盾しないことを検証する。
4. tenant-wide な `schema_column` は候補に使う。
5. `mapping_example` は同一 tenant でも呼び出し user が `can_view` できる pipeline 由来、または tenant admin が共有を許可した履歴だけ evidence に使う。
6. response に返す `mappingExamplePublicIds` は、呼び出し user が閲覧可能な履歴だけに限定する。
7. sample values / neighbor columns は raw 値を audit log と metrics label に残さない。

Current implementation:

- `POST /api/v1/data-pipelines/schema-mapping/candidates` is implemented.
- `pipelinePublicId` / `versionPublicId` are optional. Provided values are resolved and checked with existing Data Pipeline view authorization.
- `domain` / `schemaType` filter is applied to `data_pipeline_schema_columns`.
- Keyword search uses normalized `sourceColumn` against `local_search_documents` for `schema_column`. This avoids over-constraining Postgres full-text search with sample values or neighbor columns.
- If tenant vector policy is enabled and `schema_column` embeddings exist, semantic search embeds `sourceColumn` + `sheetName` + `sampleValues` + `neighborColumns`, searches pgvector, and merges vector hits with keyword hits.
- `matchMethod` is `keyword`, `vector`, or `hybrid`. Vector errors are ignored for this API so existing keyword behavior remains available during local runtime outages.
- accepted/rejected evidence counts are returned. The current public response exposes counts only, not mapping example snippets or public IDs.

追加 API:

```text
POST /api/v1/data-pipelines/schema-mapping/examples
```

Current implementation:

- records `accepted` / `rejected` decisions for a pipeline and schema column.
- requires Data Pipeline update permission.
- defaults to private evidence scoped to the pipeline.
- validates optional version belongs to the pipeline.
- upserts duplicate evidence by `(tenant_id, pipeline_id, version_id, source_column, schema_column_id, decision)` or the equivalent `version_id IS NULL` key.
- materializes the mapping example into `local_search_documents` with resource kind `mapping_example`.
- tenant admin rebuild API bulk materializes both `schema_column` and existing `mapping_example` documents. The response includes total `indexed`, `schemaColumnsIndexed`, and `mappingExamplesIndexed`.
- tenant admin sharing API switches a mapping example between `private` and `tenant`. Demoting back to `private` clears `shared_by_user_id` and `shared_at`.
- tenant admin list API returns mapping examples with pipeline/source/target/decision/shared-scope/materialized status for management UI.
- Tenant Admin Data screen lists schema mapping evidence, supports search/filter, promote/demote actions, and triggers schema mapping search document rebuild.
- Tenant Admin Data screen also lists local search index jobs and can enqueue a full local search rebuild through the existing Drive governance admin API.

Response:

```json
{
  "items": [
    {
      "sourceColumn": "品目名称",
      "candidates": [
        {
          "targetColumn": "product_name",
          "score": 0.91,
          "matchMethod": "hybrid",
          "reason": "カラム名、サンプル値、周辺カラムが商品名に近い",
          "evidence": {
            "schemaColumnPublicId": "...",
            "mappingExamplePublicIds": ["..."]
          }
        }
      ]
    }
  ]
}
```

Ranking:

1. 厳密ルールで JAN/SKU/date/price/id 系を先に判定する。
2. domain / schemaType / tenant で filter する。
3. full-text rank と vector similarity を取る。
4. accepted mapping history を加点する。
5. rejected mapping history を減点する。
6. score threshold 未満は自動適用せず UI confirmation に回す。

Current backend ranking:

- `sourceColumn` keyword match over indexed `schema_column` text.
- Optional pgvector cosine search over indexed `schema_column` embeddings. Semantic query text includes source column, sheet name, preview sample values, and neighbor columns.
- Keyword and vector candidates are merged by schema column ID.
- strict hints for invoice/vendor/due-date style mappings.
- accepted evidence adds score.
- rejected evidence subtracts score.
- `matchMethod` is `keyword`, `vector`, or `hybrid`.
- No automatic mapping application.

Smoke result on 2026-05-07:

- Input columns: `請求No`, `支払先`, `税込合計`, `社内メモ`.
- Seeded schema columns: `invoice_number`, `vendor_name`, `total_amount`.
- Result: first three inputs returned expected candidates; `社内メモ` returned no candidates.
- Negative checks: empty `columns` and missing `X-CSRF-Token` returned `422`.

### UI

`DataPipelineDetailView` の `schema_mapping` inspector に追加する。

- 「候補を取得」action
- source column ごとの candidate list
- score、match method、evidence 表示
- candidate を mapping config に適用
- 低 confidence は warning 表示

自動確定は v1 ではしない。

## Phase 4. Drive Semantic / Hybrid Search

Status: implemented for API mode plumbing, generated frontend types, keyword compatibility, semantic `drive_file` vector search, hybrid merge, low-score semantic hit filtering, OpenFGA file filtering, and Drive search mode UI. Real embedding model accuracy/latency evaluation remains.

既存の `GET /api/v1/drive/search/documents` を拡張する。

Query:

- `mode=keyword|semantic|hybrid`
- default は `keyword`
- `hybrid` は full-text と vector を併用する

Flow:

1. query を normalize する。
2. `keyword` は `local_search_documents` の full-text / title / body match から candidate file IDs を取得する。
3. `semantic` は tenant policy の `localSearch.vectorEnabled` と local embedding runtime が有効な場合に query を embed し、`local_search_embeddings` から `resource_kind='drive_file'` の candidate file IDs を取得する。
4. `hybrid` は keyword candidates と semantic candidates を dedupe して merge する。semantic runtime error は `semantic` mode では error とし、`hybrid` mode では keyword result を維持する。
5. semantic candidate は `localSearchMinSemanticScore=0.05` 未満を除外する。pgvector は `ORDER BY embedding <=> query LIMIT n` だけだと score 0 近傍の unrelated rows も返せるため、候補数上限とは別に下限 score を持つ。
6. DB guard:
   - tenant
   - purpose
   - deleted
   - scan_status
   - dlp_blocked
   - workspace
7. OpenFGA `can_view` filter を通す。
8. permission-filter 後の snippet / match metadata だけ返す。

Index には `drive_file`、`ocr_run`、`product_extraction`、`gold_table` を含める。

`drive_search_documents` は既存互換のため残すが、新しい semantic/hybrid flow は `local_search_documents` に寄せる。

## Phase 5. Permission-filtered RAG

追加 API:

```text
POST /api/v1/drive/rag/query
```

Request:

```json
{
  "query": "この請求書の支払期限と合計金額は？",
  "workspacePublicId": "...",
  "mode": "hybrid",
  "limit": 8
}
```

Response:

```json
{
  "answer": "...",
  "citations": [
    {
      "resourceKind": "ocr_run",
      "resourcePublicId": "...",
      "filePublicId": "...",
      "snippet": "...",
      "score": 0.88
    }
  ],
  "matches": []
}
```

RAG flow:

1. hybrid search で chunks を取得する。
2. DB guard と OpenFGA filter を通す。
3. DLP blocked / scan unsafe は除外する。
4. context budget に合わせて snippet を切る。
5. LLM provider に渡す。
6. answer は citation 必須にする。
7. citation がない主張は生成しない。

v1 の generation provider は local runtime のみ許可する。remote LLM provider は data handling policy と audit redaction が固まるまで扱わない。

Citation guard:

- LLM には structured JSON output を要求し、`answer` と `claims[]` を返させる。
- 各 claim は検索済み context chunk の `citationId` を 1 つ以上参照する。
- backend は `citationId` が今回 permission-filter を通った context に存在することを検証する。
- citation のない claim、存在しない citation、または context と明らかに対応しない citation は response から除外する。
- 除外後に answer が空、または citation coverage が閾値未満なら retry する。retry 後も満たせない場合は no-citation guard として回答を block し、参照可能な match だけ返す。

RAG は embedding runtime とは別の tenant policy として追加する。

Policy fields:

- `ragEnabled`
- `generationRuntime` (`none`, `ollama`, `lmstudio`)
- `generationRuntimeURL`
- `generationModel`
- `maxContextChunks`
- `maxContextRunes`

`ragEnabled=true` の場合、`generationRuntime != none`、`generationModel` 必須、runtime URL は localhost / 127.0.0.1 のみ許可する。`localSearch.vectorEnabled=false` の tenant では RAG を有効化できない。

## Security / Privacy

- Vector DB は派生 index とし、正式な保存先にしない。
- index に raw storage key、share token、password hash、KMS secret、provider credential を保存しない。
- audit log に raw query、raw prompt、raw context を保存しない。
- metrics label に tenant ID、user ID、public ID、query、file name を入れない。
- response 前に DB tenant guard と OpenFGA filter を必ず通す。
- snippet は permission-filter 後の resource だけ返す。
- zero-knowledge / encrypted file mode では server-side semantic search と RAG を無効化する。

## Metrics

追加 metric:

- embedding job duration
- embedding job completed / failed / skipped count
- embedding provider latency
- vector search latency
- vector search candidate count
- permission-filter denied count
- RAG no-citation answer blocked count
- index lag seconds: `source_updated_at` がある resource は `indexed_at - source_updated_at`、ない resource は embedding job `completed_at - created_at`

## Evaluation

Schema mapping は評価 dataset を作る。

目安:

- 100-500 件の source column example
- 正解 target column
- domain / schemaType
- sample values
- neighbor columns

測定:

- top-1 accuracy
- top-3 accuracy
- top-5 accuracy
- 誤候補率
- 人間が修正した割合
- accepted mapping reuse rate

Evaluation dataset は schema mapping candidate search を local development で有効化する前に作成する。最低限、代表 domain 1 つで 100 件以上の labeled source column を用意し、MVP の acceptance gate を top-3 accuracy と誤候補率で定義する。現在は `samples/evaluation/schema-mapping-invoice-ja.json` を 100 cases まで拡張済み。LM Studio `text-embedding-mxbai-embed-large-v1`、`HAOHAO_EVAL_SCHEMA_SCORE_THRESHOLD=0.86`、`HAOHAO_EVAL_SCHEMA_KEYWORD_WEIGHT=0.8` で broad gate を通過済み。

Drive/RAG は次を測る。

- keyword only と hybrid の検索成功率
- permission-filter 後の result count
- citation coverage
- hallucination review rate
- p50 / p95 latency

このセッションの検証で、fake embedding runtime でも retrieval の shape は確認できた。現在は `samples/evaluation/drive-rag-retrieval-ja.json` に、請求書、発注書、契約、経費、営業 note、forbidden / DLP blocked document を含む 16 documents / 52 queries を置いている。`scripts/evaluate-vector-rag-retrieval.mjs` はこの dataset と schema mapping dataset を `fake` / Ollama / LM Studio embedding provider で評価でき、citation coverage に加えて expected answer facts が取得 context に含まれるかも測る。LM Studio `text-embedding-mxbai-embed-large-v1`、`HAOHAO_EVAL_DRIVE_SCORE_THRESHOLD=0.75`、top-k `5` では Drive/RAG retrieval broad gate を通過済み。実 API path では model score scale が異なったため、Drive file search は `0.51` threshold と query expansion で smoke 通過済み。RAG 実 API smoke も `text-embedding-mxbai-embed-large-v1` + `qwen/qwen3.5-9b` で通過済み。

## Test Plan

Current verification:

- `make gen`: passed.
- `go test ./backend/...`: passed.
- `npm run build`: passed.
- One-off pgvector container on port `55433`: verified `vector` extension availability.
- Migration chain on fresh verification DB: `up`, `down 1`, `up 1` passed for version 44.
- Local DB: 0045 applied; `schema_migrations.version=45`, `dirty=false`, `vector` extension installed.
- Scenario verification: inserted realistic invoice schema mapping documents and synthetic 1024-dimensional embeddings. Query corresponding to `請求No / invoice number` returned `請求書番号 / invoice_no` as top `schema_column` hit, and tenant filter prevented another tenant's similar vector from appearing.
- HTTP API smoke: inserted realistic invoice schema columns for a dedicated `invoice_smoke_20260507` domain, indexed them into `local_search_documents`, logged in as `demo@example.com`, and called `POST /api/v1/data-pipelines/schema-mapping/candidates`. `請求No -> invoice_number`, `支払先 -> vendor_name`, `税込合計 -> total_amount`, and `社内メモ -> no candidates` were confirmed. The smoke seed rows were removed after verification.
- HTTP API negative smoke: `columns=[]` and missing `X-CSRF-Token` returned `422`.
- After the HTTP smoke, `go test ./backend/...` passed again.
- Seed/rebuild smoke: `scripts/seed-schema-mapping-columns.mjs` inserted 10 invoice schema columns for tenant `acme`; `POST /api/v1/admin/tenants/acme/data-pipelines/schema-mapping/search-documents/rebuild` materialized 10 `schema_column` documents; schema mapping candidate API then returned `請求No -> invoice_number`, `支払先 -> vendor_name`, `税込合計 -> total_amount`, and no candidate for `社内メモ`.
- Frontend build after inspector UI and preview sample values integration: `npm run build` passed with existing chunk-size warnings.
- Backend verification after pgvector/hybrid ranking connection: `go test ./backend/internal/service` and `go test ./backend/...` passed.
- Mapping example capture verification: `make gen`, `go test ./backend/internal/service`, `go test ./backend/...`, and `npm run build` passed. Local Postgres parse validation for both mapping example upsert shapes reached FK validation, confirming the `ON CONFLICT` targets match existing partial unique indexes.
- Mapping example HTTP smoke on 2026-05-07:
  - Started the backend with local password login enabled and confirmed `GET /api/v1/auth/settings` returned `200`.
  - Logged in as `demo@example.com`, selected tenant `acme`, created a temporary Data Pipeline, seeded 10 invoice schema columns, and rebuilt `schema_column` search documents.
  - Called `POST /api/v1/data-pipelines/schema-mapping/candidates` for source column `請求No`; `invoice_number` was returned with `acceptedEvidence=0`.
  - Called `POST /api/v1/data-pipelines/schema-mapping/examples` with `decision=accepted` for `請求No -> invoice_number`; response returned the new mapping example public ID.
  - Called candidates again and confirmed `acceptedEvidence=1` and score increased from `1.800000011920929` to `1.880000011920929`.
  - Queried Postgres and confirmed the row is `shared_scope='private'` and has a materialized `local_search_documents` row with `resource_kind='mapping_example'` and title `請求No -> invoice_number`.
  - Re-sent the same example and confirmed the duplicate key row count stayed `1`.
  - Confirmed missing CSRF and `decision='maybe'` both return `422`.
  - Deleted the temporary pipeline, mapping example, and `mapping_example` local search document after verification. The invoice schema seed remains for repeatable local testing.
- Bulk mapping example rebuild verification: `make gen`, `go test ./backend/internal/service`, `go test ./backend/internal/api`, `go test ./backend/...`, and `npm run build` passed. The new `ListDataPipelineMappingExamplesForIndex` SQL was executed against local Postgres for tenant `acme`; it returned 0 rows after smoke cleanup but validated the query shape against the live schema.
- Tenant-shared evidence API verification:
  - `make gen`, `go test ./backend/internal/service`, `go test ./backend/internal/api`, `go test ./backend/...`, and `npm run build` passed.
  - The new get / update sharing SQL was executed against local Postgres for tenant `acme`; it returned 0 rows after prior smoke cleanup but validated the query shape against the live schema.
  - Started the backend with local password login enabled, logged in as `demo@example.com`, selected tenant `acme`, and created two temporary Data Pipelines: one source pipeline and one consumer pipeline.
  - Seeded 10 invoice schema columns and rebuilt schema mapping search documents. The rebuild response returned `indexed=10`, `schemaColumnsIndexed=10`, `mappingExamplesIndexed=0`.
  - Candidate search from the consumer pipeline for source column `請求No` returned `invoice_number` with `acceptedEvidence=0`.
  - Recorded an accepted mapping example in the source pipeline for `請求No -> invoice_number`; the response returned `sharedScope=private`.
  - Candidate search from the consumer pipeline still returned `acceptedEvidence=0`, confirming private evidence from another pipeline is not reused cross-pipeline.
  - Promoted the example through `PATCH /api/v1/admin/tenants/acme/data-pipelines/schema-mapping/examples/{mappingExamplePublicId}/sharing` with `{"sharedScope":"tenant"}`. Candidate search from the consumer pipeline then returned `acceptedEvidence=1` and score increased from `1.800000011920929` to `1.880000011920929`.
  - Demoted the same example with `{"sharedScope":"private"}`. Candidate search from the consumer pipeline returned `acceptedEvidence=0` and score returned to `1.800000011920929`.
  - Invalid `{"sharedScope":"workspace"}` returned `422`.
  - Postgres confirmed demoted state as `private|false|false` for `shared_scope`, `shared_by_user_id IS NOT NULL`, and `shared_at IS NOT NULL`.
  - Deleted the temporary source pipeline, consumer pipeline, mapping example, and materialized `mapping_example` local search document after verification.
- Tenant admin evidence management UI verification: `make gen`, `go test ./backend/internal/service ./backend/internal/api`, `go test ./backend/...`, and `npm --prefix frontend run build` passed.
- Tenant admin evidence management UI browser smoke:
  - Started a temporary local-login backend on port 8080 so the existing Vite dev server proxy could authenticate as `demo@example.com`.
  - Created temporary invoice evidence `請求No -> invoice_number` and confirmed `GET /api/v1/admin/tenants/acme/data-pipelines/schema-mapping/examples` listed it.
  - Opened `/tenant-admin/acme/data` in agent-browser and confirmed the `Schema Mapping Evidence` table showed source column, target column, pipeline, decision, sharing scope, materialized status, and actions.
  - Confirmed search filter for `invoice_number` keeps the row.
  - Promoted the evidence to tenant shared and demoted it back to private through the UI event handlers; PATCH requests returned 200 and the table updated between `TENANT` and `PRIVATE`. The smoke used DOM event dispatch because agent-browser's normal click did not reliably trigger the table action buttons.
  - Confirmed `sharedScope=tenant` filter excludes the row after demotion.
  - Triggered schema mapping search document rebuild from the UI; POST returned 200 and the page displayed `Indexed 11: schema columns 10, mapping examples 1`.
  - Fixed one UI issue found during smoke: after promote/demote, the table now reloads from the backend so active filters are respected.
  - Deleted the temporary pipeline, mapping example, and materialized `mapping_example` document after verification.
  - Stopped the temporary local-login backend after verification and confirmed ports 8080 / 18080 were left without backend listeners. The existing Vite dev server on 5173 was left running.
- Tenant admin evidence management UI pointer-click regression:
  - Added `e2e/schema-mapping-evidence.spec.ts`.
  - The regression uses real Playwright clicks for `Rebuild local search`, `Share`, `Make private`, and `Rebuild search docs` while route-mocking only the local search job and schema mapping evidence admin APIs. Login, tenant selection, routing, rendering, and button interaction use the normal frontend path.
  - `E2E_BASE_URL=http://127.0.0.1:5174 npm --prefix frontend run e2e -- ../e2e/schema-mapping-evidence.spec.ts` passed against a temporary local-login backend on 18080 and a Vite dev server on 5174.
  - `frontend/vite.config.ts` now supports `VITE_API_PROXY_TARGET`, defaulting to `http://127.0.0.1:8080`, so temporary frontend instances can point at alternate local backends without stopping the user's 8080 server.
  - The temporary API-created pipeline/evidence/local search document used during agent-browser investigation was cleaned up after verification.
- Tenant admin local search rebuild/status UI verification:
  - Added a Local Search Index panel to `/tenant-admin/:tenantSlug/data`.
  - The panel loads `GET /api/v1/admin/tenants/{tenantSlug}/drive/search/local-index/jobs`, shows the latest jobs, and calls `POST /api/v1/admin/tenants/{tenantSlug}/drive/search/local-index/rebuilds` to enqueue a full rebuild.
  - `npm --prefix frontend run build` passed with the existing large chunk warnings.
  - The targeted Playwright regression passed on 2026-05-07 after starting a temporary local-login backend on 18080 and Vite on 5174. An attempted run against the existing 5173/8080 stack failed because the 8080 backend was in Zitadel/OIDC mode and did not expose the local password login form expected by the E2E helper.
- Drive search mode API / UI verification:
  - Added `mode=keyword|semantic|hybrid` to `GET /api/v1/drive/search/documents`.
  - Regenerated OpenAPI and frontend generated types; `SearchDriveDocumentsData.query.mode` is typed as `keyword | semantic | hybrid`.
  - Added unit coverage for Drive search mode normalization and semantic match dedupe/prepend behavior.
  - `go test ./backend/internal/service`, `go test ./backend/...`, and `npm --prefix frontend run build` passed.
  - End-to-end Drive smoke passed with a temporary local-login backend on 18080 and a fake Ollama-compatible embedding runtime on 18081. The smoke temporarily enabled `acme` local search vector policy, uploaded a realistic invoice text containing `振込期限: 2026-06-30` plus an unrelated weekly sales note, rebuilt local search, and verified:
    - `mode=keyword&q=支払期限` returned no result because the exact term was absent.
    - `mode=semantic&q=支払期限` returned only the invoice file and excluded the unrelated note.
    - `mode=hybrid&q=支払期限` returned only the invoice file.
    - default mode remained keyword-compatible and returned the invoice for `q=振込期限`.
  - The smoke exposed zero-score semantic noise from low-similarity pgvector rows. `SearchSemantic` now filters hits below `localSearchMinSemanticScore=0.05`; `go test ./backend/internal/service` and `go test ./backend/...` passed after the fix.
  - The smoke restored tenant settings after completion, soft-deleted temporary uploaded files, and stopped the temporary backend / embedding runtime. Real embedding model accuracy and latency evaluation is covered by the in-memory evaluation harness below; LM Studio-backed real API / real pgvector index smoke was completed afterward with `text-embedding-mxbai-embed-large-v1`.
- Evaluation dataset starter verification:
  - Added `samples/evaluation/schema-mapping-invoice-ja.json` with 30 Japanese invoice schema mapping cases, including 4 no-candidate negative cases and acceptance gates for top-1 / top-3 / false positive rate.
  - Added `samples/evaluation/drive-rag-retrieval-ja.json` with 8 Drive-style documents and 12 retrieval/RAG queries, including forbidden and DLP-blocked no-answer cases.
  - Added `scripts/validate-vector-rag-evaluation-datasets.mjs` and `make validate-vector-rag-eval`.
  - `node scripts/validate-vector-rag-evaluation-datasets.mjs` passed and reported schema mapping `30` cases / `4` no-candidate cases / `10` target columns, and Drive/RAG `8` documents / `12` queries / `2` denied queries.
- Evaluation harness verification:
  - Added `scripts/evaluate-vector-rag-retrieval.mjs` and `make eval-vector-rag-retrieval`.
  - The harness supports `HAOHAO_EVAL_EMBEDDING_RUNTIME=fake|ollama|lmstudio`, `HAOHAO_EVAL_EMBEDDING_MODEL`, `HAOHAO_EVAL_EMBEDDING_URL`, `HAOHAO_EVAL_SCORE_THRESHOLD`, `HAOHAO_EVAL_SCHEMA_SCORE_THRESHOLD`, `HAOHAO_EVAL_DRIVE_SCORE_THRESHOLD`, `HAOHAO_EVAL_TOP_K`, and `HAOHAO_EVAL_ENFORCE_GATES=1`.
  - Default fake runtime run passed as a script smoke: `make eval-vector-rag-retrieval` exited `0`, calculated schema mapping top-1 / top-3 / no-candidate metrics, Drive semantic / hybrid recall, forbidden exclusion, citation coverage, latency, and emitted failure samples. Fake runtime is not an accuracy substitute, so gates are reported but not enforced by default.
  - LM Studio real embedding run on `192.168.1.28:1234` with `text-embedding-mxbai-embed-large-v1` passed with gate enforcement after tuning thresholds separately for schema mapping and Drive retrieval:
    - command: `HAOHAO_EVAL_SCHEMA_SCORE_THRESHOLD=0.78 HAOHAO_EVAL_DRIVE_SCORE_THRESHOLD=0.75 HAOHAO_EVAL_EMBEDDING_RUNTIME=lmstudio HAOHAO_EVAL_EMBEDDING_URL=http://192.168.1.28:1234 HAOHAO_EVAL_EMBEDDING_MODEL=text-embedding-mxbai-embed-large-v1 HAOHAO_EVAL_ENFORCE_GATES=1 make eval-vector-rag-retrieval`
    - schema mapping: top-1 `0.8077`, top-3 `1.0`, no-candidate precision `1.0`, false positive rate `0`.
    - Drive/RAG retrieval: semantic recall@5 `1.0`, hybrid recall@5 `1.0`, forbidden document exclusion `1.0`, citation coverage `1.0`, no-citation answer rate `0`.
    - observed latency in the gate-enforced run: schema mapping embedding/evaluation `907ms`, Drive/RAG embedding/evaluation `282ms`.
  - Broad evaluation dataset run on `192.168.1.28:1234` with `text-embedding-mxbai-embed-large-v1` passed with gate enforcement after expanding datasets:
    - command: `HAOHAO_EVAL_SCHEMA_SCORE_THRESHOLD=0.86 HAOHAO_EVAL_DRIVE_SCORE_THRESHOLD=0.75 HAOHAO_EVAL_EMBEDDING_RUNTIME=lmstudio HAOHAO_EVAL_EMBEDDING_URL=http://192.168.1.28:1234 HAOHAO_EVAL_EMBEDDING_MODEL=text-embedding-mxbai-embed-large-v1 HAOHAO_EVAL_ENFORCE_GATES=1 make eval-vector-rag-retrieval`
    - schema mapping: 100 cases, top-1 `0.8023`, top-3 `0.9884`, no-candidate precision `0.9286`, false positive rate `0.01`.
    - Drive/RAG retrieval: 16 documents / 52 queries, semantic recall@5 `1.0`, hybrid recall@5 `1.0`, forbidden document exclusion `1.0`, citation coverage `1.0`, answer fact coverage `1.0`, no-citation answer rate `0`.
    - observed latency in the gate-enforced run: schema mapping embedding/evaluation `1761ms`, Drive/RAG embedding/evaluation `985ms`.
  - LM Studio `ruri-v3-310m` 指定でも一度は gate enforcement run が通過した:
    - command: `HAOHAO_EVAL_SCHEMA_SCORE_THRESHOLD=0.78 HAOHAO_EVAL_DRIVE_SCORE_THRESHOLD=0.75 HAOHAO_EVAL_EMBEDDING_RUNTIME=lmstudio HAOHAO_EVAL_EMBEDDING_URL=http://127.0.0.1:11234 HAOHAO_EVAL_EMBEDDING_MODEL=ruri-v3-310m HAOHAO_EVAL_ENFORCE_GATES=1 make eval-vector-rag-retrieval`
    - schema mapping: top-1 `0.8077`, top-3 `1.0`, no-candidate precision `1.0`, false positive rate `0`.
    - Drive/RAG retrieval: semantic recall@5 `1.0`, hybrid recall@5 `1.0`, forbidden document exclusion `1.0`, citation coverage `1.0`, no-citation answer rate `0`.
    - observed latency in the gate-enforced run: schema mapping embedding/evaluation `897ms`, Drive/RAG embedding/evaluation `271ms`.
    - caveat: direct LM Studio embeddings request with `model='ruri-v3-310m'` returned response metadata `model='text-embedding-mxbai-embed-large-v1'` and 1024 dimensions. Later attempts against `/v1/embeddings` returned `No models loaded`, and the local CLI load flow did not expose a stable `ruri-v3-310m` model key. Treat this as an abandoned experiment, not as an accepted project standard.
  - Real API / real pgvector smoke with `HAOHAO_EVAL_EMBEDDING_MODEL=ruri-v3-310m` did not pass the stricter semantic-quality check:
    - tenant policy was enabled temporarily through `features.drive.localSearch`; new Drive files produced completed `local_search_embeddings` rows with `model='ruri-v3-310m'` and dimension `1024`.
    - `mode=semantic&q=支払期限` returned the expected invoice file but also returned an unrelated product weekly report, with the report ranked above the invoice.
    - cleanup restored tenant settings and soft-deleted the temporary files.
  - After adding Drive-specific query expansion and resource-specific semantic thresholds, the real API / real pgvector smoke passed in the ruri experiment, but the project standard is now `text-embedding-mxbai-embed-large-v1`:
    - command: `HAOHAO_SMOKE_BASE_URL=http://127.0.0.1:18080 HAOHAO_LMSTUDIO_PROXY_URL=http://127.0.0.1:11234 HAOHAO_EVAL_EMBEDDING_MODEL=ruri-v3-310m HAOHAO_SMOKE_POLL_ATTEMPTS=80 make smoke-lmstudio-vector-api`
    - checks: keyword synonym miss `true`, semantic finds invoice `true`, semantic excludes unrelated report `true`, hybrid finds invoice `true`, default keyword exact-term compatibility `true`.
    - result names: `semanticNames=["haohao-lmstudio-vector-invoice-1778153124236.txt"]`, `hybridNames=["haohao-lmstudio-vector-invoice-1778153124236.txt"]`, `keywordNames=[]`.
    - cleanup restored tenant settings and soft-deleted the temporary files.
  - RAG API verification:
    - Added `features.drive.rag`, local `ollama|lmstudio` generation provider, `DriveService.QueryRAG`, and `POST /api/v1/drive/rag/query`.
    - Added structured-output citation validation. Claims that do not cite the current permission-filtered context are removed; responses with no valid citations are blocked.
    - `go test ./backend/internal/service`, `go test ./backend/internal/api`, `go test ./backend/...`, and `npm --prefix frontend run build` passed after the RAG API implementation.
    - Real API smoke passed with `text-embedding-mxbai-embed-large-v1` for embeddings and `qwen/qwen3.5-9b` for generation:
      - command: `HAOHAO_SMOKE_BASE_URL=http://127.0.0.1:18080 HAOHAO_LMSTUDIO_PROXY_URL=http://127.0.0.1:11234 HAOHAO_EVAL_EMBEDDING_MODEL=text-embedding-mxbai-embed-large-v1 HAOHAO_RAG_GENERATION_MODEL=qwen/qwen3.5-9b HAOHAO_SMOKE_POLL_ATTEMPTS=80 make smoke-lmstudio-drive-rag`
      - answer: `支払期限は2026年6月30日、税込合計は128,000円です。`
      - checks: answer not blocked, answer text present, citation points to the uploaded invoice, deadline present, total amount present.
      - cleanup restored tenant settings and soft-deleted temporary files.
  - During tuning, schema mapping ranking in the harness gained a lexical signal to better match the implemented keyword/vector hybrid candidate flow. A single Japanese keyword query in the Drive/RAG dataset was changed from `semantic` to `hybrid`, and the harness keyword scorer gained CJK n-gram tokenization so Japanese keyword checks are not treated as one opaque token.
  - Broad RAG smoke verification:
    - Added `HAOHAO_SMOKE_RAG_BROAD=1` to `scripts/smoke-lmstudio-drive-rag.mjs`.
    - Added `scripts/openai-compatible-proxy.mjs` so real LM Studio on `192.168.1.28:1234` can be reached through a localhost URL accepted by tenant settings validation.
    - Added RAG-specific retrieval reranking in `DriveService.QueryRAG`: search candidates are widened, then file name / snippet / match lexical overlap promotes query-specific documents before context construction.
    - Added unit coverage for the reranker and hyphenated identifiers such as `TP-2026-0412`.
    - Fixed the broad smoke script so RAG execution uses the same marker query as readiness polling.
    - `go test ./backend/internal/service`, `node --check scripts/smoke-lmstudio-drive-rag.mjs`, and `git diff --check` passed after these changes.
    - LM Studio-backed 2-query broad smoke reached the RAG endpoint and proved the 青葉商事 invoice path end to end: expected citation hit `true`, expected fact pass `1/1`, answer included `2026-06-30`.
    - The remaining `TP-2026-0412` broad smoke miss is tracked as evaluation/infrastructure tuning, not as a Phase 5 implementation blocker. In practice, broad smoke depends heavily on LM Studio embedding latency and local outbox settings; `OUTBOX_WORKER_BATCH_SIZE=1` and a longer `OUTBOX_WORKER_TIMEOUT` improved indexing stability but do not make the full broad gate a required per-change check.

Backend unit:

- chunking
- content hash
- dimension validation
- fake embedding provider
- pgvector query scoring
- schema mapping strict rule
- accepted/rejected mapping ranking

DB/sqlc:

- migration up/down
- embedding upsert
- document delete cascade
- tenant/resource/model filtered vector search
- stale embedding rebuild target query

Service/API:

- `HandleEmbeddingRequested`
- `RequestRebuild`
- schema mapping candidate API: keyword MVP implemented and smoke tested
- schema mapping example capture API: private accepted evidence save, duplicate upsert, local search document materialization, and negative validation smoke tested
- schema mapping search document rebuild API: bulk `schema_column` and `mapping_example` materialization implemented; response exposes total and per-resource counts
- schema mapping tenant-shared evidence API: tenant admin can promote/demote evidence between `private` and `tenant`; HTTP smoke confirmed cross-pipeline ranking only uses tenant-shared evidence
- schema mapping tenant admin evidence list API: list/search/filter mapping examples with materialized status for UI
- schema mapping candidate API OpenFGA / authorization filtering: pipeline/version view checks implemented; candidate API counts only current-pipeline private evidence plus tenant-shared evidence and does not expose row-level evidence payloads
- Drive semantic/hybrid search: `mode` API, generated frontend types, keyword default, semantic `drive_file` vector path, hybrid merge, and OpenFGA file filtering implemented
- RAG no-citation guard: implemented for Drive RAG structured output
- OpenFGA denied result exclusion
- DLP blocked exclusion

Frontend:

- schema mapping candidate display and apply
- schema mapping candidate apply records accepted private mapping evidence when `pipelinePublicId` is available
- tenant admin schema mapping evidence management UI
- tenant admin local search rebuild/status UI
- Drive search mode toggle for keyword / semantic / hybrid
- RAG answer panel with citations: Drive search view に query form、blocked / answered state、citation list、retrieved matches details を追加済み
- tenant admin RAG policy controls: Drive Policy 画面に `features.drive.rag` の enable、generation runtime、runtime URL、model、context budget controls を追加済み
- tenant settings validation update for fixed `dimension=1024`

Commands:

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
make validate-vector-rag-eval
make eval-vector-rag-retrieval
make smoke-lmstudio-drive-rag
HAOHAO_SMOKE_RAG_BROAD=1 HAOHAO_LMSTUDIO_PROXY_URL=http://192.168.1.28:1234 make smoke-lmstudio-drive-rag
HAOHAO_OPENAI_PROXY_TARGET=http://192.168.1.28:1234 node scripts/openai-compatible-proxy.mjs
HAOHAO_SMOKE_RAG_BROAD=1 HAOHAO_LMSTUDIO_PROXY_URL=http://127.0.0.1:11234 make smoke-lmstudio-drive-rag
E2E_BASE_URL=http://127.0.0.1:5174 npm --prefix frontend run e2e -- ../e2e/schema-mapping-evidence.spec.ts
RUN_DRIVE_SEARCH_E2E=1 make e2e
```

Note: this repository currently does not have a `make check-generated` target.

## Rollout

1. Done: confirm pgvector availability locally and update local compose image to `pgvector/pgvector:0.8.2-pg18-trixie`.
2. Done: add pgvector migration and disabled-by-default code paths.
3. Done: tenant setting validation for fixed `dimension=1024`. `fake` remains test-only and is not accepted as a tenant `embeddingRuntime`.
4. Done: add local runtime embedding job implementation for Ollama / LM Studio and pgvector upsert.
5. Done: implement schema mapping candidate source tables and keyword API MVP.
6. Done: regenerate sqlc / OpenAPI / frontend generated types for schema mapping candidate API.
7. Done: create starter schema mapping and Drive/RAG retrieval evaluation datasets with local MVP acceptance gates.
8. Done: add invoice schema column seed/import path and tenant admin rebuild trigger so `data_pipeline_schema_columns` materializes into `local_search_documents`.
9. Done: implement first schema mapping candidate UI in the `schema_mapping` inspector.
10. Done: connect preview rows / sample values to schema mapping candidate requests so ranking can use data content, not only source column names.
11. Done: connect schema mapping candidate ranking to pgvector/hybrid search and use sample values / neighbor columns without over-constraining keyword search.
12. Done: implement private mapping example capture and per-example `mapping_example` document materialization.
13. Done: implement bulk `mapping_example` rebuild.
14. Done: implement explicit tenant-shared evidence backend flow.
15. Done: add tenant admin schema mapping evidence management UI.
16. Done: browser-smoke the new tenant admin evidence management UI against local backend.
17. Done: add a Playwright regression for tenant admin evidence table pointer clicks. It covers Share, Make private, Rebuild search docs, and post-update state refresh.
18. Done: add `docs/RUNBOOK_VECTOR_SEARCH_RAG.md` for production migration rollout, pgvector readiness, HNSW initial build, concurrent index handling, seed/rebuild, embedding backfill, tenant enablement, observability, and rollback.
19. Done: add tenant admin local search rebuild / vector status UI on the Tenant Admin Data screen.
20. Done: update Drive search `mode` API and generated frontend types for `keyword` / `semantic` / `hybrid`.
21. Done: enable Drive semantic/hybrid search behind tenant vector policy and local embedding runtime.
22. Done: add an evaluation harness for schema mapping and Drive/RAG retrieval datasets with fake / Ollama / LM Studio embedding providers.
23. Done: run the evaluation harness against LM Studio `text-embedding-mxbai-embed-large-v1` and confirm starter thresholds: schema `0.78`, Drive `0.75`, top-k `5`. `ruri-v3-310m` was attempted but abandoned because LM Studio did not provide a stable embeddings endpoint/model key for it.
24. Done: run a real API smoke with LM Studio-backed tenant vector policy and actual local search / pgvector index. It verified indexing but initially failed semantic quality because Drive API admitted low-score / misranked semantic hits.
25. Done: implement resource-specific semantic score thresholds and Drive business-term query expansion in `LocalSearchService.SearchSemantic`; rerun real API smoke and confirm semantic/hybrid Drive search passes.
26. Done: enable RAG only for explicit tenant policy and local model runtime.
27. Done: implement Drive RAG API with structured-output citation guard and run a realistic invoice smoke with `text-embedding-mxbai-embed-large-v1` + `qwen/qwen3.5-9b`.
28. Done: expand evaluation datasets to broad rollout size: schema mapping 100 cases and Drive/RAG 52 queries with answer/citation checks; run LM Studio `text-embedding-mxbai-embed-large-v1` gate enforcement with schema threshold `0.86`, Drive threshold `0.75`, and top-k `5`.
29. Done: add frontend RAG answer panel with citations and tenant admin controls for `features.drive.rag`; `npm --prefix frontend run build` passed.
30. Done: browser-smoke the RAG answer panel and tenant admin RAG policy controls with agent-browser against a current-source backend and Vite dev server. The Drive RAG panel displayed, submitted to the RAG endpoint, surfaced policy-denied errors, and the tenant admin Drive Policy form exposed and reflected `features.drive.rag` controls.
31. Done: add real backend / pgvector / LM Studio broad RAG smoke beyond the starter invoice scenario and use it to tune the implementation. The script-side broad mode uploads dataset documents, polls real local search / pgvector readiness, and measures citation coverage, answer fact coverage, no-citation block rate, denied exclusion, and p50/p95 latency. LM Studio broad runs are executable through the localhost proxy. The run exposed two actionable issues that were fixed: RAG needed wider candidate retrieval with query-specific reranking, and the smoke had to use the same marker query for readiness and RAG execution. After the fixes, unit tests and syntax checks passed and the 青葉商事 invoice broad query succeeded end to end. Remaining misses in the full broad run are classified as optional evaluation hardening around LM Studio latency / outbox indexing readiness, not as a blocker for the vector search / RAG implementation plan.

## References

- `docs/VectorDB.md`
- `docs/RUNBOOK_VECTOR_SEARCH_RAG.md`
- `docs/data-pipeline.md`
- `docs/data-pipeline-unstructured-processing.md`
- `docs/data-pipeline-draft-run-preview.md`
- `docs/TUTORIAL_OPENFGA_P8_DRIVE_PRODUCT_EXPANSION.md`
- `db/migrations/0039_local_search_layer.up.sql`
- `db/migrations/0044_local_search_pgvector.up.sql`
- `db/migrations/0045_schema_mapping_candidates.up.sql`
- `backend/internal/service/local_search_service.go`
- `backend/internal/service/data_pipeline_service.go`
- `backend/internal/service/embedding_types.go`
- `backend/internal/service/local_embedding_provider.go`
- `backend/internal/service/vector_store.go`
- `backend/internal/service/tenant_settings_service.go`
- pgvector: https://github.com/pgvector/pgvector
- pgvector-go: https://github.com/pgvector/pgvector-go
- Redis vector search docs: https://redis.io/docs/latest/develop/ai/search-and-query/vectors/
