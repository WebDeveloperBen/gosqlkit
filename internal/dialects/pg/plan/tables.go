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
			if table.PreviousName != "" {
				oldKey := qualified(table.Schema, table.PreviousName)
				renamed, hasOld := prev[oldKey]
				if hasOld {
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationRename,
							migrateplan.Ref(migrateplan.ObjectKindTable, key),
							"rename table "+oldKey+" to "+key,
							migrateplan.SQL(renderRenameTable(renamed.Schema, renamed.Name, table.Name)),
						).WithReverse(
							migrateplan.SQL(renderReverseRenameTable(table.Schema, table.Name, table.PreviousName)),
						),
					)
					if err := p.table(renamed, table); err != nil {
						return err
					}
					delete(prev, oldKey)
					continue
				}
				return unsupported("table " + key + " previousName " + table.PreviousName + " does not match any table in the previous snapshot")
			}
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
		for _, table := range sortedTables(removedTables(prev)) {
			key := tableKey(table)
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindTable, key),
					"drop table "+key,
					migrateplan.SQL("DROP TABLE "+renderTableName(table)+";"),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
				).WithReverse(
					migrateplan.SQL(reverseCreateTable(table)),
				),
			)
		}
	}
	return nil
}

func (p planner) table(previous, current ast.Table) error {
	prevColumns := mapBy(previous.Columns, func(column ast.Column) string { return column.Name })
	for _, column := range current.Columns {
		old, ok := prevColumns[column.Name]
		if !ok {
			if column.PreviousName != "" {
				oldCol, hasOld := prevColumns[column.PreviousName]
				if hasOld {
					if err := ensureRenameOnlyColumn(current, column, oldCol); err != nil {
						return err
					}
					key := tableKey(current) + "." + column.Name
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationRename,
							migrateplan.Ref(migrateplan.ObjectKindColumn, key),
							"rename column "+tableKey(current)+"."+column.PreviousName+" to "+key,
							migrateplan.SQL(renderRenameColumn(current.Schema, current.Name, column.PreviousName, column.Name)),
						).WithDependencies(
							migrateplan.Ref(migrateplan.ObjectKindTable, tableKey(current)),
						).WithReverse(
							migrateplan.SQL(renderReverseRenameColumn(current.Schema, current.Name, column.Name, column.PreviousName)),
						),
					)
					delete(prevColumns, column.PreviousName)
					continue
				}
				return unsupported("column " + tableKey(current) + "." + column.Name + " previousName " + column.PreviousName + " does not match any column in the previous snapshot")
			}
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
		for _, column := range sortedStrings(removedColumns(prevColumns)) {
			key := tableKey(current) + "." + column
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindColumn, key),
					"drop column "+key,
					migrateplan.SQL("ALTER TABLE "+renderTableName(current)+" DROP COLUMN "+column+";"),
				).WithDependencies(
					migrateplan.Ref(migrateplan.ObjectKindTable, tableKey(current)),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
				).WithReverse(
					migrateplan.SQL(reverseAddColumn(current, prevColumns[column])),
				),
			)
		}
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
	withoutColumnsPrevious.PreviousName = ""
	withoutColumnsCurrent.PreviousName = ""
	withoutColumnsPrevious.Name = withoutColumnsCurrent.Name
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
				oldPK, hasOld := prev[primaryKey.PreviousName]
				if hasOld {
					if err := ensureRenameOnlyPrimaryKey(current, primaryKey, oldPK); err != nil {
						return err
					}
					key := tableKey(current) + "." + primaryKey.Name
					p.addConstraintRename(current, primaryKey.PreviousName, primaryKey.Name, "primary key", key)
					delete(prev, primaryKey.PreviousName)
					continue
				}
				return unsupported("primary key " + tableKey(current) + "." + primaryKey.Name + " previousName " + primaryKey.PreviousName + " does not match any primary key in the previous snapshot")
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
		for _, name := range sortedStrings(removedNames(prev)) {
			key := tableKey(current) + "." + name
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindConstraint, key),
					"drop primary key "+key,
					migrateplan.SQL("ALTER TABLE "+renderTableName(current)+" DROP CONSTRAINT "+name+";"),
				).WithDependencies(
					migrateplan.Ref(migrateplan.ObjectKindTable, tableKey(current)),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskLockHeavy,
				).WithReverse(
					migrateplan.SQL("ALTER TABLE " + renderTableName(current) + " ADD " + renderPrimaryKeyConstraint(prev[name]) + ";"),
				),
			)
		}
	}
	return nil
}

