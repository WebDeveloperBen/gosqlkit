# gosqlkit Feature Roadmap

This document tracks the product requirements for `gosqlkit` against the current implementation and the broader schema-management surface exposed by tools such as Drizzle.

Legend:

- `[x]` implemented
- `[~]` partially implemented
- `[ ]` not implemented
- `[later]` intentionally later scope
- `[no]` non-goal

## Product Boundary

- `[x]` Go-native schema DSL.
- `[x]` Dialect registry with PostgreSQL as the first provider.
- `[x]` Self-describing dialect provider interface with canonical names, aliases, and capabilities.
- `[x]` Deterministic PostgreSQL SQL generation.
- `[x]` No runtime ORM.
- `[x]` No query builder.
- `[x]` Preserve `sqlc` and `pgx` for runtime database access.
- `[~]` CLI generation workflow.
- `[ ]` Migration diff workflow.
- `[ ]` Review-first destructive-change handling.
- `[x]` Snapshot metadata for stable diffs and rename support.

## Current Vertical Slice

- `[x]` Define tables in Go.
- `[x]` Define columns in Go.
- `[x]` Register schema from Go package imports.
- `[x]` Render canonical SQL for the PostgreSQL dialect.
- `[x]` Stable table ordering that respects foreign-key dependencies.
- `[x]` Stable index ordering.
- `[x]` PostgreSQL schemas/namespaces.
- `[x]` PostgreSQL extensions.
- `[x]` PostgreSQL enums.
- `[x]` Deterministic schema snapshot JSON.
- `[x]` Golden-style SQL tests.
- `[x]` Example schema with two related tables.
- `[x]` Example schema with a composite-key child table.
- `[x]` Example `sqlc` config and query files.
- `[x]` CLI `generate`.
- `[x]` CLI `generate --out`.
- `[x]` CLI `generate --check`.
- `[x]` CLI `snapshot`.
- `[x]` CLI `snapshot --out`.
- `[x]` CLI `snapshot --check`.

## Dialect Provider Boundary

- `[x]` Dialect-neutral package for provider registration and selection.
- `[x]` Providers expose canonical dialect name.
- `[x]` Providers expose dialect aliases, for example `postgres`, `pg`, and `postgresql`.
- `[x]` Providers expose coarse capabilities for command and workflow planning.
- `[x]` SQL generation routes through the selected provider.
- `[x]` Snapshot generation routes through the selected provider.
- `[x]` PostgreSQL schema envelope separated from the shared schema core.
- `[x]` PostgreSQL snapshots wrap shared table snapshots with PostgreSQL objects.
- `[~]` Shared table/index core explicitly separated from all dialect-specific options.
- `[ ]` Move PostgreSQL-only index options out of the shared index model.
- `[ ]` First non-PostgreSQL provider proving the boundary.
- `[ ]` Dialect capability checks in CLI commands where a command needs unsupported features.

## Schema Object Model

- `[x]` Tables.
- `[x]` Columns.
- `[x]` PostgreSQL schemas/namespaces.
- `[x]` Extensions.
- `[x]` Enums.
- `[x]` Sequences.
- `[x]` Views.
- `[x]` Materialized views.
- `[x]` Functions.
- `[x]` Triggers.
- `[x]` Row-level security policies.
- `[x]` Roles.
- `[x]` Domains.
- `[x]` Composite types.
- `[ ]` Partitioned tables.
- `[x]` Comments.
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
- `[x]` Column default as raw SQL expression.
- `[x]` Safe default helpers for strings, numbers, booleans, JSON, arrays, and dates.
- `[x]` Generated stored columns.
- `[x]` Identity columns.
- `[x]` Array columns.
- `[x]` Column comments.
- `[ ]` Column-level collation.
- `[x]` Type schema qualification, for example enum types in non-public schemas.
- `[x]` Rename metadata.

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

- `[x]` `smallint`
- `[x]` `serial`
- `[x]` `smallserial`
- `[x]` `bigserial`
- `[x]` `real`
- `[x]` `double precision`
- `[x]` `char(n)`
- `[x]` `time`
- `[x]` `timetz`
- `[x]` `interval`
- `[x]` `json`
- `[x]` `bytea`
- `[x]` `inet`
- `[x]` `cidr`
- `[x]` `macaddr`
- `[x]` `macaddr8`
- `[x]` `point`
- `[x]` `line`
- `[ ]` PostGIS `geometry`
- `[ ]` pgvector `vector`
- `[ ]` pgvector `halfvec`
- `[ ]` pgvector `sparsevec`
- `[ ]` pgvector `bit`
- `[x]` Custom type escape hatch.

