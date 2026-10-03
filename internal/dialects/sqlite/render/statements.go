package render

import (
	"fmt"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

// Statement-level renderers reused by the migration planner. They return a
// single statement terminated by ";" with no leading comment and no trailing
// newline, so the planner can compose them into migration files. The schema
// renderer shares the same underlying builders, so DDL stays identical across
// the generate and migrate surfaces.

// CreateTable renders a CREATE TABLE statement.
func CreateTable(table sqliteschema.Table) string {
	return trimStatement(createTableStatement(table))
}

// CreateIndex renders a CREATE [UNIQUE] INDEX statement for a table's index.
func CreateIndex(tableName string, index sqliteschema.Index) string {
	return trimStatement(renderIndex(tableName, index))
}

// CreateView renders a CREATE VIEW statement.
func CreateView(view sqliteschema.View) string {
	return trimStatement(createViewStatement(view))
}

// CreateTrigger renders a CREATE TRIGGER statement.
func CreateTrigger(trigger sqliteschema.Trigger) string {
	return trimStatement(createTriggerStatement(trigger))
}

// AddColumn renders an ALTER TABLE ... ADD COLUMN statement.
func AddColumn(tableName string, column sqliteschema.Column) string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", tableName, renderColumn(column))
}

// ColumnDefinition renders a single column definition (used by table rebuilds).
func ColumnDefinition(column sqliteschema.Column) string {
	return renderColumn(column)
}

func trimStatement(statement string) string {
	return strings.TrimRight(statement, "\n")
}
