package plan_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func TestNewChangeBuildsTypedChange(t *testing.T) {
	change := plan.NewChange(
		plan.OperationCreate,
		plan.Ref(plan.ObjectKindIndex, "public.users.users_email_idx"),
		"create users email index",
		plan.SQL("CREATE INDEX users_email_idx ON users (email);"),
	).WithDependencies(
		plan.Ref(plan.ObjectKindTable, "public.users"),
	).WithRisks(
		plan.RiskManualReview,
	).WithReverse(
		plan.SQL("DROP INDEX users_email_idx;"),
	)

	if change.Object.Kind != plan.ObjectKindIndex || change.Object.Key != "public.users.users_email_idx" {
		t.Fatalf("object = %#v", change.Object)
	}
	if len(change.Dependencies) != 1 || change.Dependencies[0].Kind != plan.ObjectKindTable {
		t.Fatalf("dependencies = %#v", change.Dependencies)
	}
	if len(change.Statements) != 1 || change.Statements[0].SQL == "" {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if len(change.ReverseStatements) != 1 || !change.Reversible {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSummaryJSON(t *testing.T) {
	data, err := plan.SummaryJSON(&plan.Plan{
		Changes: []plan.Change{
			plan.NewChange(
				plan.OperationAlter,
				plan.Ref(plan.ObjectKindColumn, "public.users.email"),
				"add column public.users.email",
				plan.SQL("ALTER TABLE users ADD COLUMN email text;"),
			),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		`"object": {`,
		`"kind": "column"`,
		`"key": "public.users.email"`,
		`"statements": [`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q\n%s", want, got)
		}
	}
}

func TestSummaryJSONRequiresPlan(t *testing.T) {
	_, err := plan.SummaryJSON(nil)
	if err == nil || !strings.Contains(err.Error(), "plan is required") {
		t.Fatalf("expected plan required error, got %v", err)
	}
}