## Constraints

- `[x]` Inline primary key.
- `[x]` Table-level primary key.
- `[x]` Composite primary key.
- `[x]` Inline unique constraint.
- `[x]` Named unique constraint.
- `[x]` Composite unique constraint.
- `[x]` `NULLS NOT DISTINCT` unique constraints.
- `[x]` Inline single-column foreign key.
- `[x]` Named foreign key.
- `[x]` Composite foreign key.
- `[x]` `ON DELETE` action rendering.
- `[x]` `ON UPDATE` action rendering.
- `[x]` Basic table check constraints.
- `[ ]` Named check helper ergonomics beyond raw expression strings.
- `[x]` Exclusion constraints.
- `[x]` Deferrable constraints.

## Indexes

- `[x]` Basic index.
- `[x]` Basic unique index.
- `[x]` Multi-column index by column names.
- `[x]` Named auto-generation helpers.
- `[x]` Index methods: `btree`, `hash`, `gist`, `spgist`, `gin`, `brin`.
- `[x]` Extension index methods: `hnsw`, `ivfflat`, and custom methods.
- `[x]` Per-column sort direction.
- `[x]` Per-column `NULLS FIRST` / `NULLS LAST`.
- `[x]` Per-column operator class.
- `[x]` Expression indexes.
- `[x]` Partial indexes with `WHERE`.
- `[x]` `CREATE INDEX CONCURRENTLY`.
- `[x]` `ONLY` table indexes.
- `[x]` Index storage parameters with `WITH (...)`.

## RLS, Roles, and Policies

- `[x]` Enable row-level security on a table.
- `[x]` Force row-level security.
- `[x]` Create roles.
- `[x]` Policy `AS PERMISSIVE` / `AS RESTRICTIVE`.
- `[x]` Policy command: `ALL`, `SELECT`, `INSERT`, `UPDATE`, `DELETE`.
- `[x]` Policy target roles.
- `[x]` Policy `USING` expression.
- `[x]` Policy `WITH CHECK` expression.

## Views and Materialized Views

- `[x]` Views.
- `[x]` Existing view declarations for introspection/diff compatibility.
- `[x]` Materialized views.
- `[x]` View column metadata.
- `[x]` View definition SQL.
- `[x]` View check options.
- `[x]` View security options.
- `[x]` Materialized view storage options.
- `[x]` Materialized view `WITH NO DATA`.
- `[ ]` Refresh materialized view support.

## Sequences

- `[x]` Named sequences.
- `[x]` Schema-qualified sequences.
- `[x]` Increment.
- `[x]` Min/max values.
- `[x]` Start value.
- `[x]` Cache.
- `[x]` Cycle.
- `[x]` Sequence ownership.
- `[x]` Identity-column generated sequences.

## Serialisation and Diff Readiness

Drizzle's serializer models schema as a structured snapshot before diffing. `gosqlkit` should adopt the same general idea, but Go-native and SQL-focused.

- `[x]` Internal snapshot format separate from rendered SQL.
- `[x]` Snapshot version.
- `[x]` Dialect marker.
- `[x]` Stable snapshot IDs.
- `[x]` Previous snapshot ID.
- `[x]` Table metadata map.
- `[x]` Column metadata map.
- `[x]` Schema metadata map.
- `[x]` Stable object keys for schema-qualified names.
- `[ ]` Squashed/normalised representation for diffing.
- `[x]` Deterministic serialisation to JSON.
- `[ ]` Diff input from current database introspection.
- `[ ]` Diff input from generated desired snapshot.
- `[ ]` Drift check from database to generated schema.
- `[x]` Rename annotations for tables.
- `[x]` Rename annotations for columns.
- `[x]` Rename annotations for indexes and constraints.
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
- `gosqlkit` should model this as a token provider, not as a stored password.

AWS-specific requirement:

- Amazon RDS and Aurora PostgreSQL IAM database authentication uses an authentication token instead of a password.
- The token is generated with AWS Signature Version 4 and is short-lived.
- Token generation depends on host, port, region, and database username, so `gosqlkit` must not treat it as a generic static secret.
- The implementation should support the AWS SDK credential chain and CLI-compatible local development.
- The implementation must preserve TLS/SSL configuration because IAM database authentication is intended to be used with encrypted connections.

Google Cloud-specific requirement:

