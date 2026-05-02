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
-- Name: dataset_columns; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_columns (
    id bigint NOT NULL,
    dataset_id bigint NOT NULL,
    ordinal integer NOT NULL,
    original_name text NOT NULL,
    column_name text NOT NULL,
    clickhouse_type text DEFAULT 'Nullable(String)'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT dataset_columns_clickhouse_type_check CHECK ((btrim(clickhouse_type) <> ''::text)),
    CONSTRAINT dataset_columns_column_name_check CHECK ((btrim(column_name) <> ''::text)),
    CONSTRAINT dataset_columns_ordinal_check CHECK ((ordinal > 0))
);


--
-- Name: dataset_columns_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_columns ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_columns_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dataset_gold_publications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_gold_publications (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    source_work_table_id bigint NOT NULL,
    created_by_user_id bigint,
    updated_by_user_id bigint,
    published_by_user_id bigint,
    unpublished_by_user_id bigint,
    archived_by_user_id bigint,
    last_publish_run_id bigint,
    display_name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    gold_database text NOT NULL,
    gold_table text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    row_count bigint DEFAULT 0 NOT NULL,
    total_bytes bigint DEFAULT 0 NOT NULL,
    schema_summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    refresh_policy text DEFAULT 'manual'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    published_at timestamp with time zone,
    unpublished_at timestamp with time zone,
    archived_at timestamp with time zone,
    CONSTRAINT dataset_gold_publications_display_name_check CHECK ((btrim(display_name) <> ''::text)),
    CONSTRAINT dataset_gold_publications_gold_database_check CHECK ((btrim(gold_database) <> ''::text)),
    CONSTRAINT dataset_gold_publications_gold_table_check CHECK ((btrim(gold_table) <> ''::text)),
    CONSTRAINT dataset_gold_publications_refresh_policy_check CHECK ((refresh_policy = 'manual'::text)),
    CONSTRAINT dataset_gold_publications_row_count_check CHECK ((row_count >= 0)),
    CONSTRAINT dataset_gold_publications_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'active'::text, 'failed'::text, 'unpublished'::text, 'archived'::text]))),
    CONSTRAINT dataset_gold_publications_total_bytes_check CHECK ((total_bytes >= 0))
);


--
-- Name: dataset_gold_publications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_gold_publications ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_gold_publications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dataset_gold_publish_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_gold_publish_runs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    publication_id bigint NOT NULL,
    source_work_table_id bigint NOT NULL,
    requested_by_user_id bigint,
    outbox_event_id bigint,
    status text DEFAULT 'pending'::text NOT NULL,
    gold_database text NOT NULL,
    gold_table text NOT NULL,
    internal_database text NOT NULL,
    internal_table text NOT NULL,
    row_count bigint DEFAULT 0 NOT NULL,
    total_bytes bigint DEFAULT 0 NOT NULL,
    schema_summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    error_summary text,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT dataset_gold_publish_runs_gold_database_check CHECK ((btrim(gold_database) <> ''::text)),
    CONSTRAINT dataset_gold_publish_runs_gold_table_check CHECK ((btrim(gold_table) <> ''::text)),
    CONSTRAINT dataset_gold_publish_runs_internal_database_check CHECK ((btrim(internal_database) <> ''::text)),
    CONSTRAINT dataset_gold_publish_runs_internal_table_check CHECK ((btrim(internal_table) <> ''::text)),
    CONSTRAINT dataset_gold_publish_runs_row_count_check CHECK ((row_count >= 0)),
    CONSTRAINT dataset_gold_publish_runs_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'processing'::text, 'completed'::text, 'failed'::text]))),
    CONSTRAINT dataset_gold_publish_runs_total_bytes_check CHECK ((total_bytes >= 0))
);


--
-- Name: dataset_gold_publish_runs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_gold_publish_runs ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_gold_publish_runs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dataset_import_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_import_jobs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    dataset_id bigint NOT NULL,
    source_file_object_id bigint NOT NULL,
    requested_by_user_id bigint,
    outbox_event_id bigint,
    status text DEFAULT 'pending'::text NOT NULL,
    total_rows bigint DEFAULT 0 NOT NULL,
    valid_rows bigint DEFAULT 0 NOT NULL,
    invalid_rows bigint DEFAULT 0 NOT NULL,
    error_sample jsonb DEFAULT '[]'::jsonb NOT NULL,
    error_summary text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    CONSTRAINT dataset_import_jobs_invalid_rows_check CHECK ((invalid_rows >= 0)),
    CONSTRAINT dataset_import_jobs_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'processing'::text, 'completed'::text, 'failed'::text]))),
    CONSTRAINT dataset_import_jobs_total_rows_check CHECK ((total_rows >= 0)),
    CONSTRAINT dataset_import_jobs_valid_rows_check CHECK ((valid_rows >= 0))
);


--
-- Name: dataset_import_jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_import_jobs ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_import_jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dataset_lineage_change_sets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_lineage_change_sets (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    query_job_id bigint,
    root_resource_type text NOT NULL,
    root_resource_public_id uuid,
    source_kind text NOT NULL,
    status text DEFAULT 'draft'::text NOT NULL,
    title text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    created_by_user_id bigint,
    published_by_user_id bigint,
    rejected_by_user_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    published_at timestamp with time zone,
    rejected_at timestamp with time zone,
    archived_at timestamp with time zone,
    CONSTRAINT dataset_lineage_change_sets_root_resource_type_check CHECK ((btrim(root_resource_type) <> ''::text)),
    CONSTRAINT dataset_lineage_change_sets_source_kind_check CHECK ((source_kind = ANY (ARRAY['parser'::text, 'manual'::text]))),
    CONSTRAINT dataset_lineage_change_sets_status_check CHECK ((status = ANY (ARRAY['draft'::text, 'published'::text, 'rejected'::text, 'archived'::text]))),
    CONSTRAINT dataset_lineage_change_sets_title_check CHECK ((btrim(title) <> ''::text))
);


--
-- Name: dataset_lineage_change_sets_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_lineage_change_sets ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_lineage_change_sets_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dataset_lineage_edges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_lineage_edges (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    change_set_id bigint NOT NULL,
    edge_key text NOT NULL,
    source_node_key text NOT NULL,
    target_node_key text NOT NULL,
    relation_type text NOT NULL,
    source_kind text NOT NULL,
    confidence text NOT NULL,
    label text DEFAULT ''::text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    expression text DEFAULT ''::text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT dataset_lineage_edges_confidence_check CHECK ((confidence = ANY (ARRAY['parser_exact'::text, 'parser_partial'::text, 'manual'::text]))),
    CONSTRAINT dataset_lineage_edges_edge_key_check CHECK ((btrim(edge_key) <> ''::text)),
    CONSTRAINT dataset_lineage_edges_no_self_loop_check CHECK ((source_node_key <> target_node_key)),
    CONSTRAINT dataset_lineage_edges_relation_type_check CHECK ((btrim(relation_type) <> ''::text)),
    CONSTRAINT dataset_lineage_edges_source_kind_check CHECK ((source_kind = ANY (ARRAY['parser'::text, 'manual'::text])))
);


--
-- Name: dataset_lineage_edges_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_lineage_edges ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_lineage_edges_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dataset_lineage_nodes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_lineage_nodes (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    change_set_id bigint NOT NULL,
    node_key text NOT NULL,
    node_kind text NOT NULL,
    source_kind text NOT NULL,
    resource_type text NOT NULL,
    resource_public_id uuid,
    parent_node_key text,
    column_name text,
    label text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    position_x double precision,
    position_y double precision,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT dataset_lineage_nodes_label_check CHECK ((btrim(label) <> ''::text)),
    CONSTRAINT dataset_lineage_nodes_node_key_check CHECK ((btrim(node_key) <> ''::text)),
    CONSTRAINT dataset_lineage_nodes_node_kind_check CHECK ((node_kind = ANY (ARRAY['resource'::text, 'column'::text, 'custom'::text]))),
    CONSTRAINT dataset_lineage_nodes_resource_shape_check CHECK ((((node_kind = 'resource'::text) AND (resource_public_id IS NOT NULL)) OR ((node_kind = 'column'::text) AND (column_name IS NOT NULL) AND (btrim(column_name) <> ''::text)) OR ((node_kind = 'custom'::text) AND (resource_public_id IS NULL)))),
    CONSTRAINT dataset_lineage_nodes_resource_type_check CHECK ((btrim(resource_type) <> ''::text)),
    CONSTRAINT dataset_lineage_nodes_source_kind_check CHECK ((source_kind = ANY (ARRAY['parser'::text, 'manual'::text])))
);


--
-- Name: dataset_lineage_nodes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_lineage_nodes ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_lineage_nodes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dataset_lineage_parse_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_lineage_parse_runs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    query_job_id bigint NOT NULL,
    change_set_id bigint,
    requested_by_user_id bigint,
    status text DEFAULT 'processing'::text NOT NULL,
    table_ref_count integer DEFAULT 0 NOT NULL,
    column_edge_count integer DEFAULT 0 NOT NULL,
    error_summary text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    CONSTRAINT dataset_lineage_parse_runs_column_edge_count_check CHECK ((column_edge_count >= 0)),
    CONSTRAINT dataset_lineage_parse_runs_status_check CHECK ((status = ANY (ARRAY['processing'::text, 'completed'::text, 'failed'::text]))),
    CONSTRAINT dataset_lineage_parse_runs_table_ref_count_check CHECK ((table_ref_count >= 0))
);


--
-- Name: dataset_lineage_parse_runs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_lineage_parse_runs ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_lineage_parse_runs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dataset_query_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_query_jobs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    requested_by_user_id bigint,
    statement text NOT NULL,
    status text DEFAULT 'running'::text NOT NULL,
    result_columns jsonb DEFAULT '[]'::jsonb NOT NULL,
    result_rows jsonb DEFAULT '[]'::jsonb NOT NULL,
    row_count integer DEFAULT 0 NOT NULL,
    error_summary text,
    duration_ms bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    dataset_id bigint,
    CONSTRAINT dataset_query_jobs_duration_ms_check CHECK ((duration_ms >= 0)),
    CONSTRAINT dataset_query_jobs_row_count_check CHECK ((row_count >= 0)),
    CONSTRAINT dataset_query_jobs_statement_check CHECK ((btrim(statement) <> ''::text)),
    CONSTRAINT dataset_query_jobs_status_check CHECK ((status = ANY (ARRAY['running'::text, 'completed'::text, 'failed'::text])))
);


--
-- Name: dataset_query_jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_query_jobs ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_query_jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dataset_sync_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_sync_jobs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    dataset_id bigint NOT NULL,
    source_work_table_id bigint NOT NULL,
    requested_by_user_id bigint,
    outbox_event_id bigint,
    mode text DEFAULT 'full_refresh'::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    old_raw_database text NOT NULL,
    old_raw_table text NOT NULL,
    new_raw_database text NOT NULL,
    new_raw_table text NOT NULL,
    row_count bigint DEFAULT 0 NOT NULL,
    total_bytes bigint DEFAULT 0 NOT NULL,
    error_summary text,
    cleanup_status text,
    cleanup_error_summary text,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT dataset_sync_jobs_cleanup_status_check CHECK (((cleanup_status IS NULL) OR (cleanup_status = ANY (ARRAY['completed'::text, 'failed'::text, 'skipped'::text])))),
    CONSTRAINT dataset_sync_jobs_mode_check CHECK ((mode = 'full_refresh'::text)),
    CONSTRAINT dataset_sync_jobs_new_raw_database_check CHECK ((btrim(new_raw_database) <> ''::text)),
    CONSTRAINT dataset_sync_jobs_new_raw_table_check CHECK ((btrim(new_raw_table) <> ''::text)),
    CONSTRAINT dataset_sync_jobs_old_raw_database_check CHECK ((btrim(old_raw_database) <> ''::text)),
    CONSTRAINT dataset_sync_jobs_old_raw_table_check CHECK ((btrim(old_raw_table) <> ''::text)),
    CONSTRAINT dataset_sync_jobs_row_count_check CHECK ((row_count >= 0)),
    CONSTRAINT dataset_sync_jobs_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'processing'::text, 'completed'::text, 'failed'::text]))),
    CONSTRAINT dataset_sync_jobs_total_bytes_check CHECK ((total_bytes >= 0))
);


--
-- Name: dataset_sync_jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_sync_jobs ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_sync_jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dataset_work_table_export_schedules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_work_table_export_schedules (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    work_table_id bigint NOT NULL,
    created_by_user_id bigint,
    format text DEFAULT 'csv'::text NOT NULL,
    frequency text NOT NULL,
    timezone text NOT NULL,
    run_time text NOT NULL,
    weekday smallint,
    month_day smallint,
    retention_days integer DEFAULT 7 NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    next_run_at timestamp with time zone NOT NULL,
    last_run_at timestamp with time zone,
    last_status text,
    last_error_summary text,
    last_export_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT dataset_work_table_export_schedules_format_check CHECK ((format = ANY (ARRAY['csv'::text, 'json'::text, 'parquet'::text]))),
    CONSTRAINT dataset_work_table_export_schedules_frequency_check CHECK ((frequency = ANY (ARRAY['daily'::text, 'weekly'::text, 'monthly'::text]))),
    CONSTRAINT dataset_work_table_export_schedules_frequency_shape_check CHECK ((((frequency = 'daily'::text) AND (weekday IS NULL) AND (month_day IS NULL)) OR ((frequency = 'weekly'::text) AND (weekday IS NOT NULL) AND (month_day IS NULL)) OR ((frequency = 'monthly'::text) AND (weekday IS NULL) AND (month_day IS NOT NULL)))),
    CONSTRAINT dataset_work_table_export_schedules_last_status_check CHECK (((last_status IS NULL) OR (last_status = ANY (ARRAY['created'::text, 'skipped'::text, 'failed'::text, 'ready'::text, 'disabled'::text])))),
    CONSTRAINT dataset_work_table_export_schedules_month_day_check CHECK (((month_day IS NULL) OR ((month_day >= 1) AND (month_day <= 28)))),
    CONSTRAINT dataset_work_table_export_schedules_retention_days_check CHECK (((retention_days >= 1) AND (retention_days <= 365))),
    CONSTRAINT dataset_work_table_export_schedules_run_time_check CHECK ((run_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'::text)),
    CONSTRAINT dataset_work_table_export_schedules_weekday_check CHECK (((weekday IS NULL) OR ((weekday >= 1) AND (weekday <= 7))))
);


--
-- Name: dataset_work_table_export_schedules_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_work_table_export_schedules ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_work_table_export_schedules_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dataset_work_table_exports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_work_table_exports (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    work_table_id bigint NOT NULL,
    requested_by_user_id bigint,
    file_object_id bigint,
    outbox_event_id bigint,
    format text DEFAULT 'csv'::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    expires_at timestamp with time zone DEFAULT (now() + '7 days'::interval) NOT NULL,
    error_summary text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    deleted_at timestamp with time zone,
    schedule_id bigint,
    scheduled_for timestamp with time zone,
    CONSTRAINT dataset_work_table_exports_format_check CHECK ((format = ANY (ARRAY['csv'::text, 'json'::text, 'parquet'::text]))),
    CONSTRAINT dataset_work_table_exports_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'processing'::text, 'ready'::text, 'failed'::text, 'deleted'::text])))
);


--
-- Name: dataset_work_table_exports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_work_table_exports ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_work_table_exports_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dataset_work_tables; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dataset_work_tables (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    source_dataset_id bigint,
    created_from_query_job_id bigint,
    created_by_user_id bigint,
    work_database text NOT NULL,
    work_table text NOT NULL,
    display_name text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    row_count bigint DEFAULT 0 NOT NULL,
    total_bytes bigint DEFAULT 0 NOT NULL,
    engine text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    dropped_at timestamp with time zone,
    CONSTRAINT dataset_work_tables_display_name_check CHECK ((btrim(display_name) <> ''::text)),
    CONSTRAINT dataset_work_tables_row_count_check CHECK ((row_count >= 0)),
    CONSTRAINT dataset_work_tables_status_check CHECK ((status = ANY (ARRAY['active'::text, 'dropped'::text]))),
    CONSTRAINT dataset_work_tables_total_bytes_check CHECK ((total_bytes >= 0)),
    CONSTRAINT dataset_work_tables_work_database_check CHECK ((btrim(work_database) <> ''::text)),
    CONSTRAINT dataset_work_tables_work_table_check CHECK ((btrim(work_table) <> ''::text))
);


--
-- Name: dataset_work_tables_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dataset_work_tables ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.dataset_work_tables_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: datasets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.datasets (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    created_by_user_id bigint,
    source_file_object_id bigint,
    name text NOT NULL,
    original_filename text NOT NULL,
    content_type text NOT NULL,
    byte_size bigint NOT NULL,
    raw_database text NOT NULL,
    raw_table text NOT NULL,
    work_database text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    row_count bigint DEFAULT 0 NOT NULL,
    error_summary text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    imported_at timestamp with time zone,
    deleted_at timestamp with time zone,
    source_kind text DEFAULT 'file'::text NOT NULL,
    source_work_table_id bigint,
    CONSTRAINT datasets_byte_size_check CHECK ((byte_size >= 0)),
    CONSTRAINT datasets_file_source_check CHECK (((source_kind <> 'file'::text) OR (source_file_object_id IS NOT NULL))),
    CONSTRAINT datasets_name_check CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT datasets_raw_database_check CHECK ((btrim(raw_database) <> ''::text)),
    CONSTRAINT datasets_raw_table_check CHECK ((btrim(raw_table) <> ''::text)),
    CONSTRAINT datasets_row_count_check CHECK ((row_count >= 0)),
    CONSTRAINT datasets_source_kind_check CHECK ((source_kind = ANY (ARRAY['file'::text, 'work_table'::text]))),
    CONSTRAINT datasets_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'importing'::text, 'ready'::text, 'failed'::text, 'deleted'::text]))),
    CONSTRAINT datasets_work_database_check CHECK ((btrim(work_database) <> ''::text)),
    CONSTRAINT datasets_work_table_source_check CHECK (((source_kind <> 'work_table'::text) OR (source_work_table_id IS NOT NULL)))
);


--
-- Name: datasets_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.datasets ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.datasets_id_seq
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
-- Name: drive_ai_classifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_ai_classifications (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    file_revision text DEFAULT '1'::text NOT NULL,
    label text NOT NULL,
    confidence numeric(5,4) NOT NULL,
    provider text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: drive_ai_classifications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_ai_classifications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_ai_classifications_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_ai_classifications_id_seq OWNED BY public.drive_ai_classifications.id;


--
-- Name: drive_ai_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_ai_jobs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    file_revision text DEFAULT '1'::text NOT NULL,
    job_type text NOT NULL,
    provider text NOT NULL,
    status text DEFAULT 'completed'::text NOT NULL,
    requested_by_user_id bigint,
    error_message text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_ai_jobs_job_type_check CHECK ((job_type = ANY (ARRAY['classification'::text, 'summary'::text]))),
    CONSTRAINT drive_ai_jobs_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'completed'::text, 'failed'::text, 'denied'::text])))
);


--
-- Name: drive_ai_jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_ai_jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_ai_jobs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_ai_jobs_id_seq OWNED BY public.drive_ai_jobs.id;


