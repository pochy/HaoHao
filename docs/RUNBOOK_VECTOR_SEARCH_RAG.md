# Vector search / RAG rollout runbook

This runbook covers the production rollout for the local search pgvector layer, schema mapping candidate search, and tenant-admin schema mapping evidence UI.

## Scope

Included:

- `0044_local_search_pgvector`
- `0045_schema_mapping_candidates`
- schema mapping search document rebuild
- embedding backfill / rebuild
- tenant-gated enablement

Not included:

- Drive semantic / hybrid search rollout
- Permission-filtered RAG rollout
- production local LLM runtime sizing

Those remain later phases.

## Preconditions

1. Take a PostgreSQL backup or confirm managed PITR can restore to a point before migration.
2. Confirm the deploy image and app commit include the generated DB, OpenAPI, and frontend artifacts for migrations 0044 / 0045.
3. Keep vector features disabled for all tenants before migration:

```sql
SELECT t.slug, s.local_search
FROM tenant_settings s
JOIN tenants t ON t.id = s.tenant_id
WHERE (s.local_search->>'vectorEnabled')::boolean IS TRUE;
```

4. Confirm migration state:

```sql
SELECT version, dirty FROM schema_migrations;
```

5. Confirm pgvector is available before running migration 0044:

```sql
SELECT name, installed_version, default_version
FROM pg_available_extensions
WHERE name = 'vector';
```

If this returns no row, install pgvector or switch to a pgvector-enabled Postgres image before continuing. Do not run 0044 until `CREATE EXTENSION vector` is expected to succeed.

## Preflight Sizing

Run the sizing queries before deciding whether the normal HNSW index in 0044 is acceptable.

```sql
SELECT count(*) AS local_search_documents FROM local_search_documents;

SELECT
  count(*) AS embeddings,
  count(*) FILTER (WHERE status = 'completed') AS completed_embeddings
FROM local_search_embeddings;

SELECT resource_kind, count(*) AS documents
FROM local_search_documents
GROUP BY resource_kind
ORDER BY documents DESC;
```

If `completed_embeddings` is small enough for the planned maintenance window, the current 0044 migration can run as written.

If `completed_embeddings` is large, do not run the current 0044 migration unchanged in production. Prepare a production migration split before release:

- schema migration: extension, constraint changes, denormalized columns, `embedding vector(1024)`, and B-tree indexes
- index migration/runbook step: HNSW created with `CREATE INDEX CONCURRENTLY`

Use only one HNSW creation path. Do not create the same HNSW index once in a regular migration and again concurrently.

## HNSW Index Step

The local migration currently creates:

```sql
CREATE INDEX local_search_embeddings_hnsw_cosine_idx
    ON local_search_embeddings USING hnsw (embedding vector_cosine_ops)
    WHERE status = 'completed' AND embedding IS NOT NULL;
```

For a populated production table, prefer the concurrent form in a separate non-transaction operation:

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS local_search_embeddings_hnsw_cosine_idx
    ON local_search_embeddings USING hnsw (embedding vector_cosine_ops)
    WHERE status = 'completed' AND embedding IS NOT NULL;
```

`CREATE INDEX CONCURRENTLY` must not run inside a transaction. If the migration runner wraps files in a transaction or cannot guarantee non-transaction execution, run this statement as an explicit DBA operation after the schema migration and before enabling vector search for tenants.

Monitor progress:

```sql
SELECT
  pid,
  phase,
  lockers_total,
  lockers_done,
  blocks_total,
  blocks_done,
  tuples_total,
  tuples_done
FROM pg_stat_progress_create_index;
```

If the concurrent index fails, drop the invalid index before retrying:

```sql
DROP INDEX CONCURRENTLY IF EXISTS local_search_embeddings_hnsw_cosine_idx;
```

## Migration Order

1. Put the app in a state where existing keyword search still works and vector features are disabled.
2. Apply migration 0044.
3. If HNSW was split out, create the HNSW index concurrently.
4. Apply migration 0045.
5. Deploy the application build that understands `schema_column` / `mapping_example` search documents.
6. Verify readiness:

```sql
SELECT extversion FROM pg_extension WHERE extname = 'vector';

SELECT
  column_name,
  udt_name
FROM information_schema.columns
WHERE table_name = 'local_search_embeddings'
  AND column_name IN ('tenant_id', 'resource_kind', 'resource_public_id', 'embedding', 'indexed_at');

SELECT indexname
FROM pg_indexes
WHERE tablename = 'local_search_embeddings'
  AND indexname IN (
    'local_search_embeddings_tenant_kind_model_status_idx',
    'local_search_embeddings_hnsw_cosine_idx'
  );
