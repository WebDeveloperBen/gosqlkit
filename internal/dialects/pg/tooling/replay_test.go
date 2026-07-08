package tooling_test

import (
	"context"
	"strings"
	"testing"

	pgtooling "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/tooling"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
)

type fakeExecutor struct {
	err        error
	statements []string
}

func (f *fakeExecutor) Exec(_ context.Context, sql string) error {
	f.statements = append(f.statements, sql)
	return f.err
}

func TestReplayAppliesGooseUpSectionsInOrder(t *testing.T) {
	exec := &fakeExecutor{}
	result, err := pgtooling.Replay(context.Background(), exec, pgtooling.ReplayOptions{
		Runner: "goose",
		Migrations: []migrate.Migration{
			{
				Name:    "20260706143000_init.sql",
				Path:    "db/migrations/20260706143000_init.sql",
				Content: "-- +goose Up\nCREATE TABLE users (id uuid);\n-- +goose Down\nDROP TABLE users;",
			},
			{
				Name:    "20260706143100_add_email.sql",
				Path:    "db/migrations/20260706143100_add_email.sql",
				Content: "-- +goose Up\nALTER TABLE users ADD COLUMN email text;",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 2 || result.LastFile != "20260706143100_add_email.sql" {
		t.Fatalf("result = %#v", result)
	}
	got := strings.Join(exec.statements, "\n---\n")
	want := "CREATE TABLE users (id uuid);\n---\nALTER TABLE users ADD COLUMN email text;"
	if got != want {
		t.Fatalf("statements mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestRedactURLHidesCredentials(t *testing.T) {
	got := pgtooling.RedactURL("postgres://user:secret@localhost:5432/app?sslmode=require&token=abc")
	if strings.Contains(got, "secret") || strings.Contains(got, "abc") {
		t.Fatalf("URL was not redacted: %s", got)
	}
	if !strings.Contains(got, "user:xxxxx@") || !strings.Contains(got, "token=xxxxx") {
		t.Fatalf("URL redaction lost expected structure: %s", got)
	}
}
