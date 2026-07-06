// Package pg provides the PostgreSQL schema definition DSL.
//
// It owns PostgreSQL-specific schema declarations, registration, and helper
// APIs. The package registers itself with package kit as the "postgres"
// provider with common aliases so the CLI can render PostgreSQL output through
// the dialect-neutral registry.
package pg
