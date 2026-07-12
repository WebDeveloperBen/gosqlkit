package sqlite

import (
	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

// Element is a table member (column, constraint, or index). Its apply method is
// unexported so members can only be attached by passing them to Table.
type Element interface {
	apply(table *sqliteschema.Table)
}

// Definition is a registered table handle supporting post-construction table
// options. It wraps a pointer so mutations persist in the registry.
type Definition struct {
	def *sqliteschema.Table
}

type CheckDef struct {
	def ast.Check
}

type PrimaryKeyDef struct {
	def ast.PrimaryKey
}

type UniqueConstraintDef struct {
	def ast.UniqueConstraint
}

type ForeignKeyDef struct {
	def ast.ForeignKeyConstraint
}

type IndexDef struct {
	def sqliteschema.Index
}

type IndexColumnDef struct {
	def sqliteschema.IndexColumn
}

func Table(name string, elements ...Element) *Definition {
	t := &sqliteschema.Table{}
	t.Name = name
	for _, element := range elements {
		element.apply(t)
	}
	register(t)
	return &Definition{def: t}
}

// WithoutRowID renders the table as WITHOUT ROWID. Such tables require a primary
// key (enforced by validation).
func (d *Definition) WithoutRowID() *Definition {
	d.def.WithoutRowID = true
	return d
}

// Strict renders the table as STRICT (strict type-affinity enforcement).
func (d *Definition) Strict() *Definition {
	d.def.Strict = true
	return d
}

func (d *Definition) Comment(text string) *Definition {
	d.def.Comment = text
	return d
}

func Check(name, expression string) *CheckDef {
	return &CheckDef{def: ast.Check{Name: name, Expression: expression}}
}

func (c *CheckDef) apply(table *sqliteschema.Table) {
	table.Checks = append(table.Checks, c.def)
}

func PrimaryKey(name string, columns ...string) *PrimaryKeyDef {
	return &PrimaryKeyDef{def: ast.PrimaryKey{Name: name, Columns: columns}}
}

func (p *PrimaryKeyDef) apply(table *sqliteschema.Table) {
	table.PrimaryKeys = append(table.PrimaryKeys, p.def)
}

func Unique(name string, columns ...string) *UniqueConstraintDef {
	return &UniqueConstraintDef{def: ast.UniqueConstraint{Name: name, Columns: columns}}
}

func (u *UniqueConstraintDef) apply(table *sqliteschema.Table) {
	table.UniqueConstraints = append(table.UniqueConstraints, u.def)
}

func ForeignKey(name string, columns ...string) *ForeignKeyDef {
	return &ForeignKeyDef{def: ast.ForeignKeyConstraint{Name: name, Columns: columns}}
}

func (f *ForeignKeyDef) References(table string, columns ...string) *ForeignKeyDef {
	f.def.ReferencedTable = table
	f.def.ReferencedColumns = columns
	return f
}

func (f *ForeignKeyDef) OnDelete(action ForeignKeyAction) *ForeignKeyDef {
	f.def.OnDelete = string(action)
	return f
}

func (f *ForeignKeyDef) OnUpdate(action ForeignKeyAction) *ForeignKeyDef {
	f.def.OnUpdate = string(action)
	return f
}

func (f *ForeignKeyDef) Deferrable() *ForeignKeyDef {
	f.def.Deferrable = true
	return f
}

func (f *ForeignKeyDef) InitiallyDeferred() *ForeignKeyDef {
	f.def.Deferrable = true
	f.def.Initially = "deferred"
	return f
}

func (f *ForeignKeyDef) InitiallyImmediate() *ForeignKeyDef {
	f.def.Deferrable = true
	f.def.Initially = "immediate"
	return f
}

func (f *ForeignKeyDef) apply(table *sqliteschema.Table) {
	table.ForeignKeys = append(table.ForeignKeys, f.def)
}

func Index(name string, columns ...string) *IndexDef {
	return indexOn(name, false, indexColumns(columns)...)
}

func UniqueIndex(name string, columns ...string) *IndexDef {
	return indexOn(name, true, indexColumns(columns)...)
}

func IndexOn(name string, columns ...*IndexColumnDef) *IndexDef {
	return indexOn(name, false, columns...)
}

func UniqueIndexOn(name string, columns ...*IndexColumnDef) *IndexDef {
	return indexOn(name, true, columns...)
}

// IndexColumn references a table column in an index.
func IndexColumn(name string) *IndexColumnDef {
	return &IndexColumnDef{def: sqliteschema.IndexColumn{IndexColumn: ast.IndexColumn{Expression: name}}}
}

// IndexExpr indexes an arbitrary SQL expression.
func IndexExpr(expression string) *IndexColumnDef {
	return &IndexColumnDef{def: sqliteschema.IndexColumn{IndexColumn: ast.IndexColumn{Expression: expression, IsExpression: true}}}
}

func (c *IndexColumnDef) Asc() *IndexColumnDef {
	c.def.Order = "ASC"
	return c
}

func (c *IndexColumnDef) Desc() *IndexColumnDef {
	c.def.Order = "DESC"
	return c
}

func (c *IndexColumnDef) Collate(name string) *IndexColumnDef {
	c.def.Collation = name
	return c
}

func (i *IndexDef) Where(predicate string) *IndexDef {
	i.def.Where = predicate
	return i
}

func (i *IndexDef) apply(table *sqliteschema.Table) {
	table.Indexes = append(table.Indexes, i.def)
}

func indexOn(name string, unique bool, columns ...*IndexColumnDef) *IndexDef {
	cols := make([]sqliteschema.IndexColumn, 0, len(columns))
	for _, column := range columns {
		cols = append(cols, column.def)
	}
	index := &IndexDef{def: sqliteschema.Index{Index: ast.Index{Name: name, Unique: unique}, Columns: cols}}
	return index
}

func indexColumns(names []string) []*IndexColumnDef {
	columns := make([]*IndexColumnDef, 0, len(names))
	for _, name := range names {
		columns = append(columns, IndexColumn(name))
	}
	return columns
}
