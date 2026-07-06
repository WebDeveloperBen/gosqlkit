# ADR: Build `pgkit` as a Go-native PostgreSQL schema DSL and migration-generation workflow

## Status

Accepted. Phase 1 vertical slice implemented.

## Executive summary

We will build `pgkit`, a Go-native PostgreSQL schema definition toolkit that allows application teams to declare database schema in Go code, generate deterministic PostgreSQL SQL schema output, and use existing migration-diff tooling to produce migration files.

The goal is to achieve a Drizzle Kit / Prisma-style schema management workflow for Go applications without adopting a runtime ORM. `pgkit` will own schema declaration and SQL generation only. Runtime database access will remain with `sqlc` and `pgx`.

The intended toolchain is:

```text
pgkit            → Go schema DSL and canonical PostgreSQL schema generation
pg-schema-diff   → schema diff and migration SQL generation
goose            → optional migration runner/history
sqlc             → SQL query code generation
pgx              → runtime PostgreSQL driver
```

This replaces the need to adopt GORM, Bun, Ent, Bob, Atlas, Prisma, or Drizzle ORM for this workflow.

## Context

The current Go ecosystem has strong runtime database tooling, particularly `sqlc` and `pgx`, but lacks a clean, free, Go-native equivalent to the Drizzle Kit / Prisma workflow.

Desired workflow:

```text
Declare tables/functions/indexes in code
→ generate canonical PostgreSQL schema SQL
→ diff desired schema against current state
→ generate reviewable SQL migrations
→ apply migrations
→ generate type-safe Go query code with sqlc
```

Existing options do not fully satisfy this:

- `sqlc` is excellent for query code generation, but does not define schema or generate migrations.
- `goose` is reliable for applying migrations, but requires manually authored migration files.
- GORM/Bun/Ent introduce ORM coupling.
- Bob is primarily database-first and code-generation focused.
- Atlas provides useful schema management features, but the desired workflow depends on paid features we do not want to rely on.
- Drizzle Kit provides the desired workflow, but introduces a TypeScript schema layer into a Go codebase.

## Decision

We will build `pgkit`.

`pgkit` will provide a Go schema DSL that compiles to deterministic PostgreSQL SQL.

It will not be an ORM.

It will not generate runtime models.

It will not replace `sqlc`.

It will not own query building.

It will focus only on:

```text
Go schema definitions
→ canonical PostgreSQL schema SQL
```

The initial integration target will be `pg-schema-diff` for generating migration SQL from the generated schema. `goose` may be used to store and apply versioned migrations.

## Target developer experience

Example schema definition:

```go
package schema

import "github.com/insurgence/pgkit/pg"

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

Example generated SQL:

```sql
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE,
    display_name text,
    created_at timestamptz NOT NULL DEFAULT now()
);
```

Example workflow:

```bash
pgkit generate --out db/schema.generated.sql ./schema
pgkit generate --out db/schema.generated.sql --check ./schema

pgkit diff \
  --from "$DATABASE_URL" \
  --to db/schema.generated.sql \
  --name add_users_table

sqlc generate
go test ./...
```

Expected project layout:

```text
schema/
  users.go
  invoices.go
  events.go

db/
  schema.generated.sql
  migrations/
  queries/

internal/db/
  sqlc generated code
