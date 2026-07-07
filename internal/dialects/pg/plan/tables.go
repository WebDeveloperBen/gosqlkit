package plan

import (
	"reflect"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

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
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindTable, key),
					"create table "+key,
					migrateplan.SQL(stmt),
				).WithDependencies(
					tableDependencyRefs(table)...,
				).WithReverse(
					migrateplan.SQL("DROP TABLE " + renderTableName(table) + ";"),
				),
			)
			if err := p.indexes(ast.Table{}, table); err != nil {
				return err
			}
			if err := p.comments(ast.Table{}, table); err != nil {
				return err
			}
			if err := p.rls(ast.Table{}, table); err != nil {
				return err
			}
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
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindColumn, key),
					"add column "+key,
					migrateplan.SQL("ALTER TABLE "+renderTableName(current)+" ADD COLUMN "+def+";"),
				).WithDependencies(
					columnDependencyRefs(current, column)...,
				).WithReverse(
					migrateplan.SQL("ALTER TABLE " + renderTableName(current) + " DROP COLUMN " + column.Name + ";"),
				),
			)
			continue
		}
		if !reflect.DeepEqual(columnWithoutComment(old), columnWithoutComment(column)) {
			return unsupported("column modifications require semantic planning")
		}
		delete(prevColumns, column.Name)
	}
	if len(prevColumns) > 0 {
		return unsupportedDestructive("columns were removed")
	}
	if err := p.constraints(previous, current); err != nil {
		return err
	}
	if err := p.indexes(previous, current); err != nil {
		return err
	}
	if err := p.comments(previous, current); err != nil {
		return err
	}
	if err := p.rls(previous, current); err != nil {
		return err
	}

	withoutColumnsPrevious := previous
	withoutColumnsCurrent := current
	withoutColumnsPrevious.Columns = nil
	withoutColumnsCurrent.Columns = nil
	withoutColumnsPrevious.PrimaryKeys = nil
	withoutColumnsCurrent.PrimaryKeys = nil
	withoutColumnsPrevious.UniqueConstraints = nil
	withoutColumnsCurrent.UniqueConstraints = nil
	withoutColumnsPrevious.ForeignKeys = nil
	withoutColumnsCurrent.ForeignKeys = nil
	withoutColumnsPrevious.Checks = nil
	withoutColumnsCurrent.Checks = nil
	withoutColumnsPrevious.Exclusions = nil
	withoutColumnsCurrent.Exclusions = nil
	withoutColumnsPrevious.Indexes = nil
	withoutColumnsCurrent.Indexes = nil
	withoutColumnsPrevious.Comment = ""
	withoutColumnsCurrent.Comment = ""
	withoutColumnsPrevious.RowLevelSecurity = false
	withoutColumnsCurrent.RowLevelSecurity = false
	withoutColumnsPrevious.ForceRLS = false
	withoutColumnsCurrent.ForceRLS = false
	if !reflect.DeepEqual(withoutColumnsPrevious, withoutColumnsCurrent) {
		return unsupported("table constraint, index, RLS, or comment changes require semantic planning")
	}
	return nil
}

func (p planner) constraints(previous, current ast.Table) error {
	if err := p.primaryKeys(previous, current); err != nil {
		return err
	}
	if err := p.uniqueConstraints(previous, current); err != nil {
		return err
	}
	if err := p.foreignKeys(previous, current); err != nil {
		return err
	}
	if err := p.checks(previous, current); err != nil {
		return err
	}
	if err := p.exclusions(previous, current); err != nil {
		return err
	}
	return nil
}

