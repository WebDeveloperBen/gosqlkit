package pgschema_test

import (
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
)

func TestNormaliseDocumentForDiffRemovesTransportMetadata(t *testing.T) {
	document := pgschema.Document{
		Dialect:            "postgresql",
		SnapshotID:         "current",
		PreviousSnapshotID: "previous",
		TableMetadata:      map[string]pgschema.TableMetadata{"public.users": {}},
		ColumnMetadata:     map[string]pgschema.ColumnMetadata{"public.users.id": {}},
		Tables: []pgschema.Table{
			{Table: ast.Table{Name: "users", PreviousName: "accounts", Columns: []ast.Column{{Name: "id", PreviousName: "account_id", Type: "uuid"}}}},
			{Table: ast.Table{Name: "accounts"}},
		},
	}

	normalised, err := pgschema.NormaliseDocumentForDiff(document)
	if err != nil {
		t.Fatal(err)
	}
	if normalised.SnapshotID != "" || normalised.PreviousSnapshotID != "" {
		t.Fatalf("snapshot identifiers were retained: %#v", normalised)
	}
	if normalised.TableMetadata != nil || normalised.ColumnMetadata != nil {
		t.Fatalf("metadata was retained: %#v", normalised)
	}
	if got, want := normalised.Tables[0].Name, "accounts"; got != want {
		t.Fatalf("first table = %q, want %q", got, want)
	}
	users := normalised.Tables[1]
	if users.PreviousName != "accounts" || users.Columns[0].PreviousName != "account_id" {
		t.Fatalf("rename metadata was removed: %#v", users)
	}
}
