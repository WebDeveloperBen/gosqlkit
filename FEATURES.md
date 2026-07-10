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
- `[~]` Migration diff workflow.
- `[x]` Review-first destructive-change handling.
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
- `[x]` CLI `migrate create` baseline migration from the current schema.
- `[x]` CLI `migrate create --empty` for runner-compatible manual migrations.

## Dialect Provider Boundary

- `[x]` Dialect-neutral package for provider registration and selection.
- `[x]` Providers expose canonical dialect name.
- `[x]` Providers expose dialect aliases, for example `postgres`, `pg`, and `postgresql`.
- `[x]` Providers expose coarse capabilities for command and workflow planning.
- `[x]` SQL generation routes through the selected provider.
- `[x]` Snapshot generation routes through the selected provider.
- `[x]` PostgreSQL schema envelope separated from the shared schema core.
- `[x]` PostgreSQL snapshots wrap shared table snapshots with PostgreSQL objects.
- `[x]` Shared table/index core explicitly separated from all dialect-specific options.
- `[x]` Move PostgreSQL-only index options out of the shared index model.
- `[ ]` First non-PostgreSQL provider proving the boundary.
- `[x]` Dialect capability checks in CLI commands where a command needs unsupported features.

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
- `[x]` Partitioned tables, partition children, and partition bounds.
- `[x]` Explicit grants and column-level table privileges.
- `[x]` Comments.
- `[x]` Tablespace assignment for tables, indexes, and materialized views.
- `[x]` Collations.
- `[x]` Raw SQL schema blocks (escape hatch: named blocks rendered verbatim
  before or after the structured schema, deterministically ordered, captured in
  the snapshot, excluded from drift, additive-only in migration planning, with
  optional `.Down()` reverse SQL so blocks can participate in down migrations).

## Extension Management

`gosqlkit` already declares extensions (`CREATE EXTENSION [WITH SCHEMA]
[VERSION] [CASCADE]`),
introspects them, and diffs add/remove. Neither Atlas (gated behind Atlas Pro)
nor Drizzle Kit (users run `CREATE EXTENSION` by hand) offers full open-source
extension management, so completing this is a differentiator. Build this out
after raw SQL schema blocks.

- `[x]` Declare extensions with `CREATE EXTENSION IF NOT EXISTS [WITH SCHEMA]
  [VERSION] [CASCADE]`.
- `[x]` Introspect installed extensions and diff add/remove.
- `[x]` Filter extension-owned objects out of drift.
- `[x]` Version pinning on create (`CREATE EXTENSION ... VERSION '<v>'`).
- `[x]` Version upgrades in the planner (`ALTER EXTENSION ... UPDATE TO '<v>'`)
  with manual-review and DDL-review risk flags.
- `[x]` `CASCADE` on create and drop.
- `[x]` Extension comments.
- `[x]` Introspect and diff extension version (`pg_extension.extversion`);
  version pin removals fail closed because PostgreSQL has no unpin operation.
- `[x]` Recognise extension-provided types (e.g. pgvector `vector`, PostGIS
  `geometry`) so their creation is attributed to the owning extension.

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
- `[x]` Column-level collation.
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
- `[x]` PostGIS `geometry`
- `[x]` pgvector `vector`
- `[x]` pgvector `halfvec`
- `[x]` pgvector `sparsevec`
- `[x]` pgvector `bit`
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
- `[x]` Named check helper ergonomics beyond raw expression strings.
- `[x]` Exclusion constraints.
- `[x]` Deferrable constraints.

## Indexes

- `[x]` Basic index.
- `[x]` Basic unique index.
- `[x]` Multi-column index by column names.
- `[x]` Named auto-generation helpers.
- `[x]` Index methods: `btree`, `hash`, `gist`, `spgist`, `gin`, `brin`.
- `[x]` Extension index methods: `hnsw`, `ivfflat`, and custom methods.
- `[x]` pgvector index operator class helpers for vector, halfvec, sparsevec,
  and bit distance families.
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
- `[x]` View and materialized-view definition builders for schema authoring.
- `[x]` View check options.
- `[x]` View security options.
- `[x]` Materialized view storage options.
- `[x]` Materialized view `WITH NO DATA`.
- `[x]` Refresh materialized view support (authored as an explicit `migrate
  refresh` migration step, not a structural snapshot field; refresh is a
  data-population operation kept out of the schema diff, matching Drizzle and
  Atlas).

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
- `[x]` Embedded target snapshots in generated migration metadata.
- `[ ]` Squashed/normalised representation for diffing.
- `[x]` Deterministic serialisation to JSON.
- `[~]` Diff input from current database introspection.
- `[~]` Diff input from generated desired snapshot.
- `[~]` Drift check from database to generated schema.
- `[x]` Rename annotations for tables.
- `[x]` Rename annotations for columns.
- `[x]` Rename annotations for indexes and constraints.
- `[x]` Destructive-change detection.
- `[x]` Destructive-change default failure mode.
- `[x]` Explicit override for destructive changes.

