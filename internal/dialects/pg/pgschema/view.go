package pgschema

import "sort"

type View struct {
	PreviousName    string   `json:"previousName,omitempty"`
	Schema          string   `json:"schema,omitempty"`
	Name            string   `json:"name"`
	Query           string   `json:"query"`
	Comment         string   `json:"comment,omitempty"`
	CheckOption     string   `json:"checkOption,omitempty"`
	ColumnAliases   []string `json:"columnAliases,omitempty"`
	DependsOn       []string `json:"dependsOn,omitempty"`
	SecurityBarrier bool     `json:"securityBarrier,omitempty"`
	SecurityInvoker bool     `json:"securityInvoker,omitempty"`
}

type MaterializedView struct {
	PreviousName  string            `json:"previousName,omitempty"`
	With          map[string]string `json:"with,omitempty"`
	Schema        string            `json:"schema,omitempty"`
	Name          string            `json:"name"`
	Query         string            `json:"query"`
	Comment       string            `json:"comment,omitempty"`
	ColumnAliases []string          `json:"columnAliases,omitempty"`
	DependsOn     []string          `json:"dependsOn,omitempty"`
	NoData        bool              `json:"noData,omitempty"`
}

func sortedViews(input []View) []View {
	items := append([]View(nil), input...)
	for i := range items {
		items[i].ColumnAliases = append([]string(nil), items[i].ColumnAliases...)
		items[i].DependsOn = append([]string(nil), items[i].DependsOn...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}

func sortedMaterializedViews(input []MaterializedView) []MaterializedView {
	items := append([]MaterializedView(nil), input...)
	for i := range items {
		if items[i].With != nil {
			sorted := make(map[string]string, len(items[i].With))
			for k, v := range items[i].With {
				sorted[k] = v
			}
			items[i].With = sorted
		}
		items[i].ColumnAliases = append([]string(nil), items[i].ColumnAliases...)
		items[i].DependsOn = append([]string(nil), items[i].DependsOn...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}
