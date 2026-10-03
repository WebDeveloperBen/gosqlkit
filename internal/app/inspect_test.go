package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
)

func TestInspectPrintsSnapshotJSON(t *testing.T) {
	var stdout bytes.Buffer
	result, err := InspectWithConfig(inspectTestConfig(t, "postgres"), InspectOptions{
		URL:    "postgres://example",
		Stdout: &stdout,
		inspectPG: func(_ context.Context, databaseURL string) (pgschema.Schema, error) {
			if databaseURL != "postgres://example" {
				t.Fatalf("databaseURL = %q", databaseURL)
			}
			return inspectTestSchema(), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Dialect != "postgresql" || result.SnapshotID == "" {
		t.Fatalf("result = %#v", result)
	}

	var doc pgschema.Document
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("parse inspect output: %v\n%s", err, stdout.String())
	}
	if doc.SnapshotID != result.SnapshotID {
		t.Fatalf("snapshot id = %q, result = %q", doc.SnapshotID, result.SnapshotID)
	}
	if len(doc.Tables) != 1 || doc.Tables[0].Name != "users" {
		t.Fatalf("tables = %#v", doc.Tables)
	}
}

func TestInspectWritesOutputRelativeToRoot(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	result, err := InspectWithConfig(inspectTestConfigWithRoot(root, "postgres"), InspectOptions{
		URL:    "postgres://example",
		Out:    "db/inspect.snapshot.json",
		Stdout: &stdout,
		inspectPG: func(context.Context, string) (pgschema.Schema, error) {
			return inspectTestSchema(), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	wantOut := filepath.Join(root, "db", "inspect.snapshot.json")
	if result == nil || result.Out != wantOut {
		t.Fatalf("result out = %#v, want %q", result, wantOut)
	}
	// #nosec G304 -- test reads the inspect output path written by the code under test.
	data, err := os.ReadFile(wantOut)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"snapshotId":`) {
		t.Fatalf("inspect output missing snapshot id:\n%s", data)
	}
}

func TestInspectJSONPrintsSnapshotWhenWritingOutput(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	_, err := InspectWithConfig(inspectTestConfigWithRoot(root, "postgres"), InspectOptions{
		URL:    "postgres://example",
		Out:    "inspect.json",
		JSON:   true,
		Stdout: &stdout,
		inspectPG: func(context.Context, string) (pgschema.Schema, error) {
			return inspectTestSchema(), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"snapshotId":`) {
		t.Fatalf("stdout missing snapshot JSON:\n%s", stdout.String())
	}
}

func TestInspectDriftProjectionNormalisesSnapshot(t *testing.T) {
	var stdout bytes.Buffer
	_, err := InspectWithConfig(inspectTestConfig(t, "postgres"), InspectOptions{
		URL:             "postgres://example",
		Stdout:          &stdout,
		DriftProjection: true,
		inspectPG: func(context.Context, string) (pgschema.Schema, error) {
			return inspectTestSchema(), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, unwanted := range []string{`"previousName"`, `::text`} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("drift projection retained %q:\n%s", unwanted, output)
		}
	}
}

func TestInspectRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := InspectWithConfig(inspectTestConfig(t, "postgres"), InspectOptions{
		inspectPG: func(context.Context, string) (pgschema.Schema, error) {
			t.Fatal("inspectPG should not be called")
			return pgschema.Schema{}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "database URL is required") {
		t.Fatalf("expected database URL error, got %v", err)
	}
}

func TestInspectRejectsConflictingTokenProviders(t *testing.T) {
	_, err := InspectWithConfig(inspectTestConfig(t, "postgres"), InspectOptions{
		URL:           "postgres://example",
		TokenCommand:  "print-token",
		AzureCLIToken: true,
		inspectPG: func(context.Context, string) (pgschema.Schema, error) {
			t.Fatal("inspectPG should not be called")
			return pgschema.Schema{}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "only one database token provider") {
		t.Fatalf("expected token provider conflict, got %v", err)
	}
}

func TestInspectRejectsUnknownDialect(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	_, err := InspectWithConfig(inspectTestConfig(t, "oracle"), InspectOptions{
		inspectPG: func(context.Context, string) (pgschema.Schema, error) {
			t.Fatal("inspectPG should not be called")
			return pgschema.Schema{}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), `unknown dialect "oracle"`) {
		t.Fatalf("expected unknown dialect error, got %v", err)
	}
}

func inspectTestSchema() pgschema.Schema {
	return pgschema.Schema{
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:         "users",
				PreviousName: "old_users",
				Columns: []ast.Column{{
					Name:         "email",
					PreviousName: "old_email",
					Type:         "text",
					Default:      "'example@example.com'::text",
				}},
			},
		}},
	}
}

func inspectTestConfig(t *testing.T, dialect string) *Config {
	t.Helper()
	return inspectTestConfigWithRoot(t.TempDir(), dialect)
}

func inspectTestConfigWithRoot(root, dialect string) *Config {
	return &Config{
		Dialect: dialect,
		rootDir: root,
		Schema: SchemaSpec{
			paths: []string{"schema"},
		},
	}
}