func (p planner) primaryKeys(previous, current ast.Table) error {
	prev := mapBy(previous.PrimaryKeys, func(primaryKey ast.PrimaryKey) string { return primaryKey.Name })
	for _, primaryKey := range sortedBy(current.PrimaryKeys, func(item ast.PrimaryKey) string { return item.Name }) {
		old, ok := prev[primaryKey.Name]
		if !ok {
			if primaryKey.PreviousName != "" {
				return unsupported("primary key " + tableKey(current) + "." + primaryKey.Name + " rename metadata requires semantic planning")
			}
			p.addConstraint(current, primaryKey.Name, "add primary key "+tableKey(current)+"."+primaryKey.Name, renderPrimaryKeyConstraint(primaryKey), nil)
			continue
		}
		if !reflect.DeepEqual(old, primaryKey) {
			return unsupported("primary key modifications require semantic planning")
		}
		delete(prev, primaryKey.Name)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("primary keys were removed")
	}
	return nil
}

func (p planner) uniqueConstraints(previous, current ast.Table) error {
	prev := mapBy(previous.UniqueConstraints, func(unique ast.UniqueConstraint) string { return unique.Name })
	for _, unique := range sortedBy(current.UniqueConstraints, func(item ast.UniqueConstraint) string { return item.Name }) {
		old, ok := prev[unique.Name]
		if !ok {
			if unique.PreviousName != "" {
				return unsupported("unique constraint " + tableKey(current) + "." + unique.Name + " rename metadata requires semantic planning")
			}
			p.addConstraint(current, unique.Name, "add unique constraint "+tableKey(current)+"."+unique.Name, renderUniqueConstraint(unique), nil)
			continue
		}
		if !reflect.DeepEqual(old, unique) {
			return unsupported("unique constraint modifications require semantic planning")
		}
		delete(prev, unique.Name)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("unique constraints were removed")
	}
	return nil
}

func (p planner) foreignKeys(previous, current ast.Table) error {
	prev := mapBy(previous.ForeignKeys, func(foreignKey ast.ForeignKeyConstraint) string { return foreignKey.Name })
	for _, foreignKey := range sortedBy(current.ForeignKeys, func(item ast.ForeignKeyConstraint) string { return item.Name }) {
		old, ok := prev[foreignKey.Name]
		if !ok {
			if foreignKey.PreviousName != "" {
				return unsupported("foreign key " + tableKey(current) + "." + foreignKey.Name + " rename metadata requires semantic planning")
			}
			p.addConstraint(
				current,
				foreignKey.Name,
				"add foreign key "+tableKey(current)+"."+foreignKey.Name,
				renderForeignKeyConstraint(foreignKey),
				[]migrateplan.ObjectRef{migrateplan.Ref(migrateplan.ObjectKindTable, referencedTableKey(foreignKey.ReferencedTable))},
			)
			continue
		}
		if !reflect.DeepEqual(old, foreignKey) {
			return unsupported("foreign key modifications require semantic planning")
		}
		delete(prev, foreignKey.Name)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("foreign keys were removed")
	}
	return nil
}

func (p planner) checks(previous, current ast.Table) error {
	prev := mapBy(previous.Checks, func(check ast.Check) string { return check.Name })
	for _, check := range sortedBy(current.Checks, func(item ast.Check) string { return item.Name }) {
		old, ok := prev[check.Name]
		if !ok {
			if check.PreviousName != "" {
				return unsupported("check constraint " + tableKey(current) + "." + check.Name + " rename metadata requires semantic planning")
			}
			p.addConstraint(current, check.Name, "add check constraint "+tableKey(current)+"."+check.Name, renderCheckConstraint(check), nil)
			continue
		}
		if !reflect.DeepEqual(old, check) {
			return unsupported("check constraint modifications require semantic planning")
		}
		delete(prev, check.Name)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("check constraints were removed")
	}
	return nil
}

