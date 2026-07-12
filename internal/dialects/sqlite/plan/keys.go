package plan

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

func unsupported(msg string) error {
	return errors.New(msg)
}

func tableName(table sqliteschema.Table) string   { return table.Name }
func viewName(view sqliteschema.View) string      { return view.Name }
func triggerName(t sqliteschema.Trigger) string   { return t.Name }
func rawSQLName(block sqliteschema.RawSQL) string { return block.Name }

func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// tableDependencies returns the names of other tables this table references via
// foreign keys, so creates order after and drops order before them.
func tableDependencies(table sqliteschema.Table) []string {
	deps := map[string]struct{}{}
	for _, column := range table.Columns {
		if column.References != nil && column.References.Table != table.Name {
			deps[column.References.Table] = struct{}{}
		}
	}
	for _, fk := range table.ForeignKeys {
		if fk.ReferencedTable != table.Name {
			deps[fk.ReferencedTable] = struct{}{}
		}
	}
	out := make([]string, 0, len(deps))
	for name := range deps {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// jsonEqual compares two values by their canonical JSON encoding.
func jsonEqual(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(left, right)
}
