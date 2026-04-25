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
