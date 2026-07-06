package render

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/pgschema"
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

	namespaces := append([]pgschema.Namespace(nil), schema.Namespaces...)
	sort.SliceStable(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})
	for i, namespace := range namespaces {
		b.WriteString("CREATE SCHEMA ")
		b.WriteString(namespace.Name)
		b.WriteString(";\n")
		if i < len(namespaces)-1 || len(tables) > 0 {
			b.WriteString("\n")
		}
	}

	extensions := append([]pgschema.Extension(nil), schema.Extensions...)
	sort.SliceStable(extensions, func(i, j int) bool {
		return qualifiedName(extensions[i].Schema, extensions[i].Name) < qualifiedName(extensions[j].Schema, extensions[j].Name)
	})
	for i, extension := range extensions {
		renderExtension(&b, extension)
		if i < len(extensions)-1 || len(schema.Enums) > 0 || len(tables) > 0 {
			b.WriteString("\n")
		}
	}

	enums := append([]pgschema.Enum(nil), schema.Enums...)
	sort.SliceStable(enums, func(i, j int) bool {
		return qualifiedName(enums[i].Schema, enums[i].Name) < qualifiedName(enums[j].Schema, enums[j].Name)
	})
	for i, enum := range enums {
		renderEnum(&b, enum)
		if i < len(enums)-1 || len(tables) > 0 {
			b.WriteString("\n")
		}
	}

	for i, table := range tables {
		if err := renderTable(&b, table); err != nil {
			return "", err
		}
		if i < len(tables)-1 || len(table.Indexes) > 0 {
			b.WriteString("\n")
		}

		indexes := append([]ast.Index(nil), table.Indexes...)
		sort.SliceStable(indexes, func(i, j int) bool {
			return indexes[i].Name < indexes[j].Name
		})
		for j, index := range indexes {
			if err := renderIndex(&b, renderTableName(table), index); err != nil {
				return "", err
			}
			if j < len(indexes)-1 || i < len(tables)-1 {
				b.WriteString("\n")
			}
		}
	}

	return b.String(), nil
}

func orderTables(input []ast.Table) ([]ast.Table, error) {
	tables := append([]ast.Table(nil), input...)
	sort.SliceStable(tables, func(i, j int) bool {
		return tableKey(tables[i]) < tableKey(tables[j])
	})

	byName := make(map[string]ast.Table, len(tables))
	for _, table := range tables {
		byName[tableKey(table)] = table
	}

	visiting := map[string]bool{}
	visited := map[string]bool{}
	ordered := make([]ast.Table, 0, len(tables))

	var visit func(table ast.Table) error
	visit = func(table ast.Table) error {
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

func tableDependencies(table ast.Table) []string {
	deps := map[string]struct{}{}
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
			if err := addConstraintName(table.Name, constraintNames, foreignKey.Name); err != nil {
				return err
			}
		}

		for _, check := range table.Checks {
			if err := addConstraintName(table.Name, constraintNames, check.Name); err != nil {
				return err
			}
		}

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

	return validateReferences(schema.Tables, tableColumns)
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

func renderTable(b *strings.Builder, table ast.Table) error {
	if err := validateIdentifier("table", table.Name); err != nil {
		return err
	}
	if len(table.Columns) == 0 {
		return fmt.Errorf("table %q must have at least one column", table.Name)
	}

	b.WriteString("CREATE TABLE ")
	b.WriteString(renderTableName(table))
	b.WriteString(" (\n")

	lines := make([]string, 0, len(table.Columns)+len(table.PrimaryKeys)+len(table.UniqueConstraints)+len(table.ForeignKeys)+len(table.Checks))
	for _, column := range table.Columns {
		line, err := renderColumn(column)
		if err != nil {
			return fmt.Errorf("table %q: %w", table.Name, err)
		}
		lines = append(lines, "    "+line)
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
		lines = append(lines, fmt.Sprintf("    CONSTRAINT %s %s (%s)", unique.Name, keyword, strings.Join(unique.Columns, ", ")))
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

	b.WriteString(strings.Join(lines, ",\n"))
	b.WriteString("\n);\n")
	return nil
}

func renderColumn(column ast.Column) (string, error) {
	if err := validateIdentifier("column", column.Name); err != nil {
		return "", err
	}
	if strings.TrimSpace(column.Type) == "" {
		return "", fmt.Errorf("column %q must have a type", column.Name)
	}

	parts := []string{column.Name, column.Type}
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

func renderIndex(b *strings.Builder, tableName string, index ast.Index) error {
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

func renderIndexColumn(column ast.IndexColumn) string {
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

func renderTableName(table ast.Table) string {
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

func tableKey(table ast.Table) string {
	return tableSchema(table) + "." + table.Name
}

func tableSchema(table ast.Table) string {
	schema := table.Schema
	if schema == "" {
		schema = "public"
	}
	return schema
}

func indexKey(table ast.Table, index ast.Index) string {
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

func validateReferences(tables []ast.Table, tableColumns map[string]map[string]struct{}) error {
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

func validateIndex(tableName string, index ast.Index, columnNames map[string]struct{}) error {
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
			continue
		}
		if err := validateIdentifier("index column", column.Expression); err != nil {
			return fmt.Errorf("index %q: %w", index.Name, err)
		}
		if _, ok := columnNames[column.Expression]; !ok {
			return fmt.Errorf("table %q index %q references unknown column %q", tableName, index.Name, column.Expression)
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
	return nil
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
