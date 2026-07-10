package pg

import (
	"fmt"
	"strings"
)

type Expression interface {
	SQL() string
}

type SQLType interface {
	TypeSQL() string
}

type expression string

func (e expression) SQL() string {
	return string(e)
}

type sqlType string

func (t sqlType) TypeSQL() string {
	return string(t)
}

func Expr(sql string) Expression {
	return expression(sql)
}

func Col(name string) Expression {
	return expression(name)
}

func TextType() SQLType {
	return sqlType("text")
}

func IntegerType() SQLType {
	return sqlType("integer")
}

func TriggerType() SQLType {
	return sqlType("trigger")
}

func NumericType(precision, scale int) SQLType {
	if precision <= 0 || scale < 0 {
		panic("numeric precision must be greater than zero and scale must be non-negative")
	}
	return sqlType(fmt.Sprintf("numeric(%d, %d)", precision, scale))
}

func CharType(length int) SQLType {
	if length <= 0 {
		panic("char length must be greater than zero")
	}
	return sqlType(fmt.Sprintf("char(%d)", length))
}

func VectorType(dimensions int) SQLType {
	return sqlType(vectorType("vector", dimensions))
}

func HalfVecType(dimensions int) SQLType {
	return sqlType(vectorType("halfvec", dimensions))
}

func SparseVecType(dimensions int) SQLType {
	return sqlType(vectorType("sparsevec", dimensions))
}

func BitType(dimensions int) SQLType {
	return sqlType(vectorType("bit", dimensions))
}

func CustomSQLType(sql string) SQLType {
	if strings.TrimSpace(sql) == "" {
		panic("custom SQL type must not be empty")
	}
	return sqlType(sql)
}

type CallExpr struct {
	name  string
	alias string
	args  []Expression
}

func Call(name string, args ...Expression) *CallExpr {
	return &CallExpr{name: name, args: append([]Expression(nil), args...)}
}

func CountAll() *CallExpr {
	return &CallExpr{name: "COUNT", args: []Expression{expression("*")}}
}

func Sum(expr Expression) *CallExpr {
	return Call("SUM", expr)
}

func Lower(expr Expression) *CallExpr {
	return Call("lower", expr)
}

func Trim(expr Expression) *CallExpr {
	return Call("trim", expr)
}

func (c *CallExpr) As(alias string) *CallExpr {
	c.alias = alias
	return c
}

func (c *CallExpr) SQL() string {
	args := make([]string, 0, len(c.args))
	for _, arg := range c.args {
		args = append(args, arg.SQL())
	}
	out := c.name + "(" + strings.Join(args, ", ") + ")"
	if c.alias != "" {
		out += " AS " + c.alias
	}
	return out
}

func IsNotNull(expr Expression) Expression {
	return expression(expr.SQL() + " IS NOT NULL")
}

type SelectQuery struct {
	from    string
	columns []Expression
	where   []Expression
	groupBy []Expression
}

func Select(columns ...Expression) *SelectQuery {
	return &SelectQuery{columns: append([]Expression(nil), columns...)}
}

func (q *SelectQuery) From(table string) *SelectQuery {
	q.from = table
	return q
}

func (q *SelectQuery) Where(expressions ...Expression) *SelectQuery {
	q.where = append(q.where, expressions...)
	return q
}

func (q *SelectQuery) GroupBy(expressions ...Expression) *SelectQuery {
	q.groupBy = append(q.groupBy, expressions...)
	return q
}

func (q *SelectQuery) SQL() string {
	columns := make([]string, 0, len(q.columns))
	for _, column := range q.columns {
		columns = append(columns, column.SQL())
	}
	var b strings.Builder
	b.WriteString("SELECT ")
	b.WriteString(strings.Join(columns, ", "))
	if q.from != "" {
		b.WriteString(" FROM ")
		b.WriteString(q.from)
	}
	if len(q.where) > 0 {
		where := make([]string, 0, len(q.where))
		for _, expression := range q.where {
			where = append(where, expression.SQL())
		}
		b.WriteString(" WHERE ")
		b.WriteString(strings.Join(where, " AND "))
	}
	if len(q.groupBy) > 0 {
		groupBy := make([]string, 0, len(q.groupBy))
		for _, expression := range q.groupBy {
			groupBy = append(groupBy, expression.SQL())
		}
		b.WriteString(" GROUP BY ")
		b.WriteString(strings.Join(groupBy, ", "))
	}
	return b.String()
}

type PLpgSQLBlock struct {
	statements []PLpgSQLStatement
}

type PLpgSQLStatement interface {
	PLpgSQL() string
}

type plpgsqlStatement string

func (s plpgsqlStatement) PLpgSQL() string {
	return string(s)
}

func Block(statements ...PLpgSQLStatement) *PLpgSQLBlock {
	return &PLpgSQLBlock{statements: append([]PLpgSQLStatement(nil), statements...)}
}

func Assign(target string, value Expression) PLpgSQLStatement {
	return plpgsqlStatement(target + " = " + value.SQL() + ";")
}

func Return(value Expression) PLpgSQLStatement {
	return plpgsqlStatement("RETURN " + value.SQL() + ";")
}

func (b *PLpgSQLBlock) SQL() string {
	lines := make([]string, 0, len(b.statements))
	for _, statement := range b.statements {
		lines = append(lines, "    "+statement.PLpgSQL())
	}
	return "BEGIN\n" + strings.Join(lines, "\n") + "\nEND"
}

func typeSQL(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case SQLType:
		return v.TypeSQL()
	case *EnumDef:
		return v.TypeName()
	case *DomainDef:
		return v.TypeName()
	case *CompositeTypeDef:
		return v.TypeName()
	default:
		panic(fmt.Sprintf("unsupported SQL type %T", value))
	}
}

func expressionSQL(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case Expression:
		return v.SQL()
	default:
		panic(fmt.Sprintf("unsupported SQL expression %T", value))
	}
}
