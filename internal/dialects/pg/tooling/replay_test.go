package tooling_test

import (
	"context"
	"fmt"
	"slices"
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
	rows         *fakeRows
	failSQL      string
	statements   []string
	transactions [][]string
	rollbacks    int
}

func (f *fakeDatabase) Exec(_ context.Context, sql string, args ...any) error {
	sql = formatFakeSQL(sql, args)
	f.statements = append(f.statements, sql)
	if f.failSQL != "" && strings.Contains(sql, f.failSQL) {
		return fmt.Errorf("fake execution failure: %s", f.failSQL)
	}
	return nil
}

func (f *fakeDatabase) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return f.rows, nil
}

func (f *fakeDatabase) Begin(context.Context) (pgx.Tx, error) {
	return &fakeTransaction{database: f}, nil
}

func formatFakeSQL(sql string, args []any) string {
	if len(args) > 0 {
		return fmt.Sprintf("%s %v", sql, args)
	}
	return sql
}

type fakeTransaction struct {
	pgx.Tx
	database   *fakeDatabase
	statements []string
}

func (f *fakeTransaction) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	sql = formatFakeSQL(sql, args)
	f.statements = append(f.statements, sql)
	if f.database.failSQL != "" && strings.Contains(sql, f.database.failSQL) {
		return pgconn.CommandTag{}, fmt.Errorf("fake execution failure: %s", f.database.failSQL)
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeTransaction) Commit(context.Context) error {
	f.database.transactions = append(f.database.transactions, append([]string(nil), f.statements...))
	f.database.statements = append(f.database.statements, f.statements...)
	return nil
}

func (f *fakeTransaction) Rollback(context.Context) error {
	f.database.rollbacks++
	return nil
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

func TestApplyRejectsUnclassifiableOpaqueSQLBeforeMigrationExecution(t *testing.T) {
	for _, tc := range []struct {
		name    string
		runner  string
		file    string
		content string
	}{
		{
			name:    "goose",
			runner:  migrate.RunnerGoose,
			file:    "20260706143200_opaque.sql",
			content: "-- +goose Up\nDO $$ BEGIN PERFORM 1; END $$;\n",
		},
		{
			name:    "golang-migrate",
			runner:  migrate.RunnerGolangMigrate,
			file:    "20260706143201_opaque.up.sql",
			content: "DO $$ BEGIN PERFORM 1; END $$;\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &fakeDatabase{rows: &fakeRows{}}
			migration := migrate.Migration{
				Name:    tc.file,
				Path:    "db/migrations/" + tc.file,
				Content: tc.content,
				Metadata: migrate.Metadata{Changes: []migrate.Change{{
					Object: migrate.ObjectRef{Kind: "raw_sql", Key: "opaque"},
				}}},
			}
			_, err := pgtooling.Apply(context.Background(), db, pgtooling.ApplyOptions{
				Runner: tc.runner, Migrations: []migrate.Migration{migration},
			})
			if err == nil || !strings.Contains(err.Error(), "cannot establish transaction safety") {
				t.Fatalf("Apply error = %v, want opaque SQL safety error", err)
			}
			for _, statement := range db.statements {
				if strings.Contains(statement, "DO $$") {
					t.Fatalf("unclassifiable migration SQL executed: %q", statement)
				}
				if tc.runner == migrate.RunnerGolangMigrate && strings.Contains(statement, "INSERT INTO schema_migrations") {
					t.Fatalf("golang-migrate state changed before SQL safety validation: %q", statement)
				}
			}
		})
	}
}

