package render_test

import (
	"os"
	"strings"
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
					{Name: "email", Type: "text", NotNull: true},
					{Name: "display_name", Type: "text"},
					{Name: "created_at", Type: "timestamptz", NotNull: true, Default: "now()"},
				},
				UniqueConstraints: []ast.UniqueConstraint{
					{Name: "users_email_unique", Columns: []string{"email"}},
				},
			},
			{
				Name: "invoices",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid", PrimaryKey: true, Default: "gen_random_uuid()"},
					{Name: "user_id", Type: "uuid", NotNull: true},
					{Name: "amount_cents", Type: "integer", NotNull: true},
					{Name: "status", Type: "text", NotNull: true, Default: "'draft'"},
					{Name: "created_at", Type: "timestamptz", NotNull: true, Default: "now()"},
				},
				ForeignKeys: []ast.ForeignKeyConstraint{
					{
						Name:              "invoices_user_id_fkey",
						Columns:           []string{"user_id"},
						ReferencedTable:   "users",
						ReferencedColumns: []string{"id"},
						OnDelete:          "cascade",
					},
				},
				Checks: []ast.Check{
					{Name: "invoices_amount_cents_positive", Expression: "amount_cents > 0"},
				},
				Indexes: []ast.Index{
					{
						Name: "invoices_user_id_created_at_idx",
						Columns: []ast.IndexColumn{
							{Expression: "user_id"},
							{Expression: "created_at", Order: "DESC", Nulls: "LAST"},
						},
						Where: "status <> 'void'",
					},
				},
			},
			{
				Name: "invoice_lines",
				Columns: []ast.Column{
					{Name: "invoice_id", Type: "uuid", NotNull: true},
					{Name: "line_no", Type: "integer", NotNull: true},
					{Name: "description", Type: "text", NotNull: true},
					{Name: "amount_cents", Type: "integer", NotNull: true},
				},
				PrimaryKeys: []ast.PrimaryKey{
					{Name: "invoice_lines_pkey", Columns: []string{"invoice_id", "line_no"}},
				},
				ForeignKeys: []ast.ForeignKeyConstraint{
					{
						Name:              "invoice_lines_invoice_id_fkey",
						Columns:           []string{"invoice_id"},
						ReferencedTable:   "invoices",
						ReferencedColumns: []string{"id"},
						OnDelete:          "cascade",
					},
				},
				Checks: []ast.Check{
					{Name: "invoice_lines_amount_cents_positive", Expression: "amount_cents > 0"},
				},
				Indexes: []ast.Index{
					{
						Name:         "invoice_lines_description_idx",
						Method:       "btree",
						Concurrently: true,
						Columns: []ast.IndexColumn{
							{Expression: "description", OpClass: "text_ops"},
						},
						With: map[string]string{"fillfactor": "90"},
					},
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

func TestPostgresRejectsIndexWithUnknownColumn(t *testing.T) {
	_, err := render.Postgres(ast.Schema{
		Tables: []ast.Table{
			{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
				},
				Indexes: []ast.Index{
					{Name: "users_email_idx", Columns: []ast.IndexColumn{{Expression: "email"}}},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `references unknown column "email"`) {
		t.Fatalf("expected unknown index column error, got %v", err)
	}
}

func TestPostgresRejectsConstraintWithUnknownColumn(t *testing.T) {
	_, err := render.Postgres(ast.Schema{
		Tables: []ast.Table{
			{
				Name: "invoice_lines",
				Columns: []ast.Column{
					{Name: "invoice_id", Type: "uuid"},
				},
				PrimaryKeys: []ast.PrimaryKey{
					{Name: "invoice_lines_pkey", Columns: []string{"invoice_id", "line_no"}},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `references unknown column "line_no"`) {
		t.Fatalf("expected unknown column error, got %v", err)
	}
}

func TestPostgresRejectsDuplicateColumns(t *testing.T) {
	_, err := render.Postgres(ast.Schema{
		Tables: []ast.Table{
			{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "id", Type: "text"},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `duplicate column "id"`) {
		t.Fatalf("expected duplicate column error, got %v", err)
	}
}

func TestPostgresRejectsUnsupportedForeignKeyAction(t *testing.T) {
	_, err := render.Postgres(ast.Schema{
		Tables: []ast.Table{
			{
				Name: "invoices",
				Columns: []ast.Column{
					{
						Name: "user_id",
						Type: "uuid",
						References: &ast.ForeignKey{
							Table:    "users",
							Column:   "id",
							OnDelete: "explode",
						},
					},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `ON DELETE action "explode" is not supported`) {
		t.Fatalf("expected unsupported FK action error, got %v", err)
	}
}
