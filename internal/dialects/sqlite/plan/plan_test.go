package plan_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/plan"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
	"github.com/webdeveloperben/gosqlkit/sqlite"
)

func snap(t *testing.T, schema sqliteschema.Schema) []byte {
	t.Helper()
	data, err := sqliteschema.JSON("sqlite", schema)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return data
}

func col(name, typ string) sqliteschema.Column {
	return sqliteschema.Column{Column: ast.Column{Name: name, Type: typ}}
}

func diff(t *testing.T, previous, current sqliteschema.Schema) *migrateplan.Plan {
	t.Helper()
	planned, err := plan.SnapshotDiff(snap(t, previous), snap(t, current))
	if err != nil {
		t.Fatalf("SnapshotDiff: %v", err)
	}
	return planned
}

func statements(planned *migrateplan.Plan) string {
	return strings.Join(planned.Statements, "\n")
}

func table(name string, columns ...sqliteschema.Column) sqliteschema.Table {
	return sqliteschema.Table{Table: ast.Table{Name: name}, Columns: columns}
}

func TestCreateTable(t *testing.T) {
	current := sqliteschema.Schema{Tables: []sqliteschema.Table{
		table("users", col("id", "integer"), col("email", "text")),
	}}
	planned := diff(t, sqliteschema.Schema{}, current)
	if len(planned.Changes) != 1 || planned.Changes[0].Op != migrateplan.OperationCreate {
		t.Fatalf("want 1 create change, got %+v", planned.Changes)
	}
	if !strings.Contains(statements(planned), "CREATE TABLE users (") {
		t.Fatalf("missing CREATE TABLE:\n%s", statements(planned))
	}
}

func TestAddColumnSafe(t *testing.T) {
	previous := sqliteschema.Schema{Tables: []sqliteschema.Table{table("users", col("id", "integer"))}}
	current := sqliteschema.Schema{Tables: []sqliteschema.Table{
		table("users", col("id", "integer"), col("nickname", "text")),
	}}
	planned := diff(t, previous, current)
	if got := statements(planned); !strings.Contains(got, "ALTER TABLE users ADD COLUMN nickname text;") {
		t.Fatalf("want ADD COLUMN, got:\n%s", got)
	}
}

func TestNotNullWithoutDefaultForcesRebuild(t *testing.T) {
	previous := sqliteschema.Schema{Tables: []sqliteschema.Table{table("users", col("id", "integer"))}}
	notNull := col("nickname", "text")
	notNull.NotNull = true
	current := sqliteschema.Schema{Tables: []sqliteschema.Table{table("users", col("id", "integer"), notNull)}}
	planned := diff(t, previous, current)
	got := statements(planned)
	if !strings.Contains(got, "PRAGMA foreign_keys=OFF;") || !strings.Contains(got, "users_gosqlkit_new") {
		t.Fatalf("want table rebuild, got:\n%s", got)
	}
}

func TestColumnTypeChangeRebuilds(t *testing.T) {
	previous := sqliteschema.Schema{Tables: []sqliteschema.Table{table("t", col("id", "integer"), col("amount", "integer"))}}
	current := sqliteschema.Schema{Tables: []sqliteschema.Table{table("t", col("id", "integer"), col("amount", "real"))}}
	planned := diff(t, previous, current)
	got := statements(planned)
	for _, want := range []string{
		"PRAGMA foreign_keys=OFF;",
		"CREATE TABLE t_gosqlkit_new (",
		"INSERT INTO t_gosqlkit_new (id, amount) SELECT id, amount FROM t;",
		"DROP TABLE t;",
		"ALTER TABLE t_gosqlkit_new RENAME TO t;",
		"PRAGMA foreign_keys=ON;",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rebuild missing %q:\n%s", want, got)
		}
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Op != migrateplan.OperationAlter {
		t.Fatalf("want 1 alter change, got %+v", planned.Changes)
	}
}

