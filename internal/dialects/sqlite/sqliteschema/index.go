package sqliteschema

import "github.com/webdeveloperben/gosqlkit/internal/ast"

type Index struct {
	Columns []IndexColumn `json:"columns,omitempty"`
	ast.Index
}

type IndexColumn struct {
	Collation string `json:"collation,omitempty"`
	ast.IndexColumn
}
