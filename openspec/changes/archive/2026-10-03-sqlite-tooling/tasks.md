# Tasks

## 1. SQLite tooling connection and app dispatch

- [x] 1.1 Add the selected pure-Go SQLite driver to the internal tooling path; verify file-backed and in-memory connections work and foreign-key enforcement is enabled on every tooling connection.
- [x] 1.2 Add a dialect-neutral app tooling boundary that keeps SQLite models and snapshots distinct from PostgreSQL types; verify the SQLite provider advertises inspect, drift, replay, and apply only after those app routes are wired.
- [x] 1.3 Route live-source snapshot acquisition for SQLite `migrate plan --from-url` and `migrate create --from-url` through SQLite tooling; verify source introspection feeds structured SQLite plans and CLI help describes a dialect-neutral source location.

## 2. SQLite catalog introspection

- [x] 2.1 Introspect tables, columns, primary/unique/foreign-key/check constraints, indexes, views, and triggers from SQLite catalog tables and PRAGMAs; verify each object family appears in a deterministic snapshot.
- [x] 2.2 Preserve STRICT, WITHOUT ROWID, AUTOINCREMENT, defaults, generated-column expressions, and supported index/view/trigger SQL; verify representative catalog fixtures round-trip into SQLite snapshots.
- [x] 2.3 Return an actionable diagnostic for catalog SQL forms that cannot be safely represented; verify unsupported parsing does not produce a falsely successful, incomplete inspection.

## 3. SQLite drift projection

- [x] 3.1 Normalize only representation differences including type affinity, rowid-backed primary keys, autoindex names, and SQLite catalog formatting; verify normalization preserves meaningful schema changes.
- [x] 3.2 Compare desired and introspected schemas and report missing, extra, and changed supported objects; verify matching schemas pass and a table, column, index, or trigger difference fails with an object-level diagnostic.
- [x] 3.3 Wire inspect and drift CLI/CI paths to SQLite tooling; verify JSON, quiet, unsupported-capability, and drift exit behavior for SQLite configs.

## 4. SQLite sandbox replay

- [x] 4.1 Create or connect to an explicitly isolated SQLite sandbox without deleting a caller-owned database; verify sandbox files are cleaned only when created by the tool.
- [x] 4.2 Replay goose and golang-migrate migrations on one controlled connection with SQLite-compatible transaction handling; verify rebuild PRAGMA toggles occur outside transactions, foreign keys are restored, and failed replay is reported.
- [x] 4.3 Introspect replayed state and compare it with the latest embedded migration target snapshot; verify a matching replay passes and a schema mismatch fails with both snapshot identities.

## 5. SQLite migration apply

- [x] 5.1 Apply goose and golang-migrate files to SQLite and maintain each runner's version/dirty state; verify already-applied versions are skipped and failed migrations do not advance successful version state.
- [x] 5.2 Wire `migrate apply` to SQLite and preserve its JSON/quiet result contract; verify SQLite capabilities are enabled only after both supported runner paths execute successfully.

## 6. Opt-in embedded integration coverage

- [x] 6.1 Add an opt-in, file-backed SQLite integration suite using `modernc.org/sqlite`; verify inspect, drift, sandbox replay, and apply against real catalog and migration execution behavior without Docker.
- [x] 6.2 Add a Taskfile target and CI/documented invocation for the opt-in suite; verify the default `task verify` remains independent of opt-in database integration.
