package sqliteschema

import (
	"sort"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
)

type Table struct {
	Columns []Column `json:"columns,omitempty"`
	Indexes []Index  `json:"indexes,omitempty"`
	ast.Table
	WithoutRowID bool `json:"withoutRowid,omitempty"`
	Strict       bool `json:"strict,omitempty"`
	IfNotExists  bool `json:"ifNotExists,omitempty"`
}

type Column struct {
	ast.Column
	AutoIncrement bool `json:"autoIncrement,omitempty"`
}

func sortedTables(input []Table) []Table {
	items := append([]Table(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	for i := range items {
		items[i] = sortedTable(items[i])
	}
	return items
}

func sortedTable(table Table) Table {
	sort.SliceStable(table.PrimaryKeys, func(i, j int) bool {
		return table.PrimaryKeys[i].Name < table.PrimaryKeys[j].Name
	})
	sort.SliceStable(table.UniqueConstraints, func(i, j int) bool {
		return table.UniqueConstraints[i].Name < table.UniqueConstraints[j].Name
	})
	sort.SliceStable(table.ForeignKeys, func(i, j int) bool {
		return table.ForeignKeys[i].Name < table.ForeignKeys[j].Name
	})
	sort.SliceStable(table.Checks, func(i, j int) bool {
		return table.Checks[i].Name < table.Checks[j].Name
	})
	sort.SliceStable(table.Indexes, func(i, j int) bool {
		return table.Indexes[i].Name < table.Indexes[j].Name
	})
	return table
}
