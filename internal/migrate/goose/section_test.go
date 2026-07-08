package goose_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/migrate/goose"
)

func TestUpSQLExtractsGooseUpSection(t *testing.T) {
	got, err := goose.UpSQL(`-- +gosqlkit Meta
-- {}

-- +goose Up
CREATE TABLE users (id uuid);

-- +goose Down
DROP TABLE users;
`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "CREATE TABLE users (id uuid);" {
		t.Fatalf("up SQL = %q", got)
	}
}

func TestUpSQLRejectsMissingGooseUp(t *testing.T) {
	_, err := goose.UpSQL("-- +goose Down\nSELECT 1;")
	if err == nil || !strings.Contains(err.Error(), "missing goose up annotation") {
		t.Fatalf("expected missing up error, got %v", err)
	}
}

func TestDownSQLExtractsGooseDownSection(t *testing.T) {
	got, err := goose.DownSQL(`-- +goose Up
CREATE TABLE users (id uuid);

-- +goose Down
DROP TABLE users;
`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "DROP TABLE users;" {
		t.Fatalf("down SQL = %q", got)
	}
}

func TestUpStatementsHonoursStatementBeginEnd(t *testing.T) {
	got, err := goose.UpStatements(`-- +goose Up
CREATE TABLE users (id integer);

-- +goose StatementBegin
CREATE FUNCTION touch_users() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE INDEX CONCURRENTLY users_id_idx ON users (id);

-- +goose Down
DROP INDEX users_id_idx;
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("statements = %#v", got)
	}
	if !strings.Contains(got[1], "NEW.updated_at = now();") {
		t.Fatalf("function body was split or dropped: %#v", got)
	}
	if !strings.Contains(got[2], "CONCURRENTLY") {
		t.Fatalf("expected concurrent index statement: %#v", got)
	}
}

func TestDownStatementsHonoursStatementBeginEnd(t *testing.T) {
	got, err := goose.DownStatements(`-- +goose Up
SELECT 1;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  RAISE NOTICE 'rollback; still one statement';
END;
$$;
-- +goose StatementEnd
SELECT 0;
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("statements = %#v", got)
	}
	if !strings.Contains(got[0], "rollback; still one statement") || got[1] != "SELECT 0;" {
		t.Fatalf("statements = %#v", got)
	}
}

func TestStatementsRejectsMismatchedStatementMarkers(t *testing.T) {
	_, err := goose.UpStatements(`-- +goose Up
-- +goose StatementBegin
SELECT 1;
`)
	if err == nil || !strings.Contains(err.Error(), "statement begin without statement end") {
		t.Fatalf("expected unclosed statement error, got %v", err)
	}

	_, err = goose.UpStatements(`-- +goose Up
-- +goose StatementEnd
SELECT 1;
`)
	if err == nil || !strings.Contains(err.Error(), "statement end without statement begin") {
		t.Fatalf("expected unopened statement error, got %v", err)
	}
}
