INSERT INTO roles (code)
VALUES
    ('drive_security_admin'),
    ('drive_sync_admin'),
    ('legal_admin'),
    ('legal_reviewer'),
    ('legal_exporter'),
    ('clean_room_admin')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE drive_search_documents (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id BIGINT REFERENCES drive_workspaces(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT '',
    extracted_text TEXT NOT NULL DEFAULT '',
    snippet TEXT NOT NULL DEFAULT '',
    content_sha256 TEXT,
    object_updated_at TIMESTAMPTZ,
    indexed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    search_vector TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(extracted_text, '')), 'B')
    ) STORED,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_search_documents_public_id_key
    ON drive_search_documents(public_id);

CREATE UNIQUE INDEX drive_search_documents_file_key
    ON drive_search_documents(file_object_id);

CREATE INDEX drive_search_documents_tenant_idx
    ON drive_search_documents(tenant_id, indexed_at DESC);

CREATE INDEX drive_search_documents_vector_idx
    ON drive_search_documents USING gin(search_vector);

CREATE TABLE drive_index_jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    reason TEXT NOT NULL DEFAULT 'metadata_changed',
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'skipped')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_index_jobs_public_id_key
    ON drive_index_jobs(public_id);

CREATE INDEX drive_index_jobs_tenant_status_idx
    ON drive_index_jobs(tenant_id, status, created_at);

CREATE TABLE drive_edit_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    actor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL DEFAULT 'lock_based',
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'ended', 'expired', 'conflicted')),
    base_revision BIGINT NOT NULL DEFAULT 0 CHECK (base_revision >= 0),
    expires_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_edit_sessions_public_id_key
    ON drive_edit_sessions(public_id);

CREATE INDEX drive_edit_sessions_file_idx
    ON drive_edit_sessions(file_object_id, status, expires_at);

CREATE TABLE drive_edit_locks (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    actor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id BIGINT NOT NULL REFERENCES drive_edit_sessions(id) ON DELETE CASCADE,
    base_revision BIGINT NOT NULL DEFAULT 0 CHECK (base_revision >= 0),
    expires_at TIMESTAMPTZ NOT NULL,
    last_heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_edit_locks_public_id_key
    ON drive_edit_locks(public_id);

CREATE UNIQUE INDEX drive_edit_locks_active_file_key
    ON drive_edit_locks(file_object_id);

CREATE INDEX drive_edit_locks_expiry_idx
    ON drive_edit_locks(expires_at);

CREATE TABLE drive_presence_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    actor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id BIGINT REFERENCES drive_edit_sessions(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'away', 'ended')),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX drive_presence_sessions_file_idx
    ON drive_presence_sessions(file_object_id, status, last_seen_at DESC);

CREATE TABLE drive_sync_devices (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_name TEXT NOT NULL CHECK (btrim(device_name) <> ''),
    platform TEXT NOT NULL DEFAULT 'desktop'
        CHECK (platform IN ('desktop', 'mobile', 'web')),
    token_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'revoked', 'lost')),
    last_seen_at TIMESTAMPTZ,
    last_ip TEXT,
    last_user_agent TEXT,
    remote_wipe_required BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_sync_devices_public_id_key
    ON drive_sync_devices(public_id);

CREATE UNIQUE INDEX drive_sync_devices_token_hash_key
    ON drive_sync_devices(token_hash);

CREATE INDEX drive_sync_devices_tenant_user_idx
    ON drive_sync_devices(tenant_id, user_id, status);

CREATE TABLE drive_sync_cursors (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    device_id BIGINT NOT NULL REFERENCES drive_sync_devices(id) ON DELETE CASCADE,
    cursor_value BIGINT NOT NULL DEFAULT 0 CHECK (cursor_value >= 0),
    last_issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (device_id)
);

CREATE TABLE drive_sync_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id BIGINT REFERENCES drive_workspaces(id) ON DELETE SET NULL,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder', 'workspace')),
    resource_id BIGINT NOT NULL,
    action TEXT NOT NULL CHECK (btrim(action) <> ''),
    object_version TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_sync_events_public_id_key
    ON drive_sync_events(public_id);

