CREATE TABLE realtime_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT REFERENCES tenants(id) ON DELETE CASCADE,
    recipient_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    resource_type TEXT NOT NULL DEFAULT '',
    resource_public_id TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '7 days',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (btrim(event_type) <> ''),
    CHECK (recipient_user_id > 0)
);

CREATE UNIQUE INDEX realtime_events_public_id_key
    ON realtime_events(public_id);

CREATE INDEX realtime_events_recipient_cursor_idx
    ON realtime_events(recipient_user_id, id);

CREATE INDEX realtime_events_tenant_recipient_cursor_idx
    ON realtime_events(tenant_id, recipient_user_id, id)
    WHERE tenant_id IS NOT NULL;

CREATE INDEX realtime_events_expires_idx
    ON realtime_events(expires_at);
