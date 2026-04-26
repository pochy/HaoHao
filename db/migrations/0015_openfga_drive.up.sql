ALTER TABLE file_objects
    DROP CONSTRAINT file_objects_purpose_check;

ALTER TABLE file_objects
    ADD CONSTRAINT file_objects_purpose_check
        CHECK (purpose IN ('attachment', 'avatar', 'import', 'export', 'drive'));

CREATE TABLE drive_folders (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    parent_folder_id BIGINT REFERENCES drive_folders(id) ON DELETE SET NULL,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    inheritance_enabled BOOLEAN NOT NULL DEFAULT true,
    deleted_at TIMESTAMPTZ,
    deleted_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_folders_public_id_key
    ON drive_folders(public_id);

CREATE UNIQUE INDEX drive_folders_active_name_key
    ON drive_folders(tenant_id, COALESCE(parent_folder_id, 0), lower(name))
    WHERE deleted_at IS NULL;

CREATE INDEX drive_folders_children_idx
    ON drive_folders(tenant_id, parent_folder_id, name, id)
    WHERE deleted_at IS NULL;

CREATE INDEX drive_folders_parent_idx
    ON drive_folders(parent_folder_id)
    WHERE deleted_at IS NULL;

ALTER TABLE file_objects
    ADD COLUMN drive_folder_id BIGINT REFERENCES drive_folders(id) ON DELETE SET NULL,
    ADD COLUMN locked_at TIMESTAMPTZ,
    ADD COLUMN locked_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN lock_reason TEXT,
    ADD COLUMN inheritance_enabled BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN deleted_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX file_objects_drive_children_idx
    ON file_objects (tenant_id, drive_folder_id, original_filename, id)
    WHERE deleted_at IS NULL AND purpose = 'drive';

CREATE INDEX file_objects_drive_folder_idx
    ON file_objects (drive_folder_id)
    WHERE deleted_at IS NULL AND purpose = 'drive';

CREATE TABLE drive_groups (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    description TEXT NOT NULL DEFAULT '',
    created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_groups_public_id_key
    ON drive_groups(public_id);

CREATE UNIQUE INDEX drive_groups_active_name_key
    ON drive_groups(tenant_id, lower(name))
    WHERE deleted_at IS NULL;

CREATE INDEX drive_groups_tenant_name_idx
    ON drive_groups(tenant_id, name, id)
    WHERE deleted_at IS NULL;

CREATE TABLE drive_group_members (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES drive_groups(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    added_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_group_members_active_key
    ON drive_group_members(group_id, user_id)
    WHERE deleted_at IS NULL;

CREATE INDEX drive_group_members_user_idx
    ON drive_group_members(user_id)
    WHERE deleted_at IS NULL;

CREATE TABLE drive_resource_shares (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder')),
    resource_id BIGINT NOT NULL,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('user', 'group')),
    subject_id BIGINT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    status TEXT NOT NULL CHECK (status IN ('active', 'revoked', 'pending_sync')),
    created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    revoked_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_resource_shares_public_id_key
    ON drive_resource_shares(public_id);

CREATE UNIQUE INDEX drive_resource_shares_active_key
    ON drive_resource_shares(tenant_id, resource_type, resource_id, subject_type, subject_id)
    WHERE status = 'active';

CREATE INDEX drive_resource_shares_resource_idx
    ON drive_resource_shares(tenant_id, resource_type, resource_id, status);

CREATE INDEX drive_resource_shares_subject_idx
    ON drive_resource_shares(tenant_id, subject_type, subject_id)
    WHERE status = 'active';

CREATE TABLE drive_share_links (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder')),
    resource_id BIGINT NOT NULL,
    token_hash TEXT NOT NULL CHECK (btrim(token_hash) <> ''),
    role TEXT NOT NULL CHECK (role = 'viewer'),
    can_download BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'disabled', 'expired', 'pending_sync')),
    created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    disabled_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    disabled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_share_links_public_id_key
    ON drive_share_links(public_id);

CREATE UNIQUE INDEX drive_share_links_token_hash_key
    ON drive_share_links(token_hash);

CREATE INDEX drive_share_links_resource_idx
    ON drive_share_links(tenant_id, resource_type, resource_id)
    WHERE status = 'active';

CREATE INDEX drive_share_links_active_lookup_idx
    ON drive_share_links(token_hash, expires_at)
    WHERE status = 'active';
