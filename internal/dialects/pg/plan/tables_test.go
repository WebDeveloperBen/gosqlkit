package plan_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func TestSnapshotDiffAddsTableAndColumn(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{{
					Name: "id",
					Type: "uuid",
				}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{
			Name: "billing",
		}},
		Tables: []pgschema.Table{
			{
				Table: ast.Table{
					Name: "users",
					Columns: []ast.Column{
						{Name: "id", Type: "uuid"},
						{Name: "email", Type: "text", NotNull: true},
					},
				},
			},
			{
				Table: ast.Table{
					Schema: "billing",
					Name:   "invoices",
					Columns: []ast.Column{{
						Name: "id",
						Type: "uuid",
					}},
				},
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
		Tables: []pgschema.Table{
			{
				Table: ast.Table{
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
			},
			{
				Table: ast.Table{
					Name:    "users",
					Columns: []ast.Column{{Name: "id", Type: "uuid"}},
				},
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
		Tables: []pgschema.Table{
			{
				Table: ast.Table{
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
			},
			{
				Table: ast.Table{
					Name:    "users",
					Comment: "Application users",
					Columns: []ast.Column{{Name: "id", Type: "uuid"}},
				},
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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "email", Type: "text"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "email", Type: "text"}},
			},
			Indexes: []pgschema.Index{{
				Index: ast.Index{Name: "users_email_idx"},
				Columns: []pgschema.IndexColumn{{
					IndexColumn: ast.IndexColumn{Expression: "email"},
				}},
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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "email", Type: "text"},
				},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Comment: "Application users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "email", Type: "text", Comment: "Login email"},
				},
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
		Tables: []pgschema.Table{
			{
				Table: ast.Table{
					Name:    "orgs",
					Columns: []ast.Column{{Name: "id", Type: "uuid"}},
				},
			},
			{
				Table: ast.Table{
					Name:    "users",
					Columns: []ast.Column{{Name: "org_id", Type: "uuid"}},
				},
			},
		},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{
			{
				Table: ast.Table{
					Name:    "orgs",
					Columns: []ast.Column{{Name: "id", Type: "uuid"}},
				},
			},
			{
				Table: ast.Table{
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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "bookings",
				Columns: []ast.Column{{Name: "slot", Type: "tsrange"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "bookings",
				Columns: []ast.Column{{Name: "slot", Type: "tsrange"}},
			},
			Exclusions: []pgschema.ExclusionConstraint{{
				ExclusionConstraint: ast.ExclusionConstraint{Name: "bookings_slot_excl"},
				Elements: []pgschema.ExclusionElement{{
					ExclusionElement: ast.ExclusionElement{Expression: "slot", Operator: "&&"},
				}},
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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:             "users",
				Columns:          []ast.Column{{Name: "id", Type: "uuid"}},
				RowLevelSecurity: true,
				ForceRLS:         true,
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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "email", Type: "text"},
				},
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

func TestSnapshotDiffPlansColumnTypeChange(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "email", Type: "text"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "email", Type: "varchar(320)"}},
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
	if change.Op != "alter" || change.Object.Kind != "column" || change.Object.Key != "public.users.email" {
		t.Fatalf("change = %#v", change)
	}
	if len(change.Statements) != 1 || change.Statements[0].SQL != "ALTER TABLE users ALTER COLUMN email TYPE varchar(320);" {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if !change.RisksContainDestructive() || !change.RisksContainDataLoss() {
		t.Fatalf("expected destructive data-loss risks, got %#v", change.Risks)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "ALTER TABLE users ALTER COLUMN email TYPE text;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffPlansColumnDefaultChange(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "status", Type: "text", Default: "'active'"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "status", Type: "text"}},
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
	if change.Statements[0].SQL != "ALTER TABLE users ALTER COLUMN status DROP DEFAULT;" {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if !change.Reversible || change.ReverseStatements[0].SQL != "ALTER TABLE users ALTER COLUMN status SET DEFAULT 'active';" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffPlansColumnNullabilityChange(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "email", Type: "text"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "email", Type: "text", NotNull: true}},
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
	if change.Statements[0].SQL != "ALTER TABLE users ALTER COLUMN email SET NOT NULL;" {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if !change.HasRisk(migrateplan.RiskRequiresBackfill) || !change.HasRisk(migrateplan.RiskLockHeavy) {
		t.Fatalf("expected backfill and lock risks, got %#v", change.Risks)
	}
	if !change.Reversible || change.ReverseStatements[0].SQL != "ALTER TABLE users ALTER COLUMN email DROP NOT NULL;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffPlansGeneratedColumnChange(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{
					{Name: "email", Type: "text"},
					{Name: "search", Type: "tsvector", Generated: &ast.Generated{As: "to_tsvector('english', email)", Type: "stored"}},
				},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{
					{Name: "email", Type: "text"},
					{Name: "search", Type: "tsvector", Generated: &ast.Generated{As: "to_tsvector('simple', email)", Type: "stored"}},
				},
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
	if change.Statements[0].SQL != "ALTER TABLE users ALTER COLUMN search SET EXPRESSION AS (to_tsvector('simple', email));" {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if !change.HasRisk(migrateplan.RiskManualReview) || !change.HasRisk(migrateplan.RiskRequiresBackfill) {
		t.Fatalf("expected manual-review and backfill risks, got %#v", change.Risks)
	}
	if !change.Reversible || change.ReverseStatements[0].SQL != "ALTER TABLE users ALTER COLUMN search SET EXPRESSION AS (to_tsvector('english', email));" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffPlansIdentityColumnChange(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "bigint", NotNull: true}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{{
					Name:     "id",
					Type:     "bigint",
					NotNull:  true,
					Identity: &ast.Identity{Type: "byDefault"},
				}},
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
	if change.Statements[0].SQL != "ALTER TABLE users ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY;" {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if !change.HasRisk(migrateplan.RiskManualReview) || !change.HasRisk(migrateplan.RiskRequiresDDLReview) {
		t.Fatalf("expected manual-review and DDL-review risks, got %#v", change.Risks)
	}
	if !change.Reversible || change.ReverseStatements[0].SQL != "ALTER TABLE users ALTER COLUMN id DROP IDENTITY IF EXISTS;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffPlansIdentityColumnModification(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{{
					Name:     "id",
					Type:     "bigint",
					NotNull:  true,
					Identity: &ast.Identity{Type: "always"},
				}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{{
					Name:     "id",
					Type:     "bigint",
					NotNull:  true,
					Identity: &ast.Identity{Type: "byDefault", Increment: 10},
				}},
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
	if len(change.Statements) != 2 {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if change.Statements[0].SQL != "ALTER TABLE users ALTER COLUMN id DROP IDENTITY IF EXISTS;" {
		t.Fatalf("drop identity SQL = %q", change.Statements[0].SQL)
	}
	if change.Statements[1].SQL != "ALTER TABLE users ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (INCREMENT 10);" {
		t.Fatalf("add identity SQL = %q", change.Statements[1].SQL)
	}
	if len(change.ReverseStatements) != 2 || change.ReverseStatements[1].SQL != "ALTER TABLE users ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY;" {
		t.Fatalf("reverse = %#v", change.ReverseStatements)
	}
}

func TestSnapshotDiffRejectsUnsupportedColumnModification(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{
			{Table: ast.Table{Name: "orgs", Columns: []ast.Column{{Name: "id", Type: "uuid"}}}},
			{Table: ast.Table{Name: "users", Columns: []ast.Column{{Name: "org_id", Type: "uuid"}}}},
		},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{
			{Table: ast.Table{Name: "orgs", Columns: []ast.Column{{Name: "id", Type: "uuid"}}}},
			{
				Table: ast.Table{
					Name: "users",
					Columns: []ast.Column{{
						Name:       "org_id",
						Type:       "uuid",
						References: &ast.ForeignKey{Table: "orgs", Column: "id"},
					}},
				},
			},
		},
	})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil || !strings.Contains(err.Error(), "inline constraint or reference changes") {
		t.Fatalf("expected unsupported inline constraint/reference error, got %v", err)
	}
}

func TestSnapshotDiffRejectsUnsupportedCreateTableDetails(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
		table   pgschema.Table
	}{
		{
			name: "exclusion constraint rename metadata",
			table: pgschema.Table{
				Table: ast.Table{
					Name:    "bookings",
					Columns: []ast.Column{{Name: "slot", Type: "tsrange"}},
				},
				Exclusions: []pgschema.ExclusionConstraint{{
					ExclusionConstraint: ast.ExclusionConstraint{
						Name:         "bookings_slot_excl",
						PreviousName: "old_bookings_slot_excl",
					},
					Elements: []pgschema.ExclusionElement{{
						ExclusionElement: ast.ExclusionElement{Expression: "slot", Operator: "&&"},
					}},
				}},
			},
			wantErr: "exclusion constraint public.bookings.bookings_slot_excl rename metadata requires semantic planning",
		},
		{
			name: "index rename metadata on new table",
			table: pgschema.Table{
				Table: ast.Table{
					Name:    "users",
					Columns: []ast.Column{{Name: "email", Type: "text"}},
				},
				Indexes: []pgschema.Index{{
					Index: ast.Index{
						Name:         "users_email_idx",
						PreviousName: "old_users_email_idx",
					},
					Columns: []pgschema.IndexColumn{{
						IndexColumn: ast.IndexColumn{Expression: "email"},
					}},
				}},
			},
			wantErr: "index public.users.users_email_idx previousName old_users_email_idx does not match any index in the previous snapshot",
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
				Tables:  []pgschema.Table{tt.table},
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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "email", PreviousName: "old_email", Type: "text"},
				},
			},
		}},
	})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil || !strings.Contains(err.Error(), "column public.users.email previousName old_email does not match any column in the previous snapshot") {
		t.Fatalf("expected column previousName mismatch error, got %v", err)
	}
}

func TestSnapshotDiffEmitsDestructiveDropTable(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	change := planned.Changes[0]
	if change.Op != "drop" || change.Object.Kind != "table" || change.Object.Key != "public.users" {
		t.Fatalf("drop change = %#v", change)
	}
	if !change.RisksContainDestructive() {
		t.Fatalf("expected destructive risk, got %#v", change.Risks)
	}
	if len(change.Statements) != 1 || change.Statements[0].SQL != "DROP TABLE users;" {
		t.Fatalf("drop SQL = %#v", change.Statements)
	}
}

func TestSnapshotDiffEmitsDestructiveDropColumn(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "email", Type: "text"},
				},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if !planned.HasDestructive() {
		t.Fatalf("expected destructive changes, got %#v", planned.Changes)
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Object.Key != "public.users.email" {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	if !planned.Changes[0].RisksContainDestructive() {
		t.Fatalf("expected destructive risk, got %#v", planned.Changes[0].Risks)
	}
}

func TestSnapshotDiffEmitsDestructiveDisableRLS(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:             "users",
				Columns:          []ast.Column{{Name: "id", Type: "uuid"}},
				RowLevelSecurity: true,
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if !planned.HasDestructive() {
		t.Fatalf("expected destructive changes, got %#v", planned.Changes)
	}
	if len(planned.Statements) != 1 || planned.Statements[0] != "ALTER TABLE users DISABLE ROW LEVEL SECURITY;" {
		t.Fatalf("statements = %#v", planned.Statements)
	}
}

func TestSnapshotDiffEmitsDestructiveDropComment(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Comment: "Application users",
				Columns: []ast.Column{{Name: "id", Type: "uuid", Comment: "Primary key"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if !planned.HasDestructive() {
		t.Fatalf("expected destructive changes, got %#v", planned.Changes)
	}
	if len(planned.Changes) != 2 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	for _, change := range planned.Changes {
		if !change.RisksContainDestructive() {
			t.Fatalf("expected destructive risk on %s, got %#v", change.Object.Key, change.Risks)
		}
	}
}

func TestSnapshotDiffRejectsNewTableForeignKeyCycle(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{
			{
				Table: ast.Table{
					Name: "accounts",
					Columns: []ast.Column{
						{Name: "id", Type: "uuid"},
						{Name: "user_id", Type: "uuid"},
					},
					ForeignKeys: []ast.ForeignKeyConstraint{{
						Name:              "accounts_user_id_fkey",
						Columns:           []string{"user_id"},
						ReferencedTable:   "users",
						ReferencedColumns: []string{"id"},
					}},
				},
			},
			{
				Table: ast.Table{
					Name: "users",
					Columns: []ast.Column{
						{Name: "id", Type: "uuid"},
						{Name: "account_id", Type: "uuid"},
					},
					ForeignKeys: []ast.ForeignKeyConstraint{{
						Name:              "users_account_id_fkey",
						Columns:           []string{"account_id"},
						ReferencedTable:   "accounts",
						ReferencedColumns: []string{"id"},
					}},
				},
			},
		},
	})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil || !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("expected dependency cycle error, got %v", err)
	}
}

func TestSnapshotDiffEmitsRenameTable(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "old_users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:         "users",
				PreviousName: "old_users",
				Columns:      []ast.Column{{Name: "id", Type: "uuid"}},
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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "old_email", Type: "text"},
				},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "email", PreviousName: "old_email", Type: "text"},
				},
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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "email", Type: "text"}},
				UniqueConstraints: []ast.UniqueConstraint{{
					Name:    "old_users_email_key",
					Columns: []string{"email"},
				}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "email", Type: "text"}},
				UniqueConstraints: []ast.UniqueConstraint{{
					Name:         "users_email_key",
					PreviousName: "old_users_email_key",
					Columns:      []string{"email"},
				}},
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
	if planned.Changes[0].Statements[0].SQL != "ALTER TABLE users RENAME CONSTRAINT old_users_email_key TO users_email_key;" {
		t.Fatalf("sql = %q", planned.Changes[0].Statements[0].SQL)
	}
}

