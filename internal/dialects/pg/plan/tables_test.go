package plan_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
)

func TestSnapshotDiffAddsTableAndColumn(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name: "users",
			Columns: []ast.Column{{
				Name: "id",
				Type: "uuid",
			}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{
			Name: "billing",
		}},
		Tables: []ast.Table{
			{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "email", Type: "text", NotNull: true},
				},
			},
			{
				Schema: "billing",
				Name:   "invoices",
				Columns: []ast.Column{{
					Name: "id",
					Type: "uuid",
				}},
			},
		},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(planned.Statements, "\n")
	for _, want := range []string{
		"CREATE SCHEMA billing;",
		"ALTER TABLE users ADD COLUMN email text NOT NULL;",
		"CREATE TABLE billing.invoices",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in\n%s", want, got)
		}
	}
}

func TestSnapshotDiffOrdersNewTablesByForeignKeyDependency(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{
			{
				Name: "invoices",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "user_id", Type: "uuid"},
				},
				ForeignKeys: []ast.ForeignKeyConstraint{{
					Name:              "invoices_user_id_fkey",
					Columns:           []string{"user_id"},
					ReferencedTable:   "users",
					ReferencedColumns: []string{"id"},
				}},
			},
			{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(planned.Statements, "\n")
	users := strings.Index(got, "CREATE TABLE users")
	invoices := strings.Index(got, "CREATE TABLE invoices")
	if users < 0 || invoices < 0 || users > invoices {
		t.Fatalf("statements not dependency ordered:\n%s", got)
	}
	if len(planned.Changes) != 2 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	if len(planned.Changes[1].Dependencies) != 1 || planned.Changes[1].Dependencies[0].Kind != "table" || planned.Changes[1].Dependencies[0].Key != "public.users" {
		t.Fatalf("child table dependencies = %#v", planned.Changes[1].Dependencies)
	}
}

func TestSnapshotDiffOrdersNewTablesByCreateChangeWhenParentHasComment(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{
			{
				Name: "invoices",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "user_id", Type: "uuid"},
				},
				ForeignKeys: []ast.ForeignKeyConstraint{{
					Name:              "invoices_user_id_fkey",
					Columns:           []string{"user_id"},
					ReferencedTable:   "users",
					ReferencedColumns: []string{"id"},
				}},
			},
			{
				Name:    "users",
				Comment: "Application users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(planned.Statements, "\n")
	users := strings.Index(got, "CREATE TABLE users")
	comment := strings.Index(got, "COMMENT ON TABLE users")
	invoices := strings.Index(got, "CREATE TABLE invoices")
	if users < 0 || comment < 0 || invoices < 0 || users > comment || users > invoices {
		t.Fatalf("dependent changes not anchored to parent table creation:\n%s", got)
	}
}

