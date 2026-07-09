package pgschema

import "sort"

type Collation struct {
	Deterministic *bool  `json:"deterministic,omitempty"`
	PreviousName  string `json:"previousName,omitempty"`
	Schema        string `json:"schema,omitempty"`
	Name          string `json:"name"`
	Provider      string `json:"provider,omitempty"`
	Locale        string `json:"locale,omitempty"`
	LCCollate     string `json:"lcCollate,omitempty"`
	LCType        string `json:"lcCtype,omitempty"`
	Rules         string `json:"rules,omitempty"`
	Version       string `json:"version,omitempty"`
	From          string `json:"from,omitempty"`
}

func sortedCollations(input []Collation) []Collation {
	items := append([]Collation(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}
