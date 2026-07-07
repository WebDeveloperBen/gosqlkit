package pgschema

import "sort"

type CompositeType struct {
	PreviousName string               `json:"previousName,omitempty"`
	Schema       string               `json:"schema,omitempty"`
	Name         string               `json:"name"`
	Attributes   []CompositeAttribute `json:"attributes,omitempty"`
}

type CompositeAttribute struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func sortedCompositeTypes(input []CompositeType) []CompositeType {
	items := append([]CompositeType(nil), input...)
	for i := range items {
		attrs := append([]CompositeAttribute(nil), items[i].Attributes...)
		items[i].Attributes = attrs
	}
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}
