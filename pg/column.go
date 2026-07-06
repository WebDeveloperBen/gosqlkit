package pg

import (
	"fmt"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
)

type Column struct {
	def ast.Column
}

func UUID(name string) *Column {
	return column(name, "uuid")
}

func Text(name string) *Column {
	return column(name, "text")
}

func VarChar(name string, length int) *Column {
	if length <= 0 {
		panic("varchar length must be greater than zero")
	}
	return column(name, fmt.Sprintf("varchar(%d)", length))
}

func Integer(name string) *Column {
	return column(name, "integer")
}

func BigInt(name string) *Column {
	return column(name, "bigint")
}

func Boolean(name string) *Column {
	return column(name, "boolean")
}

func Numeric(name string, precisionScale ...int) *Column {
	switch len(precisionScale) {
	case 0:
		return column(name, "numeric")
	case 2:
		precision, scale := precisionScale[0], precisionScale[1]
		if precision <= 0 || scale < 0 {
			panic("numeric precision must be greater than zero and scale must be non-negative")
		}
		return column(name, fmt.Sprintf("numeric(%d, %d)", precision, scale))
	default:
		panic("numeric expects either no precision or precision and scale")
	}
}

func Date(name string) *Column {
	return column(name, "date")
}

func Timestamp(name string) *Column {
	return column(name, "timestamp")
}

func TimestampTZ(name string) *Column {
	return column(name, "timestamptz")
}

func JSONB(name string) *Column {
	return column(name, "jsonb")
}

func (c *Column) PrimaryKey() *Column {
	c.def.PrimaryKey = true
	return c
}

func (c *Column) NotNull() *Column {
	c.def.NotNull = true
	return c
}

func (c *Column) Unique() *Column {
	c.def.Unique = true
	return c
}

func (c *Column) Default(expression string) *Column {
	c.def.Default = expression
	return c
}

func (c *Column) References(table, column string) *Column {
	c.def.References = &ast.ForeignKey{
		Table:  table,
		Column: column,
	}
	return c
}

func (c *Column) OnDelete(action string) *Column {
	c.ensureForeignKey()
	c.def.References.OnDelete = action
	return c
}

func (c *Column) OnUpdate(action string) *Column {
	c.ensureForeignKey()
	c.def.References.OnUpdate = action
	return c
}

func (c *Column) apply(table *ast.Table) {
	table.Columns = append(table.Columns, c.def)
}

func (c *Column) ensureForeignKey() {
	if c.def.References == nil {
		panic("OnDelete/OnUpdate requires References to be set first")
	}
}

func column(name, typ string) *Column {
	return &Column{
		def: ast.Column{
			Name: name,
			Type: typ,
		},
	}
}
