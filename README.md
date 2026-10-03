# gosqlkit

`gosqlkit` is a Go-native schema declaration and migration workflow for
PostgreSQL and SQLite. Define database schemas in Go; generate deterministic,
reviewable SQL, structured snapshots, and runner-compatible migrations.

It is intentionally **not an ORM** or **query builder**. `sqlc` owns query code
generation, and applications choose their runtime database driver.

```text
gosqlkit        schema declarations -> snapshots -> canonical SQL
gosqlkit        migration plan/create/check/apply for supported dialect tools
sqlc            SQL queries -> type-safe Go query code
goose/golang-migrate runner-compatible migration files and version state
application     runtime database access through its chosen driver
```

Migration generation uses a cross-dialect planner with runner-compatible SQL
output. PostgreSQL database tooling includes inspect, drift, sandbox replay,
and apply; SQLite currently supports schema generation and snapshot-based
migration planning. See [MIGRATIONS.md](MIGRATIONS.md) for design decisions.

PostgreSQL is the first dialect; SQLite is also implemented as a distinct
dialect with its own supported schema semantics. The core remains dialect
neutral so additional engines can be added without emulating PostgreSQL.

## Example

```go
package schema

import "github.com/webdeveloperben/gosqlkit/pg"

var Users = pg.Table("users",
	pg.UUID("id").
		PrimaryKey().
		Default("gen_random_uuid()"),
	pg.Text("email").
		NotNull().
		Unique(),
	pg.Text("display_name"),
	pg.TimestampTZ("created_at").
		NotNull().
		Default("now()"),
)
```

Configure `gosqlkit.yaml` in your project:

```yaml
version: "1"
dialect: postgresql
schema: "schema"
out:
  sql: "db/schema.generated.sql"
  snapshot: "db/schema.snapshot.json"
migrations:
  dir: "db/migrations"
  runner: goose
```

The default migration runner is `goose`; set `runner: golang-migrate` to emit
paired `.up.sql` / `.down.sql` files instead.

Generate SQL and snapshots (reads `gosqlkit.yaml` — no args needed):

```bash
gosqlkit generate
gosqlkit snapshot
```

Check committed output is current (CI gate):

```bash
gosqlkit generate --check
gosqlkit snapshot --check
gosqlkit ci schema
```

Run the Docker-backed PostgreSQL integration suite when you want the full
render/apply/introspect/drift/replay path checked against a real engine:

```bash
task integration
task integration:postgres:matrix
task integration:postgres:pgvector
```

Create a baseline runner-compatible migration from the current schema when
the migration directory is empty:

```bash
gosqlkit migrate create init_schema
gosqlkit migrate check
gosqlkit migrate check --sandbox-url "$DATABASE_URL"
gosqlkit inspect --url "$DATABASE_URL"
gosqlkit inspect --url "$DATABASE_URL" --drift-projection
gosqlkit drift check --url "$DATABASE_URL"
gosqlkit ci database --url "$DATABASE_URL"
gosqlkit migrate apply --url "$DATABASE_URL"
gosqlkit migrate apply --azure-cli-token --url "$AZURE_POSTGRES_URL"
gosqlkit migrate apply --azure-default-credential --url "$AZURE_POSTGRES_URL"
gosqlkit migrate apply --aws-iam-token --aws-region us-east-1 --url "$AWS_RDS_POSTGRES_URL"
gosqlkit migrate apply --aws-cli-token --aws-region us-east-1 --aws-profile dev --url "$AWS_RDS_POSTGRES_URL"
gosqlkit migrate apply --gcloud-token --gcloud-instance app-prod --url "$CLOUD_SQL_POSTGRES_URL"
gosqlkit migrate apply --gcloud-adc-token --gcloud-instance app-prod --url "$CLOUD_SQL_POSTGRES_URL"
gosqlkit drift check --auth gcp-iam --cloud-sql-connector --gcloud-instance project:region:instance --url "$CLOUD_SQL_POSTGRES_URL"
gosqlkit migrate apply --token-command "az account get-access-token --resource-type oss-rdbms --query accessToken -o tsv"
```

`inspect` reads `gosqlkit.yaml` for the configured dialect, then emits the
database snapshot shape for that dialect.

Create an empty migration for manual SQL:

```bash
gosqlkit migrate create add_users_table --empty
```

Preview the next migration plan without writing a file:

```bash
gosqlkit migrate plan
gosqlkit migrate plan --json   # machine-readable
```

Human plan output is rendered as terminal tables; `--json` remains the stable
automation format. `migrate create` also accepts `--interactive` and
`--no-interactive` prompt-mode controls. Rename candidates are presented as a
numbered table when prompts are enabled; non-interactive runs fail closed with
the same `previousName` guidance.

