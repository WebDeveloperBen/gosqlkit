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
	if len(result.Differences) == 0 {
		t.Fatalf("expected drift differences, got %#v", result)
	}
}

func TestDriftCheckWithConfigUsesDatabaseURLEnv(t *testing.T) {
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

	t.Setenv("GOSQLKIT_DATABASE_URL", "postgres://localhost/app")
	var gotURL string
	_, err = DriftCheckWithConfig(config, DriftCheckOptions{
		URLEnv: "GOSQLKIT_DATABASE_URL",
		inspectPG: func(_ context.Context, databaseURL string) (pgschema.Schema, error) {
			gotURL = databaseURL
			return projectDriftDocument(desiredDoc), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "postgres://localhost/app" {
		t.Fatalf("database URL = %q", gotURL)
	}
}

func TestProjectDriftSchemaIgnoresMigrationRunnerTables(t *testing.T) {
	schema := projectDriftSchema(pgschema.Schema{
		Tables: []pgschema.Table{
			{Table: ast.Table{Name: "users"}},
			{Table: ast.Table{Name: "goose_db_version"}},
			{Table: ast.Table{Name: "schema_migrations"}},
		},
	})
	if len(schema.Tables) != 1 || schema.Tables[0].Name != "users" {
		t.Fatalf("tables = %#v", schema.Tables)
	}
}

func TestProjectDriftSchemaIncludesRicherIntrospectedObjects(t *testing.T) {
	cache := int64(10)
	start := int64(1000)
	deterministic := true
	schema := projectDriftSchema(pgschema.Schema{
		Namespaces: []pgschema.Namespace{{Name: "billing", PreviousName: "old_billing"}},
		Extensions: []pgschema.Extension{{
			Name:         "pgcrypto",
			PreviousName: "old_pgcrypto",
			Version:      "1.3",
			Cascade:      true,
			Comment:      "Cryptographic functions",
		}},
		Collations: []pgschema.Collation{{
			Name:          "stable_text",
			PreviousName:  "old_stable_text",
			Provider:      "libc",
			Locale:        "C",
			Deterministic: &deterministic,
			Version:       "2.36",
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
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name:         "users",
				PreviousName: "old_users",
				Columns: []ast.Column{{
					Name:         "email",
					PreviousName: "old_email",
					Type:         "text",
				}},
			},
			Indexes: []pgschema.Index{{
				Index: ast.Index{
					Name:         "users_email_idx",
					PreviousName: "old_users_email_idx",
				},
				Columns:      []pgschema.IndexColumn{{IndexColumn: ast.IndexColumn{Expression: "email"}}},
				Concurrently: true,
				Only:         true,
			}},
		}},
	})

	if schema.Namespaces[0].PreviousName != "" ||
		schema.Extensions[0].PreviousName != "" ||
		schema.Collations[0].PreviousName != "" ||
		schema.Enums[0].PreviousName != "" ||
		schema.CompositeTypes[0].PreviousName != "" ||
		schema.Domains[0].PreviousName != "" ||
		schema.Sequences[0].PreviousName != "" ||
		schema.Tables[0].PreviousName != "" ||
		schema.Tables[0].Columns[0].PreviousName != "" ||
		schema.Tables[0].Indexes[0].PreviousName != "" {
		t.Fatalf("projected schema retained rename metadata: %#v", schema)
	}
	if len(schema.Collations) != 1 || len(schema.CompositeTypes) != 1 || len(schema.Domains) != 1 || len(schema.Sequences) != 1 || len(schema.Tables[0].Indexes) != 1 {
		t.Fatalf("projected schema dropped richer introspected objects: %#v", schema)
	}
	if schema.Extensions[0].Version != "1.3" {
		t.Fatalf("projected extension dropped version: %#v", schema.Extensions[0])
	}
	if schema.Extensions[0].Comment != "Cryptographic functions" {
		t.Fatalf("projected extension dropped comment: %#v", schema.Extensions[0])
	}
	if schema.Extensions[0].Cascade {
		t.Fatalf("projected extension retained authoring-only cascade metadata: %#v", schema.Extensions[0])
	}
	if schema.Collations[0].Provider != "" || schema.Collations[0].Version != "" || schema.Collations[0].Deterministic != nil {
		t.Fatalf("projected collation retained default introspection fields: %#v", schema.Collations[0])
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

func TestProjectDriftSchemaNormalisesPostgreSQLExpressionRewrites(t *testing.T) {
	projected := projectDriftSchema(pgschema.Schema{
		Domains: []pgschema.Domain{{
			Name:     "email",
			BaseType: "character varying (320)",
			Default:  "'pending'::text",
			Check:    "VALUE ~ '^[^@]+@[^@]+$'::text",
		}},
		Tables: []pgschema.Table{{
			Table: ast.Table{
				Name: "events",
				Columns: []ast.Column{
					{Name: "metadata", Type: "jsonb", Default: `'{"source": "api"}'::jsonb`},
					{Name: "search_vector", Type: "text", Generated: &ast.Generated{As: "to_tsvector('english'::regconfig, description)", Type: "stored"}},
				},
				Checks: []ast.Check{{Name: "events_metadata_source", Expression: "(metadata ->> 'source'::text) = 'api'::text"}},
			},
		}},
		Policies: []pgschema.Policy{{
			Name:    "events_read_self",
			Table:   "events",
			Command: "ALL",
			Mode:    "PERMISSIVE",
			Using:   "user_id = (current_setting('app.user_id'::text))::uuid",
			Roles:   []string{"public"},
		}},
		Triggers: []pgschema.Trigger{{
			Name:     "events_touch",
			Target:   "events",
			Function: "touch_events",
			Level:    "STATEMENT",
			When:     "OLD.* IS DISTINCT FROM NEW.*",
		}},
	})

	domain := projected.Domains[0]
	if domain.BaseType != "character varying(320)" || domain.Default != "'pending'" || domain.Check != "value ~ '^[^@]+@[^@]+$'" {
		t.Fatalf("domain normalisation = %#v", domain)
	}
	table := projected.Tables[0]
	if table.Columns[0].Default != `'{"source":"api"}'` {
		t.Fatalf("json default = %q", table.Columns[0].Default)
	}
	if table.Columns[1].Generated.As != "to_tsvector('english', description)" {
		t.Fatalf("generated expression = %#v", table.Columns[1].Generated)
	}
	if table.Checks[0].Expression != "(metadata ->> 'source') = 'api'" {
		t.Fatalf("check expression = %q", table.Checks[0].Expression)
	}
	policy := projected.Policies[0]
	if policy.Table != "public.events" || policy.Command != "" || policy.Mode != "" || len(policy.Roles) != 0 || policy.Using != "user_id = current_setting('app.user_id')" {
		t.Fatalf("policy normalisation = %#v", policy)
	}
	trigger := projected.Triggers[0]
	if trigger.Target != "public.events" || trigger.Function != "public.touch_events" || trigger.Level != "" || trigger.When != "old.* IS DISTINCT FROM new.*" {
		t.Fatalf("trigger normalisation = %#v", trigger)
	}
}

func TestProjectDriftSchemaNormalisesPartitionMetadata(t *testing.T) {
	projected := projectDriftSchema(pgschema.Schema{
		Tables: []pgschema.Table{
			{
				Table: ast.Table{Name: "events"},
				Partitioning: &pgschema.Partitioning{
					Strategy: "hash",
					Keys: []pgschema.PartitionKey{{
						Expression:   "lower(email)::text",
						IsExpression: true,
					}},
				},
			},
			{
				Table: ast.Table{Name: "events_0"},
				PartitionOf: &pgschema.PartitionOf{
					Parent: "events",
					Bound: pgschema.PartitionBound{
						Type:   "list",
						Values: []string{"'draft'::text"},
					},
				},
			},
		},
	})

	parent := projected.Tables[0]
	if parent.Partitioning.Keys[0].Expression != "lower(email)" {
		t.Fatalf("partition key = %#v", parent.Partitioning.Keys[0])
	}
	child := projected.Tables[1]
	if child.PartitionOf.Parent != "public.events" || child.PartitionOf.Bound.Values[0] != "'draft'" {
		t.Fatalf("partition child = %#v", child.PartitionOf)
	}
}

func TestProjectDriftSchemaNormalisesIndexes(t *testing.T) {
	projected := projectDriftSchema(pgschema.Schema{
		Tables: []pgschema.Table{{
			Tablespace: "pg_default",
			Table: ast.Table{
				Name: "users",
			},
			Indexes: []pgschema.Index{{
				Index: ast.Index{
					Name:  "users_email_idx",
					Where: "email IS NOT NULL::boolean",
				},
				Method:     "btree",
				Tablespace: "pg_default",
				Columns: []pgschema.IndexColumn{
					{IndexColumn: ast.IndexColumn{Expression: "email", Order: "ASC"}, OpClass: "text_ops", Nulls: "LAST"},
					{IndexColumn: ast.IndexColumn{Expression: "created_at", Order: "DESC"}, Nulls: "FIRST"},
					{IndexColumn: ast.IndexColumn{Expression: "priority", Order: "DESC"}, Nulls: "LAST"},
				},
				Concurrently: true,
				Only:         true,
			}, {
				Index:  ast.Index{Name: "users_embedding_default_idx"},
				Method: "hnsw",
				Columns: []pgschema.IndexColumn{
					{IndexColumn: ast.IndexColumn{Expression: "embedding"}, OpClass: "vector_l2_ops"},
				},
			}, {
				Index:  ast.Index{Name: "users_embedding_cosine_idx"},
				Method: "hnsw",
				Columns: []pgschema.IndexColumn{
					{IndexColumn: ast.IndexColumn{Expression: "embedding"}, OpClass: "vector_cosine_ops"},
				},
			}},
		}},
	})

	index := projected.Tables[0].Indexes[0]
	defaultVectorIndex := projected.Tables[0].Indexes[1]
	cosineVectorIndex := projected.Tables[0].Indexes[2]
	if projected.Tables[0].Tablespace != "" {
		t.Fatalf("table tablespace normalisation = %#v", projected.Tables[0])
	}
	if index.Method != "" || index.Tablespace != "" || index.Concurrently || index.Only || index.Where != "email IS NOT NULL" {
		t.Fatalf("index normalisation = %#v", index)
	}
	if index.Columns[0].OpClass != "" || index.Columns[0].Order != "" || index.Columns[0].Nulls != "" {
		t.Fatalf("default index column normalisation = %#v", index.Columns[0])
	}
	if index.Columns[1].Order != "DESC" || index.Columns[1].Nulls != "" {
		t.Fatalf("desc default nulls normalisation = %#v", index.Columns[1])
	}
	if index.Columns[2].Order != "DESC" || index.Columns[2].Nulls != "LAST" {
		t.Fatalf("explicit desc nulls last should be retained = %#v", index.Columns[2])
	}
	if defaultVectorIndex.Columns[0].OpClass != "" {
		t.Fatalf("default pgvector opclass should be normalised = %#v", defaultVectorIndex.Columns[0])
	}
	if cosineVectorIndex.Columns[0].OpClass != "vector_cosine_ops" {
		t.Fatalf("non-default pgvector opclass should be retained = %#v", cosineVectorIndex.Columns[0])
	}
}

func TestProjectDriftSchemaCanonicalisesInlineConstraints(t *testing.T) {
	projected := projectDriftSchema(pgschema.Schema{
		Tables: []pgschema.Table{
			{
				Table: ast.Table{
					Name:    "accounts",
					Columns: []ast.Column{{Name: "id", Type: "integer"}},
				},
			},
			{
				Table: ast.Table{
					Name: "users",
					Columns: []ast.Column{
						{Name: "id", Type: "integer", NotNull: true},
						{Name: "email", Type: "text"},
						{Name: "account_id", Type: "integer"},
					},
					PrimaryKeys:       []ast.PrimaryKey{{Name: "users_pkey", Columns: []string{"id"}}},
					UniqueConstraints: []ast.UniqueConstraint{{Name: "users_email_key", Columns: []string{"email"}}},
					ForeignKeys: []ast.ForeignKeyConstraint{{
						Name:              "users_account_id_fkey",
						Columns:           []string{"account_id"},
						ReferencedTable:   "accounts",
						ReferencedColumns: []string{"id"},
						OnDelete:          "CASCADE",
						OnUpdate:          "NO ACTION",
					}},
				},
			},
		},
	})

	users := projected.Tables[1]
	if len(users.PrimaryKeys) != 0 || len(users.UniqueConstraints) != 0 || len(users.ForeignKeys) != 0 {
		t.Fatalf("default single-column constraints were not canonicalised: %#v", users)
	}
	if !users.Columns[0].PrimaryKey || users.Columns[0].NotNull {
		t.Fatalf("primary key column = %#v", users.Columns[0])
	}
	if !users.Columns[1].Unique {
		t.Fatalf("unique column = %#v", users.Columns[1])
	}
	ref := users.Columns[2].References
	if ref == nil || ref.Table != "public.accounts" || ref.Column != "id" || ref.OnDelete != "cascade" || ref.OnUpdate != "no action" {
		t.Fatalf("foreign key column = %#v", users.Columns[2])
	}
}

func TestProjectDriftSchemaNormalisesViewProjection(t *testing.T) {
	projected := projectDriftSchema(pgschema.Schema{
		Views: []pgschema.View{{
			PreviousName:    "old_active_users",
			Name:            "active_users",
			Query:           "SELECT id FROM users WHERE deleted_at IS NULL",
			Comment:         "Active users.",
			CheckOption:     "LOCAL",
			ColumnAliases:   []string{"id"},
			DependsOn:       []string{"public.users"},
			SecurityBarrier: true,
			SecurityInvoker: true,
		}},
		MaterializedViews: []pgschema.MaterializedView{{
			PreviousName:  "old_user_counts",
			Name:          "user_counts",
			Query:         "SELECT count(*) FROM users",
			Comment:       "User count.",
			Tablespace:    "pg_default",
			ColumnAliases: []string{"count"},
			DependsOn:     []string{"public.users"},
			With:          map[string]string{"fillfactor": "80"},
			NoData:        true,
		}},
	})

	view := projected.Views[0]
	if view.PreviousName != "" || view.Query != "" || view.ColumnAliases != nil || view.DependsOn != nil {
		t.Fatalf("view authoring fields were retained: %#v", view)
	}
	if view.Comment != "Active users." || view.CheckOption != "LOCAL" || !view.SecurityBarrier || !view.SecurityInvoker {
		t.Fatalf("view persistent metadata was dropped: %#v", view)
	}

	materializedView := projected.MaterializedViews[0]
	if materializedView.PreviousName != "" || materializedView.Query != "" || materializedView.ColumnAliases != nil || materializedView.DependsOn != nil || materializedView.NoData {
		t.Fatalf("materialized view authoring fields were retained: %#v", materializedView)
	}
	if materializedView.Comment != "User count." || materializedView.Tablespace != "" || materializedView.With["fillfactor"] != "80" {
		t.Fatalf("materialized view persistent metadata was dropped: %#v", materializedView)
	}
}