func TestDropColumnRebuildIsDestructive(t *testing.T) {
	previous := sqliteschema.Schema{Tables: []sqliteschema.Table{table("t", col("id", "integer"), col("stale", "text"))}}
	current := sqliteschema.Schema{Tables: []sqliteschema.Table{table("t", col("id", "integer"))}}
	planned := diff(t, previous, current)
	if !planned.HasDataLoss() {
		t.Fatalf("dropping a column via rebuild should be data-loss:\n%s", statements(planned))
	}
	got := statements(planned)
	if !strings.Contains(got, "INSERT INTO t_gosqlkit_new (id) SELECT id FROM t;") {
		t.Fatalf("rebuild should copy only surviving columns:\n%s", got)
	}
}

func TestDropTableDestructive(t *testing.T) {
	previous := sqliteschema.Schema{Tables: []sqliteschema.Table{table("t", col("id", "integer"))}}
	planned := diff(t, previous, sqliteschema.Schema{})
	if !planned.HasDestructive() {
		t.Fatalf("dropping a table should be destructive")
	}
	if !strings.Contains(statements(planned), "DROP TABLE t;") {
		t.Fatalf("missing DROP TABLE:\n%s", statements(planned))
	}
}

func TestRenameTable(t *testing.T) {
	previous := sqliteschema.Schema{Tables: []sqliteschema.Table{table("accounts", col("id", "integer"))}}
	renamed := table("users", col("id", "integer"))
	renamed.PreviousName = "accounts"
	planned := diff(t, previous, sqliteschema.Schema{Tables: []sqliteschema.Table{renamed}})
	if len(planned.Changes) != 1 || planned.Changes[0].Op != migrateplan.OperationRename {
		t.Fatalf("want 1 rename change, got %+v", planned.Changes)
	}
	if !strings.Contains(statements(planned), "ALTER TABLE accounts RENAME TO users;") {
		t.Fatalf("missing RENAME TO:\n%s", statements(planned))
	}
}

func TestRenameTablePlusChangeFailsClosed(t *testing.T) {
	previous := sqliteschema.Schema{Tables: []sqliteschema.Table{table("accounts", col("id", "integer"))}}
	renamed := table("users", col("id", "integer"), col("email", "text"))
	renamed.PreviousName = "accounts"
	_, err := plan.SnapshotDiff(snap(t, previous), snap(t, sqliteschema.Schema{Tables: []sqliteschema.Table{renamed}}))
	if err == nil || !strings.Contains(err.Error(), "combined with other changes") {
		t.Fatalf("want rename-plus-change failure, got %v", err)
	}
}

func TestRenameColumn(t *testing.T) {
	previous := sqliteschema.Schema{Tables: []sqliteschema.Table{table("t", col("id", "integer"), col("email", "text"))}}
	renamedCol := col("email_address", "text")
	renamedCol.PreviousName = "email"
	current := sqliteschema.Schema{Tables: []sqliteschema.Table{table("t", col("id", "integer"), renamedCol)}}
	planned := diff(t, previous, current)
	if !strings.Contains(statements(planned), "ALTER TABLE t RENAME COLUMN email TO email_address;") {
		t.Fatalf("missing RENAME COLUMN:\n%s", statements(planned))
	}
}

func TestIndexAddAndDropOnExistingTable(t *testing.T) {
	base := table("t", col("id", "integer"), col("email", "text"))
	withIndex := base
	withIndex.Indexes = []sqliteschema.Index{{
		Index:   ast.Index{Name: "t_email_idx", Unique: true},
		Columns: []sqliteschema.IndexColumn{{IndexColumn: ast.IndexColumn{Expression: "email"}}},
	}}
	// add
	planned := diff(t, sqliteschema.Schema{Tables: []sqliteschema.Table{base}}, sqliteschema.Schema{Tables: []sqliteschema.Table{withIndex}})
	if !strings.Contains(statements(planned), "CREATE UNIQUE INDEX t_email_idx ON t (email);") {
		t.Fatalf("missing CREATE INDEX:\n%s", statements(planned))
	}
	// drop
	planned = diff(t, sqliteschema.Schema{Tables: []sqliteschema.Table{withIndex}}, sqliteschema.Schema{Tables: []sqliteschema.Table{base}})
	if !strings.Contains(statements(planned), "DROP INDEX t_email_idx;") {
		t.Fatalf("missing DROP INDEX:\n%s", statements(planned))
	}
}

