package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/webdeveloperben/gosqlkit/internal/dialects/sqlite/sqliteschema"
)

const sqliteCLIBinaryEnv = "GOSQLKIT_SQLITE_BINARY"

type sqliteCLIProject struct {
	binary string
	root   string
}

type sqliteCLIResult struct {
	stderr   string
	stdout   string
	exitCode int
}

func requireSQLiteCLIBinary(t *testing.T) string {
	t.Helper()
	if os.Getenv("GOSQLKIT_SQLITE_INTEGRATION") != "1" {
		t.Skip("set GOSQLKIT_SQLITE_INTEGRATION=1 to run SQLite CLI integration tests")
	}
	binary := os.Getenv(sqliteCLIBinaryEnv)
	if binary == "" {
		t.Fatalf("%s must point to a built gosqlkit CLI binary", sqliteCLIBinaryEnv)
	}
	return binary
}

func newSQLiteCLIProject(t *testing.T, binary string) *sqliteCLIProject {
	t.Helper()
	root, err := os.MkdirTemp(mustModuleDir(t), "gosqlkit-sqlite-e2e-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove SQLite CLI fixture %q: %v", root, err)
			return
		}
		if _, err := os.Stat(root); err == nil {
			t.Errorf("SQLite CLI fixture remains after cleanup: %q", root)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("check SQLite CLI fixture cleanup %q: %v", root, err)
		}
	})
	return &sqliteCLIProject{binary: binary, root: root}
}

func (p *sqliteCLIProject) writeSchema(t *testing.T, name, source string) {
	t.Helper()
	directory := filepath.Join(p.root, name)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "schema.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (p *sqliteCLIProject) writeConfig(t *testing.T, schema, runner string) {
	t.Helper()
	config := fmt.Sprintf(`version: "1"
dialect: sqlite
schema: %q
out:
  sql: "db/schema.generated.sql"
  snapshot: "db/schema.snapshot.json"
migrations:
  dir: "db/migrations"
  runner: %q
`, schema, runner)
	if err := os.WriteFile(filepath.Join(p.root, "gosqlkit.yaml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (p *sqliteCLIProject) run(t *testing.T, args ...string) sqliteCLIResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, p.binary, args...) // #nosec G204 -- The integration harness executes the explicitly configured gosqlkit test binary with fixed test arguments.
	command.Dir = p.root
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	result := sqliteCLIResult{stdout: stdout.String(), stderr: stderr.String()}
	err := command.Run()
	result.stdout = stdout.String()
	result.stderr = stderr.String()
	if err == nil {
		return result
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.exitCode = exitError.ExitCode()
		return result
	}
	t.Fatalf("run gosqlkit %v: %v", args, err)
	return sqliteCLIResult{}
}

func TestSQLiteCLIIntegrationGenerateSnapshot(t *testing.T) {
	binary := requireSQLiteCLIBinary(t)
	project := newSQLiteCLIProject(t, binary)
	project.writeSchema(t, "schema", `package schema

import "github.com/webdeveloperben/gosqlkit/sqlite"

var Users = sqlite.Table(
	"users",
	sqlite.Integer("id").PrimaryKey(),
	sqlite.Text("email").NotNull(),
	sqlite.IndexOn("users_email_idx", sqlite.IndexColumn("email")).IfNotExists(),
).IfNotExists()

var UserEmails = sqlite.View("user_emails").
	As("SELECT email FROM users").
	IfNotExists()
`)
	project.writeConfig(t, "schema", "goose")

	for _, args := range [][]string{{"generate"}, {"snapshot"}} {
		result := project.run(t, args...)
		if result.exitCode != 0 {
			t.Fatalf("gosqlkit %v exited %d: %s", args, result.exitCode, result.stderr)
		}
	}

	sqlPath := filepath.Join(project.root, "db", "schema.generated.sql")
	snapshotPath := filepath.Join(project.root, "db", "schema.snapshot.json")
	// #nosec G304 -- The generated artifact path is beneath the harness's temporary project directory.
	generatedSQL, err := os.ReadFile(sqlPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, clause := range []string{
		"CREATE TABLE IF NOT EXISTS users",
		"CREATE INDEX IF NOT EXISTS users_email_idx",
		"CREATE VIEW IF NOT EXISTS user_emails",
	} {
		if !strings.Contains(string(generatedSQL), clause) {
			t.Fatalf("generated SQL missing %q:\n%s", clause, generatedSQL)
		}
	}

	// #nosec G304 -- The generated artifact path is beneath the harness's temporary project directory.
	snapshotJSON, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var document sqliteschema.Document
	if err := json.Unmarshal(snapshotJSON, &document); err != nil {
		t.Fatalf("parse generated SQLite snapshot: %v\n%s", err, snapshotJSON)
	}
	if document.Dialect != "sqlite" || document.SnapshotID == "" || len(document.Tables) != 1 || len(document.Tables[0].Indexes) != 1 || len(document.Views) != 1 {
		t.Fatalf("generated snapshot has unexpected shape: %#v", document)
	}
	if !document.Tables[0].IfNotExists || !document.Tables[0].Indexes[0].IfNotExists || !document.Views[0].IfNotExists {
		t.Fatalf("generated snapshot lost creation options: %#v", document)
	}

	for _, args := range [][]string{{"generate"}, {"snapshot"}} {
		result := project.run(t, args...)
		if result.exitCode != 0 {
			t.Fatalf("repeat gosqlkit %v exited %d: %s", args, result.exitCode, result.stderr)
		}
	}
	// #nosec G304 -- The generated artifact path is beneath the harness's temporary project directory.
	repeatedSQL, err := os.ReadFile(sqlPath)
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G304 -- The generated artifact path is beneath the harness's temporary project directory.
	repeatedSnapshot, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generatedSQL, repeatedSQL) || !bytes.Equal(snapshotJSON, repeatedSnapshot) {
		t.Fatal("repeated SQLite generation or snapshot output changed")
	}

	for _, args := range [][]string{{"generate", "--check"}, {"snapshot", "--check"}} {
		result := project.run(t, args...)
		if result.exitCode != 0 {
			t.Fatalf("current gosqlkit %v exited %d: %s", args, result.exitCode, result.stderr)
		}
	}

	staleSQL := append(bytes.Clone(generatedSQL), []byte("-- stale\n")...)
	// #nosec G703 -- The stale artifact path is beneath the harness's temporary project directory.
	if err := os.WriteFile(sqlPath, staleSQL, 0o600); err != nil {
		t.Fatal(err)
	}
	result := project.run(t, "generate", "--check")
	if result.exitCode != 1 || !strings.Contains(result.stderr, "is out of date") {
		t.Fatalf("stale generate --check = %#v, want exit 1 and stale-output diagnostic", result)
	}

	staleSnapshot := append(bytes.Clone(snapshotJSON), []byte("{}\n")...)
	// #nosec G703 -- The stale artifact path is beneath the harness's temporary project directory.
	if err := os.WriteFile(snapshotPath, staleSnapshot, 0o600); err != nil {
		t.Fatal(err)
	}
	result = project.run(t, "snapshot", "--check")
	if result.exitCode != 1 || !strings.Contains(result.stderr, "is out of date") {
		t.Fatalf("stale snapshot --check = %#v, want exit 1 and stale-output diagnostic", result)
	}
}
