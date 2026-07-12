package plan_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/plan"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
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
