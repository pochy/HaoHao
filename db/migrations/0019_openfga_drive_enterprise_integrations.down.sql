DROP TABLE IF EXISTS drive_app_webhook_deliveries;
DROP TABLE IF EXISTS drive_marketplace_installation_scopes;
DROP TABLE IF EXISTS drive_marketplace_installations;
DROP TABLE IF EXISTS drive_marketplace_app_versions;
DROP TABLE IF EXISTS drive_marketplace_apps;
DROP TABLE IF EXISTS drive_ai_summaries;
DROP TABLE IF EXISTS drive_ai_classifications;
DROP TABLE IF EXISTS drive_ai_jobs;
DROP TABLE IF EXISTS drive_e2ee_key_envelopes;
DROP TABLE IF EXISTS drive_e2ee_file_keys;
DROP TABLE IF EXISTS drive_e2ee_user_keys;
DROP TABLE IF EXISTS drive_gateway_transfers;
DROP TABLE IF EXISTS drive_gateway_objects;
ALTER TABLE file_objects DROP CONSTRAINT IF EXISTS file_objects_storage_gateway_id_fkey;
DROP TABLE IF EXISTS drive_storage_gateways;
DROP TABLE IF EXISTS drive_hsm_key_bindings;
DROP TABLE IF EXISTS drive_hsm_keys;
DROP TABLE IF EXISTS drive_hsm_deployments;
DROP TABLE IF EXISTS drive_ediscovery_export_items;
DROP TABLE IF EXISTS drive_ediscovery_exports;
DROP TABLE IF EXISTS drive_ediscovery_provider_connections;
DROP TABLE IF EXISTS drive_office_webhook_events;
DROP TABLE IF EXISTS drive_office_edit_sessions;
DROP TABLE IF EXISTS drive_office_provider_files;

ALTER TABLE file_objects
    DROP CONSTRAINT IF EXISTS file_objects_encryption_mode_check,
    DROP COLUMN IF EXISTS storage_gateway_id,
    DROP COLUMN IF EXISTS e2ee_file_key_public_id,
    DROP COLUMN IF EXISTS encryption_mode,
    DROP COLUMN IF EXISTS office_last_revision,
    DROP COLUMN IF EXISTS office_coauthoring_enabled,
    DROP COLUMN IF EXISTS office_mime_family;
