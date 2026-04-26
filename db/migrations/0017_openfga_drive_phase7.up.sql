INSERT INTO roles (code)
VALUES ('drive_content_admin')
ON CONFLICT (code) DO NOTHING;

ALTER TABLE drive_share_links
    DROP CONSTRAINT drive_share_links_role_check;

ALTER TABLE drive_share_links
    ADD CONSTRAINT drive_share_links_role_check
        CHECK (role IN ('viewer', 'editor'));

CREATE TABLE drive_workspaces (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    root_folder_id BIGINT REFERENCES drive_folders(id) ON DELETE SET NULL,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    storage_quota_bytes BIGINT CHECK (storage_quota_bytes IS NULL OR storage_quota_bytes >= 0),
    policy_override JSONB NOT NULL DEFAULT '{}'::jsonb,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_workspaces_public_id_key
    ON drive_workspaces(public_id);

CREATE UNIQUE INDEX drive_workspaces_active_name_key
    ON drive_workspaces(tenant_id, lower(name))
    WHERE deleted_at IS NULL;

CREATE INDEX drive_workspaces_tenant_idx
    ON drive_workspaces(tenant_id, name, id)
    WHERE deleted_at IS NULL;

INSERT INTO drive_workspaces (tenant_id, name, created_by_user_id)
SELECT t.id,
       'Default workspace',
       (
           SELECT u.id
           FROM users u
           WHERE u.default_tenant_id = t.id
           ORDER BY u.id
           LIMIT 1
       )
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1
    FROM drive_workspaces w
    WHERE w.tenant_id = t.id
      AND w.deleted_at IS NULL
);

ALTER TABLE drive_folders
    ADD COLUMN workspace_id BIGINT REFERENCES drive_workspaces(id) ON DELETE RESTRICT;

ALTER TABLE file_objects
    ADD COLUMN workspace_id BIGINT REFERENCES drive_workspaces(id) ON DELETE RESTRICT,
    ADD COLUMN storage_bucket TEXT,
    ADD COLUMN storage_version TEXT,
    ADD COLUMN content_sha256 TEXT,
    ADD COLUMN etag TEXT,
    ADD COLUMN scan_status TEXT NOT NULL DEFAULT 'skipped'
        CHECK (scan_status IN ('pending', 'clean', 'infected', 'blocked', 'failed', 'skipped')),
    ADD COLUMN scan_reason TEXT,
    ADD COLUMN scan_engine TEXT,
    ADD COLUMN scanned_at TIMESTAMPTZ,
    ADD COLUMN dlp_blocked BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN upload_state TEXT NOT NULL DEFAULT 'active'
        CHECK (upload_state IN ('reserved', 'uploading', 'active', 'failed'));

UPDATE drive_folders folder
SET workspace_id = workspace.id
FROM drive_workspaces workspace
WHERE workspace.tenant_id = folder.tenant_id
  AND workspace.deleted_at IS NULL
  AND folder.workspace_id IS NULL;

UPDATE file_objects file
SET workspace_id = workspace.id,
    content_sha256 = NULLIF(file.sha256_hex, '')
FROM drive_workspaces workspace
WHERE workspace.tenant_id = file.tenant_id
  AND workspace.deleted_at IS NULL
  AND file.purpose = 'drive'
  AND file.workspace_id IS NULL;

CREATE INDEX drive_folders_workspace_children_idx
    ON drive_folders(workspace_id, parent_folder_id, name, id)
    WHERE deleted_at IS NULL;

CREATE INDEX file_objects_drive_workspace_children_idx
    ON file_objects(workspace_id, drive_folder_id, original_filename, id)
    WHERE purpose = 'drive' AND deleted_at IS NULL;

CREATE INDEX file_objects_drive_scan_idx
    ON file_objects(tenant_id, scan_status, dlp_blocked)
    WHERE purpose = 'drive' AND deleted_at IS NULL;

CREATE TABLE drive_admin_content_access_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    actor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT NOT NULL CHECK (btrim(reason) <> ''),
    reason_category TEXT NOT NULL CHECK (reason_category IN ('manual', 'incident', 'legal', 'security')),
    expires_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_admin_content_access_sessions_public_id_key
    ON drive_admin_content_access_sessions(public_id);

CREATE INDEX drive_admin_content_access_sessions_active_idx
    ON drive_admin_content_access_sessions(tenant_id, actor_user_id, expires_at)
    WHERE ended_at IS NULL;

CREATE TABLE drive_file_revisions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    actor_type TEXT NOT NULL DEFAULT 'user',
    previous_original_filename TEXT NOT NULL,
    previous_content_type TEXT NOT NULL,
    previous_byte_size BIGINT NOT NULL CHECK (previous_byte_size >= 0),
    previous_sha256_hex TEXT NOT NULL,
    previous_storage_driver TEXT NOT NULL,
    previous_storage_key TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT 'overwrite',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_file_revisions_public_id_key
    ON drive_file_revisions(public_id);

CREATE INDEX drive_file_revisions_file_idx
    ON drive_file_revisions(file_object_id, created_at DESC);
