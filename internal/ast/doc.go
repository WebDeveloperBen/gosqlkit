// Package ast contains the dialect-neutral in-memory schema core shared by
// dialect DSLs, renderers, and snapshot generation.
//
// The model is intentionally plain table, column, constraint, and index data.
// Behaviour and database-specific schema objects belong in dialect packages,
// renderers, validators, or app-layer orchestration rather than in these types.
package ast
