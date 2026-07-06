// Package pgschema contains PostgreSQL-specific schema objects.
//
// The shared AST package owns dialect-neutral table, column, constraint, and
// index data. This package layers PostgreSQL objects such as schemas,
// extensions, and enums over that core model.
package pgschema
