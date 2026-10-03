package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	_ "modernc.org/sqlite"
)

func TestSQLiteCLIRebuildEvolutionBothRunners(t *testing.T) {
	for _, runner := range []string{migrate.RunnerGoose, migrate.RunnerGolangMigrate} {
		t.Run(runner, func(t *testing.T) {
			testSQLiteCLIRebuildEvolution(t, runner)
		})
	}
}

func testSQLiteCLIRebuildEvolution(t *testing.T, runner string) {
	binary := requireSQLiteCLIBinary(t)
	project := newSQLiteCLIProject(t, binary)
	project.writeSchema(t, "schema", sqliteRebuildSchema("sqlite.Text(\"email\")"))
	project.writeConfig(t, "schema", runner)
	sandboxRoot := t.TempDir()
	t.Setenv("TMPDIR", sandboxRoot)

	sourcePath := filepath.Join(project.root, "db", "source.sqlite")
	createEmptySQLiteFile(t, sourcePath)
	baseline := project.run(t, "migrate", "create", "initial_schema", "--from-url", sourcePath, "--no-interactive")
	if baseline.exitCode != 0 {
		t.Fatalf("create baseline migration exited %d: %s", baseline.exitCode, baseline.stderr)
	}
	apply := project.run(t, "migrate", "apply", "--url", sourcePath, "--json")
	if apply.exitCode != 0 {
		t.Fatalf("apply baseline migration exited %d: stdout=%s stderr=%s", apply.exitCode, apply.stdout, apply.stderr)
	}

	database := openSQLiteFile(t, sourcePath)
	if _, err := database.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("INSERT INTO users (id, email, legacy) VALUES (1, 'one@example.test', 'preserve-me')"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("INSERT INTO posts (id, user_id, title) VALUES (1, 1, 'preserve this post')"); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	project.writeSchema(t, "schema", sqliteRebuildSchema("sqlite.Blob(\"email\")"))
	snapshot := project.run(t, "snapshot")
	if snapshot.exitCode != 0 {
		t.Fatalf("write target snapshot exited %d: %s", snapshot.exitCode, snapshot.stderr)
	}
	var desired sqliteschema.Document
	desiredData, err := os.ReadFile(filepath.Join(project.root, "db", "schema.snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(desiredData, &desired); err != nil {
		t.Fatalf("parse target snapshot: %v", err)
	}
	replayTarget, err := projectDatabaseSnapshot(string(desiredData), "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	replayTargetID, err := snapshotIDFromJSON(replayTarget)
	if err != nil {
		t.Fatal(err)
	}

	plan := project.run(t, "migrate", "plan", "--json")
	if plan.exitCode != 0 {
		t.Fatalf("plan rebuild migration exited %d: %s", plan.exitCode, plan.stderr)
	}
	var planned MigratePlanResult
	if err := json.Unmarshal([]byte(plan.stdout), &planned); err != nil {
		t.Fatalf("parse rebuild plan: %v\n%s", err, plan.stdout)
	}
	if planned.Dialect != "sqlite" || planned.ToSnapshotID != desired.SnapshotID || len(planned.Changes) != 1 || planned.Changes[0].Object.Kind != "table" || planned.Changes[0].Object.Key != "users" {
		t.Fatalf("rebuild plan = %#v", planned)
	}
	rebuildIncluded := false
	for _, statement := range planned.Statements {
		if strings.Contains(statement.SQL, "users_gosqlkit_new") {
			rebuildIncluded = true
			break
		}
	}
	if !rebuildIncluded {
		t.Fatalf("plan does not include the SQLite table rebuild: %#v", planned.Statements)
	}
	waitForSQLiteVersionBoundary(t, runner, filepath.Join(project.root, "db", "migrations"))
	created := project.run(t, "migrate", "create", "change_email_storage", "--no-interactive")
	if created.exitCode != 0 {
		t.Fatalf("create rebuild migration exited %d: %s", created.exitCode, created.stderr)
	}

	checked := project.run(t, "migrate", "check", "--sandbox-url", "sqlite::memory:", "--json")
	if checked.exitCode != 0 {
		t.Fatalf("check rebuild replay exited %d: stdout=%s stderr=%s", checked.exitCode, checked.stdout, checked.stderr)
	}
	assertSQLiteSandboxClean(t, sandboxRoot)
	var checkResult MigrateCheckResult
	if err := json.Unmarshal([]byte(checked.stdout), &checkResult); err != nil {
		t.Fatalf("parse rebuild check result: %v\n%s", err, checked.stdout)
	}
	if checkResult.Count < 2 || checkResult.Sandbox == nil || checkResult.Sandbox.Applied != checkResult.Count || checkResult.Sandbox.ToSnapshotID != desired.SnapshotID || checkResult.Sandbox.DatabaseSnapshotID != replayTargetID {
		t.Fatalf("rebuild replay result = %+v (sandbox=%+v, projected target id=%s)", checkResult, checkResult.Sandbox, replayTargetID)
	}
	apply = project.run(t, "migrate", "apply", "--url", sourcePath, "--json")
	if apply.exitCode != 0 {
		t.Fatalf("apply rebuild migration exited %d: stdout=%s stderr=%s", apply.exitCode, apply.stdout, apply.stderr)
	}
	var applied MigrateApplyResult
	if err := json.Unmarshal([]byte(apply.stdout), &applied); err != nil {
		t.Fatalf("parse rebuild apply result: %v\n%s", err, apply.stdout)
	}
	if applied.Count != checkResult.Count || applied.Applied == 0 || applied.Skipped == 0 || applied.Applied+applied.Skipped != applied.Count {
		t.Fatalf("rebuild apply result = %#v", applied)
	}

	repeated := project.run(t, "migrate", "apply", "--url", sourcePath, "--json")
	if repeated.exitCode != 0 {
		t.Fatalf("repeat rebuild apply exited %d: stdout=%s stderr=%s", repeated.exitCode, repeated.stdout, repeated.stderr)
	}
	var repeatedApply MigrateApplyResult
	if err := json.Unmarshal([]byte(repeated.stdout), &repeatedApply); err != nil {
		t.Fatalf("parse repeat rebuild apply result: %v\n%s", err, repeated.stdout)
	}
	if repeatedApply.Count != applied.Count || repeatedApply.Applied != 0 || repeatedApply.Skipped != applied.Count {
		t.Fatalf("repeat rebuild apply result = %#v", repeatedApply)
	}

	database = openSQLiteFile(t, sourcePath)
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("close rebuilt SQLite database: %v", err)
		}
	}()
	var email, legacy, title string
	if err := database.QueryRow("SELECT email, legacy FROM users WHERE id = 1").Scan(&email, &legacy); err != nil {
		t.Fatal(err)
	}
	if email != "one@example.test" || legacy != "preserve-me" {
		t.Fatalf("rebuilt user values = (%q, %q)", email, legacy)
	}
	if err := database.QueryRow("SELECT title FROM posts WHERE id = 1 AND user_id = 1").Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "preserve this post" {
		t.Fatalf("rebuilt post title = %q", title)
	}

	var indexCount, triggerCount int
	if err := database.QueryRow("SELECT COUNT(*) FROM sqlite_schema WHERE type = 'index' AND name = 'users_email_idx'").Scan(&indexCount); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow("SELECT COUNT(*) FROM sqlite_schema WHERE type = 'trigger' AND name = 'users_email_audit'").Scan(&triggerCount); err != nil {
		t.Fatal(err)
	}
	if indexCount != 1 || triggerCount != 1 {
		t.Fatalf("rebuilt dependents: index=%d trigger=%d", indexCount, triggerCount)
	}
	if _, err := database.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	foreignKeyRows, err := database.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = foreignKeyRows.Close() }()
	if foreignKeyRows.Next() {
		t.Fatal("rebuilt database has a foreign-key violation")
	}
	if err := foreignKeyRows.Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("INSERT INTO posts (id, user_id, title) VALUES (2, 999, 'orphan')"); err == nil {
		t.Fatal("foreign-key enforcement accepted an orphan post")
	}
	if _, err := database.Exec("INSERT INTO users (id, email, legacy) VALUES (2, 'two@example.test', 'new')"); err != nil {
		t.Fatal(err)
	}
	var auditCount int
	if err := database.QueryRow("SELECT COUNT(*) FROM user_email_events WHERE user_id = 2 AND email = 'two@example.test'").Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("rebuilt trigger audit rows = %d, want 1", auditCount)
	}

	migrationPath, originalMigration := corruptSQLiteMigrationForReplayFailure(t, filepath.Join(project.root, "db", "migrations"), runner)
	failedCheck := project.run(t, "migrate", "check", "--sandbox-url", "sqlite::memory:", "--json")
	if failedCheck.exitCode != 1 {
		t.Fatalf("invalid sandbox replay exited %d, want 1: stdout=%s stderr=%s", failedCheck.exitCode, failedCheck.stdout, failedCheck.stderr)
	}
	assertSQLiteSandboxClean(t, sandboxRoot)
	if err := os.WriteFile(migrationPath, originalMigration, 0o600); err != nil {
		t.Fatalf("restore migration after failed replay: %v", err)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("live source database did not survive sandbox failure: %v", err)
	}
	var retainedEmail string
	if err := database.QueryRow("SELECT email FROM users WHERE id = 1").Scan(&retainedEmail); err != nil {
		t.Fatalf("read live source after sandbox failure: %v", err)
	}
	if retainedEmail != "one@example.test" {
		t.Fatalf("live source email after sandbox failure = %q", retainedEmail)
	}
	project.writeSchema(t, "schema", sqliteRebuildSchemaWithoutLegacy("sqlite.Blob(\"email\")"))
	destructivePlan := project.run(t, "migrate", "plan", "--json")
	if destructivePlan.exitCode != 0 {
		t.Fatalf("plan destructive migration exited %d: %s", destructivePlan.exitCode, destructivePlan.stderr)
	}
	var destructive MigratePlanResult
	if err := json.Unmarshal([]byte(destructivePlan.stdout), &destructive); err != nil {
		t.Fatalf("parse destructive plan: %v\n%s", err, destructivePlan.stdout)
	}
	if len(destructive.Destructive) == 0 {
		t.Fatalf("dropping legacy column was not marked destructive: %#v", destructive)
	}
	migrationDir := filepath.Join(project.root, "db", "migrations")
	beforeFailure, err := os.ReadDir(migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	blocked := project.run(t, "migrate", "create", "drop_legacy", "--no-interactive")
	if blocked.exitCode != 1 || !strings.Contains(blocked.stderr, "--allow-destructive") {
		t.Fatalf("destructive creation result = exit %d, stderr %s", blocked.exitCode, blocked.stderr)
	}
	afterFailure, err := os.ReadDir(migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterFailure) != len(beforeFailure) {
		t.Fatalf("blocked destructive creation wrote files: before=%v after=%v", beforeFailure, afterFailure)
	}
	authorized := project.run(t, "migrate", "create", "drop_legacy", "--no-interactive", "--allow-destructive")
	if authorized.exitCode != 0 {
		t.Fatalf("authorized destructive creation exited %d: %s", authorized.exitCode, authorized.stderr)
	}
	assertSQLiteUnsupportedRenameFailsClosed(t, runner, binary)
}

