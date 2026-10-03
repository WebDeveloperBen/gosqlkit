package sqliteschema

import "github.com/webdeveloperben/gosqlkit/internal/ast"

type Index struct {
	Columns []IndexColumn `json:"columns,omitempty"`
	ast.Index
	IfNotExists bool `json:"ifNotExists,omitempty"`
}

type IndexColumn struct {
	Collation string `json:"collation,omitempty"`
	ast.IndexColumn
}
