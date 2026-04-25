INSERT INTO roles (code)
VALUES ('customer_signal_user')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE customer_signals (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    customer_name TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'other'
        CHECK (source IN ('support', 'sales', 'customer_success', 'research', 'internal', 'other')),
    priority TEXT NOT NULL DEFAULT 'medium'
        CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
    status TEXT NOT NULL DEFAULT 'new'
        CHECK (status IN ('new', 'triaged', 'planned', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CHECK (btrim(customer_name) <> ''),
    CHECK (btrim(title) <> ''),
    CHECK (btrim(source) <> ''),
    CHECK (btrim(priority) <> ''),
    CHECK (btrim(status) <> '')
);

CREATE UNIQUE INDEX customer_signals_public_id_idx
    ON customer_signals(public_id);

CREATE INDEX customer_signals_tenant_created_at_idx
    ON customer_signals(tenant_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX customer_signals_tenant_status_created_at_idx
    ON customer_signals(tenant_id, status, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX customer_signals_tenant_open_priority_idx
    ON customer_signals(tenant_id, priority, created_at DESC, id DESC)
    WHERE deleted_at IS NULL
      AND status <> 'closed';

CREATE INDEX customer_signals_created_by_user_id_idx
    ON customer_signals(created_by_user_id)
    WHERE created_by_user_id IS NOT NULL;