## Migration Engine Slices

The migration engine should ship in reviewable vertical slices. Each slice
must keep unsupported changes fail-closed rather than writing partial SQL.

### Slice 1: Authoring V0 Hardening

- `[x]` Baseline goose migration generation from the current schema.
- `[x]` Empty goose migration generation for manual SQL.
- `[~]` Conservative additive PostgreSQL diffs from embedded target snapshots.
- `[x]` Embedded target snapshot metadata in generated migrations.
- `[x]` Migration directory validation for filenames, metadata, goose
  annotations, target snapshot IDs, and adjacent lineage.
- `[x]` Fail-closed planner errors for unsupported table details not rendered
  by v0 create-table/add-column planning.
- `[x]` Fixture-style diff tests for every supported additive v0 operation.
- `[x]` Fixture-style error tests for every unsupported/destructive v0
  operation.

### Slice 2: Structured Planner IR

- `[x]` Planner changes include stable object kind and object key.
- `[~]` Planner changes include create, drop, rename, alter, and replace
  operations.
- `[x]` Planner changes include dependency metadata.
- `[x]` Planner changes include reversibility metadata.
- `[x]` Planner changes include risk flags.
- `[x]` Planner can emit a machine-readable plan summary without writing files.
- `[x]` Goose renderer consumes structured plan data rather than plain SQL
  slices.

### Slice 3: Safe Additive PostgreSQL Coverage

- `[x]` Add standalone indexes for new tables.
- `[x]` Add standalone indexes for existing tables.
- `[x]` Add table comments.
- `[x]` Add column comments.
- `[x]` Add table-level constraints to existing tables.
- `[x]` Add exclusion constraints.
- `[x]` Add RLS enable/force changes.
- `[x]` Add RLS policies.
- `[x]` Add roles.
- `[x]` Add sequences.
- `[x]` Add composite types.
- `[x]` Add domains.
- `[x]` Add functions.
- `[x]` Add triggers.
- `[x]` Add views.
- `[x]` Add materialized views.
- `[x]` Generate best-effort down SQL for simple create operations.
- `[x]` Per-change down generation. If a migration mixes reversible and
  irreversible changes, generated down SQL now reverses the changes that can be
  reversed and marks the rest with an explicit placeholder such as
  `-- no automatic down for: <object>`, so one irreversible change no longer
  strips the whole down section.

### Slice 4: Destructive-Change Guardrails

- `[x]` Detect table removals.
- `[x]` Detect column removals.
- `[x]` Detect enum value removals.
- `[x]` Detect constraint, index, policy, trigger, view, function, sequence,
  domain, and role removals.
- `[x]` Detect column type changes.
- `[x]` Detect column default changes.
- `[x]` Detect nullability changes.
- `[x]` Detect generated-column and identity changes.
- `[x]` Risk flags for destructive, data-loss, lock-heavy, non-transactional,
  requires-backfill, and manual-review changes.
- `[x]` Destructive changes fail by default with actionable diagnostics.
- `[x]` Explicit destructive-change override.

### Slice 5: Rename-Aware Diffing

- `[x]` Table rename planning from `previousName`.
- `[x]` Column rename planning from `previousName`.
- `[x]` Constraint rename planning from `previousName`.
- `[x]` Index rename planning from `previousName`.
- `[x]` Enum/type rename planning from `previousName`.
- `[x]` Sequence, view, function, materialized view, policy, role, schema
  rename planning.
- `[x]` Rename metadata validation against previous snapshot object keys.
- `[x]` Rename-plus-alter combinations fail closed until semantic planning
  supports them.
- `[x]` Trigger rename planning.
- `[x]` Extension rename metadata detection as a manual-review replacement
  (PostgreSQL does not support `ALTER EXTENSION ... RENAME TO`).

