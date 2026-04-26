# Drive / OpenFGA DR Runbook

## Scope

This runbook covers Drive metadata in PostgreSQL, Drive file bodies in object storage, and OpenFGA tuples derived from the database.

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
