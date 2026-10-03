package sqlite

import (
	"fmt"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

// Column is a fluent SQLite column builder. SQLite uses type affinity rather
// than strict storage types, so the convenience constructors map onto one of
// the five storage classes (INTEGER, TEXT, REAL, BLOB, NUMERIC); there is no
// runtime coercion.
type Column struct {
	def sqliteschema.Column
}

func Integer(name string) *Column { return column(name, "integer") }

func Int(name string) *Column { return column(name, "integer") }

func BigInt(name string) *Column { return column(name, "integer") }

func Text(name string) *Column { return column(name, "text") }

func Real(name string) *Column { return column(name, "real") }

func Blob(name string) *Column { return column(name, "blob") }

func Boolean(name string) *Column { return column(name, "integer") }

func Date(name string) *Column { return column(name, "text") }

func Time(name string) *Column { return column(name, "text") }

func Timestamp(name string) *Column { return column(name, "text") }

func JSON(name string) *Column { return column(name, "text") }

// Any is the STRICT-table wildcard affinity.
func Any(name string) *Column { return column(name, "any") }

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

func Custom(name, sqlType string) *Column {
	if strings.TrimSpace(sqlType) == "" {
		panic("custom column type must not be empty")
	}
	return column(name, sqlType)
}

func (c *Column) PrimaryKey() *Column {
	c.def.PrimaryKey = true
	return c
}

// AutoIncrement marks an INTEGER PRIMARY KEY column as AUTOINCREMENT. The
// renderer validates that the column is in fact an INTEGER PRIMARY KEY.
func (c *Column) AutoIncrement() *Column {
	c.def.AutoIncrement = true
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

func (c *Column) Collate(name string) *Column {
	c.def.Collation = name
	return c
}

// GeneratedAlwaysAs declares a STORED generated column.
func (c *Column) GeneratedAlwaysAs(expression string) *Column {
	c.def.Generated = &ast.Generated{As: expression, Type: "stored"}
	return c
}

// GeneratedAlwaysAsVirtual declares a VIRTUAL generated column (computed on
// read, not persisted). VIRTUAL is SQLite's default when neither is specified.
func (c *Column) GeneratedAlwaysAsVirtual(expression string) *Column {
	c.def.Generated = &ast.Generated{As: expression, Type: "virtual"}
	return c
}

func (c *Column) References(table, column string) *Column {
	c.def.References = &ast.ForeignKey{Table: table, Column: column}
	return c
}

func (c *Column) OnDelete(action ForeignKeyAction) *Column {
	c.ensureForeignKey()
	c.def.References.OnDelete = string(action)
	return c
}

func (c *Column) OnUpdate(action ForeignKeyAction) *Column {
	c.ensureForeignKey()
	c.def.References.OnUpdate = string(action)
	return c
}

func (c *Column) Comment(text string) *Column {
	c.def.Comment = text
	return c
}

func (c *Column) PreviousName(name string) *Column {
	c.def.PreviousName = name
	return c
}

func (c *Column) apply(table *sqliteschema.Table) {
	table.Columns = append(table.Columns, c.def)
}

func (c *Column) ensureForeignKey() {
	if c.def.References == nil {
		panic("OnDelete/OnUpdate requires References to be set first")
	}
}

func column(name, typ string) *Column {
	return &Column{def: sqliteschema.Column{Column: ast.Column{Name: name, Type: typ}}}
}
