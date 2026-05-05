---
name: haohao-db-dev
description: "Inspect and debug the HaoHao local PostgreSQL database through `make sql` or `make psql`. Use when Codex needs database state for development: schema discovery, migration status, table sizes, row counts, constraints, indexes, locks/activity, query plans, SQLC query debugging, or checking app data while working in this repository."
---

# HaoHao DB Dev

## Overview

Use this skill to gather exact database facts during HaoHao development without leaving the repository's normal workflow. Prefer read-only SQL until the user explicitly asks for a write, migration, seed, or repair.

## Command Contract

Use `make sql` as the primary entrypoint:

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c 'select 1;'"
```

If `make sql` is unavailable in an older checkout, use the equivalent target:

```bash
make psql ARGS="-v ON_ERROR_STOP=1 -c 'select 1;'"
```

The target sources `.env` and connects to `DATABASE_URL`. Run `make db-wait` first when a DB container may not be ready.

## Workflow

1. Identify the question before querying: schema shape, migration status, data state, performance, locks, or SQLC behavior.
2. Start with narrow metadata queries. Avoid dumping full tables or sensitive columns such as tokens, password hashes, secrets, raw payloads, or large JSON blobs.
3. Use estimates from `pg_stat_user_tables` for broad sizing, then exact `count(*)` only for small or scoped tables.
4. Inspect table definitions before writing joins or integrity checks. Use `information_schema`, `pg_indexes`, and `\d` meta commands.
5. For query performance, use `EXPLAIN (ANALYZE, BUFFERS)` only on read-only statements. For writes, use `EXPLAIN` without `ANALYZE` unless the user explicitly approves a rollback-only test.
6. Relate DB findings back to repo code: migrations in `db/migrations`, generated SQLC code in `backend/internal/db`, query files in `db/queries`, and app services under `backend/internal/service`.
7. Summarize findings with concrete values and the command or SQL shape used. Mention if values are estimates.

## Common Checks

Check the connection and server version:

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c 'select current_database(), current_user, version();'"
```

Check migration state:

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c 'select * from schema_migrations order by version desc limit 10;'"
```

List public tables:

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select table_name from information_schema.tables where table_schema = 'public' order by table_name;\""
```

Get size and estimated row counts:

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select relname, n_live_tup, pg_size_pretty(pg_total_relation_size(relid)) from pg_stat_user_tables order by pg_total_relation_size(relid) desc limit 20;\""
```

## SQLC And Migration Development

When a task involves SQLC or schema changes:

- Read the relevant migration files in `db/migrations` before inferring current schema from code.
- Compare live DB schema to `db/schema.sql` when drift is suspected.
- After changing SQL queries, run `make sqlc`.
- After migration/schema changes, run `make db-up`, then `make db-schema` if the project expects `db/schema.sql` to be refreshed.
- Use `make sql` to verify the live schema and any seeded data needed by tests or smoke scripts.

## References

For copy-ready query snippets, read `references/sql-recipes.md`.
