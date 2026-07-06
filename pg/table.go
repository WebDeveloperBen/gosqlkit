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
	return &IndexDef{
		def: ast.Index{
			Name:    name,
			Columns: append([]string(nil), columns...),
		},
	}
}

func UniqueIndex(name string, columns ...string) *IndexDef {
	index := Index(name, columns...)
	index.def.Unique = true
	return index
}

func (c *CheckDef) apply(table *ast.Table) {
	table.Checks = append(table.Checks, c.def)
}

func (i *IndexDef) apply(table *ast.Table) {
	table.Indexes = append(table.Indexes, i.def)
}
