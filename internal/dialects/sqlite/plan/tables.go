package plan

import (
	"fmt"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/render"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func (p planner) tables(previous, current []sqliteschema.Table) error {
	prev := migrateplan.MapBy(previous, tableName)
	for _, table := range migrateplan.SortedBy(current, tableName) {
		old, ok := prev[table.Name]
		if !ok {
			if table.PreviousName != "" {
				oldTable, has := prev[table.PreviousName]
				if !has {
					return unsupported(fmt.Sprintf("table %q declares previousName %q but no such table exists in the previous snapshot", table.Name, table.PreviousName))
				}
				if err := p.renameTable(oldTable, table); err != nil {
					return err
				}
				delete(prev, table.PreviousName)
				continue
			}
			p.createTable(table)
			continue
		}
		if err := p.alterTable(old, table); err != nil {
			return err
		}
		delete(prev, table.Name)
	}
	for _, name := range mapKeys(prev) {
		p.dropTable(prev[name])
	}
	return nil
}

func (p planner) createTable(table sqliteschema.Table) {
	change := migrateplan.NewChange(
		migrateplan.OperationCreate,
		migrateplan.Ref(migrateplan.ObjectKindTable, table.Name),
		"create table "+table.Name,
		migrateplan.SQL(render.CreateTable(table)),
	).WithReverse(migrateplan.SQL("DROP TABLE " + table.Name + ";"))
	for _, dep := range tableDependencies(table) {
		change = change.WithDependencies(migrateplan.Ref(migrateplan.ObjectKindTable, dep))
	}
	p.addWith(change)
	for _, index := range table.Indexes {
		p.createIndex(table.Name, index)
	}
}

func (p planner) dropTable(table sqliteschema.Table) {
	change := migrateplan.NewChange(
		migrateplan.OperationDrop,
		migrateplan.Ref(migrateplan.ObjectKindTable, table.Name),
		"drop table "+table.Name,
		migrateplan.SQL("DROP TABLE "+table.Name+";"),
	).WithRisks(migrateplan.RiskDestructive, migrateplan.RiskDataLoss).
		WithReverse(reverseCreateTable(table)...)
	for _, dep := range tableDependencies(table) {
		change = change.WithDependencies(migrateplan.Ref(migrateplan.ObjectKindTable, dep))
	}
	p.addWith(change)
}

func (p planner) renameTable(old, current sqliteschema.Table) error {
	renamed := old
	renamed.Name = current.Name
	renamed.PreviousName = current.PreviousName
	if !jsonEqual(tableComparable(renamed), tableComparable(current)) {
		return unsupported(fmt.Sprintf("table rename %q -> %q combined with other changes is not supported; author the rename and the other changes as separate migrations", old.Name, current.Name))
	}
	p.addWith(migrateplan.NewChange(
		migrateplan.OperationRename,
		migrateplan.Ref(migrateplan.ObjectKindTable, current.Name),
		fmt.Sprintf("rename table %s to %s", old.Name, current.Name),
		migrateplan.SQL(fmt.Sprintf("ALTER TABLE %s RENAME TO %s;", old.Name, current.Name)),
	).WithReverse(migrateplan.SQL(fmt.Sprintf("ALTER TABLE %s RENAME TO %s;", current.Name, old.Name))))
	return nil
}

func (p planner) alterTable(old, current sqliteschema.Table) error {
	if columnsComparableEqual(old, current) && constraintsAndOptionsEqual(old, current) {
		return p.diffIndexes(old, current)
	}
	if added, ok := addOnlyColumns(old, current); ok {
		for _, column := range added {
			p.addColumn(current.Name, column)
		}
		return p.diffIndexes(old, current)
	}
	if renames, ok := renameOnlyColumns(old, current); ok {
		for _, rename := range renames {
			p.renameColumn(current.Name, rename.from, rename.to)
		}
		return p.diffIndexes(old, current)
	}
	p.rebuildTable(old, current)
	return nil
}

