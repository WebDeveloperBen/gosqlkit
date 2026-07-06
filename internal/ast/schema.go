package ast

type Schema struct {
	Tables []Table
}

type Table struct {
	Schema            string
	Name              string
	Columns           []Column
	PrimaryKeys       []PrimaryKey
	UniqueConstraints []UniqueConstraint
	ForeignKeys       []ForeignKeyConstraint
	Checks            []Check
	Indexes           []Index
}

type Column struct {
	References *ForeignKey
	Name       string
	Type       string
	Default    string
	NotNull    bool
	PrimaryKey bool
	Unique     bool
}

type ForeignKey struct {
	Table    string
	Column   string
	OnDelete string
	OnUpdate string
}

type PrimaryKey struct {
	Name    string
	Columns []string
}

type UniqueConstraint struct {
	Name             string
	Columns          []string
	NullsNotDistinct bool
}

type ForeignKeyConstraint struct {
	Name              string
	ReferencedTable   string
	OnDelete          string
	OnUpdate          string
	Columns           []string
	ReferencedColumns []string
}

type Check struct {
	Name       string
	Expression string
}

type Index struct {
	With         map[string]string
	Name         string
	Method       string
	Where        string
	Columns      []IndexColumn
	Unique       bool
	Concurrently bool
	Only         bool
}

type IndexColumn struct {
	Expression   string
	Order        string
	Nulls        string
	OpClass      string
	IsExpression bool
}
