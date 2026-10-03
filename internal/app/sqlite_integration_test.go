package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
)

func TestSQLiteIntegrationWorkflow(t *testing.T) {
	if os.Getenv("GOSQLKIT_SQLITE_INTEGRATION") != "1" {
		t.Skip("set GOSQLKIT_SQLITE_INTEGRATION=1 to run SQLite integration tests")
	}

	root := mustModuleDir(t)
	work := t.TempDir()
	migrationDir := filepath.Join(work, "migrations")
	sourceURL := filepath.Join(work, "source.db")
	config := &Config{
		Dialect:    "sqlite",
		rootDir:    root,
		Schema:     SchemaSpec{paths: []string{"./examples/sqlite/schema"}},
		Migrations: MigrationSpec{Dir: migrationDir, Runner: migrate.RunnerGoose},
	}

	plan, err := MigratePlanWithConfig(config, MigratePlanOptions{FromURL: sourceURL})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) == 0 || len(plan.Statements) == 0 {
		t.Fatalf("live-source plan = %#v", plan)
	}

	created, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:    "initial_sqlite_schema",
		Dir:     migrationDir,
		FromURL: sourceURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Files) != 1 {
		t.Fatalf("created files = %#v", created.Files)
	}

	sandboxRoot := t.TempDir()
	t.Setenv("TMPDIR", sandboxRoot)
	checked, err := MigrateCheckWithConfig(config, MigrateCheckOptions{
		Dir:        migrationDir,
		SandboxURL: "sqlite::memory:",
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked.Sandbox == nil || checked.Sandbox.Applied != 1 || checked.Sandbox.DatabaseSnapshotID == "" {
		t.Fatalf("SQLite sandbox result = %#v", checked.Sandbox)
	}
	leftover, err := filepath.Glob(filepath.Join(sandboxRoot, "gosqlkit-sqlite-sandbox-*.db*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftover) != 0 {
		t.Fatalf("SQLite sandbox files remain: %#v", leftover)
	}

	migrations, err := migrate.ScanDir(migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	var failedSandbox string
	_, err = migrateCheckSQLiteSandbox(context.Background(), config, "", migrations, func(_ context.Context, databaseURL, _ string, _ []migrate.Migration) (*SandboxReplayResult, error) {
		failedSandbox = databaseURL
		return nil, errors.New("simulated replay failure")
	}, nil)
	if err == nil || failedSandbox == "" {
		t.Fatalf("failed sandbox replay returned err=%v path=%q", err, failedSandbox)
	}
	if _, err := os.Stat(failedSandbox); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed sandbox file remains or stat failed: %v", err)
	}

	applied, err := MigrateApplyWithConfig(config, MigrateApplyOptions{Dir: migrationDir, URL: sourceURL})
	if err != nil {
		t.Fatal(err)
	}
	if applied.Applied != 1 || applied.Skipped != 0 {
		t.Fatalf("first apply = %#v", applied)
	}
	reapplied, err := MigrateApplyWithConfig(config, MigrateApplyOptions{Dir: migrationDir, URL: sourceURL})
	if err != nil {
		t.Fatal(err)
	}
	if reapplied.Applied != 0 || reapplied.Skipped != 1 {
		t.Fatalf("second apply = %#v", reapplied)
	}

	drift, err := DriftCheckWithConfig(config, DriftCheckOptions{URL: sourceURL})
	if err != nil {
		t.Fatal(err)
	}
	if drift.Drift || drift.Dialect != "sqlite" {
		t.Fatalf("drift result = %#v", drift)
	}

	ci, err := CIDatabaseWithConfig(config, CIDatabaseOptions{URL: sourceURL})
	if err != nil {
		t.Fatal(err)
	}
	if ci.Drift == nil || ci.Drift.Drift || ci.Drift.Dialect != "sqlite" {
		t.Fatalf("SQLite CI database result = %#v", ci)
	}

	var output bytes.Buffer
	inspected, err := InspectWithConfig(config, InspectOptions{URL: sourceURL, Stdout: &output})
	if err != nil {
		t.Fatal(err)
	}
	var document sqliteschema.Document
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatalf("parse SQLite inspect output: %v\n%s", err, output.String())
	}
	if inspected.Dialect != "sqlite" || inspected.SnapshotID == "" || len(document.Tables) == 0 {
		t.Fatalf("inspect result=%#v document=%#v", inspected, document)
	}
}
