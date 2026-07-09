package pgschema

import (
	"sort"
	"strings"
)

type Grant struct {
	Target      GrantTarget      `json:"target"`
	Privileges  []GrantPrivilege `json:"privileges"`
	Grantees    []string         `json:"grantees"`
	GrantOption bool             `json:"grantOption,omitempty"`
}

type GrantTarget struct {
	Type        string `json:"type"`
	Schema      string `json:"schema,omitempty"`
	Name        string `json:"name,omitempty"`
	AllInSchema bool   `json:"allInSchema,omitempty"`
}

type GrantPrivilege struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns,omitempty"`
}

func sortedGrants(input []Grant) []Grant {
	items := append([]Grant(nil), input...)
	for i := range items {
		items[i].Privileges = append([]GrantPrivilege(nil), items[i].Privileges...)
		for j := range items[i].Privileges {
			items[i].Privileges[j].Columns = append([]string(nil), items[i].Privileges[j].Columns...)
			sort.Strings(items[i].Privileges[j].Columns)
		}
		sort.SliceStable(items[i].Privileges, func(j, k int) bool {
			if items[i].Privileges[j].Name != items[i].Privileges[k].Name {
				return items[i].Privileges[j].Name < items[i].Privileges[k].Name
			}
			return stringsKey(items[i].Privileges[j].Columns) < stringsKey(items[i].Privileges[k].Columns)
		})
		items[i].Grantees = append([]string(nil), items[i].Grantees...)
		sort.Strings(items[i].Grantees)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return grantKey(items[i]) < grantKey(items[j])
	})
	return items
}

func grantKey(grant Grant) string {
	return grant.Target.Type + ":" + grant.Target.Schema + ":" + grant.Target.Name + ":" + boolKey(grant.Target.AllInSchema) + ":" + stringsKey(grant.Grantees) + ":" + stringsKeyGrantPrivileges(grant.Privileges)
}

func boolKey(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func stringsKey(values []string) string {
	if len(values) == 0 {
		return ""
	}
	out := append([]string(nil), values...)
	sort.Strings(out)
	return strings.Join(out, "\x00")
}

func stringsKeyGrantPrivileges(values []GrantPrivilege) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, value.Name+"("+stringsKey(value.Columns)+")")
	}
	sort.Strings(parts)
	return strings.Join(parts, "\x00")
}
