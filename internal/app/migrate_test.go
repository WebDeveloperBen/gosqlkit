package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
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

func TestMigrateCreateWithConfigWritesEmptyGolangMigrateMigration(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ConfigName)
	if err := os.WriteFile(configPath, []byte(`version: "1"
dialect: postgresql
schema: "schema"
migrations:
  dir: "db/migrations"
  runner: golang-migrate
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
	if len(result.Files) != 2 {
		t.Fatalf("files = %v", result.Files)
	}
	if filepath.Base(result.Files[0]) != "20260706143000_add_users.up.sql" {
		t.Fatalf("up file = %q", result.Files[0])
	}
	if filepath.Base(result.Files[1]) != "20260706143000_add_users.down.sql" {
		t.Fatalf("down file = %q", result.Files[1])
	}

	up, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"-- +gosqlkit Meta",
		`--   "dialect": "postgresql"`,
	} {
		if !strings.Contains(string(up), want) {
			t.Fatalf("up content missing %q\n%s", want, up)
		}
	}
	down, err := os.ReadFile(result.Files[1])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(down), "-- +gosqlkit Meta") {
		t.Fatalf("down content should not contain metadata\n%s", down)
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
		"-- +goose Down",
		"ALTER TABLE users DROP COLUMN tags;",
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
		Runner: "unknown",
		Empty:  true,
	})
	if err == nil || !strings.Contains(err.Error(), `unsupported migration runner "unknown"`) {
		t.Fatalf("expected unsupported runner error, got %v", err)
	}
}

func TestMigrateCreateWithConfigRejectsUnsupportedInteractionMode(t *testing.T) {
	config := &Config{
		Dialect: "postgresql",
		Migrations: MigrationSpec{
			Dir:    t.TempDir(),
			Runner: DefaultMigrationsRunner,
		},
	}

	_, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:        "add users",
		Empty:       true,
		Interaction: InteractionMode("sometimes"),
	})
	if err == nil || !strings.Contains(err.Error(), `unsupported interaction mode "sometimes"`) {
		t.Fatalf("expected unsupported interaction error, got %v", err)
	}
}

func TestDestructiveGuardErrorSuggestsRenameAnnotations(t *testing.T) {
	planned := &migrateplan.Plan{Changes: []migrateplan.Change{
		migrateplan.NewChange(
			migrateplan.OperationDrop,
			migrateplan.Ref(migrateplan.ObjectKindTable, "public.old_users"),
			"drop old users",
		).WithRisks(migrateplan.RiskDestructive),
		migrateplan.NewChange(
			migrateplan.OperationCreate,
			migrateplan.Ref(migrateplan.ObjectKindTable, "public.users"),
			"create users",
		),
	}}

	err := destructiveGuardError(planned)
	for _, want := range []string{
		"possible renames were detected",
		"table public.users: set previousName to \"old_users\"",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("guard error missing %q\n%s", want, err)
		}
	}
}

func TestDiffMigrationPlanAcceptsInteractiveRenameDecision(t *testing.T) {
	previousSnapshot, err := snapshotJSONForSchema(pgschema.Schema{Tables: []pgschema.Table{{
		Table: ast.Table{
			Name: "old_users",
			Columns: []ast.Column{{
				Name: "id",
				Type: "uuid",
			}},
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	previousSnapshotID, err := snapshotIDFromJSON(previousSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	currentSnapshot, err := snapshotJSONForSchema(pgschema.Schema{Tables: []pgschema.Table{{
		Table: ast.Table{
			Name: "users",
			Columns: []ast.Column{{
				Name: "id",
				Type: "uuid",
			}},
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	currentSnapshotID, err := snapshotIDFromJSON(currentSnapshot)
	if err != nil {
		t.Fatal(err)
	}

	var prompted []RenameCandidate
	plan, err := diffMigrationPlan(
		&Config{Dialect: "postgresql"},
		MigrateCreateOptions{
			Name: "rename users",
			RenameDecider: func(candidates []RenameCandidate) ([]RenameDecision, error) {
				prompted = append(prompted, candidates...)
				return []RenameDecision{{Candidate: candidates[0], Accept: true}}, nil
			},
		},
		migrate.Migration{Metadata: migrate.Metadata{
			ToSnapshotID:   previousSnapshotID,
			TargetSnapshot: json.RawMessage(previousSnapshot),
		}},
		currentSnapshot,
		currentSnapshotID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(prompted) != 1 {
		t.Fatalf("prompted candidates = %#v", prompted)
	}
	if prompted[0].Kind != "table" || prompted[0].FromKey != "public.old_users" || prompted[0].ToKey != "public.users" {
		t.Fatalf("unexpected candidate %#v", prompted[0])
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Op != "rename" {
		t.Fatalf("expected rename-only plan, got %#v", plan.Changes)
	}
	if len(plan.UpStatements) != 1 || !strings.Contains(plan.UpStatements[0].SQL, "ALTER TABLE old_users RENAME TO users;") {
		t.Fatalf("unexpected up statements %#v", plan.UpStatements)
	}
	if strings.Contains(plan.TargetSnapshot, "previousName") {
		t.Fatalf("target snapshot should remain the generated snapshot without prompt-only previousName metadata\n%s", plan.TargetSnapshot)
	}
	if plan.ToSnapshotID != currentSnapshotID {
		t.Fatalf("to snapshot = %q, want %q", plan.ToSnapshotID, currentSnapshotID)
	}
}

func snapshotJSONForSchema(schema pgschema.Schema) (string, error) {
	raw, err := pgschema.JSON("postgresql", schema)
	if err != nil {
		return "", err
	}
	return injectSnapshotIDs(string(raw), "")
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

func TestReverseStatementsKeepsReversibleChangesWithIrreversiblePlaceholder(t *testing.T) {
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

	if len(statements) != 2 {
		t.Fatalf("statements = %#v", statements)
	}
	if statements[0].SQL != "-- no automatic down for: column public.users.email" {
		t.Fatalf("first statement = %q", statements[0].SQL)
	}
	if statements[1].SQL != "DROP TABLE users;" {
		t.Fatalf("second statement = %q", statements[1].SQL)
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

func TestMigrateCheckWithConfigReplaysSandboxAndComparesTargetSnapshot(t *testing.T) {
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
	currentSnapshotID, err := snapshotIDFromJSON(currentSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	files, err := (goose.Renderer{}).Render(migrate.Plan{
		Name:           "baseline",
		Dialect:        "postgresql",
		ToSnapshotID:   currentSnapshotID,
		TargetSnapshot: currentSnapshot,
		CreatedAt:      time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		Changes:        []migrate.Change{{Op: "baseline", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
		UpSQL:          []string{"SELECT 1;"},
		DownSQL:        nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, files[0].Name), []byte(files[0].Content), 0o600); err != nil {
		t.Fatal(err)
	}

	var gotURL string
	var gotMigrations int
	var currentDoc pgschema.Document
	if err := json.Unmarshal([]byte(currentSnapshot), &currentDoc); err != nil {
		t.Fatal(err)
	}
	result, err := MigrateCheckWithConfig(config, MigrateCheckOptions{
		SandboxURL: "postgres://localhost/app",
		sandboxReplay: func(_ context.Context, sandboxURL, runner string, migrations []migrate.Migration) (*SandboxReplayResult, error) {
			gotURL = sandboxURL
			gotMigrations = len(migrations)
			if runner != DefaultMigrationsRunner {
				t.Fatalf("runner = %q", runner)
			}
			return &SandboxReplayResult{Applied: len(migrations), LastFile: migrations[len(migrations)-1].Name}, nil
		},
		sandboxInspect: func(context.Context, string) (pgschema.Schema, error) {
			return projectDriftDocument(currentDoc), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "postgres://localhost/app" || gotMigrations != 1 {
		t.Fatalf("sandbox replay url=%q migrations=%d", gotURL, gotMigrations)
	}
	if result.Sandbox == nil || result.Sandbox.Applied != 1 || result.Sandbox.ToSnapshotID != currentSnapshotID {
		t.Fatalf("sandbox result = %#v", result.Sandbox)
	}
	if result.Sandbox.DatabaseSnapshotID == "" {
		t.Fatalf("sandbox result missing database snapshot ID: %#v", result.Sandbox)
	}
}

func TestMigrateCheckWithConfigUsesSandboxURLEnv(t *testing.T) {
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
	currentSnapshotID, err := snapshotIDFromJSON(currentSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	files, err := (goose.Renderer{}).Render(migrate.Plan{
		Name:           "baseline",
		Dialect:        "postgresql",
		ToSnapshotID:   currentSnapshotID,
		TargetSnapshot: currentSnapshot,
		CreatedAt:      time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		Changes:        []migrate.Change{{Op: "baseline", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
		UpSQL:          []string{"SELECT 1;"},
		DownSQL:        nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, files[0].Name), []byte(files[0].Content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOSQLKIT_SANDBOX_URL", "postgres://localhost/sandbox")
	var gotURL string
	var currentDoc pgschema.Document
	if err := json.Unmarshal([]byte(currentSnapshot), &currentDoc); err != nil {
		t.Fatal(err)
	}
	_, err = MigrateCheckWithConfig(config, MigrateCheckOptions{
		SandboxURLEnv: "GOSQLKIT_SANDBOX_URL",
		sandboxReplay: func(_ context.Context, sandboxURL, _ string, migrations []migrate.Migration) (*SandboxReplayResult, error) {
			gotURL = sandboxURL
			return &SandboxReplayResult{Applied: len(migrations), LastFile: migrations[len(migrations)-1].Name}, nil
		},
		sandboxInspect: func(context.Context, string) (pgschema.Schema, error) {
			return projectDriftDocument(currentDoc), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "postgres://localhost/sandbox" {
		t.Fatalf("sandbox URL = %q", gotURL)
	}
}

func TestMigrateCheckWithConfigRejectsSandboxReplayDrift(t *testing.T) {
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
	currentSnapshotID, err := snapshotIDFromJSON(currentSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	files, err := (goose.Renderer{}).Render(migrate.Plan{
		Name:           "baseline",
		Dialect:        "postgresql",
		ToSnapshotID:   currentSnapshotID,
		TargetSnapshot: currentSnapshot,
		CreatedAt:      time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		Changes:        []migrate.Change{{Op: "baseline", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
		UpSQL:          []string{"SELECT 1;"},
		DownSQL:        nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, files[0].Name), []byte(files[0].Content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = MigrateCheckWithConfig(config, MigrateCheckOptions{
		SandboxURL: "postgres://localhost/app",
		sandboxReplay: func(_ context.Context, _ string, _ string, migrations []migrate.Migration) (*SandboxReplayResult, error) {
			return &SandboxReplayResult{Applied: len(migrations), LastFile: migrations[len(migrations)-1].Name}, nil
		},
		sandboxInspect: func(context.Context, string) (pgschema.Schema, error) {
			return pgschema.Schema{}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "sandbox replay drift detected") {
		t.Fatalf("expected sandbox replay drift error, got %v", err)
	}
}

func TestMigrateCheckWithConfigRejectsSandboxWhenLatestSnapshotIsStale(t *testing.T) {
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
	files, err := (goose.Renderer{}).Render(migrate.Plan{
		Name:         "baseline",
		Dialect:      "postgresql",
		ToSnapshotID: "stale",
		TargetSnapshot: `{
  "version": 1,
  "dialect": "postgresql",
  "snapshotId": "stale"
}`,
		CreatedAt: time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		Changes:   []migrate.Change{{Op: "baseline", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
		UpSQL:     []string{"SELECT 1;"},
		DownSQL:   nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, files[0].Name), []byte(files[0].Content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = MigrateCheckWithConfig(config, MigrateCheckOptions{
		SandboxURL: "postgres://localhost/app",
		sandboxReplay: func(context.Context, string, string, []migrate.Migration) (*SandboxReplayResult, error) {
			t.Fatal("sandbox replay should not run when metadata is stale")
			return nil, errors.New("unexpected sandbox replay")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match current schema snapshot") {
		t.Fatalf("expected stale snapshot error, got %v", err)
	}
}

func TestMigrateCheckWithConfigRejectsSandboxWithoutTargetSnapshot(t *testing.T) {
	dir := t.TempDir()
	config := &Config{
		Dialect: "postgresql",
		Migrations: MigrationSpec{
			Dir:    dir,
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

	_, err := MigrateCheckWithConfig(config, MigrateCheckOptions{
		SandboxURL: "postgres://localhost/app",
		sandboxReplay: func(context.Context, string, string, []migrate.Migration) (*SandboxReplayResult, error) {
			t.Fatal("sandbox replay should not run without target snapshot metadata")
			return nil, errors.New("unexpected sandbox replay")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "latest migration has no targetSnapshot metadata") {
		t.Fatalf("expected missing target snapshot error, got %v", err)
	}
}

func TestMigrateCreateWithConfigRejectsDestructiveChangesByDefault(t *testing.T) {
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

	currentSnapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		t.Fatal(err)
	}
	destructivePrevious := snapshotWithExtraColumn(t, currentSnapshot, "users", "legacy_column", "text")
	destructivePreviousID, err := snapshotIDFromJSON(destructivePrevious)
	if err != nil {
		t.Fatal(err)
	}
	files, err := (goose.Renderer{}).Render(migrate.Plan{
		Name:           "baseline",
		Dialect:        "postgresql",
		ToSnapshotID:   destructivePreviousID,
		TargetSnapshot: destructivePrevious,
		CreatedAt:      time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		Changes:        []migrate.Change{{Op: "baseline", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
		UpSQL:          []string{"SELECT 1;"},
		DownSQL:        nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(config.Migrations.Dir, file.Name), []byte(file.Content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	_, err = MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:      "drop legacy",
		CreatedAt: time.Date(2026, 7, 6, 14, 31, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "destructive") {
		t.Fatalf("expected destructive guard error, got %v", err)
	}

	result, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:             "drop legacy",
		AllowDestructive: true,
		CreatedAt:        time.Date(2026, 7, 6, 14, 32, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("expected success with --allow-destructive, got %v", err)
	}
	content, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "ALTER TABLE users DROP COLUMN legacy_column;") {
		t.Fatalf("expected migration to drop legacy_column, got:\n%s", content)
	}
}

func TestDiffMigrationPlanRejectsUnrenderedDestructiveChanges(t *testing.T) {
	previousSnapshot := snapshotJSON(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Enums: []pgschema.Enum{{
			Name:   "invoice_status",
			Values: []string{"draft", "issued"},
		}},
	})
	currentSnapshot := snapshotJSON(t, pgschema.Document{
		Dialect: "postgresql",
		Version: pgschema.SnapshotVersion,
		Enums: []pgschema.Enum{{
			Name:   "invoice_status",
			Values: []string{"draft"},
		}},
	})
	previousID, err := snapshotIDFromJSON(previousSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	currentID, err := snapshotIDFromJSON(currentSnapshot)
	if err != nil {
		t.Fatal(err)
	}

	config := &Config{Dialect: "postgresql"}
	previous := migrate.Migration{Metadata: migrate.Metadata{
		ToSnapshotID:   previousID,
		TargetSnapshot: []byte(previousSnapshot),
	}}

	_, err = diffMigrationPlan(config, MigrateCreateOptions{
		Name: "remove enum value",
	}, previous, currentSnapshot, currentID)
	if err == nil || !strings.Contains(err.Error(), "destructive") || !strings.Contains(err.Error(), "public.invoice_status") {
		t.Fatalf("expected destructive enum-value guard, got %v", err)
	}

	_, err = diffMigrationPlan(config, MigrateCreateOptions{
		Name:             "remove enum value",
		AllowDestructive: true,
	}, previous, currentSnapshot, currentID)
	if err == nil || !strings.Contains(err.Error(), "no executable SQL") {
		t.Fatalf("expected no executable SQL error, got %v", err)
	}
}

func TestMigratePlanWithConfigReportsDestructive(t *testing.T) {
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

	currentSnapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		t.Fatal(err)
	}
	cleanPrevious := snapshotWithoutColumn(t, currentSnapshot, "users", "tags")
	cleanPreviousID, err := snapshotIDFromJSON(cleanPrevious)
	if err != nil {
		t.Fatal(err)
	}
	files, err := (goose.Renderer{}).Render(migrate.Plan{
		Name:           "baseline",
		Dialect:        "postgresql",
		ToSnapshotID:   cleanPreviousID,
		TargetSnapshot: cleanPrevious,
		CreatedAt:      time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		Changes:        []migrate.Change{{Op: "baseline", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
		UpSQL:          []string{"SELECT 1;"},
		DownSQL:        nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(config.Migrations.Dir, file.Name), []byte(file.Content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := MigratePlanWithConfig(config, MigratePlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Destructive != nil {
		t.Fatalf("expected no destructive changes, got %d", len(result.Destructive))
	}

	destructivePrevious := snapshotWithExtraColumn(t, currentSnapshot, "users", "legacy_column", "text")
	destructivePreviousID, err := snapshotIDFromJSON(destructivePrevious)
	if err != nil {
		t.Fatal(err)
	}
	files, err = (goose.Renderer{}).Render(migrate.Plan{
		Name:           "baseline",
		Dialect:        "postgresql",
		ToSnapshotID:   destructivePreviousID,
		TargetSnapshot: destructivePrevious,
		CreatedAt:      time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		Changes:        []migrate.Change{{Op: "baseline", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
		UpSQL:          []string{"SELECT 1;"},
		DownSQL:        nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(config.Migrations.Dir, file.Name), []byte(file.Content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err = MigratePlanWithConfig(config, MigratePlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Destructive) == 0 {
		t.Fatalf("expected destructive changes, got %#v", result)
	}
}

func TestMigratePlanWithConfigUsesLivePostgresSource(t *testing.T) {
	config := &Config{
		Dialect: "postgresql",
		rootDir: mustModuleDir(t),
		Schema:  SchemaSpec{paths: []string{"./examples/basic/schema"}},
		Migrations: MigrationSpec{
			Dir:    t.TempDir(),
			Runner: DefaultMigrationsRunner,
		},
	}

	result, err := MigratePlanWithConfig(config, MigratePlanOptions{
		FromURL: "postgres://source",
		inspectPG: func(_ context.Context, databaseURL string) (pgschema.Schema, error) {
			if databaseURL != "postgres://source" {
				t.Fatalf("database URL = %q", databaseURL)
			}
			return pgschema.Schema{}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FromSnapshotID == "" || result.ToSnapshotID == "" {
		t.Fatalf("snapshot IDs = %#v", result)
	}
	if len(result.Changes) == 0 {
		t.Fatalf("changes = %#v", result)
	}
}

func TestMigrateCreateWithConfigUsesLivePostgresSource(t *testing.T) {
	config := &Config{
		Dialect: "postgresql",
		rootDir: mustModuleDir(t),
		Schema:  SchemaSpec{paths: []string{"./examples/basic/schema"}},
		Migrations: MigrationSpec{
			Dir:    t.TempDir(),
			Runner: DefaultMigrationsRunner,
		},
	}

	result, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:      "from live source",
		CreatedAt: time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC),
		FromURL:   "postgres://source",
		inspectPG: func(_ context.Context, databaseURL string) (pgschema.Schema, error) {
			if databaseURL != "postgres://source" {
				t.Fatalf("database URL = %q", databaseURL)
			}
			return pgschema.Schema{}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("files = %#v", result)
	}
	migrations, err := migrate.ScanDir(config.Migrations.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 1 || migrations[0].Metadata.FromSnapshotID == "" || migrations[0].Metadata.ToSnapshotID == "" {
		t.Fatalf("migration metadata = %#v", migrations)
	}
}

func TestMigrateApplyWithConfigUsesDatabaseURLEnv(t *testing.T) {
	dir := t.TempDir()
	config := &Config{
		Dialect: "postgresql",
		Migrations: MigrationSpec{
			Dir:    dir,
			Runner: DefaultMigrationsRunner,
		},
	}
	files, err := (goose.Renderer{}).Render(migrate.Plan{
		Name:      "add users",
		Dialect:   "postgresql",
		CreatedAt: time.Date(2026, 7, 6, 14, 30, 0, 0, time.UTC),
		Changes:   []migrate.Change{{Op: "manual", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
		UpSQL:     []string{"SELECT 1;"},
		DownSQL:   []string{"SELECT 1;"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, files[0].Name), []byte(files[0].Content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOSQLKIT_DATABASE_URL", "postgres://localhost/app")
	var gotURL string
	var gotRunner string
	var gotMigrations int
	result, err := MigrateApplyWithConfig(config, MigrateApplyOptions{
		URLEnv: "GOSQLKIT_DATABASE_URL",
		apply: func(_ context.Context, databaseURL, runner string, migrations []migrate.Migration) (*MigrateApplyResult, error) {
			gotURL = databaseURL
			gotRunner = runner
			gotMigrations = len(migrations)
			return &MigrateApplyResult{Applied: 1, LastFile: migrations[0].Name}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "postgres://localhost/app" || gotRunner != DefaultMigrationsRunner || gotMigrations != 1 {
		t.Fatalf("apply url=%q runner=%q migrations=%d", gotURL, gotRunner, gotMigrations)
	}
	if result.Dir != dir || result.Count != 1 || result.Applied != 1 || result.LastFile == "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestMigrateApplyWithConfigDefaultsToDatabaseURL(t *testing.T) {
	config := &Config{
		Dialect: "postgresql",
		Migrations: MigrationSpec{
			Dir:    t.TempDir(),
			Runner: DefaultMigrationsRunner,
		},
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/app")
	var gotURL string
	result, err := MigrateApplyWithConfig(config, MigrateApplyOptions{
		apply: func(_ context.Context, databaseURL, _ string, migrations []migrate.Migration) (*MigrateApplyResult, error) {
			gotURL = databaseURL
			if len(migrations) != 0 {
				t.Fatalf("migrations = %#v", migrations)
			}
			return &MigrateApplyResult{}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "postgres://localhost/app" {
		t.Fatalf("database URL = %q", gotURL)
	}
	if result.Count != 0 {
		t.Fatalf("result = %#v", result)
	}
}

func TestMigrateApplyWithConfigRequiresDatabaseURL(t *testing.T) {
	config := &Config{
		Dialect: "postgresql",
		Migrations: MigrationSpec{
			Dir:    t.TempDir(),
			Runner: DefaultMigrationsRunner,
		},
	}
	t.Setenv("DATABASE_URL", "")

	_, err := MigrateApplyWithConfig(config, MigrateApplyOptions{
		apply: func(context.Context, string, string, []migrate.Migration) (*MigrateApplyResult, error) {
			t.Fatal("apply should not run without a database URL")
			return nil, errors.New("unexpected apply")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "database URL is required") {
		t.Fatalf("expected database URL error, got %v", err)
	}
}

func snapshotWithoutColumn(t *testing.T, snapshot, tableName, columnName string) string {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(snapshot), &raw); err != nil {
		t.Fatal(err)
	}
	var tables []pgschema.Table
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

func snapshotJSON(t *testing.T, doc pgschema.Document) string {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	out, err := injectSnapshotIDs(string(data), "")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func snapshotWithExtraColumn(t *testing.T, snapshot, tableName, columnName, columnType string) string {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(snapshot), &raw); err != nil {
		t.Fatal(err)
	}
	var tables []pgschema.Table
	if err := json.Unmarshal(raw["tables"], &tables); err != nil {
		t.Fatal(err)
	}
	for i := range tables {
		if tables[i].Name != tableName {
			continue
		}
		tables[i].Columns = append(tables[i].Columns, ast.Column{
			Name: columnName,
			Type: columnType,
		})
	}
	data, err := json.Marshal(tables)
	if err != nil {
		t.Fatal(err)
	}
	raw["tables"] = data

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
