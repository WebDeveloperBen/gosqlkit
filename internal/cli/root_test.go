package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/app"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	migrateplan "github.com/webdeveloperben/gosqlkit/internal/migrate/plan"
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
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
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

func TestRunCISchemaPrintsJSON(t *testing.T) {
	root := filepath.Join(mustModuleDir(t), "examples", "basic")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code, err := run([]string{"--root", root, "ci", "schema", "--json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run(ci schema --json) returned error: %v\nstderr=%s", err, stderr.String())
	}
	if code != 0 {
		t.Fatalf("run(ci schema --json) returned code %d, want 0", code)
	}
	for _, want := range []string{`"generate":`, `"snapshot":`, `"migration":`, `"checked": true`} {
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

func TestPrintCISchemaRendersTable(t *testing.T) {
	var stdout bytes.Buffer
	err := printCISchema(&stdout, &app.CISchemaResult{
		Generate:  &app.GenerateResult{Out: "db/schema.generated.sql", Checked: true},
		Snapshot:  &app.SnapshotResult{Out: "db/schema.snapshot.json", Checked: true},
		Migration: &app.MigrateCheckResult{Dir: "db/migrations", Count: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, want := range []string{"Check", "Status", "generated SQL", "snapshot JSON", "migrations", "ok", "2"} {
		if !strings.Contains(output, want) {
			t.Fatalf("ci schema output missing %q\n%s", want, output)
		}
	}
}

func TestPrintCIDatabaseRendersTable(t *testing.T) {
	var stdout bytes.Buffer
	err := printCIDatabase(&stdout, &app.CIDatabaseResult{
		Drift: &app.DriftCheckResult{DatabaseSnapshotID: "database-id"},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, want := range []string{"Check", "Status", "database drift", "ok", "database-id"} {
		if !strings.Contains(output, want) {
			t.Fatalf("ci database output missing %q\n%s", want, output)
		}
	}
}

func TestPrintDriftGroupsDifferences(t *testing.T) {
	var stdout bytes.Buffer
	err := printDrift(&stdout, &app.DriftCheckResult{
		Drift: true,
		Differences: []app.DriftDifference{
			{Op: "extra", Object: app.DriftObjectRef{Kind: "column", Key: "public.users.legacy_email"}},
			{Op: "missing", Object: app.DriftObjectRef{Kind: "index", Key: "public.users.users_email_idx"}},
			{Op: "changed", Object: app.DriftObjectRef{Kind: "table", Key: "public.users"}, Fields: []string{"comment", "rowLevelSecurity"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, want := range []string{
		"database schema drift detected",
		"Status",
		"Kind",
		"Object",
		"Fields",
		"extra",
		"column",
		"public.users.legacy_email",
		"missing",
		"index",
		"public.users.users_email_idx",
		"changed",
		"table",
		"public.users",
		"comment, rowLevelSecurity",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("drift output missing %q\n%s", want, output)
		}
	}
}

func TestPrintPlanRendersStyledTables(t *testing.T) {
	var stdout bytes.Buffer
	err := printPlan(&stdout, &app.MigratePlanResult{
		Dialect:        "postgresql",
		FromSnapshotID: "from-id",
		ToSnapshotID:   "to-id",
		Changes: []migrate.Change{
			{
				Op:      "alter",
				Object:  migrate.ObjectRef{Kind: "column", Key: "public.users.email"},
				Risks:   []string{"lock-heavy"},
				Summary: "add users email column",
			},
			{
				Op:      "drop",
				Object:  migrate.ObjectRef{Kind: "table", Key: "public.legacy_users"},
				Risks:   []string{"destructive", "data-loss"},
				Summary: "drop legacy users",
			},
		},
		Statements: []migrate.Statement{{SQL: "ALTER TABLE users ADD COLUMN email text;"}},
		Destructive: []migrateplan.Change{
			migrateplan.NewChange(
				migrateplan.OperationDrop,
				migrateplan.Ref(migrateplan.ObjectKindTable, "public.legacy_users"),
				"drop legacy users",
			).WithRisks(migrateplan.RiskDestructive),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, want := range []string{
		"migration plan",
		"Field",
		"Value",
		"Dialect",
		"postgresql",
		"From snapshot",
		"from-id",
		"To snapshot",
		"to-id",
		"Changes",
		"2",
		"Up statements",
		"1",
		"Destructive",
		"pass --allow-destructive",
		"Op",
		"Kind",
		"Object",
		"Risks",
		"Summary",
		"alter",
		"column",
		"public.users.email",
		"lock-heavy",
		"drop legacy users",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("plan output missing %q\n%s", want, output)
		}
	}
}

func TestRunMigrateCreateRejectsConflictingInteractionFlags(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gosqlkit.yaml"), []byte(`version: "1"
dialect: postgresql
schema: "schema"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	code, err := run([]string{"--root", root, "migrate", "create", "add-users", "--interactive", "--no-interactive"}, nil, &stderr)
	if code != 1 {
		t.Fatalf("run(migrate create) returned code %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "--interactive and --no-interactive cannot be used together") {
		t.Fatalf("expected conflicting interaction flags error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "gosqlkit: error:") {
		t.Fatalf("expected error output, got %q", stderr.String())
	}
}

func TestPromptRenameCandidatesSelectsAcceptedRenames(t *testing.T) {
	stdin := strings.NewReader("2\n1\n")
	var stdout bytes.Buffer
	decide := promptRenameCandidates(stdin, &stdout)

	decisions, err := decide([]app.RenameCandidate{
		{Kind: "table", FromKey: "public.old_users", ToKey: "public.users", ParentKey: ""},
		{Kind: "column", FromKey: "public.users.old_email", ToKey: "public.users.email", ParentKey: "public.users"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 2 {
		t.Fatalf("decisions = %#v", decisions)
	}
	if !decisions[0].Accept || decisions[1].Accept {
		t.Fatalf("unexpected decisions %#v", decisions)
	}
	output := stdout.String()
	for _, want := range []string{
		"ambiguous rename candidate",
		"public.old_users",
		"public.users.email",
		"Is public.users table created",
		"Select action for public.users",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("prompt output missing %q\n%s", want, output)
		}
	}
}

func TestPromptRenameCandidatesConsumesSelectedSource(t *testing.T) {
	stdin := strings.NewReader("2\n")
	var stdout bytes.Buffer
	decide := promptRenameCandidates(stdin, &stdout)

	decisions, err := decide([]app.RenameCandidate{
		{Kind: "table", FromKey: "public.old_users", ToKey: "public.users"},
		{Kind: "table", FromKey: "public.old_users", ToKey: "public.accounts"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 || !decisions[0].Accept || decisions[0].Candidate.ToKey != "public.users" {
		t.Fatalf("unexpected decisions %#v", decisions)
	}
	if strings.Contains(stdout.String(), "public.accounts") {
		t.Fatalf("selected source should not be offered for a later target\n%s", stdout.String())
	}
}

func TestParseRenameDecisionSelectionSupportsCreate(t *testing.T) {
	for _, input := range []string{"", "1", "create", "none"} {
		got, err := parseRenameDecisionSelection(input, 3)
		if err != nil {
			t.Fatalf("parseRenameDecisionSelection(%q) error: %v", input, err)
		}
		if got != 0 {
			t.Fatalf("parseRenameDecisionSelection(%q) = %d, want 0", input, got)
		}
	}
	got, err := parseRenameDecisionSelection("2", 3)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("parseRenameDecisionSelection(2) = %d, want 1", got)
	}
}

func TestParseRenameDecisionSelectionRejectsInvalidInput(t *testing.T) {
	if _, err := parseRenameDecisionSelection("4", 3); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected out of range error, got %v", err)
	}
	if _, err := parseRenameDecisionSelection("nope", 3); err == nil || !strings.Contains(err.Error(), "invalid rename selection") {
		t.Fatalf("expected invalid selection error, got %v", err)
	}
}

func TestShouldPromptForRenamesHonoursModeAndTTY(t *testing.T) {
	if !shouldPromptForRenames(app.InteractionAlways, strings.NewReader("all"), &bytes.Buffer{}) {
		t.Fatal("explicit interactive mode should prompt")
	}
	if shouldPromptForRenames(app.InteractionNever, os.Stdin, os.Stdout) {
		t.Fatal("explicit non-interactive mode should not prompt")
	}
	if shouldPromptForRenames(app.InteractionAuto, strings.NewReader("all"), &bytes.Buffer{}) {
		t.Fatal("auto mode should not prompt for non-TTY IO")
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