CREATE INDEX drive_sync_events_tenant_id_idx
    ON drive_sync_events(tenant_id, id);

CREATE TABLE drive_sync_conflicts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    device_id BIGINT REFERENCES drive_sync_devices(id) ON DELETE SET NULL,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder')),
    resource_id BIGINT NOT NULL,
    reason TEXT NOT NULL CHECK (btrim(reason) <> ''),
    status TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'resolved', 'discarded')),
    resolution TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drive_remote_wipe_requests (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    device_id BIGINT NOT NULL REFERENCES drive_sync_devices(id) ON DELETE CASCADE,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT 'manual',
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'acknowledged', 'expired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    acknowledged_at TIMESTAMPTZ
);

CREATE TABLE drive_mobile_offline_operations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    device_id BIGINT NOT NULL REFERENCES drive_sync_devices(id) ON DELETE CASCADE,
    operation_type TEXT NOT NULL CHECK (btrim(operation_type) <> ''),
    resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder')),
    resource_public_id UUID NOT NULL,
    base_revision BIGINT NOT NULL DEFAULT 0 CHECK (base_revision >= 0),
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'applied', 'denied', 'conflicted')),
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    applied_at TIMESTAMPTZ
);

CREATE TABLE drive_kms_keys (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider TEXT NOT NULL DEFAULT 'external',
    key_ref TEXT NOT NULL CHECK (btrim(key_ref) <> ''),
    masked_key_ref TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled', 'unavailable', 'deleted')),
    last_verified_at TIMESTAMPTZ,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_kms_keys_public_id_key
    ON drive_kms_keys(public_id);

CREATE INDEX drive_kms_keys_tenant_status_idx
    ON drive_kms_keys(tenant_id, status);

CREATE TABLE drive_encryption_policies (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    scope TEXT NOT NULL DEFAULT 'tenant'
        CHECK (scope IN ('tenant', 'workspace', 'file')),
    mode TEXT NOT NULL DEFAULT 'service_managed'
        CHECK (mode IN ('service_managed', 'tenant_managed', 'workspace_managed', 'file_managed')),
    kms_key_id BIGINT REFERENCES drive_kms_keys(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled')),
    key_loss_policy TEXT NOT NULL DEFAULT 'fail_closed',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id)
);

CREATE TABLE drive_object_key_versions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    kms_key_id BIGINT REFERENCES drive_kms_keys(id) ON DELETE SET NULL,
    key_version TEXT NOT NULL DEFAULT 'service-managed',
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'rotating', 'stale')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (file_object_id)
);

CREATE TABLE drive_key_rotation_jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    old_kms_key_id BIGINT REFERENCES drive_kms_keys(id) ON DELETE SET NULL,
    new_kms_key_id BIGINT REFERENCES drive_kms_keys(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    progress_count BIGINT NOT NULL DEFAULT 0 CHECK (progress_count >= 0),
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drive_region_policies (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    primary_region TEXT NOT NULL DEFAULT 'global',
    allowed_regions TEXT[] NOT NULL DEFAULT ARRAY['global']::text[],
    replication_mode TEXT NOT NULL DEFAULT 'none',
    index_region TEXT NOT NULL DEFAULT 'same_as_primary',
    backup_region TEXT NOT NULL DEFAULT 'same_jurisdiction',
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'pending_migration', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id)
);

CREATE TABLE drive_workspace_region_overrides (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id BIGINT NOT NULL REFERENCES drive_workspaces(id) ON DELETE CASCADE,
    primary_region TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'pending_migration', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id)
);

