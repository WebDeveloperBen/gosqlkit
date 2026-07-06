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
pg-schema-diff  snapshot diff -> reviewable migration SQL (planned)
goose           migration runner (optional)
pgx             runtime PostgreSQL driver
```

`gosqlkit` owns the schema layer. `sqlc` owns query code generation and is the
runtime data-access layer alongside `pgx`. Migration diffing will be delegated
to `pg-schema-diff`, not rebuilt.

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

Generate SQL and snapshots:

```bash
go run ./cmd/gosqlkit generate --out db/schema.generated.sql ./schema
go run ./cmd/gosqlkit snapshot --out db/schema.snapshot.json ./schema
```

Check committed output is current (CI gate):

```bash
go run ./cmd/gosqlkit generate --out db/schema.generated.sql --check ./schema
go run ./cmd/gosqlkit snapshot --out db/schema.snapshot.json --check ./schema
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
  types, and domains
- Schema-qualified rendering
- Deterministic SQL and snapshot JSON output
- Dialect registry with PostgreSQL as the first provider
- CLI generation and stale-output checks
- `sqlc` compatibility example

## Example Project

See [examples/basic](examples/basic) for a schema that generates PostgreSQL
SQL and feeds `sqlc`.

## Boundaries

`gosqlkit` does not generate runtime models, build queries, apply migrations,
or hide SQL. It is the schema layer only — you still use `sqlc` for query code
generation and `pgx` for runtime database access. Migration diffing is planned
as an integration with `pg-schema-diff`.

## Internal Architecture

```text
pg/                              public PostgreSQL DSL (user import path)
kit/                             dialect-neutral provider registry
internal/ast/                    shared schema core (dialect-neutral)
internal/snapshot/               shared snapshot JSON format
internal/dialects/pg/            PostgreSQL internal machinery:
  pgschema/                      PG schema envelope (namespaces, enums, etc.)
  pgsnapshot/                    PG snapshot envelope
  render/                        PG SQL renderer + validation
internal/cli/                    Kong CLI commands
internal/app/                    use cases (plain Go, no CLI dep)
cmd/gosqlkit/                    process entrypoint
```

Adding a new dialect = add `internal/dialects/<name>/` with `schema/`,
`snapshot/`, and `render/` sub-packages, plus a top-level `<name>/` package
for the public DSL.

## Testing

Unit tests live beside the package they cover and test behaviour at the
package boundary. The verification path includes Go tests, generated SQL drift
checks, snapshot drift checks, CLI smoke tests, and a `sqlc` compatibility
check. See [AGENTS.md](AGENTS.md) for the full verification workflow.
