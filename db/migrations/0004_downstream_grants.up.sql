CREATE TABLE oauth_user_grants (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    resource_server TEXT NOT NULL,
    provider_subject TEXT NOT NULL,
    refresh_token_ciphertext BYTEA NOT NULL,
    refresh_token_key_version INTEGER NOT NULL,
    scope_text TEXT NOT NULL,
    granted_by_session_id TEXT NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_refreshed_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    last_error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, provider, resource_server)
);

CREATE INDEX oauth_user_grants_provider_subject_idx
    ON oauth_user_grants(provider, provider_subject);

CREATE INDEX oauth_user_grants_resource_server_idx
    ON oauth_user_grants(resource_server);
