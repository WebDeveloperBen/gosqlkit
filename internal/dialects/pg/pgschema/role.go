package pgschema

import "sort"

type Role struct {
	Login           *bool    `json:"login,omitempty"`
	Superuser       *bool    `json:"superuser,omitempty"`
	CreateDB        *bool    `json:"createDb,omitempty"`
	CreateRole      *bool    `json:"createRole,omitempty"`
	Inherit         *bool    `json:"inherit,omitempty"`
	Replication     *bool    `json:"replication,omitempty"`
	BypassRLS       *bool    `json:"bypassRls,omitempty"`
	ConnectionLimit *int     `json:"connectionLimit,omitempty"`
	Name            string   `json:"name"`
	PreviousName    string   `json:"previousName,omitempty"`
	ValidUntil      string   `json:"validUntil,omitempty"`
	MemberOf        []string `json:"memberOf,omitempty"`
	AdminOf         []string `json:"adminOf,omitempty"`
}

func sortedRoles(input []Role) []Role {
	items := append([]Role(nil), input...)
	for i := range items {
		items[i].MemberOf = append([]string(nil), items[i].MemberOf...)
		sort.Strings(items[i].MemberOf)
		items[i].AdminOf = append([]string(nil), items[i].AdminOf...)
		sort.Strings(items[i].AdminOf)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}
