CREATE TABLE machine_clients (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider TEXT NOT NULL DEFAULT 'zitadel',
    provider_client_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    default_tenant_id BIGINT REFERENCES tenants(id) ON DELETE SET NULL,
    allowed_scopes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (btrim(provider) <> ''),
    CHECK (btrim(provider_client_id) <> ''),
    CHECK (btrim(display_name) <> '')
);

ALTER TABLE machine_clients
    ADD CONSTRAINT machine_clients_provider_client_id_key
        UNIQUE (provider, provider_client_id);

CREATE INDEX machine_clients_default_tenant_id_idx
    ON machine_clients(default_tenant_id);

INSERT INTO roles (code)
VALUES ('machine_client_admin')
ON CONFLICT (code) DO NOTHING;
