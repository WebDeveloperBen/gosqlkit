package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/render"
	pgtooling "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/tooling"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/goose"
)

var integrationPostgresEnv *postgresIntegrationEnv

type postgresIntegrationEnv struct {
	container testcontainers.Container
	adminDSN  string
	mu        sync.Mutex
	next      int
}

func TestMain(m *testing.M) {
	if os.Getenv("GOSQLKIT_INTEGRATION") != "1" {
		os.Exit(m.Run())
	}

	ctx := context.Background()
	image := postgresIntegrationImage()
	env, err := setupPostgresIntegration(ctx, image)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup PostgreSQL integration container %s: %v\n", image, err)
		os.Exit(1)
	}
	integrationPostgresEnv = env

	code := m.Run()
	if err := env.container.Terminate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to terminate PostgreSQL integration container: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func TestPostgresIntegrationWorkflow(t *testing.T) {
	requireIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	config := integrationConfig(t)

	t.Run("generated schema introspects without drift and detects drift", func(t *testing.T) {
		dsn := newPostgresIntegrationDatabase(t, ctx)
		conn := openPostgres(t, ctx, dsn)
		defer func() {
			_ = conn.Close(context.Background())
		}()

		sql, _, err := renderSQLWithConfig(config)
		if err != nil {
			t.Fatal(err)
		}
		execSQLStatements(t, ctx, conn, sql)

		schema, err := pgtooling.Introspect(ctx, conn)
		if err != nil {
			t.Fatal(err)
		}
		assertExampleSchemaIntrospected(t, schema)

		result, err := DriftCheckWithConfig(config, DriftCheckOptions{URL: dsn})
		if err != nil {
			t.Logf("desired drift projection:\n%s", desiredDriftProjection(t, config))
			t.Logf("database drift projection:\n%s", databaseDriftProjection(t, schema))
			t.Fatal(err)
		}
		if result == nil || result.Drift {
			t.Fatalf("drift result = %#v", result)
		}

		if err := conn.Exec(ctx, "ALTER TABLE users ADD COLUMN drift_marker text;"); err != nil {
			t.Fatal(err)
		}
		result, err = DriftCheckWithConfig(config, DriftCheckOptions{URL: dsn})
		if err == nil || !strings.Contains(err.Error(), "database schema drift detected") {
			t.Fatalf("expected drift error, got result=%#v err=%v", result, err)
		}
	})

	t.Run("baseline migration replays and matches embedded target snapshot", func(t *testing.T) {
		config := integrationConfig(t)
		dsn := newPostgresIntegrationDatabase(t, ctx)

		if _, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
			Name:      "baseline",
			CreatedAt: time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatal(err)
		}

		result, err := MigrateCheckWithConfig(config, MigrateCheckOptions{SandboxURL: dsn})
		if err != nil {
			t.Fatal(err)
		}
		if result == nil || result.Sandbox == nil {
			t.Fatalf("missing sandbox result: %#v", result)
		}
		if result.Sandbox.Applied != 1 || result.Sandbox.ToSnapshotID == "" || result.Sandbox.DatabaseSnapshotID == "" {
			t.Fatalf("sandbox result = %#v", result.Sandbox)
		}
	})

	t.Run("extension owned objects are filtered", func(t *testing.T) {
		dsn := newPostgresIntegrationDatabase(t, ctx)
		conn := openPostgres(t, ctx, dsn)
		defer func() {
			_ = conn.Close(context.Background())
		}()

		if err := conn.Exec(ctx, "CREATE EXTENSION pgcrypto;"); err != nil {
			t.Fatal(err)
		}
		schema, err := pgtooling.Introspect(ctx, conn)
		if err != nil {
			t.Fatal(err)
		}
		if len(schema.Extensions) != 1 || schema.Extensions[0].Name != "pgcrypto" {
			t.Fatalf("extensions = %#v", schema.Extensions)
		}
		if len(schema.Functions) != 0 || len(schema.CompositeTypes) != 0 || len(schema.Domains) != 0 || len(schema.Sequences) != 0 || len(schema.Tables) != 0 {
			t.Fatalf("extension-owned objects leaked into schema: %#v", schema)
		}
	})
}

