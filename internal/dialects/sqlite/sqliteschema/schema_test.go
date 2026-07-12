package sqliteschema_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

func sampleSchema() sqliteschema.Schema {
	return sqliteschema.Schema{
		Tables: []sqliteschema.Table{
			{
				Table: ast.Table{Name: "users"},
				Columns: []sqliteschema.Column{
					{Column: ast.Column{Name: "id", Type: "integer", PrimaryKey: true}, AutoIncrement: true},
					{Column: ast.Column{Name: "email", Type: "text", NotNull: true}},
				},
				Indexes: []sqliteschema.Index{
					{
						Index: ast.Index{Name: "users_email_idx", Unique: true},
						Columns: []sqliteschema.IndexColumn{
							{IndexColumn: ast.IndexColumn{Expression: "email"}, Collation: "NOCASE"},
						},
					},
				},
				Strict:       true,
				WithoutRowID: false,
			},
		},
		Views: []sqliteschema.View{
			{Name: "active_users", Query: "SELECT id FROM users"},
		},
		Triggers: []sqliteschema.Trigger{
			{Name: "users_ai", Target: "users", Timing: "AFTER", Events: []string{"INSERT"}, Body: "SELECT 1;"},
		},
	}
}

func TestJSONDeterministic(t *testing.T) {
	first, err := sqliteschema.JSON("sqlite", sampleSchema())
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	second, err := sqliteschema.JSON("sqlite", sampleSchema())
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("snapshot not deterministic:\n%s\n---\n%s", first, second)
	}
}

func TestJSONShape(t *testing.T) {
	data, err := sqliteschema.JSON("sqlite", sampleSchema())
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := doc["dialect"]; !ok {
		t.Fatalf("missing dialect key")
	}
	if _, ok := doc["version"]; !ok {
		t.Fatalf("missing version key")
	}

	// The shadow-embedded Columns/Indexes fields must not double-emit the
	// embedded ast.Table.Columns/Indexes under the same "columns"/"indexes" key.
	var probe struct {
		Tables []struct {
			Columns []struct {
				Name          string `json:"name"`
				AutoIncrement bool   `json:"autoIncrement"`
			} `json:"columns"`
			Indexes []struct {
				Columns []struct {
					Collation string `json:"collation"`
				} `json:"columns"`
			} `json:"indexes"`
			Strict bool `json:"strict"`
		} `json:"tables"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		t.Fatalf("unmarshal probe: %v", err)
	}
	if len(probe.Tables) != 1 {
		t.Fatalf("want 1 table, got %d", len(probe.Tables))
	}
	table := probe.Tables[0]
	if !table.Strict {
		t.Fatalf("strict flag not serialised")
	}
	if len(table.Columns) != 2 {
		t.Fatalf("want 2 columns, got %d", len(table.Columns))
	}
	if !table.Columns[0].AutoIncrement {
		t.Fatalf("autoIncrement not serialised on id column")
	}
	if len(table.Indexes) != 1 || len(table.Indexes[0].Columns) != 1 {
		t.Fatalf("index columns not serialised: %+v", table.Indexes)
	}
	if table.Indexes[0].Columns[0].Collation != "NOCASE" {
		t.Fatalf("index column collation not serialised")
	}

	// withoutRowid is false + omitempty, so it must be absent.
	if strings.Contains(string(data), "withoutRowid") {
		t.Fatalf("withoutRowid should be omitted when false:\n%s", data)
	}
}
