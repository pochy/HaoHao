CREATE TABLE outbox_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT REFERENCES tenants(id) ON DELETE SET NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'dead')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 8 CHECK (max_attempts > 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    locked_by TEXT,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX outbox_events_public_id_key ON outbox_events(public_id);

CREATE INDEX outbox_events_pending_idx
    ON outbox_events(available_at, id)
    WHERE status IN ('pending', 'failed');

CREATE INDEX outbox_events_tenant_created_idx
    ON outbox_events(tenant_id, created_at DESC)
    WHERE tenant_id IS NOT NULL;

CREATE TABLE idempotency_keys (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT REFERENCES tenants(id) ON DELETE CASCADE,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    scope TEXT NOT NULL,
    idempotency_key_hash TEXT NOT NULL,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'processing'
        CHECK (status IN ('processing', 'completed', 'failed')),
    response_status INTEGER,
    response_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idempotency_keys_public_id_key ON idempotency_keys(public_id);

CREATE UNIQUE INDEX idempotency_keys_scope_key_hash_key
    ON idempotency_keys(scope, idempotency_key_hash);

CREATE INDEX idempotency_keys_expires_idx
    ON idempotency_keys(expires_at)
    WHERE status IN ('completed', 'failed');

CREATE TABLE notifications (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT REFERENCES tenants(id) ON DELETE CASCADE,
    recipient_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel TEXT NOT NULL DEFAULT 'in_app'
        CHECK (channel IN ('in_app', 'email')),
    template TEXT NOT NULL,
    subject TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'sent', 'failed', 'read', 'suppressed')),
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX notifications_public_id_key ON notifications(public_id);

CREATE INDEX notifications_recipient_unread_idx
    ON notifications(recipient_user_id, created_at DESC)
    WHERE read_at IS NULL;

CREATE INDEX notifications_tenant_created_idx
    ON notifications(tenant_id, created_at DESC)
    WHERE tenant_id IS NOT NULL;

CREATE TABLE tenant_invitations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    invited_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    accepted_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    invitee_email_normalized TEXT NOT NULL,
    role_codes JSONB NOT NULL DEFAULT '[]'::jsonb,
    token_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX tenant_invitations_public_id_key ON tenant_invitations(public_id);
CREATE UNIQUE INDEX tenant_invitations_token_hash_key ON tenant_invitations(token_hash);

CREATE INDEX tenant_invitations_pending_tenant_email_idx
    ON tenant_invitations(tenant_id, invitee_email_normalized)
    WHERE status = 'pending';

CREATE INDEX tenant_invitations_expires_idx
    ON tenant_invitations(expires_at)
    WHERE status = 'pending';

CREATE TABLE file_objects (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    uploaded_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    purpose TEXT NOT NULL DEFAULT 'attachment'
        CHECK (purpose IN ('attachment', 'avatar', 'import', 'export')),
    attached_to_type TEXT,
    attached_to_id TEXT,
    original_filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    byte_size BIGINT NOT NULL CHECK (byte_size >= 0),
    sha256_hex TEXT NOT NULL,
    storage_driver TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'deleted')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX file_objects_public_id_key ON file_objects(public_id);
CREATE UNIQUE INDEX file_objects_storage_key_key ON file_objects(storage_key);

CREATE INDEX file_objects_tenant_created_idx
    ON file_objects(tenant_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX file_objects_attachment_idx
    ON file_objects(tenant_id, attached_to_type, attached_to_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE tenant_settings (
    tenant_id BIGINT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    file_quota_bytes BIGINT NOT NULL CHECK (file_quota_bytes >= 0),
    rate_limit_login_per_minute INTEGER CHECK (rate_limit_login_per_minute IS NULL OR rate_limit_login_per_minute > 0),
    rate_limit_browser_api_per_minute INTEGER CHECK (rate_limit_browser_api_per_minute IS NULL OR rate_limit_browser_api_per_minute > 0),
    rate_limit_external_api_per_minute INTEGER CHECK (rate_limit_external_api_per_minute IS NULL OR rate_limit_external_api_per_minute > 0),
    notifications_enabled BOOLEAN NOT NULL DEFAULT true,
    features JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tenant_data_exports (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    file_object_id BIGINT REFERENCES file_objects(id) ON DELETE SET NULL,
    format TEXT NOT NULL DEFAULT 'json'
        CHECK (format IN ('json', 'csv')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'ready', 'failed', 'deleted')),
    error_summary TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX tenant_data_exports_public_id_key ON tenant_data_exports(public_id);

CREATE INDEX tenant_data_exports_tenant_created_idx
    ON tenant_data_exports(tenant_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX tenant_data_exports_pending_idx
    ON tenant_data_exports(created_at, id)
    WHERE status IN ('pending', 'processing');
