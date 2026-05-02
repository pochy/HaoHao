INSERT INTO roles (code)
VALUES
    ('drive_office_admin'),
    ('drive_legal_discovery_admin'),
    ('drive_hsm_admin'),
    ('drive_gateway_admin'),
    ('drive_ai_admin'),
    ('drive_marketplace_admin')
ON CONFLICT (code) DO NOTHING;

ALTER TABLE file_objects
    ADD COLUMN IF NOT EXISTS office_mime_family TEXT,
    ADD COLUMN IF NOT EXISTS office_coauthoring_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS office_last_revision TEXT,
    ADD COLUMN IF NOT EXISTS encryption_mode TEXT NOT NULL DEFAULT 'server_managed',
    ADD COLUMN IF NOT EXISTS e2ee_file_key_public_id UUID,
    ADD COLUMN IF NOT EXISTS storage_gateway_id BIGINT;

ALTER TABLE file_objects
    ADD CONSTRAINT file_objects_encryption_mode_check
    CHECK (encryption_mode IN ('server_managed', 'tenant_managed', 'hsm_managed', 'zero_knowledge'));

CREATE TABLE drive_office_provider_files (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_file_id TEXT NOT NULL,
    compatibility_state TEXT NOT NULL DEFAULT 'compatible',
    provider_revision TEXT NOT NULL DEFAULT '1',
    content_checksum TEXT,
    last_synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_office_provider_files_provider_check CHECK (btrim(provider) <> ''),
    CONSTRAINT drive_office_provider_files_state_check CHECK (compatibility_state IN ('compatible', 'readonly', 'unsupported', 'error'))
);

CREATE UNIQUE INDEX drive_office_provider_files_tenant_file_provider_key
    ON drive_office_provider_files(tenant_id, file_object_id, provider);
CREATE UNIQUE INDEX drive_office_provider_files_provider_file_key
    ON drive_office_provider_files(provider, provider_file_id);

CREATE TABLE drive_office_edit_sessions (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    actor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_session_id TEXT NOT NULL,
    access_level TEXT NOT NULL,
    launch_url TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_office_edit_sessions_access_check CHECK (access_level IN ('view', 'edit'))
);

CREATE UNIQUE INDEX drive_office_edit_sessions_public_id_key
    ON drive_office_edit_sessions(public_id);
CREATE INDEX drive_office_edit_sessions_file_idx
    ON drive_office_edit_sessions(tenant_id, file_object_id, revoked_at, expires_at);

CREATE TABLE drive_office_webhook_events (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT NOT NULL,
    provider_event_id TEXT NOT NULL,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT REFERENCES file_objects(id) ON DELETE SET NULL,
    payload_hash TEXT NOT NULL,
    provider_revision TEXT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    result TEXT,
    CONSTRAINT drive_office_webhook_events_result_check CHECK (result IS NULL OR result IN ('accepted', 'duplicate', 'stale', 'rejected'))
);

CREATE UNIQUE INDEX drive_office_webhook_events_provider_event_key
    ON drive_office_webhook_events(provider, provider_event_id);

CREATE TABLE drive_ediscovery_provider_connections (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    encrypted_credentials BYTEA,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_ediscovery_provider_connections_status_check CHECK (status IN ('active', 'disabled', 'error'))
);

CREATE UNIQUE INDEX drive_ediscovery_provider_connections_public_id_key
    ON drive_ediscovery_provider_connections(public_id);
CREATE UNIQUE INDEX drive_ediscovery_provider_connections_tenant_provider_key
    ON drive_ediscovery_provider_connections(tenant_id, provider);

CREATE TABLE drive_ediscovery_exports (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    case_id BIGINT REFERENCES drive_legal_cases(id) ON DELETE SET NULL,
    case_public_id UUID,
    provider_connection_id BIGINT NOT NULL REFERENCES drive_ediscovery_provider_connections(id),
    requested_by_user_id BIGINT NOT NULL REFERENCES users(id),
    approved_by_user_id BIGINT REFERENCES users(id),
    status TEXT NOT NULL DEFAULT 'pending_approval',
    manifest_hash TEXT,
    provider_export_id TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_ediscovery_exports_status_check CHECK (status IN ('pending_approval', 'approved', 'exported', 'rejected', 'failed'))
);