func TestRenameIndexProducesReversibleDropCreate(t *testing.T) {
	oldIndex := sqliteschema.Index{
		Index:   ast.Index{Name: "t_email_idx", Unique: true},
		Columns: []sqliteschema.IndexColumn{{IndexColumn: ast.IndexColumn{Expression: "email"}}},
	}
	newIndex := oldIndex
	newIndex.Name = "t_address_idx"
	newIndex.PreviousName = "t_email_idx"
	newIndex.IfNotExists = true
	oldTable := table("t", col("email", "text"))
	oldTable.Indexes = []sqliteschema.Index{oldIndex}
	newTable := table("t", col("email", "text"))
	newTable.Indexes = []sqliteschema.Index{newIndex}

	planned := diff(t, sqliteschema.Schema{Tables: []sqliteschema.Table{oldTable}}, sqliteschema.Schema{Tables: []sqliteschema.Table{newTable}})
	if len(planned.Changes) != 1 {
		t.Fatalf("want one replacement change, got %+v", planned.Changes)
	}
	change := planned.Changes[0]
	wantStatements := []string{"DROP INDEX t_email_idx;", "CREATE UNIQUE INDEX IF NOT EXISTS t_address_idx ON t (email);"}
	wantReverse := []string{"DROP INDEX t_address_idx;", "CREATE UNIQUE INDEX t_email_idx ON t (email);"}
	if change.Op != migrateplan.OperationReplace || !change.Reversible || len(change.Statements) != len(wantStatements) || len(change.ReverseStatements) != len(wantReverse) {
		t.Fatalf("rename must be a reversible drop/create replacement: %+v", change)
	}
	for i, want := range wantStatements {
		if change.Statements[i].SQL != want {
			t.Fatalf("forward statement %d: got %q, want %q", i, change.Statements[i].SQL, want)
		}
	}
	for i, want := range wantReverse {
		if change.ReverseStatements[i].SQL != want {
			t.Fatalf("reverse statement %d: got %q, want %q", i, change.ReverseStatements[i].SQL, want)
		}
	}
}

func TestIndexRenameWithDefinitionChangeFailsClosed(t *testing.T) {
	oldTable := table("t", col("email", "text"), col("address", "text"))
	oldTable.Indexes = []sqliteschema.Index{{
		Index:   ast.Index{Name: "t_email_idx"},
		Columns: []sqliteschema.IndexColumn{{IndexColumn: ast.IndexColumn{Expression: "email"}}},
	}}
	newTable := table("t", col("email", "text"), col("address", "text"))
	newTable.Indexes = []sqliteschema.Index{{
		Index:   ast.Index{Name: "t_address_idx", PreviousName: "t_email_idx"},
		Columns: []sqliteschema.IndexColumn{{IndexColumn: ast.IndexColumn{Expression: "address"}}},
	}}
	planned, err := plan.SnapshotDiff(
		snap(t, sqliteschema.Schema{Tables: []sqliteschema.Table{oldTable}}),
		snap(t, sqliteschema.Schema{Tables: []sqliteschema.Table{newTable}}),
	)
	if err == nil || planned != nil || !strings.Contains(err.Error(), "combined with definition changes") {
		t.Fatalf("want a fail-closed index rename error with no plan, got plan=%+v err=%v", planned, err)
	}
}

func TestIndexRenameWithUnmatchedPreviousNameFailsClosed(t *testing.T) {
	oldTable := table("t", col("email", "text"))
	newTable := table("t", col("email", "text"))
	newTable.Indexes = []sqliteschema.Index{{
		Index:   ast.Index{Name: "t_new_idx", PreviousName: "missing_idx"},
		Columns: []sqliteschema.IndexColumn{{IndexColumn: ast.IndexColumn{Expression: "email"}}},
	}}
	planned, err := plan.SnapshotDiff(
		snap(t, sqliteschema.Schema{Tables: []sqliteschema.Table{oldTable}}),
		snap(t, sqliteschema.Schema{Tables: []sqliteschema.Table{newTable}}),
	)
	if err == nil || planned != nil || !strings.Contains(err.Error(), "previousName") {
		t.Fatalf("want an unmatched-index error with no plan, got plan=%+v err=%v", planned, err)
	}
}

