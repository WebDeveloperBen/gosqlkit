package app

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
)

type DriftDifference struct {
	Object DriftObjectRef `json:"object"`
	Op     string         `json:"op"`
	Fields []string       `json:"fields,omitempty"`
}

type DriftObjectRef struct {
	Kind string `json:"kind"`
	Key  string `json:"key"`
}

func driftDifferences(desired, database pgschema.Schema) []DriftDifference {
	var diffs []DriftDifference
	compareDriftCollection(&diffs, "schema", desired.Namespaces, database.Namespaces, func(item pgschema.Namespace) string {
		return item.Name
	})
	compareDriftCollection(&diffs, "extension", desired.Extensions, database.Extensions, func(item pgschema.Extension) string {
		return driftQualified(item.Schema, item.Name)
	})
	compareDriftCollection(&diffs, "role", desired.Roles, database.Roles, func(item pgschema.Role) string {
		return item.Name
	})
	compareDriftCollection(&diffs, "collation", desired.Collations, database.Collations, func(item pgschema.Collation) string {
		return driftQualified(item.Schema, item.Name)
	})
	compareDriftCollection(&diffs, "enum", desired.Enums, database.Enums, func(item pgschema.Enum) string {
		return driftQualified(item.Schema, item.Name)
	})
	compareDriftCollection(&diffs, "compositeType", desired.CompositeTypes, database.CompositeTypes, func(item pgschema.CompositeType) string {
		return driftQualified(item.Schema, item.Name)
	})
	compareDriftCollection(&diffs, "domain", desired.Domains, database.Domains, func(item pgschema.Domain) string {
		return driftQualified(item.Schema, item.Name)
	})
	compareDriftCollection(&diffs, "sequence", desired.Sequences, database.Sequences, func(item pgschema.Sequence) string {
		return driftQualified(item.Schema, item.Name)
	})
	compareDriftCollection(&diffs, "function", desired.Functions, database.Functions, driftFunctionKey)
	compareDriftCollection(&diffs, "table", driftShallowTables(desired.Tables), driftShallowTables(database.Tables), driftTableKey)
	compareDriftCollection(&diffs, "view", desired.Views, database.Views, func(item pgschema.View) string {
		return driftQualified(item.Schema, item.Name)
	})
	compareDriftCollection(&diffs, "materializedView", desired.MaterializedViews, database.MaterializedViews, func(item pgschema.MaterializedView) string {
		return driftQualified(item.Schema, item.Name)
	})
	compareDriftCollection(&diffs, "trigger", desired.Triggers, database.Triggers, driftTriggerKey)
	compareDriftCollection(&diffs, "policy", desired.Policies, database.Policies, driftPolicyKey)
	compareDriftCollection(&diffs, "grant", desired.Grants, database.Grants, driftGrantKey)
	compareDriftTableChildren(&diffs, desired.Tables, database.Tables)
	return diffs
}

func compareDriftTableChildren(diffs *[]DriftDifference, desired, database []pgschema.Table) {
	desiredTables := driftMapBy(desired, driftTableKey)
	databaseTables := driftMapBy(database, driftTableKey)
	tableKeys := make([]string, 0, len(desiredTables))
	for key := range desiredTables {
		if _, ok := databaseTables[key]; ok {
			tableKeys = append(tableKeys, key)
		}
	}
	sort.Strings(tableKeys)
	for _, tableKey := range tableKeys {
		desiredTable := desiredTables[tableKey]
		databaseTable := databaseTables[tableKey]
		compareDriftCollection(diffs, "column", desiredTable.Columns, databaseTable.Columns, func(item ast.Column) string {
			return tableKey + "." + item.Name
		})
		compareDriftCollection(diffs, "primaryKey", desiredTable.PrimaryKeys, databaseTable.PrimaryKeys, func(item ast.PrimaryKey) string {
			return tableKey + "." + item.Name
		})
		compareDriftCollection(diffs, "uniqueConstraint", desiredTable.UniqueConstraints, databaseTable.UniqueConstraints, func(item ast.UniqueConstraint) string {
			return tableKey + "." + item.Name
		})
		compareDriftCollection(diffs, "foreignKey", desiredTable.ForeignKeys, databaseTable.ForeignKeys, func(item ast.ForeignKeyConstraint) string {
			return tableKey + "." + item.Name
		})
		compareDriftCollection(diffs, "check", desiredTable.Checks, databaseTable.Checks, func(item ast.Check) string {
			return tableKey + "." + item.Name
		})
		compareDriftCollection(diffs, "exclusion", desiredTable.Exclusions, databaseTable.Exclusions, func(item pgschema.ExclusionConstraint) string {
			return tableKey + "." + item.Name
		})
		compareDriftCollection(diffs, "index", desiredTable.Indexes, databaseTable.Indexes, func(item pgschema.Index) string {
			return tableKey + "." + item.Name
		})
	}
}