func TestApplyCommitsTransactionalMigrationAndVersionStateTogether(t *testing.T) {
	for _, tc := range []struct {
		name    string
		runner  string
		file    string
		content string
	}{
		{
			name:    "goose",
			runner:  migrate.RunnerGoose,
			file:    "20260706143300_atomic.sql",
			content: "-- +goose Up\nCREATE TABLE atomic_users (id integer);\nINSERT INTO atomic_users VALUES (1);\n",
		},
		{
			name:    "golang-migrate",
			runner:  migrate.RunnerGolangMigrate,
			file:    "20260706143301_atomic.up.sql",
			content: "CREATE TABLE atomic_users (id integer);\nINSERT INTO atomic_users VALUES (1);\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &fakeDatabase{rows: &fakeRows{}}
			_, err := pgtooling.Apply(context.Background(), db, pgtooling.ApplyOptions{
				Runner: tc.runner,
				Migrations: []migrate.Migration{{
					Name: tc.file, Path: tc.file, Content: tc.content,
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			var migrationTransaction []string
			for _, transaction := range db.transactions {
				if slices.Contains(transaction, "CREATE TABLE atomic_users (id integer);") {
					migrationTransaction = transaction
					break
				}
			}
			if len(migrationTransaction) == 0 || !slices.Contains(migrationTransaction, "INSERT INTO atomic_users VALUES (1);") {
				t.Fatalf("migration statements were not committed together: %#v", db.transactions)
			}
			statePrefix := "INSERT INTO goose_db_version"
			if tc.runner == migrate.RunnerGolangMigrate {
				statePrefix = "INSERT INTO schema_migrations"
			}
			stateCommitted := false
			for _, statement := range migrationTransaction {
				if strings.HasPrefix(statement, statePrefix) {
					stateCommitted = true
					break
				}
			}
			if !stateCommitted {
				t.Fatalf("success runner state was not in the migration transaction: %#v", migrationTransaction)
			}
		})
	}
}

func TestApplyRollsBackTransactionalMigrationAndPreservesRunnerState(t *testing.T) {
	for _, tc := range []struct {
		name    string
		runner  string
		file    string
		content string
	}{
		{
			name:    "goose",
			runner:  migrate.RunnerGoose,
			file:    "20260706143400_failure.sql",
			content: "-- +goose Up\nCREATE TABLE rolled_back_users (id integer);\nINSERT INTO missing_table VALUES (1);\n",
		},
		{
			name:    "golang-migrate",
			runner:  migrate.RunnerGolangMigrate,
			file:    "20260706143401_failure.up.sql",
			content: "CREATE TABLE rolled_back_users (id integer);\nINSERT INTO missing_table VALUES (1);\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &fakeDatabase{rows: &fakeRows{}, failSQL: "INSERT INTO missing_table"}
			_, err := pgtooling.Apply(context.Background(), db, pgtooling.ApplyOptions{
				Runner: tc.runner,
				Migrations: []migrate.Migration{{
					Name: tc.file, Path: tc.file, Content: tc.content,
				}},
			})
			if err == nil || !strings.Contains(err.Error(), tc.file) {
				t.Fatalf("Apply error = %v, want migration-qualified failure", err)
			}
			if db.rollbacks != 1 {
				t.Fatalf("transaction rollbacks = %d, want 1", db.rollbacks)
			}
			for _, statement := range db.statements {
				if strings.Contains(statement, "CREATE TABLE rolled_back_users") || strings.Contains(statement, "INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, true)") {
					t.Fatalf("rolled-back migration or success state was committed: %q", statement)
				}
			}
			if tc.runner == migrate.RunnerGolangMigrate {
				if len(db.transactions) != 1 || !slices.Contains(db.transactions[0], "INSERT INTO schema_migrations (version, dirty) VALUES ($1, $2); [20260706143401 true]") {
					t.Fatalf("dirty recovery state was not retained outside the failed transaction: %#v", db.transactions)
				}
			}
		})
	}
}

func TestApplyLeavesRecoveryStateForFailedNonTransactionalMigrations(t *testing.T) {
	for _, tc := range []struct {
		name    string
		runner  string
		file    string
		content string
	}{
		{
			name:   "goose",
			runner: migrate.RunnerGoose,
			file:   "20260706143500_concurrent_index.sql",
			content: "-- +goose NO TRANSACTION\n-- +goose Up\n" +
				"CREATE INDEX CONCURRENTLY users_email_idx ON users (email);\n" +
				"INSERT INTO unavailable_table VALUES (1);\n",
		},
		{
			name:   "golang-migrate",
			runner: migrate.RunnerGolangMigrate,
			file:   "20260706143501_concurrent_index.up.sql",
			content: "CREATE INDEX CONCURRENTLY users_email_idx ON users (email);\n" +
				"INSERT INTO unavailable_table VALUES (1);\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &fakeDatabase{rows: &fakeRows{}, failSQL: "INSERT INTO unavailable_table"}
			_, err := pgtooling.Apply(context.Background(), db, pgtooling.ApplyOptions{
				Runner: tc.runner,
				Migrations: []migrate.Migration{{
					Name: tc.file, Path: tc.file, Content: tc.content,
					Metadata: migrate.Metadata{Changes: []migrate.Change{{
						Risks: []string{"non-transactional"},
					}}},
				}},
			})
			if err == nil || strings.Contains(strings.ToLower(err.Error()), "rollback") {
				t.Fatalf("Apply error = %v, want explicit non-transactional failure", err)
			}
			if !slices.Contains(db.statements, "CREATE INDEX CONCURRENTLY users_email_idx ON users (email);") {
				t.Fatalf("non-transactional DDL was not executed directly: %#v", db.statements)
			}
			for _, transaction := range db.transactions {
				if slices.Contains(transaction, "CREATE INDEX CONCURRENTLY users_email_idx ON users (email);") {
					t.Fatalf("non-transactional DDL ran in transaction: %#v", transaction)
				}
			}
			switch tc.runner {
			case migrate.RunnerGoose:
				if !slices.Contains(db.statements, "INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, false); [20260706143500]") {
					t.Fatalf("failed migration lacks Goose recovery row: %#v", db.statements)
				}
				for _, statement := range db.statements {
					if strings.Contains(statement, "UPDATE goose_db_version SET is_applied = true") {
						t.Fatalf("failed migration was marked applied: %q", statement)
					}
				}
			case migrate.RunnerGolangMigrate:
				if len(db.transactions) != 1 || !slices.Contains(db.transactions[0], "INSERT INTO schema_migrations (version, dirty) VALUES ($1, $2); [20260706143501 true]") {
					t.Fatalf("failed migration lacks golang-migrate dirty state: %#v", db.transactions)
				}
			}
		})
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
