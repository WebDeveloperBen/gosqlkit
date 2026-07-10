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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:    "users",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}},
			},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "users",
				Columns: []ast.Column{
					{Name: "id", Type: "uuid"},
					{Name: "email", Type: "text"},
				},
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

func TestSnapshotDiffIgnoresTransportMetadata(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect:    "postgresql",
		SnapshotID: "previous",
		TableMetadata: map[string]pgschema.TableMetadata{
			"public.users": {},
		},
		Tables: []pgschema.Table{{Table: ast.Table{Name: "users", Columns: []ast.Column{{Name: "id", Type: "uuid"}}}}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect:            "postgresql",
		SnapshotID:         "current",
		PreviousSnapshotID: "previous",
		ColumnMetadata: map[string]pgschema.ColumnMetadata{
			"public.users.id": {},
		},
		Tables: []pgschema.Table{{Table: ast.Table{Name: "users", Columns: []ast.Column{{Name: "id", Type: "uuid"}}}}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 0 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
}
