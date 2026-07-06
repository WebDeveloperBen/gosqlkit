package pgschema

import "sort"

type Enum struct {
	Schema string   `json:"schema,omitempty"`
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

func sortedEnums(input []Enum) []Enum {
	items := append([]Enum(nil), input...)
	for i := range items {
		items[i].Values = append([]string(nil), items[i].Values...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}