CREATE UNIQUE INDEX drive_ediscovery_exports_public_id_key
    ON drive_ediscovery_exports(public_id);
CREATE INDEX drive_ediscovery_exports_tenant_status_idx
    ON drive_ediscovery_exports(tenant_id, status);

CREATE TABLE drive_ediscovery_export_items (
    id BIGSERIAL PRIMARY KEY,
    export_id BIGINT NOT NULL REFERENCES drive_ediscovery_exports(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id),
    file_revision TEXT NOT NULL DEFAULT '1',
    content_sha256 TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    provider_item_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_ediscovery_export_items_status_check CHECK (status IN ('pending', 'uploaded', 'skipped', 'failed'))
);

CREATE UNIQUE INDEX drive_ediscovery_export_items_export_file_revision_key
    ON drive_ediscovery_export_items(export_id, file_object_id, file_revision);

CREATE TABLE drive_hsm_deployments (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    endpoint_url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    attestation_hash TEXT,
    health_status TEXT NOT NULL DEFAULT 'healthy',
    last_health_checked_at TIMESTAMPTZ,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_hsm_deployments_status_check CHECK (status IN ('active', 'disabled', 'error')),
    CONSTRAINT drive_hsm_deployments_health_check CHECK (health_status IN ('healthy', 'unavailable', 'unknown'))
);

CREATE UNIQUE INDEX drive_hsm_deployments_public_id_key
    ON drive_hsm_deployments(public_id);
CREATE UNIQUE INDEX drive_hsm_deployments_tenant_provider_key
    ON drive_hsm_deployments(tenant_id, provider);

CREATE TABLE drive_hsm_keys (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    deployment_id BIGINT NOT NULL REFERENCES drive_hsm_deployments(id) ON DELETE CASCADE,
    key_ref TEXT NOT NULL,
    key_version TEXT NOT NULL DEFAULT '1',
    purpose TEXT NOT NULL DEFAULT 'drive_file',
    status TEXT NOT NULL DEFAULT 'active',
    rotation_due_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_hsm_keys_status_check CHECK (status IN ('active', 'disabled', 'destroyed', 'unavailable'))
);

CREATE UNIQUE INDEX drive_hsm_keys_public_id_key
    ON drive_hsm_keys(public_id);
CREATE UNIQUE INDEX drive_hsm_keys_tenant_ref_version_key
    ON drive_hsm_keys(tenant_id, key_ref, key_version);

CREATE TABLE drive_hsm_key_bindings (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id BIGINT REFERENCES drive_workspaces(id) ON DELETE CASCADE,
    file_object_id BIGINT REFERENCES file_objects(id) ON DELETE CASCADE,
    hsm_key_id BIGINT NOT NULL REFERENCES drive_hsm_keys(id),
    binding_scope TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_hsm_key_bindings_scope_check CHECK (binding_scope IN ('tenant', 'workspace', 'file'))
);

CREATE UNIQUE INDEX drive_hsm_key_bindings_unique_scope
    ON drive_hsm_key_bindings(tenant_id, binding_scope, COALESCE(workspace_id, 0), COALESCE(file_object_id, 0));