Author a migration that drops objects (disabled by default to keep destructive
changes reviewable):

```bash
gosqlkit migrate create drop_legacy_columns --allow-destructive
```

Author a migration that repopulates a materialized view. Refreshing is a
data-population operation rather than structural schema (matching Drizzle and
Atlas, which keep `REFRESH` out of the schema diff), so it is authored as an
explicit, reviewable migration step instead of being inferred from the schema:

```bash
gosqlkit migrate refresh cached_bookings
gosqlkit migrate refresh cached_bookings --concurrently   # allows concurrent reads; needs a unique index
```

Schema can be a single path or a list of paths:

```yaml
schema:
  - "schema/users"
  - "schema/billing"
  - "schema/events"
```

Then feed the generated SQL to `sqlc`:

```bash
sqlc generate
```

## Current Features

Behavior contracts: [PostgreSQL](openspec/specs/postgres-schema-workflow/spec.md),
[SQLite](openspec/specs/sqlite-schema-workflow/spec.md). OpenSpec is the
capability and implementation-status source; see [FEATURES.md](FEATURES.md) for
navigation.

### PostgreSQL
- All common PostgreSQL scalar types (uuid, text, varchar, integer, smallint,
  bigint, serial, smallserial, bigserial, real, double precision, boolean,
  numeric, char, date, time, timetz, timestamp, timestamptz, interval, json,
  jsonb, bytea, inet, cidr, macaddr, macaddr8, point, line)
- Array columns, PostGIS `geometry`, pgvector `vector(n)` / `halfvec(n)` /
  `sparsevec(n)` / `bit(n)`, and custom type escape hatch
- Identity columns with sequence options
- Generated stored columns
- Column and table comments
- Safe default helpers for strings, ints, bools, JSON, arrays, and dates
- Primary keys, unique constraints (with `NULLS NOT DISTINCT`), foreign keys
  (with deferrable), check constraints, exclusion constraints
- Advanced indexes (methods, opclass, pgvector `hnsw` / `ivfflat` indexes,
  order, nulls, partial predicates, `CONCURRENTLY`, `ONLY`, `WITH (...)`,
  named auto-generation)
- Strongly typed option helpers for finite PostgreSQL choices such as foreign
  key actions, index methods, and view check options
- PostgreSQL schemas/namespaces, extensions with version pinning, cascade,
  comments, upgrade planning, and extension-owned type attribution, enums,
  sequences, composite types, domains, partitioned tables with child bounds, roles, functions,
  triggers, RLS policies, explicit grants, collations, and tablespace
  assignment for tables, indexes, and materialized views
- Builder-style function bodies, composite fields, views, and materialized
  views, while keeping raw SQL escape hatches
- Views with column aliases, check options, and security options
- Materialized views with storage parameters, tablespaces, and `WITH NO DATA`
- Column-level `COLLATE` clauses
- Raw SQL schema blocks (`pg.RawSQL`) as an escape hatch for DDL the DSL does
  not model, rendered before or after the structured schema, additive-only in
  migration planning, with optional `.Down()` reverse SQL for down migrations
- Schema-qualified rendering
- Deterministic SQL and snapshot JSON with stable snapshot IDs and metadata
  maps (schema, table, column, view, role, function, trigger, policy) keyed for
  migration planning and drift comparison
- Rename annotations (previousName) across supported schema objects, including
  trigger rename planning and manual-review extension replacement detection
- Dialect registry with PostgreSQL as the first provider
- CLI generation, snapshot, and stale-output checks
- Empty runner-compatible migration files for manual SQL
- Baseline runner-compatible migration generation from the current schema, with
  target snapshots embedded in `gosqlkit` metadata
- Structured PostgreSQL migration plans from embedded snapshots, with per-change
  risk metadata, destructive guards, and best-effort reverse SQL
- Migration directory checks for timestamped SQL files, embedded metadata,
  snapshot lineage, goose `Up` / `Down` annotations, and `golang-migrate`
  `.up.sql` metadata
- Optional sandbox replay for committed goose and `golang-migrate` migrations with
  `gosqlkit migrate check --sandbox-url ...`, including replayed database
  introspection against the embedded target snapshot
- PostgreSQL database inspection with `gosqlkit inspect --url ...`, emitting
  deterministic snapshot JSON or the normalised drift-projection shape
- PostgreSQL drift checking with `gosqlkit drift check --url ...` across the
  PostgreSQL snapshot model: namespaces, extensions and extension versions,
  roles, enums, composite types, domains, sequences, functions, tables, columns,
  table constraints,
  standalone indexes, comments, RLS flags, policies, triggers, grants,
  collations, tablespaces, views, and materialized views, with object-level
  diagnostics for missing, extra, and changed objects in a styled terminal
  table and JSON output
