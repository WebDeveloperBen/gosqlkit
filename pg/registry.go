package pg

import (
	"sync"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/pgsnapshot"
	"github.com/webdeveloperben/gosqlkit/internal/render"
	"github.com/webdeveloperben/gosqlkit/kit"
)

var registry = struct {
	namespaces []pgschema.Namespace
	extensions []pgschema.Extension
	enums      []pgschema.Enum
	tables     []ast.Table
	sync.Mutex
}{}

func init() {
	kit.Register(provider{})
}

type provider struct{}

func (provider) Dialect() kit.DialectInfo {
	return kit.DialectInfo{
		Name:    "postgres",
		Aliases: []string{"pg", "postgresql"},
		Capabilities: kit.Capabilities{
			Tables:          true,
			Schemas:         true,
			Extensions:      true,
			Enums:           true,
			ForeignKeys:     true,
			Checks:          true,
			Indexes:         true,
			AdvancedIndexes: true,
			Snapshots:       true,
		},
	}
}

func (provider) RenderSQL() (string, error) {
	return Render()
}

func (provider) SnapshotJSON() ([]byte, error) {
	return SnapshotJSON()
}

func Schema() pgschema.Schema {
	registry.Lock()
	defer registry.Unlock()

	namespaces := append([]pgschema.Namespace(nil), registry.namespaces...)
	extensions := append([]pgschema.Extension(nil), registry.extensions...)
	enums := append([]pgschema.Enum(nil), registry.enums...)
	tables := append([]ast.Table(nil), registry.tables...)
	return pgschema.Schema{Namespaces: namespaces, Extensions: extensions, Enums: enums, Tables: tables}
}

func Render() (string, error) {
	return render.Postgres(Schema())
}

func SnapshotJSON() ([]byte, error) {
	schema := Schema()
	if _, err := render.Postgres(schema); err != nil {
		return nil, err
	}
	return pgsnapshot.JSON(schema)
}

func MustRender() string {
	sql, err := Render()
	if err != nil {
		panic(err)
	}
	return sql
}

func Reset() {
	registry.Lock()
	defer registry.Unlock()

	registry.tables = nil
	registry.namespaces = nil
	registry.extensions = nil
	registry.enums = nil
}

func register(table ast.Table) {
	registry.Lock()
	defer registry.Unlock()

	registry.tables = append(registry.tables, table)
}

func registerNamespace(namespace pgschema.Namespace) {
	registry.Lock()
	defer registry.Unlock()

	registry.namespaces = append(registry.namespaces, namespace)
}

func registerExtension(extension pgschema.Extension) {
	registry.Lock()
	defer registry.Unlock()

	registry.extensions = append(registry.extensions, extension)
}

func registerEnum(enum pgschema.Enum) {
	registry.Lock()
	defer registry.Unlock()

	registry.enums = append(registry.enums, enum)
}
