# pgkit

`pgkit` is a Go-native PostgreSQL schema DSL and deterministic SQL generator.

It is intentionally not an ORM. The current scope is:

```text
Go schema definitions -> canonical PostgreSQL schema SQL
```

Runtime database access is left to tools such as `sqlc` and `pgx`.

## Example

```go
package schema

import "github.com/webdeveloperben/pgkit/pg"

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
go run ./cmd/pgkit generate --out db/schema.generated.sql ./schema
```

Check committed SQL is current:

```bash
go run ./cmd/pgkit generate --out db/schema.generated.sql --check ./schema
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
- Kong-based CLI generation and stale-output checks
- Golden-style SQL tests
- `sqlc` compatibility example

## Example Project

See [examples/basic](examples/basic) for a two-table schema that generates PostgreSQL SQL and feeds `sqlc`.

```bash
go run ./cmd/pgkit generate --out examples/basic/db/schema.generated.sql ./examples/basic/schema
go run ./cmd/pgkit generate --out examples/basic/db/schema.generated.sql --check ./examples/basic/schema
```

From a copied or standalone example project with `sqlc` installed:

```bash
sqlc generate
```

## Boundaries

`pgkit` does not generate runtime models, build queries, apply migrations, or hide SQL. Migration diffing is planned as a later integration with existing PostgreSQL diff tooling rather than a custom migration engine in the first pass.

## CLI Structure

The CLI follows the same layering style as `tyche`:

- [cmd/pgkit](cmd/pgkit) is only the process entrypoint.
- [internal/cli](internal/cli) owns Kong command definitions and exit handling.
- [internal/app](internal/app) owns use cases with plain Go inputs and outputs.
- The schema DSL and renderer do not import CLI packages.
