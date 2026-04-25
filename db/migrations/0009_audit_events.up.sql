CREATE TABLE audit_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    actor_type TEXT NOT NULL CHECK (actor_type IN ('user', 'machine_client', 'system')),
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    actor_machine_client_id BIGINT REFERENCES machine_clients(id) ON DELETE SET NULL,
    tenant_id BIGINT REFERENCES tenants(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    client_ip TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (btrim(action) <> ''),
    CHECK (btrim(target_type) <> ''),
    CHECK (btrim(target_id) <> '')
);

CREATE UNIQUE INDEX audit_events_public_id_idx
    ON audit_events(public_id);

CREATE INDEX audit_events_occurred_at_idx
    ON audit_events(occurred_at DESC, id DESC);

CREATE INDEX audit_events_tenant_occurred_at_idx
    ON audit_events(tenant_id, occurred_at DESC, id DESC);

CREATE INDEX audit_events_actor_user_occurred_at_idx
    ON audit_events(actor_user_id, occurred_at DESC, id DESC);

CREATE INDEX audit_events_actor_machine_client_occurred_at_idx
    ON audit_events(actor_machine_client_id, occurred_at DESC, id DESC);

CREATE INDEX audit_events_target_idx
    ON audit_events(target_type, target_id, occurred_at DESC, id DESC);

CREATE INDEX audit_events_action_occurred_at_idx
    ON audit_events(action, occurred_at DESC, id DESC);
