package pgschema

import "sort"

type Namespace struct {
	Name string `json:"name"`
}

func sortedNamespaces(input []Namespace) []Namespace {
	items := append([]Namespace(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}
