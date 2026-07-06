package pgschema

import (
	"sort"
	"strings"
)

type Policy struct {
	Name         string   `json:"name"`
	PreviousName string   `json:"previousName,omitempty"`
	Table        string   `json:"table"`
	Command      string   `json:"command,omitempty"`
	Mode         string   `json:"mode,omitempty"`
	Using        string   `json:"using,omitempty"`
	WithCheck    string   `json:"withCheck,omitempty"`
	Roles        []string `json:"roles,omitempty"`
}

func sortedPolicies(input []Policy) []Policy {
	items := append([]Policy(nil), input...)
	for i := range items {
		items[i].Roles = append([]string(nil), items[i].Roles...)
		sort.Strings(items[i].Roles)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return policyKey(items[i]) < policyKey(items[j])
	})
	return items
}

func policyKey(policy Policy) string {
	return qualifiedPolicyTable(policy.Table) + "." + policy.Name
}

func qualifiedPolicyTable(table string) string {
	if strings.Contains(table, ".") {
		return table
	}
	return qualified("", table)
}
