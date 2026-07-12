package sqlite

import (
	"sync"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/render"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	"github.com/webdeveloperben/gosqlkit/kit"
)

var registry = struct {
	tables   []*sqliteschema.Table
	views    []*sqliteschema.View
	triggers []*sqliteschema.Trigger
	rawSQL   []*sqliteschema.RawSQL
	sync.Mutex
}{}

func init() {
	kit.Register(provider{})
}

type provider struct{}

func (provider) Dialect() kit.DialectInfo {
	return kit.DialectInfo{
		Name:    "sqlite",
		Aliases: []string{"sqlite3"},
		Capabilities: kit.Capabilities{
			Tables:        true,
			ForeignKeys:   true,
			Checks:        true,
			Indexes:       true,
			Snapshots:     true,
			Views:         true,
			Triggers:      true,
			RenderSQL:     true,
			SnapshotJSON:  true,
			MigrationPlan: true,
		},
	}
}

func (provider) RenderSQL() (string, error) {
	return Render()
}

func (provider) SnapshotJSON() ([]byte, error) {
	return SnapshotJSON()
}

func Schema() sqliteschema.Schema {
	registry.Lock()
	defer registry.Unlock()

	tables := make([]sqliteschema.Table, 0, len(registry.tables))
	for _, table := range registry.tables {
		tables = append(tables, *table)
	}

	views := make([]sqliteschema.View, 0, len(registry.views))
	for _, view := range registry.views {
		views = append(views, *view)
	}

	triggers := make([]sqliteschema.Trigger, 0, len(registry.triggers))
	for _, trigger := range registry.triggers {
		triggers = append(triggers, *trigger)
	}

	rawSQL := make([]sqliteschema.RawSQL, 0, len(registry.rawSQL))
	for _, block := range registry.rawSQL {
		rawSQL = append(rawSQL, *block)
	}

	return sqliteschema.Schema{
		Tables:   tables,
		Views:    views,
		Triggers: triggers,
		RawSQL:   rawSQL,
	}
}

func Render() (string, error) {
	return render.SQLite(Schema())
}

func SnapshotJSON() ([]byte, error) {
	schema := Schema()
	if _, err := render.SQLite(schema); err != nil {
		return nil, err
	}
	return sqliteschema.JSON("sqlite", schema)
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
	registry.views = nil
	registry.triggers = nil
	registry.rawSQL = nil
}

func register(table *sqliteschema.Table) {
	registry.Lock()
	defer registry.Unlock()

	registry.tables = append(registry.tables, table)
}

func registerView(view *sqliteschema.View) {
	registry.Lock()
	defer registry.Unlock()

	registry.views = append(registry.views, view)
}

func registerTrigger(trigger *sqliteschema.Trigger) {
	registry.Lock()
	defer registry.Unlock()

	registry.triggers = append(registry.triggers, trigger)
}

func registerRawSQL(block *sqliteschema.RawSQL) {
	registry.Lock()
	defer registry.Unlock()

	registry.rawSQL = append(registry.rawSQL, block)
}
