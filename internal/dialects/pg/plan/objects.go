package plan

import (
	"reflect"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func (p planner) roles(previous, current []pgschema.Role) error {
	prev := mapBy(previous, func(role pgschema.Role) string { return role.Name })
	renames := roleRenameMap(prev, current)
	for _, role := range sortedBy(current, func(item pgschema.Role) string { return item.Name }) {
		old, ok := prev[role.Name]
		if !ok {
			if role.PreviousName != "" {
				oldRole, hasOld := prev[role.PreviousName]
				if hasOld {
					if err := ensureRenameOnlyRole(role, oldRole); err != nil {
						return err
					}
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationRename,
							migrateplan.Ref(migrateplan.ObjectKindRole, role.Name),
							"rename role "+role.PreviousName+" to "+role.Name,
							migrateplan.SQL(renderRenameRole(role.PreviousName, role.Name)),
						).WithReverse(
							migrateplan.SQL(renderReverseRenameRole(role.Name, role.PreviousName)),
						),
					)
					delete(prev, role.PreviousName)
					continue
				}
				return unsupported("role " + role.Name + " previousName " + role.PreviousName + " does not match any role in the previous snapshot")
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindRole, role.Name),
					"create role "+role.Name,
					migrateplan.SQL(renderRole(role)),
				).WithDependencies(
					roleDependencyRefs(role)...,
				).WithReverse(
					migrateplan.SQL("DROP ROLE " + role.Name + ";"),
				),
			)
			continue
		}
		normalizedOld := old
		normalizedOld.MemberOf = sortedStrings(applyRoleRenames(old.MemberOf, renames))
		normalizedOld.AdminOf = sortedStrings(applyRoleRenames(old.AdminOf, renames))
		normalizedCurrent := role
		normalizedCurrent.MemberOf = sortedStrings(role.MemberOf)
		normalizedCurrent.AdminOf = sortedStrings(role.AdminOf)
		if !reflect.DeepEqual(normalizedOld, normalizedCurrent) {
			return unsupported("role modifications require semantic planning")
		}
		delete(prev, role.Name)
	}
	if len(prev) > 0 {
		for _, name := range sortedStrings(removedNames(prev)) {
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindRole, name),
					"drop role "+name,
					migrateplan.SQL("DROP ROLE "+name+";"),
				).WithRisks(
					migrateplan.RiskDestructive,
				).WithReverse(
					migrateplan.SQL(renderRole(prev[name])),
				),
			)
		}
	}
	return nil
}

func roleRenameMap(prev map[string]pgschema.Role, current []pgschema.Role) map[string]string {
	renames := make(map[string]string)
	for _, role := range current {
		if role.PreviousName == "" {
			continue
		}
		if _, exists := prev[role.Name]; exists {
			continue
		}
		if _, ok := prev[role.PreviousName]; ok {
			renames[role.PreviousName] = role.Name
		}
	}
	return renames
}

func applyRoleRenames(names []string, renames map[string]string) []string {
	if len(names) == 0 || len(renames) == 0 {
		return names
	}
	out := make([]string, len(names))
	for i, name := range names {
		if renamed, ok := renames[name]; ok {
			out[i] = renamed
		} else {
			out[i] = name
		}
	}
	return out
}

func roleDependencyRefs(role pgschema.Role) []migrateplan.ObjectRef {
	refs := make([]migrateplan.ObjectRef, 0, len(role.MemberOf)+len(role.AdminOf))
	for _, memberOf := range role.MemberOf {
		if memberOf != role.Name {
			refs = append(refs, migrateplan.Ref(migrateplan.ObjectKindRole, memberOf))
		}
	}
	for _, adminOf := range role.AdminOf {
		if adminOf != role.Name {
			refs = append(refs, migrateplan.Ref(migrateplan.ObjectKindRole, adminOf))
		}
	}
	return uniqueRefs(refs)
}

func occupiedSchemas(doc pgschema.Document) map[string]bool {
	occupied := make(map[string]bool)
	mark := func(schema string) {
		if schema != "" {
			occupied[schema] = true
		}
	}
	for _, table := range doc.Tables {
		mark(table.Schema)
	}
	for _, enum := range doc.Enums {
		mark(enum.Schema)
	}
	for _, compositeType := range doc.CompositeTypes {
		mark(compositeType.Schema)
	}
	for _, domain := range doc.Domains {
		mark(domain.Schema)
	}
	for _, collation := range doc.Collations {
		mark(collation.Schema)
	}
	for _, sequence := range doc.Sequences {
		mark(sequence.Schema)
	}
	for _, function := range doc.Functions {
		mark(function.Schema)
	}
	for _, view := range doc.Views {
		mark(view.Schema)
	}
	for _, materializedView := range doc.MaterializedViews {
		mark(materializedView.Schema)
	}
	return occupied
}

