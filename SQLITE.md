# SQLite Dialect Context

This document records SQLite-specific architecture and boundaries; it is not
an implementation status board. The published
[SQLite schema workflow](openspec/specs/sqlite-schema-workflow/spec.md) is the
behavior contract. Discover active work with `openspec list --json` and inspect
its ordered tasks with `openspec status --change sqlite-tooling --json` or
`openspec status --change sqlite-schema-options --json`.

## Code Boundaries

- Public declarations: [`sqlite/`](sqlite/)
- SQLite model and deterministic snapshot: `internal/dialects/sqlite/sqliteschema/`
- Validation and SQL rendering: `internal/dialects/sqlite/render/`
- Snapshot migration planner and table rebuilds: `internal/dialects/sqlite/plan/`
- End-to-end example: [`examples/sqlite/`](examples/sqlite/)
- Shared schema core and provider registry: `internal/ast/` and `kit/`

SQLite models only the objects and semantics supported by SQLite. It does not
emulate PostgreSQL schemas, extensions, enums, sequences, domains, composite
types, materialized views, roles, row-level security, grants, server-side
functions, or partitioned tables.

## Design Decisions

- Reuse the shared schema core and generic migration helpers where their
  semantics are genuinely dialect-neutral. Keep SQLite model, rendering,
  catalog interpretation, and migration execution dialect-specific.
- Use `modernc.org/sqlite` for internal runtime tooling and opt-in integration
  tests. The dependency must not enter the public schema DSL's dependency path.
- Treat foreign-key enforcement as connection-scoped state. Tooling configures
  each connection explicitly; table-rebuild migrations must toggle the pragma
  outside an active transaction and restore it on every exit path.
- Introspect SQLite catalog and PRAGMA data conservatively. If a catalog SQL
  form cannot be represented without losing meaningful schema semantics, fail
  with a diagnostic rather than reporting a false match.
- Preserve SQLite-specific migration behavior, including table rebuilds for
  changes SQLite cannot express with direct `ALTER TABLE` operations. Keep
  destructive changes guarded by the migration planner.

See [MIGRATIONS.md](MIGRATIONS.md) for cross-dialect migration decisions and
[AGENTS.md](AGENTS.md) for repository layering conventions.
