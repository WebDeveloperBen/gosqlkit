package pg

import (
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/pgschema"
)

type NamespaceDef struct {
	def pgschema.Namespace
}

type ExtensionDef struct {
	def pgschema.Extension
}

type EnumDef struct {
	def pgschema.Enum
}

func Namespace(name string) *NamespaceDef {
	namespace := pgschema.Namespace{Name: name}
	registerNamespace(namespace)
	return &NamespaceDef{def: namespace}
}

func Extension(name string) *ExtensionDef {
	return ExtensionInSchema("", name)
}

func ExtensionInSchema(schema, name string) *ExtensionDef {
	extension := pgschema.Extension{Schema: schema, Name: name}
	registerExtension(extension)
	return &ExtensionDef{def: extension}
}

func EnumType(name string, values ...string) *EnumDef {
	return EnumTypeInSchema("", name, values...)
}

func EnumTypeInSchema(schema, name string, values ...string) *EnumDef {
	enum := pgschema.Enum{Schema: schema, Name: name, Values: append([]string(nil), values...)}
	registerEnum(enum)
	return &EnumDef{def: enum}
}

func EnumColumn(name string, enum *EnumDef) *Column {
	return column(name, enum.TypeName())
}

func (e *EnumDef) TypeName() string {
	if e.def.Schema == "" {
		return e.def.Name
	}
	return strings.Join([]string{e.def.Schema, e.def.Name}, ".")
}
