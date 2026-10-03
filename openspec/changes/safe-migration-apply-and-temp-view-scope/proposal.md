# Proposal

## Why

SQLite rebuild migrations execute outside a transaction when they toggle `PRAGMA foreign_keys`, so a later statement failure can leave the live schema partially rebuilt. PostgreSQL apply likewise executes statements individually, allowing partial DDL to survive without a successful migration record. SQLite temporary views are renderable and snapshot-able, but their session-local nature and exclusion from live database comparison are not explicit in the contract.

## What Changes

- Apply transaction-compatible migrations atomically for SQLite and PostgreSQL, including migration-state updates where the selected runner permits it.
- Preserve explicit behavior for operations that cannot run inside a transaction; do not silently imply rollback guarantees for those operations, and retain actionable runner failure state.
- Execute SQLite rebuild DDL and data-copy/swap steps in a transaction while handling the connection-scoped foreign-key pragma outside that transaction; restore and verify the configured foreign-key posture on both success and failure.
- Specify and test that SQLite temporary views remain supported in generated SQL and schema snapshots but are excluded from live inspection and drift comparison because they are connection-local.

## Capabilities

### New Capabilities

### Modified Capabilities

- `sqlite-schema-workflow`: Define migration failure atomicity and temporary-view behavior in database inspection and drift comparison.
- `postgres-schema-workflow`: Define migration failure atomicity and runner state behavior for transaction-compatible and explicitly non-transactional migrations.

## Impact

- SQLite and PostgreSQL migration application in `internal/dialects/{sqlite,pg}/tooling/` and `internal/app/`, with PostgreSQL non-transactional plan-risk propagation.
- Migration runner integration and failure-path tests for Goose and golang-migrate.
- `MIGRATIONS.md` and `SQLITE.md` for migration transaction guarantees, non-transactional exceptions, and SQLite temporary-view comparison semantics.