func (p planner) uniqueConstraints(previous, current ast.Table) error {
	prev := mapBy(previous.UniqueConstraints, func(unique ast.UniqueConstraint) string { return unique.Name })
	for _, unique := range sortedBy(current.UniqueConstraints, func(item ast.UniqueConstraint) string { return item.Name }) {
		old, ok := prev[unique.Name]
		if !ok {
			if unique.PreviousName != "" {
				oldUQ, hasOld := prev[unique.PreviousName]
				if hasOld {
					if err := ensureRenameOnlyUniqueConstraint(current, unique, oldUQ); err != nil {
						return err
					}
					key := tableKey(current) + "." + unique.Name
					p.addConstraintRename(current, unique.PreviousName, unique.Name, "unique constraint", key)
					delete(prev, unique.PreviousName)
					continue
				}
				return unsupported("unique constraint " + tableKey(current) + "." + unique.Name + " previousName " + unique.PreviousName + " does not match any unique constraint in the previous snapshot")
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
		for _, name := range sortedStrings(removedNames(prev)) {
			key := tableKey(current) + "." + name
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindConstraint, key),
					"drop unique constraint "+key,
					migrateplan.SQL("ALTER TABLE "+renderTableName(current)+" DROP CONSTRAINT "+name+";"),
				).WithDependencies(
					migrateplan.Ref(migrateplan.ObjectKindTable, tableKey(current)),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskLockHeavy,
				).WithReverse(
					migrateplan.SQL("ALTER TABLE " + renderTableName(current) + " ADD " + renderUniqueConstraint(prev[name]) + ";"),
				),
			)
		}
	}
	return nil
}

func (p planner) foreignKeys(previous, current ast.Table) error {
	prev := mapBy(previous.ForeignKeys, func(foreignKey ast.ForeignKeyConstraint) string { return foreignKey.Name })
	for _, foreignKey := range sortedBy(current.ForeignKeys, func(item ast.ForeignKeyConstraint) string { return item.Name }) {
		old, ok := prev[foreignKey.Name]
		if !ok {
			if foreignKey.PreviousName != "" {
				oldFK, hasOld := prev[foreignKey.PreviousName]
				if hasOld {
					if err := ensureRenameOnlyForeignKey(current, foreignKey, oldFK); err != nil {
						return err
					}
					key := tableKey(current) + "." + foreignKey.Name
					p.addConstraintRename(current, foreignKey.PreviousName, foreignKey.Name, "foreign key", key)
					delete(prev, foreignKey.PreviousName)
					continue
				}
				return unsupported("foreign key " + tableKey(current) + "." + foreignKey.Name + " previousName " + foreignKey.PreviousName + " does not match any foreign key in the previous snapshot")
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
		for _, name := range sortedStrings(removedNames(prev)) {
			key := tableKey(current) + "." + name
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindConstraint, key),
					"drop foreign key "+key,
					migrateplan.SQL("ALTER TABLE "+renderTableName(current)+" DROP CONSTRAINT "+name+";"),
				).WithDependencies(
					migrateplan.Ref(migrateplan.ObjectKindTable, tableKey(current)),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskLockHeavy,
				).WithReverse(
					migrateplan.SQL("ALTER TABLE " + renderTableName(current) + " ADD " + renderForeignKeyConstraint(prev[name]) + ";"),
				),
			)
		}
	}
	return nil
}

