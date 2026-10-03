package tooling

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/migrate"
)

func openMigrationTestDB(t *testing.T) *Conn {
	t.Helper()
	conn, err := Open(context.Background(), filepath.Join(t.TempDir(), "migrations.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close SQLite connection: %v", err)
		}
	})
	return conn
}

func migration(name, content string) migrate.Migration {
	return migrate.Migration{Name: name, Path: name, Content: content}
}

func TestReplaySupportsBothRunners(t *testing.T) {
	for _, tc := range []struct {
		name    string
		runner  string
		file    string
		content string
	}{
		{
			name:    "goose",
			runner:  migrate.RunnerGoose,
			file:    "20260101000001_goose.sql",
			content: "-- +goose Up\nCREATE TABLE goose_replayed (id INTEGER PRIMARY KEY);\nINSERT INTO goose_replayed VALUES (1);\n-- +goose Down\nDROP TABLE goose_replayed;\n",
		},
		{
			name:    "golang-migrate",
			runner:  migrate.RunnerGolangMigrate,
			file:    "20260101000002_golang.up.sql",
			content: "CREATE TABLE migrate_replayed (id INTEGER PRIMARY KEY);\nCREATE TABLE migrate_replayed_events (id INTEGER);\nCREATE TRIGGER migrate_replayed_trigger AFTER INSERT ON migrate_replayed FOR EACH ROW BEGIN INSERT INTO migrate_replayed_events (id) VALUES (NEW.id); END;\nINSERT INTO migrate_replayed VALUES (2);\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := openMigrationTestDB(t)
			got, err := Replay(context.Background(), conn, ReplayOptions{
				Runner: tc.runner, Migrations: []migrate.Migration{migration(tc.file, tc.content)},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.Applied != 1 || got.LastFile != tc.file {
				t.Fatalf("Replay result = %+v", got)
			}
			table := "goose_replayed"
			want := int64(1)
			if tc.runner == migrate.RunnerGolangMigrate {
				table, want = "migrate_replayed", 2
			}
			var value int64
			if err := conn.QueryRow(context.Background(), "SELECT id FROM "+table).Scan(&value); err != nil {
				t.Fatal(err)
			}
			if value != want {
				t.Fatalf("replayed value = %d, want %d", value, want)
			}
			if tc.runner == migrate.RunnerGolangMigrate {
				var events int
				if err := conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM migrate_replayed_events WHERE id = 2").Scan(&events); err != nil {
					t.Fatal(err)
				}
				if events != 1 {
					t.Fatalf("trigger event count = %d, want 1", events)
				}
			}
		})
	}
}

func TestApplySkipsRecordedGooseAndGolangMigrateVersions(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name      string
		runner    string
		version   string
		files     []migrate.Migration
		wantState int64
	}{
		{
			name:   "goose",
			runner: migrate.RunnerGoose,
			files: []migrate.Migration{
				migration("20260101000001_one.sql", "-- +goose Up\nCREATE TABLE goose_one (id INTEGER);\n"),
				migration("20260101000002_two.sql", "-- +goose Up\nCREATE TABLE goose_two (id INTEGER);\n"),
			},
			version:   "SELECT COUNT(*) FROM goose_db_version WHERE is_applied = 1",
			wantState: 2,
		},
		{
			name:   "golang-migrate",
			runner: migrate.RunnerGolangMigrate,
			files: []migrate.Migration{
				migration("20260101000001_one.up.sql", "CREATE TABLE migrate_one (id INTEGER);"),
				migration("20260101000002_two.up.sql", "CREATE TABLE migrate_two (id INTEGER);"),
			},
			version:   "SELECT version FROM schema_migrations WHERE dirty = 0",
			wantState: 20260101000002,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := openMigrationTestDB(t)
			first, err := Apply(ctx, conn, ApplyOptions{Runner: tc.runner, Migrations: tc.files[:1]})
			if err != nil {
				t.Fatal(err)
			}
			if first.Applied != 1 || first.Skipped != 0 || first.LastFile != tc.files[0].Name {
				t.Fatalf("first Apply result = %+v", first)
			}
			second, err := Apply(ctx, conn, ApplyOptions{Runner: tc.runner, Migrations: tc.files})
			if err != nil {
				t.Fatal(err)
			}
			if second.Applied != 1 || second.Skipped != 1 || second.LastFile != tc.files[1].Name {
				t.Fatalf("second Apply result = %+v", second)
			}
			var state int64
			if err := conn.QueryRow(ctx, tc.version).Scan(&state); err != nil {
				t.Fatal(err)
			}
			if state != tc.wantState {
				t.Fatalf("runner version state value = %d, want %d", state, tc.wantState)
			}
		})
	}
}

