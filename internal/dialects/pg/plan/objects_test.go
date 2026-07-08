package plan_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
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

func TestSnapshotDiffDetectsEnumValueRemoval(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Enums: []pgschema.Enum{{
			Name:   "status",
			Values: []string{"draft", "issued", "void"},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Enums: []pgschema.Enum{{
			Name:   "status",
			Values: []string{"draft"},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Statements) != 0 {
		t.Fatalf("enum value removal should not emit automatic SQL, got %#v", planned.Statements)
	}
	if len(planned.Changes) != 1 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	change := planned.Changes[0]
	if change.Op != migrateplan.OperationReplace || change.Object.Kind != migrateplan.ObjectKindEnum || change.Object.Key != "public.status" {
		t.Fatalf("change = %#v", change)
	}
	for _, risk := range []migrateplan.Risk{
		migrateplan.RiskDestructive,
		migrateplan.RiskDataLoss,
		migrateplan.RiskManualReview,
		migrateplan.RiskRequiresDDLReview,
	} {
		if !change.HasRisk(risk) {
			t.Fatalf("expected risk %q in %#v", risk, change.Risks)
		}
	}
	if !strings.Contains(change.Summary, "issued, void") {
		t.Fatalf("summary should include removed values, got %q", change.Summary)
	}
	if change.Reversible || len(change.ReverseStatements) != 0 {
		t.Fatalf("enum value removal should not be automatically reversible: %#v", change)
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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Schema: "billing",
				Name:   "invoices",
				Columns: []ast.Column{{
					Name: "id",
					Type: "uuid",
				}},
			},
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

func TestSnapshotDiffDetectsExtensionRenameAsManualReplacement(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Extensions: []pgschema.Extension{{
			Name: "old_ext",
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Extensions: []pgschema.Extension{{
			Name:         "new_ext",
			PreviousName: "old_ext",
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Statements) != 0 {
		t.Fatalf("extension replacement should not emit automatic SQL, got %#v", planned.Statements)
	}
	if len(planned.Changes) != 1 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	change := planned.Changes[0]
	if change.Op != migrateplan.OperationReplace || change.Object.Kind != migrateplan.ObjectKindExtension || change.Object.Key != "public.new_ext" {
		t.Fatalf("change = %#v", change)
	}
	for _, risk := range []migrateplan.Risk{
		migrateplan.RiskDestructive,
		migrateplan.RiskManualReview,
		migrateplan.RiskRequiresDDLReview,
	} {
		if !change.HasRisk(risk) {
			t.Fatalf("expected risk %q in %#v", risk, change.Risks)
		}
	}
}

func TestSnapshotDiffProducesSequenceOwnershipDependencyAndReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "invoices",
				Columns: []ast.Column{{Name: "number", Type: "bigint"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Sequences: []pgschema.Sequence{{
			Name:    "invoice_number_seq",
			OwnedBy: "invoices.number",
		}},
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "invoices",
				Columns: []ast.Column{{Name: "number", Type: "bigint"}},
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

func TestSnapshotDiffOrdersFunctionsBeforeTables(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Functions: []pgschema.Function{{
			Name:       "new_invoice_number",
			Language:   "sql",
			ReturnType: "bigint",
			Body:       "SELECT 1",
		}},
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "invoices",
				Columns: []ast.Column{{
					Name:    "number",
					Type:    "bigint",
					Default: "new_invoice_number()",
				}},
			},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(planned.Statements, "\n")
	function := strings.Index(got, "CREATE FUNCTION new_invoice_number()")
	table := strings.Index(got, "CREATE TABLE invoices")
	if function < 0 || table < 0 || function > table {
		t.Fatalf("function should be created before table default can reference it:\n%s", got)
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

func TestSnapshotDiffEmitsRenameCompositeType(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		CompositeTypes: []pgschema.CompositeType{{
			Name:       "old_address",
			Attributes: []pgschema.CompositeAttribute{{Name: "street", Type: "text"}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		CompositeTypes: []pgschema.CompositeType{{
			Name:         "address",
			PreviousName: "old_address",
			Attributes:   []pgschema.CompositeAttribute{{Name: "street", Type: "text"}},
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
	if change.Op != migrateplan.OperationRename || change.Object.Kind != migrateplan.ObjectKindCompositeType || change.Object.Key != "public.address" {
		t.Fatalf("change = %#v", change)
	}
	if len(change.Statements) != 1 || change.Statements[0].SQL != "ALTER TYPE old_address RENAME TO address;" {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "ALTER TYPE address RENAME TO old_address;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffEmitsRenameDomain(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Domains: []pgschema.Domain{{
			Name:     "old_positive_int",
			BaseType: "integer",
			Check:    "VALUE > 0",
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Domains: []pgschema.Domain{{
			Name:         "positive_int",
			PreviousName: "old_positive_int",
			BaseType:     "integer",
			Check:        "VALUE > 0",
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
	if change.Op != migrateplan.OperationRename || change.Object.Kind != migrateplan.ObjectKindDomain || change.Object.Key != "public.positive_int" {
		t.Fatalf("change = %#v", change)
	}
	if len(change.Statements) != 1 || change.Statements[0].SQL != "ALTER TYPE old_positive_int RENAME TO positive_int;" {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "ALTER TYPE positive_int RENAME TO old_positive_int;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffEmitsRenameFunction(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Functions: []pgschema.Function{{
			Name:       "old_touch_user",
			Language:   "sql",
			ReturnType: "void",
			Body:       "SELECT 1",
			Arguments:  []pgschema.FunctionArgument{{Name: "user_id", Type: "uuid"}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Functions: []pgschema.Function{{
			Name:         "touch_user",
			PreviousName: "old_touch_user",
			Language:     "sql",
			ReturnType:   "void",
			Body:         "SELECT 1",
			Arguments:    []pgschema.FunctionArgument{{Name: "user_id", Type: "uuid"}},
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
	if change.Op != migrateplan.OperationRename || change.Object.Kind != migrateplan.ObjectKindFunction || change.Object.Key != "public.touch_user(uuid)" {
		t.Fatalf("change = %#v", change)
	}
	if len(change.Statements) != 1 || change.Statements[0].SQL != "ALTER FUNCTION old_touch_user(uuid) RENAME TO touch_user;" {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "ALTER FUNCTION touch_user(uuid) RENAME TO old_touch_user;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffEmitsRenameSequence(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Sequences: []pgschema.Sequence{{
			Name:      "old_invoice_number_seq",
			Increment: 5,
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Sequences: []pgschema.Sequence{{
			Name:         "invoice_number_seq",
			PreviousName: "old_invoice_number_seq",
			Increment:    5,
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
	if change.Op != migrateplan.OperationRename || change.Object.Kind != migrateplan.ObjectKindSequence || change.Object.Key != "public.invoice_number_seq" {
		t.Fatalf("change = %#v", change)
	}
	if len(change.Statements) != 1 || change.Statements[0].SQL != "ALTER SEQUENCE old_invoice_number_seq RENAME TO invoice_number_seq;" {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "ALTER SEQUENCE invoice_number_seq RENAME TO old_invoice_number_seq;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
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

func TestSnapshotDiffAllowsEmptySchemaRename(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect:    "postgresql",
		Version:    pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{Name: "old_billing"}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect:    "postgresql",
		Version:    pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{Name: "billing", PreviousName: "old_billing"}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Op != "rename" {
		t.Fatalf("changes = %#v", planned.Changes)
	}
}

func TestSnapshotDiffRejectsSchemaRenameWithContents(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect:    "postgresql",
		Version:    pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{Name: "old_billing"}},
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Schema:  "old_billing",
				Name:    "accounts",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect:    "postgresql",
		Version:    pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{Name: "billing", PreviousName: "old_billing"}},
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Schema:  "billing",
				Name:    "accounts",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil || !strings.Contains(err.Error(), "is not supported while it contained objects") {
		t.Fatalf("expected schema-rename-with-contents rejection, got %v", err)
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

func TestSnapshotDiffRoleRenameUpdatesMembershipReferences(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Roles: []pgschema.Role{
			{Name: "old_group"},
			{Name: "member", MemberOf: []string{"old_group"}},
		},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Roles: []pgschema.Role{
			{Name: "app_group", PreviousName: "old_group"},
			{Name: "member", MemberOf: []string{"app_group"}},
		},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatalf("expected role rename with membership reference to succeed, got %v", err)
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Op != "rename" {
		t.Fatalf("expected a single rename change, got %#v", planned.Changes)
	}
	if planned.Changes[0].Statements[0].SQL != "ALTER ROLE old_group RENAME TO app_group;" {
		t.Fatalf("sql = %q", planned.Changes[0].Statements[0].SQL)
	}
}

func TestSnapshotDiffRenamedSequenceKeepsOwnership(t *testing.T) {
	owned := "orders.id"
	previous := snapshot(t, pgschema.Document{
		Dialect:   "postgresql",
		Version:   pgschema.SnapshotVersion,
		Tables:    []pgschema.Table{{Table: ast.Table{Name: "orders", Columns: []ast.Column{{Name: "id", Type: "bigint"}}}}},
		Sequences: []pgschema.Sequence{{Name: "old_seq", OwnedBy: owned}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect:   "postgresql",
		Version:   pgschema.SnapshotVersion,
		Tables:    []pgschema.Table{{Table: ast.Table{Name: "orders", Columns: []ast.Column{{Name: "id", Type: "bigint"}}}}},
		Sequences: []pgschema.Sequence{{Name: "new_seq", PreviousName: "old_seq", OwnedBy: owned}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 {
		t.Fatalf("expected only the rename change, got %#v", planned.Changes)
	}
	for _, stmt := range planned.Statements {
		if strings.Contains(stmt, "OWNED BY") {
			t.Fatalf("unexpected ownership statement in plan: %q", stmt)
		}
	}
	for _, change := range planned.Changes {
		for _, rev := range change.ReverseStatements {
			if strings.Contains(rev.SQL, "OWNED BY NONE") {
				t.Fatalf("reverse strips pre-existing ownership: %q", rev.SQL)
			}
		}
	}
}
