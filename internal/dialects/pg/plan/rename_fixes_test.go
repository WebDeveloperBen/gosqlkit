package plan_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/plan"
)

// #1 Schema rename with contained objects must fail closed rather than emit a
// broken drop-and-recreate cascade.
func TestSnapshotDiffRejectsSchemaRenameWithContents(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect:    "postgresql",
		Version:    pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{Name: "old_billing"}},
		Tables: []ast.Table{{
			Schema:  "old_billing",
			Name:    "accounts",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect:    "postgresql",
		Version:    pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{Name: "billing", PreviousName: "old_billing"}},
		Tables: []ast.Table{{
			Schema:  "billing",
			Name:    "accounts",
			Columns: []ast.Column{{Name: "id", Type: "uuid"}},
		}},
	})

	_, err := plan.SnapshotDiff(previous, current)
	if err == nil || !strings.Contains(err.Error(), "is not supported while it contained objects") {
		t.Fatalf("expected schema-rename-with-contents rejection, got %v", err)
	}
}

// An empty-schema rename must still succeed (regression guard for #1's fix).
func TestSnapshotDiffAllowsEmptySchemaRename(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect:    "postgresql",
		Version:    pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{Name: "old_billing"}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect:    "postgresql",
		Version:    pgschema.SnapshotVersion,
		Namespaces: []pgschema.Namespace{{Name: "billing", PreviousName: "old_billing"}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Op != "rename" {
		t.Fatalf("changes = %#v", planned.Changes)
	}
}

// #2 A pure sequence rename must not re-emit ownership, and its down migration
// must not strip pre-existing ownership.
func TestSnapshotDiffRenamedSequenceKeepsOwnership(t *testing.T) {
	owned := "orders.id"
	previous := snapshot(t, pgschema.Document{
		Dialect:   "postgresql",
		Version:   pgschema.SnapshotVersion,
		Tables:    []ast.Table{{Name: "orders", Columns: []ast.Column{{Name: "id", Type: "bigint"}}}},
		Sequences: []pgschema.Sequence{{Name: "old_seq", OwnedBy: owned}},
	})
	current := snapshot(t, pgschema.Document{
		Dialect:   "postgresql",
		Version:   pgschema.SnapshotVersion,
		Tables:    []ast.Table{{Name: "orders", Columns: []ast.Column{{Name: "id", Type: "bigint"}}}},
		Sequences: []pgschema.Sequence{{Name: "new_seq", PreviousName: "old_seq", OwnedBy: owned}},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Changes) != 1 {
		t.Fatalf("expected only the rename change, got %#v", planned.Changes)
	}
	for _, stmt := range planned.Statements {
		if strings.Contains(stmt, "OWNED BY") {
			t.Fatalf("unexpected ownership statement in plan: %q", stmt)
		}
	}
	for _, change := range planned.Changes {
		for _, rev := range change.ReverseStatements {
			if strings.Contains(rev.SQL, "OWNED BY NONE") {
				t.Fatalf("reverse strips pre-existing ownership: %q", rev.SQL)
			}
		}
	}
}

// #3 Dropping inter-dependent views must drop the dependent before the object
// it depends on.
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

// #4 Reversible drops (index, constraints, policy) must carry reverse SQL so a
// single such drop does not empty the entire down migration.
func TestSnapshotDiffReversibleDropsCarryReverse(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{
			{
				Name:    "accounts",
				Columns: []ast.Column{{Name: "id", Type: "uuid"}, {Name: "email", Type: "text"}},
				Indexes: []ast.Index{{Name: "accounts_email_idx", Columns: []ast.IndexColumn{{Expression: "email"}}}},
				UniqueConstraints: []ast.UniqueConstraint{
					{Name: "accounts_email_key", Columns: []string{"email"}},
				},
			},
			{Name: "old_users", Columns: []ast.Column{{Name: "id", Type: "uuid"}}},
		},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Tables: []ast.Table{
			{Name: "accounts", Columns: []ast.Column{{Name: "id", Type: "uuid"}, {Name: "email", Type: "text"}}},
			{Name: "users", PreviousName: "old_users", Columns: []ast.Column{{Name: "id", Type: "uuid"}}},
		},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatal(err)
	}

	// Every change must be reversible, otherwise app.reverseStatements discards
	// the whole down migration.
	for _, change := range planned.Changes {
		if len(change.ReverseStatements) == 0 {
			t.Fatalf("change %q (%s) has no reverse statements", change.Summary, change.Op)
		}
	}

	// The dropped index must be recreated on rollback.
	var down []string
	for i := len(planned.Changes) - 1; i >= 0; i-- {
		for _, rev := range planned.Changes[i].ReverseStatements {
			down = append(down, rev.SQL)
		}
	}
	joined := strings.Join(down, "\n")
	if !strings.Contains(joined, "accounts_email_idx") {
		t.Fatalf("down migration missing index recreation:\n%s", joined)
	}
	if !strings.Contains(joined, "ADD") || !strings.Contains(joined, "accounts_email_key") {
		t.Fatalf("down migration missing unique constraint recreation:\n%s", joined)
	}
}

// #5 Renaming a role that another role references via membership must succeed:
// the reference follows the rename and needs no GRANT/REVOKE.
func TestSnapshotDiffRoleRenameUpdatesMembershipReferences(t *testing.T) {
	previous := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Roles: []pgschema.Role{
			{Name: "old_group"},
			{Name: "member", MemberOf: []string{"old_group"}},
		},
	})
	current := snapshot(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Roles: []pgschema.Role{
			{Name: "app_group", PreviousName: "old_group"},
			{Name: "member", MemberOf: []string{"app_group"}},
		},
	})

	planned, err := plan.SnapshotDiff(previous, current)
	if err != nil {
		t.Fatalf("expected role rename with membership reference to succeed, got %v", err)
	}
	if len(planned.Changes) != 1 || planned.Changes[0].Op != "rename" {
		t.Fatalf("expected a single rename change, got %#v", planned.Changes)
	}
	if planned.Changes[0].Statements[0].SQL != "ALTER ROLE old_group RENAME TO app_group;" {
		t.Fatalf("sql = %q", planned.Changes[0].Statements[0].SQL)
	}
}
