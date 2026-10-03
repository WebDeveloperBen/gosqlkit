# Proposal

## Why

SQLite implements the published schema and tooling workflows, but its opt-in integration coverage is narrower than PostgreSQL's: the current file-backed integration workflow selects Goose and exercises app entry points rather than the full CLI. `SQLITE.md` also contains stale links to archived changes and does not describe the completed workflow and verification surface in one place.

## What Changes

- Complete `SQLITE.md` as an accurate architecture and boundary guide, including current package responsibilities, supported workflow commands, integration invocation, and the SQLite-native meaning of parity. Remove references that treat archived changes as active work.
- Add an opt-in, file-backed end-to-end suite that drives the SQLite workflow through the CLI and verifies generated artifacts, live-source planning and migration creation, isolated replay, inspection, drift/CI checks, migration application, and runner state.
- Run the integration workflow for both Goose and golang-migrate and verify resulting database state, repeat-apply behavior, foreign-key posture, and cleanup.
- Exercise an actual schema-evolution migration against existing SQLite data, including the supported rebuild path and its safety boundaries; close any implementation gaps against the already-published SQLite contract that these scenarios expose.
- Keep the integration suite opt-in and Docker-free; wire its complete invocation through the Taskfile and document it.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- None. The published `sqlite-schema-workflow` spec already defines the required behavior. This change verifies and closes implementation gaps against that contract without changing it; `skip_specs: true` is set for this testing and documentation change.

## Impact

- Documentation: `SQLITE.md`, `README.md`, and the SQLite example guide where needed.
- Tests: SQLite app/CLI end-to-end coverage and file-backed integration tests using the existing `modernc.org/sqlite` tooling path.
- Taskfile: the opt-in `integration:sqlite` target.
- Runtime behavior changes only if the parity scenarios expose a concrete failure against an existing SQLite requirement. PostgreSQL-only objects remain outside SQLite's capability boundary.
