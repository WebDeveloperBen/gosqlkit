package snapshot_test

import (
	"encoding/json"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/snapshot"
)

func TestJSONIsDeterministicAndVersioned(t *testing.T) {
	data, err := snapshot.JSON("postgresql", ast.Schema{
		Tables: []ast.Table{
			{Name: "users", Columns: []ast.Column{{Name: "id", Type: "uuid"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Dialect string `json:"dialect"`
		Tables  []struct {
			Name string `json:"name"`
		} `json:"tables"`
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
	if got := doc.Tables[0].Name; got != "users" {
		t.Fatalf("table = %q, want users", got)
	}
}
