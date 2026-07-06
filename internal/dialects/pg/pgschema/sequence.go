package pgschema

import "sort"

type Sequence struct {
	MinValue  *int64 `json:"minValue,omitempty"`
	MaxValue  *int64 `json:"maxValue,omitempty"`
	StartWith *int64 `json:"startWith,omitempty"`
	Cache     *int64 `json:"cache,omitempty"`
	Schema    string `json:"schema,omitempty"`
	Name      string `json:"name"`
	OwnedBy   string `json:"ownedBy,omitempty"`
	Increment int64  `json:"increment,omitempty"`
	Cycle     bool   `json:"cycle,omitempty"`
}

func sortedSequences(input []Sequence) []Sequence {
	items := append([]Sequence(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}
