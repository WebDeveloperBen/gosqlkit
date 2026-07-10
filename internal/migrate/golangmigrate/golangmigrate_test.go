package golangmigrate_test

import (
	"strings"
	"testing"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/golangmigrate"
)

func TestRendererRendersPairedFiles(t *testing.T) {
	createdAt := time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC)

	files, err := (golangmigrate.Renderer{}).Render(migrate.Plan{
		Name:           "Add RLS Policies",
		Dialect:        "postgresql",
		FromSnapshotID: "abc",
		ToSnapshotID:   "def",
		CreatedAt:      createdAt,
		Changes: []migrate.Change{{
			Op:     "alter_table_enable_rls",
			Object: migrate.ObjectRef{Kind: "table", Key: "public.users"},
		}},
		UpStatements:   []migrate.Statement{{SQL: "ALTER TABLE users ENABLE ROW LEVEL SECURITY;"}},
		DownStatements: []migrate.Statement{{SQL: "ALTER TABLE users DISABLE ROW LEVEL SECURITY;"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}
	if files[0].Name != "20260706143000_add_rls_policies.up.sql" {
		t.Fatalf("up file name = %q", files[0].Name)
	}
	if files[1].Name != "20260706143000_add_rls_policies.down.sql" {
		t.Fatalf("down file name = %q", files[1].Name)
	}
	for _, want := range []string{
		"-- +gosqlkit Meta",
		`--   "version": 1`,
		`--   "dialect": "postgresql"`,
		`--   "fromSnapshotId": "abc"`,
		`--   "toSnapshotId": "def"`,
		`--   "createdAt": "2026-07-06T14:30:00Z"`,
		"ALTER TABLE users ENABLE ROW LEVEL SECURITY;",
	} {
		if !strings.Contains(files[0].Content, want) {
			t.Fatalf("up content missing %q\n%s", want, files[0].Content)
		}
	}
	if strings.Contains(files[1].Content, "-- +gosqlkit Meta") {
		t.Fatalf("down content should not contain metadata\n%s", files[1].Content)
	}
	if !strings.Contains(files[1].Content, "ALTER TABLE users DISABLE ROW LEVEL SECURITY;") {
		t.Fatalf("down content missing reverse SQL\n%s", files[1].Content)
	}
}

func TestRendererOmitsDownFileWhenPlanHasNoDown(t *testing.T) {
	files, err := (golangmigrate.Renderer{}).Render(migrate.Plan{
		Name:      "Baseline",
		Dialect:   "postgresql",
		CreatedAt: time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		UpSQL:     []string{"CREATE TABLE users (id uuid);"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1", len(files))
	}
	if files[0].Name != "20260706143000_baseline.up.sql" {
		t.Fatalf("file name = %q", files[0].Name)
	}
}

func TestRendererPrefersStructuredStatements(t *testing.T) {
	files, err := (golangmigrate.Renderer{}).Render(migrate.Plan{
		Name:           "Add Column",
		Dialect:        "postgresql",
		CreatedAt:      time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		UpStatements:   []migrate.Statement{{SQL: "ALTER TABLE users ADD COLUMN email text;"}},
		DownStatements: []migrate.Statement{{SQL: "ALTER TABLE users DROP COLUMN email;"}},
		UpSQL:          []string{"SELECT 'legacy up';"},
		DownSQL:        []string{"SELECT 'legacy down';"},
	})
	if err != nil {
		t.Fatal(err)
	}
	content := files[0].Content + "\n" + files[1].Content
	for _, want := range []string{
		"ALTER TABLE users ADD COLUMN email text;",
		"ALTER TABLE users DROP COLUMN email;",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("content missing %q\n%s", want, content)
		}
	}
	for _, unexpected := range []string{
		"SELECT 'legacy up';",
		"SELECT 'legacy down';",
	} {
		if strings.Contains(content, unexpected) {
			t.Fatalf("content contains fallback statement %q\n%s", unexpected, content)
		}
	}
}

func TestRendererKeepsIrreversibleDownPlaceholders(t *testing.T) {
	files, err := (golangmigrate.Renderer{}).Render(migrate.Plan{
		Name:         "Mixed Down",
		Dialect:      "postgresql",
		CreatedAt:    time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		UpStatements: []migrate.Statement{{SQL: "ALTER TABLE users ALTER COLUMN email TYPE citext;"}},
		DownStatements: []migrate.Statement{
			{SQL: "-- no automatic down for: column public.users.email"},
			{SQL: "DROP TABLE users;"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}
	content := files[1].Content
	for _, want := range []string{
		"-- no automatic down for: column public.users.email",
		"DROP TABLE users;",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("down content missing %q\n%s", want, content)
		}
	}
}

func TestRendererSplitsMultiStatementFilesAndPreservesConcurrentIndex(t *testing.T) {
	files, err := (golangmigrate.Renderer{}).Render(migrate.Plan{
		Name:      "Baseline",
		Dialect:   "postgresql",
		CreatedAt: time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		UpStatements: []migrate.Statement{{
			SQL: "CREATE FUNCTION touch_users() RETURNS trigger LANGUAGE plpgsql AS $fn$\nBEGIN\nRETURN NEW;\nEND\n$fn$;\nCREATE INDEX CONCURRENTLY users_email_idx ON users (email);",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}
	if files[0].Name != "202607061430000001_baseline.up.sql" {
		t.Fatalf("first file name = %q", files[0].Name)
	}
	if files[1].Name != "202607061430000002_baseline.up.sql" {
		t.Fatalf("second file name = %q", files[1].Name)
	}
	if !strings.Contains(files[0].Content, "RETURN NEW;") {
		t.Fatalf("first file missing function body\n%s", files[0].Content)
	}
	if strings.Contains(files[0].Content, "CREATE INDEX") {
		t.Fatalf("first file contains index statement\n%s", files[0].Content)
	}
	if !strings.Contains(files[1].Content, "CREATE INDEX CONCURRENTLY users_email_idx ON users (email);") {
		t.Fatalf("second file missing concurrent index\n%s", files[1].Content)
	}
	if strings.Contains(files[1].Content, "CREATE INDEX users_email_idx") {
		t.Fatalf("second file stripped CONCURRENTLY\n%s", files[1].Content)
	}
}

func TestRendererPairsSplitDownStatementsWithReverseVersions(t *testing.T) {
	files, err := (golangmigrate.Renderer{}).Render(migrate.Plan{
		Name:      "Add Table And Index",
		Dialect:   "postgresql",
		CreatedAt: time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		UpStatements: []migrate.Statement{
			{SQL: "CREATE TABLE users (id uuid PRIMARY KEY);"},
			{SQL: "CREATE INDEX CONCURRENTLY users_id_idx ON users (id);"},
		},
		DownStatements: []migrate.Statement{
			{SQL: "DROP INDEX users_id_idx;"},
			{SQL: "DROP TABLE users;"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Fatalf("file count = %d, want 4", len(files))
	}
	if files[2].Name != "202607061430000001_add_table_and_index.down.sql" {
		t.Fatalf("first down file name = %q", files[2].Name)
	}
	if !strings.Contains(files[2].Content, "DROP TABLE users;") {
		t.Fatalf("first down file should reverse first up file\n%s", files[2].Content)
	}
	if files[3].Name != "202607061430000002_add_table_and_index.down.sql" {
		t.Fatalf("second down file name = %q", files[3].Name)
	}
	if !strings.Contains(files[3].Content, "DROP INDEX users_id_idx;") {
		t.Fatalf("second down file should reverse second up file\n%s", files[3].Content)
	}
}

func TestRendererRejectsEmptySlug(t *testing.T) {
	_, err := (golangmigrate.Renderer{}).Render(migrate.Plan{Name: "!!!"})
	if err == nil || !strings.Contains(err.Error(), "migration name must contain") {
		t.Fatalf("expected migration name error, got %v", err)
	}
}

func TestUpSQLStripsMetadataBlock(t *testing.T) {
	got := golangmigrate.UpSQL(`-- +gosqlkit Meta
-- {
--   "version": 1
-- }

CREATE TABLE users (id uuid);
`)
	want := "CREATE TABLE users (id uuid);\n"
	if got != want {
		t.Fatalf("UpSQL() mismatch\nwant:\n%q\n\ngot:\n%q", want, got)
	}
}
