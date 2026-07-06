package pgschema

import "github.com/webdeveloperben/gosqlkit/internal/ast"

type SchemaMetadata struct {
	Source string `json:"source,omitempty"`
}

type TableMetadata struct {
	Source  string `json:"source,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type ColumnMetadata struct {
	Source string `json:"source,omitempty"`
}

type ViewMetadata struct {
	Source  string `json:"source,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type RoleMetadata struct {
	Source string `json:"source,omitempty"`
}

type FunctionMetadata struct {
	Source  string `json:"source,omitempty"`
	Comment string `json:"comment,omitempty"`
}

func buildSchemaMetadata(namespaces []Namespace) map[string]SchemaMetadata {
	if len(namespaces) == 0 {
		return nil
	}
	m := make(map[string]SchemaMetadata, len(namespaces))
	for _, ns := range namespaces {
		m[ns.Name] = SchemaMetadata{}
	}
	return m
}

func buildTableMetadata(tables []ast.Table) map[string]TableMetadata {
	if len(tables) == 0 {
		return nil
	}
	m := make(map[string]TableMetadata, len(tables))
	for _, t := range tables {
		key := qualified(t.Schema, t.Name)
		m[key] = TableMetadata{Comment: t.Comment}
	}
	return m
}

func buildColumnMetadata(tables []ast.Table) map[string]ColumnMetadata {
	total := 0
	for _, t := range tables {
		total += len(t.Columns)
	}
	if total == 0 {
		return nil
	}
	m := make(map[string]ColumnMetadata, total)
	for _, t := range tables {
		tableKey := qualified(t.Schema, t.Name)
		for _, col := range t.Columns {
			m[tableKey+"."+col.Name] = ColumnMetadata{}
		}
	}
	return m
}

func buildViewMetadata(views []View, mviews []MaterializedView) map[string]ViewMetadata {
	total := len(views) + len(mviews)
	if total == 0 {
		return nil
	}
	m := make(map[string]ViewMetadata, total)
	for _, v := range views {
		m[qualified(v.Schema, v.Name)] = ViewMetadata{Comment: v.Comment}
	}
	for _, mv := range mviews {
		m[qualified(mv.Schema, mv.Name)] = ViewMetadata{Comment: mv.Comment}
	}
	return m
}

func buildRoleMetadata(roles []Role) map[string]RoleMetadata {
	if len(roles) == 0 {
		return nil
	}
	m := make(map[string]RoleMetadata, len(roles))
	for _, role := range roles {
		m[role.Name] = RoleMetadata{}
	}
	return m
}

func buildFunctionMetadata(functions []Function) map[string]FunctionMetadata {
	if len(functions) == 0 {
		return nil
	}
	m := make(map[string]FunctionMetadata, len(functions))
	for _, fn := range functions {
		m[functionKey(fn)] = FunctionMetadata{Comment: fn.Comment}
	}
	return m
}
