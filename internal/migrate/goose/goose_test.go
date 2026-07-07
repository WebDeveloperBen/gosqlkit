package goose_test

import (
	"strings"
	"testing"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/goose"
)

func TestRendererRendersSingleAnnotatedFile(t *testing.T) {
	createdAt := time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC)

	files, err := (goose.Renderer{}).Render(migrate.Plan{
		Name:           "Add RLS Policies",
		Dialect:        "postgresql",
		FromSnapshotID: "abc",
		ToSnapshotID:   "def",
		CreatedAt:      createdAt,
		Changes: []migrate.Change{{
			Op:     "alter_table_enable_rls",
			Object: "public.users",
		}},
		UpSQL: []string{"ALTER TABLE users ENABLE ROW LEVEL SECURITY;"},
		DownSQL: []string{
			"ALTER TABLE users DISABLE ROW LEVEL SECURITY;",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1", len(files))
	}
	if files[0].Name != "20260706143000_add_rls_policies.sql" {
		t.Fatalf("file name = %q", files[0].Name)
	}
	for _, want := range []string{
		"-- +gosqlkit Meta",
		`--   "version": 1`,
		`--   "dialect": "postgresql"`,
		`--   "fromSnapshotId": "abc"`,
		`--   "toSnapshotId": "def"`,
		`--   "createdAt": "2026-07-06T14:30:00Z"`,
		"-- +goose Up",
		"ALTER TABLE users ENABLE ROW LEVEL SECURITY;",
		"-- +goose Down",
		"ALTER TABLE users DISABLE ROW LEVEL SECURITY;",
	} {
		if !strings.Contains(files[0].Content, want) {
			t.Fatalf("content missing %q\n%s", want, files[0].Content)
		}
	}
}

func TestRendererRejectsEmptySlug(t *testing.T) {
	_, err := (goose.Renderer{}).Render(migrate.Plan{Name: "!!!"})
	if err == nil || !strings.Contains(err.Error(), "migration name must contain") {
		t.Fatalf("expected migration name error, got %v", err)
	}
}
