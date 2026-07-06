package pgschema

import (
	"encoding/json"
	"sort"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
)

const SnapshotVersion = 1

type Schema struct {
	Namespaces     []Namespace     `json:"namespaces,omitempty"`
	Extensions     []Extension     `json:"extensions,omitempty"`
	Enums          []Enum          `json:"enums,omitempty"`
	CompositeTypes []CompositeType `json:"compositeTypes,omitempty"`
	Domains        []Domain        `json:"domains,omitempty"`
	Sequences      []Sequence      `json:"sequences,omitempty"`
	Tables         []ast.Table     `json:"tables,omitempty"`
}

type Document struct {
	ColumnMetadata     map[string]ColumnMetadata `json:"columnMetadata,omitempty"`
	TableMetadata      map[string]TableMetadata  `json:"tableMetadata,omitempty"`
	SchemaMetadata     map[string]SchemaMetadata `json:"schemaMetadata,omitempty"`
	SnapshotID         string                    `json:"snapshotId"`
	PreviousSnapshotID string                    `json:"previousSnapshotId,omitempty"`
	Dialect            string                    `json:"dialect"`
	Sequences          []Sequence                `json:"sequences,omitempty"`
	CompositeTypes     []CompositeType           `json:"compositeTypes,omitempty"`
	Domains            []Domain                  `json:"domains,omitempty"`
	Enums              []Enum                    `json:"enums,omitempty"`
	Tables             []ast.Table               `json:"tables,omitempty"`
	Extensions         []Extension               `json:"extensions,omitempty"`
	Namespaces         []Namespace               `json:"namespaces,omitempty"`
	Version            int                       `json:"version"`
}

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

type Namespace struct {
	Name string `json:"name"`
}

type Extension struct {
	Name   string `json:"name"`
	Schema string `json:"schema,omitempty"`
}

type Enum struct {
	Schema string   `json:"schema,omitempty"`
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type CompositeType struct {
	Schema     string               `json:"schema,omitempty"`
	Name       string               `json:"name"`
	Attributes []CompositeAttribute `json:"attributes,omitempty"`
}

type CompositeAttribute struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Domain struct {
	Schema   string `json:"schema,omitempty"`
	Name     string `json:"name"`
	BaseType string `json:"baseType"`
	Default  string `json:"default,omitempty"`
	Check    string `json:"check,omitempty"`
	NotNull  bool   `json:"notNull,omitempty"`
}

type Sequence struct {
	MinValue  *int64 `json:"minValue,omitempty"`
	MaxValue  *int64 `json:"maxValue,omitempty"`
	StartWith *int64 `json:"startWith,omitempty"`
	Cache     *int64 `json:"cache,omitempty"`
	Schema    string `json:"schema,omitempty"`
	Name      string `json:"name"`
	OwnedBy   string `json:"ownedBy,omitempty"`
	Increment int64  `json:"increment,omitempty"`
	Cycle     bool   `json:"cycle,omitempty"`
}

func JSON(dialect string, schema Schema) ([]byte, error) {
	doc := Document{
		Dialect:        dialect,
		Version:        SnapshotVersion,
		Namespaces:     sortedNamespaces(schema.Namespaces),
		Extensions:     sortedExtensions(schema.Extensions),
		Enums:          sortedEnums(schema.Enums),
		CompositeTypes: sortedCompositeTypes(schema.CompositeTypes),
		Domains:        sortedDomains(schema.Domains),
		Sequences:      sortedSequences(schema.Sequences),
		Tables:         sortedTables(schema.Tables),
		SchemaMetadata: buildSchemaMetadata(schema.Namespaces),
		TableMetadata:  buildTableMetadata(schema.Tables),
		ColumnMetadata: buildColumnMetadata(schema.Tables),
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
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

func sortedNamespaces(input []Namespace) []Namespace {
	items := append([]Namespace(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func sortedExtensions(input []Extension) []Extension {
	items := append([]Extension(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}

func sortedEnums(input []Enum) []Enum {
	items := append([]Enum(nil), input...)
	for i := range items {
		items[i].Values = append([]string(nil), items[i].Values...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}

func sortedCompositeTypes(input []CompositeType) []CompositeType {
	items := append([]CompositeType(nil), input...)
	for i := range items {
		attrs := append([]CompositeAttribute(nil), items[i].Attributes...)
		items[i].Attributes = attrs
	}
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}

func sortedDomains(input []Domain) []Domain {
	items := append([]Domain(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}

func sortedSequences(input []Sequence) []Sequence {
	items := append([]Sequence(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}

func sortedTables(input []ast.Table) []ast.Table {
	items := append([]ast.Table(nil), input...)
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	for i := range items {
		items[i] = sortedTable(items[i])
	}
	return items
}

func sortedTable(table ast.Table) ast.Table {
	sort.SliceStable(table.PrimaryKeys, func(i, j int) bool {
		return table.PrimaryKeys[i].Name < table.PrimaryKeys[j].Name
	})
	sort.SliceStable(table.UniqueConstraints, func(i, j int) bool {
		return table.UniqueConstraints[i].Name < table.UniqueConstraints[j].Name
	})
	sort.SliceStable(table.ForeignKeys, func(i, j int) bool {
		return table.ForeignKeys[i].Name < table.ForeignKeys[j].Name
	})
	sort.SliceStable(table.Checks, func(i, j int) bool {
		return table.Checks[i].Name < table.Checks[j].Name
	})
	sort.SliceStable(table.Exclusions, func(i, j int) bool {
		return table.Exclusions[i].Name < table.Exclusions[j].Name
	})
	sort.SliceStable(table.Indexes, func(i, j int) bool {
		return table.Indexes[i].Name < table.Indexes[j].Name
	})
	return table
}

func qualified(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}
