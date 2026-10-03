package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	_ "modernc.org/sqlite"
)

func TestSQLiteCLIMigrationWorkflowBothRunners(t *testing.T) {
	for _, runner := range []string{migrate.RunnerGoose, migrate.RunnerGolangMigrate} {
		t.Run(runner, func(t *testing.T) {
			testSQLiteCLIMigrationWorkflow(t, runner)
		})
	}
}

func testSQLiteCLIMigrationWorkflow(t *testing.T, runner string) {
	binary := requireSQLiteCLIBinary(t)
	project := newSQLiteCLIProject(t, binary)
	project.writeSchema(t, "schema", `package schema

import "github.com/webdeveloperben/gosqlkit/sqlite"

var Users = sqlite.Table(
	"users",
	sqlite.Integer("id").PrimaryKey(),
	sqlite.Text("email").NotNull(),
)
`)
	project.writeConfig(t, "schema", runner)

	for _, args := range [][]string{{"snapshot"}} {
		result := project.run(t, args...)
		if result.exitCode != 0 {
			t.Fatalf("gosqlkit %v exited %d: %s", args, result.exitCode, result.stderr)
		}
	}
	var desired sqliteschema.Document
	desiredData, err := os.ReadFile(filepath.Join(project.root, "db", "schema.snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(desiredData, &desired); err != nil {
		t.Fatalf("parse desired snapshot: %v", err)
	}
	if desired.Dialect != "sqlite" || desired.SnapshotID == "" {
		t.Fatalf("desired snapshot identity = dialect %q, id %q", desired.Dialect, desired.SnapshotID)
	}

	sourcePath := filepath.Join(project.root, "db", "source.sqlite")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o750); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.PingContext(context.Background()); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	plan := project.run(t, "migrate", "plan", "--from-url", sourcePath, "--json")
	if plan.exitCode != 0 {
		t.Fatalf("migrate plan exited %d: %s", plan.exitCode, plan.stderr)
	}
	var planned MigratePlanResult
	if err := json.Unmarshal([]byte(plan.stdout), &planned); err != nil {
		t.Fatalf("parse migration plan: %v\n%s", err, plan.stdout)
	}
	if planned.Dialect != "sqlite" || planned.FromSnapshotID == "" || planned.ToSnapshotID != desired.SnapshotID || len(planned.Changes) == 0 || len(planned.Statements) == 0 {
		t.Fatalf("live-source migration plan = %#v", planned)
	}

	created := project.run(t, "migrate", "create", "initial_users", "--from-url", sourcePath, "--no-interactive")
	if created.exitCode != 0 {
		t.Fatalf("migrate create exited %d: %s", created.exitCode, created.stderr)
	}
	migrationDir := filepath.Join(project.root, "db", "migrations")
	files, err := os.ReadDir(migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	expectedFiles := 1
	if runner == migrate.RunnerGolangMigrate {
		expectedFiles = 2
	}
	if len(files) != expectedFiles {
		t.Fatalf("created migration files = %v, want %d for %s", files, expectedFiles, runner)
	}

	checked := project.run(t, "migrate", "check", "--sandbox-url", "sqlite::memory:", "--json")
	if checked.exitCode != 0 {
		t.Fatalf("migrate check exited %d: stdout=%s stderr=%s", checked.exitCode, checked.stdout, checked.stderr)
	}
	var checkResult MigrateCheckResult
	if err := json.Unmarshal([]byte(checked.stdout), &checkResult); err != nil {
		t.Fatalf("parse migration check: %v\n%s", err, checked.stdout)
	}
	if checkResult.Count != 1 || checkResult.Sandbox == nil || checkResult.Sandbox.Applied != 1 || checkResult.Sandbox.ToSnapshotID != desired.SnapshotID || checkResult.Sandbox.DatabaseSnapshotID != desired.SnapshotID {
		t.Fatalf("migration check result = %#v", checkResult)
	}

	apply := project.run(t, "migrate", "apply", "--url", sourcePath, "--json")
	if apply.exitCode != 0 {
		t.Fatalf("first migrate apply exited %d: stdout=%s stderr=%s", apply.exitCode, apply.stdout, apply.stderr)
	}
	var firstApply MigrateApplyResult
	if err := json.Unmarshal([]byte(apply.stdout), &firstApply); err != nil {
		t.Fatalf("parse first migration apply: %v\n%s", err, apply.stdout)
	}
	if firstApply.Count != 1 || firstApply.Applied != 1 || firstApply.Skipped != 0 {
		t.Fatalf("first migration apply result = %#v", firstApply)
	}

	apply = project.run(t, "migrate", "apply", "--url", sourcePath, "--json")
	if apply.exitCode != 0 {
		t.Fatalf("repeat migrate apply exited %d: stdout=%s stderr=%s", apply.exitCode, apply.stdout, apply.stderr)
	}
	var repeatedApply MigrateApplyResult
	if err := json.Unmarshal([]byte(apply.stdout), &repeatedApply); err != nil {
		t.Fatalf("parse repeat migration apply: %v\n%s", err, apply.stdout)
	}
	if repeatedApply.Count != 1 || repeatedApply.Applied != 0 || repeatedApply.Skipped != 1 {
		t.Fatalf("repeat migration apply result = %#v", repeatedApply)
	}
	assertSQLiteRunnerVersionRecorded(t, sourcePath, runner)

	drift := project.run(t, "drift", "check", "--url", sourcePath, "--json")
	if drift.exitCode != 0 {
		t.Fatalf("applied database drift check exited %d: stdout=%s stderr=%s", drift.exitCode, drift.stdout, drift.stderr)
	}
	var appliedDrift DriftCheckResult
	if err := json.Unmarshal([]byte(drift.stdout), &appliedDrift); err != nil {
		t.Fatalf("parse applied database drift result: %v\n%s", err, drift.stdout)
	}
	if appliedDrift.Dialect != "sqlite" || appliedDrift.Drift {
		t.Fatalf("applied database drift result = %#v", appliedDrift)
	}

	inspect := project.run(t, "inspect", "--url", sourcePath)
	if inspect.exitCode != 0 {
		t.Fatalf("inspect applied database exited %d: %s", inspect.exitCode, inspect.stderr)
	}
	var applied sqliteschema.Document
	if err := json.Unmarshal([]byte(inspect.stdout), &applied); err != nil {
		t.Fatalf("parse applied database snapshot: %v\n%s", err, inspect.stdout)
	}
	if applied.Dialect != "sqlite" || applied.SnapshotID == "" || !hasSQLiteTable(applied.Tables, "users") {
		t.Fatalf("applied database snapshot = %#v", applied)
	}
}

func hasSQLiteTable(tables []sqliteschema.Table, name string) bool {
	for _, table := range tables {
		if table.Name == name {
			return true
		}
	}
	return false
}

func assertSQLiteRunnerVersionRecorded(t *testing.T, databasePath, runner string) {
	t.Helper()
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("close migration version database: %v", err)
		}
	}()

	var recorded int
	query := "SELECT COUNT(*) FROM goose_db_version WHERE is_applied = 1"
	if runner == migrate.RunnerGolangMigrate {
		query = "SELECT COUNT(*) FROM schema_migrations WHERE dirty = 0 AND version > 0"
	}
	if err := database.QueryRow(query).Scan(&recorded); err != nil {
		t.Fatalf("read %s applied-version records: %v", runner, err)
	}
	if recorded != 1 {
		t.Fatalf("%s applied-version records = %d, want 1", runner, recorded)
	}
}
