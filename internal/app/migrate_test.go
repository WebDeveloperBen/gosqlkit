package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/goose"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
)

func TestMigrateCreateWithConfigWritesEmptyGooseMigration(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ConfigName)
	if err := os.WriteFile(configPath, []byte(`version: "1"
dialect: postgresql
schema: "schema"
migrations:
  dir: "db/migrations"
  runner: goose
`), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}

	result, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:      "Add Users",
		Empty:     true,
		CreatedAt: time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Dir != filepath.Join(dir, "db", "migrations") {
		t.Fatalf("dir = %q", result.Dir)
	}
	if len(result.Files) != 1 {
		t.Fatalf("files = %v", result.Files)
	}
	if filepath.Base(result.Files[0]) != "20260706143000_add_users.sql" {
		t.Fatalf("file = %q", result.Files[0])
	}

	content, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"-- +gosqlkit Meta",
		`--   "dialect": "postgresql"`,
		"-- +goose Up",
		"-- +goose Down",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("content missing %q\n%s", want, content)
		}
	}
}

func TestMigrateCreateWithConfigWritesBaselineGooseMigration(t *testing.T) {
	config := &Config{
		Dialect: "postgres",
		rootDir: mustModuleDir(t),
		Schema: SchemaSpec{
			paths: []string{"./examples/basic/schema"},
		},
		Migrations: MigrationSpec{
			Dir:    t.TempDir(),
			Runner: DefaultMigrationsRunner,
		},
	}

	result, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:      "baseline",
		CreatedAt: time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(result.Files[0]) != "20260706143000_baseline.sql" {
		t.Fatalf("file = %q", result.Files[0])
	}

	content, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`--       "op": "baseline"`,
		`--   "toSnapshotId": "`,
		`--   "targetSnapshot": {`,
		`--     "snapshotId": "`,
		"-- +goose Up",
		"CREATE SCHEMA billing;",
		"CREATE TABLE users",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("content missing %q\n%s", want, content)
		}
	}
	if strings.Contains(string(content), "-- +goose Down") {
		t.Fatalf("baseline migration should not include an automatic down section\n%s", content)
	}
}

func TestMigrateCreateWithConfigWritesDiffMigration(t *testing.T) {
	dir := t.TempDir()
	config := &Config{
		Dialect: "postgres",
		rootDir: mustModuleDir(t),
		Schema: SchemaSpec{
			paths: []string{"./examples/basic/schema"},
		},
		Migrations: MigrationSpec{
			Dir:    dir,
			Runner: DefaultMigrationsRunner,
		},
	}

	currentSnapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		t.Fatal(err)
	}
	previousSnapshot := snapshotWithoutColumn(t, currentSnapshot, "users", "tags")
	previousSnapshotID, err := snapshotIDFromJSON(previousSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	files, err := (goose.Renderer{}).Render(migrate.Plan{
		Name:           "baseline",
		Dialect:        "postgresql",
		ToSnapshotID:   previousSnapshotID,
		TargetSnapshot: previousSnapshot,
		CreatedAt:      time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		Changes:        []migrate.Change{{Op: "baseline", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
		UpSQL:          []string{"SELECT 1;"},
		DownSQL:        nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file.Name), []byte(file.Content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:      "add tags",
		CreatedAt: time.Date(2026, 7, 6, 14, 31, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`--   "fromSnapshotId": "` + previousSnapshotID + `"`,
		`--       "op": "alter"`,
		`--       "object": {`,
		`--         "kind": "column"`,
		`--         "key": "public.users.tags"`,
		"ALTER TABLE users ADD COLUMN tags text[];",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("content missing %q\n%s", want, content)
		}
	}
}

func TestMigrateCreateWithConfigRejectsUnsupportedRunnerOverride(t *testing.T) {
	config := &Config{
		Dialect: "postgresql",
		Migrations: MigrationSpec{
			Dir:    t.TempDir(),
			Runner: DefaultMigrationsRunner,
		},
	}

	_, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:   "add users",
		Runner: "golang-migrate",
		Empty:  true,
	})
	if err == nil || !strings.Contains(err.Error(), `unsupported migration runner "golang-migrate"`) {
		t.Fatalf("expected unsupported runner error, got %v", err)
	}
}

func TestReverseStatementsUsesReverseChangeOrder(t *testing.T) {
	statements := reverseStatements([]migrateplan.Change{
		migrateplan.NewChange(
			migrateplan.OperationCreate,
			migrateplan.Ref(migrateplan.ObjectKindTable, "public.users"),
			"create users",
			migrateplan.SQL("CREATE TABLE users (id uuid);"),
		).WithReverse(migrateplan.SQL("DROP TABLE users;")),
		migrateplan.NewChange(
			migrateplan.OperationCreate,
			migrateplan.Ref(migrateplan.ObjectKindIndex, "public.users.users_email_idx"),
			"create users email index",
			migrateplan.SQL("CREATE INDEX users_email_idx ON users (email);"),
		).WithReverse(migrateplan.SQL("DROP INDEX users_email_idx;")),
	})

	if len(statements) != 2 {
		t.Fatalf("statements = %#v", statements)
	}
	if statements[0].SQL != "DROP INDEX users_email_idx;" || statements[1].SQL != "DROP TABLE users;" {
		t.Fatalf("unexpected reverse order %#v", statements)
	}
}

func TestReverseStatementsRequiresEveryChangeToBeReversible(t *testing.T) {
	statements := reverseStatements([]migrateplan.Change{
		migrateplan.NewChange(
			migrateplan.OperationCreate,
			migrateplan.Ref(migrateplan.ObjectKindTable, "public.users"),
			"create users",
			migrateplan.SQL("CREATE TABLE users (id uuid);"),
		).WithReverse(migrateplan.SQL("DROP TABLE users;")),
		migrateplan.NewChange(
			migrateplan.OperationAlter,
			migrateplan.Ref(migrateplan.ObjectKindColumn, "public.users.email"),
			"add users email",
			migrateplan.SQL("ALTER TABLE users ADD COLUMN email text;"),
		),
	})

	if statements != nil {
		t.Fatalf("statements = %#v, want nil", statements)
	}
}

func TestMigrateCreateWithConfigDoesNotOverwrite(t *testing.T) {
	config := &Config{
		Dialect: "postgresql",
		Migrations: MigrationSpec{
			Dir:    t.TempDir(),
			Runner: DefaultMigrationsRunner,
		},
	}
	opts := MigrateCreateOptions{
		Name:      "add users",
		Empty:     true,
		CreatedAt: time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
	}

	if _, err := MigrateCreateWithConfig(config, opts); err != nil {
		t.Fatal(err)
	}
	_, err := MigrateCreateWithConfig(config, opts)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing file error, got %v", err)
	}
}