func (p planner) namespaces(previous, current []pgschema.Namespace, occupied map[string]bool) error {
	prev := make(map[string]pgschema.Namespace, len(previous))
	for _, item := range previous {
		prev[item.Name] = item
	}
	for _, item := range current {
		if _, ok := prev[item.Name]; !ok {
			if item.PreviousName != "" {
				oldNS, hasOld := prev[item.PreviousName]
				if hasOld {
					if err := ensureRenameOnlyNamespace(item, oldNS); err != nil {
						return err
					}
					if occupied[item.PreviousName] {
						return unsupported("schema " + item.PreviousName + " rename to " + item.Name + " is not supported while it contained objects; ALTER SCHEMA ... RENAME moves the schema and its contents together, but those objects carry the new schema in the snapshot and would be planned as drop-and-recreate — this requires semantic planning")
					}
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationRename,
							migrateplan.Ref(migrateplan.ObjectKindSchema, item.Name),
							"rename schema "+item.PreviousName+" to "+item.Name,
							migrateplan.SQL(renderRenameSchema(item.PreviousName, item.Name)),
						).WithReverse(
							migrateplan.SQL(renderReverseRenameSchema(item.Name, item.PreviousName)),
						),
					)
					delete(prev, item.PreviousName)
					continue
				}
				return unsupported("namespace " + item.Name + " previousName " + item.PreviousName + " does not match any namespace in the previous snapshot")
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindSchema, item.Name),
					"create schema "+item.Name,
					migrateplan.SQL("CREATE SCHEMA "+item.Name+";"),
				).WithReverse(
					migrateplan.SQL("DROP SCHEMA " + item.Name + ";"),
				),
			)
			continue
		}
		delete(prev, item.Name)
	}
	if len(prev) > 0 {
		for _, name := range sortedStrings(removedNames(prev)) {
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindSchema, name),
					"drop schema "+name,
					migrateplan.SQL("DROP SCHEMA "+name+";"),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
				).WithReverse(
					migrateplan.SQL("CREATE SCHEMA " + name + ";"),
				),
			)
		}
	}
	return nil
}

func (p planner) extensions(previous, current []pgschema.Extension) error {
	prev := mapBy(previous, func(item pgschema.Extension) string { return qualified(item.Schema, item.Name) })
	for _, item := range current {
		key := qualified(item.Schema, item.Name)
		if _, ok := prev[key]; !ok {
			if item.PreviousName != "" {
				oldKey := qualified(item.Schema, item.PreviousName)
				if _, hasOld := prev[oldKey]; hasOld {
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationReplace,
							migrateplan.Ref(migrateplan.ObjectKindExtension, key),
							"replace extension "+oldKey+" with "+key,
						).WithRisks(
							migrateplan.RiskDestructive,
							migrateplan.RiskManualReview,
							migrateplan.RiskRequiresDDLReview,
						),
					)
					delete(prev, oldKey)
					continue
				}
				return unsupported("extension " + key + " previousName " + item.PreviousName + " does not match any extension in the previous snapshot")
			}
			stmt := renderCreateExtension(item)
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindExtension, key),
					"create extension "+key,
					migrateplan.SQL(stmt+";"),
				).WithReverse(
					migrateplan.SQL(renderDropExtension(item) + ";"),
				),
			)
			continue
		}
		previousItem := prev[key]
		if previousItem.Cascade != item.Cascade {
			return unsupported("extension " + key + " cascade metadata changes require manual migration authoring")
		}
		if previousItem.Version != item.Version {
			if item.Version == "" {
				return unsupported("extension " + key + " version pin removal has no automatic SQL")
			}
			change := migrateplan.NewChange(
				migrateplan.OperationAlter,
				migrateplan.Ref(migrateplan.ObjectKindExtension, key),
				"update extension "+key+" to version "+item.Version,
				migrateplan.SQL(renderAlterExtensionUpdate(item.Name, item.Version)+";"),
			).WithRisks(
				migrateplan.RiskManualReview,
				migrateplan.RiskRequiresDDLReview,
			)
			if previousItem.Version != "" {
				change = change.WithReverse(
					migrateplan.SQL(renderAlterExtensionUpdate(previousItem.Name, previousItem.Version) + ";"),
				)
			}
			p.addWith(change)
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		for _, key := range sortedStrings(removedNames(prev)) {
			extension := prev[key]
			stmt := renderCreateExtension(extension)
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindExtension, key),
					"drop extension "+key,
					migrateplan.SQL(renderDropExtension(extension)+";"),
				).WithRisks(
					migrateplan.RiskDestructive,
				).WithReverse(
					migrateplan.SQL(stmt + ";"),
				),
			)
		}
	}
	return nil
}

