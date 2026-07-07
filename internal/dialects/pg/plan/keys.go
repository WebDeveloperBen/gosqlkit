package plan

import (
	"sort"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func qualified(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}

func tableKey(table ast.Table) string {
	return qualified(table.Schema, table.Name)
}

func referencedTableKey(table string) string {
	schema, name, ok := strings.Cut(table, ".")
	if !ok {
		return qualified("", table)
	}
	return qualified(schema, name)
}

func policyKey(policy pgschema.Policy) string {
	return referencedTableKey(policy.Table) + "." + policy.Name
}

func mapBy[T any](items []T, key func(T) string) map[string]T {
	out := make(map[string]T, len(items))
	for _, item := range items {
		out[key(item)] = item
	}
	return out
}

func sortedBy[T any](items []T, key func(T) string) []T {
	out := append([]T(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		return key(out[i]) < key(out[j])
	})
	return out
}

func sortedStrings(items []string) []string {
	out := append([]string(nil), items...)
	sort.Strings(out)
	return out
}

func quoteSQL(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func uniqueRefs(refs []migrateplan.ObjectRef) []migrateplan.ObjectRef {
	seen := make(map[migrateplan.ObjectRef]struct{}, len(refs))
	out := make([]migrateplan.ObjectRef, 0, len(refs))
	for _, ref := range refs {
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}
	return out
}
