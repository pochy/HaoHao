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
