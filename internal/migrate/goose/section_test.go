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
