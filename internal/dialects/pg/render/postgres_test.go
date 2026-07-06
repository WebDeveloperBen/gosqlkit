package render_test

import (
	"os"
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/render"
)

func int64Ptr(v int64) *int64 {
	return &v
}

func TestPostgresRender(t *testing.T) {
	schema := pgschema.Schema{
		Namespaces: []pgschema.Namespace{
			{Name: "billing"},
		},
		Extensions: []pgschema.Extension{
			{Name: "pgcrypto"},
		},
		Enums: []pgschema.Enum{
			{Schema: "billing", Name: "invoice_status", Values: []string{"draft", "issued", "paid", "void"}},
		},
		CompositeTypes: []pgschema.CompositeType{
			{
				Schema: "billing",
				Name:   "money",
				Attributes: []pgschema.CompositeAttribute{
					{Name: "amount", Type: "numeric(10, 2)"},
					{Name: "currency", Type: "char(3)"},
				},
			},
		},
		Domains: []pgschema.Domain{
			{
				Name:     "email",
				BaseType: "text",
				NotNull:  true,
				Check:    "value ~ '^[^@]+@[^@]+$'",
			},
		},
		Sequences: []pgschema.Sequence{
			{
				Schema:    "billing",
				Name:      "order_number_seq",
				Increment: 1,
				StartWith: int64Ptr(1000),
				Cache:     int64Ptr(1),
			},
		},
		Tables: []ast.Table{
			{
				Name:    "users",
				Comment: "Application users.",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid", PrimaryKey: true, Default: "gen_random_uuid()"},
					{Name: "email", Type: "text", NotNull: true},
					{Name: "display_name", Type: "text"},
					{Name: "last_login_ip", Type: "inet"},
					{Name: "tags", Type: "text[]"},
					{Name: "created_at", Type: "timestamptz", NotNull: true, Default: "now()"},
				},
				UniqueConstraints: []ast.UniqueConstraint{
					{Name: "users_email_unique", Columns: []string{"email"}},
				},
			},
			{
				Schema: "billing",
				Name:   "invoices",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid", PrimaryKey: true, Default: "gen_random_uuid()"},
					{Name: "user_id", Type: "uuid", NotNull: true},
					{Name: "amount_cents", Type: "integer", NotNull: true},
					{Name: "status", Type: "billing.invoice_status", NotNull: true, Default: "'draft'"},
					{Name: "created_at", Type: "timestamptz", NotNull: true, Default: "now()"},
				},
				ForeignKeys: []ast.ForeignKeyConstraint{
					{
						Name:              "invoices_user_id_fkey",
						Columns:           []string{"user_id"},
						ReferencedTable:   "public.users",
						ReferencedColumns: []string{"id"},
						OnDelete:          "cascade",
						Deferrable:        true,
						Initially:         "DEFERRED",
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
				Schema: "billing",
				Name:   "invoice_lines",
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
						ReferencedTable:   "billing.invoices",
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
			{
				Name: "bookings",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid", PrimaryKey: true, Default: "gen_random_uuid()"},
					{Name: "resource", Type: "text", NotNull: true},
					{Name: "owner", Type: "text", NotNull: true},
					{Name: "during", Type: "tstzrange", NotNull: true},
				},
				Exclusions: []ast.ExclusionConstraint{
					{
						Name:   "bookings_no_overlap",
						Method: "gist",
						Elements: []ast.ExclusionElement{
							{Expression: "during", Operator: "&&"},
						},
					},
				},
			},
			{
				Name: "events",
				Columns: []ast.Column{
					{
						Name: "id",
						Type: "integer",
						Identity: &ast.Identity{
							Name:      "events_id_seq",
							Type:      "always",
							Increment: 1,
							Cache:     int64Ptr(20),
						},
					},
					{Name: "description", Type: "text", NotNull: true},
					{
						Name:      "search_vector",
						Type:      "text",
						Generated: &ast.Generated{As: "to_tsvector('english', description)", Type: "stored"},
					},
					{Name: "metadata", Type: "jsonb", Default: `'{"source":"api"}'::jsonb`},
					{Name: "payload", Type: "bytea"},
					{Name: "duration", Type: "interval day to second (6)", NotNull: true},
					{Name: "priority", Type: "smallint", NotNull: true, Default: "0"},
					{Name: "created_at", Type: "timestamptz", NotNull: true, Default: "now()"},
				},
				Indexes: []ast.Index{
					{
						Name: "events_priority_created_at_idx",
						Columns: []ast.IndexColumn{
							{Expression: "priority"},
							{Expression: "created_at"},
						},
					},
				},
			},
		},
	}

	got, err := render.Postgres(schema)
	if err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile("testdata/postgres.golden.sql")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("rendered SQL mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestPostgresRejectsEnumWithDuplicateValues(t *testing.T) {
	_, err := render.Postgres(pgschema.Schema{
		Enums: []pgschema.Enum{
			{Name: "invoice_status", Values: []string{"draft", "draft"}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `duplicate value "draft"`) {
		t.Fatalf("expected duplicate enum value error, got %v", err)
	}
}

func TestPostgresAllowsHyphenatedExtensionNames(t *testing.T) {
	got, err := render.Postgres(pgschema.Schema{
		Extensions: []pgschema.Extension{
			{Name: "uuid-ossp"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";\n" {
		t.Fatalf("unexpected extension SQL:\n%s", got)
	}
}

func TestPostgresRejectsIndexWithUnknownColumn(t *testing.T) {
	_, err := render.Postgres(pgschema.Schema{
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

func TestPostgresAllowsSameTableNameInDifferentSchemas(t *testing.T) {
	got, err := render.Postgres(pgschema.Schema{
		Namespaces: []pgschema.Namespace{
			{Name: "tenant_a"},
			{Name: "tenant_b"},
		},
		Tables: []ast.Table{
			{
				Schema: "tenant_a",
				Name:   "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
				},
			},
			{
				Schema: "tenant_b",
				Name:   "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "CREATE TABLE tenant_a.users") || !strings.Contains(got, "CREATE TABLE tenant_b.users") {
		t.Fatalf("expected both schema-qualified tables, got:\n%s", got)
	}
}

func TestPostgresRejectsForeignKeyToUnknownTable(t *testing.T) {
	_, err := render.Postgres(pgschema.Schema{
		Tables: []ast.Table{
			{
				Name: "invoices",
				Columns: []ast.Column{
					{Name: "user_id", Type: "uuid"},
				},
				ForeignKeys: []ast.ForeignKeyConstraint{
					{
						Name:              "invoices_user_id_fkey",
						Columns:           []string{"user_id"},
						ReferencedTable:   "users",
						ReferencedColumns: []string{"id"},
					},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `references unknown table "users"`) {
		t.Fatalf("expected unknown referenced table error, got %v", err)
	}
}

func TestPostgresRejectsForeignKeyToUnknownColumn(t *testing.T) {
	_, err := render.Postgres(pgschema.Schema{
		Tables: []ast.Table{
			{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
				},
			},
			{
				Name: "invoices",
				Columns: []ast.Column{
					{Name: "user_id", Type: "uuid"},
				},
				ForeignKeys: []ast.ForeignKeyConstraint{
					{
						Name:              "invoices_user_id_fkey",
						Columns:           []string{"user_id"},
						ReferencedTable:   "users",
						ReferencedColumns: []string{"missing_id"},
					},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `references unknown column "missing_id"`) {
		t.Fatalf("expected unknown referenced column error, got %v", err)
	}
}

func TestPostgresRejectsConstraintWithUnknownColumn(t *testing.T) {
	_, err := render.Postgres(pgschema.Schema{
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
	_, err := render.Postgres(pgschema.Schema{
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
	_, err := render.Postgres(pgschema.Schema{
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

func TestPostgresRejectsIdentityOnNonIntegerType(t *testing.T) {
	_, err := render.Postgres(pgschema.Schema{
		Tables: []ast.Table{
			{
				Name: "events",
				Columns: []ast.Column{
					{
						Name:     "id",
						Type:     "text",
						Identity: &ast.Identity{Type: "always"},
					},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "identity columns require an integer type") {
		t.Fatalf("expected identity type error, got %v", err)
	}
}

func TestPostgresRejectsIdentityWithDefault(t *testing.T) {
	_, err := render.Postgres(pgschema.Schema{
		Tables: []ast.Table{
			{
				Name: "events",
				Columns: []ast.Column{
					{
						Name:     "id",
						Type:     "integer",
						Default:  "1",
						Identity: &ast.Identity{Type: "always"},
					},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "identity columns cannot have an explicit default") {
		t.Fatalf("expected identity default error, got %v", err)
	}
}

func TestPostgresRejectsGeneratedWithDefault(t *testing.T) {
	_, err := render.Postgres(pgschema.Schema{
		Tables: []ast.Table{
			{
				Name: "events",
				Columns: []ast.Column{
					{
						Name:      "total",
						Type:      "integer",
						Default:   "0",
						Generated: &ast.Generated{As: "a + b", Type: "stored"},
					},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "generated columns cannot have a default") {
		t.Fatalf("expected generated default error, got %v", err)
	}
}

func TestPostgresRejectsExclusionWithoutOperator(t *testing.T) {
	_, err := render.Postgres(pgschema.Schema{
		Tables: []ast.Table{
			{
				Name: "bookings",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
				},
				Exclusions: []ast.ExclusionConstraint{
					{
						Name:   "bookings_no_overlap",
						Method: "gist",
						Elements: []ast.ExclusionElement{
							{Expression: "during"},
						},
					},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "must have an operator") {
		t.Fatalf("expected exclusion operator error, got %v", err)
	}
}

func TestPostgresRejectsDuplicateSequence(t *testing.T) {
	_, err := render.Postgres(pgschema.Schema{
		Sequences: []pgschema.Sequence{
			{Name: "order_seq"},
			{Name: "order_seq"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `duplicate sequence "order_seq"`) {
		t.Fatalf("expected duplicate sequence error, got %v", err)
	}
}

func TestPostgresRejectsDuplicateCompositeType(t *testing.T) {
	_, err := render.Postgres(pgschema.Schema{
		CompositeTypes: []pgschema.CompositeType{
			{Name: "money", Attributes: []pgschema.CompositeAttribute{{Name: "amount", Type: "numeric"}}},
			{Name: "money", Attributes: []pgschema.CompositeAttribute{{Name: "amount", Type: "numeric"}}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `duplicate composite type "money"`) {
		t.Fatalf("expected duplicate composite type error, got %v", err)
	}
}

func TestPostgresRejectsDuplicateDomain(t *testing.T) {
	_, err := render.Postgres(pgschema.Schema{
		Domains: []pgschema.Domain{
			{Name: "email", BaseType: "text"},
			{Name: "email", BaseType: "text"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `duplicate domain "email"`) {
		t.Fatalf("expected duplicate domain error, got %v", err)
	}
}
