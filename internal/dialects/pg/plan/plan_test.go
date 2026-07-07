package plan_test

import (
	"encoding/json"
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
			name: "table comment",
			table: ast.Table{
				Name:    "users",
				Comment: "Application users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
			wantErr: "comments require semantic planning",
		},
		{
			name: "column comment",
			table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "uuid", Comment: "Primary key"}},
			},
			wantErr: "column public.users.id comments require semantic planning",
		},
		{
			name: "row level security",
			table: ast.Table{
				Name:             "users",
				Columns:          []ast.Column{{Name: "id", Type: "uuid"}},
				RowLevelSecurity: true,
			},
			wantErr: "RLS requires semantic planning",
		},
		{
			name: "exclusion constraint",
			table: ast.Table{
				Name:    "bookings",
				Columns: []ast.Column{{Name: "slot", Type: "tsrange"}},
				Exclusions: []ast.ExclusionConstraint{{
					Name:     "bookings_slot_excl",
					Elements: []ast.ExclusionElement{{Expression: "slot", Operator: "&&"}},
				}},
			},
			wantErr: "exclusion constraints require semantic planning",
		},
		{
			name: "index",
			table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "email", Type: "text"}},
				Indexes: []ast.Index{{
					Name:    "users_email_idx",
					Columns: []ast.IndexColumn{{Expression: "email"}},
				}},
			},
			wantErr: "indexes require semantic planning",
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

func TestSnapshotDiffRejectsUnsupportedAddColumnDetails(t *testing.T) {
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
				{Name: "email", Type: "text", Comment: "Login email"},
			},
		}},
	})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil || !strings.Contains(err.Error(), "column public.users.email comments require semantic planning") {
		t.Fatalf("expected column comment error, got %v", err)
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
