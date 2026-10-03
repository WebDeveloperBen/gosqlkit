// Package sqliteschema contains SQLite-specific schema objects.
//
// The shared AST package owns dialect-neutral table, column, constraint, and
// index data. This package layers the small set of SQLite-only options over
// that core model (WITHOUT ROWID, STRICT, AUTOINCREMENT, per-column index
// collation) and owns the deterministic snapshot JSON function. The model types
// carry JSON tags directly, so the snapshot is the model serialised.
package sqliteschema
