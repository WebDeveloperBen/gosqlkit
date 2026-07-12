package render

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

var identifierPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// strictTypes are the only column types permitted on STRICT tables.
var strictTypes = map[string]struct{}{
	"INT":     {},
	"INTEGER": {},
	"REAL":    {},
	"TEXT":    {},
	"BLOB":    {},
	"ANY":     {},
}

// SQLite renders the schema to canonical SQLite DDL. It validates first so both
// generate and snapshot surfaces reject invalid schemas the same way.
func SQLite(schema sqliteschema.Schema) (string, error) {
	if err := validateSchema(schema); err != nil {
		return "", err
	}

	core, err := renderStructuredSchema(schema)
	if err != nil {
		return "", err
	}
	return joinSchemaSections(
		renderRawSQLSection(schema.RawSQL, false),
		core,
		renderRawSQLSection(schema.RawSQL, true),
	), nil
}

func renderStructuredSchema(schema sqliteschema.Schema) (string, error) {
	tables, err := orderTables(schema.Tables)
	if err != nil {
		return "", err
	}

	statements := make([]string, 0)

	for _, table := range tables {
		stmt, err := renderTable(table)
		if err != nil {
			return "", err
		}
		statements = append(statements, stmt)
	}

	for _, table := range tables {
		indexes := append([]sqliteschema.Index(nil), table.Indexes...)
		sort.SliceStable(indexes, func(i, j int) bool {
			return indexes[i].Name < indexes[j].Name
		})
		for _, index := range indexes {
			statements = append(statements, renderIndex(table.Name, index))
		}
	}

	views := append([]sqliteschema.View(nil), schema.Views...)
	sort.SliceStable(views, func(i, j int) bool {
		return views[i].Name < views[j].Name
	})
	for _, view := range views {
		statements = append(statements, renderView(view))
	}

	triggers := append([]sqliteschema.Trigger(nil), schema.Triggers...)
	sort.SliceStable(triggers, func(i, j int) bool {
		return triggerKey(triggers[i]) < triggerKey(triggers[j])
	})
	for _, trigger := range triggers {
		statements = append(statements, renderTrigger(trigger))
	}

	return strings.Join(statements, "\n"), nil
}

func renderRawSQLSection(blocks []sqliteschema.RawSQL, after bool) string {
	selected := make([]sqliteschema.RawSQL, 0, len(blocks))
	for _, block := range blocks {
		if block.Before == !after {
			selected = append(selected, block)
		}
	}
	if len(selected) == 0 {
		return ""
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].Name < selected[j].Name
	})
	parts := make([]string, 0, len(selected))
	for _, block := range selected {
		parts = append(parts, strings.TrimSpace(block.SQL)+"\n")
	}
	return strings.Join(parts, "\n")
}

func joinSchemaSections(sections ...string) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		if section != "" {
			parts = append(parts, section)
		}
	}
	return strings.Join(parts, "\n")
}

func renderTable(table sqliteschema.Table) (string, error) {
	var b strings.Builder
	if comment := strings.TrimSpace(table.Comment); comment != "" {
		writeComment(&b, comment)
	}
	b.WriteString(createTableStatement(table))
	return b.String(), nil
}

