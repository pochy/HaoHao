INSERT INTO roles (code)
VALUES ('support_agent')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE feature_definitions (
    code TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    default_enabled BOOLEAN NOT NULL DEFAULT false,
    default_limit JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (btrim(code) <> ''),
    CHECK (btrim(display_name) <> '')
);

INSERT INTO feature_definitions (
    code,
    display_name,
    description,
    default_enabled,
    default_limit
) VALUES
    ('webhooks.enabled', 'Outbound webhooks', 'Deliver signed tenant events to external HTTP endpoints.', false, '{"maxEndpoints": 5}'::jsonb),
    ('customer_signals.import_export', 'Customer Signals import/export', 'Import and export Customer Signals as CSV files.', false, '{"maxRows": 1000}'::jsonb),
    ('customer_signals.saved_filters', 'Customer Signals saved filters', 'Save tenant scoped Customer Signals filter presets.', true, '{}'::jsonb),
    ('support_access.enabled', 'Support access', 'Allow audited support impersonation sessions.', false, '{}'::jsonb)
ON CONFLICT (code) DO UPDATE
SET display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    default_enabled = EXCLUDED.default_enabled,
    default_limit = EXCLUDED.default_limit,
    updated_at = now();

CREATE TABLE tenant_entitlements (
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    feature_code TEXT NOT NULL REFERENCES feature_definitions(code) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL,
    limit_value JSONB NOT NULL DEFAULT '{}'::jsonb,
    source TEXT NOT NULL DEFAULT 'manual'
        CHECK (source IN ('default', 'manual', 'billing', 'migration')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, feature_code)
);

CREATE INDEX tenant_entitlements_feature_idx
    ON tenant_entitlements(feature_code, tenant_id);

CREATE TABLE webhook_endpoints (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    event_types TEXT[] NOT NULL DEFAULT ARRAY[]::text[],
    secret_ciphertext TEXT NOT NULL,
    secret_key_version INTEGER NOT NULL DEFAULT 1 CHECK (secret_key_version > 0),
    active BOOLEAN NOT NULL DEFAULT true,
    last_delivery_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CHECK (btrim(name) <> ''),
    CHECK (btrim(url) <> '')
);

CREATE UNIQUE INDEX webhook_endpoints_public_id_key
    ON webhook_endpoints(public_id);

CREATE INDEX webhook_endpoints_tenant_active_idx
    ON webhook_endpoints(tenant_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE webhook_deliveries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    webhook_endpoint_id BIGINT NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'delivered', 'failed', 'dead')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 8 CHECK (max_attempts > 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_http_status INTEGER,
    last_error TEXT,
    response_preview TEXT,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (btrim(event_type) <> '')
);

CREATE UNIQUE INDEX webhook_deliveries_public_id_key
    ON webhook_deliveries(public_id);

CREATE INDEX webhook_deliveries_endpoint_created_idx
    ON webhook_deliveries(webhook_endpoint_id, created_at DESC);

CREATE INDEX webhook_deliveries_pending_idx
    ON webhook_deliveries(next_attempt_at, id)
    WHERE status IN ('pending', 'failed');

CREATE TABLE customer_signal_import_jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    requested_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    input_file_object_id BIGINT NOT NULL REFERENCES file_objects(id) ON DELETE RESTRICT,
    error_file_object_id BIGINT REFERENCES file_objects(id) ON DELETE SET NULL,
    outbox_event_id BIGINT REFERENCES outbox_events(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    validate_only BOOLEAN NOT NULL DEFAULT false,
    total_rows INTEGER NOT NULL DEFAULT 0 CHECK (total_rows >= 0),
    valid_rows INTEGER NOT NULL DEFAULT 0 CHECK (valid_rows >= 0),
    invalid_rows INTEGER NOT NULL DEFAULT 0 CHECK (invalid_rows >= 0),
    inserted_rows INTEGER NOT NULL DEFAULT 0 CHECK (inserted_rows >= 0),
    error_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX customer_signal_import_jobs_public_id_key
    ON customer_signal_import_jobs(public_id);

CREATE INDEX customer_signal_import_jobs_tenant_created_idx
    ON customer_signal_import_jobs(tenant_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX customer_signal_import_jobs_pending_idx
    ON customer_signal_import_jobs(created_at, id)
    WHERE status IN ('pending', 'processing');

CREATE TABLE customer_signal_saved_filters (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    query TEXT NOT NULL DEFAULT '',
    filters JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CHECK (btrim(name) <> '')
);

CREATE UNIQUE INDEX customer_signal_saved_filters_public_id_key
    ON customer_signal_saved_filters(public_id);

CREATE INDEX customer_signal_saved_filters_owner_idx
    ON customer_signal_saved_filters(tenant_id, owner_user_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX customer_signal_saved_filters_owner_name_key
    ON customer_signal_saved_filters(tenant_id, owner_user_id, lower(name))
    WHERE deleted_at IS NULL;

CREATE INDEX customer_signals_tenant_search_idx
    ON customer_signals
    USING GIN (to_tsvector('simple', customer_name || ' ' || title || ' ' || body))
    WHERE deleted_at IS NULL;

CREATE TABLE support_access_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    support_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    impersonated_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'ended', 'expired')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (btrim(reason) <> ''),
    CHECK (support_user_id <> impersonated_user_id),
    CHECK (expires_at > started_at)
);

CREATE UNIQUE INDEX support_access_sessions_public_id_key
    ON support_access_sessions(public_id);

CREATE INDEX support_access_sessions_support_active_idx
    ON support_access_sessions(support_user_id, expires_at DESC)
    WHERE status = 'active';

CREATE INDEX support_access_sessions_tenant_created_idx
    ON support_access_sessions(tenant_id, created_at DESC);

INSERT INTO tenant_entitlements (
    tenant_id,
    feature_code,
    enabled,
    limit_value,
    source
)
SELECT
    ts.tenant_id,
    fd.code,
    COALESCE((ts.features ->> fd.code)::boolean, fd.default_enabled),
    fd.default_limit,
    'migration'
FROM tenant_settings ts
CROSS JOIN feature_definitions fd
WHERE ts.features ? fd.code
ON CONFLICT (tenant_id, feature_code) DO NOTHING;