```

## Schema Mapping Seed / Rebuild

Seed schema columns per tenant only after 0045 is applied.

Local command shape:

```bash
HAOHAO_TENANT_SLUG=acme node scripts/seed-schema-mapping-columns.mjs
```

Production seed should use a reviewed seed file and an explicit tenant slug. After seeding, trigger the tenant-admin rebuild API or the Tenant Admin Data screen action.

Expected rebuild response:

```json
{
  "indexed": 10,
  "schemaColumnsIndexed": 10,
  "mappingExamplesIndexed": 0
}
```

## Embedding Backfill

Migration 0044 resets existing completed embeddings to `pending` because old placeholder arrays are not reused as pgvector values.

Backfill order:

1. Confirm tenant settings still have vector disabled.
2. Configure a local embedding runtime only in the environment intended to run embedding jobs.
3. Rebuild or enqueue `local_search.embedding_requested` events for target documents.
4. Watch job progress:

```sql
SELECT status, count(*)
FROM local_search_index_jobs
GROUP BY status
ORDER BY status;

SELECT status, count(*)
FROM local_search_embeddings
GROUP BY status
ORDER BY status;
```

Do not enable tenant vector search until completed embeddings exist for the target resource kinds.

## Tenant Enablement

Enable in this order:

1. One internal tenant.
2. One low-volume production tenant.
3. Broader schema mapping candidate usage.
4. Drive semantic / hybrid search only after its API and OpenFGA filtering phase is complete.
5. RAG only after citation guard, OpenFGA result exclusion, and DLP exclusion are implemented.

Tenant setting constraints for v1:

- `localSearch.vectorEnabled=true`
- `embeddingRuntime` is `ollama` or `lmstudio`
- `dimension=1024`
- `fake` is test-only and must not be accepted as a tenant runtime

## Local UI Smoke: Drive Semantic Search / RAG

Use this path when validating the implemented UI locally, or when the Drive search page shows `drive policy denied` after asking a RAG question.

Prerequisites:

- Local app services are running, for example via `scripts/setup-dev-env.sh`.
- The browser user can access the target tenant, for example `acme`.
- LM Studio or Ollama is already running locally.
- The embedding model returns 1024-dimensional vectors. The current standard local smoke model is `text-embedding-mxbai-embed-large-v1`.

For LM Studio-backed local validation:

1. Open `http://127.0.0.1:5173/`.
2. Log in with the local demo user, for example `demo@example.com` / `changeme123`.
3. Open `Tenant Admin > acme > Drive Policy`.
4. Enable `Local search vector`.
5. Set `Local search runtime` to `LM Studio`.
6. Set `Local search runtime URL` to `http://127.0.0.1:1234`.
7. Set `Local search model` to `text-embedding-mxbai-embed-large-v1`.
8. Set `Local search dimension` to `1024`.
9. Enable `Drive RAG`.
10. Set `RAG generation runtime` to `LM Studio`.
11. Set `RAG generation runtime URL` to `http://127.0.0.1:1234`.
12. Set `RAG generation model` to the loaded chat/generation model, for example `qwen/qwen3.5-9b`.
13. Keep `RAG max context chunks` between `1` and `20`; the local default is `6`.
14. Keep `RAG max context characters` between `500` and `30000`; the local default is `6000`.
15. Save the policy.

Important policy details:

- RAG requires `Drive RAG` and `Local search vector` to both be enabled.
- RAG requires `RAG generation runtime` to be `ollama` or `lmstudio`; `none` is denied.
- Vector search requires `Local search runtime` to be `ollama` or `lmstudio`.
- `Local search dimension` must be `1024`.
- Runtime URLs must be `localhost` or `127.0.0.1`. A LAN IP such as `192.168.x.x` is rejected by tenant setting validation. If LM Studio runs on another host, use a local proxy such as `scripts/openai-compatible-proxy.mjs`.

After saving the policy, add a simple Drive test file:

```text
請求書 TP-2026-0412
振込期限: 2026-06-30
税込合計: 128000円
支払先: 青葉商事
```

Then rebuild or wait for the local search index:

1. Open `Tenant Admin > acme > Data`.
2. Click `Rebuild local search`.
3. Wait until the Local Search Index table shows a completed job, or refresh until the latest job is no longer pending.

Validate semantic / hybrid search:

1. Open `/drive/search?q=支払期限`.
2. Change `検索モード` to `Semantic` or `Hybrid`.
3. Confirm the uploaded invoice file appears.
4. If `Keyword` mode does not return the file for `支払期限` but `Semantic` / `Hybrid` does, the synonym/vector path is working.

Validate RAG:

1. Stay on `/drive/search?q=支払期限`.
2. In the `Drive に質問` panel, ask `支払期限と合計金額は？`.
3. Confirm the answer includes the due date and total amount.
4. Confirm the response shows at least one citation pointing at the uploaded Drive file.

If the UI shows `drive policy denied`, check the saved Drive Policy screen first:

