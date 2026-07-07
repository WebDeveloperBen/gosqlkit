package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

type Planner struct{}

func (Planner) Dialect() string {
	return "postgresql"
}

func (Planner) PlanSnapshotDiff(previousJSON, currentJSON []byte) (*migrateplan.Plan, error) {
	return SnapshotDiff(previousJSON, currentJSON)
}

func SnapshotDiff(previousJSON, currentJSON []byte) (*migrateplan.Plan, error) {
	var previous, current pgschema.Document
	if err := json.Unmarshal(previousJSON, &previous); err != nil {
		return nil, fmt.Errorf("parse previous snapshot: %w", err)
	}
	if err := json.Unmarshal(currentJSON, &current); err != nil {
		return nil, fmt.Errorf("parse current snapshot: %w", err)
	}

	planner := planner{plan: &migrateplan.Plan{}}
	if err := planner.namespaces(previous.Namespaces, current.Namespaces); err != nil {
		return nil, err
	}
	if err := planner.extensions(previous.Extensions, current.Extensions); err != nil {
		return nil, err
	}
	if err := planner.enums(previous.Enums, current.Enums); err != nil {
		return nil, err
	}
	if err := planner.tables(previous.Tables, current.Tables); err != nil {
		return nil, err
	}
	if err := rejectChangedCollection("roles", previous.Roles, current.Roles); err != nil {
		return nil, err
	}
	if err := rejectChangedCollection("composite types", previous.CompositeTypes, current.CompositeTypes); err != nil {
		return nil, err
	}
	if err := rejectChangedCollection("domains", previous.Domains, current.Domains); err != nil {
		return nil, err
	}
	if err := rejectChangedCollection("sequences", previous.Sequences, current.Sequences); err != nil {
		return nil, err
	}
	if err := rejectChangedCollection("functions", previous.Functions, current.Functions); err != nil {
		return nil, err
	}
	if err := rejectChangedCollection("views", previous.Views, current.Views); err != nil {
		return nil, err
	}
	if err := rejectChangedCollection("materialized views", previous.MaterializedViews, current.MaterializedViews); err != nil {
		return nil, err
	}
	if err := rejectChangedCollection("triggers", previous.Triggers, current.Triggers); err != nil {
		return nil, err
	}
	if err := rejectChangedCollection("policies", previous.Policies, current.Policies); err != nil {
		return nil, err
	}
	return planner.plan, nil
}

type planner struct {
	plan *migrateplan.Plan
}

func (p planner) namespaces(previous, current []pgschema.Namespace) error {
	prev := make(map[string]pgschema.Namespace, len(previous))
	for _, item := range previous {
		prev[item.Name] = item
	}
	for _, item := range current {
		if _, ok := prev[item.Name]; !ok {
			p.add(migrateplan.OperationCreate, "schema."+item.Name, "create schema "+item.Name, "CREATE SCHEMA "+item.Name+";")
			continue
		}
		delete(prev, item.Name)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("schemas were removed")
	}
	return nil
}