```

## Goals

- Provide a Go-native schema-as-code experience.
- Preserve `sqlc` and `pgx` as the runtime database layer.
- Generate deterministic, reviewable PostgreSQL SQL.
- Support generated migration files from schema diffs.
- Avoid ORM coupling.
- Avoid paid schema-management tooling.
- Keep PostgreSQL as a first-class target rather than abstracting across databases.
- Make schema review easier by reviewing desired state and generated migration output together.

## Non-goals

- Building a runtime ORM.
- Building a query builder.
- Replacing `sqlc`.
- Replacing `pgx`.
- Building a full migration engine from scratch in the first version.
- Supporting multiple databases.
- Hiding SQL from developers.
- Automatically applying destructive changes without review.

## Initial scope

Version 0 should support:

- Tables
- Columns
- Primary keys
- Unique constraints
- Foreign keys
- Indexes
- Basic checks
- Defaults
- Nullable / not-null columns
- Common PostgreSQL types:

  - `uuid`
  - `text`
  - `varchar`
  - `integer`
  - `bigint`
  - `boolean`
  - `numeric`
  - `date`
  - `timestamp`
  - `timestamptz`
  - `jsonb`

- Deterministic SQL generation
- Stable ordering of schema objects
- CLI command for generation
- CLI stale-output check for CI
- Golden-file tests for SQL output

Current implementation status:

- Implemented:
  - Tables
  - Columns
  - Column primary keys
  - Column unique constraints
  - Inline foreign keys
  - Table-level primary keys
  - Composite primary keys
  - Named unique constraints
  - Composite unique constraints
  - Named foreign keys
  - Composite foreign keys
  - Basic indexes
  - Unique indexes
  - Advanced index methods
  - Index predicates
  - Per-column index ordering/null ordering/operator classes
  - Concurrent indexes
  - Index storage parameters
  - Basic checks
  - Defaults
  - Nullable / not-null columns
  - Common PostgreSQL scalar types listed above
  - Deterministic table ordering that respects foreign-key dependencies
  - Deterministic index ordering
  - CLI `generate`
  - CLI `generate --out`
  - CLI `generate --check`
  - Golden-style SQL output tests
  - `sqlc` compatibility example
- Not implemented yet:
  - Extensions, enums, views, RLS, triggers, functions
  - Migration diff integration

## Later scope

Future versions may support:

- Extensions
- Enums
- Views
- Materialized views
- Functions
- Triggers
- RLS policies
- Composite types
- Domains
- Partitioned tables
- Comments
- Rename annotations
- Destructive-change guards
- Database introspection
- Password and OAuth2/token database authentication
- Provider-pluggable PostgreSQL authentication
- Azure Database for PostgreSQL Microsoft Entra authentication
- AWS RDS and Aurora PostgreSQL IAM database authentication
- Google Cloud SQL for PostgreSQL IAM database authentication
- Expand/contract migration helpers
- Goose migration file generation
- Embedded `pg-schema-diff` integration
- CI drift checks

## Proposed architecture

```text
pgkit/
  cmd/pgkit/
    main.go

  internal/cli/
    root.go
    generate.go
    version.go

  internal/app/
    generate.go

  pg/
    table.go
    column.go
    types.go
    constraints.go
    indexes.go
    defaults.go

  internal/ast/
    schema.go

  internal/render/
    postgres.go

  internal/diff/
    pgschemadiff.go

  internal/testing/
    golden.go
