# pgkit Feature Roadmap

This document tracks the product requirements for `pgkit` against the current implementation and the broader PostgreSQL schema-management surface exposed by Drizzle's PostgreSQL core and serializer.

Legend:

- `[x]` implemented
- `[~]` partially implemented
- `[ ]` not implemented
- `[later]` intentionally later scope
- `[no]` non-goal

## Product Boundary

- `[x]` Go-native PostgreSQL schema DSL.
- `[x]` Deterministic PostgreSQL SQL generation.
- `[x]` No runtime ORM.
- `[x]` No query builder.
- `[x]` Preserve `sqlc` and `pgx` for runtime database access.
- `[~]` CLI generation workflow.
- `[ ]` Migration diff workflow.
- `[ ]` Review-first destructive-change handling.
- `[ ]` Snapshot metadata for stable diffs and rename support.

## Current Vertical Slice

- `[x]` Define tables in Go.
- `[x]` Define columns in Go.
- `[x]` Register schema from Go package imports.
- `[x]` Render canonical PostgreSQL SQL.
- `[x]` Stable table ordering.
- `[x]` Stable index ordering.
- `[x]` Golden-style SQL tests.
- `[x]` Example schema with two related tables.
- `[x]` Example `sqlc` config and query files.
- `[x]` CLI `generate`.
- `[x]` CLI `generate --out`.
- `[x]` CLI `generate --check`.

## Schema Object Model

- `[x]` Tables.
- `[x]` Columns.
- `[ ]` PostgreSQL schemas/namespaces.
- `[ ]` Extensions.
- `[ ]` Enums.
- `[ ]` Sequences.
- `[ ]` Views.
- `[ ]` Materialized views.
- `[ ]` Functions.
- `[ ]` Triggers.
- `[ ]` Row-level security policies.
- `[ ]` Roles.
- `[ ]` Domains.
- `[ ]` Composite types.
- `[ ]` Partitioned tables.
- `[ ]` Comments.
- `[ ]` Tablespaces.
- `[ ]` Collations.
- `[ ]` Raw SQL schema blocks.

## Columns

- `[x]` Column name.
- `[x]` Column type.
- `[x]` `NOT NULL`.
- `[x]` Nullable by default.
- `[x]` Column primary key.
- `[x]` Column unique.
- `[~]` Column default as raw SQL expression.
- `[ ]` Safe default helpers for strings, numbers, booleans, JSON, arrays, and dates.
- `[ ]` Generated stored columns.
- `[ ]` Identity columns.
- `[ ]` Array columns.
- `[ ]` Column comments.
- `[ ]` Column-level collation.
- `[ ]` Type schema qualification, for example enum types in non-public schemas.
- `[ ]` Rename metadata.

## PostgreSQL Types

Initial type support:

- `[x]` `uuid`
- `[x]` `text`
- `[x]` `varchar(n)`
- `[x]` `integer`
- `[x]` `bigint`
- `[x]` `boolean`
- `[x]` `numeric`
- `[x]` `numeric(precision, scale)`
- `[x]` `date`
- `[x]` `timestamp`
- `[x]` `timestamptz`
- `[x]` `jsonb`

Missing common PostgreSQL types:

- `[ ]` `smallint`
- `[ ]` `serial`
- `[ ]` `smallserial`
- `[ ]` `bigserial`
- `[ ]` `real`
- `[ ]` `double precision`
- `[ ]` `char(n)`
- `[ ]` `time`
- `[ ]` `timetz`
- `[ ]` `interval`
- `[ ]` `json`
- `[ ]` `bytea`
- `[ ]` `inet`
- `[ ]` `cidr`
- `[ ]` `macaddr`
- `[ ]` `macaddr8`
- `[ ]` `point`
- `[ ]` `line`
- `[ ]` PostGIS `geometry`
- `[ ]` pgvector `vector`
- `[ ]` pgvector `halfvec`
- `[ ]` pgvector `sparsevec`
- `[ ]` pgvector `bit`
- `[ ]` Custom type escape hatch.

## Constraints