func (p planner) extensions(previous, current []pgschema.Extension) error {
	prev := mapBy(previous, func(item pgschema.Extension) string { return qualified(item.Schema, item.Name) })
	for _, item := range current {
		key := qualified(item.Schema, item.Name)
		if _, ok := prev[key]; !ok {
			stmt := "CREATE EXTENSION " + item.Name
			if item.Schema != "" {
				stmt += " WITH SCHEMA " + item.Schema
			}
			p.add(migrateplan.OperationCreate, "extension."+key, "create extension "+key, stmt+";")
			continue
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("extensions were removed")
	}
	return nil
}

func (p planner) enums(previous, current []pgschema.Enum) error {
	prev := mapBy(previous, func(item pgschema.Enum) string { return qualified(item.Schema, item.Name) })
	for _, item := range current {
		key := qualified(item.Schema, item.Name)
		old, ok := prev[key]
		if !ok {
			values := make([]string, 0, len(item.Values))
			for _, value := range item.Values {
				values = append(values, quoteSQL(value))
			}
			p.add(migrateplan.OperationCreate, "enum."+key, "create enum "+key,
				"CREATE TYPE "+renderQualified(item.Schema, item.Name)+" AS ENUM ("+strings.Join(values, ", ")+");")
			continue
		}
		if err := p.enumValues(old, item); err != nil {
			return err
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("enums were removed")
	}
	return nil
}

func (p planner) enumValues(previous, current pgschema.Enum) error {
	if len(current.Values) < len(previous.Values) {
		return unsupportedDestructive("enum values were removed")
	}
	for i, value := range previous.Values {
		if current.Values[i] != value {
			return unsupported("enum values were reordered or changed")
		}
	}
	for _, value := range current.Values[len(previous.Values):] {
		key := qualified(current.Schema, current.Name)
		p.add(migrateplan.OperationAlter, "enum."+key, "add enum value "+value+" to "+key,
			"ALTER TYPE "+renderQualified(current.Schema, current.Name)+" ADD VALUE "+quoteSQL(value)+";")
	}
	return nil
}

func (p planner) tables(previous, current []ast.Table) error {
	prev := mapBy(previous, tableKey)
	for _, table := range current {
		key := tableKey(table)
		old, ok := prev[key]
		if !ok {
			if err := ensureCreateTableSupported(table); err != nil {
				return err
			}
			stmt, err := renderCreateTable(table)
			if err != nil {
				return err
			}
			p.add(migrateplan.OperationCreate, "table."+key, "create table "+key, stmt)
			continue
		}
		if err := p.table(old, table); err != nil {
			return err
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("tables were removed")
	}
	return nil
}

func (p planner) table(previous, current ast.Table) error {
	prevColumns := mapBy(previous.Columns, func(column ast.Column) string { return column.Name })
	for _, column := range current.Columns {
		old, ok := prevColumns[column.Name]
		if !ok {
			if err := ensureAddColumnSupported(current, column); err != nil {
				return err
			}
			def, err := renderColumn(column)
			if err != nil {
				return err
			}
			key := tableKey(current) + "." + column.Name
			p.add(migrateplan.OperationAlter, "column."+key, "add column "+key,
				"ALTER TABLE "+renderTableName(current)+" ADD COLUMN "+def+";")
			continue
		}
		if !reflect.DeepEqual(old, column) {
			return unsupported("column modifications require semantic planning")
		}
		delete(prevColumns, column.Name)
	}
	if len(prevColumns) > 0 {
		return unsupportedDestructive("columns were removed")
	}

	withoutColumnsPrevious := previous
	withoutColumnsCurrent := current
	withoutColumnsPrevious.Columns = nil
	withoutColumnsCurrent.Columns = nil
	if !reflect.DeepEqual(withoutColumnsPrevious, withoutColumnsCurrent) {
		return unsupported("table constraint, index, RLS, or comment changes require semantic planning")
	}
	return nil
}

func (p planner) add(op migrateplan.Operation, object, summary, statement string) {
	p.plan.Changes = append(p.plan.Changes, migrateplan.Change{Op: op, Object: object, Summary: summary})
	p.plan.Statements = append(p.plan.Statements, statement)
}

func rejectChangedCollection[T any](name string, previous, current []T) error {
	if reflect.DeepEqual(previous, current) {
		return nil
	}
	return unsupported(name + " changed")
}

func ensureCreateTableSupported(table ast.Table) error {
	key := tableKey(table)
	if table.PreviousName != "" {
		return unsupported("table " + key + " rename metadata requires semantic planning")
	}
	if table.Comment != "" {
		return unsupported("table " + key + " comments require semantic planning")
	}
	if table.RowLevelSecurity || table.ForceRLS {
		return unsupported("table " + key + " RLS requires semantic planning")
	}
	if len(table.Exclusions) > 0 {
		return unsupported("table " + key + " exclusion constraints require semantic planning")
	}
	if len(table.Indexes) > 0 {
		return unsupported("table " + key + " indexes require semantic planning")
	}
	for _, column := range table.Columns {
		if err := ensureAddColumnSupported(table, column); err != nil {
			return err
		}
	}
	for _, primaryKey := range table.PrimaryKeys {
		if primaryKey.PreviousName != "" {
			return unsupported("primary key " + key + "." + primaryKey.Name + " rename metadata requires semantic planning")
		}
	}
	for _, unique := range table.UniqueConstraints {
		if unique.PreviousName != "" {
			return unsupported("unique constraint " + key + "." + unique.Name + " rename metadata requires semantic planning")
		}
	}
	for _, foreignKey := range table.ForeignKeys {
		if foreignKey.PreviousName != "" {
			return unsupported("foreign key " + key + "." + foreignKey.Name + " rename metadata requires semantic planning")
		}
	}
	for _, check := range table.Checks {
		if check.PreviousName != "" {
			return unsupported("check constraint " + key + "." + check.Name + " rename metadata requires semantic planning")
		}
	}
	return nil
}

func ensureAddColumnSupported(table ast.Table, column ast.Column) error {
	key := tableKey(table) + "." + column.Name
	if column.PreviousName != "" {
		return unsupported("column " + key + " rename metadata requires semantic planning")
	}
	if column.Comment != "" {
		return unsupported("column " + key + " comments require semantic planning")
	}
	return nil
}

func renderCreateTable(table ast.Table) (string, error) {
	var b strings.Builder
	b.WriteString("CREATE TABLE ")
	b.WriteString(renderTableName(table))
	b.WriteString(" (\n")

	lines := make([]string, 0, len(table.Columns)+len(table.PrimaryKeys)+len(table.UniqueConstraints)+len(table.ForeignKeys)+len(table.Checks))
	for _, column := range table.Columns {
		line, err := renderColumn(column)
		if err != nil {
			return "", err
		}
		lines = append(lines, "    "+line)
	}
	for _, primaryKey := range sortedBy(table.PrimaryKeys, func(item ast.PrimaryKey) string { return item.Name }) {
		lines = append(lines, fmt.Sprintf("    CONSTRAINT %s PRIMARY KEY (%s)", primaryKey.Name, strings.Join(primaryKey.Columns, ", ")))
	}
	for _, unique := range sortedBy(table.UniqueConstraints, func(item ast.UniqueConstraint) string { return item.Name }) {
		keyword := "UNIQUE"
		if unique.NullsNotDistinct {
			keyword = "UNIQUE NULLS NOT DISTINCT"
		}
		lines = append(lines, fmt.Sprintf("    CONSTRAINT %s %s (%s)%s", unique.Name, keyword, strings.Join(unique.Columns, ", "), renderDeferrable(unique.Deferrable, unique.Initially)))
	}
	for _, foreignKey := range sortedBy(table.ForeignKeys, func(item ast.ForeignKeyConstraint) string { return item.Name }) {
		line := fmt.Sprintf("    CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)", foreignKey.Name, strings.Join(foreignKey.Columns, ", "), renderReferencedTable(foreignKey.ReferencedTable), strings.Join(foreignKey.ReferencedColumns, ", "))
		if foreignKey.OnDelete != "" {
			line += " ON DELETE " + strings.ToUpper(foreignKey.OnDelete)
		}
		if foreignKey.OnUpdate != "" {
			line += " ON UPDATE " + strings.ToUpper(foreignKey.OnUpdate)
		}
		lines = append(lines, line+renderDeferrable(foreignKey.Deferrable, foreignKey.Initially))
	}
	for _, check := range sortedBy(table.Checks, func(item ast.Check) string { return item.Name }) {
		lines = append(lines, fmt.Sprintf("    CONSTRAINT %s CHECK (%s)", check.Name, check.Expression))
	}
	b.WriteString(strings.Join(lines, ",\n"))
	b.WriteString("\n);")
	return b.String(), nil
}

func renderColumn(column ast.Column) (string, error) {
	if strings.TrimSpace(column.Type) == "" {
		return "", fmt.Errorf("column %q must have a type", column.Name)
	}
	parts := []string{column.Name, column.Type}
	if column.Identity != nil {
		parts = append(parts, renderIdentity(column.Identity))
	} else {
		if column.PrimaryKey {
			parts = append(parts, "PRIMARY KEY")
		}
		if column.NotNull {
			parts = append(parts, "NOT NULL")
		}
		if column.Unique {
			parts = append(parts, "UNIQUE")
		}
		if column.Default != "" {
			parts = append(parts, "DEFAULT", column.Default)
		}
	}
	if column.Generated != nil {
		parts = append(parts, fmt.Sprintf("GENERATED ALWAYS AS (%s) STORED", column.Generated.As))
	}
	if column.References != nil {
		parts = append(parts, fmt.Sprintf("REFERENCES %s (%s)", renderReferencedTable(column.References.Table), column.References.Column))
		if column.References.OnDelete != "" {
			parts = append(parts, "ON DELETE", strings.ToUpper(column.References.OnDelete))
		}
		if column.References.OnUpdate != "" {
			parts = append(parts, "ON UPDATE", strings.ToUpper(column.References.OnUpdate))
		}
	}
	return strings.Join(parts, " "), nil
}

func renderIdentity(identity *ast.Identity) string {
	keyword := "GENERATED ALWAYS AS IDENTITY"
	if identity.Type == "by_default" {
		keyword = "GENERATED BY DEFAULT AS IDENTITY"
	}
	var opts []string
	if identity.Name != "" {
		opts = append(opts, "SEQUENCE NAME "+identity.Name)
	}
	if identity.Increment != 0 {
		opts = append(opts, fmt.Sprintf("INCREMENT BY %d", identity.Increment))
	}
	if identity.MinValue != nil {
		opts = append(opts, fmt.Sprintf("MINVALUE %d", *identity.MinValue))
	}
	if identity.MaxValue != nil {
		opts = append(opts, fmt.Sprintf("MAXVALUE %d", *identity.MaxValue))
	}
	if identity.StartWith != nil {
		opts = append(opts, fmt.Sprintf("START WITH %d", *identity.StartWith))
	}
	if identity.Cache != nil {
		opts = append(opts, fmt.Sprintf("CACHE %d", *identity.Cache))
	}
	if identity.Cycle {
		opts = append(opts, "CYCLE")
	}
	if len(opts) > 0 {
		return keyword + " (" + strings.Join(opts, " ") + ")"
	}
	return keyword
}

func renderDeferrable(deferrable bool, initially string) string {
	if !deferrable && initially == "" {
		return ""
	}
	out := " DEFERRABLE"
	if initially != "" {
		out += " INITIALLY " + strings.ToUpper(initially)
	}
	return out
}

func renderTableName(table ast.Table) string {
	return renderQualified(table.Schema, table.Name)
}

func renderReferencedTable(table string) string {
	if strings.Contains(table, ".") {
		return table
	}
	return table
}

func renderQualified(schema, name string) string {
	if schema == "" || schema == "public" {
		return name
	}
	return schema + "." + name
}

func qualified(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}

func tableKey(table ast.Table) string {
	return qualified(table.Schema, table.Name)
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

func quoteSQL(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func unsupported(message string) error {
	return fmt.Errorf("%s; semantic planner support is not implemented yet", message)
}

func unsupportedDestructive(message string) error {
	return errors.New(message + "; destructive migration support is not implemented yet")
}
