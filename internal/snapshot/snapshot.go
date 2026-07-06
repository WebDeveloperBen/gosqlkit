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
	MinValue  *int64 `json:"minValue,omitempty"`
	MaxValue  *int64 `json:"maxValue,omitempty"`
	StartWith *int64 `json:"startWith,omitempty"`
	Cache     *int64 `json:"cache,omitempty"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type"`
	Increment int64  `json:"increment,omitempty"`
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
	exclusions := append([]ast.ExclusionConstraint(nil), table.Exclusions...)
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
	sort.SliceStable(exclusions, func(i, j int) bool {
		return exclusions[i].Name < exclusions[j].Name
	})
	sort.SliceStable(indexes, func(i, j int) bool {
		return indexes[i].Name < indexes[j].Name
	})

	return Table{
		Schema:            table.Schema,
		Name:              table.Name,
		Comment:           table.Comment,
		Columns:           columns(table.Columns),
		PrimaryKeys:       primaryKeysSnapshot(primaryKeys),
		UniqueConstraints: uniqueConstraintsSnapshot(uniqueConstraints),
		ForeignKeys:       foreignKeysSnapshot(foreignKeys),
		Checks:            checksSnapshot(checks),
		Exclusions:        exclusionsSnapshot(exclusions),
		Indexes:           indexesSnapshot(indexes),
	}
}

func columns(input []ast.Column) []Column {
	items := make([]Column, 0, len(input))
	for _, column := range input {
		items = append(items, Column{
			References: foreignKeySnapshot(column.References),
			Generated:  generatedSnapshot(column.Generated),
			Identity:   identitySnapshot(column.Identity),
			Name:       column.Name,
			Type:       column.Type,
			Default:    column.Default,
			Comment:    column.Comment,
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

func generatedSnapshot(input *ast.Generated) *Generated {
	if input == nil {
		return nil
	}
	return &Generated{
		As:   input.As,
		Type: input.Type,
	}
}

func identitySnapshot(input *ast.Identity) *Identity {
	if input == nil {
		return nil
	}
	return &Identity{
		Name:      input.Name,
		Type:      input.Type,
		Increment: input.Increment,
		MinValue:  copyInt64Ptr(input.MinValue),
		MaxValue:  copyInt64Ptr(input.MaxValue),
		StartWith: copyInt64Ptr(input.StartWith),
		Cache:     copyInt64Ptr(input.Cache),
		Cycle:     input.Cycle,
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
			Deferrable:       uniqueConstraint.Deferrable,
			Initially:        uniqueConstraint.Initially,
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
			Deferrable:        foreignKey.Deferrable,
			Initially:         foreignKey.Initially,
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

func exclusionsSnapshot(input []ast.ExclusionConstraint) []ExclusionConstraint {
	items := make([]ExclusionConstraint, 0, len(input))
	for _, exclusion := range input {
		items = append(items, ExclusionConstraint{
			Name:       exclusion.Name,
			Method:     exclusion.Method,
			Elements:   exclusionElementsSnapshot(exclusion.Elements),
			Where:      exclusion.Where,
			Deferrable: exclusion.Deferrable,
			Initially:  exclusion.Initially,
		})
	}
	return items
}

func exclusionElementsSnapshot(input []ast.ExclusionElement) []ExclusionElement {
	items := make([]ExclusionElement, 0, len(input))
	for _, element := range input {
		items = append(items, ExclusionElement{
			Expression: element.Expression,
			Operator:   element.Operator,
			OpClass:    element.OpClass,
			Order:      element.Order,
			Nulls:      element.Nulls,
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

func copyInt64Ptr(input *int64) *int64 {
	if input == nil {
		return nil
	}
	value := *input
	return &value
}

func qualified(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}
