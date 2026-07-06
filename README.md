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
gosqlkit migrate snapshot diff -> reviewable migration SQL (planned)
goose           migration runner compatibility (planned)
pgx             runtime PostgreSQL driver
```

`gosqlkit` owns the schema layer. `sqlc` owns query code generation and is the
runtime data-access layer alongside `pgx`. Migration generation will use a
`gosqlkit` cross-dialect planner with goose-compatible SQL output. See
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
```

Generate SQL and snapshots (reads `gosqlkit.yaml` — no args needed):

```bash
gosqlkit generate
gosqlkit snapshot
```

Check committed output is current (CI gate):

```bash
gosqlkit generate --check
gosqlkit snapshot --check
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
- PostgreSQL schemas/namespaces, extensions, enums, sequences, composite
  types, domains, roles, functions, triggers, and RLS policies
- Views with column aliases, check options, and security options
- Materialized views with storage parameters and `WITH NO DATA`
- Schema-qualified rendering
- Deterministic SQL and snapshot JSON output with stable snapshot IDs
- Snapshot metadata maps (schema, table, column, view, role, function, trigger, policy) with
  schema-qualified keys for future diffing
- Rename annotations (previousName) on tables, columns, constraints,
  and indexes
- Dialect registry with PostgreSQL as the first provider
- CLI generation, snapshot, and stale-output checks
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
