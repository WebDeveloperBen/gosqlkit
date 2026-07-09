package render

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
)

var identifierPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func Postgres(schema pgschema.Schema) (string, error) {
	if err := validateSchema(schema); err != nil {
		return "", err
	}

	var b strings.Builder

	tables, err := orderTables(schema.Tables)
	if err != nil {
		return "", err
	}

	roles := append([]pgschema.Role(nil), schema.Roles...)
	sort.SliceStable(roles, func(i, j int) bool {
		return roles[i].Name < roles[j].Name
	})
	for i, role := range roles {
		renderRole(&b, role)
		if i < len(roles)-1 || len(schema.Namespaces) > 0 || len(schema.Extensions) > 0 || len(schema.Enums) > 0 || len(schema.CompositeTypes) > 0 || len(schema.Domains) > 0 || len(schema.Sequences) > 0 || len(schema.Functions) > 0 || len(tables) > 0 || len(schema.Policies) > 0 || len(schema.Views) > 0 || len(schema.MaterializedViews) > 0 || len(schema.Triggers) > 0 {
			b.WriteString("\n")
		}
	}

	namespaces := append([]pgschema.Namespace(nil), schema.Namespaces...)
	sort.SliceStable(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})
	for i, namespace := range namespaces {
		b.WriteString("CREATE SCHEMA ")
		b.WriteString(namespace.Name)
		b.WriteString(";\n")
		if i < len(namespaces)-1 || len(schema.Extensions) > 0 || len(schema.Enums) > 0 || len(schema.CompositeTypes) > 0 || len(schema.Domains) > 0 || len(schema.Sequences) > 0 || len(schema.Functions) > 0 || len(tables) > 0 || len(schema.Policies) > 0 || len(schema.Views) > 0 || len(schema.MaterializedViews) > 0 || len(schema.Triggers) > 0 {
			b.WriteString("\n")
		}
	}

	extensions := append([]pgschema.Extension(nil), schema.Extensions...)
	sort.SliceStable(extensions, func(i, j int) bool {
		return qualifiedName(extensions[i].Schema, extensions[i].Name) < qualifiedName(extensions[j].Schema, extensions[j].Name)
	})
	for i, extension := range extensions {
		renderExtension(&b, extension)
		if i < len(extensions)-1 || len(schema.Enums) > 0 || len(schema.CompositeTypes) > 0 || len(schema.Domains) > 0 || len(schema.Sequences) > 0 || len(schema.Functions) > 0 || len(tables) > 0 || len(schema.Policies) > 0 || len(schema.Views) > 0 || len(schema.MaterializedViews) > 0 || len(schema.Triggers) > 0 {
			b.WriteString("\n")
		}
	}

	enums := append([]pgschema.Enum(nil), schema.Enums...)
	sort.SliceStable(enums, func(i, j int) bool {
		return qualifiedName(enums[i].Schema, enums[i].Name) < qualifiedName(enums[j].Schema, enums[j].Name)
	})
	for i, enum := range enums {
		renderEnum(&b, enum)
		if i < len(enums)-1 || len(schema.CompositeTypes) > 0 || len(schema.Domains) > 0 || len(schema.Sequences) > 0 || len(schema.Functions) > 0 || len(tables) > 0 || len(schema.Policies) > 0 || len(schema.Views) > 0 || len(schema.MaterializedViews) > 0 || len(schema.Triggers) > 0 {
			b.WriteString("\n")
		}
	}

	compositeTypes := append([]pgschema.CompositeType(nil), schema.CompositeTypes...)
	sort.SliceStable(compositeTypes, func(i, j int) bool {
		return qualifiedName(compositeTypes[i].Schema, compositeTypes[i].Name) < qualifiedName(compositeTypes[j].Schema, compositeTypes[j].Name)
	})
	for i, compositeType := range compositeTypes {
		if err := renderCompositeType(&b, compositeType); err != nil {
			return "", err
		}
		if i < len(compositeTypes)-1 || len(schema.Domains) > 0 || len(schema.Sequences) > 0 || len(schema.Functions) > 0 || len(tables) > 0 || len(schema.Policies) > 0 || len(schema.Views) > 0 || len(schema.MaterializedViews) > 0 || len(schema.Triggers) > 0 {
			b.WriteString("\n")
		}
	}

	domains := append([]pgschema.Domain(nil), schema.Domains...)
	sort.SliceStable(domains, func(i, j int) bool {
		return qualifiedName(domains[i].Schema, domains[i].Name) < qualifiedName(domains[j].Schema, domains[j].Name)
	})
	for i, domain := range domains {
		if err := renderDomain(&b, domain); err != nil {
			return "", err
		}
		if i < len(domains)-1 || len(schema.Sequences) > 0 || len(schema.Functions) > 0 || len(tables) > 0 || len(schema.Policies) > 0 || len(schema.Views) > 0 || len(schema.MaterializedViews) > 0 || len(schema.Triggers) > 0 {
			b.WriteString("\n")
		}
	}

	sequences := append([]pgschema.Sequence(nil), schema.Sequences...)
	sort.SliceStable(sequences, func(i, j int) bool {
		return qualifiedName(sequences[i].Schema, sequences[i].Name) < qualifiedName(sequences[j].Schema, sequences[j].Name)
	})
	for i, sequence := range sequences {
		renderSequence(&b, sequence)
		if i < len(sequences)-1 || len(schema.Functions) > 0 || len(tables) > 0 || len(schema.Policies) > 0 || len(schema.Views) > 0 || len(schema.MaterializedViews) > 0 || len(schema.Triggers) > 0 {
			b.WriteString("\n")
		}
	}

	functions := append([]pgschema.Function(nil), schema.Functions...)
	sort.SliceStable(functions, func(i, j int) bool {
		return functionKey(functions[i]) < functionKey(functions[j])
	})
	for i, function := range functions {
		renderFunction(&b, function)
		renderFunctionComment(&b, function)
		if i < len(functions)-1 || len(tables) > 0 || len(schema.Policies) > 0 || len(schema.Views) > 0 || len(schema.MaterializedViews) > 0 || len(schema.Triggers) > 0 {
			b.WriteString("\n")
		}
	}

	hasPostTableObjects := len(schema.Policies) > 0 || len(schema.Views) > 0 || len(schema.MaterializedViews) > 0 || len(schema.Triggers) > 0
	for i, table := range tables {
		if err := renderTable(&b, table); err != nil {
			return "", err
		}
		if i < len(tables)-1 || len(table.Indexes) > 0 || hasComments(table) || hasRLS(table) || (i == len(tables)-1 && hasPostTableObjects) {
			b.WriteString("\n")
		}

		indexes := append([]pgschema.Index(nil), table.Indexes...)
		sort.SliceStable(indexes, func(i, j int) bool {
			return indexes[i].Name < indexes[j].Name
		})
		for j, index := range indexes {
			if err := renderIndex(&b, renderTableName(table), index); err != nil {
				return "", err
			}
			if j < len(indexes)-1 || i < len(tables)-1 || hasComments(table) || hasRLS(table) || (i == len(tables)-1 && hasPostTableObjects) {
				b.WriteString("\n")
			}
		}

		if err := renderComments(&b, table); err != nil {
			return "", err
		}
		if hasComments(table) && (i < len(tables)-1 || hasRLS(table) || (i == len(tables)-1 && hasPostTableObjects)) {
			b.WriteString("\n")
		}

		renderTableRLS(&b, table)
		if hasRLS(table) && (i < len(tables)-1 || (i == len(tables)-1 && hasPostTableObjects)) {
			b.WriteString("\n")
		}
	}

	policies := append([]pgschema.Policy(nil), schema.Policies...)
	sort.SliceStable(policies, func(i, j int) bool {
		return policyKey(policies[i]) < policyKey(policies[j])
	})
	for i, policy := range policies {
		renderPolicy(&b, policy)
		if i < len(policies)-1 || len(schema.Views) > 0 || len(schema.MaterializedViews) > 0 || len(schema.Triggers) > 0 {
			b.WriteString("\n")
		}
	}

	hasViews := len(schema.Views) > 0 || len(schema.MaterializedViews) > 0 || len(schema.Triggers) > 0
	lastTableHasOutput := len(tables) > 0 && (hasComments(tables[len(tables)-1]) || len(tables[len(tables)-1].Indexes) > 0 || hasRLS(tables[len(tables)-1]))
	if len(policies) == 0 && hasViews && lastTableHasOutput {
		b.WriteString("\n")
	}

	views := append([]pgschema.View(nil), schema.Views...)
	sort.SliceStable(views, func(i, j int) bool {
		return qualifiedName(views[i].Schema, views[i].Name) < qualifiedName(views[j].Schema, views[j].Name)
	})
	for i, view := range views {
		renderView(&b, view)
		renderViewComment(&b, view)
		if i < len(views)-1 || len(schema.MaterializedViews) > 0 || len(schema.Triggers) > 0 {
			b.WriteString("\n")
		}
	}

	materializedViews := append([]pgschema.MaterializedView(nil), schema.MaterializedViews...)
	sort.SliceStable(materializedViews, func(i, j int) bool {
		return qualifiedName(materializedViews[i].Schema, materializedViews[i].Name) < qualifiedName(materializedViews[j].Schema, materializedViews[j].Name)
	})
	for i, mv := range materializedViews {
		renderMaterializedView(&b, mv)
		renderMaterializedViewComment(&b, mv)
		if i < len(materializedViews)-1 || len(schema.Triggers) > 0 {
			b.WriteString("\n")
		}
	}

	triggers := append([]pgschema.Trigger(nil), schema.Triggers...)
	sort.SliceStable(triggers, func(i, j int) bool {
		return triggerKey(triggers[i]) < triggerKey(triggers[j])
	})
	for i, trigger := range triggers {
		renderTrigger(&b, trigger)
		renderTriggerComment(&b, trigger)
		if i < len(triggers)-1 {
			b.WriteString("\n")
		}
	}

	return b.String(), nil
}

func orderTables(input []pgschema.Table) ([]pgschema.Table, error) {
	tables := append([]pgschema.Table(nil), input...)
	sort.SliceStable(tables, func(i, j int) bool {
		return tableKey(tables[i]) < tableKey(tables[j])
	})

	byName := make(map[string]pgschema.Table, len(tables))
	for _, table := range tables {
		byName[tableKey(table)] = table
	}

	visiting := map[string]bool{}
	visited := map[string]bool{}
	ordered := make([]pgschema.Table, 0, len(tables))

	var visit func(table pgschema.Table) error
	visit = func(table pgschema.Table) error {
		key := tableKey(table)
		if visited[key] {
			return nil
		}
		if visiting[key] {
			return fmt.Errorf("table %q has a foreign key cycle; cyclic foreign keys are not supported in CREATE TABLE output yet", renderTableName(table))
		}

		visiting[key] = true
		for _, dependency := range tableDependencies(table) {
			depTable, ok := byName[dependency]
			if !ok {
				continue
			}
			if err := visit(depTable); err != nil {
				return err
			}
		}
		visiting[key] = false
		visited[key] = true
		ordered = append(ordered, table)
		return nil
	}

	for _, table := range tables {
		if err := visit(table); err != nil {
			return nil, err
		}
	}

	return ordered, nil
}

