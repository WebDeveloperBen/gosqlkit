package app

import (
	"reflect"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
)

func TestDriftDifferencesReportsTopLevelAndNestedObjects(t *testing.T) {
	desired := pgschema.Schema{
		Enums: []pgschema.Enum{{
			Name:   "status",
			Values: []string{"active", "disabled"},
		}},
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}, {Name: "email", Type: "text", NotNull: true}},
			},
			Indexes: []pgschema.Index{{Index: ast.Index{Name: "users_email_idx"}, Columns: []pgschema.IndexColumn{{IndexColumn: ast.IndexColumn{Expression: "email"}}}}},
		}},
	}
	database := pgschema.Schema{
		Enums: []pgschema.Enum{{
			Name:   "status",
			Values: []string{"active"},
		}},
		Tables: []pgschema.Table{
			{
				Table: ast.Table{
					Name:    "users",
					Columns: []ast.Column{{Name: "id", Type: "uuid"}, {Name: "legacy_email", Type: "text"}},
				},
			},
			{
				Table: ast.Table{
					Name:    "audit_events",
					Columns: []ast.Column{{Name: "id", Type: "uuid"}},
				},
			},
		},
	}

	got := driftDifferences(desired, database)
	want := []DriftDifference{
		{Op: "changed", Object: DriftObjectRef{Kind: "enum", Key: "public.status"}, Fields: []string{"values"}},
		{Op: "extra", Object: DriftObjectRef{Kind: "table", Key: "public.audit_events"}},
		{Op: "missing", Object: DriftObjectRef{Kind: "column", Key: "public.users.email"}},
		{Op: "extra", Object: DriftObjectRef{Kind: "column", Key: "public.users.legacy_email"}},
		{Op: "missing", Object: DriftObjectRef{Kind: "index", Key: "public.users.users_email_idx"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("driftDifferences() = %#v, want %#v", got, want)
	}
}

func TestDriftDifferencesReportsTableAttributeChangesWithoutDuplicatingNestedChanges(t *testing.T) {
	desired := pgschema.Schema{Tables: []pgschema.Table{{
		Table: ast.Table{
			Name:             "users",
			Comment:          "Application users.",
			Columns:          []ast.Column{{Name: "id", Type: "uuid"}},
			RowLevelSecurity: true,
		},
	}}}
	database := pgschema.Schema{Tables: []pgschema.Table{{
		Table: ast.Table{
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		},
	}}}

	got := driftDifferences(desired, database)
	want := []DriftDifference{{
		Op:     "changed",
		Object: DriftObjectRef{Kind: "table", Key: "public.users"},
		Fields: []string{
			"comment",
			"rowLevelSecurity",
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("driftDifferences() = %#v, want %#v", got, want)
	}
}