func (p planner) checks(previous, current ast.Table) error {
	prev := mapBy(previous.Checks, func(check ast.Check) string { return check.Name })
	for _, check := range sortedBy(current.Checks, func(item ast.Check) string { return item.Name }) {
		old, ok := prev[check.Name]
		if !ok {
			if check.PreviousName != "" {
				oldCheck, hasOld := prev[check.PreviousName]
				if hasOld {
					if err := ensureRenameOnlyCheck(current, check, oldCheck); err != nil {
						return err
					}
					key := tableKey(current) + "." + check.Name
					p.addConstraintRename(current, check.PreviousName, check.Name, "check constraint", key)
					delete(prev, check.PreviousName)
					continue
				}
				return unsupported("check constraint " + tableKey(current) + "." + check.Name + " previousName " + check.PreviousName + " does not match any check constraint in the previous snapshot")
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
		for _, name := range sortedStrings(removedNames(prev)) {
			key := tableKey(current) + "." + name
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindConstraint, key),
					"drop check constraint "+key,
					migrateplan.SQL("ALTER TABLE "+renderTableName(current)+" DROP CONSTRAINT "+name+";"),
				).WithDependencies(
					migrateplan.Ref(migrateplan.ObjectKindTable, tableKey(current)),
				).WithRisks(
					migrateplan.RiskDestructive,
				).WithReverse(
					migrateplan.SQL("ALTER TABLE " + renderTableName(current) + " ADD " + renderCheckConstraint(prev[name]) + ";"),
				),
			)
		}
	}
	return nil
}