func tableDependencies(table pgschema.Table) []string {
	deps := map[string]struct{}{}
	if table.PartitionOf != nil {
		if dependency, err := referenceKey(table.Schema, table.PartitionOf.Parent); err == nil && dependency != tableKey(table) {
			deps[dependency] = struct{}{}
		}
	}
	for _, column := range table.Columns {
		if column.References != nil {
			dependency, err := referenceKey(table.Schema, column.References.Table)
			if err == nil && dependency != tableKey(table) {
				deps[dependency] = struct{}{}
			}
		}
	}
	for _, foreignKey := range table.ForeignKeys {
		dependency, err := referenceKey(table.Schema, foreignKey.ReferencedTable)
		if err == nil && dependency != tableKey(table) {
			deps[dependency] = struct{}{}
		}
	}

	names := make([]string, 0, len(deps))
	for name := range deps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func validateSchema(schema pgschema.Schema) error {
	roleNames := map[string]struct{}{}
	for _, role := range schema.Roles {
		if err := validateRole(role, roleNames); err != nil {
			return err
		}
	}

	namespaceNames := map[string]struct{}{}
	for _, namespace := range schema.Namespaces {
		if err := validateIdentifier("schema", namespace.Name); err != nil {
			return err
		}
		if _, ok := namespaceNames[namespace.Name]; ok {
			return fmt.Errorf("duplicate schema %q", namespace.Name)
		}
		namespaceNames[namespace.Name] = struct{}{}
	}

	extensionNames := map[string]struct{}{}
	for _, extension := range schema.Extensions {
		if extension.Schema != "" {
			if err := validateIdentifier("extension schema", extension.Schema); err != nil {
				return err
			}
		}
		if err := validateExtensionName(extension.Name); err != nil {
			return err
		}
		key := qualifiedName(extension.Schema, extension.Name)
		if _, ok := extensionNames[key]; ok {
			return fmt.Errorf("duplicate extension %q", renderQualifiedName(extension.Schema, extension.Name))
		}
		extensionNames[key] = struct{}{}
	}

	enumNames := map[string]struct{}{}
	for _, enum := range schema.Enums {
		if enum.Schema != "" {
			if err := validateIdentifier("enum schema", enum.Schema); err != nil {
				return err
			}
		}
		if err := validateIdentifier("enum", enum.Name); err != nil {
			return err
		}
		if len(enum.Values) == 0 {
			return fmt.Errorf("enum %q must have at least one value", renderQualifiedName(enum.Schema, enum.Name))
		}
		seenValues := map[string]struct{}{}
		for _, value := range enum.Values {
			if value == "" {
				return fmt.Errorf("enum %q has an empty value", renderQualifiedName(enum.Schema, enum.Name))
			}
			if _, ok := seenValues[value]; ok {
				return fmt.Errorf("enum %q has duplicate value %q", renderQualifiedName(enum.Schema, enum.Name), value)
			}
			seenValues[value] = struct{}{}
		}
		key := qualifiedName(enum.Schema, enum.Name)
		if _, ok := enumNames[key]; ok {
			return fmt.Errorf("duplicate enum %q", renderQualifiedName(enum.Schema, enum.Name))
		}
		enumNames[key] = struct{}{}
	}

	sequenceNames := map[string]struct{}{}
	for _, sequence := range schema.Sequences {
		if sequence.Schema != "" {
			if err := validateIdentifier("sequence schema", sequence.Schema); err != nil {
				return err
			}
		}
		if err := validateIdentifier("sequence", sequence.Name); err != nil {
			return err
		}
		key := qualifiedName(sequence.Schema, sequence.Name)
		if _, ok := sequenceNames[key]; ok {
			return fmt.Errorf("duplicate sequence %q", renderQualifiedName(sequence.Schema, sequence.Name))
		}
		sequenceNames[key] = struct{}{}
		if sequence.OwnedBy != "" {
			parts := strings.SplitSeq(sequence.OwnedBy, ".")
			for part := range parts {
				if err := validateIdentifier("owned by", part); err != nil {
					return fmt.Errorf("sequence %q: %w", renderQualifiedName(sequence.Schema, sequence.Name), err)
				}
			}
		}
	}

	compositeTypeNames := map[string]struct{}{}
	for _, compositeType := range schema.CompositeTypes {
		if compositeType.Schema != "" {
			if err := validateIdentifier("composite type schema", compositeType.Schema); err != nil {
				return err
			}
		}
		if err := validateIdentifier("composite type", compositeType.Name); err != nil {
			return err
		}
		key := qualifiedName(compositeType.Schema, compositeType.Name)
		if _, ok := compositeTypeNames[key]; ok {
			return fmt.Errorf("duplicate composite type %q", renderQualifiedName(compositeType.Schema, compositeType.Name))
		}
		compositeTypeNames[key] = struct{}{}
		if len(compositeType.Attributes) == 0 {
			return fmt.Errorf("composite type %q must have at least one attribute", renderQualifiedName(compositeType.Schema, compositeType.Name))
		}
		seenAttrs := map[string]struct{}{}
		for _, attribute := range compositeType.Attributes {
			if err := validateIdentifier("composite attribute", attribute.Name); err != nil {
				return fmt.Errorf("composite type %q: %w", compositeType.Name, err)
			}
			if _, ok := seenAttrs[attribute.Name]; ok {
				return fmt.Errorf("composite type %q has duplicate attribute %q", compositeType.Name, attribute.Name)
			}
			seenAttrs[attribute.Name] = struct{}{}
			if strings.TrimSpace(attribute.Type) == "" {
				return fmt.Errorf("composite type %q attribute %q must have a type", compositeType.Name, attribute.Name)
			}
		}
	}

	domainNames := map[string]struct{}{}
	for _, domain := range schema.Domains {
		if domain.Schema != "" {
			if err := validateIdentifier("domain schema", domain.Schema); err != nil {
				return err
			}
		}
		if err := validateIdentifier("domain", domain.Name); err != nil {
			return err
		}
		key := qualifiedName(domain.Schema, domain.Name)
		if _, ok := domainNames[key]; ok {
			return fmt.Errorf("duplicate domain %q", renderQualifiedName(domain.Schema, domain.Name))
		}
		domainNames[key] = struct{}{}
		if strings.TrimSpace(domain.BaseType) == "" {
			return fmt.Errorf("domain %q must have a base type", renderQualifiedName(domain.Schema, domain.Name))
		}
		if domain.Default != "" {
			if err := validateExpression(domain.Default, "default expression"); err != nil {
				return fmt.Errorf("domain %q: %w", renderQualifiedName(domain.Schema, domain.Name), err)
			}
		}
		if domain.Check != "" && strings.TrimSpace(domain.Check) != domain.Check {
			return fmt.Errorf("domain %q check expression must not have leading or trailing whitespace", renderQualifiedName(domain.Schema, domain.Name))
		}
		if domain.Check != "" {
			if err := validateExpression(domain.Check, "check expression"); err != nil {
				return fmt.Errorf("domain %q: %w", renderQualifiedName(domain.Schema, domain.Name), err)
			}
		}
	}

	functionNames := map[string]struct{}{}
	for _, function := range schema.Functions {
		if err := validateFunction(function, functionNames); err != nil {
			return err
		}
	}

	viewNames := map[string]struct{}{}
	for _, view := range schema.Views {
		if view.Schema != "" {
			if err := validateIdentifier("view schema", view.Schema); err != nil {
				return err
			}
		}
		if err := validateIdentifier("view", view.Name); err != nil {
			return err
		}
		key := qualifiedName(view.Schema, view.Name)
		if _, ok := viewNames[key]; ok {
			return fmt.Errorf("duplicate view %q", renderQualifiedName(view.Schema, view.Name))
		}
		viewNames[key] = struct{}{}
		if strings.TrimSpace(view.Query) == "" {
			return fmt.Errorf("view %q must have a query", renderQualifiedName(view.Schema, view.Name))
		}
		if err := validateViewQuery(view.Query); err != nil {
			return fmt.Errorf("view %q: %w", renderQualifiedName(view.Schema, view.Name), err)
		}
		if view.CheckOption != "" {
			switch strings.ToUpper(view.CheckOption) {
			case "LOCAL", "CASCADED":
			default:
				return fmt.Errorf("view %q check option must be LOCAL or CASCADED, got %q", renderQualifiedName(view.Schema, view.Name), view.CheckOption)
			}
		}
		for _, alias := range view.ColumnAliases {
			if err := validateIdentifier("view column alias", alias); err != nil {
				return fmt.Errorf("view %q: %w", renderQualifiedName(view.Schema, view.Name), err)
			}
		}
	}

	materializedViewNames := map[string]struct{}{}
	for _, mv := range schema.MaterializedViews {
		if mv.Schema != "" {
			if err := validateIdentifier("materialized view schema", mv.Schema); err != nil {
				return err
			}
		}
		if err := validateIdentifier("materialized view", mv.Name); err != nil {
			return err
		}
		key := qualifiedName(mv.Schema, mv.Name)
		if _, ok := materializedViewNames[key]; ok {
			return fmt.Errorf("duplicate materialized view %q", renderQualifiedName(mv.Schema, mv.Name))
		}
		materializedViewNames[key] = struct{}{}
		if strings.TrimSpace(mv.Query) == "" {
			return fmt.Errorf("materialized view %q must have a query", renderQualifiedName(mv.Schema, mv.Name))
		}
		for _, alias := range mv.ColumnAliases {
			if err := validateIdentifier("materialized view column alias", alias); err != nil {
				return fmt.Errorf("materialized view %q: %w", renderQualifiedName(mv.Schema, mv.Name), err)
			}
		}
		for key := range mv.With {
			if err := validateIdentifier("materialized view storage parameter", key); err != nil {
				return fmt.Errorf("materialized view %q: %w", renderQualifiedName(mv.Schema, mv.Name), err)
			}
			if strings.TrimSpace(mv.With[key]) == "" {
				return fmt.Errorf("materialized view %q storage parameter %q has an empty value", renderQualifiedName(mv.Schema, mv.Name), key)
			}
		}
	}

	tableByKey := make(map[string]pgschema.Table, len(schema.Tables))
	for _, table := range schema.Tables {
		tableByKey[tableKey(table)] = table
	}

	tableNames := map[string]struct{}{}
	tableColumns := map[string]map[string]struct{}{}
	indexNames := map[string]struct{}{}

	for _, table := range schema.Tables {
		if table.Schema != "" {
			if err := validateIdentifier("schema", table.Schema); err != nil {
				return fmt.Errorf("table %q: %w", table.Name, err)
			}
		}
		if err := validateIdentifier("table", table.Name); err != nil {
			return err
		}
		if _, ok := tableNames[tableKey(table)]; ok {
			return fmt.Errorf("duplicate table %q", renderTableName(table))
		}
		tableNames[tableKey(table)] = struct{}{}

		columnNames := map[string]struct{}{}
		hasPrimaryKey := false
		for _, column := range table.Columns {
			if err := validateIdentifier("column", column.Name); err != nil {
				return fmt.Errorf("table %q: %w", table.Name, err)
			}
			if _, ok := columnNames[column.Name]; ok {
				return fmt.Errorf("table %q has duplicate column %q", table.Name, column.Name)
			}
			columnNames[column.Name] = struct{}{}

			if column.PrimaryKey {
				if hasPrimaryKey {
					return fmt.Errorf("table %q has multiple primary keys", table.Name)
				}
				hasPrimaryKey = true
			}

			if column.References != nil {
				if _, err := parseQualifiedIdentifier("referenced table", column.References.Table); err != nil {
					return fmt.Errorf("table %q column %q: %w", renderTableName(table), column.Name, err)
				}
				if err := validateForeignKeyAction("ON DELETE", column.References.OnDelete); err != nil {
					return fmt.Errorf("table %q column %q: %w", renderTableName(table), column.Name, err)
				}
				if err := validateForeignKeyAction("ON UPDATE", column.References.OnUpdate); err != nil {
					return fmt.Errorf("table %q column %q: %w", renderTableName(table), column.Name, err)
				}
			}
			if column.Identity != nil {
				if err := validateIdentity(column); err != nil {
					return fmt.Errorf("table %q column %q: %w", table.Name, column.Name, err)
				}
			}
			if column.Generated != nil {
				if err := validateGenerated(column); err != nil {
					return fmt.Errorf("table %q column %q: %w", table.Name, column.Name, err)
				}
			}
			if column.Default != "" {
				if err := validateDefault(column); err != nil {
					return fmt.Errorf("table %q column %q: %w", table.Name, column.Name, err)
				}
			}
		}
		tableColumns[tableKey(table)] = columnNames

		constraintNames := map[string]struct{}{}
		for _, primaryKey := range table.PrimaryKeys {
			if hasPrimaryKey {
				return fmt.Errorf("table %q has multiple primary keys", table.Name)
			}
			hasPrimaryKey = true
			if err := validateNamedColumnConstraint(table.Name, "primary key", primaryKey.Name, primaryKey.Columns, columnNames); err != nil {
				return err
			}
			if err := addConstraintName(table.Name, constraintNames, primaryKey.Name); err != nil {
				return err
			}
		}

		for _, unique := range table.UniqueConstraints {
			if err := validateNamedColumnConstraint(table.Name, "unique constraint", unique.Name, unique.Columns, columnNames); err != nil {
				return err
			}
			if err := validateInitially(unique.Initially); err != nil {
				return fmt.Errorf("table %q unique constraint %q: %w", table.Name, unique.Name, err)
			}
			if err := addConstraintName(table.Name, constraintNames, unique.Name); err != nil {
				return err
			}
		}

		for _, foreignKey := range table.ForeignKeys {
			if err := validateNamedColumnConstraint(table.Name, "foreign key", foreignKey.Name, foreignKey.Columns, columnNames); err != nil {
				return err
			}
			if len(foreignKey.ReferencedColumns) == 0 {
				return fmt.Errorf("table %q foreign key %q must reference at least one column", table.Name, foreignKey.Name)
			}
			if len(foreignKey.Columns) != len(foreignKey.ReferencedColumns) {
				return fmt.Errorf("table %q foreign key %q has %d local columns but %d referenced columns", table.Name, foreignKey.Name, len(foreignKey.Columns), len(foreignKey.ReferencedColumns))
			}
			if _, err := parseQualifiedIdentifier("referenced table", foreignKey.ReferencedTable); err != nil {
				return fmt.Errorf("table %q foreign key %q: %w", table.Name, foreignKey.Name, err)
			}
			for _, column := range foreignKey.ReferencedColumns {
				if err := validateIdentifier("referenced column", column); err != nil {
					return fmt.Errorf("table %q foreign key %q: %w", table.Name, foreignKey.Name, err)
				}
			}
			if err := validateForeignKeyAction("ON DELETE", foreignKey.OnDelete); err != nil {
				return fmt.Errorf("table %q foreign key %q: %w", table.Name, foreignKey.Name, err)
			}
			if err := validateForeignKeyAction("ON UPDATE", foreignKey.OnUpdate); err != nil {
				return fmt.Errorf("table %q foreign key %q: %w", table.Name, foreignKey.Name, err)
			}
			if err := validateInitially(foreignKey.Initially); err != nil {
				return fmt.Errorf("table %q foreign key %q: %w", table.Name, foreignKey.Name, err)
			}
			if err := addConstraintName(table.Name, constraintNames, foreignKey.Name); err != nil {
				return err
			}
		}

		for _, check := range table.Checks {
			if err := validateCheck(table.Name, check); err != nil {
				return err
			}
			if err := addConstraintName(table.Name, constraintNames, check.Name); err != nil {
				return err
			}
		}

		for _, exclusion := range table.Exclusions {
			if err := validateExclusion(exclusion); err != nil {
				return err
			}
			if err := addConstraintName(table.Name, constraintNames, exclusion.Name); err != nil {
				return err
			}
		}

		if table.PartitionOf != nil {
			if err := validatePartitionChildShape(table); err != nil {
				return err
			}
		} else {
			for _, index := range table.Indexes {
				key := indexKey(table, index)
				if _, ok := indexNames[key]; ok {
					return fmt.Errorf("duplicate index %q in schema %q", index.Name, tableSchema(table))
				}
				indexNames[key] = struct{}{}
				if err := validateIndex(table.Name, index, columnNames); err != nil {
					return err
				}
			}
		}

		if table.Partitioning != nil {
			if err := validatePartitioning(table, columnNames); err != nil {
				return err
			}
		}
	}
	for _, table := range schema.Tables {
		if table.PartitionOf == nil {
			continue
		}
		parent, err := validatePartitionOf(table, *table.PartitionOf, tableByKey)
		if err != nil {
			return err
		}
		parentColumns := tableColumns[tableKey(parent)]
		for _, index := range table.Indexes {
			key := indexKey(table, index)
			if _, ok := indexNames[key]; ok {
				return fmt.Errorf("duplicate index %q in schema %q", index.Name, tableSchema(table))
			}
			indexNames[key] = struct{}{}
			if err := validateIndex(table.Name, index, parentColumns); err != nil {
				return err
			}
		}
	}

	triggerNames := map[string]struct{}{}
	for _, trigger := range schema.Triggers {
		if err := validateTrigger(trigger, triggerNames, tableNames, tableColumns, viewNames); err != nil {
			return err
		}
	}

	policyNames := map[string]struct{}{}
	for _, policy := range schema.Policies {
		if err := validatePolicy(policy, policyNames, tableNames); err != nil {
			return err
		}
	}

	return validateReferences(schema.Tables, tableColumns)
}

func renderRole(b *strings.Builder, role pgschema.Role) {
	b.WriteString("CREATE ROLE ")
	b.WriteString(role.Name)

	parts := []string{}
	parts = appendBoolRoleOption(parts, role.Superuser, "SUPERUSER", "NOSUPERUSER")
	parts = appendBoolRoleOption(parts, role.CreateDB, "CREATEDB", "NOCREATEDB")
	parts = appendBoolRoleOption(parts, role.CreateRole, "CREATEROLE", "NOCREATEROLE")
	parts = appendBoolRoleOption(parts, role.Inherit, "INHERIT", "NOINHERIT")
	parts = appendBoolRoleOption(parts, role.Login, "LOGIN", "NOLOGIN")
	parts = appendBoolRoleOption(parts, role.Replication, "REPLICATION", "NOREPLICATION")
	parts = appendBoolRoleOption(parts, role.BypassRLS, "BYPASSRLS", "NOBYPASSRLS")
	if role.ConnectionLimit != nil {
		parts = append(parts, fmt.Sprintf("CONNECTION LIMIT %d", *role.ConnectionLimit))
	}
	if role.ValidUntil != "" {
		parts = append(parts, "VALID UNTIL "+quoteLiteral(role.ValidUntil))
	}
	if len(role.MemberOf) > 0 {
		parts = append(parts, "IN ROLE "+strings.Join(sortedStrings(role.MemberOf), ", "))
	}
	if len(role.AdminOf) > 0 {
		parts = append(parts, "ADMIN "+strings.Join(sortedStrings(role.AdminOf), ", "))
	}
	if len(parts) > 0 {
		b.WriteString(" WITH ")
		b.WriteString(strings.Join(parts, " "))
	}
	b.WriteString(";\n")
}

func appendBoolRoleOption(parts []string, value *bool, on, off string) []string {
	if value == nil {
		return parts
	}
	if *value {
		return append(parts, on)
	}
	return append(parts, off)
}

func renderExtension(b *strings.Builder, extension pgschema.Extension) {
	b.WriteString("CREATE EXTENSION IF NOT EXISTS ")
	b.WriteString(quoteIdentifier(extension.Name))
	if extension.Schema != "" {
		b.WriteString(" WITH SCHEMA ")
		b.WriteString(extension.Schema)
	}
	b.WriteString(";\n")
}

func renderEnum(b *strings.Builder, enum pgschema.Enum) {
	b.WriteString("CREATE TYPE ")
	b.WriteString(renderQualifiedName(enum.Schema, enum.Name))
	b.WriteString(" AS ENUM (")
	values := make([]string, 0, len(enum.Values))
	for _, value := range enum.Values {
		values = append(values, quoteLiteral(value))
	}
	b.WriteString(strings.Join(values, ", "))
	b.WriteString(");\n")
}

func renderSequence(b *strings.Builder, sequence pgschema.Sequence) {
	b.WriteString("CREATE SEQUENCE ")
	b.WriteString(renderQualifiedName(sequence.Schema, sequence.Name))
	if sequence.Increment != 0 {
		fmt.Fprintf(b, " INCREMENT %d", sequence.Increment)
	}
	if sequence.MinValue != nil {
		fmt.Fprintf(b, " MINVALUE %d", *sequence.MinValue)
	}
	if sequence.MaxValue != nil {
		fmt.Fprintf(b, " MAXVALUE %d", *sequence.MaxValue)
	}
	if sequence.StartWith != nil {
		fmt.Fprintf(b, " START %d", *sequence.StartWith)
	}
	if sequence.Cache != nil {
		fmt.Fprintf(b, " CACHE %d", *sequence.Cache)
	}
	if sequence.Cycle {
		b.WriteString(" CYCLE")
	}
	if sequence.OwnedBy != "" {
		b.WriteString(" OWNED BY ")
		b.WriteString(sequence.OwnedBy)
	}
	b.WriteString(";\n")
}

func renderFunction(b *strings.Builder, function pgschema.Function) {
	b.WriteString("CREATE FUNCTION ")
	b.WriteString(renderQualifiedName(function.Schema, function.Name))
	b.WriteString("(")
	args := make([]string, 0, len(function.Arguments))
	for _, arg := range function.Arguments {
		args = append(args, renderFunctionArgument(arg))
	}
	b.WriteString(strings.Join(args, ", "))
	b.WriteString(")\nRETURNS ")
	b.WriteString(function.ReturnType)
	b.WriteString("\nLANGUAGE ")
	b.WriteString(function.Language)
	if function.Volatility != "" {
		b.WriteString("\n")
		b.WriteString(strings.ToUpper(function.Volatility))
	}
	if function.Strict != nil {
		if *function.Strict {
			b.WriteString("\nSTRICT")
		} else {
			b.WriteString("\nCALLED ON NULL INPUT")
		}
	}
	if function.SecurityDefiner {
		b.WriteString("\nSECURITY DEFINER")
	}
	if function.Parallel != "" {
		b.WriteString("\nPARALLEL ")
		b.WriteString(strings.ToUpper(function.Parallel))
	}
	if function.Cost != nil {
		b.WriteString("\nCOST ")
		fmt.Fprintf(b, "%g", *function.Cost)
	}
	if function.Rows != nil {
		fmt.Fprintf(b, "\nROWS %d", *function.Rows)
	}
	for _, setting := range renderFunctionConfiguration(function.Configuration) {
		b.WriteString("\nSET ")
		b.WriteString(setting)
	}
	b.WriteString("\nAS $$\n")
	b.WriteString(function.Body)
	b.WriteString("\n$$;\n")
}

func renderFunctionArgument(arg pgschema.FunctionArgument) string {
	parts := []string{}
	if arg.Mode != "" {
		parts = append(parts, strings.ToUpper(arg.Mode))
	}
	if arg.Name != "" {
		parts = append(parts, arg.Name)
	}
	parts = append(parts, arg.Type)
	if arg.Default != "" {
		parts = append(parts, "DEFAULT", arg.Default)
	}
	return strings.Join(parts, " ")
}

func renderFunctionConfiguration(config map[string]string) []string {
	if len(config) == 0 {
		return nil
	}
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	settings := make([]string, 0, len(keys))
	for _, key := range keys {
		settings = append(settings, key+" = "+config[key])
	}
	return settings
}

func renderFunctionComment(b *strings.Builder, function pgschema.Function) {
	if function.Comment != "" {
		fmt.Fprintf(b, "COMMENT ON FUNCTION %s(%s) IS %s;\n", renderQualifiedName(function.Schema, function.Name), renderFunctionIdentityArguments(function), quoteLiteral(function.Comment))
	}
}

func renderTrigger(b *strings.Builder, trigger pgschema.Trigger) {
	b.WriteString("CREATE ")
	if trigger.Constraint {
		b.WriteString("CONSTRAINT ")
	}
	b.WriteString("TRIGGER ")
	b.WriteString(trigger.Name)
	b.WriteString("\n")
	b.WriteString(strings.ToUpper(trigger.Timing))
	b.WriteString(" ")
	b.WriteString(renderTriggerEvents(trigger))
	b.WriteString(" ON ")
	b.WriteString(renderReferencedTable(trigger.Target))
	if trigger.ReferencedTable != "" {
		b.WriteString("\nFROM ")
		b.WriteString(renderReferencedTable(trigger.ReferencedTable))
	}
	if trigger.Deferrable {
		b.WriteString(renderDeferrable(true, trigger.Initially))
	}
	level := trigger.Level
	if level == "" {
		if trigger.Constraint || strings.EqualFold(trigger.Timing, "INSTEAD OF") {
			level = "ROW"
		} else {
			level = "STATEMENT"
		}
	}
	b.WriteString("\nFOR EACH ")
	b.WriteString(strings.ToUpper(level))
	if trigger.When != "" {
		b.WriteString("\nWHEN (")
		b.WriteString(trigger.When)
		b.WriteString(")")
	}
	b.WriteString("\nEXECUTE FUNCTION ")
	b.WriteString(renderReferencedTable(trigger.Function))
	b.WriteString("(")
	args := make([]string, 0, len(trigger.Arguments))
	for _, arg := range trigger.Arguments {
		args = append(args, quoteLiteral(arg))
	}
	b.WriteString(strings.Join(args, ", "))
	b.WriteString(");\n")
}

func renderTriggerComment(b *strings.Builder, trigger pgschema.Trigger) {
	if trigger.Comment != "" {
		fmt.Fprintf(b, "COMMENT ON TRIGGER %s ON %s IS %s;\n", trigger.Name, renderReferencedTable(trigger.Target), quoteLiteral(trigger.Comment))
	}
}

func renderTriggerEvents(trigger pgschema.Trigger) string {
	events := normalisedTriggerEvents(trigger.Events)
	parts := make([]string, 0, len(events))
	for _, event := range events {
		if event == "UPDATE" && len(trigger.Columns) > 0 {
			parts = append(parts, "UPDATE OF "+strings.Join(trigger.Columns, ", "))
			continue
		}
		parts = append(parts, event)
	}
	return strings.Join(parts, " OR ")
}

func renderTableRLS(b *strings.Builder, table pgschema.Table) {
	if table.RowLevelSecurity {
		fmt.Fprintf(b, "ALTER TABLE %s ENABLE ROW LEVEL SECURITY;\n", renderTableName(table))
	}
	if table.ForceRLS {
		fmt.Fprintf(b, "ALTER TABLE %s FORCE ROW LEVEL SECURITY;\n", renderTableName(table))
	}
}

func renderPolicy(b *strings.Builder, policy pgschema.Policy) {
	b.WriteString("CREATE POLICY ")
	b.WriteString(policy.Name)
	b.WriteString(" ON ")
	b.WriteString(renderReferencedTable(policy.Table))
	if policy.Mode != "" {
		b.WriteString("\nAS ")
		b.WriteString(strings.ToUpper(policy.Mode))
	}
	if policy.Command != "" {
		b.WriteString("\nFOR ")
		b.WriteString(strings.ToUpper(policy.Command))
	}
	if len(policy.Roles) > 0 {
		b.WriteString("\nTO ")
		b.WriteString(strings.Join(sortedStrings(policy.Roles), ", "))
	}
	if policy.Using != "" {
		b.WriteString("\nUSING (")
		b.WriteString(policy.Using)
		b.WriteString(")")
	}
	if policy.WithCheck != "" {
		b.WriteString("\nWITH CHECK (")
		b.WriteString(policy.WithCheck)
		b.WriteString(")")
	}
	b.WriteString(";\n")
}

func renderView(b *strings.Builder, view pgschema.View) {
	b.WriteString("CREATE ")
	if view.SecurityBarrier {
		b.WriteString("SECURITY BARRIER ")
	}
	if view.SecurityInvoker {
		b.WriteString("SECURITY INVOKER ")
	}
	b.WriteString("VIEW ")
	b.WriteString(renderQualifiedName(view.Schema, view.Name))
	if len(view.ColumnAliases) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(view.ColumnAliases, ", "))
		b.WriteString(")")
	}
	b.WriteString(" AS\n    ")
	b.WriteString(view.Query)
	if view.CheckOption != "" {
		b.WriteString("\nWITH ")
		b.WriteString(strings.ToUpper(view.CheckOption))
		b.WriteString(" CHECK OPTION")
	}
	b.WriteString(";\n")
}