func createTableStatement(table sqliteschema.Table) string {
	var b strings.Builder
	b.WriteString("CREATE TABLE ")
	b.WriteString(table.Name)
	b.WriteString(" (\n")

	lines := make([]string, 0, len(table.Columns)+len(table.PrimaryKeys)+len(table.UniqueConstraints)+len(table.ForeignKeys)+len(table.Checks))
	for _, column := range table.Columns {
		lines = append(lines, "    "+renderColumn(column))
	}

	primaryKeys := append([]ast.PrimaryKey(nil), table.PrimaryKeys...)
	sort.SliceStable(primaryKeys, func(i, j int) bool { return primaryKeys[i].Name < primaryKeys[j].Name })
	for _, pk := range primaryKeys {
		lines = append(lines, fmt.Sprintf("    CONSTRAINT %s PRIMARY KEY (%s)", pk.Name, strings.Join(pk.Columns, ", ")))
	}

	uniques := append([]ast.UniqueConstraint(nil), table.UniqueConstraints...)
	sort.SliceStable(uniques, func(i, j int) bool { return uniques[i].Name < uniques[j].Name })
	for _, unique := range uniques {
		lines = append(lines, fmt.Sprintf("    CONSTRAINT %s UNIQUE (%s)", unique.Name, strings.Join(unique.Columns, ", ")))
	}

	foreignKeys := append([]ast.ForeignKeyConstraint(nil), table.ForeignKeys...)
	sort.SliceStable(foreignKeys, func(i, j int) bool { return foreignKeys[i].Name < foreignKeys[j].Name })
	for _, fk := range foreignKeys {
		line := fmt.Sprintf("    CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)", fk.Name, strings.Join(fk.Columns, ", "), fk.ReferencedTable, strings.Join(fk.ReferencedColumns, ", "))
		if fk.OnDelete != "" {
			line += " ON DELETE " + strings.ToUpper(fk.OnDelete)
		}
		if fk.OnUpdate != "" {
			line += " ON UPDATE " + strings.ToUpper(fk.OnUpdate)
		}
		line += renderDeferrable(fk.Deferrable, fk.Initially)
		lines = append(lines, line)
	}

	checks := append([]ast.Check(nil), table.Checks...)
	sort.SliceStable(checks, func(i, j int) bool { return checks[i].Name < checks[j].Name })
	for _, check := range checks {
		lines = append(lines, fmt.Sprintf("    CONSTRAINT %s CHECK (%s)", check.Name, check.Expression))
	}

	b.WriteString(strings.Join(lines, ",\n"))
	b.WriteString("\n)")

	options := make([]string, 0, 2)
	if table.WithoutRowID {
		options = append(options, "WITHOUT ROWID")
	}
	if table.Strict {
		options = append(options, "STRICT")
	}
	if len(options) > 0 {
		b.WriteString(" ")
		b.WriteString(strings.Join(options, ", "))
	}
	b.WriteString(";\n")
	return b.String()
}

func renderColumn(column sqliteschema.Column) string {
	parts := []string{column.Name, column.Type}

	if column.PrimaryKey {
		if column.AutoIncrement {
			parts = append(parts, "PRIMARY KEY AUTOINCREMENT")
		} else {
			parts = append(parts, "PRIMARY KEY")
		}
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
	if column.Collation != "" {
		parts = append(parts, "COLLATE", column.Collation)
	}
	if column.Generated != nil {
		storage := strings.ToUpper(column.Generated.Type)
		if storage == "" {
			storage = "STORED"
		}
		parts = append(parts, fmt.Sprintf("GENERATED ALWAYS AS (%s) %s", column.Generated.As, storage))
	}
	if column.References != nil {
		parts = append(parts, fmt.Sprintf("REFERENCES %s (%s)", column.References.Table, column.References.Column))
		if column.References.OnDelete != "" {
			parts = append(parts, "ON DELETE", strings.ToUpper(column.References.OnDelete))
		}
		if column.References.OnUpdate != "" {
			parts = append(parts, "ON UPDATE", strings.ToUpper(column.References.OnUpdate))
		}
	}

	return strings.Join(parts, " ")
}

func renderIndex(tableName string, index sqliteschema.Index) string {
	var b strings.Builder
	b.WriteString("CREATE ")
	if index.Unique {
		b.WriteString("UNIQUE ")
	}
	b.WriteString("INDEX ")
	b.WriteString(index.Name)
	b.WriteString(" ON ")
	b.WriteString(tableName)
	b.WriteString(" (")

	columns := make([]string, 0, len(index.Columns))
	for _, column := range index.Columns {
		columns = append(columns, renderIndexColumn(column))
	}
	b.WriteString(strings.Join(columns, ", "))
	b.WriteString(")")
	if strings.TrimSpace(index.Where) != "" {
		b.WriteString(" WHERE ")
		b.WriteString(index.Where)
	}
	b.WriteString(";\n")
	return b.String()
}

func renderIndexColumn(column sqliteschema.IndexColumn) string {
	expression := column.Expression
	if column.IsExpression {
		expression = "(" + expression + ")"
	}
	parts := []string{expression}
	if column.Collation != "" {
		parts = append(parts, "COLLATE", column.Collation)
	}
	if column.Order != "" {
		parts = append(parts, column.Order)
	}
	return strings.Join(parts, " ")
}

func renderView(view sqliteschema.View) string {
	var b strings.Builder
	if comment := strings.TrimSpace(view.Comment); comment != "" {
		writeComment(&b, comment)
	}
	b.WriteString(createViewStatement(view))
	return b.String()
}

func createViewStatement(view sqliteschema.View) string {
	var b strings.Builder
	b.WriteString("CREATE ")
	if view.Temporary {
		b.WriteString("TEMP ")
	}
	b.WriteString("VIEW ")
	b.WriteString(view.Name)
	if len(view.ColumnAliases) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(view.ColumnAliases, ", "))
		b.WriteString(")")
	}
	b.WriteString(" AS ")
	b.WriteString(strings.TrimSpace(view.Query))
	b.WriteString(";\n")
	return b.String()
}

