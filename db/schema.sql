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
    CONSTRAINT file_objects_byte_size_check CHECK ((byte_size >= 0)),
    CONSTRAINT file_objects_purpose_check CHECK ((purpose = ANY (ARRAY['attachment'::text, 'avatar'::text, 'import'::text, 'export'::text]))),
    CONSTRAINT file_objects_status_check CHECK ((status = ANY (ARRAY['active'::text, 'deleted'::text])))
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
-- Name: audit_events audit_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_events
    ADD CONSTRAINT audit_events_pkey PRIMARY KEY (id);


--
-- Name: customer_signals customer_signals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_signals
    ADD CONSTRAINT customer_signals_pkey PRIMARY KEY (id);


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
-- Name: tenant_data_exports tenant_data_exports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tenant_data_exports
    ADD CONSTRAINT tenant_data_exports_pkey PRIMARY KEY (id);


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
-- Name: customer_signals_tenant_status_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX customer_signals_tenant_status_created_at_idx ON public.customer_signals USING btree (tenant_id, status, created_at DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: file_objects_attachment_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX file_objects_attachment_idx ON public.file_objects USING btree (tenant_id, attached_to_type, attached_to_id, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: file_objects_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX file_objects_public_id_key ON public.file_objects USING btree (public_id);


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
-- P10 cross-cutting extension schema additions.
--

CREATE TABLE public.feature_definitions (
    code text PRIMARY KEY,
    display_name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    default_enabled boolean DEFAULT false NOT NULL,
    default_limit jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT feature_definitions_code_check CHECK ((btrim(code) <> ''::text)),
    CONSTRAINT feature_definitions_display_name_check CHECK ((btrim(display_name) <> ''::text))
);

CREATE TABLE public.tenant_entitlements (
    tenant_id bigint NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    feature_code text NOT NULL REFERENCES public.feature_definitions(code) ON DELETE CASCADE,
    enabled boolean NOT NULL,
    limit_value jsonb DEFAULT '{}'::jsonb NOT NULL,
    source text DEFAULT 'manual'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (tenant_id, feature_code),
    CONSTRAINT tenant_entitlements_source_check CHECK ((source = ANY (ARRAY['default'::text, 'manual'::text, 'billing'::text, 'migration'::text])))
);

CREATE INDEX tenant_entitlements_feature_idx ON public.tenant_entitlements USING btree (feature_code, tenant_id);

CREATE TABLE public.webhook_endpoints (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    created_by_user_id bigint REFERENCES public.users(id) ON DELETE SET NULL,
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
    CONSTRAINT webhook_endpoints_url_check CHECK ((btrim(url) <> ''::text)),
    CONSTRAINT webhook_endpoints_secret_key_version_check CHECK ((secret_key_version > 0))
);

CREATE UNIQUE INDEX webhook_endpoints_public_id_key ON public.webhook_endpoints USING btree (public_id);
CREATE INDEX webhook_endpoints_tenant_active_idx ON public.webhook_endpoints USING btree (tenant_id, created_at DESC) WHERE (deleted_at IS NULL);

CREATE TABLE public.webhook_deliveries (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    webhook_endpoint_id bigint NOT NULL REFERENCES public.webhook_endpoints(id) ON DELETE CASCADE,
    outbox_event_id bigint REFERENCES public.outbox_events(id) ON DELETE SET NULL,
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
    CONSTRAINT webhook_deliveries_event_type_check CHECK ((btrim(event_type) <> ''::text)),
    CONSTRAINT webhook_deliveries_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'delivered'::text, 'failed'::text, 'dead'::text]))),
    CONSTRAINT webhook_deliveries_attempt_count_check CHECK ((attempt_count >= 0)),
    CONSTRAINT webhook_deliveries_max_attempts_check CHECK ((max_attempts > 0))
);

