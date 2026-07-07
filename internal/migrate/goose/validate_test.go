package goose_test

import (
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/migrate/goose"
)

func TestValidateAcceptsGooseMigration(t *testing.T) {
	content := `-- +gosqlkit Meta
-- {}

-- +goose Up
SELECT 1;

-- +goose Down
SELECT 0;
`

	if err := goose.Validate(content); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsMissingUp(t *testing.T) {
	err := goose.Validate("-- +goose Down\n")
	if err == nil || !strings.Contains(err.Error(), "missing goose up annotation") {
		t.Fatalf("expected missing up error, got %v", err)
	}
}

func TestValidateRejectsDuplicateUp(t *testing.T) {
	err := goose.Validate("-- +goose Up\n-- +goose Up\n")
	if err == nil || !strings.Contains(err.Error(), "found 2 goose up annotations") {
		t.Fatalf("expected duplicate up error, got %v", err)
	}
}

func TestValidateRejectsDownBeforeUp(t *testing.T) {
	err := goose.Validate("-- +goose Down\n-- +goose Up\n")
	if err == nil || !strings.Contains(err.Error(), "must appear after") {
		t.Fatalf("expected order error, got %v", err)
	}
}
