package sqliteschema

import "sort"

type View struct {
	PreviousName  string   `json:"previousName,omitempty"`
	Name          string   `json:"name"`
	Query         string   `json:"query"`
	Comment       string   `json:"comment,omitempty"`
	ColumnAliases []string `json:"columnAliases,omitempty"`
	DependsOn     []string `json:"dependsOn,omitempty"`
	Temporary     bool     `json:"temporary,omitempty"`
	IfNotExists   bool     `json:"ifNotExists,omitempty"`
}

func sortedViews(input []View) []View {
	items := append([]View(nil), input...)
	for i := range items {
		items[i].ColumnAliases = append([]string(nil), items[i].ColumnAliases...)
		items[i].DependsOn = append([]string(nil), items[i].DependsOn...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}