func renderMaterializedView(b *strings.Builder, mv pgschema.MaterializedView) {
	b.WriteString("CREATE MATERIALIZED VIEW ")
	b.WriteString(renderQualifiedName(mv.Schema, mv.Name))
	if len(mv.ColumnAliases) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(mv.ColumnAliases, ", "))
		b.WriteString(")")
	}
	b.WriteString(" AS\n    ")
	b.WriteString(mv.Query)
	if len(mv.With) > 0 {
		b.WriteString("\nWITH (")
		b.WriteString(renderIndexWith(mv.With))
		b.WriteString(")")
	}
	if mv.NoData {
		b.WriteString("\nWITH NO DATA")
	}
	b.WriteString(";\n")
}

func renderViewComment(b *strings.Builder, view pgschema.View) {
	if view.Comment != "" {
		fmt.Fprintf(b, "COMMENT ON VIEW %s IS %s;\n", renderQualifiedName(view.Schema, view.Name), quoteLiteral(view.Comment))
	}
}

func renderMaterializedViewComment(b *strings.Builder, mv pgschema.MaterializedView) {
	if mv.Comment != "" {
		fmt.Fprintf(b, "COMMENT ON MATERIALIZED VIEW %s IS %s;\n", renderQualifiedName(mv.Schema, mv.Name), quoteLiteral(mv.Comment))
	}
}

