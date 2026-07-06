# gosqlkit

`gosqlkit` is a Go-native schema DSL and deterministic schema generator.
PostgreSQL is the first implemented dialect; the core CLI path is
dialect-selectable so SQLite, MySQL, MSSQL, SingleStore, CockroachDB, and other
dialects can be added without making PostgreSQL the application core.

It is intentionally not an ORM. The current scope is:

```text
Go schema definitions -> deterministic snapshot JSON -> canonical dialect SQL
```

Runtime database access is left to tools such as `sqlc` and `pgx`.

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

Generate SQL:

```bash
go run ./cmd/gosqlkit generate --out db/schema.generated.sql ./schema
go run ./cmd/gosqlkit snapshot --out db/schema.snapshot.json ./schema
```

Check committed SQL is current:

```bash
go run ./cmd/gosqlkit generate --out db/schema.generated.sql --check ./schema
go run ./cmd/gosqlkit snapshot --out db/schema.snapshot.json --check ./schema
```

## Current Features

- Tables and columns
- Column primary keys
- Column unique constraints
- Inline foreign keys
- Basic indexes
- Basic checks
- Defaults
- Nullable and not-null columns
- Deterministic SQL output
- Deterministic snapshot JSON output
- Dialect registry with PostgreSQL as the first provider
- Kong-based CLI generation and stale-output checks
- Golden-style SQL tests
- `sqlc` compatibility example

## Example Project

See [examples/basic](examples/basic) for a two-table schema that generates PostgreSQL SQL and feeds `sqlc`.

```bash
go run ./cmd/gosqlkit generate --out examples/basic/db/schema.generated.sql ./examples/basic/schema
go run ./cmd/gosqlkit generate --out examples/basic/db/schema.generated.sql --check ./examples/basic/schema
go run ./cmd/gosqlkit snapshot --out examples/basic/db/schema.snapshot.json ./examples/basic/schema
go run ./cmd/gosqlkit snapshot --out examples/basic/db/schema.snapshot.json --check ./examples/basic/schema
```

From a copied or standalone example project with `sqlc` installed:

```bash
sqlc generate
```

## Boundaries

`gosqlkit` does not generate runtime models, build queries, apply migrations, or hide SQL. Migration diffing is planned as a later integration with existing PostgreSQL diff tooling rather than a custom migration engine in the first pass.

## CLI Structure

The CLI follows the same layering style as `tyche`:

- [cmd/gosqlkit](cmd/gosqlkit) is only the process entrypoint.
- [internal/cli](internal/cli) owns Kong command definitions and exit handling.
- [internal/app](internal/app) owns use cases with plain Go inputs and outputs.
- The schema DSL and renderer do not import CLI packages.

## Testing Direction

Unit tests should live beside the package they cover and test behaviour at the
package boundary. The current verification path includes Go tests, generated
SQL drift checks, snapshot drift checks, CLI smoke tests, and a `sqlc`
compatibility check. Later integration tests should use testcontainers to
exercise generated SQL and introspection against real database engines.