- `[x]` Inline primary key.
- `[ ]` Table-level primary key.
- `[ ]` Composite primary key.
- `[x]` Inline unique constraint.
- `[ ]` Named unique constraint.
- `[ ]` Composite unique constraint.
- `[ ]` `NULLS NOT DISTINCT` unique constraints.
- `[x]` Inline single-column foreign key.
- `[ ]` Named foreign key.
- `[ ]` Composite foreign key.
- `[~]` `ON DELETE` action rendering.
- `[~]` `ON UPDATE` action rendering.
- `[x]` Basic table check constraints.
- `[ ]` Named check helper ergonomics beyond raw expression strings.
- `[ ]` Exclusion constraints.
- `[ ]` Deferrable constraints.

## Indexes

- `[x]` Basic index.
- `[x]` Basic unique index.
- `[x]` Multi-column index by column names.
- `[ ]` Named auto-generation helpers.
- `[ ]` Index methods: `btree`, `hash`, `gist`, `spgist`, `gin`, `brin`.
- `[ ]` Extension index methods: `hnsw`, `ivfflat`, and custom methods.
- `[ ]` Per-column sort direction.
- `[ ]` Per-column `NULLS FIRST` / `NULLS LAST`.
- `[ ]` Per-column operator class.
- `[ ]` Expression indexes.
- `[ ]` Partial indexes with `WHERE`.
- `[ ]` `CREATE INDEX CONCURRENTLY`.
- `[ ]` `ONLY` table indexes.
- `[ ]` Index storage parameters with `WITH (...)`.

## RLS, Roles, and Policies

- `[ ]` Enable row-level security on a table.
- `[ ]` Force row-level security.
- `[ ]` Create roles.
- `[ ]` Policy `AS PERMISSIVE` / `AS RESTRICTIVE`.
- `[ ]` Policy command: `ALL`, `SELECT`, `INSERT`, `UPDATE`, `DELETE`.
- `[ ]` Policy target roles.
- `[ ]` Policy `USING` expression.
- `[ ]` Policy `WITH CHECK` expression.

## Views and Materialized Views

- `[ ]` Plain views.
- `[ ]` Existing view declarations for introspection/diff compatibility.
- `[ ]` Materialized views.
- `[ ]` View column metadata.
- `[ ]` View definition SQL.
- `[ ]` View check options.
- `[ ]` View security options.
- `[ ]` Materialized view storage options.
- `[ ]` Materialized view `WITH NO DATA`.
- `[ ]` Refresh materialized view support.

## Sequences

- `[ ]` Named sequences.
- `[ ]` Schema-qualified sequences.
- `[ ]` Increment.
- `[ ]` Min/max values.
- `[ ]` Start value.
- `[ ]` Cache.
- `[ ]` Cycle.
- `[ ]` Sequence ownership.
- `[ ]` Identity-column generated sequences.

## Serialisation and Diff Readiness

Drizzle's serializer models schema as a structured snapshot before diffing. `pgkit` should adopt the same general idea, but Go-native and SQL-focused.

- `[ ]` Internal snapshot format separate from rendered SQL.
- `[ ]` Snapshot version.
- `[ ]` Dialect marker.
- `[ ]` Stable snapshot IDs.
- `[ ]` Previous snapshot ID.
- `[ ]` Table metadata map.
- `[ ]` Column metadata map.
- `[ ]` Schema metadata map.
- `[ ]` Stable object keys for schema-qualified names.
- `[ ]` Squashed/normalised representation for diffing.
- `[ ]` Deterministic serialisation to JSON.
- `[ ]` Diff input from current database introspection.
- `[ ]` Diff input from generated desired snapshot.
- `[ ]` Drift check from database to generated schema.
- `[ ]` Rename annotations for tables.
- `[ ]` Rename annotations for columns.
- `[ ]` Rename annotations for indexes and constraints.
- `[ ]` Destructive-change detection.
- `[ ]` Destructive-change default failure mode.
- `[ ]` Explicit override for destructive changes.

## Database Connectivity and Auth

