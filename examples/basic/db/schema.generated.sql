CREATE ROLE app_reader WITH NOLOGIN;

CREATE ROLE app_writer WITH LOGIN CONNECTION LIMIT 20 IN ROLE app_reader;

CREATE SCHEMA billing;

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE billing.invoice_status AS ENUM ('draft', 'issued', 'paid', 'void');

CREATE TYPE billing.money AS (
    amount numeric(10, 2),
    currency char(3)
);

CREATE DOMAIN email AS text NOT NULL CHECK (value ~ '^[^@]+@[^@]+$');

CREATE SEQUENCE billing.order_number_seq INCREMENT 1 START 1000 CACHE 1;

CREATE FUNCTION normalise_email(email text)
RETURNS text
LANGUAGE sql
IMMUTABLE
STRICT
AS $$
SELECT lower(trim(email))
$$;

CREATE FUNCTION set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
VOLATILE
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END
$$;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    display_name text,
    last_login_ip inet,
    tags text[],
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_unique UNIQUE (email)
);

COMMENT ON TABLE users IS 'Application users.';

ALTER TABLE users ENABLE ROW LEVEL SECURITY;

CREATE TABLE billing.invoices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    amount_cents integer NOT NULL,
    status billing.invoice_status NOT NULL DEFAULT 'draft',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT invoices_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT invoices_amount_cents_positive CHECK (amount_cents > 0)
);

CREATE INDEX invoices_user_id_created_at_idx ON billing.invoices (user_id, created_at DESC NULLS LAST) WHERE status <> 'void';

CREATE TABLE billing.invoice_lines (
    invoice_id uuid NOT NULL,
    line_no integer NOT NULL,
    description text NOT NULL,
    amount_cents integer NOT NULL,
    CONSTRAINT invoice_lines_pkey PRIMARY KEY (invoice_id, line_no),
    CONSTRAINT invoice_lines_invoice_id_fkey FOREIGN KEY (invoice_id) REFERENCES billing.invoices (id) ON DELETE CASCADE,
    CONSTRAINT invoice_lines_amount_cents_positive CHECK (amount_cents > 0)
);

CREATE INDEX CONCURRENTLY invoice_lines_description_idx ON billing.invoice_lines USING btree (description text_ops) WITH (fillfactor = 90);

CREATE TABLE bookings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    resource text NOT NULL,
    owner text NOT NULL,
    during tstzrange NOT NULL,
    CONSTRAINT bookings_no_overlap EXCLUDE USING gist (during WITH &&)
);

CREATE TABLE events (
    id integer GENERATED ALWAYS AS IDENTITY (SEQUENCE NAME events_id_seq INCREMENT 1 CACHE 20),
    description text NOT NULL,
    search_vector text GENERATED ALWAYS AS (to_tsvector('english', description)) STORED,
    metadata jsonb DEFAULT '{"source":"api"}'::jsonb,
    payload bytea,
    duration interval day to second (6) NOT NULL,
    priority smallint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
)
PARTITION BY RANGE (priority);

CREATE INDEX events_priority_created_at_idx ON events (priority, created_at);

CREATE TABLE events_default PARTITION OF events DEFAULT;

CREATE TABLE events_priority_low PARTITION OF events FOR VALUES FROM (0) TO (10);

CREATE POLICY users_read_self ON users
FOR SELECT
TO app_reader
USING (id = current_setting('app.user_id')::uuid);

CREATE VIEW billing.user_invoice_summary (user_id, invoice_count, total_cents) AS
    SELECT user_id, COUNT(*) AS invoice_count, SUM(amount_cents) AS total_cents FROM billing.invoices GROUP BY user_id;

CREATE VIEW active_users AS
    SELECT id, email, display_name FROM users WHERE last_login_ip IS NOT NULL;
COMMENT ON VIEW active_users IS 'Users who have logged in at least once.';

CREATE MATERIALIZED VIEW cached_bookings AS
    SELECT owner, resource, COUNT(*) AS booking_count FROM bookings GROUP BY owner, resource;
COMMENT ON MATERIALIZED VIEW cached_bookings IS 'Pre-aggregated booking counts.';

CREATE TRIGGER users_set_updated_at
BEFORE UPDATE OF email, display_name ON users
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION set_updated_at();
COMMENT ON TRIGGER users_set_updated_at ON users IS 'Automatically updates updated_at timestamp on row updates.';
