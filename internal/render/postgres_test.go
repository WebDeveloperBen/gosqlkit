package render_test

import (
	"os"
	"testing"

	"github.com/webdeveloperben/pgkit/internal/ast"
	"github.com/webdeveloperben/pgkit/internal/render"
)

func TestPostgresRender(t *testing.T) {
	schema := ast.Schema{
		Tables: []ast.Table{
			{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid", PrimaryKey: true, Default: "gen_random_uuid()"},
					{Name: "email", Type: "text", NotNull: true, Unique: true},
					{Name: "display_name", Type: "text"},
					{Name: "created_at", Type: "timestamptz", NotNull: true, Default: "now()"},
				},
			},
			{
				Name: "invoices",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid", PrimaryKey: true, Default: "gen_random_uuid()"},
					{
						Name:    "user_id",
						Type:    "uuid",
						NotNull: true,
						References: &ast.ForeignKey{
							Table:    "users",
							Column:   "id",
							OnDelete: "cascade",
						},
					},
					{Name: "amount_cents", Type: "integer", NotNull: true},
					{Name: "status", Type: "text", NotNull: true, Default: "'draft'"},
					{Name: "created_at", Type: "timestamptz", NotNull: true, Default: "now()"},
				},
				Checks: []ast.Check{
					{Name: "invoices_amount_cents_positive", Expression: "amount_cents > 0"},
				},
				Indexes: []ast.Index{
					{Name: "invoices_user_id_idx", Columns: []string{"user_id"}},
				},
			},
		},
	}

	got, err := render.Postgres(schema)
	if err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile("../../examples/basic/db/schema.generated.sql")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("rendered SQL mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
