package pg

import (
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
)

type Element interface {
	apply(table *ast.Table)
}

type Definition struct {
	def *ast.Table
}

type CheckDef struct {
	def ast.Check
}

type IndexDef struct {
	def ast.Index
}

type IndexColumnDef struct {
	def ast.IndexColumn
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

type ExclusionDef struct {
	def ast.ExclusionConstraint
}

type ExclusionElementDef struct {
	def ast.ExclusionElement
}

func Table(name string, elements ...Element) *Definition {
	return table("", name, elements...)
}

func TableInSchema(schema, name string, elements ...Element) *Definition {
	return table(schema, name, elements...)
}

func table(schema, name string, elements ...Element) *Definition {
	t := &ast.Table{Schema: schema, Name: name}
	for _, element := range elements {
		element.apply(t)
	}

	register(t)
	return &Definition{def: t}
}

func Check(name, expression string) *CheckDef {
	return &CheckDef{
		def: ast.Check{
			Name:       name,
			Expression: expression,
		},
	}
}

func Index(name string, columns ...string) *IndexDef {
	indexColumns := make([]ast.IndexColumn, 0, len(columns))
	for _, column := range columns {
		indexColumns = append(indexColumns, ast.IndexColumn{Expression: column})
	}
	return &IndexDef{
		def: ast.Index{
			Name:    name,
			Columns: indexColumns,
		},
	}
}

func UniqueIndex(name string, columns ...string) *IndexDef {
	index := Index(name, columns...)
	index.def.Unique = true
	return index
}

func IndexOn(name string, columns ...*IndexColumnDef) *IndexDef {
	return indexOn(name, false, columns...)
}

func UniqueIndexOn(name string, columns ...*IndexColumnDef) *IndexDef {
	return indexOn(name, true, columns...)
}

func IndexColumn(name string) *IndexColumnDef {
	return &IndexColumnDef{
		def: ast.IndexColumn{Expression: name},
	}
}

func IndexExpression(expression string) *IndexColumnDef {
	return &IndexColumnDef{
		def: ast.IndexColumn{
			Expression:   expression,
			IsExpression: true,
		},
	}
}

func (i *IndexDef) Using(method IndexMethod) *IndexDef {
	i.def.Method = string(method)
	return i
}

func (i *IndexDef) Where(expression string) *IndexDef {
	i.def.Where = expression
	return i
}

func (i *IndexDef) Concurrently() *IndexDef {
	i.def.Concurrently = true
	return i
}

func (i *IndexDef) Only() *IndexDef {
	i.def.Only = true
	return i
}

func (i *IndexDef) With(key, value string) *IndexDef {
	if i.def.With == nil {
		i.def.With = map[string]string{}
	}
	i.def.With[key] = value
	return i
}

func (c *IndexColumnDef) Asc() *IndexColumnDef {
	c.def.Order = "ASC"
	return c
}

func (c *IndexColumnDef) Desc() *IndexColumnDef {
	c.def.Order = "DESC"
	return c
}

func (c *IndexColumnDef) NullsFirst() *IndexColumnDef {
	c.def.Nulls = "FIRST"
	return c
}

func (c *IndexColumnDef) NullsLast() *IndexColumnDef {
	c.def.Nulls = "LAST"
	return c
}

func (c *IndexColumnDef) OpClass(opClass string) *IndexColumnDef {
	c.def.OpClass = opClass
	return c
}

func PrimaryKey(name string, columns ...string) *PrimaryKeyDef {
	return &PrimaryKeyDef{
		def: ast.PrimaryKey{
			Name:    name,
			Columns: append([]string(nil), columns...),
		},
	}
}

func Unique(name string, columns ...string) *UniqueConstraintDef {
	return &UniqueConstraintDef{
		def: ast.UniqueConstraint{
			Name:    name,
			Columns: append([]string(nil), columns...),
		},
	}
}

func ForeignKey(name string, columns ...string) *ForeignKeyDef {
	return &ForeignKeyDef{
		def: ast.ForeignKeyConstraint{
			Name:    name,
			Columns: append([]string(nil), columns...),
		},
	}
}

func (u *UniqueConstraintDef) NullsNotDistinct() *UniqueConstraintDef {
	u.def.NullsNotDistinct = true
	return u
}

func (u *UniqueConstraintDef) Deferrable() *UniqueConstraintDef {
	u.def.Deferrable = true
	if u.def.Initially == "" {
		u.def.Initially = "IMMEDIATE"
	}
	return u
}

func (u *UniqueConstraintDef) InitiallyDeferred() *UniqueConstraintDef {
	u.def.Deferrable = true
	u.def.Initially = "DEFERRED"
	return u
}

func (u *UniqueConstraintDef) InitiallyImmediate() *UniqueConstraintDef {
	u.def.Deferrable = true
	u.def.Initially = "IMMEDIATE"
	return u
}

func (f *ForeignKeyDef) References(table string, columns ...string) *ForeignKeyDef {
	f.def.ReferencedTable = table
	f.def.ReferencedColumns = append([]string(nil), columns...)
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
	if f.def.Initially == "" {
		f.def.Initially = "IMMEDIATE"
	}
	return f
}

func (f *ForeignKeyDef) InitiallyDeferred() *ForeignKeyDef {
	f.def.Deferrable = true
	f.def.Initially = "DEFERRED"
	return f
}

func (f *ForeignKeyDef) InitiallyImmediate() *ForeignKeyDef {
	f.def.Deferrable = true
	f.def.Initially = "IMMEDIATE"
	return f
}

func Exclusion(name string, elements ...*ExclusionElementDef) *ExclusionDef {
	exclusionElements := make([]ast.ExclusionElement, 0, len(elements))
	for _, element := range elements {
		exclusionElements = append(exclusionElements, element.def)
	}
	return &ExclusionDef{
		def: ast.ExclusionConstraint{
			Name:     name,
			Elements: exclusionElements,
		},
	}
}

func ExcludeWith(expression, operator string) *ExclusionElementDef {
	return &ExclusionElementDef{
		def: ast.ExclusionElement{
			Expression: expression,
			Operator:   operator,
		},
	}
}

func (e *ExclusionElementDef) OpClass(opClass string) *ExclusionElementDef {
	e.def.OpClass = opClass
	return e
}

func (e *ExclusionElementDef) Asc() *ExclusionElementDef {
	e.def.Order = "ASC"
	return e
}

func (e *ExclusionElementDef) Desc() *ExclusionElementDef {
	e.def.Order = "DESC"
	return e
}

func (e *ExclusionElementDef) NullsFirst() *ExclusionElementDef {
	e.def.Nulls = "FIRST"
	return e
}

func (e *ExclusionElementDef) NullsLast() *ExclusionElementDef {
	e.def.Nulls = "LAST"
	return e
}

func (e *ExclusionDef) Using(method IndexMethod) *ExclusionDef {
	e.def.Method = string(method)
	return e
}

func (e *ExclusionDef) Where(expression string) *ExclusionDef {
	e.def.Where = expression
	return e
}

func (e *ExclusionDef) Deferrable() *ExclusionDef {
	e.def.Deferrable = true
	if e.def.Initially == "" {
		e.def.Initially = "IMMEDIATE"
	}
	return e
}

func (e *ExclusionDef) InitiallyDeferred() *ExclusionDef {
	e.def.Deferrable = true
	e.def.Initially = "DEFERRED"
	return e
}

func (d *Definition) Comment(text string) *Definition {
	d.def.Comment = text
	return d
}

func (d *Definition) EnableRLS() *Definition {
	d.def.RowLevelSecurity = true
	return d
}

func (d *Definition) ForceRLS() *Definition {
	d.def.RowLevelSecurity = true
	d.def.ForceRLS = true
	return d
}

func (c *CheckDef) apply(table *ast.Table) {
	table.Checks = append(table.Checks, c.def)
}

func (i *IndexDef) apply(table *ast.Table) {
	if i.def.Name == "" {
		i.def.Name = autoIndexName(table.Name, i.def)
	}
	table.Indexes = append(table.Indexes, i.def)
}

func autoIndexName(tableName string, index ast.Index) string {
	parts := make([]string, 0, len(index.Columns)+2)
	parts = append(parts, tableName)
	for _, column := range index.Columns {
		if column.IsExpression {
			panic("index with expression columns requires an explicit name; use IndexOn() or UniqueIndexOn() instead of Index()")
		}
		parts = append(parts, column.Expression)
	}
	parts = append(parts, "idx")
	return strings.Join(parts, "_")
}

func indexOn(name string, unique bool, columns ...*IndexColumnDef) *IndexDef {
	indexColumns := make([]ast.IndexColumn, 0, len(columns))
	for _, column := range columns {
		indexColumns = append(indexColumns, column.def)
	}
	return &IndexDef{
		def: ast.Index{
			Name:    name,
			Columns: indexColumns,
			Unique:  unique,
		},
	}
}

func (p *PrimaryKeyDef) apply(table *ast.Table) {
	table.PrimaryKeys = append(table.PrimaryKeys, p.def)
}

func (u *UniqueConstraintDef) apply(table *ast.Table) {
	table.UniqueConstraints = append(table.UniqueConstraints, u.def)
}

func (f *ForeignKeyDef) apply(table *ast.Table) {
	table.ForeignKeys = append(table.ForeignKeys, f.def)
}

func (e *ExclusionDef) apply(table *ast.Table) {
	table.Exclusions = append(table.Exclusions, e.def)
}
