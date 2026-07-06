package pgschema

import (
	"sort"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
)

func sortedTables(input []ast.Table) []ast.Table {
	items := append([]ast.Table(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	for i := range items {
		items[i] = sortedTable(items[i])
	}
	return items
}

func sortedTable(table ast.Table) ast.Table {
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
	sort.SliceStable(table.Exclusions, func(i, j int) bool {
		return table.Exclusions[i].Name < table.Exclusions[j].Name
	})
	sort.SliceStable(table.Indexes, func(i, j int) bool {
		return table.Indexes[i].Name < table.Indexes[j].Name
	})
	return table
}
