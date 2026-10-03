# SQLite example

A small end-to-end gosqlkit example for the **SQLite** dialect.

- `schema/` — the Go-native schema definitions (`github.com/webdeveloperben/gosqlkit/sqlite`).
- `gosqlkit.yaml` — config selecting `dialect: sqlite` and the output paths.
- `db/schema.generated.sql` — canonical SQLite DDL (regenerate, don't hand-edit).
- `db/schema.snapshot.json` — deterministic schema snapshot (regenerate, don't hand-edit).

Regenerate from this directory:

```bash
go run ../../cmd/gosqlkit generate
go run ../../cmd/gosqlkit snapshot
```

The schema exercises STRICT and WITHOUT ROWID tables, `INTEGER PRIMARY KEY
AUTOINCREMENT`, generated (VIRTUAL) columns, `COLLATE NOCASE` columns and
indexes, composite primary keys, foreign keys with actions and deferral, partial
indexes, a view, a trigger, and a `PRAGMA` raw-SQL block.

## File-backed integration suite

Run from the repository root:

```bash
task integration:sqlite
```

The opt-in target builds the CLI once, then exercises the binary-backed SQLite
workflow against file databases using `modernc.org/sqlite`. It does not require
Docker and is separate from the default `task verify` gate.
