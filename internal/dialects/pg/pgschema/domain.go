package pgschema

import "sort"

type Domain struct {
	PreviousName string `json:"previousName,omitempty"`
	Schema       string `json:"schema,omitempty"`
	Name         string `json:"name"`
	BaseType     string `json:"baseType"`
	Default      string `json:"default,omitempty"`
	Check        string `json:"check,omitempty"`
	NotNull      bool   `json:"notNull,omitempty"`
}

func sortedDomains(input []Domain) []Domain {
	items := append([]Domain(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}
