-- Development-only E2E seed. Do not run this in production.
INSERT INTO users (email, display_name, password_hash)
VALUES (
    'limited@example.com',
    'Limited User',
    crypt('changeme123', gen_salt('bf'))
)
ON CONFLICT (email) DO UPDATE
SET
    display_name = EXCLUDED.display_name,
    password_hash = EXCLUDED.password_hash,
    deactivated_at = NULL,
    updated_at = now();

INSERT INTO roles (code)
VALUES
    ('todo_user'),
    ('docs_reader'),
    ('data_pipeline_user'),
    ('customer_signal_user'),
    ('tenant_admin')
ON CONFLICT (code) DO NOTHING;

INSERT INTO tenants (slug, display_name)
VALUES ('acme', 'Acme')
ON CONFLICT (slug) DO UPDATE
SET
    display_name = EXCLUDED.display_name,
    active = true,
    updated_at = now();

UPDATE users
SET
    default_tenant_id = (SELECT id FROM tenants WHERE slug = 'acme'),
    updated_at = now()
WHERE email = 'limited@example.com';

DELETE FROM user_roles
WHERE user_id = (SELECT id FROM users WHERE email = 'limited@example.com');

DELETE FROM tenant_memberships
WHERE user_id = (SELECT id FROM users WHERE email = 'limited@example.com');

INSERT INTO tenant_memberships (user_id, tenant_id, role_id, source)
SELECT u.id, t.id, r.id, 'local_override'
FROM users u
JOIN tenants t ON t.slug = 'acme'
JOIN roles r ON r.code IN ('todo_user', 'docs_reader')
WHERE u.email = 'limited@example.com'
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET
    active = true,
    updated_at = now();
