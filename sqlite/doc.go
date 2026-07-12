// Package sqlite is the public, Go-native SQLite schema DSL. Declaring package
// level variables with these builders registers schema objects; the CLI blank
// imports the user's schema package to collect them and render canonical SQLite
// DDL or a deterministic snapshot through the kit provider registry.
package sqlite
