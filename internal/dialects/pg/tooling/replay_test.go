package tooling_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	pgtooling "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/tooling"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
)

type fakeExecutor struct {
	err        error
	statements []string
}

func (f *fakeExecutor) Exec(_ context.Context, sql string, _ ...any) error {
	f.statements = append(f.statements, sql)
	return f.err
}

type fakeDatabase struct {
	rows       *fakeRows
	statements []string
}

func (f *fakeDatabase) Exec(_ context.Context, sql string, args ...any) error {
	if len(args) > 0 {
		sql = fmt.Sprintf("%s %v", sql, args)
	}
	f.statements = append(f.statements, sql)
	return nil
}

func (f *fakeDatabase) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return f.rows, nil
}

type fakeRows struct {
	err  error
	data []fakeVersionRow
	idx  int
}

type fakeVersionRow struct {
	version int64
	dirty   bool
}

func (f *fakeRows) Close() {}

func (f *fakeRows) Err() error {
	return f.err
}

func (f *fakeRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (f *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (f *fakeRows) Next() bool {
	if f.idx >= len(f.data) {
		return false
	}
	f.idx++
	return f.idx <= len(f.data)
}

func (f *fakeRows) Scan(dest ...any) error {
	row := f.data[f.idx-1]
	*dest[0].(*int64) = row.version
	*dest[1].(*bool) = row.dirty
	return nil
}

func (f *fakeRows) Values() ([]any, error) {
	return nil, nil
}

func (f *fakeRows) RawValues() [][]byte {
	return nil
}

func (f *fakeRows) Conn() *pgx.Conn {
	return nil
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

func TestReplayAppliesGolangMigrateUpFilesInOrder(t *testing.T) {
	exec := &fakeExecutor{}
	result, err := pgtooling.Replay(context.Background(), exec, pgtooling.ReplayOptions{
		Runner: "golang-migrate",
		Migrations: []migrate.Migration{
			{
				Name:    "20260706143000_init.up.sql",
				Path:    "db/migrations/20260706143000_init.up.sql",
				Content: "-- +gosqlkit Meta\n-- {}\n\nCREATE TABLE users (id uuid);",
			},
			{
				Name:    "20260706143100_add_email.up.sql",
				Path:    "db/migrations/20260706143100_add_email.up.sql",
				Content: "ALTER TABLE users ADD COLUMN email text;",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 2 || result.LastFile != "20260706143100_add_email.up.sql" {
		t.Fatalf("result = %#v", result)
	}
	got := strings.Join(exec.statements, "\n---\n")
	want := "CREATE TABLE users (id uuid);\n---\nALTER TABLE users ADD COLUMN email text;"
	if got != want {
		t.Fatalf("statements mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestApplyGolangMigrateTracksCurrentVersion(t *testing.T) {
	db := &fakeDatabase{rows: &fakeRows{}}
	result, err := pgtooling.Apply(context.Background(), db, pgtooling.ApplyOptions{
		Runner: "golang-migrate",
		Migrations: []migrate.Migration{
			{
				Name:    "20260706143000_init.up.sql",
				Path:    "db/migrations/20260706143000_init.up.sql",
				Content: "CREATE TABLE users (id uuid);",
			},
			{
				Name:    "20260706143100_add_email.up.sql",
				Path:    "db/migrations/20260706143100_add_email.up.sql",
				Content: "ALTER TABLE users ADD COLUMN email text;",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 2 || result.Skipped != 0 || result.LastFile != "20260706143100_add_email.up.sql" {
		t.Fatalf("result = %#v", result)
	}
	got := strings.Join(db.statements, "\n")
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS schema_migrations",
		"INSERT INTO schema_migrations (version, dirty) VALUES ($1, $2); [20260706143000 true]",
		"CREATE TABLE users (id uuid);",
		"INSERT INTO schema_migrations (version, dirty) VALUES ($1, $2); [20260706143000 false]",
		"INSERT INTO schema_migrations (version, dirty) VALUES ($1, $2); [20260706143100 true]",
		"ALTER TABLE users ADD COLUMN email text;",
		"INSERT INTO schema_migrations (version, dirty) VALUES ($1, $2); [20260706143100 false]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("statements missing %q\n%s", want, got)
		}
	}
}

func TestApplyGolangMigrateRejectsDirtyVersion(t *testing.T) {
	db := &fakeDatabase{rows: &fakeRows{data: []fakeVersionRow{{version: 20260706143000, dirty: true}}}}
	_, err := pgtooling.Apply(context.Background(), db, pgtooling.ApplyOptions{
		Runner: "golang-migrate",
		Migrations: []migrate.Migration{{
			Name:    "20260706143100_add_email.up.sql",
			Path:    "db/migrations/20260706143100_add_email.up.sql",
			Content: "ALTER TABLE users ADD COLUMN email text;",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "is dirty") {
		t.Fatalf("expected dirty version error, got %v", err)
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
