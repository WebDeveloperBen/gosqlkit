package pgschema

import "sort"

type Namespace struct {
	PreviousName string `json:"previousName,omitempty"`
	Name         string `json:"name"`
}

func sortedNamespaces(input []Namespace) []Namespace {
	items := append([]Namespace(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}
