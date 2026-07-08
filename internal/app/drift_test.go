package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
)

func TestDriftCheckWithConfigPassesWhenProjectedSnapshotsMatch(t *testing.T) {
	config := &Config{
		Dialect: "postgres",
		rootDir: mustModuleDir(t),
		Schema: SchemaSpec{
			paths: []string{"./examples/basic/schema"},
		},
	}
	desiredSnapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		t.Fatal(err)
	}
	var desiredDoc pgschema.Document
	if err := json.Unmarshal([]byte(desiredSnapshot), &desiredDoc); err != nil {
		t.Fatal(err)
	}
	inspected := projectDriftDocument(desiredDoc)

	result, err := DriftCheckWithConfig(config, DriftCheckOptions{
		URL: "postgres://localhost/app",
		inspectPG: func(context.Context, string) (pgschema.Schema, error) {
			return inspected, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Drift {
		t.Fatalf("expected no drift, got %#v", result)
	}
	if result.DesiredSnapshotID == "" || result.DatabaseSnapshotID == "" || result.DesiredSnapshotID != result.DatabaseSnapshotID {
		t.Fatalf("result = %#v", result)
	}
}

func TestDriftCheckWithConfigReportsProjectedSnapshotMismatch(t *testing.T) {
	config := &Config{
		Dialect: "postgres",
		rootDir: mustModuleDir(t),
		Schema: SchemaSpec{
			paths: []string{"./examples/basic/schema"},
		},
	}

	result, err := DriftCheckWithConfig(config, DriftCheckOptions{
		URL: "postgres://localhost/app",
		inspectPG: func(context.Context, string) (pgschema.Schema, error) {
			return pgschema.Schema{}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "database schema drift detected") {
		t.Fatalf("expected drift error, got %v", err)
	}
	if result == nil || !result.Drift {
		t.Fatalf("expected drift result, got %#v", result)
	}
}

func TestProjectDriftSchemaIncludesRicherIntrospectedObjects(t *testing.T) {
	cache := int64(10)
	start := int64(1000)
	schema := projectDriftSchema(pgschema.Schema{
		Namespaces: []pgschema.Namespace{{Name: "billing", PreviousName: "old_billing"}},
		Extensions: []pgschema.Extension{{
			Name:         "pgcrypto",
			PreviousName: "old_pgcrypto",
		}},
		Enums: []pgschema.Enum{{
			Schema:       "billing",
			Name:         "invoice_status",
			PreviousName: "old_invoice_status",
			Values:       []string{"draft", "issued"},
		}},
		CompositeTypes: []pgschema.CompositeType{{
			Schema:       "billing",
			Name:         "money",
			PreviousName: "old_money",
			Attributes: []pgschema.CompositeAttribute{
				{Name: "amount", Type: "numeric(10, 2)"},
			},
		}},
		Domains: []pgschema.Domain{{
			Name:         "email",
			PreviousName: "old_email",
			BaseType:     "text",
			Check:        "value ~ '^[^@]+@[^@]+$'",
			NotNull:      true,
		}},
		Sequences: []pgschema.Sequence{{
			Schema:       "billing",
			Name:         "order_number_seq",
			PreviousName: "old_order_number_seq",
			StartWith:    &start,
			Cache:        &cache,
		}},
		Tables: []ast.Table{{
			Name:         "users",
			PreviousName: "old_users",
			Columns: []ast.Column{{
				Name:         "email",
				PreviousName: "old_email",
				Type:         "text",
			}},
			Indexes: []ast.Index{{
				Name:         "users_email_idx",
				PreviousName: "old_users_email_idx",
				Columns:      []ast.IndexColumn{{Expression: "email"}},
				Concurrently: true,
				Only:         true,
			}},
		}},
	})

	if schema.Namespaces[0].PreviousName != "" ||
		schema.Extensions[0].PreviousName != "" ||
		schema.Enums[0].PreviousName != "" ||
		schema.CompositeTypes[0].PreviousName != "" ||
		schema.Domains[0].PreviousName != "" ||
		schema.Sequences[0].PreviousName != "" ||
		schema.Tables[0].PreviousName != "" ||
		schema.Tables[0].Columns[0].PreviousName != "" ||
		schema.Tables[0].Indexes[0].PreviousName != "" {
		t.Fatalf("projected schema retained rename metadata: %#v", schema)
	}
	if len(schema.CompositeTypes) != 1 || len(schema.Domains) != 1 || len(schema.Sequences) != 1 || len(schema.Tables[0].Indexes) != 1 {
		t.Fatalf("projected schema dropped richer introspected objects: %#v", schema)
	}
	if schema.Tables[0].Indexes[0].Concurrently || schema.Tables[0].Indexes[0].Only {
		t.Fatalf("projected index retained non-persistent authoring flags: %#v", schema.Tables[0].Indexes[0])
	}
}

func TestProjectDriftSchemaNormalisesSequenceDefaults(t *testing.T) {
	minValue := int64(1)
	maxValue := int64(9223372036854775807)
	startWith := int64(1)
	cache := int64(1)

	schema := projectDriftSchema(pgschema.Schema{
		Sequences: []pgschema.Sequence{{
			Name:      "events_id_seq",
			Increment: 1,
			MinValue:  &minValue,
			MaxValue:  &maxValue,
			StartWith: &startWith,
			Cache:     &cache,
		}},
	})

	sequence := schema.Sequences[0]
	if sequence.Increment != 0 || sequence.MinValue != nil || sequence.MaxValue != nil || sequence.StartWith != nil || sequence.Cache != nil {
		t.Fatalf("sequence defaults were not normalised: %#v", sequence)
	}
}