func renderTrigger(trigger sqliteschema.Trigger) string {
	var b strings.Builder
	if comment := strings.TrimSpace(trigger.Comment); comment != "" {
		writeComment(&b, comment)
	}
	b.WriteString(createTriggerStatement(trigger))
	return b.String()
}

func createTriggerStatement(trigger sqliteschema.Trigger) string {
	var b strings.Builder
	b.WriteString("CREATE TRIGGER ")
	b.WriteString(trigger.Name)
	b.WriteString("\n")

	event := strings.ToUpper(trigger.Events[0])
	line := "    "
	if trigger.Timing != "" {
		line += strings.ToUpper(trigger.Timing) + " "
	}
	line += event
	if event == "UPDATE" && len(trigger.UpdateOfColumns) > 0 {
		line += " OF " + strings.Join(trigger.UpdateOfColumns, ", ")
	}
	line += " ON " + trigger.Target
	b.WriteString(line)
	b.WriteString("\n")

	if trigger.ForEachRow {
		b.WriteString("    FOR EACH ROW\n")
	}
	if strings.TrimSpace(trigger.When) != "" {
		b.WriteString("    WHEN ")
		b.WriteString(trigger.When)
		b.WriteString("\n")
	}
	b.WriteString("BEGIN\n")
	body := strings.TrimSpace(trigger.Body)
	for bodyLine := range strings.SplitSeq(body, "\n") {
		b.WriteString("    ")
		b.WriteString(bodyLine)
		b.WriteString("\n")
	}
	b.WriteString("END;\n")
	return b.String()
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

func writeComment(b *strings.Builder, comment string) {
	for line := range strings.SplitSeq(comment, "\n") {
		b.WriteString("-- ")
		b.WriteString(line)
		b.WriteString("\n")
	}
}

func orderTables(input []sqliteschema.Table) ([]sqliteschema.Table, error) {
	tables := append([]sqliteschema.Table(nil), input...)
	sort.SliceStable(tables, func(i, j int) bool {
		return tables[i].Name < tables[j].Name
	})

	byName := make(map[string]sqliteschema.Table, len(tables))
	for _, table := range tables {
		byName[table.Name] = table
	}

	visiting := map[string]bool{}
	visited := map[string]bool{}
	ordered := make([]sqliteschema.Table, 0, len(tables))

	var visit func(table sqliteschema.Table) error
	visit = func(table sqliteschema.Table) error {
		if visited[table.Name] {
			return nil
		}
		if visiting[table.Name] {
			return fmt.Errorf("table %q has a foreign key cycle; cyclic foreign keys are not supported in CREATE TABLE output yet", table.Name)
		}
		visiting[table.Name] = true
		for _, dependency := range tableDependencies(table) {
			depTable, ok := byName[dependency]
			if !ok {
				continue
			}
			if err := visit(depTable); err != nil {
				return err
			}
		}
		visiting[table.Name] = false
		visited[table.Name] = true
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

func tableDependencies(table sqliteschema.Table) []string {
	deps := map[string]struct{}{}
	for _, column := range table.Columns {
		if column.References != nil && column.References.Table != table.Name {
			deps[column.References.Table] = struct{}{}
		}
	}
	for _, foreignKey := range table.ForeignKeys {
		if foreignKey.ReferencedTable != table.Name {
			deps[foreignKey.ReferencedTable] = struct{}{}
		}
	}
	names := make([]string, 0, len(deps))
	for name := range deps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func triggerKey(trigger sqliteschema.Trigger) string {
	return trigger.Target + "." + trigger.Name
}
