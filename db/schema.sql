--
-- PostgreSQL database dump
--


-- Dumped from database version 18.3 (Debian 18.3-1.pgdg13+1)
-- Dumped by pg_dump version 18.3 (Debian 18.3-1.pgdg13+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: audit_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.audit_events (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    actor_type text NOT NULL,
    actor_user_id bigint,
    actor_machine_client_id bigint,
    tenant_id bigint,
    action text NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    request_id text DEFAULT ''::text NOT NULL,
    client_ip text DEFAULT ''::text NOT NULL,
    user_agent text DEFAULT ''::text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    occurred_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT audit_events_action_check CHECK ((btrim(action) <> ''::text)),
    CONSTRAINT audit_events_actor_type_check CHECK ((actor_type = ANY (ARRAY['user'::text, 'machine_client'::text, 'system'::text]))),
    CONSTRAINT audit_events_target_id_check CHECK ((btrim(target_id) <> ''::text)),
    CONSTRAINT audit_events_target_type_check CHECK ((btrim(target_type) <> ''::text))
);


--
-- Name: audit_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.audit_events ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.audit_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: customer_signal_import_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.customer_signal_import_jobs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    requested_by_user_id bigint,
    input_file_object_id bigint NOT NULL,
    error_file_object_id bigint,
    outbox_event_id bigint,
    status text DEFAULT 'pending'::text NOT NULL,
    validate_only boolean DEFAULT false NOT NULL,
    total_rows integer DEFAULT 0 NOT NULL,
    valid_rows integer DEFAULT 0 NOT NULL,
    invalid_rows integer DEFAULT 0 NOT NULL,
    inserted_rows integer DEFAULT 0 NOT NULL,
    error_summary text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    deleted_at timestamp with time zone,
    CONSTRAINT customer_signal_import_jobs_inserted_rows_check CHECK ((inserted_rows >= 0)),
    CONSTRAINT customer_signal_import_jobs_invalid_rows_check CHECK ((invalid_rows >= 0)),
    CONSTRAINT customer_signal_import_jobs_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'processing'::text, 'completed'::text, 'failed'::text]))),
    CONSTRAINT customer_signal_import_jobs_total_rows_check CHECK ((total_rows >= 0)),
    CONSTRAINT customer_signal_import_jobs_valid_rows_check CHECK ((valid_rows >= 0))
);


--
-- Name: customer_signal_import_jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.customer_signal_import_jobs ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.customer_signal_import_jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: customer_signal_saved_filters; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.customer_signal_saved_filters (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    owner_user_id bigint NOT NULL,
    name text NOT NULL,
    query text DEFAULT ''::text NOT NULL,
    filters jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT customer_signal_saved_filters_name_check CHECK ((btrim(name) <> ''::text))
);


--
-- Name: customer_signal_saved_filters_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.customer_signal_saved_filters ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.customer_signal_saved_filters_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: customer_signals; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.customer_signals (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    created_by_user_id bigint,
    customer_name text NOT NULL,
    title text NOT NULL,
    body text DEFAULT ''::text NOT NULL,
    source text DEFAULT 'other'::text NOT NULL,
    priority text DEFAULT 'medium'::text NOT NULL,
    status text DEFAULT 'new'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT customer_signals_customer_name_check CHECK ((btrim(customer_name) <> ''::text)),
    CONSTRAINT customer_signals_priority_check CHECK ((priority = ANY (ARRAY['low'::text, 'medium'::text, 'high'::text, 'urgent'::text]))),
    CONSTRAINT customer_signals_priority_check1 CHECK ((btrim(priority) <> ''::text)),
    CONSTRAINT customer_signals_source_check CHECK ((source = ANY (ARRAY['support'::text, 'sales'::text, 'customer_success'::text, 'research'::text, 'internal'::text, 'other'::text]))),
    CONSTRAINT customer_signals_source_check1 CHECK ((btrim(source) <> ''::text)),
    CONSTRAINT customer_signals_status_check CHECK ((status = ANY (ARRAY['new'::text, 'triaged'::text, 'planned'::text, 'closed'::text]))),
    CONSTRAINT customer_signals_status_check1 CHECK ((btrim(status) <> ''::text)),
    CONSTRAINT customer_signals_title_check CHECK ((btrim(title) <> ''::text))
);


--
-- Name: customer_signals_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.customer_signals ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.customer_signals_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_admin_content_access_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_admin_content_access_sessions (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    actor_user_id bigint NOT NULL,
    reason text NOT NULL,
    reason_category text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    ended_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_admin_content_access_sessions_reason_category_check CHECK ((reason_category = ANY (ARRAY['manual'::text, 'incident'::text, 'legal'::text, 'security'::text]))),
    CONSTRAINT drive_admin_content_access_sessions_reason_check CHECK ((btrim(reason) <> ''::text))
);


--
-- Name: drive_admin_content_access_sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_admin_content_access_sessions ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_admin_content_access_sessions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_file_revisions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_file_revisions (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    created_by_user_id bigint,
    actor_type text DEFAULT 'user'::text NOT NULL,
    previous_original_filename text NOT NULL,
    previous_content_type text NOT NULL,
    previous_byte_size bigint NOT NULL,
    previous_sha256_hex text NOT NULL,
    previous_storage_driver text NOT NULL,
    previous_storage_key text NOT NULL,
    reason text DEFAULT 'overwrite'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_file_revisions_previous_byte_size_check CHECK ((previous_byte_size >= 0))
);


--
-- Name: drive_file_revisions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_file_revisions ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_file_revisions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_folders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_folders (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    parent_folder_id bigint,
    name text NOT NULL,
    created_by_user_id bigint NOT NULL,
    inheritance_enabled boolean DEFAULT true NOT NULL,
    deleted_at timestamp with time zone,
    deleted_by_user_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_parent_folder_id bigint,
    retention_until timestamp with time zone,
    legal_hold_at timestamp with time zone,
    legal_hold_by_user_id bigint,
    legal_hold_reason text,
    purge_block_reason text,
    workspace_id bigint,
    CONSTRAINT drive_folders_name_check CHECK ((btrim(name) <> ''::text))
);


--
-- Name: drive_folders_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_folders ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_folders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_group_external_mappings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_group_external_mappings (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    drive_group_id bigint NOT NULL,
    provider text NOT NULL,
    external_group_id text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_group_external_mappings_external_group_id_check CHECK ((btrim(external_group_id) <> ''::text)),
    CONSTRAINT drive_group_external_mappings_provider_check CHECK ((btrim(provider) <> ''::text))
);


