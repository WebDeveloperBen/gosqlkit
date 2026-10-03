package tooling

import (
	"context"
	"strings"
	"testing"
)

func TestIntrospectPreservesSQLiteCatalogAndOrdersObjects(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = conn.Close()
	}()
	ddl := []string{
		`CREATE TABLE z_parent (id INTEGER PRIMARY KEY AUTOINCREMENT, label TEXT NOT NULL UNIQUE, category TEXT NOT NULL, UNIQUE(label, category));`,
		`CREATE TABLE a_child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES z_parent(id) ON DELETE CASCADE ON UPDATE SET NULL, qty INTEGER DEFAULT (2 + 3) CHECK (qty > 0), doubled INTEGER GENERATED ALWAYS AS (qty * 2) STORED, CONSTRAINT child_check CHECK (qty < 100), UNIQUE(parent_id, qty)) STRICT;`,
		`CREATE TABLE composite_ref (label TEXT, category TEXT, FOREIGN KEY(label, category) REFERENCES z_parent(label, category) ON DELETE SET NULL);`,
		`CREATE TABLE keyed (k TEXT PRIMARY KEY, v TEXT) WITHOUT ROWID;`,
		`CREATE INDEX z_child_idx ON a_child (parent_id COLLATE NOCASE DESC) WHERE qty > 0;`,
		`CREATE VIEW child_view (child_id, total) AS SELECT id, qty FROM a_child;`,
		`CREATE TRIGGER child_touch AFTER UPDATE OF qty ON a_child FOR EACH ROW WHEN NEW.qty > 0 BEGIN UPDATE a_child SET parent_id = NEW.parent_id WHERE id = NEW.id; END;`,
		`CREATE TABLE goose_db_version (id INTEGER PRIMARY KEY);`,
	}
	for _, stmt := range ddl {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatalf("execute %q: %v", stmt, err)
		}
	}
	schema, err := Introspect(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Tables) != 4 || schema.Tables[0].Name != "a_child" || schema.Tables[1].Name != "composite_ref" || schema.Tables[2].Name != "keyed" || schema.Tables[3].Name != "z_parent" {
		t.Fatalf("tables are not sorted or migration state leaked: %#v", schema.Tables)
	}
	child := schema.Tables[0]
	if !child.Strict || len(child.Checks) != 2 || len(child.ForeignKeys) != 0 || len(child.UniqueConstraints) < 1 {
		t.Fatalf("child constraints/options not preserved: %#v", child)
	}
	if child.Columns[1].References == nil || child.Columns[1].References.OnUpdate != "SET NULL" || child.Columns[1].References.OnDelete != "CASCADE" {
		t.Fatalf("column reference actions not preserved: %#v", child.Columns[1].References)
	}
	composite := schema.Tables[1]
	if len(composite.ForeignKeys) != 1 || len(composite.ForeignKeys[0].Columns) != 2 || len(composite.ForeignKeys[0].ReferencedColumns) != 2 || composite.ForeignKeys[0].OnDelete != "SET NULL" {
		t.Fatalf("composite foreign key not modeled: %#v", composite.ForeignKeys)
	}
	if child.Columns[2].Default != "(2 + 3)" || child.Columns[3].Generated == nil || child.Columns[3].Generated.As != "qty * 2" || child.Columns[3].Generated.Type != "stored" {
		t.Fatalf("column details not preserved: %#v", child.Columns)
	}
	if len(child.Indexes) != 1 || child.Indexes[0].Name != "z_child_idx" || child.Indexes[0].Where != "qty > 0" || child.Indexes[0].Columns[0].Order != "DESC" || child.Indexes[0].Columns[0].Collation != "NOCASE" {
		t.Fatalf("index not modeled: %#v", child.Indexes)
	}
	if !schema.Tables[3].Columns[0].AutoIncrement || !schema.Tables[2].WithoutRowID {
		t.Fatalf("SQLite table options not preserved")
	}
	if len(schema.Views) != 1 || schema.Views[0].Name != "child_view" || len(schema.Views[0].ColumnAliases) != 2 || !strings.Contains(schema.Views[0].Query, "SELECT id, qty") {
		t.Fatalf("view not modeled: %#v", schema.Views)
	}
	if len(schema.Triggers) != 1 || schema.Triggers[0].Timing != "AFTER" || schema.Triggers[0].Events[0] != "UPDATE" || schema.Triggers[0].UpdateOfColumns[0] != "qty" || schema.Triggers[0].When != "NEW.qty > 0" {
		t.Fatalf("trigger not modeled: %#v", schema.Triggers)
	}
}

func TestParseTriggerCatalogSQLWithoutTrailingSemicolon(t *testing.T) {
	parsed, err := parseTrigger(
		"users_email_audit",
		"users",
		"CREATE TRIGGER users_email_audit\n    AFTER INSERT ON users\n    FOR EACH ROW\nBEGIN\n    INSERT INTO user_email_events (user_id, email) VALUES (NEW.id, NEW.email);\nEND",
	)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Name != "users_email_audit" || parsed.Target != "users" || parsed.Events[0] != "INSERT" {
		t.Fatalf("parsed trigger = %#v", parsed)
	}
}

func TestIntrospectRejectsUnmodeledIndexExpression(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = conn.Close()
	}()
	for _, stmt := range []string{`CREATE TABLE records (id INTEGER, value TEXT);`, `CREATE INDEX records_expr ON records (lower(value));`} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	_, err = Introspect(ctx, conn)
	if err == nil || !strings.Contains(err.Error(), `index "records_expr"`) || !strings.Contains(err.Error(), "expression") {
		t.Fatalf("expected actionable expression-index error, got %v", err)
	}
}
