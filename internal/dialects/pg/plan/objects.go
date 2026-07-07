package plan

import (
	"reflect"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func (p planner) roles(previous, current []pgschema.Role) error {
	prev := mapBy(previous, func(role pgschema.Role) string { return role.Name })
	for _, role := range sortedBy(current, func(item pgschema.Role) string { return item.Name }) {
		old, ok := prev[role.Name]
		if !ok {
			if role.PreviousName != "" {
				return unsupported("role " + role.Name + " rename metadata requires semantic planning")
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindRole, role.Name),
					"create role "+role.Name,
					migrateplan.SQL(renderRole(role)),
				).WithReverse(
					migrateplan.SQL("DROP ROLE " + role.Name + ";"),
				),
			)
			continue
		}
		if !reflect.DeepEqual(old, role) {
			return unsupported("role modifications require semantic planning")
		}
		delete(prev, role.Name)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("roles were removed")
	}
	return nil
}

func (p planner) namespaces(previous, current []pgschema.Namespace) error {
	prev := make(map[string]pgschema.Namespace, len(previous))
	for _, item := range previous {
		prev[item.Name] = item
	}
	for _, item := range current {
		if _, ok := prev[item.Name]; !ok {
			p.add(migrateplan.OperationCreate, migrateplan.ObjectKindSchema, item.Name, "create schema "+item.Name, "CREATE SCHEMA "+item.Name+";")
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
			p.add(migrateplan.OperationCreate, migrateplan.ObjectKindExtension, key, "create extension "+key, stmt+";")
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
			p.add(migrateplan.OperationCreate, migrateplan.ObjectKindEnum, key, "create enum "+key,
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
		p.add(migrateplan.OperationAlter, migrateplan.ObjectKindEnum, key, "add enum value "+value+" to "+key,
			"ALTER TYPE "+renderQualified(current.Schema, current.Name)+" ADD VALUE "+quoteSQL(value)+";")
	}
	return nil
}