--
-- Name: drive_group_external_mappings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_group_external_mappings ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_group_external_mappings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_group_members; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_group_members (
    id bigint NOT NULL,
    group_id bigint NOT NULL,
    user_id bigint NOT NULL,
    added_by_user_id bigint NOT NULL,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: drive_group_members_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_group_members ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_group_members_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_groups (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    created_by_user_id bigint NOT NULL,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_groups_name_check CHECK ((btrim(name) <> ''::text))
);


--
-- Name: drive_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_groups ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_groups_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_resource_shares; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_resource_shares (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    resource_type text NOT NULL,
    resource_id bigint NOT NULL,
    subject_type text NOT NULL,
    subject_id bigint NOT NULL,
    role text NOT NULL,
    status text NOT NULL,
    created_by_user_id bigint NOT NULL,
    revoked_by_user_id bigint,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_resource_shares_resource_type_check CHECK ((resource_type = ANY (ARRAY['file'::text, 'folder'::text]))),
    CONSTRAINT drive_resource_shares_role_check CHECK ((role = ANY (ARRAY['owner'::text, 'editor'::text, 'viewer'::text]))),
    CONSTRAINT drive_resource_shares_status_check CHECK ((status = ANY (ARRAY['active'::text, 'revoked'::text, 'pending_sync'::text]))),
    CONSTRAINT drive_resource_shares_subject_type_check CHECK ((subject_type = ANY (ARRAY['user'::text, 'group'::text])))
);


--
-- Name: drive_resource_shares_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_resource_shares ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_resource_shares_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_share_invitations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_share_invitations (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    resource_type text NOT NULL,
    resource_id bigint NOT NULL,
    invitee_email_hash text NOT NULL,
    invitee_email_domain text NOT NULL,
    invitee_user_id bigint,
    role text NOT NULL,
    status text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    approved_by_user_id bigint,
    approved_at timestamp with time zone,
    accepted_at timestamp with time zone,
    revoked_by_user_id bigint,
    revoked_at timestamp with time zone,
    created_by_user_id bigint NOT NULL,
    accept_token_hash text,
    accept_token_expires_at timestamp with time zone,
    masked_invitee_email text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_share_invitations_invitee_email_domain_check CHECK ((btrim(invitee_email_domain) <> ''::text)),
    CONSTRAINT drive_share_invitations_invitee_email_hash_check CHECK ((btrim(invitee_email_hash) <> ''::text)),
    CONSTRAINT drive_share_invitations_resource_type_check CHECK ((resource_type = ANY (ARRAY['file'::text, 'folder'::text]))),
    CONSTRAINT drive_share_invitations_role_check CHECK ((role = ANY (ARRAY['owner'::text, 'editor'::text, 'viewer'::text]))),
    CONSTRAINT drive_share_invitations_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'pending_approval'::text, 'accepted'::text, 'expired'::text, 'revoked'::text, 'rejected'::text])))
);


--
-- Name: drive_share_invitations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_share_invitations ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_share_invitations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_share_link_password_attempts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_share_link_password_attempts (
    id bigint NOT NULL,
    token_hash text NOT NULL,
    requester_key text NOT NULL,
    failed_count integer DEFAULT 0 NOT NULL,
    blocked_until timestamp with time zone,
    last_failed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_share_link_password_attempts_failed_count_check CHECK ((failed_count >= 0))
);


--
-- Name: drive_share_link_password_attempts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_share_link_password_attempts ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_share_link_password_attempts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_share_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_share_links (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    resource_type text NOT NULL,
    resource_id bigint NOT NULL,
    token_hash text NOT NULL,
    role text NOT NULL,
    can_download boolean DEFAULT true NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    status text NOT NULL,
    created_by_user_id bigint NOT NULL,
    disabled_by_user_id bigint,
    disabled_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    password_hash text,
    password_required boolean DEFAULT false NOT NULL,
    password_updated_at timestamp with time zone,
    CONSTRAINT drive_share_links_resource_type_check CHECK ((resource_type = ANY (ARRAY['file'::text, 'folder'::text]))),
    CONSTRAINT drive_share_links_role_check CHECK ((role = ANY (ARRAY['viewer'::text, 'editor'::text]))),
    CONSTRAINT drive_share_links_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'expired'::text, 'pending_sync'::text]))),
    CONSTRAINT drive_share_links_token_hash_check CHECK ((btrim(token_hash) <> ''::text))
);


--
-- Name: drive_share_links_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_share_links ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_share_links_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_workspaces; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_workspaces (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    name text NOT NULL,
    root_folder_id bigint,
    created_by_user_id bigint,
    storage_quota_bytes bigint,
    policy_override jsonb DEFAULT '{}'::jsonb NOT NULL,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_workspaces_name_check CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT drive_workspaces_storage_quota_bytes_check CHECK (((storage_quota_bytes IS NULL) OR (storage_quota_bytes >= 0)))
);


--
-- Name: drive_workspaces_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_workspaces ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_workspaces_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: feature_definitions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.feature_definitions (
    code text NOT NULL,
    display_name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    default_enabled boolean DEFAULT false NOT NULL,
    default_limit jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT feature_definitions_code_check CHECK ((btrim(code) <> ''::text)),
    CONSTRAINT feature_definitions_display_name_check CHECK ((btrim(display_name) <> ''::text))
);


--
-- Name: file_objects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.file_objects (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    uploaded_by_user_id bigint,
    purpose text DEFAULT 'attachment'::text NOT NULL,
    attached_to_type text,
    attached_to_id text,
    original_filename text NOT NULL,
    content_type text NOT NULL,
    byte_size bigint NOT NULL,
    sha256_hex text NOT NULL,
    storage_driver text NOT NULL,
    storage_key text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    purged_at timestamp with time zone,
    purge_attempts integer DEFAULT 0 NOT NULL,
    purge_locked_at timestamp with time zone,
    purge_locked_by text,
    last_purge_error text,
    drive_folder_id bigint,
    locked_at timestamp with time zone,
    locked_by_user_id bigint,
    lock_reason text,
    inheritance_enabled boolean DEFAULT true NOT NULL,
    deleted_by_user_id bigint,
    deleted_parent_folder_id bigint,
    retention_until timestamp with time zone,
    legal_hold_at timestamp with time zone,
    legal_hold_by_user_id bigint,
    legal_hold_reason text,
    purge_block_reason text,
    workspace_id bigint,
    storage_bucket text,
    storage_version text,
    content_sha256 text,
    etag text,
    scan_status text DEFAULT 'skipped'::text NOT NULL,
    scan_reason text,
    scan_engine text,
    scanned_at timestamp with time zone,
    dlp_blocked boolean DEFAULT false NOT NULL,
    upload_state text DEFAULT 'active'::text NOT NULL,
    CONSTRAINT file_objects_byte_size_check CHECK ((byte_size >= 0)),
    CONSTRAINT file_objects_purge_attempts_check CHECK ((purge_attempts >= 0)),
    CONSTRAINT file_objects_purpose_check CHECK ((purpose = ANY (ARRAY['attachment'::text, 'avatar'::text, 'import'::text, 'export'::text, 'drive'::text]))),
    CONSTRAINT file_objects_scan_status_check CHECK ((scan_status = ANY (ARRAY['pending'::text, 'clean'::text, 'infected'::text, 'blocked'::text, 'failed'::text, 'skipped'::text]))),
    CONSTRAINT file_objects_status_check CHECK ((status = ANY (ARRAY['active'::text, 'deleted'::text]))),
    CONSTRAINT file_objects_upload_state_check CHECK ((upload_state = ANY (ARRAY['reserved'::text, 'uploading'::text, 'active'::text, 'failed'::text])))
);


--
-- Name: file_objects_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.file_objects ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.file_objects_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: idempotency_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.idempotency_keys (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint,
    actor_user_id bigint,
    scope text NOT NULL,
    idempotency_key_hash text NOT NULL,
    method text NOT NULL,
    path text NOT NULL,
    request_hash text NOT NULL,
    status text DEFAULT 'processing'::text NOT NULL,
    response_status integer,
    response_summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    CONSTRAINT idempotency_keys_status_check CHECK ((status = ANY (ARRAY['processing'::text, 'completed'::text, 'failed'::text])))
);


--
-- Name: idempotency_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.idempotency_keys ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.idempotency_keys_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: machine_clients; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.machine_clients (
    id bigint NOT NULL,
    provider text DEFAULT 'zitadel'::text NOT NULL,
    provider_client_id text NOT NULL,
    display_name text NOT NULL,
    default_tenant_id bigint,
    allowed_scopes text[] DEFAULT ARRAY[]::text[] NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT machine_clients_display_name_check CHECK ((btrim(display_name) <> ''::text)),
    CONSTRAINT machine_clients_provider_check CHECK ((btrim(provider) <> ''::text)),
    CONSTRAINT machine_clients_provider_client_id_check CHECK ((btrim(provider_client_id) <> ''::text))
);


