ALTER TABLE users
    ADD COLUMN deactivated_at TIMESTAMPTZ;

ALTER TABLE user_identities
    ADD COLUMN external_id TEXT,
    ADD COLUMN provisioning_source TEXT;

CREATE UNIQUE INDEX user_identities_provider_external_id_key
    ON user_identities(provider, external_id)
    WHERE external_id IS NOT NULL;

CREATE TABLE provisioning_sync_state (
    source TEXT PRIMARY KEY,
    cursor_text TEXT,
    last_synced_at TIMESTAMPTZ,
    last_error_code TEXT,
    last_error_message TEXT,
    failed_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
