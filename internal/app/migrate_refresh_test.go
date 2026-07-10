package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/golangmigrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/goose"
)

func refreshTestConfig(t *testing.T, dir, runner string) *Config {
	t.Helper()
	return &Config{
		Dialect: "postgres",
		rootDir: mustModuleDir(t),
		Schema:  SchemaSpec{paths: []string{"./examples/basic/schema"}},
		Migrations: MigrationSpec{
			Dir:    dir,
			Runner: runner,
		},
	}
}

func writeRefreshBaseline(t *testing.T, config *Config, dir, runner string) string {
	t.Helper()
	snapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		t.Fatal(err)
	}
	snapshotID, err := snapshotIDFromJSON(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var renderer migrate.Renderer = goose.Renderer{}
	if runner == migrate.RunnerGolangMigrate {
		renderer = golangmigrate.Renderer{}
	}
	files, err := renderer.Render(migrate.Plan{
		Name:           "baseline",
		Dialect:        "postgresql",
		ToSnapshotID:   snapshotID,
		TargetSnapshot: snapshot,
		CreatedAt:      time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		Changes:        []migrate.Change{{Op: "baseline", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
		UpSQL:          []string{"SELECT 1;"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file.Name), []byte(file.Content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return snapshotID
}

func TestMigrateRefreshWithConfigWritesGooseMigration(t *testing.T) {
	dir := t.TempDir()
	config := refreshTestConfig(t, dir, DefaultMigrationsRunner)
	snapshotID := writeRefreshBaseline(t, config, dir, DefaultMigrationsRunner)

	result, err := MigrateRefreshWithConfig(config, MigrateRefreshOptions{
		View:      "cached_bookings",
		CreatedAt: time.Date(2026, 7, 6, 14, 31, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
	content, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	for _, want := range []string{
		`--   "fromSnapshotId": "` + snapshotID + `"`,
		`--   "toSnapshotId": "` + snapshotID + `"`,
		`--       "op": "refresh"`,
		`--         "kind": "materialized_view"`,
		`--         "key": "cached_bookings"`,
		"lock-heavy",
		"-- +goose Up",
		"REFRESH MATERIALIZED VIEW cached_bookings;",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("content missing %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "-- +goose Down") {
		t.Fatalf("refresh migration must not contain a down section\n%s", got)
	}
	if strings.Contains(got, "CONCURRENTLY") {
		t.Fatalf("plain refresh must not be concurrent\n%s", got)
	}
	if _, err := migrate.ScanDir(dir); err != nil {
		t.Fatalf("migration lineage invalid after refresh: %v", err)
	}
}

func TestMigrateRefreshWithConfigConcurrently(t *testing.T) {
	dir := t.TempDir()
	config := refreshTestConfig(t, dir, DefaultMigrationsRunner)
	writeRefreshBaseline(t, config, dir, DefaultMigrationsRunner)

	result, err := MigrateRefreshWithConfig(config, MigrateRefreshOptions{
		View:         "cached_bookings",
		Concurrently: true,
		CreatedAt:    time.Date(2026, 7, 6, 14, 31, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	if !strings.Contains(got, "REFRESH MATERIALIZED VIEW CONCURRENTLY cached_bookings;") {
		t.Fatalf("missing concurrent refresh statement\n%s", got)
	}
	if !strings.Contains(got, "requires-ddl-review") {
		t.Fatalf("concurrent refresh should carry requires-ddl-review risk\n%s", got)
	}
}

func TestMigrateRefreshWithConfigGolangMigrateOmitsDownFile(t *testing.T) {
	dir := t.TempDir()
	config := refreshTestConfig(t, dir, migrate.RunnerGolangMigrate)
	writeRefreshBaseline(t, config, dir, migrate.RunnerGolangMigrate)

	result, err := MigrateRefreshWithConfig(config, MigrateRefreshOptions{
		View:      "cached_bookings",
		CreatedAt: time.Date(2026, 7, 6, 14, 31, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 up file, got %v", result.Files)
	}
	if !strings.HasSuffix(result.Files[0], ".up.sql") {
		t.Fatalf("expected a single .up.sql file, got %s", result.Files[0])
	}
}

func TestMigrateRefreshWithConfigUnknownView(t *testing.T) {
	dir := t.TempDir()
	config := refreshTestConfig(t, dir, DefaultMigrationsRunner)
	writeRefreshBaseline(t, config, dir, DefaultMigrationsRunner)

	_, err := MigrateRefreshWithConfig(config, MigrateRefreshOptions{View: "does_not_exist"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
	if !strings.Contains(err.Error(), "cached_bookings") {
		t.Fatalf("error should list available materialized views, got %v", err)
	}
}

func TestMigrateRefreshWithConfigRequiresExistingMigrations(t *testing.T) {
	dir := t.TempDir()
	config := refreshTestConfig(t, dir, DefaultMigrationsRunner)

	_, err := MigrateRefreshWithConfig(config, MigrateRefreshOptions{View: "cached_bookings"})
	if err == nil || !strings.Contains(err.Error(), "no migrations found") {
		t.Fatalf("expected no-migrations error, got %v", err)
	}
}

func TestMigrateRefreshWithConfigRequiresViewName(t *testing.T) {
	config := refreshTestConfig(t, t.TempDir(), DefaultMigrationsRunner)

	_, err := MigrateRefreshWithConfig(config, MigrateRefreshOptions{})
	if err == nil || !strings.Contains(err.Error(), "materialized view name is required") {
		t.Fatalf("expected missing-view error, got %v", err)
	}
}