--
-- Name: machine_clients_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.machine_clients ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.machine_clients_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notifications (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint,
    recipient_user_id bigint NOT NULL,
    channel text DEFAULT 'in_app'::text NOT NULL,
    template text NOT NULL,
    subject text DEFAULT ''::text NOT NULL,
    body text DEFAULT ''::text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    outbox_event_id bigint,
    read_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT notifications_channel_check CHECK ((channel = ANY (ARRAY['in_app'::text, 'email'::text]))),
    CONSTRAINT notifications_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'sent'::text, 'failed'::text, 'read'::text, 'suppressed'::text])))
);


--
-- Name: notifications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.notifications ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.notifications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: oauth_user_grants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.oauth_user_grants (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    provider text NOT NULL,
    resource_server text NOT NULL,
    provider_subject text NOT NULL,
    refresh_token_ciphertext bytea NOT NULL,
    refresh_token_key_version integer NOT NULL,
    scope_text text NOT NULL,
    granted_by_session_id text NOT NULL,
    granted_at timestamp with time zone DEFAULT now() NOT NULL,
    last_refreshed_at timestamp with time zone,
    revoked_at timestamp with time zone,
    last_error_code text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    tenant_id bigint NOT NULL
);


--
-- Name: oauth_user_grants_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.oauth_user_grants ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.oauth_user_grants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: outbox_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.outbox_events (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint,
    aggregate_type text NOT NULL,
    aggregate_id text NOT NULL,
    event_type text NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    max_attempts integer DEFAULT 8 NOT NULL,
    available_at timestamp with time zone DEFAULT now() NOT NULL,
    locked_at timestamp with time zone,
    locked_by text,
    last_error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    processed_at timestamp with time zone,
    CONSTRAINT outbox_events_attempts_check CHECK ((attempts >= 0)),
    CONSTRAINT outbox_events_max_attempts_check CHECK ((max_attempts > 0)),
    CONSTRAINT outbox_events_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'processing'::text, 'sent'::text, 'failed'::text, 'dead'::text])))
);


--
-- Name: outbox_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.outbox_events ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.outbox_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: provisioning_sync_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.provisioning_sync_state (
    source text NOT NULL,
    cursor_text text,
    last_synced_at timestamp with time zone,
    last_error_code text,
    last_error_message text,
    failed_count integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.roles (
    id bigint NOT NULL,
    code text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: roles_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.roles ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.roles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
);


--
-- Name: support_access_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.support_access_sessions (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    support_user_id bigint NOT NULL,
    impersonated_user_id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    reason text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    ended_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT support_access_sessions_check CHECK ((support_user_id <> impersonated_user_id)),
    CONSTRAINT support_access_sessions_check1 CHECK ((expires_at > started_at)),
    CONSTRAINT support_access_sessions_reason_check CHECK ((btrim(reason) <> ''::text)),
    CONSTRAINT support_access_sessions_status_check CHECK ((status = ANY (ARRAY['active'::text, 'ended'::text, 'expired'::text])))
);


--
-- Name: support_access_sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.support_access_sessions ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.support_access_sessions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: tenant_data_exports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_data_exports (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    requested_by_user_id bigint,
    outbox_event_id bigint,
    file_object_id bigint,
    format text DEFAULT 'json'::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    error_summary text,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    deleted_at timestamp with time zone,
    CONSTRAINT tenant_data_exports_format_check CHECK ((format = ANY (ARRAY['json'::text, 'csv'::text]))),
    CONSTRAINT tenant_data_exports_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'processing'::text, 'ready'::text, 'failed'::text, 'deleted'::text])))
);


--
-- Name: tenant_data_exports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.tenant_data_exports ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.tenant_data_exports_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: tenant_entitlements; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_entitlements (
    tenant_id bigint NOT NULL,
    feature_code text NOT NULL,
    enabled boolean NOT NULL,
    limit_value jsonb DEFAULT '{}'::jsonb NOT NULL,
    source text DEFAULT 'manual'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tenant_entitlements_source_check CHECK ((source = ANY (ARRAY['default'::text, 'manual'::text, 'billing'::text, 'migration'::text])))
);


--
-- Name: tenant_invitations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_invitations (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    invited_by_user_id bigint,
    accepted_by_user_id bigint,
    invitee_email_normalized text NOT NULL,
    role_codes jsonb DEFAULT '[]'::jsonb NOT NULL,
    token_hash text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    accepted_at timestamp with time zone,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tenant_invitations_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'accepted'::text, 'revoked'::text, 'expired'::text])))
);


--
-- Name: tenant_invitations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.tenant_invitations ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.tenant_invitations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: tenant_memberships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_memberships (
    user_id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    role_id bigint NOT NULL,
    source text NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tenant_memberships_source_check CHECK ((source = ANY (ARRAY['provider_claim'::text, 'scim'::text, 'local_override'::text])))
);


--
-- Name: tenant_role_overrides; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_role_overrides (
    user_id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    role_id bigint NOT NULL,
    effect text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tenant_role_overrides_effect_check CHECK ((effect = ANY (ARRAY['allow'::text, 'deny'::text])))
);


--
-- Name: tenant_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenant_settings (
    tenant_id bigint NOT NULL,
    file_quota_bytes bigint NOT NULL,
    rate_limit_login_per_minute integer,
    rate_limit_browser_api_per_minute integer,
    rate_limit_external_api_per_minute integer,
    notifications_enabled boolean DEFAULT true NOT NULL,
    features jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tenant_settings_file_quota_bytes_check CHECK ((file_quota_bytes >= 0)),
    CONSTRAINT tenant_settings_rate_limit_browser_api_per_minute_check CHECK (((rate_limit_browser_api_per_minute IS NULL) OR (rate_limit_browser_api_per_minute > 0))),
    CONSTRAINT tenant_settings_rate_limit_external_api_per_minute_check CHECK (((rate_limit_external_api_per_minute IS NULL) OR (rate_limit_external_api_per_minute > 0))),
    CONSTRAINT tenant_settings_rate_limit_login_per_minute_check CHECK (((rate_limit_login_per_minute IS NULL) OR (rate_limit_login_per_minute > 0)))
);


--
-- Name: tenants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tenants (
    id bigint NOT NULL,
    slug text NOT NULL,
    display_name text NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tenants_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.tenants ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.tenants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: todos; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.todos (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    created_by_user_id bigint NOT NULL,
    title text NOT NULL,
    completed boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT todos_title_check CHECK ((btrim(title) <> ''::text))
);


--
-- Name: todos_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.todos ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.todos_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_identities; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_identities (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    provider text NOT NULL,
    subject text NOT NULL,
    email text NOT NULL,
    email_verified boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    external_id text,
    provisioning_source text
);


--
-- Name: user_identities_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.user_identities ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.user_identities_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_roles (
    user_id bigint NOT NULL,
    role_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    email text NOT NULL,
    display_name text NOT NULL,
    password_hash text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deactivated_at timestamp with time zone,
    default_tenant_id bigint
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.users ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: webhook_deliveries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.webhook_deliveries (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    webhook_endpoint_id bigint NOT NULL,
    outbox_event_id bigint,
    event_type text NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    attempt_count integer DEFAULT 0 NOT NULL,
    max_attempts integer DEFAULT 8 NOT NULL,
    next_attempt_at timestamp with time zone DEFAULT now() NOT NULL,
    last_http_status integer,
    last_error text,
    response_preview text,
    delivered_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT webhook_deliveries_attempt_count_check CHECK ((attempt_count >= 0)),
    CONSTRAINT webhook_deliveries_event_type_check CHECK ((btrim(event_type) <> ''::text)),
    CONSTRAINT webhook_deliveries_max_attempts_check CHECK ((max_attempts > 0)),
    CONSTRAINT webhook_deliveries_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'delivered'::text, 'failed'::text, 'dead'::text])))
);