func TestRebuildRecreatesTriggersOnTable(t *testing.T) {
	trigger := sqliteschema.Trigger{
		Name: "t_touch", Target: "t", Timing: "AFTER", Events: []string{"UPDATE"},
		Body: "SELECT 1;",
	}
	previous := sqliteschema.Schema{
		Tables:   []sqliteschema.Table{table("t", col("id", "integer"), col("amount", "integer"))},
		Triggers: []sqliteschema.Trigger{trigger},
	}
	// Only the column type changes; the trigger is unchanged.
	current := sqliteschema.Schema{
		Tables:   []sqliteschema.Table{table("t", col("id", "integer"), col("amount", "real"))},
		Triggers: []sqliteschema.Trigger{trigger},
	}
	planned := diff(t, previous, current)
	if len(planned.Changes) != 1 {
		t.Fatalf("want a single rebuild change that owns the trigger, got %+v", planned.Changes)
	}
	got := statements(planned)
	if !strings.Contains(got, "CREATE TRIGGER t_touch") {
		t.Fatalf("rebuild must recreate the unchanged trigger it dropped:\n%s", got)
	}
	// The recreate must come after the table is swapped back into place.
	if strings.Index(got, "RENAME TO t;") > strings.Index(got, "CREATE TRIGGER t_touch") {
		t.Fatalf("trigger recreated before table swap:\n%s", got)
	}
}

func TestNoChangesEmptyPlan(t *testing.T) {
	schema := sqliteschema.Schema{Tables: []sqliteschema.Table{table("t", col("id", "integer"))}}
	planned := diff(t, schema, schema)
	if len(planned.Changes) != 0 {
		t.Fatalf("want empty plan, got %+v", planned.Changes)
	}
}

func TestIfNotExistsChangesDoNotPlanDDL(t *testing.T) {
	previousTable := table("t", col("id", "integer"))
	previousTable.Indexes = []sqliteschema.Index{
		{
			Index: ast.Index{Name: "t_id_idx"},
			Columns: []sqliteschema.IndexColumn{
				{IndexColumn: ast.IndexColumn{Expression: "id"}},
			},
		},
	}
	currentTable := table("t", col("id", "integer"))
	currentTable.IfNotExists = true
	currentTable.Indexes = []sqliteschema.Index{
		{
			Index:       ast.Index{Name: "t_id_idx"},
			IfNotExists: true,
			Columns: []sqliteschema.IndexColumn{
				{IndexColumn: ast.IndexColumn{Expression: "id"}},
			},
		},
	}
	previous := sqliteschema.Schema{
		Tables: []sqliteschema.Table{previousTable},
		Views:  []sqliteschema.View{{Name: "v", Query: "SELECT id FROM t"}},
	}
	current := sqliteschema.Schema{
		Tables: []sqliteschema.Table{currentTable},
		Views:  []sqliteschema.View{{Name: "v", Query: "SELECT id FROM t", IfNotExists: true}},
	}
	planned := diff(t, previous, current)
	if len(planned.Changes) != 0 {
		t.Fatalf("IF NOT EXISTS changes must not produce migration DDL: %+v", planned.Changes)
	}
}