func TestReplayRunsForeignKeyRebuildPragmasOutsideTransactions(t *testing.T) {
	ctx := context.Background()
	conn := openMigrationTestDB(t)
	if _, err := conn.Exec(ctx, `CREATE TABLE parent (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `CREATE TABLE child (parent_id INTEGER REFERENCES parent(id))`); err != nil {
		t.Fatal(err)
	}
	file := migration("20260101000006_rebuild.sql", "-- +goose Up\nPRAGMA foreign_keys = OFF;\nINSERT INTO child (parent_id) VALUES (99);\nPRAGMA foreign_keys = ON;\n")
	if _, err := Replay(ctx, conn, ReplayOptions{Runner: migrate.RunnerGoose, Migrations: []migrate.Migration{file}}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := conn.QueryRow(ctx, "SELECT COUNT(*) FROM child WHERE parent_id = 99").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("rebuild insert count = %d, want 1", count)
	}
	var enabled int
	if err := conn.QueryRow(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys = %d after rebuild, want 1", enabled)
	}
}

func TestApplyFailurePreservesRunnerFailureState(t *testing.T) {
	ctx := context.Background()
	badGoose := migration("20260101000003_broken.sql", "-- +goose Up\nCREATE TABLE goose_partial (id INTEGER);\nTHIS IS NOT SQL;\n")
	badMigrate := migration("20260101000004_broken.up.sql", "CREATE TABLE migrate_partial (id INTEGER);\nTHIS IS NOT SQL;\n")
	for _, tc := range []struct {
		name        string
		runner      string
		stateQuery  string
		migration   migrate.Migration
		wantVersion int64
		wantDirty   int
	}{
		{name: "goose", runner: migrate.RunnerGoose, migration: badGoose, stateQuery: "SELECT COUNT(*) FROM goose_db_version"},
		{name: "golang-migrate", runner: migrate.RunnerGolangMigrate, migration: badMigrate, stateQuery: "SELECT version, dirty FROM schema_migrations", wantVersion: 20260101000004, wantDirty: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := openMigrationTestDB(t)
			_, err := Apply(ctx, conn, ApplyOptions{Runner: tc.runner, Migrations: []migrate.Migration{tc.migration}})
			if err == nil || !strings.Contains(err.Error(), tc.migration.Name) {
				t.Fatalf("Apply error = %v, want migration-qualified error", err)
			}
			if tc.runner == migrate.RunnerGoose {
				var rows int
				if err := conn.QueryRow(ctx, tc.stateQuery).Scan(&rows); err != nil {
					t.Fatal(err)
				}
				if rows != 0 {
					t.Fatalf("goose successful version records = %d, want 0", rows)
				}
				var tables int
				if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name='goose_partial'`).Scan(&tables); err != nil {
					t.Fatal(err)
				}
				if tables != 0 {
					t.Fatalf("failed goose migration left %d partial tables", tables)
				}
				return
			}
			var version int64
			var dirty int
			if err := conn.QueryRow(ctx, tc.stateQuery).Scan(&version, &dirty); err != nil {
				t.Fatal(err)
			}
			if version != tc.wantVersion || dirty != tc.wantDirty {
				t.Fatalf("golang-migrate state = (%d, %d), want (%d, %d)", version, dirty, tc.wantVersion, tc.wantDirty)
			}
		})
	}
}

func TestReplayRestoresForeignKeysAfterFailure(t *testing.T) {
	ctx := context.Background()
	conn := openMigrationTestDB(t)
	file := migration("20260101000005_rebuild.sql", "-- +goose Up\nPRAGMA foreign_keys = OFF;\nCREATE TABLE partial_rebuild (id INTEGER);\nTHIS IS NOT SQL;\nPRAGMA foreign_keys = ON;\n")
	_, err := Replay(ctx, conn, ReplayOptions{Runner: migrate.RunnerGoose, Migrations: []migrate.Migration{file}})
	if err == nil || !strings.Contains(err.Error(), file.Name) {
		t.Fatalf("Replay error = %v, want migration-qualified failure", err)
	}
	var enabled int
	if err := conn.QueryRow(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys = %d after failed migration, want 1", enabled)
	}
}
