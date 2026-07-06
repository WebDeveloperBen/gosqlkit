package pgsnapshot

import (
	"encoding/json"
	"sort"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/snapshot"
)

type Document struct {
	Dialect        string           `json:"dialect"`
	Namespaces     []Namespace      `json:"namespaces,omitempty"`
	Extensions     []Extension      `json:"extensions,omitempty"`
	Enums          []Enum           `json:"enums,omitempty"`
	CompositeTypes []CompositeType  `json:"compositeTypes,omitempty"`
	Domains        []Domain         `json:"domains,omitempty"`
	Sequences      []Sequence       `json:"sequences,omitempty"`
	Tables         []snapshot.Table `json:"tables,omitempty"`
	Version        int              `json:"version"`
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

func JSON(schema pgschema.Schema) ([]byte, error) {
	doc := Document{
		Version:        snapshot.Version,
		Dialect:        "postgresql",
		Namespaces:     namespaces(schema.Namespaces),
		Extensions:     extensions(schema.Extensions),
		Enums:          enums(schema.Enums),
		CompositeTypes: compositeTypes(schema.CompositeTypes),
		Domains:        domains(schema.Domains),
		Sequences:      sequences(schema.Sequences),
		Tables:         snapshot.Tables(schema.Tables),
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func namespaces(input []pgschema.Namespace) []Namespace {
	items := make([]Namespace, 0, len(input))
	for _, namespace := range input {
		items = append(items, Namespace{Name: namespace.Name})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func extensions(input []pgschema.Extension) []Extension {
	items := make([]Extension, 0, len(input))
	for _, extension := range input {
		items = append(items, Extension{Name: extension.Name, Schema: extension.Schema})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}

func enums(input []pgschema.Enum) []Enum {
	items := make([]Enum, 0, len(input))
	for _, enum := range input {
		items = append(items, Enum{
			Schema: enum.Schema,
			Name:   enum.Name,
			Values: append([]string(nil), enum.Values...),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}

func sequences(input []pgschema.Sequence) []Sequence {
	items := make([]Sequence, 0, len(input))
	for _, sequence := range input {
		items = append(items, Sequence{
			Schema:    sequence.Schema,
			Name:      sequence.Name,
			Increment: sequence.Increment,
			MinValue:  sequence.MinValue,
			MaxValue:  sequence.MaxValue,
			StartWith: sequence.StartWith,
			Cache:     sequence.Cache,
			Cycle:     sequence.Cycle,
			OwnedBy:   sequence.OwnedBy,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}

func compositeTypes(input []pgschema.CompositeType) []CompositeType {
	items := make([]CompositeType, 0, len(input))
	for _, compositeType := range input {
		attrs := make([]CompositeAttribute, 0, len(compositeType.Attributes))
		for _, attribute := range compositeType.Attributes {
			attrs = append(attrs, CompositeAttribute{Name: attribute.Name, Type: attribute.Type})
		}
		items = append(items, CompositeType{
			Schema:     compositeType.Schema,
			Name:       compositeType.Name,
			Attributes: attrs,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}

func domains(input []pgschema.Domain) []Domain {
	items := make([]Domain, 0, len(input))
	for _, domain := range input {
		items = append(items, Domain{
			Schema:   domain.Schema,
			Name:     domain.Name,
			BaseType: domain.BaseType,
			Default:  domain.Default,
			NotNull:  domain.NotNull,
			Check:    domain.Check,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return qualified(items[i].Schema, items[i].Name) < qualified(items[j].Schema, items[j].Name)
	})
	return items
}

func qualified(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}
