DROP TABLE IF EXISTS provisioning_sync_state;

DROP INDEX IF EXISTS user_identities_provider_external_id_key;

ALTER TABLE user_identities
    DROP COLUMN IF EXISTS provisioning_source,
    DROP COLUMN IF EXISTS external_id;

ALTER TABLE users
    DROP COLUMN IF EXISTS deactivated_at;