Database connectivity is only needed for future introspection, drift checks, and migration diff workflows. It must not become part of the runtime application database layer.

- `[ ]` Connect to PostgreSQL using a standard connection string.
- `[ ]` Connect using environment-provided connection string, for example `DATABASE_URL`.
- `[ ]` Connect using password authentication.
- `[ ]` Connect using SSL/TLS options required by managed PostgreSQL providers.
- `[ ]` Connect using token-as-password authentication.
- `[ ]` Support token providers through a refreshable token callback.
- `[ ]` Support OAuth2 access-token providers.
- `[ ]` Support cloud-specific signed database auth tokens.
- `[ ]` Support Azure Database for PostgreSQL Microsoft Entra authentication.
- `[ ]` Support Azure CLI token acquisition for local development.
- `[ ]` Support Azure managed identity/service principal token acquisition for CI and hosted workloads.
- `[ ]` Support AWS RDS and Aurora PostgreSQL IAM database authentication.
- `[ ]` Support AWS SDK/credential-chain token generation.
- `[ ]` Support AWS CLI token generation for local development.
- `[ ]` Support Google Cloud SQL for PostgreSQL IAM database authentication.
- `[ ]` Support Google Cloud SQL connector based automatic IAM auth.
- `[ ]` Support Google `gcloud` OAuth2 token acquisition for local development.
- `[ ]` Support custom token command execution for other providers.
- `[ ]` Support custom token-provider plugins/interfaces for providers not built in.
- `[ ]` Avoid storing OAuth2/Entra/IAM access tokens in generated files, snapshots, logs, or migration output.
- `[ ]` Redact credentials and tokens in diagnostics.
- `[ ]` Acquire short-lived tokens immediately before opening database connections.
- `[ ]` Refresh tokens for long-running introspection or diff operations.
- `[ ]` Avoid assuming tokens are reusable across hosts, regions, users, or instances.
- `[ ]` Support provider-specific username formats.
- `[ ]` Support provider-specific TLS/SSL requirements.
- `[ ]` Support direct connections and proxy/connector-mediated connections.
- `[ ]` Keep database auth plumbing isolated from the schema DSL and renderer.

Azure-specific requirement:

- Azure Database for PostgreSQL with Microsoft Entra authentication accepts a Microsoft Entra access token as the PostgreSQL password.
- The PostgreSQL username is the mapped Entra user, service principal, managed identity, or group name.
- The token should target Azure Database for PostgreSQL, for example Azure CLI's `--resource-type oss-rdbms` flow.
- `pgkit` should model this as a token provider, not as a stored password.

AWS-specific requirement:

- Amazon RDS and Aurora PostgreSQL IAM database authentication uses an authentication token instead of a password.
- The token is generated with AWS Signature Version 4 and is short-lived.
- Token generation depends on host, port, region, and database username, so `pgkit` must not treat it as a generic static secret.
- The implementation should support the AWS SDK credential chain and CLI-compatible local development.
- The implementation must preserve TLS/SSL configuration because IAM database authentication is intended to be used with encrypted connections.

Google Cloud-specific requirement:

- Cloud SQL for PostgreSQL IAM database authentication uses temporary OAuth2 access tokens.
- Manual IAM auth passes the access token as the PostgreSQL password.
- Automatic IAM auth is mediated by the Cloud SQL Auth Proxy or Cloud SQL language connectors.
- `pgkit` should support direct manual-token connections and connector/proxy-mediated connections.
- Google IAM auth requires SSL for manual database authentication.

Other provider requirement:

- Providers such as Neon, Supabase, Heroku, Crunchy Bridge, Aiven, DigitalOcean, and self-hosted PostgreSQL may use password auth, SSL client certificates, external secret managers, proxies, or custom token flows.
- `pgkit` should expose a generic provider-neutral auth interface so these can be supported without changing the schema DSL or renderer.

## CLI and Workflow

