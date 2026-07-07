package plan_test

import (
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
)

func TestSnapshotDiffProducesTriggerDependenciesAndReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Functions: []pgschema.Function{{
			Name:       "touch_updated_at",
			Language:   "plpgsql",
			ReturnType: "trigger",
			Body:       "BEGIN RETURN NEW; END",
		}},
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Functions: []pgschema.Function{{
			Name:       "touch_updated_at",
			Language:   "plpgsql",
			ReturnType: "trigger",
			Body:       "BEGIN RETURN NEW; END",
		}},
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
		Triggers: []pgschema.Trigger{{
			Name:      "users_touch_updated_at",
			Target:    "users",
			Function:  "touch_updated_at",
			Timing:    "BEFORE",
			Events:    []string{"UPDATE"},
			Level:     "ROW",
			Comment:   "Maintains updated_at",
			Arguments: []string{"updated_at"},
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 2 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	create := planned.Changes[0]
	if create.Object.Kind != "trigger" || create.Object.Key != "public.users.users_touch_updated_at" {
		t.Fatalf("trigger object = %#v", create.Object)
	}
	if len(create.Dependencies) != 2 {
		t.Fatalf("dependencies = %#v", create.Dependencies)
	}
	if create.Dependencies[0].Kind != "table" || create.Dependencies[0].Key != "public.users" {
		t.Fatalf("target dependency = %#v", create.Dependencies[0])
	}
	if create.Dependencies[1].Kind != "function" || create.Dependencies[1].Key != "public.touch_updated_at()" {
		t.Fatalf("function dependency = %#v", create.Dependencies[1])
	}
	if !create.Reversible || len(create.ReverseStatements) != 1 || create.ReverseStatements[0].SQL != "DROP TRIGGER users_touch_updated_at ON users;" {
		t.Fatalf("reverse = %#v reversible=%v", create.ReverseStatements, create.Reversible)
	}

	comment := planned.Changes[1]
	if comment.Object.Kind != "trigger" || comment.Object.Key != "public.users.users_touch_updated_at" {
		t.Fatalf("comment object = %#v", comment.Object)
	}
	if !comment.Reversible || len(comment.ReverseStatements) != 1 || comment.ReverseStatements[0].SQL != "COMMENT ON TRIGGER users_touch_updated_at ON users IS NULL;" {
		t.Fatalf("comment reverse = %#v reversible=%v", comment.ReverseStatements, comment.Reversible)
	}
}