CREATE TABLE drive_storage_gateways (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id BIGINT REFERENCES drive_workspaces(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    endpoint_url TEXT NOT NULL,
    certificate_fingerprint TEXT NOT NULL,
    last_seen_at TIMESTAMPTZ,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_storage_gateways_status_check CHECK (status IN ('active', 'disabled', 'disconnected'))
);

CREATE UNIQUE INDEX drive_storage_gateways_public_id_key
    ON drive_storage_gateways(public_id);
CREATE UNIQUE INDEX drive_storage_gateways_tenant_name_key
    ON drive_storage_gateways(tenant_id, name);

ALTER TABLE file_objects
    ADD CONSTRAINT file_objects_storage_gateway_id_fkey
    FOREIGN KEY (storage_gateway_id) REFERENCES drive_storage_gateways(id) ON DELETE SET NULL;

CREATE TABLE drive_gateway_objects (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    gateway_id BIGINT NOT NULL REFERENCES drive_storage_gateways(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    gateway_object_key TEXT NOT NULL,
    manifest_hash TEXT NOT NULL,
    replication_status TEXT NOT NULL DEFAULT 'active',
    last_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_gateway_objects_replication_status_check CHECK (replication_status IN ('active', 'pending', 'failed'))
);

CREATE UNIQUE INDEX drive_gateway_objects_gateway_object_key
    ON drive_gateway_objects(gateway_id, gateway_object_key);
CREATE UNIQUE INDEX drive_gateway_objects_file_key
    ON drive_gateway_objects(file_object_id);

CREATE TABLE drive_gateway_transfers (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    gateway_id BIGINT NOT NULL REFERENCES drive_storage_gateways(id) ON DELETE CASCADE,
    file_object_id BIGINT REFERENCES file_objects(id) ON DELETE SET NULL,
    direction TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    bytes_total BIGINT NOT NULL DEFAULT 0,
    bytes_transferred BIGINT NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_gateway_transfers_direction_check CHECK (direction IN ('upload', 'download', 'verify')),
    CONSTRAINT drive_gateway_transfers_status_check CHECK (status IN ('pending', 'completed', 'failed'))
);

CREATE UNIQUE INDEX drive_gateway_transfers_public_id_key
    ON drive_gateway_transfers(public_id);

CREATE TABLE drive_e2ee_user_keys (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_algorithm TEXT NOT NULL,
    public_key_jwk JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    rotated_at TIMESTAMPTZ,
    CONSTRAINT drive_e2ee_user_keys_status_check CHECK (status IN ('active', 'retired', 'revoked'))
);

CREATE UNIQUE INDEX drive_e2ee_user_keys_public_id_key
    ON drive_e2ee_user_keys(public_id);
CREATE UNIQUE INDEX drive_e2ee_user_keys_one_active
    ON drive_e2ee_user_keys(tenant_id, user_id)
    WHERE status = 'active';

CREATE TABLE drive_e2ee_file_keys (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    key_version INTEGER NOT NULL DEFAULT 1,
    encryption_algorithm TEXT NOT NULL,
    ciphertext_sha256 TEXT NOT NULL,
    encrypted_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_e2ee_file_keys_public_id_key
    ON drive_e2ee_file_keys(public_id);
CREATE UNIQUE INDEX drive_e2ee_file_keys_file_version_key
    ON drive_e2ee_file_keys(file_object_id, key_version);

CREATE TABLE drive_e2ee_key_envelopes (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_key_id BIGINT NOT NULL REFERENCES drive_e2ee_file_keys(id) ON DELETE CASCADE,
    recipient_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_key_id BIGINT NOT NULL REFERENCES drive_e2ee_user_keys(id),
    wrapped_file_key BYTEA NOT NULL,
    wrap_algorithm TEXT NOT NULL,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id),
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_e2ee_key_envelopes_unique_recipient
    ON drive_e2ee_key_envelopes(file_key_id, recipient_user_id, recipient_key_id);

CREATE TABLE drive_ai_jobs (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    file_revision TEXT NOT NULL DEFAULT '1',
    job_type TEXT NOT NULL,
    provider TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'completed',
    requested_by_user_id BIGINT REFERENCES users(id),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_ai_jobs_job_type_check CHECK (job_type IN ('classification', 'summary')),
    CONSTRAINT drive_ai_jobs_status_check CHECK (status IN ('pending', 'completed', 'failed', 'denied'))
);

CREATE UNIQUE INDEX drive_ai_jobs_public_id_key
    ON drive_ai_jobs(public_id);
CREATE UNIQUE INDEX drive_ai_jobs_file_revision_type_key
    ON drive_ai_jobs(file_object_id, file_revision, job_type);

CREATE TABLE drive_ai_classifications (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    file_revision TEXT NOT NULL DEFAULT '1',
    label TEXT NOT NULL,
    confidence NUMERIC(5,4) NOT NULL,
    provider TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_ai_classifications_file_revision_label_key
    ON drive_ai_classifications(file_object_id, file_revision, label);

CREATE TABLE drive_ai_summaries (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
    file_revision TEXT NOT NULL DEFAULT '1',
    summary_text TEXT NOT NULL,
    provider TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_ai_summaries_public_id_key
    ON drive_ai_summaries(public_id);
CREATE UNIQUE INDEX drive_ai_summaries_file_revision_key
    ON drive_ai_summaries(file_object_id, file_revision);

CREATE TABLE drive_marketplace_apps (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    publisher_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'reviewed',
    homepage_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_marketplace_apps_status_check CHECK (status IN ('draft', 'reviewed', 'rejected', 'disabled'))
);

CREATE UNIQUE INDEX drive_marketplace_apps_public_id_key
    ON drive_marketplace_apps(public_id);

CREATE TABLE drive_marketplace_app_versions (
    id BIGSERIAL PRIMARY KEY,
    app_id BIGINT NOT NULL REFERENCES drive_marketplace_apps(id) ON DELETE CASCADE,
    version TEXT NOT NULL,
    manifest_json JSONB NOT NULL,
    signature TEXT NOT NULL,
    review_status TEXT NOT NULL DEFAULT 'approved',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_marketplace_app_versions_review_check CHECK (review_status IN ('pending', 'approved', 'rejected'))
);

CREATE UNIQUE INDEX drive_marketplace_app_versions_app_version_key
    ON drive_marketplace_app_versions(app_id, version);

CREATE TABLE drive_marketplace_installations (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    app_id BIGINT NOT NULL REFERENCES drive_marketplace_apps(id),
    app_version_id BIGINT NOT NULL REFERENCES drive_marketplace_app_versions(id),
    status TEXT NOT NULL DEFAULT 'pending_approval',
    installed_by_user_id BIGINT NOT NULL REFERENCES users(id),
    approved_by_user_id BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_marketplace_installations_status_check CHECK (status IN ('pending_approval', 'active', 'rejected', 'uninstalled'))
);

CREATE UNIQUE INDEX drive_marketplace_installations_public_id_key
    ON drive_marketplace_installations(public_id);
CREATE UNIQUE INDEX drive_marketplace_installations_tenant_app_key
    ON drive_marketplace_installations(tenant_id, app_id);

CREATE TABLE drive_marketplace_installation_scopes (
    id BIGSERIAL PRIMARY KEY,
    installation_id BIGINT NOT NULL REFERENCES drive_marketplace_installations(id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_marketplace_installation_scopes_scope_key
    ON drive_marketplace_installation_scopes(installation_id, scope);

CREATE TABLE drive_app_webhook_deliveries (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    installation_id BIGINT NOT NULL REFERENCES drive_marketplace_installations(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT drive_app_webhook_deliveries_status_check CHECK (status IN ('pending', 'sent', 'failed', 'stopped'))
);

CREATE UNIQUE INDEX drive_app_webhook_deliveries_public_id_key
    ON drive_app_webhook_deliveries(public_id);

INSERT INTO drive_marketplace_apps (slug, name, publisher_name, status, homepage_url)
VALUES ('fake-drive-app', 'Fake Drive App', 'HaoHao Local Fake', 'reviewed', 'https://example.invalid/fake-drive-app')
ON CONFLICT (slug) DO UPDATE
SET status = 'reviewed',
    updated_at = now();

INSERT INTO drive_marketplace_app_versions (app_id, version, manifest_json, signature, review_status)
SELECT id,
       '1.0.0',
       '{"name":"Fake Drive App","version":"1.0.0","requestedScopes":["drive.file.read","drive.webhook.receive"],"redirectUris":["https://example.invalid/callback"],"webhookUrl":"https://example.invalid/webhook"}'::jsonb,
       'local-fake-signature',
       'approved'
FROM drive_marketplace_apps
WHERE slug = 'fake-drive-app'
ON CONFLICT (app_id, version) DO UPDATE
SET review_status = 'approved';
