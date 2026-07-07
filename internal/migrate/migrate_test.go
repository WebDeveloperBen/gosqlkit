package migrate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/goose"
)

func TestParseMetadata(t *testing.T) {
	content := `-- +gosqlkit Meta
-- {
--   "version": 1,
--   "dialect": "postgresql",
--   "fromSnapshotId": "abc",
--   "toSnapshotId": "def",
--   "createdAt": "2026-07-06T14:30:00Z",
--   "changes": []
-- }

-- +goose Up
`

	meta, err := migrate.ParseMetadata(content)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Dialect != "postgresql" || meta.FromSnapshotID != "abc" || meta.ToSnapshotID != "def" {
		t.Fatalf("metadata = %+v", meta)
	}
}

func TestParseMetadataRejectsTargetSnapshotIDMismatch(t *testing.T) {
	content := `-- +gosqlkit Meta
-- {
--   "version": 1,
--   "dialect": "postgresql",
--   "toSnapshotId": "def",
--   "targetSnapshot": {
--     "snapshotId": "other"
--   },
--   "createdAt": "2026-07-06T14:30:00Z",
--   "changes": []
-- }

-- +goose Up
`

	_, err := migrate.ParseMetadata(content)
	if err == nil || !strings.Contains(err.Error(), "does not match toSnapshotId") {
		t.Fatalf("expected target snapshot mismatch error, got %v", err)
	}
}

func TestScanDirValidatesMetadataAndLineage(t *testing.T) {
	dir := t.TempDir()
	writeRenderedMigration(t, dir, migrate.Plan{
		Name:         "create users",
		Dialect:      "postgresql",
		ToSnapshotID: "one",
		CreatedAt:    time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		Changes:      []migrate.Change{},
		UpSQL:        []string{},
		DownSQL:      []string{},
	})
	writeRenderedMigration(t, dir, migrate.Plan{
		Name:           "add posts",
		Dialect:        "postgresql",
		FromSnapshotID: "one",
		ToSnapshotID:   "two",
		CreatedAt:      time.Date(2026, 7, 6, 14, 31, 0, 0, time.UTC),
		Changes:        []migrate.Change{},
		UpSQL:          []string{},
		DownSQL:        []string{},
	})

	migrations, err := migrate.ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 2 {
		t.Fatalf("migration count = %d", len(migrations))
	}
	if migrations[0].Metadata.ToSnapshotID != "one" || migrations[1].Metadata.FromSnapshotID != "one" {
		t.Fatalf("lineage not parsed: %+v", migrations)
	}
}

func TestScanDirRejectsBrokenLineage(t *testing.T) {
	dir := t.TempDir()
	writeRenderedMigration(t, dir, migrate.Plan{
		Name:         "create users",
		Dialect:      "postgresql",
		ToSnapshotID: "one",
		CreatedAt:    time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		Changes:      []migrate.Change{},
	})
	writeRenderedMigration(t, dir, migrate.Plan{
		Name:           "add posts",
		Dialect:        "postgresql",
		FromSnapshotID: "different",
		ToSnapshotID:   "two",
		CreatedAt:      time.Date(2026, 7, 6, 14, 31, 0, 0, time.UTC),
		Changes:        []migrate.Change{},
	})

	_, err := migrate.ScanDir(dir)
	if err == nil || !strings.Contains(err.Error(), "does not match previous toSnapshotId") {
		t.Fatalf("expected lineage error, got %v", err)
	}
}

func TestScanDirRejectsUntimestampedSQL(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "add_users.sql"), []byte("-- +gosqlkit Meta\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := migrate.ScanDir(dir)
	if err == nil || !strings.Contains(err.Error(), "must start with YYYYMMDDHHMMSS_") {
		t.Fatalf("expected filename error, got %v", err)
	}
}

func writeRenderedMigration(t *testing.T, dir string, plan migrate.Plan) {
	t.Helper()

	files, err := (goose.Renderer{}).Render(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file.Name), []byte(file.Content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
