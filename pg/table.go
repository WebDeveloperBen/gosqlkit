package pg

import "github.com/webdeveloperben/pgkit/internal/ast"

type Element interface {
	apply(table *ast.Table)
}

type Definition struct {
	def ast.Table
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

func Table(name string, elements ...Element) *Definition {
	table := ast.Table{Name: name}
	for _, element := range elements {
		element.apply(&table)
	}

	register(table)
	return &Definition{def: table}
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

func (i *IndexDef) Using(method string) *IndexDef {
	i.def.Method = method
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

func (f *ForeignKeyDef) References(table string, columns ...string) *ForeignKeyDef {
	f.def.ReferencedTable = table
	f.def.ReferencedColumns = append([]string(nil), columns...)
	return f
}

func (f *ForeignKeyDef) OnDelete(action string) *ForeignKeyDef {
	f.def.OnDelete = action
	return f
}

func (f *ForeignKeyDef) OnUpdate(action string) *ForeignKeyDef {
	f.def.OnUpdate = action
	return f
}

func (c *CheckDef) apply(table *ast.Table) {
	table.Checks = append(table.Checks, c.def)
}

func (i *IndexDef) apply(table *ast.Table) {
	table.Indexes = append(table.Indexes, i.def)
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