func TestPostgresIntegrationDiffMigrationReplay(t *testing.T) {
	requireIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	tests := []struct {
		name         string
		migration    string
		wantGuardErr string
		current      pgschema.Schema
		previous     pgschema.Schema
		replayDown   bool
		destructive  bool
	}{
		{
			name:       "additive diff",
			migration:  "add_profile_column",
			previous:   integrationUsersSchema("users", "email"),
			current:    integrationUsersSchema("users", "email", ast.Column{Name: "display_name", Type: "text"}),
			replayDown: true,
		},
		{
			name:       "table rename",
			migration:  "rename_users_table",
			previous:   integrationRenamableTableSchema("old_users", ""),
			current:    integrationRenamableTableSchema("users", "old_users"),
			replayDown: true,
		},
		{
			name:       "column rename",
			migration:  "rename_email_column",
			previous:   integrationUsersSchema("users", "old_email"),
			current:    integrationRenamedColumnSchema(),
			replayDown: true,
		},
		{
			name:         "destructive drop column",
			migration:    "drop_legacy_email",
			previous:     integrationUsersSchema("users", "email", ast.Column{Name: "legacy_email", Type: "text"}),
			current:      integrationUsersSchema("users", "email"),
			destructive:  true,
			wantGuardErr: "pass --allow-destructive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previousSnapshot, previousID := integrationSnapshot(t, tt.previous)
			currentSnapshot, currentID := integrationSnapshot(t, tt.current)
			previousMigration := migrate.Migration{Metadata: migrate.Metadata{
				ToSnapshotID:   previousID,
				TargetSnapshot: []byte(previousSnapshot),
			}}

			if tt.wantGuardErr != "" {
				_, err := diffMigrationPlan(&Config{Dialect: "postgresql"}, MigrateCreateOptions{
					Name: tt.migration,
				}, previousMigration, currentSnapshot, currentID)
				if err == nil || !strings.Contains(err.Error(), tt.wantGuardErr) {
					t.Fatalf("expected guard error containing %q, got %v", tt.wantGuardErr, err)
				}
			}

			dir := t.TempDir()
			baselineSQL, err := render.Postgres(tt.previous)
			if err != nil {
				t.Fatal(err)
			}
			writeIntegrationPlan(t, dir, migrate.Plan{
				Name:           "baseline",
				Dialect:        "postgresql",
				ToSnapshotID:   previousID,
				TargetSnapshot: previousSnapshot,
				CreatedAt:      time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
				Changes:        []migrate.Change{{Op: "baseline", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
				UpStatements:   []migrate.Statement{{SQL: baselineSQL}},
				DownStatements: nil,
			})

			diffPlan, err := diffMigrationPlan(&Config{Dialect: "postgresql"}, MigrateCreateOptions{
				Name:             tt.migration,
				CreatedAt:        time.Date(2026, 7, 8, 12, 1, 0, 0, time.UTC),
				AllowDestructive: tt.destructive,
			}, previousMigration, currentSnapshot, currentID)
			if err != nil {
				t.Fatal(err)
			}
			diffContent := writeIntegrationPlan(t, dir, diffPlan)

			migrations, err := migrate.ScanDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			dsn := newPostgresIntegrationDatabase(t, ctx)
			conn := openPostgres(t, ctx, dsn)
			defer func() {
				_ = conn.Close(context.Background())
			}()

			if _, err := pgtooling.Replay(ctx, conn, pgtooling.ReplayOptions{
				Runner:     DefaultMigrationsRunner,
				Migrations: migrations,
			}); err != nil {
				t.Fatal(err)
			}
			assertDatabaseMatchesSnapshot(t, ctx, conn, currentSnapshot)

			if tt.replayDown {
				downStatements, err := goose.DownStatements(diffContent)
				if err != nil {
					t.Fatal(err)
				}
				execStatements(t, ctx, conn, downStatements)
				assertDatabaseMatchesSnapshot(t, ctx, conn, previousSnapshot)
			}
		})
	}
}

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("GOSQLKIT_INTEGRATION") != "1" {
		t.Skip("set GOSQLKIT_INTEGRATION=1 to run Docker-backed integration tests")
	}
}

func integrationConfig(t *testing.T) *Config {
	t.Helper()
	return &Config{
		Dialect: "postgres",
		rootDir: mustModuleDir(t),
		Schema: SchemaSpec{
			paths: []string{"./examples/basic/schema"},
		},
		Migrations: MigrationSpec{
			Dir:    filepath.Join(t.TempDir(), "migrations"),
			Runner: DefaultMigrationsRunner,
		},
	}
}

