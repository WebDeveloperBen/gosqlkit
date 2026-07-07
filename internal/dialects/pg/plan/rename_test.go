package plan_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
)

func TestSnapshotDiffEmitsRenameTable(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "old_users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:         "users",
			PreviousName: "old_users",
			Columns:      []ast.Column{{Name: "id", Type: "uuid"}},
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
	if change.Op != "rename" {
		t.Fatalf("op = %q", change.Op)
	}
	if change.Statements[0].SQL != "ALTER TABLE old_users RENAME TO users;" {
		t.Fatalf("sql = %q", change.Statements[0].SQL)
	}
	if change.ReverseStatements[0].SQL != "ALTER TABLE users RENAME TO old_users;" {
		t.Fatalf("reverse = %q", change.ReverseStatements[0].SQL)
	}
}

func TestSnapshotDiffEmitsRenameColumn(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name: "users",
			Columns: []ast.Column{
				{Name: "id", Type: "uuid"},
				{Name: "old_email", Type: "text"},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name: "users",
			Columns: []ast.Column{
				{Name: "id", Type: "uuid"},
				{Name: "email", PreviousName: "old_email", Type: "text"},
			},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Op != "rename" {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	if planned.Changes[0].Statements[0].SQL != "ALTER TABLE users RENAME COLUMN old_email TO email;" {
		t.Fatalf("sql = %q", planned.Changes[0].Statements[0].SQL)
	}
}

func TestSnapshotDiffEmitsRenameConstraint(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "email", Type: "text"}},
			UniqueConstraints: []ast.UniqueConstraint{{
				Name:    "old_users_email_key",
				Columns: []string{"email"},
			}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "email", Type: "text"}},
			UniqueConstraints: []ast.UniqueConstraint{{
				Name:         "users_email_key",
				PreviousName: "old_users_email_key",
				Columns:      []string{"email"},
			}},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Op != "rename" {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	if planned.Changes[0].Statements[0].SQL != "ALTER TABLE users RENAME CONSTRAINT old_users_email_key TO users_email_key;" {
		t.Fatalf("sql = %q", planned.Changes[0].Statements[0].SQL)
	}
}

func TestSnapshotDiffEmitsRenameIndex(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "email", Type: "text"}},
			Indexes: []ast.Index{{
				Name:    "old_users_email_idx",
				Columns: []ast.IndexColumn{{Expression: "email"}},
			}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "email", Type: "text"}},
			Indexes: []ast.Index{{
				Name:         "users_email_idx",
				PreviousName: "old_users_email_idx",
				Columns:      []ast.IndexColumn{{Expression: "email"}},
			}},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Op != "rename" {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	if planned.Changes[0].Statements[0].SQL != "ALTER INDEX old_users_email_idx RENAME TO users_email_idx;" {
		t.Fatalf("sql = %q", planned.Changes[0].Statements[0].SQL)
	}
}

func TestSnapshotDiffEmitsRenameEnum(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Enums: []pgschema.Enum{{
			Name:   "old_invoice_status",
			Values: []string{"draft"},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Enums: []pgschema.Enum{{
			Name:         "invoice_status",
			PreviousName: "old_invoice_status",
			Values:       []string{"draft"},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Op != "rename" {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	if planned.Changes[0].Statements[0].SQL != "ALTER TYPE old_invoice_status RENAME TO invoice_status;" {
		t.Fatalf("sql = %q", planned.Changes[0].Statements[0].SQL)
	}
}

func TestSnapshotDiffEmitsRenameSchema(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{
			Name: "old_billing",
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{
			Name:         "billing",
			PreviousName: "old_billing",
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Op != "rename" {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	if planned.Changes[0].Statements[0].SQL != "ALTER SCHEMA old_billing RENAME TO billing;" {
		t.Fatalf("sql = %q", planned.Changes[0].Statements[0].SQL)
	}
}

func TestSnapshotDiffEmitsRenameRole(t *testing.T) {
	login := true
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Roles: []pgschema.Role{{
			Name:  "old_app_reader",
			Login: &login,
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Roles: []pgschema.Role{{
			Name:         "app_reader",
			PreviousName: "old_app_reader",
			Login:        &login,
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Op != "rename" {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	if planned.Changes[0].Statements[0].SQL != "ALTER ROLE old_app_reader RENAME TO app_reader;" {
		t.Fatalf("sql = %q", planned.Changes[0].Statements[0].SQL)
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

func TestSnapshotDiffRejectsRenameColumnWithTypeChange(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name: "users",
			Columns: []ast.Column{
				{Name: "id", Type: "uuid"},
				{Name: "old_email", Type: "text"},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name: "users",
			Columns: []ast.Column{
				{Name: "id", Type: "uuid"},
				{Name: "email", PreviousName: "old_email", Type: "varchar(320)"},
			},
		}},
	})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil || !strings.Contains(err.Error(), "rename from public.users.old_email combined with other modifications requires semantic planning") {
		t.Fatalf("expected rename+alter error, got %v", err)
	}
}

func TestSnapshotDiffRejectsRenameMismatchedPreviousName(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:         "people",
			PreviousName: "missing_table",
			Columns:      []ast.Column{{Name: "id", Type: "uuid"}},
		}},
	})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil {
		t.Fatal("expected error for non-matching previousName, got nil")
	}
}

func TestSnapshotDiffRenamesBeforeDependentAlters(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "old_users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:         "users",
			PreviousName: "old_users",
			Columns: []ast.Column{
				{Name: "id", Type: "uuid"},
				{Name: "email", Type: "text"},
			},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 2 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	got := strings.Join(planned.Statements, "\n")
	renameAt := strings.Index(got, "RENAME TO users")
	addAt := strings.Index(got, "ADD COLUMN email")
	if renameAt < 0 || addAt < 0 {
		t.Fatalf("expected rename and add column in output:\n%s", got)
	}
	if renameAt > addAt {
		t.Fatalf("rename must happen before add column:\n%s", got)
	}
}