### Slice 6: Sandbox Replay and Drift Check

- `[x]` PostgreSQL database connection plumbing for tooling commands.
- `[x]` PostgreSQL introspection to snapshot-compatible model for namespaces,
  extensions, roles, enums, composite types, domains, sequences, functions,
  tables, columns, comments, RLS flags, table constraints, standalone indexes,
  policies, triggers, views, and materialized views.
- `[x]` Sandbox replay of committed migrations.
- `[x]` Replay result comparison against the latest embedded target snapshot.
- `[x]` `gosqlkit migrate check --sandbox-url ...`.
- `[x]` `gosqlkit drift check --url ...`.
- `[x]` Deterministic introspection output with drift projection normalising
  rename metadata, PostgreSQL defaults, extension-owned objects,
  non-persistent index authoring flags, identity-backed sequences, and
  authoring-only dependency hints.

### Slice 7: Auth, Apply, and Runner Expansion

- `[x]` Provider-neutral auth interface for tooling database connections.
- `[x]` Password and environment URL auth.
- `[x]` Custom token command auth.
- `[~]` Azure Entra token auth.
- `[x]` AWS IAM token auth.
- `[x]` GCP IAM token auth.
- `[x]` Credential and token redaction in diagnostics.
- `[x]` `gosqlkit migrate apply --url ...`.
- `[x]` `golang-migrate` renderer from the structured plan.
- `[x]` Machine-readable JSON command output.
- `[x]` Quiet command output mode.

## Database Connectivity and Auth

Database connectivity is only needed for future introspection, drift checks, and migration diff workflows. It must not become part of the runtime application database layer.

- `[x]` Connect to PostgreSQL using a standard connection string.
- `[x]` Connect using environment-provided connection string, for example `DATABASE_URL`.
- `[x]` Connect using password authentication.
- `[x]` Connect using SSL/TLS options required by managed PostgreSQL providers.
- `[x]` Connect using token-as-password authentication.
- `[~]` Support token providers through a refreshable token callback.
- `[~]` Support OAuth2 access-token providers.
- `[~]` Support cloud-specific signed database auth tokens.
- `[~]` Support Azure Database for PostgreSQL Microsoft Entra authentication.
- `[x]` Support Azure CLI token acquisition for local development.
- `[x]` Support Azure managed identity/service principal token acquisition for CI and hosted workloads.
- `[x]` Support AWS RDS and Aurora PostgreSQL IAM database authentication.
- `[x]` Support AWS SDK/credential-chain token generation.
- `[x]` Support AWS CLI token generation for local development.
- `[x]` Support Google Cloud SQL for PostgreSQL IAM database authentication.
- `[ ]` Support Google Cloud SQL connector based automatic IAM auth.
- `[x]` Support Google `gcloud` OAuth2 token acquisition for local development.
- `[x]` Support custom token command execution for other providers.
- `[ ]` Support custom token-provider plugins/interfaces for providers not built in.
- `[x]` Avoid storing OAuth2/Entra/IAM access tokens in generated files, snapshots, logs, or migration output.
- `[x]` Redact credentials and tokens in diagnostics.
- `[x]` Acquire short-lived tokens immediately before opening database connections.
- `[ ]` Refresh tokens for long-running introspection or diff operations.
- `[x]` Avoid assuming tokens are reusable across hosts, regions, users, or instances.
- `[ ]` Support provider-specific username formats.
- `[ ]` Support provider-specific TLS/SSL requirements.
- `[ ]` Support direct connections and proxy/connector-mediated connections.
- `[x]` Keep database auth plumbing isolated from the schema DSL and renderer.

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
- `[x]` `gosqlkit inspect` database introspection to snapshot JSON.
- `[x]` `gosqlkit inspect --url ...`.
- `[x]` `gosqlkit inspect` using password/database URL auth.
- `[ ]` `gosqlkit inspect --auth token`.
- `[ ]` `gosqlkit inspect --auth azure-entra`.
- `[ ]` `gosqlkit inspect --auth aws-iam`.
- `[ ]` `gosqlkit inspect --auth gcp-iam`.
- `[x]` `gosqlkit inspect --token-command ...`.
- `[x]` `gosqlkit snapshot` for deterministic snapshot output.
- `[x]` `gosqlkit snapshot --prev` for previous snapshot ID tracking.
- `[x]` `gosqlkit snapshot --out`.
- `[x]` `gosqlkit snapshot --check`.
- `[x]` `gosqlkit migrate create <name>` (additive, destructive, and rename
  changes, with per-change risk flags and best-effort down SQL).
