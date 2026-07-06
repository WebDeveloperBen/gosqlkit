package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerate(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))

	var stdout bytes.Buffer
	sql, err := generateFromPackage("github.com/webdeveloperben/pgkit/examples/basic/schema", mustModuleDir(t))
	if err != nil {
		t.Fatal(err)
	}
	stdout.WriteString(sql)

	want, err := os.ReadFile("../../examples/basic/db/schema.generated.sql")
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != string(want) {
		t.Fatalf("generated SQL mismatch\n--- got ---\n%s\n--- want ---\n%s", stdout.String(), want)
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
