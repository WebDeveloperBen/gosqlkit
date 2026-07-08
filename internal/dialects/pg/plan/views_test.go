package plan_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func TestSnapshotDiffProducesViewDependencyAndReverse(t *testing.T) {
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
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
		Views: []pgschema.View{{
			Name:          "active_users",
			Query:         "SELECT id FROM users",
			Comment:       "Active users",
			ColumnAliases: []string{"id"},
			DependsOn:     []string{"users"},
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
	if create.Object.Kind != "view" || create.Object.Key != "public.active_users" {
		t.Fatalf("view object = %#v", create.Object)
	}
	if len(create.Dependencies) != 1 || create.Dependencies[0].Kind != "table" || create.Dependencies[0].Key != "public.users" {
		t.Fatalf("dependencies = %#v", create.Dependencies)
	}
	if !create.Reversible || len(create.ReverseStatements) != 1 || create.ReverseStatements[0].SQL != "DROP VIEW active_users;" {
		t.Fatalf("reverse = %#v reversible=%v", create.ReverseStatements, create.Reversible)
	}

	comment := planned.Changes[1]
	if comment.Object.Kind != "view" || comment.Object.Key != "public.active_users" {
		t.Fatalf("comment object = %#v", comment.Object)
	}
	if !comment.Reversible || len(comment.ReverseStatements) != 1 || comment.ReverseStatements[0].SQL != "COMMENT ON VIEW active_users IS NULL;" {
		t.Fatalf("comment reverse = %#v reversible=%v", comment.ReverseStatements, comment.Reversible)
	}
}

func TestSnapshotDiffOrdersViewsByViewDependency(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Views: []pgschema.View{
			{
				Name:      "active_user_emails",
				Query:     "SELECT id FROM active_users",
				DependsOn: []string{"active_users"},
			},
			{
				Name:  "active_users",
				Query: "SELECT id FROM users",
			},
		},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(planned.Statements, "\n")
	base := strings.Index(got, "CREATE VIEW active_users")
	dependent := strings.Index(got, "CREATE VIEW active_user_emails")
	if base < 0 || dependent < 0 || base > dependent {
		t.Fatalf("views not dependency ordered:\n%s", got)
	}
	if len(planned.Changes) != 2 {
		t.Fatalf("changes = %#v", planned.Changes)
	}
	if len(planned.Changes[1].Dependencies) != 1 || planned.Changes[1].Dependencies[0].Kind != "view" || planned.Changes[1].Dependencies[0].Key != "public.active_users" {
		t.Fatalf("view dependencies = %#v", planned.Changes[1].Dependencies)
	}
}

func TestSnapshotDiffEmitsRenameView(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
		Views: []pgschema.View{{
			Name:      "old_active_users",
			Query:     "SELECT id FROM users",
			DependsOn: []string{"users"},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
		Views: []pgschema.View{{
			Name:         "active_users",
			PreviousName: "old_active_users",
			Query:        "SELECT id FROM users",
			DependsOn:    []string{"users"},
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
	if change.Op != migrateplan.OperationRename || change.Object.Kind != migrateplan.ObjectKindView || change.Object.Key != "public.active_users" {
		t.Fatalf("change = %#v", change)
	}
	if len(change.Dependencies) != 1 || change.Dependencies[0].Kind != migrateplan.ObjectKindTable || change.Dependencies[0].Key != "public.users" {
		t.Fatalf("dependencies = %#v", change.Dependencies)
	}
	if len(change.Statements) != 1 || change.Statements[0].SQL != "ALTER VIEW old_active_users RENAME TO active_users;" {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "ALTER VIEW active_users RENAME TO old_active_users;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffProducesMaterializedViewReverse(t *testing.T) {
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
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
		MaterializedViews: []pgschema.MaterializedView{{
			Name:      "user_counts",
			Query:     "SELECT count(*) AS total FROM users",
			DependsOn: []string{"users"},
			NoData:    true,
			With: map[string]string{
				"fillfactor": "80",
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
	if change.Object.Kind != "materialized_view" || change.Object.Key != "public.user_counts" {
		t.Fatalf("materialized view object = %#v", change.Object)
	}
	if len(change.Dependencies) != 1 || change.Dependencies[0].Kind != "table" || change.Dependencies[0].Key != "public.users" {
		t.Fatalf("dependencies = %#v", change.Dependencies)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "DROP MATERIALIZED VIEW user_counts;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffEmitsRenameMaterializedView(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
		MaterializedViews: []pgschema.MaterializedView{{
			Name:      "old_user_counts",
			Query:     "SELECT count(*) AS total FROM users",
			DependsOn: []string{"users"},
			NoData:    true,
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{{
			Name:    "users",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
		MaterializedViews: []pgschema.MaterializedView{{
			Name:         "user_counts",
			PreviousName: "old_user_counts",
			Query:        "SELECT count(*) AS total FROM users",
			DependsOn:    []string{"users"},
			NoData:       true,
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
	if change.Op != migrateplan.OperationRename || change.Object.Kind != migrateplan.ObjectKindMaterializedView || change.Object.Key != "public.user_counts" {
		t.Fatalf("change = %#v", change)
	}
	if len(change.Dependencies) != 1 || change.Dependencies[0].Kind != migrateplan.ObjectKindTable || change.Dependencies[0].Key != "public.users" {
		t.Fatalf("dependencies = %#v", change.Dependencies)
	}
	if len(change.Statements) != 1 || change.Statements[0].SQL != "ALTER MATERIALIZED VIEW old_user_counts RENAME TO user_counts;" {
		t.Fatalf("statements = %#v", change.Statements)
	}
	if !change.Reversible || len(change.ReverseStatements) != 1 || change.ReverseStatements[0].SQL != "ALTER MATERIALIZED VIEW user_counts RENAME TO old_user_counts;" {
		t.Fatalf("reverse = %#v reversible=%v", change.ReverseStatements, change.Reversible)
	}
}

func TestSnapshotDiffDropsDependentViewsInOrder(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Views: []pgschema.View{
			{Name: "a_base", Query: "SELECT 1 AS x"},
			{Name: "z_report", Query: "SELECT x FROM a_base", DependsOn: []string{"a_base"}},
		},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(planned.Statements, "\n")
	dependentAt := strings.Index(got, "DROP VIEW z_report")
	baseAt := strings.Index(got, "DROP VIEW a_base")
	if dependentAt < 0 || baseAt < 0 {
		t.Fatalf("expected both drops:\n%s", got)
	}
	if dependentAt > baseAt {
		t.Fatalf("dependent view must be dropped before its base:\n%s", got)
	}
}