func (p planner) exclusions(previous, current ast.Table) error {
	prev := mapBy(previous.Exclusions, func(exclusion ast.ExclusionConstraint) string { return exclusion.Name })
	for _, exclusion := range sortedBy(current.Exclusions, func(item ast.ExclusionConstraint) string { return item.Name }) {
		old, ok := prev[exclusion.Name]
		if !ok {
			if exclusion.PreviousName != "" {
				return unsupported("exclusion constraint " + tableKey(current) + "." + exclusion.Name + " rename metadata requires semantic planning")
			}
			p.addConstraint(current, exclusion.Name, "add exclusion constraint "+tableKey(current)+"."+exclusion.Name, renderExclusionConstraint(exclusion), nil)
			continue
		}
		if !reflect.DeepEqual(old, exclusion) {
			return unsupported("exclusion constraint modifications require semantic planning")
		}
		delete(prev, exclusion.Name)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("exclusion constraints were removed")
	}
	return nil
}

func (p planner) addConstraint(table ast.Table, name, summary, definition string, extraDependencies []migrateplan.ObjectRef) {
	tableRef := migrateplan.Ref(migrateplan.ObjectKindTable, tableKey(table))
	dependencies := append([]migrateplan.ObjectRef{tableRef}, extraDependencies...)
	dependencies = uniqueRefs(dependencies)
	p.addWith(
		migrateplan.NewChange(
			migrateplan.OperationAlter,
			migrateplan.Ref(migrateplan.ObjectKindConstraint, tableKey(table)+"."+name),
			summary,
			migrateplan.SQL("ALTER TABLE "+renderTableName(table)+" ADD "+definition+";"),
		).WithDependencies(dependencies...).WithReverse(
			migrateplan.SQL("ALTER TABLE " + renderTableName(table) + " DROP CONSTRAINT " + name + ";"),
		),
	)
}

func (p planner) indexes(previous, current ast.Table) error {
	prev := mapBy(previous.Indexes, func(index ast.Index) string { return index.Name })
	for _, index := range sortedBy(current.Indexes, func(item ast.Index) string { return item.Name }) {
		old, ok := prev[index.Name]
		if !ok {
			if err := ensureAddIndexSupported(current, index); err != nil {
				return err
			}
			stmt, err := renderIndex(current, index)
			if err != nil {
				return err
			}
			table := tableKey(current)
			key := table + "." + index.Name
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationCreate,
					migrateplan.Ref(migrateplan.ObjectKindIndex, key),
					"create index "+key,
					migrateplan.SQL(stmt),
				).WithDependencies(
					migrateplan.Ref(migrateplan.ObjectKindTable, table),
				).WithReverse(
					migrateplan.SQL("DROP INDEX " + renderQualified(current.Schema, index.Name) + ";"),
				),
			)
			continue
		}
		if !reflect.DeepEqual(old, index) {
			return unsupported("index modifications require semantic planning")
		}
		delete(prev, index.Name)
	}
	if len(prev) > 0 {
		return unsupportedDestructive("indexes were removed")
	}
	return nil
}

func (p planner) comments(previous, current ast.Table) error {
	table := tableKey(current)
	if previous.Comment != current.Comment {
		switch {
		case previous.Comment == "" && current.Comment != "":
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindTable, table),
					"add comment to table "+table,
					migrateplan.SQL("COMMENT ON TABLE "+renderTableName(current)+" IS "+quoteSQL(current.Comment)+";"),
				).WithReverse(
					migrateplan.SQL("COMMENT ON TABLE " + renderTableName(current) + " IS NULL;"),
				),
			)
		case current.Comment == "":
			return unsupportedDestructive("table comments were removed")
		default:
			return unsupported("table comment modifications require semantic planning")
		}
	}

	prevColumns := mapBy(previous.Columns, func(column ast.Column) string { return column.Name })
	for _, column := range current.Columns {
		old, ok := prevColumns[column.Name]
		if !ok {
			old = ast.Column{Name: column.Name}
		}
		if old.Comment == column.Comment {
			continue
		}
		key := table + "." + column.Name
		switch {
		case old.Comment == "" && column.Comment != "":
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindColumn, key),
					"add comment to column "+key,
					migrateplan.SQL("COMMENT ON COLUMN "+renderTableName(current)+"."+column.Name+" IS "+quoteSQL(column.Comment)+";"),
				).WithDependencies(
					migrateplan.Ref(migrateplan.ObjectKindTable, table),
				).WithReverse(
					migrateplan.SQL("COMMENT ON COLUMN " + renderTableName(current) + "." + column.Name + " IS NULL;"),
				),
			)
		case column.Comment == "":
			return unsupportedDestructive("column comments were removed")
		default:
			return unsupported("column comment modifications require semantic planning")
		}
	}
	return nil
}

