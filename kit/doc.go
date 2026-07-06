// Package kit is the dialect-neutral provider registry used by gosqlkit.
//
// Dialect packages, such as package pg, register self-describing providers
// here. Application code and CLI orchestration can then render SQL or snapshots
// for a selected dialect without importing the dialect-specific schema package
// directly.
package kit
