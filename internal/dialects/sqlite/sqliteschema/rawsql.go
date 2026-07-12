package sqliteschema

import "sort"

// RawSQL is an escape hatch for schema DDL that the structured DSL does not yet
// model (for example PRAGMA statements or virtual-table declarations). Blocks
// are rendered verbatim, ordered deterministically by Name, and placed either
// before or after the structured schema. Raw SQL is opaque to introspection, so
// it is excluded from drift comparison and only supports additive migration
// planning.
type RawSQL struct {
	Name   string `json:"name"`
	SQL    string `json:"sql"`
	Down   string `json:"down,omitempty"`
	Before bool   `json:"before,omitempty"`
}

func sortedRawSQL(input []RawSQL) []RawSQL {
	items := append([]RawSQL(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}
