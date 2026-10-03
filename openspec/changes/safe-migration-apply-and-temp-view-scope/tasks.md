# Tasks

## 1. SQLite Atomic Apply and Temporary Views

- [x] 1.1 Update SQLite migration execution to parse the generated foreign-key guard before execution, run `PRAGMA foreign_keys=OFF` outside the transaction, and execute rebuild statements, `PRAGMA foreign_key_check`, and the successful Goose state update in one transaction; restore and verify `foreign_keys=ON` after success or rollback, and reject unsupported guard sequences before DDL. Verify with SQLite tooling tests covering both successful and malformed-guard migrations.
- [x] 1.2 Add SQLite failure-path coverage for a rebuild that fails after schema/data changes have begun, for both supported runner state paths; assert the original schema/data and migration state are preserved and foreign-key enforcement is restored. Verify with `go test ./internal/dialects/sqlite/tooling/...` and the relevant app migration tests.
- [x] 1.3 Add inspection and drift regression coverage proving temporary views remain in declared snapshots/generated SQL but are absent from live persistent-object comparison. Document this connection-local boundary and SQLite rebuild transaction behavior in `SQLITE.md`; verify the targeted SQLite tooling/app tests pass.

## 2. PostgreSQL Atomic Apply and Non-Transactional Risks

- [x] 2.1 Propagate `RiskNonTransactional` from structured PostgreSQL plan changes into migration metadata for operations that cannot execute in a transaction; preserve runner-compatible migration rendering and reject unclassified opaque SQL before execution when rollback safety cannot be established. Verify with planner/metadata tests including a non-transactional operation and an unclassifiable SQL case.
- [x] 2.2 Apply transaction-compatible PostgreSQL migration statements and successful Goose or golang-migrate state updates in one transaction, retaining golang-migrate dirty state on transactional failure. Verify with tooling tests that demonstrate rollback after a later statement fails and atomic success-state commit for both runners.
- [x] 2.3 Preserve explicit non-transactional execution semantics and actionable dirty/recovery state for supported non-transactional operations; ensure failures are not described as atomic rollbacks. Verify with PostgreSQL tooling tests covering a non-transactional operation and failure-state inspection.
- [x] 2.4 Document PostgreSQL migration transaction guarantees and non-transactional recovery limits in `MIGRATIONS.md`; verify the documented distinction agrees with the apply and runner tests.

## 3. End-to-End Verification

- [x] 3.1 Run `task verify` and `task integration:sqlite`; resolve failures in the changed paths and confirm both commands pass.
- [x] 3.2 Run the opt-in PostgreSQL integration suite against supported PostgreSQL versions when Docker is available; verify transactional rollback, successful runner-state commit, and non-transactional recovery behavior against a real database.