func renderCompositeType(b *strings.Builder, compositeType pgschema.CompositeType) error {
	if err := validateIdentifier("composite type", compositeType.Name); err != nil {
		return err
	}
	if len(compositeType.Attributes) == 0 {
		return fmt.Errorf("composite type %q must have at least one attribute", renderQualifiedName(compositeType.Schema, compositeType.Name))
	}
	b.WriteString("CREATE TYPE ")
	b.WriteString(renderQualifiedName(compositeType.Schema, compositeType.Name))
	b.WriteString(" AS (\n")
	lines := make([]string, 0, len(compositeType.Attributes))
	for _, attribute := range compositeType.Attributes {
		if err := validateIdentifier("composite attribute", attribute.Name); err != nil {
			return fmt.Errorf("composite type %q: %w", compositeType.Name, err)
		}
		if strings.TrimSpace(attribute.Type) == "" {
			return fmt.Errorf("composite type %q attribute %q must have a type", compositeType.Name, attribute.Name)
		}
		lines = append(lines, "    "+attribute.Name+" "+attribute.Type)
	}
	b.WriteString(strings.Join(lines, ",\n"))
	b.WriteString("\n);\n")
	return nil
}

func renderDomain(b *strings.Builder, domain pgschema.Domain) error {
	if err := validateIdentifier("domain", domain.Name); err != nil {
		return err
	}
	if strings.TrimSpace(domain.BaseType) == "" {
		return fmt.Errorf("domain %q must have a base type", renderQualifiedName(domain.Schema, domain.Name))
	}
	b.WriteString("CREATE DOMAIN ")
	b.WriteString(renderQualifiedName(domain.Schema, domain.Name))
	b.WriteString(" AS ")
	b.WriteString(domain.BaseType)
	if domain.Default != "" {
		b.WriteString(" DEFAULT ")
		b.WriteString(domain.Default)
	}
	if domain.NotNull {
		b.WriteString(" NOT NULL")
	}
	if domain.Check != "" {
		b.WriteString(" CHECK (")
		b.WriteString(domain.Check)
		b.WriteString(")")
	}
	b.WriteString(";\n")
	return nil
}

func renderTable(b *strings.Builder, table pgschema.Table) error {
	if err := validateIdentifier("table", table.Name); err != nil {
		return err
	}
	if table.PartitionOf != nil {
		b.WriteString("CREATE TABLE ")
		b.WriteString(renderTableName(table))
		b.WriteString(" PARTITION OF ")
		b.WriteString(renderReferencedTable(table.PartitionOf.Parent))
		b.WriteString(" ")
		b.WriteString(renderPartitionBound(table.PartitionOf.Bound))
		if table.Partitioning != nil {
			b.WriteString("\n")
			b.WriteString(renderPartitioning(*table.Partitioning))
		}
		b.WriteString(";\n")
		return nil
	}
	if len(table.Columns) == 0 {
		return fmt.Errorf("table %q must have at least one column", table.Name)
	}

	b.WriteString("CREATE TABLE ")
	b.WriteString(renderTableName(table))
	b.WriteString(" (\n")

	lines := make([]string, 0, len(table.Columns)+len(table.PrimaryKeys)+len(table.UniqueConstraints)+len(table.ForeignKeys)+len(table.Checks))
	columnNames := make(map[string]struct{}, len(table.Columns))
	for _, column := range table.Columns {
		line, err := renderColumn(column)
		if err != nil {
			return fmt.Errorf("table %q: %w", table.Name, err)
		}
		lines = append(lines, "    "+line)
		columnNames[column.Name] = struct{}{}
	}

	primaryKeys := append([]ast.PrimaryKey(nil), table.PrimaryKeys...)
	sort.SliceStable(primaryKeys, func(i, j int) bool {
		return primaryKeys[i].Name < primaryKeys[j].Name
	})
	for _, primaryKey := range primaryKeys {
		lines = append(lines, fmt.Sprintf("    CONSTRAINT %s PRIMARY KEY (%s)", primaryKey.Name, strings.Join(primaryKey.Columns, ", ")))
	}

	uniqueConstraints := append([]ast.UniqueConstraint(nil), table.UniqueConstraints...)
	sort.SliceStable(uniqueConstraints, func(i, j int) bool {
		return uniqueConstraints[i].Name < uniqueConstraints[j].Name
	})
	for _, unique := range uniqueConstraints {
		keyword := "UNIQUE"
		if unique.NullsNotDistinct {
			keyword = "UNIQUE NULLS NOT DISTINCT"
		}
		line := fmt.Sprintf("    CONSTRAINT %s %s (%s)", unique.Name, keyword, strings.Join(unique.Columns, ", "))
		line += renderDeferrable(unique.Deferrable, unique.Initially)
		lines = append(lines, line)
	}

	foreignKeys := append([]ast.ForeignKeyConstraint(nil), table.ForeignKeys...)
	sort.SliceStable(foreignKeys, func(i, j int) bool {
		return foreignKeys[i].Name < foreignKeys[j].Name
	})
	for _, foreignKey := range foreignKeys {
		line := fmt.Sprintf("    CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)", foreignKey.Name, strings.Join(foreignKey.Columns, ", "), renderReferencedTable(foreignKey.ReferencedTable), strings.Join(foreignKey.ReferencedColumns, ", "))
		if foreignKey.OnDelete != "" {
			line += " ON DELETE " + strings.ToUpper(foreignKey.OnDelete)
		}
		if foreignKey.OnUpdate != "" {
			line += " ON UPDATE " + strings.ToUpper(foreignKey.OnUpdate)
		}
		line += renderDeferrable(foreignKey.Deferrable, foreignKey.Initially)
		lines = append(lines, line)
	}

	checks := append([]ast.Check(nil), table.Checks...)
	sort.SliceStable(checks, func(i, j int) bool {
		return checks[i].Name < checks[j].Name
	})
	for _, check := range checks {
		if err := validateIdentifier("check constraint", check.Name); err != nil {
			return err
		}
		if strings.TrimSpace(check.Expression) == "" {
			return fmt.Errorf("check constraint %q must have an expression", check.Name)
		}
		lines = append(lines, fmt.Sprintf("    CONSTRAINT %s CHECK (%s)", check.Name, check.Expression))
	}

	exclusions := append([]pgschema.ExclusionConstraint(nil), table.Exclusions...)
	sort.SliceStable(exclusions, func(i, j int) bool {
		return exclusions[i].Name < exclusions[j].Name
	})
	for _, exclusion := range exclusions {
		if err := validateExclusion(exclusion); err != nil {
			return err
		}
		method := exclusion.Method
		if method == "" {
			method = "gist"
		}
		elements := make([]string, 0, len(exclusion.Elements))
		for _, element := range exclusion.Elements {
			elements = append(elements, renderExclusionElement(element))
		}
		line := fmt.Sprintf("    CONSTRAINT %s EXCLUDE USING %s (%s)", exclusion.Name, method, strings.Join(elements, ", "))
		if exclusion.Where != "" {
			line += " WHERE " + exclusion.Where
		}
		line += renderDeferrable(exclusion.Deferrable, exclusion.Initially)
		lines = append(lines, line)
	}

	b.WriteString(strings.Join(lines, ",\n"))
	b.WriteString("\n)")
	if table.Partitioning != nil {
		b.WriteString("\n")
		b.WriteString(renderPartitioning(*table.Partitioning))
	}
	b.WriteString(";\n")
	return nil
}

func renderPartitioning(partitioning pgschema.Partitioning) string {
	keys := make([]string, 0, len(partitioning.Keys))
	for _, key := range partitioning.Keys {
		expression := key.Expression
		if key.IsExpression {
			expression = "(" + expression + ")"
		}
		keys = append(keys, expression)
	}
	return "PARTITION BY " + strings.ToUpper(partitioning.Strategy) + " (" + strings.Join(keys, ", ") + ")"
}

