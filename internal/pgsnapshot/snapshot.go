package pgsnapshot

import (
	"encoding/json"
	"sort"

	"github.com/webdeveloperben/gosqlkit/internal/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/snapshot"
)

type Document struct {
	Dialect    string           `json:"dialect"`
	Namespaces []Namespace      `json:"namespaces,omitempty"`
	Extensions []Extension      `json:"extensions,omitempty"`
	Enums      []Enum           `json:"enums,omitempty"`
	Tables     []snapshot.Table `json:"tables,omitempty"`
	Version    int              `json:"version"`
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

func JSON(schema pgschema.Schema) ([]byte, error) {
	doc := Document{
		Version:    snapshot.Version,
		Dialect:    "postgresql",
		Namespaces: namespaces(schema.Namespaces),
		Extensions: extensions(schema.Extensions),
		Enums:      enums(schema.Enums),
		Tables:     snapshot.Tables(schema.Tables),
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

func qualified(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}