func renderCreateExtension(extension pgschema.Extension) string {
	stmt := "CREATE EXTENSION " + extension.Name
	if extension.Schema != "" {
		stmt += " WITH SCHEMA " + extension.Schema
	}
	if extension.Version != "" {
		stmt += " VERSION " + quoteSQL(extension.Version)
	}
	if extension.Cascade {
		stmt += " CASCADE"
	}
	return stmt
}

func renderDropExtension(extension pgschema.Extension) string {
	stmt := "DROP EXTENSION " + extension.Name
	if extension.Cascade {
		stmt += " CASCADE"
	}
	return stmt
}

func renderAlterExtensionUpdate(name, version string) string {
	return "ALTER EXTENSION " + name + " UPDATE TO " + quoteSQL(version)
}

func (p planner) collations(previous, current []pgschema.Collation) error {
	prev := mapBy(previous, func(item pgschema.Collation) string { return qualified(item.Schema, item.Name) })
	for _, collation := range sortedBy(current, func(item pgschema.Collation) string { return qualified(item.Schema, item.Name) }) {
		key := qualified(collation.Schema, collation.Name)
		old, ok := prev[key]
		if !ok {
			if collation.PreviousName != "" {
				oldCollation, hasOld := prev[qualified(collation.Schema, collation.PreviousName)]
				if hasOld {
					if err := ensureRenameOnlyCollation(collation, oldCollation); err != nil {
						return err
					}
					oldKey := qualified(collation.Schema, collation.PreviousName)
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationRename,
							migrateplan.Ref(migrateplan.ObjectKindCollation, key),
							"rename collation "+oldKey+" to "+key,
							migrateplan.SQL(renderRenameCollation(collation.Schema, collation.PreviousName, collation.Name)),
						).WithReverse(
							migrateplan.SQL(renderReverseRenameCollation(collation.Schema, collation.Name, collation.PreviousName)),
						),
					)
					delete(prev, oldKey)
					continue
				}
				return unsupported("collation " + key + " previousName " + collation.PreviousName + " does not match any collation in the previous snapshot")
			}
			stmt, err := renderCollation(collation)
			if err != nil {
				return err
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindCollation, key),
					"create collation "+key,
					migrateplan.SQL(stmt),
				).WithReverse(
					migrateplan.SQL("DROP COLLATION " + renderQualified(collation.Schema, collation.Name) + ";"),
				),
			)
			continue
		}
		if !reflect.DeepEqual(old, collation) {
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationReplace,
					migrateplan.Ref(migrateplan.ObjectKindCollation, key),
					"replace collation "+key,
				).WithRisks(
					migrateplan.RiskDestructive,
					migrateplan.RiskManualReview,
					migrateplan.RiskRequiresDDLReview,
				),
			)
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		for _, key := range sortedStrings(removedNames(prev)) {
			collation := prev[key]
			stmt, err := renderCollation(collation)
			if err != nil {
				return err
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindCollation, key),
					"drop collation "+key,
					migrateplan.SQL("DROP COLLATION "+renderQualified(collation.Schema, collation.Name)+";"),
				).WithRisks(
					migrateplan.RiskDestructive,
					migrateplan.RiskDataLoss,
				).WithReverse(
					migrateplan.SQL(stmt),
				),
			)
		}
	}
	return nil
}