--
-- Name: drive_ai_summaries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_ai_summaries (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    file_revision text DEFAULT '1'::text NOT NULL,
    summary_text text NOT NULL,
    provider text NOT NULL,
    input_hash text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: drive_ai_summaries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_ai_summaries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_ai_summaries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_ai_summaries_id_seq OWNED BY public.drive_ai_summaries.id;


--
-- Name: drive_app_webhook_deliveries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_app_webhook_deliveries (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    installation_id bigint NOT NULL,
    event_type text NOT NULL,
    payload_hash text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    next_attempt_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_app_webhook_deliveries_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'sent'::text, 'failed'::text, 'stopped'::text])))
);


--
-- Name: drive_app_webhook_deliveries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_app_webhook_deliveries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_app_webhook_deliveries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_app_webhook_deliveries_id_seq OWNED BY public.drive_app_webhook_deliveries.id;


--
-- Name: drive_chain_of_custody_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_chain_of_custody_events (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    case_id bigint,
    export_id bigint,
    actor_user_id bigint,
    action text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_chain_of_custody_events_action_check CHECK ((btrim(action) <> ''::text))
);


--
-- Name: drive_chain_of_custody_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_chain_of_custody_events ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_chain_of_custody_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_clean_room_datasets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_clean_room_datasets (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    clean_room_id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    source_file_object_id bigint NOT NULL,
    submitted_by_user_id bigint NOT NULL,
    status text DEFAULT 'submitted'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_clean_room_datasets_status_check CHECK ((status = ANY (ARRAY['submitted'::text, 'accepted'::text, 'rejected'::text, 'removed'::text])))
);


--
-- Name: drive_clean_room_datasets_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_clean_room_datasets ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_clean_room_datasets_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_clean_room_exports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_clean_room_exports (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    clean_room_id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    job_id bigint,
    requested_by_user_id bigint NOT NULL,
    approved_by_user_id bigint,
    status text DEFAULT 'pending_approval'::text NOT NULL,
    raw_dataset_export boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_clean_room_exports_status_check CHECK ((status = ANY (ARRAY['pending_approval'::text, 'approved'::text, 'ready'::text, 'denied'::text])))
);


--
-- Name: drive_clean_room_exports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_clean_room_exports ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_clean_room_exports_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_clean_room_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_clean_room_jobs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    clean_room_id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    job_type text DEFAULT 'local_fake'::text NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    result_metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by_user_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_clean_room_jobs_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'running'::text, 'ready'::text, 'failed'::text])))
);


--
-- Name: drive_clean_room_jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_clean_room_jobs ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_clean_room_jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_clean_room_participants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_clean_room_participants (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    clean_room_id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    participant_tenant_id bigint NOT NULL,
    user_id bigint,
    role text DEFAULT 'participant'::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_clean_room_participants_role_check CHECK ((role = ANY (ARRAY['owner'::text, 'participant'::text, 'reviewer'::text]))),
    CONSTRAINT drive_clean_room_participants_status_check CHECK ((status = ANY (ARRAY['active'::text, 'revoked'::text])))
);


--
-- Name: drive_clean_room_participants_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_clean_room_participants ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_clean_room_participants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_clean_room_policy_decisions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_clean_room_policy_decisions (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    clean_room_id bigint,
    tenant_id bigint NOT NULL,
    actor_tenant_id bigint,
    resource_tenant_id bigint,
    decision text NOT NULL,
    reason text DEFAULT ''::text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_clean_room_policy_decisions_decision_check CHECK ((decision = ANY (ARRAY['allow'::text, 'deny'::text])))
);


--
-- Name: drive_clean_room_policy_decisions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_clean_room_policy_decisions ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_clean_room_policy_decisions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_clean_rooms; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_clean_rooms (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    name text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    policy jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by_user_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_clean_rooms_name_check CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT drive_clean_rooms_status_check CHECK ((status = ANY (ARRAY['active'::text, 'closed'::text, 'archived'::text])))
);


--
-- Name: drive_clean_rooms_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_clean_rooms ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_clean_rooms_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_e2ee_file_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_e2ee_file_keys (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    key_version integer DEFAULT 1 NOT NULL,
    encryption_algorithm text NOT NULL,
    ciphertext_sha256 text NOT NULL,
    encrypted_metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by_user_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: drive_e2ee_file_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_e2ee_file_keys_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_e2ee_file_keys_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_e2ee_file_keys_id_seq OWNED BY public.drive_e2ee_file_keys.id;


--
-- Name: drive_e2ee_key_envelopes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_e2ee_key_envelopes (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    file_key_id bigint NOT NULL,
    recipient_user_id bigint NOT NULL,
    recipient_key_id bigint NOT NULL,
    wrapped_file_key bytea NOT NULL,
    wrap_algorithm text NOT NULL,
    created_by_user_id bigint NOT NULL,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: drive_e2ee_key_envelopes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_e2ee_key_envelopes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_e2ee_key_envelopes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_e2ee_key_envelopes_id_seq OWNED BY public.drive_e2ee_key_envelopes.id;


--
-- Name: drive_e2ee_user_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_e2ee_user_keys (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    user_id bigint NOT NULL,
    key_algorithm text NOT NULL,
    public_key_jwk jsonb NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    rotated_at timestamp with time zone,
    CONSTRAINT drive_e2ee_user_keys_status_check CHECK ((status = ANY (ARRAY['active'::text, 'retired'::text, 'revoked'::text])))
);


--
-- Name: drive_e2ee_user_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_e2ee_user_keys_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_e2ee_user_keys_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_e2ee_user_keys_id_seq OWNED BY public.drive_e2ee_user_keys.id;


--
-- Name: drive_ediscovery_export_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_ediscovery_export_items (
    id bigint NOT NULL,
    export_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    file_revision text DEFAULT '1'::text NOT NULL,
    content_sha256 text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    provider_item_id text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_ediscovery_export_items_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'uploaded'::text, 'skipped'::text, 'failed'::text])))
);


--
-- Name: drive_ediscovery_export_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_ediscovery_export_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_ediscovery_export_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_ediscovery_export_items_id_seq OWNED BY public.drive_ediscovery_export_items.id;


--
-- Name: drive_ediscovery_exports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_ediscovery_exports (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    case_id bigint,
    case_public_id uuid,
    provider_connection_id bigint NOT NULL,
    requested_by_user_id bigint NOT NULL,
    approved_by_user_id bigint,
    status text DEFAULT 'pending_approval'::text NOT NULL,
    manifest_hash text,
    provider_export_id text,
    error_message text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_ediscovery_exports_status_check CHECK ((status = ANY (ARRAY['pending_approval'::text, 'approved'::text, 'exported'::text, 'rejected'::text, 'failed'::text])))
);


--
-- Name: drive_ediscovery_exports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_ediscovery_exports_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_ediscovery_exports_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_ediscovery_exports_id_seq OWNED BY public.drive_ediscovery_exports.id;


--
-- Name: drive_ediscovery_provider_connections; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_ediscovery_provider_connections (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    provider text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    config_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    encrypted_credentials bytea,
    created_by_user_id bigint CONSTRAINT drive_ediscovery_provider_connectio_created_by_user_id_not_null NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_ediscovery_provider_connections_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'error'::text])))
);


--
-- Name: drive_ediscovery_provider_connections_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_ediscovery_provider_connections_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_ediscovery_provider_connections_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_ediscovery_provider_connections_id_seq OWNED BY public.drive_ediscovery_provider_connections.id;


--
-- Name: drive_edit_locks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_edit_locks (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    actor_user_id bigint NOT NULL,
    session_id bigint NOT NULL,
    base_revision bigint DEFAULT 0 NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    last_heartbeat_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_edit_locks_base_revision_check CHECK ((base_revision >= 0))
);


--
-- Name: drive_edit_locks_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_edit_locks ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_edit_locks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_edit_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_edit_sessions (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    actor_user_id bigint NOT NULL,
    provider text DEFAULT 'lock_based'::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    base_revision bigint DEFAULT 0 NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    ended_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_edit_sessions_base_revision_check CHECK ((base_revision >= 0)),
    CONSTRAINT drive_edit_sessions_status_check CHECK ((status = ANY (ARRAY['active'::text, 'ended'::text, 'expired'::text, 'conflicted'::text])))
);


--
-- Name: drive_edit_sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_edit_sessions ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_edit_sessions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_encryption_policies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_encryption_policies (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    scope text DEFAULT 'tenant'::text NOT NULL,
    mode text DEFAULT 'service_managed'::text NOT NULL,
    kms_key_id bigint,
    status text DEFAULT 'active'::text NOT NULL,
    key_loss_policy text DEFAULT 'fail_closed'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_encryption_policies_mode_check CHECK ((mode = ANY (ARRAY['service_managed'::text, 'tenant_managed'::text, 'workspace_managed'::text, 'file_managed'::text]))),
    CONSTRAINT drive_encryption_policies_scope_check CHECK ((scope = ANY (ARRAY['tenant'::text, 'workspace'::text, 'file'::text]))),
    CONSTRAINT drive_encryption_policies_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text])))
);


--
-- Name: drive_encryption_policies_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_encryption_policies ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_encryption_policies_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_file_previews; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_file_previews (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    thumbnail_storage_key text,
    preview_storage_key text,
    content_type text DEFAULT ''::text NOT NULL,
    error_code text,
    generated_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_file_previews_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'ready'::text, 'failed'::text, 'skipped'::text])))
);


--
-- Name: drive_file_previews_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_file_previews ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_file_previews_id_seq
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
    description text DEFAULT ''::text NOT NULL,
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
-- Name: drive_gateway_objects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_gateway_objects (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    gateway_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    gateway_object_key text NOT NULL,
    manifest_hash text NOT NULL,
    replication_status text DEFAULT 'active'::text NOT NULL,
    last_verified_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_gateway_objects_replication_status_check CHECK ((replication_status = ANY (ARRAY['active'::text, 'pending'::text, 'failed'::text])))
);


--
-- Name: drive_gateway_objects_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_gateway_objects_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_gateway_objects_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_gateway_objects_id_seq OWNED BY public.drive_gateway_objects.id;


--
-- Name: drive_gateway_transfers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_gateway_transfers (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    gateway_id bigint NOT NULL,
    file_object_id bigint,
    direction text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    bytes_total bigint DEFAULT 0 NOT NULL,
    bytes_transferred bigint DEFAULT 0 NOT NULL,
    error_message text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_gateway_transfers_direction_check CHECK ((direction = ANY (ARRAY['upload'::text, 'download'::text, 'verify'::text]))),
    CONSTRAINT drive_gateway_transfers_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'completed'::text, 'failed'::text])))
);


--
-- Name: drive_gateway_transfers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_gateway_transfers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_gateway_transfers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_gateway_transfers_id_seq OWNED BY public.drive_gateway_transfers.id;


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
-- Name: drive_hsm_deployments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_hsm_deployments (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    provider text NOT NULL,
    endpoint_url text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    attestation_hash text,
    health_status text DEFAULT 'healthy'::text NOT NULL,
    last_health_checked_at timestamp with time zone,
    created_by_user_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_hsm_deployments_health_check CHECK ((health_status = ANY (ARRAY['healthy'::text, 'unavailable'::text, 'unknown'::text]))),
    CONSTRAINT drive_hsm_deployments_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'error'::text])))
);


--
-- Name: drive_hsm_deployments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_hsm_deployments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_hsm_deployments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_hsm_deployments_id_seq OWNED BY public.drive_hsm_deployments.id;


--
-- Name: drive_hsm_key_bindings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_hsm_key_bindings (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    workspace_id bigint,
    file_object_id bigint,
    hsm_key_id bigint NOT NULL,
    binding_scope text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_hsm_key_bindings_scope_check CHECK ((binding_scope = ANY (ARRAY['tenant'::text, 'workspace'::text, 'file'::text])))
);


--
-- Name: drive_hsm_key_bindings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_hsm_key_bindings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_hsm_key_bindings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_hsm_key_bindings_id_seq OWNED BY public.drive_hsm_key_bindings.id;


--
-- Name: drive_hsm_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_hsm_keys (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    deployment_id bigint NOT NULL,
    key_ref text NOT NULL,
    key_version text DEFAULT '1'::text NOT NULL,
    purpose text DEFAULT 'drive_file'::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    rotation_due_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_hsm_keys_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'destroyed'::text, 'unavailable'::text])))
);


--
-- Name: drive_hsm_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_hsm_keys_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_hsm_keys_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_hsm_keys_id_seq OWNED BY public.drive_hsm_keys.id;


--
-- Name: drive_index_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_index_jobs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    reason text DEFAULT 'metadata_changed'::text NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    last_error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_index_jobs_attempts_check CHECK ((attempts >= 0)),
    CONSTRAINT drive_index_jobs_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'running'::text, 'succeeded'::text, 'failed'::text, 'skipped'::text])))
);


--
-- Name: drive_index_jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_index_jobs ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_index_jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_item_activities; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_item_activities (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    actor_user_id bigint,
    resource_type text NOT NULL,
    resource_id bigint NOT NULL,
    action text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_item_activities_action_check CHECK ((action = ANY (ARRAY['viewed'::text, 'downloaded'::text, 'uploaded'::text, 'updated'::text, 'renamed'::text, 'moved'::text, 'shared'::text, 'unshared'::text, 'deleted'::text, 'restored'::text, 'previewed'::text]))),
    CONSTRAINT drive_item_activities_resource_type_check CHECK ((resource_type = ANY (ARRAY['file'::text, 'folder'::text])))
);


--
-- Name: drive_item_activities_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_item_activities ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_item_activities_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_item_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_item_tags (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    resource_type text NOT NULL,
    resource_id bigint NOT NULL,
    tag text NOT NULL,
    normalized_tag text NOT NULL,
    created_by_user_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_item_tags_normalized_tag_check CHECK ((btrim(normalized_tag) <> ''::text)),
    CONSTRAINT drive_item_tags_resource_type_check CHECK ((resource_type = ANY (ARRAY['file'::text, 'folder'::text]))),
    CONSTRAINT drive_item_tags_tag_check CHECK ((btrim(tag) <> ''::text))
);


--
-- Name: drive_item_tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_item_tags ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_item_tags_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_key_rotation_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_key_rotation_jobs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    old_kms_key_id bigint,
    new_kms_key_id bigint,
    status text DEFAULT 'queued'::text NOT NULL,
    progress_count bigint DEFAULT 0 NOT NULL,
    failure_reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_key_rotation_jobs_progress_count_check CHECK ((progress_count >= 0)),
    CONSTRAINT drive_key_rotation_jobs_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'running'::text, 'succeeded'::text, 'failed'::text])))
);


--
-- Name: drive_key_rotation_jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_key_rotation_jobs ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_key_rotation_jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_kms_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_kms_keys (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    provider text DEFAULT 'external'::text NOT NULL,
    key_ref text NOT NULL,
    masked_key_ref text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    last_verified_at timestamp with time zone,
    created_by_user_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_kms_keys_key_ref_check CHECK ((btrim(key_ref) <> ''::text)),
    CONSTRAINT drive_kms_keys_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'unavailable'::text, 'deleted'::text])))
);


--
-- Name: drive_kms_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_kms_keys ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_kms_keys_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_legal_case_resources; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_legal_case_resources (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    case_id bigint NOT NULL,
    resource_type text NOT NULL,
    resource_id bigint NOT NULL,
    hold_enabled boolean DEFAULT true NOT NULL,
    added_by_user_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_legal_case_resources_resource_type_check CHECK ((resource_type = ANY (ARRAY['file'::text, 'folder'::text, 'workspace'::text])))
);


--
-- Name: drive_legal_case_resources_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_legal_case_resources ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_legal_case_resources_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_legal_cases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_legal_cases (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_by_user_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_legal_cases_name_check CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT drive_legal_cases_status_check CHECK ((status = ANY (ARRAY['active'::text, 'closed'::text, 'archived'::text])))
);


--
-- Name: drive_legal_cases_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_legal_cases ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_legal_cases_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_legal_export_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_legal_export_items (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    export_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    denial_reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_legal_export_items_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'included'::text, 'denied'::text])))
);


--
-- Name: drive_legal_export_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_legal_export_items ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_legal_export_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_legal_exports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_legal_exports (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    case_id bigint NOT NULL,
    requested_by_user_id bigint NOT NULL,
    approved_by_user_id bigint,
    status text DEFAULT 'pending_approval'::text NOT NULL,
    package_storage_key text,
    expires_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_legal_exports_status_check CHECK ((status = ANY (ARRAY['pending_approval'::text, 'approved'::text, 'running'::text, 'ready'::text, 'denied'::text, 'expired'::text])))
);


--
-- Name: drive_legal_exports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_legal_exports ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_legal_exports_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_legal_holds; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_legal_holds (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    case_id bigint NOT NULL,
    resource_type text NOT NULL,
    resource_id bigint NOT NULL,
    reason text NOT NULL,
    created_by_user_id bigint NOT NULL,
    released_by_user_id bigint,
    released_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_legal_holds_reason_check CHECK ((btrim(reason) <> ''::text)),
    CONSTRAINT drive_legal_holds_resource_type_check CHECK ((resource_type = ANY (ARRAY['file'::text, 'folder'::text])))
);


--
-- Name: drive_legal_holds_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_legal_holds ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_legal_holds_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_marketplace_app_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_marketplace_app_versions (
    id bigint NOT NULL,
    app_id bigint NOT NULL,
    version text NOT NULL,
    manifest_json jsonb NOT NULL,
    signature text NOT NULL,
    review_status text DEFAULT 'approved'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_marketplace_app_versions_review_check CHECK ((review_status = ANY (ARRAY['pending'::text, 'approved'::text, 'rejected'::text])))
);


--
-- Name: drive_marketplace_app_versions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_marketplace_app_versions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_marketplace_app_versions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_marketplace_app_versions_id_seq OWNED BY public.drive_marketplace_app_versions.id;


--
-- Name: drive_marketplace_apps; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_marketplace_apps (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    slug text NOT NULL,
    name text NOT NULL,
    publisher_name text NOT NULL,
    status text DEFAULT 'reviewed'::text NOT NULL,
    homepage_url text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_marketplace_apps_status_check CHECK ((status = ANY (ARRAY['draft'::text, 'reviewed'::text, 'rejected'::text, 'disabled'::text])))
);


--
-- Name: drive_marketplace_apps_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_marketplace_apps_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_marketplace_apps_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_marketplace_apps_id_seq OWNED BY public.drive_marketplace_apps.id;


