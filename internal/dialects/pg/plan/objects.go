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

func (p planner) compositeTypes(previous, current []pgschema.CompositeType) error {
	prev := mapBy(previous, func(item pgschema.CompositeType) string { return qualified(item.Schema, item.Name) })
	for _, compositeType := range sortedBy(current, func(item pgschema.CompositeType) string { return qualified(item.Schema, item.Name) }) {
		key := qualified(compositeType.Schema, compositeType.Name)
		old, ok := prev[key]
		if !ok {
			stmt, err := renderCompositeType(compositeType)
			if err != nil {
				return err
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindCompositeType, key),
					"create composite type "+key,
					migrateplan.SQL(stmt),
				).WithReverse(
					migrateplan.SQL("DROP TYPE " + renderQualified(compositeType.Schema, compositeType.Name) + ";"),
				),
			)
			continue
		}
		if !reflect.DeepEqual(old, compositeType) {
			return unsupported("composite type modifications require semantic planning")
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("composite types were removed")
	}
	return nil
}

func (p planner) domains(previous, current []pgschema.Domain) error {
	prev := mapBy(previous, func(item pgschema.Domain) string { return qualified(item.Schema, item.Name) })
	for _, domain := range sortedBy(current, func(item pgschema.Domain) string { return qualified(item.Schema, item.Name) }) {
		key := qualified(domain.Schema, domain.Name)
		old, ok := prev[key]
		if !ok {
			stmt, err := renderDomain(domain)
			if err != nil {
				return err
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindDomain, key),
					"create domain "+key,
					migrateplan.SQL(stmt),
				).WithReverse(
					migrateplan.SQL("DROP DOMAIN " + renderQualified(domain.Schema, domain.Name) + ";"),
				),
			)
			continue
		}
		if !reflect.DeepEqual(old, domain) {
			return unsupported("domain modifications require semantic planning")
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("domains were removed")
	}
	return nil
}

func (p planner) functions(previous, current []pgschema.Function) error {
	prev := mapBy(previous, functionKey)
	for _, function := range sortedBy(current, functionKey) {
		key := functionKey(function)
		old, ok := prev[key]
		if !ok {
			if function.PreviousName != "" {
				return unsupported("function " + key + " rename metadata requires semantic planning")
			}
			stmt, err := renderFunction(function)
			if err != nil {
				return err
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindFunction, key),
					"create function "+key,
					migrateplan.SQL(stmt),
				).WithReverse(
					migrateplan.SQL("DROP FUNCTION " + renderQualified(function.Schema, function.Name) + "(" + renderFunctionIdentityArguments(function) + ");"),
				),
			)
			if function.Comment != "" {
				p.addWith(functionCommentChange(function))
			}
			continue
		}
		oldWithoutComment, currentWithoutComment := old, function
		oldWithoutComment.Comment = ""
		currentWithoutComment.Comment = ""
		if !reflect.DeepEqual(oldWithoutComment, currentWithoutComment) {
			return unsupported("function modifications require semantic planning")
		}
		if old.Comment == "" && function.Comment != "" {
			p.addWith(functionCommentChange(function))
		} else if old.Comment != function.Comment {
			if function.Comment == "" {
				return unsupportedDestructive("function comments were removed")
			}
			return unsupported("function comment modifications require semantic planning")
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("functions were removed")
	}
	return nil
}

func functionCommentChange(function pgschema.Function) migrateplan.Change {
	key := functionKey(function)
	return migrateplan.NewChange(
		migrateplan.OperationAlter,
		migrateplan.Ref(migrateplan.ObjectKindFunction, key),
		"comment on function "+key,
		migrateplan.SQL("COMMENT ON FUNCTION "+renderQualified(function.Schema, function.Name)+"("+renderFunctionIdentityArguments(function)+") IS "+quoteSQL(function.Comment)+";"),
	).WithDependencies(
		migrateplan.Ref(migrateplan.ObjectKindFunction, key),
	).WithReverse(
		migrateplan.SQL("COMMENT ON FUNCTION " + renderQualified(function.Schema, function.Name) + "(" + renderFunctionIdentityArguments(function) + ") IS NULL;"),
	)
}

func (p planner) sequences(previous, current []pgschema.Sequence) error {
	prev := mapBy(previous, sequenceKey)
	for _, sequence := range sortedBy(current, sequenceKey) {
		key := sequenceKey(sequence)
		old, ok := prev[key]
		if !ok {
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindSequence, key),
					"create sequence "+key,
					migrateplan.SQL(renderSequence(sequence, false)),
				).WithReverse(
					migrateplan.SQL("DROP SEQUENCE " + renderQualified(sequence.Schema, sequence.Name) + ";"),
				),
			)
			continue
		}
		if !reflect.DeepEqual(old, sequence) {
			return unsupported("sequence modifications require semantic planning")
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("sequences were removed")
	}
	return nil
}

func (p planner) sequenceOwnerships(previous, current []pgschema.Sequence) error {
	prev := mapBy(previous, sequenceKey)
	for _, sequence := range sortedBy(current, sequenceKey) {
		if sequence.OwnedBy == "" {
			continue
		}
		if _, ok := prev[sequenceKey(sequence)]; ok {
			continue
		}
		change := migrateplan.NewChange(
			migrateplan.OperationAlter,
			migrateplan.Ref(migrateplan.ObjectKindSequence, sequenceKey(sequence)),
			"set ownership for sequence "+sequenceKey(sequence),
			migrateplan.SQL("ALTER SEQUENCE "+renderQualified(sequence.Schema, sequence.Name)+" OWNED BY "+sequence.OwnedBy+";"),
		).WithDependencies(
			migrateplan.Ref(migrateplan.ObjectKindSequence, sequenceKey(sequence)),
		).WithReverse(
			migrateplan.SQL("ALTER SEQUENCE " + renderQualified(sequence.Schema, sequence.Name) + " OWNED BY NONE;"),
		)
		if table, ok := sequenceOwnedByTable(sequence.OwnedBy); ok {
			change = change.WithDependencies(migrateplan.Ref(migrateplan.ObjectKindTable, referencedTableKey(table)))
		}
		p.addWith(change)
	}
	return nil
}
