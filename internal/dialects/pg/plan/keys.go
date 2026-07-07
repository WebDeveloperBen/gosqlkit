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

func triggerKey(trigger pgschema.Trigger) string {
	return referencedTableKey(trigger.Target) + "." + trigger.Name
}

func viewKey(view pgschema.View) string {
	return qualified(view.Schema, view.Name)
}

func materializedViewKey(view pgschema.MaterializedView) string {
	return qualified(view.Schema, view.Name)
}

func sequenceKey(sequence pgschema.Sequence) string {
	return qualified(sequence.Schema, sequence.Name)
}

func functionKey(function pgschema.Function) string {
	return qualified(function.Schema, function.Name) + "(" + renderFunctionIdentityArguments(function) + ")"
}

func renderFunctionIdentityArguments(function pgschema.Function) string {
	parts := make([]string, 0, len(function.Arguments))
	for _, arg := range function.Arguments {
		if strings.EqualFold(arg.Mode, "OUT") {
			continue
		}
		parts = append(parts, arg.Type)
	}
	return strings.Join(parts, ", ")
}

func triggerFunctionKey(trigger pgschema.Trigger) string {
	return referencedTableKey(trigger.Function) + "()"
}

func sequenceOwnedByTable(ownedBy string) (string, bool) {
	parts := strings.Split(ownedBy, ".")
	if len(parts) < 2 {
		return "", false
	}
	return strings.Join(parts[:len(parts)-1], "."), true
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