--
-- Name: webhook_deliveries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.webhook_deliveries ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.webhook_deliveries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: webhook_endpoints; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.webhook_endpoints (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    created_by_user_id bigint,
    name text NOT NULL,
    url text NOT NULL,
    event_types text[] DEFAULT ARRAY[]::text[] NOT NULL,
    secret_ciphertext text NOT NULL,
    secret_key_version integer DEFAULT 1 NOT NULL,
    active boolean DEFAULT true NOT NULL,
    last_delivery_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT webhook_endpoints_name_check CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT webhook_endpoints_secret_key_version_check CHECK ((secret_key_version > 0)),
    CONSTRAINT webhook_endpoints_url_check CHECK ((btrim(url) <> ''::text))
);


--
-- Name: webhook_endpoints_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.webhook_endpoints ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.webhook_endpoints_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: audit_events audit_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_events
    ADD CONSTRAINT audit_events_pkey PRIMARY KEY (id);


--
-- Name: customer_signal_import_jobs customer_signal_import_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signal_import_jobs
    ADD CONSTRAINT customer_signal_import_jobs_pkey PRIMARY KEY (id);


--
-- Name: customer_signal_saved_filters customer_signal_saved_filters_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signal_saved_filters
    ADD CONSTRAINT customer_signal_saved_filters_pkey PRIMARY KEY (id);


--
-- Name: customer_signals customer_signals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signals
    ADD CONSTRAINT customer_signals_pkey PRIMARY KEY (id);


--
-- Name: drive_admin_content_access_sessions drive_admin_content_access_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_admin_content_access_sessions
    ADD CONSTRAINT drive_admin_content_access_sessions_pkey PRIMARY KEY (id);


--
-- Name: drive_file_revisions drive_file_revisions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_file_revisions
    ADD CONSTRAINT drive_file_revisions_pkey PRIMARY KEY (id);


--
-- Name: drive_folders drive_folders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_folders
    ADD CONSTRAINT drive_folders_pkey PRIMARY KEY (id);


--
-- Name: drive_group_external_mappings drive_group_external_mappings_drive_group_id_provider_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_group_external_mappings
    ADD CONSTRAINT drive_group_external_mappings_drive_group_id_provider_key UNIQUE (drive_group_id, provider);


--
-- Name: drive_group_external_mappings drive_group_external_mappings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_group_external_mappings
    ADD CONSTRAINT drive_group_external_mappings_pkey PRIMARY KEY (id);


--
-- Name: drive_group_external_mappings drive_group_external_mappings_tenant_id_provider_external_g_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_group_external_mappings
    ADD CONSTRAINT drive_group_external_mappings_tenant_id_provider_external_g_key UNIQUE (tenant_id, provider, external_group_id);


--
-- Name: drive_group_members drive_group_members_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_group_members
    ADD CONSTRAINT drive_group_members_pkey PRIMARY KEY (id);


--
-- Name: drive_groups drive_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_groups
    ADD CONSTRAINT drive_groups_pkey PRIMARY KEY (id);


--
-- Name: drive_resource_shares drive_resource_shares_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_resource_shares
    ADD CONSTRAINT drive_resource_shares_pkey PRIMARY KEY (id);


--
-- Name: drive_share_invitations drive_share_invitations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_share_invitations
    ADD CONSTRAINT drive_share_invitations_pkey PRIMARY KEY (id);


--
-- Name: drive_share_link_password_attempts drive_share_link_password_attempts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_share_link_password_attempts
    ADD CONSTRAINT drive_share_link_password_attempts_pkey PRIMARY KEY (id);


--
-- Name: drive_share_link_password_attempts drive_share_link_password_attempts_token_hash_requester_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_share_link_password_attempts
    ADD CONSTRAINT drive_share_link_password_attempts_token_hash_requester_key_key UNIQUE (token_hash, requester_key);


--
-- Name: drive_share_links drive_share_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_share_links
    ADD CONSTRAINT drive_share_links_pkey PRIMARY KEY (id);


--
-- Name: drive_workspaces drive_workspaces_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_workspaces
    ADD CONSTRAINT drive_workspaces_pkey PRIMARY KEY (id);


--
-- Name: feature_definitions feature_definitions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.feature_definitions
    ADD CONSTRAINT feature_definitions_pkey PRIMARY KEY (code);


--
-- Name: file_objects file_objects_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file_objects
    ADD CONSTRAINT file_objects_pkey PRIMARY KEY (id);


--
-- Name: idempotency_keys idempotency_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.idempotency_keys
    ADD CONSTRAINT idempotency_keys_pkey PRIMARY KEY (id);


--
-- Name: machine_clients machine_clients_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.machine_clients
    ADD CONSTRAINT machine_clients_pkey PRIMARY KEY (id);


