package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runWithRecover(t *testing.T, args []string) (code int, err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			if ep, ok := r.(*ExitPanic); ok {
				code = ep.Code
				return
			}
			t.Fatalf("unexpected panic in Run: %v", r)
		}
	}()
	code, err = Run(args)
	return code, err
}

func TestRunNoArgsPrintsHelp(t *testing.T) {
	code, err := runWithRecover(t, nil)
	if err != nil {
		t.Fatalf("Run(nil) returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("Run(nil) returned code %d, want 0", code)
	}
}

func TestRunVersionExitsZero(t *testing.T) {
	code, err := runWithRecover(t, []string{"version"})
	if err != nil {
		t.Fatalf("Run(version) returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("Run(version) returned code %d, want 0", code)
	}
}

func TestRunUnknownCommandExitsTwo(t *testing.T) {
	code, _ := runWithRecover(t, []string{"unknown-cmd"})
	if code != 2 {
		t.Fatalf("Run(unknown-cmd) returned code %d, want 2", code)
	}
}

func TestRunGenerateCheckRequiresOut(t *testing.T) {
	var stderr bytes.Buffer
	code, err := run([]string{"generate", "--check", "./examples/basic/schema"}, nil, &stderr)
	if code != 1 {
		t.Fatalf("run(generate --check) returned code %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "--check requires --out") {
		t.Fatalf("expected --check error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "gosqlkit: error:") {
		t.Fatalf("expected error output, got %q", stderr.String())
	}
}

func TestRunInspectRequiresURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gosqlkit.yaml"), []byte(`version: "1"
dialect: postgres
schema: "schema"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code, err := run([]string{"--root", root, "inspect"}, nil, &stderr)
	if code != 1 {
		t.Fatalf("run(inspect) returned code %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "database URL is required") {
		t.Fatalf("expected database URL error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "gosqlkit: error:") {
		t.Fatalf("expected error output, got %q", stderr.String())
	}
}

func TestRunMigrateCreateReportsMissingSchemaPackage(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gosqlkit.yaml"), []byte(`version: "1"
schema: "schema"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code, err := run([]string{"--root", root, "migrate", "create", "add-users"}, nil, &stderr)
	if code != 1 {
		t.Fatalf("run(migrate create) returned code %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), `go list package`) {
		t.Fatalf("expected go list error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "gosqlkit: error:") {
		t.Fatalf("expected error output, got %q", stderr.String())
	}
}

func TestRunMigrateCheckReportsInvalidMigration(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gosqlkit.yaml"), []byte(`version: "1"
schema: "schema"
migrations:
  dir: "db/migrations"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	migrationDir := filepath.Join(root, "db", "migrations")
	if err := os.MkdirAll(migrationDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationDir, "20260706143000_add_users.sql"), []byte("-- +goose Up\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code, err := run([]string{"--root", root, "migrate", "check"}, nil, &stderr)
	if code != 1 {
		t.Fatalf("run(migrate check) returned code %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "missing gosqlkit metadata block") {
		t.Fatalf("expected metadata error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "gosqlkit: error:") {
		t.Fatalf("expected error output, got %q", stderr.String())
	}
}

func TestRunMigrateCheckReportsMissingGooseUp(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gosqlkit.yaml"), []byte(`version: "1"
schema: "schema"
migrations:
  dir: "db/migrations"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	migrationDir := filepath.Join(root, "db", "migrations")
	if err := os.MkdirAll(migrationDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `-- +gosqlkit Meta
-- {
--   "version": 1,
--   "dialect": "postgresql",
--   "createdAt": "2026-07-06T14:30:00Z",
--   "changes": []
-- }
`
	if err := os.WriteFile(filepath.Join(migrationDir, "20260706143000_add_users.sql"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code, err := run([]string{"--root", root, "migrate", "check"}, nil, &stderr)
	if code != 1 {
		t.Fatalf("run(migrate check) returned code %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "missing goose up annotation") {
		t.Fatalf("expected goose annotation error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "gosqlkit: error:") {
		t.Fatalf("expected error output, got %q", stderr.String())
	}
}

func TestRunMigrateCheckPrintsJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gosqlkit.yaml"), []byte(`version: "1"
schema: "schema"
migrations:
  dir: "db/migrations"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	migrationDir := filepath.Join(root, "db", "migrations")
	if err := os.MkdirAll(migrationDir, 0o750); err != nil {
		t.Fatal(err)
	}
	content := `-- +gosqlkit Meta
-- {
--   "version": 1,
--   "dialect": "postgresql",
--   "createdAt": "2026-07-06T14:30:00Z",
--   "changes": []
-- }
-- +goose Up
-- +goose Down
`
	if err := os.WriteFile(filepath.Join(migrationDir, "20260706143000_add_users.sql"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code, err := run([]string{"--root", root, "migrate", "check", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run(migrate check --json) returned error: %v\nstderr=%s", err, stderr.String())
	}
	if code != 0 {
		t.Fatalf("run(migrate check --json) returned code %d, want 0", code)
	}
	for _, want := range []string{`"dir":`, `"count": 1`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q\n%s", want, stdout.String())
		}
	}
}

func TestPrintHumanHonoursQuietAndJSON(t *testing.T) {
	tests := []struct {
		name  string
		json  bool
		quiet bool
		want  bool
	}{
		{name: "human", want: true},
		{name: "quiet", quiet: true},
		{name: "json", json: true},
		{name: "json quiet", json: true, quiet: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := printHuman(tt.json, tt.quiet); got != tt.want {
				t.Fatalf("printHuman(%v, %v) = %v, want %v", tt.json, tt.quiet, got, tt.want)
			}
		})
	}
}

func TestExitErrorWraps(t *testing.T) {
	inner := errors.New("boom")
	got := Exit(7, inner)

	var exitErr *ExitError
	if !errors.As(got, &exitErr) || exitErr.Code != 7 {
		t.Errorf("Exit(7, err) lost code: got %T, %v", got, got)
	}
}
