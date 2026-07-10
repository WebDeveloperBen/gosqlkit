package plan_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
)

func TestSnapshotDiffAddsRawSQLWithReviewRisk(t *testing.T) {
	previous := snapshot(t, pgschema.Document{Dialect: "postgresql", Version: pgschema.SnapshotVersion})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		RawSQL:  []pgschema.RawSQL{{Name: "stats", SQL: "CREATE STATISTICS s ON a, b FROM t;"}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	change := planned.Changes[0]
	if change.Object.Kind != "raw_sql" || change.Object.Key != "stats" || change.Op != "create" {
		t.Fatalf("change metadata = %#v", change)
	}
	if !change.HasRisk("requires-ddl-review") {
		t.Fatalf("expected requires-ddl-review risk, got %#v", change.Risks)
	}
	if change.Reversible {
		t.Fatal("raw SQL create must not claim reversibility")
	}
}

func TestSnapshotDiffRawSQLWithDownIsReversible(t *testing.T) {
	previous := snapshot(t, pgschema.Document{Dialect: "postgresql", Version: pgschema.SnapshotVersion})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		RawSQL: []pgschema.RawSQL{{
			Name: "stats",
			SQL:  "CREATE STATISTICS s ON a, b FROM t;",
			Down: "DROP STATISTICS IF EXISTS s;",
		}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	change := planned.Changes[0]
	if !change.Reversible {
		t.Fatal("raw SQL block with Down should be reversible")
	}
	if len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "DROP STATISTICS IF EXISTS s;" {
		t.Fatalf("unexpected reverse statements: %#v", change.ReverseStatements)
	}
}

func TestSnapshotDiffRawSQLModificationFailsClosed(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		RawSQL:  []pgschema.RawSQL{{Name: "stats", SQL: "CREATE STATISTICS s ON a FROM t;"}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		RawSQL:  []pgschema.RawSQL{{Name: "stats", SQL: "CREATE STATISTICS s ON a, b FROM t;"}},
	})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil || !strings.Contains(err.Error(), "modification requires manual migration authoring") {
		t.Fatalf("expected fail-closed modification error, got %v", err)
	}
}

func TestSnapshotDiffRawSQLRemovalFailsClosed(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		RawSQL:  []pgschema.RawSQL{{Name: "stats", SQL: "CREATE STATISTICS s ON a FROM t;"}},
	})
	current := snapshot(t, pgschema.Document{Dialect: "postgresql", Version: pgschema.SnapshotVersion})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil || !strings.Contains(err.Error(), "removal requires manual migration authoring") {
		t.Fatalf("expected fail-closed removal error, got %v", err)
	}
}
