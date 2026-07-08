package pgschema

import "github.com/webdeveloperben/gosqlkit/internal/ast"

type Index struct {
	With    map[string]string `json:"with,omitempty"`
	Method  string            `json:"method,omitempty"`
	Columns []IndexColumn     `json:"columns,omitempty"`
	ast.Index
	Concurrently bool `json:"concurrently,omitempty"`
	Only         bool `json:"only,omitempty"`
}

type IndexColumn struct {
	OpClass string `json:"opClass,omitempty"`
	Nulls   string `json:"nulls,omitempty"`
	ast.IndexColumn
}
