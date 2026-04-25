DELETE FROM user_roles
WHERE role_id IN (
    SELECT id
    FROM roles
    WHERE code = 'machine_client_admin'
);

DELETE FROM roles
WHERE code = 'machine_client_admin';

DROP TABLE IF EXISTS machine_clients;