--
-- Name: drive_marketplace_installation_scopes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_marketplace_installation_scopes (
    id bigint NOT NULL,
    installation_id bigint NOT NULL,
    scope text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: drive_marketplace_installation_scopes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_marketplace_installation_scopes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_marketplace_installation_scopes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_marketplace_installation_scopes_id_seq OWNED BY public.drive_marketplace_installation_scopes.id;


--
-- Name: drive_marketplace_installations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_marketplace_installations (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    app_id bigint NOT NULL,
    app_version_id bigint NOT NULL,
    status text DEFAULT 'pending_approval'::text NOT NULL,
    installed_by_user_id bigint NOT NULL,
    approved_by_user_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_marketplace_installations_status_check CHECK ((status = ANY (ARRAY['pending_approval'::text, 'active'::text, 'rejected'::text, 'uninstalled'::text])))
);


--
-- Name: drive_marketplace_installations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_marketplace_installations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_marketplace_installations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_marketplace_installations_id_seq OWNED BY public.drive_marketplace_installations.id;


--
-- Name: drive_mobile_offline_operations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_mobile_offline_operations (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    device_id bigint NOT NULL,
    operation_type text NOT NULL,
    resource_type text NOT NULL,
    resource_public_id uuid NOT NULL,
    base_revision bigint DEFAULT 0 NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    failure_reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    applied_at timestamp with time zone,
    CONSTRAINT drive_mobile_offline_operations_base_revision_check CHECK ((base_revision >= 0)),
    CONSTRAINT drive_mobile_offline_operations_operation_type_check CHECK ((btrim(operation_type) <> ''::text)),
    CONSTRAINT drive_mobile_offline_operations_resource_type_check CHECK ((resource_type = ANY (ARRAY['file'::text, 'folder'::text]))),
    CONSTRAINT drive_mobile_offline_operations_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'applied'::text, 'denied'::text, 'conflicted'::text])))
);


--
-- Name: drive_mobile_offline_operations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_mobile_offline_operations ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_mobile_offline_operations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_object_key_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_object_key_versions (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    kms_key_id bigint,
    key_version text DEFAULT 'service-managed'::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_object_key_versions_status_check CHECK ((status = ANY (ARRAY['active'::text, 'rotating'::text, 'stale'::text])))
);


--
-- Name: drive_object_key_versions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_object_key_versions ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_object_key_versions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_ocr_pages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_ocr_pages (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    ocr_run_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    page_number integer NOT NULL,
    raw_text text DEFAULT ''::text NOT NULL,
    average_confidence numeric(5,4),
    layout_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    boxes_json jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_ocr_pages_page_number_check CHECK ((page_number > 0))
);


--
-- Name: drive_ocr_pages_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_ocr_pages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_ocr_pages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_ocr_pages_id_seq OWNED BY public.drive_ocr_pages.id;


--
-- Name: drive_ocr_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_ocr_runs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    file_revision text DEFAULT '1'::text NOT NULL,
    content_sha256 text DEFAULT ''::text NOT NULL,
    engine text NOT NULL,
    languages text[] DEFAULT ARRAY['jpn'::text, 'eng'::text] NOT NULL,
    structured_extractor text DEFAULT 'rules'::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    reason text DEFAULT 'upload'::text NOT NULL,
    page_count integer DEFAULT 0 NOT NULL,
    processed_page_count integer DEFAULT 0 NOT NULL,
    average_confidence numeric(5,4),
    extracted_text text DEFAULT ''::text NOT NULL,
    error_code text,
    error_message text,
    requested_by_user_id bigint,
    outbox_event_id bigint,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    artifact_schema_version text DEFAULT 'drive_image_pdf_v1'::text NOT NULL,
    pipeline_config_hash text DEFAULT 'legacy'::text NOT NULL,
    CONSTRAINT drive_ocr_runs_artifact_schema_version_check CHECK ((btrim(artifact_schema_version) <> ''::text)),
    CONSTRAINT drive_ocr_runs_engine_check CHECK ((engine = ANY (ARRAY['tesseract'::text, 'docling'::text, 'paddleocr'::text]))),
    CONSTRAINT drive_ocr_runs_page_count_check CHECK ((page_count >= 0)),
    CONSTRAINT drive_ocr_runs_pipeline_config_hash_check CHECK ((btrim(pipeline_config_hash) <> ''::text)),
    CONSTRAINT drive_ocr_runs_processed_page_count_check CHECK ((processed_page_count >= 0)),
    CONSTRAINT drive_ocr_runs_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'running'::text, 'completed'::text, 'failed'::text, 'skipped'::text]))),
    CONSTRAINT drive_ocr_runs_structured_extractor_check CHECK ((structured_extractor = ANY (ARRAY['rules'::text, 'ollama'::text, 'lmstudio'::text, 'gemini'::text, 'codex'::text, 'claude'::text, 'python'::text, 'ginza'::text, 'sudachipy'::text, 'docling'::text])))
);


--
-- Name: drive_ocr_runs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_ocr_runs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_ocr_runs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_ocr_runs_id_seq OWNED BY public.drive_ocr_runs.id;


--
-- Name: drive_office_edit_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_office_edit_sessions (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    actor_user_id bigint NOT NULL,
    provider text NOT NULL,
    provider_session_id text NOT NULL,
    access_level text NOT NULL,
    launch_url text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_office_edit_sessions_access_check CHECK ((access_level = ANY (ARRAY['view'::text, 'edit'::text])))
);


--
-- Name: drive_office_edit_sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_office_edit_sessions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_office_edit_sessions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_office_edit_sessions_id_seq OWNED BY public.drive_office_edit_sessions.id;


--
-- Name: drive_office_provider_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_office_provider_files (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    provider text NOT NULL,
    provider_file_id text NOT NULL,
    compatibility_state text DEFAULT 'compatible'::text NOT NULL,
    provider_revision text DEFAULT '1'::text NOT NULL,
    content_checksum text,
    last_synced_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_office_provider_files_provider_check CHECK ((btrim(provider) <> ''::text)),
    CONSTRAINT drive_office_provider_files_state_check CHECK ((compatibility_state = ANY (ARRAY['compatible'::text, 'readonly'::text, 'unsupported'::text, 'error'::text])))
);


--
-- Name: drive_office_provider_files_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_office_provider_files_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_office_provider_files_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_office_provider_files_id_seq OWNED BY public.drive_office_provider_files.id;


--
-- Name: drive_office_webhook_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_office_webhook_events (
    id bigint NOT NULL,
    provider text NOT NULL,
    provider_event_id text NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint,
    payload_hash text NOT NULL,
    provider_revision text,
    received_at timestamp with time zone DEFAULT now() NOT NULL,
    processed_at timestamp with time zone,
    result text,
    CONSTRAINT drive_office_webhook_events_result_check CHECK (((result IS NULL) OR (result = ANY (ARRAY['accepted'::text, 'duplicate'::text, 'stale'::text, 'rejected'::text]))))
);


--
-- Name: drive_office_webhook_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_office_webhook_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_office_webhook_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_office_webhook_events_id_seq OWNED BY public.drive_office_webhook_events.id;


--
-- Name: drive_presence_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_presence_sessions (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    actor_user_id bigint NOT NULL,
    session_id bigint,
    status text DEFAULT 'active'::text NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_presence_sessions_status_check CHECK ((status = ANY (ARRAY['active'::text, 'away'::text, 'ended'::text])))
);


--
-- Name: drive_presence_sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_presence_sessions ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_presence_sessions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_product_extraction_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_product_extraction_items (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    ocr_run_id bigint NOT NULL,
    file_object_id bigint NOT NULL,
    item_type text NOT NULL,
    name text NOT NULL,
    brand text,
    manufacturer text,
    model text,
    sku text,
    jan_code text,
    category text,
    description text,
    price jsonb DEFAULT '{}'::jsonb NOT NULL,
    promotion jsonb DEFAULT '{}'::jsonb NOT NULL,
    availability jsonb DEFAULT '{}'::jsonb NOT NULL,
    source_text text DEFAULT ''::text NOT NULL,
    evidence jsonb DEFAULT '[]'::jsonb NOT NULL,
    attributes jsonb DEFAULT '{}'::jsonb NOT NULL,
    confidence numeric(5,4),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_product_extraction_items_item_type_check CHECK ((item_type = ANY (ARRAY['product'::text, 'promotion'::text, 'bundle'::text, 'unknown'::text])))
);


--
-- Name: drive_product_extraction_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_product_extraction_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_product_extraction_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_product_extraction_items_id_seq OWNED BY public.drive_product_extraction_items.id;


--
-- Name: drive_region_migration_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_region_migration_jobs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    workspace_id bigint,
    source_region text NOT NULL,
    target_region text NOT NULL,
    dry_run boolean DEFAULT true NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    rollback_plan text,
    created_by_user_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_region_migration_jobs_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'running'::text, 'requires_approval'::text, 'succeeded'::text, 'failed'::text, 'rolled_back'::text])))
);


--
-- Name: drive_region_migration_jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_region_migration_jobs ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_region_migration_jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_region_placement_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_region_placement_events (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    workspace_id bigint,
    file_object_id bigint,
    subsystem text NOT NULL,
    region text NOT NULL,
    decision text DEFAULT 'allowed'::text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_region_placement_events_region_check CHECK ((btrim(region) <> ''::text)),
    CONSTRAINT drive_region_placement_events_subsystem_check CHECK ((btrim(subsystem) <> ''::text))
);


--
-- Name: drive_region_placement_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_region_placement_events ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_region_placement_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_region_policies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_region_policies (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    primary_region text DEFAULT 'global'::text NOT NULL,
    allowed_regions text[] DEFAULT ARRAY['global'::text] NOT NULL,
    replication_mode text DEFAULT 'none'::text NOT NULL,
    index_region text DEFAULT 'same_as_primary'::text NOT NULL,
    backup_region text DEFAULT 'same_jurisdiction'::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_region_policies_status_check CHECK ((status = ANY (ARRAY['active'::text, 'pending_migration'::text, 'disabled'::text])))
);


--
-- Name: drive_region_policies_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_region_policies ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_region_policies_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_remote_wipe_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_remote_wipe_requests (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    device_id bigint NOT NULL,
    requested_by_user_id bigint,
    reason text DEFAULT 'manual'::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    acknowledged_at timestamp with time zone,
    CONSTRAINT drive_remote_wipe_requests_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'acknowledged'::text, 'expired'::text])))
);


--
-- Name: drive_remote_wipe_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_remote_wipe_requests ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_remote_wipe_requests_id_seq
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
-- Name: drive_search_documents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_search_documents (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    workspace_id bigint,
    file_object_id bigint NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    content_type text DEFAULT ''::text NOT NULL,
    extracted_text text DEFAULT ''::text NOT NULL,
    snippet text DEFAULT ''::text NOT NULL,
    content_sha256 text,
    object_updated_at timestamp with time zone,
    indexed_at timestamp with time zone DEFAULT now() NOT NULL,
    search_vector tsvector GENERATED ALWAYS AS ((setweight(to_tsvector('simple'::regconfig, COALESCE(title, ''::text)), 'A'::"char") || setweight(to_tsvector('simple'::regconfig, COALESCE(extracted_text, ''::text)), 'B'::"char"))) STORED,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: drive_search_documents_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_search_documents ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_search_documents_id_seq
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
-- Name: drive_starred_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_starred_items (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    user_id bigint NOT NULL,
    resource_type text NOT NULL,
    resource_id bigint NOT NULL,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_starred_items_resource_type_check CHECK ((resource_type = ANY (ARRAY['file'::text, 'folder'::text])))
);


--
-- Name: drive_starred_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_starred_items ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_starred_items_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_storage_gateways; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_storage_gateways (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    workspace_id bigint,
    name text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    endpoint_url text NOT NULL,
    certificate_fingerprint text NOT NULL,
    last_seen_at timestamp with time zone,
    created_by_user_id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_storage_gateways_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'disconnected'::text])))
);


--
-- Name: drive_storage_gateways_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.drive_storage_gateways_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: drive_storage_gateways_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.drive_storage_gateways_id_seq OWNED BY public.drive_storage_gateways.id;


--
-- Name: drive_sync_conflicts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_sync_conflicts (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    device_id bigint,
    resource_type text NOT NULL,
    resource_id bigint NOT NULL,
    reason text NOT NULL,
    status text DEFAULT 'open'::text NOT NULL,
    resolution text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_sync_conflicts_reason_check CHECK ((btrim(reason) <> ''::text)),
    CONSTRAINT drive_sync_conflicts_resource_type_check CHECK ((resource_type = ANY (ARRAY['file'::text, 'folder'::text]))),
    CONSTRAINT drive_sync_conflicts_status_check CHECK ((status = ANY (ARRAY['open'::text, 'resolved'::text, 'discarded'::text])))
);


--
-- Name: drive_sync_conflicts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_sync_conflicts ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_sync_conflicts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_sync_cursors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_sync_cursors (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    device_id bigint NOT NULL,
    cursor_value bigint DEFAULT 0 NOT NULL,
    last_issued_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_sync_cursors_cursor_value_check CHECK ((cursor_value >= 0))
);


--
-- Name: drive_sync_cursors_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_sync_cursors ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_sync_cursors_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_sync_devices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_sync_devices (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    user_id bigint NOT NULL,
    device_name text NOT NULL,
    platform text DEFAULT 'desktop'::text NOT NULL,
    token_hash text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    last_seen_at timestamp with time zone,
    last_ip text,
    last_user_agent text,
    remote_wipe_required boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_sync_devices_device_name_check CHECK ((btrim(device_name) <> ''::text)),
    CONSTRAINT drive_sync_devices_platform_check CHECK ((platform = ANY (ARRAY['desktop'::text, 'mobile'::text, 'web'::text]))),
    CONSTRAINT drive_sync_devices_status_check CHECK ((status = ANY (ARRAY['active'::text, 'revoked'::text, 'lost'::text])))
);


--
-- Name: drive_sync_devices_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_sync_devices ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_sync_devices_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_sync_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_sync_events (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    workspace_id bigint,
    resource_type text NOT NULL,
    resource_id bigint NOT NULL,
    action text NOT NULL,
    object_version text,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_sync_events_action_check CHECK ((btrim(action) <> ''::text)),
    CONSTRAINT drive_sync_events_resource_type_check CHECK ((resource_type = ANY (ARRAY['file'::text, 'folder'::text, 'workspace'::text])))
);


--
-- Name: drive_sync_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_sync_events ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_sync_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: drive_workspace_region_overrides; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.drive_workspace_region_overrides (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    workspace_id bigint NOT NULL,
    primary_region text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT drive_workspace_region_overrides_status_check CHECK ((status = ANY (ARRAY['active'::text, 'pending_migration'::text, 'disabled'::text])))
);


--
-- Name: drive_workspace_region_overrides_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.drive_workspace_region_overrides ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.drive_workspace_region_overrides_id_seq
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
    office_mime_family text,
    office_coauthoring_enabled boolean DEFAULT false NOT NULL,
    office_last_revision text,
    encryption_mode text DEFAULT 'server_managed'::text NOT NULL,
    e2ee_file_key_public_id uuid,
    storage_gateway_id bigint,
    description text DEFAULT ''::text NOT NULL,
    CONSTRAINT file_objects_byte_size_check CHECK ((byte_size >= 0)),
    CONSTRAINT file_objects_encryption_mode_check CHECK ((encryption_mode = ANY (ARRAY['server_managed'::text, 'tenant_managed'::text, 'hsm_managed'::text, 'zero_knowledge'::text]))),
    CONSTRAINT file_objects_purge_attempts_check CHECK ((purge_attempts >= 0)),
    CONSTRAINT file_objects_purpose_check CHECK ((purpose = ANY (ARRAY['attachment'::text, 'avatar'::text, 'import'::text, 'export'::text, 'drive'::text, 'dataset_source'::text]))),
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
-- Name: medallion_asset_edges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.medallion_asset_edges (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    source_asset_id bigint NOT NULL,
    target_asset_id bigint NOT NULL,
    relation_type text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT medallion_asset_edges_no_self_loop_check CHECK ((source_asset_id <> target_asset_id)),
    CONSTRAINT medallion_asset_edges_relation_type_check CHECK ((btrim(relation_type) <> ''::text))
);


--
-- Name: medallion_asset_edges_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.medallion_asset_edges ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.medallion_asset_edges_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: medallion_assets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.medallion_assets (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    layer text NOT NULL,
    resource_kind text NOT NULL,
    resource_id bigint NOT NULL,
    resource_public_id uuid NOT NULL,
    display_name text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    row_count bigint,
    byte_size bigint,
    schema_summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by_user_id bigint,
    updated_by_user_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    archived_at timestamp with time zone,
    CONSTRAINT medallion_assets_byte_size_check CHECK (((byte_size IS NULL) OR (byte_size >= 0))),
    CONSTRAINT medallion_assets_display_name_check CHECK ((btrim(display_name) <> ''::text)),
    CONSTRAINT medallion_assets_layer_check CHECK ((layer = ANY (ARRAY['bronze'::text, 'silver'::text, 'gold'::text]))),
    CONSTRAINT medallion_assets_resource_kind_check CHECK ((resource_kind = ANY (ARRAY['drive_file'::text, 'dataset'::text, 'work_table'::text, 'ocr_run'::text, 'product_extraction'::text, 'gold_table'::text]))),
    CONSTRAINT medallion_assets_row_count_check CHECK (((row_count IS NULL) OR (row_count >= 0))),
    CONSTRAINT medallion_assets_status_check CHECK ((status = ANY (ARRAY['active'::text, 'building'::text, 'failed'::text, 'skipped'::text, 'archived'::text])))
);


--
-- Name: medallion_assets_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.medallion_assets ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.medallion_assets_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: medallion_pipeline_run_assets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.medallion_pipeline_run_assets (
    id bigint NOT NULL,
    tenant_id bigint NOT NULL,
    pipeline_run_id bigint NOT NULL,
    asset_id bigint NOT NULL,
    role text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT medallion_pipeline_run_assets_role_check CHECK ((role = ANY (ARRAY['source'::text, 'target'::text, 'related'::text])))
);


--
-- Name: medallion_pipeline_run_assets_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.medallion_pipeline_run_assets ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.medallion_pipeline_run_assets_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: medallion_pipeline_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.medallion_pipeline_runs (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint NOT NULL,
    pipeline_type text NOT NULL,
    run_key text NOT NULL,
    source_resource_kind text,
    source_resource_id bigint,
    source_resource_public_id uuid,
    target_resource_kind text,
    target_resource_id bigint,
    target_resource_public_id uuid,
    status text DEFAULT 'pending'::text NOT NULL,
    runtime text DEFAULT ''::text NOT NULL,
    trigger_kind text DEFAULT 'system'::text NOT NULL,
    retryable boolean DEFAULT false NOT NULL,
    error_summary text,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    requested_by_user_id bigint,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT medallion_pipeline_runs_pipeline_type_check CHECK ((pipeline_type = ANY (ARRAY['dataset_import'::text, 'work_table_register'::text, 'work_table_promote'::text, 'dataset_sync'::text, 'drive_ocr'::text, 'product_extraction'::text, 'gold_publish'::text]))),
    CONSTRAINT medallion_pipeline_runs_run_key_check CHECK ((btrim(run_key) <> ''::text)),
    CONSTRAINT medallion_pipeline_runs_source_resource_kind_check CHECK (((source_resource_kind IS NULL) OR (source_resource_kind = ANY (ARRAY['drive_file'::text, 'dataset'::text, 'work_table'::text, 'ocr_run'::text, 'product_extraction'::text, 'gold_table'::text])))),
    CONSTRAINT medallion_pipeline_runs_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'processing'::text, 'completed'::text, 'failed'::text, 'skipped'::text]))),
    CONSTRAINT medallion_pipeline_runs_target_resource_kind_check CHECK (((target_resource_kind IS NULL) OR (target_resource_kind = ANY (ARRAY['drive_file'::text, 'dataset'::text, 'work_table'::text, 'ocr_run'::text, 'product_extraction'::text, 'gold_table'::text])))),
    CONSTRAINT medallion_pipeline_runs_trigger_kind_check CHECK ((trigger_kind = ANY (ARRAY['manual'::text, 'upload'::text, 'scheduled'::text, 'system'::text, 'read_repair'::text])))
);


