package pgschema

import "github.com/webdeveloperben/gosqlkit/internal/ast"

type Schema struct {
	Namespaces     []Namespace
	Extensions     []Extension
	Enums          []Enum
	CompositeTypes []CompositeType
	Domains        []Domain
	Sequences      []Sequence
	Tables         []ast.Table
}

type Namespace struct {
	Name string
}

type Extension struct {
	Name   string
	Schema string
}

type Enum struct {
	Schema string
	Name   string
	Values []string
}

type CompositeType struct {
	Schema     string
	Name       string
	Attributes []CompositeAttribute
}

type CompositeAttribute struct {
	Name string
	Type string
}

type Domain struct {
	Schema   string
	Name     string
	BaseType string
	Default  string
	Check    string
	NotNull  bool
}

type Sequence struct {
	MinValue  *int64
	MaxValue  *int64
	StartWith *int64
	Cache     *int64
	Schema    string
	Name      string
	OwnedBy   string
	Increment int64
	Cycle     bool
}
