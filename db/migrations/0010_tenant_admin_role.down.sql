DELETE FROM user_roles
WHERE role_id IN (
    SELECT id
    FROM roles
    WHERE code = 'tenant_admin'
);

DELETE FROM roles
WHERE code = 'tenant_admin';