func (p planner) enums(previous, current []pgschema.Enum) error {
	prev := mapBy(previous, func(item pgschema.Enum) string { return qualified(item.Schema, item.Name) })
	for _, item := range current {
		key := qualified(item.Schema, item.Name)
		old, ok := prev[key]
		if !ok {
			if item.PreviousName != "" {
				oldEnum, hasOld := prev[qualified(item.Schema, item.PreviousName)]
				if hasOld {
					if err := ensureRenameOnlyEnum(item, oldEnum); err != nil {
						return err
					}
					oldKey := qualified(item.Schema, item.PreviousName)
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationRename,
							migrateplan.Ref(migrateplan.ObjectKindEnum, key),
							"rename enum "+oldKey+" to "+key,
							migrateplan.SQL(renderRenameType(item.Schema, item.PreviousName, item.Name)),
						).WithReverse(
							migrateplan.SQL(renderReverseRenameType(item.Schema, item.Name, item.PreviousName)),
						),
					)
					delete(prev, oldKey)
					continue
				}
				return unsupported("enum " + key + " previousName " + item.PreviousName + " does not match any enum in the previous snapshot")
			}
			values := make([]string, 0, len(item.Values))
			for _, value := range item.Values {
				values = append(values, quoteSQL(value))
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindEnum, key),
					"create enum "+key,
					migrateplan.SQL("CREATE TYPE "+renderQualified(item.Schema, item.Name)+" AS ENUM ("+strings.Join(values, ", ")+");"),
				).WithReverse(
					migrateplan.SQL("DROP TYPE " + renderQualified(item.Schema, item.Name) + ";"),
				),
			)
			continue
		}
		if err := p.enumValues(old, item); err != nil {
			return err
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		for _, key := range sortedStrings(removedNames(prev)) {
			enum := prev[key]
			values := make([]string, 0, len(enum.Values))
			for _, value := range enum.Values {
				values = append(values, quoteSQL(value))
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindEnum, key),
					"drop enum "+key,
					migrateplan.SQL("DROP TYPE "+renderQualified(enum.Schema, enum.Name)+";"),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
				).WithReverse(
					migrateplan.SQL("CREATE TYPE " + renderQualified(enum.Schema, enum.Name) + " AS ENUM (" + strings.Join(values, ", ") + ");"),
				),
			)
		}
	}
	return nil
}

func (p planner) enumValues(previous, current pgschema.Enum) error {
	if len(current.Values) < len(previous.Values) {
		key := qualified(current.Schema, current.Name)
		removed := removedEnumValues(previous.Values, current.Values)
		p.addWith(
			migrateplan.NewChange(
				migrateplan.OperationReplace,
				migrateplan.Ref(migrateplan.ObjectKindEnum, key),
				"remove enum value(s) "+strings.Join(removed, ", ")+" from "+key,
			).WithRisks(
				migrateplan.RiskDestructive,
				migrateplan.RiskDataLoss,
				migrateplan.RiskManualReview,
				migrateplan.RiskRequiresDDLReview,
			),
		)
		return nil
	}
	for i, value := range previous.Values {
		if current.Values[i] != value {
			return unsupported("enum values were reordered or changed")
		}
	}
	for _, value := range current.Values[len(previous.Values):] {
		key := qualified(current.Schema, current.Name)
		p.addWith(
			migrateplan.NewChange(
				migrateplan.OperationAlter,
				migrateplan.Ref(migrateplan.ObjectKindEnum, key),
				"add enum value "+value+" to "+key,
				migrateplan.SQL("ALTER TYPE "+renderQualified(current.Schema, current.Name)+" ADD VALUE "+quoteSQL(value)+";"),
			).WithRisks(
				migrateplan.RiskManualReview,
			),
		)
	}
	return nil
}

func removedEnumValues(previous, current []string) []string {
	currentValues := make(map[string]bool, len(current))
	for _, value := range current {
		currentValues[value] = true
	}
	removed := make([]string, 0)
	for _, value := range previous {
		if !currentValues[value] {
			removed = append(removed, value)
		}
	}
	return removed
}