func TestSnapshotDiffEmitsRenameIndex(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "email", Type: "text"}},
			},
			Indexes: []pgschema.Index{{
				Index: ast.Index{Name: "old_users_email_idx"},
				Columns: []pgschema.IndexColumn{{
					IndexColumn: ast.IndexColumn{Expression: "email"},
				}},
			}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "email", Type: "text"}},
			},
			Indexes: []pgschema.Index{{
				Index: ast.Index{
					Name:         "users_email_idx",
					PreviousName: "old_users_email_idx",
				},
				Columns: []pgschema.IndexColumn{{
					IndexColumn: ast.IndexColumn{Expression: "email"},
				}},
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

func TestSnapshotDiffRejectsRenameColumnWithTypeChange(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "old_email", Type: "text"},
				},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "email", PreviousName: "old_email", Type: "varchar(320)"},
				},
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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:         "people",
				PreviousName: "missing_table",
				Columns:      []ast.Column{{Name: "id", Type: "uuid"}},
			},
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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "old_users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:         "users",
				PreviousName: "old_users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "email", Type: "text"},
				},
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

func TestSnapshotDiffReversibleDropsCarryReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{
			{
				Table: ast.Table{
					Name:    "accounts",
					Columns: []ast.Column{{Name: "id", Type: "uuid"}, {Name: "email", Type: "text"}},
					UniqueConstraints: []ast.UniqueConstraint{
						{Name: "accounts_email_key", Columns: []string{"email"}},
					},
				},
				Indexes: []pgschema.Index{{Index: ast.Index{Name: "accounts_email_idx"}, Columns: []pgschema.IndexColumn{{IndexColumn: ast.IndexColumn{Expression: "email"}}}}},
			},
			{Table: ast.Table{Name: "old_users", Columns: []ast.Column{{Name: "id", Type: "uuid"}}}},
		},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{
			{Table: ast.Table{Name: "accounts", Columns: []ast.Column{{Name: "id", Type: "uuid"}, {Name: "email", Type: "text"}}}},
			{Table: ast.Table{Name: "users", PreviousName: "old_users", Columns: []ast.Column{{Name: "id", Type: "uuid"}}}},
		},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range planned.Changes {
		if len(change.ReverseStatements) == 0 {
			t.Fatalf("change %q (%s) has no reverse statements", change.Summary, change.Op)
		}
	}

	var down []string
	for i := len(planned.Changes) - 1; i >= 0; i-- {
		for _, rev := range planned.Changes[i].ReverseStatements {
			down = append(down, rev.SQL)
		}
	}
	joined := strings.Join(down, "\n")
	if !strings.Contains(joined, "accounts_email_idx") {
		t.Fatalf("down migration missing index recreation:\n%s", joined)
	}
	if !strings.Contains(joined, "ADD") || !strings.Contains(joined, "accounts_email_key") {
		t.Fatalf("down migration missing unique constraint recreation:\n%s", joined)
	}
}