- Cloud SQL for PostgreSQL IAM database authentication uses temporary OAuth2 access tokens.
- Manual IAM auth passes the access token as the PostgreSQL password.
- Automatic IAM auth is mediated by the Cloud SQL Auth Proxy or Cloud SQL language connectors.
- `gosqlkit` should support direct manual-token connections and connector/proxy-mediated connections.
- Google IAM auth requires SSL for manual database authentication.

Other provider requirement:

- Providers such as Neon, Supabase, Heroku, Crunchy Bridge, Aiven, DigitalOcean, and self-hosted PostgreSQL may use password auth, SSL client certificates, external secret managers, proxies, or custom token flows.
- `gosqlkit` should expose a generic provider-neutral auth interface so these can be supported without changing the schema DSL or renderer.

## CLI and Workflow

- `[x]` `gosqlkit generate`.
- `[x]` `gosqlkit generate --out`.
- `[x]` `gosqlkit generate --check`.
- `[x]` `gosqlkit version`.
- `[ ]` `gosqlkit inspect` or equivalent database introspection.
- `[ ]` `gosqlkit inspect --url ...`.
- `[ ]` `gosqlkit inspect --auth password`.
- `[ ]` `gosqlkit inspect --auth token`.
- `[ ]` `gosqlkit inspect --auth azure-entra`.
- `[ ]` `gosqlkit inspect --auth aws-iam`.
- `[ ]` `gosqlkit inspect --auth gcp-iam`.
- `[ ]` `gosqlkit inspect --auth custom-token-command`.
- `[x]` `gosqlkit snapshot` for deterministic snapshot output.
- `[x]` `gosqlkit snapshot --prev` for previous snapshot ID tracking.
- `[x]` `gosqlkit snapshot --out`.
- `[x]` `gosqlkit snapshot --check`.
- `[ ]` `gosqlkit diff --from ... --to ...`.
- `[ ]` `gosqlkit diff` support for token-authenticated source databases.
- `[ ]` `gosqlkit diff` support for provider-specific auth on both source and target inputs.
- `[ ]` `gosqlkit migrate create <name>`.
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
- `[x]` Unknown local constraint column validation.
- `[x]` Unknown index column validation.
- `[x]` Duplicate schema object validation across schemas.
- `[~]` Duplicate constraint validation scoped like PostgreSQL.
- `[x]` Unknown referenced table validation.
- `[x]` Unknown referenced column validation.
- `[ ]` Invalid default expression validation where practical.
- `[ ]` Invalid generated-column expression validation where practical.
- `[x]` Invalid RLS policy validation.
- `[ ]` Validation diagnostics with actionable messages.

## Testing Requirements

- `[x]` Renderer golden tests.
- `[x]` CLI generation test.
- `[x]` `sqlc` compatibility smoke validation.
- `[x]` Snapshot golden tests.
- `[x]` Schema-qualified object tests.
- `[x]` Constraint rendering tests.
- `[x]` Advanced index rendering tests.
- `[x]` Default literal rendering tests.
- `[x]` Array type rendering tests.
- `[x]` Enum rendering tests.
- `[x]` Sequence rendering tests.
- `[x]` View rendering tests.
- `[x]` Role rendering tests.
- `[x]` Function rendering tests.
- `[x]` Trigger rendering tests.
- `[x]` RLS policy rendering tests.
- `[ ]` Diff fixture tests.
- `[ ]` Destructive-change fixture tests.
- `[ ]` Integration test against live PostgreSQL.
- `[ ]` Testcontainers integration test against live PostgreSQL.
- `[ ]` Testcontainers integration test per supported dialect.

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
11. Expand into partitioning, grants, and other advanced PostgreSQL features.

## Drizzle Reference Notes

The Drizzle source is useful as a requirements reference, not as an implementation blueprint. Important patterns to carry forward:

- PostgreSQL schema declarations include more than tables: enums, schemas, sequences, views, materialized views, roles, functions, triggers, policies, and relations.
- The serializer collects exported objects into typed groups before producing a snapshot.
- The snapshot stores columns, indexes, foreign keys, composite primary keys, unique constraints, policies, check constraints, RLS state, views, sequences, roles, functions, triggers, and metadata.
- Index modelling needs to account for methods, expressions, operator classes, sort direction, null ordering, partial predicates, concurrent creation, and storage parameters.
- Defaults and generated expressions need structured handling, not just raw strings.
- Migration-quality diffing needs stable metadata and object identity, not only generated SQL text.
