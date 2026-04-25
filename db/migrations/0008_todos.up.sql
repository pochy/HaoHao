CREATE TABLE todos (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (btrim(title) <> '')
);

CREATE UNIQUE INDEX todos_public_id_idx
    ON todos(public_id);

CREATE INDEX todos_tenant_id_created_at_idx
    ON todos(tenant_id, created_at DESC, id DESC);

CREATE INDEX todos_created_by_user_id_idx
    ON todos(created_by_user_id);
