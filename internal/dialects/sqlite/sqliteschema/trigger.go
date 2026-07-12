package sqliteschema

import "sort"

type Trigger struct {
	PreviousName    string   `json:"previousName,omitempty"`
	Name            string   `json:"name"`
	Target          string   `json:"target"`
	Timing          string   `json:"timing,omitempty"`
	When            string   `json:"when,omitempty"`
	Body            string   `json:"body"`
	Comment         string   `json:"comment,omitempty"`
	Events          []string `json:"events"`
	UpdateOfColumns []string `json:"updateOfColumns,omitempty"`
	ForEachRow      bool     `json:"forEachRow,omitempty"`
}

func sortedTriggers(input []Trigger) []Trigger {
	items := append([]Trigger(nil), input...)
	for i := range items {
		items[i].Events = append([]string(nil), items[i].Events...)
		items[i].UpdateOfColumns = append([]string(nil), items[i].UpdateOfColumns...)
		sort.Strings(items[i].Events)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return triggerKey(items[i]) < triggerKey(items[j])
	})
	return items
}

func triggerKey(trigger Trigger) string {
	return trigger.Target + "." + trigger.Name
}
