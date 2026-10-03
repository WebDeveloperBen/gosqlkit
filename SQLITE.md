# SQLite Dialect Context

This document records SQLite-specific architecture, semantics, and operational
guidance; it is not an implementation-status board. The published
[SQLite schema workflow](openspec/specs/sqlite-schema-workflow/spec.md) is the
behavior contract. Discover active work with `openspec list --json` and inspect
its tasks with `openspec status --change <name> --json`.

## Scope and Parity

SQLite is a distinct dialect with its own schema model and SQL semantics.
Parity with PostgreSQL means an equivalent supported schema-to-SQL, snapshot,
migration, database-tooling, and CLI workflow where SQLite has native
semantics. It does not mean emulating PostgreSQL-only objects or making the
SQLite model interchangeable with PostgreSQL.

The public DSL models SQLite storage affinities and supported table options,
columns, constraints, indexes, views, triggers, and ordered raw SQL. It
supports SQLite-specific behavior such as `STRICT`, `WITHOUT ROWID`,
`AUTOINCREMENT`, generated columns, foreign-key actions, partial indexes, and
`IF NOT EXISTS`. Previous-name metadata feeds SQLite migration planning.

SQLite does not model PostgreSQL schemas, extensions, enums, standalone
sequences, domains, composite types, materialized views, roles, row-level
security, policies, grants, server-side functions, or partitioned tables.

## Code Boundaries

- Public declarations and dialect registration: [`sqlite/`](sqlite/)
- SQLite model and deterministic snapshot JSON:
  `internal/dialects/sqlite/sqliteschema/`
- Validation and canonical SQL rendering:
  `internal/dialects/sqlite/render/`
- Snapshot migration planning, including table rebuilds:
  `internal/dialects/sqlite/plan/`
- SQLite connections, catalog introspection, and migration execution:
  `internal/dialects/sqlite/tooling/`
- Dialect-neutral workflow dispatch and CLI:
  `internal/app/` and `internal/cli/`
- Runnable SQLite example: [`examples/sqlite`](examples/sqlite)

The public DSL does not depend on a database driver. Internal SQLite tooling
and its file-backed integration suite use `modernc.org/sqlite`.

## Rendering, Snapshots, and Planning

Rendering validates SQLite identifiers and supported option combinations and
produces deterministic DDL. Snapshots are versioned, dialect-tagged, and
deterministically ordered. The snapshot model is the planner input; SQLite
snapshots are not converted into PostgreSQL models.

The planner uses direct SQLite alterations where supported and rebuilds
tables for changes that SQLite cannot express with `ALTER TABLE`. Rebuilds
preserve surviving column data and recreate affected indexes and triggers.
Destructive changes require explicit authorization. Table and column renames
use SQLite rename statements; index renames use drop/create; view and trigger
renames replace the old object. Ambiguous or unsupported rename combinations
fail closed without a partial plan.

## Database Tooling and CLI

Tooling accepts local SQLite file paths and `sqlite:` URLs. It exposes
inspection, drift checking, live-source migration planning and creation,
isolated migration replay, migration application, and the CI database check.
The selected runner is Goose or golang-migrate; apply records and skips each
runner's migration state.

With `dialect: sqlite` configured, representative commands are:

```bash
gosqlkit generate
gosqlkit snapshot
gosqlkit inspect --url ./app.db
gosqlkit drift check --url ./app.db
gosqlkit ci database --url ./app.db
gosqlkit migrate plan --from-url ./app.db
gosqlkit migrate create add_schema_change --from-url ./app.db
gosqlkit migrate check --sandbox-url sqlite::memory:
gosqlkit migrate apply --url ./app.db
```

Supplying a non-empty `--sandbox-url` enables migration replay, but the
supplied URL is not opened or modified. SQLite replay uses a separate
temporary database and removes it after success or failure. Tooling enables
foreign-key enforcement on its connections; table rebuilds handle SQLite's
connection-scoped foreign-key pragma outside active transactions and restore
the configured posture.

## Verification

The normal `task verify` gate runs unit tests, build, formatting, generated
artifact checks, CLI smoke checks, lint, and vulnerability checks. It does not
require database integration. The opt-in `task integration:sqlite` target
builds the CLI once and exercises the binary-backed, file-database workflow
suite plus the SQLite app integration workflow. It uses `modernc.org/sqlite`
and does not require Docker:

```bash
task integration:sqlite
```

The checked-in example SQL and snapshot can be regenerated and checked with:

```bash
task generate:sqlite
task snapshot:sqlite
task generate:sqlite:check
task snapshot:sqlite:check
```

See [MIGRATIONS.md](MIGRATIONS.md) for cross-dialect migration decisions,
[AGENTS.md](AGENTS.md) for repository layering and verification conventions,
and [README.md](README.md) for the user-facing feature summary.