CREATE UNIQUE INDEX webhook_deliveries_public_id_key ON public.webhook_deliveries USING btree (public_id);
CREATE INDEX webhook_deliveries_endpoint_created_idx ON public.webhook_deliveries USING btree (webhook_endpoint_id, created_at DESC);
CREATE INDEX webhook_deliveries_pending_idx ON public.webhook_deliveries USING btree (next_attempt_at, id) WHERE (status = ANY (ARRAY['pending'::text, 'failed'::text]));

CREATE TABLE public.customer_signal_import_jobs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    requested_by_user_id bigint REFERENCES public.users(id) ON DELETE SET NULL,
    input_file_object_id bigint NOT NULL REFERENCES public.file_objects(id) ON DELETE RESTRICT,
    error_file_object_id bigint REFERENCES public.file_objects(id) ON DELETE SET NULL,
    outbox_event_id bigint REFERENCES public.outbox_events(id) ON DELETE SET NULL,
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
    CONSTRAINT customer_signal_import_jobs_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'processing'::text, 'completed'::text, 'failed'::text]))),
    CONSTRAINT customer_signal_import_jobs_total_rows_check CHECK ((total_rows >= 0)),
    CONSTRAINT customer_signal_import_jobs_valid_rows_check CHECK ((valid_rows >= 0)),
    CONSTRAINT customer_signal_import_jobs_invalid_rows_check CHECK ((invalid_rows >= 0)),
    CONSTRAINT customer_signal_import_jobs_inserted_rows_check CHECK ((inserted_rows >= 0))
);

CREATE UNIQUE INDEX customer_signal_import_jobs_public_id_key ON public.customer_signal_import_jobs USING btree (public_id);
CREATE INDEX customer_signal_import_jobs_tenant_created_idx ON public.customer_signal_import_jobs USING btree (tenant_id, created_at DESC) WHERE (deleted_at IS NULL);
CREATE INDEX customer_signal_import_jobs_pending_idx ON public.customer_signal_import_jobs USING btree (created_at, id) WHERE (status = ANY (ARRAY['pending'::text, 'processing'::text]));

CREATE TABLE public.customer_signal_saved_filters (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    owner_user_id bigint NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    name text NOT NULL,
    query text DEFAULT ''::text NOT NULL,
    filters jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT customer_signal_saved_filters_name_check CHECK ((btrim(name) <> ''::text))
);

CREATE UNIQUE INDEX customer_signal_saved_filters_public_id_key ON public.customer_signal_saved_filters USING btree (public_id);
CREATE INDEX customer_signal_saved_filters_owner_idx ON public.customer_signal_saved_filters USING btree (tenant_id, owner_user_id, created_at DESC) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX customer_signal_saved_filters_owner_name_key ON public.customer_signal_saved_filters USING btree (tenant_id, owner_user_id, lower(name)) WHERE (deleted_at IS NULL);
CREATE INDEX customer_signals_tenant_search_idx ON public.customer_signals USING gin (to_tsvector('simple'::regconfig, (((customer_name || ' '::text) || title) || ' '::text) || body)) WHERE (deleted_at IS NULL);

CREATE TABLE public.support_access_sessions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    support_user_id bigint NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    impersonated_user_id bigint NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    tenant_id bigint NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    reason text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    ended_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT support_access_sessions_reason_check CHECK ((btrim(reason) <> ''::text)),
    CONSTRAINT support_access_sessions_status_check CHECK ((status = ANY (ARRAY['active'::text, 'ended'::text, 'expired'::text]))),
    CONSTRAINT support_access_sessions_users_check CHECK ((support_user_id <> impersonated_user_id)),
    CONSTRAINT support_access_sessions_expires_check CHECK ((expires_at > started_at))
);

CREATE UNIQUE INDEX support_access_sessions_public_id_key ON public.support_access_sessions USING btree (public_id);
CREATE INDEX support_access_sessions_support_active_idx ON public.support_access_sessions USING btree (support_user_id, expires_at DESC) WHERE (status = 'active'::text);
CREATE INDEX support_access_sessions_tenant_created_idx ON public.support_access_sessions USING btree (tenant_id, created_at DESC);

--
-- PostgreSQL database dump complete
--
