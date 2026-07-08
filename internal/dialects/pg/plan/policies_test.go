package plan_test

import (
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
)

func TestSnapshotDiffProducesPolicyDependencyAndReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:             "users",
			Columns:          []ast.Column{{Name: "id", Type: "uuid"}},
			RowLevelSecurity: true,
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:             "users",
			Columns:          []ast.Column{{Name: "id", Type: "uuid"}},
			RowLevelSecurity: true,
		}},
		Policies: []pgschema.Policy{{
			Name:    "users_read_self",
			Table:   "users",
			Command: "SELECT",
			Using:   "id = current_setting('app.user_id')::uuid",
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	change := planned.Changes[0]
	if change.Object.Kind != "policy" || change.Object.Key != "public.users.users_read_self" {
		t.Fatalf("policy object = %#v", change.Object)
	}
	if len(change.Dependencies) != 1 || change.Dependencies[0].Kind != "table" || change.Dependencies[0].Key != "public.users" {
		t.Fatalf("dependencies = %#v", change.Dependencies)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "DROP POLICY users_read_self ON users;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffEmitsRenamePolicy(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:             "users",
			Columns:          []ast.Column{{Name: "id", Type: "uuid"}},
			RowLevelSecurity: true,
		}},
		Policies: []pgschema.Policy{{
			Name:    "old_users_read_self",
			Table:   "users",
			Command: "SELECT",
			Using:   "true",
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:             "users",
			Columns:          []ast.Column{{Name: "id", Type: "uuid"}},
			RowLevelSecurity: true,
		}},
		Policies: []pgschema.Policy{{
			Name:         "users_read_self",
			PreviousName: "old_users_read_self",
			Table:        "users",
			Command:      "SELECT",
			Using:        "true",
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Op != "rename" {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	if planned.Changes[0].Statements[0].SQL != "ALTER POLICY old_users_read_self ON users RENAME TO users_read_self;" {
		t.Fatalf("sql = %q", planned.Changes[0].Statements[0].SQL)
	}
}
