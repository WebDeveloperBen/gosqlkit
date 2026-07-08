package pgschema

import "github.com/webdeveloperben/gosqlkit/internal/ast"

type ExclusionConstraint struct {
	Method   string             `json:"method,omitempty"`
	Elements []ExclusionElement `json:"elements,omitempty"`
	ast.ExclusionConstraint
}

type ExclusionElement struct {
	ast.ExclusionElement
	OpClass string `json:"opClass,omitempty"`
	Nulls   string `json:"nulls,omitempty"`
}
