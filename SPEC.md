# ADR: Build `gosqlkit` as a Go-native SQL schema DSL and migration-generation workflow

## Status

Accepted. Current capability requirements and implementation work are tracked in OpenSpec.

## Executive summary

We will build `gosqlkit`, a Go-native schema definition toolkit that allows application teams to declare database schema in Go code, generate deterministic schema snapshots, generate dialect-specific SQL schema output, and produce reviewable migration files.

The goal is to achieve a Drizzle Kit / Prisma-style schema management workflow for Go applications without adopting a runtime ORM. `gosqlkit` will own schema declaration, snapshot generation, and SQL generation only. Runtime database access will remain with `sqlc`, `pgx`, and dialect equivalents.

The intended toolchain is:

```text
gosqlkit         → Go schema DSL, snapshots, and canonical dialect SQL generation
gosqlkit migrate → cross-dialect snapshot diff and migration SQL generation
goose            → migration runner compatibility
sqlc            → SQL query code generation
pgx             → runtime PostgreSQL driver
```

PostgreSQL was the first implemented dialect; SQLite is now a separate
implemented dialect. The core stays dialect-neutral so additional engines such
as MySQL, MSSQL, SingleStore, and CockroachDB can be added independently.

## Context

The current Go ecosystem has strong runtime database tooling, particularly `sqlc` and `pgx`, but lacks a clean, free, Go-native equivalent to the Drizzle Kit / Prisma workflow.

Desired workflow:

```text
Declare tables/functions/indexes in code
→ generate canonical dialect schema SQL
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

We will build `gosqlkit`.

`gosqlkit` will provide Go schema DSL packages that compile to deterministic dialect-specific SQL.

It will not be an ORM.

It will not generate runtime models.

It will not replace `sqlc`.

It will not own query building.

It will focus only on:

```text
Go schema definitions
→ deterministic schema snapshot
→ canonical dialect-specific schema SQL
```

Migration generation will use a `gosqlkit` cross-dialect planner rather than a
PostgreSQL-only diff engine as the core abstraction. PostgreSQL-specific tools
such as Drizzle Kit, Atlas, `pgschema`, and `pg-schema-diff` remain references
and possible validation or implementation aids. Generated migrations should be
goose-compatible by default. See [MIGRATIONS.md](MIGRATIONS.md).

## Target developer experience

Example schema definition:

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
gosqlkit generate --out db/schema.generated.sql ./schema
gosqlkit generate --out db/schema.generated.sql --check ./schema
gosqlkit snapshot --out db/schema.snapshot.json ./schema
gosqlkit snapshot --out db/schema.snapshot.json --check ./schema

gosqlkit migrate create add_users_table

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
- Generate deterministic, reviewable dialect-specific SQL.
- Support generated migration files from schema diffs.
- Avoid ORM coupling.
- Avoid paid schema-management tooling.
- Keep each supported database dialect first-class rather than flattening them into a lowest-common-denominator abstraction.
- Make schema review easier by reviewing desired state and generated migration output together.

## Non-goals

- Building a runtime ORM.
- Building a query builder.
- Replacing `sqlc`.
- Replacing `pgx`.
- Building a full migration engine from scratch in the first version.
- Building runtime database clients for each dialect.
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

## Current Capability Status

This ADR records the original product decision and initial scope. Published
behavior contracts are [PostgreSQL](openspec/specs/postgres-schema-workflow/spec.md),
[SQLite](openspec/specs/sqlite-schema-workflow/spec.md), and the
[project status](openspec/specs/project-status-tracking/spec.md). Active
changes hold ordered implementation tasks. Use `openspec list --specs --json`
and `openspec list --json` for current capability and work status rather than
reconstructing status from this decision record.

The original PostgreSQL vertical slice has expanded into the PostgreSQL and
SQLite workflows defined by their capability specs. Additional dialects or
capability changes require a proposal grounded in current code and tests.

## Proposed architecture

```text
gosqlkit/
  cmd/gosqlkit/
    main.go

  internal/cli/
    root.go
    generate.go
    snapshot.go
    version.go

  internal/app/
    generate.go

  kit/
    registry.go

  pg/                              public PostgreSQL DSL (user import path)
    table.go
    column.go
    schema.go
    registry.go
    defaults.go

  internal/dialects/pg/            PostgreSQL internal machinery
    pgschema/
      schema.go                    PG schema envelope + snapshot JSON
    render/
      postgres.go

  internal/ast/
    schema.go
```

Core flow:

```text
User Go schema definitions
→ dialect package registry, for example pg
→ selected kit provider
→ shared schema model plus dialect-specific extensions
→ deterministic dialect renderer
→ internal/app generation use case
→ internal/cli command adapter
→ schema.generated.sql
→ gosqlkit migration planner
→ goose-compatible migration file
```

CLI layering follows the same broad shape as `tyche`:

- `cmd/gosqlkit` is only the process boundary: call the CLI runner, translate panics/exit codes, and exit.
- `internal/cli` owns Kong command structs, help text, argument parsing, and exit-code mapping.
- `internal/app` owns user-facing use cases with plain Go option/result types and no dependency on Kong.
- `kit` owns dialect-neutral provider registration, lookup, aliases, and capabilities.
- Dialect packages such as `pg` own their schema DSL, registry, renderer selection, and snapshot dialect marker.
- `internal/ast` owns the shared schema core: tables, columns, constraints, and indexes.
- Dialect-specific schema envelopes, such as `internal/dialects/pg/pgschema`, own database-specific objects such as PostgreSQL namespaces, extensions, and enums. The model types carry JSON tags directly so the snapshot is the model serialised — no separate snapshot conversion layer.
- `pg`, `kit`, `internal/ast`, `internal/dialects` do not import CLI packages.
- New CLI commands should first become app-layer functions, then thin CLI adapters.

## Design principles

- Schema declarations should feel like Go, not stringly typed SQL everywhere.
- Generated SQL must be deterministic.
- CLI parsing should stay separate from schema generation and application behaviour.
- Runtime database access remains explicit SQL via `sqlc`.
- Each dialect DSL should expose native database features rather than flattening every database into a lowest-common-denominator abstraction.
- Shared core types should cover concepts common across SQL stores, while dialect-specific features stay owned by their provider package.
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

### Delegate all migration planning to PostgreSQL-only tools

Rejected as the core architecture. PostgreSQL-only tools are useful references,
validators, and possible implementation aids, but `gosqlkit` needs a
cross-dialect migration model. The detailed decision is recorded in
[MIGRATIONS.md](MIGRATIONS.md).

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
- Cross-dialect migration plans depend on dialect-specific, semantics-aware
  implementations.
- Destructive changes and renames must fail closed when safe planning is
  unavailable.
- Initial version will not be as feature-complete as mature ORM ecosystems.

## Current Capability References

The original phased implementation roadmap is retired. Published OpenSpec
capability specs define behavior, and active OpenSpec changes define unfinished
work. This ADR remains the record of product goals and architectural decisions,
not a release-phase checklist.

## Further Decisions

Questions raised during the initial design have since been resolved where
needed by implementation and later decisions. Record new product decisions in
this ADR and track proposed or unfinished behavior through OpenSpec changes.

## Initial Slice (Historical Acceptance)

The initial acceptance criterion was a minimal Go schema DSL that generated
stable PostgreSQL SQL for a two-table example consumable by `sqlc`. That
criterion records the starting point for this project; current behavior and
remaining work are defined by the published OpenSpec capabilities and active
changes.