--
-- Name: machine_clients machine_clients_provider_client_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.machine_clients
    ADD CONSTRAINT machine_clients_provider_client_id_key UNIQUE (provider, provider_client_id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: oauth_user_grants oauth_user_grants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_pkey PRIMARY KEY (id);


--
-- Name: oauth_user_grants oauth_user_grants_user_id_provider_resource_server_tenant_id_ke; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_user_id_provider_resource_server_tenant_id_ke UNIQUE (user_id, provider, resource_server, tenant_id);


--
-- Name: outbox_events outbox_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_pkey PRIMARY KEY (id);


--
-- Name: provisioning_sync_state provisioning_sync_state_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.provisioning_sync_state
    ADD CONSTRAINT provisioning_sync_state_pkey PRIMARY KEY (source);


--
-- Name: roles roles_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_code_key UNIQUE (code);


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: support_access_sessions support_access_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.support_access_sessions
    ADD CONSTRAINT support_access_sessions_pkey PRIMARY KEY (id);


--
-- Name: tenant_data_exports tenant_data_exports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_data_exports
    ADD CONSTRAINT tenant_data_exports_pkey PRIMARY KEY (id);


--
-- Name: tenant_entitlements tenant_entitlements_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_entitlements
    ADD CONSTRAINT tenant_entitlements_pkey PRIMARY KEY (tenant_id, feature_code);


--
-- Name: tenant_invitations tenant_invitations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_invitations
    ADD CONSTRAINT tenant_invitations_pkey PRIMARY KEY (id);


--
-- Name: tenant_memberships tenant_memberships_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_memberships
    ADD CONSTRAINT tenant_memberships_pkey PRIMARY KEY (user_id, tenant_id, role_id, source);


--
-- Name: tenant_role_overrides tenant_role_overrides_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_role_overrides
    ADD CONSTRAINT tenant_role_overrides_pkey PRIMARY KEY (user_id, tenant_id, role_id, effect);


--
-- Name: tenant_settings tenant_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_settings
    ADD CONSTRAINT tenant_settings_pkey PRIMARY KEY (tenant_id);


--
-- Name: tenants tenants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_pkey PRIMARY KEY (id);


--
-- Name: tenants tenants_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_slug_key UNIQUE (slug);


--
-- Name: todos todos_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.todos
    ADD CONSTRAINT todos_pkey PRIMARY KEY (id);


--
-- Name: user_identities user_identities_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_pkey PRIMARY KEY (id);


--
-- Name: user_identities user_identities_provider_subject_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_provider_subject_key UNIQUE (provider, subject);


--
-- Name: user_roles user_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_pkey PRIMARY KEY (user_id, role_id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: webhook_deliveries webhook_deliveries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.webhook_deliveries
    ADD CONSTRAINT webhook_deliveries_pkey PRIMARY KEY (id);


--
-- Name: webhook_endpoints webhook_endpoints_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.webhook_endpoints
    ADD CONSTRAINT webhook_endpoints_pkey PRIMARY KEY (id);


--
-- Name: audit_events_action_occurred_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_events_action_occurred_at_idx ON public.audit_events USING btree (action, occurred_at DESC, id DESC);


--
-- Name: audit_events_actor_machine_client_occurred_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_events_actor_machine_client_occurred_at_idx ON public.audit_events USING btree (actor_machine_client_id, occurred_at DESC, id DESC);


--
-- Name: audit_events_actor_user_occurred_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_events_actor_user_occurred_at_idx ON public.audit_events USING btree (actor_user_id, occurred_at DESC, id DESC);


--
-- Name: audit_events_occurred_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_events_occurred_at_idx ON public.audit_events USING btree (occurred_at DESC, id DESC);


--
-- Name: audit_events_public_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX audit_events_public_id_idx ON public.audit_events USING btree (public_id);


--
-- Name: audit_events_target_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_events_target_idx ON public.audit_events USING btree (target_type, target_id, occurred_at DESC, id DESC);


--
-- Name: audit_events_tenant_occurred_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_events_tenant_occurred_at_idx ON public.audit_events USING btree (tenant_id, occurred_at DESC, id DESC);


--
-- Name: customer_signal_import_jobs_pending_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX customer_signal_import_jobs_pending_idx ON public.customer_signal_import_jobs USING btree (created_at, id) WHERE (status = ANY (ARRAY['pending'::text, 'processing'::text]));


--
-- Name: customer_signal_import_jobs_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX customer_signal_import_jobs_public_id_key ON public.customer_signal_import_jobs USING btree (public_id);


--
-- Name: customer_signal_import_jobs_tenant_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX customer_signal_import_jobs_tenant_created_idx ON public.customer_signal_import_jobs USING btree (tenant_id, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: customer_signal_saved_filters_owner_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX customer_signal_saved_filters_owner_idx ON public.customer_signal_saved_filters USING btree (tenant_id, owner_user_id, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: customer_signal_saved_filters_owner_name_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX customer_signal_saved_filters_owner_name_key ON public.customer_signal_saved_filters USING btree (tenant_id, owner_user_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: customer_signal_saved_filters_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX customer_signal_saved_filters_public_id_key ON public.customer_signal_saved_filters USING btree (public_id);


--
-- Name: customer_signals_created_by_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX customer_signals_created_by_user_id_idx ON public.customer_signals USING btree (created_by_user_id) WHERE (created_by_user_id IS NOT NULL);


--
-- Name: customer_signals_public_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX customer_signals_public_id_idx ON public.customer_signals USING btree (public_id);


--
-- Name: customer_signals_tenant_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX customer_signals_tenant_created_at_idx ON public.customer_signals USING btree (tenant_id, created_at DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: customer_signals_tenant_open_priority_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX customer_signals_tenant_open_priority_idx ON public.customer_signals USING btree (tenant_id, priority, created_at DESC, id DESC) WHERE ((deleted_at IS NULL) AND (status <> 'closed'::text));


--
-- Name: customer_signals_tenant_search_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX customer_signals_tenant_search_idx ON public.customer_signals USING gin (to_tsvector('simple'::regconfig, ((((customer_name || ' '::text) || title) || ' '::text) || body))) WHERE (deleted_at IS NULL);


--
-- Name: customer_signals_tenant_status_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX customer_signals_tenant_status_created_at_idx ON public.customer_signals USING btree (tenant_id, status, created_at DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: drive_admin_content_access_sessions_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_admin_content_access_sessions_active_idx ON public.drive_admin_content_access_sessions USING btree (tenant_id, actor_user_id, expires_at) WHERE (ended_at IS NULL);


--
-- Name: drive_admin_content_access_sessions_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_admin_content_access_sessions_public_id_key ON public.drive_admin_content_access_sessions USING btree (public_id);


--
-- Name: drive_file_revisions_file_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_file_revisions_file_idx ON public.drive_file_revisions USING btree (file_object_id, created_at DESC);


--
-- Name: drive_file_revisions_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_file_revisions_public_id_key ON public.drive_file_revisions USING btree (public_id);


--
-- Name: drive_folders_active_name_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_folders_active_name_key ON public.drive_folders USING btree (tenant_id, COALESCE(parent_folder_id, (0)::bigint), lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: drive_folders_children_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_folders_children_idx ON public.drive_folders USING btree (tenant_id, parent_folder_id, name, id) WHERE (deleted_at IS NULL);


--
-- Name: drive_folders_legal_hold_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_folders_legal_hold_idx ON public.drive_folders USING btree (tenant_id, legal_hold_at) WHERE (legal_hold_at IS NOT NULL);


--
-- Name: drive_folders_parent_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_folders_parent_idx ON public.drive_folders USING btree (parent_folder_id) WHERE (deleted_at IS NULL);


--
-- Name: drive_folders_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_folders_public_id_key ON public.drive_folders USING btree (public_id);


--
-- Name: drive_folders_retention_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_folders_retention_idx ON public.drive_folders USING btree (tenant_id, retention_until) WHERE (retention_until IS NOT NULL);


--
-- Name: drive_folders_workspace_children_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_folders_workspace_children_idx ON public.drive_folders USING btree (workspace_id, parent_folder_id, name, id) WHERE (deleted_at IS NULL);


--
-- Name: drive_group_members_active_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_group_members_active_key ON public.drive_group_members USING btree (group_id, user_id) WHERE (deleted_at IS NULL);


--
-- Name: drive_group_members_user_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_group_members_user_idx ON public.drive_group_members USING btree (user_id) WHERE (deleted_at IS NULL);


--
-- Name: drive_groups_active_name_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_groups_active_name_key ON public.drive_groups USING btree (tenant_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: drive_groups_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_groups_public_id_key ON public.drive_groups USING btree (public_id);


--
-- Name: drive_groups_tenant_name_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_groups_tenant_name_idx ON public.drive_groups USING btree (tenant_id, name, id) WHERE (deleted_at IS NULL);


--
-- Name: drive_resource_shares_active_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_resource_shares_active_key ON public.drive_resource_shares USING btree (tenant_id, resource_type, resource_id, subject_type, subject_id) WHERE (status = 'active'::text);


--
-- Name: drive_resource_shares_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_resource_shares_public_id_key ON public.drive_resource_shares USING btree (public_id);


--
-- Name: drive_resource_shares_resource_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_resource_shares_resource_idx ON public.drive_resource_shares USING btree (tenant_id, resource_type, resource_id, status);


--
-- Name: drive_resource_shares_subject_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_resource_shares_subject_idx ON public.drive_resource_shares USING btree (tenant_id, subject_type, subject_id) WHERE (status = 'active'::text);


--
-- Name: drive_share_invitations_accept_token_hash_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_share_invitations_accept_token_hash_key ON public.drive_share_invitations USING btree (accept_token_hash) WHERE (accept_token_hash IS NOT NULL);


--
-- Name: drive_share_invitations_invitee_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_share_invitations_invitee_idx ON public.drive_share_invitations USING btree (invitee_email_hash, status, expires_at);


--
-- Name: drive_share_invitations_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_share_invitations_public_id_key ON public.drive_share_invitations USING btree (public_id);


--
-- Name: drive_share_invitations_tenant_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_share_invitations_tenant_status_idx ON public.drive_share_invitations USING btree (tenant_id, status, created_at DESC);


--
-- Name: drive_share_invitations_user_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_share_invitations_user_idx ON public.drive_share_invitations USING btree (invitee_user_id, status, expires_at) WHERE (invitee_user_id IS NOT NULL);


--
-- Name: drive_share_link_password_attempts_block_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_share_link_password_attempts_block_idx ON public.drive_share_link_password_attempts USING btree (blocked_until) WHERE (blocked_until IS NOT NULL);


--
-- Name: drive_share_links_active_lookup_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_share_links_active_lookup_idx ON public.drive_share_links USING btree (token_hash, expires_at) WHERE (status = 'active'::text);


--
-- Name: drive_share_links_password_required_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_share_links_password_required_idx ON public.drive_share_links USING btree (tenant_id, password_required, status) WHERE (status = 'active'::text);


--
-- Name: drive_share_links_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_share_links_public_id_key ON public.drive_share_links USING btree (public_id);


--
-- Name: drive_share_links_resource_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_share_links_resource_idx ON public.drive_share_links USING btree (tenant_id, resource_type, resource_id) WHERE (status = 'active'::text);


--
-- Name: drive_share_links_token_hash_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_share_links_token_hash_key ON public.drive_share_links USING btree (token_hash);


--
-- Name: drive_workspaces_active_name_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_workspaces_active_name_key ON public.drive_workspaces USING btree (tenant_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: drive_workspaces_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_workspaces_public_id_key ON public.drive_workspaces USING btree (public_id);


--
-- Name: drive_workspaces_tenant_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_workspaces_tenant_idx ON public.drive_workspaces USING btree (tenant_id, name, id) WHERE (deleted_at IS NULL);


--
-- Name: file_objects_attachment_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX file_objects_attachment_idx ON public.file_objects USING btree (tenant_id, attached_to_type, attached_to_id, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: file_objects_drive_children_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX file_objects_drive_children_idx ON public.file_objects USING btree (tenant_id, drive_folder_id, original_filename, id) WHERE ((deleted_at IS NULL) AND (purpose = 'drive'::text));


--
-- Name: file_objects_drive_folder_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX file_objects_drive_folder_idx ON public.file_objects USING btree (drive_folder_id) WHERE ((deleted_at IS NULL) AND (purpose = 'drive'::text));


--
-- Name: file_objects_drive_legal_hold_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX file_objects_drive_legal_hold_idx ON public.file_objects USING btree (tenant_id, legal_hold_at) WHERE ((purpose = 'drive'::text) AND (legal_hold_at IS NOT NULL));


--
-- Name: file_objects_drive_retention_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX file_objects_drive_retention_idx ON public.file_objects USING btree (tenant_id, retention_until) WHERE ((purpose = 'drive'::text) AND (retention_until IS NOT NULL));


--
-- Name: file_objects_drive_scan_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX file_objects_drive_scan_idx ON public.file_objects USING btree (tenant_id, scan_status, dlp_blocked) WHERE ((purpose = 'drive'::text) AND (deleted_at IS NULL));


--
-- Name: file_objects_drive_workspace_children_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX file_objects_drive_workspace_children_idx ON public.file_objects USING btree (workspace_id, drive_folder_id, original_filename, id) WHERE ((purpose = 'drive'::text) AND (deleted_at IS NULL));


--
-- Name: file_objects_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX file_objects_public_id_key ON public.file_objects USING btree (public_id);


--
-- Name: file_objects_purge_candidates_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX file_objects_purge_candidates_idx ON public.file_objects USING btree (deleted_at, id) WHERE ((deleted_at IS NOT NULL) AND (purged_at IS NULL));


--
-- Name: file_objects_purge_lock_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX file_objects_purge_lock_idx ON public.file_objects USING btree (purge_locked_at) WHERE ((purge_locked_at IS NOT NULL) AND (purged_at IS NULL));


--
-- Name: file_objects_storage_key_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX file_objects_storage_key_key ON public.file_objects USING btree (storage_key);


--
-- Name: file_objects_tenant_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX file_objects_tenant_created_idx ON public.file_objects USING btree (tenant_id, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: idempotency_keys_expires_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idempotency_keys_expires_idx ON public.idempotency_keys USING btree (expires_at) WHERE (status = ANY (ARRAY['completed'::text, 'failed'::text]));


--
-- Name: idempotency_keys_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idempotency_keys_public_id_key ON public.idempotency_keys USING btree (public_id);


--
-- Name: idempotency_keys_scope_key_hash_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idempotency_keys_scope_key_hash_key ON public.idempotency_keys USING btree (scope, idempotency_key_hash);


--
-- Name: machine_clients_default_tenant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX machine_clients_default_tenant_id_idx ON public.machine_clients USING btree (default_tenant_id);


--
-- Name: notifications_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX notifications_public_id_key ON public.notifications USING btree (public_id);


--
-- Name: notifications_recipient_unread_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notifications_recipient_unread_idx ON public.notifications USING btree (recipient_user_id, created_at DESC) WHERE (read_at IS NULL);


--
-- Name: notifications_tenant_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notifications_tenant_created_idx ON public.notifications USING btree (tenant_id, created_at DESC) WHERE (tenant_id IS NOT NULL);


--
-- Name: oauth_user_grants_provider_subject_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX oauth_user_grants_provider_subject_idx ON public.oauth_user_grants USING btree (provider, provider_subject);


--
-- Name: oauth_user_grants_resource_server_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX oauth_user_grants_resource_server_idx ON public.oauth_user_grants USING btree (resource_server);


--
-- Name: oauth_user_grants_tenant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX oauth_user_grants_tenant_id_idx ON public.oauth_user_grants USING btree (tenant_id);


--
-- Name: outbox_events_pending_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX outbox_events_pending_idx ON public.outbox_events USING btree (available_at, id) WHERE (status = ANY (ARRAY['pending'::text, 'failed'::text]));


--
-- Name: outbox_events_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX outbox_events_public_id_key ON public.outbox_events USING btree (public_id);


--
-- Name: outbox_events_tenant_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX outbox_events_tenant_created_idx ON public.outbox_events USING btree (tenant_id, created_at DESC) WHERE (tenant_id IS NOT NULL);


--
-- Name: support_access_sessions_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX support_access_sessions_public_id_key ON public.support_access_sessions USING btree (public_id);


--
-- Name: support_access_sessions_support_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX support_access_sessions_support_active_idx ON public.support_access_sessions USING btree (support_user_id, expires_at DESC) WHERE (status = 'active'::text);


--
-- Name: support_access_sessions_tenant_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX support_access_sessions_tenant_created_idx ON public.support_access_sessions USING btree (tenant_id, created_at DESC);


--
-- Name: tenant_data_exports_pending_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_data_exports_pending_idx ON public.tenant_data_exports USING btree (created_at, id) WHERE (status = ANY (ARRAY['pending'::text, 'processing'::text]));


--
-- Name: tenant_data_exports_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX tenant_data_exports_public_id_key ON public.tenant_data_exports USING btree (public_id);


--
-- Name: tenant_data_exports_tenant_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_data_exports_tenant_created_idx ON public.tenant_data_exports USING btree (tenant_id, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: tenant_entitlements_feature_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_entitlements_feature_idx ON public.tenant_entitlements USING btree (feature_code, tenant_id);


--
-- Name: tenant_invitations_expires_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_invitations_expires_idx ON public.tenant_invitations USING btree (expires_at) WHERE (status = 'pending'::text);


--
-- Name: tenant_invitations_pending_tenant_email_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_invitations_pending_tenant_email_idx ON public.tenant_invitations USING btree (tenant_id, invitee_email_normalized) WHERE (status = 'pending'::text);


--
-- Name: tenant_invitations_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX tenant_invitations_public_id_key ON public.tenant_invitations USING btree (public_id);


--
-- Name: tenant_invitations_token_hash_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX tenant_invitations_token_hash_key ON public.tenant_invitations USING btree (token_hash);


--
-- Name: tenant_memberships_role_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_memberships_role_id_idx ON public.tenant_memberships USING btree (role_id);


--
-- Name: tenant_memberships_tenant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_memberships_tenant_id_idx ON public.tenant_memberships USING btree (tenant_id);


--
-- Name: tenant_role_overrides_tenant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX tenant_role_overrides_tenant_id_idx ON public.tenant_role_overrides USING btree (tenant_id);


--
-- Name: todos_created_by_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX todos_created_by_user_id_idx ON public.todos USING btree (created_by_user_id);


--
-- Name: todos_public_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX todos_public_id_idx ON public.todos USING btree (public_id);


--
-- Name: todos_tenant_id_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX todos_tenant_id_created_at_idx ON public.todos USING btree (tenant_id, created_at DESC, id DESC);


--
-- Name: user_identities_provider_external_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX user_identities_provider_external_id_key ON public.user_identities USING btree (provider, external_id) WHERE (external_id IS NOT NULL);


--
-- Name: user_identities_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_identities_user_id_idx ON public.user_identities USING btree (user_id);


--
-- Name: user_roles_role_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_roles_role_id_idx ON public.user_roles USING btree (role_id);


--
-- Name: webhook_deliveries_endpoint_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX webhook_deliveries_endpoint_created_idx ON public.webhook_deliveries USING btree (webhook_endpoint_id, created_at DESC);


--
-- Name: webhook_deliveries_pending_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX webhook_deliveries_pending_idx ON public.webhook_deliveries USING btree (next_attempt_at, id) WHERE (status = ANY (ARRAY['pending'::text, 'failed'::text]));


--
-- Name: webhook_deliveries_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX webhook_deliveries_public_id_key ON public.webhook_deliveries USING btree (public_id);


--
-- Name: webhook_endpoints_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX webhook_endpoints_public_id_key ON public.webhook_endpoints USING btree (public_id);


--
-- Name: webhook_endpoints_tenant_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX webhook_endpoints_tenant_active_idx ON public.webhook_endpoints USING btree (tenant_id, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: audit_events audit_events_actor_machine_client_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_events
    ADD CONSTRAINT audit_events_actor_machine_client_id_fkey FOREIGN KEY (actor_machine_client_id) REFERENCES public.machine_clients(id) ON DELETE SET NULL;


--
-- Name: audit_events audit_events_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_events
    ADD CONSTRAINT audit_events_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: audit_events audit_events_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_events
    ADD CONSTRAINT audit_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE SET NULL;


--
-- Name: customer_signal_import_jobs customer_signal_import_jobs_error_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signal_import_jobs
    ADD CONSTRAINT customer_signal_import_jobs_error_file_object_id_fkey FOREIGN KEY (error_file_object_id) REFERENCES public.file_objects(id) ON DELETE SET NULL;


--
-- Name: customer_signal_import_jobs customer_signal_import_jobs_input_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signal_import_jobs
    ADD CONSTRAINT customer_signal_import_jobs_input_file_object_id_fkey FOREIGN KEY (input_file_object_id) REFERENCES public.file_objects(id) ON DELETE RESTRICT;


--
-- Name: customer_signal_import_jobs customer_signal_import_jobs_outbox_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signal_import_jobs
    ADD CONSTRAINT customer_signal_import_jobs_outbox_event_id_fkey FOREIGN KEY (outbox_event_id) REFERENCES public.outbox_events(id) ON DELETE SET NULL;


--
-- Name: customer_signal_import_jobs customer_signal_import_jobs_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signal_import_jobs
    ADD CONSTRAINT customer_signal_import_jobs_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: customer_signal_import_jobs customer_signal_import_jobs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signal_import_jobs
    ADD CONSTRAINT customer_signal_import_jobs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: customer_signal_saved_filters customer_signal_saved_filters_owner_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signal_saved_filters
    ADD CONSTRAINT customer_signal_saved_filters_owner_user_id_fkey FOREIGN KEY (owner_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: customer_signal_saved_filters customer_signal_saved_filters_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signal_saved_filters
    ADD CONSTRAINT customer_signal_saved_filters_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: customer_signals customer_signals_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signals
    ADD CONSTRAINT customer_signals_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: customer_signals customer_signals_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signals
    ADD CONSTRAINT customer_signals_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_admin_content_access_sessions drive_admin_content_access_sessions_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_admin_content_access_sessions
    ADD CONSTRAINT drive_admin_content_access_sessions_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: drive_admin_content_access_sessions drive_admin_content_access_sessions_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_admin_content_access_sessions
    ADD CONSTRAINT drive_admin_content_access_sessions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_file_revisions drive_file_revisions_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_file_revisions
    ADD CONSTRAINT drive_file_revisions_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_file_revisions drive_file_revisions_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_file_revisions
    ADD CONSTRAINT drive_file_revisions_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_file_revisions drive_file_revisions_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_file_revisions
    ADD CONSTRAINT drive_file_revisions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_folders drive_folders_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_folders
    ADD CONSTRAINT drive_folders_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_folders drive_folders_deleted_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_folders
    ADD CONSTRAINT drive_folders_deleted_by_user_id_fkey FOREIGN KEY (deleted_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_folders drive_folders_deleted_parent_folder_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_folders
    ADD CONSTRAINT drive_folders_deleted_parent_folder_id_fkey FOREIGN KEY (deleted_parent_folder_id) REFERENCES public.drive_folders(id) ON DELETE SET NULL;


--
-- Name: drive_folders drive_folders_legal_hold_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_folders
    ADD CONSTRAINT drive_folders_legal_hold_by_user_id_fkey FOREIGN KEY (legal_hold_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_folders drive_folders_parent_folder_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_folders
    ADD CONSTRAINT drive_folders_parent_folder_id_fkey FOREIGN KEY (parent_folder_id) REFERENCES public.drive_folders(id) ON DELETE SET NULL;


--
-- Name: drive_folders drive_folders_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_folders
    ADD CONSTRAINT drive_folders_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: drive_folders drive_folders_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_folders
    ADD CONSTRAINT drive_folders_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.drive_workspaces(id) ON DELETE RESTRICT;


--
-- Name: drive_group_external_mappings drive_group_external_mappings_drive_group_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_group_external_mappings
    ADD CONSTRAINT drive_group_external_mappings_drive_group_id_fkey FOREIGN KEY (drive_group_id) REFERENCES public.drive_groups(id) ON DELETE CASCADE;


--
-- Name: drive_group_external_mappings drive_group_external_mappings_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_group_external_mappings
    ADD CONSTRAINT drive_group_external_mappings_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_group_members drive_group_members_added_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_group_members
    ADD CONSTRAINT drive_group_members_added_by_user_id_fkey FOREIGN KEY (added_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_group_members drive_group_members_group_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_group_members
    ADD CONSTRAINT drive_group_members_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.drive_groups(id) ON DELETE CASCADE;


--
-- Name: drive_group_members drive_group_members_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_group_members
    ADD CONSTRAINT drive_group_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: drive_groups drive_groups_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_groups
    ADD CONSTRAINT drive_groups_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_groups drive_groups_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_groups
    ADD CONSTRAINT drive_groups_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: drive_resource_shares drive_resource_shares_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_resource_shares
    ADD CONSTRAINT drive_resource_shares_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_resource_shares drive_resource_shares_revoked_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_resource_shares
    ADD CONSTRAINT drive_resource_shares_revoked_by_user_id_fkey FOREIGN KEY (revoked_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_resource_shares drive_resource_shares_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_resource_shares
    ADD CONSTRAINT drive_resource_shares_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: drive_share_invitations drive_share_invitations_approved_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_share_invitations
    ADD CONSTRAINT drive_share_invitations_approved_by_user_id_fkey FOREIGN KEY (approved_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_share_invitations drive_share_invitations_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_share_invitations
    ADD CONSTRAINT drive_share_invitations_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_share_invitations drive_share_invitations_invitee_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_share_invitations
    ADD CONSTRAINT drive_share_invitations_invitee_user_id_fkey FOREIGN KEY (invitee_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_share_invitations drive_share_invitations_revoked_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_share_invitations
    ADD CONSTRAINT drive_share_invitations_revoked_by_user_id_fkey FOREIGN KEY (revoked_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_share_invitations drive_share_invitations_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_share_invitations
    ADD CONSTRAINT drive_share_invitations_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: drive_share_links drive_share_links_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_share_links
    ADD CONSTRAINT drive_share_links_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_share_links drive_share_links_disabled_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_share_links
    ADD CONSTRAINT drive_share_links_disabled_by_user_id_fkey FOREIGN KEY (disabled_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_share_links drive_share_links_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_share_links
    ADD CONSTRAINT drive_share_links_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: drive_workspaces drive_workspaces_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_workspaces
    ADD CONSTRAINT drive_workspaces_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_workspaces drive_workspaces_root_folder_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_workspaces
    ADD CONSTRAINT drive_workspaces_root_folder_id_fkey FOREIGN KEY (root_folder_id) REFERENCES public.drive_folders(id) ON DELETE SET NULL;


--
-- Name: drive_workspaces drive_workspaces_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_workspaces
    ADD CONSTRAINT drive_workspaces_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE RESTRICT;


--
-- Name: file_objects file_objects_deleted_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file_objects
    ADD CONSTRAINT file_objects_deleted_by_user_id_fkey FOREIGN KEY (deleted_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: file_objects file_objects_deleted_parent_folder_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file_objects
    ADD CONSTRAINT file_objects_deleted_parent_folder_id_fkey FOREIGN KEY (deleted_parent_folder_id) REFERENCES public.drive_folders(id) ON DELETE SET NULL;


--
-- Name: file_objects file_objects_drive_folder_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file_objects
    ADD CONSTRAINT file_objects_drive_folder_id_fkey FOREIGN KEY (drive_folder_id) REFERENCES public.drive_folders(id) ON DELETE SET NULL;


--
-- Name: file_objects file_objects_legal_hold_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file_objects
    ADD CONSTRAINT file_objects_legal_hold_by_user_id_fkey FOREIGN KEY (legal_hold_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: file_objects file_objects_locked_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file_objects
    ADD CONSTRAINT file_objects_locked_by_user_id_fkey FOREIGN KEY (locked_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: file_objects file_objects_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file_objects
    ADD CONSTRAINT file_objects_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: file_objects file_objects_uploaded_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file_objects
    ADD CONSTRAINT file_objects_uploaded_by_user_id_fkey FOREIGN KEY (uploaded_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: file_objects file_objects_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file_objects
    ADD CONSTRAINT file_objects_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.drive_workspaces(id) ON DELETE RESTRICT;


--
-- Name: idempotency_keys idempotency_keys_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.idempotency_keys
    ADD CONSTRAINT idempotency_keys_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: idempotency_keys idempotency_keys_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.idempotency_keys
    ADD CONSTRAINT idempotency_keys_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: machine_clients machine_clients_default_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.machine_clients
    ADD CONSTRAINT machine_clients_default_tenant_id_fkey FOREIGN KEY (default_tenant_id) REFERENCES public.tenants(id) ON DELETE SET NULL;


--
-- Name: notifications notifications_outbox_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_outbox_event_id_fkey FOREIGN KEY (outbox_event_id) REFERENCES public.outbox_events(id) ON DELETE SET NULL;


--
-- Name: notifications notifications_recipient_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_recipient_user_id_fkey FOREIGN KEY (recipient_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: notifications notifications_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: oauth_user_grants oauth_user_grants_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: oauth_user_grants oauth_user_grants_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.oauth_user_grants
    ADD CONSTRAINT oauth_user_grants_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: outbox_events outbox_events_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE SET NULL;


--
-- Name: support_access_sessions support_access_sessions_impersonated_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.support_access_sessions
    ADD CONSTRAINT support_access_sessions_impersonated_user_id_fkey FOREIGN KEY (impersonated_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: support_access_sessions support_access_sessions_support_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.support_access_sessions
    ADD CONSTRAINT support_access_sessions_support_user_id_fkey FOREIGN KEY (support_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: support_access_sessions support_access_sessions_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.support_access_sessions
    ADD CONSTRAINT support_access_sessions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_data_exports tenant_data_exports_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_data_exports
    ADD CONSTRAINT tenant_data_exports_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE SET NULL;


--
-- Name: tenant_data_exports tenant_data_exports_outbox_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_data_exports
    ADD CONSTRAINT tenant_data_exports_outbox_event_id_fkey FOREIGN KEY (outbox_event_id) REFERENCES public.outbox_events(id) ON DELETE SET NULL;


--
-- Name: tenant_data_exports tenant_data_exports_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_data_exports
    ADD CONSTRAINT tenant_data_exports_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: tenant_data_exports tenant_data_exports_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_data_exports
    ADD CONSTRAINT tenant_data_exports_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_entitlements tenant_entitlements_feature_code_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_entitlements
    ADD CONSTRAINT tenant_entitlements_feature_code_fkey FOREIGN KEY (feature_code) REFERENCES public.feature_definitions(code) ON DELETE CASCADE;


--
-- Name: tenant_entitlements tenant_entitlements_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_entitlements
    ADD CONSTRAINT tenant_entitlements_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_invitations tenant_invitations_accepted_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_invitations
    ADD CONSTRAINT tenant_invitations_accepted_by_user_id_fkey FOREIGN KEY (accepted_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: tenant_invitations tenant_invitations_invited_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_invitations
    ADD CONSTRAINT tenant_invitations_invited_by_user_id_fkey FOREIGN KEY (invited_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: tenant_invitations tenant_invitations_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_invitations
    ADD CONSTRAINT tenant_invitations_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_memberships tenant_memberships_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_memberships
    ADD CONSTRAINT tenant_memberships_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: tenant_memberships tenant_memberships_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_memberships
    ADD CONSTRAINT tenant_memberships_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_memberships tenant_memberships_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_memberships
    ADD CONSTRAINT tenant_memberships_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: tenant_role_overrides tenant_role_overrides_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_role_overrides
    ADD CONSTRAINT tenant_role_overrides_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: tenant_role_overrides tenant_role_overrides_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_role_overrides
    ADD CONSTRAINT tenant_role_overrides_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_role_overrides tenant_role_overrides_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_role_overrides
    ADD CONSTRAINT tenant_role_overrides_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: tenant_settings tenant_settings_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_settings
    ADD CONSTRAINT tenant_settings_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: todos todos_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.todos
    ADD CONSTRAINT todos_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: todos todos_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.todos
    ADD CONSTRAINT todos_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: user_identities user_identities_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_roles
    ADD CONSTRAINT user_roles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: users users_default_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_default_tenant_id_fkey FOREIGN KEY (default_tenant_id) REFERENCES public.tenants(id) ON DELETE SET NULL;


--
-- Name: webhook_deliveries webhook_deliveries_outbox_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.webhook_deliveries
    ADD CONSTRAINT webhook_deliveries_outbox_event_id_fkey FOREIGN KEY (outbox_event_id) REFERENCES public.outbox_events(id) ON DELETE SET NULL;


--
-- Name: webhook_deliveries webhook_deliveries_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.webhook_deliveries
    ADD CONSTRAINT webhook_deliveries_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: webhook_deliveries webhook_deliveries_webhook_endpoint_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.webhook_deliveries
    ADD CONSTRAINT webhook_deliveries_webhook_endpoint_id_fkey FOREIGN KEY (webhook_endpoint_id) REFERENCES public.webhook_endpoints(id) ON DELETE CASCADE;


--
-- Name: webhook_endpoints webhook_endpoints_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.webhook_endpoints
    ADD CONSTRAINT webhook_endpoints_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: webhook_endpoints webhook_endpoints_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.webhook_endpoints
    ADD CONSTRAINT webhook_endpoints_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--