func sqliteRebuildSchema(emailType string) string {
	return `package schema

import "github.com/webdeveloperben/gosqlkit/sqlite"

var Users = sqlite.Table(
	"users",
	sqlite.Integer("id").PrimaryKey(),
	` + emailType + `.NotNull(),
	sqlite.Text("legacy"),
	sqlite.IndexOn("users_email_idx", sqlite.IndexColumn("email")),
)

var Posts = sqlite.Table(
	"posts",
	sqlite.Integer("id").PrimaryKey(),
	sqlite.Integer("user_id").NotNull().References("users", "id"),
	sqlite.Text("title").NotNull(),
)

var UserEmailEvents = sqlite.Table(
	"user_email_events",
	sqlite.Integer("id").PrimaryKey(),
	sqlite.Integer("user_id").NotNull(),
	sqlite.Text("email").NotNull(),
)

var UserEmailAudit = sqlite.Trigger("users_email_audit", "users").
	After().
	Insert().
	ForEachRow().
	Body("INSERT INTO user_email_events (user_id, email) VALUES (NEW.id, NEW.email);")
`
}

func sqliteRebuildSchemaWithoutLegacy(emailType string) string {
	schema := sqliteRebuildSchema(emailType)
	return strings.Replace(schema, "\tsqlite.Text(\"legacy\"),\n", "", 1)
}

func createEmptySQLiteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	database := openSQLiteFile(t, path)
	if err := database.PingContext(context.Background()); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
}

func openSQLiteFile(t *testing.T, path string) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	return database
}

func assertSQLiteUnsupportedRenameFailsClosed(t *testing.T, runner, binary string) {
	t.Helper()
	project := newSQLiteCLIProject(t, binary)
	project.writeSchema(t, "schema", `package schema

import "github.com/webdeveloperben/gosqlkit/sqlite"

var Items = sqlite.Table("items", sqlite.Integer("id").PrimaryKey())
`)
	project.writeConfig(t, "schema", runner)
	sourcePath := filepath.Join(project.root, "db", "source.sqlite")
	createEmptySQLiteFile(t, sourcePath)
	baseline := project.run(t, "migrate", "create", "initial_items", "--from-url", sourcePath, "--no-interactive")
	if baseline.exitCode != 0 {
		t.Fatalf("create unsupported-case baseline exited %d: %s", baseline.exitCode, baseline.stderr)
	}
	project.writeSchema(t, "schema", `package schema

import "github.com/webdeveloperben/gosqlkit/sqlite"

var RenamedItems = sqlite.Table(
	"renamed_items",
	sqlite.Integer("id").PrimaryKey(),
	sqlite.Text("extra"),
).PreviousName("items")
`)
	migrationDir := filepath.Join(project.root, "db", "migrations")
	before, err := os.ReadDir(migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	unsupported := project.run(t, "migrate", "create", "unsupported_rename", "--no-interactive")
	if unsupported.exitCode != 1 {
		t.Fatalf("unsupported rename creation exited %d, want 1: stdout=%s stderr=%s", unsupported.exitCode, unsupported.stdout, unsupported.stderr)
	}
	after, err := os.ReadDir(migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("unsupported rename creation wrote files: before=%v after=%v", before, after)
	}
}

func waitForSQLiteVersionBoundary(t *testing.T, runner, migrationDir string) {
	t.Helper()
	if runner != migrate.RunnerGolangMigrate {
		return
	}
	files, err := os.ReadDir(migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("baseline migration directory is empty")
	}
	version := strings.SplitN(files[len(files)-1].Name(), "_", 2)[0]
	if len(version) < 14 {
		t.Fatalf("baseline golang-migrate version %q has no timestamp", version)
	}
	createdAt, err := time.Parse("20060102150405", version[:14])
	if err != nil {
		t.Fatalf("parse baseline golang-migrate version %q: %v", version, err)
	}
	nextVersionAt := createdAt.Add(time.Second)
	for time.Now().Before(nextVersionAt) {
		time.Sleep(10 * time.Millisecond)
	}
}

func assertSQLiteSandboxClean(t *testing.T, sandboxRoot string) {
	t.Helper()
	leftovers, err := filepath.Glob(filepath.Join(sandboxRoot, "gosqlkit-sqlite-sandbox-*.db*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("SQLite sandbox files remain: %#v", leftovers)
	}
}

func corruptSQLiteMigrationForReplayFailure(t *testing.T, migrationDir, runner string) (string, []byte) {
	t.Helper()
	files, err := os.ReadDir(migrationDir)
	if err != nil {
		t.Fatal(err)
	}
	for i := len(files) - 1; i >= 0; i-- {
		name := files[i].Name()
		if runner == migrate.RunnerGolangMigrate && !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		if runner == migrate.RunnerGoose && !strings.HasSuffix(name, ".sql") {
			continue
		}
		path := filepath.Join(migrationDir, name)
		// #nosec G304 -- Migration entries are read from the harness's temporary project directory.
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		corrupted := append([]byte(nil), content...)
		if runner == migrate.RunnerGoose {
			marker := []byte("-- +goose Down")
			section := bytes.Index(corrupted, marker)
			if section < 0 {
				t.Fatalf("Goose migration %q has no down section", name)
			}
			prefix := append([]byte(nil), corrupted[:section]...)
			prefix = append(prefix, []byte("THIS IS NOT VALID SQL;\n\n")...)
			corrupted = append(prefix, corrupted[section:]...)
		} else {
			corrupted = append(corrupted, []byte("\nTHIS IS NOT VALID SQL;\n")...)
		}
		// #nosec G703 -- Migration entries are rewritten only within the harness's temporary project directory.
		if err := os.WriteFile(path, corrupted, 0o600); err != nil {
			t.Fatal(err)
		}
		return path, content
	}
	t.Fatalf("no migration file available to corrupt in %q", migrationDir)
	return "", nil
}