func (p planner) compositeTypes(previous, current []pgschema.CompositeType) error {
	prev := mapBy(previous, func(item pgschema.CompositeType) string { return qualified(item.Schema, item.Name) })
	for _, compositeType := range sortedBy(current, func(item pgschema.CompositeType) string { return qualified(item.Schema, item.Name) }) {
		key := qualified(compositeType.Schema, compositeType.Name)
		old, ok := prev[key]
		if !ok {
			if compositeType.PreviousName != "" {
				oldCT, hasOld := prev[qualified(compositeType.Schema, compositeType.PreviousName)]
				if hasOld {
					if err := ensureRenameOnlyCompositeType(compositeType, oldCT); err != nil {
						return err
					}
					oldKey := qualified(compositeType.Schema, compositeType.PreviousName)
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationRename,
							migrateplan.Ref(migrateplan.ObjectKindCompositeType, key),
							"rename composite type "+oldKey+" to "+key,
							migrateplan.SQL(renderRenameType(compositeType.Schema, compositeType.PreviousName, compositeType.Name)),
						).WithReverse(
							migrateplan.SQL(renderReverseRenameType(compositeType.Schema, compositeType.Name, compositeType.PreviousName)),
						),
					)
					delete(prev, oldKey)
					continue
				}
				return unsupported("composite type " + key + " previousName " + compositeType.PreviousName + " does not match any composite type in the previous snapshot")
			}
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
		for _, key := range sortedStrings(removedNames(prev)) {
			compositeType := prev[key]
			stmt, err := renderCompositeType(compositeType)
			if err != nil {
				return err
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindCompositeType, key),
					"drop composite type "+key,
					migrateplan.SQL("DROP TYPE "+renderQualified(compositeType.Schema, compositeType.Name)+";"),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
				).WithReverse(
					migrateplan.SQL(stmt),
				),
			)
		}
	}
	return nil
}

