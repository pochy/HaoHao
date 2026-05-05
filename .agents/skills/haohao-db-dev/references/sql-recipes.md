# SQL Recipes

Use these with:

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"SQL HERE\""
```

For multi-line SQL, prefer a quoted heredoc through `ARGS="-v ON_ERROR_STOP=1 -f -"` only when the shell invocation remains readable. Otherwise use a temporary scratch file outside the repo or run separate focused `-c` queries.

## Connectivity

```sql
select
  current_database() as database,
  current_user as username,
  inet_server_addr() as server_addr,
  inet_server_port() as server_port,
  version() as version;
```

```sql
select now() - pg_postmaster_start_time() as uptime, pg_postmaster_start_time() as started_at;
```

## Database And Schema Overview

```sql
select
  datname as database,
  pg_get_userbyid(datdba) as owner,
  pg_size_pretty(pg_database_size(datname)) as size,
  datallowconn as allow_connections
from pg_database
order by datname;
```

```sql
select
  n.nspname as schema,
  pg_size_pretty(sum(pg_total_relation_size(c.oid))) as total_size,
  count(*) as relations
from pg_class c
join pg_namespace n on n.oid = c.relnamespace
where n.nspname not in ('pg_catalog', 'information_schema')
  and n.nspname not like 'pg_toast%'
group by n.nspname
order by sum(pg_total_relation_size(c.oid)) desc;
```

## Migrations

```sql
select * from schema_migrations order by version desc limit 10;
```

## Tables And Sizes

```sql
select table_name, table_type
from information_schema.tables
where table_schema = 'public'
order by table_name;
```

```sql
select
  relname as table,
  n_live_tup as estimated_rows,
  pg_size_pretty(pg_total_relation_size(relid)) as total_size,
  pg_size_pretty(pg_relation_size(relid)) as table_size,
  pg_size_pretty(pg_indexes_size(relid)) as index_size
from pg_stat_user_tables
order by pg_total_relation_size(relid) desc, relname
limit 30;
```

```sql
select
  count(*) filter (where n_live_tup > 0) as tables_with_rows,
  coalesce(sum(n_live_tup), 0) as estimated_rows
from pg_stat_user_tables;
```

## Columns

Replace `file_objects` before running.

```sql
select
  ordinal_position,
  column_name,
  data_type,
  is_nullable,
  column_default
from information_schema.columns
where table_schema = 'public'
  and table_name = 'file_objects'
order by ordinal_position;
```

Search for columns by name:

```sql
select table_name, column_name, data_type
from information_schema.columns
where table_schema = 'public'
  and column_name ilike '%tenant%'
order by table_name, ordinal_position;
```

## Indexes And Constraints

```sql
select count(*) as indexes
from pg_indexes
where schemaname = 'public';
```

```sql
select schemaname, tablename, indexname, indexdef
from pg_indexes
where schemaname = 'public'
  and tablename = 'file_objects'
order by indexname;
```

```sql
select constraint_type, count(*)
from information_schema.table_constraints
where constraint_schema = 'public'
group by constraint_type
order by constraint_type;
```

```sql
select
  tc.table_name,
  kcu.column_name,
  ccu.table_name as foreign_table_name,
  ccu.column_name as foreign_column_name,
  tc.constraint_name
from information_schema.table_constraints tc
join information_schema.key_column_usage kcu
  on tc.constraint_name = kcu.constraint_name
 and tc.constraint_schema = kcu.constraint_schema
join information_schema.constraint_column_usage ccu
  on ccu.constraint_name = tc.constraint_name
 and ccu.constraint_schema = tc.constraint_schema
where tc.constraint_type = 'FOREIGN KEY'
  and tc.table_schema = 'public'
  and tc.table_name = 'file_objects'
order by kcu.column_name;
```

## Activity And Locks

```sql
select state, count(*)
from pg_stat_activity
where datname = current_database()
group by state
order by state nulls first;
```

```sql
select
  pid,
  usename,
  state,
  wait_event_type,
  wait_event,
  now() - query_start as query_age,
  left(query, 160) as query
from pg_stat_activity
where datname = current_database()
order by query_start nulls last
limit 20;
```

```sql
select
  locktype,
  mode,
  granted,
  count(*)
from pg_locks
group by locktype, mode, granted
order by granted, count(*) desc;
```

## Query Plans

For read-only queries, use:

```sql
explain (analyze, buffers, verbose)
select ...
```

For `insert`, `update`, `delete`, or any function call with side effects, do not use `analyze` unless the user explicitly approves and the statement is wrapped in a rollback-only transaction.

## Data Checks

Use exact counts only for small tables or when the user needs correctness over speed:

```sql
select count(*) from file_objects;
```

Prefer scoped samples and avoid dumping sensitive fields:

```sql
select public_id, status, created_at, updated_at
from file_objects
order by created_at desc
limit 20;
```
