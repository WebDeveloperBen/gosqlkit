# gosqlkit

`gosqlkit` is the **schema declaration layer** for Go applications that use
PostgreSQL. It is the Go-native equivalent of Drizzle Kit's schema declarative
piece — you define your database schema in Go, and `gosqlkit` generates
deterministic, reviewable SQL and structured snapshots.

It is intentionally **not an ORM**, **not a query builder**, and **not a
migration runner**. The toolchain is intentionally split:

```text
gosqlkit        schema declarations -> snapshot JSON -> canonical SQL
sqlc            SQL queries -> type-safe Go query code
gosqlkit migrate snapshot diff -> reviewable migration SQL
goose/golang-migrate runner-compatible migration files
pgx             runtime PostgreSQL driver
```

`gosqlkit` owns the schema layer. `sqlc` owns query code generation and is the
runtime data-access layer alongside `pgx`. Migration generation uses a
`gosqlkit` cross-dialect planner with runner-compatible SQL output. See
[MIGRATIONS.md](MIGRATIONS.md) for the migration design.

PostgreSQL is the first implemented dialect; the core is dialect-neutral so
SQLite, MySQL, MSSQL, and other engines can be added as separate dialect
packages under `internal/dialects/`.

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

- All common PostgreSQL scalar types (uuid, text, varchar, integer, smallint,
  bigint, serial, smallserial, bigserial, real, double precision, boolean,
  numeric, char, date, time, timetz, timestamp, timestamptz, interval, json,
  jsonb, bytea, inet, cidr, macaddr, macaddr8, point, line)
- Array columns and custom type escape hatch
- Identity columns with sequence options
- Generated stored columns
- Column and table comments
- Safe default helpers for strings, ints, bools, JSON, arrays, and dates
- Primary keys, unique constraints (with `NULLS NOT DISTINCT`), foreign keys
  (with deferrable), check constraints, exclusion constraints
- Advanced indexes (methods, opclass, order, nulls, partial predicates,
  `CONCURRENTLY`, `ONLY`, `WITH (...)`, named auto-generation)
- Strongly typed option helpers for finite PostgreSQL choices such as foreign
  key actions, index methods, and view check options
- PostgreSQL schemas/namespaces, extensions, enums, sequences, composite
  types, domains, partitioned tables with child bounds, roles, functions,
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
- Deterministic SQL and snapshot JSON output with stable snapshot IDs
- Snapshot metadata maps (schema, table, column, view, role, function, trigger, policy) with
  schema-qualified keys for future diffing
- Rename annotations (previousName) across supported schema objects, including
  trigger rename planning and manual-review extension replacement detection
- Dialect registry with PostgreSQL as the first provider
- CLI generation, snapshot, and stale-output checks
- Empty runner-compatible migration files for manual SQL
- Baseline runner-compatible migration generation from the current schema, with
  target snapshots embedded in `gosqlkit` metadata
- Conservative additive PostgreSQL diff migrations from embedded snapshots
- Migration directory checks for timestamped SQL files, embedded metadata,
  snapshot lineage, goose `Up` / `Down` annotations, and `golang-migrate`
  `.up.sql` metadata
- Optional sandbox replay for committed goose and `golang-migrate` migrations with
  `gosqlkit migrate check --sandbox-url ...`, including replayed database
  introspection against the embedded target snapshot
- PostgreSQL database inspection with `gosqlkit inspect --url ...`, emitting
  deterministic snapshot JSON or the normalised drift-projection shape
- PostgreSQL drift checking with `gosqlkit drift check --url ...` across the
  PostgreSQL snapshot model: namespaces, extensions, roles, enums, composite
  types, domains, sequences, functions, tables, columns, table constraints,
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

## Example Project

See [examples/basic](examples/basic) for a schema that generates PostgreSQL
SQL and feeds `sqlc`.

## Boundaries

`gosqlkit` does not generate runtime models, build queries, apply migrations,
or hide SQL. It is the schema layer only — you still use `sqlc` for query code
generation and `pgx` for runtime database access. Migration generation is
planned as a versioned, review-first workflow documented in
[MIGRATIONS.md](MIGRATIONS.md).

## Internal Architecture

```text
pg/                              public PostgreSQL DSL (user import path)
kit/                             dialect-neutral provider registry
internal/ast/                    shared schema core (carries JSON tags = snapshot)
internal/dialects/pg/            PostgreSQL internal machinery:
  pgschema/                      PG schema envelope + snapshot JSON function
  render/                        PG SQL renderer + validation
internal/cli/                    Kong CLI commands
internal/app/                    use cases + config file parsing
cmd/gosqlkit/                    process entrypoint
```

Adding a new dialect = add `internal/dialects/<name>/` with `schema/` and
`render/` sub-packages, plus a top-level `<name>/` package for the public DSL.

## Testing

Unit tests live beside the package they cover and test behaviour at the
package boundary. The verification path includes Go tests, generated SQL drift
checks, snapshot drift checks, CLI smoke tests, and a `sqlc` compatibility
check. See [AGENTS.md](AGENTS.md) for the full verification workflow.
