package render

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/webdeveloperben/pgkit/internal/ast"
)

var identifierPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func Postgres(schema ast.Schema) (string, error) {
	if err := validateSchema(schema); err != nil {
		return "", err
	}

	var b strings.Builder

	tables := append([]ast.Table(nil), schema.Tables...)
	sort.SliceStable(tables, func(i, j int) bool {
		return tables[i].Name < tables[j].Name
	})

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
			if err := renderIndex(&b, table.Name, index); err != nil {
				return "", err
			}
			if j < len(indexes)-1 || i < len(tables)-1 {
				b.WriteString("\n")
			}
		}
	}

	return b.String(), nil
}

func validateSchema(schema ast.Schema) error {
	tableNames := map[string]struct{}{}
	indexNames := map[string]struct{}{}

	for _, table := range schema.Tables {
		if err := validateIdentifier("table", table.Name); err != nil {
			return err
		}
		if _, ok := tableNames[table.Name]; ok {
			return fmt.Errorf("duplicate table %q", table.Name)
		}
		tableNames[table.Name] = struct{}{}

		columnNames := map[string]struct{}{}
		for _, column := range table.Columns {
			if err := validateIdentifier("column", column.Name); err != nil {
				return fmt.Errorf("table %q: %w", table.Name, err)
			}
			if _, ok := columnNames[column.Name]; ok {
				return fmt.Errorf("table %q has duplicate column %q", table.Name, column.Name)
			}
			columnNames[column.Name] = struct{}{}

			if column.References != nil {
				if err := validateForeignKeyAction("ON DELETE", column.References.OnDelete); err != nil {
					return fmt.Errorf("table %q column %q: %w", table.Name, column.Name, err)
				}
				if err := validateForeignKeyAction("ON UPDATE", column.References.OnUpdate); err != nil {
					return fmt.Errorf("table %q column %q: %w", table.Name, column.Name, err)
				}
			}
		}

		checkNames := map[string]struct{}{}
		for _, check := range table.Checks {
			if _, ok := checkNames[check.Name]; ok {
				return fmt.Errorf("table %q has duplicate check constraint %q", table.Name, check.Name)
			}
			checkNames[check.Name] = struct{}{}
		}

		for _, index := range table.Indexes {
			if _, ok := indexNames[index.Name]; ok {
				return fmt.Errorf("duplicate index %q", index.Name)
			}
			indexNames[index.Name] = struct{}{}
		}
	}

	return nil
}

func renderTable(b *strings.Builder, table ast.Table) error {
	if err := validateIdentifier("table", table.Name); err != nil {
		return err
	}
	if len(table.Columns) == 0 {
		return fmt.Errorf("table %q must have at least one column", table.Name)
	}

	b.WriteString("CREATE TABLE ")
	b.WriteString(table.Name)
	b.WriteString(" (\n")

	lines := make([]string, 0, len(table.Columns)+len(table.Checks))
	for _, column := range table.Columns {
		line, err := renderColumn(column)
		if err != nil {
			return fmt.Errorf("table %q: %w", table.Name, err)
		}
		lines = append(lines, "    "+line)
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
		if err := validateIdentifier("referenced table", column.References.Table); err != nil {
			return "", err
		}
		if err := validateIdentifier("referenced column", column.References.Column); err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("REFERENCES %s (%s)", column.References.Table, column.References.Column))
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
	for _, column := range index.Columns {
		if err := validateIdentifier("index column", column); err != nil {
			return err
		}
	}

	b.WriteString("CREATE ")
	if index.Unique {
		b.WriteString("UNIQUE ")
	}
	b.WriteString("INDEX ")
	b.WriteString(index.Name)
	b.WriteString(" ON ")
	b.WriteString(tableName)
	b.WriteString(" (")
	b.WriteString(strings.Join(index.Columns, ", "))
	b.WriteString(");\n")
	return nil
}

func validateIdentifier(kind, value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("%s identifier %q must match %s", kind, value, identifierPattern.String())
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
