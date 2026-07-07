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

func snapshot(t *testing.T, doc pgschema.Document) []byte {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return data
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

func TestSnapshotDiffRejectsNewTableForeignKeyCycle(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{
			{
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
			{
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
	})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil || !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("expected dependency cycle error, got %v", err)
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
		Tables: []ast.Table{{
			Name: "invoices",
			Columns: []ast.Column{{
				Name:    "number",
				Type:    "bigint",
				Default: "new_invoice_number()",
			}},
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
