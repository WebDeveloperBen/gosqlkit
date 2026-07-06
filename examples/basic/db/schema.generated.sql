CREATE SCHEMA billing;

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE billing.invoice_status AS ENUM ('draft', 'issued', 'paid', 'void');

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    display_name text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_unique UNIQUE (email)
);

CREATE TABLE billing.invoices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    amount_cents integer NOT NULL,
    status billing.invoice_status NOT NULL DEFAULT 'draft',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT invoices_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE,
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