func (p planner) exclusions(previous, current ast.Table) error {
	prev := mapBy(previous.Exclusions, func(exclusion ast.ExclusionConstraint) string { return exclusion.Name })
	for _, exclusion := range sortedBy(current.Exclusions, func(item ast.ExclusionConstraint) string { return item.Name }) {
		old, ok := prev[exclusion.Name]
		if !ok {
			if exclusion.PreviousName != "" {
				oldExcl, hasOld := prev[exclusion.PreviousName]
				if hasOld {
					if err := ensureRenameOnlyExclusion(current, exclusion, oldExcl); err != nil {
						return err
					}
					key := tableKey(current) + "." + exclusion.Name
					p.addConstraintRename(current, exclusion.PreviousName, exclusion.Name, "exclusion constraint", key)
					delete(prev, exclusion.PreviousName)
					continue
				}
				return unsupported("exclusion constraint " + tableKey(current) + "." + exclusion.Name + " previousName " + exclusion.PreviousName + " does not match any exclusion constraint in the previous snapshot")
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
		for _, name := range sortedStrings(removedNames(prev)) {
			key := tableKey(current) + "." + name
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindConstraint, key),
					"drop exclusion constraint "+key,
					migrateplan.SQL("ALTER TABLE "+renderTableName(current)+" DROP CONSTRAINT "+name+";"),
				).WithDependencies(
					migrateplan.Ref(migrateplan.ObjectKindTable, tableKey(current)),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskLockHeavy,
				).WithReverse(
					migrateplan.SQL("ALTER TABLE " + renderTableName(current) + " ADD " + renderExclusionConstraint(prev[name]) + ";"),
				),
			)
		}
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

func (p planner) addConstraintRename(table ast.Table, oldName, newName, kindLabel, key string) {
	p.addWith(
		migrateplan.NewChange(
			migrateplan.OperationRename,
			migrateplan.Ref(migrateplan.ObjectKindConstraint, key),
			"rename "+kindLabel+" "+tableKey(table)+"."+oldName+" to "+key,
			migrateplan.SQL(renderRenameConstraint(table.Schema, table.Name, oldName, newName)),
		).WithDependencies(
			migrateplan.Ref(migrateplan.ObjectKindTable, tableKey(table)),
		).WithReverse(
			migrateplan.SQL(renderReverseRenameConstraint(table.Schema, table.Name, newName, oldName)),
		),
	)
}

func (p planner) indexes(previous, current ast.Table) error {
	prev := mapBy(previous.Indexes, func(index ast.Index) string { return index.Name })
	for _, index := range sortedBy(current.Indexes, func(item ast.Index) string { return item.Name }) {
		old, ok := prev[index.Name]
		if !ok {
			if index.PreviousName != "" {
				oldIdx, hasOld := prev[index.PreviousName]
				if hasOld {
					if err := ensureRenameOnlyIndex(current, index, oldIdx); err != nil {
						return err
					}
					table := tableKey(current)
					key := table + "." + index.Name
					p.addWith(
						migrateplan.NewChange(
							migrateplan.OperationRename,
							migrateplan.Ref(migrateplan.ObjectKindIndex, key),
							"rename index "+table+"."+index.PreviousName+" to "+key,
							migrateplan.SQL(renderRenameIndex(current.Schema, index.PreviousName, index.Name)),
						).WithDependencies(
							migrateplan.Ref(migrateplan.ObjectKindTable, table),
						).WithReverse(
							migrateplan.SQL(renderReverseRenameIndex(current.Schema, index.Name, index.PreviousName)),
						),
					)
					delete(prev, index.PreviousName)
					continue
				}
				return unsupported("index " + tableKey(current) + "." + index.Name + " previousName " + index.PreviousName + " does not match any index in the previous snapshot")
			}
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
		for _, name := range sortedStrings(removedNames(prev)) {
			key := tableKey(current) + "." + name
			reverse, err := renderIndex(current, prev[name])
			if err != nil {
				return err
			}
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationDrop,
					migrateplan.Ref(migrateplan.ObjectKindIndex, key),
					"drop index "+key,
					migrateplan.SQL("DROP INDEX "+renderQualified(current.Schema, name)+";"),
				).WithDependencies(
					migrateplan.Ref(migrateplan.ObjectKindTable, tableKey(current)),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskLockHeavy,
				).WithReverse(
					migrateplan.SQL(reverse),
				),
			)
		}
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
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindTable, table),
					"drop comment from table "+table,
					migrateplan.SQL("COMMENT ON TABLE "+renderTableName(current)+" IS NULL;"),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
				).WithReverse(
					migrateplan.SQL("COMMENT ON TABLE " + renderTableName(current) + " IS " + quoteSQL(previous.Comment) + ";"),
				),
			)
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
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindColumn, key),
					"drop comment from column "+key,
					migrateplan.SQL("COMMENT ON COLUMN "+renderTableName(current)+"."+column.Name+" IS NULL;"),
				).WithDependencies(
					migrateplan.Ref(migrateplan.ObjectKindTable, table),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
				).WithReverse(
					migrateplan.SQL("COMMENT ON COLUMN " + renderTableName(current) + "." + column.Name + " IS " + quoteSQL(old.Comment) + ";"),
				),
			)
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
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindTable, table),
					"disable row level security on table "+table,
					migrateplan.SQL("ALTER TABLE "+renderTableName(current)+" DISABLE ROW LEVEL SECURITY;"),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
				).WithReverse(
					migrateplan.SQL("ALTER TABLE " + renderTableName(current) + " ENABLE ROW LEVEL SECURITY;"),
				),
			)
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
			p.addWith(
				migrateplan.NewChange(
					migrateplan.OperationAlter,
					migrateplan.Ref(migrateplan.ObjectKindTable, table),
					"unforce row level security on table "+table,
					migrateplan.SQL("ALTER TABLE "+renderTableName(current)+" NO FORCE ROW LEVEL SECURITY;"),
				).WithRisks(
					migrateplan.RiskDestructive, migrateplan.RiskDataLoss,
				).WithReverse(
					migrateplan.SQL("ALTER TABLE " + renderTableName(current) + " FORCE ROW LEVEL SECURITY;"),
				),
			)
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

