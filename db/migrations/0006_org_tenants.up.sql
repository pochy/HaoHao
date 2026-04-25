CREATE TABLE tenants (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users
    ADD COLUMN default_tenant_id BIGINT REFERENCES tenants(id) ON DELETE SET NULL;

CREATE TABLE tenant_memberships (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (source IN ('provider_claim', 'scim', 'local_override')),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, tenant_id, role_id, source)
);

CREATE INDEX tenant_memberships_tenant_id_idx
    ON tenant_memberships(tenant_id);

CREATE INDEX tenant_memberships_role_id_idx
    ON tenant_memberships(role_id);

CREATE TABLE tenant_role_overrides (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    effect TEXT NOT NULL CHECK (effect IN ('allow', 'deny')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, tenant_id, role_id, effect)
);

CREATE INDEX tenant_role_overrides_tenant_id_idx
    ON tenant_role_overrides(tenant_id);

ALTER TABLE oauth_user_grants
    ADD COLUMN tenant_id BIGINT REFERENCES tenants(id) ON DELETE CASCADE;

UPDATE oauth_user_grants g
SET tenant_id = u.default_tenant_id
FROM users u
WHERE g.user_id = u.id
  AND u.default_tenant_id IS NOT NULL;

DELETE FROM oauth_user_grants
WHERE tenant_id IS NULL;

ALTER TABLE oauth_user_grants
    ALTER COLUMN tenant_id SET NOT NULL,
    DROP CONSTRAINT oauth_user_grants_user_id_provider_resource_server_key,
    ADD CONSTRAINT oauth_user_grants_user_id_provider_resource_server_tenant_id_key
        UNIQUE (user_id, provider, resource_server, tenant_id);

CREATE INDEX oauth_user_grants_tenant_id_idx
    ON oauth_user_grants(tenant_id);
