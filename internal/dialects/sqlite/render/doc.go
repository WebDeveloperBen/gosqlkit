// Package render turns a SQLite schema model into deterministic, canonical
// SQLite DDL and owns all SQLite schema validation.
//
// Validation lives here (not in the DSL constructors) so both the CLI generate
// path and direct-AST users (tests, future introspection) run the same rules.
package render