func (p planner) domains(previous, current []pgschema.Domain) error {
	prev := mapBy(previous, func(item pgschema.Domain) string { return qualified(item.Schema, item.Name) })
	for _, domain := range sortedBy(current, func(item pgschema.Domain) string { return qualified(item.Schema, item.Name) }) {
		key := qualified(domain.Schema, domain.Name)
		old, ok := prev[key]
		if !ok {
			if domain.PreviousName != "" {
				oldDomain, hasOld := prev[qualified(domain.Schema, domain.PreviousName)]
				if hasOld {
					if err := ensureRenameOnlyDomain(domain, oldDomain); err != nil {
						return err
					}
					oldKey := qualified(domain.Schema, domain.PreviousName)
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationRename,
							migrateplan.Ref(migrateplan.ObjectKindDomain, key),
							"rename domain "+oldKey+" to "+key,
							migrateplan.SQL(renderRenameType(domain.Schema, domain.PreviousName, domain.Name)),
						).WithReverse(
							migrateplan.SQL(renderReverseRenameType(domain.Schema, domain.Name, domain.PreviousName)),
						),
					)
					delete(prev, oldKey)
					continue
				}
				return unsupported("domain " + key + " previousName " + domain.PreviousName + " does not match any domain in the previous snapshot")
			}
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
		for _, key := range sortedStrings(removedNames(prev)) {
			domain := prev[key]
			stmt, err := renderDomain(domain)
			if err != nil {
				return err
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindDomain, key),
					"drop domain "+key,
					migrateplan.SQL("DROP DOMAIN "+renderQualified(domain.Schema, domain.Name)+";"),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
				).WithReverse(
					migrateplan.SQL(stmt),
				),
			)
		}
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
				oldFn, hasOld := prev[functionPreviousKey(function)]
				if hasOld {
					if err := ensureRenameOnlyFunction(function, oldFn); err != nil {
						return err
					}
					identity := renderFunctionIdentityArguments(function)
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationRename,
							migrateplan.Ref(migrateplan.ObjectKindFunction, key),
							"rename function "+functionPreviousKey(function)+" to "+key,
							migrateplan.SQL(renderRenameFunction(function.Schema, function.PreviousName, function.Name, identity)),
						).WithReverse(
							migrateplan.SQL(renderReverseRenameFunction(function.Schema, function.Name, function.PreviousName, identity)),
						),
					)
					delete(prev, functionPreviousKey(function))
					continue
				}
				return unsupported("function " + key + " previousName " + function.PreviousName + " does not match any function in the previous snapshot")
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
				p.addWith(
					migrateplan.NewChange(
						migrateplan.OperationAlter,
						migrateplan.Ref(migrateplan.ObjectKindFunction, key),
						"drop comment from function "+key,
						migrateplan.SQL("COMMENT ON FUNCTION "+renderQualified(function.Schema, function.Name)+"("+renderFunctionIdentityArguments(function)+") IS NULL;"),
					).WithDependencies(
						migrateplan.Ref(migrateplan.ObjectKindFunction, key),
					).WithRisks(
						migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
					).WithReverse(
						migrateplan.SQL("COMMENT ON FUNCTION " + renderQualified(function.Schema, function.Name) + "(" + renderFunctionIdentityArguments(function) + ") IS " + quoteSQL(old.Comment) + ";"),
					),
				)
			} else {
				return unsupported("function comment modifications require semantic planning")
			}
		}
		delete(prev, key)
	}
	if len(prev) > 0 {
		for _, key := range sortedStrings(removedNames(prev)) {
			function := prev[key]
			stmt, err := renderFunction(function)
			if err != nil {
				return err
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindFunction, key),
					"drop function "+key,
					migrateplan.SQL("DROP FUNCTION "+renderQualified(function.Schema, function.Name)+"("+renderFunctionIdentityArguments(function)+");"),
				).WithRisks(
					migrateplan.RiskDestructive,
				).WithReverse(
					migrateplan.SQL(stmt),
				),
			)
		}
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
			if sequence.PreviousName != "" {
				oldSeq, hasOld := prev[sequencePreviousKey(sequence)]
				if hasOld {
					if err := ensureRenameOnlySequence(sequence, oldSeq); err != nil {
						return err
					}
					oldKey := sequencePreviousKey(sequence)
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationRename,
							migrateplan.Ref(migrateplan.ObjectKindSequence, key),
							"rename sequence "+oldKey+" to "+key,
							migrateplan.SQL(renderRenameSequence(sequence.Schema, sequence.PreviousName, sequence.Name)),
						).WithReverse(
							migrateplan.SQL(renderReverseRenameSequence(sequence.Schema, sequence.Name, sequence.PreviousName)),
						),
					)
					delete(prev, oldKey)
					continue
				}
				return unsupported("sequence " + key + " previousName " + sequence.PreviousName + " does not match any sequence in the previous snapshot")
			}
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
		for _, key := range sortedStrings(removedNames(prev)) {
			sequence := prev[key]
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindSequence, key),
					"drop sequence "+key,
					migrateplan.SQL("DROP SEQUENCE "+renderQualified(sequence.Schema, sequence.Name)+";"),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
				).WithReverse(
					migrateplan.SQL(renderSequence(sequence, false)),
				),
			)
		}
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
		if sequence.PreviousName != "" {
			if _, ok := prev[sequencePreviousKey(sequence)]; ok {
				continue
			}
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

func ensureRenameOnlyRole(role, oldRole pgschema.Role) error {
	renamed := oldRole
	renamed.Name = role.Name
	renamed.PreviousName = ""
	current := role
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("role " + role.Name + " rename from " + oldRole.Name + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyNamespace(item, old pgschema.Namespace) error {
	renamed := old
	renamed.Name = item.Name
	renamed.PreviousName = ""
	current := item
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("namespace " + item.Name + " rename from " + old.Name + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyEnum(item, old pgschema.Enum) error {
	renamed := old
	renamed.Name = item.Name
	renamed.PreviousName = ""
	current := item
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("enum " + qualified(item.Schema, item.Name) + " rename from " + qualified(old.Schema, old.Name) + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyCollation(item, old pgschema.Collation) error {
	renamed := old
	renamed.Name = item.Name
	renamed.PreviousName = ""
	current := item
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("collation " + qualified(item.Schema, item.Name) + " rename from " + qualified(old.Schema, old.Name) + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyCompositeType(item, old pgschema.CompositeType) error {
	renamed := old
	renamed.Name = item.Name
	renamed.PreviousName = ""
	current := item
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("composite type " + qualified(item.Schema, item.Name) + " rename from " + qualified(old.Schema, old.Name) + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyDomain(item, old pgschema.Domain) error {
	renamed := old
	renamed.Name = item.Name
	renamed.PreviousName = ""
	current := item
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("domain " + qualified(item.Schema, item.Name) + " rename from " + qualified(old.Schema, old.Name) + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyFunction(function, old pgschema.Function) error {
	renamed := old
	renamed.Name = function.Name
	renamed.PreviousName = ""
	current := function
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("function " + functionKey(function) + " rename from " + functionPreviousKey(function) + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlySequence(sequence, old pgschema.Sequence) error {
	renamed := old
	renamed.Name = sequence.Name
	renamed.PreviousName = ""
	current := sequence
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("sequence " + sequenceKey(sequence) + " rename from " + sequencePreviousKey(sequence) + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyPolicy(policy, old pgschema.Policy) error {
	renamed := old
	renamed.Name = policy.Name
	renamed.PreviousName = ""
	current := policy
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("policy " + policyKey(policy) + " rename from " + policyPreviousKey(policy) + " combined with other modifications requires semantic planning")
	}
	return nil
}
