package tooling

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenEnablesForeignKeysOnFileAndMemoryConnections(t *testing.T) {
	for _, test := range []struct {
		name string
		url  string
	}{
		{name: "file", url: filepath.Join(t.TempDir(), "schema.db")},
		{name: "memory", url: ":memory:"},
	} {
		t.Run(test.name, func(t *testing.T) {
			conn, err := Open(context.Background(), test.url)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := conn.Close(); err != nil {
					t.Error(err)
				}
			}()
			if _, err := conn.Exec(context.Background(), `CREATE TABLE parent (id INTEGER PRIMARY KEY);
CREATE TABLE child (parent_id INTEGER REFERENCES parent(id));`); err != nil {
				t.Fatal(err)
			}
			if _, err := conn.Exec(context.Background(), "INSERT INTO child (parent_id) VALUES (1)"); err == nil {
				t.Fatal("insert with a missing referenced row succeeded")
			}
		})
	}
}

func TestOpenSQLiteURLFileCanBeReopened(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.db")
	conn, err := Open(context.Background(), "sqlite://"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(context.Background(), "CREATE TABLE records (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	}()
	var count int
	if err := reopened.QueryRow(context.Background(), "SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'records'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("records table count = %d, want 1", count)
	}
}

func TestOpenRejectsRemoteSQLiteURL(t *testing.T) {
	_, err := Open(context.Background(), "sqlite://remote.example/schema.db")
	if err == nil {
		t.Fatal("expected remote SQLite URL to be rejected")
	}
}