```

Core flow:

```text
User Go schema definitions
→ pgkit schema registry
→ internal schema AST
→ deterministic PostgreSQL renderer
→ internal/app generation use case
→ internal/cli command adapter
→ schema.generated.sql
→ pg-schema-diff migration generation
→ optional goose migration file
```

CLI layering follows the same broad shape as `tyche`:

- `cmd/pgkit` is only the process boundary: call the CLI runner, translate panics/exit codes, and exit.
- `internal/cli` owns Kong command structs, help text, argument parsing, and exit-code mapping.
- `internal/app` owns user-facing use cases with plain Go option/result types and no dependency on Kong.
- `pg`, `internal/ast`, and `internal/render` do not import CLI packages.
- New CLI commands should first become app-layer functions, then thin CLI adapters.

## Design principles

- Schema declarations should feel like Go, not stringly typed SQL everywhere.
- Generated SQL must be deterministic.
- CLI parsing should stay separate from schema generation and application behaviour.
- Runtime database access remains explicit SQL via `sqlc`.
- The schema DSL should expose PostgreSQL features rather than flatten them into a generic abstraction.
- Migration generation should be review-first, apply-second.
- Destructive changes should be obvious and guarded.
- Database connectivity should support both password authentication and short-lived token authentication.
- Database authentication should be provider-pluggable so Azure, AWS, GCP, hosted PostgreSQL vendors, and self-hosted deployments can be supported without changing schema declarations.
- OAuth2/Entra/IAM access tokens must be treated as ephemeral secrets and never written to generated artefacts.
- Provider-specific token lifetimes, username formats, host binding, region binding, and TLS requirements must be explicit in the connection layer.
- The library should be small enough to understand and extend.

## Alternatives considered

### Continue with goose and sqlc only

Rejected for this workflow. It is reliable but requires manually authored migrations and does not provide a declarative desired-state schema.

### Use GORM/Bun/Ent

Rejected because these introduce ORM concepts and runtime coupling. The desired tool should manage schema only.

### Use Bob

Rejected because Bob is closer to database-first code generation than schema-first migration generation.

### Use Atlas

Rejected for now because the desired feature set depends on paid functionality.

### Use Drizzle Kit

Technically viable, but rejected as the primary direction because it introduces TypeScript schema tooling into a Go-first project.

### Build full migration engine from scratch

Rejected for initial scope. Migration planning is complex and risky. We will initially delegate this to `pg-schema-diff` or another existing PostgreSQL diff engine.

## Consequences

Positive:

- Go-native schema declaration.
- No runtime ORM coupling.
- Preserves `sqlc` and explicit SQL.
- Easier schema review.
- Generated migrations can still be committed and audited.
- Keeps the toolchain small and focused.

Negative:

- Requires building and maintaining custom schema DSL tooling.
- Migration quality depends on the chosen diff engine.
- Advanced PostgreSQL features will require deliberate support.
- Rename detection and destructive-change handling will need careful design.
- Initial version will not be as feature-complete as mature ORM ecosystems.

## Recommended implementation phases

### Phase 1: Schema DSL and SQL generation

Build only:

```text
Go schema DSL
→ canonical schema.generated.sql
```

Deliverables:

- `pg.Table`
- common column types
- constraints
- indexes
- deterministic renderer
- CLI `pgkit generate`
- golden-file tests

### Phase 2: sqlc compatibility

Ensure generated schema SQL can be consumed by `sqlc`.

Deliverables:

- example app using `pgkit + sqlc + pgx`
- CI command to regenerate schema
- validation that committed schema output is up to date

### Phase 3: Migration diff integration

Integrate with `pg-schema-diff`.

Deliverables:

- `pgkit diff`
- database connection layer for introspection/diff inputs
- password auth and provider-pluggable token auth
- Azure Database for PostgreSQL Microsoft Entra token-as-password support
- AWS RDS/Aurora PostgreSQL IAM token support
- Google Cloud SQL PostgreSQL IAM auth support
- custom token command/provider support for other hosted PostgreSQL environments
- generated migration SQL
- safety warnings
- diff output suitable for PR review

### Phase 4: Migration history integration

Optionally generate goose-compatible migration files.

Deliverables:

- `pgkit migration create <name>`
- generated `Up` SQL
- best-effort `Down` SQL where safe
- explicit warnings where down migration is unsafe or unavailable

### Phase 5: Advanced PostgreSQL support

Add support for extensions, functions, views, RLS, triggers, and advanced index options.

## Open questions

- Should schema registration be explicit or automatic through package init?
- Should generated SQL be one file or multiple files?
- Should `pgkit` generate `Down` migrations, or require explicit manual review?
- Should destructive changes fail by default?
- How should table/column renames be represented?
- Should the DSL support raw SQL escape hatches from day one?
- Should `pgkit` own migration application, or leave that entirely to goose?

## Initial agent task

Build the first vertical slice:

```text
Minimal Go schema DSL
→ deterministic PostgreSQL SQL generation
→ CLI command
→ golden tests
→ example project using sqlc
```

Do not build a migration engine in the first pass.

Do not build an ORM.

Do not build query generation.

The success criteria for the first version is that a developer can define two related tables in Go and generate stable PostgreSQL SQL that can be reviewed, committed, and consumed by `sqlc`.
