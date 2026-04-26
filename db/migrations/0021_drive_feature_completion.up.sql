ALTER TABLE drive_folders
    ADD COLUMN description TEXT NOT NULL DEFAULT '';

ALTER TABLE file_objects
    ADD COLUMN description TEXT NOT NULL DEFAULT '';

CREATE TABLE drive_starred_items (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder')),
    resource_id BIGINT NOT NULL,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_starred_items_public_id_key
    ON drive_starred_items(public_id);

CREATE UNIQUE INDEX drive_starred_items_active_key
    ON drive_starred_items(tenant_id, user_id, resource_type, resource_id)
    WHERE deleted_at IS NULL;

CREATE INDEX drive_starred_items_user_idx
    ON drive_starred_items(tenant_id, user_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE drive_item_activities (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder')),
    resource_id BIGINT NOT NULL,
    action TEXT NOT NULL CHECK (action IN (
        'viewed',
        'downloaded',
        'uploaded',
        'updated',
        'renamed',
        'moved',
        'shared',
        'unshared',
        'deleted',
        'restored',
        'previewed'
    )),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_item_activities_public_id_key
    ON drive_item_activities(public_id);

CREATE INDEX drive_item_activities_recent_idx
    ON drive_item_activities(tenant_id, actor_user_id, created_at DESC, id DESC);

CREATE INDEX drive_item_activities_resource_idx
    ON drive_item_activities(tenant_id, resource_type, resource_id, created_at DESC, id DESC);

CREATE TABLE drive_file_previews (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready', 'failed', 'skipped')),
    thumbnail_storage_key TEXT,
    preview_storage_key TEXT,
    content_type TEXT NOT NULL DEFAULT '',
    error_code TEXT,
    generated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_file_previews_public_id_key
    ON drive_file_previews(public_id);

CREATE UNIQUE INDEX drive_file_previews_file_key
    ON drive_file_previews(tenant_id, file_object_id);

CREATE INDEX drive_file_previews_status_idx
    ON drive_file_previews(tenant_id, status, updated_at);

CREATE TABLE drive_item_tags (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder')),
    resource_id BIGINT NOT NULL,
    tag TEXT NOT NULL CHECK (btrim(tag) <> ''),
    normalized_tag TEXT NOT NULL CHECK (btrim(normalized_tag) <> ''),
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_item_tags_active_key
    ON drive_item_tags(tenant_id, resource_type, resource_id, normalized_tag);

CREATE INDEX drive_item_tags_resource_idx
    ON drive_item_tags(tenant_id, resource_type, resource_id, tag);
