package render

import (
	"fmt"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

func validateSchema(schema sqliteschema.Schema) error {
	rawSQLNames := map[string]struct{}{}
	for _, block := range schema.RawSQL {
		if err := validateIdentifier("raw SQL block", block.Name); err != nil {
			return err
		}
		if _, ok := rawSQLNames[block.Name]; ok {
			return fmt.Errorf("duplicate raw SQL block %q", block.Name)
		}
		rawSQLNames[block.Name] = struct{}{}
		if strings.TrimSpace(block.SQL) == "" {
			return fmt.Errorf("raw SQL block %q must contain SQL", block.Name)
		}
	}

	tableNames := map[string]struct{}{}
	tableColumns := map[string]map[string]struct{}{}
	indexNames := map[string]struct{}{}

	for _, table := range schema.Tables {
		if err := validateIdentifier("table", table.Name); err != nil {
			return err
		}
		if _, ok := tableNames[table.Name]; ok {
			return fmt.Errorf("duplicate table %q", table.Name)
		}
		tableNames[table.Name] = struct{}{}

		if len(table.Columns) == 0 {
			return fmt.Errorf("table %q must have at least one column", table.Name)
		}

		columnNames, err := validateTableColumns(table)
		if err != nil {
			return err
		}
		tableColumns[table.Name] = columnNames

		if err := validateTableConstraints(table, columnNames); err != nil {
			return err
		}

		if table.WithoutRowID && !tableHasPrimaryKey(table) {
			return fmt.Errorf("table %q is WITHOUT ROWID but has no primary key", table.Name)
		}
	}

	for _, table := range schema.Tables {
		for _, index := range table.Indexes {
			if err := validateIndex(table, index, tableColumns[table.Name]); err != nil {
				return err
			}
			if _, ok := indexNames[index.Name]; ok {
				return fmt.Errorf("duplicate index %q", index.Name)
			}
			indexNames[index.Name] = struct{}{}
		}
	}

	viewNames := map[string]struct{}{}
	for _, view := range schema.Views {
		if err := validateIdentifier("view", view.Name); err != nil {
			return err
		}
		if _, ok := viewNames[view.Name]; ok {
			return fmt.Errorf("duplicate view %q", view.Name)
		}
		if _, ok := tableNames[view.Name]; ok {
			return fmt.Errorf("view %q collides with a table of the same name", view.Name)
		}
		viewNames[view.Name] = struct{}{}
		if strings.TrimSpace(view.Query) == "" {
			return fmt.Errorf("view %q must have a query", view.Name)
		}
	}

	triggerNames := map[string]struct{}{}
	for _, trigger := range schema.Triggers {
		if err := validateTrigger(trigger, triggerNames, tableNames, viewNames); err != nil {
			return err
		}
	}

	return validateReferences(schema.Tables, tableColumns)
}

func validateTableColumns(table sqliteschema.Table) (map[string]struct{}, error) {
	columnNames := map[string]struct{}{}
	inlinePrimaryKeys := 0

	for _, column := range table.Columns {
		if err := validateIdentifier("column", column.Name); err != nil {
			return nil, fmt.Errorf("table %q: %w", table.Name, err)
		}
		if _, ok := columnNames[column.Name]; ok {
			return nil, fmt.Errorf("table %q has duplicate column %q", table.Name, column.Name)
		}
		columnNames[column.Name] = struct{}{}

		if strings.TrimSpace(column.Type) == "" {
			return nil, fmt.Errorf("table %q column %q must have a type", table.Name, column.Name)
		}
		if table.Strict {
			if _, ok := strictTypes[strings.ToUpper(strings.TrimSpace(column.Type))]; !ok {
				return nil, fmt.Errorf("table %q column %q type %q is not allowed on a STRICT table (use INT, INTEGER, REAL, TEXT, BLOB, or ANY)", table.Name, column.Name, column.Type)
			}
		}

		if column.PrimaryKey {
			inlinePrimaryKeys++
		}
		if column.AutoIncrement {
			if !column.PrimaryKey || !strings.EqualFold(strings.TrimSpace(column.Type), "integer") {
				return nil, fmt.Errorf("table %q column %q: AUTOINCREMENT requires INTEGER PRIMARY KEY", table.Name, column.Name)
			}
		}

		if column.Generated != nil {
			if strings.TrimSpace(column.Generated.As) == "" {
				return nil, fmt.Errorf("table %q column %q: generated expression must not be empty", table.Name, column.Name)
			}
			if column.Default != "" {
				return nil, fmt.Errorf("table %q column %q: a generated column cannot have a default", table.Name, column.Name)
			}
			if column.PrimaryKey {
				return nil, fmt.Errorf("table %q column %q: a generated column cannot be a primary key", table.Name, column.Name)
			}
			switch strings.ToUpper(strings.TrimSpace(column.Generated.Type)) {
			case "", "STORED", "VIRTUAL":
			default:
				return nil, fmt.Errorf("table %q column %q: generated storage %q must be STORED or VIRTUAL", table.Name, column.Name, column.Generated.Type)
			}
		}

		if column.References != nil {
			if err := validateIdentifier("referenced table", column.References.Table); err != nil {
				return nil, fmt.Errorf("table %q column %q: %w", table.Name, column.Name, err)
			}
			if err := validateIdentifier("referenced column", column.References.Column); err != nil {
				return nil, fmt.Errorf("table %q column %q: %w", table.Name, column.Name, err)
			}
			if err := validateForeignKeyAction("ON DELETE", column.References.OnDelete); err != nil {
				return nil, fmt.Errorf("table %q column %q: %w", table.Name, column.Name, err)
			}
			if err := validateForeignKeyAction("ON UPDATE", column.References.OnUpdate); err != nil {
				return nil, fmt.Errorf("table %q column %q: %w", table.Name, column.Name, err)
			}
		}
	}

	if inlinePrimaryKeys > 1 {
		return nil, fmt.Errorf("table %q has multiple inline primary keys", table.Name)
	}
	if inlinePrimaryKeys == 1 && len(table.PrimaryKeys) > 0 {
		return nil, fmt.Errorf("table %q has both an inline and a table-level primary key", table.Name)
	}
	return columnNames, nil
}

func validateTableConstraints(table sqliteschema.Table, columnNames map[string]struct{}) error {
	constraintNames := map[string]struct{}{}

	if len(table.PrimaryKeys) > 1 {
		return fmt.Errorf("table %q has multiple primary keys", table.Name)
	}
	for _, pk := range table.PrimaryKeys {
		if err := validateNamedColumnConstraint(table.Name, "primary key", pk.Name, pk.Columns, columnNames); err != nil {
			return err
		}
		if err := addConstraintName(table.Name, constraintNames, pk.Name); err != nil {
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

	for _, fk := range table.ForeignKeys {
		if err := validateNamedColumnConstraint(table.Name, "foreign key", fk.Name, fk.Columns, columnNames); err != nil {
			return err
		}
		if len(fk.ReferencedColumns) == 0 {
			return fmt.Errorf("table %q foreign key %q must reference at least one column", table.Name, fk.Name)
		}
		if len(fk.Columns) != len(fk.ReferencedColumns) {
			return fmt.Errorf("table %q foreign key %q has %d local columns but %d referenced columns", table.Name, fk.Name, len(fk.Columns), len(fk.ReferencedColumns))
		}
		if err := validateForeignKeyAction("ON DELETE", fk.OnDelete); err != nil {
			return fmt.Errorf("table %q foreign key %q: %w", table.Name, fk.Name, err)
		}
		if err := validateForeignKeyAction("ON UPDATE", fk.OnUpdate); err != nil {
			return fmt.Errorf("table %q foreign key %q: %w", table.Name, fk.Name, err)
		}
		if err := validateInitially(fk.Initially); err != nil {
			return fmt.Errorf("table %q foreign key %q: %w", table.Name, fk.Name, err)
		}
		if err := addConstraintName(table.Name, constraintNames, fk.Name); err != nil {
			return err
		}
	}

	for _, check := range table.Checks {
		if err := validateIdentifier("check constraint", check.Name); err != nil {
			return err
		}
		if strings.TrimSpace(check.Expression) == "" {
			return fmt.Errorf("table %q check constraint %q must have an expression", table.Name, check.Name)
		}
		if err := addConstraintName(table.Name, constraintNames, check.Name); err != nil {
			return err
		}
	}

	return nil
}

func validateIndex(table sqliteschema.Table, index sqliteschema.Index, columnNames map[string]struct{}) error {
	if err := validateIdentifier("index", index.Name); err != nil {
		return err
	}
	if len(index.Columns) == 0 {
		return fmt.Errorf("index %q must have at least one column", index.Name)
	}
	for _, column := range index.Columns {
		if column.IsExpression {
			if strings.TrimSpace(column.Expression) == "" {
				return fmt.Errorf("index %q has an empty expression", index.Name)
			}
			continue
		}
		if _, ok := columnNames[column.Expression]; !ok {
			return fmt.Errorf("index %q on table %q references unknown column %q", index.Name, table.Name, column.Expression)
		}
	}
	return nil
}

func validateTrigger(trigger sqliteschema.Trigger, triggerNames, tableNames, viewNames map[string]struct{}) error {
	if err := validateIdentifier("trigger", trigger.Name); err != nil {
		return err
	}
	if _, ok := triggerNames[trigger.Name]; ok {
		return fmt.Errorf("duplicate trigger %q", trigger.Name)
	}
	triggerNames[trigger.Name] = struct{}{}

	if err := validateIdentifier("trigger target", trigger.Target); err != nil {
		return err
	}
	_, onTable := tableNames[trigger.Target]
	_, onView := viewNames[trigger.Target]
	if !onTable && !onView {
		return fmt.Errorf("trigger %q targets unknown table or view %q", trigger.Name, trigger.Target)
	}

	switch strings.ToUpper(strings.TrimSpace(trigger.Timing)) {
	case "", "BEFORE", "AFTER", "INSTEAD OF":
	default:
		return fmt.Errorf("trigger %q timing %q must be BEFORE, AFTER, or INSTEAD OF", trigger.Name, trigger.Timing)
	}
	if onView && !strings.EqualFold(strings.TrimSpace(trigger.Timing), "INSTEAD OF") {
		return fmt.Errorf("trigger %q on view %q must use INSTEAD OF", trigger.Name, trigger.Target)
	}

	if len(trigger.Events) != 1 {
		return fmt.Errorf("trigger %q must have exactly one event (INSERT, UPDATE, or DELETE)", trigger.Name)
	}
	event := strings.ToUpper(strings.TrimSpace(trigger.Events[0]))
	switch event {
	case "INSERT", "UPDATE", "DELETE":
	default:
		return fmt.Errorf("trigger %q event %q must be INSERT, UPDATE, or DELETE", trigger.Name, trigger.Events[0])
	}
	if len(trigger.UpdateOfColumns) > 0 && event != "UPDATE" {
		return fmt.Errorf("trigger %q lists UPDATE OF columns but its event is %q", trigger.Name, event)
	}
	if strings.TrimSpace(trigger.Body) == "" {
		return fmt.Errorf("trigger %q must have a body", trigger.Name)
	}
	return nil
}

func validateReferences(tables []sqliteschema.Table, tableColumns map[string]map[string]struct{}) error {
	for _, table := range tables {
		for _, column := range table.Columns {
			if column.References == nil {
				continue
			}
			columns, ok := tableColumns[column.References.Table]
			if !ok {
				return fmt.Errorf("table %q column %q references unknown table %q", table.Name, column.Name, column.References.Table)
			}
			if _, ok := columns[column.References.Column]; !ok {
				return fmt.Errorf("table %q column %q references unknown column %q on table %q", table.Name, column.Name, column.References.Column, column.References.Table)
			}
		}
		for _, fk := range table.ForeignKeys {
			columns, ok := tableColumns[fk.ReferencedTable]
			if !ok {
				return fmt.Errorf("table %q foreign key %q references unknown table %q", table.Name, fk.Name, fk.ReferencedTable)
			}
			for _, column := range fk.ReferencedColumns {
				if _, ok := columns[column]; !ok {
					return fmt.Errorf("table %q foreign key %q references unknown column %q on table %q", table.Name, fk.Name, column, fk.ReferencedTable)
				}
			}
		}
	}
	return nil
}

func tableHasPrimaryKey(table sqliteschema.Table) bool {
	for _, column := range table.Columns {
		if column.PrimaryKey {
			return true
		}
	}
	return len(table.PrimaryKeys) > 0
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

func validateInitially(initially string) error {
	if initially == "" {
		return nil
	}
	switch strings.ToUpper(initially) {
	case "DEFERRED", "IMMEDIATE":
		return nil
	default:
		return fmt.Errorf("INITIALLY %q must be DEFERRED or IMMEDIATE", initially)
	}
}
