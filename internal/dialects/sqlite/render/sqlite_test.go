package render_test

import (
	"os"
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/render"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

func col(name, typ string) sqliteschema.Column {
	return sqliteschema.Column{Column: ast.Column{Name: name, Type: typ}}
}

func goldenSchema() sqliteschema.Schema {
	users := sqliteschema.Table{
		Table:  ast.Table{Name: "users", Comment: "Application users."},
		Strict: true,
		Columns: []sqliteschema.Column{
			{Column: ast.Column{Name: "id", Type: "integer", PrimaryKey: true}, AutoIncrement: true},
			{Column: ast.Column{Name: "email", Type: "text", NotNull: true, Unique: true, Collation: "NOCASE"}},
			{Column: ast.Column{Name: "display_name", Type: "text"}},
			{Column: ast.Column{
				Name:      "search_name",
				Type:      "text",
				Generated: &ast.Generated{As: "lower(email)", Type: "virtual"},
			}},
			{Column: ast.Column{Name: "created_at", Type: "text", NotNull: true, Default: "CURRENT_TIMESTAMP"}},
		},
	}

	tags := sqliteschema.Table{
		Table:        ast.Table{Name: "tags"},
		WithoutRowID: true,
		Columns: []sqliteschema.Column{
			{Column: ast.Column{Name: "id", Type: "integer", PrimaryKey: true}},
			{Column: ast.Column{Name: "name", Type: "text", NotNull: true}},
		},
	}

	posts := sqliteschema.Table{
		Table: ast.Table{
			Name: "posts",
			Checks: []ast.Check{
				{Name: "posts_status_check", Expression: "status IN ('draft', 'published', 'deleted')"},
			},
		},
		Columns: []sqliteschema.Column{
			{Column: ast.Column{Name: "id", Type: "integer", PrimaryKey: true}, AutoIncrement: true},
			{Column: ast.Column{Name: "author_id", Type: "integer", NotNull: true, References: &ast.ForeignKey{Table: "users", Column: "id", OnDelete: "cascade"}}},
			{Column: ast.Column{Name: "title", Type: "text", NotNull: true}},
			{Column: ast.Column{Name: "status", Type: "text", NotNull: true, Default: "'draft'"}},
			{Column: ast.Column{Name: "created_at", Type: "text", NotNull: true, Default: "CURRENT_TIMESTAMP"}},
		},
		Indexes: []sqliteschema.Index{
			{
				Index: ast.Index{Name: "posts_author_created_idx", Where: "status <> 'deleted'"},
				Columns: []sqliteschema.IndexColumn{
					{IndexColumn: ast.IndexColumn{Expression: "author_id"}},
					{IndexColumn: ast.IndexColumn{Expression: "created_at", Order: "DESC"}},
				},
			},
			{
				Index: ast.Index{Name: "posts_title_unique", Unique: true},
				Columns: []sqliteschema.IndexColumn{
					{IndexColumn: ast.IndexColumn{Expression: "title"}, Collation: "NOCASE"},
				},
			},
		},
	}

	postTags := sqliteschema.Table{
		Table: ast.Table{
			Name: "post_tags",
			PrimaryKeys: []ast.PrimaryKey{
				{Name: "post_tags_pk", Columns: []string{"post_id", "tag_id"}},
			},
			ForeignKeys: []ast.ForeignKeyConstraint{
				{Name: "post_tags_post_fk", Columns: []string{"post_id"}, ReferencedTable: "posts", ReferencedColumns: []string{"id"}, OnDelete: "cascade"},
				{Name: "post_tags_tag_fk", Columns: []string{"tag_id"}, ReferencedTable: "tags", ReferencedColumns: []string{"id"}, OnDelete: "cascade", Deferrable: true, Initially: "deferred"},
			},
		},
		Columns: []sqliteschema.Column{
			{Column: ast.Column{Name: "post_id", Type: "integer", NotNull: true}},
			{Column: ast.Column{Name: "tag_id", Type: "integer", NotNull: true}},
		},
	}

	return sqliteschema.Schema{
		Tables: []sqliteschema.Table{posts, users, postTags, tags},
		Views: []sqliteschema.View{
			{
				Name:    "active_posts",
				Comment: "Posts that are published.",
				Query:   "SELECT id, title FROM posts WHERE status = 'published'",
			},
		},
		Triggers: []sqliteschema.Trigger{
			{
				Name:            "posts_touch_created",
				Target:          "posts",
				Timing:          "AFTER",
				Events:          []string{"UPDATE"},
				UpdateOfColumns: []string{"title", "status"},
				ForEachRow:      true,
				When:            "old.status <> new.status",
				Body:            "UPDATE posts SET created_at = CURRENT_TIMESTAMP WHERE id = new.id;",
			},
		},
		RawSQL: []sqliteschema.RawSQL{
			{Name: "enable_fk", SQL: "PRAGMA foreign_keys = ON;", Before: true},
			{Name: "analyze", SQL: "ANALYZE;"},
		},
	}
}

func TestSQLiteRender(t *testing.T) {
	got, err := render.SQLite(goldenSchema())
	if err != nil {
		t.Fatal(err)
	}

	const goldenPath = "testdata/sqlite.golden.sql"
	if os.Getenv("GOSQLKIT_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, []byte(got), 0o600); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("rendered SQL mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSQLiteRejectsStrictType(t *testing.T) {
	_, err := render.SQLite(sqliteschema.Schema{
		Tables: []sqliteschema.Table{
			{
				Table:   ast.Table{Name: "t"},
				Strict:  true,
				Columns: []sqliteschema.Column{col("a", "varchar(10)")},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "STRICT") {
		t.Fatalf("want STRICT type error, got %v", err)
	}
}

func TestSQLiteRejectsAutoIncrementWithoutIntegerPK(t *testing.T) {
	_, err := render.SQLite(sqliteschema.Schema{
		Tables: []sqliteschema.Table{
			{
				Table:   ast.Table{Name: "t"},
				Columns: []sqliteschema.Column{{Column: ast.Column{Name: "a", Type: "text"}, AutoIncrement: true}},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "AUTOINCREMENT") {
		t.Fatalf("want AUTOINCREMENT error, got %v", err)
	}
}

func TestSQLiteRejectsGeneratedWithDefault(t *testing.T) {
	_, err := render.SQLite(sqliteschema.Schema{
		Tables: []sqliteschema.Table{
			{
				Table: ast.Table{Name: "t"},
				Columns: []sqliteschema.Column{
					col("a", "integer"),
					{Column: ast.Column{Name: "b", Type: "integer", Default: "1", Generated: &ast.Generated{As: "a + 1", Type: "stored"}}},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "generated column cannot have a default") {
		t.Fatalf("want generated+default error, got %v", err)
	}
}

func TestSQLiteRejectsForeignKeyToUnknownTable(t *testing.T) {
	_, err := render.SQLite(sqliteschema.Schema{
		Tables: []sqliteschema.Table{
			{
				Table: ast.Table{Name: "t"},
				Columns: []sqliteschema.Column{
					{Column: ast.Column{Name: "a", Type: "integer", References: &ast.ForeignKey{Table: "missing", Column: "id"}}},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown table") {
		t.Fatalf("want unknown table error, got %v", err)
	}
}

func TestSQLiteRejectsWithoutRowIDNoPrimaryKey(t *testing.T) {
	_, err := render.SQLite(sqliteschema.Schema{
		Tables: []sqliteschema.Table{
			{
				Table:        ast.Table{Name: "t"},
				WithoutRowID: true,
				Columns:      []sqliteschema.Column{col("a", "integer")},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "WITHOUT ROWID") {
		t.Fatalf("want WITHOUT ROWID error, got %v", err)
	}
}

func TestSQLiteRejectsTriggerUnknownTarget(t *testing.T) {
	_, err := render.SQLite(sqliteschema.Schema{
		Triggers: []sqliteschema.Trigger{
			{Name: "trg", Target: "missing", Timing: "AFTER", Events: []string{"INSERT"}, Body: "SELECT 1;"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown table or view") {
		t.Fatalf("want unknown trigger target error, got %v", err)
	}
}
