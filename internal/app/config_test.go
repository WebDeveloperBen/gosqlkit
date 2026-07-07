package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/app"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, app.ConfigName)
	if err := os.WriteFile(configPath, []byte(`version: "1"
dialect: postgresql
schema: "schema"
out:
  sql: "db/schema.sql"
  snapshot: "db/schema.json"
migrations:
  dir: "db/migrations"
  runner: goose
`), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := app.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if config.Dialect != "postgresql" {
		t.Fatalf("dialect = %q, want postgresql", config.Dialect)
	}
	if len(config.Schema.Paths()) != 1 || config.Schema.Paths()[0] != "schema" {
		t.Fatalf("schema paths = %v, want [schema]", config.Schema.Paths())
	}
	if config.Out.SQL != "db/schema.sql" {
		t.Fatalf("out.sql = %q, want db/schema.sql", config.Out.SQL)
	}
	if config.Out.Snapshot != "db/schema.json" {
		t.Fatalf("out.snapshot = %q, want db/schema.json", config.Out.Snapshot)
	}
	if config.Migrations.Dir != "db/migrations" {
		t.Fatalf("migrations.dir = %q, want db/migrations", config.Migrations.Dir)
	}
	if config.Migrations.Runner != "goose" {
		t.Fatalf("migrations.runner = %q, want goose", config.Migrations.Runner)
	}
}

func TestLoadConfigSchemaAsList(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, app.ConfigName)
	if err := os.WriteFile(configPath, []byte(`version: "1"
schema:
  - "schema/users"
  - "schema/billing"
out:
  sql: "db/schema.sql"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := app.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	paths := config.Schema.Paths()
	if len(paths) != 2 || paths[0] != "schema/users" || paths[1] != "schema/billing" {
		t.Fatalf("schema paths = %v, want [schema/users schema/billing]", paths)
	}
}

func TestLoadConfigDefaultsDialectToPostgres(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, app.ConfigName)
	if err := os.WriteFile(configPath, []byte(`version: "1"
schema: "schema"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := app.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if config.Dialect != "postgres" {
		t.Fatalf("default dialect = %q, want postgres", config.Dialect)
	}
}

func TestLoadConfigDefaultsMigrations(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, app.ConfigName)
	if err := os.WriteFile(configPath, []byte(`version: "1"
schema: "schema"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := app.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if config.Migrations.Dir != app.DefaultMigrationsDir {
		t.Fatalf("migrations.dir = %q, want %q", config.Migrations.Dir, app.DefaultMigrationsDir)
	}
	if config.Migrations.Runner != app.DefaultMigrationsRunner {
		t.Fatalf("migrations.runner = %q, want %q", config.Migrations.Runner, app.DefaultMigrationsRunner)
	}
	if got := config.MigrationsDir(); got != filepath.Join(dir, app.DefaultMigrationsDir) {
		t.Fatalf("MigrationsDir() = %q", got)
	}
}

func TestLoadConfigRejectsUnsupportedMigrationRunner(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, app.ConfigName)
	if err := os.WriteFile(configPath, []byte(`version: "1"
schema: "schema"
migrations:
  runner: golang-migrate
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := app.LoadConfig(configPath)
	if err == nil {
		t.Fatal("expected error for unsupported migration runner")
	}
}

func TestLoadConfigRejectsMissingSchema(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, app.ConfigName)
	if err := os.WriteFile(configPath, []byte(`version: "1"
dialect: postgresql
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := app.LoadConfig(configPath)
	if err == nil {
		t.Fatal("expected error for missing schema")
	}
}

func TestDiscoverConfigSearchesParents(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, app.ConfigName)
	if err := os.WriteFile(configPath, []byte(`version: "1"
schema: "schema"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	subdir := filepath.Join(root, "deep", "nested")
	if err := os.MkdirAll(subdir, 0o750); err != nil {
		t.Fatal(err)
	}

	config, err := app.DiscoverConfig(subdir)
	if err != nil {
		t.Fatal(err)
	}
	if config.RootDir() != root {
		t.Fatalf("rootDir = %q, want %q", config.RootDir(), root)
	}
}

func TestDiscoverConfigFailsWhenNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := app.DiscoverConfig(dir)
	if err == nil {
		t.Fatal("expected error when config not found")
	}
}
