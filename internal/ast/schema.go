package ast

type Schema struct {
	Tables []Table `json:"tables,omitempty"`
}

type Table struct {
	Schema            string                 `json:"schema,omitempty"`
	Name              string                 `json:"name"`
	Comment           string                 `json:"comment,omitempty"`
	Columns           []Column               `json:"columns,omitempty"`
	PrimaryKeys       []PrimaryKey           `json:"primaryKeys,omitempty"`
	UniqueConstraints []UniqueConstraint     `json:"uniqueConstraints,omitempty"`
	ForeignKeys       []ForeignKeyConstraint `json:"foreignKeys,omitempty"`
	Checks            []Check                `json:"checks,omitempty"`
	Exclusions        []ExclusionConstraint  `json:"exclusions,omitempty"`
	Indexes           []Index                `json:"indexes,omitempty"`
}

type Column struct {
	References *ForeignKey `json:"references,omitempty"`
	Generated  *Generated  `json:"generated,omitempty"`
	Identity   *Identity   `json:"identity,omitempty"`
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Default    string      `json:"default,omitempty"`
	Comment    string      `json:"comment,omitempty"`
	NotNull    bool        `json:"notNull,omitempty"`
	PrimaryKey bool        `json:"primaryKey,omitempty"`
	Unique     bool        `json:"unique,omitempty"`
}

type ForeignKey struct {
	Table    string `json:"table"`
	Column   string `json:"column"`
	OnDelete string `json:"onDelete,omitempty"`
	OnUpdate string `json:"onUpdate,omitempty"`
}

type Generated struct {
	As   string `json:"as"`
	Type string `json:"type"`
}

type Identity struct {
	Name      string `json:"name,omitempty"`
	Type      string `json:"type"`
	Increment int64  `json:"increment,omitempty"`
	MinValue  *int64 `json:"minValue,omitempty"`
	MaxValue  *int64 `json:"maxValue,omitempty"`
	StartWith *int64 `json:"startWith,omitempty"`
	Cache     *int64 `json:"cache,omitempty"`
	Cycle     bool   `json:"cycle,omitempty"`
}

type PrimaryKey struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

type UniqueConstraint struct {
	Name             string   `json:"name"`
	Initially        string   `json:"initially,omitempty"`
	Columns          []string `json:"columns"`
	NullsNotDistinct bool     `json:"nullsNotDistinct,omitempty"`
	Deferrable       bool     `json:"deferrable,omitempty"`
}

type ForeignKeyConstraint struct {
	Name              string   `json:"name"`
	ReferencedTable   string   `json:"referencedTable"`
	OnDelete          string   `json:"onDelete,omitempty"`
	OnUpdate          string   `json:"onUpdate,omitempty"`
	Initially         string   `json:"initially,omitempty"`
	Columns           []string `json:"columns"`
	ReferencedColumns []string `json:"referencedColumns"`
	Deferrable        bool     `json:"deferrable,omitempty"`
}

type Check struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}

type ExclusionConstraint struct {
	Name       string             `json:"name"`
	Method     string             `json:"method,omitempty"`
	Where      string             `json:"where,omitempty"`
	Initially  string             `json:"initially,omitempty"`
	Elements   []ExclusionElement `json:"elements,omitempty"`
	Deferrable bool               `json:"deferrable,omitempty"`
}

type ExclusionElement struct {
	Expression string `json:"expression"`
	Operator   string `json:"operator"`
	OpClass    string `json:"opClass,omitempty"`
	Order      string `json:"order,omitempty"`
	Nulls      string `json:"nulls,omitempty"`
}

type Index struct {
	With         map[string]string `json:"with,omitempty"`
	Name         string            `json:"name"`
	Method       string            `json:"method,omitempty"`
	Where        string            `json:"where,omitempty"`
	Columns      []IndexColumn     `json:"columns,omitempty"`
	Unique       bool              `json:"unique,omitempty"`
	Concurrently bool              `json:"concurrently,omitempty"`
	Only         bool              `json:"only,omitempty"`
}

type IndexColumn struct {
	Expression   string `json:"expression"`
	Order        string `json:"order,omitempty"`
	Nulls        string `json:"nulls,omitempty"`
	OpClass      string `json:"opClass,omitempty"`
	IsExpression bool   `json:"isExpression,omitempty"`
}
