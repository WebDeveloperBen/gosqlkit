# Product Capability References

This file is a navigation page, not an implementation checklist. Published
behavioral requirements live in OpenSpec:

- [PostgreSQL schema workflow](openspec/specs/postgres-schema-workflow/spec.md)
- [SQLite schema workflow](openspec/specs/sqlite-schema-workflow/spec.md)
- [Project status tracking](openspec/specs/project-status-tracking/spec.md)

Use `openspec list --specs --json` to discover published capabilities and
`openspec list --json` / `openspec status --change <name> --json` to discover
active implementation work. Each active change's `tasks.md` is its executable
work plan. Verify implementation claims against code, tests, examples, and
Taskfile commands; a requirement is not evidence that it has shipped.

`README.md` is the user-facing feature summary. `SPEC.md` and `MIGRATIONS.md`
remain decision records. `AGENTS.md` documents architecture and working
conventions.

## Reference Background

The PostgreSQL workflow uses the breadth of Drizzle Kit as a schema-management
reference, not as an implementation blueprint. The project keeps the Go-native
DSL and structured snapshot model, excludes ORM/query-builder behavior, and
leaves runtime database access to application-owned drivers. See
[AGENTS.md](AGENTS.md) for the design principles and reference locations.

SQLite is a separate dialect model for SQLite-supported semantics, not
PostgreSQL parity by emulation. Its dialect-specific behavior and active
changes are linked above; PostgreSQL-only server objects remain outside its
scope.

[SQLITE.md](SQLITE.md) records SQLite architectural context. It is not a second
status board.
