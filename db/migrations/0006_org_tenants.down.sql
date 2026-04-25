DROP INDEX IF EXISTS oauth_user_grants_tenant_id_idx;

ALTER TABLE oauth_user_grants
    DROP CONSTRAINT IF EXISTS oauth_user_grants_user_id_provider_resource_server_tenant_id_key,
    DROP COLUMN IF EXISTS tenant_id,
    ADD CONSTRAINT oauth_user_grants_user_id_provider_resource_server_key
        UNIQUE (user_id, provider, resource_server);

DROP TABLE IF EXISTS tenant_role_overrides;
DROP TABLE IF EXISTS tenant_memberships;

ALTER TABLE users
    DROP COLUMN IF EXISTS default_tenant_id;

DROP TABLE IF EXISTS tenants;