- `Drive RAG` is enabled.
- `Local search vector` is enabled.
- `RAG generation runtime` is not `Disabled`.
- `RAG generation runtime URL` and `RAG generation model` are filled.
- `Local search runtime`, `Local search runtime URL`, `Local search model`, and `Local search dimension=1024` are filled.
- The policy was saved successfully before returning to `/drive/search`.

If the policy is correct but the answer is blocked with no citation, that is a different failure mode: retrieval did not produce usable cited context. Rebuild local search, confirm the file is searchable in `Semantic` / `Hybrid` mode, then retry the RAG question.

If the UI shows `引用付き回答は生成されませんでした。`, the policy gate has already passed. This message means one of these happened:

- RAG search returned no usable Drive file context after OpenFGA / DLP filtering.
- Semantic hits were below the Drive score threshold.
- The generation model returned text that was not valid JSON.
- The generation model returned JSON, but no claim referenced one of the provided citation IDs such as `c1`.

Use this order to debug:

1. In `/drive/search`, run the same query with `検索モード=Semantic`.
2. If no file appears, click `Tenant Admin > acme > Data > Rebuild local search`, wait for a completed job, and retry.
3. If Semantic still returns nothing, confirm the uploaded file body contains the searched facts and was not filtered out by DLP or permissions.
4. Run the same query with `検索モード=Hybrid`; RAG uses Hybrid automatically when the UI is in Keyword mode.
5. If Semantic / Hybrid shows the file but RAG still blocks, check the LM Studio or Ollama generation model. It must return compact JSON in the shape `{"answer":"...","claims":[{"text":"...","citationIds":["c1"]}]}`. The backend also accepts markdown-fenced JSON and common citation aliases such as `citation_ids`, but claims with no known citation ID are still dropped. If usable context exists but the generation model still fails to return cited JSON, the backend falls back to a citation-backed excerpt answer instead of returning no answer.
6. Prefer a generation model that follows JSON schema / JSON mode reliably. For local smoke, `qwen/qwen3.5-9b` has been used successfully.
7. Increase `RAG max context chunks` or `RAG max context characters` only if the expected file appears in search but the relevant snippet is not included in the RAG matches.

If `Drive に質問` shows unrelated documents in the matches, check the query and search mode first. RAG retrieves with Hybrid mode when the UI is in Keyword mode, so semantic candidates can appear even when keyword search would not return them. The backend reranks candidates by filename / snippet lexical overlap and drops zero-overlap candidates from the RAG context, including the case where semantic search returns only one unrelated candidate. If unrelated documents still appear:

- Confirm the expected document title or body contains the query terms, for example `ゲーム`, `企画書`, or the exact project identifier.
- Rebuild local search after changing file contents.
- Use a more specific question, such as `ゲーム企画書の目的と主要機能は？`.
- Switch `/drive/search` to `Semantic` and `Hybrid` separately to see whether the unrelated document is coming from vector retrieval or keyword retrieval.

## Smoke Checks

API / UI:

- schema mapping candidate API returns keyword candidates while vector runtime is disabled
- accepted evidence is private by default
- tenant admin can promote evidence to `tenant`
- cross-pipeline candidate ranking only uses current-pipeline private evidence plus tenant-shared evidence
- Tenant Admin Data screen can list evidence, promote/demote sharing, and rebuild search documents

Commands already used locally:

```bash
make gen
go test ./backend/...
npm --prefix frontend run build
E2E_BASE_URL=http://127.0.0.1:5174 npm --prefix frontend run e2e -- ../e2e/schema-mapping-evidence.spec.ts
```

## Observability

Track:

- embedding pending / completed / failed counts
- local search index job lag from `source_updated_at` to `indexed_at`
- HNSW index build duration
- vector query latency by tenant / resource kind / model
- fallback count from vector / hybrid to keyword
- candidate API no-result rate for schema mapping

## Rollback

Application rollback:

1. Disable tenant vector settings.
2. Roll back to the previous app image.
3. Keep migrations in place unless a separate DB restore decision is made.

Database rollback:

- Do not run down migrations automatically in production.
- If schema rollback is required, first decide whether to restore from backup / PITR.
- Dropping the HNSW index is safe as a performance rollback:

```sql
DROP INDEX CONCURRENTLY IF EXISTS local_search_embeddings_hnsw_cosine_idx;
```

- Dropping `vector` extension is not a normal rollback step. Only drop it after confirming no object in the database depends on it.

## Exit Criteria

- migrations 0044 / 0045 are applied and `schema_migrations.dirty=false`
- `vector` extension is installed
- HNSW index exists or the environment has an explicit exact-scan exception
- schema mapping seed and rebuild are complete for the target tenant
- candidate API and tenant admin evidence UI smoke checks pass
- vector features remain disabled except for explicitly approved tenants