func renderPartitionBound(bound pgschema.PartitionBound) string {
	switch strings.ToLower(bound.Type) {
	case "range":
		return "FOR VALUES FROM (" + strings.Join(bound.From, ", ") + ") TO (" + strings.Join(bound.To, ", ") + ")"
	case "list":
		return "FOR VALUES IN (" + strings.Join(bound.Values, ", ") + ")"
	case "hash":
		return fmt.Sprintf("FOR VALUES WITH (modulus %d, remainder %d)", bound.Modulus, bound.Remainder)
	case "default":
		return "DEFAULT"
	default:
		return ""
	}
}

func renderColumn(column ast.Column) (string, error) {
	if err := validateIdentifier("column", column.Name); err != nil {
		return "", err
	}
	if strings.TrimSpace(column.Type) == "" {
		return "", fmt.Errorf("column %q must have a type", column.Name)
	}

	parts := []string{column.Name, column.Type}
	if column.Identity != nil {
		if err := validateIdentity(column); err != nil {
			return "", fmt.Errorf("column %q: %w", column.Name, err)
		}
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
		if err := validateGenerated(column); err != nil {
			return "", fmt.Errorf("column %q: %w", column.Name, err)
		}
		parts = append(parts, fmt.Sprintf("GENERATED ALWAYS AS (%s) STORED", column.Generated.As))
	}
	if column.References != nil {
		if _, err := parseQualifiedIdentifier("referenced table", column.References.Table); err != nil {
			return "", err
		}
		if err := validateIdentifier("referenced column", column.References.Column); err != nil {
			return "", err
		}
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
	if strings.EqualFold(identity.Type, "bydefault") {
		keyword = "GENERATED BY DEFAULT AS IDENTITY"
	}
	options := renderIdentityOptions(identity)
	if options == "" {
		return keyword
	}
	return keyword + " (" + options + ")"
}

func renderIdentityOptions(identity *ast.Identity) string {
	var parts []string
	if identity.Name != "" {
		parts = append(parts, "SEQUENCE NAME "+identity.Name)
	}
	if identity.Increment != 0 {
		parts = append(parts, fmt.Sprintf("INCREMENT %d", identity.Increment))
	}
	if identity.MinValue != nil {
		parts = append(parts, fmt.Sprintf("MINVALUE %d", *identity.MinValue))
	}
	if identity.MaxValue != nil {
		parts = append(parts, fmt.Sprintf("MAXVALUE %d", *identity.MaxValue))
	}
	if identity.StartWith != nil {
		parts = append(parts, fmt.Sprintf("START %d", *identity.StartWith))
	}
	if identity.Cache != nil {
		parts = append(parts, fmt.Sprintf("CACHE %d", *identity.Cache))
	}
	if identity.Cycle {
		parts = append(parts, "CYCLE")
	}
	return strings.Join(parts, " ")
}

func validateIdentity(column ast.Column) error {
	if column.Identity == nil {
		return nil
	}
	switch strings.ToLower(column.Identity.Type) {
	case "always", "bydefault":
	default:
		return fmt.Errorf("identity type %q must be always or byDefault", column.Identity.Type)
	}
	baseType := strings.TrimSpace(strings.TrimSuffix(column.Type, "[]"))
	switch baseType {
	case "smallint", "integer", "bigint":
	default:
		return fmt.Errorf("identity columns require an integer type, got %q", column.Type)
	}
	if column.Generated != nil {
		return errors.New("a column cannot be both identity and generated")
	}
	if column.Default != "" {
		return errors.New("identity columns cannot have an explicit default")
	}
	if column.Identity.Increment < 1 {
		return errors.New("identity increment must be positive")
	}
	if column.Identity.MinValue != nil && column.Identity.MaxValue != nil {
		if *column.Identity.MinValue >= *column.Identity.MaxValue {
			return errors.New("identity min value must be less than max value")
		}
	}
	if column.Identity.StartWith != nil {
		if column.Identity.MinValue != nil && *column.Identity.StartWith < *column.Identity.MinValue {
			return errors.New("identity startWith must be >= minValue")
		}
		if column.Identity.MaxValue != nil && *column.Identity.StartWith > *column.Identity.MaxValue {
			return errors.New("identity startWith must be <= maxValue")
		}
	}
	return nil
}

func validateGenerated(column ast.Column) error {
	if column.Generated == nil {
		return nil
	}
	if strings.TrimSpace(column.Generated.As) == "" {
		return errors.New("generated column expression must not be empty")
	}
	if !strings.EqualFold(column.Generated.Type, "stored") {
		return fmt.Errorf("generated column type %q must be stored", column.Generated.Type)
	}
	if column.Default != "" {
		return errors.New("generated columns cannot have a default")
	}
	if column.Identity != nil {
		return errors.New("a column cannot be both generated and identity")
	}
	lower := strings.ToLower(column.Generated.As)
	if strings.Contains(lower, "select") || strings.Contains(lower, "insert") ||
		strings.Contains(lower, "update") || strings.Contains(lower, "delete") {
		return errors.New("generated columns cannot contain DML statements")
	}
	return validateExpression(column.Generated.As, "generated column expression")
}

func validateCheck(tableName string, check ast.Check) error {
	if strings.TrimSpace(check.Expression) == "" {
		return fmt.Errorf("table %q check constraint %q must have an expression", tableName, check.Name)
	}
	if err := validateExpression(check.Expression, "check expression"); err != nil {
		return fmt.Errorf("table %q check constraint %q: %w", tableName, check.Name, err)
	}
	return nil
}

func validateDefault(column ast.Column) error {
	if column.Default == "" {
		return nil
	}
	return validateExpression(column.Default, "default expression")
}

func validateExpression(expression, kind string) error {
	if strings.TrimSpace(expression) == "" {
		return fmt.Errorf("%s must not be empty", kind)
	}
	depth := 0
	for i := 0; i < len(expression); {
		switch expression[i] {
		case '\'':
			next, ok := scanSingleQuoted(expression, i)
			if !ok {
				return fmt.Errorf("%s has an unterminated string literal", kind)
			}
			i = next
		case '"':
			next, ok := scanDoubleQuoted(expression, i)
			if !ok {
				return fmt.Errorf("%s has an unterminated quoted identifier", kind)
			}
			i = next
		case '$':
			next, matched, closed := scanDollarQuoted(expression, i)
			if matched && !closed {
				return fmt.Errorf("%s has an unterminated dollar-quoted string", kind)
			}
			if matched {
				i = next
				continue
			}
			i++
		case ';':
			return fmt.Errorf("%s contains semicolon; use one SQL expression without statement terminators", kind)
		case '-':
			if hasPrefixAt(expression, i, "--") {
				return fmt.Errorf("%s contains a line comment; use one SQL expression without comments", kind)
			}
			i++
		case '/':
			if hasPrefixAt(expression, i, "/*") {
				return fmt.Errorf("%s contains a block comment; use one SQL expression without comments", kind)
			}
			i++
		case '*':
			if hasPrefixAt(expression, i, "*/") {
				return fmt.Errorf("%s contains a block comment terminator without an opening comment", kind)
			}
			i++
		case '(':
			depth++
			i++
		case ')':
			if depth == 0 {
				return fmt.Errorf("%s has an unmatched closing parenthesis", kind)
			}
			depth--
			i++
		default:
			if isIdentifierStart(expression[i]) {
				next := scanIdentifier(expression, i)
				token := strings.ToLower(expression[i:next])
				if isStatementKeyword(token) {
					return fmt.Errorf("%s contains keyword %s; use one SQL expression without subqueries or DML/DDL statements", kind, strings.ToUpper(token))
				}
				i = next
				continue
			}
			i++
		}
	}
	if depth != 0 {
		return fmt.Errorf("%s has unbalanced parentheses", kind)
	}
	return nil
}

func scanSingleQuoted(expression string, start int) (int, bool) {
	for i := start + 1; i < len(expression); i++ {
		if expression[i] != '\'' {
			continue
		}
		if i+1 < len(expression) && expression[i+1] == '\'' {
			i++
			continue
		}
		return i + 1, true
	}
	return len(expression), false
}

func scanDoubleQuoted(expression string, start int) (int, bool) {
	for i := start + 1; i < len(expression); i++ {
		if expression[i] != '"' {
			continue
		}
		if i+1 < len(expression) && expression[i+1] == '"' {
			i++
			continue
		}
		return i + 1, true
	}
	return len(expression), false
}

func scanDollarQuoted(expression string, start int) (int, bool, bool) {
	end := start + 1
	for end < len(expression) && isDollarTagChar(expression[end]) {
		end++
	}
	if end >= len(expression) || expression[end] != '$' {
		return start + 1, false, false
	}
	delimiter := expression[start : end+1]
	closeAt := strings.Index(expression[end+1:], delimiter)
	if closeAt == -1 {
		return len(expression), true, false
	}
	return end + 1 + closeAt + len(delimiter), true, true
}

func hasPrefixAt(value string, offset int, prefix string) bool {
	return offset+len(prefix) <= len(value) && value[offset:offset+len(prefix)] == prefix
}

func isIdentifierStart(value byte) bool {
	return value == '_' || (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z')
}

func scanIdentifier(value string, start int) int {
	i := start + 1
	for i < len(value) && isIdentifierPart(value[i]) {
		i++
	}
	return i
}

func isIdentifierPart(value byte) bool {
	return isIdentifierStart(value) || (value >= '0' && value <= '9')
}

func isDollarTagChar(value byte) bool {
	return isIdentifierPart(value)
}

func isStatementKeyword(value string) bool {
	switch value {
	case "select", "insert", "update", "delete", "merge", "create", "alter", "drop", "truncate", "grant", "revoke":
		return true
	default:
		return false
	}
}

func renderIndex(b *strings.Builder, tableName string, index pgschema.Index) error {
	if err := validateIdentifier("index", index.Name); err != nil {
		return err
	}
	if len(index.Columns) == 0 {
		return fmt.Errorf("index %q must have at least one column", index.Name)
	}

	var line strings.Builder
	line.WriteString("CREATE ")
	if index.Unique {
		line.WriteString("UNIQUE ")
	}
	line.WriteString("INDEX ")
	if index.Concurrently {
		line.WriteString("CONCURRENTLY ")
	}
	line.WriteString(index.Name)
	line.WriteString(" ON ")
	if index.Only {
		line.WriteString("ONLY ")
	}
	line.WriteString(tableName)
	if index.Method != "" {
		line.WriteString(" USING ")
		line.WriteString(index.Method)
	}
	line.WriteString(" (")
	columns := make([]string, 0, len(index.Columns))
	for _, column := range index.Columns {
		columns = append(columns, renderIndexColumn(column))
	}
	line.WriteString(strings.Join(columns, ", "))
	line.WriteString(")")
	if len(index.With) > 0 {
		line.WriteString(" WITH (")
		line.WriteString(renderIndexWith(index.With))
		line.WriteString(")")
	}
	if index.Where != "" {
		line.WriteString(" WHERE ")
		line.WriteString(index.Where)
	}
	line.WriteString(";\n")
	b.WriteString(line.String())
	return nil
}

func renderIndexColumn(column pgschema.IndexColumn) string {
	expression := column.Expression
	if column.IsExpression {
		expression = "(" + expression + ")"
	}
	parts := []string{expression}
	if column.OpClass != "" {
		parts = append(parts, column.OpClass)
	}
	if column.Order != "" {
		parts = append(parts, column.Order)
	}
	if column.Nulls != "" {
		parts = append(parts, "NULLS", column.Nulls)
	}
	return strings.Join(parts, " ")
}

func renderIndexWith(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+" = "+values[key])
	}
	return strings.Join(parts, ", ")
}

func renderDeferrable(deferrable bool, initially string) string {
	if !deferrable {
		return ""
	}
	out := " DEFERRABLE"
	if initially != "" {
		out += " INITIALLY " + strings.ToUpper(initially)
	}
	return out
}

func renderExclusionElement(element pgschema.ExclusionElement) string {
	parts := []string{element.Expression}
	if element.OpClass != "" {
		parts = append(parts, element.OpClass)
	}
	parts = append(parts, "WITH", element.Operator)
	if element.Order != "" {
		parts = append(parts, element.Order)
	}
	if element.Nulls != "" {
		parts = append(parts, "NULLS", element.Nulls)
	}
	return strings.Join(parts, " ")
}