- CI-friendly wrappers: `gosqlkit ci schema` checks committed generated SQL,
  snapshot JSON, and migration metadata; `gosqlkit ci database` checks live
  database drift
- `gosqlkit migrate apply --url ...` for goose and `golang-migrate` migrations,
  with runner state tracked in `goose_db_version` or `schema_migrations`
- `DATABASE_URL` / `--url-env` support for database-backed commands, plus JSON
  and quiet output modes for drift and migrate status/apply commands
- Provider-neutral token-as-password auth with `--token-command` /
  `--sandbox-token-command` for managed database workflows
- Shared `--auth password|token-command|azure-entra|aws-iam|gcp-iam` mode
  selection, with the existing provider-specific flags retained as compatible
  aliases; use `--from-auth` for live migration sources and `--sandbox-auth`
  for sandbox replay
- First-class Azure CLI token acquisition with `--azure-cli-token` and
  `--sandbox-azure-cli-token` for Azure Database for PostgreSQL Entra auth
- Azure SDK `DefaultAzureCredential` support with `--azure-default-credential`
  and `--sandbox-azure-default-credential` for hosted and CI workloads
- AWS RDS/Aurora PostgreSQL IAM auth token generation with `--aws-iam-token`
  / `--sandbox-aws-iam-token` via the AWS SDK credential chain, or
  `--aws-cli-token` / `--sandbox-aws-cli-token` via AWS CLI
- Google Cloud SQL PostgreSQL IAM login token generation with `--gcloud-token`
  / `--sandbox-gcloud-token`, or `--gcloud-adc-token` /
  `--sandbox-gcloud-adc-token` for Application Default Credentials
- Native Cloud SQL Go Connector support with `--cloud-sql-connector` (and
  source/sandbox variants) for automatic IAM authentication; an externally
  managed Cloud SQL Auth Proxy continues to work through its PostgreSQL URL
- Destructive-change detection with `--allow-destructive` override and
  `gosqlkit migrate plan` to preview the structured plan before writing, using
  styled terminal tables for humans and JSON for automation
- `gosqlkit migrate refresh <matview> [--concurrently]` to author an explicit,
  risk-flagged migration that repopulates a materialized view
- Fail-closed rename-candidate diagnostics for simple destructive drop/create
  plans, with `previousName` guidance where rename intent should be explicit,
  plus interactive rename selection for TTY workflows
- Structured column alteration planning for type, default, nullability,
  generated expression, and identity changes, with risk metadata and reverse
  SQL where practical
- Enum value removal detection with manual-review risk metadata; automatic
  SQL is intentionally withheld for PostgreSQL enum rebuilds
- `sqlc` compatibility example

### SQLite

- SQLite-native schema declarations, deterministic SQL rendering, and
  versioned snapshots.
- Snapshot-based migration planning, including table rebuilds for changes
  SQLite cannot express with direct `ALTER TABLE` operations.
- CLI `generate`, `snapshot`, `migrate create`, and `migrate plan` support for
  SQLite projects; see [examples/sqlite](examples/sqlite).
- SQLite inspection, drift checking, sandbox replay, migration apply, and
  live-database migration sources are not yet available. Their work is tracked
  in [sqlite-tooling](openspec/changes/sqlite-tooling/tasks.md). Additional
  SQLite schema options are tracked in
  [sqlite-schema-options](openspec/changes/sqlite-schema-options/tasks.md).

## Example Project

See [examples/basic](examples/basic) for a schema that generates PostgreSQL
SQL and feeds `sqlc`.

## Boundaries

`gosqlkit` does not provide an application runtime ORM or query layer, and it
does not replace `sqlc` or the runtime database driver chosen by the
application. It does generate and validate migrations, sandbox-check supported
PostgreSQL migrations, and apply PostgreSQL migrations. SQLite tooling
capabilities are documented in the SQLite contract and active changes linked
above.

## Internal Architecture

```text
pg/                              public PostgreSQL DSL
sqlite/                          public SQLite DSL
kit/                             dialect-neutral provider registry
internal/ast/                    shared schema core (carries JSON tags = snapshot)
internal/dialects/pg/            PostgreSQL schema, renderer, planner, tooling
internal/dialects/sqlite/        SQLite schema, renderer, planner
internal/cli/                    Kong CLI commands
internal/app/                    use cases + config file parsing
cmd/gosqlkit/                    process entrypoint
```

Each dialect has a public DSL and dialect-specific schema and rendering
machinery; see [AGENTS.md](AGENTS.md) for the full layering rules.

## Testing

Unit tests live beside the package they cover and test behaviour at the
package boundary. The verification path includes Go tests, generated SQL drift
checks, snapshot drift checks, CLI smoke tests, and a `sqlc` compatibility
check. See [AGENTS.md](AGENTS.md) for the full verification workflow.
