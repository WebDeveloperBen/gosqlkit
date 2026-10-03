package app

import (
	"reflect"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

func TestSQLiteDriftProjectionIgnoresMigrationStateAndUnobservableMetadata(t *testing.T) {
	schema := sqliteschema.Schema{
		Tables: []sqliteschema.Table{
			{Table: ast.Table{Name: "users", PreviousName: "old_users", Comment: "schema-only comment"}, Columns: []sqliteschema.Column{{Column: ast.Column{Name: "id", Type: "integer", PreviousName: "old_id", Comment: "not stored in SQLite"}}}},
			{Table: ast.Table{Name: "goose_db_version"}},
		},
		Views:    []sqliteschema.View{{Name: "user_view", PreviousName: "old_view", Query: "SELECT id FROM users", Comment: "not stored"}},
		Triggers: []sqliteschema.Trigger{{Name: "users_touch", Target: "users", PreviousName: "old_trigger", Events: []string{"UPDATE"}, Body: "SELECT 1;", Comment: "not stored"}},
		RawSQL:   []sqliteschema.RawSQL{{Name: "opaque", SQL: "CREATE VIRTUAL TABLE x USING extension"}},
	}
	projected := projectSQLiteDriftSchema(schema)
	if len(projected.Tables) != 1 || projected.Tables[0].Name != "users" {
		t.Fatalf("projected tables = %#v", projected.Tables)
	}
	if projected.Tables[0].PreviousName != "" || projected.Tables[0].Comment != "" || projected.Tables[0].Columns[0].PreviousName != "" || projected.Tables[0].Columns[0].Comment != "" {
		t.Fatalf("table metadata remains in projection: %#v", projected.Tables[0])
	}
	if projected.Views[0].PreviousName != "" || projected.Views[0].Comment != "" || projected.Triggers[0].PreviousName != "" || projected.Triggers[0].Comment != "" {
		t.Fatalf("view/trigger metadata remains in projection: %#v %#v", projected.Views, projected.Triggers)
	}
	if len(projected.RawSQL) != 0 {
		t.Fatalf("opaque raw SQL entered drift projection: %#v", projected.RawSQL)
	}
}

func TestSQLiteDriftProjectionIgnoresIfNotExistsOptions(t *testing.T) {
	desired := sqliteschema.Schema{
		Tables: []sqliteschema.Table{{
			Table:       ast.Table{Name: "users"},
			Indexes:     []sqliteschema.Index{{Index: ast.Index{Name: "users_email_idx"}, IfNotExists: true}},
			IfNotExists: true,
		}},
		Views: []sqliteschema.View{{Name: "user_emails", Query: "SELECT email FROM users", IfNotExists: true}},
	}
	database := sqliteschema.Schema{
		Tables: []sqliteschema.Table{{
			Table:   ast.Table{Name: "users"},
			Indexes: []sqliteschema.Index{{Index: ast.Index{Name: "users_email_idx"}}},
		}},
		Views: []sqliteschema.View{{Name: "user_emails", Query: "SELECT email FROM users"}},
	}
	projected := projectSQLiteDriftSchema(desired)
	actual := projectSQLiteDriftSchema(database)
	if !reflect.DeepEqual(projected, actual) {
		t.Fatalf("SQLite creation options changed drift projection:\nwant %#v\ngot  %#v", actual, projected)
	}
	if len(projected.Views) != 1 || projected.Views[0].IfNotExists {
		t.Fatalf("view creation option was not projected away: %#v", projected.Views)
	}
}

func TestSQLiteDriftDifferencesReportNestedColumnChanges(t *testing.T) {
	desired := sqliteschema.Schema{Tables: []sqliteschema.Table{{
		Table:   ast.Table{Name: "users"},
		Columns: []sqliteschema.Column{{Column: ast.Column{Name: "id", Type: "integer"}}},
	}}}
	database := sqliteschema.Schema{Tables: []sqliteschema.Table{{
		Table:   ast.Table{Name: "users"},
		Columns: []sqliteschema.Column{{Column: ast.Column{Name: "id", Type: "text"}}},
	}}}
	got := sqliteDriftDifferences(projectSQLiteDriftSchema(desired), projectSQLiteDriftSchema(database))
	want := []DriftDifference{{
		Op:     "changed",
		Object: DriftObjectRef{Kind: "column", Key: "users.id"},
		Fields: []string{"type"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SQLite drift differences = %#v, want %#v", got, want)
	}
}

func TestSQLiteDriftProjectionNormalizesAffinityRowIDAndCatalogFormatting(t *testing.T) {
	desired := sqliteschema.Schema{Tables: []sqliteschema.Table{{
		Table: ast.Table{
			Name:        "users",
			PrimaryKeys: []ast.PrimaryKey{{Name: "users_pkey", Columns: []string{"id"}}},
		},
		Columns: []sqliteschema.Column{
			{Column: ast.Column{Name: "id", Type: "INTEGER"}},
			{Column: ast.Column{Name: "label", Type: "VARCHAR(32)", Default: "('Mixed  Case')"}},
		},
	}}}
	database := sqliteschema.Schema{Tables: []sqliteschema.Table{{
		Table: ast.Table{Name: "users"},
		Columns: []sqliteschema.Column{
			{Column: ast.Column{Name: "id", Type: "integer", PrimaryKey: true}},
			{Column: ast.Column{Name: "label", Type: "TEXT", Default: "(  'Mixed  Case'  )"}},
		},
	}}}
	if projected, actual := projectSQLiteDriftSchema(desired), projectSQLiteDriftSchema(database); !reflect.DeepEqual(projected, actual) {
		t.Fatalf("SQLite equivalent schema projections differ:\nwant %#v\ngot  %#v", projected, actual)
	}
}

func TestSQLiteTemporaryViewsDoNotCreateDrift(t *testing.T) {
	declared := sqliteschema.Schema{Views: []sqliteschema.View{{
		Name:      "session_user_ids",
		Query:     "SELECT id FROM users",
		Temporary: true,
	}}}
	differences := sqliteDriftDifferences(projectSQLiteDriftSchema(declared), projectSQLiteDriftSchema(sqliteschema.Schema{}))
	if len(differences) != 0 {
		t.Fatalf("temporary view produced persistent-object drift: %#v", differences)
	}
	if len(declared.Views) != 1 || !declared.Views[0].Temporary {
		t.Fatalf("declared temporary view was lost: %#v", declared.Views)
	}
}
