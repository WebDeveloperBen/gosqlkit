// Command gosqlkit is the schema generation CLI.
//
// The command package is intentionally small: it delegates parsing and command
// execution to internal/cli, translates structured exits into process status
// codes, and keeps panic handling at the process boundary.
package main