func (p planner) addColumn(table string, column sqliteschema.Column) {
	p.addWith(migrateplan.NewChange(
		migrateplan.OperationAlter,
		migrateplan.Ref(migrateplan.ObjectKindColumn, table+"."+column.Name),
		fmt.Sprintf("add column %s.%s", table, column.Name),
		migrateplan.SQL(render.AddColumn(table, column)),
	).WithDependencies(migrateplan.Ref(migrateplan.ObjectKindTable, table)).
		WithReverse(migrateplan.SQL(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", table, column.Name))))
}

func (p planner) renameColumn(table, from, to string) {
	p.addWith(migrateplan.NewChange(
		migrateplan.OperationRename,
		migrateplan.Ref(migrateplan.ObjectKindColumn, table+"."+to),
		fmt.Sprintf("rename column %s.%s to %s", table, from, to),
		migrateplan.SQL(fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", table, from, to)),
	).WithDependencies(migrateplan.Ref(migrateplan.ObjectKindTable, table)).
		WithReverse(migrateplan.SQL(fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", table, to, from))))
}

func (p planner) diffIndexes(old, current sqliteschema.Table) error {
	oldIdx := migrateplan.MapBy(old.Indexes, indexName)
	for _, index := range migrateplan.SortedBy(current.Indexes, indexName) {
		if index.PreviousName != "" {
			oldIndex, found := oldIdx[index.PreviousName]
			if !found || index.PreviousName == index.Name {
				return unsupported(fmt.Sprintf("index %q declares unmatched previousName %q", index.Name, index.PreviousName))
			}
			if _, found := oldIdx[index.Name]; found {
				return unsupported(fmt.Sprintf("index rename %q -> %q targets an existing previous index", oldIndex.Name, index.Name))
			}
			if !jsonEqual(indexComparable(oldIndex), indexComparable(index)) {
				return unsupported(fmt.Sprintf("index rename %q -> %q combined with definition changes is not supported", oldIndex.Name, index.Name))
			}
			p.renameIndex(current.Name, oldIndex, index)
			delete(oldIdx, index.PreviousName)
			continue
		}
		prevIndex, ok := oldIdx[index.Name]
		if !ok {
			p.createIndex(current.Name, index)
			continue
		}
		if !jsonEqual(indexComparable(prevIndex), indexComparable(index)) {
			p.dropIndex(current.Name, prevIndex)
			p.createIndex(current.Name, index)
		}
		delete(oldIdx, index.Name)
	}
	for _, name := range mapKeys(oldIdx) {
		p.dropIndex(current.Name, oldIdx[name])
	}
	return nil
}

// rebuildTable performs the generalised 12-step ALTER TABLE for changes SQLite
// cannot do in place. It copies data into a new table built from the desired
// schema, swaps it in, and recreates indexes, all wrapped in a foreign-key
// pragma guard.
func (p planner) rebuildTable(old, current sqliteschema.Table) {
	// A rebuild drops and recreates the table, which also drops any triggers
	// defined on it. The rebuild therefore owns the table's triggers in both
	// directions, and the trigger pass skips rebuilt tables.
	p.rebuiltTables[current.Name] = true

	forward := rebuildStatements(current, columnSourceMap(old, current))
	forward = append(forward, triggerStatements(p.currentTriggers, current.Name)...)

	change := migrateplan.NewChange(
		migrateplan.OperationAlter,
		migrateplan.Ref(migrateplan.ObjectKindTable, current.Name),
		"rebuild table "+current.Name,
		forward...,
	).WithRisks(migrateplan.RiskLockHeavy, migrateplan.RiskRequiresDDLReview)
	if columnsDropped(old, current) {
		change = change.WithRisks(migrateplan.RiskDestructive, migrateplan.RiskDataLoss)
	}
	reverse := rebuildStatements(old, reverseColumnSourceMap(current, old))
	reverse = append(reverse, triggerStatements(p.previousTriggers, old.Name)...)
	change = change.WithReverse(reverse...)
	p.addWith(change)
}

func triggerStatements(triggers []sqliteschema.Trigger, target string) []migrateplan.Statement {
	out := make([]migrateplan.Statement, 0)
	for _, trigger := range migrateplan.SortedBy(triggers, triggerName) {
		if trigger.Target == target {
			out = append(out, migrateplan.SQL(render.CreateTrigger(trigger)))
		}
	}
	return out
}

func rebuildStatements(target sqliteschema.Table, sources map[string]string) []migrateplan.Statement {
	tempName := target.Name + "_gosqlkit_new"
	temp := target
	temp.Name = tempName

	var targetCols, sourceCols []string
	for _, column := range target.Columns {
		src, ok := sources[column.Name]
		if !ok {
			continue
		}
		targetCols = append(targetCols, column.Name)
		sourceCols = append(sourceCols, src)
	}

	statements := []migrateplan.Statement{
		migrateplan.SQL("PRAGMA foreign_keys=OFF;"),
		migrateplan.SQL(render.CreateTable(temp)),
	}
	if len(targetCols) > 0 {
		statements = append(statements, migrateplan.SQL(fmt.Sprintf(
			"INSERT INTO %s (%s) SELECT %s FROM %s;",
			tempName, strings.Join(targetCols, ", "), strings.Join(sourceCols, ", "), target.Name,
		)))
	}
	statements = append(
		statements,
		migrateplan.SQL("DROP TABLE "+target.Name+";"),
		migrateplan.SQL(fmt.Sprintf("ALTER TABLE %s RENAME TO %s;", tempName, target.Name)),
	)
	for _, index := range target.Indexes {
		statements = append(statements, migrateplan.SQL(render.CreateIndex(target.Name, index)))
	}
	statements = append(statements, migrateplan.SQL("PRAGMA foreign_keys=ON;"))
	return statements
}

func reverseCreateTable(table sqliteschema.Table) []migrateplan.Statement {
	statements := []migrateplan.Statement{migrateplan.SQL(render.CreateTable(table))}
	for _, index := range table.Indexes {
		statements = append(statements, migrateplan.SQL(render.CreateIndex(table.Name, index)))
	}
	return statements
}

// columnSourceMap maps each target (current) column to the previous column it
// copies from, following rename metadata. Generated columns and brand-new
// columns have no source and are skipped.
func columnSourceMap(old, current sqliteschema.Table) map[string]string {
	oldByName := columnNameSet(old)
	out := make(map[string]string, len(current.Columns))
	for _, column := range current.Columns {
		if column.Generated != nil {
			continue
		}
		src := ""
		if column.PreviousName != "" && oldByName[column.PreviousName] {
			src = column.PreviousName
		} else if oldByName[column.Name] {
			src = column.Name
		}
		if src == "" {
			continue
		}
		out[column.Name] = src
	}
	return out
}

// reverseColumnSourceMap maps old columns back from the current table by name
// (best-effort; renamed columns reverse only where names still match).
func reverseColumnSourceMap(current, old sqliteschema.Table) map[string]string {
	currentByName := columnNameSet(current)
	out := make(map[string]string, len(old.Columns))
	for _, column := range old.Columns {
		if column.Generated != nil {
			continue
		}
		if currentByName[column.Name] {
			out[column.Name] = column.Name
		}
	}
	return out
}

func columnsDropped(old, current sqliteschema.Table) bool {
	kept := make(map[string]struct{}, len(current.Columns))
	for _, column := range current.Columns {
		kept[column.Name] = struct{}{}
		if column.PreviousName != "" {
			kept[column.PreviousName] = struct{}{}
		}
	}
	for _, column := range old.Columns {
		if _, ok := kept[column.Name]; !ok {
			return true
		}
	}
	return false
}

type columnRename struct {
	from string
	to   string
}

func addOnlyColumns(old, current sqliteschema.Table) ([]sqliteschema.Column, bool) {
	if !constraintsAndOptionsEqual(old, current) {
		return nil, false
	}
	oldByName := columnMap(old)
	currentByName := columnMap(current)

	for name, oldColumn := range oldByName {
		newColumn, ok := currentByName[name]
		if !ok {
			return nil, false // dropped
		}
		if !columnDefEqual(oldColumn, newColumn) {
			return nil, false // changed
		}
	}

	added := make([]sqliteschema.Column, 0)
	for _, column := range current.Columns {
		if _, ok := oldByName[column.Name]; ok {
			continue
		}
		if column.PreviousName != "" {
			if _, ok := oldByName[column.PreviousName]; ok {
				return nil, false // rename, not add
			}
		}
		if !addColumnSafe(column) {
			return nil, false
		}
		added = append(added, column)
	}
	return added, len(added) > 0
}

func renameOnlyColumns(old, current sqliteschema.Table) ([]columnRename, bool) {
	if !constraintsAndOptionsEqual(old, current) {
		return nil, false
	}
	if len(old.Columns) != len(current.Columns) {
		return nil, false
	}
	oldByName := columnMap(old)
	renames := make([]columnRename, 0)
	for _, column := range current.Columns {
		src := column.Name
		if column.PreviousName != "" {
			if _, ok := oldByName[column.PreviousName]; ok {
				src = column.PreviousName
				renames = append(renames, columnRename{from: column.PreviousName, to: column.Name})
			}
		}
		oldColumn, ok := oldByName[src]
		if !ok {
			return nil, false
		}
		if !columnDefEqual(oldColumn, column) {
			return nil, false
		}
		delete(oldByName, src)
	}
	if len(oldByName) != 0 {
		return nil, false
	}
	return renames, len(renames) > 0
}

func addColumnSafe(column sqliteschema.Column) bool {
	if column.PrimaryKey || column.Unique {
		return false
	}
	if column.Generated != nil && strings.EqualFold(column.Generated.Type, "stored") {
		return false
	}
	if column.NotNull && column.Default == "" && column.Generated == nil {
		return false
	}
	if nonConstantDefault(column.Default) {
		return false
	}
	return true
}

func nonConstantDefault(def string) bool {
	def = strings.TrimSpace(def)
	if def == "" {
		return false
	}
	switch strings.ToUpper(def) {
	case "CURRENT_TIME", "CURRENT_DATE", "CURRENT_TIMESTAMP":
		return true
	}
	// Parenthesised expression defaults are not constant for ADD COLUMN.
	return strings.HasPrefix(def, "(")
}

func indexName(index sqliteschema.Index) string { return index.Name }

func indexComparable(index sqliteschema.Index) sqliteschema.Index {
	index.Name = ""
	index.PreviousName = ""
	index.IfNotExists = false
	return index
}

func tableComparable(table sqliteschema.Table) sqliteschema.Table {
	table.Name = ""
	table.PreviousName = ""
	table.Comment = ""
	table.IfNotExists = false
	return table
}

func columnsComparableEqual(old, current sqliteschema.Table) bool {
	return jsonEqual(comparableColumns(old.Columns), comparableColumns(current.Columns))
}

func comparableColumns(columns []sqliteschema.Column) []sqliteschema.Column {
	out := make([]sqliteschema.Column, len(columns))
	for i, column := range columns {
		out[i] = comparableColumn(column)
	}
	return out
}

func comparableColumn(column sqliteschema.Column) sqliteschema.Column {
	column.PreviousName = ""
	column.Comment = ""
	return column
}

func columnDefEqual(a, b sqliteschema.Column) bool {
	a.Name = ""
	b.Name = ""
	return jsonEqual(comparableColumn(a), comparableColumn(b))
}

func constraintsAndOptionsEqual(old, current sqliteschema.Table) bool {
	if old.WithoutRowID != current.WithoutRowID || old.Strict != current.Strict {
		return false
	}
	return jsonEqual(old.PrimaryKeys, current.PrimaryKeys) &&
		jsonEqual(old.UniqueConstraints, current.UniqueConstraints) &&
		jsonEqual(old.ForeignKeys, current.ForeignKeys) &&
		jsonEqual(old.Checks, current.Checks)
}

func columnMap(table sqliteschema.Table) map[string]sqliteschema.Column {
	out := make(map[string]sqliteschema.Column, len(table.Columns))
	for _, column := range table.Columns {
		out[column.Name] = column
	}
	return out
}

func columnNameSet(table sqliteschema.Table) map[string]bool {
	out := make(map[string]bool, len(table.Columns))
	for _, column := range table.Columns {
		out[column.Name] = true
	}
	return out
}
