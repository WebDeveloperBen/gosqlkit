package pgschema

import "sort"

type Extension struct {
	PreviousName string `json:"previousName,omitempty"`
	Version      string `json:"version,omitempty"`
	Name         string `json:"name"`
	Schema       string `json:"schema,omitempty"`
	Cascade      bool   `json:"cascade,omitempty"`
}

func sortedExtensions(input []Extension) []Extension {
	items := append([]Extension(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}
