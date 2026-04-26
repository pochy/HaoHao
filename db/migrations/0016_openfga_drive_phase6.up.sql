ALTER TABLE drive_share_links
    ADD COLUMN password_hash TEXT,
    ADD COLUMN password_required BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN password_updated_at TIMESTAMPTZ;

CREATE INDEX drive_share_links_password_required_idx
    ON drive_share_links(tenant_id, password_required, status)
    WHERE status = 'active';

CREATE TABLE drive_share_link_password_attempts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash TEXT NOT NULL,
    requester_key TEXT NOT NULL,
    failed_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
    blocked_until TIMESTAMPTZ,
    last_failed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (token_hash, requester_key)
);

CREATE INDEX drive_share_link_password_attempts_block_idx
    ON drive_share_link_password_attempts(blocked_until)
    WHERE blocked_until IS NOT NULL;

CREATE TABLE drive_share_invitations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT uuidv7(),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('file', 'folder')),
    resource_id BIGINT NOT NULL,
    invitee_email_hash TEXT NOT NULL CHECK (btrim(invitee_email_hash) <> ''),
    invitee_email_domain TEXT NOT NULL CHECK (btrim(invitee_email_domain) <> ''),
    invitee_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'pending_approval', 'accepted', 'expired', 'revoked', 'rejected')),
    expires_at TIMESTAMPTZ NOT NULL,
    approved_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ,
    revoked_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    revoked_at TIMESTAMPTZ,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    accept_token_hash TEXT,
    accept_token_expires_at TIMESTAMPTZ,
    masked_invitee_email TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX drive_share_invitations_public_id_key
    ON drive_share_invitations(public_id);

CREATE UNIQUE INDEX drive_share_invitations_accept_token_hash_key
    ON drive_share_invitations(accept_token_hash)
    WHERE accept_token_hash IS NOT NULL;

CREATE INDEX drive_share_invitations_tenant_status_idx
    ON drive_share_invitations(tenant_id, status, created_at DESC);

CREATE INDEX drive_share_invitations_invitee_idx
    ON drive_share_invitations(invitee_email_hash, status, expires_at);

CREATE INDEX drive_share_invitations_user_idx
    ON drive_share_invitations(invitee_user_id, status, expires_at)
    WHERE invitee_user_id IS NOT NULL;

CREATE TABLE drive_group_external_mappings (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    drive_group_id BIGINT NOT NULL REFERENCES drive_groups(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (btrim(provider) <> ''),
    external_group_id TEXT NOT NULL CHECK (btrim(external_group_id) <> ''),
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, provider, external_group_id),
    UNIQUE (drive_group_id, provider)
);

ALTER TABLE file_objects
    ADD COLUMN deleted_parent_folder_id BIGINT REFERENCES drive_folders(id) ON DELETE SET NULL,
    ADD COLUMN retention_until TIMESTAMPTZ,
    ADD COLUMN legal_hold_at TIMESTAMPTZ,
    ADD COLUMN legal_hold_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN legal_hold_reason TEXT,
    ADD COLUMN purge_block_reason TEXT;

ALTER TABLE drive_folders
    ADD COLUMN deleted_parent_folder_id BIGINT REFERENCES drive_folders(id) ON DELETE SET NULL,
    ADD COLUMN retention_until TIMESTAMPTZ,
    ADD COLUMN legal_hold_at TIMESTAMPTZ,
    ADD COLUMN legal_hold_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN legal_hold_reason TEXT,
    ADD COLUMN purge_block_reason TEXT;

CREATE INDEX file_objects_drive_retention_idx
    ON file_objects(tenant_id, retention_until)
    WHERE purpose = 'drive' AND retention_until IS NOT NULL;

CREATE INDEX file_objects_drive_legal_hold_idx
    ON file_objects(tenant_id, legal_hold_at)
    WHERE purpose = 'drive' AND legal_hold_at IS NOT NULL;

CREATE INDEX drive_folders_retention_idx
    ON drive_folders(tenant_id, retention_until)
    WHERE retention_until IS NOT NULL;

CREATE INDEX drive_folders_legal_hold_idx
    ON drive_folders(tenant_id, legal_hold_at)
    WHERE legal_hold_at IS NOT NULL;
