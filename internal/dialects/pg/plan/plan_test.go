package plan_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
)

type diffFixture struct {
	Name           string            `json:"name"`
	WantErr        string            `json:"wantErr,omitempty"`
	WantStatements []string          `json:"wantStatements,omitempty"`
	Previous       pgschema.Document `json:"previous"`
	Current        pgschema.Document `json:"current"`
}

func TestSnapshotDiffFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/diff/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}

	var fixtures []diffFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			previous := snapshot(t, fixture.Previous)
			current := snapshot(t, fixture.Current)

			planned, err := plan.SnapshotDiff(previous, current)
			if fixture.WantErr != "" {
				if err == nil || !strings.Contains(err.Error(), fixture.WantErr) {
					t.Fatalf("expected error containing %q, got %v", fixture.WantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got, want := strings.Join(planned.Statements, "\n"), strings.Join(fixture.WantStatements, "\n"); got != want {
				t.Fatalf("statements mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
			}
		})
	}
}

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

func TestSnapshotDiffAddsEnumValue(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Enums: []pgschema.Enum{{
			Name:   "status",
			Values: []string{"draft"},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Enums: []pgschema.Enum{{
			Name:   "status",
			Values: []string{"draft", "issued"},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Statements) != 1 || planned.Statements[0] != "ALTER TYPE status ADD VALUE 'issued';" {
		t.Fatalf("statements = %#v", planned.Statements)
	}
}

func TestSnapshotDiffProducesStructuredChangeMetadata(t *testing.T) {
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
	if change.Object.Kind != "column" || change.Object.Key != "public.users.email" || change.Op != "alter" {
		t.Fatalf("change metadata = %#v", change)
	}
	if len(change.Statements) != 1 || change.Statements[0].SQL != "ALTER TABLE users ADD COLUMN email text;" {
		t.Fatalf("change statements = %#v", change.Statements)
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

func TestSnapshotDiffProducesRoleReverse(t *testing.T) {
	login := true
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Roles: []pgschema.Role{{
			Name:  "app_reader",
			Login: &login,
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
	if change.Object.Kind != "role" || change.Object.Key != "app_reader" {
		t.Fatalf("role object = %#v", change.Object)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "DROP ROLE app_reader;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffProducesSequenceOwnershipDependencyAndReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "invoices",
			Columns: []ast.Column{{Name: "number", Type: "bigint"}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Sequences: []pgschema.Sequence{{
			Name:    "invoice_number_seq",
			OwnedBy: "invoices.number",
		}},
		Tables: []ast.Table{{
			Name:    "invoices",
			Columns: []ast.Column{{Name: "number", Type: "bigint"}},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 2 {
		t.Fatalf("changes = %#v", planned.Changes)
	}

	create := planned.Changes[0]
	if create.Object.Kind != "sequence" || create.Object.Key != "public.invoice_number_seq" {
		t.Fatalf("sequence object = %#v", create.Object)
	}
	if !create.Reversible || len(create.ReverseStatements) != 1 || create.ReverseStatements[0].SQL != "DROP SEQUENCE invoice_number_seq;" {
		t.Fatalf("sequence reverse = %#v reversible=%v", create.ReverseStatements, create.Reversible)
	}

	ownership := planned.Changes[1]
	if ownership.Object.Kind != "sequence" || ownership.Object.Key != "public.invoice_number_seq" {
		t.Fatalf("ownership object = %#v", ownership.Object)
	}
	if len(ownership.Dependencies) != 2 {
		t.Fatalf("ownership dependencies = %#v", ownership.Dependencies)
	}
	if ownership.Dependencies[0].Kind != "sequence" || ownership.Dependencies[0].Key != "public.invoice_number_seq" {
		t.Fatalf("sequence dependency = %#v", ownership.Dependencies[0])
	}
	if ownership.Dependencies[1].Kind != "table" || ownership.Dependencies[1].Key != "public.invoices" {
		t.Fatalf("table dependency = %#v", ownership.Dependencies[1])
	}
	if !ownership.Reversible || len(ownership.ReverseStatements) != 1 || ownership.ReverseStatements[0].SQL != "ALTER SEQUENCE invoice_number_seq OWNED BY NONE;" {
		t.Fatalf("ownership reverse = %#v reversible=%v", ownership.ReverseStatements, ownership.Reversible)
	}
}

func TestSnapshotDiffProducesCompositeTypeReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		CompositeTypes: []pgschema.CompositeType{{
			Schema: "billing",
			Name:   "money_amount",
			Attributes: []pgschema.CompositeAttribute{
				{Name: "amount", Type: "numeric"},
				{Name: "currency", Type: "text"},
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
	if change.Object.Kind != "composite_type" || change.Object.Key != "billing.money_amount" {
		t.Fatalf("composite type object = %#v", change.Object)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "DROP TYPE billing.money_amount;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffProducesDomainReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Domains: []pgschema.Domain{{
			Schema:   "billing",
			Name:     "email_address",
			BaseType: "text",
			Check:    "position('@' in VALUE) > 1",
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
	if change.Object.Kind != "domain" || change.Object.Key != "billing.email_address" {
		t.Fatalf("domain object = %#v", change.Object)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "DROP DOMAIN billing.email_address;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffProducesFunctionReverseAndComment(t *testing.T) {
	strict := true
	cost := 5.0
	rows := int64(10)
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Functions: []pgschema.Function{{
			Schema:          "billing",
			Name:            "invoice_total",
			Language:        "sql",
			ReturnType:      "integer",
			Body:            "SELECT 1",
			Volatility:      "STABLE",
			Parallel:        "SAFE",
			Strict:          &strict,
			SecurityDefiner: true,
			Cost:            &cost,
			Rows:            &rows,
			Configuration: map[string]string{
				"search_path": "billing, public",
			},
			Comment: "Calculates invoice totals",
			Arguments: []pgschema.FunctionArgument{
				{Name: "invoice_id", Type: "uuid"},
				{Name: "scale", Type: "integer", Default: "1"},
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
	create := planned.Changes[0]
	if create.Object.Kind != "function" || create.Object.Key != "billing.invoice_total(uuid, integer)" {
		t.Fatalf("function object = %#v", create.Object)
	}
	if !strings.Contains(create.Statements[0].SQL, "CREATE FUNCTION billing.invoice_total(invoice_id uuid, scale integer DEFAULT 1)") {
		t.Fatalf("create statement = %q", create.Statements[0].SQL)
	}
	if !create.Reversible || len(create.ReverseStatements) != 1 || create.ReverseStatements[0].SQL != "DROP FUNCTION billing.invoice_total(uuid, integer);" {
		t.Fatalf("reverse = %#v reversible=%v", create.ReverseStatements, create.Reversible)
	}

	comment := planned.Changes[1]
	if comment.Object.Kind != "function" || comment.Object.Key != "billing.invoice_total(uuid, integer)" {
		t.Fatalf("comment object = %#v", comment.Object)
	}
	if len(comment.Dependencies) != 1 || comment.Dependencies[0].Kind != "function" || comment.Dependencies[0].Key != "billing.invoice_total(uuid, integer)" {
		t.Fatalf("comment dependencies = %#v", comment.Dependencies)
	}
	if !comment.Reversible || len(comment.ReverseStatements) != 1 || comment.ReverseStatements[0].SQL != "COMMENT ON FUNCTION billing.invoice_total(uuid, integer) IS NULL;" {
		t.Fatalf("comment reverse = %#v reversible=%v", comment.ReverseStatements, comment.Reversible)
	}
}

func TestSnapshotDiffProducesTriggerDependenciesAndReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Functions: []pgschema.Function{{
			Name:       "touch_updated_at",
			Language:   "plpgsql",
			ReturnType: "trigger",
			Body:       "BEGIN RETURN NEW; END",
		}},
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Functions: []pgschema.Function{{
			Name:       "touch_updated_at",
			Language:   "plpgsql",
			ReturnType: "trigger",
			Body:       "BEGIN RETURN NEW; END",
		}},
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
		Triggers: []pgschema.Trigger{{
			Name:      "users_touch_updated_at",
			Target:    "users",
			Function:  "touch_updated_at",
			Timing:    "BEFORE",
			Events:    []string{"UPDATE"},
			Level:     "ROW",
			Comment:   "Maintains updated_at",
			Arguments: []string{"updated_at"},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 2 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	create := planned.Changes[0]
	if create.Object.Kind != "trigger" || create.Object.Key != "public.users.users_touch_updated_at" {
		t.Fatalf("trigger object = %#v", create.Object)
	}
	if len(create.Dependencies) != 2 {
		t.Fatalf("dependencies = %#v", create.Dependencies)
	}
	if create.Dependencies[0].Kind != "table" || create.Dependencies[0].Key != "public.users" {
		t.Fatalf("target dependency = %#v", create.Dependencies[0])
	}
	if create.Dependencies[1].Kind != "function" || create.Dependencies[1].Key != "public.touch_updated_at()" {
		t.Fatalf("function dependency = %#v", create.Dependencies[1])
	}
	if !create.Reversible || len(create.ReverseStatements) != 1 || create.ReverseStatements[0].SQL != "DROP TRIGGER users_touch_updated_at ON users;" {
		t.Fatalf("reverse = %#v reversible=%v", create.ReverseStatements, create.Reversible)
	}

	comment := planned.Changes[1]
	if comment.Object.Kind != "trigger" || comment.Object.Key != "public.users.users_touch_updated_at" {
		t.Fatalf("comment object = %#v", comment.Object)
	}
	if !comment.Reversible || len(comment.ReverseStatements) != 1 || comment.ReverseStatements[0].SQL != "COMMENT ON TRIGGER users_touch_updated_at ON users IS NULL;" {
		t.Fatalf("comment reverse = %#v reversible=%v", comment.ReverseStatements, comment.Reversible)
	}
}

func TestSnapshotDiffProducesViewDependencyAndReverse(t *testing.T) {
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
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
		Views: []pgschema.View{{
			Name:          "active_users",
			Query:         "SELECT id FROM users",
			Comment:       "Active users",
			ColumnAliases: []string{"id"},
			DependsOn:     []string{"users"},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 2 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	create := planned.Changes[0]
	if create.Object.Kind != "view" || create.Object.Key != "public.active_users" {
		t.Fatalf("view object = %#v", create.Object)
	}
	if len(create.Dependencies) != 1 || create.Dependencies[0].Kind != "table" || create.Dependencies[0].Key != "public.users" {
		t.Fatalf("dependencies = %#v", create.Dependencies)
	}
	if !create.Reversible || len(create.ReverseStatements) != 1 || create.ReverseStatements[0].SQL != "DROP VIEW active_users;" {
		t.Fatalf("reverse = %#v reversible=%v", create.ReverseStatements, create.Reversible)
	}

	comment := planned.Changes[1]
	if comment.Object.Kind != "view" || comment.Object.Key != "public.active_users" {
		t.Fatalf("comment object = %#v", comment.Object)
	}
	if !comment.Reversible || len(comment.ReverseStatements) != 1 || comment.ReverseStatements[0].SQL != "COMMENT ON VIEW active_users IS NULL;" {
		t.Fatalf("comment reverse = %#v reversible=%v", comment.ReverseStatements, comment.Reversible)
	}
}

func TestSnapshotDiffProducesMaterializedViewReverse(t *testing.T) {
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
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
		MaterializedViews: []pgschema.MaterializedView{{
			Name:      "user_counts",
			Query:     "SELECT count(*) AS total FROM users",
			DependsOn: []string{"users"},
			NoData:    true,
			With: map[string]string{
				"fillfactor": "80",
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
	if change.Object.Kind != "materialized_view" || change.Object.Key != "public.user_counts" {
		t.Fatalf("materialized view object = %#v", change.Object)
	}
	if len(change.Dependencies) != 1 || change.Dependencies[0].Kind != "table" || change.Dependencies[0].Key != "public.users" {
		t.Fatalf("dependencies = %#v", change.Dependencies)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "DROP MATERIALIZED VIEW user_counts;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
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

func snapshot(t *testing.T, doc pgschema.Document) []byte {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