func setupPostgresIntegration(ctx context.Context, image string) (*postgresIntegrationEnv, error) {
	container, err := postgres.Run(
		ctx,
		image,
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, err
	}
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, err
	}
	return &postgresIntegrationEnv{container: container, adminDSN: dsn}, nil
}

func postgresIntegrationImage() string {
	if image := strings.TrimSpace(os.Getenv("GOSQLKIT_POSTGRES_IMAGE")); image != "" {
		return image
	}
	return "postgres:16-alpine"
}

func newPostgresIntegrationDatabase(t *testing.T, ctx context.Context) string {
	t.Helper()
	if integrationPostgresEnv == nil {
		t.Fatal("PostgreSQL integration container is not initialised")
	}
	name := integrationPostgresEnv.nextDatabaseName()
	admin := openPostgres(t, ctx, integrationPostgresEnv.adminDSN)
	defer func() {
		_ = admin.Close(context.Background())
	}()
	initialRoles := integrationRoleNames(t, ctx, admin)
	if err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		conn, err := pgtooling.Open(cleanupCtx, integrationPostgresEnv.adminDSN)
		if err != nil {
			t.Logf("open PostgreSQL admin connection for cleanup: %v", err)
			return
		}
		defer func() {
			_ = conn.Close(context.Background())
		}()
		_ = conn.Exec(cleanupCtx, fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s';", name))
		if err := conn.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+name); err != nil {
			t.Logf("drop PostgreSQL integration database %s: %v", name, err)
		}
		dropIntegrationRoles(t, cleanupCtx, conn, initialRoles)
	})
	return databaseDSN(t, integrationPostgresEnv.adminDSN, name)
}

func (env *postgresIntegrationEnv) nextDatabaseName() string {
	env.mu.Lock()
	defer env.mu.Unlock()
	env.next++
	return fmt.Sprintf("gosqlkit_test_%d", env.next)
}

func databaseDSN(t *testing.T, dsn, name string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + name
	return parsed.String()
}

