---
name: haohao-drive-debug
description: "Debug HaoHao Drive visibility, upload, listing, search, OCR, and RAG issues with a narrow DB/API/code workflow. Use when files are missing from the Drive UI, uploads appear stuck, search/RAG cannot find files, thumbnails/OCR are absent, or Drive behavior differs between DB, API, and frontend."
---

# HaoHao Drive Debug

## Overview

Use this skill to debug Drive issues without broad repository searches or large API dumps. Prefer a narrow path:

1. Confirm the data exists in PostgreSQL.
2. Confirm the browser API returns or omits it.
3. Check permission/OpenFGA filtering only if DB and API disagree.
4. Check frontend filters/state only if API returns the item.
5. Check SQL `ORDER BY` / `LIMIT` before assuming upload or auth failure.

Pair this skill with `haohao-db-dev` for read-only SQL access through `make sql`.

## Ripgrep Contract

In this repository, use `RIPGREP_CONFIG_PATH=` on `rg` commands so a missing user config does not add noise.

Start narrow. Prefer exact symbols and likely files over repo-wide terms:

```bash
RIPGREP_CONFIG_PATH= rg -n "ListDriveChildFiles|fetchDriveItems|applyDriveListFilter" db/queries backend/internal/service frontend/src
```

Avoid first-pass searches like `rg drive` or `rg file_objects` across the whole repository. If broad search is unavoidable, cap output and immediately narrow to concrete files.

## Missing From Drive UI

Use this workflow when the user says an uploaded Drive file is not visible.

### 1. Check Recent Drive Rows

Keep columns limited and avoid storage keys or raw metadata unless needed:

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select id, public_id, original_filename, content_type, byte_size, workspace_id, drive_folder_id, scan_status, dlp_blocked, deleted_at, purged_at, created_at, updated_at from file_objects where purpose='drive' order by created_at desc limit 30;\""
```

For image-only reports:

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select id, public_id, original_filename, content_type, byte_size, workspace_id, drive_folder_id, scan_status, dlp_blocked, deleted_at, purged_at, created_at, updated_at from file_objects where purpose='drive' and content_type like 'image/%' order by created_at desc limit 30;\""
```

Interpretation:

- No row: inspect upload route, request error, quota, max size, storage failure, or transaction rollback.
- Row with `deleted_at`: user is likely looking outside Trash or the file was cleaned up by a test/smoke.
- Row with `dlp_blocked=true` or blocked/infected scan status: inspect Drive policy and scan/DLP path.
- Row exists and active: continue to API/listing checks.

### 2. Check Workspace And Root Counts

Identify whether default UI is looking at the same workspace/folder and whether pagination/limit can hide files:

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select id, public_id, tenant_id, name, root_folder_id, created_by_user_id, deleted_at, created_at from drive_workspaces order by id;\""
```

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select tenant_id, workspace_id, drive_folder_id, count(*) as active_files, max(updated_at) as newest_update from file_objects where purpose='drive' and deleted_at is null group by tenant_id, workspace_id, drive_folder_id order by newest_update desc nulls last limit 20;\""
```

If the folder has more rows than the API default limit, inspect ordering before investigating auth.

### 3. Check Listing SQL Before API Guesswork

Read these files first for UI list visibility:

```bash
sed -n '78,105p' db/queries/drive_files.sql
sed -n '48,75p' db/queries/drive_folders.sql
sed -n '35,125p' backend/internal/service/drive_service_api.go
sed -n '250,285p' frontend/src/api/drive.ts
sed -n '330,385p' frontend/src/stores/drive.ts
sed -n '120,145p' frontend/src/components/DriveFilePickerDialog.vue
```

Look specifically for:

- SQL `ORDER BY` that does not match the UI sort.
- `LIMIT` applied before the intended sort/filter.
- `workspace_id` and `drive_folder_id` mismatches.
- Frontend filters such as `type`, `owner`, `source`, and sort direction.
- Secondary consumers such as file picker dialogs that call the same list API with a different sort.

### 4. Check API With Small Output

Log in with the local demo credentials only in local development:

```bash
curl -fsS -c /tmp/haohao-drive-debug.cookies -H 'Content-Type: application/json' -d '{"email":"demo@example.com","password":"changeme123"}' http://127.0.0.1:8080/api/v1/login
```

Query the Drive list and extract only filenames or counts. Do not paste full JSON into the conversation:

```bash
curl -fsS -b /tmp/haohao-drive-debug.cookies 'http://127.0.0.1:8080/api/v1/drive/items?workspacePublicId=<workspace-public-id>' | RIPGREP_CONFIG_PATH= rg -o 'originalFilename":"[^"]+'
```

Use a larger `limit` only to test pagination/limit hypotheses:

```bash
curl -fsS -b /tmp/haohao-drive-debug.cookies 'http://127.0.0.1:8080/api/v1/drive/items?workspacePublicId=<workspace-public-id>&limit=200' | RIPGREP_CONFIG_PATH= rg -o 'originalFilename":"[^"]+'
```

Interpretation:

- API returns the item, UI does not: inspect frontend route/state/filter/rendering.
- API omits the item, DB query includes it: inspect auth filtering, SQL ordering/limit, workspace/folder params.
- API returns it only with larger `limit`: inspect DB-side ordering and pagination design.

### 5. Check OpenFGA Only When Needed

Do this after confirming DB rows exist and API omits them for the active user. Start with readiness:

```bash
curl -fsS http://127.0.0.1:8080/readyz
```

Then inspect the authorization code around:

```bash
RIPGREP_CONFIG_PATH= rg -n "FilterViewableFiles|WriteResourceCreateTuplesWithWorkspace|CanViewFile" backend/internal/service
```

Do not assume OpenFGA is the cause when active rows are merely hidden by SQL ordering, pagination, or frontend filters.

## Search / RAG Visibility

For search or RAG misses, first determine whether Drive list visibility works. Then check search documents and index jobs:

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select resource_kind, status, count(*) from local_search_index_jobs group by resource_kind, status order by resource_kind, status;\""
```

```bash
make sql ARGS="-v ON_ERROR_STOP=1 -c \"select f.public_id, f.original_filename, d.updated_at from file_objects f left join drive_search_documents d on d.file_object_id=f.id where f.purpose='drive' order by f.created_at desc limit 20;\""
```

If files are visible in Drive but absent from search/RAG, inspect indexing/OCR/outbox next. If files are not visible in Drive, fix list/auth first.

## Verification Loop

After diagnosing or fixing a Drive issue:

1. Re-run the minimal DB query that proves the data state.
2. Re-run the minimal API query that proves the behavior.
3. Run focused Go tests:

```bash
GOCACHE=/tmp/haohao-go-build go test ./backend/internal/service ./backend/internal/api
```

4. Check `git status --short` before and after real-server smoke tests so generated or checksum files do not accidentally enter the fix.
5. If the debug path used more than one broad search or produced large JSON output, update this skill with the narrower command that would have worked.