func validateExclusion(exclusion pgschema.ExclusionConstraint) error {
	if err := validateIdentifier("exclusion constraint", exclusion.Name); err != nil {
		return err
	}
	if exclusion.Method != "" {
		if err := validateIdentifier("exclusion method", exclusion.Method); err != nil {
			return fmt.Errorf("exclusion %q: %w", exclusion.Name, err)
		}
	}
	if len(exclusion.Elements) == 0 {
		return fmt.Errorf("exclusion constraint %q must have at least one element", exclusion.Name)
	}
	for _, element := range exclusion.Elements {
		if strings.TrimSpace(element.Expression) == "" {
			return fmt.Errorf("exclusion constraint %q has an empty element expression", exclusion.Name)
		}
		if err := validateExpression(element.Expression, "exclusion element expression"); err != nil {
			return fmt.Errorf("exclusion constraint %q element %q: %w", exclusion.Name, element.Expression, err)
		}
		if strings.TrimSpace(element.Operator) == "" {
			return fmt.Errorf("exclusion constraint %q element %q must have an operator", exclusion.Name, element.Expression)
		}
		if element.OpClass != "" {
			if err := validateIdentifier("operator class", element.OpClass); err != nil {
				return fmt.Errorf("exclusion %q element %q: %w", exclusion.Name, element.Expression, err)
			}
		}
	}
	if exclusion.Where != "" && strings.TrimSpace(exclusion.Where) != exclusion.Where {
		return fmt.Errorf("exclusion constraint %q WHERE expression must not have leading or trailing whitespace", exclusion.Name)
	}
	if exclusion.Where != "" {
		if err := validateExpression(exclusion.Where, "exclusion WHERE expression"); err != nil {
			return fmt.Errorf("exclusion constraint %q: %w", exclusion.Name, err)
		}
	}
	if err := validateInitially(exclusion.Initially); err != nil {
		return fmt.Errorf("exclusion constraint %q: %w", exclusion.Name, err)
	}
	return nil
}

func validateInitially(initially string) error {
	switch strings.ToUpper(initially) {
	case "", "IMMEDIATE", "DEFERRED":
		return nil
	default:
		return fmt.Errorf("INITIALLY value %q must be IMMEDIATE or DEFERRED", initially)
	}
}

func hasComments(table pgschema.Table) bool {
	if table.Comment != "" {
		return true
	}
	for _, column := range table.Columns {
		if column.Comment != "" {
			return true
		}
	}
	return false
}

func hasRLS(table pgschema.Table) bool {
	return table.RowLevelSecurity || table.ForceRLS
}

func renderComments(b *strings.Builder, table pgschema.Table) error {
	if table.Comment != "" {
		fmt.Fprintf(b, "COMMENT ON TABLE %s IS %s;\n", renderTableName(table), quoteLiteral(table.Comment))
	}
	for _, column := range table.Columns {
		if column.Comment == "" {
			continue
		}
		if err := validateIdentifier("column", column.Name); err != nil {
			return err
		}
		fmt.Fprintf(b, "COMMENT ON COLUMN %s.%s IS %s;\n", renderTableName(table), column.Name, quoteLiteral(column.Comment))
	}
	return nil
}

func renderTableName(table pgschema.Table) string {
	if table.Schema == "" {
		return table.Name
	}
	return table.Schema + "." + table.Name
}

func renderReferencedTable(name string) string {
	parts, err := parseQualifiedIdentifier("referenced table", name)
	if err != nil {
		return name
	}
	return strings.Join(parts, ".")
}

func renderQualifiedName(schema, name string) string {
	if schema == "" {
		return name
	}
	return schema + "." + name
}

func qualifiedName(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}

func tableKey(table pgschema.Table) string {
	return tableSchema(table) + "." + table.Name
}

func tableSchema(table pgschema.Table) string {
	schema := table.Schema
	if schema == "" {
		schema = "public"
	}
	return schema
}

func indexKey(table pgschema.Table, index pgschema.Index) string {
	return tableSchema(table) + "." + index.Name
}

func referenceKey(defaultSchema, name string) (string, error) {
	parts, err := parseQualifiedIdentifier("referenced table", name)
	if err != nil {
		return "", err
	}
	if len(parts) == 2 {
		return parts[0] + "." + parts[1], nil
	}
	schema := defaultSchema
	if schema == "" {
		schema = "public"
	}
	return schema + "." + parts[0], nil
}

func parseQualifiedIdentifier(kind, value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("%s identifier must not be empty", kind)
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return nil, fmt.Errorf("%s identifier %q must be unqualified or schema-qualified", kind, value)
	}
	for _, part := range parts {
		if err := validateIdentifier(kind, part); err != nil {
			return nil, err
		}
	}
	return parts, nil
}

func validateExtensionName(value string) error {
	if value == "" {
		return errors.New("extension identifier must not be empty")
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("extension identifier %q may only contain letters, numbers, underscores, and hyphens", value)
	}
	return nil
}

func validateRole(role pgschema.Role, names map[string]struct{}) error {
	if err := validateIdentifier("role", role.Name); err != nil {
		return err
	}
	if _, ok := names[role.Name]; ok {
		return fmt.Errorf("duplicate role %q", role.Name)
	}
	names[role.Name] = struct{}{}
	if role.ConnectionLimit != nil && *role.ConnectionLimit < -1 {
		return fmt.Errorf("role %q connection limit must be -1 or greater", role.Name)
	}
	if role.ValidUntil != "" && strings.TrimSpace(role.ValidUntil) != role.ValidUntil {
		return fmt.Errorf("role %q valid until value must not have leading or trailing whitespace", role.Name)
	}
	if err := validateRoleList(role.Name, "memberOf", role.MemberOf); err != nil {
		return err
	}
	if err := validateRoleList(role.Name, "adminOf", role.AdminOf); err != nil {
		return err
	}
	return nil
}

