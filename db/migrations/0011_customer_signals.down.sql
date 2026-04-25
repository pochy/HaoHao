DROP TABLE IF EXISTS customer_signals;

DELETE FROM tenant_memberships
WHERE role_id IN (
    SELECT id
    FROM roles
    WHERE code = 'customer_signal_user'
);

DELETE FROM roles
WHERE code = 'customer_signal_user';
