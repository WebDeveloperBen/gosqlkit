package pg

import (
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
)

type Element interface {
	apply(table *pgschema.Table)
}

type Definition struct {
	def *pgschema.Table
}

type CheckDef struct {
	def ast.Check
}

type IndexDef struct {
	def pgschema.Index
}

type IndexColumnDef struct {
	def pgschema.IndexColumn
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
	def pgschema.ExclusionConstraint
}

type ExclusionElementDef struct {
	def pgschema.ExclusionElement
}

type PartitionDef struct {
	def pgschema.Partitioning
}

type PartitionKeyDef struct {
	def pgschema.PartitionKey
}

type PartitionOfDef struct {
	def pgschema.PartitionOf
}

type PartitionBoundDef struct {
	def pgschema.PartitionBound
}

func Table(name string, elements ...Element) *Definition {
	return table("", name, elements...)
}

func TableInSchema(schema, name string, elements ...Element) *Definition {
	return table(schema, name, elements...)
}

func table(schema, name string, elements ...Element) *Definition {
	t := &pgschema.Table{}
	t.Schema = schema
	t.Name = name
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
	indexColumns := make([]pgschema.IndexColumn, 0, len(columns))
	for _, column := range columns {
		indexColumns = append(indexColumns, pgschema.IndexColumn{IndexColumn: ast.IndexColumn{Expression: column}})
	}
	return &IndexDef{
		def: pgschema.Index{
			Index:   ast.Index{Name: name},
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
		def: pgschema.IndexColumn{IndexColumn: ast.IndexColumn{Expression: name}},
	}
}

func IndexExpression(expression string) *IndexColumnDef {
	return &IndexColumnDef{
		def: pgschema.IndexColumn{
			IndexColumn: ast.IndexColumn{
				Expression:   expression,
				IsExpression: true,
			},
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
	exclusionElements := make([]pgschema.ExclusionElement, 0, len(elements))
	for _, element := range elements {
		exclusionElements = append(exclusionElements, element.def)
	}
	return &ExclusionDef{
		def: pgschema.ExclusionConstraint{
			ExclusionConstraint: ast.ExclusionConstraint{Name: name},
			Elements:            exclusionElements,
		},
	}
}

func ExcludeWith(expression, operator string) *ExclusionElementDef {
	return &ExclusionElementDef{
		def: pgschema.ExclusionElement{
			ExclusionElement: ast.ExclusionElement{Expression: expression, Operator: operator},
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

func PartitionByRange(columns ...string) *PartitionDef {
	return partitionBy("range", columns...)
}

func PartitionByRangeOn(keys ...*PartitionKeyDef) *PartitionDef {
	return partitionByOn("range", keys...)
}

func PartitionByList(columns ...string) *PartitionDef {
	return partitionBy("list", columns...)
}

func PartitionByListOn(keys ...*PartitionKeyDef) *PartitionDef {
	return partitionByOn("list", keys...)
}

func PartitionByHash(columns ...string) *PartitionDef {
	return partitionBy("hash", columns...)
}

func PartitionByHashOn(keys ...*PartitionKeyDef) *PartitionDef {
	return partitionByOn("hash", keys...)
}

func PartitionColumn(name string) *PartitionKeyDef {
	return &PartitionKeyDef{def: pgschema.PartitionKey{Expression: name}}
}

func PartitionExpression(expression string) *PartitionKeyDef {
	return &PartitionKeyDef{
		def: pgschema.PartitionKey{
			Expression:   expression,
			IsExpression: true,
		},
	}
}

func (d *Definition) PartitionByRange(columns ...string) *Definition {
	d.def.Partitioning = partitionBy("range", columns...).partitioning()
	return d
}

func (d *Definition) PartitionByRangeOn(keys ...*PartitionKeyDef) *Definition {
	d.def.Partitioning = partitionByOn("range", keys...).partitioning()
	return d
}

func (d *Definition) PartitionByList(columns ...string) *Definition {
	d.def.Partitioning = partitionBy("list", columns...).partitioning()
	return d
}

func (d *Definition) PartitionByListOn(keys ...*PartitionKeyDef) *Definition {
	d.def.Partitioning = partitionByOn("list", keys...).partitioning()
	return d
}

func (d *Definition) PartitionByHash(columns ...string) *Definition {
	d.def.Partitioning = partitionBy("hash", columns...).partitioning()
	return d
}

func (d *Definition) PartitionByHashOn(keys ...*PartitionKeyDef) *Definition {
	d.def.Partitioning = partitionByOn("hash", keys...).partitioning()
	return d
}

func PartitionValues(values ...string) []string {
	return append([]string(nil), values...)
}

func ForValuesFromTo(from, to []string) *PartitionBoundDef {
	return &PartitionBoundDef{
		def: pgschema.PartitionBound{
			Type: "range",
			From: append([]string(nil), from...),
			To:   append([]string(nil), to...),
		},
	}
}

func ForValuesIn(values ...string) *PartitionBoundDef {
	return &PartitionBoundDef{
		def: pgschema.PartitionBound{
			Type:   "list",
			Values: append([]string(nil), values...),
		},
	}
}

func ForValuesWith(modulus, remainder int) *PartitionBoundDef {
	return &PartitionBoundDef{
		def: pgschema.PartitionBound{
			Type:      "hash",
			Modulus:   modulus,
			Remainder: remainder,
		},
	}
}

func ForValuesDefault() *PartitionBoundDef {
	return &PartitionBoundDef{def: pgschema.PartitionBound{Type: "default"}}
}

func PartitionOf(parent string, bound *PartitionBoundDef) *PartitionOfDef {
	def := pgschema.PartitionOf{Parent: parent}
	if bound != nil {
		def.Bound = bound.partitionBound()
	}
	return &PartitionOfDef{def: def}
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

func (c *CheckDef) apply(table *pgschema.Table) {
	table.Checks = append(table.Checks, c.def)
}

func (i *IndexDef) apply(table *pgschema.Table) {
	if i.def.Name == "" {
		i.def.Name = autoIndexName(table.Name, i.def)
	}
	table.Indexes = append(table.Indexes, i.def)
}

func autoIndexName(tableName string, index pgschema.Index) string {
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
	indexColumns := make([]pgschema.IndexColumn, 0, len(columns))
	for _, column := range columns {
		indexColumns = append(indexColumns, column.def)
	}
	return &IndexDef{
		def: pgschema.Index{
			Index:   ast.Index{Name: name, Unique: unique},
			Columns: indexColumns,
		},
	}
}

func (p *PrimaryKeyDef) apply(table *pgschema.Table) {
	table.PrimaryKeys = append(table.PrimaryKeys, p.def)
}

func (u *UniqueConstraintDef) apply(table *pgschema.Table) {
	table.UniqueConstraints = append(table.UniqueConstraints, u.def)
}

func (f *ForeignKeyDef) apply(table *pgschema.Table) {
	table.ForeignKeys = append(table.ForeignKeys, f.def)
}

func (e *ExclusionDef) apply(table *pgschema.Table) {
	table.Exclusions = append(table.Exclusions, e.def)
}

func (p *PartitionDef) apply(table *pgschema.Table) {
	table.Partitioning = p.partitioning()
}

func (p *PartitionOfDef) apply(table *pgschema.Table) {
	table.PartitionOf = p.partitionOf()
}

func (p *PartitionDef) partitioning() *pgschema.Partitioning {
	def := p.def
	def.Keys = append([]pgschema.PartitionKey(nil), p.def.Keys...)
	return &def
}

func (p *PartitionOfDef) partitionOf() *pgschema.PartitionOf {
	def := p.def
	def.Bound = copyPartitionBound(p.def.Bound)
	return &def
}

func (p *PartitionBoundDef) partitionBound() pgschema.PartitionBound {
	return copyPartitionBound(p.def)
}

func copyPartitionBound(bound pgschema.PartitionBound) pgschema.PartitionBound {
	bound.From = append([]string(nil), bound.From...)
	bound.To = append([]string(nil), bound.To...)
	bound.Values = append([]string(nil), bound.Values...)
	return bound
}

func partitionBy(strategy string, columns ...string) *PartitionDef {
	keys := make([]pgschema.PartitionKey, 0, len(columns))
	for _, column := range columns {
		keys = append(keys, pgschema.PartitionKey{Expression: column})
	}
	return newPartitionDef(strategy, keys)
}

func partitionByOn(strategy string, keys ...*PartitionKeyDef) *PartitionDef {
	out := make([]pgschema.PartitionKey, 0, len(keys))
	for _, key := range keys {
		out = append(out, key.def)
	}
	return newPartitionDef(strategy, out)
}

func newPartitionDef(strategy string, keys []pgschema.PartitionKey) *PartitionDef {
	return &PartitionDef{
		def: pgschema.Partitioning{
			Strategy: strategy,
			Keys:     keys,
		},
	}
}
