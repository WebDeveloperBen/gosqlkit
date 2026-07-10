package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/webdeveloperben/gosqlkit/internal/ast"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/pgschema"
	"github.com/webdeveloperben/gosqlkit/internal/dialects/pg/render"
	pgtooling "github.com/webdeveloperben/gosqlkit/internal/dialects/pg/tooling"
	"github.com/webdeveloperben/gosqlkit/internal/migrate"
	"github.com/webdeveloperben/gosqlkit/internal/migrate/golangmigrate"
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

		var inspectOutput strings.Builder
		inspectResult, err := InspectWithConfig(config, InspectOptions{
			URL:    dsn,
			Stdout: &inspectOutput,
		})
		if err != nil {
			t.Fatal(err)
		}
		var inspected pgschema.Document
		if err := json.Unmarshal([]byte(inspectOutput.String()), &inspected); err != nil {
			t.Fatalf("parse inspect output: %v\n%s", err, inspectOutput.String())
		}
		if inspectResult == nil || inspectResult.SnapshotID == "" || len(inspected.Tables) == 0 {
			t.Fatalf("inspect result = %#v, document = %#v", inspectResult, inspected)
		}

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
		if result.Sandbox.Applied == 0 || result.Sandbox.ToSnapshotID == "" || result.Sandbox.DatabaseSnapshotID == "" {
			t.Fatalf("sandbox result = %#v", result.Sandbox)
		}
	})

	t.Run("golang-migrate baseline migration replays and matches embedded target snapshot", func(t *testing.T) {
		config := integrationConfig(t)
		config.Migrations.Runner = migrate.RunnerGolangMigrate
		dsn := newPostgresIntegrationDatabase(t, ctx)

		if _, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
			Name:      "baseline",
			CreatedAt: time.Date(2026, 7, 8, 12, 5, 0, 0, time.UTC),
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
		if result.Sandbox.Applied == 0 || result.Sandbox.ToSnapshotID == "" || result.Sandbox.DatabaseSnapshotID == "" {
			t.Fatalf("sandbox result = %#v", result.Sandbox)
		}
	})

	t.Run("migrate apply records goose versions and skips already applied files", func(t *testing.T) {
		dsn := newPostgresIntegrationDatabase(t, ctx)
		config := &Config{
			Dialect: "postgres",
			Migrations: MigrationSpec{
				Dir:    filepath.Join(t.TempDir(), "migrations"),
				Runner: DefaultMigrationsRunner,
			},
		}
		files, err := (goose.Renderer{}).Render(migrate.Plan{
			Name:      "add applied users",
			Dialect:   "postgresql",
			CreatedAt: time.Date(2026, 7, 8, 12, 30, 0, 0, time.UTC),
			Changes:   []migrate.Change{{Op: "manual", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
			UpSQL:     []string{"CREATE TABLE applied_users (id integer PRIMARY KEY);"},
			DownSQL:   []string{"DROP TABLE applied_users;"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(config.Migrations.Dir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(config.Migrations.Dir, files[0].Name), []byte(files[0].Content), 0o600); err != nil {
			t.Fatal(err)
		}

		first, err := MigrateApplyWithConfig(config, MigrateApplyOptions{
			URL:          passwordlessPostgresDSN(t, dsn),
			TokenCommand: writeTokenCommand(t, "postgres", 0),
		})
		if err != nil {
			t.Fatal(err)
		}
		if first.Applied != 1 || first.Skipped != 0 || first.LastFile != files[0].Name {
			t.Fatalf("first apply = %#v", first)
		}

		conn := openPostgres(t, ctx, dsn)
		defer func() {
			_ = conn.Close(context.Background())
		}()
		if err := conn.Exec(ctx, "INSERT INTO applied_users (id) VALUES (1);"); err != nil {
			t.Fatal(err)
		}
		assertGooseVersionApplied(t, ctx, conn, 20260708123000)

		second, err := MigrateApplyWithConfig(config, MigrateApplyOptions{URL: dsn})
		if err != nil {
			t.Fatal(err)
		}
		if second.Applied != 0 || second.Skipped != 1 || second.LastFile != "" {
			t.Fatalf("second apply = %#v", second)
		}
	})

	t.Run("migrate apply records golang-migrate state and skips current version", func(t *testing.T) {
		dsn := newPostgresIntegrationDatabase(t, ctx)
		config := &Config{
			Dialect: "postgres",
			Migrations: MigrationSpec{
				Dir:    filepath.Join(t.TempDir(), "migrations"),
				Runner: migrate.RunnerGolangMigrate,
			},
		}
		files, err := (golangmigrate.Renderer{}).Render(migrate.Plan{
			Name:      "add applied users",
			Dialect:   "postgresql",
			CreatedAt: time.Date(2026, 7, 8, 12, 35, 0, 0, time.UTC),
			Changes:   []migrate.Change{{Op: "manual", Object: migrate.ObjectRef{Kind: "schema", Key: "schema"}}},
			UpSQL:     []string{"CREATE TABLE applied_users (id integer PRIMARY KEY);"},
			DownSQL:   []string{"DROP TABLE applied_users;"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(config.Migrations.Dir, 0o750); err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if err := os.WriteFile(filepath.Join(config.Migrations.Dir, file.Name), []byte(file.Content), 0o600); err != nil {
				t.Fatal(err)
			}
		}

		first, err := MigrateApplyWithConfig(config, MigrateApplyOptions{URL: dsn})
		if err != nil {
			t.Fatal(err)
		}
		if first.Applied != 1 || first.Skipped != 0 || first.LastFile != files[0].Name {
			t.Fatalf("first apply = %#v", first)
		}

		conn := openPostgres(t, ctx, dsn)
		defer func() {
			_ = conn.Close(context.Background())
		}()
		if err := conn.Exec(ctx, "INSERT INTO applied_users (id) VALUES (1);"); err != nil {
			t.Fatal(err)
		}
		assertGolangMigrateVersion(t, ctx, conn, 20260708123500, false)

		second, err := MigrateApplyWithConfig(config, MigrateApplyOptions{URL: dsn})
		if err != nil {
			t.Fatal(err)
		}
		if second.Applied != 0 || second.Skipped != 1 || second.LastFile != "" {
			t.Fatalf("second apply = %#v", second)
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
		if schema.Extensions[0].Version == "" {
			t.Fatalf("extension version was not introspected: %#v", schema.Extensions[0])
		}
		if len(schema.Functions) != 0 || len(schema.CompositeTypes) != 0 || len(schema.Domains) != 0 || len(schema.Sequences) != 0 || len(schema.Tables) != 0 {
			t.Fatalf("extension-owned objects leaked into schema: %#v", schema)
		}
	})
}

func TestPostgresIntegrationGolangMigrateDiffReplay(t *testing.T) {
	requireIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	previous := integrationUsersSchema("users", "email")
	current := integrationUsersSchema("users", "email", ast.Column{Name: "display_name", Type: "text"})
	previousSnapshot, previousID := integrationSnapshot(t, previous)
	currentSnapshot, currentID := integrationSnapshot(t, current)
	previousMigration := migrate.Migration{Metadata: migrate.Metadata{
		ToSnapshotID:   previousID,
		TargetSnapshot: []byte(previousSnapshot),
	}}

	dir := t.TempDir()
	baselineSQL, err := render.Postgres(previous)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationPlan(t, dir, golangmigrate.Renderer{}, migrate.Plan{
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
		Name:      "add_profile_column",
		CreatedAt: time.Date(2026, 7, 8, 12, 1, 0, 0, time.UTC),
	}, previousMigration, currentSnapshot, currentID)
	if err != nil {
		t.Fatal(err)
	}
	writeIntegrationPlan(t, dir, golangmigrate.Renderer{}, diffPlan)

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
		Runner:     migrate.RunnerGolangMigrate,
		Migrations: migrations,
	}); err != nil {
		t.Fatal(err)
	}
	assertDatabaseMatchesSnapshot(t, ctx, conn, currentSnapshot)
}

func TestPostgresIntegrationExternalGolangMigrateDockerCLI(t *testing.T) {
	requireIntegration(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	config := integrationConfig(t)
	config.Migrations.Runner = migrate.RunnerGolangMigrate
	dsn := newPostgresIntegrationDatabase(t, ctx)
	created, err := MigrateCreateWithConfig(config, MigrateCreateOptions{
		Name:      "baseline",
		CreatedAt: time.Date(2026, 7, 8, 12, 10, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedVersion := lastGeneratedUpVersion(t, created.Files)

	containerDSN := dockerContainerPostgresDSN(t, ctx, dsn)
	runGolangMigrateDockerCLI(t, ctx, config.Migrations.Dir, containerDSN, "up")

	conn := openPostgres(t, ctx, dsn)
	defer func() {
		_ = conn.Close(context.Background())
	}()
	snapshot, _, err := renderSnapshotWithConfig(config, "")
	if err != nil {
		t.Fatal(err)
	}
	assertDatabaseMatchesSnapshot(t, ctx, conn, snapshot)
	assertGolangMigrateVersion(t, ctx, conn, expectedVersion, false)
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
			writeIntegrationPlan(t, dir, goose.Renderer{}, migrate.Plan{
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
			diffContent := writeIntegrationPlan(t, dir, goose.Renderer{}, diffPlan)

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

func dockerContainerPostgresDSN(t *testing.T, ctx context.Context, dsn string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if integrationPostgresEnv == nil || integrationPostgresEnv.container == nil {
		t.Fatal("PostgreSQL integration container is not initialised")
	}
	ip, err := integrationPostgresEnv.container.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("read PostgreSQL container IP: %v", err)
	}
	parsed.Host = ip + ":5432"
	query := parsed.Query()
	query.Set("sslmode", "disable")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func runGolangMigrateDockerCLI(t *testing.T, ctx context.Context, migrationDir string, databaseURL string, command string) {
	t.Helper()
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "migrate/migrate:v4.18.3",
			Cmd: []string{
				"-path=/migrations",
				"-database", databaseURL,
				command,
			},
			WaitingFor: wait.ForExit().WithExitTimeout(2 * time.Minute),
			HostConfigModifier: func(hc *container.HostConfig) {
				hc.Binds = append(hc.Binds, migrationDir+":/migrations:ro")
			},
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("run golang-migrate Docker CLI: %v", err)
	}
	defer func() {
		_ = ctr.Terminate(context.Background())
	}()
	state, err := ctr.State(ctx)
	if err != nil {
		t.Fatalf("read golang-migrate Docker CLI state: %v", err)
	}
	if state.ExitCode != 0 {
		logs, logErr := ctr.Logs(ctx)
		if logErr != nil {
			t.Fatalf("golang-migrate Docker CLI exited %d; read logs: %v", state.ExitCode, logErr)
		}
		defer func() {
			_ = logs.Close()
		}()
		raw, _ := io.ReadAll(logs)
		t.Fatalf("golang-migrate Docker CLI exited %d:\n%s", state.ExitCode, raw)
	}
}

func passwordlessPostgresDSN(t *testing.T, dsn string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.User == nil {
		return dsn
	}
	parsed.User = url.User(parsed.User.Username())
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

func lastGeneratedUpVersion(t *testing.T, files []string) int64 {
	t.Helper()
	var latest int64 = -1
	for _, file := range files {
		name := filepath.Base(file)
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		version, err := migrate.VersionID(name)
		if err != nil {
			t.Fatal(err)
		}
		if version > latest {
			latest = version
		}
	}
	if latest < 0 {
		t.Fatalf("no up migration files: %#v", files)
	}
	return latest
}

func assertGooseVersionApplied(t *testing.T, ctx context.Context, conn *pgtooling.Conn, version int64) {
	t.Helper()
	rows, err := conn.Query(ctx, "SELECT count(*) FROM goose_db_version WHERE version_id = $1 AND is_applied;", version)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("goose version query returned no rows")
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("goose version %d applied rows = %d, want 1", version, count)
	}
}

func assertGolangMigrateVersion(t *testing.T, ctx context.Context, conn *pgtooling.Conn, version int64, dirty bool) {
	t.Helper()
	rows, err := conn.Query(ctx, "SELECT version, dirty FROM schema_migrations;")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("golang-migrate version query returned no rows")
	}
	var gotVersion int64
	var gotDirty bool
	if err := rows.Scan(&gotVersion, &gotDirty); err != nil {
		t.Fatal(err)
	}
	if rows.Next() {
		t.Fatal("golang-migrate version table returned more than one row")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if gotVersion != version || gotDirty != dirty {
		t.Fatalf("golang-migrate version = (%d, %t), want (%d, %t)", gotVersion, gotDirty, version, dirty)
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
		{name: "tables", got: len(schema.Tables), want: 6},
		{name: "views", got: len(schema.Views), want: 2},
		{name: "materialized views", got: len(schema.MaterializedViews), want: 1},
		{name: "triggers", got: len(schema.Triggers), want: 1},
		{name: "policies", got: len(schema.Policies), want: 1},
		{name: "grants", got: len(schema.Grants), want: 6},
	}
	for _, check := range checks {
		if check.got < check.want {
			t.Fatalf("%s = %d, want at least %d\nschema = %#v", check.name, check.got, check.want, schema)
		}
	}
	var events *pgschema.Table
	for i := range schema.Tables {
		if schema.Tables[i].Name == "events" {
			events = &schema.Tables[i]
			break
		}
	}
	if events == nil || events.Partitioning == nil || events.Partitioning.Strategy != "range" ||
		len(events.Partitioning.Keys) != 1 || events.Partitioning.Keys[0].Expression != "priority" {
		t.Fatalf("events partitioning = %#v", events)
	}
	var eventsPriorityLow *pgschema.Table
	for i := range schema.Tables {
		if schema.Tables[i].Name == "events_priority_low" {
			eventsPriorityLow = &schema.Tables[i]
			break
		}
	}
	if eventsPriorityLow == nil || eventsPriorityLow.PartitionOf == nil || eventsPriorityLow.PartitionOf.Parent != "public.events" ||
		eventsPriorityLow.PartitionOf.Bound.Type != "range" || len(eventsPriorityLow.PartitionOf.Bound.From) != 1 ||
		eventsPriorityLow.PartitionOf.Bound.From[0] != "0" {
		t.Fatalf("events_priority_low partition = %#v", eventsPriorityLow)
	}
	hasUserReaderGrant := false
	for _, grant := range schema.Grants {
		if grant.Target.Type == "table" && grant.Target.Name == "public.users" &&
			len(grant.Grantees) == 1 && grant.Grantees[0] == "app_reader" &&
			len(grant.Privileges) == 1 && grant.Privileges[0].Name == "SELECT" {
			hasUserReaderGrant = true
			break
		}
	}
	if !hasUserReaderGrant {
		t.Fatalf("missing users app_reader SELECT grant in %#v", schema.Grants)
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
	return pgschema.Schema{Tables: []pgschema.Table{{
		Table: ast.Table{
			Name:    tableName,
			Columns: columns,
		},
	}}}
}

func integrationRenamableTableSchema(tableName, previousName string) pgschema.Schema {
	table := pgschema.Table{
		Table: ast.Table{
			Name: tableName,
			Columns: []ast.Column{
				{Name: "id", Type: "integer"},
				{Name: "email", Type: "text", NotNull: true},
			},
		},
	}
	table.PreviousName = previousName
	return pgschema.Schema{Tables: []pgschema.Table{table}}
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

func writeIntegrationPlan(t *testing.T, dir string, renderer migrate.Renderer, plan migrate.Plan) string {
	t.Helper()
	files, err := renderer.Render(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		path := filepath.Join(dir, file.Name)
		if err := os.WriteFile(path, []byte(file.Content), 0o600); err != nil {
			t.Fatal(err)
		}
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