func TestRenameAnnotationsFailClosed(t *testing.T) {
	unmatchedColumn := col("new_column", "text")
	unmatchedColumn.PreviousName = "missing_column"
	firstColumn := col("first_column", "text")
	firstColumn.PreviousName = "old_column"
	secondColumn := col("second_column", "text")
	secondColumn.PreviousName = "old_column"

	unmatchedTable := table("new_table", col("id", "integer"))
	unmatchedTable.PreviousName = "missing_table"
	tableRenameA := table("new_a", col("id", "integer"))
	tableRenameA.PreviousName = "old_table"
	tableRenameB := table("new_b", col("id", "integer"))
	tableRenameB.PreviousName = "old_table"

	columnRenameTable := table("t", col("id", "integer"), unmatchedColumn)
	ambiguousColumnTable := table("t", col("id", "integer"), firstColumn, secondColumn)

	viewRenameA := sqliteschema.View{Name: "new_a", PreviousName: "old_view", Query: "SELECT 1"}
	viewRenameB := sqliteschema.View{Name: "new_b", PreviousName: "old_view", Query: "SELECT 1"}
	triggerRenameA := sqliteschema.Trigger{Name: "new_a", PreviousName: "old_trigger", Target: "t"}
	triggerRenameB := sqliteschema.Trigger{Name: "new_b", PreviousName: "old_trigger", Target: "t"}

	tests := []struct {
		name     string
		previous sqliteschema.Schema
		current  sqliteschema.Schema
	}{
		{
			name:     "unmatched table",
			previous: sqliteschema.Schema{Tables: []sqliteschema.Table{table("old_table", col("id", "integer"))}},
			current:  sqliteschema.Schema{Tables: []sqliteschema.Table{unmatchedTable}},
		},
		{
			name:     "ambiguous table",
			previous: sqliteschema.Schema{Tables: []sqliteschema.Table{table("old_table", col("id", "integer"))}},
			current:  sqliteschema.Schema{Tables: []sqliteschema.Table{tableRenameA, tableRenameB}},
		},
		{
			name:     "unmatched column",
			previous: sqliteschema.Schema{Tables: []sqliteschema.Table{table("t", col("id", "integer"))}},
			current:  sqliteschema.Schema{Tables: []sqliteschema.Table{columnRenameTable}},
		},
		{
			name: "ambiguous column",
			previous: sqliteschema.Schema{Tables: []sqliteschema.Table{
				table("t", col("id", "integer"), col("old_column", "text")),
			}},
			current: sqliteschema.Schema{Tables: []sqliteschema.Table{ambiguousColumnTable}},
		},
		{
			name:     "unmatched view",
			current:  sqliteschema.Schema{Views: []sqliteschema.View{{Name: "new_view", PreviousName: "missing_view", Query: "SELECT 1"}}},
			previous: sqliteschema.Schema{},
		},
		{
			name:     "ambiguous view",
			previous: sqliteschema.Schema{Views: []sqliteschema.View{{Name: "old_view", Query: "SELECT 1"}}},
			current:  sqliteschema.Schema{Views: []sqliteschema.View{viewRenameA, viewRenameB}},
		},
		{
			name:     "unmatched trigger",
			current:  sqliteschema.Schema{Triggers: []sqliteschema.Trigger{{Name: "new_trigger", PreviousName: "missing_trigger", Target: "t"}}},
			previous: sqliteschema.Schema{},
		},
		{
			name:     "ambiguous trigger",
			previous: sqliteschema.Schema{Triggers: []sqliteschema.Trigger{{Name: "old_trigger", Target: "t"}}},
			current:  sqliteschema.Schema{Triggers: []sqliteschema.Trigger{triggerRenameA, triggerRenameB}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			planned, err := plan.SnapshotDiff(snap(t, test.previous), snap(t, test.current))
			if err == nil || planned != nil {
				t.Fatalf("want an annotation error and no partial plan, got plan=%+v err=%v", planned, err)
			}
		})
	}
}

