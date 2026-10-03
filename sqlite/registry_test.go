package sqlite_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/kit"
	"github.com/webdeveloperben/gosqlkit/sqlite"
)

func TestProviderRegistersSQLiteDialectMetadata(t *testing.T) {
	info, err := kit.DialectInfoFor("sqlite3")
	if err != nil {
		t.Fatal(err)
	}

	if info.Name != "sqlite" {
		t.Fatalf("unexpected dialect name %q", info.Name)
	}
	if !slices.Contains(info.Aliases, "sqlite3") {
		t.Fatalf("expected sqlite3 alias, got %#v", info.Aliases)
	}
	caps := info.Capabilities
	if !caps.Tables || !caps.ForeignKeys || !caps.Checks || !caps.Indexes || !caps.Snapshots ||
		!caps.Views || !caps.Triggers || !caps.RenderSQL || !caps.SnapshotJSON {
		t.Fatalf("missing expected capability %#v", caps)
	}
	// SQLite does not model these PostgreSQL objects.
	if caps.Schemas || caps.Enums || caps.Extensions || caps.Roles || caps.Functions ||
		caps.RLS || caps.MaterializedViews || caps.AdvancedIndexes {
		t.Fatalf("unexpected capability enabled for SQLite %#v", caps)
	}
}

func TestRenderThroughRegistry(t *testing.T) {
	sqlite.Reset()
	t.Cleanup(sqlite.Reset)

	sqlite.Table(
		"users",
		sqlite.Integer("id").PrimaryKey().AutoIncrement(),
		sqlite.Text("email").NotNull().Unique().Collate(sqlite.NoCase),
		sqlite.Text("created_at").NotNull().DefaultCurrentTimestamp(),
	).Strict()

	sql, err := kit.RenderSQL("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"CREATE TABLE users (",
		"id integer PRIMARY KEY AUTOINCREMENT",
		"email text NOT NULL UNIQUE COLLATE NOCASE",
		"created_at text NOT NULL DEFAULT CURRENT_TIMESTAMP",
		") STRICT;",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("rendered SQL missing %q:\n%s", want, sql)
		}
	}

	snapshot, err := kit.SnapshotJSON("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(snapshot), `"dialect": "sqlite"`) {
		t.Fatalf("snapshot missing sqlite dialect marker:\n%s", snapshot)
	}
}

func TestCreationOptionsAppearInSQLiteSnapshot(t *testing.T) {
	sqlite.Reset()
	t.Cleanup(sqlite.Reset)

	sqlite.Table("users", sqlite.Integer("id").PrimaryKey()).IfNotExists()
	sqlite.Table(
		"posts",
		sqlite.Integer("id").PrimaryKey(),
		sqlite.Index("posts_id_idx", "id").IfNotExists(),
	)
	sqlite.View("user_ids").As("SELECT id FROM users").IfNotExists()

	snapshot, err := sqlite.SnapshotJSON()
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Tables []struct {
			Name    string `json:"name"`
			Indexes []struct {
				IfNotExists bool `json:"ifNotExists"`
			} `json:"indexes"`
			IfNotExists bool `json:"ifNotExists"`
		} `json:"tables"`
		Views []struct {
			IfNotExists bool `json:"ifNotExists"`
		} `json:"views"`
	}
	if err := json.Unmarshal(snapshot, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Tables) != 2 || document.Tables[0].Name != "posts" || document.Tables[1].Name != "users" || !document.Tables[1].IfNotExists || !document.Tables[0].Indexes[0].IfNotExists || len(document.Views) != 1 || !document.Views[0].IfNotExists {
		t.Fatalf("snapshot did not preserve creation options: %s", snapshot)
	}
}

func TestPreviousNameBuildersAppearInSQLiteSnapshot(t *testing.T) {
	sqlite.Reset()
	t.Cleanup(sqlite.Reset)

	sqlite.Table(
		"new_table",
		sqlite.Text("new_column").PreviousName("old_column"),
		sqlite.Index("new_index", "new_column").PreviousName("old_index"),
	).PreviousName("old_table")
	sqlite.View("new_view").As("SELECT new_column FROM new_table").PreviousName("old_view")
	sqlite.Trigger("new_trigger", "new_table").
		After().
		Insert().
		ForEachRow().
		Body("SELECT 1;").
		PreviousName("old_trigger")

	snapshot, err := sqlite.SnapshotJSON()
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Tables []struct {
			PreviousName string `json:"previousName"`
			Columns      []struct {
				PreviousName string `json:"previousName"`
			} `json:"columns"`
			Indexes []struct {
				PreviousName string `json:"previousName"`
			} `json:"indexes"`
		} `json:"tables"`
		Views []struct {
			PreviousName string `json:"previousName"`
		} `json:"views"`
		Triggers []struct {
			PreviousName string `json:"previousName"`
		} `json:"triggers"`
	}
	if err := json.Unmarshal(snapshot, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Tables) != 1 || document.Tables[0].PreviousName != "old_table" ||
		document.Tables[0].Columns[0].PreviousName != "old_column" ||
		document.Tables[0].Indexes[0].PreviousName != "old_index" ||
		len(document.Views) != 1 || document.Views[0].PreviousName != "old_view" ||
		len(document.Triggers) != 1 || document.Triggers[0].PreviousName != "old_trigger" {
		t.Fatalf("snapshot did not preserve previous names: %s", snapshot)
	}
}
