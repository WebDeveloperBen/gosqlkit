package pgsnapshot_test

import (
	"encoding/json"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/pgsnapshot"
	"github.com/webdeveloperben/gosqlkit/internal/snapshot"
)

func TestJSONIncludesPostgresObjects(t *testing.T) {
	data, err := pgsnapshot.JSON(pgschema.Schema{
		Namespaces: []pgschema.Namespace{{Name: "z"}, {Name: "a"}},
		Extensions: []pgschema.Extension{
			{Name: "pgcrypto"},
		},
		Enums: []pgschema.Enum{
			{Name: "status", Values: []string{"draft", "paid"}},
		},
		Tables: []ast.Table{
			{Name: "users", Columns: []ast.Column{{Name: "id", Type: "uuid"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Dialect    string `json:"dialect"`
		Namespaces []struct {
			Name string `json:"name"`
		} `json:"namespaces"`
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Version != snapshot.Version {
		t.Fatalf("version = %d, want %d", doc.Version, snapshot.Version)
	}
	if doc.Dialect != "postgresql" {
		t.Fatalf("dialect = %q, want postgresql", doc.Dialect)
	}
	if got := []string{doc.Namespaces[0].Name, doc.Namespaces[1].Name}; got[0] != "a" || got[1] != "z" {
		t.Fatalf("namespaces not sorted: %v", got)
	}
}