func validateRoleList(roleName, field string, roles []string) error {
	seen := map[string]struct{}{}
	for _, name := range roles {
		if err := validateIdentifier("role "+field, name); err != nil {
			return fmt.Errorf("role %q: %w", roleName, err)
		}
		if name == roleName {
			return fmt.Errorf("role %q cannot reference itself in %s", roleName, field)
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("role %q has duplicate %s role %q", roleName, field, name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func validateFunction(function pgschema.Function, names map[string]struct{}) error {
	if function.Schema != "" {
		if err := validateIdentifier("function schema", function.Schema); err != nil {
			return err
		}
	}
	if err := validateIdentifier("function", function.Name); err != nil {
		return err
	}
	key := functionKey(function)
	if _, ok := names[key]; ok {
		return fmt.Errorf("duplicate function %q", renderFunctionDisplayName(function))
	}
	names[key] = struct{}{}
	if strings.TrimSpace(function.Language) == "" {
		return fmt.Errorf("function %q must have a language", renderFunctionDisplayName(function))
	}
	if err := validateIdentifier("function language", function.Language); err != nil {
		return fmt.Errorf("function %q: %w", renderFunctionDisplayName(function), err)
	}
	if strings.TrimSpace(function.ReturnType) == "" {
		return fmt.Errorf("function %q must have a return type", renderFunctionDisplayName(function))
	}
	if strings.TrimSpace(function.Body) == "" {
		return fmt.Errorf("function %q must have a body", renderFunctionDisplayName(function))
	}
	if strings.Contains(function.Body, "$$") {
		return fmt.Errorf("function %q body must not contain $$ because gosqlkit uses $$ dollar quoting", renderFunctionDisplayName(function))
	}
	if err := validateFunctionVolatility(function); err != nil {
		return err
	}
	if err := validateFunctionParallel(function); err != nil {
		return err
	}
	if function.Cost != nil && *function.Cost <= 0 {
		return fmt.Errorf("function %q cost must be greater than zero", renderFunctionDisplayName(function))
	}
	if function.Rows != nil && *function.Rows <= 0 {
		return fmt.Errorf("function %q rows must be greater than zero", renderFunctionDisplayName(function))
	}
	if err := validateFunctionArguments(function); err != nil {
		return err
	}
	if function.Comment != "" && strings.TrimSpace(function.Comment) != function.Comment {
		return fmt.Errorf("function %q comment must not have leading or trailing whitespace", renderFunctionDisplayName(function))
	}
	for key, value := range function.Configuration {
		if err := validateConfigKey("function configuration", key); err != nil {
			return fmt.Errorf("function %q: %w", renderFunctionDisplayName(function), err)
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("function %q configuration %q has an empty value", renderFunctionDisplayName(function), key)
		}
	}
	return nil
}

func validateFunctionVolatility(function pgschema.Function) error {
	switch strings.ToUpper(function.Volatility) {
	case "", "IMMUTABLE", "STABLE", "VOLATILE":
		return nil
	default:
		return fmt.Errorf("function %q volatility must be IMMUTABLE, STABLE, or VOLATILE, got %q", renderFunctionDisplayName(function), function.Volatility)
	}
}

func validateFunctionParallel(function pgschema.Function) error {
	switch strings.ToUpper(function.Parallel) {
	case "", "SAFE", "RESTRICTED", "UNSAFE":
		return nil
	default:
		return fmt.Errorf("function %q parallel mode must be SAFE, RESTRICTED, or UNSAFE, got %q", renderFunctionDisplayName(function), function.Parallel)
	}
}

func validateFunctionArguments(function pgschema.Function) error {
	defaultSeen := false
	for i, arg := range function.Arguments {
		if arg.Name != "" {
			if err := validateIdentifier("function argument", arg.Name); err != nil {
				return fmt.Errorf("function %q: %w", renderFunctionDisplayName(function), err)
			}
		}
		if strings.TrimSpace(arg.Type) == "" {
			return fmt.Errorf("function %q argument %d must have a type", renderFunctionDisplayName(function), i+1)
		}
		mode := strings.ToUpper(arg.Mode)
		switch mode {
		case "", "IN", "OUT", "INOUT", "VARIADIC":
		default:
			return fmt.Errorf("function %q argument %d mode must be IN, OUT, INOUT, or VARIADIC, got %q", renderFunctionDisplayName(function), i+1, arg.Mode)
		}
		if mode == "VARIADIC" && i != len(function.Arguments)-1 {
			return fmt.Errorf("function %q variadic argument must be last", renderFunctionDisplayName(function))
		}
		if arg.Default != "" {
			if mode == "OUT" {
				return fmt.Errorf("function %q OUT argument %q cannot have a default", renderFunctionDisplayName(function), arg.Name)
			}
			if strings.TrimSpace(arg.Default) != arg.Default {
				return fmt.Errorf("function %q argument %q default must not have leading or trailing whitespace", renderFunctionDisplayName(function), arg.Name)
			}
			if err := validateExpression(arg.Default, "argument default expression"); err != nil {
				return fmt.Errorf("function %q argument %q: %w", renderFunctionDisplayName(function), arg.Name, err)
			}
			defaultSeen = true
			continue
		}
		if defaultSeen && mode != "OUT" {
			return fmt.Errorf("function %q input arguments after a default must also have defaults", renderFunctionDisplayName(function))
		}
	}
	return nil
}

func validateTrigger(trigger pgschema.Trigger, names map[string]struct{}, tableNames map[string]struct{}, tableColumns map[string]map[string]struct{}, viewNames map[string]struct{}) error {
	if err := validateIdentifier("trigger", trigger.Name); err != nil {
		return err
	}
	targetKey, targetKind, err := validateTriggerTarget(trigger.Target, tableNames, viewNames)
	if err != nil {
		return fmt.Errorf("trigger %q: %w", trigger.Name, err)
	}
	key := targetKey + "." + trigger.Name
	if _, ok := names[key]; ok {
		return fmt.Errorf("duplicate trigger %q on %q", trigger.Name, renderReferencedTable(trigger.Target))
	}
	names[key] = struct{}{}
	if _, err := parseQualifiedIdentifier("trigger function", trigger.Function); err != nil {
		return fmt.Errorf("trigger %q: %w", trigger.Name, err)
	}
	// Validate trigger arguments
	for i, arg := range trigger.Arguments {
		if strings.TrimSpace(arg) != arg {
			return fmt.Errorf("trigger %q argument %d must not have leading or trailing whitespace", trigger.Name, i+1)
		}
		if arg == "" {
			return fmt.Errorf("trigger %q argument %d cannot be empty", trigger.Name, i+1)
		}
	}
	if err := validateTriggerTiming(trigger); err != nil {
		return err
	}
	events, err := validateTriggerEvents(trigger)
	if err != nil {
		return err
	}
	level := strings.ToUpper(trigger.Level)
	switch level {
	case "", "ROW", "STATEMENT":
	default:
		return fmt.Errorf("trigger %q level must be ROW or STATEMENT, got %q", trigger.Name, trigger.Level)
	}
	if trigger.When != "" && strings.TrimSpace(trigger.When) != trigger.When {
		return fmt.Errorf("trigger %q WHEN expression must not have leading or trailing whitespace", trigger.Name)
	}
	if len(trigger.Columns) > 0 {
		if len(events) != 1 || events[0] != "UPDATE" {
			return fmt.Errorf("trigger %q UPDATE OF columns require exactly one UPDATE event", trigger.Name)
		}
		columns, ok := tableColumns[targetKey]
		if !ok {
			return fmt.Errorf("trigger %q UPDATE OF columns can only be validated for table targets", trigger.Name)
		}
		seenColumns := map[string]struct{}{}
		for _, column := range trigger.Columns {
			if err := validateIdentifier("trigger update column", column); err != nil {
				return fmt.Errorf("trigger %q: %w", trigger.Name, err)
			}
			if _, ok := seenColumns[column]; ok {
				return fmt.Errorf("trigger %q has duplicate update column %q", trigger.Name, column)
			}
			seenColumns[column] = struct{}{}
			if _, ok := columns[column]; !ok {
				return fmt.Errorf("trigger %q references unknown update column %q on %q", trigger.Name, column, renderReferencedTable(trigger.Target))
			}
		}
	}
	if strings.EqualFold(trigger.Timing, "INSTEAD OF") {
		if targetKind != "view" {
			return fmt.Errorf("trigger %q INSTEAD OF timing requires a view target", trigger.Name)
		}
		if level != "" && level != "ROW" {
			return fmt.Errorf("trigger %q INSTEAD OF triggers must be FOR EACH ROW", trigger.Name)
		}
	}
	if containsString(events, "TRUNCATE") && level == "ROW" {
		return fmt.Errorf("trigger %q TRUNCATE triggers must be FOR EACH STATEMENT", trigger.Name)
	}
	if err := validateConstraintTrigger(trigger, events, level, targetKind, tableNames); err != nil {
		return err
	}
	return nil
}

func validateTriggerTarget(target string, tableNames map[string]struct{}, viewNames map[string]struct{}) (string, string, error) {
	key, err := referenceKey("", target)
	if err != nil {
		return "", "", err
	}
	if _, ok := tableNames[key]; ok {
		return key, "table", nil
	}
	if _, ok := viewNames[key]; ok {
		return key, "view", nil
	}
	return "", "", fmt.Errorf("references unknown trigger target %q", renderReferencedTable(target))
}

func validateTriggerTiming(trigger pgschema.Trigger) error {
	switch strings.ToUpper(trigger.Timing) {
	case "BEFORE", "AFTER", "INSTEAD OF":
		return nil
	case "":
		return fmt.Errorf("trigger %q must have a timing", trigger.Name)
	default:
		return fmt.Errorf("trigger %q timing must be BEFORE, AFTER, or INSTEAD OF, got %q", trigger.Name, trigger.Timing)
	}
}

func validateTriggerEvents(trigger pgschema.Trigger) ([]string, error) {
	events := normalisedTriggerEvents(trigger.Events)
	if len(events) == 0 {
		return nil, fmt.Errorf("trigger %q must have at least one event", trigger.Name)
	}
	seen := map[string]struct{}{}
	for _, event := range events {
		switch event {
		case "INSERT", "UPDATE", "DELETE", "TRUNCATE":
		default:
			return nil, fmt.Errorf("trigger %q event must be INSERT, UPDATE, DELETE, or TRUNCATE, got %q", trigger.Name, event)
		}
		if _, ok := seen[event]; ok {
			return nil, fmt.Errorf("trigger %q has duplicate event %q", trigger.Name, event)
		}
		seen[event] = struct{}{}
	}
	return events, nil
}

func validateConstraintTrigger(trigger pgschema.Trigger, events []string, level, targetKind string, tableNames map[string]struct{}) error {
	if !trigger.Constraint {
		if trigger.ReferencedTable != "" {
			return fmt.Errorf("trigger %q FROM table requires a constraint trigger", trigger.Name)
		}
		if trigger.Deferrable || trigger.Initially != "" {
			return fmt.Errorf("trigger %q deferrability requires a constraint trigger", trigger.Name)
		}
		return nil
	}
	if targetKind != "table" {
		return fmt.Errorf("trigger %q constraint triggers require a table target", trigger.Name)
	}
	if !strings.EqualFold(trigger.Timing, "AFTER") {
		return fmt.Errorf("trigger %q constraint triggers must be AFTER triggers", trigger.Name)
	}
	if level != "" && level != "ROW" {
		return fmt.Errorf("trigger %q constraint triggers must be FOR EACH ROW", trigger.Name)
	}
	if containsString(events, "TRUNCATE") {
		return fmt.Errorf("trigger %q constraint triggers cannot use TRUNCATE", trigger.Name)
	}
	// Constraint triggers can only fire on INSERT, UPDATE, DELETE
	for _, event := range events {
		switch event {
		case "INSERT", "UPDATE", "DELETE":
			// OK
		default:
			return fmt.Errorf("trigger %q constraint triggers can only fire on INSERT, UPDATE, or DELETE, got %q", trigger.Name, event)
		}
	}
	if trigger.ReferencedTable != "" {
		key, err := referenceKey("", trigger.ReferencedTable)
		if err != nil {
			return fmt.Errorf("trigger %q FROM table: %w", trigger.Name, err)
		}
		if _, ok := tableNames[key]; !ok {
			return fmt.Errorf("trigger %q references unknown FROM table %q", trigger.Name, renderReferencedTable(trigger.ReferencedTable))
		}
	}
	if err := validateInitially(trigger.Initially); err != nil {
		return fmt.Errorf("trigger %q: %w", trigger.Name, err)
	}
	if trigger.Initially != "" && !trigger.Deferrable {
		return fmt.Errorf("trigger %q INITIALLY requires DEFERRABLE", trigger.Name)
	}
	return nil
}

func validatePolicy(policy pgschema.Policy, names map[string]struct{}, tableNames map[string]struct{}) error {
	if err := validateIdentifier("policy", policy.Name); err != nil {
		return err
	}
	tableKey, err := referenceKey("", policy.Table)
	if err != nil {
		return fmt.Errorf("policy %q: %w", policy.Name, err)
	}
	if _, ok := tableNames[tableKey]; !ok {
		return fmt.Errorf("policy %q references unknown table %q", policy.Name, renderReferencedTable(policy.Table))
	}
	key := tableKey + "." + policy.Name
	if _, ok := names[key]; ok {
		return fmt.Errorf("duplicate policy %q on %q", policy.Name, renderReferencedTable(policy.Table))
	}
	names[key] = struct{}{}
	switch strings.ToUpper(policy.Command) {
	case "", "ALL", "SELECT", "INSERT", "UPDATE", "DELETE":
	default:
		return fmt.Errorf("policy %q command must be ALL, SELECT, INSERT, UPDATE, or DELETE, got %q", policy.Name, policy.Command)
	}
	switch strings.ToUpper(policy.Mode) {
	case "", "PERMISSIVE", "RESTRICTIVE":
	default:
		return fmt.Errorf("policy %q mode must be PERMISSIVE or RESTRICTIVE, got %q", policy.Name, policy.Mode)
	}
	if policy.Using != "" && strings.TrimSpace(policy.Using) != policy.Using {
		return fmt.Errorf("policy %q USING expression must not have leading or trailing whitespace", policy.Name)
	}
	if policy.WithCheck != "" && strings.TrimSpace(policy.WithCheck) != policy.WithCheck {
		return fmt.Errorf("policy %q WITH CHECK expression must not have leading or trailing whitespace", policy.Name)
	}
	if err := validatePolicyRoles(policy); err != nil {
		return err
	}
	return validatePolicyExpressions(policy)
}

func validatePolicyRoles(policy pgschema.Policy) error {
	seen := map[string]struct{}{}
	for _, role := range policy.Roles {
		if err := validatePolicyRole(role); err != nil {
			return fmt.Errorf("policy %q: %w", policy.Name, err)
		}
		normalised := strings.ToLower(role)
		if _, ok := seen[normalised]; ok {
			return fmt.Errorf("policy %q has duplicate role %q", policy.Name, role)
		}
		seen[normalised] = struct{}{}
	}
	return nil
}

func validatePolicyRole(role string) error {
	switch strings.ToUpper(role) {
	case "PUBLIC", "CURRENT_ROLE", "CURRENT_USER", "SESSION_USER":
		return nil
	default:
		return validateIdentifier("policy role", role)
	}
}

func validatePolicyExpressions(policy pgschema.Policy) error {
	switch strings.ToUpper(policy.Command) {
	case "INSERT":
		if policy.Using != "" {
			return fmt.Errorf("policy %q INSERT policies cannot have a USING expression", policy.Name)
		}
	case "SELECT", "DELETE":
		if policy.WithCheck != "" {
			return fmt.Errorf("policy %q %s policies cannot have a WITH CHECK expression", policy.Name, strings.ToUpper(policy.Command))
		}
	}
	if policy.Using != "" {
		if err := validateExpression(policy.Using, "USING expression"); err != nil {
			return fmt.Errorf("policy %q: %w", policy.Name, err)
		}
	}
	if policy.WithCheck != "" {
		if err := validateExpression(policy.WithCheck, "WITH CHECK expression"); err != nil {
			return fmt.Errorf("policy %q: %w", policy.Name, err)
		}
	}
	return nil
}

func normalisedTriggerEvents(events []string) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, strings.ToUpper(event))
	}
	sort.Strings(out)
	return out
}

func containsString(values []string, needle string) bool {
	return slices.Contains(values, needle)
}

func validateViewQuery(query string) error {
	if strings.TrimSpace(query) != query {
		return errors.New("query must not have leading or trailing whitespace")
	}
	return nil
}

func validateConfigKey(kind, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s identifier must not be empty", kind)
	}
	parts := strings.SplitSeq(value, ".")
	for part := range parts {
		if err := validateIdentifier(kind, part); err != nil {
			return err
		}
	}
	return nil
}

func functionKey(function pgschema.Function) string {
	return qualifiedName(function.Schema, function.Name) + "(" + renderFunctionIdentityArguments(function) + ")"
}

func triggerKey(trigger pgschema.Trigger) string {
	key, err := referenceKey("", trigger.Target)
	if err != nil {
		return trigger.Target + "." + trigger.Name
	}
	return key + "." + trigger.Name
}

func policyKey(policy pgschema.Policy) string {
	key, err := referenceKey("", policy.Table)
	if err != nil {
		return policy.Table + "." + policy.Name
	}
	return key + "." + policy.Name
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

func renderFunctionDisplayName(function pgschema.Function) string {
	return renderQualifiedName(function.Schema, function.Name) + "(" + renderFunctionIdentityArguments(function) + ")"
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func quoteLiteral(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `''`) + `'`
}

func validateIdentifier(kind, value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("%s identifier %q must match %s", kind, value, identifierPattern.String())
	}
	return nil
}

func validateNamedColumnConstraint(tableName, kind, name string, columns []string, columnNames map[string]struct{}) error {
	if err := validateIdentifier(kind, name); err != nil {
		return fmt.Errorf("table %q: %w", tableName, err)
	}
	if len(columns) == 0 {
		return fmt.Errorf("table %q %s %q must include at least one column", tableName, kind, name)
	}
	for _, column := range columns {
		if err := validateIdentifier(kind+" column", column); err != nil {
			return fmt.Errorf("table %q %s %q: %w", tableName, kind, name, err)
		}
		if _, ok := columnNames[column]; !ok {
			return fmt.Errorf("table %q %s %q references unknown column %q", tableName, kind, name, column)
		}
	}
	return nil
}