func TestSnapshotDiffProducesIndexDependencyAndReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "email", Type: "text"}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "email", Type: "text"}},
			Indexes: []ast.Index{{
				Name:    "users_email_idx",
				Columns: []ast.IndexColumn{{Expression: "email"}},
			}},
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
	if change.Object.Kind != "index" || change.Object.Key != "public.users.users_email_idx" {
		t.Fatalf("change object = %#v", change.Object)
	}
	if len(change.Dependencies) != 1 || change.Dependencies[0].Kind != "table" || change.Dependencies[0].Key != "public.users" {
		t.Fatalf("dependencies = %#v", change.Dependencies)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "DROP INDEX users_email_idx;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffProducesCommentDependencyAndReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name: "users",
			Columns: []ast.Column{
				{Name: "id", Type: "uuid"},
				{Name: "email", Type: "text"},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Comment: "Application users",
			Columns: []ast.Column{
				{Name: "id", Type: "uuid"},
				{Name: "email", Type: "text", Comment: "Login email"},
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

	tableChange := planned.Changes[0]
	if tableChange.Object.Kind != "table" || tableChange.Object.Key != "public.users" {
		t.Fatalf("table comment object = %#v", tableChange.Object)
	}
	if !tableChange.Reversible || len(tableChange.ReverseStatements) != 1 || tableChange.ReverseStatements[0].SQL != "COMMENT ON TABLE users IS NULL;" {
		t.Fatalf("table comment reverse = %#v reversible=%v", tableChange.ReverseStatements, tableChange.Reversible)
	}

	columnChange := planned.Changes[1]
	if columnChange.Object.Kind != "column" || columnChange.Object.Key != "public.users.email" {
		t.Fatalf("column comment object = %#v", columnChange.Object)
	}
	if len(columnChange.Dependencies) != 1 || columnChange.Dependencies[0].Kind != "table" || columnChange.Dependencies[0].Key != "public.users" {
		t.Fatalf("column comment dependencies = %#v", columnChange.Dependencies)
	}
	if !columnChange.Reversible || len(columnChange.ReverseStatements) != 1 || columnChange.ReverseStatements[0].SQL != "COMMENT ON COLUMN users.email IS NULL;" {
		t.Fatalf("column comment reverse = %#v reversible=%v", columnChange.ReverseStatements, columnChange.Reversible)
	}
}

func TestSnapshotDiffProducesForeignKeyDependencyAndReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{
			{
				Name:    "orgs",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
			{
				Name:    "users",
				Columns: []ast.Column{{Name: "org_id", Type: "uuid"}},
			},
		},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{
			{
				Name:    "orgs",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
			{
				Name:    "users",
				Columns: []ast.Column{{Name: "org_id", Type: "uuid"}},
				ForeignKeys: []ast.ForeignKeyConstraint{{
					Name:              "users_org_id_fkey",
					Columns:           []string{"org_id"},
					ReferencedTable:   "orgs",
					ReferencedColumns: []string{"id"},
				}},
			},
		},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	change := planned.Changes[0]
	if change.Object.Kind != "constraint" || change.Object.Key != "public.users.users_org_id_fkey" {
		t.Fatalf("constraint object = %#v", change.Object)
	}
	if len(change.Dependencies) != 2 {
		t.Fatalf("dependencies = %#v", change.Dependencies)
	}
	if change.Dependencies[0].Kind != "table" || change.Dependencies[0].Key != "public.users" {
		t.Fatalf("local table dependency = %#v", change.Dependencies[0])
	}
	if change.Dependencies[1].Kind != "table" || change.Dependencies[1].Key != "public.orgs" {
		t.Fatalf("referenced table dependency = %#v", change.Dependencies[1])
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "ALTER TABLE users DROP CONSTRAINT users_org_id_fkey;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffProducesExclusionConstraintReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "bookings",
			Columns: []ast.Column{{Name: "slot", Type: "tsrange"}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "bookings",
			Columns: []ast.Column{{Name: "slot", Type: "tsrange"}},
			Exclusions: []ast.ExclusionConstraint{{
				Name:     "bookings_slot_excl",
				Elements: []ast.ExclusionElement{{Expression: "slot", Operator: "&&"}},
			}},
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
	if change.Object.Kind != "constraint" || change.Object.Key != "public.bookings.bookings_slot_excl" {
		t.Fatalf("constraint object = %#v", change.Object)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "ALTER TABLE bookings DROP CONSTRAINT bookings_slot_excl;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffProducesRLSReverse(t *testing.T) {
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
			Name:             "users",
			Columns:          []ast.Column{{Name: "id", Type: "uuid"}},
			RowLevelSecurity: true,
			ForceRLS:         true,
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 2 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	if planned.Changes[0].Object.Kind != "table" || planned.Changes[0].Object.Key != "public.users" {
		t.Fatalf("enable RLS object = %#v", planned.Changes[0].Object)
	}
	if !planned.Changes[0].Reversible || planned.Changes[0].ReverseStatements[0].SQL != "ALTER TABLE users DISABLE ROW LEVEL SECURITY;" {
		t.Fatalf("enable RLS reverse = %#v reversible=%v", planned.Changes[0].ReverseStatements, planned.Changes[0].Reversible)
	}
	if planned.Changes[1].Object.Kind != "table" || planned.Changes[1].Object.Key != "public.users" {
		t.Fatalf("force RLS object = %#v", planned.Changes[1].Object)
	}
	if !planned.Changes[1].Reversible || planned.Changes[1].ReverseStatements[0].SQL != "ALTER TABLE users NO FORCE ROW LEVEL SECURITY;" {
		t.Fatalf("force RLS reverse = %#v reversible=%v", planned.Changes[1].ReverseStatements, planned.Changes[1].Reversible)
	}
}

func TestSnapshotDiffProducesAddColumnReverse(t *testing.T) {
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
			Name: "users",
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
	if len(planned.Changes) != 1 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	change := planned.Changes[0]
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "ALTER TABLE users DROP COLUMN email;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
	if len(change.Dependencies) != 1 || change.Dependencies[0].Kind != "table" || change.Dependencies[0].Key != "public.users" {
		t.Fatalf("dependencies = %#v", change.Dependencies)
	}
}

func TestSnapshotDiffRejectsColumnModification(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "email", Type: "text"}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "email", Type: "varchar(320)"}},
		}},
	})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil || !strings.Contains(err.Error(), "column modifications") {
		t.Fatalf("expected column modification error, got %v", err)
	}
}

func TestSnapshotDiffRejectsUnsupportedCreateTableDetails(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		table   ast.Table
	}{
		{
			name: "exclusion constraint rename metadata",
			table: ast.Table{
				Name:    "bookings",
				Columns: []ast.Column{{Name: "slot", Type: "tsrange"}},
				Exclusions: []ast.ExclusionConstraint{{
					Name:         "bookings_slot_excl",
					PreviousName: "old_bookings_slot_excl",
					Elements:     []ast.ExclusionElement{{Expression: "slot", Operator: "&&"}},
				}},
			},
			wantErr: "exclusion constraint public.bookings.bookings_slot_excl rename metadata requires semantic planning",
		},
		{
			name: "index rename metadata",
			table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "email", Type: "text"}},
				Indexes: []ast.Index{{
					Name:         "users_email_idx",
					PreviousName: "old_users_email_idx",
					Columns:      []ast.IndexColumn{{Expression: "email"}},
				}},
			},
			wantErr: "index public.users.users_email_idx rename metadata requires semantic planning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := snapshot(t, pgschema.Document{
				Dialect: "postgresql",
				Version: pgschema.SnapshotVersion,
			})
			current := snapshot(t, pgschema.Document{
				Dialect: "postgresql",
				Version: pgschema.SnapshotVersion,
				Tables:  []ast.Table{tt.table},
			})

			_, err := plan.SnapshotDiff(previous, current)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSnapshotDiffRejectsAddColumnRenameMetadata(t *testing.T) {
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
			Name: "users",
			Columns: []ast.Column{
				{Name: "id", Type: "uuid"},
				{Name: "email", PreviousName: "old_email", Type: "text"},
			},
		}},
	})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil || !strings.Contains(err.Error(), "column public.users.email rename metadata requires semantic planning") {
		t.Fatalf("expected column rename metadata error, got %v", err)
	}
}
