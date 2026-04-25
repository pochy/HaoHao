INSERT INTO roles (code)
VALUES ('tenant_admin')
ON CONFLICT (code) DO NOTHING;
