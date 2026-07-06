// Package app contains CLI use-case orchestration.
//
// Functions in this package take plain Go option structs and return typed
// results or errors. They deliberately avoid depending on the CLI parser so the
// behaviour can be tested directly and reused by future command surfaces.
package app
