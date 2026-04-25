# Deployment Runbook

## P7 Common Services

P7 keeps the single binary / PostgreSQL / Redis shape.

- Set `SECURITY_HEADERS_ENABLED=true`.
- Enable `SECURITY_HSTS_ENABLED=true` only behind HTTPS.
- Set `TRUSTED_PROXY_CIDRS` to the reverse proxy CIDR, not arbitrary client networks.
- Persist `FILE_LOCAL_DIR` on a backed-up volume.
- Keep `OUTBOX_WORKER_ENABLED=true` for one process per environment until worker coordination is explicit.
- Keep `EMAIL_DELIVERY_MODE=log` outside staging / production SMTP configuration.

## Retention

- `audit_events`: keep indefinitely unless tenant contract requires export/deletion workflow.
- `outbox_events`: purge `sent` / `dead` after `OUTBOX_RETENTION`.
- `notifications`: purge read notifications after `NOTIFICATION_RETENTION`.
- `idempotency_keys`: purge after `IDEMPOTENCY_TTL`.
- `file_objects`: soft-deleted metadata is retained for `FILE_DELETED_RETENTION`; body deletion can be added after restore drills are stable.
- `tenant_data_exports`: expire by `DATA_EXPORT_TTL`.

## Backup

- PostgreSQL: logical `pg_dump` or managed PITR.
- Local file storage: archive the `FILE_LOCAL_DIR` volume with the same cadence as PostgreSQL backup.
- Redis: session and rate-limit state can be regenerated; persistent backup is not required for P7.

## Restore Drill

1. Restore PostgreSQL into an isolated database.
2. Restore `FILE_LOCAL_DIR` into an isolated path.
3. Start the binary with restored `DATABASE_URL` and `FILE_LOCAL_DIR`.
4. Run `make smoke-operability`, `make smoke-observability`, and `make smoke-common-services`.
5. Confirm `tenant_settings`, `file_objects`, `outbox_events`, and `tenant_data_exports` exist in the restored database.
