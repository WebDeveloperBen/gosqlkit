# Design

## Context

The SQLite provider advertises rendering, snapshots, and migration planning, but not inspect, drift, replay, or apply. `kit.Provider` only exposes dialect information, SQL rendering, and snapshots; the app's database tooling currently routes to PostgreSQL-specific schema and connection types. SQLite snapshots use a separate `sqliteschema.Document`, so routing through PostgreSQL document structs would risk losing SQLite fields. See `proposal.md` for motivation.

## Goals / Non-Goals

**Goals:**
- Add SQLite database tooling through an app-level, dialect-neutral boundary while retaining SQLite-native schema and catalog handling.
- Reuse configured goose and golang-migrate file formats and version semantics where compatible.
- Enable SQLite's four tooling capabilities only when their app paths work.
- Use a pure-Go SQLite driver for application tooling and opt-in integration tests; keep it out of the public schema DSL dependency path.

**Non-Goals:**
- Rework SQLite declarations, SQL rendering, snapshots, migration planning, or table rebuild planning already in place.
- Add PostgreSQL-only schema objects to SQLite.
- Treat arbitrary catalog SQL parsing as complete when a form cannot be safely normalized.

## Decisions

1. **Use `modernc.org/sqlite` for internal database tooling.** A live CLI connection needs an embedded driver; a test-only driver would leave inspect, replay, and apply without a real connection path. The dependency is internal to tooling/app usage, not the public schema DSL API. Opt-in integration tests exercise the same driver against temporary file-backed databases.

2. **Keep SQLite schema models distinct across the tooling boundary.** Add SQLite-specific connection and introspection code under `internal/dialects/sqlite/tooling/`. App dispatch converts through dialect-tagged serialized snapshots or a small internal tooling adapter; it must not unmarshal SQLite snapshots into `pgschema.Document`. The same SQLite snapshot adapter supplies live-source inputs to `migrate plan --from-url` and `migrate create --from-url`, then feeds the existing SQLite planner. Preserve the existing `kit.Provider` interface and use its capability metadata only to advertise working commands.

3. **Treat every connection's PRAGMA state as explicit.** Open and configure a dedicated SQLite connection for each tooling operation, enabling foreign-key enforcement before introspection or migration work. Execute rebuild migrations on a connection/transaction strategy that permits the planner's `PRAGMA foreign_keys=OFF/ON` statements; restore enforcement and report failures if execution exits abnormally.

4. **Normalize catalog state conservatively.** Introspection reads `sqlite_schema` and the relevant table/index/foreign-key PRAGMAs. Parse SQLite-owned SQL needed for checks, generated expressions, indexes, views, and triggers. Normalize only semantics known to be representation-only (including rowid primary keys and autoindex names); unsupported or ambiguous catalog forms must produce diagnostics rather than false no-drift results.

5. **Replay into an isolated database and compare modeled state.** Sandbox replay uses a unique temporary file or isolated in-memory connection, executes migrations using the configured runner grammar, and compares introspected output against the latest embedded target snapshot using SQLite drift projection. The isolated database is closed and removed on success or failure.

6. **Keep migration runner state runner-specific.** Reuse shared migration scanning and goose/golang-migrate parsing, but implement SQLite execution and version bookkeeping in the dialect tooling. Preserve runner-specific transaction and dirty-version semantics; do not assume PostgreSQL transaction behavior applies to SQLite.

## Risks / Trade-offs

- The pure-Go driver adds code and compile cost to tooling binaries; isolating imports prevents it from becoming a dependency of applications that only use the public DSL.
- SQLite foreign-key enforcement is connection-scoped and cannot be toggled inside an active transaction; runner transaction defaults may need SQLite-specific no-transaction handling for rebuild migrations.
- SQLite stores much schema text in `sqlite_schema`; expression parsing and SQL normalization can be incomplete. Fail closed when a safe comparison cannot be formed.
- SQLite in-memory databases are connection-local unless shared-cache URI semantics are used; file-backed temporary databases provide more reliable multi-connection replay behavior.