func TestPublicSQLiteRenamesPlanWithReverseSQL(t *testing.T) {
	sqlite.Reset()
	t.Cleanup(sqlite.Reset)

	sqlite.Table("accounts", sqlite.Integer("id").PrimaryKey())
	sqlite.Table(
		"profiles",
		sqlite.Integer("id").PrimaryKey(),
		sqlite.Text("handle"),
	)
	sqlite.Table(
		"activity",
		sqlite.Integer("id"),
		sqlite.Index("activity_id_old", "id"),
	)
	sqlite.View("old_view").As("SELECT id FROM profiles")
	sqlite.Trigger("old_trigger", "profiles").
		After().
		Insert().
		ForEachRow().
		Body("SELECT 1;")
	previous, err := sqlite.SnapshotJSON()
	if err != nil {
		t.Fatal(err)
	}

	sqlite.Reset()
	sqlite.Table("users", sqlite.Integer("id").PrimaryKey()).PreviousName("accounts")
	sqlite.Table(
		"profiles",
		sqlite.Integer("id").PrimaryKey(),
		sqlite.Text("display_name").PreviousName("handle"),
	)
	sqlite.Table(
		"activity",
		sqlite.Integer("id"),
		sqlite.Index("activity_id_new", "id").PreviousName("activity_id_old"),
	)
	sqlite.View("new_view").
		As("SELECT id FROM profiles").
		PreviousName("old_view")
	sqlite.Trigger("new_trigger", "profiles").
		After().
		Insert().
		ForEachRow().
		Body("SELECT 1;").
		PreviousName("old_trigger")
	current, err := sqlite.SnapshotJSON()
	if err != nil {
		t.Fatal(err)
	}

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	changes := make(map[string]migrateplan.Change, len(planned.Changes))
	for _, change := range planned.Changes {
		changes[string(change.Object.Kind)+":"+change.Object.Key] = change
	}
	assertChange := func(key string, operation migrateplan.Operation, forward, reverse []string) {
		t.Helper()
		change, found := changes[key]
		if !found {
			t.Fatalf("missing change %q in %+v", key, planned.Changes)
		}
		if change.Op != operation || !change.Reversible || len(change.Statements) != len(forward) || len(change.ReverseStatements) != len(reverse) {
			t.Fatalf("unexpected change %q: %+v", key, change)
		}
		for i, want := range forward {
			if got := strings.TrimSpace(change.Statements[i].SQL); got != want {
				t.Fatalf("change %q forward statement %d: got %q, want %q", key, i, got, want)
			}
		}
		for i, want := range reverse {
			if got := strings.TrimSpace(change.ReverseStatements[i].SQL); got != want {
				t.Fatalf("change %q reverse statement %d: got %q, want %q", key, i, got, want)
			}
		}
	}
	assertChange(
		"table:users", migrateplan.OperationRename,
		[]string{"ALTER TABLE accounts RENAME TO users;"},
		[]string{"ALTER TABLE users RENAME TO accounts;"},
	)
	assertChange(
		"column:profiles.display_name", migrateplan.OperationRename,
		[]string{"ALTER TABLE profiles RENAME COLUMN handle TO display_name;"},
		[]string{"ALTER TABLE profiles RENAME COLUMN display_name TO handle;"},
	)
	assertChange(
		"index:activity_id_new", migrateplan.OperationReplace,
		[]string{"DROP INDEX activity_id_old;", "CREATE INDEX activity_id_new ON activity (id);"},
		[]string{"DROP INDEX activity_id_new;", "CREATE INDEX activity_id_old ON activity (id);"},
	)
	assertChange(
		"view:new_view", migrateplan.OperationReplace,
		[]string{"DROP VIEW old_view;", "CREATE VIEW new_view AS SELECT id FROM profiles;"},
		[]string{"DROP VIEW new_view;", "CREATE VIEW old_view AS SELECT id FROM profiles;"},
	)
	assertChange(
		"trigger:new_trigger", migrateplan.OperationReplace,
		[]string{
			"DROP TRIGGER old_trigger;",
			"CREATE TRIGGER new_trigger\n    AFTER INSERT ON profiles\n    FOR EACH ROW\nBEGIN\n    SELECT 1;\nEND;",
		},
		[]string{
			"DROP TRIGGER new_trigger;",
			"CREATE TRIGGER old_trigger\n    AFTER INSERT ON profiles\n    FOR EACH ROW\nBEGIN\n    SELECT 1;\nEND;",
		},
	)
}

func TestPublicSQLiteTableRenamePlusChangeFailsClosed(t *testing.T) {
	sqlite.Reset()
	t.Cleanup(sqlite.Reset)

	sqlite.Table("accounts", sqlite.Integer("id").PrimaryKey())
	previous, err := sqlite.SnapshotJSON()
	if err != nil {
		t.Fatal(err)
	}

	sqlite.Reset()
	sqlite.Table(
		"users",
		sqlite.Integer("id").PrimaryKey(),
		sqlite.Text("email"),
	).PreviousName("accounts")
	current, err := sqlite.SnapshotJSON()
	if err != nil {
		t.Fatal(err)
	}

	planned, err := plan.SnapshotDiff(previous, current)
	if err == nil || planned != nil || !strings.Contains(err.Error(), "combined with other changes") {
		t.Fatalf("want fail-closed rename-plus-change error and no plan, got plan=%+v err=%v", planned, err)
	}
}