--
-- Name: medallion_pipeline_runs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.medallion_pipeline_runs ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.medallion_pipeline_runs_id_seq
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
-- Name: realtime_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.realtime_events (
    id bigint NOT NULL,
    public_id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id bigint,
    recipient_user_id bigint NOT NULL,
    event_type text NOT NULL,
    resource_type text DEFAULT ''::text NOT NULL,
    resource_public_id text DEFAULT ''::text NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    expires_at timestamp with time zone DEFAULT (now() + '7 days'::interval) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT realtime_events_event_type_check CHECK ((btrim(event_type) <> ''::text)),
    CONSTRAINT realtime_events_recipient_user_id_check CHECK ((recipient_user_id > 0))
);


--
-- Name: realtime_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.realtime_events ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.realtime_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
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
-- Name: drive_ai_classifications id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_classifications ALTER COLUMN id SET DEFAULT nextval('public.drive_ai_classifications_id_seq'::regclass);


--
-- Name: drive_ai_jobs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_jobs ALTER COLUMN id SET DEFAULT nextval('public.drive_ai_jobs_id_seq'::regclass);


--
-- Name: drive_ai_summaries id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_summaries ALTER COLUMN id SET DEFAULT nextval('public.drive_ai_summaries_id_seq'::regclass);


--
-- Name: drive_app_webhook_deliveries id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_app_webhook_deliveries ALTER COLUMN id SET DEFAULT nextval('public.drive_app_webhook_deliveries_id_seq'::regclass);


--
-- Name: drive_e2ee_file_keys id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_file_keys ALTER COLUMN id SET DEFAULT nextval('public.drive_e2ee_file_keys_id_seq'::regclass);


--
-- Name: drive_e2ee_key_envelopes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_key_envelopes ALTER COLUMN id SET DEFAULT nextval('public.drive_e2ee_key_envelopes_id_seq'::regclass);


--
-- Name: drive_e2ee_user_keys id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_user_keys ALTER COLUMN id SET DEFAULT nextval('public.drive_e2ee_user_keys_id_seq'::regclass);


--
-- Name: drive_ediscovery_export_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_export_items ALTER COLUMN id SET DEFAULT nextval('public.drive_ediscovery_export_items_id_seq'::regclass);


--
-- Name: drive_ediscovery_exports id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_exports ALTER COLUMN id SET DEFAULT nextval('public.drive_ediscovery_exports_id_seq'::regclass);


--
-- Name: drive_ediscovery_provider_connections id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_provider_connections ALTER COLUMN id SET DEFAULT nextval('public.drive_ediscovery_provider_connections_id_seq'::regclass);


--
-- Name: drive_gateway_objects id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_gateway_objects ALTER COLUMN id SET DEFAULT nextval('public.drive_gateway_objects_id_seq'::regclass);


--
-- Name: drive_gateway_transfers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_gateway_transfers ALTER COLUMN id SET DEFAULT nextval('public.drive_gateway_transfers_id_seq'::regclass);


--
-- Name: drive_hsm_deployments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_deployments ALTER COLUMN id SET DEFAULT nextval('public.drive_hsm_deployments_id_seq'::regclass);


--
-- Name: drive_hsm_key_bindings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_key_bindings ALTER COLUMN id SET DEFAULT nextval('public.drive_hsm_key_bindings_id_seq'::regclass);


--
-- Name: drive_hsm_keys id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_keys ALTER COLUMN id SET DEFAULT nextval('public.drive_hsm_keys_id_seq'::regclass);


--
-- Name: drive_marketplace_app_versions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_app_versions ALTER COLUMN id SET DEFAULT nextval('public.drive_marketplace_app_versions_id_seq'::regclass);


--
-- Name: drive_marketplace_apps id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_apps ALTER COLUMN id SET DEFAULT nextval('public.drive_marketplace_apps_id_seq'::regclass);


--
-- Name: drive_marketplace_installation_scopes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_installation_scopes ALTER COLUMN id SET DEFAULT nextval('public.drive_marketplace_installation_scopes_id_seq'::regclass);


--
-- Name: drive_marketplace_installations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_installations ALTER COLUMN id SET DEFAULT nextval('public.drive_marketplace_installations_id_seq'::regclass);


--
-- Name: drive_ocr_pages id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ocr_pages ALTER COLUMN id SET DEFAULT nextval('public.drive_ocr_pages_id_seq'::regclass);


--
-- Name: drive_ocr_runs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ocr_runs ALTER COLUMN id SET DEFAULT nextval('public.drive_ocr_runs_id_seq'::regclass);


--
-- Name: drive_office_edit_sessions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_edit_sessions ALTER COLUMN id SET DEFAULT nextval('public.drive_office_edit_sessions_id_seq'::regclass);


--
-- Name: drive_office_provider_files id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_provider_files ALTER COLUMN id SET DEFAULT nextval('public.drive_office_provider_files_id_seq'::regclass);


--
-- Name: drive_office_webhook_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_webhook_events ALTER COLUMN id SET DEFAULT nextval('public.drive_office_webhook_events_id_seq'::regclass);


--
-- Name: drive_product_extraction_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_product_extraction_items ALTER COLUMN id SET DEFAULT nextval('public.drive_product_extraction_items_id_seq'::regclass);


--
-- Name: drive_storage_gateways id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_storage_gateways ALTER COLUMN id SET DEFAULT nextval('public.drive_storage_gateways_id_seq'::regclass);


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
-- Name: dataset_columns dataset_columns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_columns
    ADD CONSTRAINT dataset_columns_pkey PRIMARY KEY (id);


--
-- Name: dataset_gold_publications dataset_gold_publications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publications
    ADD CONSTRAINT dataset_gold_publications_pkey PRIMARY KEY (id);


--
-- Name: dataset_gold_publications dataset_gold_publications_public_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publications
    ADD CONSTRAINT dataset_gold_publications_public_id_key UNIQUE (public_id);


--
-- Name: dataset_gold_publish_runs dataset_gold_publish_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publish_runs
    ADD CONSTRAINT dataset_gold_publish_runs_pkey PRIMARY KEY (id);


--
-- Name: dataset_gold_publish_runs dataset_gold_publish_runs_public_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publish_runs
    ADD CONSTRAINT dataset_gold_publish_runs_public_id_key UNIQUE (public_id);


--
-- Name: dataset_import_jobs dataset_import_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_import_jobs
    ADD CONSTRAINT dataset_import_jobs_pkey PRIMARY KEY (id);


--
-- Name: dataset_lineage_change_sets dataset_lineage_change_sets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_change_sets
    ADD CONSTRAINT dataset_lineage_change_sets_pkey PRIMARY KEY (id);


--
-- Name: dataset_lineage_change_sets dataset_lineage_change_sets_public_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_change_sets
    ADD CONSTRAINT dataset_lineage_change_sets_public_id_key UNIQUE (public_id);


--
-- Name: dataset_lineage_edges dataset_lineage_edges_change_set_edge_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_edges
    ADD CONSTRAINT dataset_lineage_edges_change_set_edge_key_key UNIQUE (change_set_id, edge_key);


--
-- Name: dataset_lineage_edges dataset_lineage_edges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_edges
    ADD CONSTRAINT dataset_lineage_edges_pkey PRIMARY KEY (id);


--
-- Name: dataset_lineage_edges dataset_lineage_edges_public_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_edges
    ADD CONSTRAINT dataset_lineage_edges_public_id_key UNIQUE (public_id);


--
-- Name: dataset_lineage_nodes dataset_lineage_nodes_change_set_node_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_nodes
    ADD CONSTRAINT dataset_lineage_nodes_change_set_node_key_key UNIQUE (change_set_id, node_key);


--
-- Name: dataset_lineage_nodes dataset_lineage_nodes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_nodes
    ADD CONSTRAINT dataset_lineage_nodes_pkey PRIMARY KEY (id);


--
-- Name: dataset_lineage_nodes dataset_lineage_nodes_public_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_nodes
    ADD CONSTRAINT dataset_lineage_nodes_public_id_key UNIQUE (public_id);


--
-- Name: dataset_lineage_parse_runs dataset_lineage_parse_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_parse_runs
    ADD CONSTRAINT dataset_lineage_parse_runs_pkey PRIMARY KEY (id);


--
-- Name: dataset_lineage_parse_runs dataset_lineage_parse_runs_public_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_parse_runs
    ADD CONSTRAINT dataset_lineage_parse_runs_public_id_key UNIQUE (public_id);


--
-- Name: dataset_query_jobs dataset_query_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_query_jobs
    ADD CONSTRAINT dataset_query_jobs_pkey PRIMARY KEY (id);


--
-- Name: dataset_sync_jobs dataset_sync_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_sync_jobs
    ADD CONSTRAINT dataset_sync_jobs_pkey PRIMARY KEY (id);


--
-- Name: dataset_sync_jobs dataset_sync_jobs_public_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_sync_jobs
    ADD CONSTRAINT dataset_sync_jobs_public_id_key UNIQUE (public_id);


--
-- Name: dataset_work_table_export_schedules dataset_work_table_export_schedules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_export_schedules
    ADD CONSTRAINT dataset_work_table_export_schedules_pkey PRIMARY KEY (id);


--
-- Name: dataset_work_table_export_schedules dataset_work_table_export_schedules_public_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_export_schedules
    ADD CONSTRAINT dataset_work_table_export_schedules_public_id_key UNIQUE (public_id);


--
-- Name: dataset_work_table_exports dataset_work_table_exports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_exports
    ADD CONSTRAINT dataset_work_table_exports_pkey PRIMARY KEY (id);


--
-- Name: dataset_work_tables dataset_work_tables_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_tables
    ADD CONSTRAINT dataset_work_tables_pkey PRIMARY KEY (id);


--
-- Name: datasets datasets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.datasets
    ADD CONSTRAINT datasets_pkey PRIMARY KEY (id);


--
-- Name: drive_admin_content_access_sessions drive_admin_content_access_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_admin_content_access_sessions
    ADD CONSTRAINT drive_admin_content_access_sessions_pkey PRIMARY KEY (id);


--
-- Name: drive_ai_classifications drive_ai_classifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_classifications
    ADD CONSTRAINT drive_ai_classifications_pkey PRIMARY KEY (id);


--
-- Name: drive_ai_jobs drive_ai_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_jobs
    ADD CONSTRAINT drive_ai_jobs_pkey PRIMARY KEY (id);


--
-- Name: drive_ai_summaries drive_ai_summaries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_summaries
    ADD CONSTRAINT drive_ai_summaries_pkey PRIMARY KEY (id);


--
-- Name: drive_app_webhook_deliveries drive_app_webhook_deliveries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_app_webhook_deliveries
    ADD CONSTRAINT drive_app_webhook_deliveries_pkey PRIMARY KEY (id);


--
-- Name: drive_chain_of_custody_events drive_chain_of_custody_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_chain_of_custody_events
    ADD CONSTRAINT drive_chain_of_custody_events_pkey PRIMARY KEY (id);


--
-- Name: drive_clean_room_datasets drive_clean_room_datasets_clean_room_id_source_file_object__key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_datasets
    ADD CONSTRAINT drive_clean_room_datasets_clean_room_id_source_file_object__key UNIQUE (clean_room_id, source_file_object_id);


--
-- Name: drive_clean_room_datasets drive_clean_room_datasets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_datasets
    ADD CONSTRAINT drive_clean_room_datasets_pkey PRIMARY KEY (id);


--
-- Name: drive_clean_room_exports drive_clean_room_exports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_exports
    ADD CONSTRAINT drive_clean_room_exports_pkey PRIMARY KEY (id);


--
-- Name: drive_clean_room_jobs drive_clean_room_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_jobs
    ADD CONSTRAINT drive_clean_room_jobs_pkey PRIMARY KEY (id);


--
-- Name: drive_clean_room_participants drive_clean_room_participants_clean_room_id_participant_ten_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_participants
    ADD CONSTRAINT drive_clean_room_participants_clean_room_id_participant_ten_key UNIQUE (clean_room_id, participant_tenant_id, user_id, role);


--
-- Name: drive_clean_room_participants drive_clean_room_participants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_participants
    ADD CONSTRAINT drive_clean_room_participants_pkey PRIMARY KEY (id);


--
-- Name: drive_clean_room_policy_decisions drive_clean_room_policy_decisions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_policy_decisions
    ADD CONSTRAINT drive_clean_room_policy_decisions_pkey PRIMARY KEY (id);


--
-- Name: drive_clean_rooms drive_clean_rooms_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_rooms
    ADD CONSTRAINT drive_clean_rooms_pkey PRIMARY KEY (id);


--
-- Name: drive_e2ee_file_keys drive_e2ee_file_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_file_keys
    ADD CONSTRAINT drive_e2ee_file_keys_pkey PRIMARY KEY (id);


--
-- Name: drive_e2ee_key_envelopes drive_e2ee_key_envelopes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_key_envelopes
    ADD CONSTRAINT drive_e2ee_key_envelopes_pkey PRIMARY KEY (id);


--
-- Name: drive_e2ee_user_keys drive_e2ee_user_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_user_keys
    ADD CONSTRAINT drive_e2ee_user_keys_pkey PRIMARY KEY (id);


--
-- Name: drive_ediscovery_export_items drive_ediscovery_export_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_export_items
    ADD CONSTRAINT drive_ediscovery_export_items_pkey PRIMARY KEY (id);


--
-- Name: drive_ediscovery_exports drive_ediscovery_exports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_exports
    ADD CONSTRAINT drive_ediscovery_exports_pkey PRIMARY KEY (id);


--
-- Name: drive_ediscovery_provider_connections drive_ediscovery_provider_connections_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_provider_connections
    ADD CONSTRAINT drive_ediscovery_provider_connections_pkey PRIMARY KEY (id);


--
-- Name: drive_edit_locks drive_edit_locks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_edit_locks
    ADD CONSTRAINT drive_edit_locks_pkey PRIMARY KEY (id);


--
-- Name: drive_edit_sessions drive_edit_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_edit_sessions
    ADD CONSTRAINT drive_edit_sessions_pkey PRIMARY KEY (id);


--
-- Name: drive_encryption_policies drive_encryption_policies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_encryption_policies
    ADD CONSTRAINT drive_encryption_policies_pkey PRIMARY KEY (id);


--
-- Name: drive_encryption_policies drive_encryption_policies_tenant_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_encryption_policies
    ADD CONSTRAINT drive_encryption_policies_tenant_id_key UNIQUE (tenant_id);


--
-- Name: drive_file_previews drive_file_previews_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_file_previews
    ADD CONSTRAINT drive_file_previews_pkey PRIMARY KEY (id);


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
-- Name: drive_gateway_objects drive_gateway_objects_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_gateway_objects
    ADD CONSTRAINT drive_gateway_objects_pkey PRIMARY KEY (id);


--
-- Name: drive_gateway_transfers drive_gateway_transfers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_gateway_transfers
    ADD CONSTRAINT drive_gateway_transfers_pkey PRIMARY KEY (id);


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
-- Name: drive_hsm_deployments drive_hsm_deployments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_deployments
    ADD CONSTRAINT drive_hsm_deployments_pkey PRIMARY KEY (id);


--
-- Name: drive_hsm_key_bindings drive_hsm_key_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_key_bindings
    ADD CONSTRAINT drive_hsm_key_bindings_pkey PRIMARY KEY (id);


--
-- Name: drive_hsm_keys drive_hsm_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_keys
    ADD CONSTRAINT drive_hsm_keys_pkey PRIMARY KEY (id);


--
-- Name: drive_index_jobs drive_index_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_index_jobs
    ADD CONSTRAINT drive_index_jobs_pkey PRIMARY KEY (id);


--
-- Name: drive_item_activities drive_item_activities_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_item_activities
    ADD CONSTRAINT drive_item_activities_pkey PRIMARY KEY (id);


--
-- Name: drive_item_tags drive_item_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_item_tags
    ADD CONSTRAINT drive_item_tags_pkey PRIMARY KEY (id);


--
-- Name: drive_key_rotation_jobs drive_key_rotation_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_key_rotation_jobs
    ADD CONSTRAINT drive_key_rotation_jobs_pkey PRIMARY KEY (id);


--
-- Name: drive_kms_keys drive_kms_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_kms_keys
    ADD CONSTRAINT drive_kms_keys_pkey PRIMARY KEY (id);


--
-- Name: drive_legal_case_resources drive_legal_case_resources_case_id_resource_type_resource_i_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_case_resources
    ADD CONSTRAINT drive_legal_case_resources_case_id_resource_type_resource_i_key UNIQUE (case_id, resource_type, resource_id);


--
-- Name: drive_legal_case_resources drive_legal_case_resources_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_case_resources
    ADD CONSTRAINT drive_legal_case_resources_pkey PRIMARY KEY (id);


--
-- Name: drive_legal_cases drive_legal_cases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_cases
    ADD CONSTRAINT drive_legal_cases_pkey PRIMARY KEY (id);


--
-- Name: drive_legal_export_items drive_legal_export_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_export_items
    ADD CONSTRAINT drive_legal_export_items_pkey PRIMARY KEY (id);


--
-- Name: drive_legal_exports drive_legal_exports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_exports
    ADD CONSTRAINT drive_legal_exports_pkey PRIMARY KEY (id);


--
-- Name: drive_legal_holds drive_legal_holds_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_holds
    ADD CONSTRAINT drive_legal_holds_pkey PRIMARY KEY (id);


--
-- Name: drive_marketplace_app_versions drive_marketplace_app_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_app_versions
    ADD CONSTRAINT drive_marketplace_app_versions_pkey PRIMARY KEY (id);


--
-- Name: drive_marketplace_apps drive_marketplace_apps_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_apps
    ADD CONSTRAINT drive_marketplace_apps_pkey PRIMARY KEY (id);


--
-- Name: drive_marketplace_apps drive_marketplace_apps_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_apps
    ADD CONSTRAINT drive_marketplace_apps_slug_key UNIQUE (slug);


--
-- Name: drive_marketplace_installation_scopes drive_marketplace_installation_scopes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_installation_scopes
    ADD CONSTRAINT drive_marketplace_installation_scopes_pkey PRIMARY KEY (id);


--
-- Name: drive_marketplace_installations drive_marketplace_installations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_installations
    ADD CONSTRAINT drive_marketplace_installations_pkey PRIMARY KEY (id);


--
-- Name: drive_mobile_offline_operations drive_mobile_offline_operations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_mobile_offline_operations
    ADD CONSTRAINT drive_mobile_offline_operations_pkey PRIMARY KEY (id);


--
-- Name: drive_object_key_versions drive_object_key_versions_file_object_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_object_key_versions
    ADD CONSTRAINT drive_object_key_versions_file_object_id_key UNIQUE (file_object_id);


--
-- Name: drive_object_key_versions drive_object_key_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_object_key_versions
    ADD CONSTRAINT drive_object_key_versions_pkey PRIMARY KEY (id);


--
-- Name: drive_ocr_pages drive_ocr_pages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ocr_pages
    ADD CONSTRAINT drive_ocr_pages_pkey PRIMARY KEY (id);