func addConstraintName(tableName string, names map[string]struct{}, name string) error {
	if _, ok := names[name]; ok {
		return fmt.Errorf("table %q has duplicate constraint %q", tableName, name)
	}
	names[name] = struct{}{}
	return nil
}

func validateReferences(tables []pgschema.Table, tableColumns map[string]map[string]struct{}) error {
	for _, table := range tables {
		for _, column := range table.Columns {
			if column.References == nil {
				continue
			}
			refKey, err := referenceKey(table.Schema, column.References.Table)
			if err != nil {
				return fmt.Errorf("table %q column %q: %w", renderTableName(table), column.Name, err)
			}
			columns, ok := tableColumns[refKey]
			if !ok {
				return fmt.Errorf("table %q column %q references unknown table %q", renderTableName(table), column.Name, renderReferencedTable(column.References.Table))
			}
			if _, ok := columns[column.References.Column]; !ok {
				return fmt.Errorf("table %q column %q references unknown column %q on table %q", renderTableName(table), column.Name, column.References.Column, renderReferencedTable(column.References.Table))
			}
		}
		for _, foreignKey := range table.ForeignKeys {
			refKey, err := referenceKey(table.Schema, foreignKey.ReferencedTable)
			if err != nil {
				return fmt.Errorf("table %q foreign key %q: %w", renderTableName(table), foreignKey.Name, err)
			}
			columns, ok := tableColumns[refKey]
			if !ok {
				return fmt.Errorf("table %q foreign key %q references unknown table %q", renderTableName(table), foreignKey.Name, renderReferencedTable(foreignKey.ReferencedTable))
			}
			for _, column := range foreignKey.ReferencedColumns {
				if _, ok := columns[column]; !ok {
					return fmt.Errorf("table %q foreign key %q references unknown column %q on table %q", renderTableName(table), foreignKey.Name, column, renderReferencedTable(foreignKey.ReferencedTable))
				}
			}
		}
	}
	return nil
}

func validateIndex(tableName string, index pgschema.Index, columnNames map[string]struct{}) error {
	if err := validateIdentifier("index", index.Name); err != nil {
		return err
	}
	if index.Method != "" {
		if err := validateIdentifier("index method", index.Method); err != nil {
			return fmt.Errorf("index %q: %w", index.Name, err)
		}
	}
	if len(index.Columns) == 0 {
		return fmt.Errorf("index %q must have at least one column", index.Name)
	}
	for _, column := range index.Columns {
		if strings.TrimSpace(column.Expression) == "" {
			return fmt.Errorf("index %q has an empty column expression", index.Name)
		}
		if column.IsExpression {
			if err := validateExpression(column.Expression, "index column expression"); err != nil {
				return fmt.Errorf("index %q: %w", index.Name, err)
			}
		} else {
			if err := validateIdentifier("index column", column.Expression); err != nil {
				return fmt.Errorf("index %q: %w", index.Name, err)
			}
			if _, ok := columnNames[column.Expression]; !ok {
				return fmt.Errorf("table %q index %q references unknown column %q", tableName, index.Name, column.Expression)
			}
		}
		if column.OpClass != "" {
			if err := validateIdentifier("operator class", column.OpClass); err != nil {
				return fmt.Errorf("index %q column %q: %w", index.Name, column.Expression, err)
			}
		}
	}
	for key := range index.With {
		if err := validateIdentifier("index storage parameter", key); err != nil {
			return fmt.Errorf("index %q: %w", index.Name, err)
		}
		if strings.TrimSpace(index.With[key]) == "" {
			return fmt.Errorf("index %q storage parameter %q has an empty value", index.Name, key)
		}
	}
	if strings.TrimSpace(index.Where) != index.Where {
		return fmt.Errorf("index %q WHERE expression must not have leading or trailing whitespace", index.Name)
	}
	if index.Where != "" {
		if err := validateExpression(index.Where, "index WHERE expression"); err != nil {
			return fmt.Errorf("index %q: %w", index.Name, err)
		}
	}
	return nil
}

func validatePartitioning(table pgschema.Table, columnNames map[string]struct{}) error {
	partitioning := table.Partitioning
	if partitioning == nil {
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(partitioning.Strategy)) {
	case "range", "list", "hash":
	default:
		return fmt.Errorf("table %q partition strategy %q must be range, list, or hash", renderTableName(table), partitioning.Strategy)
	}
	if len(partitioning.Keys) == 0 {
		return fmt.Errorf("table %q partitioning must include at least one key", renderTableName(table))
	}

	partitionColumns := make([]string, 0, len(partitioning.Keys))
	seenColumns := map[string]struct{}{}
	hasExpressionKey := false
	for _, key := range partitioning.Keys {
		expression := strings.TrimSpace(key.Expression)
		if expression == "" {
			return fmt.Errorf("table %q has an empty partition key", renderTableName(table))
		}
		if key.IsExpression {
			hasExpressionKey = true
			if err := validateExpression(expression, "partition key expression"); err != nil {
				return fmt.Errorf("table %q: %w", renderTableName(table), err)
			}
			continue
		}
		if err := validateIdentifier("partition key", expression); err != nil {
			return fmt.Errorf("table %q: %w", renderTableName(table), err)
		}
		if _, ok := columnNames[expression]; !ok {
			return fmt.Errorf("table %q partition key references unknown column %q", renderTableName(table), expression)
		}
		if _, ok := seenColumns[expression]; ok {
			return fmt.Errorf("table %q has duplicate partition key column %q", renderTableName(table), expression)
		}
		seenColumns[expression] = struct{}{}
		partitionColumns = append(partitionColumns, expression)
	}

	if hasExpressionKey {
		if tableHasUniqueSemantics(table) {
			return fmt.Errorf("table %q partition expression keys cannot be combined with primary keys, unique constraints, or unique indexes", renderTableName(table))
		}
		return nil
	}
	for _, column := range table.Columns {
		if column.PrimaryKey {
			if err := validatePartitionUniqueColumns(table, "primary key", []string{column.Name}, partitionColumns); err != nil {
				return err
			}
		}
		if column.Unique {
			if err := validatePartitionUniqueColumns(table, "unique constraint", []string{column.Name}, partitionColumns); err != nil {
				return err
			}
		}
	}
	for _, primaryKey := range table.PrimaryKeys {
		if err := validatePartitionUniqueColumns(table, "primary key "+primaryKey.Name, primaryKey.Columns, partitionColumns); err != nil {
			return err
		}
	}
	for _, unique := range table.UniqueConstraints {
		if err := validatePartitionUniqueColumns(table, "unique constraint "+unique.Name, unique.Columns, partitionColumns); err != nil {
			return err
		}
	}
	for _, index := range table.Indexes {
		if !index.Unique {
			continue
		}
		columns := make([]string, 0, len(index.Columns))
		for _, column := range index.Columns {
			if column.IsExpression {
				return fmt.Errorf("table %q unique index %q on a partitioned table must include all partition key columns", renderTableName(table), index.Name)
			}
			columns = append(columns, column.Expression)
		}
		if err := validatePartitionUniqueColumns(table, "unique index "+index.Name, columns, partitionColumns); err != nil {
			return err
		}
	}
	return nil
}

func tableHasUniqueSemantics(table pgschema.Table) bool {
	for _, column := range table.Columns {
		if column.PrimaryKey || column.Unique {
			return true
		}
	}
	if len(table.PrimaryKeys) > 0 || len(table.UniqueConstraints) > 0 {
		return true
	}
	for _, index := range table.Indexes {
		if index.Unique {
			return true
		}
	}
	return false
}

func validatePartitionUniqueColumns(table pgschema.Table, kind string, columns, partitionColumns []string) error {
	if containsAllColumns(columns, partitionColumns) {
		return nil
	}
	return fmt.Errorf("table %q %s must include all partition key columns", renderTableName(table), kind)
}

func containsAllColumns(columns, required []string) bool {
	seen := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		seen[column] = struct{}{}
	}
	for _, column := range required {
		if _, ok := seen[column]; !ok {
			return false
		}
	}
	return true
}

func validatePartitionChildShape(table pgschema.Table) error {
	key := renderTableName(table)
	if len(table.Columns) > 0 {
		return fmt.Errorf("partition table %q must not declare columns; columns are inherited from its parent", key)
	}
	if len(table.PrimaryKeys) > 0 || len(table.UniqueConstraints) > 0 || len(table.ForeignKeys) > 0 ||
		len(table.Checks) > 0 || len(table.Exclusions) > 0 {
		return fmt.Errorf("partition table %q must not declare table constraints; declare constraints on the parent or use a manual migration", key)
	}
	if table.RowLevelSecurity || table.ForceRLS {
		return fmt.Errorf("partition table %q must not declare row-level security; declare RLS on the partitioned parent", key)
	}
	return nil
}

func validatePartitionOf(table pgschema.Table, partition pgschema.PartitionOf, tables map[string]pgschema.Table) (pgschema.Table, error) {
	if strings.TrimSpace(partition.Parent) == "" {
		return pgschema.Table{}, fmt.Errorf("partition table %q must reference a parent table", renderTableName(table))
	}
	parentKey, err := referenceKey(table.Schema, partition.Parent)
	if err != nil {
		return pgschema.Table{}, fmt.Errorf("partition table %q: %w", renderTableName(table), err)
	}
	parent, ok := tables[parentKey]
	if !ok {
		return pgschema.Table{}, fmt.Errorf("partition table %q references unknown parent table %q", renderTableName(table), renderReferencedTable(partition.Parent))
	}
	if parent.Partitioning == nil {
		return pgschema.Table{}, fmt.Errorf("partition table %q references non-partitioned parent table %q", renderTableName(table), renderReferencedTable(partition.Parent))
	}
	if err := validatePartitionBound(table, parent, partition.Bound); err != nil {
		return pgschema.Table{}, err
	}
	return parent, nil
}

func validatePartitionBound(table, parent pgschema.Table, bound pgschema.PartitionBound) error {
	kind := strings.ToLower(strings.TrimSpace(bound.Type))
	if kind == "" {
		return fmt.Errorf("partition table %q must include a partition bound", renderTableName(table))
	}
	parentStrategy := strings.ToLower(parent.Partitioning.Strategy)
	if kind != "default" && kind != parentStrategy {
		return fmt.Errorf("partition table %q has %s bound for %s-partitioned parent %q", renderTableName(table), kind, parentStrategy, renderTableName(parent))
	}
	keyCount := len(parent.Partitioning.Keys)
	switch kind {
	case "range":
		if len(bound.From) != keyCount || len(bound.To) != keyCount {
			return fmt.Errorf("partition table %q range bound must include %d FROM and TO value(s)", renderTableName(table), keyCount)
		}
		for _, value := range append(append([]string(nil), bound.From...), bound.To...) {
			if err := validatePartitionBoundValue(value); err != nil {
				return fmt.Errorf("partition table %q: %w", renderTableName(table), err)
			}
		}
	case "list":
		if len(bound.Values) == 0 {
			return fmt.Errorf("partition table %q list bound must include at least one value", renderTableName(table))
		}
		for _, value := range bound.Values {
			if err := validatePartitionBoundValue(value); err != nil {
				return fmt.Errorf("partition table %q: %w", renderTableName(table), err)
			}
		}
	case "hash":
		if bound.Modulus <= 0 {
			return fmt.Errorf("partition table %q hash modulus must be positive", renderTableName(table))
		}
		if bound.Remainder < 0 || bound.Remainder >= bound.Modulus {
			return fmt.Errorf("partition table %q hash remainder must be >= 0 and less than modulus", renderTableName(table))
		}
	case "default":
		return nil
	default:
		return fmt.Errorf("partition table %q partition bound type %q must be range, list, hash, or default", renderTableName(table), bound.Type)
	}
	return nil
}

func validatePartitionBoundValue(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("partition bound values must not be empty")
	}
	if strings.EqualFold(value, "MINVALUE") || strings.EqualFold(value, "MAXVALUE") {
		return nil
	}
	return validateExpression(value, "partition bound value")
}

func validateForeignKeyAction(kind, action string) error {
	if action == "" {
		return nil
	}

	switch strings.ToUpper(action) {
	case "NO ACTION", "RESTRICT", "CASCADE", "SET NULL", "SET DEFAULT":
		return nil
	default:
		return fmt.Errorf("%s action %q is not supported", kind, action)
	}
}
