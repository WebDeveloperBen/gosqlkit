package ast

type Schema struct {
	Tables []Table
}

type Table struct {
	Schema            string
	Name              string
	Comment           string
	Columns           []Column
	PrimaryKeys       []PrimaryKey
	UniqueConstraints []UniqueConstraint
	ForeignKeys       []ForeignKeyConstraint
	Checks            []Check
	Exclusions        []ExclusionConstraint
	Indexes           []Index
}

type Column struct {
	References *ForeignKey
	Generated  *Generated
	Identity   *Identity
	Name       string
	Type       string
	Default    string
	Comment    string
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

type Generated struct {
	As   string
	Type string
}

type Identity struct {
	MinValue  *int64
	MaxValue  *int64
	StartWith *int64
	Cache     *int64
	Name      string
	Type      string
	Increment int64
	Cycle     bool
}

type PrimaryKey struct {
	Name    string
	Columns []string
}

type UniqueConstraint struct {
	Name             string
	Initially        string
	Columns          []string
	NullsNotDistinct bool
	Deferrable       bool
}

type ForeignKeyConstraint struct {
	Name              string
	ReferencedTable   string
	OnDelete          string
	OnUpdate          string
	Initially         string
	Columns           []string
	ReferencedColumns []string
	Deferrable        bool
}

type Check struct {
	Name       string
	Expression string
}

type ExclusionConstraint struct {
	Name       string
	Method     string
	Where      string
	Initially  string
	Elements   []ExclusionElement
	Deferrable bool
}

type ExclusionElement struct {
	Expression string
	Operator   string
	OpClass    string
	Order      string
	Nulls      string
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
