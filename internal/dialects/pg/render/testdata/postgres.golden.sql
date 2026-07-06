CREATE ROLE app_admin WITH CREATEDB CREATEROLE LOGIN BYPASSRLS CONNECTION LIMIT 5 VALID UNTIL '2030-01-01 00:00:00+00' ADMIN app_reader;

CREATE ROLE app_reader WITH LOGIN CONNECTION LIMIT 20 IN ROLE pg_read_all_data;

CREATE SCHEMA billing;

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE billing.invoice_status AS ENUM ('draft', 'issued', 'paid', 'void');

CREATE TYPE billing.money AS (
    amount numeric(10, 2),
    currency char(3)
);

CREATE DOMAIN email AS text NOT NULL CHECK (value ~ '^[^@]+@[^@]+$');

CREATE SEQUENCE billing.order_number_seq INCREMENT 1 START 1000 CACHE 1;

CREATE FUNCTION billing.invoice_total_cents(invoice_id uuid)
RETURNS integer
LANGUAGE sql
STABLE
STRICT
PARALLEL SAFE
COST 10
SET search_path = billing, public
AS $$
SELECT COALESCE(SUM(amount_cents), 0)::integer FROM billing.invoice_lines WHERE invoice_id = $1
$$;
COMMENT ON FUNCTION billing.invoice_total_cents(uuid) IS 'Calculates invoice total cents.';

CREATE FUNCTION normalise_email(email text)
RETURNS text
LANGUAGE sql
IMMUTABLE
STRICT
AS $$
SELECT lower(trim(email))
$$;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    display_name text,
    last_login_ip inet,
    tags text[],
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_unique UNIQUE (email)
);

COMMENT ON TABLE users IS 'Application users.';

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
);

CREATE INDEX events_priority_created_at_idx ON events (priority, created_at);

CREATE SECURITY BARRIER VIEW billing.user_invoice_summary (user_id, invoice_count, total_cents) AS
    SELECT user_id, COUNT(*) AS invoice_count, SUM(amount_cents) AS total_cents FROM billing.invoices GROUP BY user_id
WITH CASCADED CHECK OPTION;

CREATE VIEW active_users AS
    SELECT id, email, display_name FROM users WHERE last_login_ip IS NOT NULL;
COMMENT ON VIEW active_users IS 'Users who have logged in at least once.';

CREATE MATERIALIZED VIEW cached_bookings AS
    SELECT owner, resource, COUNT(*) AS booking_count FROM bookings GROUP BY owner, resource
WITH (fillfactor = 90);
COMMENT ON MATERIALIZED VIEW cached_bookings IS 'Pre-aggregated booking counts.';

CREATE MATERIALIZED VIEW pending_events AS
    SELECT * FROM events WHERE created_at > now() - interval '1 day'
WITH NO DATA;
