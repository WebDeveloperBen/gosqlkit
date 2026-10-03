# Design

## Context

See `proposal.md` and the spec deltas for motivation and observable guarantees. SQLite currently wraps ordinary migrations in transactions but executes any migration containing a foreign-key pragma statement-by-statement; generated rebuilds place `PRAGMA foreign_keys=OFF` before the table-copy/swap sequence and `ON` after it. PostgreSQL apply currently executes migration statements individually. `migrate/plan.RiskNonTransactional` exists, while PostgreSQL structured plans do not currently use it. SQLite introspection reads persistent objects from `sqlite_schema`, and drift projection already drops temporary views.

## Goals / Non-Goals

**Goals:**
- Roll back transaction-compatible DDL and migration success state together on failure for SQLite and PostgreSQL.
- Preserve runner-specific dirty/recovery state and supported non-transactional operations.
- Keep SQLite foreign-key enforcement enabled outside the rebuild transaction and restore it after both success and failure.
- Make temporary-view behavior explicit without treating connection-local objects as persistent database drift.

**Non-Goals:**
- Change the Goose or golang-migrate file formats, snapshot formats, or migration identity/version scheme.
- Promise rollback for PostgreSQL operations that cannot execute within a transaction.
- Make a CLI process inspect temporary views created on an unrelated application connection.

## Decisions

1. **Use one transaction per transaction-compatible migration.** Apply ordinary migration statements within a database transaction. For Goose, write its successful version row in that transaction. For golang-migrate, persist the dirty marker before execution; commit DDL and the clean version state together. A failed transactional migration rolls back DDL, while golang-migrate retains its dirty marker for explicit recovery.

2. **Keep SQLite foreign-key pragmas outside, but the rebuild body inside, the transaction.** Recognize the generated `foreign_keys=OFF` / `foreign_keys=ON` guard as connection setup/cleanup, execute the OFF setting before beginning the transaction, and execute the rebuild body, foreign-key check, and successful Goose state update in the transaction. Commit only if the body and `PRAGMA foreign_key_check` succeed. Roll back on failure, then restore and verify `foreign_keys=ON`; join restoration failures with the original error. Reject malformed or unsupported pragma guard sequences before applying migration DDL rather than silently falling back to statement-by-statement execution.

3. **Use explicit migration risk metadata for PostgreSQL non-transactional work.** Mark structured changes that cannot run inside a transaction with the existing `RiskNonTransactional` risk and carry that risk in migration metadata. Apply those changes through the runner's non-transactional path and preserve actionable failure state; do not label them atomic. Transaction-compatible migrations use the transactional path. Opaque SQL that cannot be classified safely must not be silently treated as rollback-safe.

4. **Keep temporary SQLite views ephemeral for database comparison.** Preserve them in declared-schema snapshots and generated SQL. Continue excluding them from live inspection/drift projection because each CLI command opens its own connection and temporary views are not durable objects in the file database. Add tests and documentation to pin this boundary.

5. **Keep dialect transaction handling in dialect tooling.** The shared app layer continues to dispatch by dialect; SQLite pragma rules and PostgreSQL runner behavior remain in their dialect-specific tooling. Both runner formats receive the same database safety guarantees where the SQL is transaction-compatible.

## Risks / Trade-offs

- [SQLite foreign-key restoration can fail after the transaction ends] → Return both the migration and restoration errors, verify the pragma value, and retain the connection failure state rather than claiming success.
- [Foreign-key validation can expose pre-existing invalid data] → Fail before commit and include the reported table/row context; do not commit a rebuild with violations.
- [Non-transactional PostgreSQL SQL can leave partial effects] → Mark structured non-transactional operations in migration metadata and retain runner-specific recovery state; reject unclassified cases before execution when atomicity cannot be established.
- [Version-table changes can become inconsistent with DDL] → Place successful Goose state and golang-migrate clean state in the same transaction as DDL; retain golang-migrate dirty state outside the transaction on failure.

## Migration Plan

No database schema or snapshot-format migration is required. Update the apply execution paths and migration risk propagation, then cover successful and failed execution for both runners on SQLite and PostgreSQL. Verify SQLite temporary-view exclusion through inspection/drift tests and document it in `SQLITE.md`. Run `task verify`, `task integration:sqlite`, and the opt-in PostgreSQL integration suite where Docker is available. Rollback after a failed transactional migration is automatic; failed non-transactional operations require the selected runner's documented recovery procedure.
