package snapshot

import (
	"encoding/json"
	"sort"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
)

const Version = 1

type Document struct {
	Dialect string  `json:"dialect"`
	Tables  []Table `json:"tables,omitempty"`
	Version int     `json:"version"`
}

type Table struct {
	Schema            string                 `json:"schema,omitempty"`
	Name              string                 `json:"name"`
	Columns           []Column               `json:"columns,omitempty"`
	PrimaryKeys       []PrimaryKey           `json:"primaryKeys,omitempty"`
	UniqueConstraints []UniqueConstraint     `json:"uniqueConstraints,omitempty"`
	ForeignKeys       []ForeignKeyConstraint `json:"foreignKeys,omitempty"`
	Checks            []Check                `json:"checks,omitempty"`
	Indexes           []Index                `json:"indexes,omitempty"`
}

type Column struct {
	References *ForeignKey `json:"references,omitempty"`
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Default    string      `json:"default,omitempty"`
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

type PrimaryKey struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

type UniqueConstraint struct {
	Name             string   `json:"name"`
	Columns          []string `json:"columns"`
	NullsNotDistinct bool     `json:"nullsNotDistinct,omitempty"`
}

type ForeignKeyConstraint struct {
	Name              string   `json:"name"`
	ReferencedTable   string   `json:"referencedTable"`
	OnDelete          string   `json:"onDelete,omitempty"`
	OnUpdate          string   `json:"onUpdate,omitempty"`
	Columns           []string `json:"columns"`
	ReferencedColumns []string `json:"referencedColumns"`
}

type Check struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
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

func JSON(dialect string, schema ast.Schema) ([]byte, error) {
	doc := Document{
		Version: Version,
		Dialect: dialect,
		Tables:  Tables(schema.Tables),
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func Tables(input []ast.Table) []Table {
	astTables := append([]ast.Table(nil), input...)
	sort.SliceStable(astTables, func(i, j int) bool {
		return qualified(astTables[i].Schema, astTables[i].Name) < qualified(astTables[j].Schema, astTables[j].Name)
	})

	items := make([]Table, 0, len(astTables))
	for _, table := range astTables {
		items = append(items, snapshotTable(table))
	}
	return items
}

func snapshotTable(table ast.Table) Table {
	primaryKeys := append([]ast.PrimaryKey(nil), table.PrimaryKeys...)
	uniqueConstraints := append([]ast.UniqueConstraint(nil), table.UniqueConstraints...)
	foreignKeys := append([]ast.ForeignKeyConstraint(nil), table.ForeignKeys...)
	checks := append([]ast.Check(nil), table.Checks...)
	indexes := append([]ast.Index(nil), table.Indexes...)

	sort.SliceStable(primaryKeys, func(i, j int) bool {
		return primaryKeys[i].Name < primaryKeys[j].Name
	})
	sort.SliceStable(uniqueConstraints, func(i, j int) bool {
		return uniqueConstraints[i].Name < uniqueConstraints[j].Name
	})
	sort.SliceStable(foreignKeys, func(i, j int) bool {
		return foreignKeys[i].Name < foreignKeys[j].Name
	})
	sort.SliceStable(checks, func(i, j int) bool {
		return checks[i].Name < checks[j].Name
	})
	sort.SliceStable(indexes, func(i, j int) bool {
		return indexes[i].Name < indexes[j].Name
	})

	return Table{
		Schema:            table.Schema,
		Name:              table.Name,
		Columns:           columns(table.Columns),
		PrimaryKeys:       primaryKeysSnapshot(primaryKeys),
		UniqueConstraints: uniqueConstraintsSnapshot(uniqueConstraints),
		ForeignKeys:       foreignKeysSnapshot(foreignKeys),
		Checks:            checksSnapshot(checks),
		Indexes:           indexesSnapshot(indexes),
	}
}

func columns(input []ast.Column) []Column {
	items := make([]Column, 0, len(input))
	for _, column := range input {
		items = append(items, Column{
			References: foreignKeySnapshot(column.References),
			Name:       column.Name,
			Type:       column.Type,
			Default:    column.Default,
			NotNull:    column.NotNull,
			PrimaryKey: column.PrimaryKey,
			Unique:     column.Unique,
		})
	}
	return items
}

func foreignKeySnapshot(input *ast.ForeignKey) *ForeignKey {
	if input == nil {
		return nil
	}
	return &ForeignKey{
		Table:    input.Table,
		Column:   input.Column,
		OnDelete: input.OnDelete,
		OnUpdate: input.OnUpdate,
	}
}

func primaryKeysSnapshot(input []ast.PrimaryKey) []PrimaryKey {
	items := make([]PrimaryKey, 0, len(input))
	for _, primaryKey := range input {
		items = append(items, PrimaryKey{
			Name:    primaryKey.Name,
			Columns: append([]string(nil), primaryKey.Columns...),
		})
	}
	return items
}

func uniqueConstraintsSnapshot(input []ast.UniqueConstraint) []UniqueConstraint {
	items := make([]UniqueConstraint, 0, len(input))
	for _, uniqueConstraint := range input {
		items = append(items, UniqueConstraint{
			Name:             uniqueConstraint.Name,
			Columns:          append([]string(nil), uniqueConstraint.Columns...),
			NullsNotDistinct: uniqueConstraint.NullsNotDistinct,
		})
	}
	return items
}

func foreignKeysSnapshot(input []ast.ForeignKeyConstraint) []ForeignKeyConstraint {
	items := make([]ForeignKeyConstraint, 0, len(input))
	for _, foreignKey := range input {
		items = append(items, ForeignKeyConstraint{
			Name:              foreignKey.Name,
			ReferencedTable:   foreignKey.ReferencedTable,
			OnDelete:          foreignKey.OnDelete,
			OnUpdate:          foreignKey.OnUpdate,
			Columns:           append([]string(nil), foreignKey.Columns...),
			ReferencedColumns: append([]string(nil), foreignKey.ReferencedColumns...),
		})
	}
	return items
}

func checksSnapshot(input []ast.Check) []Check {
	items := make([]Check, 0, len(input))
	for _, check := range input {
		items = append(items, Check{
			Name:       check.Name,
			Expression: check.Expression,
		})
	}
	return items
}

func indexesSnapshot(input []ast.Index) []Index {
	items := make([]Index, 0, len(input))
	for _, index := range input {
		items = append(items, Index{
			With:         copyMap(index.With),
			Name:         index.Name,
			Method:       index.Method,
			Where:        index.Where,
			Columns:      indexColumnsSnapshot(index.Columns),
			Unique:       index.Unique,
			Concurrently: index.Concurrently,
			Only:         index.Only,
		})
	}
	return items
}

func indexColumnsSnapshot(input []ast.IndexColumn) []IndexColumn {
	items := make([]IndexColumn, 0, len(input))
	for _, column := range input {
		items = append(items, IndexColumn{
			Expression:   column.Expression,
			Order:        column.Order,
			Nulls:        column.Nulls,
			OpClass:      column.OpClass,
			IsExpression: column.IsExpression,
		})
	}
	return items
}

func copyMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func qualified(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}
