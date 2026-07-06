package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateWritesToStdout(t *testing.T) {
	var stdout bytes.Buffer
	_, err := Generate(GenerateOptions{
		Package: "./examples/basic/schema",
		Root:    mustModuleDir(t),
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile("../../examples/basic/db/schema.generated.sql")
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != string(want) {
		t.Fatalf("generated SQL mismatch\n--- got ---\n%s\n--- want ---\n%s", stdout.String(), want)
	}
}

func TestGenerateAcceptsPostgresAlias(t *testing.T) {
	var stdout bytes.Buffer
	_, err := Generate(GenerateOptions{
		Package: "./examples/basic/schema",
		Dialect: "postgresql",
		Root:    mustModuleDir(t),
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "CREATE TABLE users") {
		t.Fatalf("expected generated PostgreSQL SQL, got %q", stdout.String())
	}
}

func TestGenerateWritesAndChecksOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db", "schema.sql")
	content := "CREATE TABLE users (id uuid PRIMARY KEY);\n"

	if err := writeOutput(path, content); err != nil {
		t.Fatal(err)
	}
	if err := checkOutput(path, content); err != nil {
		t.Fatal(err)
	}

	err := checkOutput(path, "different\n")
	if err == nil || !strings.Contains(err.Error(), "is out of date") {
		t.Fatalf("expected out of date error, got %v", err)
	}
}

func TestGenerateRejectsUnknownDialect(t *testing.T) {
	var stdout bytes.Buffer
	_, err := Generate(GenerateOptions{
		Package: "./examples/basic/schema",
		Dialect: "sqlite",
		Root:    mustModuleDir(t),
		Stdout:  &stdout,
	})
	if err == nil || !strings.Contains(err.Error(), `unknown dialect "sqlite"`) {
		t.Fatalf("expected unknown dialect error, got %v", err)
	}
}

func mustModuleDir(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod")
		}
		dir = parent
	}
}