func (p planner) rls(previous, current ast.Table) error {
	table := tableKey(current)
	if previous.RowLevelSecurity != current.RowLevelSecurity {
		if !previous.RowLevelSecurity && current.RowLevelSecurity {
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindTable, table),
					"enable row level security on table "+table,
					migrateplan.SQL("ALTER TABLE "+renderTableName(current)+" ENABLE ROW LEVEL SECURITY;"),
				).WithReverse(
					migrateplan.SQL("ALTER TABLE " + renderTableName(current) + " DISABLE ROW LEVEL SECURITY;"),
				),
			)
		} else {
			return unsupportedDestructive("row level security was disabled")
		}
	}
	if previous.ForceRLS != current.ForceRLS {
		if !previous.ForceRLS && current.ForceRLS {
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindTable, table),
					"force row level security on table "+table,
					migrateplan.SQL("ALTER TABLE "+renderTableName(current)+" FORCE ROW LEVEL SECURITY;"),
				).WithReverse(
					migrateplan.SQL("ALTER TABLE " + renderTableName(current) + " NO FORCE ROW LEVEL SECURITY;"),
				),
			)
		} else {
			return unsupportedDestructive("forced row level security was disabled")
		}
	}
	return nil
}

func ensureCreateTableSupported(table ast.Table) error {
	key := tableKey(table)
	if table.PreviousName != "" {
		return unsupported("table " + key + " rename metadata requires semantic planning")
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
	for _, exclusion := range table.Exclusions {
		if exclusion.PreviousName != "" {
			return unsupported("exclusion constraint " + key + "." + exclusion.Name + " rename metadata requires semantic planning")
		}
	}
	return nil
}

func ensureAddIndexSupported(table ast.Table, index ast.Index) error {
	key := tableKey(table) + "." + index.Name
	if index.PreviousName != "" {
		return unsupported("index " + key + " rename metadata requires semantic planning")
	}
	return nil
}

func ensureAddColumnSupported(table ast.Table, column ast.Column) error {
	key := tableKey(table) + "." + column.Name
	if column.PreviousName != "" {
		return unsupported("column " + key + " rename metadata requires semantic planning")
	}
	return nil
}

func columnDependencyRefs(table ast.Table, column ast.Column) []migrateplan.ObjectRef {
	refs := []migrateplan.ObjectRef{migrateplan.Ref(migrateplan.ObjectKindTable, tableKey(table))}
	if column.References != nil {
		ref := migrateplan.Ref(migrateplan.ObjectKindTable, referencedTableKey(column.References.Table))
		if ref.Key != tableKey(table) {
			refs = append(refs, ref)
		}
	}
	return uniqueRefs(refs)
}

func tableDependencyRefs(table ast.Table) []migrateplan.ObjectRef {
	refs := make([]migrateplan.ObjectRef, 0, len(table.Columns)+len(table.ForeignKeys))
	for _, column := range table.Columns {
		if column.References == nil {
			continue
		}
		ref := migrateplan.Ref(migrateplan.ObjectKindTable, referencedTableKey(column.References.Table))
		if ref.Key != tableKey(table) {
			refs = append(refs, ref)
		}
	}
	for _, foreignKey := range table.ForeignKeys {
		ref := migrateplan.Ref(migrateplan.ObjectKindTable, referencedTableKey(foreignKey.ReferencedTable))
		if ref.Key != tableKey(table) {
			refs = append(refs, ref)
		}
	}
	return uniqueRefs(refs)
}

func columnWithoutComment(column ast.Column) ast.Column {
	column.Comment = ""
	return column
}
