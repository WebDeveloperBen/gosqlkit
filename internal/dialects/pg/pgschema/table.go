package pgschema

import (
	"sort"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
)

type Table struct {
	Indexes    []Index               `json:"indexes,omitempty"`
	Exclusions []ExclusionConstraint `json:"exclusions,omitempty"`
	ast.Table
}

func sortedTables(input []Table) []Table {
	items := append([]Table(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
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
	sort.SliceStable(table.Exclusions, func(i, j int) bool {
		return table.Exclusions[i].Name < table.Exclusions[j].Name
	})
	sort.SliceStable(table.Indexes, func(i, j int) bool {
		return table.Indexes[i].Name < table.Indexes[j].Name
	})
	for i := range table.Indexes {
		if table.Indexes[i].With != nil {
			sorted := make(map[string]string, len(table.Indexes[i].With))
			for k, v := range table.Indexes[i].With {
				sorted[k] = v
			}
			table.Indexes[i].With = sorted
		}
	}
	return table
}