CREATE TABLE drive_region_migration_jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id BIGINT REFERENCES drive_workspaces(id) ON DELETE SET NULL,
    source_region TEXT NOT NULL,
    target_region TEXT NOT NULL,
    dry_run BOOLEAN NOT NULL DEFAULT true,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'requires_approval', 'succeeded', 'failed', 'rolled_back')),
    rollback_plan TEXT,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drive_region_placement_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id BIGINT REFERENCES drive_workspaces(id) ON DELETE SET NULL,
    file_object_id BIGINT REFERENCES file_objects(id) ON DELETE SET NULL,
    subsystem TEXT NOT NULL CHECK (btrim(subsystem) <> ''),
    region TEXT NOT NULL CHECK (btrim(region) <> ''),
    decision TEXT NOT NULL DEFAULT 'allowed',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drive_legal_cases (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'closed', 'archived')),
    created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_legal_cases_public_id_key
    ON drive_legal_cases(public_id);

CREATE INDEX drive_legal_cases_tenant_status_idx
    ON drive_legal_cases(tenant_id, status, created_at DESC);

CREATE TABLE drive_legal_case_resources (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    case_id BIGINT NOT NULL REFERENCES drive_legal_cases(id) ON DELETE CASCADE,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder', 'workspace')),
    resource_id BIGINT NOT NULL,
    hold_enabled BOOLEAN NOT NULL DEFAULT true,
    added_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (case_id, resource_type, resource_id)
);

CREATE TABLE drive_legal_holds (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    case_id BIGINT NOT NULL REFERENCES drive_legal_cases(id) ON DELETE CASCADE,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder')),
    resource_id BIGINT NOT NULL,
    reason TEXT NOT NULL CHECK (btrim(reason) <> ''),
    created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    released_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    released_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX drive_legal_holds_active_idx
    ON drive_legal_holds(tenant_id, resource_type, resource_id)
    WHERE released_at IS NULL;

CREATE TABLE drive_legal_exports (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    case_id BIGINT NOT NULL REFERENCES drive_legal_cases(id) ON DELETE CASCADE,
    requested_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    approved_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending_approval'
        CHECK (status IN ('pending_approval', 'approved', 'running', 'ready', 'denied', 'expired')),
    package_storage_key TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drive_legal_export_items (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    export_id BIGINT NOT NULL REFERENCES drive_legal_exports(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'included', 'denied')),
    denial_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drive_chain_of_custody_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    case_id BIGINT REFERENCES drive_legal_cases(id) ON DELETE CASCADE,
    export_id BIGINT REFERENCES drive_legal_exports(id) ON DELETE CASCADE,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (btrim(action) <> ''),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drive_clean_rooms (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'closed', 'archived')),
    policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_clean_rooms_public_id_key
    ON drive_clean_rooms(public_id);

CREATE INDEX drive_clean_rooms_tenant_status_idx
    ON drive_clean_rooms(tenant_id, status, created_at DESC);

CREATE TABLE drive_clean_room_participants (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    clean_room_id BIGINT NOT NULL REFERENCES drive_clean_rooms(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    participant_tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'participant'
        CHECK (role IN ('owner', 'participant', 'reviewer')),
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'revoked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (clean_room_id, participant_tenant_id, user_id, role)
);

CREATE TABLE drive_clean_room_datasets (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    clean_room_id BIGINT NOT NULL REFERENCES drive_clean_rooms(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source_file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE RESTRICT,
    submitted_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'submitted'
        CHECK (status IN ('submitted', 'accepted', 'rejected', 'removed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (clean_room_id, source_file_object_id)
);

CREATE TABLE drive_clean_room_jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    clean_room_id BIGINT NOT NULL REFERENCES drive_clean_rooms(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    job_type TEXT NOT NULL DEFAULT 'local_fake',
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'ready', 'failed')),
    result_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drive_clean_room_exports (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    clean_room_id BIGINT NOT NULL REFERENCES drive_clean_rooms(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    job_id BIGINT REFERENCES drive_clean_room_jobs(id) ON DELETE SET NULL,
    requested_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    approved_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending_approval'
        CHECK (status IN ('pending_approval', 'approved', 'ready', 'denied')),
    raw_dataset_export BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drive_clean_room_policy_decisions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    clean_room_id BIGINT REFERENCES drive_clean_rooms(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    actor_tenant_id BIGINT,
    resource_tenant_id BIGINT,
    decision TEXT NOT NULL CHECK (decision IN ('allow', 'deny')),
    reason TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
