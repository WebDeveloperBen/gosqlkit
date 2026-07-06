package pg

import (
	"sync"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgsnapshot"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/render"
	"github.com/webdeveloperben/gosqlkit/kit"
)

var registry = struct {
	namespaces     []pgschema.Namespace
	extensions     []pgschema.Extension
	enums          []pgschema.Enum
	sequences      []pgschema.Sequence
	compositeTypes []pgschema.CompositeType
	domains        []*pgschema.Domain
	tables         []*ast.Table
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
	sequences := append([]pgschema.Sequence(nil), registry.sequences...)
	compositeTypes := append([]pgschema.CompositeType(nil), registry.compositeTypes...)
	domains := make([]pgschema.Domain, 0, len(registry.domains))

	for _, domain := range registry.domains {
		domains = append(domains, *domain)
	}

	tables := make([]ast.Table, 0, len(registry.tables))

	for _, table := range registry.tables {
		tables = append(tables, *table)
	}

	return pgschema.Schema{
		Namespaces:     namespaces,
		Extensions:     extensions,
		Enums:          enums,
		Sequences:      sequences,
		CompositeTypes: compositeTypes,
		Domains:        domains,
		Tables:         tables,
	}
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
	registry.sequences = nil
	registry.compositeTypes = nil
	registry.domains = nil
}

func register(table *ast.Table) {
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

func registerSequence(sequence pgschema.Sequence) {
	registry.Lock()
	defer registry.Unlock()

	registry.sequences = append(registry.sequences, sequence)
}

func registerCompositeType(compositeType pgschema.CompositeType) {
	registry.Lock()
	defer registry.Unlock()

	registry.compositeTypes = append(registry.compositeTypes, compositeType)
}

func registerDomain(domain *pgschema.Domain) {
	registry.Lock()
	defer registry.Unlock()

	registry.domains = append(registry.domains, domain)
}