func integrationRoleNames(t *testing.T, ctx context.Context, conn *pgtooling.Conn) map[string]struct{} {
	t.Helper()
	rows, err := conn.Query(ctx, `
SELECT rolname
FROM pg_roles
WHERE rolname NOT LIKE 'pg_%'
  AND rolname <> 'postgres'
ORDER BY rolname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		out[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func dropIntegrationRoles(t *testing.T, ctx context.Context, conn *pgtooling.Conn, initial map[string]struct{}) {
	t.Helper()
	current := integrationRoleNames(t, ctx, conn)
	var extra []string
	for name := range current {
		if _, ok := initial[name]; !ok && safeIntegrationIdentifier(name) {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	for _, role := range extra {
		for _, member := range extra {
			if role == member {
				continue
			}
			_ = conn.Exec(ctx, "REVOKE "+role+" FROM "+member+";")
		}
	}
	for i := len(extra) - 1; i >= 0; i-- {
		role := extra[i]
		_ = conn.Exec(ctx, "DROP OWNED BY "+role+";")
		if err := conn.Exec(ctx, "DROP ROLE IF EXISTS "+role+";"); err != nil {
			t.Logf("drop PostgreSQL integration role %s: %v", role, err)
		}
	}
}

func safeIntegrationIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if i == 0 {
			if (r < 'a' || r > 'z') && r != '_' {
				return false
			}
			continue
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func openPostgres(t *testing.T, ctx context.Context, dsn string) *pgtooling.Conn {
	t.Helper()
	conn, err := pgtooling.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func execSQLStatements(t *testing.T, ctx context.Context, conn *pgtooling.Conn, sql string) {
	t.Helper()
	for _, statement := range pgtooling.SplitSQLStatements(sql) {
		if err := conn.Exec(ctx, statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
}

func execStatements(t *testing.T, ctx context.Context, conn *pgtooling.Conn, statements []string) {
	t.Helper()
	for _, statement := range statements {
		if err := conn.Exec(ctx, statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
}

func assertExampleSchemaIntrospected(t *testing.T, schema pgschema.Schema) {
	t.Helper()
	checks := []struct {
		name string
		got  int
		want int
	}{
		{name: "namespaces", got: len(schema.Namespaces), want: 1},
		{name: "extensions", got: len(schema.Extensions), want: 1},
		{name: "roles", got: len(schema.Roles), want: 2},
		{name: "enums", got: len(schema.Enums), want: 1},
		{name: "composite types", got: len(schema.CompositeTypes), want: 1},
		{name: "domains", got: len(schema.Domains), want: 1},
		{name: "sequences", got: len(schema.Sequences), want: 1},
		{name: "functions", got: len(schema.Functions), want: 2},
		{name: "tables", got: len(schema.Tables), want: 4},
		{name: "views", got: len(schema.Views), want: 2},
		{name: "materialized views", got: len(schema.MaterializedViews), want: 1},
		{name: "triggers", got: len(schema.Triggers), want: 1},
		{name: "policies", got: len(schema.Policies), want: 1},
	}
	for _, check := range checks {
		if check.got < check.want {
			t.Fatalf("%s = %d, want at least %d\nschema = %#v", check.name, check.got, check.want, schema)
		}
	}
}

func desiredDriftProjection(t *testing.T, config *Config) string {
	t.Helper()
	snapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		t.Fatal(err)
	}
	var doc pgschema.Document
	if err := json.Unmarshal([]byte(snapshot), &doc); err != nil {
		t.Fatal(err)
	}
	return driftProjectionJSON(t, projectDriftDocument(doc))
}

func databaseDriftProjection(t *testing.T, schema pgschema.Schema) string {
	t.Helper()
	return driftProjectionJSON(t, projectDriftSchema(schema))
}

func driftProjectionJSON(t *testing.T, schema pgschema.Schema) string {
	t.Helper()
	raw, err := pgschema.JSON("postgresql", schema)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func integrationUsersSchema(tableName, emailColumn string, extraColumns ...ast.Column) pgschema.Schema {
	columns := []ast.Column{
		{Name: "id", Type: "integer", PrimaryKey: true},
		{Name: emailColumn, Type: "text", NotNull: true},
	}
	columns = append(columns, extraColumns...)
	return pgschema.Schema{Tables: []ast.Table{{
		Name:    tableName,
		Columns: columns,
	}}}
}

func integrationRenamableTableSchema(tableName, previousName string) pgschema.Schema {
	table := ast.Table{
		Name: tableName,
		Columns: []ast.Column{
			{Name: "id", Type: "integer"},
			{Name: "email", Type: "text", NotNull: true},
		},
	}
	table.PreviousName = previousName
	return pgschema.Schema{Tables: []ast.Table{table}}
}

func integrationRenamedColumnSchema() pgschema.Schema {
	schema := integrationUsersSchema("users", "email")
	schema.Tables[0].Columns[1].PreviousName = "old_email"
	return schema
}

func integrationSnapshot(t *testing.T, schema pgschema.Schema) (string, string) {
	t.Helper()
	raw, err := pgschema.JSON("postgresql", schema)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := injectSnapshotIDs(string(raw), "")
	if err != nil {
		t.Fatal(err)
	}
	snapshotID, err := snapshotIDFromJSON(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, snapshotID
}

func writeIntegrationPlan(t *testing.T, dir string, plan migrate.Plan) string {
	t.Helper()
	files, err := (goose.Renderer{}).Render(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %#v", files)
	}
	path := filepath.Join(dir, files[0].Name)
	if err := os.WriteFile(path, []byte(files[0].Content), 0o600); err != nil {
		t.Fatal(err)
	}
	return files[0].Content
}

func assertDatabaseMatchesSnapshot(t *testing.T, ctx context.Context, conn *pgtooling.Conn, snapshot string) {
	t.Helper()
	var target pgschema.Document
	if err := json.Unmarshal([]byte(snapshot), &target); err != nil {
		t.Fatal(err)
	}
	targetID, err := driftSnapshotID(projectDriftDocument(target))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := pgtooling.Introspect(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	databaseID, err := driftSnapshotID(projectDriftSchema(schema))
	if err != nil {
		t.Fatal(err)
	}
	if databaseID != targetID {
		t.Fatalf("database snapshot %q does not match target snapshot %q\ntarget:\n%s\ndatabase:\n%s",
			databaseID, targetID, driftProjectionJSON(t, projectDriftDocument(target)), databaseDriftProjection(t, schema))
	}
}