--
-- Name: drive_ocr_runs drive_ocr_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ocr_runs
    ADD CONSTRAINT drive_ocr_runs_pkey PRIMARY KEY (id);


--
-- Name: drive_office_edit_sessions drive_office_edit_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_edit_sessions
    ADD CONSTRAINT drive_office_edit_sessions_pkey PRIMARY KEY (id);


--
-- Name: drive_office_provider_files drive_office_provider_files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_provider_files
    ADD CONSTRAINT drive_office_provider_files_pkey PRIMARY KEY (id);


--
-- Name: drive_office_webhook_events drive_office_webhook_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_webhook_events
    ADD CONSTRAINT drive_office_webhook_events_pkey PRIMARY KEY (id);


--
-- Name: drive_presence_sessions drive_presence_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_presence_sessions
    ADD CONSTRAINT drive_presence_sessions_pkey PRIMARY KEY (id);


--
-- Name: drive_product_extraction_items drive_product_extraction_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_product_extraction_items
    ADD CONSTRAINT drive_product_extraction_items_pkey PRIMARY KEY (id);


--
-- Name: drive_region_migration_jobs drive_region_migration_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_region_migration_jobs
    ADD CONSTRAINT drive_region_migration_jobs_pkey PRIMARY KEY (id);


--
-- Name: drive_region_placement_events drive_region_placement_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_region_placement_events
    ADD CONSTRAINT drive_region_placement_events_pkey PRIMARY KEY (id);


--
-- Name: drive_region_policies drive_region_policies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_region_policies
    ADD CONSTRAINT drive_region_policies_pkey PRIMARY KEY (id);


--
-- Name: drive_region_policies drive_region_policies_tenant_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_region_policies
    ADD CONSTRAINT drive_region_policies_tenant_id_key UNIQUE (tenant_id);


--
-- Name: drive_remote_wipe_requests drive_remote_wipe_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_remote_wipe_requests
    ADD CONSTRAINT drive_remote_wipe_requests_pkey PRIMARY KEY (id);


--
-- Name: drive_resource_shares drive_resource_shares_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_resource_shares
    ADD CONSTRAINT drive_resource_shares_pkey PRIMARY KEY (id);


--
-- Name: drive_search_documents drive_search_documents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_search_documents
    ADD CONSTRAINT drive_search_documents_pkey PRIMARY KEY (id);


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
-- Name: drive_starred_items drive_starred_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_starred_items
    ADD CONSTRAINT drive_starred_items_pkey PRIMARY KEY (id);


--
-- Name: drive_storage_gateways drive_storage_gateways_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_storage_gateways
    ADD CONSTRAINT drive_storage_gateways_pkey PRIMARY KEY (id);


--
-- Name: drive_sync_conflicts drive_sync_conflicts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_conflicts
    ADD CONSTRAINT drive_sync_conflicts_pkey PRIMARY KEY (id);


--
-- Name: drive_sync_cursors drive_sync_cursors_device_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_cursors
    ADD CONSTRAINT drive_sync_cursors_device_id_key UNIQUE (device_id);


--
-- Name: drive_sync_cursors drive_sync_cursors_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_cursors
    ADD CONSTRAINT drive_sync_cursors_pkey PRIMARY KEY (id);


--
-- Name: drive_sync_devices drive_sync_devices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_devices
    ADD CONSTRAINT drive_sync_devices_pkey PRIMARY KEY (id);


--
-- Name: drive_sync_events drive_sync_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_events
    ADD CONSTRAINT drive_sync_events_pkey PRIMARY KEY (id);


--
-- Name: drive_workspace_region_overrides drive_workspace_region_overrides_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_workspace_region_overrides
    ADD CONSTRAINT drive_workspace_region_overrides_pkey PRIMARY KEY (id);


--
-- Name: drive_workspace_region_overrides drive_workspace_region_overrides_workspace_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_workspace_region_overrides
    ADD CONSTRAINT drive_workspace_region_overrides_workspace_id_key UNIQUE (workspace_id);


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
-- Name: medallion_asset_edges medallion_asset_edges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_asset_edges
    ADD CONSTRAINT medallion_asset_edges_pkey PRIMARY KEY (id);


--
-- Name: medallion_asset_edges medallion_asset_edges_public_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_asset_edges
    ADD CONSTRAINT medallion_asset_edges_public_id_key UNIQUE (public_id);


--
-- Name: medallion_asset_edges medallion_asset_edges_unique_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_asset_edges
    ADD CONSTRAINT medallion_asset_edges_unique_key UNIQUE (tenant_id, source_asset_id, target_asset_id, relation_type);


--
-- Name: medallion_assets medallion_assets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_assets
    ADD CONSTRAINT medallion_assets_pkey PRIMARY KEY (id);


--
-- Name: medallion_assets medallion_assets_public_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_assets
    ADD CONSTRAINT medallion_assets_public_id_key UNIQUE (public_id);


--
-- Name: medallion_assets medallion_assets_resource_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_assets
    ADD CONSTRAINT medallion_assets_resource_key UNIQUE (tenant_id, resource_kind, resource_id);


--
-- Name: medallion_pipeline_run_assets medallion_pipeline_run_assets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_pipeline_run_assets
    ADD CONSTRAINT medallion_pipeline_run_assets_pkey PRIMARY KEY (id);


--
-- Name: medallion_pipeline_run_assets medallion_pipeline_run_assets_unique_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_pipeline_run_assets
    ADD CONSTRAINT medallion_pipeline_run_assets_unique_key UNIQUE (pipeline_run_id, asset_id, role);


--
-- Name: medallion_pipeline_runs medallion_pipeline_runs_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_pipeline_runs
    ADD CONSTRAINT medallion_pipeline_runs_key UNIQUE (tenant_id, pipeline_type, run_key);


--
-- Name: medallion_pipeline_runs medallion_pipeline_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_pipeline_runs
    ADD CONSTRAINT medallion_pipeline_runs_pkey PRIMARY KEY (id);


--
-- Name: medallion_pipeline_runs medallion_pipeline_runs_public_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_pipeline_runs
    ADD CONSTRAINT medallion_pipeline_runs_public_id_key UNIQUE (public_id);


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
-- Name: realtime_events realtime_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realtime_events
    ADD CONSTRAINT realtime_events_pkey PRIMARY KEY (id);


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
-- Name: dataset_columns_dataset_column_name_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dataset_columns_dataset_column_name_key ON public.dataset_columns USING btree (dataset_id, column_name);


--
-- Name: dataset_columns_dataset_ordinal_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dataset_columns_dataset_ordinal_key ON public.dataset_columns USING btree (dataset_id, ordinal);


--
-- Name: dataset_gold_publications_active_table_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dataset_gold_publications_active_table_key ON public.dataset_gold_publications USING btree (tenant_id, gold_database, gold_table) WHERE (archived_at IS NULL);


--
-- Name: dataset_gold_publications_last_publish_run_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_gold_publications_last_publish_run_idx ON public.dataset_gold_publications USING btree (last_publish_run_id) WHERE (last_publish_run_id IS NOT NULL);


--
-- Name: dataset_gold_publications_source_work_table_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_gold_publications_source_work_table_idx ON public.dataset_gold_publications USING btree (source_work_table_id, updated_at DESC, id DESC);


--
-- Name: dataset_gold_publications_tenant_status_updated_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_gold_publications_tenant_status_updated_idx ON public.dataset_gold_publications USING btree (tenant_id, status, updated_at DESC, id DESC);


--
-- Name: dataset_gold_publish_runs_active_publication_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dataset_gold_publish_runs_active_publication_key ON public.dataset_gold_publish_runs USING btree (publication_id) WHERE (status = ANY (ARRAY['pending'::text, 'processing'::text]));


--
-- Name: dataset_gold_publish_runs_outbox_event_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_gold_publish_runs_outbox_event_idx ON public.dataset_gold_publish_runs USING btree (outbox_event_id) WHERE (outbox_event_id IS NOT NULL);


--
-- Name: dataset_gold_publish_runs_publication_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_gold_publish_runs_publication_created_idx ON public.dataset_gold_publish_runs USING btree (publication_id, created_at DESC, id DESC);


--
-- Name: dataset_gold_publish_runs_source_work_table_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_gold_publish_runs_source_work_table_idx ON public.dataset_gold_publish_runs USING btree (source_work_table_id, created_at DESC, id DESC);


--
-- Name: dataset_gold_publish_runs_tenant_status_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_gold_publish_runs_tenant_status_created_idx ON public.dataset_gold_publish_runs USING btree (tenant_id, status, created_at DESC, id DESC);


--
-- Name: dataset_import_jobs_pending_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_import_jobs_pending_idx ON public.dataset_import_jobs USING btree (created_at, id) WHERE (status = ANY (ARRAY['pending'::text, 'processing'::text]));


--
-- Name: dataset_import_jobs_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dataset_import_jobs_public_id_key ON public.dataset_import_jobs USING btree (public_id);


--
-- Name: dataset_import_jobs_tenant_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_import_jobs_tenant_created_idx ON public.dataset_import_jobs USING btree (tenant_id, created_at DESC, id DESC);


--
-- Name: dataset_lineage_change_sets_root_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_lineage_change_sets_root_idx ON public.dataset_lineage_change_sets USING btree (tenant_id, root_resource_type, root_resource_public_id, status, updated_at DESC, id DESC);


--
-- Name: dataset_lineage_change_sets_tenant_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_lineage_change_sets_tenant_status_idx ON public.dataset_lineage_change_sets USING btree (tenant_id, status, updated_at DESC, id DESC);


--
-- Name: dataset_lineage_edges_change_set_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_lineage_edges_change_set_idx ON public.dataset_lineage_edges USING btree (change_set_id, id);


--
-- Name: dataset_lineage_nodes_change_set_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_lineage_nodes_change_set_idx ON public.dataset_lineage_nodes USING btree (change_set_id, id);


--
-- Name: dataset_lineage_nodes_resource_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_lineage_nodes_resource_idx ON public.dataset_lineage_nodes USING btree (tenant_id, resource_type, resource_public_id) WHERE (resource_public_id IS NOT NULL);


--
-- Name: dataset_lineage_parse_runs_query_job_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_lineage_parse_runs_query_job_idx ON public.dataset_lineage_parse_runs USING btree (tenant_id, query_job_id, created_at DESC, id DESC);


--
-- Name: dataset_query_jobs_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dataset_query_jobs_public_id_key ON public.dataset_query_jobs USING btree (public_id);


--
-- Name: dataset_query_jobs_tenant_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_query_jobs_tenant_created_idx ON public.dataset_query_jobs USING btree (tenant_id, created_at DESC, id DESC);


--
-- Name: dataset_query_jobs_tenant_dataset_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_query_jobs_tenant_dataset_created_idx ON public.dataset_query_jobs USING btree (tenant_id, dataset_id, created_at DESC, id DESC) WHERE (dataset_id IS NOT NULL);


--
-- Name: dataset_sync_jobs_active_dataset_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dataset_sync_jobs_active_dataset_key ON public.dataset_sync_jobs USING btree (dataset_id) WHERE (status = ANY (ARRAY['pending'::text, 'processing'::text]));


--
-- Name: dataset_sync_jobs_dataset_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_sync_jobs_dataset_created_idx ON public.dataset_sync_jobs USING btree (dataset_id, created_at DESC, id DESC);


--
-- Name: dataset_sync_jobs_source_work_table_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_sync_jobs_source_work_table_created_idx ON public.dataset_sync_jobs USING btree (source_work_table_id, created_at DESC, id DESC);


--
-- Name: dataset_sync_jobs_tenant_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_sync_jobs_tenant_status_idx ON public.dataset_sync_jobs USING btree (tenant_id, status, created_at DESC, id DESC);


--
-- Name: dataset_work_table_export_schedules_due_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_work_table_export_schedules_due_idx ON public.dataset_work_table_export_schedules USING btree (next_run_at, id) WHERE enabled;


--
-- Name: dataset_work_table_export_schedules_tenant_enabled_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_work_table_export_schedules_tenant_enabled_idx ON public.dataset_work_table_export_schedules USING btree (tenant_id, enabled, next_run_at);


--
-- Name: dataset_work_table_export_schedules_work_table_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_work_table_export_schedules_work_table_created_idx ON public.dataset_work_table_export_schedules USING btree (work_table_id, created_at DESC, id DESC);


--
-- Name: dataset_work_table_exports_pending_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_work_table_exports_pending_idx ON public.dataset_work_table_exports USING btree (created_at, id) WHERE (status = ANY (ARRAY['pending'::text, 'processing'::text]));


--
-- Name: dataset_work_table_exports_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dataset_work_table_exports_public_id_key ON public.dataset_work_table_exports USING btree (public_id);


--
-- Name: dataset_work_table_exports_schedule_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_work_table_exports_schedule_created_idx ON public.dataset_work_table_exports USING btree (schedule_id, created_at DESC, id DESC) WHERE (schedule_id IS NOT NULL);


