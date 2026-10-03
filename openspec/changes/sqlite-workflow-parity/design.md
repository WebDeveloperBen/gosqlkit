# Design

## Context

See `proposal.md` for motivation. The SQLite capability contract is already published and includes generation, planning, inspection, drift, replay, and apply. `internal/app/sqlite_integration_test.go` currently runs one file-backed flow through app functions with Goose; `internal/dialects/sqlite/tooling/migrations_test.go` covers both runners at the tooling layer. PostgreSQL has broader database-backed integration scenarios and a CLI verification path. `task integration:sqlite` is opt-in and uses `modernc.org/sqlite`; it does not require Docker.

`SQLITE.md` is architecture context, not a second status checklist. Its current references to `sqlite-tooling` and `sqlite-schema-options` no longer identify active work.

## Goals / Non-Goals

**Goals:**
- Verify SQLite's existing published workflow through the public executable against real file-backed databases.
- Cover both Goose and golang-migrate end to end, not only in tooling-unit tests.
- Verify data/state transitions, not just successful command output: generated artifacts, migration replay and apply, recorded runner state, drift, and cleanup.
- Make `SQLITE.md` a current guide to the implemented SQLite architecture, command surface, parity boundary, and test entrypoints.

**Non-Goals:**
- Emulate PostgreSQL-only objects or claim literal object-model parity. SQLite remains limited to native SQLite semantics in `sqlite-schema-workflow`.
- Make Docker or SQLite integration tests part of the default `task verify` gate.
- Add an external SQLite runner or another database dependency.
- Change the published SQLite behavior contract. Fixes discovered by the tests must satisfy existing requirements.

## Decisions

1. **Exercise the actual CLI process.** Build the `gosqlkit` binary once for the opt-in suite and invoke it with isolated project configuration, capturing stdout, stderr, and exit status. Calling app functions directly cannot prove argument parsing, config discovery, exit mapping, JSON output, or command routing.

2. **Use isolated in-module schema fixtures and file-backed databases.** Generate temporary schema packages under the Go module so the CLI's schema-loading harness can compile them normally. Give each runner scenario its own temporary project, migration directory, and SQLite database. This exercises the same Go package/config path as a user project while keeping checked-in examples and user data untouched.

3. **Run the same workflow scenarios for both migration formats.** Table-drive Goose and golang-migrate cases through live-source `migrate plan`/`migrate create`, isolated `migrate check`, `migrate apply`, repeat apply, inspect, drift, and the CI database check. Assert the applied schema and preserved data, not merely result counts. Keep runner-specific file/version assertions where their formats differ.

4. **Include an actual schema evolution.** Start from a file containing existing rows, author a change that requires SQLite's supported table-rebuild path, apply/replay it, and verify surviving values, foreign-key posture, indexes/triggers, and the embedded target snapshot. Exercise fail-closed/destructive behavior through CLI-visible errors where the existing contract requires it.

5. **Keep opt-in integration separate from unit verification.** `task integration:sqlite` is the single documented entrypoint for the binary-backed file suite. Default `task verify` remains fast and independent of database integration, matching the existing repository gate.

6. **Document the durable architecture, not task status.** Update `SQLITE.md` with current package boundaries, native object limitations, command capabilities, migration runner behavior, and exact verification commands. Remove stale change-status links; active implementation status remains discoverable through OpenSpec.

## Risks / Trade-offs

- **[CLI process tests are slower than direct app tests]** → Build one executable per test suite and share it across cases; retain focused unit tests for narrow branches.
- **[A passing CLI scenario can miss unsupported catalog forms]** → Keep existing parser, renderer, planner, and tooling unit tests as separate required coverage; the E2E suite proves the integrated supported path, not every SQLite SQL grammar form.
- **[Goose and golang-migrate differ in file layout and state tables]** → Assert each runner's own durable state and normalize only where comparing the shared resulting SQLite schema.
- **[Temporary databases or fixture packages can leak]** → Place all files under test-owned temporary directories and assert cleanup of sandbox artifacts on success and failure.

## Migration Plan

No production database migration is required. The implementation changes tests, Taskfile wiring, and documentation. The opt-in suite must pass with `task integration:sqlite`; the normal repository gate remains `task verify`.
