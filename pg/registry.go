package pg

import (
	"sync"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/render"
	"github.com/webdeveloperben/gosqlkit/kit"
)

var registry = struct {
	namespaces        []pgschema.Namespace
	extensions        []pgschema.Extension
	roles             []*pgschema.Role
	enums             []pgschema.Enum
	sequences         []pgschema.Sequence
	compositeTypes    []pgschema.CompositeType
	domains           []*pgschema.Domain
	functions         []*pgschema.Function
	tables            []*pgschema.Table
	views             []*pgschema.View
	materializedViews []*pgschema.MaterializedView
	triggers          []*pgschema.Trigger
	policies          []*pgschema.Policy
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
			Tables:            true,
			Schemas:           true,
			Extensions:        true,
			Enums:             true,
			ForeignKeys:       true,
			Checks:            true,
			Indexes:           true,
			AdvancedIndexes:   true,
			Snapshots:         true,
			Views:             true,
			MaterializedViews: true,
			Roles:             true,
			Functions:         true,
			Triggers:          true,
			RLS:               true,
			RenderSQL:         true,
			SnapshotJSON:      true,
			InspectDatabase:   true,
			DriftCheck:        true,
			MigrationPlan:     true,
			MigrationApply:    true,
			SandboxReplay:     true,
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
	roles := make([]pgschema.Role, 0, len(registry.roles))
	enums := append([]pgschema.Enum(nil), registry.enums...)
	sequences := append([]pgschema.Sequence(nil), registry.sequences...)
	compositeTypes := append([]pgschema.CompositeType(nil), registry.compositeTypes...)
	domains := make([]pgschema.Domain, 0, len(registry.domains))
	functions := make([]pgschema.Function, 0, len(registry.functions))

	for _, role := range registry.roles {
		roles = append(roles, *role)
	}

	for _, domain := range registry.domains {
		domains = append(domains, *domain)
	}

	for _, function := range registry.functions {
		functions = append(functions, *function)
	}

	tables := make([]pgschema.Table, 0, len(registry.tables))

	for _, table := range registry.tables {
		tables = append(tables, *table)
	}

	views := make([]pgschema.View, 0, len(registry.views))
	for _, view := range registry.views {
		views = append(views, *view)
	}

	materializedViews := make([]pgschema.MaterializedView, 0, len(registry.materializedViews))
	for _, mv := range registry.materializedViews {
		materializedViews = append(materializedViews, *mv)
	}

	triggers := make([]pgschema.Trigger, 0, len(registry.triggers))
	for _, trigger := range registry.triggers {
		triggers = append(triggers, *trigger)
	}

	policies := make([]pgschema.Policy, 0, len(registry.policies))
	for _, policy := range registry.policies {
		policies = append(policies, *policy)
	}

	return pgschema.Schema{
		Namespaces:        namespaces,
		Extensions:        extensions,
		Roles:             roles,
		Enums:             enums,
		Sequences:         sequences,
		CompositeTypes:    compositeTypes,
		Domains:           domains,
		Functions:         functions,
		Tables:            tables,
		Views:             views,
		MaterializedViews: materializedViews,
		Triggers:          triggers,
		Policies:          policies,
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

	return pgschema.JSON("postgresql", schema)
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
	registry.roles = nil
	registry.enums = nil
	registry.sequences = nil
	registry.compositeTypes = nil
	registry.domains = nil
	registry.functions = nil
	registry.views = nil
	registry.materializedViews = nil
	registry.triggers = nil
	registry.policies = nil
}

func register(table *pgschema.Table) {
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

func registerRole(role *pgschema.Role) {
	registry.Lock()
	defer registry.Unlock()

	registry.roles = append(registry.roles, role)
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

func registerFunction(function *pgschema.Function) {
	registry.Lock()
	defer registry.Unlock()

	registry.functions = append(registry.functions, function)
}

func registerView(view *pgschema.View) {
	registry.Lock()
	defer registry.Unlock()

	registry.views = append(registry.views, view)
}

func registerMaterializedView(mv *pgschema.MaterializedView) {
	registry.Lock()
	defer registry.Unlock()

	registry.materializedViews = append(registry.materializedViews, mv)
}

func registerTrigger(trigger *pgschema.Trigger) {
	registry.Lock()
	defer registry.Unlock()

	registry.triggers = append(registry.triggers, trigger)
}

func registerPolicy(policy *pgschema.Policy) {
	registry.Lock()
	defer registry.Unlock()

	registry.policies = append(registry.policies, policy)
}
