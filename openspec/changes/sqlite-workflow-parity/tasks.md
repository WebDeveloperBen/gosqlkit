# Tasks

## 1. Complete SQLite architecture guidance

- [x] 1.1 Update `SQLITE.md` to document current SQLite package boundaries, supported native semantics and CLI workflows, both migration runners, and the opt-in verification commands; remove stale links to archived changes and verify every remaining repository link and command target resolves.

## 2. Exercise the public SQLite CLI

- [x] 2.1 Add a binary-backed CLI test fixture using an isolated in-module schema package; verify `generate`, `snapshot`, and their `--check` paths produce deterministic SQLite artifacts, pass when current, and fail when stale.
- [x] 2.2 Exercise SQLite `inspect`, `drift check`, and `ci database` through the CLI against a file-backed database; verify snapshot dialect/identity, clean results, object-level drift diagnostics, JSON output, and nonzero drift exit status.

## 3. Verify migration workflows against real SQLite files

- [x] 3.1 Run the live-source `migrate plan`/`migrate create`, isolated `migrate check`, `migrate apply`, and repeat-apply workflow through the CLI for both Goose and golang-migrate; verify replay matches the embedded target snapshot, applied schema matches the desired state, and each runner records and skips its own applied versions.
- [x] 3.2 Exercise a non-empty schema evolution that requires the supported table-rebuild path for both runners; verify surviving row values, foreign-key enforcement, and dependent indexes/triggers after apply and replay, and verify unsupported/destructive cases fail closed unless explicitly authorized.
- [x] 3.3 Verify file-backed sandbox databases and temporary schema fixtures are removed after successful and failed CLI runs; retain the database supplied as the live source and assert its expected post-apply state.

## 4. Wire and run the complete opt-in suite

- [x] 4.1 Update `task integration:sqlite` to build the CLI once and run the complete binary-backed file suite with `modernc.org/sqlite`; document the command and Docker-free prerequisite in `SQLITE.md` and the SQLite example guide, and verify default `task verify` remains opt-in-free.
- [x] 4.2 Run `task verify` and `task integration:sqlite`; confirm both PostgreSQL/SQLite generated-artifact checks remain current and the complete SQLite CLI integration suite passes for both runners.
