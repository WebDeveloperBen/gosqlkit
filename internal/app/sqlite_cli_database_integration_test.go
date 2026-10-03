package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
	_ "modernc.org/sqlite"
)

func TestSQLiteCLIIntegrationInspectDriftAndCI(t *testing.T) {
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
	project.writeConfig(t, "schema", "goose")

	for _, args := range [][]string{{"generate"}, {"snapshot"}} {
		result := project.run(t, args...)
		if result.exitCode != 0 {
			t.Fatalf("gosqlkit %v exited %d: %s", args, result.exitCode, result.stderr)
		}
	}
	generatedSQL, err := os.ReadFile(filepath.Join(project.root, "db", "schema.generated.sql"))
	if err != nil {
		t.Fatal(err)
	}

	databasePath := filepath.Join(project.root, "db", "live.sqlite")
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if _, err := database.ExecContext(ctx, string(generatedSQL)); err != nil {
		cancel()
		_ = database.Close()
		t.Fatalf("execute generated SQLite schema: %v", err)
	}
	if _, err := database.ExecContext(ctx, "INSERT INTO users (id, email) VALUES (1, 'one@example.test')"); err != nil {
		cancel()
		_ = database.Close()
		t.Fatalf("seed SQLite database: %v", err)
	}
	cancel()
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	inspect := project.run(t, "inspect", "--url", databasePath)
	if inspect.exitCode != 0 {
		t.Fatalf("inspect exited %d: %s", inspect.exitCode, inspect.stderr)
	}
	var inspected sqliteschema.Document
	if err := json.Unmarshal([]byte(inspect.stdout), &inspected); err != nil {
		t.Fatalf("parse inspect output: %v\n%s", err, inspect.stdout)
	}
	if inspected.Dialect != "sqlite" || inspected.SnapshotID == "" || len(inspected.Tables) != 1 {
		t.Fatalf("inspect returned unexpected SQLite snapshot: %#v", inspected)
	}

	drift := project.run(t, "drift", "check", "--url", databasePath, "--json")
	if drift.exitCode != 0 {
		t.Fatalf("clean drift check exited %d: %s", drift.exitCode, drift.stderr)
	}
	var cleanDrift DriftCheckResult
	if err := json.Unmarshal([]byte(drift.stdout), &cleanDrift); err != nil {
		t.Fatalf("parse clean drift output: %v\n%s", err, drift.stdout)
	}
	if cleanDrift.Dialect != "sqlite" || cleanDrift.DesiredSnapshotID == "" || cleanDrift.DatabaseSnapshotID == "" || cleanDrift.Drift {
		t.Fatalf("clean drift result = %#v", cleanDrift)
	}

	ci := project.run(t, "ci", "database", "--url", databasePath, "--json")
	if ci.exitCode != 0 {
		t.Fatalf("clean CI database check exited %d: %s", ci.exitCode, ci.stderr)
	}
	var cleanCI CIDatabaseResult
	if err := json.Unmarshal([]byte(ci.stdout), &cleanCI); err != nil {
		t.Fatalf("parse clean CI database output: %v\n%s", err, ci.stdout)
	}
	if cleanCI.Drift == nil || cleanCI.Drift.Dialect != "sqlite" || cleanCI.Drift.Drift {
		t.Fatalf("clean CI database result = %#v", cleanCI)
	}

	database, err = sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	if _, err := database.Exec("ALTER TABLE users ADD COLUMN unexpected TEXT"); err != nil {
		_ = database.Close()
		t.Fatalf("introduce SQLite drift: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	drift = project.run(t, "drift", "check", "--url", databasePath, "--json")
	if drift.exitCode != 1 {
		t.Fatalf("drifted CLI check exited %d, want 1: stdout=%s stderr=%s", drift.exitCode, drift.stdout, drift.stderr)
	}
	var changedDrift DriftCheckResult
	if err := json.Unmarshal([]byte(drift.stdout), &changedDrift); err != nil {
		t.Fatalf("parse drifted output: %v\n%s", err, drift.stdout)
	}
	if !changedDrift.Drift || !hasSQLiteColumnDrift(changedDrift.Differences, "users.unexpected") {
		t.Fatalf("drifted result did not identify users.unexpected: %#v", changedDrift)
	}

	ci = project.run(t, "ci", "database", "--url", databasePath, "--json")
	if ci.exitCode != 1 {
		t.Fatalf("drifted CI database check exited %d, want 1: stdout=%s stderr=%s", ci.exitCode, ci.stdout, ci.stderr)
	}
	var changedCI CIDatabaseResult
	if err := json.Unmarshal([]byte(ci.stdout), &changedCI); err != nil {
		t.Fatalf("parse drifted CI database output: %v\n%s", err, ci.stdout)
	}
	if changedCI.Drift == nil || !changedCI.Drift.Drift || !hasSQLiteColumnDrift(changedCI.Drift.Differences, "users.unexpected") {
		t.Fatalf("drifted CI result did not identify users.unexpected: %#v", changedCI)
	}
}

func hasSQLiteColumnDrift(differences []DriftDifference, key string) bool {
	for _, difference := range differences {
		if difference.Op == "extra" && difference.Object.Kind == "column" && difference.Object.Key == key {
			return true
		}
	}
	return false
}
