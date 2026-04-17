CREATE TABLE app_users (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id uuid NOT NULL DEFAULT uuidv7() UNIQUE,
    zitadel_subject text NOT NULL UNIQUE,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

