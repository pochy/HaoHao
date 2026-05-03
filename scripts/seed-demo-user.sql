-- Development-only local password login seed. Do not run this in production.
INSERT INTO users (email, display_name, password_hash)
VALUES (
    'demo@example.com',
    'Demo User',
    crypt('changeme123', gen_salt('bf'))
)
ON CONFLICT (email) DO UPDATE
SET
    display_name = EXCLUDED.display_name,
    password_hash = EXCLUDED.password_hash,
    updated_at = now();

INSERT INTO roles (code)
VALUES
    ('customer_signal_user'),
    ('data_pipeline_user'),
    ('docs_reader'),
    ('machine_client_admin'),
    ('tenant_admin'),
    ('todo_user')
ON CONFLICT (code) DO NOTHING;

INSERT INTO tenants (slug, display_name)
VALUES
    ('acme', 'Acme'),
    ('beta', 'Beta')
ON CONFLICT (slug) DO UPDATE
SET
    display_name = EXCLUDED.display_name,
    active = true,
    updated_at = now();

UPDATE users
SET
    default_tenant_id = (SELECT id FROM tenants WHERE slug = 'acme'),
    updated_at = now()
WHERE email = 'demo@example.com';

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code IN ('machine_client_admin', 'tenant_admin')
WHERE u.email = 'demo@example.com'
ON CONFLICT (user_id, role_id) DO NOTHING;

INSERT INTO tenant_memberships (user_id, tenant_id, role_id, source)
SELECT u.id, t.id, r.id, 'local_override'
FROM users u
JOIN tenants t ON t.slug IN ('acme', 'beta')
JOIN roles r ON r.code IN ('customer_signal_user', 'data_pipeline_user', 'docs_reader', 'todo_user')
WHERE u.email = 'demo@example.com'
ON CONFLICT (user_id, tenant_id, role_id, source) DO UPDATE
SET
    active = true,
    updated_at = now();
