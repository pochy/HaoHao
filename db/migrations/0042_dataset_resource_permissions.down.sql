DROP INDEX IF EXISTS dataset_permission_grants_resource_idx;
DROP INDEX IF EXISTS dataset_permission_grants_active_group_key;
DROP INDEX IF EXISTS dataset_permission_grants_active_user_key;
DROP TABLE IF EXISTS dataset_permission_grants;

DROP INDEX IF EXISTS dataset_permission_group_members_user_idx;
DROP INDEX IF EXISTS dataset_permission_group_members_active_key;
DROP TABLE IF EXISTS dataset_permission_group_members;

DROP INDEX IF EXISTS dataset_permission_groups_tenant_updated_idx;
DROP INDEX IF EXISTS dataset_permission_groups_tenant_system_key;
DROP INDEX IF EXISTS dataset_permission_groups_public_id_key;
DROP TABLE IF EXISTS dataset_permission_groups;

DROP INDEX IF EXISTS tenant_data_access_scopes_tenant_key;
DROP INDEX IF EXISTS tenant_data_access_scopes_public_id_key;
DROP TABLE IF EXISTS tenant_data_access_scopes;
