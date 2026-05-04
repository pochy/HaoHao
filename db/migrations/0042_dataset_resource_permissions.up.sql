CREATE TABLE tenant_data_access_scopes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX tenant_data_access_scopes_public_id_key
    ON tenant_data_access_scopes(public_id);

CREATE UNIQUE INDEX tenant_data_access_scopes_tenant_key
    ON tenant_data_access_scopes(tenant_id);

CREATE TABLE dataset_permission_groups (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    system_key TEXT,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CHECK (btrim(name) <> ''),
    CHECK (system_key IS NULL OR btrim(system_key) <> '')
);

CREATE UNIQUE INDEX dataset_permission_groups_public_id_key
    ON dataset_permission_groups(public_id);

CREATE UNIQUE INDEX dataset_permission_groups_tenant_system_key
    ON dataset_permission_groups(tenant_id, system_key)
    WHERE system_key IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX dataset_permission_groups_tenant_updated_idx
    ON dataset_permission_groups(tenant_id, updated_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE dataset_permission_group_members (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES dataset_permission_groups(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    added_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX dataset_permission_group_members_active_key
    ON dataset_permission_group_members(group_id, user_id)
    WHERE deleted_at IS NULL;

CREATE INDEX dataset_permission_group_members_user_idx
    ON dataset_permission_group_members(user_id)
    WHERE deleted_at IS NULL;

CREATE TABLE dataset_permission_grants (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    resource_type TEXT NOT NULL,
    resource_public_id UUID,
    subject_type TEXT NOT NULL,
    subject_user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    subject_group_id BIGINT REFERENCES dataset_permission_groups(id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    CHECK (resource_type IN ('data_scope', 'dataset', 'work_table', 'data_pipeline')),
    CHECK ((resource_type = 'data_scope' AND resource_public_id IS NULL) OR (resource_type <> 'data_scope' AND resource_public_id IS NOT NULL)),
    CHECK (subject_type IN ('user', 'group')),
    CHECK ((subject_type = 'user' AND subject_user_id IS NOT NULL AND subject_group_id IS NULL) OR (subject_type = 'group' AND subject_group_id IS NOT NULL AND subject_user_id IS NULL)),
    CHECK (btrim(action) <> '')
);

CREATE UNIQUE INDEX dataset_permission_grants_active_user_key
    ON dataset_permission_grants(tenant_id, resource_type, COALESCE(resource_public_id, '00000000-0000-0000-0000-000000000000'::uuid), subject_user_id, action)
    WHERE revoked_at IS NULL AND subject_type = 'user';

CREATE UNIQUE INDEX dataset_permission_grants_active_group_key
    ON dataset_permission_grants(tenant_id, resource_type, COALESCE(resource_public_id, '00000000-0000-0000-0000-000000000000'::uuid), subject_group_id, action)
    WHERE revoked_at IS NULL AND subject_type = 'group';

CREATE INDEX dataset_permission_grants_resource_idx
    ON dataset_permission_grants(tenant_id, resource_type, resource_public_id, subject_type, created_at DESC)
    WHERE revoked_at IS NULL;

INSERT INTO tenant_data_access_scopes (tenant_id)
SELECT t.id
FROM tenants t
ON CONFLICT (tenant_id) DO NOTHING;

INSERT INTO dataset_permission_groups (tenant_id, name, description, system_key)
SELECT t.id, 'Dataset Managers', 'System group for Dataset, Work table, and Data Pipeline owners.', 'dataset_managers'
FROM tenants t
ON CONFLICT (tenant_id, system_key) WHERE system_key IS NOT NULL AND deleted_at IS NULL DO NOTHING;

INSERT INTO dataset_permission_group_members (group_id, user_id)
SELECT g.id, tm.user_id
FROM dataset_permission_groups g
JOIN tenant_memberships tm ON tm.tenant_id = g.tenant_id AND tm.active
JOIN roles r ON r.id = tm.role_id AND r.code = 'tenant_admin'
WHERE g.system_key = 'dataset_managers'
ON CONFLICT (group_id, user_id) WHERE deleted_at IS NULL DO NOTHING;