func ensureRenameOnlyColumn(table ast.Table, column, oldColumn ast.Column) error {
	key := tableKey(table) + "." + column.Name
	oldKey := tableKey(table) + "." + oldColumn.Name
	renamed := oldColumn
	renamed.Name = column.Name
	renamed.PreviousName = ""
	current := column
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("column " + key + " rename from " + oldKey + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyPrimaryKey(table ast.Table, primaryKey, oldPrimaryKey ast.PrimaryKey) error {
	key := tableKey(table) + "." + primaryKey.Name
	oldKey := tableKey(table) + "." + oldPrimaryKey.Name
	renamed := oldPrimaryKey
	renamed.Name = primaryKey.Name
	renamed.PreviousName = ""
	current := primaryKey
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("primary key " + key + " rename from " + oldKey + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyUniqueConstraint(table ast.Table, unique, oldUnique ast.UniqueConstraint) error {
	key := tableKey(table) + "." + unique.Name
	oldKey := tableKey(table) + "." + oldUnique.Name
	renamed := oldUnique
	renamed.Name = unique.Name
	renamed.PreviousName = ""
	current := unique
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("unique constraint " + key + " rename from " + oldKey + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyForeignKey(table ast.Table, foreignKey, oldForeignKey ast.ForeignKeyConstraint) error {
	key := tableKey(table) + "." + foreignKey.Name
	oldKey := tableKey(table) + "." + oldForeignKey.Name
	renamed := oldForeignKey
	renamed.Name = foreignKey.Name
	renamed.PreviousName = ""
	current := foreignKey
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("foreign key " + key + " rename from " + oldKey + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyCheck(table ast.Table, check, oldCheck ast.Check) error {
	key := tableKey(table) + "." + check.Name
	oldKey := tableKey(table) + "." + oldCheck.Name
	renamed := oldCheck
	renamed.Name = check.Name
	renamed.PreviousName = ""
	current := check
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("check constraint " + key + " rename from " + oldKey + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyExclusion(table ast.Table, exclusion, oldExclusion ast.ExclusionConstraint) error {
	key := tableKey(table) + "." + exclusion.Name
	oldKey := tableKey(table) + "." + oldExclusion.Name
	renamed := oldExclusion
	renamed.Name = exclusion.Name
	renamed.PreviousName = ""
	current := exclusion
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("exclusion constraint " + key + " rename from " + oldKey + " combined with other modifications requires semantic planning")
	}
	return nil
}

func ensureRenameOnlyIndex(table ast.Table, index, oldIndex ast.Index) error {
	key := tableKey(table) + "." + index.Name
	oldKey := tableKey(table) + "." + oldIndex.Name
	renamed := oldIndex
	renamed.Name = index.Name
	renamed.PreviousName = ""
	current := index
	current.PreviousName = ""
	if !reflect.DeepEqual(renamed, current) {
		return unsupported("index " + key + " rename from " + oldKey + " combined with other modifications requires semantic planning")
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

func removedTables(prev map[string]ast.Table) []ast.Table {
	out := make([]ast.Table, 0, len(prev))
	for _, table := range prev {
		out = append(out, table)
	}
	return out
}

func sortedTables(tables []ast.Table) []ast.Table {
	return sortedBy(tables, tableKey)
}

func removedColumns(prev map[string]ast.Column) []string {
	out := make([]string, 0, len(prev))
	for name := range prev {
		out = append(out, name)
	}
	return out
}

func removedNames[T any](prev map[string]T) []string {
	out := make([]string, 0, len(prev))
	for name := range prev {
		out = append(out, name)
	}
	return out
}

func reverseCreateTable(table ast.Table) string {
	stmt, err := renderCreateTable(table)
	if err != nil {
		return ""
	}
	return stmt
}

func reverseAddColumn(table ast.Table, column ast.Column) string {
	def, err := renderColumn(column)
	if err != nil {
		return ""
	}
	return "ALTER TABLE " + renderTableName(table) + " ADD COLUMN " + def + ";"
}