func TestMigrateCreateWithConfigRequiresDiffPlanningAfterBaseline(t *testing.T) {
	config := &Config{
		Dialect: "postgresql",
		Migrations: MigrationSpec{
			Dir:    t.TempDir(),
			Runner: DefaultMigrationsRunner,
		},
	}
	if _, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:      "manual",
		Empty:     true,
		CreatedAt: time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	_, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:      "next",
		CreatedAt: time.Date(2026, 7, 6, 14, 31, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "has no targetSnapshot metadata") {
		t.Fatalf("expected target snapshot error, got %v", err)
	}
}

func TestMigrateCheckWithConfigScansMigrationDir(t *testing.T) {
	dir := t.TempDir()
	config := &Config{
		Dialect: "postgresql",
		Migrations: MigrationSpec{
			Dir:    dir,
			Runner: DefaultMigrationsRunner,
		},
	}
	if _, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:      "add users",
		Empty:     true,
		CreatedAt: time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	result, err := MigrateCheckWithConfig(config, MigrateCheckOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Dir != dir || result.Count != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestMigrateCheckWithConfigValidatesGooseAnnotations(t *testing.T) {
	dir := t.TempDir()
	config := &Config{
		Dialect: "postgresql",
		Migrations: MigrationSpec{
			Dir:    dir,
			Runner: DefaultMigrationsRunner,
		},
	}
	content := `-- +gosqlkit Meta
-- {
--   "version": 1,
--   "dialect": "postgresql",
--   "createdAt": "2026-07-06T14:30:00Z",
--   "changes": []
-- }
`
	if err := os.WriteFile(filepath.Join(dir, "20260706143000_add_users.sql"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := MigrateCheckWithConfig(config, MigrateCheckOptions{})
	if err == nil || !strings.Contains(err.Error(), "missing goose up annotation") {
		t.Fatalf("expected goose annotation error, got %v", err)
	}
}

func snapshotWithoutColumn(t *testing.T, snapshot, tableName, columnName string) string {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(snapshot), &raw); err != nil {
		t.Fatal(err)
	}
	var tables []ast.Table
	if err := json.Unmarshal(raw["tables"], &tables); err != nil {
		t.Fatal(err)
	}
	for i := range tables {
		if tables[i].Name != tableName {
			continue
		}
		columns := tables[i].Columns[:0]
		for _, column := range tables[i].Columns {
			if column.Name != columnName {
				columns = append(columns, column)
			}
		}
		tables[i].Columns = columns
	}
	data, err := json.Marshal(tables)
	if err != nil {
		t.Fatal(err)
	}
	raw["tables"] = data

	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(raw["columnMetadata"], &metadata); err != nil {
		t.Fatal(err)
	}
	delete(metadata, "public."+tableName+"."+columnName)
	data, err = json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	raw["columnMetadata"] = data

	data, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	out, err := injectSnapshotIDs(string(data), "")
	if err != nil {
		t.Fatal(err)
	}
	return out
}