--
-- Name: dataset_work_table_exports_work_table_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_work_table_exports_work_table_created_idx ON public.dataset_work_table_exports USING btree (work_table_id, created_at DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: dataset_work_tables_active_table_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dataset_work_tables_active_table_key ON public.dataset_work_tables USING btree (tenant_id, work_database, work_table) WHERE ((status = 'active'::text) AND (dropped_at IS NULL));


--
-- Name: dataset_work_tables_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX dataset_work_tables_public_id_key ON public.dataset_work_tables USING btree (public_id);


--
-- Name: dataset_work_tables_tenant_dataset_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_work_tables_tenant_dataset_idx ON public.dataset_work_tables USING btree (tenant_id, source_dataset_id, updated_at DESC, id DESC) WHERE (source_dataset_id IS NOT NULL);


--
-- Name: dataset_work_tables_tenant_query_job_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_work_tables_tenant_query_job_idx ON public.dataset_work_tables USING btree (tenant_id, created_from_query_job_id, updated_at DESC, id DESC) WHERE (created_from_query_job_id IS NOT NULL);


--
-- Name: dataset_work_tables_tenant_updated_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX dataset_work_tables_tenant_updated_idx ON public.dataset_work_tables USING btree (tenant_id, updated_at DESC, id DESC);


--
-- Name: datasets_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX datasets_public_id_key ON public.datasets USING btree (public_id);


--
-- Name: datasets_source_file_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX datasets_source_file_idx ON public.datasets USING btree (source_file_object_id);


--
-- Name: datasets_source_work_table_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX datasets_source_work_table_idx ON public.datasets USING btree (source_work_table_id) WHERE (source_work_table_id IS NOT NULL);


--
-- Name: datasets_tenant_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX datasets_tenant_created_idx ON public.datasets USING btree (tenant_id, created_at DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: datasets_tenant_raw_table_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX datasets_tenant_raw_table_key ON public.datasets USING btree (tenant_id, raw_table);


--
-- Name: drive_admin_content_access_sessions_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_admin_content_access_sessions_active_idx ON public.drive_admin_content_access_sessions USING btree (tenant_id, actor_user_id, expires_at) WHERE (ended_at IS NULL);


--
-- Name: drive_admin_content_access_sessions_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_admin_content_access_sessions_public_id_key ON public.drive_admin_content_access_sessions USING btree (public_id);


--
-- Name: drive_ai_classifications_file_revision_label_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_ai_classifications_file_revision_label_key ON public.drive_ai_classifications USING btree (file_object_id, file_revision, label);


--
-- Name: drive_ai_jobs_file_revision_type_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_ai_jobs_file_revision_type_key ON public.drive_ai_jobs USING btree (file_object_id, file_revision, job_type);


--
-- Name: drive_ai_jobs_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_ai_jobs_public_id_key ON public.drive_ai_jobs USING btree (public_id);


--
-- Name: drive_ai_summaries_file_revision_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_ai_summaries_file_revision_key ON public.drive_ai_summaries USING btree (file_object_id, file_revision);


--
-- Name: drive_ai_summaries_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_ai_summaries_public_id_key ON public.drive_ai_summaries USING btree (public_id);


--
-- Name: drive_app_webhook_deliveries_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_app_webhook_deliveries_public_id_key ON public.drive_app_webhook_deliveries USING btree (public_id);


--
-- Name: drive_clean_rooms_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_clean_rooms_public_id_key ON public.drive_clean_rooms USING btree (public_id);


--
-- Name: drive_clean_rooms_tenant_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_clean_rooms_tenant_status_idx ON public.drive_clean_rooms USING btree (tenant_id, status, created_at DESC);


--
-- Name: drive_e2ee_file_keys_file_version_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_e2ee_file_keys_file_version_key ON public.drive_e2ee_file_keys USING btree (file_object_id, key_version);


--
-- Name: drive_e2ee_file_keys_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_e2ee_file_keys_public_id_key ON public.drive_e2ee_file_keys USING btree (public_id);


--
-- Name: drive_e2ee_key_envelopes_unique_recipient; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_e2ee_key_envelopes_unique_recipient ON public.drive_e2ee_key_envelopes USING btree (file_key_id, recipient_user_id, recipient_key_id);


--
-- Name: drive_e2ee_user_keys_one_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_e2ee_user_keys_one_active ON public.drive_e2ee_user_keys USING btree (tenant_id, user_id) WHERE (status = 'active'::text);


--
-- Name: drive_e2ee_user_keys_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_e2ee_user_keys_public_id_key ON public.drive_e2ee_user_keys USING btree (public_id);


--
-- Name: drive_ediscovery_export_items_export_file_revision_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_ediscovery_export_items_export_file_revision_key ON public.drive_ediscovery_export_items USING btree (export_id, file_object_id, file_revision);


--
-- Name: drive_ediscovery_exports_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_ediscovery_exports_public_id_key ON public.drive_ediscovery_exports USING btree (public_id);


--
-- Name: drive_ediscovery_exports_tenant_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_ediscovery_exports_tenant_status_idx ON public.drive_ediscovery_exports USING btree (tenant_id, status);


--
-- Name: drive_ediscovery_provider_connections_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_ediscovery_provider_connections_public_id_key ON public.drive_ediscovery_provider_connections USING btree (public_id);


--
-- Name: drive_ediscovery_provider_connections_tenant_provider_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_ediscovery_provider_connections_tenant_provider_key ON public.drive_ediscovery_provider_connections USING btree (tenant_id, provider);


--
-- Name: drive_edit_locks_active_file_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_edit_locks_active_file_key ON public.drive_edit_locks USING btree (file_object_id);


--
-- Name: drive_edit_locks_expiry_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_edit_locks_expiry_idx ON public.drive_edit_locks USING btree (expires_at);


--
-- Name: drive_edit_locks_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_edit_locks_public_id_key ON public.drive_edit_locks USING btree (public_id);


--
-- Name: drive_edit_sessions_file_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_edit_sessions_file_idx ON public.drive_edit_sessions USING btree (file_object_id, status, expires_at);


--
-- Name: drive_edit_sessions_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_edit_sessions_public_id_key ON public.drive_edit_sessions USING btree (public_id);


--
-- Name: drive_file_previews_file_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_file_previews_file_key ON public.drive_file_previews USING btree (tenant_id, file_object_id);


--
-- Name: drive_file_previews_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_file_previews_public_id_key ON public.drive_file_previews USING btree (public_id);


--
-- Name: drive_file_previews_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_file_previews_status_idx ON public.drive_file_previews USING btree (tenant_id, status, updated_at);


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
-- Name: drive_folders_trash_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_folders_trash_idx ON public.drive_folders USING btree (tenant_id, deleted_at DESC, id DESC) WHERE (deleted_at IS NOT NULL);


--
-- Name: drive_folders_workspace_children_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_folders_workspace_children_idx ON public.drive_folders USING btree (workspace_id, parent_folder_id, name, id) WHERE (deleted_at IS NULL);


--
-- Name: drive_gateway_objects_file_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_gateway_objects_file_key ON public.drive_gateway_objects USING btree (file_object_id);


--
-- Name: drive_gateway_objects_gateway_object_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_gateway_objects_gateway_object_key ON public.drive_gateway_objects USING btree (gateway_id, gateway_object_key);


--
-- Name: drive_gateway_transfers_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_gateway_transfers_public_id_key ON public.drive_gateway_transfers USING btree (public_id);


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
-- Name: drive_hsm_deployments_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_hsm_deployments_public_id_key ON public.drive_hsm_deployments USING btree (public_id);


--
-- Name: drive_hsm_deployments_tenant_provider_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_hsm_deployments_tenant_provider_key ON public.drive_hsm_deployments USING btree (tenant_id, provider);


--
-- Name: drive_hsm_key_bindings_unique_scope; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_hsm_key_bindings_unique_scope ON public.drive_hsm_key_bindings USING btree (tenant_id, binding_scope, COALESCE(workspace_id, (0)::bigint), COALESCE(file_object_id, (0)::bigint));


--
-- Name: drive_hsm_keys_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_hsm_keys_public_id_key ON public.drive_hsm_keys USING btree (public_id);


--
-- Name: drive_hsm_keys_tenant_ref_version_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_hsm_keys_tenant_ref_version_key ON public.drive_hsm_keys USING btree (tenant_id, key_ref, key_version);


--
-- Name: drive_index_jobs_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_index_jobs_public_id_key ON public.drive_index_jobs USING btree (public_id);


--
-- Name: drive_index_jobs_tenant_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_index_jobs_tenant_status_idx ON public.drive_index_jobs USING btree (tenant_id, status, created_at);


--
-- Name: drive_item_activities_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_item_activities_public_id_key ON public.drive_item_activities USING btree (public_id);


--
-- Name: drive_item_activities_recent_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_item_activities_recent_idx ON public.drive_item_activities USING btree (tenant_id, actor_user_id, created_at DESC, id DESC);


--
-- Name: drive_item_activities_resource_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_item_activities_resource_idx ON public.drive_item_activities USING btree (tenant_id, resource_type, resource_id, created_at DESC, id DESC);


--
-- Name: drive_item_tags_active_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_item_tags_active_key ON public.drive_item_tags USING btree (tenant_id, resource_type, resource_id, normalized_tag);


--
-- Name: drive_item_tags_resource_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_item_tags_resource_idx ON public.drive_item_tags USING btree (tenant_id, resource_type, resource_id, tag);


--
-- Name: drive_kms_keys_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_kms_keys_public_id_key ON public.drive_kms_keys USING btree (public_id);


--
-- Name: drive_kms_keys_tenant_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_kms_keys_tenant_status_idx ON public.drive_kms_keys USING btree (tenant_id, status);


--
-- Name: drive_legal_cases_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_legal_cases_public_id_key ON public.drive_legal_cases USING btree (public_id);


--
-- Name: drive_legal_cases_tenant_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_legal_cases_tenant_status_idx ON public.drive_legal_cases USING btree (tenant_id, status, created_at DESC);


--
-- Name: drive_legal_holds_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_legal_holds_active_idx ON public.drive_legal_holds USING btree (tenant_id, resource_type, resource_id) WHERE (released_at IS NULL);


--
-- Name: drive_marketplace_app_versions_app_version_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_marketplace_app_versions_app_version_key ON public.drive_marketplace_app_versions USING btree (app_id, version);


--
-- Name: drive_marketplace_apps_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_marketplace_apps_public_id_key ON public.drive_marketplace_apps USING btree (public_id);


--
-- Name: drive_marketplace_installation_scopes_scope_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_marketplace_installation_scopes_scope_key ON public.drive_marketplace_installation_scopes USING btree (installation_id, scope);


--
-- Name: drive_marketplace_installations_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_marketplace_installations_public_id_key ON public.drive_marketplace_installations USING btree (public_id);


--
-- Name: drive_marketplace_installations_tenant_app_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_marketplace_installations_tenant_app_key ON public.drive_marketplace_installations USING btree (tenant_id, app_id);


--
-- Name: drive_ocr_pages_file_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_ocr_pages_file_idx ON public.drive_ocr_pages USING btree (tenant_id, file_object_id, page_number);


--
-- Name: drive_ocr_pages_run_page_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_ocr_pages_run_page_key ON public.drive_ocr_pages USING btree (ocr_run_id, page_number);


--
-- Name: drive_ocr_runs_file_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_ocr_runs_file_idx ON public.drive_ocr_runs USING btree (tenant_id, file_object_id, created_at DESC);


--
-- Name: drive_ocr_runs_file_revision_provider_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_ocr_runs_file_revision_provider_key ON public.drive_ocr_runs USING btree (file_object_id, file_revision, content_sha256, engine, structured_extractor, pipeline_config_hash);


--
-- Name: drive_ocr_runs_pending_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_ocr_runs_pending_idx ON public.drive_ocr_runs USING btree (tenant_id, created_at, id) WHERE (status = ANY (ARRAY['pending'::text, 'running'::text]));


--
-- Name: drive_ocr_runs_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_ocr_runs_public_id_key ON public.drive_ocr_runs USING btree (public_id);


--
-- Name: drive_ocr_runs_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_ocr_runs_status_idx ON public.drive_ocr_runs USING btree (tenant_id, status, created_at DESC);


--
-- Name: drive_office_edit_sessions_file_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_office_edit_sessions_file_idx ON public.drive_office_edit_sessions USING btree (tenant_id, file_object_id, revoked_at, expires_at);


--
-- Name: drive_office_edit_sessions_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_office_edit_sessions_public_id_key ON public.drive_office_edit_sessions USING btree (public_id);


--
-- Name: drive_office_provider_files_provider_file_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_office_provider_files_provider_file_key ON public.drive_office_provider_files USING btree (provider, provider_file_id);


--
-- Name: drive_office_provider_files_tenant_file_provider_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_office_provider_files_tenant_file_provider_key ON public.drive_office_provider_files USING btree (tenant_id, file_object_id, provider);


--
-- Name: drive_office_webhook_events_provider_event_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_office_webhook_events_provider_event_key ON public.drive_office_webhook_events USING btree (provider, provider_event_id);


--
-- Name: drive_presence_sessions_file_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_presence_sessions_file_idx ON public.drive_presence_sessions USING btree (file_object_id, status, last_seen_at DESC);


--
-- Name: drive_product_extraction_items_file_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_product_extraction_items_file_idx ON public.drive_product_extraction_items USING btree (tenant_id, file_object_id, created_at DESC);


--
-- Name: drive_product_extraction_items_jan_code_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_product_extraction_items_jan_code_idx ON public.drive_product_extraction_items USING btree (tenant_id, jan_code) WHERE (jan_code IS NOT NULL);


--
-- Name: drive_product_extraction_items_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_product_extraction_items_public_id_key ON public.drive_product_extraction_items USING btree (public_id);


--
-- Name: drive_product_extraction_items_run_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_product_extraction_items_run_idx ON public.drive_product_extraction_items USING btree (ocr_run_id, id);


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
-- Name: drive_search_documents_file_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_search_documents_file_key ON public.drive_search_documents USING btree (file_object_id);


--
-- Name: drive_search_documents_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_search_documents_public_id_key ON public.drive_search_documents USING btree (public_id);


--
-- Name: drive_search_documents_tenant_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_search_documents_tenant_idx ON public.drive_search_documents USING btree (tenant_id, indexed_at DESC);


--
-- Name: drive_search_documents_vector_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_search_documents_vector_idx ON public.drive_search_documents USING gin (search_vector);


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
-- Name: drive_starred_items_active_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_starred_items_active_key ON public.drive_starred_items USING btree (tenant_id, user_id, resource_type, resource_id) WHERE (deleted_at IS NULL);


--
-- Name: drive_starred_items_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_starred_items_public_id_key ON public.drive_starred_items USING btree (public_id);


--
-- Name: drive_starred_items_user_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_starred_items_user_idx ON public.drive_starred_items USING btree (tenant_id, user_id, created_at DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: drive_storage_gateways_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_storage_gateways_public_id_key ON public.drive_storage_gateways USING btree (public_id);


--
-- Name: drive_storage_gateways_tenant_name_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_storage_gateways_tenant_name_key ON public.drive_storage_gateways USING btree (tenant_id, name);


--
-- Name: drive_sync_devices_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_sync_devices_public_id_key ON public.drive_sync_devices USING btree (public_id);


--
-- Name: drive_sync_devices_tenant_user_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_sync_devices_tenant_user_idx ON public.drive_sync_devices USING btree (tenant_id, user_id, status);


--
-- Name: drive_sync_devices_token_hash_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_sync_devices_token_hash_key ON public.drive_sync_devices USING btree (token_hash);


--
-- Name: drive_sync_events_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX drive_sync_events_public_id_key ON public.drive_sync_events USING btree (public_id);


--
-- Name: drive_sync_events_tenant_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX drive_sync_events_tenant_id_idx ON public.drive_sync_events USING btree (tenant_id, id);


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
-- Name: file_objects_drive_trash_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX file_objects_drive_trash_idx ON public.file_objects USING btree (tenant_id, deleted_at DESC, id DESC) WHERE ((purpose = 'drive'::text) AND (deleted_at IS NOT NULL) AND (purged_at IS NULL));


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
-- Name: medallion_asset_edges_source_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX medallion_asset_edges_source_idx ON public.medallion_asset_edges USING btree (source_asset_id, created_at DESC, id DESC);


--
-- Name: medallion_asset_edges_target_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX medallion_asset_edges_target_idx ON public.medallion_asset_edges USING btree (target_asset_id, created_at DESC, id DESC);


--
-- Name: medallion_assets_tenant_layer_updated_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX medallion_assets_tenant_layer_updated_idx ON public.medallion_assets USING btree (tenant_id, layer, updated_at DESC, id DESC);


--
-- Name: medallion_assets_tenant_resource_public_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX medallion_assets_tenant_resource_public_idx ON public.medallion_assets USING btree (tenant_id, resource_kind, resource_public_id);


--
-- Name: medallion_pipeline_run_assets_asset_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX medallion_pipeline_run_assets_asset_idx ON public.medallion_pipeline_run_assets USING btree (asset_id, created_at DESC, id DESC);


--
-- Name: medallion_pipeline_runs_source_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX medallion_pipeline_runs_source_idx ON public.medallion_pipeline_runs USING btree (tenant_id, source_resource_kind, source_resource_id, created_at DESC) WHERE ((source_resource_kind IS NOT NULL) AND (source_resource_id IS NOT NULL));


--
-- Name: medallion_pipeline_runs_target_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX medallion_pipeline_runs_target_idx ON public.medallion_pipeline_runs USING btree (tenant_id, target_resource_kind, target_resource_id, created_at DESC) WHERE ((target_resource_kind IS NOT NULL) AND (target_resource_id IS NOT NULL));


--
-- Name: medallion_pipeline_runs_tenant_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX medallion_pipeline_runs_tenant_created_idx ON public.medallion_pipeline_runs USING btree (tenant_id, created_at DESC, id DESC);


--
-- Name: notifications_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX notifications_public_id_key ON public.notifications USING btree (public_id);


--
-- Name: notifications_recipient_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notifications_recipient_created_idx ON public.notifications USING btree (recipient_user_id, created_at DESC, id DESC);


--
-- Name: notifications_recipient_unread_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notifications_recipient_unread_created_idx ON public.notifications USING btree (recipient_user_id, created_at DESC, id DESC) WHERE (read_at IS NULL);


--
-- Name: notifications_recipient_unread_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notifications_recipient_unread_idx ON public.notifications USING btree (recipient_user_id, created_at DESC) WHERE (read_at IS NULL);


--
-- Name: notifications_search_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notifications_search_idx ON public.notifications USING gin (to_tsvector('simple'::regconfig, ((((subject || ' '::text) || body) || ' '::text) || template)));


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
-- Name: realtime_events_expires_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX realtime_events_expires_idx ON public.realtime_events USING btree (expires_at);


--
-- Name: realtime_events_public_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX realtime_events_public_id_key ON public.realtime_events USING btree (public_id);


--
-- Name: realtime_events_recipient_cursor_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX realtime_events_recipient_cursor_idx ON public.realtime_events USING btree (recipient_user_id, id);


--
-- Name: realtime_events_tenant_recipient_cursor_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX realtime_events_tenant_recipient_cursor_idx ON public.realtime_events USING btree (tenant_id, recipient_user_id, id) WHERE (tenant_id IS NOT NULL);


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
-- Name: dataset_columns dataset_columns_dataset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_columns
    ADD CONSTRAINT dataset_columns_dataset_id_fkey FOREIGN KEY (dataset_id) REFERENCES public.datasets(id) ON DELETE CASCADE;


--
-- Name: dataset_gold_publications dataset_gold_publications_archived_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publications
    ADD CONSTRAINT dataset_gold_publications_archived_by_user_id_fkey FOREIGN KEY (archived_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_gold_publications dataset_gold_publications_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publications
    ADD CONSTRAINT dataset_gold_publications_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_gold_publications dataset_gold_publications_last_publish_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publications
    ADD CONSTRAINT dataset_gold_publications_last_publish_run_id_fkey FOREIGN KEY (last_publish_run_id) REFERENCES public.dataset_gold_publish_runs(id) ON DELETE SET NULL;


--
-- Name: dataset_gold_publications dataset_gold_publications_published_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publications
    ADD CONSTRAINT dataset_gold_publications_published_by_user_id_fkey FOREIGN KEY (published_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_gold_publications dataset_gold_publications_source_work_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publications
    ADD CONSTRAINT dataset_gold_publications_source_work_table_id_fkey FOREIGN KEY (source_work_table_id) REFERENCES public.dataset_work_tables(id) ON DELETE CASCADE;


--
-- Name: dataset_gold_publications dataset_gold_publications_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publications
    ADD CONSTRAINT dataset_gold_publications_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dataset_gold_publications dataset_gold_publications_unpublished_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publications
    ADD CONSTRAINT dataset_gold_publications_unpublished_by_user_id_fkey FOREIGN KEY (unpublished_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_gold_publications dataset_gold_publications_updated_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publications
    ADD CONSTRAINT dataset_gold_publications_updated_by_user_id_fkey FOREIGN KEY (updated_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_gold_publish_runs dataset_gold_publish_runs_outbox_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publish_runs
    ADD CONSTRAINT dataset_gold_publish_runs_outbox_event_id_fkey FOREIGN KEY (outbox_event_id) REFERENCES public.outbox_events(id) ON DELETE SET NULL;


--
-- Name: dataset_gold_publish_runs dataset_gold_publish_runs_publication_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publish_runs
    ADD CONSTRAINT dataset_gold_publish_runs_publication_id_fkey FOREIGN KEY (publication_id) REFERENCES public.dataset_gold_publications(id) ON DELETE CASCADE;


--
-- Name: dataset_gold_publish_runs dataset_gold_publish_runs_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publish_runs
    ADD CONSTRAINT dataset_gold_publish_runs_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_gold_publish_runs dataset_gold_publish_runs_source_work_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publish_runs
    ADD CONSTRAINT dataset_gold_publish_runs_source_work_table_id_fkey FOREIGN KEY (source_work_table_id) REFERENCES public.dataset_work_tables(id) ON DELETE CASCADE;


--
-- Name: dataset_gold_publish_runs dataset_gold_publish_runs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_gold_publish_runs
    ADD CONSTRAINT dataset_gold_publish_runs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dataset_import_jobs dataset_import_jobs_dataset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_import_jobs
    ADD CONSTRAINT dataset_import_jobs_dataset_id_fkey FOREIGN KEY (dataset_id) REFERENCES public.datasets(id) ON DELETE CASCADE;


--
-- Name: dataset_import_jobs dataset_import_jobs_outbox_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_import_jobs
    ADD CONSTRAINT dataset_import_jobs_outbox_event_id_fkey FOREIGN KEY (outbox_event_id) REFERENCES public.outbox_events(id) ON DELETE SET NULL;


--
-- Name: dataset_import_jobs dataset_import_jobs_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_import_jobs
    ADD CONSTRAINT dataset_import_jobs_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_import_jobs dataset_import_jobs_source_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_import_jobs
    ADD CONSTRAINT dataset_import_jobs_source_file_object_id_fkey FOREIGN KEY (source_file_object_id) REFERENCES public.file_objects(id) ON DELETE RESTRICT;


--
-- Name: dataset_import_jobs dataset_import_jobs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_import_jobs
    ADD CONSTRAINT dataset_import_jobs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dataset_lineage_change_sets dataset_lineage_change_sets_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_change_sets
    ADD CONSTRAINT dataset_lineage_change_sets_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_lineage_change_sets dataset_lineage_change_sets_published_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_change_sets
    ADD CONSTRAINT dataset_lineage_change_sets_published_by_user_id_fkey FOREIGN KEY (published_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_lineage_change_sets dataset_lineage_change_sets_query_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_change_sets
    ADD CONSTRAINT dataset_lineage_change_sets_query_job_id_fkey FOREIGN KEY (query_job_id) REFERENCES public.dataset_query_jobs(id) ON DELETE SET NULL;


--
-- Name: dataset_lineage_change_sets dataset_lineage_change_sets_rejected_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_change_sets
    ADD CONSTRAINT dataset_lineage_change_sets_rejected_by_user_id_fkey FOREIGN KEY (rejected_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_lineage_change_sets dataset_lineage_change_sets_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_change_sets
    ADD CONSTRAINT dataset_lineage_change_sets_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dataset_lineage_edges dataset_lineage_edges_change_set_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_edges
    ADD CONSTRAINT dataset_lineage_edges_change_set_id_fkey FOREIGN KEY (change_set_id) REFERENCES public.dataset_lineage_change_sets(id) ON DELETE CASCADE;


--
-- Name: dataset_lineage_edges dataset_lineage_edges_source_node_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_edges
    ADD CONSTRAINT dataset_lineage_edges_source_node_fkey FOREIGN KEY (change_set_id, source_node_key) REFERENCES public.dataset_lineage_nodes(change_set_id, node_key) ON DELETE CASCADE;


--
-- Name: dataset_lineage_edges dataset_lineage_edges_target_node_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_edges
    ADD CONSTRAINT dataset_lineage_edges_target_node_fkey FOREIGN KEY (change_set_id, target_node_key) REFERENCES public.dataset_lineage_nodes(change_set_id, node_key) ON DELETE CASCADE;


--
-- Name: dataset_lineage_edges dataset_lineage_edges_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_edges
    ADD CONSTRAINT dataset_lineage_edges_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dataset_lineage_nodes dataset_lineage_nodes_change_set_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_nodes
    ADD CONSTRAINT dataset_lineage_nodes_change_set_id_fkey FOREIGN KEY (change_set_id) REFERENCES public.dataset_lineage_change_sets(id) ON DELETE CASCADE;


--
-- Name: dataset_lineage_nodes dataset_lineage_nodes_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_nodes
    ADD CONSTRAINT dataset_lineage_nodes_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dataset_lineage_parse_runs dataset_lineage_parse_runs_change_set_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_parse_runs
    ADD CONSTRAINT dataset_lineage_parse_runs_change_set_id_fkey FOREIGN KEY (change_set_id) REFERENCES public.dataset_lineage_change_sets(id) ON DELETE SET NULL;


--
-- Name: dataset_lineage_parse_runs dataset_lineage_parse_runs_query_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_parse_runs
    ADD CONSTRAINT dataset_lineage_parse_runs_query_job_id_fkey FOREIGN KEY (query_job_id) REFERENCES public.dataset_query_jobs(id) ON DELETE CASCADE;


--
-- Name: dataset_lineage_parse_runs dataset_lineage_parse_runs_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_parse_runs
    ADD CONSTRAINT dataset_lineage_parse_runs_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_lineage_parse_runs dataset_lineage_parse_runs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_lineage_parse_runs
    ADD CONSTRAINT dataset_lineage_parse_runs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dataset_query_jobs dataset_query_jobs_dataset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_query_jobs
    ADD CONSTRAINT dataset_query_jobs_dataset_id_fkey FOREIGN KEY (dataset_id) REFERENCES public.datasets(id) ON DELETE SET NULL;


--
-- Name: dataset_query_jobs dataset_query_jobs_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_query_jobs
    ADD CONSTRAINT dataset_query_jobs_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_query_jobs dataset_query_jobs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_query_jobs
    ADD CONSTRAINT dataset_query_jobs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dataset_sync_jobs dataset_sync_jobs_dataset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_sync_jobs
    ADD CONSTRAINT dataset_sync_jobs_dataset_id_fkey FOREIGN KEY (dataset_id) REFERENCES public.datasets(id) ON DELETE CASCADE;


--
-- Name: dataset_sync_jobs dataset_sync_jobs_outbox_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_sync_jobs
    ADD CONSTRAINT dataset_sync_jobs_outbox_event_id_fkey FOREIGN KEY (outbox_event_id) REFERENCES public.outbox_events(id) ON DELETE SET NULL;


--
-- Name: dataset_sync_jobs dataset_sync_jobs_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_sync_jobs
    ADD CONSTRAINT dataset_sync_jobs_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_sync_jobs dataset_sync_jobs_source_work_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_sync_jobs
    ADD CONSTRAINT dataset_sync_jobs_source_work_table_id_fkey FOREIGN KEY (source_work_table_id) REFERENCES public.dataset_work_tables(id) ON DELETE CASCADE;


--
-- Name: dataset_sync_jobs dataset_sync_jobs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_sync_jobs
    ADD CONSTRAINT dataset_sync_jobs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dataset_work_table_export_schedules dataset_work_table_export_schedules_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_export_schedules
    ADD CONSTRAINT dataset_work_table_export_schedules_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_work_table_export_schedules dataset_work_table_export_schedules_last_export_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_export_schedules
    ADD CONSTRAINT dataset_work_table_export_schedules_last_export_id_fkey FOREIGN KEY (last_export_id) REFERENCES public.dataset_work_table_exports(id) ON DELETE SET NULL;


--
-- Name: dataset_work_table_export_schedules dataset_work_table_export_schedules_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_export_schedules
    ADD CONSTRAINT dataset_work_table_export_schedules_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dataset_work_table_export_schedules dataset_work_table_export_schedules_work_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_export_schedules
    ADD CONSTRAINT dataset_work_table_export_schedules_work_table_id_fkey FOREIGN KEY (work_table_id) REFERENCES public.dataset_work_tables(id) ON DELETE CASCADE;


--
-- Name: dataset_work_table_exports dataset_work_table_exports_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_exports
    ADD CONSTRAINT dataset_work_table_exports_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE SET NULL;


--
-- Name: dataset_work_table_exports dataset_work_table_exports_outbox_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_exports
    ADD CONSTRAINT dataset_work_table_exports_outbox_event_id_fkey FOREIGN KEY (outbox_event_id) REFERENCES public.outbox_events(id) ON DELETE SET NULL;


--
-- Name: dataset_work_table_exports dataset_work_table_exports_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_exports
    ADD CONSTRAINT dataset_work_table_exports_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_work_table_exports dataset_work_table_exports_schedule_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_exports
    ADD CONSTRAINT dataset_work_table_exports_schedule_id_fkey FOREIGN KEY (schedule_id) REFERENCES public.dataset_work_table_export_schedules(id) ON DELETE SET NULL;


--
-- Name: dataset_work_table_exports dataset_work_table_exports_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_exports
    ADD CONSTRAINT dataset_work_table_exports_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: dataset_work_table_exports dataset_work_table_exports_work_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_table_exports
    ADD CONSTRAINT dataset_work_table_exports_work_table_id_fkey FOREIGN KEY (work_table_id) REFERENCES public.dataset_work_tables(id) ON DELETE CASCADE;


--
-- Name: dataset_work_tables dataset_work_tables_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_tables
    ADD CONSTRAINT dataset_work_tables_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: dataset_work_tables dataset_work_tables_created_from_query_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_tables
    ADD CONSTRAINT dataset_work_tables_created_from_query_job_id_fkey FOREIGN KEY (created_from_query_job_id) REFERENCES public.dataset_query_jobs(id) ON DELETE SET NULL;


--
-- Name: dataset_work_tables dataset_work_tables_source_dataset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_tables
    ADD CONSTRAINT dataset_work_tables_source_dataset_id_fkey FOREIGN KEY (source_dataset_id) REFERENCES public.datasets(id) ON DELETE SET NULL;


--
-- Name: dataset_work_tables dataset_work_tables_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dataset_work_tables
    ADD CONSTRAINT dataset_work_tables_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: datasets datasets_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.datasets
    ADD CONSTRAINT datasets_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: datasets datasets_source_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.datasets
    ADD CONSTRAINT datasets_source_file_object_id_fkey FOREIGN KEY (source_file_object_id) REFERENCES public.file_objects(id) ON DELETE RESTRICT;


--
-- Name: datasets datasets_source_work_table_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.datasets
    ADD CONSTRAINT datasets_source_work_table_id_fkey FOREIGN KEY (source_work_table_id) REFERENCES public.dataset_work_tables(id) ON DELETE SET NULL;


--
-- Name: datasets datasets_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.datasets
    ADD CONSTRAINT datasets_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


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
-- Name: drive_ai_classifications drive_ai_classifications_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_classifications
    ADD CONSTRAINT drive_ai_classifications_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_ai_classifications drive_ai_classifications_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_classifications
    ADD CONSTRAINT drive_ai_classifications_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_ai_jobs drive_ai_jobs_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_jobs
    ADD CONSTRAINT drive_ai_jobs_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_ai_jobs drive_ai_jobs_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_jobs
    ADD CONSTRAINT drive_ai_jobs_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id);


--
-- Name: drive_ai_jobs drive_ai_jobs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_jobs
    ADD CONSTRAINT drive_ai_jobs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_ai_summaries drive_ai_summaries_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_summaries
    ADD CONSTRAINT drive_ai_summaries_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_ai_summaries drive_ai_summaries_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ai_summaries
    ADD CONSTRAINT drive_ai_summaries_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_app_webhook_deliveries drive_app_webhook_deliveries_installation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_app_webhook_deliveries
    ADD CONSTRAINT drive_app_webhook_deliveries_installation_id_fkey FOREIGN KEY (installation_id) REFERENCES public.drive_marketplace_installations(id) ON DELETE CASCADE;


--
-- Name: drive_app_webhook_deliveries drive_app_webhook_deliveries_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_app_webhook_deliveries
    ADD CONSTRAINT drive_app_webhook_deliveries_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_chain_of_custody_events drive_chain_of_custody_events_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_chain_of_custody_events
    ADD CONSTRAINT drive_chain_of_custody_events_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_chain_of_custody_events drive_chain_of_custody_events_case_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_chain_of_custody_events
    ADD CONSTRAINT drive_chain_of_custody_events_case_id_fkey FOREIGN KEY (case_id) REFERENCES public.drive_legal_cases(id) ON DELETE CASCADE;


--
-- Name: drive_chain_of_custody_events drive_chain_of_custody_events_export_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_chain_of_custody_events
    ADD CONSTRAINT drive_chain_of_custody_events_export_id_fkey FOREIGN KEY (export_id) REFERENCES public.drive_legal_exports(id) ON DELETE CASCADE;


--
-- Name: drive_chain_of_custody_events drive_chain_of_custody_events_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_chain_of_custody_events
    ADD CONSTRAINT drive_chain_of_custody_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_clean_room_datasets drive_clean_room_datasets_clean_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_datasets
    ADD CONSTRAINT drive_clean_room_datasets_clean_room_id_fkey FOREIGN KEY (clean_room_id) REFERENCES public.drive_clean_rooms(id) ON DELETE CASCADE;


--
-- Name: drive_clean_room_datasets drive_clean_room_datasets_source_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_datasets
    ADD CONSTRAINT drive_clean_room_datasets_source_file_object_id_fkey FOREIGN KEY (source_file_object_id) REFERENCES public.file_objects(id) ON DELETE RESTRICT;


--
-- Name: drive_clean_room_datasets drive_clean_room_datasets_submitted_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_datasets
    ADD CONSTRAINT drive_clean_room_datasets_submitted_by_user_id_fkey FOREIGN KEY (submitted_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_clean_room_datasets drive_clean_room_datasets_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_datasets
    ADD CONSTRAINT drive_clean_room_datasets_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_clean_room_exports drive_clean_room_exports_approved_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_exports
    ADD CONSTRAINT drive_clean_room_exports_approved_by_user_id_fkey FOREIGN KEY (approved_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_clean_room_exports drive_clean_room_exports_clean_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_exports
    ADD CONSTRAINT drive_clean_room_exports_clean_room_id_fkey FOREIGN KEY (clean_room_id) REFERENCES public.drive_clean_rooms(id) ON DELETE CASCADE;


--
-- Name: drive_clean_room_exports drive_clean_room_exports_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_exports
    ADD CONSTRAINT drive_clean_room_exports_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.drive_clean_room_jobs(id) ON DELETE SET NULL;


--
-- Name: drive_clean_room_exports drive_clean_room_exports_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_exports
    ADD CONSTRAINT drive_clean_room_exports_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_clean_room_exports drive_clean_room_exports_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_exports
    ADD CONSTRAINT drive_clean_room_exports_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_clean_room_jobs drive_clean_room_jobs_clean_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_jobs
    ADD CONSTRAINT drive_clean_room_jobs_clean_room_id_fkey FOREIGN KEY (clean_room_id) REFERENCES public.drive_clean_rooms(id) ON DELETE CASCADE;


--
-- Name: drive_clean_room_jobs drive_clean_room_jobs_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_jobs
    ADD CONSTRAINT drive_clean_room_jobs_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_clean_room_jobs drive_clean_room_jobs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_jobs
    ADD CONSTRAINT drive_clean_room_jobs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_clean_room_participants drive_clean_room_participants_clean_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_participants
    ADD CONSTRAINT drive_clean_room_participants_clean_room_id_fkey FOREIGN KEY (clean_room_id) REFERENCES public.drive_clean_rooms(id) ON DELETE CASCADE;


--
-- Name: drive_clean_room_participants drive_clean_room_participants_participant_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_participants
    ADD CONSTRAINT drive_clean_room_participants_participant_tenant_id_fkey FOREIGN KEY (participant_tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_clean_room_participants drive_clean_room_participants_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_participants
    ADD CONSTRAINT drive_clean_room_participants_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_clean_room_participants drive_clean_room_participants_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_participants
    ADD CONSTRAINT drive_clean_room_participants_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: drive_clean_room_policy_decisions drive_clean_room_policy_decisions_clean_room_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_policy_decisions
    ADD CONSTRAINT drive_clean_room_policy_decisions_clean_room_id_fkey FOREIGN KEY (clean_room_id) REFERENCES public.drive_clean_rooms(id) ON DELETE CASCADE;


--
-- Name: drive_clean_room_policy_decisions drive_clean_room_policy_decisions_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_room_policy_decisions
    ADD CONSTRAINT drive_clean_room_policy_decisions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_clean_rooms drive_clean_rooms_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_rooms
    ADD CONSTRAINT drive_clean_rooms_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_clean_rooms drive_clean_rooms_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_clean_rooms
    ADD CONSTRAINT drive_clean_rooms_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_e2ee_file_keys drive_e2ee_file_keys_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_file_keys
    ADD CONSTRAINT drive_e2ee_file_keys_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id);


--
-- Name: drive_e2ee_file_keys drive_e2ee_file_keys_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_file_keys
    ADD CONSTRAINT drive_e2ee_file_keys_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_e2ee_file_keys drive_e2ee_file_keys_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_file_keys
    ADD CONSTRAINT drive_e2ee_file_keys_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_e2ee_key_envelopes drive_e2ee_key_envelopes_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_key_envelopes
    ADD CONSTRAINT drive_e2ee_key_envelopes_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id);


--
-- Name: drive_e2ee_key_envelopes drive_e2ee_key_envelopes_file_key_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_key_envelopes
    ADD CONSTRAINT drive_e2ee_key_envelopes_file_key_id_fkey FOREIGN KEY (file_key_id) REFERENCES public.drive_e2ee_file_keys(id) ON DELETE CASCADE;


--
-- Name: drive_e2ee_key_envelopes drive_e2ee_key_envelopes_recipient_key_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_key_envelopes
    ADD CONSTRAINT drive_e2ee_key_envelopes_recipient_key_id_fkey FOREIGN KEY (recipient_key_id) REFERENCES public.drive_e2ee_user_keys(id);


--
-- Name: drive_e2ee_key_envelopes drive_e2ee_key_envelopes_recipient_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_key_envelopes
    ADD CONSTRAINT drive_e2ee_key_envelopes_recipient_user_id_fkey FOREIGN KEY (recipient_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: drive_e2ee_key_envelopes drive_e2ee_key_envelopes_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_key_envelopes
    ADD CONSTRAINT drive_e2ee_key_envelopes_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_e2ee_user_keys drive_e2ee_user_keys_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_user_keys
    ADD CONSTRAINT drive_e2ee_user_keys_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_e2ee_user_keys drive_e2ee_user_keys_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_e2ee_user_keys
    ADD CONSTRAINT drive_e2ee_user_keys_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: drive_ediscovery_export_items drive_ediscovery_export_items_export_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_export_items
    ADD CONSTRAINT drive_ediscovery_export_items_export_id_fkey FOREIGN KEY (export_id) REFERENCES public.drive_ediscovery_exports(id) ON DELETE CASCADE;


--
-- Name: drive_ediscovery_export_items drive_ediscovery_export_items_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_export_items
    ADD CONSTRAINT drive_ediscovery_export_items_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id);


--
-- Name: drive_ediscovery_exports drive_ediscovery_exports_approved_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_exports
    ADD CONSTRAINT drive_ediscovery_exports_approved_by_user_id_fkey FOREIGN KEY (approved_by_user_id) REFERENCES public.users(id);


--
-- Name: drive_ediscovery_exports drive_ediscovery_exports_case_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_exports
    ADD CONSTRAINT drive_ediscovery_exports_case_id_fkey FOREIGN KEY (case_id) REFERENCES public.drive_legal_cases(id) ON DELETE SET NULL;


--
-- Name: drive_ediscovery_exports drive_ediscovery_exports_provider_connection_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_exports
    ADD CONSTRAINT drive_ediscovery_exports_provider_connection_id_fkey FOREIGN KEY (provider_connection_id) REFERENCES public.drive_ediscovery_provider_connections(id);


--
-- Name: drive_ediscovery_exports drive_ediscovery_exports_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_exports
    ADD CONSTRAINT drive_ediscovery_exports_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id);


--
-- Name: drive_ediscovery_exports drive_ediscovery_exports_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_exports
    ADD CONSTRAINT drive_ediscovery_exports_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_ediscovery_provider_connections drive_ediscovery_provider_connections_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_provider_connections
    ADD CONSTRAINT drive_ediscovery_provider_connections_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id);


--
-- Name: drive_ediscovery_provider_connections drive_ediscovery_provider_connections_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ediscovery_provider_connections
    ADD CONSTRAINT drive_ediscovery_provider_connections_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_edit_locks drive_edit_locks_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_edit_locks
    ADD CONSTRAINT drive_edit_locks_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: drive_edit_locks drive_edit_locks_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_edit_locks
    ADD CONSTRAINT drive_edit_locks_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_edit_locks drive_edit_locks_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_edit_locks
    ADD CONSTRAINT drive_edit_locks_session_id_fkey FOREIGN KEY (session_id) REFERENCES public.drive_edit_sessions(id) ON DELETE CASCADE;


--
-- Name: drive_edit_locks drive_edit_locks_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_edit_locks
    ADD CONSTRAINT drive_edit_locks_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_edit_sessions drive_edit_sessions_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_edit_sessions
    ADD CONSTRAINT drive_edit_sessions_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: drive_edit_sessions drive_edit_sessions_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_edit_sessions
    ADD CONSTRAINT drive_edit_sessions_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_edit_sessions drive_edit_sessions_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_edit_sessions
    ADD CONSTRAINT drive_edit_sessions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_encryption_policies drive_encryption_policies_kms_key_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_encryption_policies
    ADD CONSTRAINT drive_encryption_policies_kms_key_id_fkey FOREIGN KEY (kms_key_id) REFERENCES public.drive_kms_keys(id) ON DELETE SET NULL;


--
-- Name: drive_encryption_policies drive_encryption_policies_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_encryption_policies
    ADD CONSTRAINT drive_encryption_policies_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_file_previews drive_file_previews_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_file_previews
    ADD CONSTRAINT drive_file_previews_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_file_previews drive_file_previews_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_file_previews
    ADD CONSTRAINT drive_file_previews_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


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
-- Name: drive_gateway_objects drive_gateway_objects_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_gateway_objects
    ADD CONSTRAINT drive_gateway_objects_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_gateway_objects drive_gateway_objects_gateway_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_gateway_objects
    ADD CONSTRAINT drive_gateway_objects_gateway_id_fkey FOREIGN KEY (gateway_id) REFERENCES public.drive_storage_gateways(id) ON DELETE CASCADE;


--
-- Name: drive_gateway_objects drive_gateway_objects_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_gateway_objects
    ADD CONSTRAINT drive_gateway_objects_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_gateway_transfers drive_gateway_transfers_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_gateway_transfers
    ADD CONSTRAINT drive_gateway_transfers_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE SET NULL;


--
-- Name: drive_gateway_transfers drive_gateway_transfers_gateway_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_gateway_transfers
    ADD CONSTRAINT drive_gateway_transfers_gateway_id_fkey FOREIGN KEY (gateway_id) REFERENCES public.drive_storage_gateways(id) ON DELETE CASCADE;


--
-- Name: drive_gateway_transfers drive_gateway_transfers_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_gateway_transfers
    ADD CONSTRAINT drive_gateway_transfers_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


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
-- Name: drive_hsm_deployments drive_hsm_deployments_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_deployments
    ADD CONSTRAINT drive_hsm_deployments_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id);


--
-- Name: drive_hsm_deployments drive_hsm_deployments_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_deployments
    ADD CONSTRAINT drive_hsm_deployments_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_hsm_key_bindings drive_hsm_key_bindings_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_key_bindings
    ADD CONSTRAINT drive_hsm_key_bindings_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_hsm_key_bindings drive_hsm_key_bindings_hsm_key_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_key_bindings
    ADD CONSTRAINT drive_hsm_key_bindings_hsm_key_id_fkey FOREIGN KEY (hsm_key_id) REFERENCES public.drive_hsm_keys(id);


--
-- Name: drive_hsm_key_bindings drive_hsm_key_bindings_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_key_bindings
    ADD CONSTRAINT drive_hsm_key_bindings_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_hsm_key_bindings drive_hsm_key_bindings_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_key_bindings
    ADD CONSTRAINT drive_hsm_key_bindings_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.drive_workspaces(id) ON DELETE CASCADE;


--
-- Name: drive_hsm_keys drive_hsm_keys_deployment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_keys
    ADD CONSTRAINT drive_hsm_keys_deployment_id_fkey FOREIGN KEY (deployment_id) REFERENCES public.drive_hsm_deployments(id) ON DELETE CASCADE;


--
-- Name: drive_hsm_keys drive_hsm_keys_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_hsm_keys
    ADD CONSTRAINT drive_hsm_keys_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_index_jobs drive_index_jobs_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_index_jobs
    ADD CONSTRAINT drive_index_jobs_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_index_jobs drive_index_jobs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_index_jobs
    ADD CONSTRAINT drive_index_jobs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_item_activities drive_item_activities_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_item_activities
    ADD CONSTRAINT drive_item_activities_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_item_activities drive_item_activities_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_item_activities
    ADD CONSTRAINT drive_item_activities_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_item_tags drive_item_tags_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_item_tags
    ADD CONSTRAINT drive_item_tags_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_item_tags drive_item_tags_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_item_tags
    ADD CONSTRAINT drive_item_tags_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_key_rotation_jobs drive_key_rotation_jobs_new_kms_key_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_key_rotation_jobs
    ADD CONSTRAINT drive_key_rotation_jobs_new_kms_key_id_fkey FOREIGN KEY (new_kms_key_id) REFERENCES public.drive_kms_keys(id) ON DELETE SET NULL;


--
-- Name: drive_key_rotation_jobs drive_key_rotation_jobs_old_kms_key_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_key_rotation_jobs
    ADD CONSTRAINT drive_key_rotation_jobs_old_kms_key_id_fkey FOREIGN KEY (old_kms_key_id) REFERENCES public.drive_kms_keys(id) ON DELETE SET NULL;


--
-- Name: drive_key_rotation_jobs drive_key_rotation_jobs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_key_rotation_jobs
    ADD CONSTRAINT drive_key_rotation_jobs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_kms_keys drive_kms_keys_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_kms_keys
    ADD CONSTRAINT drive_kms_keys_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_kms_keys drive_kms_keys_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_kms_keys
    ADD CONSTRAINT drive_kms_keys_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_legal_case_resources drive_legal_case_resources_added_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_case_resources
    ADD CONSTRAINT drive_legal_case_resources_added_by_user_id_fkey FOREIGN KEY (added_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_legal_case_resources drive_legal_case_resources_case_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_case_resources
    ADD CONSTRAINT drive_legal_case_resources_case_id_fkey FOREIGN KEY (case_id) REFERENCES public.drive_legal_cases(id) ON DELETE CASCADE;


--
-- Name: drive_legal_case_resources drive_legal_case_resources_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_case_resources
    ADD CONSTRAINT drive_legal_case_resources_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_legal_cases drive_legal_cases_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_cases
    ADD CONSTRAINT drive_legal_cases_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_legal_cases drive_legal_cases_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_cases
    ADD CONSTRAINT drive_legal_cases_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_legal_export_items drive_legal_export_items_export_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_export_items
    ADD CONSTRAINT drive_legal_export_items_export_id_fkey FOREIGN KEY (export_id) REFERENCES public.drive_legal_exports(id) ON DELETE CASCADE;


--
-- Name: drive_legal_export_items drive_legal_export_items_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_export_items
    ADD CONSTRAINT drive_legal_export_items_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_legal_export_items drive_legal_export_items_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_export_items
    ADD CONSTRAINT drive_legal_export_items_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_legal_exports drive_legal_exports_approved_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_exports
    ADD CONSTRAINT drive_legal_exports_approved_by_user_id_fkey FOREIGN KEY (approved_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_legal_exports drive_legal_exports_case_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_exports
    ADD CONSTRAINT drive_legal_exports_case_id_fkey FOREIGN KEY (case_id) REFERENCES public.drive_legal_cases(id) ON DELETE CASCADE;


--
-- Name: drive_legal_exports drive_legal_exports_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_exports
    ADD CONSTRAINT drive_legal_exports_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_legal_exports drive_legal_exports_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_exports
    ADD CONSTRAINT drive_legal_exports_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_legal_holds drive_legal_holds_case_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_holds
    ADD CONSTRAINT drive_legal_holds_case_id_fkey FOREIGN KEY (case_id) REFERENCES public.drive_legal_cases(id) ON DELETE CASCADE;


--
-- Name: drive_legal_holds drive_legal_holds_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_holds
    ADD CONSTRAINT drive_legal_holds_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE RESTRICT;


--
-- Name: drive_legal_holds drive_legal_holds_released_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_holds
    ADD CONSTRAINT drive_legal_holds_released_by_user_id_fkey FOREIGN KEY (released_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_legal_holds drive_legal_holds_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_legal_holds
    ADD CONSTRAINT drive_legal_holds_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_marketplace_app_versions drive_marketplace_app_versions_app_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_app_versions
    ADD CONSTRAINT drive_marketplace_app_versions_app_id_fkey FOREIGN KEY (app_id) REFERENCES public.drive_marketplace_apps(id) ON DELETE CASCADE;


--
-- Name: drive_marketplace_installation_scopes drive_marketplace_installation_scopes_installation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_installation_scopes
    ADD CONSTRAINT drive_marketplace_installation_scopes_installation_id_fkey FOREIGN KEY (installation_id) REFERENCES public.drive_marketplace_installations(id) ON DELETE CASCADE;


--
-- Name: drive_marketplace_installations drive_marketplace_installations_app_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_installations
    ADD CONSTRAINT drive_marketplace_installations_app_id_fkey FOREIGN KEY (app_id) REFERENCES public.drive_marketplace_apps(id);


--
-- Name: drive_marketplace_installations drive_marketplace_installations_app_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_installations
    ADD CONSTRAINT drive_marketplace_installations_app_version_id_fkey FOREIGN KEY (app_version_id) REFERENCES public.drive_marketplace_app_versions(id);


--
-- Name: drive_marketplace_installations drive_marketplace_installations_approved_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_installations
    ADD CONSTRAINT drive_marketplace_installations_approved_by_user_id_fkey FOREIGN KEY (approved_by_user_id) REFERENCES public.users(id);


--
-- Name: drive_marketplace_installations drive_marketplace_installations_installed_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_installations
    ADD CONSTRAINT drive_marketplace_installations_installed_by_user_id_fkey FOREIGN KEY (installed_by_user_id) REFERENCES public.users(id);


--
-- Name: drive_marketplace_installations drive_marketplace_installations_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_marketplace_installations
    ADD CONSTRAINT drive_marketplace_installations_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_mobile_offline_operations drive_mobile_offline_operations_device_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_mobile_offline_operations
    ADD CONSTRAINT drive_mobile_offline_operations_device_id_fkey FOREIGN KEY (device_id) REFERENCES public.drive_sync_devices(id) ON DELETE CASCADE;


--
-- Name: drive_mobile_offline_operations drive_mobile_offline_operations_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_mobile_offline_operations
    ADD CONSTRAINT drive_mobile_offline_operations_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_object_key_versions drive_object_key_versions_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_object_key_versions
    ADD CONSTRAINT drive_object_key_versions_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_object_key_versions drive_object_key_versions_kms_key_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_object_key_versions
    ADD CONSTRAINT drive_object_key_versions_kms_key_id_fkey FOREIGN KEY (kms_key_id) REFERENCES public.drive_kms_keys(id) ON DELETE SET NULL;


--
-- Name: drive_object_key_versions drive_object_key_versions_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_object_key_versions
    ADD CONSTRAINT drive_object_key_versions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_ocr_pages drive_ocr_pages_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ocr_pages
    ADD CONSTRAINT drive_ocr_pages_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_ocr_pages drive_ocr_pages_ocr_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ocr_pages
    ADD CONSTRAINT drive_ocr_pages_ocr_run_id_fkey FOREIGN KEY (ocr_run_id) REFERENCES public.drive_ocr_runs(id) ON DELETE CASCADE;


--
-- Name: drive_ocr_pages drive_ocr_pages_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ocr_pages
    ADD CONSTRAINT drive_ocr_pages_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_ocr_runs drive_ocr_runs_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ocr_runs
    ADD CONSTRAINT drive_ocr_runs_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_ocr_runs drive_ocr_runs_outbox_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ocr_runs
    ADD CONSTRAINT drive_ocr_runs_outbox_event_id_fkey FOREIGN KEY (outbox_event_id) REFERENCES public.outbox_events(id) ON DELETE SET NULL;


--
-- Name: drive_ocr_runs drive_ocr_runs_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ocr_runs
    ADD CONSTRAINT drive_ocr_runs_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_ocr_runs drive_ocr_runs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_ocr_runs
    ADD CONSTRAINT drive_ocr_runs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_office_edit_sessions drive_office_edit_sessions_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_edit_sessions
    ADD CONSTRAINT drive_office_edit_sessions_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: drive_office_edit_sessions drive_office_edit_sessions_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_edit_sessions
    ADD CONSTRAINT drive_office_edit_sessions_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_office_edit_sessions drive_office_edit_sessions_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_edit_sessions
    ADD CONSTRAINT drive_office_edit_sessions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_office_provider_files drive_office_provider_files_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_provider_files
    ADD CONSTRAINT drive_office_provider_files_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_office_provider_files drive_office_provider_files_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_provider_files
    ADD CONSTRAINT drive_office_provider_files_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_office_webhook_events drive_office_webhook_events_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_webhook_events
    ADD CONSTRAINT drive_office_webhook_events_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE SET NULL;


--
-- Name: drive_office_webhook_events drive_office_webhook_events_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_office_webhook_events
    ADD CONSTRAINT drive_office_webhook_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_presence_sessions drive_presence_sessions_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_presence_sessions
    ADD CONSTRAINT drive_presence_sessions_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: drive_presence_sessions drive_presence_sessions_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_presence_sessions
    ADD CONSTRAINT drive_presence_sessions_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_presence_sessions drive_presence_sessions_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_presence_sessions
    ADD CONSTRAINT drive_presence_sessions_session_id_fkey FOREIGN KEY (session_id) REFERENCES public.drive_edit_sessions(id) ON DELETE CASCADE;


--
-- Name: drive_presence_sessions drive_presence_sessions_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_presence_sessions
    ADD CONSTRAINT drive_presence_sessions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_product_extraction_items drive_product_extraction_items_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_product_extraction_items
    ADD CONSTRAINT drive_product_extraction_items_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_product_extraction_items drive_product_extraction_items_ocr_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_product_extraction_items
    ADD CONSTRAINT drive_product_extraction_items_ocr_run_id_fkey FOREIGN KEY (ocr_run_id) REFERENCES public.drive_ocr_runs(id) ON DELETE CASCADE;


--
-- Name: drive_product_extraction_items drive_product_extraction_items_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_product_extraction_items
    ADD CONSTRAINT drive_product_extraction_items_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_region_migration_jobs drive_region_migration_jobs_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_region_migration_jobs
    ADD CONSTRAINT drive_region_migration_jobs_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_region_migration_jobs drive_region_migration_jobs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_region_migration_jobs
    ADD CONSTRAINT drive_region_migration_jobs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_region_migration_jobs drive_region_migration_jobs_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_region_migration_jobs
    ADD CONSTRAINT drive_region_migration_jobs_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.drive_workspaces(id) ON DELETE SET NULL;


--
-- Name: drive_region_placement_events drive_region_placement_events_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_region_placement_events
    ADD CONSTRAINT drive_region_placement_events_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE SET NULL;


--
-- Name: drive_region_placement_events drive_region_placement_events_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_region_placement_events
    ADD CONSTRAINT drive_region_placement_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_region_placement_events drive_region_placement_events_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_region_placement_events
    ADD CONSTRAINT drive_region_placement_events_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.drive_workspaces(id) ON DELETE SET NULL;


--
-- Name: drive_region_policies drive_region_policies_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_region_policies
    ADD CONSTRAINT drive_region_policies_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_remote_wipe_requests drive_remote_wipe_requests_device_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_remote_wipe_requests
    ADD CONSTRAINT drive_remote_wipe_requests_device_id_fkey FOREIGN KEY (device_id) REFERENCES public.drive_sync_devices(id) ON DELETE CASCADE;


--
-- Name: drive_remote_wipe_requests drive_remote_wipe_requests_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_remote_wipe_requests
    ADD CONSTRAINT drive_remote_wipe_requests_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: drive_remote_wipe_requests drive_remote_wipe_requests_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_remote_wipe_requests
    ADD CONSTRAINT drive_remote_wipe_requests_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


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
-- Name: drive_search_documents drive_search_documents_file_object_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_search_documents
    ADD CONSTRAINT drive_search_documents_file_object_id_fkey FOREIGN KEY (file_object_id) REFERENCES public.file_objects(id) ON DELETE CASCADE;


--
-- Name: drive_search_documents drive_search_documents_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_search_documents
    ADD CONSTRAINT drive_search_documents_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_search_documents drive_search_documents_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_search_documents
    ADD CONSTRAINT drive_search_documents_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.drive_workspaces(id) ON DELETE CASCADE;


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
-- Name: drive_starred_items drive_starred_items_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_starred_items
    ADD CONSTRAINT drive_starred_items_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_starred_items drive_starred_items_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_starred_items
    ADD CONSTRAINT drive_starred_items_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: drive_storage_gateways drive_storage_gateways_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_storage_gateways
    ADD CONSTRAINT drive_storage_gateways_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id);


--
-- Name: drive_storage_gateways drive_storage_gateways_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_storage_gateways
    ADD CONSTRAINT drive_storage_gateways_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_storage_gateways drive_storage_gateways_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_storage_gateways
    ADD CONSTRAINT drive_storage_gateways_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.drive_workspaces(id) ON DELETE SET NULL;


--
-- Name: drive_sync_conflicts drive_sync_conflicts_device_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_conflicts
    ADD CONSTRAINT drive_sync_conflicts_device_id_fkey FOREIGN KEY (device_id) REFERENCES public.drive_sync_devices(id) ON DELETE SET NULL;


--
-- Name: drive_sync_conflicts drive_sync_conflicts_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_conflicts
    ADD CONSTRAINT drive_sync_conflicts_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_sync_cursors drive_sync_cursors_device_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_cursors
    ADD CONSTRAINT drive_sync_cursors_device_id_fkey FOREIGN KEY (device_id) REFERENCES public.drive_sync_devices(id) ON DELETE CASCADE;


--
-- Name: drive_sync_cursors drive_sync_cursors_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_cursors
    ADD CONSTRAINT drive_sync_cursors_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_sync_devices drive_sync_devices_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_devices
    ADD CONSTRAINT drive_sync_devices_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_sync_devices drive_sync_devices_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_devices
    ADD CONSTRAINT drive_sync_devices_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: drive_sync_events drive_sync_events_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_events
    ADD CONSTRAINT drive_sync_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_sync_events drive_sync_events_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_sync_events
    ADD CONSTRAINT drive_sync_events_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.drive_workspaces(id) ON DELETE SET NULL;


--
-- Name: drive_workspace_region_overrides drive_workspace_region_overrides_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_workspace_region_overrides
    ADD CONSTRAINT drive_workspace_region_overrides_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: drive_workspace_region_overrides drive_workspace_region_overrides_workspace_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.drive_workspace_region_overrides
    ADD CONSTRAINT drive_workspace_region_overrides_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES public.drive_workspaces(id) ON DELETE CASCADE;


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
-- Name: file_objects file_objects_storage_gateway_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.file_objects
    ADD CONSTRAINT file_objects_storage_gateway_id_fkey FOREIGN KEY (storage_gateway_id) REFERENCES public.drive_storage_gateways(id) ON DELETE SET NULL;


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
-- Name: medallion_asset_edges medallion_asset_edges_source_asset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_asset_edges
    ADD CONSTRAINT medallion_asset_edges_source_asset_id_fkey FOREIGN KEY (source_asset_id) REFERENCES public.medallion_assets(id) ON DELETE CASCADE;


--
-- Name: medallion_asset_edges medallion_asset_edges_target_asset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_asset_edges
    ADD CONSTRAINT medallion_asset_edges_target_asset_id_fkey FOREIGN KEY (target_asset_id) REFERENCES public.medallion_assets(id) ON DELETE CASCADE;


--
-- Name: medallion_asset_edges medallion_asset_edges_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_asset_edges
    ADD CONSTRAINT medallion_asset_edges_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: medallion_assets medallion_assets_created_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_assets
    ADD CONSTRAINT medallion_assets_created_by_user_id_fkey FOREIGN KEY (created_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: medallion_assets medallion_assets_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_assets
    ADD CONSTRAINT medallion_assets_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: medallion_assets medallion_assets_updated_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_assets
    ADD CONSTRAINT medallion_assets_updated_by_user_id_fkey FOREIGN KEY (updated_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: medallion_pipeline_run_assets medallion_pipeline_run_assets_asset_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_pipeline_run_assets
    ADD CONSTRAINT medallion_pipeline_run_assets_asset_id_fkey FOREIGN KEY (asset_id) REFERENCES public.medallion_assets(id) ON DELETE CASCADE;


--
-- Name: medallion_pipeline_run_assets medallion_pipeline_run_assets_pipeline_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_pipeline_run_assets
    ADD CONSTRAINT medallion_pipeline_run_assets_pipeline_run_id_fkey FOREIGN KEY (pipeline_run_id) REFERENCES public.medallion_pipeline_runs(id) ON DELETE CASCADE;


--
-- Name: medallion_pipeline_run_assets medallion_pipeline_run_assets_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_pipeline_run_assets
    ADD CONSTRAINT medallion_pipeline_run_assets_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: medallion_pipeline_runs medallion_pipeline_runs_requested_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_pipeline_runs
    ADD CONSTRAINT medallion_pipeline_runs_requested_by_user_id_fkey FOREIGN KEY (requested_by_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: medallion_pipeline_runs medallion_pipeline_runs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.medallion_pipeline_runs
    ADD CONSTRAINT medallion_pipeline_runs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


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
-- Name: realtime_events realtime_events_recipient_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realtime_events
    ADD CONSTRAINT realtime_events_recipient_user_id_fkey FOREIGN KEY (recipient_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: realtime_events realtime_events_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.realtime_events
    ADD CONSTRAINT realtime_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


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
