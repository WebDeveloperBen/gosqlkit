package pgschema

import "github.com/webdeveloperben/gosqlkit/internal/ast"

type Schema struct {
	Namespaces []Namespace
	Extensions []Extension
	Enums      []Enum
	Tables     []ast.Table
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
