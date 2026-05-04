# Drive / OpenFGA DR Runbook

## Scope

This runbook covers Drive metadata in PostgreSQL, Drive file bodies in object storage, and OpenFGA tuples derived from the database.

It also covers Data Access authorization for Dataset, Work table, and Data Pipeline resources, because those permissions use the same OpenFGA store and authorization model.

## Restore Order

1. Restore PostgreSQL first.
2. Verify object storage is reachable with a read-only sample check.
3. Verify the configured OpenFGA store and authorization model are reachable.
4. Run Drive operations health for the restored tenant.
5. Run OpenFGA drift dry-run.
6. Run pending-sync repair only after reviewing the dry-run diff.
7. Run Drive read-only smoke.
8. Run Drive mutation smoke.

## Commands

```bash
make db-up
make gen
go test ./backend/...
RUN_DRIVE_DRIFT_SMOKE=1 make smoke-openfga
RUN_DRIVE_STORAGE_CONSISTENCY_SMOKE=1 make smoke-openfga
```

## Checks

- `GET /api/v1/admin/tenants/{tenantSlug}/drive/operations/health`
- `POST /api/v1/admin/tenants/{tenantSlug}/drive/operations/drift-check`
- `POST /api/v1/admin/tenants/{tenantSlug}/drive/operations/repair`

OpenFGA is treated as derived state. PostgreSQL remains the source of truth for expected Drive shares, share links, workspaces, scan state, and lifecycle state.

Do not place raw tokens, storage keys, signed URLs, or file body content in incident notes or audit metadata.

## Data Access Authorization Model Mismatch

### Symptom

The tenant admin Data Access UI at `/tenant-admin/{tenantSlug}/data-access` shows:

```text
Data Access の権限モデルが未更新です。
```

Before UI sanitization, the same issue could surface as a raw OpenFGA validation error similar to:

```text
data resource authorization unavailable: POST validation error for Write POST ... Reason: type 'dataset_group' not found
```

The backend request commonly fails as:

```text
GET /api/v1/admin/tenants/{tenantSlug}/data-access/groups?limit=200 503
```

Older builds may return `500` with the raw OpenFGA validation payload.

### Cause

The backend is running with an `OPENFGA_AUTHORIZATION_MODEL_ID` that points to an older OpenFGA model. The current `openfga/drive.fga` includes Data Access types such as `dataset_group`, `data_scope`, `dataset`, `work_table`, and `data_pipeline`; the configured model in OpenFGA does not.

The application does not update OpenFGA authorization models at runtime. Any change to `openfga/drive.fga` requires an explicit bootstrap/model write and a backend restart with the new model ID.

### Resolution

1. Confirm the checked-in model tests pass:

```bash
make test-openfga-model
```

2. Register the latest model in the existing OpenFGA store:

```bash
bash -lc 'set -a; source .env; set +a; make openfga-bootstrap'
```

3. Copy the printed `OPENFGA_AUTHORIZATION_MODEL_ID` into `.env`. Keep the existing `OPENFGA_STORE_ID` unless the bootstrap output intentionally created a new store.

4. Restart the backend so it reads the new model ID.

5. Reload `/tenant-admin/{tenantSlug}/data-access`.

6. If the page still fails, check backend stdout logs for the request ID and verify `.env` contains the newly printed model ID.

### Notes

- `make openfga-bootstrap` does not edit `.env` automatically.
- A migration-only fix is not enough for this symptom. PostgreSQL may already have the Data Access tables while OpenFGA still rejects tuples for missing model types.
- Treat OpenFGA tuple state as derived state. The source of truth for Data Access groups and grants is PostgreSQL; OpenFGA tuples are repaired/synchronized from the application.
