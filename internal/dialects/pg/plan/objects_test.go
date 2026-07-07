package plan_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
)

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
	if len(planned.Changes) != 1 || len(planned.Changes[0].Risks) != 1 || planned.Changes[0].Risks[0] != "manual-review" {
		t.Fatalf("change risks = %#v", planned.Changes)
	}
	if planned.Changes[0].Reversible || len(planned.Changes[0].ReverseStatements) != 0 {
		t.Fatalf("enum value append should not be automatically reversible: %#v", planned.Changes[0])
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

func TestSnapshotDiffOrdersRolesByMembershipDependency(t *testing.T) {
	login := true
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Roles: []pgschema.Role{
			{
				Name:     "app_writer",
				Login:    &login,
				MemberOf: []string{"app_base"},
			},
			{
				Name: "app_base",
			},
		},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(planned.Statements, "\n")
	base := strings.Index(got, "CREATE ROLE app_base")
	writer := strings.Index(got, "CREATE ROLE app_writer")
	if base < 0 || writer < 0 || base > writer {
		t.Fatalf("roles not dependency ordered:\n%s", got)
	}
	if len(planned.Changes) != 2 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	if len(planned.Changes[1].Dependencies) != 1 || planned.Changes[1].Dependencies[0].Kind != "role" || planned.Changes[1].Dependencies[0].Key != "app_base" {
		t.Fatalf("role dependencies = %#v", planned.Changes[1].Dependencies)
	}
}

func TestSnapshotDiffProducesSimpleCreateReverses(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{
			Name: "billing",
		}},
		Extensions: []pgschema.Extension{{
			Name: "pgcrypto",
		}},
		Enums: []pgschema.Enum{{
			Schema: "billing",
			Name:   "invoice_status",
			Values: []string{"draft"},
		}},
		Tables: []ast.Table{{
			Schema: "billing",
			Name:   "invoices",
			Columns: []ast.Column{{
				Name: "id",
				Type: "uuid",
			}},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"schema:billing":              "DROP SCHEMA billing;",
		"extension:public.pgcrypto":   "DROP EXTENSION pgcrypto;",
		"enum:billing.invoice_status": "DROP TYPE billing.invoice_status;",
		"table:billing.invoices":      "DROP TABLE billing.invoices;",
	}
	for _, change := range planned.Changes {
		key := string(change.Object.Kind) + ":" + change.Object.Key
		reverse, ok := want[key]
		if !ok {
			continue
		}
		if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != reverse {
			t.Fatalf("%s reverse = %#v reversible=%v", key, change.ReverseStatements, change.Reversible)
		}
		delete(want, key)
	}
	if len(want) > 0 {
		t.Fatalf("missing reverses for %#v in %#v", want, planned.Changes)
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
