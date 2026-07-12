// Package plan diffs two SQLite schema snapshots into a dialect-neutral
// migration plan (internal/migrate/plan). It emits direct ALTER TABLE
// statements where SQLite supports them (ADD COLUMN, RENAME TABLE, RENAME
// COLUMN, DROP COLUMN, CREATE/DROP INDEX) and falls back to the generalised
// table-rebuild procedure (PRAGMA foreign_keys wrapping, copy into a new table,
// swap) for changes SQLite cannot perform in place. Unsupported or ambiguous
// changes fail closed.
package plan