func compareDriftCollection[T any](diffs *[]DriftDifference, kind string, desired, database []T, keyFunc func(T) string) {
	desiredByKey := driftMapBy(desired, keyFunc)
	databaseByKey := driftMapBy(database, keyFunc)
	keys := make([]string, 0, len(desiredByKey)+len(databaseByKey))
	seen := map[string]struct{}{}
	for key := range desiredByKey {
		keys = append(keys, key)
		seen[key] = struct{}{}
	}
	for key := range databaseByKey {
		if _, ok := seen[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		desiredValue, desiredOK := desiredByKey[key]
		databaseValue, databaseOK := databaseByKey[key]
		switch {
		case !desiredOK && databaseOK:
			*diffs = append(*diffs, DriftDifference{Op: "extra", Object: DriftObjectRef{Kind: kind, Key: key}})
		case desiredOK && !databaseOK:
			*diffs = append(*diffs, DriftDifference{Op: "missing", Object: DriftObjectRef{Kind: kind, Key: key}})
		case desiredOK && databaseOK && !driftEqual(desiredValue, databaseValue):
			*diffs = append(*diffs, DriftDifference{
				Op:     "changed",
				Object: DriftObjectRef{Kind: kind, Key: key},
				Fields: driftChangedFields(desiredValue, databaseValue),
			})
		}
	}
}

func driftMapBy[T any](items []T, keyFunc func(T) string) map[string]T {
	out := make(map[string]T, len(items))
	for _, item := range items {
		out[keyFunc(item)] = item
	}
	return out
}

func driftEqual(left, right any) bool {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return bytes.Equal(leftJSON, rightJSON)
}

func driftChangedFields(desired, database any) []string {
	var desiredFields map[string]json.RawMessage
	var databaseFields map[string]json.RawMessage
	if json.Unmarshal(mustDriftJSON(desired), &desiredFields) != nil || json.Unmarshal(mustDriftJSON(database), &databaseFields) != nil {
		return nil
	}
	seen := map[string]struct{}{}
	fields := make([]string, 0, len(desiredFields)+len(databaseFields))
	for field, desiredValue := range desiredFields {
		seen[field] = struct{}{}
		if databaseValue, ok := databaseFields[field]; !ok || string(desiredValue) != string(databaseValue) {
			fields = append(fields, field)
		}
	}
	for field := range databaseFields {
		if _, ok := seen[field]; !ok {
			fields = append(fields, field)
		}
	}
	sort.Strings(fields)
	return fields
}

func mustDriftJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return data
}

func driftShallowTables(input []pgschema.Table) []pgschema.Table {
	items := append([]pgschema.Table(nil), input...)
	for i := range items {
		items[i].Columns = nil
		items[i].PrimaryKeys = nil
		items[i].UniqueConstraints = nil
		items[i].ForeignKeys = nil
		items[i].Checks = nil
		items[i].Table.Exclusions = nil
		items[i].Table.Indexes = nil
		items[i].Exclusions = nil
		items[i].Indexes = nil
	}
	return items
}

func driftTableKey(table pgschema.Table) string {
	return driftQualified(table.Schema, table.Name)
}

func driftFunctionKey(fn pgschema.Function) string {
	parts := make([]string, 0, len(fn.Arguments))
	for _, arg := range fn.Arguments {
		if strings.EqualFold(arg.Mode, "OUT") {
			continue
		}
		parts = append(parts, arg.Type)
	}
	return driftQualified(fn.Schema, fn.Name) + "(" + strings.Join(parts, ", ") + ")"
}

func driftTriggerKey(trigger pgschema.Trigger) string {
	return driftQualifiedReference(trigger.Target) + "." + trigger.Name
}

func driftPolicyKey(policy pgschema.Policy) string {
	return driftQualifiedReference(policy.Table) + "." + policy.Name
}

func driftGrantKey(grant pgschema.Grant) string {
	parts := []string{grant.Target.Type, grant.Target.Schema, grant.Target.Name}
	if grant.Target.AllInSchema {
		parts = append(parts, "all")
	}
	parts = append(parts, strings.Join(driftSortedStrings(grant.Grantees), ","))
	return strings.Join(parts, ":")
}

func driftSortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func driftQualified(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}

func driftQualifiedReference(value string) string {
	if strings.Contains(value, ".") {
		return value
	}
	return driftQualified("", value)
}
