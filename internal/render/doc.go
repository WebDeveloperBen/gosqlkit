// Package render converts schema models into deterministic SQL.
//
// The current implementation targets PostgreSQL. It performs validation at the
// SQL rendering boundary so generated output fails early when schema objects
// reference unknown columns, invalid identifiers, or unsupported options.
package render