- `[x]` `gosqlkit migrate create <name> --empty`.
- `[x]` `gosqlkit migrate create <name> --allow-destructive` to author a
  migration containing drops, RLS disables, or comment removals.
- `[x]` `gosqlkit migrate refresh <matview> [--concurrently]` to author an
  explicit, risk-flagged migration that repopulates a materialized view.
- `[x]` `gosqlkit migrate plan` to print the structured migration plan without
  writing any files (`--json` for machine-readable output).
- `[~]` `gosqlkit migrate check`.
- `[x]` `gosqlkit migrate check --sandbox-url ...`.
- `[x]` `gosqlkit migrate apply --url ...`.
- `[x]` `gosqlkit drift check --url ...`.
- `[x]` Object-level drift diagnostics for missing, extra, and changed schema
  objects in human and JSON output.
- `[x]` Styled terminal tables for human drift diagnostics using
  `charm.land/lipgloss/v2`.
- `[x]` Styled terminal tables for migration plan summaries.
- `[~]` Interactive ambiguity prompts for migration authoring when stdout/stdin
  are TTYs. Implemented for rename candidates; other manual-review ambiguity
  classes remain fail-closed.
- `[x]` Non-interactive fail-closed ambiguity diagnostics with exact schema
  annotations to accept rename intent.
- `[x]` Interactive rename-candidate selection for tables, columns,
  constraints, indexes, schema objects, views, triggers, policies, roles, and
  functions.
- `[x]` `--interactive` / `--no-interactive` controls for prompt behaviour.
- `[x]` Script-safe prompt bypass for CI, keeping `--json` and `--quiet`
  non-interactive.
- `[ ]` Migration creation support for token-authenticated source databases.
- `[ ]` Migration creation support for provider-specific auth on both source and target inputs.
- `[~]` Goose-compatible migration file output.
- `[x]` CI command for committed schema drift (`gosqlkit ci schema`).
- `[x]` CI command for database drift (`gosqlkit ci database`).
- `[x]` Machine-readable JSON output for commands.
- `[x]` Quiet output mode for scripts.
- `[ ]` Shell completion.

## Validation

- `[x]` Invalid identifier validation.
- `[x]` Duplicate table validation.
- `[x]` Duplicate column validation.
- `[x]` Duplicate index validation.
- `[x]` Duplicate check validation.
- `[x]` Foreign key action validation.
- `[x]` Strongly typed PostgreSQL option builders for foreign key actions, index methods, and view check options.
- `[x]` Unknown local constraint column validation.
- `[x]` Unknown index column validation.
- `[x]` Duplicate schema object validation across schemas.
- `[x]` Duplicate constraint validation scoped like PostgreSQL.
- `[x]` Unknown referenced table validation.
- `[x]` Unknown referenced column validation.
- `[x]` Invalid default expression validation where practical.
- `[x]` Invalid generated-column expression validation where practical.
- `[x]` Invalid RLS policy validation.
- `[x]` Validation diagnostics with actionable messages.

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
- `[x]` Grant rendering tests.
- `[x]` Diff fixture tests.
- `[x]` Destructive-change fixture tests.
- `[x]` Rename-aware diff tests.
- `[x]` Integration test against live PostgreSQL.
- `[x]` Testcontainers integration test against live PostgreSQL for generated schema apply, introspection, drift detection, and sandbox replay.
- `[x]` Testcontainers replay tests for non-baseline diff migrations.
- `[x]` Testcontainers replay tests for rename migrations.
- `[x]` Testcontainers replay tests for destructive-change guardrails and allowed destructive SQL.
- `[x]` Down SQL replay tests for reversible generated migrations.
- `[x]` Testcontainers replay test for a generated `migrate refresh` migration,
  verifying a `WITH NO DATA` materialized view is unqueryable until the refresh
  migration populates it.
