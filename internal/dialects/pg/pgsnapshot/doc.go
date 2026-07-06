// Package pgsnapshot serialises PostgreSQL schema snapshots.
//
// It wraps the dialect-neutral table snapshot produced by package snapshot
// with PostgreSQL schema objects such as namespaces, extensions, and enums.
package pgsnapshot
