package pg

import (
	"fmt"
	"strings"

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

func Char(name string, length ...int) *Column {
	switch len(length) {
	case 0:
		return column(name, "char")
	case 1:
		if length[0] <= 0 {
			panic("char length must be greater than zero")
		}
		return column(name, fmt.Sprintf("char(%d)", length[0]))
	default:
		panic("char expects either no length or a single length")
	}
}

func Integer(name string) *Column {
	return column(name, "integer")
}

func SmallInt(name string) *Column {
	return column(name, "smallint")
}

func BigInt(name string) *Column {
	return column(name, "bigint")
}

func Serial(name string) *Column {
	return serialColumn(name, "serial")
}

func SmallSerial(name string) *Column {
	return serialColumn(name, "smallserial")
}

func BigSerial(name string) *Column {
	return serialColumn(name, "bigserial")
}

func Real(name string) *Column {
	return column(name, "real")
}

func DoublePrecision(name string) *Column {
	return column(name, "double precision")
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

func Time(name string, precision ...int) *Column {
	return column(name, timeType("time", precision))
}

func TimeTZ(name string, precision ...int) *Column {
	return column(name, timeType("timetz", precision))
}

func Timestamp(name string, precision ...int) *Column {
	return column(name, timeType("timestamp", precision))
}

func TimestampTZ(name string, precision ...int) *Column {
	return column(name, timeType("timestamptz", precision))
}

func JSONB(name string) *Column {
	return column(name, "jsonb")
}

func JSON(name string) *Column {
	return column(name, "json")
}

func Bytea(name string) *Column {
	return column(name, "bytea")
}

func Inet(name string) *Column {
	return column(name, "inet")
}

func Cidr(name string) *Column {
	return column(name, "cidr")
}

func Macaddr(name string) *Column {
	return column(name, "macaddr")
}

func Macaddr8(name string) *Column {
	return column(name, "macaddr8")
}

func Point(name string) *Column {
	return column(name, "point")
}

func Line(name string) *Column {
	return column(name, "line")
}

type IntervalConfig struct {
	Fields    string
	Precision int
}

func Interval(name string, config ...IntervalConfig) *Column {
	switch len(config) {
	case 0:
		return column(name, "interval")
	case 1:
		return column(name, intervalType(config[0]))
	default:
		panic("interval expects either no config or a single IntervalConfig")
	}
}

func Custom(name string, sqlType string) *Column {
	if strings.TrimSpace(sqlType) == "" {
		panic("custom column type must not be empty")
	}
	return column(name, sqlType)
}

func CustomType(sqlType string) func(string) *Column {
	if strings.TrimSpace(sqlType) == "" {
		panic("custom type sql type must not be empty")
	}
	return func(name string) *Column {
		return column(name, sqlType)
	}
}

func serialColumn(name, typ string) *Column {
	col := column(name, typ)
	col.def.NotNull = true
	return col
}

func timeType(base string, precision []int) string {
	switch len(precision) {
	case 0:
		return base
	case 1:
		if precision[0] < 0 {
			panic(base + " precision must be non-negative")
		}
		return fmt.Sprintf("%s(%d)", base, precision[0])
	default:
		panic(base + " expects either no precision or a single precision")
	}
}

func intervalType(config IntervalConfig) string {
	var b strings.Builder
	b.WriteString("interval")
	if config.Fields != "" {
		b.WriteString(" ")
		b.WriteString(config.Fields)
	}
	if config.Precision > 0 {
		fmt.Fprintf(&b, " (%d)", config.Precision)
	}
	return b.String()
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

func (c *Column) Array() *Column {
	c.def.Type += "[]"
	return c
}

func (c *Column) ArrayN(dimensions int) *Column {
	if dimensions < 1 {
		panic("array dimensions must be greater than zero")
	}
	c.def.Type += strings.Repeat("[]", dimensions)
	return c
}

func (c *Column) Comment(text string) *Column {
	c.def.Comment = text
	return c
}

func (c *Column) GeneratedAlwaysAs(expression string) *Column {
	c.def.Generated = &ast.Generated{
		As:   expression,
		Type: "stored",
	}
	return c
}

type SequenceOptions struct {
	MinValue  *int64
	MaxValue  *int64
	StartWith *int64
	Cache     *int64
	Name      string
	OwnedBy   string
	Increment int64
	Cycle     bool
}

func (c *Column) GeneratedAlwaysAsIdentity(opts ...SequenceOptions) *Column {
	c.def.Identity = identityConfig("always", opts)
	c.def.NotNull = true
	return c
}

func (c *Column) GeneratedByDefaultAsIdentity(opts ...SequenceOptions) *Column {
	c.def.Identity = identityConfig("byDefault", opts)
	c.def.NotNull = true
	return c
}

func identityConfig(kind string, opts []SequenceOptions) *ast.Identity {
	identity := &ast.Identity{Type: kind}
	if len(opts) > 1 {
		panic("identity expects either no options or a single SequenceOptions")
	}
	if len(opts) == 1 {
		identity.Name = opts[0].Name
		identity.Increment = opts[0].Increment
		identity.MinValue = opts[0].MinValue
		identity.MaxValue = opts[0].MaxValue
		identity.StartWith = opts[0].StartWith
		identity.Cache = opts[0].Cache
		identity.Cycle = opts[0].Cycle
	}
	return identity
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