- `[x]` `pgkit generate`.
- `[x]` `pgkit generate --out`.
- `[x]` `pgkit generate --check`.
- `[ ]` `pgkit version`.
- `[ ]` `pgkit inspect` or equivalent database introspection.
- `[ ]` `pgkit inspect --url ...`.
- `[ ]` `pgkit inspect --auth password`.
- `[ ]` `pgkit inspect --auth token`.
- `[ ]` `pgkit inspect --auth azure-entra`.
- `[ ]` `pgkit inspect --auth aws-iam`.
- `[ ]` `pgkit inspect --auth gcp-iam`.
- `[ ]` `pgkit inspect --auth custom-token-command`.
- `[ ]` `pgkit snapshot` for deterministic snapshot output.
- `[ ]` `pgkit diff --from ... --to ...`.
- `[ ]` `pgkit diff` support for token-authenticated source databases.
- `[ ]` `pgkit diff` support for provider-specific auth on both source and target inputs.
- `[ ]` `pgkit migrate create <name>`.
- `[ ]` Goose-compatible migration file output.
- `[ ]` CI command for committed schema drift.
- `[ ]` CI command for database drift.
- `[ ]` Machine-readable JSON output for commands.
- `[ ]` Quiet output mode for scripts.
- `[ ]` Shell completion.

## Validation

- `[x]` Invalid identifier validation.
- `[x]` Duplicate table validation.
- `[x]` Duplicate column validation.
- `[x]` Duplicate index validation.
- `[x]` Duplicate check validation.
- `[x]` Foreign key action validation.
- `[ ]` Duplicate schema object validation across schemas.
- `[ ]` Duplicate constraint validation scoped like PostgreSQL.
- `[ ]` Unknown referenced table validation.
- `[ ]` Unknown referenced column validation.
- `[ ]` Unknown index column validation.
- `[ ]` Invalid default expression validation where practical.
- `[ ]` Invalid generated-column expression validation where practical.
- `[ ]` Invalid RLS policy validation.
- `[ ]` Validation diagnostics with actionable messages.

## Testing Requirements

- `[x]` Renderer golden tests.
- `[x]` CLI generation test.
- `[x]` `sqlc` compatibility smoke validation.
- `[ ]` Snapshot golden tests.
- `[ ]` Schema-qualified object tests.
- `[ ]` Constraint rendering tests.
- `[ ]` Advanced index rendering tests.
- `[ ]` Default literal rendering tests.
- `[ ]` Array type rendering tests.
- `[ ]` Enum rendering tests.
- `[ ]` Sequence rendering tests.
- `[ ]` View rendering tests.
- `[ ]` RLS policy rendering tests.
- `[ ]` Diff fixture tests.
- `[ ]` Destructive-change fixture tests.
- `[ ]` Integration test against live PostgreSQL.

## Recommended Build Order

1. Stabilise the CLI layer before adding more commands.
2. Add table-level constraints: composite primary keys, named unique constraints, named foreign keys.
3. Add richer index support: method, where, order, nulls, opclass, concurrently.
4. Add schemas/namespaces and schema-qualified rendering.
5. Add enums and extensions because they are common prerequisites for real applications.
6. Add snapshot JSON output before implementing migration diffing.
7. Add database connection plumbing with password and refreshable token auth.
8. Add database introspection and drift checks.
9. Integrate `pg-schema-diff` or another PostgreSQL diff engine.
10. Add migration file generation and destructive-change guardrails.
11. Expand into views, sequences, RLS, roles, comments, and advanced PostgreSQL features.

## Drizzle Reference Notes

The Drizzle source is useful as a requirements reference, not as an implementation blueprint. Important patterns to carry forward:

- PostgreSQL schema declarations include more than tables: enums, schemas, sequences, views, materialized views, roles, policies, and relations.
- The serializer collects exported objects into typed groups before producing a snapshot.
- The snapshot stores columns, indexes, foreign keys, composite primary keys, unique constraints, policies, check constraints, RLS state, views, sequences, roles, and metadata.
- Index modelling needs to account for methods, expressions, operator classes, sort direction, null ordering, partial predicates, concurrent creation, and storage parameters.
- Defaults and generated expressions need structured handling, not just raw strings.
- Migration-quality diffing needs stable metadata and object identity, not only generated SQL text.
