package ast

type Schema struct {
	Tables []Table
}

type Table struct {
	Name    string
	Columns []Column
	Checks  []Check
	Indexes []Index
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

type Check struct {
	Name       string
	Expression string
}

type Index struct {
	Name    string
	Columns []string
	Unique  bool
}