- `[x]` Drift normalisation matrix tests for casts, default opclasses, index ordering/nulls, inline constraints, qualified references, and PostgreSQL expression rewrites.
- `[x]` Goose statement parser tests for `StatementBegin` / `StatementEnd`, tagged dollar quotes, quoted semicolons, comments, empty statements, and non-transactional statements.
- `[x]` Migration metadata tamper tests for broken JSON, missing target snapshots, stale IDs, and incoherent lineage.
- `[x]` `golang-migrate` renderer tests for split files, metadata placement, down-file omission, and structured-statement precedence.
- `[x]` `golang-migrate` migration scan tests proving `.down.sql` files are ignored for metadata lineage.
- `[x]` `golang-migrate` PostgreSQL replay and apply tests covering `.up.sql` execution, `schema_migrations` current-version state, dirty-state failure, and skip semantics.
- `[x]` Testcontainers replay/apply tests for generated `golang-migrate` baseline and diff migrations.
- `[x]` Dockerised external `golang-migrate` CLI compatibility test for generated PostgreSQL migrations, including split single-statement files that preserve `CREATE INDEX CONCURRENTLY`.
- `[x]` PostgreSQL version matrix for Testcontainers integration, at least PostgreSQL 15, 16, and 17 where CI time allows.
- `[x]` pgvector image Testcontainers integration covering extension create,
  vector type DDL, inserts, introspection, and drift comparison.
- `[x]` Extension-owned object filtering tests.
- `[x]` View and materialized-view drift projection tests documenting query-text normalisation and persisted metadata.
- `[x]` Drift diagnostics tests for top-level objects and table-nested objects.
- `[x]` Partitioned table and partition-child render, diff, drift, and introspection tests.
- `[x]` Grant render, diff, drift, and introspection coverage.
- `[x]` Tablespace render, diff, drift projection, and introspection coverage.
- `[x]` Collation and column-level collation render, diff, drift projection, and introspection coverage.
- `[x]` CLI table-rendering tests for drift diagnostics.
- `[x]` CLI table-rendering tests for migration plan diagnostics.
- `[x]` CLI flag validation tests for conflicting prompt-mode controls.
- `[x]` CLI prompt tests for interactive migration ambiguity decisions.
- `[x]` Non-TTY tests proving ambiguous migrations fail closed without prompts.
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
9. Add the cross-dialect migration IR and dialect planner boundary.
10. Add runner-compatible migration file generation and destructive-change guardrails.
11. Add styled CLI summaries and interactive ambiguity resolution for migration
    authoring.
12. Add raw SQL schema blocks as an escape hatch for unmodelled DDL. (Done.)
13. Complete extension management (version pinning, `ALTER EXTENSION ... UPDATE
    TO`, `CASCADE`, comments, version introspection/diff).
14. Expand into remaining advanced PostgreSQL features and the first
    non-PostgreSQL provider.

## Drizzle Reference Notes

The Drizzle source is useful as a requirements reference, not as an implementation blueprint. Important patterns to carry forward:

- PostgreSQL schema declarations include more than tables: enums, schemas, sequences, views, materialized views, roles, functions, triggers, policies, grants, and relations.
- The serializer collects exported objects into typed groups before producing a snapshot.
- The snapshot stores columns, indexes, foreign keys, composite primary keys, unique constraints, policies, check constraints, RLS state, views, sequences, roles, functions, triggers, and metadata.
- Index modelling needs to account for methods, expressions, operator classes, sort direction, null ordering, partial predicates, concurrent creation, and storage parameters.
- Defaults and generated expressions need structured handling, not just raw strings.
- Migration-quality diffing needs stable metadata and object identity, not only generated SQL text.

## Migration Planner Reference Notes

Atlas and Drizzle Kit are architecture references, not runtime dependencies.
Clone them into `/tmp` while working on migration planning:

```bash
rtk git clone --depth 1 https://github.com/ariga/atlas.git /tmp/gosqlkit-atlas-ref
rtk git clone --depth 1 https://github.com/drizzle-team/drizzle-orm.git /tmp/gosqlkit-drizzle-ref
```

Reference patterns to mirror in Go-native form:

- Atlas `sql/schema/migrate.go`: typed change IR.
- Atlas `sql/postgres/{diff,migrate}.go`: dialect-specific diff and SQL planning.
- Drizzle Kit `drizzle-kit/src/{jsonDiffer.js,jsonStatements.ts,sqlgenerator.ts}`:
  snapshot differ -> structured statements -> SQL generator.

Keep the implementation split as:

- `internal/migrate/plan` for shared interfaces and change metadata.
- `internal/dialects/<dialect>/plan` for dialect-specific diff/planning.
- `internal/migrate/{goose,golangmigrate}` for runner-specific file output.
