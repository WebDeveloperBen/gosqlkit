package pgschema

import (
	"sort"
	"strings"
)

type Trigger struct {
	Initially       string   `json:"initially,omitempty"`
	ReferencedTable string   `json:"referencedTable,omitempty"`
	Target          string   `json:"target"`
	Function        string   `json:"function"`
	Timing          string   `json:"timing"`
	Level           string   `json:"level,omitempty"`
	Name            string   `json:"name"`
	Comment         string   `json:"comment,omitempty"`
	When            string   `json:"when,omitempty"`
	PreviousName    string   `json:"previousName,omitempty"`
	Arguments       []string `json:"arguments,omitempty"`
	Events          []string `json:"events"`
	Columns         []string `json:"columns,omitempty"`
	Constraint      bool     `json:"constraint,omitempty"`
	Deferrable      bool     `json:"deferrable,omitempty"`
}

func sortedTriggers(input []Trigger) []Trigger {
	items := append([]Trigger(nil), input...)
	for i := range items {
		items[i].Events = append([]string(nil), items[i].Events...)
		items[i].Columns = append([]string(nil), items[i].Columns...)
		items[i].Arguments = append([]string(nil), items[i].Arguments...)
		sort.Strings(items[i].Events)
		sort.Strings(items[i].Columns)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return triggerKey(items[i]) < triggerKey(items[j])
	})
	return items
}

func triggerKey(trigger Trigger) string {
	return qualifiedTriggerTarget(trigger.Target) + "." + trigger.Name
}

func qualifiedTriggerTarget(target string) string {
	if strings.Contains(target, ".") {
		return target
	}
	return qualified("", target)
}
